package main

import (
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// costruisce un CryXmlB minimo da un albero
type tnode struct {
	name  string
	attrs [][2]string
	kids  []*tnode
}

func mkCryXML(root *tnode) []byte {
	var strs bytes.Buffer
	off := map[string]uint32{}
	s := func(v string) uint32 {
		if o, ok := off[v]; ok {
			return o
		}
		o := uint32(strs.Len())
		off[v] = o
		strs.WriteString(v)
		strs.WriteByte(0)
		return o
	}
	s("")
	var nodes, attrs, kids bytes.Buffer
	le := binary.LittleEndian
	type item struct {
		n      *tnode
		parent int32
	}
	q := []item{{root, -1}}
	nAttr := uint32(0)
	for i := 0; i < len(q); i++ {
		it := q[i]
		binary.Write(&nodes, le, s(it.n.name))
		binary.Write(&nodes, le, s(""))
		binary.Write(&nodes, le, uint16(len(it.n.attrs)))
		binary.Write(&nodes, le, uint16(len(it.n.kids)))
		binary.Write(&nodes, le, it.parent)
		binary.Write(&nodes, le, nAttr)
		binary.Write(&nodes, le, uint32(0))
		binary.Write(&nodes, le, uint32(0))
		for _, a := range it.n.attrs {
			binary.Write(&attrs, le, s(a[0]))
			binary.Write(&attrs, le, s(a[1]))
			nAttr++
		}
		for _, k := range it.n.kids {
			q = append(q, item{k, int32(i)})
		}
	}
	// NB: i nodi sono in ordine di visita in ampiezza; il lettore li riattacca per ParentNodeID
	head := 44
	nodeOff := head
	attrOff := nodeOff + nodes.Len()
	kidOff := attrOff + attrs.Len()
	strOff := kidOff + kids.Len()
	total := strOff + strs.Len()
	var out bytes.Buffer
	out.WriteString("CryXmlB\x00")
	for _, v := range []int{total, nodeOff, len(q), attrOff, int(nAttr), kidOff, 0, strOff, strs.Len()} {
		binary.Write(&out, le, uint32(v))
	}
	out.Write(nodes.Bytes())
	out.Write(attrs.Bytes())
	out.Write(kids.Bytes())
	out.Write(strs.Bytes())
	return out.Bytes()
}

func TestCryXML(t *testing.T) {
	tree := &tnode{name: "profile", attrs: [][2]string{{"version", "1"}}, kids: []*tnode{
		{name: "actionmap", attrs: [][2]string{{"name", "spaceship_movement"}, {"UILabel", "@ui_CGSpaceFlightMovement"}}, kids: []*tnode{
			{name: "action", attrs: [][2]string{{"name", "v_pitch"}, {"joystick", "y"}, {"UILabel", "@ui_CIPitch"}}},
			{name: "action", attrs: [][2]string{{"name", "v_a<b"}, {"keyboard", "a&b"}}},
		}},
		{name: "actionmap", attrs: [][2]string{{"name", "seat_general"}}},
	}}
	txt, err := cryXMLToText(mkCryXML(tree))
	if err != nil {
		t.Fatal(err)
	}
	s := string(txt)
	for _, want := range []string{`<profile version="1">`, `<actionmap name="spaceship_movement" UILabel="@ui_CGSpaceFlightMovement">`, `<action name="v_pitch" joystick="y" UILabel="@ui_CIPitch" />`, `name="v_a&lt;b" keyboard="a&amp;b"`, `<actionmap name="seat_general" />`, `</profile>`} {
		if !strings.Contains(s, want) {
			t.Errorf("manca %q in:\n%s", want, s)
		}
	}
	if strings.Index(s, "v_pitch") > strings.Index(s, "seat_general") {
		t.Error("ordine dei nodi sbagliato")
	}
	if _, err := cryXMLToText([]byte("CryXmlB\x00garbage-garbage-garbage-garbage-garbage-xx")); err == nil {
		t.Error("file rovinato accettato")
	}
}

// frame zstd con un solo blocco "raw" (niente compressione vera: basta per provare il percorso)
func rawZstd(b []byte) []byte {
	out := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x20, byte(len(b))}
	h := uint32(len(b))<<3 | 1
	out = append(out, byte(h), byte(h>>8), byte(h>>16))
	return append(out, b...)
}

