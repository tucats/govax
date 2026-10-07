//go:build ignore

// dumpcode disassembles the CODE program section of an object module from
// the module's ANALYZE/OBJECT report (the .anl files in vax/): the
// instructions real MACRO made for a probe, one group per macro call,
// with each relocated longword annotated. It is how govax's macros are
// compared with real MACRO's by eye before the object comparison test
// (internal/asm's TestServiceMacroObjects) exists. Usage, from the
// repository root:
//
//	go run testdata/mp/macros/dumpcode.go testdata/mp/macros/vax/svc_wake.anl
//
// A group ends at each CALLS, CALLG, or RET. With -calls, the stream is
// split instead at the markers gen.go puts after each call. "reloc" lists the relocations
// within the instruction: the symbol, or PSECT+offset, the longword is
// relocated to, and PIC for a PC-relative one.
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/disasm"
)

type item struct {
	sym   string
	psect int
	value int
	isSym bool
}

type reloc struct {
	off  int
	size int
	pic  bool
	to   item
}

var (
	storeRE = regexp.MustCompile(`Store Immediate, (\d+) bytes?:`)
	rowRE   = regexp.MustCompile(`^\t\t((?: [0-9A-F]{2}| {3})+)\|`)
	cmdRE   = regexp.MustCompile(`^\t\d+\)\s+TIR\$C_(\w+)`)
	symRE   = regexp.MustCompile(`symbol: "([^"]*)"`)
	psectRE = regexp.MustCompile(`psect: (\d+)`)
	valueRE = regexp.MustCompile(`value: (-?\d+)`)
	codeRE  = regexp.MustCompile(`<-- psect (\d+)`)
)

