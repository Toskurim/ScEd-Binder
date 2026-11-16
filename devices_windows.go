//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Lettura delle periferiche con DirectInput 8 (la stessa API che usa Star Citizen),
// chiamata direttamente da dinput8.dll senza librerie esterne.

type guid struct {
	D1 uint32
	D2 uint16
	D3 uint16
	D4 [8]byte
}

func (g guid) String() string {
	return fmt.Sprintf("{%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}",
		g.D1, g.D2, g.D3, g.D4[0], g.D4[1], g.D4[2], g.D4[3], g.D4[4], g.D4[5], g.D4[6], g.D4[7])
}

func axisGUID(d1 uint32) guid {
	return guid{d1, 0xC9F3, 0x11CF, [8]byte{0xBF, 0xC7, 0x44, 0x45, 0x53, 0x54, 0x00, 0x00}}
}

var (
	iidIDirectInput8W = guid{0xBF798031, 0x483A, 0x4DA2, [8]byte{0xAA, 0x99, 0x5D, 0x64, 0xED, 0x36, 0x97, 0x00}}
	guidXAxis         = axisGUID(0xA36D02E0)
	guidYAxis         = axisGUID(0xA36D02E1)
	guidZAxis         = axisGUID(0xA36D02E2)
	guidRxAxis        = axisGUID(0xA36D02F4)
	guidRyAxis        = axisGUID(0xA36D02F5)
	guidRzAxis        = axisGUID(0xA36D02E3)
	guidSlider        = axisGUID(0xA36D02E4)
	guidPOV           = axisGUID(0xA36D02F2)
)

const (
	diVersion          = 0x0800
	di8DevClassGameCtl = 4
	diEDFLAttachedOnly = 1
	diDFTAxis          = 0x00000003
	diDFTButton        = 0x0000000C
	diDFTPOV           = 0x00000010
	diDFTAnyInstance   = 0x00FFFF00
	diDFTOptional      = 0x80000000
	diDOIAspectPos     = 0x00000100
	diDFAbsAxis        = 1
	diSCLNonExclusive  = 2
	diSCLBackground    = 8
	diPHDevice         = 0
	diPHByOffset       = 1
	diPropGuidAndPath  = 12
	joyStateSize       = 272
)

type diObjectDataFormat struct {
	pguid   *guid
	dwOfs   uint32
	dwType  uint32
	dwFlags uint32
}

type diDataFormat struct {
	dwSize     uint32
	dwObjSize  uint32
	dwFlags    uint32
	dwDataSize uint32
	dwNumObjs  uint32
	rgodf      *diObjectDataFormat
}

type diJoyState2 struct {
	Axes    [6]int32
	Sliders [2]int32
	POV     [4]uint32
	Buttons [128]byte
	Rest    [24]int32
}

type diDevCaps struct {
	dwSize, dwFlags, dwDevType, dwAxes, dwButtons, dwPOVs     uint32
	dwFFSamplePeriod, dwFFMinTimeResolution                   uint32
	dwFirmwareRevision, dwHardwareRevision, dwFFDriverVersion uint32
}

type diDeviceInstanceW struct {
	dwSize          uint32
	guidInstance    guid
	guidProduct     guid
	dwDevType       uint32
	tszInstanceName [260]uint16
	tszProductName  [260]uint16
	guidFFDriver    guid
	wUsagePage      uint16
	wUsage          uint16
}

type diDeviceObjectInstanceW struct {
	dwSize              uint32
	guidType            guid
	dwOfs               uint32
	dwType              uint32
	dwFlags             uint32
	tszName             [260]uint16
	dwFFMaxForce        uint32
	dwFFForceResolution uint32
	wCollectionNumber   uint16
	wDesignatorIndex    uint16
	wUsagePage          uint16
	wUsage              uint16
	dwDimension         uint32
	wExponent           uint16
	wReportId           uint16
}

type diPropGuidAndPathT struct {
	dwSize       uint32
	dwHeaderSize uint32
	dwObj        uint32
	dwHow        uint32
	guidClass    guid
	wszPath      [260]uint16
}

