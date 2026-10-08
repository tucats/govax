package dbgsym

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// listedLine is a line of a real MACRO listing that has a location: its
// listing line number, the psect it's in, its offset there, whether it
// shows any binary, and whether it's a data directive (.WORD, .LONG,
// ...).
type listedLine struct {
	line   int
	psect  string
	offset uint32
	binary bool
	data   bool
}

var (
	// listedRE splits a listing line: the binary field (no tabs), the
	// location (four hex digits, the offset in the current psect), the
	// line number, and the source.
	listedRE = regexp.MustCompile(`^([^\t]*)\s([0-9A-F]{4})\s+(\d+)[ \t](.*)$`)

	// psectRE finds a .PSECT directive's psect name in a line's source.
	psectRE = regexp.MustCompile(`(?i)^\s*\.PSECT\s+([A-Z0-9_$.]+)`)

	// labelRE is a label at the start of a line's source.
	labelRE = regexp.MustCompile(`^[A-Z0-9_$.]+::?`)

	// assignRE is a direct assignment (XDO1 = ^XD07E8BD0), whose value
	// the listing shows in the binary field.
	assignRE = regexp.MustCompile(`^[A-Z0-9_$.]+\s*==?`)
)

// listing reads the lines of a MACRO listing that have a location.
func listing(t *testing.T, path string) []listedLine {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var (
		out   []listedLine
		psect string
	)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		text := sc.Text()

		// A .PSECT line's location is eight digits, so the line itself
		// doesn't match listedRE; its source is after the first tab.
		if _, source, ok := strings.Cut(text, "\t"); ok {
			if m := psectRE.FindStringSubmatch(source); m != nil {
				psect = strings.ToUpper(m[1])

				continue
			}
		}

		m := listedRE.FindStringSubmatch(text)
		if m == nil || assignRE.MatchString(strings.TrimSpace(m[4])) {
			continue
		}

		n, _ := strconv.Atoi(m[3])
		offset, _ := strconv.ParseUint(m[2], 16, 32)
		source := strings.TrimSpace(labelRE.ReplaceAllString(strings.TrimSpace(m[4]), ""))

		out = append(out, listedLine{
			line: n, psect: psect, offset: uint32(offset),
			binary: strings.TrimSpace(m[1]) != "",
			// A data directive; .ENTRY's mask word has a row.
			data: strings.HasPrefix(source, ".") && !strings.HasPrefix(strings.ToUpper(source), ".ENTRY"),
		})
	}

	_ = sc.Err()

	return out
}

// TestLinesMatchListings checks each module's line-number table against
// real MACRO's listing of it, in the module's code psect:
//
//   - every row of the table is a listed line, at the address the listing
//     gives (the psect's base in the module plus the location);
//   - every listed instruction (a line with binary that isn't a data
//     directive) is in the row for its line;
//   - data in code (CASEL's .WORD table) gets no row of its own: it's in
//     the row before it, as real MACRO's table has it.
func TestLinesMatchListings(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")

	for _, tc := range []struct {
		image, module, listing, psect string
	}{
		{"dbg/vax/dbgdis.exe", "DBGDIS", "dbg/vax/dbgdis.lis", "CODE"},
		{"dbg/vax/dbgdis.exe", "DBGSUB", "dbg/vax/dbgsub.lis", "CODE"},
		{"dbg/vax/trdbglnk.exe", "TRACE", "mar/list/vax/trdebug.lis", "$CODE"},
		{"dbg/vax/forth.exe", "FORTH", "mar/dst/vax/forth.lis", "FORTH_CODE"},
	} {
		t.Run(tc.module, func(t *testing.T) {
			p := readImageFile(t, filepath.Join(root, tc.image))

			m, ok := p.ModuleNamed(tc.module)
			if !ok {
				t.Fatalf("no module %s", tc.module)
			}

			var base uint32

			for _, ps := range m.Psects {
				if ps.Name == tc.psect {
					base = ps.Address
				}
			}

			byLine := map[int]uint32{}
			checked := 0

			for _, l := range listing(t, filepath.Join(root, tc.listing)) {
				if l.psect != tc.psect {
					continue
				}

				addr := base + l.offset
				if _, seen := byLine[l.line]; !seen {
					byLine[l.line] = addr
				}

				if !l.binary {
					continue
				}

				got, ok := m.LineAt(addr)

				switch {
				case l.data && ok && got.Address == addr:
					t.Errorf("line %d, data at %#x, has a row of its own", l.line, addr)

				case !l.data && (!ok || got.Line != l.line):
					t.Errorf("address %#x: line %d (%v), want %d", addr, got.Line, ok, l.line)
				}

				checked++
			}

			if checked == 0 {
				t.Fatal("no listing lines checked")
			}

			for _, row := range m.Lines {
				if want, ok := byLine[row.Line]; !ok || want != row.Address {
					t.Errorf("row %+v: the listing has line %d at %#x (%v)", row, row.Line, want, ok)
				}
			}
		})
	}
}

func readImageFile(t *testing.T, path string) *Program {
	t.Helper()

	dir, file := filepath.Split(path)
	saved := probe
	probe = dir

	defer func() { probe = saved }()

	return readProgram(t, strings.TrimSuffix(file, ".exe"))
}

