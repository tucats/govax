//go:build ignore

// gen.go writes the probe of NAMELIST I/O (docs/PHASE-80 - FORTRAN formatted
// IO.md, subtask 11). One table of cases becomes two programs that print the
// same lines: nml80.mar calls VMS's FORRTL, whose output is FTNRTL's
// requirement, and nml80f.for is the same cases as FORTRAN statements, for
// FTNRTL. Run this from the repository root:
//
//	go run testdata/nml80/gen.go
//
// It writes
//
//	nml80.mar     the cases, on VMS's FORRTL
//	nml80f.for    the same cases as FORTRAN, for FTNRTL
//	nml80r.com    runs either image, with the records its READs take
//	nml80e.mar    one READ with no handler, for the conditions' messages
//	nml80e.for    the same READ as FORTRAN
//	nml80e.com    runs either once per set of records
//	nml80.opt     the link options, on VMS
//	nml80.com     builds and runs everything, on VMS
//	exchange.cmd, copyout.cmd  the govax console scripts for the exchange volume
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/nml80"

// entity is a variable or array of a group. kind is I4, I2, L4, L2, L1, R4, R8,
// RG, R16, C8, C16, CG, or Tn. bounds is lower and upper bound pairs, empty for
// a scalar; values are its initial values, in FORTRAN's form.
type entity struct {
	name   string
	kind   string
	bounds []int
	values []string
}

// kase is one case: a read of the main group from its lines (reads times), or a
// write of a group of its own.
type kase struct {
	title string
	lines []string
	reads int
	g     bool     // the G_floating group
	group string   // a write's group's name
	ents  []entity // a write's entities
}

var (
	cases    []kase
	sections = map[int]string{}
)

func section(s string) { sections[len(cases)] = s }

func rd(lines ...string) { cases = append(cases, kase{lines: lines, reads: 1}) }

func rd2(lines ...string) { cases = append(cases, kase{lines: lines, reads: 2}) }

func rdg(lines ...string) { cases = append(cases, kase{lines: lines, reads: 1, g: true}) }

func wr(title string, ents ...entity) {
	cases = append(cases, kase{title: title, group: "GRP", ents: ents})
}

func wrn(title, group string, ents ...entity) {
	cases = append(cases, kase{title: title, group: group, ents: ents})
}

func e(name, kind string, values ...string) entity {
	return entity{name: name, kind: kind, values: values}
}

func a(name, kind string, upper int, values ...string) entity {
	return entity{name: name, kind: kind, bounds: []int{1, upper}, values: values}
}

// The main group: every type, as scalars and arrays.
var mainGroup = []entity{
	e("I", "I4", "42"),
	e("J", "I2", "43"),
	e("L", "L4", ".TRUE."),
	e("M", "L2", ".FALSE."),
	e("N", "L1", ".TRUE."),
	e("R", "R4", "2.5"),
	e("D", "R8", "3.5"),
	e("Q", "R16", "4.5"),
	e("C", "C8", "(1.5,-2.5)"),
	e("E", "C16", "(3.5,-4.5)"),
	e("T", "T5", "'HELLO'"),
	a("A", "I4", 3, "1", "1", "1"),
	{name: "B", kind: "I4", bounds: []int{1, 2, 1, 2}, values: []string{"1", "2", "3", "4"}},
	{name: "AL", kind: "I4", bounds: []int{-1, 1}, values: []string{"7", "8", "9"}},
	a("RA", "R4", 3, "1.0", "1.0", "1.0"),
	a("TA", "T3", 3, "'AAA'", "'BBB'", "'CCC'"),
	a("CA", "C8", 2, "(1.0,1.0)", "(2.0,2.0)"),
	a("LA", "L4", 2, ".TRUE.", ".FALSE."),
	e("LONGNAME1", "I4", "-5"),
}

// The G_floating group.
var gGroup = []entity{
	e("G", "RG", "2.5"),
	e("H", "CG", "(1.5,-2.5)"),
	a("GA", "RG", 2, "1.0", "1.0"),
}

