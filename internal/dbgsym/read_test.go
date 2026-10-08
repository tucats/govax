package dbgsym

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vmsimage"
)

// probe is where Phase 41's debugger probe left its images and the VMS
// debugger's sessions (testdata/dbg/README.md).
var probe = filepath.Join("..", "..", "testdata", "dbg", "vax")

func readProgram(t *testing.T, name string) *Program {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(probe, name+".exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	p, err := Read(img, data, 0)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// debuggerSymbol is one symbol as the VMS debugger's SHOW SYMBOL/ADDRESS
// listed it: "routine DBGDIS\START" and "address: 00000400, size:
// 00000132 bytes", "data DBGDIS\LIMIT" and "constant: 0000000A", and so
// on.
type debuggerSymbol struct {
	kind, path string
	value      string // "address: ...", "constant: ...", with the size if given
}

// debuggerSymbols reads every SHOW SYMBOL/ADDRESS listing in a session
// log.
func debuggerSymbols(t *testing.T, name string) []debuggerSymbol {
	t.Helper()

	f, err := os.Open(filepath.Join(probe, name+".dlg"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var (
		out    []debuggerSymbol
		inList bool
	)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimPrefix(sc.Text(), "!")

		switch {
		case strings.HasPrefix(line, " SHOW SYMBOL/ADDRESS"):
			inList = true

		case strings.HasPrefix(line, " "+strings.TrimLeft(line, " ")) && !strings.HasPrefix(line, "    "):
			// The next command ends the listing.
			inList = false

		case inList && strings.HasPrefix(line, "    "):
			out[len(out)-1].value = strings.TrimSpace(line)

		case inList:
			kind, path, _ := strings.Cut(line, " ")
			out = append(out, debuggerSymbol{kind: kind, path: path})
		}
	}

	_ = sc.Err()

	return out
}

// describe gives a symbol as the debugger listed it.
func describe(p *Program, s *symtab.Symbol) debuggerSymbol {
	d := debuggerSymbol{path: Path(s)}

	switch {
	case s.IsEntry():
		d.kind = "routine"
		d.value = fmt.Sprintf("address: %08X, size: %08X bytes", s.Value, s.Size)

	case s.Has(symtab.Psect):
		d.kind = "label"
		d.value = fmt.Sprintf("address: %08X, size: %08X bytes", s.Value, s.Size)

	case s.IsLabel():
		d.kind = "label"
		d.value = fmt.Sprintf("address: %08X", s.Value)

	case s.Has(symtab.Literal):
		d.kind = "data"
		d.value = fmt.Sprintf("constant: %08X", s.Value)

	default:
		d.kind = "data"
		d.value = fmt.Sprintf("address: %08X", s.Value)

		m, _ := p.ModuleNamed(strings.Split(s.Scope, `\`)[0])
		for _, datum := range m.Data {
			if datum.Name == s.Name && datum.Kind == DescriptorAddress {
				d.value = fmt.Sprintf("descriptor address: %08X", s.Value)
			}
		}
	}

	return d
}

// TestSymbolsMatchDebugger checks every symbol the VMS debugger listed
// (SHOW SYMBOL/ADDRESS * IN each module) against the reader's: its kind,
// path, and address, size, or constant. For DBGDIS, GVDBGDIS, and DBGTRC
// (a traceback link: routines and psects only) the sets must be the
// same. A datum the debugger describes by a descriptor it built
// ("descriptor address" in its own memory) is matched by path alone.
func TestSymbolsMatchDebugger(t *testing.T) {
	for _, name := range []string{"dbgdis", "gvdbgdis", "dbgtrc"} {
		t.Run(name, func(t *testing.T) {
			p := readProgram(t, name)

			if len(p.Skipped) != 0 {
				t.Errorf("records skipped: %v", p.Skipped)
			}

			ours := map[string]debuggerSymbol{}

			for _, m := range p.Modules {
				for _, s := range m.Symbols.All() {
					d := describe(p, s)
					ours[d.path] = d
				}
			}

			theirs := debuggerSymbols(t, name)
			if len(theirs) == 0 {
				t.Fatal("no SHOW SYMBOL listing in the session")
			}

			for _, want := range theirs {
				got, ok := ours[want.path]
				if !ok {
					t.Errorf("%s %s: not read", want.kind, want.path)

					continue
				}

				delete(ours, want.path)

				if got.kind != want.kind {
					t.Errorf("%s: kind %s, want %s", want.path, got.kind, want.kind)
				}

				if got.value != want.value && !(strings.HasPrefix(want.value, "descriptor address") && got.kind == "data") {
					t.Errorf("%s: %s, want %s", want.path, got.value, want.value)
				}
			}

			for path, d := range ours {
				t.Errorf("%s %s: read, but the debugger didn't list it", d.kind, path)
			}
		})
	}
}

// TestForthLabels checks the labels the debugger listed for FORTH
// (SHOW SYMBOL/ADDRESS F_A*: macro-defined labels in a module of
// thousands of symbols).
func TestForthLabels(t *testing.T) {
	p := readProgram(t, "forth")

	if len(p.Skipped) != 0 {
		t.Errorf("records skipped: %v", p.Skipped)
	}

	theirs := debuggerSymbols(t, "forth")
	if len(theirs) != 7 {
		t.Fatalf("the session lists %d symbols, want 7", len(theirs))
	}

	for _, want := range theirs {
		s, ok := p.Lookup(want.path)
		if !ok {
			t.Errorf("%s: not found", want.path)

			continue
		}

		if got := describe(p, s); got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	}
}

// TestDescriptors checks the descriptors of DBGDIS's arrays and string,
// and the data addresses taken from them.
func TestDescriptors(t *testing.T) {
	m, _ := readProgram(t, "dbgdis").ModuleNamed("DBGDIS")

	data := map[string]*Datum{}
	for _, d := range m.Data {
		data[d.Name] = d
	}

	for _, tc := range []struct {
		name   string
		class  byte
		typ    byte
		length uint16
		addr   uint32
		bounds []Bound
	}{
		{"TABLE", descClassA, 8, 4, 0x204, []Bound{{0, 3}}},
		{"BYTES", descClassA, 6, 1, 0x214, []Bound{{0, 7}}},
		{"TEXT", descClassS, 14, 8, 0x246, nil},
	} {
		d := data[tc.name]
		if d == nil || d.Descriptor == nil {
			t.Errorf("%s: no descriptor", tc.name)

			continue
		}

		desc := d.Descriptor
		if desc.Class != tc.class || desc.Type != tc.typ || desc.Length != tc.length || d.Value != tc.addr ||
			fmt.Sprint(desc.Bounds) != fmt.Sprint(tc.bounds) {
			t.Errorf("%s = %+v (value %#x), want class %d type %d length %d at %#x bounds %v",
				tc.name, *desc, d.Value, tc.class, tc.typ, tc.length, tc.addr, tc.bounds)
		}
	}

	if d := data["MSG"]; d == nil || d.Kind != DescriptorAddress || d.Value != 0x232 {
		t.Errorf("MSG = %+v, want the descriptor's address 0x232", d)
	}
}

// TestLookupAndRoutines checks path lookups (a label is in its routine's
// scope), RoutineAt and ModuleAt, and relocation by a base address.
func TestLookupAndRoutines(t *testing.T) {
	p := readProgram(t, "dbgdis")

	for _, tc := range []struct {
		path  string
		value uint32
		ok    bool
	}{
		{`DBGDIS\START`, 0x400, true},
		{`DBGDIS\START\LOOP`, 0x4DD, true},
		{`DBGDIS\LOOP`, 0, false},
		{`DBGSUB\SUB2\SUBEND`, 0x567, true},
		{`DBGSUB\SUBEND`, 0, false},
		{`dbgdis\count`, 0x200, true},
		{`LIMIT`, 10, true},
		{`SUBDATA`, 0x268, true},
		{`NOSUCH`, 0, false},
	} {
		s, ok := p.Lookup(tc.path)
		if ok != tc.ok || (ok && s.Value != tc.value) {
			t.Errorf("Lookup(%s) = %v, %v; want %#x, %v", tc.path, s, ok, tc.value, tc.ok)
		}
	}

	if r, m, ok := p.RoutineAt(0x53B); !ok || r.Name != "LOCALR" || m.Name != "DBGDIS" {
		t.Errorf("RoutineAt(0x53B) = %v, %v", r, ok)
	}

	if _, _, ok := p.RoutineAt(0x300); ok {
		t.Error("RoutineAt(0x300) found a routine in data")
	}

	if m, ok := p.ModuleAt(0x268); !ok || m.Name != "DBGSUB" {
		t.Errorf("ModuleAt(0x268) = %v, %v", m, ok)
	}

	data, _ := os.ReadFile(filepath.Join(probe, "dbgdis.exe"))
	img, _ := vmsimage.ReadImage(data)

	moved, err := Read(img, data, 0x10000)
	if err != nil {
		t.Fatal(err)
	}

	if s, _ := moved.Lookup(`DBGDIS\START\LOOP`); s.Value != 0x104DD {
		t.Errorf("relocated LOOP = %#x", s.Value)
	}

	if s, _ := moved.Lookup("LIMIT"); s.Value != 10 {
		t.Errorf("relocated LIMIT = %#x; a constant isn't an address", s.Value)
	}
}

// TestNoDST checks an image linked /NOTRACEBACK.
func TestNoDST(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(probe, "dbgnotb.exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Read(img, data, 0); !errors.Is(err, ErrNoDST) {
		t.Errorf("Read = %v, want ErrNoDST", err)
	}
}

// TestReadEveryImage reads every fixture image's DST: Phase 41's probe,
// Phase 29's traceback images, and FORTH. Each reads without error and
// with no record skipped, and every routine has an extent.
func TestReadEveryImage(t *testing.T) {
	var files []string

	for _, dir := range []string{probe, filepath.Join("..", "..", "testdata", "mar", "list", "vax"), filepath.Join("..", "..", "testdata", "mar", "dst", "vax")} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.exe"))
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, matches...)
	}

	read := 0

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		img, err := vmsimage.ReadImage(data)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}

		p, err := Read(img, data, 0)
		if errors.Is(err, ErrNoDST) {
			continue
		}

		if err != nil {
			t.Errorf("%s: %v", file, err)

			continue
		}

		read++

		if len(p.Skipped) != 0 || len(p.Modules) == 0 {
			t.Errorf("%s: %d modules, skipped %v", file, len(p.Modules), p.Skipped)
		}

		for _, m := range p.Modules {
			for _, r := range m.Routines {
				if r.Size == 0 {
					t.Errorf("%s: %s\\%s has no extent", file, m.Name, r.Name)
				}
			}
		}
	}

	if read < 15 {
		t.Errorf("read %d images with a DST, want at least 15", read)
	}
}
