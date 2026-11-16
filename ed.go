package main

import (
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Supporto a Elite Dangerous: lettura e scrittura del file dei comandi (.binds).

//go:embed data/ed_commands.json
var edCommandsJSON []byte

type EDCommand struct {
	Tag   string `json:"tag"`
	EN    string `json:"en"`
	IT    string `json:"it"`
	Group string `json:"group"`
	Kind  string `json:"kind"` // axis | button | toggle
}

var edCommands []EDCommand

func loadEDCommands() {
	if err := json.Unmarshal(edCommandsJSON, &edCommands); err != nil {
		logf("ERRORE lettura comandi Elite: %v", err)
	}
}

// EDSlot: un'assegnazione (periferica + tasto), come nel file di Elite.
type EDSlot struct {
	D string   `json:"d"`           // Device: codice USB (VID+PID), nome noto a Elite, Keyboard, Mouse o {NoDevice}
	I string   `json:"i,omitempty"` // DeviceIndex (seconda periferica identica = "1")
	K string   `json:"k"`           // Key: Joy_5, Joy_XAxis, Pos_Joy_RXAxis, Joy_POV1Up, Key_A ...
	M []EDSlot `json:"m,omitempty"` // modificatori
	H string   `json:"h,omitempty"` // Hold (tieni premuto), dove presente
}

type EDBind struct {
	P   *EDSlot `json:"p,omitempty"`   // primario
	S   *EDSlot `json:"s,omitempty"`   // secondario
	A   *EDSlot `json:"a,omitempty"`   // asse
	Inv bool    `json:"inv,omitempty"` // asse invertito
	DZ  string  `json:"dz,omitempty"`  // zona morta
	Tog string  `json:"tog,omitempty"` // ToggleOn (0/1), se il comando lo prevede
}

type EDState struct {
	Bindings map[string]*EDBind `json:"bindings"`
	Imported bool               `json:"imported"`
}

type xEDSlot struct {
	Device   string    `xml:"Device,attr"`
	Index    string    `xml:"DeviceIndex,attr"`
	Key      string    `xml:"Key,attr"`
	Modifier []xEDSlot `xml:"Modifier"`
	Hold     *xEDVal   `xml:"Hold"`
}
type xEDVal struct {
	Value string `xml:"Value,attr"`
}
type xEDBind struct {
	Primary   *xEDSlot `xml:"Primary"`
	Secondary *xEDSlot `xml:"Secondary"`
	Binding   *xEDSlot `xml:"Binding"`
	Inverted  *xEDVal  `xml:"Inverted"`
	Deadzone  *xEDVal  `xml:"Deadzone"`
	ToggleOn  *xEDVal  `xml:"ToggleOn"`
}

func edSlotFromX(x *xEDSlot) *EDSlot {
	if x == nil {
		return nil
	}
	s := &EDSlot{D: x.Device, I: x.Index, K: x.Key}
	if x.Hold != nil {
		s.H = x.Hold.Value
	}
	for _, m := range x.Modifier {
		s.M = append(s.M, EDSlot{D: m.Device, I: m.Index, K: m.Key})
	}
	return s
}

func edDir() string {
	if d := os.Getenv("SCBIND_ED_DIR"); d != "" {
		return d
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Frontier Developments", "Elite Dangerous", "Options", "Bindings")
}

var edFileRe = regexp.MustCompile(`^(.+)\.(\d+)\.(\d+)\.binds$`)

type EDFile struct {
	Dir    string   `json:"dir"`
	Path   string   `json:"path"`
	Preset string   `json:"preset"`
	Start  []string `json:"start"` // preset scelti nel gioco (generale, nave, SRV, a piedi)
	Error  string   `json:"error,omitempty"`
}

// edActiveFile trova il file .binds del preset personalizzato in uso.
func edActiveFile() EDFile {
	f := EDFile{Dir: edDir(), Start: []string{}}
	if f.Dir == "" {
		f.Error = "Elite Dangerous non trovato su questo PC"
		return f
	}
	entries, err := os.ReadDir(f.Dir)
	if err != nil {
		f.Error = "Non trovo la cartella dei comandi di Elite Dangerous (" + f.Dir + "). Il gioco è installato ed è stato avviato almeno una volta?"
		return f
	}
	for _, name := range []string{"StartPreset.4.start", "StartPreset.start"} {
		if b, err := os.ReadFile(filepath.Join(f.Dir, name)); err == nil {
			for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
				if strings.TrimSpace(l) != "" {
					f.Start = append(f.Start, strings.TrimSpace(l))
				}
			}
			break
		}
	}
	type cand struct {
		name       string
		maj, minor int
		path       string
	}
	var cands []cand
	for _, e := range entries {
		if m := edFileRe.FindStringSubmatch(e.Name()); m != nil {
			a, _ := strconv.Atoi(m[2])
			b, _ := strconv.Atoi(m[3])
			cands = append(cands, cand{m[1], a, b, filepath.Join(f.Dir, e.Name())})
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].maj != cands[j].maj {
			return cands[i].maj > cands[j].maj
		}
		return cands[i].minor > cands[j].minor
	})
	want := "Custom"
	if len(f.Start) > 0 {
		want = f.Start[0]
	}
	for _, pass := range []string{want, "Custom", ""} {
		for _, c := range cands {
			if pass == "" || strings.EqualFold(c.name, pass) {
				f.Path, f.Preset = c.path, c.name
				return f
			}
		}
	}
	f.Error = "In Elite Dangerous non c'è ancora un preset di comandi personalizzato. Apri il gioco, vai in Opzioni > Comandi, cambia un comando qualsiasi e conferma: il gioco creerà il file \"Custom\" su cui l'app può lavorare."
	return f
}

