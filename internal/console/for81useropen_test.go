package console

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// useropenRoutine is a USEROPEN routine in MACRO, as testdata/for81's UOLOG
// is: it shows its arguments, the FAB, and the RAB in the probe's words, then
// by the mode SETMODE was last given opens the file: 0 $CREATE and $CONNECT, 1
// $OPEN and $CONNECT, 2 fails with RMS$_FNF, 3 $CREATE alone, 4 $CREATE with
// RFM changed to STM.
const useropenRoutine = `	.TITLE	UOLOG
	$FABDEF
	$RABDEF
	.PSECT	UODATA,NOEXE,WRT,LONG
MODE:	.LONG	0
OLEN:	.WORD	0
OBUF:	.ASCID	/                                                                                                                                /
ODSC:	.LONG	0, 0
MUOA:	.ASCID	/ uo n=!UL unit=!SL fab?=!UL rab?=!UL/
MFAB:	.ASCID	/ fab fac=!XB shr=!XB org=!XB rfm=!XB rat=!XB mrs=!UL mrn=!UL alq=!UL deq=!UL fop=!XL bks=!UL bls=!UL fsz=!UL/
MRAB:	.ASCID	/ rab rop=!XL rac=!UL mbc=!UL mbf=!UL rsz=!UL usz=!UL ksz=!UL/
MUOF:	.ASCID	/ uo fna=|!AF| dna=|!AF| nam=!UL/
MUOS:	.ASCID	/=uo-rms r=!XL stv=!XL/
	.PSECT	UOCODE,EXE,NOWRT,LONG
	.ENTRY	SETMODE,^M<>
	MOVL	@4(AP),MODE
	RET
	.ENTRY	OUT,^M<>
	PUSHAL	8(AP)
	PUSHAQ	OBUF
	PUSHAW	OLEN
	PUSHL	4(AP)
	CALLS	#4,G^SYS$FAOL
	MOVZWL	OLEN,ODSC
	MOVL	OBUF+4,ODSC+4
	PUSHAQ	ODSC
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.ENTRY	UOLOG,^M<R2,R3,R4,R5,R6>
	MOVL	4(AP),R2
	MOVL	8(AP),R3
	MOVL	12(AP),R4
	CLRL	-(SP)
	CMPB	RAB$B_BID(R3),#RAB$C_BID
	BNEQ	10$
	MOVL	#1,(SP)
10$:	CLRL	-(SP)
	CMPB	FAB$B_BID(R2),#FAB$C_BID
	BNEQ	20$
	MOVL	#1,(SP)
20$:	PUSHL	(R4)
	MOVZBL	(AP),-(SP)
	PUSHAB	MUOA
	CALLS	#5,OUT
	MOVZBL	FAB$B_FSZ(R2),-(SP)
	MOVZWL	FAB$W_BLS(R2),-(SP)
	MOVZBL	FAB$B_BKS(R2),-(SP)
	PUSHL	FAB$L_FOP(R2)
	MOVZWL	FAB$W_DEQ(R2),-(SP)
	PUSHL	FAB$L_ALQ(R2)
	PUSHL	FAB$L_MRN(R2)
	MOVZWL	FAB$W_MRS(R2),-(SP)
	MOVZBL	FAB$B_RAT(R2),-(SP)
	MOVZBL	FAB$B_RFM(R2),-(SP)
	MOVZBL	FAB$B_ORG(R2),-(SP)
	MOVZBL	FAB$B_SHR(R2),-(SP)
	MOVZBL	FAB$B_FAC(R2),-(SP)
	PUSHAB	MFAB
	CALLS	#14,OUT
	MOVZBL	RAB$B_KSZ(R3),-(SP)
	MOVZWL	RAB$W_USZ(R3),-(SP)
	MOVZWL	RAB$W_RSZ(R3),-(SP)
	MOVZBL	RAB$B_MBF(R3),-(SP)
	MOVZBL	RAB$B_MBC(R3),-(SP)
	MOVZBL	RAB$B_RAC(R3),-(SP)
	PUSHL	RAB$L_ROP(R3)
	PUSHAB	MRAB
	CALLS	#8,OUT
	MOVL	FAB$L_NAM(R2),R0
	BEQL	25$
	MOVL	#1,R0
25$:	PUSHL	R0
	PUSHL	FAB$L_DNA(R2)
	MOVZBL	FAB$B_DNS(R2),-(SP)
	PUSHL	FAB$L_FNA(R2)
	MOVZBL	FAB$B_FNS(R2),-(SP)
	PUSHAB	MUOF
	CALLS	#6,OUT
	MOVL	MODE,R6
	CMPL	R6,#2
	BNEQ	42$
	MOVL	#^X18292,R0
	RET
42$:	CMPL	R6,#4
	BNEQ	44$
	MOVB	#FAB$C_STM,FAB$B_RFM(R2)
44$:	CMPL	R6,#1
	BEQL	50$
	$CREATE	FAB=(R2)
	BRB	60$
50$:	$OPEN	FAB=(R2)
60$:	MOVL	R0,R5
	PUSHL	FAB$L_STV(R2)
	PUSHL	R5
	PUSHAB	MUOS
	CALLS	#3,OUT
	BLBC	R5,90$
	CMPL	R6,#3
	BEQL	90$
	$CONNECT RAB=(R3)
	MOVL	R0,R5
	PUSHL	RAB$L_STV(R3)
	PUSHL	R5
	PUSHAB	MUOS
	CALLS	#3,OUT
90$:	MOVL	R5,R0
	RET
	.END
`

