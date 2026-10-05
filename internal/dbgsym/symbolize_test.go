package dbgsym

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/vmsimage"
)

// imageMemory is an image's sections laid out at their virtual
// addresses, as the image activator maps a main image: what the
// disassembler reads instructions from in these tests.
type imageMemory map[uint32]byte

func loadImageMemory(t *testing.T, name string) imageMemory {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(probe, name+".exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	mem := imageMemory{}

	for _, isd := range img.ISDs {
		if isd.VBN == 0 || isd.Flags&vmsimage.ISDFlagGBL != 0 {
			continue // demand zero, or a shareable image's section
		}

		va := isd.VPN * vmsimage.BlockSize
		from := int(isd.VBN-1) * vmsimage.BlockSize

		for i := 0; i < int(isd.Pages)*vmsimage.BlockSize && from+i < len(data); i++ {
			mem[va+uint32(i)] = data[from+i]
		}
	}

	return mem
}

func (m imageMemory) ByteAt(addr uint32) byte { return m[addr] }

// examineLine is one line the debugger printed for EXAMINE/INSTRUCTION:
// a location and an instruction, or (location "") a CASE table's entry.
type examineLine struct {
	location, text string
}

// examineBlock is one EXAMINE/INSTRUCTION command and what it printed,
// with the session's mode and radix at the time.
type examineBlock struct {
	start    string // the range's start, as typed
	symbolic bool
	radix    int
	lines    []examineLine
}

// examineBlocks reads every EXAMINE/INSTRUCTION in a session log.
func examineBlocks(t *testing.T, name string) []examineBlock {
	t.Helper()

	f, err := os.Open(filepath.Join(probe, name+".dlg"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var (
		out      []examineBlock
		cur      *examineBlock
		symbolic = true
		radix    = 16
	)

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, ok := strings.CutPrefix(sc.Text(), "!")
		if !ok {
			continue
		}

		// A command is echoed after one space; output starts in the first
		// column, or (a CASE table's entries) after 16 spaces.
		if strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "  ") {
			cur = nil
			cmd := strings.TrimSpace(line)

			switch {
			case cmd == "SET MODE NOSYMBOLIC":
				symbolic = false
			case cmd == "SET MODE SYMBOLIC":
				symbolic = true
			case cmd == "SET RADIX DECIMAL":
				radix = 10
			case cmd == "CANCEL RADIX":
				radix = 16
			case strings.HasPrefix(cmd, "EXAMINE/INSTRUCTION "):
				start, _, _ := strings.Cut(strings.TrimPrefix(cmd, "EXAMINE/INSTRUCTION "), ":")
				out = append(out, examineBlock{start: start, symbolic: symbolic, radix: radix})
				cur = &out[len(out)-1]
			}

			continue
		}

		if cur == nil || strings.HasPrefix(line, "%") {
			continue
		}

		if strings.HasPrefix(line, strings.Repeat(" ", 16)) {
			cur.lines = append(cur.lines, examineLine{text: strings.TrimSpace(line)})

			continue
		}

		loc, text, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("%s: no location in %q", name, line)
		}

		cur.lines = append(cur.lines, examineLine{location: loc, text: strings.TrimLeft(text, " ")})
	}

	return out
}