var (
	modDinput8             = syscall.NewLazyDLL("dinput8.dll")
	procDirectInput8Create = modDinput8.NewProc("DirectInput8Create")
	modKernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW   = modKernel32.NewProc("GetModuleHandleW")
	modUser32              = syscall.NewLazyDLL("user32.dll")
	procCreateWindowExW    = modUser32.NewProc("CreateWindowExW")
	procGetDesktopWindow   = modUser32.NewProc("GetDesktopWindow")
)

type xinputState struct {
	dwPacketNumber uint32
	wButtons       uint16
	bLeftTrigger   byte
	bRightTrigger  byte
	sThumbLX       int16
	sThumbLY       int16
	sThumbRX       int16
	sThumbRY       int16
}

var (
	procXInputGetState *syscall.LazyProc
	xiState            xinputState
	xiIndex            = -1
)

var xiButtons = []struct {
	mask uint16
	name string
}{{0x0001, "dpad_up"}, {0x0002, "dpad_down"}, {0x0004, "dpad_left"}, {0x0008, "dpad_right"}, {0x0010, "start"}, {0x0020, "back"},
	{0x0040, "thumbl"}, {0x0080, "thumbr"}, {0x0100, "shoulderl"}, {0x0200, "shoulderr"}, {0x1000, "a"}, {0x2000, "b"}, {0x4000, "x"}, {0x8000, "y"}}

func xinputInit() {
	for _, name := range []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"} {
		dll := syscall.NewLazyDLL(name)
		if dll.Load() == nil {
			procXInputGetState = dll.NewProc("XInputGetState")
			if procXInputGetState.Find() == nil {
				logf("XInput disponibile (%s)", name)
				return
			}
		}
	}
	procXInputGetState = nil
	logf("XInput non disponibile: i gamepad non verranno letti")
}

// xinputFind cerca il primo gamepad collegato (0..3).
func xinputFind() {
	if procXInputGetState == nil {
		return
	}
	old := xiIndex
	xiIndex = -1
	for i := 0; i < 4; i++ {
		r, _, _ := procXInputGetState.Call(uintptr(i), uintptr(unsafe.Pointer(&xiState)))
		if r == 0 {
			xiIndex = i
			break
		}
	}
	if old != xiIndex {
		logf("gamepad XInput: indice %d", xiIndex)
	}
}

func xinputPoll() GPState {
	g := GPState{Pressed: []string{}, Axes: map[string]int{}}
	if procXInputGetState == nil || xiIndex < 0 {
		return g
	}
	r, _, _ := procXInputGetState.Call(uintptr(xiIndex), uintptr(unsafe.Pointer(&xiState)))
	if r != 0 {
		xiIndex = -1
		return g
	}
	g.Connected = true
	for _, b := range xiButtons {
		if xiState.wButtons&b.mask != 0 {
			g.Pressed = append(g.Pressed, b.name)
		}
	}
	if xiState.bLeftTrigger > 60 {
		g.Pressed = append(g.Pressed, "triggerl_btn")
	}
	if xiState.bRightTrigger > 60 {
		g.Pressed = append(g.Pressed, "triggerr_btn")
	}
	g.Axes["thumblx"] = int(xiState.sThumbLX) + 32768
	g.Axes["thumbly"] = int(xiState.sThumbLY) + 32768
	g.Axes["thumbrx"] = int(xiState.sThumbRX) + 32768
	g.Axes["thumbry"] = int(xiState.sThumbRY) + 32768
	return g
}

// Gli argomenti sono puntatori passati come uintptr: la direttiva li tiene
// fermi e vivi per tutta la durata della chiamata.
//
//go:uintptrescapes
func comCall(obj uintptr, idx int, args ...uintptr) uint32 {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	a := make([]uintptr, 0, len(args)+1)
	a = append(a, obj)
	a = append(a, args...)
	r, _, _ := syscall.SyscallN(fn, a...)
	return uint32(r)
}

func failed(hr uint32) bool { return hr&0x80000000 != 0 }

