//go:build ignore

// gen.go writes the Phase 113 probe of the system services subtask 8 added
// (docs/planning/PHASE-113 - Fortran errata.md): svc113.mar, a MACRO program
// that calls the UTC services ($GETUTC, $ASCUTC, $BINUTC, $NUMUTC, $TIMCON)
// and the device information services ($GETCHN, $GETDEV) case by case and
// prints what each returned, signaled, and stored; and def_dib.mar, $DIBDEF's
// values. It is laid out as testdata/str113's: each case is a routine under a
// handler that records a signaled condition and unwinds, so a status returned
// shows apart from one signaled. Run this from the repository root:
//
//	go run testdata/svc113/gen.go
//
// It runs in Phase 113's probe round (subtask 12); its README has the steps.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// show is one value a case prints after its R0 line.
type show struct {
	kind  string // "mem" (longwords), "txt" (an S or D string)
	label string // the value's address (a label; "%" is the case's prefix)
	count int    // mem: how many longwords
}

func mem(label string, n int) show { return show{"mem", label, n} }
func txt(label string) show        { return show{"txt", label, 0} }

// probeCase is one call of one routine.
type probeCase struct {
	name string   // printed first (at most 31 characters)
	data []string // data lines; "%" is the case's label prefix
	pre  []string // instructions before the call
	call string   // the routine; "" for none (the case only shows values)
	args []string // its arguments, first first
	post []string // instructions after the call, R0 preserved
	show []show
}

// push returns the instructions that push args, last first, as a CALLS
// wants them: "#n" or a register is pushed as a value, "0" omits an optional
// argument (a 0 address), and anything else is an address (PUSHAB).
func push(args []string) []string {
	var lines []string

	for i := len(args) - 1; i >= 0; i-- {
		a := args[i]

		switch {
		case a == "0":
			lines = append(lines, "PUSHL\t#0")
		case strings.HasPrefix(a, "#"), len(a) == 2 && a[0] == 'R':
			lines = append(lines, "PUSHL\t"+a)
		default:
			lines = append(lines, "PUSHAB\t"+a)
		}
	}

	return lines
}

// ascid is a class S text descriptor for s at label, its text after it.
func ascid(label, s string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\t.ASCID\t\"", label)

	for i := 0; i < len(s); i++ {
		if s[i] < 0x80 {
			b.WriteByte(s[i])
		} else {
			fmt.Fprintf(&b, "\"<%d>\"", s[i])
		}
	}

	b.WriteByte('"')

	return b.String()
}

// desc is a descriptor at label: length, data type, class, then the
// pointer (a label, or 0).
func desc(label string, length, dtype, class int, ptr string) string {
	return fmt.Sprintf("%s:\t.WORD\t%d\n\t.BYTE\t%d,%d\n\t.ADDRESS\t%s", label, length, dtype, class, ptr)
}

// long is the data line of a longword.
func long(label string, v int) string { return fmt.Sprintf("%s:\t.LONG\t%d", label, v) }

// The UTC octaword's parts (utc.go in internal/corevms says what govax takes
// them to be; these cases are what VMS's are to settle).
const (
	ticksPerSecond = 10_000_000
	ticksPerDay    = 86_400 * ticksPerSecond
	utcBase        = 100_840 * ticksPerDay // 15-Oct-1582 to 17-Nov-1858
	infinite       = 1<<48 - 1
)

// smithsonian is a local date and time as a VMS system time.
func smithsonian(year int, month time.Month, day, hour, minute, second, hundredths int) uint64 {
	t := time.Date(year, month, day, hour, minute, second, hundredths*10_000_000, time.UTC)
	base := time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

	return uint64(t.Sub(base) / 100)
}

// octa is an octaword's data line: the time (ticks since 1582), the
// inaccuracy, the TDF in minutes, and the version.
func octa(label string, ticks, inacc uint64, tdf, version int) string {
	w := uint64(version)<<12 | uint64(tdf)&0xFFF
	hi := inacc | w<<48

	return fmt.Sprintf("%s:\t.LONG\t^X%08X,^X%08X,^X%08X,^X%08X", label, uint32(ticks), uint32(ticks>>32), uint32(hi), uint32(hi>>32))
}

