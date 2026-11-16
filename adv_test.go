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