type enumEntry struct {
	inst    guid
	product guid
	name    string
}

var (
	enumResult []enumEntry
	enumCB     uintptr
	joyFormat  diDataFormat
	joyObjs    []diObjectDataFormat
)

func enumDevicesCallback(lpddi uintptr, ref uintptr) uintptr {
	d := (*diDeviceInstanceW)(unsafe.Pointer(lpddi))
	enumResult = append(enumResult, enumEntry{
		inst:    d.guidInstance,
		product: d.guidProduct,
		name:    syscall.UTF16ToString(d.tszProductName[:]),
	})
	return 1 // DIENUM_CONTINUE
}

func buildJoyFormat() {
	t := func(kind uint32) uint32 { return diDFTOptional | diDFTAnyInstance | kind }
	axes := []*guid{&guidXAxis, &guidYAxis, &guidZAxis, &guidRxAxis, &guidRyAxis, &guidRzAxis, &guidSlider, &guidSlider}
	for i, g := range axes {
		joyObjs = append(joyObjs, diObjectDataFormat{g, uint32(i * 4), t(diDFTAxis), diDOIAspectPos})
	}
	for i := 0; i < 4; i++ {
		joyObjs = append(joyObjs, diObjectDataFormat{&guidPOV, uint32(32 + i*4), t(diDFTPOV), 0})
	}
	for i := 0; i < 128; i++ {
		joyObjs = append(joyObjs, diObjectDataFormat{nil, uint32(48 + i), t(diDFTButton), 0})
	}
	joyFormat = diDataFormat{
		dwSize:     uint32(unsafe.Sizeof(diDataFormat{})),
		dwObjSize:  uint32(unsafe.Sizeof(diObjectDataFormat{})),
		dwFlags:    diDFAbsAxis,
		dwDataSize: joyStateSize,
		dwNumObjs:  uint32(len(joyObjs)),
		rgodf:      &joyObjs[0],
	}
}

type openDev struct {
	inst  guid
	ptr   uintptr
	info  Device
	state *diJoyState2
}

// buffer di appoggio globali (indirizzo fisso)
var (
	scratchCaps diDevCaps
	scratchObj  diDeviceObjectInstanceW
	scratchProp diPropGuidAndPathT
	scratchInst guid
	scratchDev  uintptr
	scratchDI   uintptr
)

func startDevices() {
	go diThread()
}

