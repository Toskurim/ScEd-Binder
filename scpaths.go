package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

type Install struct {
	Name       string `json:"name"` // LIVE, PTU, ...
	Path       string `json:"path"`
	ActionMaps string `json:"actionMaps"`
	Exists     bool   `json:"exists"`  // actionmaps.xml presente
	Changed    bool   `json:"changed"` // modificato dopo l'ultima scrittura dell'app
	Hash       string `json:"hash"`
	LastRun    int64  `json:"lastRun"`  // ultimo avvio del gioco da questa cartella (Game.log)
	Launcher   bool   `json:"launcher"` // è la cartella da cui il launcher RSI ha avviato il gioco l\'ultima volta
}

var exeDir string

func dataDir() string {
	d := filepath.Join(exeDir, "dati")
	os.MkdirAll(d, 0755)
	return d
}

func appendLogFile(line string) {
	f, err := os.OpenFile(filepath.Join(dataDir(), "log.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	f.WriteString(line + "\r\n")
	f.Close()
}

var launchRe = regexp.MustCompile(`Launching Star Citizen \S+ from \(([^)]+)\)`)

// launcherLastPath: cartella da cui il launcher RSI ha avviato il gioco l'ultima volta (dal suo registro).
func launcherLastPath() string {
	p := os.Getenv("SCBIND_LAUNCHER_LOG")
	if p == "" {
		base := os.Getenv("APPDATA")
		if base == "" {
			return ""
		}
		p = filepath.Join(base, "rsilauncher", "logs", "log.log")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	if len(b) > 4<<20 {
		b = b[len(b)-(4<<20):]
	}
	m := launchRe.FindAllSubmatch(b, -1)
	if len(m) == 0 {
		return ""
	}
	path := string(m[len(m)-1][1])
	path = strings.ReplaceAll(path, `\\`, `\`)
	return strings.TrimRight(path, `\/`)
}

func scRoots(custom string) []string {
	roots := []string{}
	if lp := launcherLastPath(); lp != "" {
		roots = append(roots, lp, filepath.Dir(lp))
	}
	if custom != "" {
		roots = append(roots, custom)
	}
	if r := os.Getenv("SCBIND_SC_ROOT"); r != "" {
		roots = append(roots, r)
	}
	if runtime.GOOS == "windows" {
		subs := []string{`Program Files\Roberts Space Industries\StarCitizen`, `Roberts Space Industries\StarCitizen`,
			`Games\Roberts Space Industries\StarCitizen`, `Giochi\Roberts Space Industries\StarCitizen`,
			`StarCitizen`, `Games\StarCitizen`, `Giochi\StarCitizen`, `Program Files\StarCitizen`, `RSI\StarCitizen`}
		for c := 'C'; c <= 'Z'; c++ {
			drive := string(c) + `:\`
			if _, err := os.Stat(drive); err != nil {
				continue
			}
			for _, s := range subs {
				roots = append(roots, drive+s)
			}
		}
	}
	return roots
}

// fileHash: impronta del CONTENUTO UTILE di un actionmaps.xml (non dei byte: il gioco lo riscrive a ogni uscita).
func fileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha1.Sum([]byte(signature(b)))
	return hex.EncodeToString(h[:])
}

func findInstalls(p *Profile) []Install {
	res := []Install{}
	seen := map[string]bool{}
	lastLaunch := launcherLastPath()
	add := func(dir string) {
		key := strings.ToLower(dir)
		if seen[key] {
			return
		}
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			return
		}
		hasUser := false
		if s, err := os.Stat(filepath.Join(dir, "user")); err == nil && s.IsDir() {
			hasUser = true
		}
		if _, err := os.Stat(filepath.Join(dir, "Data.p4k")); err != nil && !hasUser {
			return
		}
		seen[key] = true
		in := Install{Name: filepath.Base(dir), Path: dir}
		in.ActionMaps = filepath.Join(dir, "user", "client", "0", "Profiles", "default", "actionmaps.xml")
		in.Hash = fileHash(in.ActionMaps)
		in.Exists = in.Hash != ""
		in.Changed = in.Exists && p.LastWrite[key] != in.Hash
		if st, err := os.Stat(filepath.Join(dir, "Game.log")); err == nil {
			in.LastRun = st.ModTime().Unix()
		}
		in.Launcher = lastLaunch != "" && strings.EqualFold(filepath.Clean(dir), filepath.Clean(lastLaunch))
		res = append(res, in)
	}
	for _, root := range scRoots(p.CustomRoot) {
		// la cartella indicata può essere già un canale (LIVE) oppure la cartella StarCitizen
		add(root)
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				add(filepath.Join(root, e.Name()))
			}
		}
	}
	sort.SliceStable(res, func(a, b int) bool {
		if res[a].Launcher != res[b].Launcher {
			return res[a].Launcher
		}
		return res[a].LastRun > res[b].LastRun
	})
	return res
}

// gameLogProducts legge da Game.log le stringhe "Product" che il gioco ha registrato all'ultimo avvio.
func gameLogProducts(installPath string) map[string][]string {
	res := map[string][]string{}
	f, err := os.Open(filepath.Join(installPath, "Game.log"))
	if err != nil {
		return res
	}
	defer f.Close()
	byIdx := map[string]string{}
	order := []string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		i := strings.Index(line, "Connected joystick")
		if i < 0 {
			continue
		}
		rest := line[i+len("Connected joystick"):]
		c := strings.Index(rest, ":")
		if c < 0 {
			continue
		}
		idx := rest[:c]
		prod := rest[c+1:]
		if strings.HasPrefix(prod, " ") {
			prod = prod[1:]
		}
		prod = strings.TrimRight(prod, "\r\n")
		if productGUID(prod) == "" {
			continue
		}
		if _, ok := byIdx[idx]; !ok {
			order = append(order, idx)
		}
		byIdx[idx] = prod
	}
	for _, idx := range order {
		p := byIdx[idx]
		g := productGUID(p)
		res[g] = append(res[g], p)
	}
	return res
}

func backupFile(src, channel string) (string, error) {
	b, err := os.ReadFile(src)
	if err != nil {
		return "", nil // niente da salvare
	}
	dir := filepath.Join(dataDir(), "backup")
	os.MkdirAll(dir, 0755)
	dst := filepath.Join(dir, fmt.Sprintf("actionmaps_%s_%s.xml", channel, time.Now().Format("2006-01-02_15-04-05")))
	return dst, os.WriteFile(dst, b, 0644)
}
