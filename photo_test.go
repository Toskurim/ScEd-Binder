package main

import "testing"

func TestGuess(t *testing.T) {
	loadPhotoDevices()
	cases := map[string][2]string{
		"LEFT VPC Stick MT-50CM2":  {"{812F3344-0000-0000-0000-504944564944}", ""},
		" VKB-Sim T-Rudder ":       {"{011F231D-0000-0000-0000-504944564944}", "foto:vkb-t-rudder"},
		"Joystick - HOTAS Warthog": {"{0402044F-0000-0000-0000-504944564944}", "foto:tm-warthog-stick"},
		"VKBsim Gladiator EVO  R":  {"{0200231D-0000-0000-0000-504944564944}", "foto:vkb-gladiator-scg"},
		"T.16000M":                 {"{B10A044F-0000-0000-0000-504944564944}", "foto:tm-t16000m"},
		"LEFT VPC MongoosT-50CM3":  {"{81943344-0000-0000-0000-504944564944}", ""},
		"FANATEC Wheel":            {"{00200EB7-0000-0000-0000-504944564944}", ""},
	}
	for name, c := range cases {
		if g := guessPhoto(name, c[0]); g != c[1] {
			t.Errorf("%q -> %q (atteso %q)", name, g, c[1])
		}
	}
}
