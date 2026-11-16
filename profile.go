package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Hotspot struct {
	Input string  `json:"input"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type DevInfo struct {
	Name     string                         `json:"name"`
	GUID     string                         `json:"guid"`
	Product  string                         `json:"product"`  // stringa "Product" esatta usata dal gioco
	Options  string                         `json:"options"`  // contenuto di <options> (curve, inversioni) conservato
	Image    string                         `json:"image"`    // "tpl:Nome" oppure "img:file"
	Hotspots []Hotspot                      `json:"hotspots"` // etichette posizionate a mano sull'immagine
	Alias    string                         `json:"alias"`    // nome scelto dall'utente
	Invert   map[string]bool                `json:"invert"`   // gruppo opzioni -> asse invertito
	Ignored  []string                       `json:"ignored"`  // ingressi da non rilevare
	PhotoMap map[string]map[string][]string `json:"photoMap"` // modello -> comando -> ingressi corretti dall\'utente
}

type Binding struct {
	Map      string `json:"map"`
	Action   string `json:"action"`
	Dev      string `json:"dev"`   // "kb", "mo", "gp", "jsclear" oppure chiave periferica "{GUID}#n"
	Input    string `json:"input"` // button5, x, hat1_up, lalt+f ... (" " = predefinito disattivato)
	MultiTap string `json:"multiTap,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

// Layout: un insieme completo di assegnazioni con il suo nome (es. "Combattimento", "Mining").
type Layout struct {
	Name     string                     `json:"name"`
	Bindings []Binding                  `json:"bindings"`
	Invert   map[string]map[string]bool `json:"invert"` // periferica -> gruppo -> invertito
}

type Profile struct {
	Devices         map[string]*DevInfo `json:"devices"`
	Order           []string            `json:"order"` // ordine preferito dei joystick (js1, js2, ...)
	Bindings        []Binding           `json:"bindings"`
	HeaderAttrs     [][2]string         `json:"headerAttrs"`
	OtherOptions    []string            `json:"otherOptions"`
	Extras          []string            `json:"extras"`
	Modifiers       string              `json:"modifiers"`
	ClearJSDefaults bool                `json:"clearJsDefaults"`
	LastWrite       map[string]string   `json:"lastWrite"`
	Install         string              `json:"install"`
	CustomRoot      string              `json:"customRoot"`
	Imported        bool                `json:"imported"`
	OrderManual     bool                `json:"orderManual"` // ordine scelto a mano con le frecce (altrimenti segue quello del gioco)
	Layouts         []Layout            `json:"layouts"`     // layout salvati (assegnazioni + inversioni); quello attivo è anche in Bindings
	Layout          string              `json:"layout"`      // nome del layout attivo
	ED              *EDState            `json:"ed"`          // assegnazioni per Elite Dangerous
	Game            string              `json:"game"`        // gioco selezionato: "sc" o "ed"
}

var (
	profMu  sync.Mutex
	profile = newProfile()
)

func newProfile() *Profile {
	return &Profile{Devices: map[string]*DevInfo{}, Order: []string{}, Bindings: []Binding{}, LastWrite: map[string]string{}, ClearJSDefaults: true,
		HeaderAttrs: [][2]string{{"version", "1"}, {"optionsVersion", "2"}, {"rebindVersion", "2"}, {"profileName", "default"}}}
}

func (p *Profile) normalize() {
	if p.Devices == nil {
		p.Devices = map[string]*DevInfo{}
	}
	if p.Order == nil {
		p.Order = []string{}
	}
	if p.Bindings == nil {
		p.Bindings = []Binding{}
	}
	if p.LastWrite == nil {
		p.LastWrite = map[string]string{}
	}
	if p.Layout == "" {
		p.Layout = "Principale"
	}
	if p.Layouts == nil {
		p.Layouts = []Layout{}
	}
	if len(p.HeaderAttrs) == 0 {
		p.HeaderAttrs = newProfile().HeaderAttrs
	}
	for _, d := range p.Devices {
		if d.Hotspots == nil {
			d.Hotspots = []Hotspot{}
		}
		if d.Invert == nil {
			d.Invert = map[string]bool{}
		}
		if d.Ignored == nil {
			d.Ignored = []string{}
		}
		if d.PhotoMap == nil {
			d.PhotoMap = map[string]map[string][]string{}
		}
	}
}

func profilePath() string { return filepath.Join(dataDir(), "profilo.json") }

func loadProfile() {
	b, err := os.ReadFile(profilePath())
	if err != nil {
		return
	}
	p := newProfile()
	if json.Unmarshal(b, p) == nil {
		p.normalize()
		profile = p
	} else {
		logf("profilo.json illeggibile, parto da zero")
	}
}

func saveProfile() error {
	b, err := json.MarshalIndent(profile, "", " ")
	if err != nil {
		return err
	}
	return writeAtomic(profilePath(), b)
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return os.WriteFile(path, b, 0644)
	}
	return nil
}

