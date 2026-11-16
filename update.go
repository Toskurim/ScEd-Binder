package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Aggiornamento automatico: all'avvio l'app chiede a GitHub qual è l'ultima versione pubblicata;
// se è più nuova scarica il file .exe accanto a quello in uso, ne controlla l'impronta SHA-256
// e lo avvia. La nuova versione chiude questa da sola (stesso meccanismo di "versione rimasta aperta").

const updateAPI = "https://api.github.com/repos/Toskurim/ScEd-Binder/releases/latest"
const releasesPage = "https://github.com/Toskurim/ScEd-Binder/releases/latest"

type relAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"` // "sha256:…"
}
type relInfo struct {
	Tag        string     `json:"tag_name"`
	Draft      bool       `json:"draft"`
	Prerelease bool       `json:"prerelease"`
	Assets     []relAsset `json:"assets"`
}

// UpdState è quello che l'interfaccia mostra sull'aggiornamento.
type UpdState struct {
	Status  string `json:"status"` // "", checking, downloading, starting, error
	Version string `json:"version,omitempty"`
	Pct     int    `json:"pct"`
	Msg     string `json:"msg,omitempty"`
}

var (
	updMu sync.Mutex
	upd   UpdState
)

func setUpd(s UpdState) { updMu.Lock(); upd = s; updMu.Unlock() }
func getUpd() UpdState  { updMu.Lock(); defer updMu.Unlock(); return upd }

// verLess dice se la versione a è più vecchia di b ("4.1" < "4.10" < "5.0").
func verLess(a, b string) bool {
	pa, pb := verParts(a), verParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		x, y := 0, 0
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}
func verParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
	out := []int{}
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil // etichetta non numerica: non la considero una versione
		}
		out = append(out, n)
	}
	return out
}

func startUpdateCheck() {
	api := updateAPI
	if u := os.Getenv("SCBIND_UPDATE_URL"); u != "" {
		api = u // per le prove
	} else if runtime.GOOS != "windows" || os.Getenv("SCBIND_NOUPDATE") != "" {
		return
	}
	profMu.Lock()
	off := profile.NoAutoUpdate
	profMu.Unlock()
	if off {
		logf("aggiornamento automatico disattivato nelle Opzioni")
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logf("ERRORE aggiornamento (panic): %v", r)
			}
		}()
		if err := runUpdate(api); err != nil {
			logf("aggiornamento non riuscito: %v", err)
			setUpd(UpdState{Status: "error", Msg: err.Error()})
		}
	}()
}

func runUpdate(api string) error {
	setUpd(UpdState{Status: "checking"})
	cl := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ScEd-Binder/"+appVersion)
	resp, err := cl.Do(req)
	if err != nil {
		setUpd(UpdState{}) // niente rete: non è un errore da mostrare
		logf("controllo aggiornamenti: GitHub non raggiungibile (%v)", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		setUpd(UpdState{})
		logf("controllo aggiornamenti: risposta %d da GitHub", resp.StatusCode)
		return nil
	}
	var rel relInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		setUpd(UpdState{})
		logf("controllo aggiornamenti: risposta non leggibile (%v)", err)
		return nil
	}
	latest := strings.TrimPrefix(rel.Tag, "v")
	if rel.Draft || rel.Prerelease || verParts(rel.Tag) == nil || !verLess(appVersion, latest) {
		setUpd(UpdState{})
		logf("controllo aggiornamenti: sei all'ultima versione (pubblicata: %s)", rel.Tag)
		return nil
	}
	var as *relAsset
	for i := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(rel.Assets[i].Name), ".exe") {
			as = &rel.Assets[i]
			break
		}
	}
	if as == nil || as.URL == "" {
		return fmt.Errorf("la versione %s è pubblicata ma senza file .exe: scaricala a mano da %s", latest, releasesPage)
	}
	if !strings.HasPrefix(as.URL, "https://github.com/Toskurim/ScEd-Binder/") && os.Getenv("SCBIND_UPDATE_URL") == "" {
		return fmt.Errorf("indirizzo di scarico inatteso (%s): aggiornamento annullato", as.URL)
	}
	logf("disponibile la versione %s (in uso: %s)", latest, appVersion)
	name := filepath.Base(as.Name)
	dst := filepath.Join(exeDir, name)
	if self, e := os.Executable(); e == nil && strings.EqualFold(filepath.Clean(self), filepath.Clean(dst)) {
		setUpd(UpdState{})
		logf("il file della versione %s è questo stesso programma: non faccio nulla", latest)
		return nil
	}
	want := strings.TrimPrefix(strings.ToLower(as.Digest), "sha256:")
	if !fileMatches(dst, as.Size, want) {
		setUpd(UpdState{Status: "downloading", Version: latest})
		if err := download(as.URL, dst, as.Size, want, latest); err != nil {
			return fmt.Errorf("scarico della versione %s non riuscito (%v). Puoi scaricarla a mano da %s", latest, err, releasesPage)
		}
		logf("scaricata la versione %s in %s", latest, dst)
	} else {
		logf("la versione %s è già nella cartella: %s", latest, dst)
	}
	setUpd(UpdState{Status: "starting", Version: latest, Pct: 100})
	if os.Getenv("SCBIND_UPDATE_TEST") != "" {
		return nil // prove: non avvio nulla
	}
	cmd := exec.Command(dst)
	cmd.Dir = exeDir
	cmd.Env = append(os.Environ(), "SCBIND_NOBROWSER=1") // la finestra già aperta passa da sola alla nuova versione
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("la versione %s è stata scaricata in %s ma non si avvia (%v): aprila a mano", latest, dst, err)
	}
	logf("avviata la versione %s: questa si chiude appena la nuova è pronta", latest)
	return nil
}

func fileMatches(path string, size int64, sha string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() || (size > 0 && st.Size() != size) {
		return false
	}
	if sha == "" {
		return size > 0
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == sha
}

func download(url, dst string, size int64, sha, ver string) error {
	tmp := dst + ".download"
	defer os.Remove(tmp)
	cl := &http.Client{Timeout: 30 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "ScEd-Binder/"+appVersion)
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("risposta %d", resp.StatusCode)
	}
	if size <= 0 {
		size = resp.ContentLength
	}
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("non posso scrivere nella cartella del programma: %v", err)
	}
	h := sha256.New()
	buf := make([]byte, 256<<10)
	var done int64
	last := -1
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if size > 0 {
				if pct := int(done * 100 / size); pct != last {
					last = pct
					setUpd(UpdState{Status: "downloading", Version: ver, Pct: pct})
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if size > 0 && done != size {
		return fmt.Errorf("file incompleto (%d byte su %d)", done, size)
	}
	if sha != "" && hex.EncodeToString(h.Sum(nil)) != sha {
		return fmt.Errorf("il file scaricato non corrisponde all'impronta pubblicata su GitHub: scartato")
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}