// utcOf is the octaword for the local time v in a zone tdf minutes east.
func utcOf(label string, v uint64, tdf int) string {
	return octa(label, v+utcBase-uint64(int64(tdf)*60*ticksPerSecond), infinite, tdf, 1)
}

// quad is a quadword's data line.
func quad(label string, v uint64) string {
	return fmt.Sprintf("%s:\t.LONG\t^X%08X,^X%08X", label, uint32(v), uint32(v>>32))
}

// fill is n bytes of ^X5A, so a case shows what a service wrote.
func fill(label string, n int) string {
	return fmt.Sprintf("%s:\t.BYTE\t^X5A[%d]", label, n)
}

// ascutc is $ASCUTC(timlen, timbuf, utcadr, cvtflg) into an n-byte buffer.
func ascutc(name string, utc string, n, cvtflg int) probeCase {
	return probeCase{name: name, data: []string{utc, "%L:\t.WORD\t^X5A5A,^X5A5A", ascid("%D", strings.Repeat("-", n))},
		call: "SYS$ASCUTC", args: []string{"%L", "%D", "%U", fmt.Sprintf("#%d", cvtflg)},
		show: []show{mem("%L", 1), txt("%D")}}
}

// numutc is $NUMUTC(timbuf, utcadr): its 13 words, shown as 7 longwords
// over a buffer of ^X5A.
func numutc(name string, utc string) probeCase {
	return probeCase{name: name, data: []string{utc, fill("%W", 28)}, call: "SYS$NUMUTC", args: []string{"%W", "%U"},
		show: []show{mem("%W", 7)}}
}

// timcon is $TIMCON(smnadr, utcadr, cvtflg), showing both.
func timcon(name string, data []string, cvtflg int) probeCase {
	return probeCase{name: name, data: data, call: "SYS$TIMCON", args: []string{"%S", "%U", fmt.Sprintf("#%d", cvtflg)},
		show: []show{mem("%S", 2), mem("%U", 4)}}
}

// binutc is $BINUTC(timbuf, utcadr).
func binutc(name, text string) probeCase {
	return probeCase{name: name, data: []string{ascid("%T", text), fill("%U", 16)}, call: "SYS$BINUTC",
		args: []string{"%T", "%U"}, show: []show{mem("%U", 4)}}
}

// dibSize is the buffers' size: past govax's block of 116 bytes and its names.
const dibSize = 160

// getdev is $GETDEV(devnam, prilen, pribuf, scdlen, scdbuf), each buffer of
// size bytes of ^X5A, shown whole.
func getdev(name, device string, size int) probeCase {
	return probeCase{name: name, data: deviceBuffers(size, ascid("%N", device)), call: "SYS$GETDEV",
		args: []string{"%N", "%PL", "%PD", "%SL", "%SD"}, show: deviceShows(size)}
}

// deviceBuffers is the data of a $GETCHN or $GETDEV case: lengths and buffers.
func deviceBuffers(size int, more ...string) []string {
	return append(more,
		"%PL:\t.WORD\t^X5A5A,^X5A5A", "%SL:\t.WORD\t^X5A5A,^X5A5A",
		fmt.Sprintf("%%PD:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%%PB", size),
		fmt.Sprintf("%%SD:\t.WORD\t%d\n\t.BYTE\t14,1\n\t.ADDRESS\t%%SB", size),
		fill("%PB", (size+3)/4*4), fill("%SB", (size+3)/4*4))
}

// deviceShows shows the lengths, then each buffer 8 longwords to a line.
func deviceShows(size int) []show {
	shows := []show{mem("%PL", 1)}

	for _, buf := range []string{"%PB", "%SB"} {
		for at := 0; at < size; at += 32 {
			label := buf
			if at > 0 {
				label = fmt.Sprintf("%s+%d", buf, at)
			}

			shows = append(shows, mem(label, min(8, (size-at+3)/4)))
		}
	}

	return shows
}