var guidRe = regexp.MustCompile(`\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}`)

func productGUID(product string) string { return strings.ToUpper(guidRe.FindString(product)) }
func productName(product string) string {
	return strings.TrimSpace(guidRe.ReplaceAllString(product, ""))
}

func guessTemplate(name string) string {
	n := strings.ToLower(name)
	has := func(s ...string) bool {
		for _, x := range s {
			if !strings.Contains(n, x) {
				return false
			}
		}
		return true
	}
	switch {
	case has("vpc", "throttle"), has("mongoost-50cm3"), has("vpc", "cm3"):
		return "tpl:Virpil Throttle MT-50 CM2"
	case has("left", "vpc", "stick"), has("vpc", "alpha", "l"):
		return "tpl:Virpil Constellation ALPHA L"
	case has("vpc", "stick"), has("vpc", "alpha"):
		return "tpl:Virpil Constellation ALPHA R"
	case has("gladiator") && (strings.HasSuffix(strings.TrimSpace(n), " l") || has(" l ")):
		return "tpl:VKB-Sim Gladiator NXT L"
	case has("gladiator"):
		return "tpl:VKB-Sim Gladiator NXT R"
	case has("t.16000m"), has("t16000"):
		return "tpl:Thrustmaster T.16000M Joystick Right Handed"
	case has("twcs"):
		return "tpl:Thrustmaster T.16000M Throttle"
	case has("warthog", "throttle"):
		return "tpl:Thrustmaster Warthog - Throttle"
	case has("warthog"):
		return "tpl:Thrustmaster Warthog - Joystick"
	case has("x52"):
		return "tpl:Saitek - Logitech X52_H.O.T.A.S."
	case has("x56", "throttle"), has("x-56", "throttle"):
		return "tpl:Saitek - Logitech X56 H.O.T.A.S. Throttle"
	case has("x56"), has("x-56"):
		return "tpl:Saitek - Logitech X56 H.O.T.A.S. Stick"
	}
	return ""
}

// ensureDevices aggiunge al profilo le periferiche collegate non ancora note.
func (p *Profile) ensureDevices(connected []Device, logProducts map[string][]string) bool {
	changed := false
	seen := map[string]int{}
	for _, d := range connected {
		if d.Gamepad {
			continue
		}
		seen[d.GUID]++
		di := p.Devices[d.Key]
		if di == nil {
			di = &DevInfo{Name: strings.TrimSpace(d.Name), GUID: d.GUID, Hotspots: []Hotspot{}}
			di.Image = guessImage(d.Name, d.GUID)
			p.Devices[d.Key] = di
			changed = true
		}
		if di.Name == "" {
			di.Name = strings.TrimSpace(d.Name)
			changed = true
		}
		if di.Product == "" {
			if lst := logProducts[d.GUID]; len(lst) >= seen[d.GUID] {
				di.Product = lst[seen[d.GUID]-1]
				changed = true
			}
		}
		found := false
		for _, k := range p.Order {
			if k == d.Key {
				found = true
			}
		}
		if !found {
			p.Order = append(p.Order, d.Key)
			changed = true
		}
	}
	return changed
}