// useropenCase is one of testdata/for81's USEROPEN cases: the routine's mode,
// the file's name (or none, for the unit's default), the OPEN's other
// keywords, its unit, whether a unit 11 makes the file first, and whether the
// CLOSE deletes it.
type useropenCase struct {
	number   int
	mode     int
	name     string
	keywords string
	unit     int
	first    bool
	delete   bool
}

var useropenCases = []useropenCase{
	{374, 0, "F301.DAT", "STATUS='NEW'", 10, false, false},
	{375, 1, "F302.DAT", "STATUS='OLD'", 10, true, false},
	{376, 2, "F303.DAT", "STATUS='NEW'", 10, false, false},
	{377, 3, "F304.DAT", "STATUS='NEW'", 10, false, false},
	{378, 4, "F305.DAT", "STATUS='NEW'", 10, false, false},
	{379, 0, "", "STATUS='NEW'", 10, false, true},
	{380, 0, "UO.DAT", "DEFAULTFILE='.XYZ', STATUS='NEW'", 10, false, true},
	{381, 0, "", "FORM='UNFORMATTED', STATUS='NEW'", 10, false, true},
	{382, 0, "", "ACCESS='DIRECT', RECL=4, STATUS='NEW'", 10, false, true},
	{383, 0, "", "ACCESS='DIRECT', RECL=40, FORM='FORMATTED', STATUS='NEW'", 10, false, true},
	{384, 0, "", "ACCESS='KEYED', STATUS='NEW'", 10, false, true},
	{385, 0, "", "ORGANIZATION='RELATIVE', RECL=10, STATUS='NEW'", 10, false, true},
	{386, 0, "", "ORGANIZATION='RELATIVE', RECL=10, ACCESS='DIRECT', MAXREC=9, STATUS='NEW'", 10, false, true},
	{387, 0, "", "RECORDTYPE='FIXED', RECL=10, STATUS='NEW'", 10, false, true},
	{388, 0, "", "RECORDTYPE='SEGMENTED', FORM='UNFORMATTED', STATUS='NEW'", 10, false, true},
	{389, 0, "", "RECORDTYPE='STREAM', STATUS='NEW'", 10, false, true},
	{390, 0, "", "RECORDTYPE='STREAM_CR', STATUS='NEW'", 10, false, true},
	{391, 0, "", "RECORDTYPE='STREAM_LF', STATUS='NEW'", 10, false, true},
	{392, 0, "", "CARRIAGECONTROL='LIST', STATUS='NEW'", 10, false, true},
	{393, 0, "", "CARRIAGECONTROL='NONE', STATUS='NEW'", 10, false, true},
	{394, 0, "", "STATUS='SCRATCH'", 10, false, true},
	{395, 0, "", "STATUS='UNKNOWN'", 10, false, true},
	{396, 0, "", "SHARED, STATUS='NEW'", 10, false, true},
	{397, 0, "", "NOSPANBLOCKS, STATUS='NEW'", 10, false, true},
	{398, 0, "", "INITIALSIZE=7, EXTENDSIZE=9, BLOCKSIZE=1024, BUFFERCOUNT=3, STATUS='NEW'", 10, false, true},
	{406, 0, "", "STATUS='NEW'", 10, false, true},
}

