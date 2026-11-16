package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Device è una periferica di gioco vista da DirectInput.
type Device struct {
	Key      string   `json:"key"`     // "{GUID}#n" (n = ordinale fra periferiche con lo stesso GUID)
	GUID     string   `json:"guid"`    // GUID prodotto, come lo scrive Star Citizen
	Name     string   `json:"name"`    // nome prodotto
	Path     string   `json:"path"`    // percorso HID (diagnostica)
	Gamepad  bool     `json:"gamepad"` // dispositivo XInput: il gioco lo tratta come gamepad, non come joystick
	Buttons  int      `json:"buttons"`
	Axes     []string `json:"axes"` // x y z rotx roty rotz slider1 slider2 (solo quelli presenti)
	AxisName []string `json:"axisNames"`
	Hats     int      `json:"hats"`
	Err      string   `json:"err,omitempty"`
}

// DevState è lo stato istantaneo di una periferica.
type DevState struct {
	Key     string         `json:"key"`
	Buttons []int          `json:"b"` // pulsanti premuti (1-based)
	Axes    map[string]int `json:"a"` // 0..65535
	Hats    []int          `json:"h"` // -1 = centro, altrimenti centesimi di grado
}

// GPState è lo stato del gamepad (XInput), con i nomi usati da Star Citizen.
type GPState struct {
	Connected bool           `json:"on"`
	Pressed   []string       `json:"p"`
	Axes      map[string]int `json:"a"` // thumblx thumbly thumbrx thumbry, 0..65535
}

var axisOrder = []string{"x", "y", "z", "rotx", "roty", "rotz", "slider1", "slider2"}

type deviceHub struct {
	mu      sync.RWMutex
	devices []Device
	states  []DevState
	gp      GPState
	rev     int
	warn    string // avviso sulla lettura delle periferiche, mostrato nell'app
}

var hub = &deviceHub{}

func (h *deviceHub) set(devs []Device) {
	h.mu.Lock()
	h.devices = devs
	h.rev++
	h.mu.Unlock()
}
func (h *deviceHub) setWarn(w string) {
	h.mu.Lock()
	h.warn = w
	h.mu.Unlock()
}
func (h *deviceHub) warning() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.warn
}
func (h *deviceHub) setStates(st []DevState) {
	h.mu.Lock()
	h.states = st
	h.mu.Unlock()
}
func (h *deviceHub) setGP(g GPState) {
	h.mu.Lock()
	h.gp = g
	h.mu.Unlock()
}
func (h *deviceHub) gamepad() GPState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gp
}
func (h *deviceHub) list() []Device {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]Device(nil), h.devices...)
}
func (h *deviceHub) snapshot() []DevState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.states
}

// assignKeys calcola le chiavi "{GUID}#n" nell'ordine di enumerazione.
func assignKeys(devs []Device) {
	count := map[string]int{}
	for i := range devs {
		g := strings.ToUpper(devs[i].GUID)
		devs[i].GUID = g
		count[g]++
		devs[i].Key = fmt.Sprintf("%s#%d", g, count[g])
	}
}

// ---- log diagnostico ----

var (
	logMu    sync.Mutex
	logLines []string
)

func logf(format string, a ...any) {
	line := time.Now().Format("15:04:05") + " " + fmt.Sprintf(format, a...)
	logMu.Lock()
	logLines = append(logLines, line)
	if len(logLines) > 500 {
		logLines = logLines[len(logLines)-500:]
	}
	logMu.Unlock()
	appendLogFile(line)
}

func getLog() []string {
	logMu.Lock()
	defer logMu.Unlock()
	return append([]string(nil), logLines...)
}
