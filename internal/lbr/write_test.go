package lbr

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/obj"
)

// checkLayout opens a written library and checks what Open doesn't: every
// index block's parent, the free index block list, the data block chain,
// and each data block's record count, worked out afresh by reading every
// record in order.
func checkLayout(t *testing.T, data []byte) *Library {
	t.Helper()

	l, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}

	h := data[:blockSize]
	hipreal := le32(h, lhdHiPreAl)
	nextVBN := le32(h, lhdNextVBN)

	if len(data) != int(nextVBN-1)*blockSize {
		t.Errorf("the file has %d blocks; the next free VBN is %d", len(data)/blockSize, nextVBN)
	}

	// The index trees: each block's parent is the block that points to it.
	used := map[uint32]bool{}
	overhead := 0

	var walk func(vbn, parent uint32)

	walk = func(vbn, parent uint32) {
		used[vbn] = true
		b, _ := l.block(vbn)

		if got := le32(b, indexParent); got != parent {
			t.Errorf("index block %d's parent is %d, want %d", vbn, got, parent)
		}

		end := indexEntries + int(le16(b, 0))
		for pos := indexEntries; pos < end; pos += keyOverhead + int(b[pos+6]) {
			if le16(b, pos+4) == rfaIndex {
				overhead++

				walk(le32(b, pos), vbn)
			}
		}
	}

	for i := range l.Indexes {
		if root := le32(h, lhdIdxDesc+i*iddLength+4); root != 0 {
			walk(root, 0)
		}
	}

	for vbn := range used {
		if vbn < 2 || vbn > hipreal {
			t.Errorf("index block %d is outside the preallocated blocks", vbn)
		}
	}

	if got := le32(h, lhdIdxBlks); int(got) != len(used) {
		t.Errorf("LHD$L_IDXBLKS is %d; the trees have %d blocks", got, len(used))
	}

	if got := le32(h, lhdIdxOvh); int(got) != overhead {
		t.Errorf("LHD$L_IDXOVH is %d; the trees have %d upper-level entries", got, overhead)
	}

	// The free list holds the other preallocated blocks.
	free := 0
	for vbn := le32(h, lhdFreeIdx); vbn != 0; free++ {
		if used[vbn] || vbn < 2 || vbn > hipreal || free > int(hipreal) {
			t.Fatalf("free index block %d is in use, out of range, or in a loop", vbn)
		}

		vbn = le32(data, int(vbn-1)*blockSize)
	}

	if free != int(le32(h, lhdFreIdxBlk)) || free+len(used) != int(hipreal-1) {
		t.Errorf("%d free and %d used index blocks; LHD$L_FREIDXBLK is %d, LHD$L_HIPREAL %d", free, len(used), le32(h, lhdFreIdxBlk), hipreal)
	}

	// The data chain runs through the rest of the file, in order.
	for vbn := hipreal + 1; vbn < nextVBN; vbn++ {
		want := vbn + 1
		if want == nextVBN {
			want = 0
		}

		if got := le32(data, int(vbn-1)*blockSize+dataLink); got != want {
			t.Errorf("data block %d links to %d, want %d", vbn, got, want)
		}
	}

	// Each block's record count: the records with any part in it.
	recs := map[uint32]int{}
	at := RFA{VBN: hipreal + 1, Offset: dataData}
	nextRFA := RFA{VBN: le32(h, lhdNextRFA), Offset: le16(h, lhdNextRFA+4)}

	for nextVBN > hipreal+1 && rfaLess(at, nextRFA) {
		rec, after, err := l.readRecord(at)
		if err != nil {
			t.Fatalf("record at %v: %v", at, err)
		}

		last := after.VBN
		if after.Offset == dataData {
			last--
		}

		if at.Offset == blockSize-2 && len(rec) == 0 {
			last++ // the length word filled the block
		}

		for v := at.VBN; v <= last; v++ {
			recs[v]++
		}

		at = after
	}

	if nextVBN > hipreal+1 && at != nextRFA {
		t.Errorf("the records end at %v; LHD$B_NEXTRFA is %v", at, nextRFA)
	}

	for vbn := hipreal + 1; vbn < nextVBN; vbn++ {
		if got := int(data[(vbn-1)*blockSize]); got != recs[vbn] {
			t.Errorf("data block %d: DATA$B_RECS is %d; %d records are in it", vbn, got, recs[vbn])
		}
	}

	return l
}

