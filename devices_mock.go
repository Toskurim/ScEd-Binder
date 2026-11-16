//go:build !windows

package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Periferiche simulate: servono solo per provare l'app fuori da Windows.

var (
	mockMu      sync.Mutex
	mockAll     []Device
	mockOn      = map[int]bool{}
	mockPressed = map[string]map[int]bool{}
	mockAxes    = map[string]map[string]int{}
)

func startDevices() {
	mk := func(name, guid string, buttons int, axes []string, hats int) Device {
		return Device{Name: name, GUID: guid, Buttons: buttons, Axes: axes, AxisName: make([]string, len(axes)), Hats: hats}
	}
	mockAll = []Device{
		mk("LEFT VPC Stick MT-50CM2", "{812F3344-0000-0000-0000-504944564944}", 31, []string{"x", "y", "z", "rotx", "roty", "slider1"}, 0),
		mk("LEFT VPC MongoosT-50CM3", "{81943344-0000-0000-0000-504944564944}", 84, []string{"x", "y", "z", "rotx", "roty", "slider1"}, 0),
		mk(" VKB-Sim T-Rudder ", "{011F231D-0000-0000-0000-504944564944}", 0, []string{"x", "y", "z"}, 0),
		mk("RIGHT VPC Stick MT-50CM2", "{41303344-0000-0000-0000-504944564944}", 31, []string{"x", "y", "z", "rotx", "roty", "slider1"}, 1),
		mk("FANATEC Wheel", "{00040EB7-0000-0000-0000-504944564944}", 48, []string{"x", "y", "z", "rotz"}, 1),
		mk("FANATEC Wheel", "{00040EB7-0000-0000-0000-504944564944}", 16, []string{"x"}, 0),
		{Name: "Controller (Xbox One For Windows)", GUID: "{02FF045E-0000-0000-0000-504944564944}", Gamepad: true, Buttons: 10, Axes: []string{"x", "y"}, AxisName: []string{"", ""}, Hats: 1},
	}
	for i := range mockAll {
		mockOn[i] = true
	}
	mockRefresh()
	http.HandleFunc("/api/debug", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mockMu.Lock()
		if v := q.Get("toggle"); v != "" {
			i, _ := strconv.Atoi(v)
			mockOn[i] = !mockOn[i]
		}
		key := q.Get("key")
		if v := q.Get("button"); v != "" {
			b, _ := strconv.Atoi(v)
			if mockPressed[key] == nil {
				mockPressed[key] = map[int]bool{}
			}
			mockPressed[key][b] = q.Get("down") != "0"
		}
		if v := q.Get("gp"); v != "" {
			g := GPState{Connected: true, Pressed: []string{}, Axes: map[string]int{"thumblx": 32768, "thumbly": 32768, "thumbrx": 32768, "thumbry": 32768}}
			if v != "-" {
				g.Pressed = strings.Split(v, ",")
			}
			hub.setGP(g)
		}
		if v := q.Get("axis"); v != "" {
			val, _ := strconv.Atoi(q.Get("value"))
			if mockAxes[key] == nil {
				mockAxes[key] = map[string]int{}
			}
			mockAxes[key][v] = val
		}
		mockMu.Unlock()
		mockRefresh()
		w.Write([]byte("ok"))
	})
}

func mockRefresh() {
	mockMu.Lock()
	defer mockMu.Unlock()
	devs := []Device{}
	for i, d := range mockAll {
		if mockOn[i] {
			devs = append(devs, d)
		}
	}
	assignKeys(devs)
	hub.set(devs)
	st := []DevState{}
	for _, d := range devs {
		s := DevState{Key: d.Key, Buttons: []int{}, Axes: map[string]int{}, Hats: []int{}}
		for b, on := range mockPressed[d.Key] {
			if on {
				s.Buttons = append(s.Buttons, b)
			}
		}
		for _, a := range d.Axes {
			v, ok := mockAxes[d.Key][a]
			if !ok {
				v = 32767
			}
			s.Axes[a] = v
		}
		for i := 0; i < d.Hats; i++ {
			s.Hats = append(s.Hats, -1)
		}
		st = append(st, s)
	}
	hub.setStates(st)
	_ = strings.ToLower
}