func TestP4K(t *testing.T) {
	le := binary.LittleEndian
	big := bytes.Repeat([]byte("riga di prova per il file grande\r\n"), 5000)
	var defl bytes.Buffer
	fw, _ := flate.NewWriter(&defl, 6)
	fw.Write(big)
	fw.Close()
	enc := func(b []byte) []byte {
		p := append([]byte{}, b...)
		for len(p)%16 != 0 {
			p = append(p, 0)
		}
		blk, _ := aes.NewCipher(p4kKey)
		cipher.NewCBCEncrypter(blk, make([]byte, 16)).CryptBlocks(p, p)
		return p
	}
	small := []byte("contenuto piccolo compresso zstd e cifrato")
	files := []struct {
		name   string
		method uint16
		data   []byte
		usize  int
		enc    bool
		want   []byte
	}{
		{`Data\Altro\inutile.bin`, 0, []byte("xxxx"), 4, false, nil},
		{`Data\Libs\Config\defaultProfile.xml`, 100, enc(rawZstd(small)), len(small), true, small},
		{`Data\Localization\english\global.ini`, 8, enc(defl.Bytes()), len(big), true, big},
		{`Data\Scripts\chiaro.txt`, 0, []byte("in chiaro"), 9, false, []byte("in chiaro")},
		{`Data\Scripts\zstd_chiaro.txt`, 100, rawZstd(small), len(small), false, small},
	}
	var arc, cd bytes.Buffer
	arc.WriteString("preambolo che sposta gli offset")
	for _, f := range files {
		off := arc.Len()
		arc.WriteString("PK\x03\x14")
		arc.Write(make([]byte, 22))
		binary.Write(&arc, le, uint16(len(f.name)))
		binary.Write(&arc, le, uint16(5))
		arc.WriteString(f.name)
		arc.Write(make([]byte, 5))
		arc.Write(f.data)
		extra := new(bytes.Buffer)
		binary.Write(extra, le, uint16(1))
		binary.Write(extra, le, uint16(24))
		binary.Write(extra, le, uint64(f.usize))
		binary.Write(extra, le, uint64(len(f.data)))
		binary.Write(extra, le, uint64(off))
		binary.Write(extra, le, uint16(0x5000))
		binary.Write(extra, le, uint16(200))
		pay := make([]byte, 200)
		if f.enc {
			pay[168-32] = 1
		}
		extra.Write(pay)
		cd.WriteString("PK\x01\x02")
		cd.Write(make([]byte, 6))
		binary.Write(&cd, le, f.method)
		cd.Write(make([]byte, 8))
		binary.Write(&cd, le, uint32(0xFFFFFFFF))
		binary.Write(&cd, le, uint32(0xFFFFFFFF))
		binary.Write(&cd, le, uint16(len(f.name)))
		binary.Write(&cd, le, uint16(extra.Len()))
		cd.Write(make([]byte, 10))
		binary.Write(&cd, le, uint32(0xFFFFFFFF))
		cd.WriteString(f.name)
		cd.Write(extra.Bytes())
	}
	cdOff := arc.Len()
	arc.Write(cd.Bytes())
	z64 := arc.Len()
	arc.WriteString("PK\x06\x06")
	binary.Write(&arc, le, uint64(44))
	arc.Write(make([]byte, 12))
	binary.Write(&arc, le, uint64(len(files)))
	binary.Write(&arc, le, uint64(len(files)))
	binary.Write(&arc, le, uint64(cd.Len()))
	binary.Write(&arc, le, uint64(cdOff))
	arc.WriteString("PK\x06\x07")
	binary.Write(&arc, le, uint32(0))
	binary.Write(&arc, le, uint64(z64))
	binary.Write(&arc, le, uint32(1))
	arc.WriteString("PK\x05\x06")
	arc.Write(make([]byte, 4))
	binary.Write(&arc, le, uint16(0xFFFF))
	binary.Write(&arc, le, uint16(0xFFFF))
	binary.Write(&arc, le, uint32(0xFFFFFFFF))
	binary.Write(&arc, le, uint32(0xFFFFFFFF))
	binary.Write(&arc, le, uint16(0))
	p := filepath.Join(t.TempDir(), "Data.p4k")
	os.WriteFile(p, arc.Bytes(), 0o644)
	f, _ := os.Open(p)
	defer f.Close()
	want := []string{}
	for _, x := range files[1:] {
		want = append(want, p4kNorm(x.name))
	}
	got, n, err := p4kFind(f, want)
	if err != nil || len(got) != 4 || n != 5 {
		t.Fatalf("p4kFind: %v, trovati %d, contati %d", err, len(got), n)
	}
	for _, x := range files[1:] {
		b, err := p4kRead(f, got[p4kNorm(x.name)])
		if err != nil {
			t.Errorf("%s: %v", x.name, err)
		} else if !bytes.Equal(b, x.want) {
			t.Errorf("%s: contenuto diverso (%d byte invece di %d)", x.name, len(b), len(x.want))
		}
	}
	if _, _, err := p4kFind(f, []string{"data/non/esiste.xml"}); err != nil {
		t.Errorf("ricerca di un file assente: %v", err)
	}
}