func diThread() {
	runtime.LockOSThread()
	defer func() {
		if r := recover(); r != nil {
			logf("ERRORE DirectInput (panic): %v", r)
		}
	}()
	if err := modDinput8.Load(); err != nil {
		logf("ERRORE: dinput8.dll non caricabile: %v", err)
		return
	}
	hinst, _, _ := procGetModuleHandleW.Call(0)
	r, _, _ := procDirectInput8Create.Call(hinst, diVersion, uintptr(unsafe.Pointer(&iidIDirectInput8W)), uintptr(unsafe.Pointer(&scratchDI)), 0)
	di := scratchDI
	if failed(uint32(r)) || di == 0 {
		logf("ERRORE: DirectInput8Create fallita (0x%08X)", uint32(r))
		return
	}
	logf("DirectInput 8 inizializzato")
	cls, _ := syscall.UTF16PtrFromString("STATIC")
	ttl, _ := syscall.UTF16PtrFromString("SCBindingInput")
	hwnd, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(ttl)), 0, 0, 0, 0, 0, 0, 0, hinst, 0)
	if hwnd == 0 {
		hwnd, _, _ = procGetDesktopWindow.Call()
		logf("finestra nascosta non creata, uso il desktop")
	}
	buildJoyFormat()
	enumCB = syscall.NewCallback(enumDevicesCallback)
	xinputInit()

	open := []*openDev{}
	lastEnum := time.Time{}
	for {
		if time.Since(lastEnum) > 2500*time.Millisecond {
			lastEnum = time.Now()
			xinputFind()
			enumResult = enumResult[:0]
			hr := comCall(di, 4, di8DevClassGameCtl, enumCB, 0, diEDFLAttachedOnly)
			if failed(hr) {
				logf("EnumDevices fallita (0x%08X)", hr)
			} else if enumChanged(open, enumResult) {
				open = reopen(di, hwnd, open, enumResult)
				devs := make([]Device, len(open))
				for i, o := range open {
					devs[i] = o.info
				}
				assignKeys(devs)
				for i := range open {
					open[i].info = devs[i]
				}
				hub.set(devs)
				logf("periferiche rilevate: %d", len(devs))
				for _, d := range devs {
					logf("  %s  %s  pulsanti=%d assi=%v hat=%d gamepad=%v %s", d.Name, d.GUID, d.Buttons, d.Axes, d.Hats, d.Gamepad, d.Err)
				}
			}
		}
		states := make([]DevState, 0, len(open))
		for _, o := range open {
			st := DevState{Key: o.info.Key, Buttons: []int{}, Axes: map[string]int{}, Hats: []int{}}
			if o.ptr != 0 && !o.info.Gamepad {
				js := o.state
				comCall(o.ptr, 25) // Poll
				hr := comCall(o.ptr, 9, joyStateSize, uintptr(unsafe.Pointer(js)))
				if failed(hr) {
					comCall(o.ptr, 7) // Acquire
				} else {
					for i := 0; i < o.info.Buttons && i < 128; i++ {
						if js.Buttons[i]&0x80 != 0 {
							st.Buttons = append(st.Buttons, i+1)
						}
					}
					for _, a := range o.info.Axes {
						st.Axes[a] = int(axisValue(js, a))
					}
					for i := 0; i < o.info.Hats && i < 4; i++ {
						v := js.POV[i]
						if v&0xFFFF == 0xFFFF {
							st.Hats = append(st.Hats, -1)
						} else {
							st.Hats = append(st.Hats, int(v))
						}
					}
				}
			}
			states = append(states, st)
		}
		hub.setStates(states)
		hub.setGP(xinputPoll())
		time.Sleep(16 * time.Millisecond)
	}
}

func axisValue(js *diJoyState2, name string) int32 {
	switch name {
	case "x":
		return js.Axes[0]
	case "y":
		return js.Axes[1]
	case "z":
		return js.Axes[2]
	case "rotx":
		return js.Axes[3]
	case "roty":
		return js.Axes[4]
	case "rotz":
		return js.Axes[5]
	case "slider1":
		return js.Sliders[0]
	case "slider2":
		return js.Sliders[1]
	}
	return 0
}

func enumChanged(open []*openDev, found []enumEntry) bool {
	if len(open) != len(found) {
		return true
	}
	for i := range found {
		if open[i].inst != found[i].inst {
			return true
		}
	}
	return false
}

func reopen(di, hwnd uintptr, old []*openDev, found []enumEntry) []*openDev {
	byInst := map[guid]*openDev{}
	for _, o := range old {
		byInst[o.inst] = o
	}
	res := make([]*openDev, 0, len(found))
	for _, e := range found {
		if o, ok := byInst[e.inst]; ok {
			delete(byInst, e.inst)
			res = append(res, o)
			continue
		}
		res = append(res, openDevice(di, hwnd, e))
	}
	for _, o := range byInst { // scollegate
		if o.ptr != 0 {
			comCall(o.ptr, 8) // Unacquire
			comCall(o.ptr, 2) // Release
		}
	}
	return res
}

