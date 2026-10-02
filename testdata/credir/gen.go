//go:build ignore

// gen writes the LIB$CREATE_DIR probe (docs/PHASE-34.md, subtask 13): a VAX
// MACRO program that calls LIB$CREATE_DIR once per case and writes what it
// returned to a file, the command procedure that builds and runs it on VMS,
// and the govax console script that builds its exchange volume. Run it from
// the repository root:
//
//	go run testdata/credir/gen.go
//
// The cases come from the LIB$ manual's description of the routine (VMS
// 5.0, AA-LA76A-TE, LIB-35 to LIB-39): each argument, its defaults, and
// the conditions it documents, plus what the manual leaves open.
//
// The probe writes LIBCRD.DMP in [000000]: one variable-length record per
// case, three longwords: "STAT", the case number, and R0.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// An argument: a string by descriptor, a longword or word by reference,
// or an omitted (0) argument.
type argument struct {
	kind  byte // 'd', 'l', 'w', '0'
	text  string
	value uint32
}

func desc(s string) argument   { return argument{kind: 'd', text: s} }
func long(v uint32) argument   { return argument{kind: 'l', value: v} }
func word(v uint32) argument   { return argument{kind: 'w', value: v} }
func omitted() argument        { return argument{kind: '0'} }
func uic(g, m uint32) argument { return long(g<<16 | m) }
func long256() argument        { return argument{kind: 'L'} } // a 256-character spec
func nullDescriptor() argument { return argument{kind: 'n'} } // a 0 descriptor address

// testCase is one call. A handled case runs in a subroutine that
// establishes LIB$SIG_TO_RET first, so a condition LIB$CREATE_DIR signals
// comes back as the subroutine's status instead of ending the probe.
type testCase struct {
	comment string
	args    []argument
	handled bool
}

var cases = []testCase{
	{"defaults", []argument{desc("[P1]")}, false},
	{"again: it exists", []argument{desc("[P1]")}, false},
	{"several levels", []argument{desc("[P2.A.B]")}, false},
	{"a new leaf under existing levels", []argument{desc("[P2.A.C]")}, false},
	{"owner-UIC", []argument{desc("[P3]"), uic(0o200, 0o201)}, false},
	{"owner-UIC 0: the parent's", []argument{desc("[P3.CHILD]"), long(0)}, false},
	{"owner-UIC omitted: the parent's", []argument{desc("[P3.OMIT]")}, false},
	{"enable world, value W:R", []argument{desc("[P4]"), omitted(), word(0xF000), word(0xE000)}, false},
	{"enable 0: value ignored", []argument{desc("[P4E]"), omitted(), word(0), word(0xFFFF)}, false},
	{"value omitted with enable", []argument{desc("[P4V]"), omitted(), word(0xF000)}, false},
	{"the manual's parent, %X13FF", []argument{desc("[P5]"), omitted(), word(0xFFFF), word(0x13FF)}, false},
	{"the manual's example, %XDBFF/%X37FF", []argument{desc("[P5.EX]"), omitted(), word(0xDBFF), word(0x37FF)}, false},
	{"maximum-versions 3", []argument{desc("[P6]"), omitted(), omitted(), omitted(), word(3)}, false},
	{"maximum-versions omitted: the parent's", []argument{desc("[P6.INH]")}, false},
	{"maximum-versions 0: no limit", []argument{desc("[P6.ZERO]"), omitted(), omitted(), omitted(), word(0)}, false},
	{"all six arguments, volume 1", []argument{desc("[P7]"), uic(0o300, 0o301), word(0x000F), word(0x0000), word(2), word(1)}, false},
	{"a seventh argument (initial allocation?)", []argument{desc("[P8]"), omitted(), omitted(), omitted(), omitted(), omitted(), long(4)}, false},
	{"a device by logical name", []argument{desc("CRDDEV:[P10]")}, false},
	{"UIC format", []argument{desc("[123,321]")}, false},
	{"UIC format with owner-UIC", []argument{desc("[1,4]"), uic(0o200, 0o201)}, false},
	{"relative to the default", []argument{desc("[.SUBREL]")}, false},
	{"relative, above the MFD", []argument{desc("[-.UP]")}, false},
	{"no directory", []argument{desc("CRDDEV:")}, false},
	{"a directory from a logical name only", []argument{desc("CRDLOG:")}, false},
	{"a file name", []argument{desc("[P1]X.DAT")}, false},
	{"a version", []argument{desc("[P1];1")}, false},
	{"a wildcard", []argument{desc("[P*]")}, false},
	{"a node", []argument{desc("NODE::[P9]")}, false},
	{"no such device", []argument{desc("NOSUCH:[X]")}, false},
	{"nine levels", []argument{desc("[L1.L2.L3.L4.L5.L6.L7.L8.L9]")}, false},
	{"a 40-character name", []argument{desc("[ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCD]")}, false},
	{"a 256-character spec", []argument{long256()}, false},
	{"no arguments", nil, false},
	// VMS 7.3 signals an access violation (virtual address 4) here: on the
	// first run, unhandled, it ended the probe.
	{"a 0 descriptor address, under LIB$SIG_TO_RET", []argument{nullDescriptor()}, true},
}

func main() {
	dir := filepath.Join("testdata", "credir")

	write(filepath.Join(dir, "libcrd.mar"), probe())
	write(filepath.Join(dir, "libcrd.com"), procedure())
	write(filepath.Join(dir, "libcrd.cmd"), exchange())
}