func table() {
	section("Finding the group")
	rd(" $GRP I=7 $END")
	rd(" &GRP I=7 &END")
	rd(" $grp i=7 $end")
	rd("$GRP I=7 $END")
	rd("X$GRP I=7 $END")
	rd("  $GRP I=7 $END")
	rd("     $GRP I=7 $END")
	rd("\t$GRP I=7 $END")
	rd(" X $GRP I=7 $END")
	rd(" I=7 $GRP I=8 $END")
	rd(" IGNORED", " $GRP I=7 $END")
	rd(" $OTHER I=7 $END", " $GRP I=8 $END")
	rd(" $GRPX I=7 $END", " $GRP I=8 $END")
	rd(" $GR I=7 $END", " $GRP I=8 $END")
	rd(" $ GRP I=7 $END", " $GRP I=8 $END")
	rd(" $OTHER $GRP I=7 $END", " $GRP I=8 $END")
	rd(" $GRP,I=7 $END")
	rd(" $GRP=7 $END")
	rd(" $GRPI=7 $END", " $GRP I=8 $END")
	rd(" $GRP")
	rd(" $GRP", " I=7", " $END")
	rd(" $GRP", "", " I=7", "", " $END")
	rd("")
	rd(" $")
	rd(" &")

	section("Ending the group")
	rd(" $GRP I=7 $")
	rd(" $GRP I=7 &")
	rd(" $GRP I=7 &END")
	rd(" &GRP I=7 $END")
	rd(" $GRP I=7$END")
	rd(" $GRP I=7 $E")
	rd(" $GRP I=7 $ENDX")
	rd(" $GRP I=7 $OTHER")
	rd(" $GRP I=7 $END R=1")
	rd(" $GRP I=7 $END", " R=1")
	rd(" $GRP $END")
	rd(" $GRP $")
	rd(" $GRP I=7 $ R=9")
	rd(" $GRP I=7 /")
	rd(" $GRP / I=7 $END")
	rd(" $GRP I=7, $END")
	rd(" $GRP T='A$B' $END")
	rd(" $GRP I=$END")
	rd(" $GRP I= $END")
	rd(" $GRP R=2.5$END")
	rd(" $GRP A=1,2$END")
	rd(" $GRP L=T$END")
	rd(" $GRP C=(1,2)$END")
	rd(" $GRP T='X'$END")
	rd2(" $GRP I=7 $END", " $GRP R=1 $END")
	rd2(" $GRP I=7 $END $GRP R=1 $END", " $GRP J=9 $END")

	section("Separators between assignments")
	rd(" $GRP I = 7 $END")
	rd(" $GRP I =7 $END")
	rd(" $GRP I= 7 $END")
	rd(" $GRP\tI\t=\t7\t$END")
	rd(" $GRP I=7 R=1 $END")
	rd(" $GRP I=7,R=1 $END")
	rd(" $GRP I=7 , R=1 $END")
	rd(" $GRP I=7,", " R=1 $END")
	rd(" $GRP I=7", " ,R=1 $END")
	rd(" $GRP I=7,,R=1 $END")
	rd(" $GRP ,I=7 $END")
	rd(" $GRP I=7 ,, $END")
	rd(" $GRP I=", " 7 $END")
	rd(" $GRP I", " =7 $END")
	rd(" $GRP I=7 I=8 $END")
	rd(" $GRP A(1)=7 A(1)=8 $END")

	section("Scalars and how many values")
	rd(" $GRP I=7,8 $END")
	rd(" $GRP I=7 8 $END")
	rd(" $GRP I=2*7 $END")
	rd(" $GRP I=1*7 $END")
	rd(" $GRP I=0*7 $END")
	rd(" $GRP I=1* $END")
	rd(" $GRP I=2* $END")
	rd(" $GRP I=, $END")
	rd(" $GRP I=,, $END")
	rd(" $GRP I=,8 $END")
	rd(" $GRP I=,,R=1 $END")
	rd(" $GRP I=, R=1 $END")
	rd(" $GRP I=7, 8, R=1 $END")
	rd(" $GRP R=1,2 $END")
	rd(" $GRP T='A','B' $END")
	rd(" $GRP L=T,F $END")
	rd(" $GRP L=T F $END")
	rd(" $GRP C=(1,2),(3,4) $END")

	section("Names")
	rd(" $GRP i=7 $END")
	rd(" $GRP NOPE=1 $END")
	rd(" $GRP NOPE $END")
	rd(" $GRP LONGNAME1=7 $END")
	rd(" $GRP LONGNAME=7 $END")
	rd(" $GRP LONGNAME12=7 $END")
	rd(" $GRP LONG NAME1=7 $END")
	rd(" $GRP GRP=7 $END")
	rd(" $GRP I 7 $END")
	rd(" $GRP I $END")
	rd(" $GRP =7 $END")
	rd(" $GRP 7 $END")
	rd(" $GRP 7=1 $END")
	rd(" $GRP 'I'=7 $END")
	rd(" $GRP I==7 $END")
	rd(" $GRP I=7 8=9 $END")
	rd(" $GRP I=7 R $END")
	rd(" $GRP I=7 NOPE=1 $END")
	rd(" $GRP I=7 NOPE $END")
	rd(" $GRP A=1 2 I=3 $END")
	rd(" $GRP A=1 2 NOPE=3 $END")
	rd(" $GRP A=1 2 NOPE $END")
	rd(" $GRP LA=T F I=3 $END")
	rd(" $GRP LA=T I=3 $END")
	rd(" $GRP LA=T L=F $END")
	rd(" $GRP LA=T,L=F $END")
	rd(" $GRP LA=F T=1 $END")
	rd(" $GRP LA=T(1)='X' $END")

	section("Arrays: values")
	rd(" $GRP A=5,6,7 $END")
	rd(" $GRP A=5 6 7 $END")
	rd(" $GRP A=5 , 6 , 7 $END")
	rd(" $GRP A=5,", " 6,", " 7 $END")
	rd(" $GRP A=5,6 $END")
	rd(" $GRP A=5,6,7,8 $END")
	rd(" $GRP A=5,6,7,8,9 $END")
	rd(" $GRP A=5,,7 $END")
	rd(" $GRP A=,6 $END")
	rd(" $GRP A=,,7 $END")
	rd(" $GRP A=5,6, $END")
	rd(" $GRP A=5,6,7, $END")
	rd(" $GRP A=5,6,7,, $END")
	rd(" $GRP A=,,,8 $END")
	rd(" $GRP A=3*5 $END")
	rd(" $GRP A=2*5 $END")
	rd(" $GRP A=4*5 $END")
	rd(" $GRP A=2*5,6 $END")
	rd(" $GRP A=2*5,6,7 $END")
	rd(" $GRP A=1*5,6 $END")
	rd(" $GRP A=0*5,6 $END")
	rd(" $GRP A=2*,6 $END")
	rd(" $GRP A=2* 6 $END")
	rd(" $GRP A=3* $END")
	rd(" $GRP A=4* $END")
	rd(" $GRP A=2 * 5 $END")
	rd(" $GRP A=2* 5,6 $END")
	rd(" $GRP A=5,6,7 R=1 $END")
	rd(" $GRP A=5,6,7,R=1 $END")
	rd(" $GRP A=5,6,R=1 $END")
	rd(" $GRP A=5,6,,R=1 $END")
	rd(" $GRP RA=1.5,2.5 $END")
	rd(" $GRP RA=2*1.5 $END")
	rd(" $GRP LA=T,T $END")
	rd(" $GRP LA=2*F $END")
	rd(" $GRP TA='X','Y','Z' $END")
	rd(" $GRP TA=2*'QQ' $END")
	rd(" $GRP TA='A','B','C','D' $END")
	rd(" $GRP CA=(5,6),(7,8) $END")
	rd(" $GRP CA=2*(5,6) $END")
	rd(" $GRP CA=(5,6) (7,8) $END")
	rd(" $GRP CA=,(7,8) $END")
	rd(" $GRP CA=1,2 $END")
	rd(" $GRP B=5,6,7,8 $END")
	rd(" $GRP B=5,6 $END")
	rd(" $GRP B=4*5 $END")
	rd(" $GRP B=5*5 $END")
	rd(" $GRP AL=5,6,7 $END")

	section("Arrays: subscripts")
	rd(" $GRP A(2)=5 $END")
	rd(" $GRP A(1)=5 $END")
	rd(" $GRP A(3)=5 $END")
	rd(" $GRP A(0)=5 $END")
	rd(" $GRP A(4)=5 $END")
	rd(" $GRP A(-1)=5 $END")
	rd(" $GRP A(+2)=5 $END")
	rd(" $GRP A( 2 )=5 $END")
	rd(" $GRP A (2)=5 $END")
	rd(" $GRP A(2) =5 $END")
	rd(" $GRP A(2)= 5 $END")
	rd(" $GRP A(2)=5,6 $END")
	rd(" $GRP A(2)=5,6,7 $END")
	rd(" $GRP A(2)=2*5 $END")
	rd(" $GRP A(2)=3*5 $END")
	rd(" $GRP A(2)=,6 $END")
	rd(" $GRP A(2)=,,7 $END")
	rd(" $GRP A(3)=5,6 $END")
	rd(" $GRP A(1,1)=5 $END")
	rd(" $GRP A()=5 $END")
	rd(" $GRP A(2=5 $END")
	rd(" $GRP A(2 $END")
	rd(" $GRP A(I)=5 $END")
	rd(" $GRP A(1.5)=5 $END")
	rd(" $GRP A(1+1)=5 $END")
	rd(" $GRP A(99999999999)=5 $END")
	rd(" $GRP A(2)(1:1)=5 $END")
	rd(" $GRP A(1:2)=5 $END")
	rd(" $GRP A(2", " )=5 $END")
	rd(" $GRP B(1,2)=5 $END")
	rd(" $GRP B(2,1)=5 $END")
	rd(" $GRP B(2,2)=5 $END")
	rd(" $GRP B( 1 , 2 )=5 $END")
	rd(" $GRP B(2)=5 $END")
	rd(" $GRP B(3,1)=5 $END")
	rd(" $GRP B(1,3)=5 $END")
	rd(" $GRP B(0,1)=5 $END")
	rd(" $GRP B(1,2,1)=5 $END")
	rd(" $GRP B(2,1)=5,6,7 $END")
	rd(" $GRP B(2,1)=5,6,7,8 $END")
	rd(" $GRP B(1,)=5 $END")
	rd(" $GRP B(,1)=5 $END")
	rd(" $GRP AL(-1)=5 $END")
	rd(" $GRP AL(0)=5 $END")
	rd(" $GRP AL(0)=5,6 $END")
	rd(" $GRP AL(0)=5,6,7 $END")
	rd(" $GRP AL(1)=5 $END")
	rd(" $GRP AL(-2)=5 $END")
	rd(" $GRP AL(2)=5 $END")
	rd(" $GRP I(1)=5 $END")
	rd(" $GRP R(1)=5 $END")
	rd(" $GRP CA(2)=(5,6) $END")
	rd(" $GRP LA(2)=T $END")
	rd(" $GRP RA(2)=1.5,2.5 $END")

	section("Character values and substrings")
	rd(" $GRP T='ABC' $END")
	rd(" $GRP T='' $END")
	rd(" $GRP T=' ' $END")
	rd(" $GRP T='ABCDE' $END")
	rd(" $GRP T='ABCDEFGH' $END")
	rd(" $GRP T='A''B' $END")
	rd(" $GRP T='''' $END")
	rd(" $GRP T='AB' 'CD' $END")
	rd(" $GRP T='AB'X $END")
	rd(" $GRP T='AB'R=1 $END")
	rd(" $GRP T='AB',R=1 $END")
	rd(" $GRP T=ABC $END")
	rd(" $GRP T=7 $END")
	rd(" $GRP T=\"ABC\" $END")
	rd(" $GRP T=.TRUE. $END")
	rd(" $GRP T=(1,2) $END")
	rd(" $GRP T='AB", " CD' $END")
	rd(" $GRP T='AB", "CD' $END")
	rd(" $GRP T='AB", " ", " CD' $END")
	rd(" $GRP T='AB")
	rd(" $GRP T(2:3)='ZZ' $END")
	rd(" $GRP T(2:)='ZZ' $END")
	rd(" $GRP T(:2)='ZZ' $END")
	rd(" $GRP T(:)='ZZ' $END")
	rd(" $GRP T(0:2)='ZZ' $END")
	rd(" $GRP T(2:9)='ZZ' $END")
	rd(" $GRP T(3:2)='ZZ' $END")
	rd(" $GRP T(2:2)='ZZ' $END")
	rd(" $GRP T( 2 : 3 )='ZZ' $END")
	rd(" $GRP T (2:3)='ZZ' $END")
	rd(" $GRP T(2:3) ='ZZ' $END")
	rd(" $GRP T(2)='ZZ' $END")
	rd(" $GRP T(2:3)='A','B' $END")
	rd(" $GRP T(2:3)='VERYLONG' $END")
	rd(" $GRP TA(2)='X' $END")
	rd(" $GRP TA(2)='X','Y' $END")
	rd(" $GRP TA(2)='X','Y','Z' $END")
	rd(" $GRP TA(2)(2:)='Z' $END")
	rd(" $GRP TA(2)(2:3)='Y','Z' $END")
	rd(" $GRP TA(2)(1:1)='Y','Z' $END")
	rd(" $GRP TA(2) (2:)='Z' $END")
	rd(" $GRP TA(4)='X' $END")
	rd(" $GRP TA(2:3)='X' $END")
	rd(" $GRP TA(2)(5:6)='X' $END")
	rd(" $GRP I(1:2)=5 $END")
	rd(" $GRP R(1:2)=5 $END")

	section("Numbers to numeric items")
	for _, v := range []string{"-3", "+3", "1.5", "14.6", "1E2", "1.5E3", "1D2", "1Q2", ".5", "5.", "-.5", "1E", "E2", "1.2.3", "2147483647", "2147483648", "-2147483648",
		"T", "F", ".TRUE.", ".FALSE.", "'A'", "(1,2)", "^X10", "\"17", "'10'X", "3HABC", "7ABC", "+", "-", "."} {
		rd(" $GRP I=" + v + " $END")
	}

	for _, v := range []string{"70000", "-32768", "32768", "1.5"} {
		rd(" $GRP J=" + v + " $END")
	}

	for _, v := range []string{"-3", "1", "1.5", "1.5E2", "1.5D2", "1.5Q2", ".5", "1E40", "1E-40", "T", ".TRUE.", "'A'", "(1,2)", "ABC", "1.2.3"} {
		rd(" $GRP R=" + v + " $END")
	}

	for _, v := range []string{"1", "0.1", "1.5D0", "0.1D0", "1D40", "1E40", "1D-40", "T"} {
		rd(" $GRP D=" + v + " $END")
	}

	for _, v := range []string{"1", "0.1", "1.5Q0", "0.1Q0", "1Q4000", "1Q5000"} {
		rd(" $GRP Q=" + v + " $END")
	}

	section("Logical values")
	for _, v := range []string{"T", "F", ".T", ".F", ".T.", ".F.", ".TRUE.", ".FALSE.", "TRUE", "FALSE", "t", "f", ".true.", "TX", "FX", ".X", ".", "1", "0", "-1", "2", "1.5", "'T'", "(1,2)", "X", "ABC", "TT"} {
		rd(" $GRP L=" + v + " $END")
	}

	for _, v := range []string{"T", "F", "1", "300"} {
		rd(" $GRP M=" + v + " $END")
		rd(" $GRP N=" + v + " $END")
	}

	section("Complex values")
	for _, v := range []string{"(5,6)", "( 5 , 6 )", "(5.5,-6.5)", "(1E2,1D2)", "5", "5.5", "(5)", "(5,)", "(,6)", "(,)", "()", "(5,6,7)", "(5 6)", "(5,6", "(A,B)", "T", "'A'", ",", "2*(5,6)", "1*(5,6)", "(5,6)X"} {
		rd(" $GRP C=" + v + " $END")
	}

	rd(" $GRP C=(5,", " 6) $END")
	rd(" $GRP C=(", " 5,6) $END")
	rd(" $GRP C=(5", " ,6) $END")
	rd(" $GRP C=(5,6", " ) $END")
	rd(" $GRP E=(5.5D0,-6.5D0) $END")
	rd(" $GRP E=(0.1,0.2) $END")
	rd(" $GRP E=0.1 $END")

	section("Prompting, at a file")
	rd(" ?")
	rd(" =?")
	rd("?")
	rd("=?")
	rd(" ? $GRP I=7 $END")
	rd(" ?", " $GRP I=7 $END")
	rd(" =?", " $GRP I=7 $END")
	rd(" $GRP ? $END")
	rd(" $GRP I=7 ? $END")
	rd(" $GRP I=? $END")
	rd(" $GRP =? $END")

	section("G_floating")
	rdg(" $GRP G=1.5 $END")
	rdg(" $GRP G=0.1 $END")
	rdg(" $GRP G=0.1D0 $END")
	rdg(" $GRP G=1D300 $END")
	rdg(" $GRP H=(0.1,0.2) $END")
	rdg(" $GRP H=0.1 $END")
	rdg(" $GRP GA=2*1.5 $END")

	section("Round 2: what ends a value")
	for _, v := range []string{"A=1 8=9", "A=8=9", "I=8=9", "A=1 'X'=9", "A=(1,2)=9", "I=1.5=2", "A=1(2)", "I=7)", "I=7'A'", "I=7/", "I=7&END", "I=7*",
		"A=2*7*", "I=1E+", "I=1E+2", "I=1.E2", "I=1E2.5", "I=1EE2", "I=1D", "I=1Q", "I=+.5E1", "I=1 E2", "I=1.5F", "I=1E-1", "R=1.5$END", "M=T$END", "M=.TRUE.$END", "M=TRUE$END", "M=T&END", "A=T,F,T", "A=TX FX", "A=2*T", "A=2*TX",
		"T", "F=1", "TX=1", "A=1 TEST=3", "A=1 TEST", "A=1 FOO", "A=1 BAR", "A=T(1)", "L=F(1)=1", "L=T,R=1", "L=.TRUE.R=1", "M=T1", "M=T_1", "M=T$", "M=T$X", "M=T(", "A=.T,.F", "A=.X", "A=.1.2", "NOPE$=1", "NOPE_1=1", "NOPE$", "M=.T.X", "M=.FX", "M=T$END I=5", "M=TRUE$X I=5", "LONGNAME1$=7", "I=5$X=6", "I=5 $X=6", "M=T$=1"} {
		rd(" $GRP " + v + " $END")
	}

	section("Round 2: complex parts and references")
	for _, v := range []string{"C=(T,F)", "C=(.TRUE.,1)", "C=(1,T)", "C=(1E,2)", "C=(+1,+2)", "C=(1.2.3,2)", "T(2:3)(1:1)='Z'", "TA(2)(1:2)(1:1)='Z'", "T(A:B)='Z'", "T(1.5:2)='Z'", "T(1:2:3)='Z'", "T(1,2)='Z'", "A(1,I)=5", "A(2I)=5", "A($)=5", "A('1')=5", "A(1)5", "T(2:3)5", "A(2)(1)=5", "TA(2)(2)='Z'", "TA(2)(2:3)", "T(+2:+3)='Z'", "T(-1:3)='Z'", "T(99999999999:3)='Z'", "T(1:99999999999)='Z'", "A(0002)=5", "A(+)=5", "A(-)=5", "T(:)(:)='Z'", "A(2,)=5"} {
		rd(" $GRP " + v + " $END")
	}

	section("Round 2: repeats")
	for _, v := range []string{"A=1,0*5", "A=02*5", "A=2*-5", "A=2*+5", "A=2*5*6", "A=2*'A'", "A=99999999999*5", "A=2*(1,2)", "A=2*", "A=2*$END", "A=2*,", "A=2*,,7", "A=1*", "A=1*,5", "T=2*'X'", "T=2*", "T=1*'XY'", "CA=2*5", "A=*5", "A=+2*5", "A=2.*5", "A=2.5*5", "TA=1*'X',1*'Y'"} {
		rd(" $GRP " + v + " $END")
	}

	section("Round 2: question marks at a file")
	rd(" $GRP ? I=7 $END")
	rd(" $GRP ?", " I=7 $END")
	rd(" $GRP I=7 ?", " R=1 $END")
	rd(" $GRP I=? R=1 $END")
	rd(" $GRP =?", " I=7 $END")
	rd(" $GRP ?? I=7 $END")
	rd(" $GRP I=7 ?R=1 $END")
	rd(" $GRP A=1 ? 2 $END")
	rd(" ?X")
	rd(" ?X $GRP I=1 $END")
	rd(" $GRP A=1,?,3 $END")
	rd(" $GRP T='?' $END")

	section("Writes: where a record breaks around repeats and separators")
	for n := 15; n <= 20; n++ {
		wr(fmt.Sprintf("two runs of texts of %d", n), a("A", fmt.Sprintf("T%d", n), 7, "'"+strings.Repeat("A", n)+"'", "'"+strings.Repeat("B", n)+"'", "'"+strings.Repeat("C", n)+"'", "'"+strings.Repeat("C", n)+"'", "'"+strings.Repeat("D", n)+"'", "'"+strings.Repeat("E", n)+"'", "'"+strings.Repeat("E", n)+"'"))
	}

	for n := 56; n <= 70; n++ {
		wr(fmt.Sprintf("a text of %d, a repeat", n), a("A", fmt.Sprintf("T%d", n), 3, "'"+strings.Repeat("A", n)+"'", "'"+strings.Repeat("B", n)+"'", "'"+strings.Repeat("B", n)+"'"))
	}

	for n := 60; n <= 70; n++ {
		wr(fmt.Sprintf("a repeat of a text of %d", n), a("A", fmt.Sprintf("T%d", n), 2, "'"+strings.Repeat("A", n)+"'", "'"+strings.Repeat("A", n)+"'"))
	}

	wr("logicals differing but both true", a("A", "L4", 2, ".TRUE.", ".TRUE."))

	// Where a repeat that would fit only without its field's blanks goes.
	for _, n := range []int{1, 9, 17, 25} {
		for k := 0; k <= 5; k++ {
			v := make([]string, 0, k+4)
			for i := 1; i <= k; i++ {
				v = append(v, fmt.Sprint(i))
			}

			v = append(v, "6", "6", "6", "7")
			wr(fmt.Sprintf("a repeat after %d values, a name of %d", k, n), a(strings.Repeat("N", n), "I4", k+4, v...))
		}
	}

	for _, k := range []int{21, 22, 23, 24} {
		v := make([]string, 0, k+4)
		for i := range k {
			if i%2 == 0 {
				v = append(v, ".TRUE.")
			} else {
				v = append(v, ".FALSE.")
			}
		}

		last := ".TRUE."
		if k%2 == 1 {
			last = ".FALSE."
		}

		v = append(v, last, last, last, ".TRUE.")
		wr(fmt.Sprintf("a logical repeat after %d values", k), a("A", "L4", k+4, v...))
	}

	section("Writes: every type")
	wr("one integer", e("I", "I4", "42"))
	wr("the extreme integers", e("I", "I4", "2147483647"), e("J", "I4", "(-2147483647-1)"))
	wr("a word", e("J", "I2", "-5"))
	wr("logicals", e("L", "L4", ".TRUE."), e("M", "L2", ".FALSE."), e("N", "L1", ".TRUE."))
	wr("a byte logical", e("N", "L1", ".FALSE."))
	wr("reals", e("R", "R4", "2.5"), e("S", "R4", "-1.0E10"), e("U", "R4", "0.0"), e("V", "R4", "1.0E-5"))
	wr("doubles", e("D", "R8", "2.5D0"), e("F", "R8", "-1.0D30"), e("G", "R8", "0.0D0"))
	wr("quads", e("Q", "R16", "2.5Q0"), e("P", "R16", "-1.0Q300"))
	wr("complex", e("C", "C8", "(1.5,-2.5)"), e("D", "C8", "(0.0,1.0E10)"))
	wr("double complex", e("E", "C16", "(1.5D0,-2.5D0)"))
	wr("characters", e("T", "T5", "'HELLO'"), e("U", "T1", "'X'"), e("V", "T3", "'   '"))
	wr("apostrophes", e("T", "T5", "'A''B  '"), e("U", "T1", "''''"), e("V", "T3", "''''''''"))
	wrn("a long group name", "GROUPNAMEOFTHIRTYONECHARACTERS1", e("I", "I4", "1"))
	wrn("a short group name", "G", e("I", "I4", "1"))

	section("Writes: names")
	for n := 1; n <= 17; n++ {
		wr(fmt.Sprintf("a name of %d", n), e(strings.Repeat("N", n), "I4", "1"))
	}

	wr("a name of 24", e(strings.Repeat("N", 24), "I4", "1"))
	wr("a name of 25", e(strings.Repeat("N", 25), "I4", "1"))
	wr("a name of 31", e(strings.Repeat("N", 31), "I4", "1"))
	wr("a name of 31, an array", a(strings.Repeat("N", 31), "I4", 8, "1", "2", "3", "4", "5", "6", "7", "8"))
	wr("a name of 31, a long text", e(strings.Repeat("N", 31), "T60", "'"+strings.Repeat("X", 60)+"'"))

	section("Writes: arrays and repeats")
	seq := func(n int, f func(i int) string) []string {
		v := make([]string, n)
		for i := range v {
			v[i] = f(i)
		}

		return v
	}
	num := func(i int) string { return fmt.Sprint(i + 1) }
	wr("one element", a("A", "I4", 1, "5"))
	wr("all the same", a("A", "I4", 4, "5", "5", "5", "5"))
	wr("repeats first", a("A", "I4", 4, "5", "5", "5", "6"))
	wr("repeats last", a("A", "I4", 4, "6", "5", "5", "5"))
	wr("repeats between", a("A", "I4", 4, "6", "5", "5", "7"))
	wr("two runs", a("A", "I4", 4, "5", "5", "6", "6"))
	wr("no repeats", a("A", "I4", 4, "1", "2", "3", "4"))
	wr("an array, then a scalar", a("A", "I4", 2, "1", "2"), e("I", "I4", "3"))
	wr("a scalar, then an array", e("I", "I4", "3"), a("A", "I4", 2, "1", "2"))
	wr("two dimensions", entity{name: "B", kind: "I4", bounds: []int{1, 2, 1, 2}, values: []string{"1", "2", "3", "4"}})
	wr("lower bounds", entity{name: "B", kind: "I4", bounds: []int{-1, 1}, values: []string{"1", "1", "4"}})
	wr("words", a("A", "I2", 16, seq(16, num)...))
	wr("words, the same", a("A", "I2", 16, seq(16, func(int) string { return "-7" })...))
	wr("byte logicals", a("A", "L1", 20, seq(20, func(i int) string {
		if i%2 == 0 {
			return ".TRUE."
		}

		return ".FALSE."
	})...))
	wr("logicals", a("A", "L4", 30, seq(30, func(i int) string {
		if i%2 == 0 {
			return ".TRUE."
		}

		return ".FALSE."
	})...))
	wr("logicals, the same", a("A", "L2", 3, ".TRUE.", ".TRUE.", ".TRUE."), a("B", "L4", 2, ".FALSE.", ".FALSE."))
	wr("reals", a("A", "R4", 7, seq(7, func(i int) string { return fmt.Sprintf("%d.5", i) })...))
	wr("reals, repeats", a("A", "R4", 6, "1.0", "1.0", "2.0", "2.0", "2.0", "3.0"))
	wr("doubles", a("A", "R8", 5, seq(5, func(i int) string { return fmt.Sprintf("%d.5D0", i) })...))
	wr("doubles, repeats", a("A", "R8", 5, "1.0D0", "1.0D0", "2.0D0", "2.0D0", "2.0D0"))
	wr("quads", a("A", "R16", 3, "1.5Q0", "2.5Q0", "3.5Q0"))
	wr("complex", a("A", "C8", 5, seq(5, func(i int) string { return fmt.Sprintf("(%d.5,-%d.5)", i, i) })...))
	wr("complex, repeats", a("A", "C8", 3, "(1.0,2.0)", "(1.0,2.0)", "(3.0,4.0)"))
	wr("double complex", a("A", "C16", 3, "(1.5D0,2.5D0)", "(3.5D0,4.5D0)", "(5.5D0,6.5D0)"))
	wr("characters", a("A", "T3", 3, "'AAA'", "'BBB'", "'CCC'"))
	wr("characters, repeats", a("A", "T3", 4, "'AAA'", "'AAA'", "'AAA'", "'CCC'"))
	wr("integers across records", a("A", "I4", 20, seq(20, num)...))
	wr("integers, repeats at the record's end", a("A", "I4", 9, "1", "2", "3", "4", "5", "6", "6", "6", "7"))

	section("Writes: where records break")
	for n := 56; n <= 82; n++ {
		wr(fmt.Sprintf("a text of %d, last", n), e("T", fmt.Sprintf("T%d", n), "'"+strings.Repeat("X", n)+"'"))
		wr(fmt.Sprintf("a text of %d, then another", n), e("T", fmt.Sprintf("T%d", n), "'"+strings.Repeat("X", n)+"'"), e("U", "T1", "'U'"))
	}

	for _, n := range []int{100, 158, 159, 160, 161, 162, 200, 255} {
		wr(fmt.Sprintf("a text of %d", n), e("T", fmt.Sprintf("T%d", n), "'"+strings.Repeat("X", n)+"'"))
	}

	wr("apostrophes across a record", e("T", "T100", "'"+strings.Repeat("''", 100)+"'"))
	wr("an apostrophe at the break", e("T", "T80", "'"+strings.Repeat("X", 67)+"''"+strings.Repeat("Y", 12)+"'"))

	for n := 10; n <= 40; n++ {
		wr(fmt.Sprintf("texts of %d", n), a("A", fmt.Sprintf("T%d", n), 4, seq(4, func(i int) string { return "'" + strings.Repeat(string(rune('A'+i)), n) + "'" })...))
	}

	for n := 30; n <= 36; n++ {
		wr(fmt.Sprintf("repeated texts of %d", n), a("A", fmt.Sprintf("T%d", n), 5, seq(5, func(i int) string { return "'" + strings.Repeat(string(rune('A'+i/2)), n) + "'" })...))
	}

	wr("a repeat of a long text", a("A", "T100", 2, "'"+strings.Repeat("X", 100)+"'", "'"+strings.Repeat("X", 100)+"'"))

	for n := 1; n <= 8; n++ {
		// names 1 to 8 pad the same; 9 to 16 move the first value along
		wr(fmt.Sprintf("integers after a name of %d", 8+n), a(strings.Repeat("N", 8+n), "I4", 7, seq(7, num)...))
		wr(fmt.Sprintf("reals after a name of %d", 16+n), a(strings.Repeat("N", 16+n), "R4", 5, seq(5, func(i int) string { return fmt.Sprintf("%d.5", i) })...))
	}

	wr("repeats at a record's end, reals", a("A", "R4", 9, "1.0", "2.0", "3.0", "4.0", "5.0", "5.0", "6.0", "6.0", "7.0"))
	wr("complex across a record", a("A", "C8", 8, seq(8, func(i int) string { return fmt.Sprintf("(%d.5,-%d.5)", i, i) })...))
	wr("double complex across a record", a("A", "C16", 8, seq(8, func(i int) string { return fmt.Sprintf("(%d.5D0,-%d.5D0)", i, i) })...))
	wr("a complex that ends a record", a("A", "C16", 2, "(1.0D30,-1.0D30)", "(1.0D30,-1.0D30)"))

	section("Writes: G_floating")
	cases = append(cases, kase{title: "G", group: "GRP", g: true, ents: []entity{e("G", "RG", "2.5D0"), e("H", "CG", "(1.5D0,-2.5D0)")}})
	cases = append(cases, kase{title: "G arrays", group: "GRP", g: true, ents: []entity{a("A", "RG", 5, "1.5D0", "1.5D0", "2.5D0", "3.5D0", "4.5D0")}})

	// Last: the input's end.
	section("The end of SYS$INPUT")
	cases = append(cases, kase{reads: 1})
	cases = append(cases, kase{reads: 1})
}