func openDevice(di, hwnd uintptr, e enumEntry) *openDev {
	o := &openDev{inst: e.inst, state: new(diJoyState2)}
	o.info = Device{GUID: e.product.String(), Name: e.name, Axes: []string{}, AxisName: []string{}}
	scratchInst = e.inst
	scratchDev = 0
	hr := comCall(di, 3, uintptr(unsafe.Pointer(&scratchInst)), uintptr(unsafe.Pointer(&scratchDev)), 0)
	dev := scratchDev
	if failed(hr) || dev == 0 {
		o.info.Err = fmt.Sprintf("CreateDevice 0x%08X", hr)
		return o
	}
	o.ptr = dev

	// percorso HID: serve a riconoscere i gamepad XInput ("IG_" nel percorso)
	scratchProp = diPropGuidAndPathT{}
	scratchProp.dwSize = uint32(unsafe.Sizeof(scratchProp))
	scratchProp.dwHeaderSize = 16
	scratchProp.dwHow = diPHDevice
	if hr = comCall(dev, 5, diPropGuidAndPath, uintptr(unsafe.Pointer(&scratchProp))); !failed(hr) {
		o.info.Path = syscall.UTF16ToString(scratchProp.wszPath[:])
		if strings.Contains(strings.ToLower(o.info.Path), "ig_") {
			o.info.Gamepad = true
		}
	}

	if hr = comCall(dev, 11, uintptr(unsafe.Pointer(&joyFormat))); failed(hr) {
		o.info.Err = fmt.Sprintf("SetDataFormat 0x%08X", hr)
		return o
	}
	if hr = comCall(dev, 13, hwnd, diSCLBackground|diSCLNonExclusive); failed(hr) {
		o.info.Err = fmt.Sprintf("SetCooperativeLevel 0x%08X", hr)
	}
	scratchCaps = diDevCaps{}
	caps := &scratchCaps
	caps.dwSize = uint32(unsafe.Sizeof(scratchCaps))
	if hr = comCall(dev, 3, uintptr(unsafe.Pointer(&scratchCaps))); !failed(hr) {
		o.info.Buttons = int(caps.dwButtons)
		if o.info.Buttons > 128 {
			o.info.Buttons = 128
		}
		o.info.Hats = int(caps.dwPOVs)
		if o.info.Hats > 4 {
			o.info.Hats = 4
		}
	}
	// quali assi esistono: interrogo ogni posizione del formato dati
	for i, name := range axisOrder {
		scratchObj = diDeviceObjectInstanceW{}
		scratchObj.dwSize = uint32(unsafe.Sizeof(scratchObj))
		if hr = comCall(dev, 14, uintptr(unsafe.Pointer(&scratchObj)), uintptr(i*4), diPHByOffset); !failed(hr) {
			o.info.Axes = append(o.info.Axes, name)
			o.info.AxisName = append(o.info.AxisName, syscall.UTF16ToString(scratchObj.tszName[:]))
		}
	}
	if len(o.info.Axes) == 0 && caps.dwAxes > 0 {
		n := int(caps.dwAxes)
		if n > 8 {
			n = 8
		}
		o.info.Axes = append(o.info.Axes, axisOrder[:n]...)
		for range o.info.Axes {
			o.info.AxisName = append(o.info.AxisName, "")
		}
	}
	comCall(dev, 7) // Acquire
	return o
}

// controlli a tempo di compilazione: le strutture devono avere esattamente
// le dimensioni che DirectInput si aspetta su Windows a 64 bit
const (
	_szInst = unsafe.Sizeof(diDeviceInstanceW{})
	_szObj  = unsafe.Sizeof(diDeviceObjectInstanceW{})
	_szProp = unsafe.Sizeof(diPropGuidAndPathT{})
	_szCaps = unsafe.Sizeof(diDevCaps{})
	_szJoy  = unsafe.Sizeof(diJoyState2{})
	_szFmt  = unsafe.Sizeof(diDataFormat{})
	_szOFmt = unsafe.Sizeof(diObjectDataFormat{})
)

var (
	_ [_szInst - 1100]byte
	_ [1100 - _szInst]byte
	_ [_szObj - 576]byte
	_ [576 - _szObj]byte
	_ [_szProp - 552]byte
	_ [552 - _szProp]byte
	_ [_szCaps - 44]byte
	_ [44 - _szCaps]byte
	_ [_szJoy - 272]byte
	_ [272 - _szJoy]byte
	_ [_szFmt - 32]byte
	_ [32 - _szFmt]byte
	_ [_szOFmt - 24]byte
	_ [24 - _szOFmt]byte
	_ [unsafe.Sizeof(xinputState{}) - 16]byte
	_ [16 - unsafe.Sizeof(xinputState{})]byte
)
