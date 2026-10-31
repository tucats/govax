//go:build ignore

// gen.go writes the Phase 80 probe of FORRTL's formatted I/O (docs/PHASE-80 -
// FORTRAN formatted IO.md, subtask 2). VMS's FORRTL is the requirement for
// FTNRTL: the LRM leaves many run-time rules open, so a MACRO program calls
// FORRTL's statement routines, case by case, and records what they do. Only
// what FORRTL does is used, never its code. Run this from the repository root:
//
//	go run testdata/for80/gen.go
//
// It writes
//
//	for80.mar    the cases
//	for80d.mar   prints a file's records between bars, with their control bytes
//	for80e.mar   conditions with no handler, to show VMS's messages and dumps
//	for80t.mar   carriage control at a real terminal (run by hand)
//	for80.opt    the link options
//	for80r.com   runs FOR80, with the data its unit 5 cases read
//	for80.com    builds and runs everything
//	exchange.cmd, copyout.cmd
//
// A case is a routine of its own under a condition handler, HND. The handler
// records each condition, continues from one below severe (the statement goes
// on, as FORRTL's does for OUTCONERR), and unwinds to the driver from a severe
// one, so a case can't stop the program.
//
// Everything is written through unit 6, so a case's own records and the
// driver's lines are one stream in order. FOR006 names a file, so its records
// keep their first bytes, which a terminal or a log would take as carriage
// control, and FOR80D prints each between bars. A case is
//
//	#n  format or other title         the driver's title, format (A)
//	... the records the case wrote
//	=n r=x e=n x x u x                FOR$IO_END's R0, then FOR$ERRSNS's five values
//	=c cond n=x a=x x                 per condition: the signal array's size and first
//	                                  two arguments (an address, or a PC, as 0)
//	=u                                the handler unwound
//	 v0=x x  or  v0=|text|            what an input case stored, slots preset to 55
//
// The FORMAT text is passed to FORRTL as it stands: FOR$WRITE_SO takes a run-time
// format's address, the text ending in a NUL. So the syntax cases are held to the
// same parser a run-time format gets. FORTRAN's FORMAT statements go through the
// compiler's own check (Decision 3), which is held to the same answers.
//
// The table is plain data so that a later subtask can write the same cases as
// FORTRAN statements (Decision 2) and hold FTNRTL to VMS's answers.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/for80"

// An item is one element of an I/O list. Its kind names FORRTL's entry:
//
//	L      FOR$IO_L_V, by value
//	l w b  INTEGER*4, *2, *1 by reference (FOR$IO_L_R, _W_R, _B_R)
//	lu wu  LOGICAL*4, *2 by reference (FOR$IO_LU_R, _WU_R)
//	f d g h  REAL*4, *8 in D, in G, REAL*16 by reference
//	fc dc gc  COMPLEX by reference, val "re,im"
//	t      CHARACTER, by descriptor (FOR$IO_T_DS)
//
// An input item's val is empty, except t's, its length in decimal.
type item struct{ kind, val string }

func L(v int) item          { return item{"L", fmt.Sprint(v)} }
func I(v int) item          { return item{"l", fmt.Sprint(v)} }
func W(v int) item          { return item{"w", fmt.Sprint(v)} }
func B(v int) item          { return item{"b", fmt.Sprint(v)} }
func LU(v int) item         { return item{"lu", fmt.Sprint(v)} }
func WU(v int) item         { return item{"wu", fmt.Sprint(v)} }
func F(v string) item       { return item{"f", v} }
func D(v string) item       { return item{"d", v} }
func G(v string) item       { return item{"g", v} }
func H(v string) item       { return item{"h", v} }
func FC(re, im string) item { return item{"fc", re + "," + im} }
func DC(re, im string) item { return item{"dc", re + "," + im} }
func GC(re, im string) item { return item{"gc", re + "," + im} }
func T(v string) item       { return item{"t", v} }

// Input items, into slots.
var (
	iL  = item{kind: "l"}
	iW  = item{kind: "w"}
	iB  = item{kind: "b"}
	iLU = item{kind: "lu"}
	iWU = item{kind: "wu"}
	iF  = item{kind: "f"}
	iD  = item{kind: "d"}
	iG  = item{kind: "g"}
	iH  = item{kind: "h"}
	iFC = item{kind: "fc"}
	iDC = item{kind: "dc"}
)

func iT(n int) item { return item{"t", fmt.Sprint(n)} }

// A kase is one case. op says how its statement begins:
//
//	out   FOR$WRITE_SO(unit, format)
//	list  FOR$WRITE_SL(unit)
//	rd    FOR$READ_IO(internal record text, format)
//	rd5   FOR$READ_SO(5, format), lines being the case's records on SYS$INPUT
//	rl5   FOR$READ_SL(5)
//	ril   FOR$READ_IL(internal record text)
//	wi    FOR$WRITE_IO(internal record of size bytes, format)
//	wil   FOR$WRITE_IL(internal record of size bytes)
//	wia   FOR$WRITE_IO to an array of 3 records of size bytes
//	ria   FOR$READ_IO from an array of text's records of size bytes
//	enc   FOR$ENCODE_MO(size, format, buffer)
//	dec   FOR$DECODE_MO(size, format, text)
//	raw   code and data as given
type kase struct {
	op     string
	unit   int
	format string
	items  []item
	text   string
	size   int
	lines  []string
	title  string
	code   string
	data   string
	show   []string // extra code after the result line
}

var (
	cases    []kase
	sections []string // a title line before the case with the same index
	secAt    = map[int]string{}
)

func section(s string) { secAt[len(cases)] = s }

func out(format string, items ...item) {
	cases = append(cases, kase{op: "out", unit: 6, format: format, items: items})
}
func outU(unit int, format string, items ...item) {
	cases = append(cases, kase{op: "out", unit: unit, format: format, items: items})
}
func list(items ...item) { cases = append(cases, kase{op: "list", unit: 6, items: items}) }
func rd(format, text string, items ...item) {
	cases = append(cases, kase{op: "rd", format: format, text: text, items: items})
}
func rd5(format string, lines []string, items ...item) {
	cases = append(cases, kase{op: "rd5", unit: 5, format: format, lines: lines, items: items})
}
func rl5(lines []string, items ...item) {
	cases = append(cases, kase{op: "rl5", unit: 5, lines: lines, items: items})
}
func ril(text string, items ...item) {
	cases = append(cases, kase{op: "ril", text: text, items: items})
}
func wi(size int, format string, items ...item) {
	cases = append(cases, kase{op: "wi", size: size, format: format, items: items})
}
func wil(size int, items ...item) { cases = append(cases, kase{op: "wil", size: size, items: items}) }
func wia(size int, format string, items ...item) {
	cases = append(cases, kase{op: "wia", size: size, format: format, items: items})
}
func ria(size int, text, format string, items ...item) {
	cases = append(cases, kase{op: "ria", size: size, text: text, format: format, items: items})
}
func enc(size int, format string, items ...item) {
	cases = append(cases, kase{op: "enc", size: size, format: format, items: items})
}
func dec(size int, text, format string, items ...item) {
	cases = append(cases, kase{op: "dec", size: size, text: text, format: format, items: items})
}