// The types' MACRO forms and FORTRAN declarations.
var typeInfo = map[string]struct {
	decl   string
	length int
	dtype  int
	dir    string
}{
	"I4":  {"INTEGER*4", 4, 8, ".LONG"},
	"I2":  {"INTEGER*2", 2, 7, ".WORD"},
	"L4":  {"LOGICAL*4", 4, 4, ".LONG"},
	"L2":  {"LOGICAL*2", 2, 3, ".WORD"},
	"L1":  {"LOGICAL*1", 1, 2, ".BYTE"},
	"R4":  {"REAL*4", 4, 10, ".F_FLOATING"},
	"R8":  {"REAL*8", 8, 11, ".D_FLOATING"},
	"RG":  {"REAL*8", 8, 27, ".G_FLOATING"},
	"R16": {"REAL*16", 16, 28, ".H_FLOATING"},
	"C8":  {"COMPLEX*8", 8, 12, ".F_FLOATING"},
	"C16": {"COMPLEX*16", 16, 13, ".D_FLOATING"},
	"CG":  {"COMPLEX*16", 16, 29, ".G_FLOATING"},
}

func typeOf(kind string) (decl string, length, dtype int, dir string) {
	if kind[0] == 'T' {
		var n int
		fmt.Sscan(kind[1:], &n)

		return fmt.Sprintf("CHARACTER*%d", n), n, 14, ".ASCII"
	}

	t := typeInfo[kind]

	return t.decl, t.length, t.dtype, t.dir
}

