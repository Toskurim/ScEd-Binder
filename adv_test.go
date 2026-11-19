package main

import (
	"strings"
	"testing"
)

// Curve, zone morte e saturazione devono seguire la periferica quando cambia il numero jsN.
func TestAdvancedFollowDevice(t *testing.T) {
	loadActions()
	R := "{41303344-0000-0000-0000-504944564944}"
	T := "{81943344-0000-0000-0000-504944564944}"
	disk := `<ActionMaps>
 <ActionProfiles version="1" optionsVersion="2" rebindVersion="2" profileName="default">
  <deviceoptions name="RIGHT VPC Stick MT-50CM2  ` + R + `">
   <option input="x" deadzone="0.02" saturation="0.9"/>
   <option input="y" deadzone="0.02"/>
  </deviceoptions>
  <options type="keyboard" instance="1" Product="Tastiera  {6F1D2B61-D5A0-11CF-BFC7-444553540000}"/>
  <options type="joystick" instance="1" Product="RIGHT VPC Stick MT-50CM2  ` + R + `">
   <flight_move_pitch invert="1" exponent="1.5"/>
   <flight_move_roll>
    <nonlinearity_curve>
     <point in="0.5" out="0.2"/>
    </nonlinearity_curve>
   </flight_move_roll>
  </options>
  <options type="joystick" instance="2" Product="LEFT VPC MongoosT-50CM3  ` + T + `"/>
  <modifiers />
 </ActionProfiles>
</ActionMaps>`
	devs := []Device{{Name: "LEFT VPC MongoosT-50CM3", GUID: T}, {Name: "RIGHT VPC Stick MT-50CM2", GUID: R}}
	assignKeys(devs)
	p, err := importActionMaps([]byte(disk), newProfile())
	if err != nil {
		t.Fatal(err)
	}
	p.ensureDevices(devs, nil)
	// 1) stesso ordine del gioco: tutto resta su js1
	x, _ := p.buildXML(devs)
	if !strings.Contains(x, `instance="1" Product="RIGHT VPC Stick`) || !strings.Contains(x, `<flight_move_pitch invert="1" exponent="1.5"/>`) || !strings.Contains(x, `<point in="0.5" out="0.2"/>`) || !strings.Contains(x, `saturation="0.9"`) {
		t.Fatalf("impostazioni perse:\n%s", x)
	}
	// 2) la numerazione cambia (manetta js1, stick js2): le curve devono spostarsi con lo stick
	p.Order = []string{T + "#1", R + "#1"}
	x, _ = p.buildXML(devs)
	i := strings.Index(x, `instance="2" Product="RIGHT VPC Stick`)
	jx := strings.Index(x, `exponent="1.5"`)
	k := strings.Index(x, `instance="1" Product="LEFT VPC MongoosT`)
	if i < 0 || k < 0 || jx < i || !strings.Contains(x, `<point in="0.5" out="0.2"/>`) || !strings.Contains(x, `deadzone="0.02" saturation="0.9"`) {
		t.Fatalf("le curve non hanno seguito la periferica:\n%s", x)
	}
	// 3) l'utente cambia una curva nel gioco: alla scrittura successiva l'app deve prendere quella nuova
	disk2 := strings.Replace(strings.Replace(disk, `exponent="1.5"`, `exponent="2.2"`, 1), `saturation="0.9"`, `saturation="0.7"`, 1)
	if !p.refreshFromGame([]byte(disk2)) {
		t.Fatal("modifica fatta nel gioco non rilevata")
	}
	x, _ = p.buildXML(devs)
	if !strings.Contains(x, `exponent="2.2"`) || strings.Contains(x, `exponent="1.5"`) || !strings.Contains(x, `saturation="0.7"`) {
		t.Fatalf("curva nuova non adottata:\n%s", x)
	}
	if s := p.advancedSummary()[R+"#1"]; !strings.Contains(s, "flight_move_roll") || !strings.Contains(s, "2 assi") {
		t.Errorf("riassunto: %q", s)
	}
}