func main() {
	// -longs FROM prints the stream from offset FROM (hex) as longwords,
	// for the argument lists the long macro forms (no _S) lay down.
	longsFrom := -1
	callsFile := ""
	args := os.Args[1:]

	// -calls FILE.calls splits the stream at gen.go's markers and shows
	// each call (a line of FILE.calls) above its code.
	if len(args) == 3 && args[0] == "-calls" {
		callsFile, args = args[1], args[2:]
	}

	if len(args) == 3 && args[0] == "-longs" {
		v, _ := strconv.ParseInt(args[1], 16, 32)
		longsFrom, args = int(v), args[2:]
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: dumpcode [-longs FROM] FILE.anl")
		os.Exit(2)
	}

	f, err := os.Open(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	var (
		lines   []string
		scanner = bufio.NewScanner(f)
	)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// The CODE psect's number: the one named CODE in the GSD.
	codePsect := 2
	lastPsect := 0

	for _, l := range lines {
		if m := codeRE.FindStringSubmatch(l); m != nil {
			lastPsect, _ = strconv.Atoi(m[1])
		}

		if strings.Contains(l, `symbol: "CODE"`) && lastPsect > 0 {
			codePsect = lastPsect
		}
	}

	var (
		code    = map[int]byte{}
		relocs  []reloc
		stack   []item
		curP    = -1
		off     int
		maxOff  int
		pending []byte
		want    int
		inStore bool
	)

	flush := func() {
		if inStore && curP == codePsect {
			for i, b := range pending {
				code[off+i] = b
			}

			off += len(pending)
			maxOff = max(maxOff, off)
		} else if inStore {
			off += len(pending)
		}

		pending, inStore = nil, false
	}

	pop := func() item {
		if len(stack) == 0 {
			return item{}
		}

		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		return v
	}

	for i := 0; i < len(lines); i++ {
		l := lines[i]

		if m := storeRE.FindStringSubmatch(l); m != nil {
			flush()

			want, _ = strconv.Atoi(m[1])
			inStore = true

			continue
		}

		if inStore {
			if m := rowRE.FindStringSubmatch(l); m != nil {
				fields := strings.Fields(m[1])
				for j := len(fields) - 1; j >= 0; j-- {
					b, _ := strconv.ParseUint(fields[j], 16, 8)
					pending = append(pending, byte(b))
				}

				if len(pending) >= want {
					flush()
				}
			}

			continue
		}

		m := cmdRE.FindStringSubmatch(l)
		if m == nil {
			continue
		}

		cmd := m[1]

		// The command's detail lines follow, up to a blank line.
		var sym string

		psect, value := -1, 0

		for j := i + 1; j < len(lines) && strings.TrimSpace(lines[j]) != ""; j++ {
			if s := symRE.FindStringSubmatch(lines[j]); s != nil {
				sym = s[1]
			}

			if s := psectRE.FindStringSubmatch(lines[j]); s != nil {
				psect, _ = strconv.Atoi(s[1])
			}

			if s := valueRE.FindStringSubmatch(lines[j]); s != nil {
				value, _ = strconv.Atoi(s[1])
			}
		}

		switch cmd {
		case "STA_GBL":
			stack = append(stack, item{sym: sym, isSym: true})
		case "STA_PB":
			stack = append(stack, item{psect: psect, value: value})
		case "STA_UB", "STA_UW", "STA_LW":
			stack = append(stack, item{value: value})
		case "CTL_SETRB":
			v := pop()
			curP, off = v.psect, v.value
		case "STO_LD", "STO_PICR", "STO_W", "STO_B", "STO_PIDR", "STO_L", "STO_LW":
			to := pop()
			size := map[string]int{"STO_LD": 4, "STO_PICR": 5, "STO_W": 2, "STO_B": 1, "STO_PIDR": 4, "STO_L": 4, "STO_LW": 4}[cmd]

			if curP == codePsect {
				// STO_PICR stores a whole PC-relative operand: the
				// mode byte 0xEF, then the longword displacement.
				at := off
				if cmd == "STO_PICR" {
					code[off] = 0xEF
					at++
				}

				relocs = append(relocs, reloc{off: at, size: size, pic: strings.HasPrefix(cmd, "STO_PI"), to: to})
				maxOff = max(maxOff, off+size)
			}

			off += size
		}
	}

	flush()

	// The bytes, with the relocated ones as zeros.
	bytes := make([]byte, maxOff)
	for o, b := range code {
		if o < len(bytes) {
			bytes[o] = b
		}
	}

	sort.Slice(relocs, func(i, j int) bool { return relocs[i].off < relocs[j].off })

	if longsFrom >= 0 {
		for o := longsFrom; o+4 <= len(bytes); o += 4 {
			v := uint32(bytes[o]) | uint32(bytes[o+1])<<8 | uint32(bytes[o+2])<<16 | uint32(bytes[o+3])<<24
			note := ""

			for _, r := range relocs {
				if r.off >= o && r.off < o+4 {
					target := fmt.Sprintf("psect %d+%d", r.to.psect, r.to.value)
					if r.to.isSym {
						target = r.to.sym
					}

					note = "    ; reloc " + target
					if r.pic {
						note += " PIC"
					}
				}
			}

			fmt.Printf("%04X  %08X%s\n", o, v, note)
		}

		return
	}

	reader := disasm.SliceReader(bytes)
	pc := uint32(0)

	if callsFile != "" {
		chunked(bytes, relocs, callsFile)

		return
	}

	mask := disasm.EntryMask(reader, 0, "")
	fmt.Printf("%04X  %s\n", 0, mask.String())

	pc += mask.Length

	for int(pc) < len(bytes) {
		dec, err := disasm.Disassemble(reader, pc)
		if err != nil {
			fmt.Printf("%04X  ?? %v\n", pc, err)

			break
		}

		var notes []string

		for _, r := range relocs {
			if r.off >= int(pc) && r.off < int(pc+dec.Length) {
				target := fmt.Sprintf("psect %d+%d", r.to.psect, r.to.value)
				if r.to.isSym {
					target = r.to.sym
				}

				kind := ""
				if r.pic {
					kind = " PIC"
				}

				notes = append(notes, fmt.Sprintf("@%d:%s%s", r.off-int(pc), target, kind))
			}
		}

		text := dec.String()
		if len(notes) > 0 {
			text += "    ; reloc " + strings.Join(notes, ", ")
		}

		fmt.Printf("%04X  %s\n", pc, text)

		if dec.Mnemonic == "CALLS" || dec.Mnemonic == "CALLG" || dec.Mnemonic == "RET" {
			fmt.Println()
		}

		pc += dec.Length
	}
}

// chunked prints the code between gen.go's markers, one chunk per call,
// each under the text of its call: disassembled for a _S call, as longwords
// for the argument-list (no suffix) form.
func chunked(bytes []byte, relocs []reloc, callsFile string) {
	text, err := os.ReadFile(callsFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	calls := strings.Split(strings.TrimRight(string(text), "\n"), "\n")

	find := func(n int, from int) int {
		want := []byte{byte(n), byte(n >> 8), 0x7A, 0x7A}

		for o := from; o+4 <= len(bytes); o++ {
			if bytes[o] == want[0] && bytes[o+1] == want[1] && bytes[o+2] == want[2] && bytes[o+3] == want[3] {
				return o
			}
		}

		return -1
	}

	start := 2 // after the entry mask

	for n := 1; n <= len(calls); n++ {
		end := find(n, start)
		if end < 0 {
			fmt.Printf("=== %d: %s\n  (marker not found)\n", n, calls[n-1])

			break
		}

		fmt.Printf("=== %d: %s\n", n, calls[n-1])

		chunk := bytes[start:end]
		first := strings.Fields(calls[n-1] + " x")[0]
		isShort := strings.HasSuffix(first, "_S") || strings.HasSuffix(first, "_G")

		if isShort {
			reader := disasm.SliceReader(bytes[:end])
			pc := uint32(start)

			for int(pc) < end {
				dec, err := disasm.Disassemble(reader, pc)
				if err != nil || int(pc+dec.Length) > end {
					fmt.Printf("  ?? bytes % X\n", bytes[pc:end])

					break
				}

				fmt.Printf("  %s%s\n", dec.String(), notesFor(relocs, int(pc), int(dec.Length)))

				pc += dec.Length
			}
		} else {
			for o := 0; o < len(chunk); o += 4 {
				var v uint32

				for i := 0; i < 4 && o+i < len(chunk); i++ {
					v |= uint32(chunk[o+i]) << (8 * i)
				}

				fmt.Printf("  .LONG %08X%s\n", v, notesFor(relocs, start+o, 4))
			}
		}

		start = end + 4
	}
}

// notesFor lists the relocations within [off, off+n).
func notesFor(relocs []reloc, off, n int) string {
	var notes []string

	for _, r := range relocs {
		if r.off >= off && r.off < off+n {
			target := fmt.Sprintf("psect %d+%d", r.to.psect, r.to.value)
			if r.to.isSym {
				target = r.to.sym
			}

			if r.pic {
				target += " PIC"
			}

			notes = append(notes, fmt.Sprintf("@%d:%s", r.off-off, target))
		}
	}

	if len(notes) == 0 {
		return ""
	}

	return "    ; reloc " + strings.Join(notes, ", ")
}