// The local time most cases use, and its octawords in three zones.
var (
	when     = smithsonian(1999, time.March, 4, 13, 30, 15, 25)
	eastern  = utcOf("%U", when, -300)
	plusHour = utcOf("%U", when, 60)
)

func buildCases() []probeCase {
	var c []probeCase

	add := func(more ...probeCase) { c = append(c, more...) }

	// $GETUTC: the current time's last eight bytes (its inaccuracy, TDF,
	// and version), which don't change; then the time through $TIMCON
	// against $GETTIM's, shown only as whether they're within a second.
	add(probeCase{name: "GETUTC now", data: []string{fill("%U", 8), fill("%UH", 8)}, call: "SYS$GETUTC",
		args: []string{"%U"}, show: []show{mem("%UH", 2)}})
	add(probeCase{name: "GETUTC 0", call: "SYS$GETUTC", args: []string{"0"}})
	add(probeCase{name: "GETUTC no args", call: "SYS$GETUTC"})
	add(probeCase{name: "GETUTC vs GETTIM", data: []string{fill("%U", 16), quad("%S", 0), quad("%G", 0), "%Z:\t.LONG\t0"},
		pre:  []string{"PUSHAB\t%G", "CALLS\t#1,G^SYS$GETTIM", "PUSHAB\t%U", "CALLS\t#1,G^SYS$GETUTC"},
		call: "SYS$TIMCON", args: []string{"%S", "%U", "#0"},
		post: []string{"SUBL2\t%G,%S", "SBWC\t%G+4,%S+4", "CLRL\t%Z", "TSTL\t%S+4", "BNEQ\t%X", "CMPL\t%S,#10000000", "BGTRU\t%X", "MOVL\t#1,%Z", "%X:"},
		show: []show{mem("%Z", 1)}})

	// $BINUTC: local text to an octaword in the local zone.
	add(binutc("BINUTC 1999", " 4-MAR-1999 13:30:15.25"))
	add(binutc("BINUTC base date", "17-NOV-1858 00:00:00.00"))
	add(binutc("BINUTC leap day", "29-FEB-2000 23:59:59.99"))
	add(binutc("BINUTC delta", "5 3:00:00.00"))
	add(binutc("BINUTC bad", "BAD"))
	add(binutc("BINUTC 9999", "31-DEC-9999 23:59:59.99"))

	// $TIMCON, both ways.
	add(timcon("TIMCON 1 base", []string{quad("%S", 0), fill("%U", 16)}, 1))
	add(timcon("TIMCON 1 1999", []string{quad("%S", when), fill("%U", 16)}, 1))
	add(timcon("TIMCON 1 delta", []string{quad("%S", uint64(1<<64-10_000_000)), fill("%U", 16)}, 1))
	add(timcon("TIMCON 0 eastern", []string{quad("%S", 0), eastern}, 0))
	add(timcon("TIMCON 0 plus hour", []string{quad("%S", 0), plusHour}, 0))
	add(timcon("TIMCON 0 finite", []string{quad("%S", 0), octa("%U", when+utcBase, 0, 0, 1)}, 0))
	add(timcon("TIMCON 0 version 0", []string{quad("%S", 0), octa("%U", when+utcBase, infinite, 0, 0)}, 0))
	add(timcon("TIMCON 0 version 2", []string{quad("%S", 0), octa("%U", when+utcBase, infinite, 0, 2)}, 0))
	add(timcon("TIMCON 0 before 1858", []string{quad("%S", 0), octa("%U", utcBase-ticksPerDay, infinite, 0, 1)}, 0))
	add(timcon("TIMCON 0 1582", []string{quad("%S", 0), octa("%U", 0, infinite, 0, 1)}, 0))
	add(timcon("TIMCON 2", []string{quad("%S", when), fill("%U", 16)}, 2))
	add(timcon("TIMCON 3", []string{quad("%S", when), fill("%U", 16)}, 3))

	k := timcon("TIMCON 0 no smnadr", []string{quad("%S", 0), eastern}, 0)
	k.args[0] = "0"
	add(k)

	k = timcon("TIMCON 1 no utcadr", []string{quad("%S", when), fill("%U", 16)}, 1)
	k.args[1] = "0"
	add(k)

	// $ASCUTC.
	add(ascutc("ASCUTC eastern", eastern, 23, 0))
	add(ascutc("ASCUTC eastern time", eastern, 11, 1))
	add(ascutc("ASCUTC plus hour", plusHour, 23, 0))
	add(ascutc("ASCUTC short 12", eastern, 12, 0))
	add(ascutc("ASCUTC short 5", eastern, 5, 0))
	add(ascutc("ASCUTC finite", octa("%U", when+utcBase, 12345, 0, 1), 23, 0))
	add(ascutc("ASCUTC version 2", octa("%U", when+utcBase, infinite, 0, 2), 23, 0))
	add(ascutc("ASCUTC before 1858", octa("%U", utcBase-ticksPerDay, infinite, 0, 1), 23, 0))
	add(ascutc("ASCUTC TDF -720", utcOf("%U", when, -720), 23, 0))
	add(ascutc("ASCUTC TDF 2047", octa("%U", when+utcBase, infinite, 2047, 1), 23, 0))

	k = ascutc("ASCUTC now length", eastern, 23, 0)
	k.args[2] = "0"
	k.show = []show{mem("%L", 1)}
	add(k)

	// $NUMUTC: the 13 words.
	add(numutc("NUMUTC eastern", eastern))
	add(numutc("NUMUTC plus hour", plusHour))
	add(numutc("NUMUTC finite", octa("%U", when+utcBase, ticksPerDay+(2*3600+3*60+4)*ticksPerSecond+5*100_000, 0, 1)))
	add(numutc("NUMUTC zero inaccuracy", octa("%U", when+utcBase, 0, 0, 1)))
	add(numutc("NUMUTC version 2", octa("%U", when+utcBase, infinite, 0, 2)))
	add(numutc("NUMUTC before 1858", octa("%U", utcBase-ticksPerDay, infinite, 0, 1)))

	k = numutc("NUMUTC now", eastern)
	k.data = []string{fill("%W", 14), fill("%WI", 14)}
	k.args = []string{"%W", "0"}
	k.show = []show{mem("%WI", 3)}
	add(k)

	// $GETDEV and $GETCHN: the device information buffer, whose layout no
	// manual gives. NLA0: is the same on every VMS; the others show the
	// names' and the volume's places.
	add(getdev("GETDEV NLA0", "NLA0:", dibSize))
	add(getdev("GETDEV _NLA0", "_NLA0:", dibSize))
	add(getdev("GETDEV TT", "TT:", dibSize))
	add(getdev("GETDEV SYS$DISK", "SYS$DISK", dibSize))
	add(getdev("GETDEV short 10", "NLA0:", 10))
	add(getdev("GETDEV short 44", "NLA0:", 44))
	add(getdev("GETDEV nosuch", "NOSUCH0:", dibSize))
	add(getdev("GETDEV empty", "", dibSize))

	k = getdev("GETDEV no name", "NLA0:", dibSize)
	k.args[0] = "0"
	add(k)

	k = getdev("GETDEV primary only", "NLA0:", dibSize)
	k.args = k.args[:3]
	add(k)

	k = getdev("GETDEV length only", "NLA0:", dibSize)
	k.args[2] = "0"
	k.args[4] = "0"
	add(k)

	assign := []string{"PUSHL\t#0", "PUSHL\t#0", "PUSHAW\t%C", "PUSHAB\t%N", "CALLS\t#4,G^SYS$ASSIGN"}
	add(probeCase{name: "GETCHN NLA0", data: deviceBuffers(dibSize, ascid("%N", "NLA0:"), "%C:\t.WORD\t0"),
		pre: assign, call: "SYS$GETCHN", args: []string{"#0", "%PL", "%PD", "%SL", "%SD"}, show: deviceShows(dibSize)})
	c[len(c)-1].pre = append(c[len(c)-1].pre, "MOVZWL\t%C,R2")
	c[len(c)-1].args[0] = "R2"

	add(probeCase{name: "GETCHN 0", data: deviceBuffers(dibSize), call: "SYS$GETCHN",
		args: []string{"#0", "%PL", "%PD", "%SL", "%SD"}, show: []show{mem("%PL", 1)}})
	add(probeCase{name: "GETCHN 999", data: deviceBuffers(dibSize), call: "SYS$GETCHN",
		args: []string{"#999", "%PL", "%PD", "%SL", "%SD"}, show: []show{mem("%PL", 1)}})
	add(probeCase{name: "GETCHN null args", call: "SYS$GETCHN", args: []string{"#0", "0", "0", "0", "0"}})

	return c
}