// table is every case, by subject. The sections follow Part A of the phase doc.
func table() {
	section("I, O, Z: the manual's cases")
	for _, c := range []struct {
		f string
		v item
	}{
		{"I3", I(284)}, {"I4", I(-284)}, {"I4", I(0)}, {"I5", I(174)}, {"I2", I(3244)}, {"I3", I(-473)},
		{"I4.0", I(0)}, {"I4.2", I(1)}, {"I4.4", I(1)}, {"I3.4", I(1)}, {"I1", I(0)},
		{"I6", W(-7)}, {"I6", B(-7)}, {"I6", WU(1)}, {"I6", LU(1)},
		{"O6", I(32767)}, {"O6", W(-32767)}, {"O2", I(14261)}, {"O4", I(27)}, {"O4.2", I(7)}, {"O4.4", I(7)},
		{"O8", F("10.5")}, {"O12", I(-1)}, {"O6", I(-1)}, {"O4.0", I(0)}, {"O4", B(-1)},
		{"Z4", I(32767)}, {"Z5", W(-32767)}, {"Z2", I(16)}, {"Z4", F("-10.5")}, {"Z3.3", I(2708)}, {"Z6.4", I(2708)},
		{"Z9", I(-1)}, {"Z4", I(-1)}, {"Z8", F("1.0")}, {"O12", F("1.0")}, {"Z16", D("1.0E0")}, {"Z4", B(-1)},
		{"I5", F("3.25")}, {"I5", T("HI")}, {"I5", D("3.25E0")}, {"O5", T("HI")}, {"Z5", T("HI")},
	} {
		out("(1X,"+c.f+")", c.v)
	}

	section("F: the manual's cases, and the edges of a field")
	for _, c := range []struct{ f, v string }{
		{"F8.5", "2.3547188"}, {"F9.3", "8789.7361"}, {"F2.1", "51.44"}, {"F10.4", "-23.24352"}, {"F5.2", "325.013"},
		{"F5.2", "-0.2"}, {"F4.2", "-0.2"}, {"F3.1", "0.5"}, {"F2.1", "0.5"}, {"F1.0", "0.0"}, {"F2.0", "0.0"},
		{"F8.0", "0.0"}, {"F5.0", "0.4"}, {"F5.0", "0.6"}, {"F6.1", "-0.01"}, {"F6.1", "-0.06"}, {"F5.1", "2.25"},
		{"F5.1", "2.35"}, {"F5.1", "2.45"}, {"F8.2", "99.995"}, {"F2.2", "0.5"}, {"F3.2", "0.5"}, {"F8.2", "1.0E20"},
	} {
		out("(1X,"+c.f+")", F(c.v))
	}
	out("(1X,F20.15)", D("0.1E0"))
	out("(1X,F30.25)", G("0.1E0"))
	out("(1X,F45.40)", H("0.1E0"))
	out("(1X,F8.2)", I(3))
	out("(1X,F8.2)", T("HI"))
	out("(1X,F8.2)", W(3))

	section("E, D: the manual's cases")
	for _, c := range []struct{ f, v string }{
		{"E9.2", "475867.222"}, {"E12.5", "475867.222"}, {"E12.3", "0.00069"}, {"E10.3", "-0.5555"}, {"E12.3", "-0.5555"},
		{"E5.3", "56.12"}, {"E14.5E4", "-1.001"}, {"E14.3E6", "0.000123"}, {"E10.3", "0.0"}, {"E9.3", "0.5555"},
		{"E8.3", "0.5555"}, {"E7.3", "0.5555"}, {"E12.4E1", "1.0E5"}, {"E12.4E1", "1.0E12"}, {"E12.4E4", "1.0E5"},
		{"E5.0", "1.0"}, {"E6.0", "1.0"}, {"E7.0", "1.0"}, {"E8.0", "1.0"}, {"D14.3", "0.0363"}, {"D9.6", "1.2"},
		{"E12.3", "1.0E-38"}, {"E12.3", "1.0E38"}, {"E9.2E1", "1.0E9"}, {"E9.2E2", "1.0E9"},
	} {
		out("(1X,"+c.f+")", F(c.v))
	}
	out("(1X,D23.12)", D("5413.87625793E0"))
	out("(1X,D14.3)", D("0.0363E0"))
	out("(1X,E14.4)", D("1.0E30"))
	out("(1X,E14.4)", I(1))

	section("Exponents past 99: G and H values")
	for _, f := range []string{"E14.4", "D14.4", "E14.4E3", "E14.4E2", "G14.4", "E14.4E4"} {
		out("(1X,"+f+")", G("1.0E100"))
		out("(1X,"+f+")", G("1.0E-100"))
		out("(1X,"+f+")", H("1.0E1000"))
		out("(1X,"+f+")", H("1.0E-1000"))
	}
	out("(1X,D14.4)", D("1.0E30"))
	out("(1X,D14.4)", G("1.0E30"))

	section("P: scale factors, and how long one lasts")
	for _, c := range []struct{ f, v string }{
		{"1PE12.3", "-270.139"}, {"1PE12.2", "-270.139"}, {"-1PE12.2", "-270.139"}, {"2PE12.3", "270.139"},
		{"4PE12.3", "270.139"}, {"5PE12.3", "270.139"}, {"-2PE12.3", "270.139"}, {"-3PE12.3", "270.139"},
		{"-3PE12.2", "270.139"}, {"-4PE12.2", "270.139"}, {"0PE12.3", "270.139"},
		{"2PF8.2", "25.0"}, {"-2PF8.2", "25.0"}, {"2PG13.3", "5.0"}, {"2PG13.3", "5.0E8"}, {"1PD14.3", "0.0363"},
	} {
		out("(1X,"+c.f+")", F(c.v))
	}
	out("(1X,F8.2,2PF8.2,F8.2)", F("25.0"), F("25.0"), F("25.0"))
	out("(1X,2PF8.2,0PF8.2)", F("25.0"), F("25.0"))
	out("(1X,F8.2,2PF8.2,F8.2)", F("25.0"), F("25.0"), F("25.0"), F("25.0"), F("25.0"), F("25.0"))
	out("(1X,F8.2,2P,F8.2)", F("25.0"), F("25.0"), F("25.0"))
	out("(1X,2P,I4,F8.2)", I(5), F("25.0"))

	section("G: the form, chosen after rounding")
	for _, c := range []struct{ f, v string }{
		{"G13.6", "0.01234567"}, {"G13.6", "-0.12345678"}, {"G13.6", "1.23456789"}, {"G13.6", "12.3456789"},
		{"G13.6", "123.456789"}, {"G13.6", "-1234.56789"}, {"G13.6", "12345.6789"}, {"G13.6", "123456.789"},
		{"G13.6", "-1234567.89"}, {"G13.6", "0.0"}, {"G15.7E2", "0.0"}, {"1PG15.7E2", "0.0"}, {"G13.6", "0.09999999"},
		{"G13.3", "999.6"}, {"G13.3", "0.09996"}, {"G8.3", "5.0"}, {"G7.3", "5.0"}, {"G6.3", "5.0"}, {"G13.0", "5.0"},
		{"G13.3E4", "5.0"}, {"G13.3E4", "5.0E8"},
	} {
		out("(1X,"+c.f+")", F(c.v))
	}
	out("(1X,G13.6)", I(5))
	out("(1X,G13.6)", T("HI"))
	out("(1X,G25.15)", D("5.0E0"))

	section("S, SP, SS: signs")
	out("(1X,SP,F8.2,SS,F8.2,S,F8.2)", F("1.0"), F("1.0"), F("1.0"))
	out("(1X,SP,I5,SS,I5,S,I5)", I(1), I(1), I(1))
	out("(1X,SP,E12.3,G13.3,D12.3)", F("1.0"), F("1.0"), F("1.0"))
	out("(1X,SP,O5,Z5)", I(1), I(1))
	out("(1X,SP,I1)", I(1))
	out("(1X,SP,F3.1)", F("1.0"))
	out("(1X,SP,I5)", I(0))

	section("L")
	out("(1X,L5)", LU(1))
	out("(1X,L1)", LU(0))
	out("(1X,L5)", I(1))
	out("(1X,L5)", I(2))
	out("(1X,L5)", WU(1))
	out("(1X,L5)", F("3.25"))
	out("(1X,L5)", T("HI"))
	out("(1X,L0)", LU(1))

	section("A: the manual's cases, and each item size")
	out("(1X,A5)", T("OHMS"))
	out("(1X,A5)", T("VOLTS"))
	out("(1X,A5)", T("AMPERES"))
	out("(1X,A)", T("HELLO"))
	out("(1X,A3)", T("HELLO"))
	out("(1X,A8)", T("HELLO"))
	out("(1X,A)", I(0x41424344))
	out("(1X,A2)", I(0x41424344))
	out("(1X,A6)", I(0x41424344))
	out("(1X,A)", W(0x4142))
	out("(1X,A)", B(0x41))
	out("(1X,A)", F("1.0"))
	out("(1X,A)", D("1.0E0"))
	out("(1X,A)", LU(0x41424344))

	section("Default widths: Table 8-4")
	for _, f := range []string{"I", "O", "Z", "L", "A", "F", "E", "G", "D"} {
		out("(1X,"+f+")", I(42))
		out("(1X,"+f+")", W(42))
		out("(1X,"+f+")", B(42))
		out("(1X,"+f+")", LU(1))
		out("(1X,"+f+")", WU(1))
		out("(1X,"+f+")", F("3.25"))
		out("(1X,"+f+")", D("3.25E0"))
		out("(1X,"+f+")", G("3.25E0"))
		out("(1X,"+f+")", H("3.25E0"))
		out("(1X,"+f+")", T("HELLO"))
	}

	section("Complex: two fields")
	out("(1X,2F8.5)", FC("2.3547188", "3.456732"))
	out("(1X,E9.2,', ',E5.3)", FC("47587.222", "56.123"))
	out("(1X,F8.2)", FC("1.0", "2.0"))
	out("(1X,F8.2,'|',F8.2,'|',F8.2,'|',F8.2)", FC("1.0", "2.0"), FC("3.0", "4.0"))
	out("(1X,2F12.5)", DC("1.5E0", "-2.5E0"))
	out("(1X,2F12.5)", GC("1.5E0", "-2.5E0"))
	out("(1X,I5,I5)", FC("1.0", "2.0"))
	out("(1X,F)", FC("1.0", "2.0"))
	out("(1X,D)", DC("1.0E0", "2.0E0"))

	section("Text, H, positions, and a record's length")
	out("('A',T5,'B',TL2,'C',TR2,'D',3X,'E',5X)")
	out("(T10,'X',T1,' Y')")
	out("(1X,'K=',I6,5X)", I(5))
	out("(3HABC,2H12)")
	out("(' A''B')")
	out("(3H'B')")
	out("(TL5,' A')")
	out("(' ABC',TL100,'D')")
	out("(T1,'A',T1,'B')")
	out("(T200,'X')")
	out("(1X,'ABC',T2,'D')")
	out("(1X,'A',0X,'B')")
	out("(' ',TR0,'A')")
	out("(' ',T0,'A')")
	out("(1X'ABC')")
	out("('A'1X)")
	out("('A'1X'B')")
	out("(' A'/' B')")
	out("(1X,I3'A')", I(1))
	out("(1X'A'I3)", I(1))
	out("(1X,2'A')")
	out("(1X,3HAB)")
	out("(1X,'')")

	section("Syntax: at run time, SYNERRFOR or accepted")
	for _, f := range []string{
		"(1X,I3,,I3)", "(1X I3 I3)", "(1XI3I3)", "( 1 X , I 3 )", "(1x,i3,i3)", "(1X,0I3)", "(1X,I0)", "(1X,0X,'A')",
		"(1X,J3)", "(1X,I3", "1X,I3)", "(1X,I3))", "(1X,I3)XYZ", "  (1X,I3)", "(1X,2)", "(1X,2X3)", "(1X,+3I3)",
		"(1X,-1I3)", "(1X,I-3)", "(1X,I3.-1)", "(1X,F8)", "(1X,F.2)", "(1X,F8.)", "(1X,E8.2E)", "(1X,G8.2E0)",
		"(1X,I3.2.1)", "(1X,1P)", "(1X,P)", "(1X,P,I3)", "(1X,2P3I3)", "(1X,2PI3)", "(1X,+2PF8.2)", "(1X,I32768)",
		"(1X,32768I3)", "(1X,I32767)", "(1X,F8.256)", "(1X,F8.255)", "(1X,E12.4E256)", "(1X,128PF8.2)", "(1X,-129PF8.2)",
		"(1X,<3>I3)", "(1X,I<3>)", "(1X,'A)", "(1X,3HAB", "(1X,0HA)", "(1X,H)", "(1X,T)", "(1X,TL)", "(1X,TR)", "(1X,X)",
		"(1X,3$)", "(1X,3:)", "(1X,3/)", "(1X,BN3)", "(1X,3BN)", "(1X,Q3)", "(1X,3Q)", "(1X,SP3)", "(1X,3SP)", "(1X,3T5)",
		"(1X,I3:I3)", "(1X,I3/I3)", "(1X,I3$)", "(1X,/I3)", "(1X,I3/,/I3)", "(1X,:I3)", "(1X,$,I3)", "(1X,/,I3)",
	} {
		out(f, I(1), I(2))
	}
	for _, f := range []string{"", " ", "()", "( )", "(,)", "(1X,)", "(,1X)"} {
		out(f)
	}

	section("Groups, and how deep")
	out("(1X,2(I2,1X))", I(1), I(2))
	out("(1X,2(I2,1X),I3)", I(1), I(2), I(3))
	out("(1X,2(2(I2,1X)))", I(1), I(2), I(3), I(4))
	out("(1X,(I2))", I(1))
	out("(1X,0(I2))", I(1))
	out("(1X,())", I(1))
	for _, n := range []int{3, 8, 16, 20, 21, 32, 64} {
		out("(1X,"+strings.Repeat("(", n)+"I3"+strings.Repeat(")", n)+")", I(n))
	}

	section("Reversion, :, /, $, and records")
	out("(1X,I3)", I(1), I(2), I(3))
	out("(1X,I3,/,1X,I3)", I(1), I(2), I(3))
	out("(1X,I3,2(1X,I2,1X,I4))", I(1), I(2), I(3), I(4), I(5), I(6), I(7), I(8))
	out("(1X,I3,(1X,I2),I4)", I(1), I(2), I(3), I(4), I(5), I(6), I(7), I(8))
	out("(1X,I3,(1X,I2),(1X,I4))", I(1), I(2), I(3), I(4), I(5), I(6))
	out("(1X,2PF8.2,(F8.2))", F("1.0"), F("1.0"), F("1.0"))
	out("(1X,SP,I3,(I3))", I(1), I(2), I(3))
	out("(//,' A'//)")
	out("(' X',/)")
	out("(/)")
	out("(//)")
	out("()")
	out("(' I=',I2,' J=',I2)", I(3))
	out("(' K=',I2,:,' L=',I2)", I(4))
	out("(' K=',I2,:)", I(4), I(5))
	out("(:,' A')")
	out("(' A',:,' B')")
	out("(1X,'A')", I(1))
	out("(1X)", I(1))
	out("('')", I(1))
	out("(1X,Q,I3)", I(9), I(1))
	out("(1X,Q)", I(1))
	out("(1X,BN,I3,BZ)", I(1))

	section("$ and carriage control's first byte, as stored")
	out("(' A',$)")
	out("(' B')")
	out("('+C',$)")
	out("('+D')")
	out("('$E',$)")
	out("('$F')")
	out("('0G',$)")
	out("('0H')")
	out("('1I',$)")
	out("('1J')")
	out("('XK',$)")
	out("('XL')")
	out("(1X,$)")
	out("($)")
	out("($,' M')")
	out("(' N',$,' O')")
	out("(' P',$/' Q')")
	out("(' R'/$)")
	out("(1X,I3,$)", I(1), I(2))

	section("A record's size at unit 6")
	for _, n := range []int{79, 80, 81, 132, 133, 134, 256, 1000, 32767, 32768} {
		out(fmt.Sprintf("(1X,T%d,'X')", n))
	}
	for _, n := range []int{131, 132, 133, 134} {
		out(fmt.Sprintf("(1X,%dA1)", n), T("A"))
	}
	out("(1X,I5000)", I(1))

	section("Units: which exist, and which a program may write")
	outU(5, "(' WRITE TO 5')")
	outU(-1, "(' WRITE TO -1')")
	outU(-2, "(' WRITE TO -2')")
	outU(-3, "(' WRITE TO -3')")
	outU(-4, "(' WRITE TO -4')")
	outU(-5, "(' WRITE TO -5')")
	outU(0, "(' WRITE TO 0')")
	outU(1, "(' WRITE TO 1')")
	outU(99, "(' WRITE TO 99')")
	outU(100, "(' WRITE TO 100')")
	outU(101, "(' WRITE TO 101')")
	outU(119, "(' WRITE TO 119')")
	outU(120, "(' WRITE TO 120')")
	outU(1000, "(' WRITE TO 1000')")

	section("List-directed output: Table 7-1, and a record's size")
	list(I(1))
	list(I(1), I(2), I(3), I(4), I(5), I(6), I(7), I(8), I(9), I(10))
	list(I(-2147483647), I(2147483647))
	list(W(7), B(7), LU(1), WU(1), LU(0))
	list(F("3.4"))
	list(F("3.4"), F("3.4"), F("3.4"), F("3.4"), F("3.4"))
	list(F("3.4"), F("3.4"), F("3.4"), F("3.4"), F("3.4"), F("3.4"))
	list(F("0.0"), F("1.0E20"), F("-1.0E-20"), F("0.1"), F("1.0E6"), F("1.0E7"))
	list(D("3.4E0"))
	list(D("3.4E0"), D("3.4E0"), D("3.4E0"), D("3.4E0"))
	list(D("0.0E0"), D("1.0E20"), D("-1.0E-20"))
	list(G("3.4E0"), G("1.0E200"))
	list(H("3.4E0"), H("1.0E2000"))
	list(FC("1.0", "2.0"))
	list(FC("1.0", "2.0"), FC("3.0", "4.0"))
	list(FC("1.0", "2.0"), FC("3.0", "4.0"), FC("5.0", "6.0"), FC("7.0", "8.0"), FC("9.0", "1.0"))
	list(DC("1.0E0", "-2.5E0"))
	list(DC("1.0E0", "-2.5E0"), DC("1.0E0", "-2.5E0"), DC("1.0E0", "-2.5E0"))
	list(GC("1.0E0", "-2.5E0"))
	list(T("HELLO"))
	list(T("HELLO"), T("WORLD"))
	list(I(1), T("HELLO"), I(2))
	list(T("HELLO"), I(1))
	list(I(1), F("1.0"), T("A"), LU(1), D("1.0E0"))
	list(T(""))
	list(T(strings.Repeat("A", 79)))
	list(T(strings.Repeat("A", 80)))
	list(T(strings.Repeat("A", 81)))
	list(T(strings.Repeat("A", 100)))
	list(T("AB"), T(strings.Repeat("C", 100)), LU(1), LU(1), LU(1), I(-3))
	list()

	section("Internal files: writing")
	wi(20, "(' I=',I5)", I(42))
	wi(20, "(' I=',I5,' F=',F7.3)", I(42), F("2.5"))
	wi(10, "(' I=',I5,' F=',F7.3)", I(42), F("2.5"))
	wi(10, "(' A',/,' B')")
	wi(10, "(/)")
	wi(10, "()")
	wi(10, "('ABC',$)")
	wi(10, "('+ABC')")
	wi(5, "(A)", T("HELLO"))
	wi(4, "(A)", T("HELLO"))
	wi(6, "(A)", T("HELLO"))
	wil(40, I(42), F("2.5"))
	wil(10, I(42), F("2.5"))
	wil(40, T("HELLO"))
	wia(5, "(A)", T("ONE"))
	wia(5, "(A,/,A)", T("ONE"), T("TWO"))
	wia(5, "(A,/,A,/,A)", T("ONE"), T("TWO"), T("THREE"))
	wia(5, "(A,/,A,/,A,/,A)", T("ONE"), T("TWO"), T("THREE"), T("FOUR"))
	enc(12, "(3I4)", I(9012), I(5678), I(1234))
	enc(20, "(3I4)", I(9012), I(5678), I(1234))
	enc(8, "(3I4)", I(9012), I(5678), I(1234))
	enc(12, "(I4,/,I4)", I(1), I(2))
	enc(12, "(I4)", I(1), I(2))
	enc(0, "(I4)", I(1))
	enc(-1, "(I4)", I(1))

	section("I, O, Z, L input, on an internal record")
	for _, c := range []struct {
		f, t string
		i    item
	}{
		{"I4", "2788", iL}, {"I3", "-26", iL}, {"I9", "      312", iL}, {"I3", "   ", iL}, {"I3", "1 2", iL},
		{"BN,I3", "1 2", iL}, {"BZ,I3", "1 2", iL}, {"I3", "12 ", iL}, {"BN,I3", "12 ", iL}, {"I3", " 12", iL},
		{"I12", "2147483647", iL}, {"I12", "2147483648", iL}, {"I12", "-2147483648", iL}, {"I12", "-2147483649", iL},
		{"I3", "ABC", iL}, {"I3", "1.5", iL}, {"I3", "+12", iL}, {"I3", "- 1", iL}, {"I3", "1-2", iL}, {"I3", "1,2", iL},
		{"I5", "32767", iW}, {"I5", "32768", iW}, {"I6", "-32768", iW}, {"I3", "127", iB}, {"I3", "128", iB},
		{"I3", "12", iL}, {"I4.2", "12", iL}, {"I", "42", iL}, {"I", "  42", iL}, {"I", "42", iW}, {"I", "42", iB},
		{"I3", "1", iF}, {"I3", "1", iLU}, {"I3", "ABC", iT(3)},
		{"O5", "32767", iL}, {"O4", "16234", iL}, {"O3", "970", iL}, {"O", "17", iL}, {"O3", "-7", iL}, {"O3", "1 2", iL},
		{"O12", "37777777777", iL}, {"O12", "77777777777", iL}, {"O6", "177777", iW}, {"O6", "777777", iW},
		{"O3", "7", iF}, {"O12", "7", iD},
		{"Z3", "A94", iL}, {"Z5", "A23DEF", iL}, {"Z5", "95.AF2", iL}, {"Z", "FF", iL}, {"Z3", "a9f", iL}, {"Z3", "-A9", iL},
		{"Z8", "FFFFFFFF", iL}, {"Z9", "1FFFFFFFF", iL}, {"Z4", "FFFF", iW}, {"Z5", "FFFFF", iW}, {"Z8", "40800000", iF},
		{"Z16", "40800000", iD}, {"Z3", "G", iL},
		{"L1", "T", iLU}, {"L1", "t", iLU}, {"L2", ".T", iLU}, {"L2", ".t", iLU}, {"L1", "F", iLU}, {"L1", "f", iLU},
		{"L2", ".F", iLU}, {"L1", " ", iLU}, {"L5", "    T", iLU}, {"L5", "TRUE ", iLU}, {"L5", ".TRUE", iLU},
		{"L5", "FALSE", iLU}, {"L1", "X", iLU}, {"L1", "1", iLU}, {"L", "T", iLU}, {"L1", "T", iWU}, {"L1", "T", iL},
		{"L1", "T", iF}, {"L1", "T", iT(1)},
	} {
		rd("("+c.f+")", c.t, c.i)
	}

	section("F, E, D, G input")
	for _, c := range []struct{ f, t string }{
		{"F8.5", "123456789"}, {"F9.5", "-1234.567"}, {"F8.5", "24.77E+2"}, {"F5.2", "1234567.89"}, {"F8.0", "    1234"},
		{"F8.2", "    1234"}, {"F8.2", "  1 2  3"}, {"BN,F8.2", "  1 2  3"}, {"BZ,F8.2", "  1 2  3"}, {"F5.2", "     "},
		{"F5.2", "1.5E3"}, {"F5.2", "1.5D3"}, {"F5.2", "1.5Q3"}, {"F5.2", "1.5+3"}, {"F5.2", "1.5-3"}, {"F5.2", "1E"},
		{"F5.2", "ABC"}, {"F5.2", "."}, {"F5.2", "+"}, {"F5.2", "-.5"}, {"F5.2", "1.2.3"}, {"F5.2", "1,5"},
		{"F9.2", "1.0E39"}, {"F9.2", "1.0E-39"}, {"F9.2", "1.0E+39"},
		{"E9.3", "734.432E3"}, {"E12.4", "  1022.43E-6"}, {"E15.3", "52.3759663     "}, {"E12.5", "210.5271D+10"},
		{"BZ,D10.2", "12345     "}, {"D10.2", "   123.45 "}, {"D15.3", "367.4981763D+04"}, {"G13.6", "1.5"}, {"G13.6", "1.5E3"},
		{"F", "1.5"}, {"E", "1.5"}, {"G", "1.5"}, {"D", "1.5"},
		{"3PE10.5", "   37.614 "}, {"3PE10.5", "  37.614E2"}, {"-3PE10.5", "    37.614"}, {"2PF8.2", "    1234"}, {"2PF8.2", "   12.34"},
	} {
		rd("("+c.f+")", c.t, iF)
	}
	rd("(F8.5)", "123456789", iD)
	rd("(F8.5)", "123456789", iG)
	rd("(F8.5)", "123456789", iH)
	rd("(F4.1)", "12.5", iL)
	rd("(F4.1)", "12.5", iW)
	rd("(F4.1)", "12.5", iLU)
	rd("(F4.1)", "12.5", iT(4))
	rd("(E9.3)", "1.0E99", iD)
	rd("(E9.3)", "1.0E100", iG)
	rd("(E12.3)", "1.0E1000", iH)
	rd("(2F8.5)", "1234567812345.67", iFC)
	rd("(E9.1,F9.3)", "   1.5E3     2.5", iFC)
	rd("(2F8.5)", "1234567812345.67", iDC)
	rd("(F8.5)", "12345678", iFC)

	section("A input: Table 8-3, by item size")
	for _, i := range []item{iT(1), iT(3), iT(6), iT(8), iB, iW, iL, iF, iD, iH, iLU} {
		rd("(A6)", "PAGE #", i)
	}
	rd("(A)", "HELLO", iT(5))
	rd("(A)", "HELLO", iT(8))
	rd("(A)", "HELLO", iT(3))
	rd("(A)", "ABCD", iL)
	rd("(A)", "ABCDEFGH", iD)
	rd("(A)", "AB", iW)
	rd("(A)", "AB", iL)

	section("Positions, Q, H, separators, on an internal record")
	rd("(T7,A3,T1,A3)", "ABC   XYZ", iT(3), iT(3))
	rd("(TR2,A1,TL2,A1)", "ABCDE", iT(1), iT(1))
	rd("(1X,I2,3X,I2)", "A12BBB34", iL, iL)
	rd("(T50,A1)", "ABC", iT(1))
	rd("(TL50,A1)", "ABC", iT(1))
	rd("(Q,A3)", "ABCDE", iL, iT(3))
	rd("(A2,Q)", "ABCDE", iT(2), iL)
	rd("(Q)", "ABCDE", iW)
	rd("(Q)", "ABCDE", iF)
	rd("(Q)", "ABCDE", iT(1))
	rd("(5HXXXXX,A1)", "ABCDEF", iT(1))
	rd("('XXXXX',A1)", "ABCDEF", iT(1))
	rd("(2I6,2F10.2)", "1,-2,1.0,35", iL, iL, iF, iF)
	rd("(2I6)", ",,", iL, iL)
	rd("(2I6)", "1,", iL, iL)
	rd("(I6,A3)", "1,ABC", iL, iT(3))
	rd("(A3,I3)", "A,B123", iT(3), iL)
	rd("(L6,I3)", "T,123", iLU, iL)
	rd("(O6,Z6,I3)", "7,F,1", iL, iL, iL)
	rd("(I3,I3)", "123,456", iL, iL)
	rd("(3I3)", "1", iL, iL, iL)
	rd("(A3,A3)", "AB", iT(3), iT(3))
	rd("(I3,/,I3)", "123", iL, iL)
	rd("(I3)", "123", iL, iL)
	rd("(I3,:,I3)", "123", iL)
	rd("(I3,$)", "123", iL)
	rd("(SP,I3)", "123", iL)
	rd("(BN,I3,BZ,I3)", "1 21 2", iL, iL)
	rd("(BZ,I3,BN,I3)", "1 21 2", iL, iL)
	ril("1 2 3", iL, iL, iL)
	ril("1,2/", iL, iL, iL)
	ril("'ABC' 1.5", iT(3), iF)

	section("Internal arrays, DECODE, end of an internal file")
	ria(3, "123456", "(I3)", iL)
	ria(3, "123456", "(I3,/,I3)", iL, iL)
	ria(3, "123456", "(I3,/,I3,/,I3)", iL, iL, iL)
	ria(3, "123456", "(I3)", iL, iL, iL)
	dec(12, "123456789012", "(3I4)", iL, iL, iL)
	dec(8, "123456789012", "(3I4)", iL, iL, iL)
	dec(12, "123456789012", "(I4,/,I4)", iL, iL)
	dec(12, "123456789012", "(I4)", iL, iL)
	dec(0, "12", "(I4)", iL)

	section("Records from SYS$INPUT: formatted")
	rd5("(I3)", []string{"123"}, iL)
	rd5("(I3)", []string{"1 2"}, iL)
	rd5("(BN,I3)", []string{"1 2"}, iL)
	rd5("(I3)", []string{"12"}, iL)
	rd5("(I3,/,I3)", []string{"123", "456"}, iL, iL)
	rd5("(I3)", []string{"123", "456"}, iL, iL)
	rd5("(/,I3)", []string{"123", "456"}, iL)
	rd5("(//,I3)", []string{"123", "456", "789"}, iL)
	rd5("(I3,/)", []string{"123", "456"}, iL)
	rd5("(A)", []string{"HELLO THERE"}, iT(5))
	rd5("(A)", []string{""}, iT(5))
	rd5("(5HXXXXX)", []string{"ABCDE"})
	rd5("(Q,A)", []string{"HELLO"}, iL, iT(5))
	rd5("(A)", []string{strings.Repeat("A", 132)}, iT(4))
	rd5("(Q)", []string{strings.Repeat("A", 132)}, iL)
	rd5("(Q)", []string{strings.Repeat("A", 133)}, iL)
	rd5("(Q)", []string{strings.Repeat("A", 200)}, iL)
	rd5("(Q)", []string{strings.Repeat("A", 255)}, iL)
	rd5("(Q)", []string{strings.Repeat("A", 256)}, iL)
	rd5("(Q)", []string{strings.Repeat("A", 500)}, iL)
	rd5("(I2,I2,I2)", []string{"1"}, iL, iL, iL)
	rd5("()", []string{"1"})
	rd5("(1X)", []string{"1"})
	rd5("(1X,'A')", []string{"1"}, iL)
	rd5("(I3)", []string{"ABC"}, iL)
	rd5("(I3)", []string{"456"}, iL)
	rd5("(I3)", []string{"123"}, iL, iL)
	rd5("(T1,I1,T1,I1)", []string{"7"}, iL, iL)
	rd5("(I1,$)", []string{"7"}, iL)
	// The manual's four READs of a ten-record file (8-44).
	rd5("(I1,/,F4.1)", []string{"1", "1.5"}, iL, iF)
	rd5("(F4.1)", []string{"1.5", "2.5"}, iF, iF)
	rd5("(I1,F4.1,(I1))", []string{"11.5", "2", "3"}, iL, iF, iL, iL)
	rd5("(2I1,/,I1)", []string{"12", "3", "45", "6"}, iL, iL, iL, iL, iL, iL)

	section("List-directed input")
	rl5([]string{"4 6.3 (3.4,4.2), (3, 2)    T,F ,, 3*14.6 ,'ABC,DEF/GHI '' JK'/"},
		iL, iF, iFC, iFC, iLU, iLU, iL, iL, iF, iD, iT(14), iF, iF)
	rl5([]string{"1 2 3"}, iL, iL, iL)
	rl5([]string{"1,2,3"}, iL, iL, iL)
	rl5([]string{"1\t2\t3"}, iL, iL, iL)
	rl5([]string{"1", "2", "3"}, iL, iL, iL)
	rl5([]string{"1,", "2,", "3"}, iL, iL, iL)
	rl5([]string{",2,", "3"}, iL, iL, iL)
	rl5([]string{"1,,3"}, iL, iL, iL)
	rl5([]string{"1,2/", "9"}, iL, iL, iL)
	rl5([]string{"/", "9"}, iL)
	rl5([]string{"2*7"}, iL, iL)
	rl5([]string{"2*", "7"}, iL, iL, iL)
	rl5([]string{"0*7", "8"}, iL)
	rl5([]string{"3*7"}, iL, iL)
	rl5([]string{"1.5"}, iL)
	rl5([]string{"7"}, iF)
	rl5([]string{"7"}, iD)
	rl5([]string{"1.5E3 1.5D3 1.5Q3 1.5+3"}, iF, iF, iF, iF)
	rl5([]string{"ABC"}, iL)
	rl5([]string{"ABC"}, iF)
	rl5([]string{"'A'"}, iL)
	rl5([]string{"(1.0,2.0)"}, iFC)
	rl5([]string{"( 1.0 , 2.0 )"}, iFC)
	rl5([]string{"(1.0,", "2.0)"}, iFC)
	rl5([]string{"(1.0", ",2.0)"}, iFC)
	rl5([]string{"(,2.0)"}, iFC)
	rl5([]string{"(1.0,)"}, iFC)
	rl5([]string{"(1,2)"}, iDC)
	rl5([]string{"1.0"}, iFC)
	rl5([]string{"2*(1.0,2.0)"}, iFC, iFC)
	rl5([]string{"T F .T .F t f .t .f"}, iLU, iLU, iLU, iLU, iLU, iLU, iLU, iLU)
	rl5([]string{"TRUE FALSE .TRUE. .FALSE. TX"}, iLU, iLU, iLU, iLU, iLU)
	rl5([]string{"X"}, iLU)
	rl5([]string{"1"}, iLU)
	rl5([]string{"'ABC'"}, iT(5))
	rl5([]string{"'ABCDEFG'"}, iT(5))
	rl5([]string{"'A''B'"}, iT(3))
	rl5([]string{"'AB", "CD'"}, iT(4))
	rl5([]string{"'AB ", " CD'"}, iT(6))
	rl5([]string{"ABC"}, iT(3))
	rl5([]string{"7"}, iT(3))
	rl5([]string{"\"ABC\""}, iT(3))
	rl5([]string{"2*'AB'"}, iT(2), iT(2))
	rl5([]string{"'AB'"}, iL)
	rl5([]string{"\"17"}, iL)
	rl5([]string{"'17'O"}, iL)
	rl5([]string{"'FF'X"}, iL)
	rl5([]string{"3HABC"}, iL)
	rl5([]string{"", "", "7"}, iL)
	rl5([]string{"   ", "7"}, iL)
	rl5([]string{"7"}, iW)
	rl5([]string{"70000"}, iW)
	rl5([]string{"7"}, iB)
	rl5([]string{"300"}, iB)
	rl5([]string{"2147483648"}, iL)

	section("Namelist: the block, found by trial")
	nmlWrite()
	nmlRead(" $GRP VAL=7 $END")
	nmlRead(" $GRP VAL=7, R=1.5, T='ABC', A(2)=8,9 $END")
	nmlRead(" &GRP VAL=7 &END")
	nmlRead(" $GRP VAL=7 $")
	nmlRead(" &GRP VAL=7 &")
	nmlRead(" $GRP VAL=7 /")
	nmlRead(" $OTHER VAL=7 $END", " $GRP VAL=8 $END")
	nmlRead(" IGNORED", " $GRP VAL=7 $END")
	nmlRead(" $GRP", " VAL=7,", " R=2.5", " $END")
	nmlRead(" $GRP A=1,2,3 $END")
	nmlRead(" $GRP A=3*5 $END")
	nmlRead(" $GRP A=2*5 $END")
	nmlRead(" $GRP A(2)=5 $END")
	nmlRead(" $GRP A(4)=5 $END")
	nmlRead(" $GRP A=1,2,3,4 $END")
	nmlRead(" $GRP A(2)=,6 $END")
	nmlRead(" $GRP T(2:3)='ZZ' $END")
	nmlRead(" $GRP NOPE=1 $END")
	nmlRead(" $GRP VAL=ABC $END")
	nmlRead(" $GRP VAL 7 $END")
	nmlRead(" $GRP $END")
	nmlRead(" $GRP")
	nmlRead(" $grp val=7 $end")
	nmlRead(" $GRP R=1 $END")
	nmlRead(" $GRP VAL=1.5 $END")
	nmlRead("$GRP VAL=7 $END")
	nmlRead("X$GRP VAL=7 $END")
	nmlRead(" ?")
	nmlRead(" =?")

	section("The statements around a statement")
	nested("the same unit", "PUSHAB\tFMT_A\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO")
	nested("another unit", "PUSHAB\tFMT_A\n\tPUSHL\t#7\n\tCALLS\t#2,G^FOR$WRITE_SO")
	nested("an internal record", "PUSHAB\tFMT_A\n\tPUSHAB\tNEST_DESC\n\tCALLS\t#2,G^FOR$WRITE_IO")
	endTwice()
	noEnd()
	itemNoStatement()

	// The input's end: FOR80R's data stops after this marker, so these come last.
	section("The end of SYS$INPUT")
	rd5("(I3)", nil, iL)
	rd5("(I3)", nil, iL)
	rl5(nil, iL)
}

