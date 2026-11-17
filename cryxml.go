package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
)

// CryXmlB: il formato XML binario del motore di gioco. Lo riconverto in XML di testo.
// Struttura: "CryXmlB\0", lunghezza del file, poi quattro tabelle (nodi da 28 byte, attributi da 8,
// figli da 4, stringhe terminate da zero) con i relativi inizio e numero di elementi.

func isCryXML(b []byte) bool { return bytes.HasPrefix(b, []byte("CryXmlB\x00")) }

func cryXMLToText(b []byte) ([]byte, error) {
	if !isCryXML(b) || len(b) < 44 {
		return nil, errors.New("non è un file CryXmlB")
	}
	var bo binary.ByteOrder = binary.LittleEndian
	if int(bo.Uint32(b[8:])) != len(b) {
		bo = binary.BigEndian
		if int(bo.Uint32(b[8:])) != len(b) {
			return nil, errors.New("CryXmlB: lunghezza dichiarata diversa da quella del file")
		}
	}
	u := func(off int) int { return int(bo.Uint32(b[off:])) }
	nodeOff, nodeN, attrOff, attrN := u(12), u(16), u(20), u(24)
	strOff, strLen := u(36), u(40)
	if nodeN <= 0 || nodeN > 5_000_000 || nodeOff < 0 || nodeOff+nodeN*28 > len(b) || attrN < 0 || attrOff < 0 || attrOff+attrN*8 > len(b) || strOff < 0 || strOff > len(b) || strLen < 0 || strOff+strLen > len(b) {
		return nil, errors.New("CryXmlB: tabelle fuori dai limiti")
	}
	strs := b[strOff : strOff+strLen]
	str := func(o int) string {
		if o < 0 || o >= len(strs) {
			return ""
		}
		e := bytes.IndexByte(strs[o:], 0)
		if e < 0 {
			e = len(strs) - o
		}
		return string(strs[o : o+e])
	}
	type node struct {
		name, content string
		attrs         [][2]string
		kids          []int
	}
	nodes := make([]node, nodeN)
	ai := 0
	root := -1
	for i := 0; i < nodeN; i++ {
		p := nodeOff + i*28
		n := &nodes[i]
		n.name, n.content = str(u(p)), str(u(p+4))
		na := int(bo.Uint16(b[p+8:]))
		parent := int(int32(bo.Uint32(b[p+12:])))
		for k := 0; k < na && ai < attrN; k++ {
			q := attrOff + ai*8
			n.attrs = append(n.attrs, [2]string{str(u(q)), str(u(q + 4))})
			ai++
		}
		if parent >= 0 && parent < nodeN && parent != i {
			nodes[parent].kids = append(nodes[parent].kids, i)
		} else if root < 0 {
			root = i
		}
	}
	if root < 0 {
		return nil, errors.New("CryXmlB: nessun nodo radice")
	}
	var sb strings.Builder
	var emit func(i, depth int)
	emit = func(i, depth int) {
		n := &nodes[i]
		if n.name == "" || depth > 200 {
			return
		}
		ind := strings.Repeat(" ", depth)
		sb.WriteString(ind + "<" + n.name)
		for _, a := range n.attrs {
			if a[0] != "" {
				sb.WriteString(" " + a[0] + `="` + xmlEsc(a[1]) + `"`)
			}
		}
		if len(n.kids) == 0 && n.content == "" {
			sb.WriteString(" />\n")
			return
		}
		sb.WriteString(">")
		if n.content != "" {
			sb.WriteString(xmlEsc(n.content))
		}
		if len(n.kids) > 0 {
			sb.WriteString("\n")
			for _, k := range n.kids {
				emit(k, depth+1)
			}
			sb.WriteString(ind)
		}
		sb.WriteString("</" + n.name + ">\n")
	}
	emit(root, 0)
	return []byte(sb.String()), nil
}