// numbering: numero jsN di ogni joystick collegato, secondo l'ordine preferito.
func (p *Profile) numbering(connected []Device) (map[string]int, []Device) {
	pos := map[string]int{}
	for i, k := range p.Order {
		pos[k] = i
	}
	js := []Device{}
	for _, d := range connected {
		if !d.Gamepad {
			js = append(js, d)
		}
	}
	sort.SliceStable(js, func(a, b int) bool {
		pa, oka := pos[js[a].Key]
		pb, okb := pos[js[b].Key]
		if oka && okb {
			return pa < pb
		}
		return oka && !okb
	})
	num := map[string]int{}
	for i, d := range js {
		num[d.Key] = i + 1
	}
	return num, js
}

// ---------- inversione assi dentro <options> ----------

var invAttrRe = regexp.MustCompile(`\s+invert="([^"]*)"`)

func optionKids(inner string) []rawEl {
	if strings.TrimSpace(inner) == "" {
		return nil
	}
	_, kids, err := splitChildren([]byte("<o>" + inner + "</o>"))
	if err != nil {
		return nil
	}
	return kids
}

// extractInvert separa le inversioni dal resto delle opzioni (curve ecc.).
func extractInvert(inner string) (string, map[string]bool) {
	inv := map[string]bool{}
	rest := []string{}
	for _, k := range optionKids(inner) {
		end := strings.Index(k.raw, ">")
		if end < 0 {
			rest = append(rest, k.raw)
			continue
		}
		tag, tail := k.raw[:end], k.raw[end:]
		if m := invAttrRe.FindStringSubmatch(tag); m != nil {
			inv[k.name] = m[1] == "1"
			tag = invAttrRe.ReplaceAllString(tag, "")
			if strings.TrimSpace(strings.TrimSuffix(tag, "/")) == "<"+k.name && (tail == ">" && strings.HasSuffix(tag, "/")) {
				continue // conteneva solo l'inversione
			}
		}
		rest = append(rest, tag+tail)
	}
	return strings.Join(rest, "\n   "), inv
}

// mergeInvert rimette le inversioni nelle opzioni da scrivere.
func mergeInvert(inner string, inv map[string]bool) string {
	out := []string{}
	done := map[string]bool{}
	for _, k := range optionKids(inner) {
		raw := k.raw
		if v, ok := inv[k.name]; ok && !done[k.name] {
			done[k.name] = true
			val := "0"
			if v {
				val = "1"
			}
			raw = "<" + k.name + ` invert="` + val + `"` + raw[len(k.name)+1:]
		}
		out = append(out, raw)
	}
	names := []string{}
	for g := range inv {
		if !done[g] {
			names = append(names, g)
		}
	}
	sort.Strings(names)
	for _, g := range names {
		val := "0"
		if inv[g] {
			val = "1"
		}
		out = append(out, "<"+g+` invert="`+val+`"/>`)
	}
	return strings.Join(out, "\n   ")
}

// ---------- lettura di actionmaps.xml ----------

type rawEl struct {
	name string
	raw  string
}

func splitChildren(data []byte) (attrs []xml.Attr, kids []rawEl, err error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	depth := 0
	base := -1 // profondità del contenitore
	var start int64
	var cur string
	for {
		off := dec.InputOffset()
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, nil, e
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				attrs = t.Attr
				base = 1
			} else if depth == 2 && t.Name.Local == "ActionProfiles" && base == 1 && len(kids) == 0 {
				attrs = t.Attr
				base = 2
			} else if depth == base+1 {
				start = off
				cur = t.Name.Local
			}
		case xml.EndElement:
			if depth == base+1 && cur != "" {
				kids = append(kids, rawEl{cur, string(data[start:dec.InputOffset()])})
				cur = ""
			}
			depth--
		}
	}
	if base < 0 {
		return nil, nil, fmt.Errorf("file vuoto o non valido")
	}
	return attrs, kids, nil
}

