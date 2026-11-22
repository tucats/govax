//go:build ignore

// Command gen writes a probe of VMS MACRO's TIR records (docs/planning/PHASE-111 -
// Fortran 90 milestone.md, subtask 11): where VMS 7.1's MACRO stores a byte of a run of
// immediate data through the stack (STA_UB, STO_B), as it did in three places of fcalc's
// objects (FCFUNCS twice, FCMATRIX once) where govax's MACRO stores the run whole; and how
// it writes repetition factors and string directives with nothing after them. Run it
// from the repository root:
//
//	go run testdata/tir111/gen.go
//
// Each variant is a module of its own, so each object's record packing starts afresh.
// The sources are fcalc's .MAR files as round 2 brought them back (testdata/f90r/vax/fcalc).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	dir    = "testdata/tir111"
	volume = "testdata/disks/tir111-exchange.dsk"
	device = "DUA5:" // DUA2 is the author's XFER volume, mounted at startup
	fcalc  = "testdata/f90r/vax/fcalc"
)

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// lines are a file's lines, 1 being the first.
func lines(path string) []string {
	text, err := os.ReadFile(path)
	check(err)

	return strings.Split(strings.TrimRight(string(text), "\n"), "\n")
}

// span is lines first to last of a file, inclusive.
func span(all []string, first, last int) []string {
	return append([]string(nil), all[first-1:last]...)
}

// terms are a .ASCII line's operand terms, each a delimited string or <n>, or nil if
// the operands aren't only such terms.
func terms(operands string) []string {
	var out []string

	for i := 0; i < len(operands); {
		var end int

		if operands[i] == '<' {
			end = strings.IndexByte(operands[i:], '>')
		} else {
			end = strings.IndexByte(operands[i+1:], operands[i])
			if end >= 0 {
				end++
			}
		}

		if end < 0 {
			return nil
		}

		out = append(out, operands[i:i+end+1])
		i += end + 1
	}

	return out
}

// splitASCII makes each .ASCII line of the table two: its terms split at the one
// nearest the middle.
func splitASCII(src []string, first, last int) []string {
	var out []string

	for i, line := range src {
		n := i + 1
		if n < first || n > last || !strings.HasPrefix(line, "\t.ASCII\t") {
			out = append(out, line)

			continue
		}

		operands := strings.TrimPrefix(line, "\t.ASCII\t")
		terms := terms(operands)

		if len(terms) < 2 {
			out = append(out, line) // not a line of plain terms: kept

			continue
		}

		half := len(terms) / 2
		out = append(out, "\t.ASCII\t"+strings.Join(terms[:half], ""), "\t.ASCII\t"+strings.Join(terms[half:], ""))
	}

	return out
}

// ones makes each <0> in the table <1>.
func ones(src []string, first, last int) []string {
	out := append([]string(nil), src...)
	for i := first - 1; i < last; i++ {
		out[i] = strings.ReplaceAll(out[i], "<0>", "<1>")
	}

	return out
}

// padded inserts .BLKB n after line at (a .PSECT line), shifting what follows.
func padded(src []string, at, n int) []string {
	out := append([]string(nil), src[:at]...)
	out = append(out, fmt.Sprintf("\t.BLKB\t%d", n))

	return append(out, src[at:]...)
}

type module struct {
	name, why string
	text      []string
}

