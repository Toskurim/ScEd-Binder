package main

import (
	"bytes"
	"crypto/sha1"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const appVersion = "4.5"
const listenAddr = "127.0.0.1:47811"

//go:embed web
var webFS embed.FS

//go:embed templates
var tplFS embed.FS

var (
	pingMu   sync.Mutex
	lastPing = time.Now()
	byeAt    time.Time
	profRev  = 1
)

func main() {
	exe, err := os.Executable()
	if err == nil {
		exeDir = filepath.Dir(exe)
	} else {
		exeDir, _ = os.Getwd()
	}
	if d := os.Getenv("SCBIND_DIR"); d != "" {
		exeDir = d
	}
	url := "http://" + listenAddr + "/"
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		// la porta è occupata: c'è già un'istanza dell'app
		running, isApp := runningVersion(url)
		if isApp && running == appVersion {
			openBrowser(url) // stessa versione: riapro solo la finestra
			return
		}
		if !isApp {
			return // la porta è usata da un altro programma: non tocco nulla
		}
		// è una versione diversa rimasta aperta: la chiudo e prendo il suo posto
		if req, e := http.NewRequest("POST", url+"api/quit", nil); e == nil {
			req.Header.Set("X-SCB", "1")
			if resp, e2 := (&http.Client{Timeout: 2 * time.Second}).Do(req); e2 == nil {
				resp.Body.Close()
			}
		}
		for i := 0; i < 40 && err != nil; i++ {
			time.Sleep(250 * time.Millisecond)
			if i == 6 {
				killPortOwner(listenAddr)
			}
			ln, err = net.Listen("tcp", listenAddr)
		}
		if err != nil {
			openBrowser(url)
			return
		}
		replaced = running
	}
	logf("===== ScEd Binder v%s avviato =====", appVersion)
	if replaced != "" {
		logf("chiusa la versione %s rimasta aperta", replaced)
	}
	if err := loadActions(); err != nil {
		logf("ERRORE lettura comandi: %v", err)
	}
	loadPhotoDevices()
	loadEDCommands()
	loadProfile()
	startGameData()
	startDevices()
	routes()
	startUpdateCheck()
	go watchdog()
	if os.Getenv("SCBIND_NOBROWSER") == "" {
		go func() { time.Sleep(300 * time.Millisecond); openBrowser(url) }()
	}
	http.Serve(ln, nil)
}

var replaced string

// runningVersion chiede all'istanza già aperta che versione è. isApp=false se alla porta non risponde ScEd Binder (o SC Binding, il suo vecchio nome).
func runningVersion(url string) (version string, isApp bool) {
	c := &http.Client{Timeout: 2 * time.Second}
	if resp, err := c.Get(url + "api/version"); err == nil {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 100))
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return strings.TrimSpace(string(b)), true
		}
	}
	// le versioni fino alla 2.4 non hanno /api/version: le riconosco dalla pagina
	resp, err := c.Get(url)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	if strings.Contains(string(b), "SC Binding") {
		return "precedente", true
	}
	return "", false
}

func watchdog() {
	for {
		time.Sleep(time.Second)
		pingMu.Lock()
		idle := time.Since(lastPing)
		bye := !byeAt.IsZero() && time.Now().After(byeAt)
		pingMu.Unlock()
		if bye || idle > 180*time.Second {
			logf("finestra chiusa: esco")
			os.Exit(0)
		}
	}
}