type xOptions struct {
	Type     string `xml:"type,attr"`
	Instance int    `xml:"instance,attr"`
	Product  string `xml:"Product,attr"`
	Inner    string `xml:",innerxml"`
}
type xRebind struct {
	Input    string `xml:"input,attr"`
	MultiTap string `xml:"multiTap,attr"`
	Mode     string `xml:"activationMode,attr"`
}
type xAction struct {
	Name    string    `xml:"name,attr"`
	Rebinds []xRebind `xml:"rebind"`
}
type xMap struct {
	Name    string    `xml:"name,attr"`
	Actions []xAction `xml:"action"`
}

var inputRe = regexp.MustCompile(`^(js|kb|mo|gp)(\d+)_(.*)$`)

// importActionMaps trasforma un actionmaps.xml nel profilo dell'app.
// Le immagini e le etichette già scelte per le periferiche vengono mantenute.
func importActionMaps(data []byte, old *Profile) (*Profile, error) {
	attrs, kids, err := splitChildren(data)
	if err != nil {
		return nil, err
	}
	p := newProfile()
	p.ClearJSDefaults = old.ClearJSDefaults
	p.LastWrite = old.LastWrite
	p.Install = old.Install
	p.CustomRoot = old.CustomRoot
	p.Layouts = old.Layouts
	p.Layout = old.Layout
	p.OrderManual = old.OrderManual
	p.ED = old.ED
	p.Game = old.Game
	p.Imported = true
	if len(attrs) > 0 {
		p.HeaderAttrs = nil
		for _, a := range attrs {
			p.HeaderAttrs = append(p.HeaderAttrs, [2]string{a.Name.Local, a.Value})
		}
	}
	jsKey := map[int]string{}
	type jsOpt struct {
		inst int
		o    xOptions
	}
	var jsOpts []jsOpt
	var maps []xMap
	for _, k := range kids {
		switch k.name {
		case "options":
			var o xOptions
			if xml.Unmarshal([]byte(k.raw), &o) != nil {
				continue
			}
			if o.Type == "joystick" {
				jsOpts = append(jsOpts, jsOpt{o.Instance, o})
			} else {
				p.OtherOptions = append(p.OtherOptions, k.raw)
			}
		case "modifiers":
			p.Modifiers = k.raw
		case "actionmap":
			var m xMap
			if xml.Unmarshal([]byte(k.raw), &m) == nil {
				maps = append(maps, m)
			}
		case "CustomisationUIHeader":
			// intestazione dei profili esportati: non serve
		default:
			p.Extras = append(p.Extras, k.raw)
		}
	}
	sort.SliceStable(jsOpts, func(a, b int) bool { return jsOpts[a].inst < jsOpts[b].inst })
	count := map[string]int{}
	for _, jo := range jsOpts {
		g := productGUID(jo.o.Product)
		if g == "" {
			continue
		}
		count[g]++
		key := fmt.Sprintf("%s#%d", g, count[g])
		jsKey[jo.inst] = key
		rest, inv := extractInvert(jo.o.Inner)
		di := &DevInfo{Name: productName(jo.o.Product), GUID: g, Product: jo.o.Product, Options: strings.TrimSpace(rest), Hotspots: []Hotspot{}, Invert: inv}
		if od := old.Devices[key]; od != nil {
			di.Image, di.Hotspots, di.Alias, di.Ignored, di.PhotoMap = od.Image, od.Hotspots, od.Alias, od.Ignored, od.PhotoMap
			if di.Hotspots == nil {
				di.Hotspots = []Hotspot{}
			}
		} else {
			di.Image = guessImage(di.Name, g)
		}
		p.Devices[key] = di
		p.Order = append(p.Order, key)
	}
	// periferiche note solo al vecchio profilo: le tengo in coda
	for _, k := range old.Order {
		if _, ok := p.Devices[k]; !ok && old.Devices[k] != nil && !strings.HasPrefix(k, "?") {
			d := *old.Devices[k]
			p.Devices[k] = &d
			p.Order = append(p.Order, k)
		}
	}
	for _, m := range maps {
		for _, a := range m.Actions {
			for _, r := range a.Rebinds {
				mm := inputRe.FindStringSubmatch(r.Input)
				if mm == nil {
					continue
				}
				b := Binding{Map: m.Name, Action: a.Name, Input: mm[3], MultiTap: r.MultiTap, Mode: r.Mode}
				inst, _ := strconv.Atoi(mm[2])
				switch mm[1] {
				case "js":
					if strings.TrimSpace(mm[3]) == "" {
						b.Dev, b.Input = "jsclear", " "
					} else if k, ok := jsKey[inst]; ok {
						b.Dev = k
					} else {
						k := fmt.Sprintf("?js%d", inst)
						if p.Devices[k] == nil {
							p.Devices[k] = &DevInfo{Name: fmt.Sprintf("Joystick %d (periferica non identificata)", inst), Hotspots: []Hotspot{}}
							p.Order = append(p.Order, k)
						}
						b.Dev = k
					}
				default:
					b.Dev = mm[1]
					if strings.TrimSpace(b.Input) == "" {
						b.Input = " "
					}
				}
				p.Bindings = append(p.Bindings, b)
			}
		}
	}
	p.normalize()
	return p, nil
}