// nmlWrite and nmlRead use the block PHASE-80's Part A describes: the group
// name's address, a count, then each entity's name's and descriptor's addresses.
const nmlData = `NML_GRP:	.ASCIC	/GRP/
NML_N1:	.ASCIC	/VAL/
NML_N2:	.ASCIC	/R/
NML_N3:	.ASCIC	/T/
NML_N4:	.ASCIC	/A/
NML_N5:	.ASCIC	/LONGNAME1/
NML_N6:	.ASCIC	/C/
NML_N7:	.ASCIC	/L/
NML_N8:	.ASCIC	/DD/
	.ALIGN	LONG
NML_V1:	.LONG	42
NML_V2:	.F_FLOATING 2.5
NML_V3:	.ASCII	/HELLO/
	.ALIGN	LONG
NML_V4:	.LONG	1,1,1
NML_V5:	.LONG	-5
NML_V6:	.F_FLOATING 1.5
	.F_FLOATING -2.5
NML_V7:	.LONG	-1
NML_V8:	.D_FLOATING 2.5
NML_D1:	.WORD	4
	.BYTE	8,1
	.ADDRESS NML_V1
NML_D2:	.WORD	4
	.BYTE	10,1
	.ADDRESS NML_V2
NML_D3:	.WORD	5
	.BYTE	14,1
	.ADDRESS NML_V3
NML_D4:	.WORD	4
	.BYTE	8,4
	.ADDRESS NML_V4
	.BYTE	0,0,^XC0,1
	.LONG	12
	.ADDRESS NML_V4
	.LONG	3
	.LONG	1,3
NML_D5:	.WORD	4
	.BYTE	8,1
	.ADDRESS NML_V5
NML_D6:	.WORD	8
	.BYTE	12,1
	.ADDRESS NML_V6
NML_D7:	.WORD	4
	.BYTE	8,1
	.ADDRESS NML_V7
NML_D8:	.WORD	8
	.BYTE	11,1
	.ADDRESS NML_V8
NML_BLK:	.ADDRESS NML_GRP
	.LONG	8
	.ADDRESS NML_N1,NML_D1,NML_N2,NML_D2,NML_N3,NML_D3,NML_N4,NML_D4
	.ADDRESS NML_N5,NML_D5,NML_N6,NML_D6,NML_N7,NML_D7,NML_N8,NML_D8
NML_INI:	.LONG	42
	.F_FLOATING 2.5
	.ASCII	/HELLO/
	.ALIGN	LONG
	.LONG	1,1,1
NEST_B:	.BLKB	12
NEST_DESC:	.WORD	12
	.BYTE	14,1
	.ADDRESS NEST_B
FMT_A:	.ASCIZ	~(1X,I3)~
`

