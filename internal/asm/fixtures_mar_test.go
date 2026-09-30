package asm

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/obj"
)

// realObject reads testdata/mar/vax/name.obj, the object real MACRO made
// from testdata/mar/name.mar. It skips the test when that object hasn't
// come back from the VAX yet.
func realObject(t *testing.T, name string) *obj.Module {
	t.Helper()

	f, err := os.Open(filepath.Join("..", "..", "testdata", "mar", "vax", name+".obj"))
	if errors.Is(err, fs.ErrNotExist) {
		// A fixture waiting for its trip to the VAX.
		t.Skipf("no real MACRO object for %s.mar yet", name)
	}

	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	raw, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// objectSummary describes what an object module declares, one line each,
// in a form an assembly can be described in too: the module name and
// version, each psect (name, index, alignment, flags, and allocation),
// and each global symbol (defined or referred to, weak, psect, value, and
// entry mask).
type objectSummary struct {
	module  string
	psects  []string
	symbols []string
}

func summarizeObject(m *obj.Module) objectSummary {
	var s objectSummary

	index := 0

	for _, rec := range m.Records {
		switch r := rec.(type) {
		case *obj.MainHeader:
			s.module = r.Name + " " + r.Version

		case *obj.GSD:
			for _, sub := range r.Subrecords {
				switch e := sub.(type) {
				case *obj.Psect:
					s.psects = append(s.psects, fmt.Sprintf("%d %s align=%d flags=%03X alloc=%d", index, e.Name, e.Align, e.Flags, e.Alloc))
					index++

				case *obj.Symbol:
					s.symbols = append(s.symbols, globalLine(e.Name, e.Defined(), e.Flags&obj.SymWEAK != 0, int(e.Psect), e.Value, e.GSDType() == obj.GSDEntry, e.Mask))
				}
			}
		}
	}

	sort.Strings(s.symbols)

	return s
}

func globalLine(name string, defined, weak bool, psect int, value uint32, entry bool, mask uint16) string {
	line := name

	if weak {
		line += " weak"
	}

	if !defined {
		return line + " ref"
	}

	line += fmt.Sprintf(" def %d:%X", psect, value)

	if entry {
		line += fmt.Sprintf(" mask=%04X", mask)
	}

	return line
}

// summarizeAssembly describes an assembly the way summarizeObject does an
// object module.
func summarizeAssembly(a *Assembler) objectSummary {
	var s objectSummary

	name, _ := a.Title()
	s.module = name + " " + a.Ident()

	for _, sect := range a.sections {
		alloc := sect.hi
		if !sect.relocatable {
			alloc = 0 // an absolute psect allocates nothing
		}

		s.psects = append(s.psects, fmt.Sprintf("%d %s align=%d flags=%03X alloc=%d", sect.index, sect.name, sect.align, sect.flags, alloc))
	}

	for name, sym := range a.symbols.byName {
		switch {
		case sym.flags&SymExternal != 0:
			s.symbols = append(s.symbols, globalLine(name, false, sym.flags&SymWeak != 0, 0, 0, false, 0))

		case sym.flags&SymGlobal != 0:
			psect := 0

			switch {
			case sym.sect != nil:
				psect = sym.sect.index
			case sym.absSect != nil:
				psect = sym.absSect.index
			}

			s.symbols = append(s.symbols, globalLine(name, true, sym.flags&SymWeak != 0, psect, sym.value, sym.flags&SymEntry != 0, sym.mask))
		}
	}

	sort.Strings(s.symbols)

	return s
}

// TestFixtureLadderDeclarations assembles each testdata/mar fixture and
// checks that it declares what real MACRO's object for it does: the
// module name and version, each psect's index, attributes, and
// allocation, and the global symbols it defines and refers to.
func TestFixtureLadderDeclarations(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssemble(t, string(src))

			got, want := summarizeAssembly(a), summarizeObject(realObject(t, name))

			if got.module != want.module {
				t.Errorf("module = %q, want %q", got.module, want.module)
			}

			compareLines(t, "psects", got.psects, want.psects)
			compareLines(t, "symbols", got.symbols, want.symbols)
		})
	}
}