// macroValue is one FORTRAN value as a MACRO data directive's line.
func macroValue(kind, v string) string {
	_, _, _, dir := typeOf(kind)

	switch {
	case kind[0] == 'T':
		text := strings.ReplaceAll(v[1:len(v)-1], "''", "'")

		return "\t.ASCII\t~" + text + "~\n"
	case kind[0] == 'L':
		if v == ".TRUE." {
			return "\t" + dir + "\t-1\n"
		}

		return "\t" + dir + "\t0\n"
	case kind[0] == 'I':
		if strings.HasPrefix(v, "(") {
			return "\t" + dir + "\t^X80000000\n"
		}

		return "\t" + dir + "\t" + v + "\n"
	case kind[0] == 'C':
		parts := strings.Split(v[1:len(v)-1], ",")

		return "\t" + dir + "\t" + macroFloat(parts[0]) + "\n\t" + dir + "\t" + macroFloat(parts[1]) + "\n"
	}

	return "\t" + dir + "\t" + macroFloat(v) + "\n"
}

// macroFloat is a floating constant with the exponent letter MACRO takes.
func macroFloat(v string) string {
	return strings.NewReplacer("D", "E", "Q", "E").Replace(v)
}

// macroGroup writes a group's values, descriptors, names, and block. The values
// are the only part another copy (init) is made of, to reset them from.
func macroGroup(b *strings.Builder, prefix, group string, ents []entity, init bool) {
	fmt.Fprintf(b, "%sG:\t.ASCIC\t~%s~\n", prefix, group)

	for i, en := range ents {
		fmt.Fprintf(b, "%sN%d:\t.ASCIC\t~%s~\n", prefix, i, en.name)
	}

	values := func(label string) {
		fmt.Fprintf(b, "\t.ALIGN\tLONG\n%s:\n", label)

		for i, en := range ents {
			fmt.Fprintf(b, "\t.ALIGN\tLONG\n%s%sV%d:\n", label, prefix, i)

			for _, v := range en.values {
				b.WriteString(macroValue(en.kind, v))
			}
		}

		fmt.Fprintf(b, "\t.ALIGN\tLONG\n%sE:\n", label)
	}

	values(prefix + "A")

	if init {
		values(prefix + "B")
	}

	for i, en := range ents {
		_, length, dtype, _ := typeOf(en.kind)
		value := fmt.Sprintf("%sA%sV%d", prefix, prefix, i)

		if len(en.bounds) == 0 {
			fmt.Fprintf(b, "%sD%d:\t.WORD\t%d\n\t.BYTE\t%d,1\n\t.ADDRESS %s\n", prefix, i, length, dtype, value)

			continue
		}

		dims := len(en.bounds) / 2
		offset, step := 0, 1
		var mults, bounds []string

		for d := range dims {
			lo, hi := en.bounds[2*d], en.bounds[2*d+1]
			offset += lo * step
			step *= hi - lo + 1
			mults = append(mults, fmt.Sprint(hi-lo+1))
			bounds = append(bounds, fmt.Sprint(lo), fmt.Sprint(hi))
		}

		fmt.Fprintf(b, "%sD%d:\t.WORD\t%d\n\t.BYTE\t%d,4\n\t.ADDRESS %s\n\t.BYTE\t0,0,^XE0,%d\n\t.LONG\t%d\n\t.ADDRESS %s-<%d>\n\t.LONG\t%s\n\t.LONG\t%s\n",
			prefix, i, length, dtype, value, dims, step*length, value, offset*length, strings.Join(mults, ","), strings.Join(bounds, ","))
	}

	fmt.Fprintf(b, "%sK:\t.ADDRESS %sG\n\t.LONG\t%d\n", prefix, prefix, len(ents))

	for i := range ents {
		fmt.Fprintf(b, "\t.ADDRESS %sN%d,%sD%d\n", prefix, i, prefix, i)
	}
}