func write(path, text string) {
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func probe() string {
	var data, code, subs strings.Builder

	for i, c := range cases {
		n := i + 1
		fmt.Fprintf(&data, "; %d: %s\n", n, c.comment)
		fmt.Fprintf(&data, "A%d:\t.LONG\t%d\n", n, len(c.args))

		var extra strings.Builder

		for j, a := range c.args {
			label := fmt.Sprintf("A%d_%d", n, j+1)

			switch a.kind {
			case '0', 'n':
				fmt.Fprintf(&data, "\t.LONG\t0\n")
			default:
				fmt.Fprintf(&data, "\t.ADDRESS %s\n", label)
			}

			switch a.kind {
			case 'd':
				fmt.Fprintf(&extra, "%s:\t.ASCID\t|%s|\n", label, a.text)
			case 'l':
				fmt.Fprintf(&extra, "%s:\t.LONG\t^X%X\n", label, a.value)
			case 'w':
				fmt.Fprintf(&extra, "%s:\t.WORD\t^X%X\n\t.ALIGN\tLONG\n", label, a.value)
			case 'L':
				fmt.Fprintf(&extra, "%s:\t.WORD\t256\n\t.BYTE\t14,1\n\t.ADDRESS %s_S\n%s_S:\t.ASCII\t|[|\n", label, label, label)
				for k := 0; k < 5; k++ {
					fmt.Fprintf(&extra, "\t.ASCII\t|%s|\n", strings.Repeat("ABCDEFGHIJ", 5))
				}
				fmt.Fprintf(&extra, "\t.ASCII\t|DIRS]|\n")
			}
		}

		data.WriteString(extra.String())

		if c.handled {
			fmt.Fprintf(&code, "\tMOVL\t#%d,STEP\n\tCALLS\t#0,C%d\n\tJSB\tSAVE\n", n, n)
			fmt.Fprintf(&subs, "; %d, with LIB$SIG_TO_RET as its handler.\n\t.ENTRY\tC%d,^M<>\n\tPUSHAB\tG^LIB$SIG_TO_RET\n\tCALLS\t#1,G^LIB$ESTABLISH\n\tCALLG\tA%d,G^LIB$CREATE_DIR\n\tRET\n\n", n, n, n)

			continue
		}

		fmt.Fprintf(&code, "\tMOVL\t#%d,STEP\n\tCALLG\tA%d,G^LIB$CREATE_DIR\n\tJSB\tSAVE\n", n, n)
	}

	return fmt.Sprintf(`	.TITLE	LIBCRD	Phase 34 LIB$CREATE_DIR probe
	.IDENT	/V1.0/

; Written by testdata/credir/gen.go. Calls LIB$CREATE_DIR once per case
; and writes "STAT", the case number, and R0 to LIBCRD.DMP.

	.PSECT	DATA,NOEXE,WRT,LONG
OFAB:	$FAB	FNM=<LIBCRD.DMP>,FAC=PUT,RFM=VAR,MRS=64
ORAB:	$RAB	FAB=OFAB,RBF=REC,RSZ=12
REC:	.ASCII	/STAT/
STEP:	.LONG	0
STAT:	.LONG	0
	.ALIGN	LONG
%s
	.PSECT	CODE,EXE,NOWRT,LONG
	.ENTRY	LIBCRD,^M<R2,R3,R4,R5,R6>
	$CREATE	FAB=OFAB
	BLBS	R0,1$
	RET
1$:	$CONNECT RAB=ORAB
	BLBS	R0,2$
	RET
2$:
%s	$CLOSE	FAB=OFAB
	MOVL	#1,R0
	RET

; SAVE writes the case number and R0.
SAVE:	MOVL	R0,STAT
	$PUT	RAB=ORAB
	RSB

%s	.END	LIBCRD
`, data.String(), code.String(), subs.String())
}

func procedure() string {
	return `$ ! LIBCRD.COM - the Phase 34 LIB$CREATE_DIR probe. Written by
$ ! testdata/credir/gen.go. Run it once, with the exchange volume's
$ ! [000000] as the default directory:
$ !
$ !     @LIBCRD/OUTPUT=LIBCRD.LOG
$ !
$ SET NOON
$ SET VERIFY
$ DEV = F$PARSE("[000000]",,,"DEVICE")
$ SET DEFAULT 'DEV'[000000]
$ DEFINE CRDDEV 'DEV'
$ DEFINE CRDLOG 'DEV'[PLOG]
$ MACRO/NOLIST LIBCRD
$ LINK LIBCRD
$ RUN LIBCRD
$ DEASSIGN CRDDEV
$ DEASSIGN CRDLOG
$ DIRECTORY/FULL [000000...]*.DIR
$ DIRECTORY/OWNER/PROTECTION/SIZE=ALL [000000...]*.DIR
`
}

func exchange() string {
	return `! LIBCRD.CMD - builds the Phase 34 LIB$CREATE_DIR probe's exchange
! volume with govax. Written by testdata/credir/gen.go. Run from the
! repository root:
!
!     govax console < testdata/credir/libcrd.cmd
!
INITIALIZE/CONTAINER "testdata/disks/libcrd-exchange.dsk" /DEVICE=RD51 LIBCRD
MOUNT/WRITE DUA1 "testdata/disks/libcrd-exchange.dsk"
COPY "testdata/credir/libcrd.mar"/HOST DUA1:[000000]LIBCRD.MAR
COPY "testdata/credir/libcrd.com"/HOST DUA1:[000000]LIBCRD.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
`
}