func nmlWrite() {
	cases = append(cases, kase{op: "raw", title: "namelist write", code: `	PUSHAB	NML_BLK
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SN
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`})
}

func nmlRead(lines ...string) {
	cases = append(cases, kase{op: "raw", unit: 5, lines: lines, title: "namelist read " + lines[0], code: `	MOVC3	#12,NML_INI,NML_V1
	MOVC3	#5,NML_INI+8,NML_V3
	MOVL	#1,NML_V4
	MOVL	#1,NML_V4+4
	MOVL	#1,NML_V4+8
	PUSHAB	NML_BLK
	PUSHL	#5
	CALLS	#2,G^FOR$READ_SN
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`, show: []string{`	PUSHAB	NML_BLK
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SN
	CALLS	#0,G^FOR$IO_END
`}})
}

// nested begins a statement inside another, as a function in an I/O list may.
func nested(what, begin string) {
	cases = append(cases, kase{op: "raw", title: "nested statement: " + what, code: `	PUSHAB	FMT_A
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	#1
	CALLS	#1,G^FOR$IO_L_V
	` + begin + `
	PUSHL	#2
	CALLS	#1,G^FOR$IO_L_V
	CALLS	#0,G^FOR$IO_END
	PUSHL	#3
	CALLS	#1,G^FOR$IO_L_V
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`})
}