func TestCreateEmpty(t *testing.T) {
	for _, tc := range []struct {
		typ      Type
		indexes  int
		flags    uint16
		keyLen   uint16
		prealloc int
	}{
		{TypeMacro, 1, 0x05, 32, 9},      // 128 entries, 13 to a block
		{TypeObject, 2, 0x1D, 32, 49},    // 640 entries
		{TypeHelp, 1, 0x35, 16, 5},       // 15-character keys: 23 to a block
		{TypeText, 1, 0x05, 40, 11},      // 39-character keys: 11 to a block
		{TypeShareable, 2, 0x1D, 40, 58}, // as IMAGELIB.OLB's
	} {
		t.Run(tc.typ.String(), func(t *testing.T) {
			b, err := Create(tc.typ)
			if err != nil {
				t.Fatal(err)
			}

			data := b.Bytes()
			l := checkLayout(t, data)

			if l.Type != tc.typ || l.Modules != 0 || len(l.Indexes) != tc.indexes || l.Librarian != LibrarianName || l.History != 20 {
				t.Errorf("header: %+v", l)
			}

			for _, x := range l.Indexes {
				if x.Flags != tc.flags || x.KeyLen != tc.keyLen || len(x.Keys) != 0 {
					t.Errorf("index: flags %#x, key length %d, %d keys", x.Flags, x.KeyLen, len(x.Keys))
				}
			}

			// A new library, as LBR$OPEN leaves it: the preallocated index
			// blocks all free, and no data blocks yet.
			h := data[:blockSize]
			n := uint32(tc.prealloc)

			if le32(h, lhdHiPreAl) != n+1 || le32(h, lhdFreIdxBlk) != n || le32(h, lhdFreeIdx) != 2 ||
				le32(h, lhdNextVBN) != n+2 || le32(h, lhdNextRFA) != n+2 || le16(h, lhdNextRFA+4) != 0 ||
				len(data) != int(n+1)*blockSize {
				t.Errorf("layout: hipreal %d, free %d from %d, next VBN %d, next RFA %d.%d, %d blocks",
					le32(h, lhdHiPreAl), le32(h, lhdFreIdxBlk), le32(h, lhdFreeIdx), le32(h, lhdNextVBN),
					le32(h, lhdNextRFA), le16(h, lhdNextRFA+4), len(data)/blockSize)
			}
		})
	}

	if _, err := Create(TypeNCS); err == nil {
		t.Error("an NCS library was created")
	}
}

// sameRecords compares records, an empty record matching a nil one.
func sameRecords(a, b [][]byte) bool {
	return slices.EqualFunc(a, b, bytes.Equal)
}

func lines(s string) []string { return strings.Split(strings.TrimPrefix(s, "\n"), "\n") }

func records(lines ...string) [][]byte {
	out := make([][]byte, len(lines))
	for i, s := range lines {
		out[i] = []byte(s)
	}

	return out
}

const macroSource = `
; Macros for the librarian's tests.

	.MACRO	store arg1,arg2	; store a value
	MOVL	ARG1,ARG2   ; the move

	.ENDM	STORE

	.TITLE	not in a macro
lab:	.macro OUTER A
	.MACRO INNER B
	.BYTE B
	.ENDM INNER
	.IRP X,<1,2>
	.WORD X
	.ENDM
	.REPT 2
	.ERROR ; A is bad;
	.ENDR
	.ASCII /a;b/
	.ENDM OUTER
`