var cases = buildCases()

// The data every case may use.
var shared = []string{
	"\t.ALIGN\tQUAD",
	"SAVEP:\t.LONG\t0",
}

const programHead = `	.TITLE	SVC113	Phase 113 services probe
	.IDENT	/V1.0/

; Written by testdata/svc113/gen.go: do not edit. Calls the UTC services
; ($GETUTC, $ASCUTC, $BINUTC, $NUMUTC, $TIMCON) and the device information
; services ($GETCHN, $GETDEV), case by case, each in a routine of its own
; under HANDLER, and prints with LIB$PUT_OUTPUT what each returned in R0, any
; condition it signaled, and what it stored. See gen.go.

	$SSDEF

	.PSECT	DATA,NOEXE,WRT,QUAD
SIGCOND:	.LONG	0		; the condition HANDLER was given
SIGARG1:	.LONG	0		; an access violation's reason mask
SIGARG2:	.LONG	0		; and virtual address
LINLEN:	.WORD	0
	.ALIGN	LONG
LINDSC:	.WORD	132
	.BYTE	14,1
	.ADDRESS	LINE
PARTDSC:	.WORD	9
	.BYTE	14,1
	.LONG	0
OUTDSC:	.WORD	0
	.BYTE	14,1
	.ADDRESS	LINE
LINE:	.BLKB	132
R0CTL:	.ASCID	"!AC R0=!XL"
SIGCTL:	.ASCID	"!AC R0=!XL signal !XL !XL !XL"
MEMCTL:	.ASCID	"  !AC:"
LWCTL:	.ASCID	" !XL"
TXTCTL:	.ASCID	/  !AC: !UL "!AF"/
`