func openBrowser(url string) {
	if runtime.GOOS != "windows" {
		return
	}
	cands := []string{}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
		if base := os.Getenv(env); base != "" {
			cands = append(cands, filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`), filepath.Join(base, `Google\Chrome\Application\chrome.exe`))
		}
	}
	sort.SliceStable(cands, func(a, b int) bool {
		return strings.Contains(cands[a], "msedge") && !strings.Contains(cands[b], "msedge")
	})
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			if exec.Command(c, "--app="+url, "--window-size=1600,980").Start() == nil {
				return
			}
		}
	}
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// guard: solo richieste locali fatte dalla pagina dell'app
func guard(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if !strings.HasPrefix(host, "127.0.0.1:") && !strings.HasPrefix(host, "localhost:") {
			http.Error(w, "forbidden", 403)
			return
		}
		if r.Method == "POST" && r.Header.Get("X-SCB") != "1" {
			http.Error(w, "forbidden", 403)
			return
		}
		pingMu.Lock()
		lastPing = time.Now()
		byeAt = time.Time{}
		pingMu.Unlock()
		h(w, r)
	}
}

type devOut struct {
	Device
	JS int `json:"js"`
}

// aggiunge allo stato le informazioni sui comandi letti dal gioco e, ogni tanto, ricontrolla la versione installata
var lastGameCheck time.Time

func withGame(st map[string]any) map[string]any {
	if p, _ := st["selInstall"].(string); p != "" {
		gdMu.Lock()
		due := gdPath != p || time.Since(lastGameCheck) > 30*time.Second
		if due {
			lastGameCheck = time.Now()
		}
		gdMu.Unlock()
		if due {
			go syncGameData(p)
		}
	}
	st["game"] = gameState()
	return st
}

func currentState() map[string]any {
	connected := hub.list()
	profMu.Lock()
	defer profMu.Unlock()
	installs := findInstalls(profile)
	sel := selectedInstall(installs)
	logProd := map[string][]string{}
	selPath := ""
	if sel != nil {
		logProd = gameLogProducts(sel.Path)
		selPath = sel.Path
	}
	changed := profile.ensureDevices(connected, logProd)
	var disk []byte
	gameOrder := []string{}
	if sel != nil {
		disk, _ = os.ReadFile(sel.ActionMaps)
		gameOrder = fileJoystickKeys(disk)
		// curve, zone morte e saturazione si regolano nel gioco: le rileggo ogni volta e le tengo legate alla periferica
		if len(disk) > 0 && profile.refreshFromGame(disk) {
			changed = true
		}
		// se l'ordine non è stato scelto a mano, seguo la numerazione che usa il gioco
		if !profile.OrderManual && profile.adoptGameOrder(gameOrder) {
			changed = true
			logf("ordine periferiche allineato a quello del gioco")
		}
	}
	if changed {
		profRev++
		saveProfile()
	}
	sync := ""
	if sel != nil {
		x, _ := profile.buildXML(connected)
		var err error
		if disk == nil {
			err = os.ErrNotExist
		}
		switch {
		case err == nil && signature(disk) == signature([]byte(x)):
			sync = "ok"
		case err != nil || profile.LastWrite[strings.ToLower(sel.Path)] == sel.Hash:
			sync = "todo"
		default:
			sync = "ext"
		}
	}
	num, _ := profile.numbering(connected)
	out := []devOut{}
	for _, d := range connected {
		out = append(out, devOut{d, num[d.Key]})
	}
	return map[string]any{"devices": out, "installs": installs, "profRev": profRev, "sync": sync, "gameOrder": gameOrder, "version": appVersion, "advanced": profile.advancedSummary(), "devWarn": hub.warning(), "update": getUpd(), "selInstall": selPath}
}

func selectedInstall(installs []Install) *Install {
	for i := range installs {
		if strings.EqualFold(installs[i].Path, profile.Install) {
			return &installs[i]
		}
	}
	for i := range installs {
		if installs[i].Launcher {
			return &installs[i]
		}
	}
	for i := range installs {
		if strings.EqualFold(installs[i].Name, "LIVE") {
			return &installs[i]
		}
	}
	if len(installs) > 0 {
		return &installs[0]
	}
	return nil
}

func templateNames() []string {
	names := []string{}
	entries, _ := tplFS.ReadDir("templates")
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".svg") {
			names = append(names, strings.TrimSuffix(e.Name(), ".svg"))
		}
	}
	sort.Strings(names)
	return names
}

func routes() {
	sub, _ := fs.Sub(webFS, "web")
	static := http.FileServer(http.FS(sub))
	http.HandleFunc("/app.js", func(w http.ResponseWriter, r *http.Request) {
		b, err := fs.ReadFile(sub, "app.js")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(bytes.Replace(b, []byte("__APPVER__"), []byte(appVersion), 1))
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		static.ServeHTTP(w, r)
	})
	http.HandleFunc("/tpl/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/tpl/"))
		b, err := tplFS.ReadFile("templates/" + name + ".svg")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(b)
	})
	http.HandleFunc("/photo/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/photo/"))
		b, err := photoFS.ReadFile("photos/" + name + ".webp")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(b)
	})
	// salva actionmaps.xml nella cartella dell'app (per chi vuole copiarlo a mano nel gioco)
	http.HandleFunc("/api/savefile", guard(func(w http.ResponseWriter, r *http.Request) {
		profMu.Lock()
		defer profMu.Unlock()
		x, st := profile.buildXML(hub.list())
		dir := filepath.Join(dataDir(), "esportati")
		os.MkdirAll(dir, 0755)
		path := filepath.Join(dir, "actionmaps.xml")
		if err := os.WriteFile(path, []byte(x), 0644); err != nil {
			fail(w, 500, "non riesco a salvare il file: "+err.Error())
			return
		}
		st.Path = path
		if runtime.GOOS == "windows" {
			exec.Command("explorer.exe", dir).Start()
		}
		logf("salvato %s", path)
		writeJSON(w, map[string]any{"stats": st})
	}))
	// finestra di Windows per scegliere la cartella del gioco
	http.HandleFunc("/api/browse", guard(func(w http.ResponseWriter, r *http.Request) {
		p, err := pickFolder("Scegli la cartella di Star Citizen (quella che contiene LIVE, oppure LIVE stessa)")
		if err != nil {
			fail(w, 500, "non riesco ad aprire la finestra di scelta cartella: "+err.Error()+". Incolla il percorso a mano.")
			return
		}
		writeJSON(w, map[string]string{"path": p})
	}))
	http.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/img/"))
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:")
		http.ServeFile(w, r, filepath.Join(dataDir(), "immagini", name))
	})

	http.HandleFunc("/api/boot", guard(func(w http.ResponseWriter, r *http.Request) {
		st := withGame(currentState())
		profMu.Lock()
		st["profile"] = profile
		profMu.Unlock()
		st["version"] = appVersion
		st["gameData"] = getGameDataNote()
		st["maps"] = getMaps()
		st["templates"] = templateNames()
		st["invertDefaults"] = getInvertDefaults()
		st["photoDevices"] = photoDevices
		st["dataDir"] = dataDir()
		writeJSON(w, st)
	}))
	http.HandleFunc("/api/state", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, withGame(currentState()))
	}))
	http.HandleFunc("/api/input", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"d": hub.snapshot(), "gp": hub.gamepad()})
	}))
	http.HandleFunc("/api/profile", guard(func(w http.ResponseWriter, r *http.Request) {
		profMu.Lock()
		defer profMu.Unlock()
		if r.Method == "POST" {
			p := newProfile()
			if err := json.NewDecoder(io.LimitReader(r.Body, 32<<20)).Decode(p); err != nil {
				fail(w, 400, "profilo non valido: "+err.Error())
				return
			}
			p.normalize()
			p.LastWrite = profile.LastWrite // gestito solo dal server
			profile = p
			profRev++
			if err := saveProfile(); err != nil {
				fail(w, 500, "impossibile salvare il profilo: "+err.Error())
				return
			}
		}
		writeJSON(w, map[string]any{"profile": profile, "profRev": profRev})
	}))
	http.HandleFunc("/api/import", guard(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		profMu.Lock()
		defer profMu.Unlock()
		in := installByPath(req.Path)
		if in == nil {
			fail(w, 400, "installazione di Star Citizen non trovata")
			return
		}
		data, err := os.ReadFile(in.ActionMaps)
		if err != nil {
			fail(w, 400, "non riesco a leggere "+in.ActionMaps+": "+err.Error())
			return
		}
		p, err := importActionMaps(data, profile)
		if err != nil {
			fail(w, 400, "actionmaps.xml non valido: "+err.Error())
			return
		}
		// copia di sicurezza del profilo precedente
		if b, e := os.ReadFile(profilePath()); e == nil {
			os.MkdirAll(filepath.Join(dataDir(), "backup"), 0755)
			os.WriteFile(filepath.Join(dataDir(), "backup", "profilo_"+time.Now().Format("2006-01-02_15-04-05")+".json"), b, 0644)
		}
		p.Install = in.Path
		p.LastWrite[strings.ToLower(in.Path)] = in.Hash // il file sul disco coincide con il profilo
		p.ensureDevices(hub.list(), gameLogProducts(in.Path))
		profile = p
		profRev++
		saveProfile()
		logf("importato %s: %d assegnazioni, %d periferiche", in.ActionMaps, len(p.Bindings), len(p.Devices))
		writeJSON(w, map[string]any{"profile": profile, "profRev": profRev})
	}))
	http.HandleFunc("/api/preview", guard(func(w http.ResponseWriter, r *http.Request) {
		profMu.Lock()
		defer profMu.Unlock()
		x, st := profile.buildXML(hub.list())
		writeJSON(w, map[string]any{"xml": x, "stats": st})
	}))
	http.HandleFunc("/api/write", guard(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Path string }
		json.NewDecoder(r.Body).Decode(&req)
		profMu.Lock()
		defer profMu.Unlock()
		in := installByPath(req.Path)
		if in == nil {
			fail(w, 400, "installazione di Star Citizen non trovata")
			return
		}
		if cur, err := os.ReadFile(in.ActionMaps); err == nil {
			profile.refreshFromGame(cur) // prendo le curve e le zone morte più recenti impostate nel gioco
		}
		x, st := profile.buildXML(hub.list())
		if _, err := importActionMaps([]byte(x), newProfile()); err != nil {
			fail(w, 500, "errore interno: il file generato non è valido ("+err.Error()+"). Non ho scritto nulla.")
			return
		}
		bk, err := backupFile(in.ActionMaps, in.Name)
		if err != nil {
			fail(w, 500, "non riesco a fare la copia di sicurezza: "+err.Error())
			return
		}
		if err := os.MkdirAll(filepath.Dir(in.ActionMaps), 0755); err != nil {
			fail(w, 500, "non riesco a creare la cartella del profilo: "+err.Error())
			return
		}
		if err := os.WriteFile(in.ActionMaps, []byte(x), 0644); err != nil {
			fail(w, 500, "non riesco a scrivere "+in.ActionMaps+": "+err.Error()+". Chiudi Star Citizen; se la cartella è protetta, avvia l'app come amministratore.")
			return
		}
		st.Path, st.Backup = in.ActionMaps, bk
		profile.Install = in.Path
		profile.LastWrite[strings.ToLower(in.Path)] = fileHash(in.ActionMaps)
		profRev++
		saveProfile()
		logf("layout \"%s\"", profile.Layout)
		logf("scritto %s: %d assegnazioni, %d saltate, %d predefiniti disattivati; %s", in.ActionMaps, st.Written, st.Skipped, st.Cleared, strings.Join(st.Joysticks, ", "))
		writeJSON(w, map[string]any{"stats": st, "profile": profile, "profRev": profRev})
	}))
	http.HandleFunc("/api/upload", guard(func(w http.ResponseWriter, r *http.Request) {
		ext := strings.ToLower(r.URL.Query().Get("ext"))
		ok := map[string]bool{"png": true, "jpg": true, "jpeg": true, "webp": true, "gif": true, "svg": true}
		if !ok[ext] {
			fail(w, 400, "formato immagine non supportato (usa PNG, JPG, WEBP, GIF o SVG)")
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 30<<20))
		if err != nil || len(b) == 0 {
			fail(w, 400, "immagine vuota")
			return
		}
		h := sha1.Sum(b)
		name := hex.EncodeToString(h[:6]) + "." + ext
		dir := filepath.Join(dataDir(), "immagini")
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			fail(w, 500, err.Error())
			return
		}
		writeJSON(w, map[string]string{"image": "img:" + name})
	}))
	// schema condivisibile: immagine + posizione delle etichette di una periferica
	http.HandleFunc("/api/schema", guard(func(w http.ResponseWriter, r *http.Request) {
		type schema struct {
			Format   string              `json:"format"`
			Name     string              `json:"name"`
			GUID     string              `json:"guid"`
			Template string              `json:"template,omitempty"`
			Ext      string              `json:"ext,omitempty"`
			Data     []byte              `json:"data,omitempty"`
			Hotspots []Hotspot           `json:"hotspots"`
			PhotoMap map[string][]string `json:"photoMap,omitempty"`
		}
		if r.Method == "POST" {
			var sc schema
			if err := json.NewDecoder(io.LimitReader(r.Body, 60<<20)).Decode(&sc); err != nil || sc.Format != "scbinding-schema-1" {
				fail(w, 400, "il file non è uno schema di ScEd Binder")
				return
			}
			image := ""
			if strings.HasPrefix(sc.Template, "foto:") {
				image = "foto:" + filepath.Base(strings.TrimPrefix(sc.Template, "foto:"))
			} else if sc.Template != "" {
				image = "tpl:" + filepath.Base(sc.Template)
			} else {
				ok := map[string]bool{"png": true, "jpg": true, "jpeg": true, "webp": true, "gif": true, "svg": true}
				if !ok[sc.Ext] || len(sc.Data) == 0 {
					fail(w, 400, "lo schema non contiene un'immagine valida")
					return
				}
				h := sha1.Sum(sc.Data)
				name := hex.EncodeToString(h[:6]) + "." + sc.Ext
				dir := filepath.Join(dataDir(), "immagini")
				os.MkdirAll(dir, 0755)
				if err := os.WriteFile(filepath.Join(dir, name), sc.Data, 0644); err != nil {
					fail(w, 500, err.Error())
					return
				}
				image = "img:" + name
			}
			if sc.Hotspots == nil {
				sc.Hotspots = []Hotspot{}
			}
			writeJSON(w, map[string]any{"image": image, "hotspots": sc.Hotspots, "name": sc.Name, "photoMap": sc.PhotoMap})
			return
		}
		profMu.Lock()
		di := profile.Devices[r.URL.Query().Get("key")]
		profMu.Unlock()
		if di == nil || di.Image == "" {
			fail(w, 400, "questa periferica non ha un'immagine")
			return
		}
		sc := schema{Format: "scbinding-schema-1", Name: di.Name, GUID: di.GUID, Hotspots: di.Hotspots}
		if strings.HasPrefix(di.Image, "foto:") {
			sc.Template = di.Image
			sc.PhotoMap = di.PhotoMap[strings.TrimPrefix(di.Image, "foto:")]
		} else if strings.HasPrefix(di.Image, "tpl:") {
			sc.Template = strings.TrimPrefix(di.Image, "tpl:")
		} else {
			file := filepath.Base(strings.TrimPrefix(di.Image, "img:"))
			b, err := os.ReadFile(filepath.Join(dataDir(), "immagini", file))
			if err != nil {
				fail(w, 500, "immagine non trovata")
				return
			}
			sc.Ext = strings.TrimPrefix(filepath.Ext(file), ".")
			sc.Data = b
		}
		writeJSON(w, sc)
	}))
	http.HandleFunc("/api/open", guard(func(w http.ResponseWriter, r *http.Request) {
		var req struct{ What, Path string }
		json.NewDecoder(r.Body).Decode(&req)
		target := dataDir()
		if req.What == "game" {
			profMu.Lock()
			in := installByPath(req.Path)
			profMu.Unlock()
			if in == nil {
				fail(w, 400, "cartella non trovata")
				return
			}
			target = filepath.Dir(in.ActionMaps)
		}
		if runtime.GOOS == "windows" {
			exec.Command("explorer.exe", target).Start()
		}
		writeJSON(w, map[string]string{"path": target})
	}))
	http.HandleFunc("/api/log", guard(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, getLog())
	}))
	http.HandleFunc("/api/ed/state", guard(func(w http.ResponseWriter, r *http.Request) {
		profMu.Lock()
		defer profMu.Unlock()
		writeJSON(w, edState())
	}))
	http.HandleFunc("/api/ed/write", guard(func(w http.ResponseWriter, r *http.Request) {
		profMu.Lock()
		defer profMu.Unlock()
		res, err := edWrite()
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		writeJSON(w, res)
	}))
	http.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(appVersion))
	})
	http.HandleFunc("/api/quit", guard(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
		logf("richiesta di chiusura da una versione più recente: esco")
		go func() { time.Sleep(200 * time.Millisecond); os.Exit(0) }()
	}))
	http.HandleFunc("/api/bye", func(w http.ResponseWriter, r *http.Request) {
		pingMu.Lock()
		byeAt = time.Now().Add(5 * time.Second)
		pingMu.Unlock()
	})
}

func installByPath(path string) *Install {
	installs := findInstalls(profile)
	for i := range installs {
		if strings.EqualFold(installs[i].Path, path) {
			return &installs[i]
		}
	}
	return nil
}

var _ = fmt.Sprintf