func TestMacroModules(t *testing.T) {
	b, _ := Create(TypeMacro)

	for _, tc := range []struct {
		squeeze bool
		want    map[string][]string
	}{
		{true, map[string][]string{
			"STORE": {"\t.MACRO\tstore arg1,arg2\t; store a value", "\tMOVL\tARG1,ARG2", "", "\t.ENDM\tSTORE"},
			"OUTER": {"lab:\t.macro OUTER A", "\t.MACRO INNER B", "\t.BYTE B", "\t.ENDM INNER", "\t.IRP X,<1,2>",
				"\t.WORD X", "\t.ENDM", "\t.REPT 2", "\t.ERROR ; A is bad;", "\t.ENDR", "\t.ASCII /a", "\t.ENDM OUTER"},
		}},
		{false, map[string][]string{
			"STORE": {"\t.MACRO\tstore arg1,arg2\t; store a value", "\tMOVL\tARG1,ARG2   ; the move", "", "\t.ENDM\tSTORE"},
		}},
	} {
		got, warns, err := b.MacroModules(lines(macroSource), tc.squeeze)
		if err != nil || len(warns) != 0 {
			t.Fatalf("squeeze %v: %v, %v", tc.squeeze, err, warns)
		}

		if len(got) != 2 || got[0].Name != "STORE" || got[1].Name != "OUTER" {
			t.Fatalf("squeeze %v: modules %v", tc.squeeze, got)
		}

		for _, e := range got {
			if want, ok := tc.want[e.Name]; ok && !sameRecords(e.Records, records(want...)) {
				t.Errorf("squeeze %v: %s is\n%q\nwant\n%q", tc.squeeze, e.Name, e.Records, records(want...))
			}
		}
	}

	// Errors and warnings.
	_, warns, err := b.MacroModules(lines(".MACRO A\n.ENDM B\n.MACRO C\n.ENDR\n.ENDM"), true)
	if err != nil || len(warns) != 2 {
		t.Errorf("mismatches: %v, %v", err, warns)
	}

	if _, _, err := b.MacroModules(lines("; nothing\n\tNOP"), true); err != ErrNoMacro {
		t.Errorf("no macro: %v", err)
	}

	got, _, err := b.MacroModules(lines(".MACRO A\n.ENDM\n.MACRO B\n.MACRO C\n.ENDM C"), true)
	if len(got) != 1 || got[0].Name != "A" || err == nil || !strings.Contains(err.Error(), "B: no matching .ENDM") {
		t.Errorf("unfinished macro: %v, %v", got, err)
	}

	if _, _, err := b.MacroModules(lines(".MACRO "+strings.Repeat("X", 32)+"\n.ENDM"), true); err == nil {
		t.Error("a 32-character macro name was accepted")
	}

	// A case-sensitive library keeps the name's case.
	b.flags[0] |= IndexNoCaseEnt

	if got, _, _ := b.MacroModules(lines(".MACRO Mixed\n.ENDM Mixed"), true); len(got) != 1 || got[0].Name != "Mixed" {
		t.Errorf("case-sensitive: %v", got)
	}
}

func TestScanMacroLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		kw   int
		word string
	}{
		{"\t.MACRO\tFOO A,B", kwMacro, "FOO"},
		{".macro foo", kwMacro, "foo"},
		{"L1: L2:\t.ENDM X ; done", kwEndm, "X"},
		{".ENDM", kwEndm, ""},
		{".ENDM ; comment", kwEndm, ""},
		{".ENDM;X", kwEndm, "X"}, // as inputmac: the ";" ended the word, not the line
		{"; .MACRO X", -1, ""},
		{".IIF NE X, .ERROR", kwIIF, "NE"},
		{".REPT = 3", -1, ""}, // an assignment
		{"\tMOVL R0,R1", -1, "R0"},
		{"", -1, ""},
		{"   ", -1, ""},
		{"X:", -1, ""},
	} {
		kw, word := scanMacroLine(tc.line)
		if kw != tc.kw || (kw >= 0 && word != tc.word) {
			t.Errorf("%q: %d %q, want %d %q", tc.line, kw, word, tc.kw, tc.word)
		}
	}
}