type edDoc struct {
	attrs []xml.Attr
	kids  []rawEl
	crlf  bool
	binds map[string]*EDBind
}

func parseBinds(data []byte) (*edDoc, error) {
	attrs, kids, err := splitChildren(data)
	if err != nil {
		return nil, err
	}
	d := &edDoc{attrs: attrs, kids: kids, crlf: strings.Contains(string(data), "\r\n"), binds: map[string]*EDBind{}}
	for _, k := range kids {
		var x xEDBind
		if xml.Unmarshal([]byte(k.raw), &x) != nil {
			continue
		}
		if x.Primary == nil && x.Secondary == nil && x.Binding == nil {
			continue
		}
		if _, dup := d.binds[k.name]; dup {
			continue
		}
		b := &EDBind{P: edSlotFromX(x.Primary), S: edSlotFromX(x.Secondary), A: edSlotFromX(x.Binding)}
		if x.Inverted != nil {
			b.Inv = x.Inverted.Value == "1"
		}
		if x.Deadzone != nil {
			b.DZ = x.Deadzone.Value
		}
		if x.ToggleOn != nil {
			b.Tog = x.ToggleOn.Value
		}
		d.binds[k.name] = b
	}
	return d, nil
}

func edSlotXML(tag string, s *EDSlot, ok func(EDSlot) bool, skipped *int) string {
	if s == nil || s.D == "" || s.D == "{NoDevice}" || s.K == "" {
		return fmt.Sprintf("\t\t<%s Device=\"{NoDevice}\" Key=\"\" />\n", tag)
	}
	if !ok(*s) {
		*skipped++
		return fmt.Sprintf("\t\t<%s Device=\"{NoDevice}\" Key=\"\" />\n", tag)
	}
	attr := func(x EDSlot) string {
		a := fmt.Sprintf(`Device="%s"`, xmlEsc(x.D))
		if x.I != "" {
			a += fmt.Sprintf(` DeviceIndex="%s"`, xmlEsc(x.I))
		}
		return a + fmt.Sprintf(` Key="%s"`, xmlEsc(x.K))
	}
	mods := []EDSlot{}
	for _, m := range s.M {
		if ok(m) {
			mods = append(mods, m)
		}
	}
	if len(mods) == 0 && s.H == "" {
		return fmt.Sprintf("\t\t<%s %s />\n", tag, attr(*s))
	}
	out := fmt.Sprintf("\t\t<%s %s>\n", tag, attr(*s))
	for _, m := range mods {
		out += fmt.Sprintf("\t\t\t<Modifier %s />\n", attr(m))
	}
	if s.H != "" {
		out += fmt.Sprintf("\t\t\t<Hold Value=\"%s\" />\n", xmlEsc(s.H))
	}
	return out + fmt.Sprintf("\t\t</%s>\n", tag)
}