func compareLines(t *testing.T, what string, got, want []string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n%s\nwant:\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// objectText is what an object module's TIR records put in each psect:
// its bytes, and the values left for the linker, one line each in
// Relocations' form ("CODE+2 LD DATA:0"), in psect and offset order.
type objectText struct {
	bytes  map[string][]byte
	relocs []string
}

// replayText runs an object module's TIR records the way the linker
// would, without choosing psect bases: STORE IMMEDIATE and CTL_AUGRB fill
// and move through the psect CTL_SETRB chose, and each other store
// records its stack program as a relocation. A stored value that is a
// constant (like an .ENTRY mask's STA_UB, STO_W) is data. Only the
// commands real MACRO's objects use are handled.
func replayText(t *testing.T, m *obj.Module) objectText {
	t.Helper()

	var (
		names  []string
		allocs []uint32
	)

	for _, rec := range m.Records {
		if g, ok := rec.(*obj.GSD); ok {
			for _, sub := range g.Subrecords {
				if p, ok := sub.(*obj.Psect); ok {
					names = append(names, p.Name)
					allocs = append(allocs, p.Alloc)
				}
			}
		}
	}

	out := objectText{bytes: map[string][]byte{}}
	for i, name := range names {
		out.bytes[name] = make([]byte, allocs[i])
	}

	type entry struct {
		text     string
		constant bool
		value    uint32
		psect    int
		offset   uint32
	}

	type reloc struct {
		psect  int
		offset uint32
		line   string
	}

	var (
		stack  []entry
		relocs []reloc
		psect  int
		offset uint32
	)

	pop := func() entry {
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		return e
	}

	put := func(v uint32, n int) {
		for i := 0; i < n; i++ {
			out.bytes[names[psect]][offset+uint32(i)] = byte(v >> (8 * i))
		}
	}

	binary := map[string]string{"OPR_ADD": "+", "OPR_SUB": "-", "OPR_MUL": "*", "OPR_DIV": "/", "OPR_AND": "&", "OPR_IOR": "!", "OPR_EOR": "\\", "OPR_ASH": "@"}
	// data marks the stores whose constant value is stored as it is:
	// the others store it relative to the location (a displacement) or
	// as the linker chooses (PIC), so the linker finishes them.
	stores := map[string]struct {
		kind string
		size int
		data bool
	}{
		"STO_B": {"B", 1, true}, "STO_W": {"W", 2, true}, "STO_L": {"L", 4, true},
		"STO_SB": {"SB", 1, true}, "STO_SW": {"SW", 2, true},
		"STO_BD": {"BD", 1, false}, "STO_WD": {"WD", 2, false}, "STO_LD": {"LD", 4, false},
		"STO_PIDR": {"PIDR", 4, false}, "STO_PICR": {"PICR", 5, false},
	}

	for _, rec := range m.Records {
		tir, ok := rec.(*obj.TIR)
		if !ok || tir.Type != obj.RecTIR {
			continue
		}

		for _, c := range tir.Commands {
			name := c.Op.String()

			switch {
			case name == "STO_IMM":
				copy(out.bytes[names[psect]][offset:], c.Data)
				offset += uint32(len(c.Data))

			case name == "STA_PB" || name == "STA_PW" || name == "STA_PL":
				v := c.StackedValue()
				stack = append(stack, entry{text: fmt.Sprintf("%s:%X", names[c.Psect], v), psect: int(c.Psect), offset: v})

			case name == "STA_GBL":
				stack = append(stack, entry{text: c.Name})

			case strings.HasPrefix(name, "STA_") && (strings.HasSuffix(name, "B") || strings.HasSuffix(name, "W")) || name == "STA_LW":
				v := c.StackedValue()
				stack = append(stack, entry{text: fmt.Sprintf("%d", int32(v)), constant: true, value: v})

			case binary[name] != "":
				r, l := pop(), pop()
				stack = append(stack, entry{text: l.text + " " + r.text + " " + binary[name]})

			case name == "OPR_NEG" || name == "OPR_COM":
				l := pop()
				stack = append(stack, entry{text: l.text + " " + strings.TrimPrefix(name, "OPR_")})

			case name == "CTL_SETRB":
				e := pop()
				psect, offset = e.psect, e.offset

			case name == "CTL_AUGRB":
				offset += c.StackedValue()

			case stores[name].kind != "":
				st := stores[name]
				e := pop()

				if e.constant && st.data {
					put(e.value, st.size)
				} else {
					relocs = append(relocs, reloc{psect, offset, fmt.Sprintf("%s+%X %s %s", names[psect], offset, st.kind, e.text)})
				}

				offset += uint32(st.size)

			default:
				t.Fatalf("replay: unhandled TIR command %s", name)
			}
		}
	}

	if len(stack) != 0 {
		t.Fatalf("replay: %d values left on the stack", len(stack))
	}

	sort.SliceStable(relocs, func(i, j int) bool {
		if relocs[i].psect != relocs[j].psect {
			return relocs[i].psect < relocs[j].psect
		}

		return relocs[i].offset < relocs[j].offset
	})

	for _, r := range relocs {
		out.relocs = append(out.relocs, r.line)
	}

	return out
}

// TestFixtureLadderText assembles each testdata/mar fixture and checks
// that every psect holds the bytes real MACRO's object puts there, and
// that it leaves the linker the same values to finish, as the same stack
// programs.
func TestFixtureLadderText(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssemble(t, string(src))
			want := replayText(t, realObject(t, name))

			for _, s := range a.sections {
				// An absolute psect's offsets are only symbol values;
				// it holds nothing (see TestFixtureLadderDeclarations).
				if !s.relocatable {
					continue
				}

				got := s.img.Bytes(0, s.hi)
				if fmt.Sprintf("% X", got) != fmt.Sprintf("% X", want.bytes[s.name]) {
					t.Errorf("psect %s:\n% X\nwant:\n% X", s.name, got, want.bytes[s.name])
				}
			}

			compareLines(t, "relocations", a.Relocations(), want.relocs)
		})
	}
}