func title(n int, c kase) string {
	t := fmt.Sprintf(" #%d", n)
	if c.title != "" {
		t += " " + c.title
	}

	if len(c.lines) > 0 {
		t += " <" + strings.Join(c.lines, "/") + ">"
	}

	t = strings.ReplaceAll(t, "\t", "^I")
	if len(t) > 100 {
		t = t[:100]
	}

	return t
}

func macroText(s string) string {
	if strings.Contains(s, "~") {
		log.Fatalf("a ~ in %q", s)
	}

	return "~" + s + "~"
}

const macroProgram = `	.TITLE	NML80 - the probe of NAMELIST I/O, on VMS's FORRTL
; Written by testdata/nml80/gen.go.
	$SSDEF
	.PSECT	D,NOEXE,WRT,LONG
E1:	.LONG	0
E2:	.LONG	0
E3:	.LONG	0
E4:	.LONG	0
E5:	.LONG	0
BD:	.WORD	240
	.BYTE	14,1
	.ADDRESS BF
BF:	.BLKB	240
LD:	.WORD	0
	.BYTE	14,1
	.ADDRESS BF
RES1:	.ASCID	" =!5UL e=!5UL"
FMTT:	.ASCIZ	~(A)~
LINEB:	.BLKB	600
LINED:	.WORD	600
	.BYTE	14,1
	.ADDRESS LINEB
	.ALIGN	LONG
%s
	.PSECT	C,EXE,NOWRT
; HND continues from a condition below severe and unwinds from the rest.
HND:	.WORD	^M<>
	MOVL	4(AP),R0
	MOVL	4(R0),R1
	CMPL	R1,#SS$_UNWIND
	BEQL	50$%s
	BICL3	#^XFFFFFFF8,R1,R1
	CMPL	R1,#4
	BEQL	40$
	MOVL	#SS$_CONTINUE,R0
	RET
40$:	$UNWIND_S
50$:	MOVL	#SS$_CONTINUE,R0
	RET
; PUTT(descriptor) writes a line at unit 6, format (A).
	.ENTRY	PUTT,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	PUSHAB	FMTT
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SO
	PUSHL	4(AP)
	CALLS	#1,G^FOR$IO_T_DS
	CALLS	#0,G^FOR$IO_END
	RET
; SYNC(descriptor) reads SYS$INPUT up to the case's marker line.
	.ENTRY	SYNC,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
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
; READ(block): one READ, under the handler.
	.ENTRY	READ,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	HND,(FP)
	PUSHL	4(AP)
	PUSHL	#5
	CALLS	#2,G^FOR$READ_SN
	CALLS	#0,G^FOR$IO_END
	RET
; WRITE(block): one WRITE, under the handler.
	.ENTRY	WRITE,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>
	MOVAB	HND,(FP)
	PUSHL	4(AP)
	PUSHL	#6
	CALLS	#2,G^FOR$WRITE_SN
	CALLS	#0,G^FOR$IO_END
	RET
; RESULT(n) shows the number of the last error.
	.ENTRY	RESULT,^M<R2,R3,R4,R5,R6>
	MOVAL	E5,-(SP)
	MOVAL	E4,-(SP)
	MOVAL	E3,-(SP)
	MOVAL	E2,-(SP)
	MOVAL	E1,-(SP)
	CALLS	#5,G^FOR$ERRSNS
	$FAO_S	CTRSTR=RES1,OUTLEN=LD,OUTBUF=BD,P1=4(AP),P2=E1
	PUSHAB	LD
	CALLS	#1,PUTT
	RET
	.ENTRY	NML80,^M<>
%s	MOVL	#1,R0
	RET
	.END	NML80
`

