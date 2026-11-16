package main

import "testing"

func TestInvert(t *testing.T) {
	in := "<flight_move_pitch invert=\"1\"/>\n   <flight_move_yaw exponent=\"1.3\"/>\n <flight_move_roll invert=\"1\" exponent=\"2\"><nonlinearity_curve><point in=\"0\" out=\"0\"/></nonlinearity_curve></flight_move_roll>"
	rest, inv := extractInvert(in)
	t.Log(rest, inv)
	if !inv["flight_move_pitch"] || !inv["flight_move_roll"] || len(inv) != 2 {
		t.Fatal("inv", inv)
	}
	inv["flight_move_yaw"] = true
	inv["flight_move_roll"] = false
	inv["mgv_move"] = false
	out := mergeInvert(rest, inv)
	t.Log(out)
	r2, inv2 := extractInvert(out)
	if !inv2["flight_move_yaw"] || inv2["flight_move_roll"] || !inv2["flight_move_pitch"] || inv2["mgv_move"] || len(inv2) != 4 {
		t.Fatal(inv2)
	}
	if r2 != rest {
		t.Fatalf("rest changed:\n%s\n%s", r2, rest)
	}
}