// ---------- scrittura di actionmaps.xml ----------

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

type WriteStats struct {
	Written      int      `json:"written"`
	Skipped      int      `json:"skipped"`
	SkippedDevs  []string `json:"skippedDevs"`
	Cleared      int      `json:"cleared"`
	Joysticks    []string `json:"joysticks"`
	Path         string   `json:"path"`
	Backup       string   `json:"backup"`
	ProfileSaved string   `json:"profileSaved"`
}

func (p *Profile) productString(key string, d Device) string {
	if di := p.Devices[key]; di != nil && di.Product != "" {
		return di.Product
	}
	return d.Name + "  " + d.GUID
}

func (p *Profile) buildXML(connected []Device) (string, WriteStats) {
	var st WriteStats
	st.SkippedDevs = []string{}
	st.Joysticks = []string{}
	num, js := p.numbering(connected)
	var sb strings.Builder
	sb.WriteString("<ActionMaps>\n <ActionProfiles")
	for _, a := range p.HeaderAttrs {
		fmt.Fprintf(&sb, ` %s="%s"`, a[0], xmlEsc(a[1]))
	}
	sb.WriteString(">\n")
	for _, e := range p.Extras {
		sb.WriteString("  " + strings.TrimSpace(e) + "\n")
	}
	if len(p.OtherOptions) == 0 {
		sb.WriteString("  <options type=\"keyboard\" instance=\"1\"/>\n")
	}
	for _, o := range p.OtherOptions {
		sb.WriteString("  " + strings.TrimSpace(o) + "\n")
	}
	for i, d := range js {
		prod := p.productString(d.Key, d)
		st.Joysticks = append(st.Joysticks, fmt.Sprintf("js%d = %s", i+1, strings.TrimSpace(productName(prod))))
		inner := ""
		if di := p.Devices[d.Key]; di != nil {
			inner = strings.TrimSpace(mergeInvert(di.Options, di.Invert))
		}
		if inner == "" {
			// un'opzione esplicita (valore predefinito) fa sì che il gioco rispetti il numero assegnato a questa periferica
			inner = `<flight_move_pitch invert="0"/>`
		}
		if inner == "" {
			fmt.Fprintf(&sb, "  <options type=\"joystick\" instance=\"%d\" Product=\"%s\"/>\n", i+1, xmlEsc(prod))
		} else {
			fmt.Fprintf(&sb, "  <options type=\"joystick\" instance=\"%d\" Product=\"%s\">\n   %s\n  </options>\n", i+1, xmlEsc(prod), inner)
		}
	}
	for i := len(js) + 1; i <= 8; i++ {
		fmt.Fprintf(&sb, "  <options type=\"joystick\" instance=\"%d\"/>\n", i)
	}
	if strings.TrimSpace(p.Modifiers) != "" {
		sb.WriteString("  " + strings.TrimSpace(p.Modifiers) + "\n")
	} else {
		sb.WriteString("  <modifiers />\n")
	}

	// raggruppo per mappa/azione
	type key struct{ m, a string }
	byAct := map[key][]string{}
	hasJS := map[key]bool{}
	skipped := map[string]bool{}
	rank := map[string]int{"kb": 0, "mo": 1, "gp": 2}
	type ent struct {
		rank int
		s    string
	}
	tmp := map[key][]ent{}
	for _, b := range p.Bindings {
		k := key{b.Map, b.Action}
		var input string
		r := 3
		switch b.Dev {
		case "kb", "mo", "gp":
			input = b.Dev + "1_" + b.Input
			r = rank[b.Dev]
		case "jsclear":
			if len(js) == 0 {
				continue
			}
			input = "js1_ "
			hasJS[k] = true
		default:
			n, ok := num[b.Dev]
			if !ok {
				st.Skipped++
				if !skipped[b.Dev] {
					skipped[b.Dev] = true
					name := b.Dev
					if di := p.Devices[b.Dev]; di != nil {
						name = di.Name
						if di.Alias != "" {
							name = di.Alias
						}
					}
					st.SkippedDevs = append(st.SkippedDevs, name)
				}
				continue
			}
			input = fmt.Sprintf("js%d_%s", n, b.Input)
			hasJS[k] = true
		}
		s := fmt.Sprintf(`<rebind input="%s"`, xmlEsc(input))
		if b.Mode != "" {
			s += fmt.Sprintf(` activationMode="%s"`, xmlEsc(b.Mode))
		}
		if b.MultiTap != "" {
			s += fmt.Sprintf(` multiTap="%s"`, xmlEsc(b.MultiTap))
		}
		s += "/>"
		tmp[k] = append(tmp[k], ent{r, s})
		st.Written++
	}
	if p.ClearJSDefaults && len(js) > 0 {
		for _, am := range actionMaps {
			for _, a := range am.Actions {
				k := key{a.Map, a.Name}
				if a.DefJS != "" && !hasJS[k] {
					tmp[k] = append(tmp[k], ent{3, `<rebind input="js1_ "/>`})
					st.Cleared++
				}
			}
		}
	}
	for k, v := range tmp {
		sort.SliceStable(v, func(a, b int) bool { return v[a].rank < v[b].rank })
		for _, e := range v {
			byAct[k] = append(byAct[k], e.s)
		}
	}
	// ordine: quello del gioco, poi il resto in ordine alfabetico
	done := map[key]bool{}
	writeMap := func(name string, acts []string) {
		var inner strings.Builder
		for _, a := range acts {
			k := key{name, a}
			if done[k] || len(byAct[k]) == 0 {
				continue
			}
			done[k] = true
			fmt.Fprintf(&inner, "   <action name=\"%s\">\n", xmlEsc(a))
			for _, r := range byAct[k] {
				inner.WriteString("    " + r + "\n")
			}
			inner.WriteString("   </action>\n")
		}
		if inner.Len() > 0 {
			fmt.Fprintf(&sb, "  <actionmap name=\"%s\">\n%s  </actionmap>\n", xmlEsc(name), inner.String())
		}
	}
	rest := map[string][]string{}
	for k := range byAct {
		rest[k.m] = append(rest[k.m], k.a)
	}
	for _, am := range actionMaps {
		names := []string{}
		known := map[string]bool{}
		for _, a := range am.Actions {
			names = append(names, a.Name)
			known[a.Name] = true
		}
		extra := []string{}
		for _, a := range rest[am.Name] {
			if !known[a] {
				extra = append(extra, a)
			}
		}
		sort.Strings(extra)
		writeMap(am.Name, append(names, extra...))
		delete(rest, am.Name)
	}
	others := []string{}
	for m := range rest {
		others = append(others, m)
	}
	sort.Strings(others)
	for _, m := range others {
		sort.Strings(rest[m])
		writeMap(m, rest[m])
	}
	sb.WriteString(" </ActionProfiles>\n</ActionMaps>\n")
	out := strings.ReplaceAll(sb.String(), "\r\n", "\n")
	return strings.ReplaceAll(out, "\n", "\r\n"), st
}

