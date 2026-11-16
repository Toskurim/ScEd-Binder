package main

import (
	"embed"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed data/defaultProfile.xml data/labels_en.json data/labels_it.json
var dataFS embed.FS

type Action struct {
	Map       string `json:"map"`
	Name      string `json:"name"`
	Label     string `json:"label"`   // italiano
	LabelEN   string `json:"labelEn"` // inglese (come nel gioco)
	Desc      string `json:"desc,omitempty"`
	DefKB     string `json:"defKb,omitempty"`
	DefMo     string `json:"defMo,omitempty"`
	DefJS     string `json:"defJs,omitempty"`
	DefGP     string `json:"defGp,omitempty"`
	OptGroup  string `json:"optGroup,omitempty"` // gruppo opzioni (inversione, curve)
	CanInvert bool   `json:"canInvert"`          // l'asse si può invertire dal file del gioco
	Visible   bool   `json:"visible"`            // compare nel menu del gioco
	Axis      bool   `json:"axis"`               // comando analogico (asse)
}

type ActionMap struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	LabelEN  string   `json:"labelEn"`
	Category string   `json:"category"`
	Actions  []Action `json:"actions"`
}

type xOptGroup struct {
	Name       string      `xml:"name,attr"`
	Invert     string      `xml:"invert,attr"`
	ShowInvert string      `xml:"UIShowInvert,attr"`
	Groups     []xOptGroup `xml:"optiongroup"`
}

// invertDefaults: gruppi invertibili per i joystick -> valore predefinito del gioco
var invertDefaults = map[string]bool{}

func walkOpt(g xOptGroup) {
	if g.ShowInvert == "1" && g.Name != "" {
		invertDefaults[g.Name] = g.Invert == "1"
	}
	for _, c := range g.Groups {
		walkOpt(c)
	}
}

type xDefProfile struct {
	Trees []struct {
		Type   string      `xml:"type,attr"`
		Groups []xOptGroup `xml:"optiongroup"`
	} `xml:"optiontree"`
	Maps []struct {
		Name       string `xml:"name,attr"`
		UILabel    string `xml:"UILabel,attr"`
		UICategory string `xml:"UICategory,attr"`
		Actions    []struct {
			Attrs []xml.Attr `xml:",any,attr"`
		} `xml:"action"`
	} `xml:"actionmap"`
}

var (
	actionMaps   []ActionMap
	actionIndex  = map[string]*Action{} // "map/name"
	gameDataNote string
)

var axisHint = regexp.MustCompile(`(?i)^(x|y|z|rot[xyz]|slider\d|thumb[lr][xy]?$|thumb[lr][xy]|trigger[lr]|maxis_[xyz])`)

func loadActions() error {
	xmlData, _ := dataFS.ReadFile("data/defaultProfile.xml")
	gameDataNote = "Star Citizen Alpha 4.10.2 (integrato)"
	// un defaultProfile.xml più recente messo in "dati" ha la precedenza
	if b, err := os.ReadFile(filepath.Join(dataDir(), "defaultProfile.xml")); err == nil && len(b) > 1000 {
		xmlData = b
		gameDataNote = "defaultProfile.xml personalizzato (cartella dati)"
	}
	en, it := map[string]string{}, map[string]string{}
	b, _ := dataFS.ReadFile("data/labels_en.json")
	json.Unmarshal(b, &en)
	b, _ = dataFS.ReadFile("data/labels_it.json")
	json.Unmarshal(b, &it)
	if b, err := os.ReadFile(filepath.Join(dataDir(), "traduzioni.json")); err == nil {
		extra := map[string]string{}
		if json.Unmarshal(b, &extra) == nil {
			for k, v := range extra {
				it[strings.ToLower(k)] = v
			}
		}
	}
	tr := func(key string, m map[string]string) string {
		if !strings.HasPrefix(key, "@") {
			return ""
		}
		return m[strings.ToLower(key[1:])]
	}
	var p xDefProfile
	if err := xml.Unmarshal(xmlData, &p); err != nil {
		return err
	}
	actionMaps = nil
	for _, t := range p.Trees {
		if t.Type == "joystick" {
			for _, g := range t.Groups {
				walkOpt(g)
			}
		}
	}
	for _, m := range p.Maps {
		am := ActionMap{Name: m.Name, Label: tr(m.UILabel, it), LabelEN: tr(m.UILabel, en), Category: tr(m.UICategory, it)}
		if am.LabelEN == "" {
			am.LabelEN = m.Name
		}
		if am.Label == "" {
			am.Label = am.LabelEN
		}
		for _, xa := range m.Actions {
			at := map[string]string{}
			for _, a := range xa.Attrs {
				at[a.Name.Local] = a.Value
			}
			a := Action{Map: m.Name, Name: at["name"]}
			if a.Name == "" {
				continue
			}
			a.LabelEN = tr(at["UILabel"], en)
			a.Label = tr(at["UILabel"], it)
			a.Visible = at["UILabel"] != "" && m.UILabel != ""
			if a.LabelEN == "" {
				a.LabelEN = a.Name
			}
			if a.Label == "" {
				a.Label = a.LabelEN
			}
			if strings.HasSuffix(a.Name, "_mouse") && !strings.Contains(strings.ToLower(a.Label), "mouse") {
				a.Label += " (mouse)"
			}
			a.Desc = tr(at["UIDescription"], it)
			if a.Desc == "" {
				a.Desc = tr(at["UIDescription"], en)
			}
			a.OptGroup = at["optionGroup"]
			_, a.CanInvert = invertDefaults[a.OptGroup]
			a.DefKB = strings.TrimSpace(at["keyboard"])
			a.DefMo = strings.TrimSpace(at["mouse"])
			a.DefJS = strings.TrimSpace(at["joystick"])
			a.DefGP = strings.TrimSpace(at["gamepad"])
			for _, p := range []*string{&a.DefKB, &a.DefMo, &a.DefJS, &a.DefGP} {
				if *p == "0" || *p == "1" {
					*p = ""
				}
			}
			for _, d := range []string{a.DefJS, a.DefGP, a.DefMo} {
				parts := strings.Split(d, "+")
				if axisHint.MatchString(parts[len(parts)-1]) {
					a.Axis = true
				}
			}
			_, press := at["onPress"]
			_, mode := at["activationMode"]
			if press || mode {
				a.Axis = false
			}
			am.Actions = append(am.Actions, a)
		}
		actionMaps = append(actionMaps, am)
	}
	for i := range actionMaps {
		for j := range actionMaps[i].Actions {
			a := &actionMaps[i].Actions[j]
			actionIndex[a.Map+"/"+a.Name] = a
		}
	}
	return nil
}