func endTwice() {
	cases = append(cases, kase{op: "raw", title: "FOR$IO_END twice", code: `	PUSHAB	FMT_A
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	#1
	CALLS	#1,G^FOR$IO_L_V
	CALLS	#0,G^FOR$IO_END
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`})
}

func noEnd() {
	cases = append(cases, kase{op: "raw", title: "a statement with no FOR$IO_END, then another", code: `	PUSHAB	FMT_A
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	#1
	CALLS	#1,G^FOR$IO_L_V
	PUSHAB	FMT_A
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	#2
	CALLS	#1,G^FOR$IO_L_V
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`})
}

func itemNoStatement() {
	cases = append(cases, kase{op: "raw", title: "an item, no statement", code: `	PUSHL	#1
	CALLS	#1,G^FOR$IO_L_V
	CALLS	#0,G^FOR$IO_END
	MOVL	R0,ENDR0
`})
}

// ascii writes s as a MACRO string's body: text between ~s, and each tab as <9>.
func ascii(s string) string {
	if strings.Contains(s, "~") {
		log.Fatalf("a ~ in %q", s)
	}

	parts := strings.Split(s, "\t")
	for i, p := range parts {
		parts[i] = "~" + p + "~"
	}

	return strings.Join(parts, "<9>")
}

// slotShow is how many longwords an input item's slot shows.
func slotShow(k string) int {
	switch k {
	case "l", "w", "b", "lu", "wu", "f":
		return 1
	case "d", "g", "fc":
		return 2
	default:
		return 4
	}
}

// entry is FORRTL's routine for an item of kind k, by reference.
var entry = map[string]string{
	"l": "L_R", "w": "W_R", "b": "B_R", "lu": "LU_R", "wu": "WU_R",
	"f": "F_R", "d": "D_R", "g": "G_R", "h": "H_R", "fc": "FC_R", "dc": "DC_R", "gc": "GC_R",
}

// outItem writes the data and code for an output item.
func outItem(n, j int, it item, data, code *strings.Builder) {
	label := fmt.Sprintf("V%d_%d", n, j)

	switch it.kind {
	case "L":
		fmt.Fprintf(code, "\tPUSHL\t#%s\n\tCALLS\t#1,G^FOR$IO_L_V\n", it.val)
		return
	case "t":
		fmt.Fprintf(data, "%s:\t.ASCID\t%s\n", label, ascii(it.val))
		fmt.Fprintf(code, "\tPUSHAB\t%s\n\tCALLS\t#1,G^FOR$IO_T_DS\n", label)
		return
	case "l", "lu":
		fmt.Fprintf(data, "\t.ALIGN\tLONG\n%s:\t.LONG\t%s\n", label, it.val)
	case "w", "wu":
		fmt.Fprintf(data, "\t.ALIGN\tLONG\n%s:\t.WORD\t%s\n", label, it.val)
	case "b":
		fmt.Fprintf(data, "%s:\t.BYTE\t%s\n", label, it.val)
	case "f", "d", "g", "h":
		fmt.Fprintf(data, "\t.ALIGN\tLONG\n%s:\t.%s_FLOATING %s\n", label, strings.ToUpper(it.kind), it.val)
	case "fc", "dc", "gc":
		parts := strings.Split(it.val, ",")
		fmt.Fprintf(data, "\t.ALIGN\tLONG\n%s:\t.%s_FLOATING %s\n\t.%s_FLOATING %s\n", label,
			strings.ToUpper(it.kind[:1]), parts[0], strings.ToUpper(it.kind[:1]), parts[1])
	default:
		log.Fatalf("output item kind %q", it.kind)
	}

	fmt.Fprintf(code, "\tPUSHAB\t%s\n\tCALLS\t#1,G^FOR$IO_%s\n", label, entry[it.kind])
}

// inItem writes the code for input item j, into slot j, and returns the code that shows it.
func inItem(n, j int, it item, data, code *strings.Builder) string {
	slot := fmt.Sprintf("SLOTS+%d", j*16)

	if it.kind == "t" {
		label := fmt.Sprintf("V%d_%d", n, j)
		text := fmt.Sprintf("TEXTS+%d", j*64)
		fmt.Fprintf(data, "\t.ALIGN\tLONG\n%s:\t.WORD\t%s\n\t.BYTE\t14,1\n\t.ADDRESS %s\n", label, it.val, text)
		fmt.Fprintf(code, "\tPUSHAB\t%s\n\tCALLS\t#1,G^FOR$IO_T_DS\n", label)

		return fmt.Sprintf("\tPUSHL\t#%s\n\tPUSHAB\t%s\n\tPUSHL\t#%d\n\tCALLS\t#3,SHOWT\n", it.val, text, j)
	}

	fmt.Fprintf(code, "\tPUSHAB\t%s\n\tCALLS\t#1,G^FOR$IO_%s\n", slot, entry[it.kind])

	return fmt.Sprintf("\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHL\t#%d\n\tCALLS\t#3,SHOWV\n", slotShow(it.kind), slot, j)
}

