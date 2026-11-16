package main

import "testing"

func TestVerLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"4.1", "4.2", true}, {"4.1", "v4.10", true}, {"4.10", "4.2", false}, {"4.1", "4.1", false}, {"4.1", "5.0", true}, {"4.1", "4.1.1", true}, {"5.0", "4.9", false}} {
		if got := verLess(c.a, c.b); got != c.want {
			t.Errorf("verLess(%q,%q)=%v", c.a, c.b, got)
		}
	}
	if verParts("untagged-abc") != nil {
		t.Error("etichetta non numerica accettata")
	}
}
