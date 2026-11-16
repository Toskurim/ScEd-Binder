package main

import (
	"os"
	"strings"
	"testing"
)

// Il file vero dell'utente, riletto e rigenerato senza modifiche, deve tornare identico (a parte i doppioni).
func TestEDRoundTrip(t *testing.T) {
	data, err := os.ReadFile("testdata/ed_custom.binds")
	if err != nil {
		t.Skip("file di prova assente")
	}
	doc, err := parseBinds(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.binds) != 422 {
		t.Errorf("comandi letti: %d", len(doc.binds))
	}
	out, _ := doc.generate(doc.binds, func(EDSlot) bool { return true })
	a := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	b := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	diff := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		if strings.TrimRight(a[i], " \t") != strings.TrimRight(b[i], " \t") {
			if diff < 6 {
				t.Logf("riga %d:\n  file: %q\n  app:  %q", i+1, a[i], b[i])
			}
			diff++
		}
	}
	if diff > 0 || len(a) != len(b) {
		t.Errorf("righe diverse: %d (file %d righe, app %d)", diff, len(a), len(b))
	}
	// una modifica: beccheggio sull'asse Y dello stick destro, invertito
	doc.binds["PitchAxisRaw"] = &EDBind{A: &EDSlot{D: "33444130", K: "Joy_YAxis"}, Inv: false}
	out2, _ := doc.generate(doc.binds, func(EDSlot) bool { return true })
	d2, err := parseBinds([]byte(out2))
	if err != nil || d2.binds["PitchAxisRaw"].Inv || d2.binds["PitchAxisRaw"].A.K != "Joy_YAxis" || d2.binds["DeployHardpointToggle"].S.M[0].K != "Joy_31" {
		t.Error("modifica non riletta correttamente")
	}
}
