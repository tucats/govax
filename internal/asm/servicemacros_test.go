package asm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/obj"
)

// The Phase 45 system service macro oracle (docs/PHASE-45.md,
// testdata/mp/macros): programs that call each service's short-form macro
// in many ways, and the objects real VAX MACRO made of them on VMS. govax's
// own macros (internal/bootdata/files/starlet.mar), written from the System
// Services Reference Manual and these objects, must make the same code for
// every call real MACRO took without an error.
var serviceDir = filepath.Join("..", "..", "testdata", "mp", "macros")

// serviceReloc is a relocated field in a CODE psect stream: where it is,
// how long, and what it refers to.
type serviceReloc struct {
	off, size int
	pic       bool
	target    string
}

// serviceCodeStream rebuilds the bytes of m's CODE program section, and its
// relocated fields, from the TIR commands. A relocated field is zero in
// the bytes. STO_PICR stores a whole PC-relative operand: the mode byte
// 0xEF, then the displacement.
func serviceCodeStream(t *testing.T, m *obj.Module) ([]byte, []serviceReloc) {
	t.Helper()

	type item struct {
		sym   string
		isSym bool
		psect int
		value uint32
	}

	var (
		psects   []string
		code     = map[int]byte{}
		relocs   []serviceReloc
		stack    []item
		cur, off = -1, 0
		size     = 0
	)

	// Program section numbers are the order of definition from 0, the
	// absolute section (named ".  ABS  .").
	codePsect := -1

	for _, rec := range m.Records {
		if g, ok := rec.(*obj.GSD); ok {
			for _, sub := range g.Subrecords {
				if p, ok := sub.(*obj.Psect); ok {
					psects = append(psects, p.Name)
					if p.Name == "CODE" {
						codePsect = len(psects) - 1
					}
				}
			}
		}
	}

	if codePsect < 0 {
		t.Fatal("no CODE program section")
	}

	op := func(name string) obj.Op {
		o, ok := obj.OpByName(name)
		if !ok {
			t.Fatalf("no TIR command %s", name)
		}

		return o
	}

	var (
		staGbl, staPb, staPl = op("STA_GBL"), op("STA_PB"), op("STA_PL")
		staUb, staLw         = op("STA_UB"), op("STA_LW")
		setrb                = op("CTL_SETRB")
		stores               = map[obj.Op]int{
			op("STO_LD"): 4, op("STO_PICR"): 5, op("STO_PIDR"): 4, op("STO_L"): 4, op("STO_W"): 2, op("STO_B"): 1,
		}
		picr = op("STO_PICR")
		pidr = op("STO_PIDR")
	)

	pop := func() item {
		if len(stack) == 0 {
			return item{}
		}

		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		return v
	}

	for _, rec := range m.Records {
		tir, ok := rec.(*obj.TIR)
		if !ok || tir.Type != obj.RecTIR {
			continue
		}

		for _, c := range tir.Commands {
			switch {
			case c.Op == obj.OpStoreImmediate:
				if cur == codePsect {
					for i, b := range c.Data {
						code[off+i] = b
					}

					size = max(size, off+len(c.Data))
				}

				off += len(c.Data)

			case c.Op == staGbl:
				stack = append(stack, item{sym: c.Name, isSym: true})
			case c.Op == staPb || c.Op == staPl:
				stack = append(stack, item{psect: int(c.Psect), value: c.Value})
			case c.Op == staUb || c.Op == staLw:
				stack = append(stack, item{value: c.Value})
			case c.Op == setrb:
				v := pop()
				cur, off = v.psect, int(v.value)
			default:
				n, isStore := stores[c.Op]
				if !isStore {
					continue
				}

				to := pop()

				if cur == codePsect {
					at := off
					if c.Op == picr {
						code[off] = 0xEF
						at++
					}

					target := fmt.Sprintf("P%d+%d", to.psect, to.value)
					if to.isSym {
						target = to.sym
					}

					relocs = append(relocs, serviceReloc{off: at, size: n, pic: c.Op == picr || c.Op == pidr, target: target})
					size = max(size, off+n)
				}

				off += n
			}
		}
	}

	bytes := make([]byte, size)
	for o, b := range code {
		bytes[o] = b
	}

	sort.Slice(relocs, func(i, j int) bool { return relocs[i].off < relocs[j].off })

	return bytes, relocs
}