// TestLinesMatchDebugger checks the answers the VMS debugger gave about
// lines in the probe's sessions.
func TestLinesMatchDebugger(t *testing.T) {
	p := readProgram(t, "dbgdis")
	dis, _ := p.ModuleNamed("DBGDIS")
	sub, _ := p.ModuleNamed("DBGSUB")

	// EVALUATE/ADDRESS %LINE 47 and DBGSUB\%LINE 14.
	if a, ok := dis.AddressOfLine(47); !ok || a != 0x419 {
		t.Errorf("DBGDIS %%LINE 47 = %#x, %v; want 0x419", a, ok)
	}

	if a, ok := sub.AddressOfLine(14); !ok || a != 0x546 {
		t.Errorf("DBGSUB %%LINE 14 = %#x, %v; want 0x546", a, ok)
	}

	// SYMBOLIZE: START is line 41 (its entry mask), START+2 line 42,
	// JSBRTN+3 (0x53E) line 110 plus 3, and line 85 is LOOP (0x4DD).
	for _, tc := range []struct {
		addr   uint32
		line   int
		offset uint32
	}{
		{0x400, 41, 0}, {0x402, 42, 0}, {0x53E, 110, 3}, {0x4DD, 85, 0},
		// The case table's three targets (lines 100 to 102) and the
		// stepping session's lines.
		{0x525, 100, 0}, {0x527, 101, 0}, {0x528, 102, 0}, {0x425, 49, 0},
	} {
		l, m, ok := p.LineAt(tc.addr)
		if !ok || m != dis || l.Line != tc.line || tc.addr-l.Address != tc.offset {
			t.Errorf("LineAt(%#x) = %+v in %v; want line %d+%d", tc.addr, l, m, tc.line, tc.offset)
		}
	}

	// FAILLNK's access violation: FAILSUB\SUB2\%LINE 12 at 0x21C, and
	// the frames' lines (SUB1's call at 7, FAILMAIN's at 14).
	fail := readProgram(t, "faillnk")

	for _, tc := range []struct {
		pc     uint32
		module string
		line   int
	}{
		{0x21C, "FAILSUB", 12}, {0x216 - 1, "FAILSUB", 7}, {0x209 - 1, "FAILMAIN", 14},
	} {
		l, m, ok := fail.LineAt(tc.pc)
		if !ok || m.Name != tc.module || l.Line != tc.line {
			t.Errorf("FAILLNK LineAt(%#x) = %+v in %v; want %s line %d", tc.pc, l, m, tc.module, tc.line)
		}
	}
}

// TestSourceFiles checks the source correlation: each probe module's
// file, and that listing lines map one to one to its records.
func TestSourceFiles(t *testing.T) {
	p := readProgram(t, "dbgdis")

	for _, tc := range []struct {
		module, spec string
		lines        int
	}{
		{"DBGDIS", "DUA1:[000000]DBGDIS.MAR;1", 0x71},
		{"DBGSUB", "DUA1:[000000]DBGSUB.MAR;1", 0x1A},
	} {
		m, _ := p.ModuleNamed(tc.module)

		if len(m.Files) != 1 || m.Files[0].Spec != tc.spec || m.Files[0].ID != 1 {
			t.Errorf("%s files = %+v, want %s", tc.module, m.Files, tc.spec)

			continue
		}

		for _, n := range []int{1, 12, tc.lines} {
			f, rec, ok := m.SourceOf(n)
			if !ok || f.Spec != tc.spec || rec != n {
				t.Errorf("%s SourceOf(%d) = %v, %d, %v; want record %d", tc.module, n, f, rec, ok, n)
			}
		}

		if _, _, ok := m.SourceOf(tc.lines + 1); ok {
			t.Errorf("%s SourceOf(%d) found a record past the end", tc.module, tc.lines+1)
		}
	}

	// A traceback link has no line or source records.
	trc, _ := readProgram(t, "dbgtrc").ModuleNamed("DBGDIS")
	if len(trc.Lines) != 0 || len(trc.Files) != 0 {
		t.Errorf("DBGTRC has %d lines and %d files", len(trc.Lines), len(trc.Files))
	}
}

// TestLineProgram runs hand-made line-number programs through the
// commands MACRO doesn't use.
func TestLineProgram(t *testing.T) {
	prog := []byte{
		16, 0x00, 0x10, 0, 0, // SET_ABS_PC 0x1000
		9, 9, 0, // SET_LINUM 9
		0xFF, // Delta-PC 1: line 10 at 0x1001
		4, 2, // SET_LINUM_INCR 2
		1, 0x10, 0x00, // DELTA_PC_W 16: line 12 at 0x1011
		14, 4, // TERM 4
		10, 0x20, // SET_PC 0x20 from START_PC (0x2000): 0x2020
		7,              // BEG_STMT_MODE
		0x00,           // Delta-PC 0: line 12, statement 2, at 0x2020
		0xFE,           // statement 3 at 0x2022
		8,              // END_STMT_MODE
		21, 6, 0, 0, 0, // TERM_L 6
	}

	rows, err := lineTable(prog, 0x2000, 0)
	if err != nil {
		t.Fatal(err)
	}

	want := []Line{
		{Line: 10, Stmt: 1, Address: 0x1001, Length: 0x10},
		{Line: 12, Stmt: 1, Address: 0x1011, Length: 4},
		{Line: 12, Stmt: 2, Address: 0x2020, Length: 2},
		{Line: 12, Stmt: 3, Address: 0x2022, Length: 6},
	}

	if len(rows) != len(want) {
		t.Fatalf("rows = %+v", rows)
	}

	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}

	if _, err := lineTable([]byte{99}, 0, 0); err == nil {
		t.Error("an undefined command wasn't an error")
	}
}
