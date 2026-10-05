package dbgsym

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vmsimage"
)

// mapSymbols reads the "Symbols By Name" section of a LINK map: each
// symbol the link's own modules defined, and its value.
func mapSymbols(t *testing.T, name string) map[string]uint32 {
	t.Helper()

	f, err := os.Open(filepath.Join(probe, name+".map"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	out := map[string]uint32{}
	section := false

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()

		switch {
		case strings.Contains(line, "! Symbols By Name !"):
			section = true

		case !section || strings.HasPrefix(line, "Symbol") || strings.HasPrefix(line, "------"):

		case strings.TrimSpace(line) == "" && len(out) > 0:
			return out

		default:
			// Up to four columns of a name and a value, the value
			// marked -R when relocatable.
			f := strings.Fields(line)
			for i := 0; i+1 < len(f); i += 2 {
				v, err := strconv.ParseUint(strings.TrimSuffix(f[i+1], "-R"), 16, 32)
				if err != nil {
					t.Fatalf("%s.map: %q: %v", name, line, err)
				}

				out[f[i]] = uint32(v)
			}
		}
	}

	return out
}

// TestGlobalsMatchMap checks each /DEBUG image's global symbol table
// against its map: every symbol the map lists is a global with the
// map's value, and the only others are the system services the
// libraries defined (SYS$IMGSTA, and FORTH's RMS services). Routines
// are entry points; data and constants aren't.
func TestGlobalsMatchMap(t *testing.T) {
	for _, name := range []string{"dbgdis", "forth", "trdbglnk", "trlnkdbg", "faillnk", "gvdbgdis", "gvtrace"} {
		t.Run(name, func(t *testing.T) {
			p := readProgram(t, name)
			if p.Globals == nil {
				t.Fatal("no global symbol table")
			}

			want := mapSymbols(t, name)
			if len(want) == 0 {
				t.Fatal("no symbols in the map")
			}

			for n, v := range want {
				s, ok := p.Globals.Get(n)
				if !ok {
					t.Errorf("%s isn't in the GST", n)
				} else if s.Value != v {
					t.Errorf("%s = %#x, the map says %#x", n, s.Value, v)
				}
			}

			for _, s := range p.Globals.All() {
				if _, ok := want[s.Name]; !ok && !strings.HasPrefix(s.Name, "SYS$") {
					t.Errorf("%s (%#x) is in the GST, not in the map", s.Name, s.Value)
				}

				if s.Flags&symtab.Global == 0 {
					t.Errorf("%s isn't flagged global", s.Name)
				}

				if r, _, ok := p.RoutineAt(s.Value); ok && r.Address == s.Value && !r.NoCall && s.Flags&symtab.Entry == 0 {
					t.Errorf("routine %s isn't an entry point in the GST", s.Name)
				}
			}
		})
	}

	p := readProgram(t, "forth")
	if s, ok := p.Globals.Get("SYS$OPEN"); !ok || s.Value != 0x7FFEE208 {
		t.Errorf("FORTH's SYS$OPEN = %v", s)
	}

	if s, _ := p.Globals.Get("FORTH"); s == nil || s.Flags&symtab.Entry == 0 {
		t.Errorf("FORTH's entry point = %v", s)
	}
}

// TestGlobalsMatchSymbolize checks the GST against the VMS debugger's
// SYMBOLIZE: the "(global)" half of each answer names the global at or
// below the address, plus an offset (GLIMIT+209, SECOND+0F), in the
// session's radix.
func TestGlobalsMatchSymbolize(t *testing.T) {
	checked := 0

	for _, name := range []string{"dbgdis", "gvdbgdis", "forth", "trdbglnk", "trlnkdbg", "gvtrace"} {
		p := readProgram(t, name)

		data, err := os.ReadFile(filepath.Join(probe, name+".dlg"))
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(string(data), "\n")
		decimal := false

		for i, line := range lines {
			line = strings.TrimPrefix(strings.TrimRight(line, "\r"), "!")

			switch {
			case strings.HasPrefix(line, " SET RADIX DECIMAL"):
				decimal = true

			case strings.HasPrefix(line, " CANCEL RADIX"), strings.HasPrefix(line, " SET RADIX HEX"):
				decimal = false
			}

			if !strings.HasPrefix(line, "address ") || !strings.HasSuffix(line, ": (global)") || i+1 >= len(lines) {
				continue
			}

			addr, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(line, "address "), ": (global)"), 16, 32)
			if err != nil {
				t.Fatalf("%s.dlg: %q: %v", name, line, err)
			}

			want := strings.TrimSpace(strings.TrimPrefix(lines[i+1], "!"))

			got := "nothing"
			if s, off, ok := p.Globals.Nearest(uint32(addr), nil); ok {
				got = s.Name + offset(off, decimal)
			}

			if got != want {
				t.Errorf("%s: SYMBOLIZE %08X gave %s from the GST, the debugger %s", name, addr, got, want)
			}

			checked++
		}
	}

	if checked < 15 {
		t.Errorf("checked %d answers, want at least 15", checked)
	}
}