// withoutTraceback returns m without its traceback records, which govax
// doesn't write yet.
func withoutTraceback(m *obj.Module) *obj.Module {
	out := &obj.Module{}

	for _, rec := range m.Records {
		if rec.RecordType() != obj.RecTBT {
			out.Records = append(out.Records, rec)
		}
	}

	return out
}

func dumpText(t *testing.T, m *obj.Module) string {
	t.Helper()

	var sb strings.Builder
	if err := obj.Dump(&sb, m); err != nil {
		t.Fatal(err)
	}

	return sb.String()
}

// TestFixtureLadderObjects assembles each testdata/mar fixture into an
// object module and checks it against real MACRO's, record for record:
// the same headers, GSD and TIR records, in the same order, holding the
// same subrecords and commands, and the same end of module record. Only
// what govax doesn't write yet, the traceback records, is left out of the
// comparison, and the headers that name the language processor, its
// command line, and the time are given real MACRO's values. The object
// must also encode and decode unchanged, and pass Check.
func TestFixtureLadderObjects(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "*.mar"))
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			realModule := realObject(t, name)

			var opts ObjectOptions

			for _, rec := range realModule.Records {
				switch h := rec.(type) {
				case *obj.MainHeader:
					if opts.Created, err = time.Parse("02-Jan-2006 15:04", h.Created); err != nil {
						t.Fatal(err)
					}

				case *obj.TextHeader:
					switch h.Type {
					case obj.HdrLNM:
						opts.Language = h.Text
					case obj.HdrSRC:
						opts.Source = h.Text
					}
				}
			}

			m, err := macroAssemble(t, string(src)).Object(opts)
			if err != nil {
				t.Fatal(err)
			}

			raw, err := obj.Encode(m)
			if err != nil {
				t.Fatal(err)
			}

			back, err := obj.Decode(raw)
			if err != nil {
				t.Fatal(err)
			}

			if problems := obj.Check(back); len(problems) > 0 {
				t.Errorf("Check: %v", problems)
			}

			if got, want := dumpText(t, back), dumpText(t, withoutTraceback(realModule)); got != want {
				t.Errorf("object:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