// callChunks splits a CODE stream at gen.go's markers (.LONG ^X7A7Axxxx,
// xxxx the call's number), returning each marked call's text: its
// instructions, one per line, with the relocations in them named and the
// PC-relative displacements masked (they depend on where the code is).
// ok[n] is false for a chunk that doesn't disassemble to a complete call
// of the short form's instructions. Calls with no marker are missing.
func callChunks(bytes []byte, relocs []serviceReloc, count int) (text map[int]string, ok map[int]bool) {
	text, ok = map[int]string{}, map[int]bool{}

	find := func(n, from int) int {
		for o := from; o+4 <= len(bytes); o++ {
			if bytes[o] == byte(n) && bytes[o+1] == byte(n>>8) && bytes[o+2] == 0x7A && bytes[o+3] == 0x7A {
				return o
			}
		}

		return -1
	}

	allowed := map[string]bool{"PUSHL": true, "PUSHAB": true, "PUSHAW": true, "PUSHAL": true, "PUSHAQ": true,
		"MOVZWL": true, "CLRQ": true, "CALLS": true}
	mask := regexp.MustCompile(`L\^\^X[0-9A-F]{8}`)
	start := 2

	for n := 1; n <= count; n++ {
		end := find(n, start)
		if end < 0 {
			continue // a call the module doesn't have
		}

		var (
			lines    []string
			complete = true
			last     string
		)

		reader := disasm.SliceReader(bytes[:end])

		for pc := uint32(start); int(pc) < end; {
			dec, err := disasm.Disassemble(reader, pc)
			if err != nil || int(pc+dec.Length) > end || !allowed[dec.Mnemonic] {
				complete = false

				break
			}

			line := mask.ReplaceAllString(dec.String(), "L^")

			for _, r := range relocs {
				if r.off >= int(pc) && r.off < int(pc+dec.Length) {
					line += fmt.Sprintf(" [@%d %s]", r.off-int(pc), r.target)
				}
			}

			lines = append(lines, line)
			last = dec.Mnemonic
			pc += dec.Length
		}

		text[n] = strings.Join(lines, "\n")
		ok[n] = complete && last == "CALLS"
		start = end + 4
	}

	return text, ok
}

var (
	shortCallRE = regexp.MustCompile(`^\t\$\w+_S(\t|$)`)
	markerRE    = regexp.MustCompile(`^\t\.LONG\t\^X7A7A([0-9A-F]{4})$`)
)

// TestServiceMacroObjects compares, for every probe, the code of each
// short-form call real MACRO made without an error with the code govax's
// macros make for the same call.
func TestServiceMacroObjects(t *testing.T) {
	probes, err := filepath.Glob(filepath.Join(serviceDir, "svc_*.mar"))
	if err != nil || len(probes) == 0 {
		t.Fatalf("no probes: %v", err)
	}

	for _, path := range probes {
		name := strings.TrimSuffix(filepath.Base(path), ".mar")

		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			callList, err := os.ReadFile(filepath.Join(serviceDir, name+".calls"))
			if err != nil {
				t.Fatal(err)
			}

			calls := strings.Split(strings.TrimRight(string(callList), "\n"), "\n")

			real := readObjectFile(t, filepath.Join(serviceDir, "vax", name+".obj"))
			realBytes, realRelocs := serviceCodeStream(t, real)
			realText, realOK := callChunks(realBytes, realRelocs, len(calls))

			// The probe without the calls real MACRO had an error for and
			// the argument-list forms, which govax's macros don't have. A
			// call is a line starting with a tab and "$"; the marker line
			// after it goes with it.
			var (
				kept   []string
				keep   = map[int]bool{}
				number = 0
				drop   = false
			)

			for _, l := range strings.Split(string(src), "\n") {
				switch {
				case strings.HasPrefix(l, "\t$"):
					number++

					drop = !(shortCallRE.MatchString(l) && realOK[number])
					if !drop {
						keep[number] = true
					}

					if drop {
						continue
					}

				case markerRE.MatchString(l):
					if drop {
						drop = false

						continue
					}
				}

				kept = append(kept, l)
			}

			if len(keep) == 0 {
				t.Fatal("no call to compare")
			}

			a := macroAssembler()
			a.SetMacroLibraries(govaxStarlet(t))

			if _, err := a.Assemble(strings.Join(kept, "\n")); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			ours := objectLike(t, a, real)
			ourBytes, ourRelocs := serviceCodeStream(t, ours)
			ourText, _ := callChunks(ourBytes, ourRelocs, len(calls))

			bad := 0

			for n := 1; n <= len(calls); n++ {
				if !keep[n] {
					continue
				}

				if ourText[n] != realText[n] {
					bad++

					if bad <= 5 {
						t.Errorf("call %d, %s:\nmacros made:\n%s\nreal MACRO made:\n%s", n, calls[n-1], ourText[n], realText[n])
					}
				}
			}

			if bad > 5 {
				t.Errorf("%d more calls differ", bad-5)
			}

			t.Logf("%d of %d calls compared", len(keep), len(calls))
		})
	}
}