// sampleEntries are macro modules with records of many lengths, so that
// records start, end, and split at every word offset of a block.
func sampleEntries(n int) []*Entry {
	var out []*Entry

	size := 0

	for i := range n {
		e := &Entry{Name: fmt.Sprintf("M%04d", (i*7919)%n), Inserted: uint64(1000 + i)}

		for j := range i % 7 {
			size = (size*31 + 17 + j) % 700
			e.Records = append(e.Records, bytes.Repeat([]byte{byte('A' + j)}, size))
		}

		out = append(out, e)
	}

	return out
}

func TestWriteMacroLibrary(t *testing.T) {
	b, _ := Create(TypeMacro)
	b.Created, b.Updated = 111, 222

	entries := sampleEntries(400)
	for _, e := range entries {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	data := b.Bytes()
	l := checkLayout(t, data)

	if l.Modules != 400 || l.Created != 111 || l.Updated != 222 {
		t.Fatalf("header: %+v", l)
	}

	for _, e := range entries {
		rfa, ok := l.Lookup(e.Name)
		if !ok {
			t.Fatalf("no %s", e.Name)
		}

		m, err := l.Module(rfa)
		if err != nil {
			t.Fatal(err)
		}

		if m.Header.RefCount != 1 || m.Header.Inserted != e.Inserted || len(m.Header.UserData) != 0 {
			t.Errorf("%s: header %+v", e.Name, m.Header)
		}

		if !sameRecords(m.Records, e.Records) {
			t.Errorf("%s: records differ", e.Name)
		}
	}

	// Written again from what it reads back, the library is the same.
	again, err := Edit(l)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(again.Bytes(), data) {
		t.Error("rewriting the library changed it")
	}
}

// TestDeepIndex has enough long keys for a three-level index, more index
// blocks than LIBRARY/CREATE preallocates.
func TestDeepIndex(t *testing.T) {
	b, _ := Create(TypeMacro)

	var names []string

	for i := range 6000 {
		name := fmt.Sprintf("%s_%05d", strings.Repeat("LONG_NAME", 3)[:25], (i*4099)%6000)
		names = append(names, name)

		if err := b.Insert(&Entry{Name: name}); err != nil {
			t.Fatal(err)
		}
	}

	l := checkLayout(t, b.Bytes())
	slices.Sort(names)

	var got []string
	for _, k := range l.Indexes[0].Keys {
		got = append(got, k.Name)
	}

	if !slices.Equal(got, names) {
		t.Fatal("the keys read back differ")
	}

	root, _ := l.block(le32(l.data, lhdIdxDesc+4))
	child, _ := l.block(le32(root, indexEntries))

	if le16(child, indexEntries+4) != rfaIndex {
		t.Error("the index has fewer than three levels")
	}
}

// objectFile builds an object file of two modules.
func objectFile(t *testing.T) [][]byte {
	t.Helper()

	var out [][]byte

	for _, name := range []string{"FIRST", "SECOND"} {
		b := &obj.Builder{Name: name, Version: name + "-1", Language: "test", Created: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)}
		code := b.AddPsect(obj.Psect{Align: 2, Flags: obj.PsectREL | obj.PsectEXE | obj.PsectRD, Alloc: 4, Name: "$CODE"})

		b.AddSymbol(obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF | obj.SymREL, Psect: code, Name: name + "_DATA"})
		b.AddSymbol(obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF | obj.SymWEAK, Name: name + "_WEAK"})
		b.AddSymbol(obj.Symbol{Type: obj.GSDSymbol, Name: "EXTERNAL"})

		if name == "FIRST" {
			b.AddSymbol(obj.Symbol{Type: obj.GSDEntry, Flags: obj.SymDEF | obj.SymREL, Psect: code, Mask: 4, Name: "First_Entry"})
			b.AddSymbol(obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF | obj.SymREL, Psect: code, Name: "FIRST_DATA"})
			b.SetLocation(code, 0)
			b.Store([]byte{4, 0, 4, 0})
		}

		m, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}

		recs, err := obj.Encode(m)
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, recs...)
	}

	return out
}

