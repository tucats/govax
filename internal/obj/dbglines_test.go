package obj

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realLineTable reads a real object's line-number information: its DBG
// records that hold it (the ones before its first symbol record), the
// source file's record, and the table's commands, from the
// DSTLineNumbers records joined.
func realLineTable(t *testing.T, path string) (recs []*TIR, source DSTRecord, cmds []DSTItem) {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	raw, err := ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	var groups [][]Command

	for _, r := range m.Records {
		if tir, ok := r.(*TIR); ok && tir.Type == RecDBG {
			groups = append(groups, tir.Commands)
			recs = append(recs, tir)
		}
	}

	dsts, ends, err := DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	// The table's items, each address after the bytes before it.
	var stream []DSTItem

	last := 0

	for i, d := range dsts {
		switch d.Type {
		case DSTSourceFile:
			if i == 0 {
				source = d
			}

			continue
		case DSTLineNumbers:
		default:
			continue
		}

		last = ends[i]
		at := 0

		for _, a := range d.Addresses {
			stream = append(stream, DSTItem{Bytes: d.Data[at:a.Offset], Address: a.Commands})
			at = a.Offset + 4
		}

		stream = append(stream, DSTItem{Bytes: d.Data[at:]})
	}

	recs = recs[:last+1]

	// Split the stream into commands.
	var pending []byte

	for _, it := range stream {
		pending = append(pending, it.Bytes...)

		for len(pending) > 0 {
			op := pending[0]

			n := 1

			switch op {
			case 0x02, 0x0E, 0x13:
				n = 2
			case 0x01, 0x03, 0x0F:
				n = 3
			case 0x10:
				if it.Address == nil || len(pending) != 1 {
					t.Fatalf("%s: 10 without its address", path)
				}

				cmds = append(cmds, DSTItem{Bytes: []byte{0x10}, Address: it.Address})
				pending = nil

				continue
			}

			if len(pending) < n {
				t.Fatalf("%s: a command split across records", path)
			}

			cmds = append(cmds, DSTItem{Bytes: append([]byte(nil), pending[:n]...)})
			pending = pending[n:]
		}
	}

	return recs, source, cmds
}

func formatRecords(recs []*TIR) string {
	var b strings.Builder

	for i, r := range recs {
		fmt.Fprintf(&b, "record %d:\n", i+1)

		for _, c := range r.Commands {
			fmt.Fprintf(&b, "    %s\n", FormatCommand(c))
		}
	}

	return b.String()
}

// TestLineTableRealObjects packs the line-number tables of real MACRO's
// /DEBUG objects again, from their commands, and checks the DBG records
// match: FORTH's four (two filled, the rest, then the last part) and each
// smaller table's one.
func TestLineTableRealObjects(t *testing.T) {
	mar := filepath.Join("..", "..", "testdata", "mar")

	for _, path := range []string{
		"dst/vax/forth.obj", "dst/vax/dstln1.obj", "dst/vax/dstln2.obj", "dst/vax/dstln3.obj",
		"dst/vax/dstsym.obj", "list/vax/trdebug.obj", "list/vax/failmaid.obj",
	} {
		t.Run(path, func(t *testing.T) {
			want, source, cmds := realLineTable(t, filepath.Join(mar, path))

			// The last command is the last segment's end, and the
			// line count comes before it.
			end := cmds[len(cmds)-1]
			if end.Bytes[0] != 0x0E {
				t.Fatalf("last command % x isn't an end", end.Bytes)
			}

			countRec := realLineCount(t, filepath.Join(mar, path))

			lt := NewLineTable(source)

			var got []*TIR

			for _, c := range cmds[:len(cmds)-1] {
				got = append(got, lt.Add(c)...)
			}

			got = append(got, lt.Finish(countRec, end)...)

			if g, w := formatRecords(got), formatRecords(want); g != w {
				t.Errorf("records:\n%s\nwant:\n%s", g, w)
			}
		})
	}
}

// realLineCount returns the line count a real object's line-number
// information ends with.
func realLineCount(t *testing.T, path string) int {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	raw, err := ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	var groups [][]Command

	for _, r := range m.Records {
		if tir, ok := r.(*TIR); ok && tir.Type == RecDBG {
			groups = append(groups, tir.Commands)
		}
	}

	dsts, _, err := DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	for i, d := range dsts {
		if i > 0 && d.Type == DSTSourceFile {
			switch d.Data[0] {
			case 0x0A:
				return int(d.Data[1]) | int(d.Data[2])<<8
			case 0x0B:
				return int(d.Data[1])
			}
		}
	}

	t.Fatal("no line count")

	return 0
}