func TestAdvancedDetail(t *testing.T) {
	p := &Profile{Devices: map[string]*DevInfo{"{40CB3344-0000-0000-0000-504944564944}#1": {GUID: "{40CB3344-0000-0000-0000-504944564944}", Options: `<flight_move_pitch exponent="1.5"/><flight_move_yaw><nonlinearity_curve><point in="0.1" out="0.05"/><point in="0.5" out="0.3"/><point in="1" out="1"/></nonlinearity_curve></flight_move_yaw>`}, "{00CB3344-0000-0000-0000-504944564944}#1": {GUID: "{00CB3344-0000-0000-0000-504944564944}"}},
		Extras: []string{`<deviceoptions name="Stick  {40CB3344-0000-0000-0000-504944564944}"><option input="x" saturation="0.74250001"/><option input="x" saturation="0.74250001"/><option input="z" saturation="0.9"/><option input="z" deadzone="0.19799998"/></deviceoptions>`}}
	d := p.advancedDetail()
	a := d["{40CB3344-0000-0000-0000-504944564944}#1"]
	if a.Axes["x"].Saturation != "0.74250001" || a.Axes["z"].Deadzone != "0.19799998" || a.Axes["z"].Saturation != "0.9" {
		t.Errorf("assi: %+v", a.Axes)
	}
	if a.Groups["flight_move_pitch"].Exponent != "1.5" || a.Groups["flight_move_yaw"].Points != 3 {
		t.Errorf("gruppi: %+v", a.Groups)
	}
	if _, ok := d["{00CB3344-0000-0000-0000-504944564944}#1"]; ok {
		t.Error("periferica senza impostazioni presente")
	}
}

func TestAdvEdit(t *testing.T) {
	sp := func(s string) *string { return &s }
	g := "{40CB3344-0000-0000-0000-504944564944}"
	p := &Profile{Devices: map[string]*DevInfo{g + "#1": {GUID: g, Product: "Stick  " + g, Options: `<flight_move_pitch exponent="1.5"/><flight_move_yaw><nonlinearity_curve><point in="0.5" out="0.3"/></nonlinearity_curve></flight_move_yaw><flight_move_roll invert="1"/>`,
		Adv: &AdvEdit{Axes: map[string]AdvAxisEdit{"x": {Deadzone: sp("0.05")}, "z": {Saturation: sp("")}}, Exp: map[string]*string{"flight_move_pitch": sp("2"), "flight_move_yaw": sp("3"), "flight_move_strafe_vertical": sp("1.3"), "flight_move_roll": sp("1")}}}},
		Extras: []string{`<deviceoptions name="Stick  ` + g + `"><option input="x" saturation="0.74250001"/><option input="x" saturation="0.74250001"/><option input="z" saturation="0.9"/><option input="z" deadzone="0.2"/></deviceoptions>`, `<altro/>`}}
	di := p.Devices[g+"#1"]
	o := effOptions(di)
	for _, w := range []string{`<flight_move_pitch exponent="2"/>`, `<nonlinearity_curve>`, `<flight_move_strafe_vertical exponent="1.3"/>`, `<flight_move_roll invert="1"/>`} {
		if !strings.Contains(o, w) {
			t.Errorf("opzioni: manca %s in %s", w, o)
		}
	}
	if strings.Contains(o, `exponent="3"`) {
		t.Error("la curva personalizzata è stata toccata")
	}
	ex := p.effExtras(nil)
	if len(ex) != 2 || ex[1] != `<altro/>` {
		t.Fatalf("extras: %v", ex)
	}
	if strings.Count(ex[0], `input="x" saturation="0.74250001"`) != 1 || !strings.Contains(ex[0], `<option input="x" deadzone="0.05"/>`) || strings.Contains(ex[0], `input="z" saturation`) || !strings.Contains(ex[0], `<option input="z" deadzone="0.2"/>`) {
		t.Errorf("deviceoptions: %s", ex[0])
	}
	d := p.advancedDetail()[g+"#1"]
	if d.Axes["x"].Deadzone != "0.05" || d.Axes["z"].Saturation != "" || d.Groups["flight_move_pitch"].Exponent != "2" {
		t.Errorf("dettaglio effettivo: %+v", d)
	}
	// il file del gioco ora contiene i valori scelti: le modifiche in sospeso spariscono
	p.Extras = []string{ex[0], `<altro/>`}
	di.Options = o
	if !p.dropWrittenAdv() || di.Adv != nil {
		t.Errorf("modifiche non tolte dopo la scrittura: %+v", di.Adv)
	}
	if p.hasAdvPending() {
		t.Error("risulta ancora da scrivere")
	}
	// periferica senza blocco: viene creato; valori standard: nessun blocco
	p2 := &Profile{Devices: map[string]*DevInfo{g + "#1": {GUID: g, Adv: &AdvEdit{Axes: map[string]AdvAxisEdit{"y": {Saturation: sp("0.8")}}}}}}
	if e := p2.effExtras(map[string]string{g + "#1": "Nuovo  " + g}); len(e) != 1 || !strings.Contains(e[0], `<deviceoptions name="Nuovo  `+g+`">`) || !strings.Contains(e[0], `<option input="y" saturation="0.8"/>`) {
		t.Errorf("blocco nuovo: %v", e)
	}
	p2.Devices[g+"#1"].Adv.Axes["y"] = AdvAxisEdit{Saturation: sp("1")}
	if e := p2.effExtras(map[string]string{g + "#1": "Nuovo  " + g}); len(e) != 0 {
		t.Errorf("valori standard ma blocco scritto: %v", e)
	}
}