func main() {
	funcs := lines(filepath.Join(fcalc, "fcfuncs.mar"))
	matrix := lines(filepath.Join(fcalc, "fcmatrix.mar"))

	// FCFUNCS to the end of BUILTIN_INDEX's data: its table, $K1, is lines 211 to 241,
	// at $PDATA 0, after the .PSECT at line 210. VMS stored $PDATA+^X216 by STO_B.
	funcsA := span(funcs, 1, 248)
	// FCMATRIX to the end of APPLY_MATRIX's $K1, lines 346 to 362, at $PDATA ^X418; VMS
	// stored $PDATA+^X551 by STO_B.
	matrixA := span(matrix, 1, 365)

	var mods []module

	add := func(name, why string, text []string) {
		mods = append(mods, module{name, why, text})
	}

	add("TIRA", "FCFUNCS to line 248: does VMS store ^X216 by STO_B again?", funcsA)
	add("TIRB", "FCFUNCS's $PDATA alone, lines 210 to 248: without the psects before it", span(funcs, 210, 248))

	for _, n := range []int{1, 2, 3, 4, 8} {
		add(fmt.Sprintf("TIRC%d", n), fmt.Sprintf("TIRA with .BLKB %d before the table: the bytes %d further on", n, n), padded(funcsA, 210, n))
	}

	add("TIRD", "TIRA with each of the table's .ASCII lines made two", splitASCII(funcsA, 211, 241))
	add("TIRE", "TIRA with every <0> of the table <1>", ones(funcsA, 211, 241))
	add("TIRF", "FCMATRIX to line 365: does VMS store ^X551 by STO_B again?", matrixA)
	add("TIRG", "FCMATRIX's last $PDATA alone, lines 341 to 365", span(matrix, 341, 365))

	add("TIRR", "repetition factors", strings.Split(`	.PSECT	R1,LONG
	.BYTE	7[3]
	.PSECT	R2,LONG
	.WORD	7[3]
	.PSECT	R3,LONG
	.LONG	7[1]
	.PSECT	R4,LONG
	.LONG	300[2]
	.PSECT	R5,LONG
	.LONG	-1[2]
	.PSECT	R6,LONG
	.LONG	70000[2]
	.PSECT	R7,LONG
Y = 5
	.LONG	Y[2]
	.PSECT	R8,LONG
	.LONG	Z[2]
Z = 6
	.PSECT	R9,LONG
	.BYTE	0[5]
	.PSECT	R10,LONG
	.WORD	0[2],1,0[3]
	.PSECT	R11,LONG
	.LONG	1,0[2],2
	.PSECT	R12,LONG
	.LONG	R1[2]`, "\n"))

	add("TIRX", "string directives with nothing after them, and empty strings", strings.Split(`	.PSECT	X1,LONG
	.ASCIZ
	.PSECT	X2,LONG
	.ASCIC
	.PSECT	X3,LONG
	.ASCID
	.PSECT	X4,LONG
	.ASCIZ	//
	.PSECT	X5,LONG
	.ASCIC	//
	.PSECT	X6,LONG
	.ASCID	//`, "\n"))

	var ex, com, out strings.Builder

	fmt.Fprintf(&ex, `! EXCHANGE.CMD - builds the TIR probe's exchange volume with govax. Written by
! testdata/tir111/gen.go. Run from the repository root:
!
!     govax console < testdata/tir111/exchange.cmd
!
INITIALIZE/CONTAINER "%[1]s" /SIZE=20000 TIR111
MOUNT/WRITE %[2]s "%[1]s"
SET DEFAULT %[2]s[000000]
COPY/QUIET "%[3]s/tir111.com"/HOST TIR111.COM
`, volume, device, dir)

	fmt.Fprint(&com, `$ ! TIR111.COM - VMS MACRO's TIR records, a module per question (testdata/tir111's
$ ! README). Written by testdata/tir111/gen.go. Run it with the exchange volume's
$ ! [000000] the default directory:
$ !
$ !     @TIR111/OUTPUT=TIR111.LOG
$ !
$ SET NOON
`)

	fmt.Fprintf(&out, `! COPYOUT.CMD - copies VMS's objects and log off the exchange volume. Written by
! testdata/tir111/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/tir111/vax
!     govax console < testdata/tir111/copyout.cmd
!
MOUNT/NOWRITE %[1]s "%[2]s"
COPY/QUIET %[1]s[000000]TIR111.LOG "%[3]s/vax/tir111.log"/HOST
`, device, volume, dir)

	var table strings.Builder

	for _, m := range mods {
		text := strings.Join(m.text, "\n")
		if !strings.Contains(text, "\t.END") {
			text += "\n\t.END"
		}

		text = fmt.Sprintf("; %s.MAR - %s (written by testdata/tir111/gen.go)\n%s\n", m.name, m.why, text)
		check(os.WriteFile(filepath.Join(dir, strings.ToLower(m.name)+".mar"), []byte(text), 0o644))

		fmt.Fprintf(&ex, "COPY/QUIET \"%s/%s.mar\"/HOST %s.MAR\n", dir, strings.ToLower(m.name), m.name)
		fmt.Fprintf(&com, "$ WRITE SYS$OUTPUT \"==== %[1]s\"\n$ MACRO/NOLIST %[1]s\n$ SHOW SYMBOL $STATUS\n", m.name)
		fmt.Fprintf(&out, "COPY/QUIET %[1]s[000000]%[2]s.OBJ \"%[3]s/vax/%[4]s.obj\"/HOST\n", device, m.name, dir, strings.ToLower(m.name))
		fmt.Fprintf(&table, "| `%s` | %s |\n", strings.ToLower(m.name)+".mar", m.why)
	}

	fmt.Fprintf(&ex, "SET DEFAULT DUA2:[000000]\nDISMOUNT %s\nEXIT\n", device)
	fmt.Fprint(&com, "$ WRITE SYS$OUTPUT \"==== END\"\n$ EXIT\n")
	fmt.Fprintf(&out, "DISMOUNT %s\nEXIT\n", device)

	check(os.WriteFile(filepath.Join(dir, "exchange.cmd"), []byte(ex.String()), 0o644))
	check(os.WriteFile(filepath.Join(dir, "tir111.com"), []byte(com.String()), 0o644))
	check(os.WriteFile(filepath.Join(dir, "copyout.cmd"), []byte(out.String()), 0o644))
	check(os.WriteFile(filepath.Join(dir, "modules.md"), []byte("| Module | Question |\n| --- | --- |\n"+table.String()), 0o644))
}
