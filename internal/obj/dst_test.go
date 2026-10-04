package obj

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// realObjectPaths returns every real VAX MACRO object in testdata: the
// Phase 27 fixtures', the Phase 28 macro fixtures', and the Phase 29
// probe's.
func realObjectPaths(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, dir := range []string{"mar/vax", "mar/macros/vax", "mar/list/vax"} {
		paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", filepath.FromSlash(dir), "*.obj"))
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, paths...)
	}

	return out
}

// readObject reads and decodes the object module at path.
func readObject(t *testing.T, path string) *Module {
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

	return m
}

// commandGroups returns the commands of m's records of type typ, a group
// for each record.
func commandGroups(m *Module, typ RecordType) [][]Command {
	var out [][]Command

	for _, r := range m.Records {
		if tir, ok := r.(*TIR); ok && tir.Type == typ {
			out = append(out, tir.Commands)
		}
	}

	return out
}

// mergedImmediates returns groups' commands as one list, adjacent STORE
// IMMEDIATE commands joined: the same bytes, whichever record (or
// command) each started in.
func mergedImmediates(groups [][]Command) []Command {
	var out []Command

	for _, cmds := range groups {
		for _, c := range cmds {
			if n := len(out); c.Op == OpStoreImmediate && n > 0 && out[n-1].Op == OpStoreImmediate {
				out[n-1].Data = append(append([]byte(nil), out[n-1].Data...), c.Data...)

				continue
			}

			out = append(out, c)
		}
	}

	return out
}

// TestDSTRealObjects decodes the DST records in every real object's TBT
// and DBG records and encodes them back: the same bytes and the same
// address commands. Every traceback record is one of the four kinds
// DSTType names, and the constructors build each one exactly.
func TestDSTRealObjects(t *testing.T) {
	paths := realObjectPaths(t)
	traced := 0

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			m := readObject(t, path)

			for _, typ := range []RecordType{RecTBT, RecDBG} {
				groups := commandGroups(m, typ)
				if len(groups) == 0 {
					continue
				}

				recs, ends, err := DecodeDST(groups)
				if err != nil {
					t.Fatalf("%s: %v", typ, err)
				}

				if len(ends) != len(recs) || (len(ends) > 0 && ends[len(ends)-1] != len(groups)-1) {
					t.Errorf("%s: record ends %v for %d records", typ, ends, len(groups))
				}

				cmds, err := EncodeDST(recs)
				if err != nil {
					t.Fatalf("%s: encode: %v", typ, err)
				}

				if want := mergedImmediates(groups); !reflect.DeepEqual(mergedImmediates([][]Command{cmds}), want) {
					t.Errorf("%s: re-encodes as\n%v\nnot\n%v", typ, cmds, want)
				}

				if typ == RecTBT {
					traced++

					checkTracebackRecords(t, recs)
				}
			}
		})
	}

	// The 21 fixtures, and the probe's objects with traceback.
	if traced < 21 {
		t.Errorf("%d objects with traceback records, want at least 21", traced)
	}
}

// checkTracebackRecords checks that recs, a real object's traceback
// records, are each a kind DSTType names, built exactly by its
// constructor.
func checkTracebackRecords(t *testing.T, recs []DSTRecord) {
	t.Helper()

	for i, r := range recs {
		var want DSTRecord

		switch r.Type {
		case DSTModuleBegin:
			want = DSTModuleBeginRecord(r.Name())
		case DSTModuleEnd:
			want = DSTModuleEndRecord()
		case DSTRoutineBegin, DSTPsect:
			if len(r.Addresses) != 1 || len(r.Addresses[0].Commands) != 2 {
				t.Errorf("record %d (%s): addresses %v", i+1, r.Type, r.Addresses)

				continue
			}

			stack := r.Addresses[0].Commands[0]

			if r.Type == DSTRoutineBegin {
				want = DSTRoutineBeginRecord(r.Name(), stack.Psect, stack.Value)
			} else {
				n := len(r.Data)
				size := uint32(r.Data[n-4]) | uint32(r.Data[n-3])<<8 | uint32(r.Data[n-2])<<16 | uint32(r.Data[n-1])<<24
				want = DSTPsectRecord(r.Name(), stack.Psect, size)
			}
		default:
			t.Errorf("record %d: traceback record of %s", i+1, r.Type)

			continue
		}

		if !reflect.DeepEqual(r, want) {
			t.Errorf("record %d (%s):\n%+v\nbuilt as\n%+v", i+1, r.Type, r, want)
		}
	}
}