func itemsTitle(items []item) string {
	var b strings.Builder

	for _, it := range items {
		b.WriteByte(' ')
		b.WriteString(it.kind)

		if it.val != "" {
			b.WriteByte('=')

			if len(it.val) > 12 {
				fmt.Fprintf(&b, "%s..%d", it.val[:8], len(it.val))
			} else {
				b.WriteString(it.val)
			}
		}
	}

	return b.String()
}

func clip(s string, n int) string {
	if len(s) > n {
		return fmt.Sprintf("%s..%d", s[:n], len(s))
	}

	return s
}

// program writes for80.mar.
func program() string {
	var data, code, main strings.Builder

	for n, c := range cases {
		if s, ok := secAt[n]; ok {
			fmt.Fprintf(&data, "SC%d:\t.ASCID\t%s\n", n, ascii("##### "+s))
			fmt.Fprintf(&main, "\tPUSHAB\tSC%d\n\tCALLS\t#1,PUTT\n", n)
		}

		title := fmt.Sprintf("#%d", n)

		switch c.op {
		case "raw":
			title += " " + c.title
		case "out", "list":
			if c.unit != 6 {
				title += fmt.Sprintf(" unit %d", c.unit)
			}

			title += " " + c.op + " " + clip(c.format, 60) + itemsTitle(c.items)
		case "wi", "wil", "wia", "enc":
			title += fmt.Sprintf(" %s %d %s%s", c.op, c.size, clip(c.format, 50), itemsTitle(c.items))
		default:
			title += fmt.Sprintf(" %s %s |%s|%s", c.op, clip(c.format, 40), clip(c.text, 30), itemsTitle(c.items))
		}

		if len(c.lines) > 0 {
			title += " <" + clip(strings.Join(c.lines, "/"), 40) + ">"
		}

		title = strings.ReplaceAll(title, "\t", "^I")
		fmt.Fprintf(&data, "HD%d:\t.ASCID\t%s\n", n, ascii(clip(title, 120)))
		fmt.Fprintf(&main, "\tPUSHAB\tHD%d\n\tCALLS\t#1,PUTT\n\tCALLS\t#0,PRE\n", n)

		if len(c.lines) > 0 || c.op == "rd5" || c.op == "rl5" {
			fmt.Fprintf(&data, "SE%d:\t.ASCID\t%s\n", n, ascii(fmt.Sprintf("#%d", n)))
			fmt.Fprintf(&main, "\tPUSHAB\tSE%d\n\tCALLS\t#1,SYNC\n", n)
		}

		fmt.Fprintf(&code, "\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHND,(FP)\n", n)

		if c.format != "" || c.op == "out" || c.op == "rd" || c.op == "rd5" || c.op == "wi" || c.op == "wia" ||
			c.op == "ria" || c.op == "enc" || c.op == "dec" {
			fmt.Fprintf(&data, "FM%d:\t.ASCIZ\t%s\n", n, ascii(c.format))
		}

		var shows strings.Builder

		input := false

		switch c.op {
		case "out":
			fmt.Fprintf(&code, "\tPUSHAB\tFM%d\n\tPUSHL\t#%d\n\tCALLS\t#2,G^FOR$WRITE_SO\n", n, c.unit)
		case "list":
			fmt.Fprintf(&code, "\tPUSHL\t#%d\n\tCALLS\t#1,G^FOR$WRITE_SL\n", c.unit)
		case "rd":
			input = true
			fmt.Fprintf(&data, "IN%d:\t.ASCID\t%s\n", n, ascii(c.text))
			fmt.Fprintf(&code, "\tPUSHAB\tFM%d\n\tPUSHAB\tIN%d\n\tCALLS\t#2,G^FOR$READ_IO\n", n, n)
		case "ril":
			input = true
			fmt.Fprintf(&data, "IN%d:\t.ASCID\t%s\n", n, ascii(c.text))
			fmt.Fprintf(&code, "\tPUSHAB\tIN%d\n\tCALLS\t#1,G^FOR$READ_IL\n", n)
		case "rd5":
			input = true
			fmt.Fprintf(&code, "\tPUSHAB\tFM%d\n\tPUSHL\t#5\n\tCALLS\t#2,G^FOR$READ_SO\n", n)
		case "rl5":
			input = true
			fmt.Fprintf(&code, "\tPUSHL\t#5\n\tCALLS\t#1,G^FOR$READ_SL\n")
		case "wi", "wil", "wia":
			fmt.Fprintf(&data, "\t.ALIGN\tLONG\nIO%d:\t.WORD\t%d\n", n, c.size)

			if c.op == "wia" {
				fmt.Fprintf(&data, "\t.BYTE\t14,4\n\t.ADDRESS\tTEXTS\n\t.BYTE\t0,0,^XC0,1\n\t.LONG\t%d\n\t.ADDRESS\tTEXTS\n\t.LONG\t3\n\t.LONG\t1,3\n", 3*c.size)
			} else {
				fmt.Fprintf(&data, "\t.BYTE\t14,1\n\t.ADDRESS\tTEXTS\n")
			}

			if c.op == "wil" {
				fmt.Fprintf(&code, "\tPUSHAB\tIO%d\n\tCALLS\t#1,G^FOR$WRITE_IL\n", n)
			} else {
				fmt.Fprintf(&code, "\tPUSHAB\tFM%d\n\tPUSHAB\tIO%d\n\tCALLS\t#2,G^FOR$WRITE_IO\n", n, n)
			}

			length := c.size
			if c.op == "wia" {
				length *= 3
			}

			fmt.Fprintf(&shows, "\tPUSHL\t#%d\n\tPUSHAB\tTEXTS\n\tPUSHL\t#99\n\tCALLS\t#3,SHOWT\n", length)
		case "ria":
			input = true
			fmt.Fprintf(&data, "IB%d:\t.ASCII\t%s\n\t.ALIGN\tLONG\nIO%d:\t.WORD\t%d\n\t.BYTE\t14,4\n\t.ADDRESS\tIB%d\n\t.BYTE\t0,0,^XC0,1\n\t.LONG\t%d\n\t.ADDRESS\tIB%d\n\t.LONG\t%d\n\t.LONG\t1,%d\n",
				n, ascii(c.text), n, c.size, n, len(c.text), n, len(c.text)/c.size, len(c.text)/c.size)
			fmt.Fprintf(&code, "\tPUSHAB\tFM%d\n\tPUSHAB\tIO%d\n\tCALLS\t#2,G^FOR$READ_IO\n", n, n)
		case "enc":
			fmt.Fprintf(&code, "\tPUSHAB\tTEXTS\n\tPUSHAB\tFM%d\n\tPUSHL\t#%d\n\tCALLS\t#3,G^FOR$ENCODE_MO\n", n, c.size)

			length := c.size
			if length < 1 {
				length = 4
			}

			fmt.Fprintf(&shows, "\tPUSHL\t#%d\n\tPUSHAB\tTEXTS\n\tPUSHL\t#99\n\tCALLS\t#3,SHOWT\n", length)
		case "dec":
			input = true
			fmt.Fprintf(&data, "IB%d:\t.ASCII\t%s\n", n, ascii(c.text))
			fmt.Fprintf(&code, "\tPUSHAB\tIB%d\n\tPUSHAB\tFM%d\n\tPUSHL\t#%d\n\tCALLS\t#3,G^FOR$DECODE_MO\n", n, n, c.size)
		case "raw":
			code.WriteString(c.code)
			code.WriteString("\tRET\n")

			for _, s := range c.show {
				fmt.Fprintf(&shows, "%s", s)
			}
		default:
			log.Fatalf("op %q", c.op)
		}

		if c.op != "raw" {
			for j, it := range c.items {
				if input {
					shows.WriteString(inItem(n, j, it, &data, &code))
				} else {
					outItem(n, j, it, &data, &code)
				}
			}

			code.WriteString("\tCALLS\t#0,G^FOR$IO_END\n\tMOVL\tR0,ENDR0\n\tRET\n")
		}

		fmt.Fprintf(&main, "\tCALLS\t#0,C%d\n\tPUSHL\t#%d\n\tCALLS\t#1,RESULT\n%s", n, n, shows.String())
	}

	return fmt.Sprintf(programText, data.String(), nmlData, code.String(), main.String())
}