func macro() string {
	var data, main strings.Builder

	macroGroup(&data, "Y", "GRP", mainGroup, true)
	macroGroup(&data, "Z", "GRP", gGroup, true)

	for n, c := range cases {
		if s, ok := sections[n]; ok {
			fmt.Fprintf(&data, "SC%d:\t.ASCID\t%s\n", n, macroText(" ##### "+s))
			fmt.Fprintf(&main, "\tPUSHAB\tSC%d\n\tCALLS\t#1,PUTT\n", n)
		}

		fmt.Fprintf(&data, "HD%d:\t.ASCID\t%s\n", n, macroText(title(n, c)))
		fmt.Fprintf(&main, "\tPUSHAB\tHD%d\n\tCALLS\t#1,PUTT\n", n)

		if c.reads == 0 {
			p := fmt.Sprintf("W%d", n)
			macroGroup(&data, p, c.group, c.ents, false)
			fmt.Fprintf(&main, "\tPUSHAB\t%sK\n\tCALLS\t#1,WRITE\n", p)

			continue
		}

		block := "YK"
		reset := "\tMOVC3\t#YBE-YB,YB,YA\n"

		if c.g {
			block = "ZK"
			reset = "\tMOVC3\t#ZBE-ZB,ZB,ZA\n"
		}

		main.WriteString(reset)

		if len(c.lines) > 0 {
			fmt.Fprintf(&data, "SE%d:\t.ASCID\t%s\n", n, macroText(fmt.Sprintf("#%d", n)))
			fmt.Fprintf(&main, "\tPUSHAB\tSE%d\n\tCALLS\t#1,SYNC\n", n)
		}

		for range c.reads {
			fmt.Fprintf(&main, "\tPUSHAB\t%s\n\tCALLS\t#1,READ\n", block)
		}

		fmt.Fprintf(&main, "\tPUSHL\t#%d\n\tCALLS\t#1,RESULT\n\tPUSHAB\t%s\n\tCALLS\t#1,WRITE\n", n, block)
	}

	return fmt.Sprintf(macroProgram, data.String(), "", main.String())
}