func TestWriteObjectLibrary(t *testing.T) {
	b, _ := Create(TypeObject)
	file := objectFile(t)

	entries, err := b.ObjectModules(file, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 || !slices.Equal(entries[0].Symbols, []string{"FIRST_DATA", "First_Entry"}) ||
		!slices.Equal(entries[1].Symbols, []string{"SECOND_DATA"}) {
		t.Fatalf("entries: %+v", entries)
	}

	for _, e := range entries {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	l := checkLayout(t, b.Bytes())

	var syms []string
	for _, k := range l.Indexes[1].Keys {
		syms = append(syms, k.Name)
	}

	if !slices.Equal(syms, []string{"FIRST_DATA", "First_Entry", "SECOND_DATA"}) {
		t.Errorf("index 2: %v", syms)
	}

	for i, name := range []string{"FIRST", "SECOND"} {
		rfa, _ := l.Lookup(name)

		m, err := l.Module(rfa)
		if err != nil {
			t.Fatal(err)
		}

		// FIRST has TIR records (MHD$M_OBJTIR); SECOND has none.
		wantStat := byte(2 - 2*i)
		if m.Header.RefCount != uint32(1+len(entries[i].Symbols)) || m.Header.UserData[0] != wantStat ||
			m.Header.ObjectIdent() != name+"-1" || len(m.Header.UserData) != 33 || m.Header.SelectiveSearch() {
			t.Errorf("%s: header %+v", name, m.Header)
		}

		om, err := obj.Decode(m.Records)
		if err != nil || om.Name() != name {
			t.Errorf("%s: module %v, %v", name, om, err)
		}
	}

	if got, _ := l.ModuleName(l.Indexes[1].Keys[2].RFA); got != "SECOND" {
		t.Errorf("SECOND_DATA is in %s", got)
	}

	sel, _ := b.ObjectModules(file, true)
	if sel[1].UserData[0]&mhdSelectiveSearch == 0 {
		t.Error("/SELECTIVE_SEARCH wasn't recorded")
	}

	if _, err := b.ObjectModules(file[:len(file)-1], false); err == nil {
		t.Error("an object file without its last EOM was accepted")
	}

	mac, _ := Create(TypeMacro)
	if _, err := mac.ObjectModules(file, false); err == nil {
		t.Error("object modules were read for a macro library")
	}
}

func TestInsertReplaceDelete(t *testing.T) {
	b, _ := Create(TypeObject)

	a := &Entry{Name: "A", Symbols: []string{"X", "Y"}}
	if err := b.Insert(a); err != nil {
		t.Fatal(err)
	}

	if err := b.Insert(&Entry{Name: "B", Symbols: []string{"Z"}}); err != nil {
		t.Fatal(err)
	}

	for _, e := range []*Entry{
		{Name: "A"},                                        // already there
		{Name: "C", Symbols: []string{"X"}},                // A defines X
		{Name: "C", Symbols: []string{"W", "W"}},           // twice
		{Name: ""},                                         // no name
		{Name: strings.Repeat("N", 32)},                    // too long
		{Name: "C", Records: [][]byte{make([]byte, 2049)}}, // too long a record
	} {
		if err := b.Insert(e); err == nil {
			t.Errorf("inserted %s %v", e.Name, e.Symbols)
		}
	}

	// Replacing A takes its symbols away, and its data goes last.
	if ok, err := b.Replace(&Entry{Name: "A", Symbols: []string{"Y", "V"}}); !ok || err != nil {
		t.Fatalf("replace: %v, %v", ok, err)
	}

	if m, _ := b.SymbolModule("X"); m != "" || !slices.Equal(b.order, []string{"B", "A"}) {
		t.Errorf("after replacing: X in %q, order %v", m, b.order)
	}

	// A failed replacement leaves the old module.
	if _, err := b.Replace(&Entry{Name: "A", Symbols: []string{"Z"}}); err == nil {
		t.Fatal("replaced A with a module defining B's symbol")
	}

	if m, _ := b.SymbolModule("V"); m != "A" || !slices.Equal(b.order, []string{"B", "A"}) {
		t.Errorf("after a failed replacement: V in %q, order %v", m, b.order)
	}

	if ok, err := b.Replace(&Entry{Name: "C"}); ok || err != nil {
		t.Errorf("replace inserting C: %v, %v", ok, err)
	}

	if err := b.Delete("B"); err != nil {
		t.Fatal(err)
	}

	if _, ok := b.SymbolModule("Z"); ok || !slices.Equal(b.Names(), []string{"A", "C"}) {
		t.Errorf("after deleting B: %v", b.Names())
	}

	if err := b.Delete("B"); err == nil {
		t.Error("deleted B twice")
	}

	mac, _ := Create(TypeMacro)
	if err := mac.Insert(&Entry{Name: "M", Symbols: []string{"S"}}); err == nil {
		t.Error("a macro library took a symbol")
	}

	l := checkLayout(t, b.Bytes())
	if n := len(l.Indexes[1].Keys); n != 2 {
		t.Errorf("%d symbols", n)
	}
}

// TestRewriteVMSLibraries rewrites real libraries through Edit: every
// module reads back unchanged, with the same header, keys, and symbols,
// and rewriting the result changes nothing.
func TestRewriteVMSLibraries(t *testing.T) {
	for _, file := range []string{"starlet.mlb", "starlet.olb", "imagelib.olb"} {
		t.Run(file, func(t *testing.T) {
			orig := vmsLibrary(t, file)

			b, err := Edit(orig)
			if err != nil {
				t.Fatal(err)
			}

			data := b.Bytes()
			l := checkLayout(t, data)

			if l.Type != orig.Type || l.Librarian != orig.Librarian || l.Created != orig.Created || l.Updated != orig.Updated ||
				l.Modules != orig.Modules || l.History != orig.History || l.dcx != nil || l.mhdUserSize != orig.mhdUserSize {
				t.Errorf("header: %+v", l)
			}

			for i, x := range orig.Indexes {
				y := l.Indexes[i]
				if x.Flags != y.Flags || x.KeyLen != y.KeyLen || len(x.Keys) != len(y.Keys) {
					t.Fatalf("index %d: flags %#x/%#x, key length %d/%d, %d/%d keys", i+1, x.Flags, y.Flags, x.KeyLen, y.KeyLen, len(x.Keys), len(y.Keys))
				}

				for j, k := range x.Keys {
					was, _ := orig.ModuleName(k.RFA)
					now, _ := l.ModuleName(y.Keys[j].RFA)

					if y.Keys[j].Name != k.Name || now != was {
						t.Fatalf("index %d key %d: %s in %s, was %s in %s", i+1, j, y.Keys[j].Name, now, k.Name, was)
					}
				}
			}

			for _, k := range orig.Indexes[0].Keys {
				want, _ := orig.Module(k.RFA)
				rfa, _ := l.Lookup(k.Name)

				got, err := l.Module(rfa)
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(got.Header, want.Header) || !sameRecords(got.Records, want.Records) {
					t.Fatalf("module %s differs", k.Name)
				}
			}

			again, err := Edit(l)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(again.Bytes(), data) {
				t.Error("rewriting the rewritten library changed it")
			}
		})
	}
}

func TestEditOrder(t *testing.T) {
	// Edit keeps the modules in the order of their data, not their names.
	b, _ := Create(TypeText)
	for _, n := range []string{"ZED", "ALPHA", "MIKE"} {
		_ = b.Insert(&Entry{Name: n, Records: records(n)})
	}

	l, _ := Open(b.Bytes())

	e, err := Edit(l)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(e.order, []string{"ZED", "ALPHA", "MIKE"}) {
		t.Errorf("order %v", e.order)
	}
}