// TestDSTSpansRecords checks a DST record that starts in one TBT record
// and ends in the next, and a STORE IMMEDIATE longer than MaxImmediate.
func TestDSTSpansRecords(t *testing.T) {
	long := strings.Repeat("N", 120)
	recs := []DSTRecord{DSTModuleBeginRecord(long), DSTRoutineBeginRecord("GO", 1, 8), DSTModuleEndRecord()}

	cmds, err := EncodeDST(recs)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range cmds {
		if c.Op == OpStoreImmediate && len(c.Data) > MaxImmediate {
			t.Errorf("STORE IMMEDIATE of %d bytes", len(c.Data))
		}
	}

	// Split the first command (the module begin record, and the start of
	// the routine's) between two records.
	first := cmds[0]
	groups := [][]Command{
		{{Op: OpStoreImmediate, Data: first.Data[:50]}},
		append([]Command{{Op: OpStoreImmediate, Data: first.Data[50:]}}, cmds[1:]...),
	}

	got, ends, err := DecodeDST(groups)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, recs) {
		t.Errorf("decoded\n%+v\nwant\n%+v", got, recs)
	}

	if !reflect.DeepEqual(ends, []int{1, 1, 1}) {
		t.Errorf("ends %v, want [1 1 1]", ends)
	}

	if got := FormatDST(recs[1]); got != `DST routine begin "GO"; at 1: STA_PL psect 1 offset 0x8, STO_PIDR` {
		t.Errorf("FormatDST = %q", got)
	}
}

// TestDSTErrors checks streams that aren't DST records.
func TestDSTErrors(t *testing.T) {
	stack := Command{Op: OpStackPsectLong, Psect: 1}
	store := Command{Op: OpStorePIDataRef}

	for _, tc := range []struct {
		name string
		cmds []Command
	}{
		{"too short", []Command{{Op: OpStoreImmediate, Data: []byte{5, 0xBC, 0}}}},
		{"zero length", []Command{{Op: OpStoreImmediate, Data: []byte{0}}}},
		{"never stored", []Command{{Op: OpStoreImmediate, Data: []byte{1, 0xBD}}, stack}},
		{"stored length", []Command{stack, store}},
		{"past the end", []Command{{Op: OpStoreImmediate, Data: []byte{3, 0xBE, 0}}, stack, store}},
		{"not a store", []Command{{Op: OpStoreImmediate, Data: []byte{6, 0xBE}}, stack, {Op: opsByName["STO_RB"]}}},
	} {
		if _, _, err := DecodeDST([][]Command{tc.cmds}); err == nil {
			t.Errorf("%s: no error", tc.name)
		}
	}
}

// TestDumpDST checks that Dump shows the DST records after the TBT
// record each ends in (the probe's trace.obj, whose routines FIRST and
// SECOND are in psect 2), and a stream that isn't DST records.
func TestDumpDST(t *testing.T) {
	var buf bytes.Buffer
	if err := Dump(&buf, readObject(t, filepath.Join("..", "..", "testdata", "mar", "list", "vax", "trace.obj"))); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`DST module begin "TRACE"`,
		`DST routine begin "FIRST"; at 1: STA_PL psect 2 offset 0x1e, STO_PIDR`,
		`DST routine begin "SECOND"; at 1: STA_PL psect 2 offset 0x2d, STO_PIDR`,
		`DST psect "$CODE", 67 bytes; at 1: STA_PB psect 2 offset 0x0, STO_PIDR`,
		`DST module end`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("Dump lacks %q:\n%s", want, buf.String())
		}
	}

	buf.Reset()

	bad := &Module{Records: []Record{&TIR{Type: RecTBT, Commands: []Command{{Op: OpStoreImmediate, Data: []byte{7}}}}}}
	if err := Dump(&buf, bad); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "DST records: DST record 1: length 7, 0 bytes left") {
		t.Errorf("Dump of a bad stream:\n%s", buf.String())
	}
}