// offset writes an offset past a symbol as the debugger does: nothing
// for none, else "+" and the number, in hexadecimal with a leading 0
// when its first digit is a letter (+0F), or in decimal.
func offset(off uint32, decimal bool) string {
	switch {
	case off == 0:
		return ""
	case decimal:
		return fmt.Sprintf("+%d", off)
	}

	s := fmt.Sprintf("%X", off)
	if s[0] >= 'A' {
		s = "0" + s
	}

	return "+" + s
}

// TestModuleTable checks the debug module table of each /DEBUG image:
// an entry per module, at the offset and with the size of the module's
// records in the DST, whose ranges are the module's PSECT records',
// in the same order. ModuleAt then works from the ranges.
func TestModuleTable(t *testing.T) {
	for _, name := range []string{"dbgdis", "forth", "trdbglnk", "trlnkdbg", "faillnk", "gvdbgdis", "gvtrace"} {
		t.Run(name, func(t *testing.T) {
			p := readProgram(t, name)

			for i, m := range p.Modules {
				if len(m.Ranges) == 0 {
					t.Fatalf("module %s has no DMT entry", m.Name)
				}

				if m.DSTSize == 0 || (i+1 < len(p.Modules) && m.DSTOffset+m.DSTSize != p.Modules[i+1].DSTOffset) {
					t.Errorf("module %s's records: %#x, %d bytes", m.Name, m.DSTOffset, m.DSTSize)
				}

				if len(m.Ranges) != len(m.Psects) {
					t.Fatalf("module %s: %d ranges, %d psects", m.Name, len(m.Ranges), len(m.Psects))
				}

				for j, r := range m.Ranges {
					if ps := m.Psects[j]; r.Address != ps.Address || r.Size != ps.Size {
						t.Errorf("module %s: range %d is %#x+%#x, psect %s %#x+%#x", m.Name, j, r.Address, r.Size, ps.Name, ps.Address, ps.Size)
					}

					for _, a := range []uint32{r.Address, r.Address + r.Size - 1} {
						if got, ok := p.ModuleAt(a); !ok || got != m {
							t.Errorf("ModuleAt(%#x) isn't %s", a, m.Name)
						}
					}
				}
			}
		})
	}

	// DBGDIS's two modules, by the map's psect contributions.
	p := readProgram(t, "dbgdis")
	for addr, want := range map[uint32]string{0x200: "DBGDIS", 0x267: "DBGDIS", 0x268: "DBGSUB", 0x541: "DBGDIS", 0x544: "DBGSUB"} {
		if m, ok := p.ModuleAt(addr); !ok || m.Name != want {
			t.Errorf("ModuleAt(%#x) = %v, want %s", addr, m, want)
		}
	}

	if _, ok := p.ModuleAt(0x542); ok {
		t.Error("ModuleAt(0x542), between the two modules' code, found one")
	}
}

// TestTracebackOnly checks that an image linked with traceback, which
// has a DST but no DMT or GST, reads from the DST alone.
func TestTracebackOnly(t *testing.T) {
	for _, name := range []string{"dbgtrc", "trdbgtrc"} {
		p := readProgram(t, name)

		if p.Globals != nil {
			t.Errorf("%s: a GST of %d symbols", name, p.Globals.Len())
		}

		for _, m := range p.Modules {
			if len(m.Ranges) != 0 {
				t.Errorf("%s: module %s has DMT ranges", name, m.Name)
			}

			if len(m.Psects) == 0 {
				t.Errorf("%s: module %s has no psects", name, m.Name)

				continue
			}

			if got, ok := p.ModuleAt(m.Psects[0].Address); !ok || got != m {
				t.Errorf("%s: ModuleAt(%#x) isn't %s", name, m.Psects[0].Address, m.Name)
			}
		}
	}
}

// TestRelocatedTables checks that the DMT's ranges and the GST's values
// move with the load base, as the DST's addresses do.
func TestRelocatedTables(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(probe, "dbgdis.exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := Read(img, data, 0x10000)
	if err != nil {
		t.Fatal(err)
	}

	p := readProgram(t, "dbgdis")

	if got, want := moved.Modules[1].Ranges[1].Address, p.Modules[1].Ranges[1].Address+0x10000; got != want {
		t.Errorf("relocated DBGSUB code range at %#x, want %#x", got, want)
	}

	if s, _ := moved.Globals.Get("SUB2"); s.Value != 0x10558 {
		t.Errorf("relocated SUB2 = %#x", s.Value)
	}
}