const programText = `	.TITLE	FOR80 - Phase 80's probe of FORRTL's formatted I/O
; Written by testdata/for80/gen.go.
	$SSDEF
	.PSECT	D,NOEXE,WRT,LONG
ENDR0:	.LONG	0
UNW:	.LONG	0
LOST:	.LONG	0
CCNT:	.LONG	0
CCOND:	.BLKL	8
CCN:	.BLKL	8
CA2:	.BLKL	8
CA3:	.BLKL	8
E1:	.LONG	0
E2:	.LONG	0
E3:	.LONG	0
E4:	.LONG	0
E5:	.LONG	0
LN:	.LONG	0
BD:	.WORD	240
	.BYTE	14,1
	.ADDRESS BF
BF:	.BLKB	240
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
RES1:	.ASCID	"=!UL r=!XL e=!UL !XL !XL !SL !XL"
RES2:	.ASCID	"=c !XL n=!UL a=!XL !XL"
RES3:	.ASCID	"=u"
SHV1:	.ASCID	" v!UL=!XL"
SHV2:	.ASCID	" v!UL=!XL !XL"
SHV4:	.ASCID	" v!UL=!XL !XL !XL !XL"
SHT:	.ASCID	" v!UL=|!AF|"
LOSTM:	.ASCID	"FOR80: a driver line was lost at unit 6"
ENDM:	.ASCID	"##### the end of the cases"
MSGH:	.ASCID	"##### FOR's messages, by $GETMSG"
MSG1:	.ASCID	"M!UL !XL s=!XL fao=!UB |!AD|"
MSG2:	.ASCID	"M+ |!AD|"
FMTT:	.ASCIZ	~(A)~
MLEN:	.LONG	0
MOUT:	.LONG	0
MBUF:	.BLKB	256
MBD:	.WORD	256
	.BYTE	14,1
	.ADDRESS MBUF
LINEB:	.BLKB	600
LINED:	.WORD	600
	.BYTE	14,1
	.ADDRESS LINEB
	.ALIGN	LONG
SLOTS:	.BLKB	16*16
TEXTS:	.BLKB	64*16
SLOTE:
%s
%s
	.PSECT	C,EXE,NOWRT
; HND records a condition. It continues from one below severe and unwinds from
; the rest, to the frame that called the routine it's established in.
HND:	.WORD	^M<R2>
	MOVL	4(AP),R0
	MOVL	4(R0),R1
	CMPL	R1,#SS$_UNWIND
	BEQL	50$
	MOVL	CCNT,R2
	CMPL	R2,#8
	BGEQ	40$
	MOVL	R1,CCOND[R2]
	MOVL	(R0),CCN[R2]
	MOVL	8(R0),CA2[R2]
	MOVL	12(R0),CA3[R2]
	INCL	CCNT
	BICL3	#^XFFFFFFF8,R1,R1
	CMPL	R1,#4
	BEQL	40$
	MOVL	#SS$_CONTINUE,R0
	RET
40$:	INCL	UNW
	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
; PUTH is PUTT's handler: a driver line that can't be written is counted, and
; the log says so.
PUTH:	.WORD	^M<>
	MOVL	4(AP),R0
	CMPL	4(R0),#SS$_UNWIND
	BEQL	50$
	INCL	LOST
	PUSHAB	LOSTM
	CALLS	#1,G^LIB$PUT_OUTPUT
	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
; PUTT(descriptor) writes a line at unit 6, format (A).
	.ENTRY	PUTT,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	PUTH,(FP)
	PUSHAB	FMTT
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	4(AP)
	CALLS	#1,G^FOR$IO_T_DS
	CALLS	#0,G^FOR$IO_END
	RET
; PRE readies a case.
	.ENTRY	PRE,^M<R2,R3,R4,R5>
	MOVC5	#0,SLOTS,#^X55,#SLOTE-SLOTS,SLOTS
	CLRL	CCNT
	CLRL	UNW
	MOVL	#^XDEADBEEF,ENDR0
	RET
; SYNC(descriptor) reads SYS$INPUT up to the case's marker line.
	.ENTRY	SYNC,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	HND,(FP)
10$:	MOVC5	#0,LINEB,#^A/ /,#600,LINEB
	PUSHAB	FMTT
	PUSHL	#5
	CALLS	#2,G^FOR$READ_SO
	PUSHAB	LINED
	CALLS	#1,G^FOR$IO_T_DS
	CALLS	#0,G^FOR$IO_END
	MOVL	4(AP),R0
	MOVZWL	(R0),R1
	MOVL	4(R0),R2
	CMPC3	R1,(R2),LINEB
	BNEQ	10$
	MOVL	4(AP),R0
	MOVZWL	(R0),R1
	CMPB	LINEB(R1),#^A/ /
	BNEQ	10$
	RET
; RESULT(n) shows what the case left.
	.ENTRY	RESULT,^M<R2,R3,R4,R5,R6>
	MOVAL	E5,-(SP)
	MOVAL	E4,-(SP)
	MOVAL	E3,-(SP)
	MOVAL	E2,-(SP)
	MOVAL	E1,-(SP)
	CALLS	#5,G^FOR$ERRSNS
	$FAO_S	CTRSTR=RES1,OUTLEN=LD,OUTBUF=BD,-
		P1=4(AP),P2=ENDR0,P3=E1,P4=E2,P5=E3,P6=E4,P7=E5
	PUSHAB	LD
	CALLS	#1,PUTT
	CLRL	R6
10$:	CMPL	R6,CCNT
	BGEQ	30$
	MOVL	CA3[R6],R3
	CMPL	R3,#^XFFFF
	BLEQU	20$
	CLRL	R3
20$:	MOVL	CA2[R6],R2
	CMPL	R2,#^X1FF
	BLEQU	25$
	CLRL	R2
25$:	$FAO_S	CTRSTR=RES2,OUTLEN=LD,OUTBUF=BD,P1=CCOND[R6],P2=CCN[R6],P3=R2,P4=R3
	PUSHAB	LD
	CALLS	#1,PUTT
	INCL	R6
	BRW	10$
30$:	TSTL	UNW
	BEQL	40$
	PUSHAB	RES3
	CALLS	#1,PUTT
40$:	RET
; SHOWV(slot, address, count of longwords)
	.ENTRY	SHOWV,^M<R2,R3,R4,R5,R6,R7>
	MOVL	8(AP),R2
	CMPL	12(AP),#1
	BNEQ	10$
	$FAO_S	CTRSTR=SHV1,OUTLEN=LD,OUTBUF=BD,P1=4(AP),P2=(R2)
	BRB	30$
10$:	CMPL	12(AP),#2
	BNEQ	20$
	$FAO_S	CTRSTR=SHV2,OUTLEN=LD,OUTBUF=BD,P1=4(AP),P2=(R2),P3=4(R2)
	BRB	30$
20$:	$FAO_S	CTRSTR=SHV4,OUTLEN=LD,OUTBUF=BD,P1=4(AP),P2=(R2),P3=4(R2),P4=8(R2),P5=12(R2)
30$:	PUSHAB	LD
	CALLS	#1,PUTT
	RET
; SHOWT(slot, address, length)
	.ENTRY	SHOWT,^M<R2,R3,R4,R5,R6,R7>
	$FAO_S	CTRSTR=SHT,OUTLEN=LD,OUTBUF=BD,P1=4(AP),P2=12(AP),P3=8(AP)
	PUSHAB	LD
	CALLS	#1,PUTT
	RET
; MESSAGES shows every FOR$ number's text. FOR's values are 00188000 plus eight
; times the number; the severity comes back in the text.
	.ENTRY	MESSAGES,^M<R2,R3,R4,R5,R6,R7>
	PUSHAB	MSGH
	CALLS	#1,PUTT
	CLRL	R6
10$:	MULL3	#8,R6,R7
	ADDL2	#^X188002,R7
	MOVW	#256,MBD
	$GETMSG_S MSGID=R7,MSGLEN=MLEN,BUFADR=MBD,FLAGS=#15,OUTADR=MOUT
	MOVL	R0,R5
	EXTZV	#8,#8,MOUT,R4
	MOVZWL	MLEN,R3
	MOVL	R3,R2
	CMPL	R2,#100
	BLEQ	20$
	MOVL	#100,R2
20$:	$FAO_S	CTRSTR=MSG1,OUTLEN=LD,OUTBUF=BD,P1=R6,P2=R7,P3=R5,P4=R4,P5=R2,P6=#MBUF
	PUSHAB	LD
	CALLS	#1,PUTT
	CMPL	R3,#100
	BLEQ	30$
	SUBL3	#100,R3,R2
	MOVAB	MBUF+100,R3
	$FAO_S	CTRSTR=MSG2,OUTLEN=LD,OUTBUF=BD,P1=R2,P2=R3
	PUSHAB	LD
	CALLS	#1,PUTT
30$:	INCL	R6
	CMPL	R6,#127
	BGTR	40$
	BRW	10$
40$:	RET
%s
	.ENTRY	FOR80,^M<>
%s	PUSHAB	ENDM
	CALLS	#1,PUTT
	CALLS	#0,MESSAGES
	MOVL	#1,R0
	RET
	.END	FOR80
`

// run writes for80r.com: FOR80, then its data, each case's lines under its marker.
func runProc() string {
	var b strings.Builder

	b.WriteString(`$ ! FOR80R.COM - runs FOR80, with the records its unit 5 cases read.
$ ! Written by testdata/for80/gen.go. Each case's lines are under a line
$ ! holding its number, which the driver reads up to (SYNC).
$ RUN FOR80
$ DECK
`)

	// The last cases read the input's end, and have no lines at all.
	lastData := 0

	for n, c := range cases {
		if len(c.lines) > 0 {
			lastData = n
		}
	}

	for n, c := range cases {
		if len(c.lines) == 0 && c.op != "rd5" && c.op != "rl5" {
			continue
		}

		if n > lastData {
			break
		}

		fmt.Fprintf(&b, "#%d\n", n)

		for _, l := range c.lines {
			b.WriteString(l + "\n")
		}

		// A case that reads past its own lines must stop before the next
		// marker: at a group, for a namelist, which searches for its own, and
		// at any / for the others (an error for a number, the end for a list).
		if c.op == "raw" {
			b.WriteString(" $GRP $END\n")
		}

		if n < lastData {
			b.WriteString("/\n/\n/\n")
		}
	}

	b.WriteString("$ EOD\n$ EXIT\n")

	return b.String()
}

const dumpProgram = `	.TITLE	FOR80D - prints FOR80.DAT's records between bars
; Written by testdata/for80/gen.go. A record's bytes as stored: a first byte
; that carriage control would use shows here, and trailing blanks. A byte that
; doesn't print shows as a period, and a record past 300 bytes shows its first
; 300 and its length.
	$RMSDEF
	.PSECT	D,NOEXE,WRT,LONG
FAB:	$FAB	FNM=<FOR80.DAT>,FAC=<GET>
RAB:	$RAB	FAB=FAB,UBF=REC,USZ=32767
REC:	.BLKB	32767
BF:	.BLKB	400
BD:	.WORD	400
	.BYTE	14,1
	.ADDRESS BF
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
CS:	.ASCID	"|!AF|"
CL:	.ASCID	"|!AF|..!UL"
CE:	.ASCID	"$GET error !XL"
ERRS:	.LONG	20
	.PSECT	C,EXE,NOWRT
	.ENTRY	FOR80D,^M<R2>
	$OPEN	FAB=FAB
	BLBS	R0,5$
	BRW	90$
5$:	$CONNECT RAB=RAB
	BLBS	R0,10$
	BRW	90$
10$:	$GET	RAB=RAB
	BLBS	R0,15$
	BRW	80$
15$:	MOVZWL	RAB+RAB$W_RSZ,R2
	CMPL	R2,#300
	BLEQ	20$
	$FAO_S	CTRSTR=CL,OUTLEN=LD,OUTBUF=BD,P1=#300,P2=#REC,P3=R2
	BRB	30$
20$:	$FAO_S	CTRSTR=CS,OUTLEN=LD,OUTBUF=BD,P1=R2,P2=#REC
30$:	PUSHAB	LD
	CALLS	#1,G^LIB$PUT_OUTPUT
	BRW	10$
80$:	CMPL	R0,#RMS$_EOF
	BEQL	85$
	MOVL	R0,R2
	$FAO_S	CTRSTR=CE,OUTLEN=LD,OUTBUF=BD,P1=R2
	PUSHAB	LD
	CALLS	#1,G^LIB$PUT_OUTPUT
	SOBGTR	ERRS,70$
	BRB	85$
70$:	BRW	10$
85$:	$CLOSE	FAB=FAB
	MOVL	#1,R0
90$:	RET
	.END	FOR80D
`

