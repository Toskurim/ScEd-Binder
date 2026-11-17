package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Comandi letti dal gioco installato.
// A ogni avvio leggo la versione del gioco (build_manifest.id). Se è una versione che non ho ancora visto,
// estraggo dall'archivio Data.p4k l'elenco dei comandi (defaultProfile.xml) e i loro nomi inglesi (global.ini),
// li salvo in dati\gioco\<versione>\ e ricarico l'elenco: così l'app segue le patch senza una nuova release.

const labelsITURL = "https://raw.githubusercontent.com/Toskurim/ScEd-Binder/main/data/labels_it.json"

type ChangeItem struct {
	Map      string `json:"map"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	MapLabel string `json:"mapLabel"`
}
type GameChange struct {
	From    string       `json:"from"`
	To      string       `json:"to"`
	At      string       `json:"at"`
	Added   []ChangeItem `json:"added"`
	Removed []ChangeItem `json:"removed"`
}
type GameDataState struct {
	Version string      `json:"version,omitempty"` // versione del gioco installato
	Branch  string      `json:"branch,omitempty"`
	Status  string      `json:"status"` // "", reading, ok, error
	Msg     string      `json:"msg,omitempty"`
	Note    string      `json:"note"`
	Change  *GameChange `json:"change,omitempty"`
	Rev     int         `json:"rev"`
}

var (
	gdMu       sync.Mutex
	gd         GameDataState
	gdPath     string // installazione per cui i dati sono stati caricati
	gdBusy     bool
	gdFailed   = map[string]bool{} // versioni per cui l'estrazione è già fallita in questa sessione
	keyRefRe   = regexp.MustCompile(`UI(?:Label|Description|Category)="@([^"]+)"`)
	safeNameRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)

func gameState() GameDataState {
	gdMu.Lock()
	s := gd
	gdMu.Unlock()
	s.Note = getGameDataNote()
	profMu.Lock()
	seen := profile.GameDataSeen
	profMu.Unlock()
	if s.Change != nil && s.Change.To == seen {
		s.Change = nil
	}
	return s
}

func readManifest(install string) (ver, branch string) {
	b, err := os.ReadFile(filepath.Join(install, "build_manifest.id"))
	if err != nil {
		return "", ""
	}
	var m struct {
		Data struct {
			Branch  string `json:"Branch"`
			Version string `json:"Version"`
			Change  string `json:"RequestedP4ChangeNum"`
		} `json:"Data"`
	}
	if json.Unmarshal(b, &m) != nil {
		return "", ""
	}
	ver = strings.TrimSpace(m.Data.Version)
	if ver == "" {
		ver = strings.TrimSpace(m.Data.Change)
	}
	return ver, strings.TrimSpace(m.Data.Branch)
}

func gameCacheDir(ver string) string {
	return filepath.Join(dataDir(), "gioco", safeNameRe.ReplaceAllString(ver, "_"))
}

func gameNote(ver, branch string) string {
	n := "Star Citizen " + strings.TrimPrefix(branch, "sc-alpha-")
	if branch == "" {
		n = "Star Citizen"
	}
	return fmt.Sprintf("%s, build %s (letto dal gioco installato)", n, ver)
}

func parseINI(b []byte, keep map[string]bool) map[string]string {
	out := map[string]string{}
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) // BOM
	for _, line := range strings.Split(string(b), "\n") {
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(strings.SplitN(line[:i], ",", 2)[0]))
		if keep != nil && !keep[k] {
			continue
		}
		out[k] = strings.TrimSpace(strings.ReplaceAll(strings.TrimRight(line[i+1:], "\r"), `\n`, " "))
	}
	return out
}

func refKeys(xml []byte) map[string]bool {
	keys := map[string]bool{}
	for _, m := range keyRefRe.FindAllSubmatch(xml, -1) {
		keys[strings.ToLower(string(m[1]))] = true
	}
	return keys
}

// traduzione italiana della comunità, se l'utente l'ha installata nel gioco
func localItalian(install string, keys map[string]bool) map[string]string {
	dir := filepath.Join(install, "data", "Localization")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "italian") {
			if b, err := os.ReadFile(filepath.Join(dir, e.Name(), "global.ini")); err == nil && len(b) < 200<<20 {
				return parseINI(b, keys)
			}
		}
	}
	return nil
}

func loadGameCache(ver string) *gameSource {
	dir := gameCacheDir(ver)
	x, err := os.ReadFile(filepath.Join(dir, "defaultProfile.xml"))
	if err != nil || len(x) < 1000 {
		return nil
	}
	src := &gameSource{XML: x, EN: map[string]string{}}
	if b, err := os.ReadFile(filepath.Join(dir, "labels_en.json")); err == nil {
		json.Unmarshal(b, &src.EN)
	}
	return src
}