const programCode = `
	.PSECT	CODE,NOWRT,EXE

; HANDLER records the condition (and an access violation's reason mask
; and address), then unwinds to the case routine's caller, which gets
; the condition in R0. It ignores SS$_UNWIND, the call an unwind makes.
	.ENTRY	HANDLER,^M<>
	MOVL	4(AP),R0		; the signal array
	CMPL	4(R0),#SS$_UNWIND
	BEQL	9$
	MOVL	4(R0),SIGCOND
	CLRQ	SIGARG1
	CMPL	SIGCOND,#SS$_ACCVIO
	BNEQ	1$
	MOVQ	8(R0),SIGARG1
1$:	MOVL	8(AP),R1		; the mechanism array
	MOVL	SIGCOND,12(R1)		; CHF$L_MCH_SAVR0
	$UNWIND_S
9$:	RET

; SHOWR0 name, r0: the case's line.
	.ENTRY	SHOWR0,^M<>
	TSTL	SIGCOND
	BNEQ	1$
	$FAO_S	CTRSTR=R0CTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP)
	BRB	2$
1$:	$FAO_S	CTRSTR=SIGCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=8(AP),-
		P3=SIGCOND,P4=SIGARG1,P5=SIGARG2
2$:	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWMEM name, address, count: count longwords in hex.
	.ENTRY	SHOWMEM,^M<R2,R3,R4>
	MOVL	8(AP),R2
	MOVL	12(AP),R3
	$FAO_S	CTRSTR=MEMCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP)
	MOVZWL	LINLEN,R4
1$:	TSTL	R3
	BEQL	2$
	MOVAB	LINE[R4],PARTDSC+4
	$FAO_S	CTRSTR=LWCTL,OUTBUF=PARTDSC,P1=(R2)+
	ADDL2	#9,R4
	DECL	R3
	BRB	1$
2$:	MOVW	R4,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET

; SHOWTXT name, descriptor: a class S or D string's length and text.
	.ENTRY	SHOWTXT,^M<R2>
	MOVL	8(AP),R2
	MOVZWL	(R2),R0
	$FAO_S	CTRSTR=TXTCTL,OUTLEN=LINLEN,OUTBUF=LINDSC,P1=4(AP),P2=R0,P3=R0,P4=4(R2)
	MOVW	LINLEN,OUTDSC
	PUSHAQ	OUTDSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
`