// Each part of FOR80E has no handler, so VMS's own message and traceback show.
var errorParts = []struct{ letter, what, code string }{
	{"A", "OUTCONERR, then the statement goes on", "\tPUSHAB\tFI2\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHL\t#3244\n\tCALLS\t#1,G^FOR$IO_L_V\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"B", "FORVARMIS", "\tPUSHAB\tFI5\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHAL\tV1\n\tCALLS\t#1,G^FOR$IO_F_R\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"C", "SYNERRFOR", "\tPUSHAB\tFBAD\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHL\t#1\n\tCALLS\t#1,G^FOR$IO_L_V\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"D", "INFFORLOO", "\tPUSHAB\tFLOOP\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHL\t#1\n\tCALLS\t#1,G^FOR$IO_L_V\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"E", "INPCONERR, from an internal record", "\tPUSHAB\tFI3\n\tPUSHAB\tTABC\n\tCALLS\t#2,G^FOR$READ_IO\n\tPUSHAL\tV2\n\tCALLS\t#1,G^FOR$IO_L_R\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"F", "INPCONERR, from SYS$INPUT", "\tPUSHAB\tFI3\n\tPUSHL\t#5\n\tCALLS\t#2,G^FOR$READ_SO\n\tPUSHAL\tV2\n\tCALLS\t#1,G^FOR$IO_L_R\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"G", "ENDDURREA, at SYS$INPUT's end", "\tPUSHAB\tFI3\n\tPUSHL\t#5\n\tCALLS\t#2,G^FOR$READ_SO\n\tPUSHAL\tV2\n\tCALLS\t#1,G^FOR$IO_L_R\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"H", "OUTSTAOVE, an internal record", "\tPUSHAB\tFI5\n\tPUSHAB\tT2\n\tCALLS\t#2,G^FOR$WRITE_IO\n\tPUSHL\t#1\n\tCALLS\t#1,G^FOR$IO_L_V\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"I", "INVLOGUNI", "\tPUSHAB\tFI5\n\tPUSHL\t#1000\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHL\t#1\n\tCALLS\t#1,G^FOR$IO_L_V\n\tCALLS\t#0,G^FOR$IO_END\n"},
	{"J", "RECIO_OPE", "\tPUSHAB\tFI5\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tPUSHAB\tFI5\n\tPUSHL\t#6\n\tCALLS\t#2,G^FOR$WRITE_SO\n\tCALLS\t#0,G^FOR$IO_END\n"},
}

func errorProgram() string {
	var parts, dispatch strings.Builder

	for _, p := range errorParts {
		fmt.Fprintf(&parts, "; %s\n\t.ENTRY\tPART_%s,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n%s\tRET\n", p.what, p.letter, p.code)
		fmt.Fprintf(&dispatch, "\tCMPB\tPART,#^A/%s/\n\tBNEQ\t%s$\n\tCALLS\t#0,PART_%s\n", p.letter, "1"+fmt.Sprint(int(p.letter[0])-'A'), p.letter)
		fmt.Fprintf(&dispatch, "1%d$:\n", int(p.letter[0])-'A')
	}

	return fmt.Sprintf(`	.TITLE	FOR80E - FOR's conditions with no handler
; Written by testdata/for80/gen.go. The part is the foreign command's text.
	.PSECT	D,NOEXE,WRT,LONG
PART:	.BLKB	8
PD:	.WORD	8
	.BYTE	14,1
	.ADDRESS PART
FI2:	.ASCIZ	~(1X,I2)~
FI3:	.ASCIZ	~(I3)~
FI5:	.ASCIZ	~(1X,I5)~
FBAD:	.ASCIZ	~(1X,J3)~
FLOOP:	.ASCIZ	~(1X,'A')~
V1:	.F_FLOATING 3.25
V2:	.LONG	0
TABC:	.ASCID	~ABC~
TB:	.BLKB	2
T2:	.WORD	2
	.BYTE	14,1
	.ADDRESS TB
AFTER:	.ASCID	~FOR80E: the statement returned~
	.PSECT	C,EXE,NOWRT
%s
	.ENTRY	FOR80E,^M<>
	PUSHAB	PD
	CALLS	#1,G^LIB$GET_FOREIGN
%s	PUSHAB	AFTER
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVL	#1,R0
	RET
	.END	FOR80E
`, parts.String(), dispatch.String())
}

const terminalProgram = `	.TITLE	FOR80T - carriage control at a terminal
; Written by testdata/for80/gen.go. Run by hand, with a log of the console: what
; matters is where the terminal's lines fall, which only a real terminal shows.
	.PSECT	D,NOEXE,WRT,LONG
ANS:	.BLKB	20
AD:	.WORD	20
	.BYTE	14,1
	.ADDRESS ANS
F1:	.ASCIZ	~(' blank control')~
F2:	.ASCIZ	~('+plus: over the last line')~
F3:	.ASCIZ	~('0zero: a blank line before')~
F4:	.ASCIZ	~('1one: a form feed before')~
F5:	.ASCIZ	~('$dollar: no return after')~
F6:	.ASCIZ	~(' after dollar')~
F7:	.ASCIZ	~('Xother: as blank')~
F8:	.ASCIZ	~(' with $',$)~
F9:	.ASCIZ	~(' after $')~
F10:	.ASCIZ	~('+plus with $',$)~
F11:	.ASCIZ	~('+ after plus with $')~
F12:	.ASCIZ	~('0zero with $',$)~
F13:	.ASCIZ	~('1one with $',$)~
F14:	.ASCIZ	~(' Type ABC: ',$)~
F15:	.ASCIZ	~(A)~
F16:	.ASCIZ	~(' You typed |',A,'|')~
F17:	.ASCIZ	~(/,' a blank record first')~
F18:	.ASCIZ	~(' two records',/,' here')~
F19:	.ASCIZ	~()~
	.PSECT	C,EXE,NOWRT
	.MACRO	PR	FM
	PUSHAB	FM
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	CALLS	#0,G^FOR$IO_END
	.ENDM	PR
	.ENTRY	FOR80T,^M<>
	PR	F1
	PR	F2
	PR	F3
	PR	F4
	PR	F1
	PR	F5
	PR	F6
	PR	F7
	PR	F8
	PR	F9
	PR	F10
	PR	F11
	PR	F12
	PR	F1
	PR	F13
	PR	F1
	PR	F17
	PR	F18
	PR	F19
	PR	F1
	PR	F14
	PUSHAB	F15
	PUSHL	#5
	CALLS	#2,G^FOR$READ_SO
	PUSHAB	AD
	CALLS	#1,G^FOR$IO_T_DS
	CALLS	#0,G^FOR$IO_END
	PUSHAB	F16
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHAB	AD
	CALLS	#1,G^FOR$IO_T_DS
	CALLS	#0,G^FOR$IO_END
	MOVL	#1,R0
	RET
	.END	FOR80T
`

const procedure = `$ ! FOR80.COM - the Phase 80 probe of FORRTL's formatted I/O.
$ ! Written by testdata/for80/gen.go. Run it with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @FOR80/OUTPUT=FOR80.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ DEFINE/USER_MODE SYS$OUTPUT FOR80.MLG
$ MACRO/NOLIST FOR80
$ DEFINE/USER_MODE SYS$OUTPUT FOR80.LLG
$ LINK/NOMAP FOR80,FOR80/OPTIONS
$ MACRO/NOLIST FOR80D
$ LINK/NOMAP FOR80D
$ MACRO/NOLIST FOR80E
$ LINK/NOMAP/TRACEBACK FOR80E,FOR80/OPTIONS
$ MACRO/NOLIST FOR80T
$ LINK/NOMAP FOR80T,FOR80/OPTIONS
$ !
$ ! Run 1: unit 6 is a file, so its records keep their first bytes.
$ DEFINE/NOLOG FOR006 FOR80.DAT
$ @FOR80R
$ SHOW SYMBOL $STATUS
$ DEASSIGN FOR006
$ DIRECTORY/FULL FOR80.DAT
$ DEFINE/USER_MODE SYS$OUTPUT FOR80-VAX.TXT
$ RUN FOR80D
$ !
$ ! Run 2: unit 6 is SYS$OUTPUT, a process-permanent file, as a batch job's is.
$ @FOR80R/OUTPUT=FOR80P.OUT
$ SHOW SYMBOL $STATUS
$ !
$ ! Run 3: each condition with no handler.
$ FOR80E := $'F$ENVIRONMENT("DEFAULT")'FOR80E
$ FOR80E A
$ SHOW SYMBOL $STATUS
$ FOR80E B
$ SHOW SYMBOL $STATUS
$ FOR80E C
$ SHOW SYMBOL $STATUS
$ FOR80E D
$ SHOW SYMBOL $STATUS
$ FOR80E E
$ SHOW SYMBOL $STATUS
$ FOR80E F
ABC
$ SHOW SYMBOL $STATUS
$ FOR80E G
$ SHOW SYMBOL $STATUS
$ FOR80E H
$ SHOW SYMBOL $STATUS
$ FOR80E I
$ SHOW SYMBOL $STATUS
$ FOR80E J
$ SHOW SYMBOL $STATUS
$ DIRECTORY/SIZE FOR*.*
$ EXIT
`

func main() {
	table()

	files := map[string]string{
		"for80.mar":  program(),
		"for80d.mar": dumpProgram,
		"for80e.mar": errorProgram(),
		"for80t.mar": terminalProgram,
		"for80.opt":  "SYS$SHARE:FORRTL/SHAREABLE\n",
		"for80r.com": runProc(),
		"for80.com":  procedure,
	}

	var copies strings.Builder

	for _, name := range []string{"for80.mar", "for80d.mar", "for80e.mar", "for80t.mar", "for80.opt", "for80r.com", "for80.com"} {
		fmt.Fprintf(&copies, "COPY \"testdata/for80/%s\"/HOST DUA2:[000000]%s\n", name, strings.ToUpper(name))
	}

	files["exchange.cmd"] = `! EXCHANGE.CMD - builds the Phase 80 probe's exchange volume with govax.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/for80-exchange.dsk" /DEVICE=RD51 FOR80
MOUNT/WRITE DUA2 "testdata/disks/for80-exchange.dsk"
` + copies.String() + `DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`

	files["copyout.cmd"] = `! COPYOUT.CMD - copies the VAX run's output off the exchange volume.
! Written by testdata/for80/gen.go. Run from the repository root:
!
!     govax console < testdata/for80/copyout.cmd
!
MOUNT DUA2 "testdata/disks/for80-exchange.dsk"
COPY DUA2:[000000]FOR80-VAX.TXT "testdata/for80/vax/for80-vax.txt"/HOST
COPY DUA2:[000000]FOR80P.OUT "testdata/for80/vax/for80p-vax.txt"/HOST
COPY DUA2:[000000]FOR80.LOG "testdata/for80/vax/for80.log"/HOST
DISMOUNT DUA2
`

	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println(len(cases), "cases")
}