func extractGameData(install, ver string) (*gameSource, error) {
	t0 := time.Now()
	f, err := os.Open(filepath.Join(install, "Data.p4k"))
	if err != nil {
		return nil, fmt.Errorf("non riesco ad aprire Data.p4k (%v)", err)
	}
	defer f.Close()
	const pXML, pINI = "data/libs/config/defaultprofile.xml", "data/localization/english/global.ini"
	ents, n, err := p4kFind(f, []string{pXML, pINI})
	if err != nil {
		return nil, err
	}
	ex, ok := ents[pXML]
	if !ok {
		return nil, fmt.Errorf("defaultProfile.xml non trovato nell'archivio (%d file esaminati)", n)
	}
	raw, err := p4kRead(f, ex)
	if err != nil {
		return nil, err
	}
	xmlData := raw
	if isCryXML(raw) {
		if xmlData, err = cryXMLToText(raw); err != nil {
			return nil, err
		}
	} else if !strings.Contains(string(raw[:min(len(raw), 400)]), "<") {
		return nil, fmt.Errorf("defaultProfile.xml in un formato non riconosciuto (inizia con %q)", string(raw[:min(len(raw), 12)]))
	}
	src := &gameSource{XML: xmlData, EN: map[string]string{}}
	if ei, ok := ents[pINI]; ok {
		if b, err := p4kRead(f, ei); err == nil {
			src.EN = parseINI(b, refKeys(xmlData))
		} else {
			logf("nomi inglesi dei comandi non letti dal gioco (%v): uso quelli integrati", err)
		}
	} else {
		logf("global.ini inglese non trovato nell'archivio: uso i nomi integrati")
	}
	dir := gameCacheDir(ver)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "defaultProfile.xml"), xmlData, 0644); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(src.EN)
	os.WriteFile(filepath.Join(dir, "labels_en.json"), b, 0644)
	logf("letti dal gioco (Data.p4k, %s): defaultProfile.xml %d byte, %d nomi inglesi, in %.1f s", install, len(xmlData), len(src.EN), time.Since(t0).Seconds())
	return src, nil
}

func snapshotActions() map[string]ChangeItem {
	out := map[string]ChangeItem{}
	for _, am := range getMaps() {
		for _, a := range am.Actions {
			if a.Visible {
				out[a.Map+"/"+a.Name] = ChangeItem{Map: a.Map, Name: a.Name, Label: a.Label, MapLabel: am.Label}
			}
		}
	}
	return out
}

// applica una sorgente e ricarica l'elenco; se non è valida torna a quella di prima
func applyGameSource(src *gameSource) error {
	actMu.Lock()
	old := gameSrc
	gameSrc = src
	actMu.Unlock()
	if err := loadActions(); err != nil {
		actMu.Lock()
		gameSrc = old
		actMu.Unlock()
		loadActions()
		return err
	}
	return nil
}

func changePath() string { return filepath.Join(dataDir(), "gioco", "ultimo_cambio.json") }