// generate ricostruisce il file: le impostazioni restano com'erano, le assegnazioni vengono da binds.
// ok decide se una periferica è utilizzabile (collegata); le altre assegnazioni vengono lasciate vuote.
func (d *edDoc) generate(binds map[string]*EDBind, ok func(EDSlot) bool) (string, int) {
	skipped := 0
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n<Root")
	for _, a := range d.attrs {
		fmt.Fprintf(&sb, ` %s="%s"`, a.Name.Local, xmlEsc(a.Value))
	}
	sb.WriteString(">\n")
	seen := map[string]bool{}
	for _, k := range d.kids {
		orig, isBind := d.binds[k.name]
		if !isBind || seen[k.name] {
			sb.WriteString("\t" + strings.ReplaceAll(strings.TrimSpace(k.raw), "\r\n", "\n") + "\n")
			continue
		}
		seen[k.name] = true
		b := binds[k.name]
		if b == nil {
			b = &EDBind{}
		}
		fmt.Fprintf(&sb, "\t<%s>\n", k.name)
		if orig.A != nil { // comando ad asse
			sb.WriteString(edSlotXML("Binding", b.A, ok, &skipped))
			inv := "0"
			if b.Inv {
				inv = "1"
			}
			dz := b.DZ
			if dz == "" {
				dz = orig.DZ
			}
			if dz == "" {
				dz = "0.00000000"
			}
			fmt.Fprintf(&sb, "\t\t<Inverted Value=\"%s\" />\n\t\t<Deadzone Value=\"%s\" />\n", inv, xmlEsc(dz))
		} else {
			sb.WriteString(edSlotXML("Primary", b.P, ok, &skipped))
			sb.WriteString(edSlotXML("Secondary", b.S, ok, &skipped))
			if orig.Tog != "" {
				tog := b.Tog
				if tog == "" {
					tog = orig.Tog
				}
				fmt.Fprintf(&sb, "\t\t<ToggleOn Value=\"%s\" />\n", xmlEsc(tog))
			}
		}
		fmt.Fprintf(&sb, "\t</%s>\n", k.name)
	}
	sb.WriteString("</Root>\n")
	out := sb.String()
	if d.crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out, skipped
}