// ---------- confronto "di sostanza" tra file ----------

// fileJoystickKeys: chiavi delle periferiche nell'ordine (instance) in cui compaiono in un actionmaps.xml.
func fileJoystickKeys(data []byte) []string {
	_, kids, err := splitChildren(data)
	if err != nil {
		return nil
	}
	type jo struct {
		inst int
		guid string
	}
	var list []jo
	for _, k := range kids {
		if k.name != "options" {
			continue
		}
		var o xOptions
		if xml.Unmarshal([]byte(k.raw), &o) != nil || o.Type != "joystick" {
			continue
		}
		if g := productGUID(o.Product); g != "" {
			list = append(list, jo{o.Instance, g})
		}
	}
	sort.SliceStable(list, func(a, b int) bool { return list[a].inst < list[b].inst })
	count := map[string]int{}
	keys := []string{}
	for _, j := range list {
		count[j.guid]++
		keys = append(keys, fmt.Sprintf("%s#%d", j.guid, count[j.guid]))
	}
	return keys
}

// signature riassume ciò che conta di un actionmaps.xml (numerazione, inversioni, assegnazioni),
// ignorando ordine delle righe e spaziature: il gioco riscrive il file a modo suo a ogni uscita.
func signature(data []byte) string {
	_, kids, err := splitChildren(data)
	if err != nil {
		return "!"
	}
	lines := []string{}
	for _, k := range kids {
		switch k.name {
		case "options":
			var o xOptions
			if xml.Unmarshal([]byte(k.raw), &o) != nil || o.Type != "joystick" || productGUID(o.Product) == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("dev %d %s", o.Instance, productGUID(o.Product)))
			_, inv := extractInvert(o.Inner)
			for g, v := range inv {
				if v != invertDefaults[g] {
					lines = append(lines, fmt.Sprintf("inv %d %s %v", o.Instance, g, v))
				}
			}
		case "actionmap":
			var m xMap
			if xml.Unmarshal([]byte(k.raw), &m) != nil {
				continue
			}
			for _, a := range m.Actions {
				for _, r := range a.Rebinds {
					in := strings.TrimSpace(r.Input)
					if strings.HasSuffix(in, "_") {
						in += " "
					}
					lines = append(lines, fmt.Sprintf("bind %s/%s %s %s %s", m.Name, a.Name, in, r.Mode, r.MultiTap))
				}
			}
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// adoptGameOrder mette le periferiche nell'ordine usato dal file del gioco (le altre restano in coda).
func (p *Profile) adoptGameOrder(keys []string) bool {
	if len(keys) == 0 {
		return false
	}
	in := map[string]bool{}
	for _, k := range keys {
		in[k] = true
	}
	next := []string{}
	have := map[string]bool{}
	for _, k := range p.Order {
		have[k] = true
	}
	for _, k := range keys {
		if have[k] {
			next = append(next, k)
		}
	}
	for _, k := range p.Order {
		if !in[k] {
			next = append(next, k)
		}
	}
	same := len(next) == len(p.Order)
	if same {
		for i := range next {
			if next[i] != p.Order[i] {
				same = false
			}
		}
	}
	if same {
		return false
	}
	p.Order = next
	return true
}

// ---------- impostazioni avanzate (curve, sensibilità, zone morte, saturazione) ----------

var devOptNameRe = regexp.MustCompile(`<deviceoptions\s+name="([^"]*)"`)

// refreshFromGame rilegge dal file del gioco le impostazioni avanzate, che si regolano nel gioco:
// curve/sensibilità (dentro <options> di ogni joystick), zone morte e saturazione (<deviceoptions>),
// opzioni di tastiera/mouse/gamepad. Vengono legate alla PERIFERICA, non al numero jsN, così quando
// la numerazione cambia l'app le riscrive sotto il numero giusto. Le inversioni restano quelle dell'app.
func (p *Profile) refreshFromGame(disk []byte) bool {
	_, kids, err := splitChildren(disk)
	if err != nil || len(kids) == 0 {
		return false
	}
	type jo struct {
		inst int
		o    xOptions
	}
	var js []jo
	other, extras := []string{}, []string{}
	mods := ""
	for _, k := range kids {
		switch k.name {
		case "options":
			var o xOptions
			if xml.Unmarshal([]byte(k.raw), &o) != nil {
				continue
			}
			if o.Type == "joystick" {
				js = append(js, jo{o.Instance, o})
			} else {
				other = append(other, k.raw)
			}
		case "modifiers":
			mods = k.raw
		case "actionmap", "CustomisationUIHeader":
		default:
			extras = append(extras, k.raw)
		}
	}
	changed := false
	set := func(dst *[]string, v []string) {
		if strings.Join(*dst, "\x00") != strings.Join(v, "\x00") {
			*dst = v
			changed = true
		}
	}
	set(&p.OtherOptions, other)
	set(&p.Extras, extras)
	if mods != "" && mods != p.Modifiers {
		p.Modifiers = mods
		changed = true
	}
	sort.SliceStable(js, func(a, b int) bool { return js[a].inst < js[b].inst })
	count := map[string]int{}
	for _, j := range js {
		g := productGUID(j.o.Product)
		if g == "" {
			continue
		}
		count[g]++
		di := p.Devices[fmt.Sprintf("%s#%d", g, count[g])]
		if di == nil {
			continue
		}
		rest, _ := extractInvert(j.o.Inner)
		rest = strings.TrimSpace(rest)
		if rest == `<flight_move_pitch/>` {
			rest = ""
		}
		if di.Options != rest {
			di.Options = rest
			changed = true
		}
		if di.Product == "" {
			di.Product = j.o.Product
			changed = true
		}
	}
	return changed
}

// advancedSummary: riassunto leggibile delle impostazioni avanzate conservate per ogni periferica.
func (p *Profile) advancedSummary() map[string]string {
	res := map[string]string{}
	for key, di := range p.Devices {
		parts := []string{}
		names := []string{}
		for _, k := range optionKids(di.Options) {
			names = append(names, k.name)
		}
		if len(names) > 0 {
			parts = append(parts, fmt.Sprintf("curve/sensibilità: %s", strings.Join(names, ", ")))
		}
		for _, e := range p.Extras {
			m := devOptNameRe.FindStringSubmatch(e)
			if m == nil || di.GUID == "" || productGUID(m[1]) != di.GUID {
				continue
			}
			n := strings.Count(e, "<option ")
			parts = append(parts, fmt.Sprintf("zone morte/saturazione: %d assi", n))
		}
		if len(parts) > 0 {
			res[key] = strings.Join(parts, " · ")
		}
	}
	return res
}
