package asm

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmsdef"
)

// realSourceFile returns what a real object's source correlation record
// says of its source file, for govax's object to say the same, or nil if
// it has none (no debugger records).
func realSourceFile(t *testing.T, m *obj.Module) *obj.SourceFile {
	t.Helper()

	var groups [][]obj.Command

	for _, r := range m.Records {
		if tir, ok := r.(*obj.TIR); ok && tir.Type == obj.RecDBG {
			groups = append(groups, tir.Commands)
		}
	}

	if len(groups) == 0 {
		return nil
	}

	recs, _, err := obj.DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	for _, r := range recs {
		if r.Type != obj.DSTSourceFile || len(r.Data) < 22 || r.Data[1] != 0x01 {
			continue
		}

		d := r.Data
		n := int(d[21])

		return &obj.SourceFile{
			Created:   vmsdef.GoTime(binary.LittleEndian.Uint64(d[6:])),
			EOFBlock:  binary.LittleEndian.Uint32(d[14:]),
			FirstFree: binary.LittleEndian.Uint16(d[18:]),
			Format:    d[20],
			Spec:      string(d[22 : 22+n]),
		}
	}

	return nil
}

// TestDebugRecords assembles the sources of the debugger-record probes
// (testdata/mar/dst, and the Phase 29 probe's in testdata/mar/list) as
// real MACRO did, and checks govax's objects match real MACRO's whole:
// the line-number tables' DBG records among the TIR records, and the
// symbol records.
func TestDebugRecords(t *testing.T) {
	mar := filepath.Join("..", "..", "testdata", "mar")
	debug := []string{"DEBUG", "TRACEBACK"}

	for _, tc := range []struct {
		name, source, object string
		enable, disable      []string
		// edit replaces text in the source first.
		edit [2]string
	}{
		// DSTSYM's .QUAD 1, 2 is an error real MACRO wrote an object
		// for, storing the 1; govax writes none, so it's assembled
		// with the 1 alone.
		{name: "dstsym", source: "dst/dstsym.mar", object: "dst/vax/dstsym.obj", enable: debug, edit: [2]string{".QUAD\t1, 2", ".QUAD\t1"}},
		{name: "dstln1", source: "dst/dstln1.mar", object: "dst/vax/dstln1.obj", enable: debug},
		{name: "dstvar", source: "dst/dstln1.mar", object: "dst/vax/dstvar.obj", enable: debug},
		{name: "dstln2", source: "dst/dstln2.mar", object: "dst/vax/dstln2.obj", enable: debug},
		{name: "dstln3", source: "dst/dstln3.mar", object: "dst/vax/dstln3.obj", enable: debug},
		{name: "dstdbg", source: "dst/dstdbg.mar", object: "dst/vax/dstdbg.obj"},
		{name: "dstdis", source: "dst/dstdis.mar", object: "dst/vax/dstdis.obj", enable: debug},
		{name: "trdebug", source: "list/trace.mar", object: "list/vax/trdebug.obj", enable: debug},
		{name: "trdbgall", source: "list/trace.mar", object: "list/vax/trdbgall.obj", enable: debug},
		{name: "trenadbg", source: "list/trace.mar", object: "list/vax/trenadbg.obj", enable: []string{"DEBUG"}},
		{name: "trdbgsym", source: "list/trace.mar", object: "list/vax/trdbgsym.obj", enable: []string{"DEBUG"}, disable: []string{"TRACEBACK"}},
		{name: "failmaid", source: "list/failmain.mar", object: "list/vax/failmaid.obj", enable: debug},
		{name: "failsubd", source: "list/failsub.mar", object: "list/vax/failsubd.obj", enable: debug},
		// Phase 41's debugger probe: DBGDIS has every addressing mode,
		// with MOVAB START+2,R0's word displacement.
		{name: "dbgdis", source: "../dbg/dbgdis.mar", object: "../dbg/vax/dbgdis.obj", enable: debug},
		{name: "dbgsub", source: "../dbg/dbgsub.mar", object: "../dbg/vax/dbgsub.obj", enable: debug},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(mar, tc.source))
			if err != nil {
				t.Fatal(err)
			}

			text := string(src)
			if tc.edit[0] != "" {
				text = strings.Replace(text, tc.edit[0], tc.edit[1], 1)
			}

			a := macroAssembler()
			if err := a.SetFunctions(tc.enable, tc.disable); err != nil {
				t.Fatal(err)
			}

			if _, err := a.Assemble(text); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			// The edited DSTSYM has no error, so its severity is
			// success.
			allow := func(want string) string {
				if tc.edit[0] == "" {
					return want
				}

				return strings.Replace(want, "severity ERROR", "severity SUCCESS", 1)
			}

			requireSameObjectAllowing(t, a, readObjectFile(t, filepath.Join(mar, tc.object)), allow)
		})
	}
}

// TestDebugRecordsForth assembles FORTH /DEBUG with govax's STARLET and
// compares the object with real MACRO's, which used VMS's. VMS's system
// macros define symbols of their own ($$.TAB, $$.TMP, ...), as govax's
// do theirs ($$RMSBLK, ...), so the symbol records of "$$" names can't
// match, and with them the DBG records that hold the symbol records.
// Everything else is compared whole: the line-number table, and its DBG
// records among the TIR records. The other symbol records are compared
// one by one.
func TestDebugRecordsForth(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mar", "forth.mar"))
	if err != nil {
		t.Fatal(err)
	}

	a := macroAssembler()
	a.SetMacroLibraries(govaxStarlet(t))

	if err := a.SetFunctions([]string{"DEBUG", "TRACEBACK"}, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Assemble(string(src)); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	real := readObjectFile(t, filepath.Join("..", "..", "testdata", "mar", "dst", "vax", "forth.obj"))
	got := objectLike(t, a, real)

	gotRest, gotSyms := splitSymbolRecords(t, got)
	wantRest, wantSyms := splitSymbolRecords(t, real)

	if g, w := dumpText(t, gotRest), dumpText(t, wantRest); g != w {
		t.Errorf("object without symbol records:\n%s\nwant:\n%s", g, w)
	}

	if g, w := strings.Join(gotSyms, "\n"), strings.Join(wantSyms, "\n"); g != w {
		t.Errorf("symbol records:\n%s\nwant:\n%s", g, w)
	}

	if len(wantSyms) < 1400 {
		t.Errorf("%d symbol records compared, want FORTH's 1,405", len(wantSyms))
	}
}

// splitSymbolRecords returns m without its DBG records of symbol records
// (those after its TBT record of routine begins, the second), and those
// symbol records described, but the ones of "$$" names.
func splitSymbolRecords(t *testing.T, m *obj.Module) (*obj.Module, []string) {
	t.Helper()

	rest := &obj.Module{}

	var groups [][]obj.Command

	tbt := 0

	for _, r := range m.Records {
		if tir, ok := r.(*obj.TIR); ok {
			if tir.Type == obj.RecTBT {
				tbt++
			}

			if tir.Type == obj.RecDBG && tbt >= 2 {
				groups = append(groups, tir.Commands)

				continue
			}
		}

		rest.Records = append(rest.Records, r)
	}

	recs, _, err := obj.DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	var syms []string

	for _, r := range recs {
		if !strings.HasPrefix(r.Name(), "$$") {
			syms = append(syms, obj.FormatDST(r))
		}
	}

	return rest, syms
}