// procedure is the deck both programs read, the image the parameter.
func procedure() string {
	var b strings.Builder

	b.WriteString(`$ ! NML80R.COM - runs an image with the records the NAMELIST probe reads.
$ ! Written by testdata/nml80/gen.go. Each case's lines are under a line
$ ! holding its number, which the program reads up to.
$ RUN 'P1'
$ DECK
`)

	for n, c := range cases {
		if len(c.lines) == 0 {
			continue
		}

		fmt.Fprintf(&b, "#%d\n", n)

		for _, l := range c.lines {
			b.WriteString(l + "\n")
		}

		// A case that reads past its lines stops at this, which ends a group
		// and is a group for one still searching; the lines after it end a
		// character constant that is still open and are passed over otherwise.
		b.WriteString(" $GRP $END\n '\n $END\n")
	}

	b.WriteString("$ EOD\n$ EXIT\n")

	return b.String()
}

func statement(b *strings.Builder, text string) {
	const width = 66

	b.WriteString("      " + text[:min(len(text), width)] + "\n")

	for text = text[min(len(text), width):]; text != ""; text = text[min(len(text), width):] {
		b.WriteString("     &" + text[:min(len(text), width)] + "\n")
	}
}

func ftnText(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// declare writes a group's declarations, and its NAMELIST statement.
func declare(b *strings.Builder, group string, ents []entity) {
	var names []string

	for _, en := range ents {
		decl, _, _, _ := typeOf(en.kind)
		name := en.name

		if len(en.bounds) > 0 {
			var dims []string

			for d := 0; d < len(en.bounds); d += 2 {
				dims = append(dims, fmt.Sprintf("%d:%d", en.bounds[d], en.bounds[d+1]))
			}

			name += "(" + strings.Join(dims, ",") + ")"
		}

		statement(b, decl+" "+name)
		names = append(names, en.name)
	}

	statement(b, "NAMELIST /"+group+"/ "+strings.Join(names, ", "))
}

// setValues assigns a group's initial values, an element at a time.
func setValues(b *strings.Builder, ents []entity) {
	for _, en := range ents {
		for i, v := range en.values {
			if len(en.bounds) == 0 {
				statement(b, en.name+"="+v)

				continue
			}

			statement(b, assignElement(en, i)+"="+v)
		}
	}
}

const ftnHead = `C     NML80F.FOR - written by testdata/nml80/gen.go: the NAMELIST probe's
C     cases as FORTRAN statements, for FTNRTL. TestNML80Oracle holds its
C     output to VMS's FORRTL's, vax/nml80-forrtl.txt.
`

func fortran() string {
	var b, routines strings.Builder

	b.WriteString(ftnHead)
	b.WriteString("      PROGRAM NML80F\n")

	for n, c := range cases {
		if s, ok := sections[n]; ok {
			statement(&b, "WRITE (6,'(A)') "+ftnText(" ##### "+s))
		}

		statement(&b, "WRITE (6,'(A)') "+ftnText(title(n, c)))

		if c.reads == 0 {
			if c.g {
				fmt.Fprintf(&routines, "      OPTIONS /G_FLOATING\n")
			}

			fmt.Fprintf(&routines, "      SUBROUTINE W%d\n", n)
			declare(&routines, c.group, c.ents)

			setValues(&routines, c.ents)

			statement(&routines, "WRITE (6,"+c.group+")")
			routines.WriteString("      END\n")
			statement(&b, fmt.Sprintf("CALL W%d", n))

			continue
		}

		routine := "RM"
		if c.g {
			routine = "RG"
		}

		mark := "' '"
		if len(c.lines) > 0 {
			mark = ftnText(fmt.Sprintf("#%d", n))
		}

		statement(&b, fmt.Sprintf("CALL %s(%d,%d,%s)", routine, n, c.reads, mark))
	}

	b.WriteString("      END\n")

	reader := func(name string, ents []entity, options string) {
		b.WriteString(options)
		fmt.Fprintf(&b, "      SUBROUTINE %s(NCASE9,NTIME9,MARK9)\n      INTEGER NCASE9,NTIME9,IOS9\n      CHARACTER*(*) MARK9\n      CHARACTER*200 LINE9\n", name)
		declare(&b, "GRP", ents)

		setValues(&b, ents)
		b.WriteString(`      IF (MARK9 .EQ. ' ') GO TO 20
   10 READ (5,'(A)') LINE9
      IF (LINE9 .NE. MARK9) GO TO 10
   20 DO 30 K9=1,NTIME9
      IOS9=0
      READ (5,GRP,IOSTAT=IOS9)
   30 CONTINUE
      IF (IOS9 .LT. 0) IOS9=24
      WRITE (6,'(1X,''='',I5,'' e='',I5)') NCASE9,IOS9
      WRITE (6,GRP)
      END
`)
	}

	reader("RM", mainGroup, "")
	reader("RG", gGroup, "      OPTIONS /G_FLOATING\n")
	b.WriteString(routines.String())

	return b.String()
}

// assignElement is the array element a group's ith value goes to.
func assignElement(en entity, i int) string {
	var subs []string

	for d := 0; d < len(en.bounds); d += 2 {
		lo, hi := en.bounds[d], en.bounds[d+1]
		extent := hi - lo + 1
		subs = append(subs, fmt.Sprint(lo+i%extent))
		i /= extent
	}

	return en.name + "(" + strings.Join(subs, ",") + ")"
}

// The runs with no handler: each READ's condition goes to the catch-all, for
// its message. Each has a run of its own, and data lines of its own, so that it
// can also end at the end of the file wherever it likes.
var messageRuns = [][]string{
	{" $GRP / I=7 $END"},
	{" $GRP I=7 /"},
	{" $GRP I 7 $END"},
	{" $GRP I==7 $END"},
	{" $GRP 7 $END"},
	{" $GRP 'I'=7 $END"},
	{" $GRP I=7ABC $END"},
	{" $GRP I=1.2.3 $END"},
	{" $GRP A(2=5 $END"},
	{" $GRP A (2)=5 $END"},
	{" $GRP C=(5,) $END"},
	{" $GRP C=(5,6 $END"},
	{" $GRP", " I=7", " I 8", " $END"},
	{" $GRP I=7,8 $END"},
	{" $GRP A=5,6,7,8 $END"},
	{" $GRP LONGNAME1=5,6 $END"},
	{" $GRP A(3)=5,6 $END"},
	{" $GRP NOPE=1 $END"},
	{" $GRP NOPE $END"},
	{" $GRP A(4)=5 $END"},
	{" $GRP T(2)=5 $END"},
	{" $GRP TA(2)(5:6)='X' $END"},
	{" $GRP B(1,3)=5 $END"},
	{" $GRP I=ABC $END"},
	{" $GRP T='AB'X $END"},
	{" $GRP LONG NAME1=7 $END"},
	{" $GRP nope=1 $END"},
	{" $GRP t(2)=5 $END"},
	{" $GRP I='A' $END"},
	{" $GRP T=7 $END"},
	{" $GRP R=1E40 $END"},
	{" $GRP J=70000 $END"},
	{},
	{" IGNORED"},
	{" $GRP"},
	{" $GRP I=7"},
	{" $GRP I=7,"},
	{" $GRP T='AB"},
	{" $GRP C=(1,"},
	{" $GRP C=("},
	{" $GRP A(2"},
	{" $GRP A=2*"},
	{" $GRP I"},
	{" $GRP I="},
}

const messageProgram = `	.TITLE	NML80E - NAMELIST conditions with no handler
; Written by testdata/nml80/gen.go.
	.PSECT	D,NOEXE,WRT,LONG
GRPN:	.ASCIC	/GRP/
NI:	.ASCIC	/I/
NJ:	.ASCIC	/J/
NR:	.ASCIC	/R/
NC:	.ASCIC	/C/
NT:	.ASCIC	/T/
NA:	.ASCIC	/A/
NB:	.ASCIC	/B/
NTA:	.ASCIC	/TA/
NLN:	.ASCIC	/LONGNAME1/
	.ALIGN	LONG
VI:	.LONG	0
VJ:	.WORD	0
	.ALIGN	LONG
VR:	.F_FLOATING 0.0
VC:	.F_FLOATING 0.0,0.0
VT:	.ASCII	/     /
	.ALIGN	LONG
VA:	.LONG	0,0,0
VB:	.LONG	0,0,0,0
VTA:	.ASCII	/         /
	.ALIGN	LONG
VLN:	.LONG	0
DI:	.WORD	4
	.BYTE	8,1
	.ADDRESS VI
DJ:	.WORD	2
	.BYTE	7,1
	.ADDRESS VJ
DR:	.WORD	4
	.BYTE	10,1
	.ADDRESS VR
DC:	.WORD	8
	.BYTE	12,1
	.ADDRESS VC
DT:	.WORD	5
	.BYTE	14,1
	.ADDRESS VT
DA:	.WORD	4
	.BYTE	8,4
	.ADDRESS VA
	.BYTE	0,0,^XE0,1
	.LONG	12
	.ADDRESS VA-4
	.LONG	3
	.LONG	1,3
DB:	.WORD	4
	.BYTE	8,4
	.ADDRESS VB
	.BYTE	0,0,^XE0,2
	.LONG	16
	.ADDRESS VB-12
	.LONG	2,2
	.LONG	1,2,1,2
DTA:	.WORD	3
	.BYTE	14,4
	.ADDRESS VTA
	.BYTE	0,0,^XE0,1
	.LONG	9
	.ADDRESS VTA-3
	.LONG	3
	.LONG	1,3
DLN:	.WORD	4
	.BYTE	8,1
	.ADDRESS VLN
BLK:	.ADDRESS GRPN
	.LONG	9
	.ADDRESS NI,DI,NJ,DJ,NR,DR,NC,DC,NT,DT,NA,DA,NB,DB,NTA,DTA,NLN,DLN
	.PSECT	C,EXE,NOWRT
	.ENTRY	NML80E,^M<>
	PUSHAB	BLK
	PUSHL	#5
	CALLS	#2,G^FOR$READ_SN
	CALLS	#0,G^FOR$IO_END
	MOVL	#1,R0
	RET
	.END	NML80E
`

const messageTwin = `C     NML80E.FOR - written by testdata/nml80/gen.go: NML80E.MAR as FORTRAN.
      PROGRAM NML80E
      INTEGER I, A(3), B(2,2), LONGNAME1
      INTEGER*2 J
      REAL R
      COMPLEX C
      CHARACTER*5 T
      CHARACTER*3 TA(3)
      NAMELIST /GRP/ I, J, R, C, T, A, B, TA, LONGNAME1
      DATA T /' '/, TA /3*' '/
      READ (5, GRP)
      END
`

func messageProcedure() string {
	var b strings.Builder

	b.WriteString(`$ ! NML80E.COM - runs an image once per set of records, each a condition
$ ! with no handler. Written by testdata/nml80/gen.go.
$ SET NOON
`)

	for n, run := range messageRuns {
		fmt.Fprintf(&b, "$ WRITE SYS$OUTPUT \"%%%d\"\n$ RUN 'P1'\n", n)

		for _, l := range run {
			b.WriteString(l + "\n")
		}

		b.WriteString("$ WRITE SYS$OUTPUT F$INTEGER($STATUS)\n")
	}

	b.WriteString("$ EXIT\n")

	return b.String()
}

// vmsProcedure builds and runs the probe on VMS. The ? and =? cases at a
// terminal are run by hand: RUN NML80E, then type
//
//	?
//	=?
//	$GRP ?
//	=?
//	I=7 $END
const vmsProcedure = `$ ! NML80.COM - the NAMELIST probe on VMS. Written by testdata/nml80/gen.go.
$ ! Run it with the exchange volume's [000000] as the default directory:
$ !
$ !     @NML80/OUTPUT=NML80.LOG
$ !
$ SET NOON
$ SET VERIFY
$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")
$ MACRO/NOLIST NML80
$ LINK/NOMAP NML80,NML80/OPTIONS
$ MACRO/NOLIST NML80E
$ LINK/NOMAP NML80E,NML80/OPTIONS
$ DEFINE/USER_MODE SYS$OUTPUT NML80-VAX.TXT
$ @NML80R NML80
$ SHOW SYMBOL $STATUS
$ @NML80E/OUTPUT=NML80E-VAX.TXT NML80E
$ EXIT
`

func main() {
	table()

	files := map[string]string{
		"nml80.mar":  macro(),
		"nml80f.for": fortran(),
		"nml80r.com": procedure(),
		"nml80e.mar": messageProgram,
		"nml80e.for": messageTwin,
		"nml80e.com": messageProcedure(),
		"nml80.opt":  "SYS$SHARE:FORRTL/SHAREABLE\n",
		"nml80.com":  vmsProcedure,
		"exchange.cmd": `! EXCHANGE.CMD - builds the NAMELIST probe's exchange volume with govax.
! Written by testdata/nml80/gen.go. Run from the repository root:
!
!     govax console < testdata/nml80/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/nml80-exchange.dsk" /DEVICE=RD51 NML80
MOUNT/WRITE DUA2 "testdata/disks/nml80-exchange.dsk"
COPY "testdata/nml80/nml80.mar"/HOST DUA2:[000000]NML80.MAR
COPY "testdata/nml80/nml80r.com"/HOST DUA2:[000000]NML80R.COM
COPY "testdata/nml80/nml80e.mar"/HOST DUA2:[000000]NML80E.MAR
COPY "testdata/nml80/nml80e.com"/HOST DUA2:[000000]NML80E.COM
COPY "testdata/nml80/nml80.opt"/HOST DUA2:[000000]NML80.OPT
COPY "testdata/nml80/nml80.com"/HOST DUA2:[000000]NML80.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
`,
		"copyout.cmd": `! COPYOUT.CMD - copies the VAX's run off the exchange volume.
! Written by testdata/nml80/gen.go. Run from the repository root:
!
!     govax console < testdata/nml80/copyout.cmd
!
MOUNT DUA2 "testdata/disks/nml80-exchange.dsk"
COPY DUA2:[000000]NML80-VAX.TXT "testdata/nml80/vax/nml80-vax.txt"/HOST
COPY DUA2:[000000]NML80E-VAX.TXT "testdata/nml80/vax/nml80e-vax.txt"/HOST
COPY DUA2:[000000]NML80.LOG "testdata/nml80/vax/nml80.log"/HOST
DISMOUNT DUA2
`,
	}

	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println(len(cases), "cases")
}