// prefix substitutes the case's label prefix for "%".
func prefix(s string, n int) string {
	return strings.ReplaceAll(s, "%", fmt.Sprintf("C%d_", n))
}

// shownName is how a shown value is named in the output: its label
// without the case's prefix.
func shownName(s show) string {
	return strings.ReplaceAll(s.label, "%", "")
}

func program() string {
	var b strings.Builder

	b.WriteString(programHead)
	b.WriteString("\n; Shared data.\n")

	for _, line := range shared {
		b.WriteString(line + "\n")
	}

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ALIGN\tQUAD\n", n, c.name)

		for _, line := range c.data {
			b.WriteString(prefix(line, n) + "\n")
		}

		fmt.Fprintf(&b, "C%d_NAME:\t.ASCIC\t\"%s\"\n", n, c.name)

		for k, sh := range c.show {
			fmt.Fprintf(&b, "C%d_S%d:\t.ASCIC\t\"%s\"\n", n, k+1, shownName(sh))
		}
	}

	b.WriteString(programCode)

	// Each case's routine.
	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n; %d: %s\n\t.ENTRY\tC%d,^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11>\n\tMOVAB\tHANDLER,(FP)\n", n, c.name, n)

		for _, line := range c.pre {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		var post []string

		switch {
		case c.call == "":
			b.WriteString("\tCLRL\tR0\n")
		default:
			for _, line := range push(c.args) {
				b.WriteString("\t" + prefix(line, n) + "\n")
			}

			fmt.Fprintf(&b, "\tCALLS\t#%d,G^%s\n", len(c.args), c.call)
		}

		// The post instructions keep R0: it's saved around them.
		b.WriteString("\tPUSHL\tR0\n")

		for _, line := range append(post, c.post...) {
			b.WriteString("\t" + prefix(line, n) + "\n")
		}

		b.WriteString("\tMOVL\t(SP)+,R0\n\tRET\n")
	}

	// The main routine: each case, then its lines.
	b.WriteString("\n\t.ENTRY\tSTART,^M<>\n")

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&b, "\n\tCLRL\tSIGCOND\t\t\t; %d: %s\n\tCALLS\t#0,C%d\n\tPUSHL\tR0\n\tPUSHAB\tC%d_NAME\n\tCALLS\t#2,SHOWR0\n",
			n, c.name, n, n)

		for k, sh := range c.show {
			switch sh.kind {
			case "mem":
				fmt.Fprintf(&b, "\tPUSHL\t#%d\n\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#3,SHOWMEM\n",
					sh.count, prefix(sh.label, n), n, k+1)
			case "txt":
				fmt.Fprintf(&b, "\tPUSHAB\t%s\n\tPUSHAB\tC%d_S%d\n\tCALLS\t#2,SHOWTXT\n", prefix(sh.label, n), n, k+1)
			}
		}
	}

	b.WriteString("\n\tMOVL\t#SS$_NORMAL,R0\n\tRET\n\t.END\tSTART\n")

	return b.String()
}

// defDIB is def_dib.mar: $DIBDEF's values as global symbols, which the
// round's object, decoded as testdata/for83's decode.go decodes def_opc.obj,
// turns into a list.
const defDIB = `	.TITLE	DEF_DIB	$DIBDEF's values
; Written by testdata/svc113/gen.go: do not edit. Assembled on VMS, its object's
; global symbols are $DIBDEF's names and values.
	$DIBDEF	GLOBAL
	.END
`

func main() {
	dir := filepath.Join("testdata", "svc113")
	if err := os.WriteFile(filepath.Join(dir, "svc113.mar"), []byte(program()), 0o644); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "def_dib.mar"), []byte(defDIB), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println(len(cases), "cases")
}
