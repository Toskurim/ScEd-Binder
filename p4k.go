package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	zstd "scbind/izstd"
)

// Lettura dell'archivio Data.p4k di Star Citizen: è uno Zip (zip64) con in più la compressione
// Zstandard (metodo 100) e, per alcuni file, una cifratura AES-128-CBC con una chiave pubblica nota,
// la stessa usata dagli strumenti open source della comunità (unp4k, scdatatools).
// Leggo solo l'indice e i pochi file che servono: l'archivio (oltre 100 GB) non viene mai caricato per intero.

var p4kKey = []byte{0x5E, 0x7A, 0x20, 0x02, 0x30, 0x2E, 0xEB, 0x1A, 0x3B, 0xB6, 0x17, 0xC3, 0x0F, 0xDE, 0x1E, 0x47}

type p4kEntry struct {
	Name         string
	Method       uint16
	CSize, USize uint64
	Offset       uint64
	Encrypted    bool
}

func p4kNorm(n string) string { return strings.ToLower(strings.ReplaceAll(n, "\\", "/")) }

// p4kFind cerca nell'indice i file i cui nomi (minuscoli, con "/") sono in want. Restituisce anche quanti file ha l'archivio.
func p4kFind(f *os.File, want []string) (map[string]p4kEntry, int, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	size := st.Size()
	tail := int64(66000)
	if tail > size {
		tail = size
	}
	buf := make([]byte, tail)
	if _, err := f.ReadAt(buf, size-tail); err != nil && err != io.EOF {
		return nil, 0, err
	}
	i := bytes.LastIndex(buf, []byte("PK\x05\x06"))
	if i < 0 || i+22 > len(buf) {
		return nil, 0, errors.New("fine dell'archivio non trovata (non sembra un file p4k valido)")
	}
	cdSize := uint64(binary.LittleEndian.Uint32(buf[i+12:]))
	cdOff := uint64(binary.LittleEndian.Uint32(buf[i+16:]))
	if i >= 20 && bytes.Equal(buf[i-20:i-16], []byte("PK\x06\x07")) { // zip64
		z64 := int64(binary.LittleEndian.Uint64(buf[i-12:]))
		h := make([]byte, 56)
		if _, err := f.ReadAt(h, z64); err != nil || !bytes.Equal(h[:4], []byte("PK\x06\x06")) {
			return nil, 0, errors.New("indice zip64 non leggibile")
		}
		cdSize = binary.LittleEndian.Uint64(h[40:])
		cdOff = binary.LittleEndian.Uint64(h[48:])
	}
	if cdOff+cdSize > uint64(size) || cdSize == 0 {
		return nil, 0, errors.New("indice dell'archivio fuori dai limiti del file")
	}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	res := map[string]p4kEntry{}
	r := bufio.NewReaderSize(io.NewSectionReader(f, int64(cdOff), int64(cdSize)), 4<<20)
	hdr := make([]byte, 46)
	n := 0
	for {
		if _, err := io.ReadFull(r, hdr); err != nil {
			break
		}
		if !bytes.Equal(hdr[:4], []byte("PK\x01\x02")) {
			break
		}
		n++
		fn, ex, cm := int(binary.LittleEndian.Uint16(hdr[28:])), int(binary.LittleEndian.Uint16(hdr[30:])), int(binary.LittleEndian.Uint16(hdr[32:]))
		name := make([]byte, fn)
		if _, err := io.ReadFull(r, name); err != nil {
			return res, n, err
		}
		key := p4kNorm(string(name))
		if !wanted[key] {
			if _, err := r.Discard(ex + cm); err != nil {
				return res, n, err
			}
			continue
		}
		extra := make([]byte, ex)
		if _, err := io.ReadFull(r, extra); err != nil {
			return res, n, err
		}
		r.Discard(cm)
		e := p4kEntry{Name: string(name), Method: binary.LittleEndian.Uint16(hdr[10:]),
			CSize: uint64(binary.LittleEndian.Uint32(hdr[20:])), USize: uint64(binary.LittleEndian.Uint32(hdr[24:])),
			Offset: uint64(binary.LittleEndian.Uint32(hdr[42:]))}
		e.Encrypted = len(extra) > 168 && extra[168] > 0
		for p := 0; p+4 <= len(extra); {
			tp, ln := binary.LittleEndian.Uint16(extra[p:]), int(binary.LittleEndian.Uint16(extra[p+2:]))
			if p+4+ln > len(extra) {
				break
			}
			if tp == 1 { // zip64: i valori compaiono solo per i campi segnati 0xFFFFFFFF, in quest'ordine
				v := extra[p+4 : p+4+ln]
				take := func(dst *uint64) {
					if *dst == 0xFFFFFFFF && len(v) >= 8 {
						*dst = binary.LittleEndian.Uint64(v)
						v = v[8:]
					}
				}
				take(&e.USize)
				take(&e.CSize)
				take(&e.Offset)
			}
			p += 4 + ln
		}
		res[key] = e
		if len(res) == len(wanted) {
			// trovati tutti: inutile leggere il resto dell'indice (ma il conteggio è parziale)
			return res, n, nil
		}
	}
	return res, n, nil
}

// p4kRead estrae un file dell'archivio (al massimo 256 MB).
func p4kRead(f *os.File, e p4kEntry) ([]byte, error) {
	const max = 256 << 20
	if e.CSize > max || e.USize > max {
		return nil, fmt.Errorf("%s è troppo grande (%d byte)", e.Name, e.USize)
	}
	h := make([]byte, 30)
	if _, err := f.ReadAt(h, int64(e.Offset)); err != nil {
		return nil, err
	}
	if !bytes.Equal(h[:4], []byte("PK\x03\x14")) && !bytes.Equal(h[:4], []byte("PK\x03\x04")) {
		return nil, fmt.Errorf("intestazione di %s non riconosciuta", e.Name)
	}
	start := int64(e.Offset) + 30 + int64(binary.LittleEndian.Uint16(h[26:])) + int64(binary.LittleEndian.Uint16(h[28:]))
	data := make([]byte, e.CSize)
	if _, err := f.ReadAt(data, start); err != nil {
		return nil, err
	}
	if e.Encrypted {
		blk, err := aes.NewCipher(p4kKey)
		if err != nil {
			return nil, err
		}
		n := len(data) / 16 * 16
		cipher.NewCBCDecrypter(blk, make([]byte, 16)).CryptBlocks(data[:n], data[:n])
	}
	var rd io.Reader
	switch e.Method {
	case 0:
		if uint64(len(data)) < e.USize {
			return nil, fmt.Errorf("%s: dati più corti del previsto", e.Name)
		}
		return data[:e.USize], nil
	case 8:
		rd = flate.NewReader(bytes.NewReader(data))
	case 100:
		rd = zstd.NewReader(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("%s: compressione %d non gestita", e.Name, e.Method)
	}
	out := make([]byte, e.USize)
	if _, err := io.ReadFull(rd, out); err != nil {
		return nil, fmt.Errorf("%s: decompressione non riuscita (%v)", e.Name, err)
	}
	return out, nil
}