// syncGameData allinea l'elenco dei comandi alla versione del gioco dell'installazione scelta.
// wait=true: carica subito ciò che è già in cache (avvio); l'estrazione dall'archivio avviene sempre in secondo piano.
func syncGameData(install string) {
	if install == "" {
		return
	}
	ver, branch := readManifest(install)
	gdMu.Lock()
	if gdBusy || (gdPath == install && gd.Version == ver) || ver == "" || gdFailed[ver] {
		if ver == "" && gdPath != install {
			gdPath = install
			gdMu.Unlock()
			logf("versione del gioco non leggibile in %s (manca build_manifest.id): uso l'elenco comandi integrato", install)
			return
		}
		gdMu.Unlock()
		return
	}
	gdBusy = true
	prevVer := gd.Version
	gdMu.Unlock()
	finish := func(st, msg string, ch *GameChange, bump bool) {
		gdMu.Lock()
		gdBusy = false
		gdPath, gd.Version, gd.Branch, gd.Status, gd.Msg = install, ver, branch, st, msg
		if ch != nil {
			gd.Change = ch
		}
		if bump {
			gd.Rev++
		}
		gdMu.Unlock()
	}
	before := snapshotActions()
	src := loadGameCache(ver)
	fresh := false
	if src == nil {
		gdMu.Lock()
		gd.Status, gd.Msg = "reading", ""
		gdMu.Unlock()
		var err error
		if src, err = extractGameData(install, ver); err != nil {
			logf("lettura dei comandi dal gioco non riuscita (versione %s): %v. Uso l'elenco già caricato", ver, err)
			gdMu.Lock()
			gdFailed[ver] = true
			gdMu.Unlock()
			finish("error", err.Error(), nil, false)
			return
		}
		fresh = true
	}
	src.Note = gameNote(ver, branch)
	src.ITLocal = localItalian(install, refKeys(src.XML))
	if err := applyGameSource(src); err != nil {
		logf("i comandi letti dal gioco (versione %s) non sono utilizzabili: %v. Uso l'elenco già caricato", ver, err)
		if fresh {
			os.RemoveAll(gameCacheDir(ver))
		}
		gdMu.Lock()
		gdFailed[ver] = true
		gdMu.Unlock()
		finish("error", err.Error(), nil, false)
		return
	}
	after := snapshotActions()
	var ch *GameChange
	if fresh {
		c := GameChange{From: prevVer, To: ver, At: time.Now().Format("02/01/2006 15:04")}
		if c.From == "" {
			c.From = "elenco integrato (4.10.2)"
		}
		for k, v := range after {
			if _, ok := before[k]; !ok {
				c.Added = append(c.Added, v)
			}
		}
		for k, v := range before {
			if _, ok := after[k]; !ok {
				c.Removed = append(c.Removed, v)
			}
		}
		by := func(l []ChangeItem) {
			sort.Slice(l, func(i, j int) bool { return l[i].MapLabel+l[i].Label < l[j].MapLabel+l[j].Label })
		}
		by(c.Added)
		by(c.Removed)
		logf("comandi del gioco versione %s: %d in elenco, %d nuovi e %d non più presenti rispetto a %s", ver, len(after), len(c.Added), len(c.Removed), c.From)
		if len(c.Added)+len(c.Removed) > 0 {
			ch = &c
			if b, err := json.MarshalIndent(c, "", " "); err == nil {
				os.WriteFile(changePath(), b, 0644)
			}
		}
	} else {
		logf("comandi del gioco versione %s: %d in elenco (già letti in precedenza)", ver, len(after))
	}
	finish("ok", "", ch, true)
}

// all'avvio: carica l'ultimo riepilogo dei cambiamenti e allinea i comandi all'installazione scelta
func startGameData() {
	if b, err := os.ReadFile(changePath()); err == nil {
		var c GameChange
		if json.Unmarshal(b, &c) == nil && c.To != "" {
			gdMu.Lock()
			gd.Change = &c
			gdMu.Unlock()
		}
	}
	if b, err := os.ReadFile(filepath.Join(dataDir(), "gioco", "labels_it_online.json")); err == nil {
		m := map[string]string{}
		if json.Unmarshal(b, &m) == nil && len(m) > 500 {
			actMu.Lock()
			onlineIT = m
			actMu.Unlock()
			loadActions()
		}
	}
	profMu.Lock()
	sel := selectedInstall(findInstalls(profile))
	profMu.Unlock()
	if sel != nil {
		if ver, _ := readManifest(sel.Path); ver != "" && loadGameCache(ver) != nil {
			syncGameData(sel.Path) // già in cache: immediato
		} else {
			go syncGameData(sel.Path)
		}
	}
	go fetchOnlineIT()
}

// traduzioni italiane aggiornate: un file piccolo su GitHub, così i nomi dei comandi nuovi arrivano senza una nuova release
func fetchOnlineIT() {
	url := labelsITURL
	if u := os.Getenv("SCBIND_LABELS_URL"); u != "" {
		url = u
	} else if runtime.GOOS != "windows" || os.Getenv("SCBIND_NOUPDATE") != "" {
		return
	}
	profMu.Lock()
	off := profile.NoAutoUpdate
	profMu.Unlock()
	if off {
		return
	}
	defer func() { recover() }()
	cl := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "ScEd-Binder/"+appVersion)
	resp, err := cl.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		logf("traduzioni aggiornate: risposta %d da GitHub", resp.StatusCode)
		return
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	m := map[string]string{}
	if err != nil || json.Unmarshal(b, &m) != nil || len(m) < 500 {
		logf("traduzioni aggiornate: file non valido, ignorato")
		return
	}
	actMu.RLock()
	same := len(onlineIT) == len(m)
	if same {
		for k, v := range m {
			if onlineIT[k] != v {
				same = false
				break
			}
		}
	}
	actMu.RUnlock()
	if same {
		return
	}
	before := snapshotActions()
	os.MkdirAll(filepath.Join(dataDir(), "gioco"), 0755)
	os.WriteFile(filepath.Join(dataDir(), "gioco", "labels_it_online.json"), b, 0644)
	actMu.Lock()
	onlineIT = m
	actMu.Unlock()
	if loadActions() != nil {
		return
	}
	n := 0
	for k, v := range snapshotActions() {
		if before[k].Label != v.Label {
			n++
		}
	}
	if n > 0 {
		logf("traduzioni aggiornate da GitHub: %d nomi di comandi cambiati", n)
		gdMu.Lock()
		gd.Rev++
		gdMu.Unlock()
	}
}