// resolve finds an EXAMINE range's start: NAME, MODULE\NAME, NAME+hex,
// %LINE n (in the first module), or MODULE\%LINE n.
func resolve(t *testing.T, p *Program, start string) uint32 {
	t.Helper()

	m := p.Modules[0]
	path := start

	if mod, rest, ok := strings.Cut(start, `\`); ok && strings.HasPrefix(rest, "%LINE") {
		if m, ok = p.ModuleNamed(mod); !ok {
			t.Fatalf("no module in %q", start)
		}

		path = rest
	}

	if n, ok := strings.CutPrefix(path, "%LINE "); ok {
		line, err := strconv.Atoi(n)
		if err != nil {
			t.Fatal(err)
		}

		addr, ok := m.AddressOfLine(line)
		if !ok {
			t.Fatalf("no line %d", line)
		}

		return addr
	}

	var off uint64

	if name, hex, ok := strings.Cut(path, "+"); ok {
		var err error
		if off, err = strconv.ParseUint(hex, 16, 32); err != nil {
			t.Fatal(err)
		}

		path = name
	}

	s, ok := p.Lookup(path)
	if !ok {
		t.Fatalf("no symbol %q", path)
	}

	return s.Value + uint32(off)
}

// TestSymbolicInstructions disassembles every range the probe's sessions
// examined, as the debugger did, and compares each line: its location
// (Symbolize, or the address under SET MODE NOSYMBOLIC) and the
// instruction in the debugger's style with its operands named. This is
// every rule of Symbolize and of disasm's StyleDebugger the sessions
// show: /DEBUG links (DBGDIS, govax's GVDBGDIS, FAILLNK, FORTH) and a
// traceback link (DBGTRC), in both radixes.
func TestSymbolicInstructions(t *testing.T) {
	for _, name := range []string{"dbgdis", "gvdbgdis", "dbgtrc", "faillnk", "forth"} {
		t.Run(name, func(t *testing.T) {
			p := readProgram(t, name)
			mem := loadImageMemory(t, name)
			checked := 0

			for _, b := range examineBlocks(t, name) {
				if b.start == ".PC" {
					continue // wherever the program had stopped
				}

				pc := resolve(t, p, b.start)
				caseTable := uint32(0)

				opts := disasm.Options{Style: disasm.StyleDebugger}
				if b.symbolic {
					opts.Symbolizer = Names{Program: p, Radix: b.radix}
				}

				for i, want := range b.lines {
					if want.location == "" {
						// A CASE table entry: a word displacement from the
						// table's start.
						disp := int16(uint16(mem.ByteAt(pc)) | uint16(mem.ByteAt(pc+1))<<8)
						got := fmt.Sprintf("%08X", caseTable+uint32(int32(disp)))

						if b.symbolic {
							if n, ok := p.Symbolize(caseTable+uint32(int32(disp)), b.radix); ok {
								got = n
							}
						}

						if got != want.text {
							t.Errorf("%08X: case entry %q, want %q", pc, got, want.text)
						}

						pc += 2
						checked++

						continue
					}

					// A range typed as a line starts with the line's
					// name (LineName).
					loc := fmt.Sprintf("%08X", pc)
					if b.symbolic {
						if n, ok := p.LineName(pc, b.radix); ok && i == 0 && strings.Contains(b.start, "%LINE") {
							loc = n
						} else if n, ok := p.Symbolize(pc, b.radix); ok {
							loc = n
						}
					}

					var dec disasm.Decoded

					if r, _, ok := p.RoutineAt(pc); ok && r.Address == pc && !r.NoCall {
						dec = disasm.EntryMask(mem, pc, r.Name)
					} else {
						var err error
						if dec, err = disasm.Disassemble(mem, pc); err != nil {
							t.Fatalf("%08X: %v", pc, err)
						}
					}

					if got := dec.Format(opts); loc != want.location || got != want.text {
						t.Errorf("%08X: %q: %q, want %q: %q", pc, loc, got, want.location, want.text)
					}

					pc += dec.Length
					checked++

					if strings.HasPrefix(dec.Mnemonic, "CASE") {
						caseTable = pc
					}
				}
			}

			if checked == 0 {
				t.Fatal("no lines checked")
			}

			t.Logf("%d lines", checked)
		})
	}
}

// TestSymbolizeRules checks each of Symbolize's rules at an address the
// probe's sessions name (docs/PHASE-41.md, subtask 1's results).
func TestSymbolizeRules(t *testing.T) {
	cases := []struct {
		image string
		addr  uint32
		radix int
		want  string // "" for no name
	}{
		// 1: routines, labels, and data, by path, the module always given.
		{"dbgdis", 0x400, 16, `DBGDIS\START`},
		{"dbgdis", 0x4DD, 16, `DBGDIS\START\LOOP`},
		{"dbgdis", 0x53B, 16, `DBGDIS\LOCALR\JSBRTN`},
		{"dbgdis", 0x200, 16, `DBGDIS\COUNT`},
		{"dbgdis", 0x268, 16, `DBGSUB\SUBDATA`},
		{"forth", 0x4600, 16, "FORTH"},
		{"faillnk", 0x200, 16, "FAILMAIN"},
		// 2: array elements.
		{"dbgdis", 0x204, 16, `DBGDIS\TABLE[0]`},
		{"dbgdis", 0x20C, 16, `DBGDIS\TABLE[2]`},
		{"dbgdis", 0x217, 16, `DBGDIS\BYTES[3]`},
		// 3: lines, at their start and past it.
		{"dbgdis", 0x402, 16, `DBGDIS\START\%LINE 42`},
		{"dbgdis", 0x53E, 16, `DBGDIS\LOCALR\%LINE 110+3`},
		// 4: a routine's code with no line table.
		{"dbgtrc", 0x402, 16, `DBGDIS\START+2`},
		{"dbgtrc", 0x40C, 16, `DBGDIS\START+0C`},
		{"dbgtrc", 0x561, 16, `DBGSUB\SUB2+9`},
		{"trdbgtrc", 0x610, 16, "TRACE+10"},
		{"faillnk", 0x20A, 16, "FAILMAIN+0A"},
		// 5: the globals, passing over strings, psects, and constants
		// in the modules.
		{"dbgdis", 0x250, 16, "GLIMIT+24D"},
		{"dbgdis", 0x250, 10, "GLIMIT+589"},
		{"dbgdis", 0x232, 16, "GLIMIT+22F"},
		{"dbgdis", 0x260, 16, "GLIMIT+25D"},
		{"dbgdis", 0x648, 16, "SUB2+0F0"},
		{"trlnkdbg", 0x400, 16, "LEVEL+3FE"},
		// Nothing names it.
		{"trdbgtrc", 0x400, 16, ""},
		{"dbgtrc", 0x200, 16, ""},
		{"forth", 0x3408, 16, ""},
		{"faillnk", 0, 16, ""},
	}

	programs := map[string]*Program{}

	for _, tc := range cases {
		p, ok := programs[tc.image]
		if !ok {
			p = readProgram(t, tc.image)
			programs[tc.image] = p
		}

		got, ok := p.Symbolize(tc.addr, tc.radix)
		if !ok {
			got = ""
		}

		if got != tc.want {
			t.Errorf("%s: Symbolize(%08X, %d) = %q, want %q", tc.image, tc.addr, tc.radix, got, tc.want)
		}
	}

	// A system service's address is its global's: @#SYS$OPEN.
	forth := programs["forth"]

	open, ok := forth.Globals.Get("SYS$OPEN")
	if !ok {
		t.Fatal("FORTH's GST has no SYS$OPEN")
	}

	if got, _ := forth.Symbolize(open.Value, 16); got != "SYS$OPEN" {
		t.Errorf("SYS$OPEN's address is %q", got)
	}

	// A nil Program names nothing.
	if _, ok := (*Program)(nil).Symbolize(0x200, 16); ok {
		t.Error("a nil Program named an address")
	}
}

// TestConstants checks the constant-name option (Decision 5) on DBGDIS's
// START: a value exactly one constant has is named, and one two share,
// or none has, isn't. Off, the default, the line is the debugger's.
func TestConstants(t *testing.T) {
	p := readProgram(t, "dbgdis")
	mem := loadImageMemory(t, "dbgdis")
	names := Names{Program: p}

	cases := []struct {
		addr     uint32
		debugger string
		named    string
	}{
		// MOVL #LIMIT,R2 (LIMIT = 0A).
		{0x402, "MOVL     S^#0A,R2", `MOVL     S^#DBGDIS\LIMIT,R2`},
		// MOVL #BIG,R3 (BIG = 3E8): an immediate.
		{0x405, "MOVL     I^#000003E8,R3", `MOVL     I^#DBGDIS\BIG,R3`},
		// MOVL #GLIMIT,R0 (GLIMIT = 3).
		{0x413, "MOVL     S^#03,R0", `MOVL     S^#DBGDIS\GLIMIT,R0`},
		// MOVF #1.5,R1: a floating literal is never a constant's.
		{0x416, "MOVF     S^#1.500000,R1", "MOVF     S^#1.500000,R1"},
		// MOVL S^#02,R4: no constant is 2.
		{0x45B, "MOVL     S^#02,R4", "MOVL     S^#02,R4"},
	}

	for _, tc := range cases {
		dec, err := disasm.Disassemble(mem, tc.addr)
		if err != nil {
			t.Fatal(err)
		}

		if got := dec.Format(disasm.Options{Style: disasm.StyleDebugger, Symbolizer: names}); got != tc.debugger {
			t.Errorf("%08X: %q, want %q", tc.addr, got, tc.debugger)
		}

		if got := dec.Format(disasm.Options{Style: disasm.StyleDebugger, Symbolizer: names, Constants: names}); got != tc.named {
			t.Errorf("%08X with constants: %q, want %q", tc.addr, got, tc.named)
		}
	}

	// LIMIT and GLIMIT differ; a module with two constants of one value
	// names neither.
	m := p.Modules[0]
	m.Data = append(m.Data, &Datum{Name: "TEN", Type: dtypeLongword, Kind: Literal, Value: 0x0A})

	if name, ok := p.Constant(0x402, 0x0A); ok {
		t.Errorf("a shared value was named %q", name)
	}
}
