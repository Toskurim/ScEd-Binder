package main

import (
	"embed"
	"encoding/json"
	"strings"
)

// Modelli di periferica con foto e mappa dei comandi (dal progetto SC Keymap).

//go:embed data/devices.json
var devicesJSON []byte

//go:embed photos
var photoFS embed.FS

type PhotoMatch struct {
	Name        string `json:"name"`
	Vendor      string `json:"vendor"`
	Product     string `json:"product"`
	ExcludeName string `json:"excludeName"`
	Boundary    bool   `json:"nameBoundary"`
}

type PhotoView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Photo string `json:"photo"`
}

type PhotoControl struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Label  string   `json:"label"`
	Inputs []string `json:"inputs"`
	View   string   `json:"view"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
}

type PhotoDevice struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Brand    string         `json:"brand"`
	Category string         `json:"category"`
	Match    []PhotoMatch   `json:"match"`
	Notes    string         `json:"notes"`
	Views    []PhotoView    `json:"views"`
	Controls []PhotoControl `json:"controls"`
}

var photoDevices []PhotoDevice

func loadPhotoDevices() {
	if err := json.Unmarshal(devicesJSON, &photoDevices); err != nil {
		logf("ERRORE lettura modelli periferiche: %v", err)
	}
}

// guessPhoto cerca il modello con foto adatto a una periferica (codice USB, poi nome).
func guessPhoto(name, guid string) string {
	vendor, product := "", ""
	g := strings.Trim(guid, "{}")
	if len(g) >= 8 {
		product, vendor = strings.ToUpper(g[0:4]), strings.ToUpper(g[4:8])
	}
	for _, d := range photoDevices {
		for _, m := range d.Match {
			if m.Vendor != "" && strings.EqualFold(m.Vendor, vendor) && (m.Product == "" || strings.EqualFold(m.Product, product)) {
				return "foto:" + d.ID
			}
		}
	}
	n := strings.ToLower(name)
	best, bestLen := "", 0
	for _, d := range photoDevices {
		if len(d.Controls) == 0 {
			continue
		}
		for _, m := range d.Match {
			if m.Name == "" || !nameMatch(n, strings.ToLower(m.Name), m.Boundary) {
				continue
			}
			if m.ExcludeName != "" && strings.Contains(n, strings.ToLower(m.ExcludeName)) {
				continue
			}
			if len(m.Name) > bestLen {
				best, bestLen = "foto:"+d.ID, len(m.Name)
			}
		}
	}
	return best
}

func guessImage(name, guid string) string {
	if g := guessPhoto(name, guid); g != "" {
		return g
	}
	return guessTemplate(name)
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// nameMatch: il nome contiene la parola cercata (se boundary, come parola intera).
func nameMatch(name, needle string, boundary bool) bool {
	from := 0
	for {
		i := strings.Index(name[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		end := i + len(needle)
		if !boundary || ((i == 0 || !isAlnum(name[i-1])) && (end == len(name) || !isAlnum(name[end]))) {
			return true
		}
		from = i + 1
	}
}