// useropenProgram is a case's FORTRAN program.
func useropenProgram(uc useropenCase) []string {
	lines := []string{
		fmt.Sprintf("      PROGRAM UO%d", uc.number),
		"      EXTERNAL UOLOG",
		"      INTEGER IOS",
		fmt.Sprintf("      CALL SETMODE(%d)", uc.mode),
	}

	file := ""
	if uc.name != "" {
		file = "FILE='" + uc.name + "', "
	}

	if uc.first {
		lines = append(lines, "      OPEN (11, "+file+"STATUS='NEW', IOSTAT=IOS)",
			"      WRITE (6, '(1X,A,I4)') '=OPEN', IOS", "      CLOSE (11, IOSTAT=IOS)",
			"      WRITE (6, '(1X,A,I4)') '=CLOSE', IOS")
	}

	dispose := ""
	if uc.delete {
		dispose = "DISPOSE='DELETE', "
	}

	return append(lines,
		fmt.Sprintf("      OPEN (%d, %sUSEROPEN=UOLOG, %s,", uc.unit, file, uc.keywords),
		"     1 IOSTAT=IOS)",
		"      WRITE (6, '(1X,A,I4)') '=OPEN', IOS",
		fmt.Sprintf("      CLOSE (%d, %sIOSTAT=IOS)", uc.unit, dispose),
		"      WRITE (6, '(1X,A,I4)') '=CLOSE', IOS",
		"      END")
}

var useropenStatus = regexp.MustCompile(`^=(OPEN|CLOSE) r=([0-9A-F]+)$`)

// useropenExpected is what VMS's run shows of a case, as the program prints
// it: each OPEN's and CLOSE's IOSTAT, and the routine's lines but for the XAB
// chain (VMS's has an XABSUM after the XABFHC, FTNRTL's none) and RMS's
// status values (a $CREATE's is VMS's channel; govax's RMS assigns none).
func useropenExpected(t *testing.T, text string, number int) []string {
	t.Helper()

	start := strings.Index(text, fmt.Sprintf("\n#%d ", number))
	if start < 0 {
		t.Fatalf("no case #%d", number)
	}

	var want []string

	for _, line := range strings.Split(text[start+1:], "\n")[1:] {
		switch {
		case strings.HasPrefix(line, "#"):
			return want
		case strings.HasPrefix(line, " uo xab"), strings.HasPrefix(line, " stat "):
		case useropenStatus.MatchString(line):
			m := useropenStatus.FindStringSubmatch(line)
			r, _ := strconv.ParseUint(m[2], 16, 32)

			ios := 0
			if r > 1 {
				ios = forNumber(r)
			}

			want = append(want, fmt.Sprintf("=%s%4d", m[1], ios))
		default:
			want = append(want, useropenMasked(line))
		}
	}

	return want
}

var useropenSTV = regexp.MustCompile(`stv=[0-9A-F]{8}`)

func useropenMasked(line string) string { return useropenSTV.ReplaceAllString(line, "stv=~") }

// TestFor81UserOpenOracle holds OPEN's USEROPEN= to VMS's FORRTL
// (testdata/for81, #374 to #398 and #406): the arguments its routine gets, the
// FAB and RAB as FTNRTL would give them to RMS, the names, and what OPEN does
// with the routine's open, its failure, and an unconnected RAB.
func TestFor81UserOpenOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "for81", "vax", "for81-vax.txt"))
	if err != nil {
		t.Fatal(err)
	}

	d, c, buf := shareFindConsole(t, false)

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]UOLOG.MAR"},
		rms.TextRecords, splitSource(useropenRoutine)); err != nil {
		t.Fatal(err)
	}

	deleteFilesAfter(t, d, "UOLOG")
	shareFindRun(t, d, "MACRO UOLOG")

	t.Cleanup(func() {
		for _, uc := range useropenCases {
			if uc.name != "" {
				_ = d.Dispatch("DELETE " + uc.name + ";*")
			}
		}
	})

	for _, uc := range useropenCases {
		name := fmt.Sprintf("UO%d", uc.number)

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".FOR"},
			rms.TextRecords, splitSource(strings.Join(fixedForm(useropenProgram(uc)), "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		buf.Reset()

		for _, command := range []string{"FORTRAN " + name, "MACRO " + name, "LINK " + name + ",UOLOG"} {
			if err := d.Dispatch(command); err != nil {
				t.Fatalf("%s: %v\n%s", command, err, buf.String())
			}
		}

		out := strings.ReplaceAll(shareRun(t, d, buf, name), "\r\n", "\n")
		out = out[:strings.LastIndex(out, "$STATUS")]
		shareFindRun(t, d, "DELETE "+name+".EXE;*,"+name+".OBJ;*,"+name+".MAR;*,"+name+".FOR;*")

		var got []string

		for _, line := range strings.Split(strings.TrimRight(out, " \n"), "\n") {
			got = append(got, useropenMasked(strings.TrimRight(line, " ")))
		}

		want := useropenExpected(t, string(text), uc.number)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("#%d printed:\n%s\nVMS's:\n%s", uc.number, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}