// edDeviceNames: periferiche che Elite chiama per nome invece che per codice (da DeviceMappings.xml del gioco).
func edDeviceNames() map[string]string {
	res := map[string]string{}
	var files []string
	if p := os.Getenv("SCBIND_ED_MAPPINGS"); p != "" {
		files = append(files, p)
	}
	if runtime.GOOS == "windows" {
		subs := []string{`SteamLibrary\steamapps\common\Elite Dangerous\Products`, `Program Files (x86)\Steam\steamapps\common\Elite Dangerous\Products`,
			`Steam\steamapps\common\Elite Dangerous\Products`, `Program Files (x86)\Frontier\Products`, `Program Files\Epic Games\EliteDangerous\Products`, `Epic Games\EliteDangerous\Products`}
		for c := 'C'; c <= 'Z'; c++ {
			drive := string(c) + `:\`
			if _, err := os.Stat(drive); err != nil {
				continue
			}
			for _, s := range subs {
				m, _ := filepath.Glob(filepath.Join(drive+s, "*", "ControlSchemes", "DeviceMappings.xml"))
				files = append(files, m...)
			}
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		_, kids, err := splitChildren(b)
		if err != nil {
			continue
		}
		vidRe := regexp.MustCompile(`(?is)<VID>\s*([0-9A-Fa-f]{4})\s*</VID>`)
		pidRe := regexp.MustCompile(`(?is)<PID>\s*([0-9A-Fa-f]{4})\s*</PID>`)
		for _, k := range kids {
			v, p := vidRe.FindStringSubmatch(k.raw), pidRe.FindStringSubmatch(k.raw)
			if v != nil && p != nil {
				res[strings.ToUpper(v[1]+p[1])] = k.name
			}
		}
		if len(res) > 0 {
			break
		}
	}
	return res
}

var (
	edNamesCache map[string]string
	edNamesAt    time.Time
)

func edNames() map[string]string {
	if edNamesCache == nil || time.Since(edNamesAt) > 5*time.Minute {
		edNamesCache, edNamesAt = edDeviceNames(), time.Now()
	}
	return edNamesCache
}

// edID: come Elite chiama una periferica collegata (codice VID+PID oppure nome noto) e il suo indice.
func edID(d Device, names map[string]string) (string, string) {
	g := strings.Trim(d.GUID, "{}")
	if len(g) < 8 {
		return "", ""
	}
	id := strings.ToUpper(g[4:8] + g[0:4])
	if n, ok := names[id]; ok {
		id = n
	}
	idx := ""
	if i := strings.LastIndex(d.Key, "#"); i >= 0 {
		if n, _ := strconv.Atoi(d.Key[i+1:]); n > 1 {
			idx = strconv.Itoa(n - 1)
		}
	}
	return id, idx
}

func edConnected() (map[string]bool, map[string][2]string) {
	names := edNames()
	set := map[string]bool{"Keyboard": true, "Mouse": true}
	byKey := map[string][2]string{}
	for _, d := range hub.list() {
		if d.Gamepad {
			continue
		}
		id, idx := edID(d, names)
		if id == "" {
			continue
		}
		set[id+"|"+idx] = true
		byKey[d.Key] = [2]string{id, idx}
	}
	return set, byKey
}

func edState() map[string]any {
	f := edActiveFile()
	res := map[string]any{"file": f, "commands": edCommands, "sync": "", "found": f.Error == ""}
	set, byKey := edConnected()
	res["devIds"] = byKey
	if f.Error != "" {
		return res
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		f.Error = "Non riesco a leggere " + f.Path
		res["file"], res["found"] = f, false
		return res
	}
	doc, err := parseBinds(data)
	if err != nil {
		f.Error = "File dei comandi di Elite non valido: " + err.Error()
		res["file"], res["found"] = f, false
		return res
	}
	res["disk"] = doc.binds
	tags := []string{}
	for _, k := range doc.kids {
		if _, ok := doc.binds[k.name]; ok {
			tags = append(tags, k.name)
		}
	}
	res["tags"] = tags
	_ = set
	if profile.ED != nil && profile.ED.Imported {
		all := func(EDSlot) bool { return true }
		a, _ := doc.generate(profile.ED.Bindings, all)
		b, _ := doc.generate(doc.binds, all)
		if a == b {
			res["sync"] = "ok"
		} else {
			res["sync"] = "todo"
		}
	}
	return res
}

func edWrite() (map[string]any, error) {
	f := edActiveFile()
	if f.Error != "" {
		return nil, fmt.Errorf("%s", f.Error)
	}
	if profile.ED == nil || !profile.ED.Imported {
		return nil, fmt.Errorf("prima importa i comandi da Elite Dangerous")
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, fmt.Errorf("non riesco a leggere %s", f.Path)
	}
	doc, err := parseBinds(data)
	if err != nil {
		return nil, err
	}
	// le assegnazioni di periferiche scollegate restano nel file: Elite le ignora finché la periferica non torna
	out, skipped := doc.generate(profile.ED.Bindings, func(EDSlot) bool { return true })
	if _, err := parseBinds([]byte(out)); err != nil {
		return nil, fmt.Errorf("errore interno: il file generato non è valido (%v). Non ho scritto nulla", err)
	}
	dir := filepath.Join(dataDir(), "backup")
	os.MkdirAll(dir, 0755)
	bk := filepath.Join(dir, fmt.Sprintf("elite_%s_%s.binds", f.Preset, time.Now().Format("2006-01-02_15-04-05")))
	if err := os.WriteFile(bk, data, 0644); err != nil {
		return nil, fmt.Errorf("non riesco a fare la copia di sicurezza: %v", err)
	}
	if err := os.WriteFile(f.Path, []byte(out), 0644); err != nil {
		return nil, fmt.Errorf("non riesco a scrivere %s: %v", f.Path, err)
	}
	logf("Elite: scritto %s", f.Path)
	return map[string]any{"path": f.Path, "backup": bk, "skipped": skipped}, nil
}
