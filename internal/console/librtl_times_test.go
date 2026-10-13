package console

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestLibrtlTimeArithmetic: a program works a delta time through all four
// arithmetic routines by their stubs, then writes the result with
// $ASCTIM and LIB$PUT_OUTPUT (docs/PHASE-56, subtask 2). One hour times 3
// is 3 hours; plus an hour, 4; times 1.5, 6; minus an hour, 5. Each
// routine returns LIB$_NORMAL, whose low bit the program tests.
func TestLibrtlTimeArithmetic(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"HOUR:\t.LONG\t^X9E3B9800,^XFFFFFFF7\n"+ // a delta of one hour
		"WORK:\t.LONG\t^X9E3B9800,^XFFFFFFF7\n"+
		"SUM:\t.QUAD\t0\n"+
		"DIFF:\t.QUAD\t0\n"+
		"THREE:\t.LONG\t3\n"+
		"HALF:\t.F_FLOATING\t1.5\n"+
		"TEXT:\t.ASCID\t/................/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tWORK\n\tPUSHAL\tTHREE\n"+
		"\tCALLS\t#2,G^LIB$MULT_DELTA_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tSUM\n\tPUSHAQ\tHOUR\n\tPUSHAQ\tWORK\n"+
		"\tCALLS\t#3,G^LIB$ADD_TIMES\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tSUM\n\tPUSHAF\tHALF\n"+
		"\tCALLS\t#2,G^LIB$MULTF_DELTA_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tDIFF\n\tPUSHAQ\tHOUR\n\tPUSHAQ\tSUM\n"+
		"\tCALLS\t#3,G^LIB$SUB_TIMES\n\tBLBC\tR0,DONE\n"+
		"\tCLRL\t-(SP)\n\tPUSHAQ\tDIFF\n\tPUSHAQ\tTEXT\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#4,G^SYS$ASCTIM\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tTEXT\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#1,R0\nDONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 1 {
		t.Errorf("R0 = %#x, want 1", r0)
	}

	if want := "   0 05:00:00.00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestLibrtlTimeConversion: a program turns $NUMTIM's vector for
// 13-JAN-1988 09:13:25.14 into a time with LIB$CVT_VECTIM and writes it,
// then makes a delta of 6 hours with LIB$CVT_TO_INTERNAL_TIME, takes it
// to F-floating days and back (LIB$CVTF_FROM_ and LIB$CVTF_TO_), and
// writes that (a quarter day is exact in F_floating; 5 hours, 5/24 of a
// day, would come back a fraction of a millisecond short); it ends with the date's hour of the year (297, counted from 0) in R0, from
// LIB$CVT_FROM_INTERNAL_TIME (docs/PHASE-56, subtask 3). The operation
// codes are LIB$K_HOUR_OF_YEAR (3), LIB$K_DELTA_HOURS (23), and
// LIB$K_DELTA_DAYS_F (27).
func TestLibrtlTimeConversion(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"VECTOR:\t.WORD\t1988,1,13,9,13,25,14\n"+
		"DATE:\t.QUAD\t0\n"+
		"DELTA:\t.QUAD\t0\n"+
		"HOY:\t.LONG\t3\n"+
		"HOURS:\t.LONG\t23\n"+
		"DAYSF:\t.LONG\t27\n"+
		"SIX:\t.LONG\t6\n"+
		"DAYS:\t.F_FLOATING\t0\n"+
		"RESULT:\t.LONG\t0\n"+
		"TEXT:\t.ASCID\t/......................./\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAW\tVECTOR\n"+
		"\tCALLS\t#2,G^LIB$CVT_VECTIM\n\tBLBC\tR0,DONE\n"+
		"\tMOVZWL\t#23,TEXT\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAQ\tTEXT\n\tBSBW\tSHOW\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAL\tSIX\n\tPUSHAL\tHOURS\n"+
		"\tCALLS\t#3,G^LIB$CVT_TO_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAF\tDAYS\n\tPUSHAL\tDAYSF\n"+
		"\tCALLS\t#3,G^LIB$CVTF_FROM_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		// DONE is mid-way, so every BLBC reaches it.
		"\tBRB\tGO\nDONE:\tRET\nGO:"+
		"\tCLRQ\tDELTA\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAF\tDAYS\n\tPUSHAL\tDAYSF\n"+
		"\tCALLS\t#3,G^LIB$CVTF_TO_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tMOVZWL\t#16,TEXT\n"+
		"\tPUSHAQ\tDELTA\n\tPUSHAQ\tTEXT\n\tBSBW\tSHOW\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAL\tRESULT\n\tPUSHAL\tHOY\n"+
		"\tCALLS\t#3,G^LIB$CVT_FROM_INTERNAL_TIME\n\tBLBC\tR0,DONE\n"+
		"\tMOVL\tRESULT,R0\n\tRET\n"+
		// SHOW: $ASCTIM of the time pushed into the descriptor pushed,
		// then LIB$PUT_OUTPUT of it.
		"SHOW:\tMOVL\t(SP)+,R2\n\tMOVQ\t(SP)+,R3\n"+
		"\tCLRL\t-(SP)\n\tPUSHL\tR4\n\tPUSHL\tR3\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#4,G^SYS$ASCTIM\n"+
		"\tPUSHL\tR3\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tJMP\t(R2)\n"+
		"\t.END\tSTART\n")

	if r0 != 297 {
		t.Errorf("R0 = %#x, want 297", r0)
	}

	if want := "13-JAN-1988 09:13:25.14\n   0 06:00:00.00\n"; out != want {
		t.Errorf("output %q, want %q", out, want)
	}
}

// TestLibrtlDays: a program reads 13-JAN-1988 09:13:25.14 with $BINTIM,
// takes its day number and day-time with LIB$DAY and its day of the week
// with LIB$DAY_OF_WEEK, writes the current time from LIB$DATE_TIME into
// a dynamic string, and ends with day number x 10 + day of the week in R0
// (47,173 and Wednesday, 3), or 0 if the day-time isn't 3,320,514
// hundredths (docs/PHASE-56, subtask 4).
func TestLibrtlDays(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"TIME:\t.ASCID\t/13-JAN-1988 09:13:25.14/\n"+
		"DATE:\t.QUAD\t0\n"+
		"DAYS:\t.LONG\t0\n"+
		"HUND:\t.LONG\t0\n"+
		"WDAY:\t.LONG\t0\n"+
		"NOW:\t.WORD\t0\n\t.BYTE\t14,2\n\t.LONG\t0\n"+ // a dynamic string
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tDATE\n\tPUSHAQ\tTIME\n"+
		"\tCALLS\t#2,G^SYS$BINTIM\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAL\tHUND\n\tPUSHAQ\tDATE\n\tPUSHAL\tDAYS\n"+
		"\tCALLS\t#3,G^LIB$DAY\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAL\tWDAY\n\tPUSHAQ\tDATE\n"+
		"\tCALLS\t#2,G^LIB$DAY_OF_WEEK\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tNOW\n"+
		"\tCALLS\t#1,G^LIB$DATE_TIME\n\tBLBC\tR0,DONE\n"+
		"\tPUSHAQ\tNOW\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tCLRL\tR0\n"+
		"\tCMPL\tHUND,#3320514\n\tBNEQ\tDONE\n"+
		"\tMULL3\t#10,DAYS,R0\n\tADDL2\tWDAY,R0\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 471733 {
		t.Errorf("R0 = %d, want 471733", r0)
	}

	// The current time varies, but not its form.
	if !regexp.MustCompile(`^[ 1-3]\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d\n$`).MatchString(out) {
		t.Errorf("output %q, want an $ASCTIM time", out)
	}
}

// TestLibrtlDateContext: LIB$INIT_DATE_TIME_CONTEXT and
// LIB$FREE_DATE_TIME_CONTEXT through their stubs: INIT allocates a
// context into a zero cell, refuses a component with no name
// (LIB$_ILLCOMPONENT, 0x15843C), and FREE zeroes the cell; FREE without
// an argument frees the static context.
func TestLibrtlDateContext(t *testing.T) {
	r0, _ := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"CTX:\t.LONG\t0\n"+
		"COMP:\t.LONG\t12\n"+ // LIB$K_MERIDIEM_INDICATOR
		"BAD:\t.LONG\t5\n"+
		"MI:\t.ASCID\t/|AM|PM|/\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tPUSHAQ\tMI\n\tPUSHAL\tCOMP\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBC\tR0,DONE\n"+
		"\tTSTL\tCTX\n\tBEQL\tFAIL\n"+
		"\tPUSHAQ\tMI\n\tPUSHAL\tBAD\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n"+
		"\tCMPL\tR0,#^X15843C\n\tBNEQ\tFAIL\n"+
		"\tPUSHAL\tCTX\n\tCALLS\t#1,G^LIB$FREE_DATE_TIME_CONTEXT\n\tBLBC\tR0,DONE\n"+
		"\tTSTL\tCTX\n\tBNEQ\tFAIL\n"+
		"\tCALLS\t#0,G^LIB$FREE_DATE_TIME_CONTEXT\n\tBLBC\tR0,DONE\n"+
		"\tMOVL\t#42,R0\n\tRET\n"+
		"FAIL:\tCLRL\tR0\n"+
		"DONE:\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 42 {
		t.Errorf("R0 = %#x, want 42", r0)
	}
}

// TestLibrtlDateOutput: a program sets a context's output and input
// formats, then asks LIB$GET_MAXIMUM_DATE_LENGTH for the widest text (27,
// the manual's example), formats a time with LIB$FORMAT_DATE_TIME, and
// writes it and LIB$GET_DATE_FORMAT's legible input format with
// LIB$PUT_OUTPUT (docs/PHASE-56, subtask 6).
func TestLibrtlDateOutput(t *testing.T) {
	v := vmsdef.Time(time.Date(1988, time.September, 21, 23, 24, 0, 0, time.UTC))

	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"CTX:\t.LONG\t0\n"+
		"OUTF:\t.LONG\t1\n"+ // LIB$K_OUTPUT_FORMAT
		"LANG:\t.LONG\t4\n"+ // LIB$K_LANGUAGE
		"ENGL:\t.ASCID\t/|ENGLISH|/\n"+
		"INF:\t.LONG\t2\n"+ // LIB$K_INPUT_FORMAT
		"OFMT:\t.ASCID\t/|!MAU !DD, !Y4|!HH2:!M0 !MIU|/\n"+
		"IFMT:\t.ASCID\t/|!MN0-!D0-!Y2 !H04:!M0:!S0.!C2|/\n"+
		fmt.Sprintf("TIME:\t.LONG\t^X%08X,^X%08X\n", uint32(v), uint32(v>>32))+
		"LEN:\t.LONG\t0\n"+
		"MAX:\t.LONG\t0\n"+
		"BUF:\t.LONG\t^X010E0028,TEXT\n"+ // 40 bytes, fixed
		"TEXT:\t.BLKB\t40\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tBRB\tGO\n"+ // the exits, within a byte branch of every test
		"FAIL:\tCLRL\tR0\n"+
		"DONE:\tRET\n"+
		"GO:\tPUSHAQ\tENGL\n\tPUSHAL\tLANG\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBS\tR0,1$\n\tRET\n1$:"+
		"\tPUSHAQ\tOFMT\n\tPUSHAL\tOUTF\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBS\tR0,2$\n\tRET\n2$:"+
		"\tPUSHAQ\tIFMT\n\tPUSHAL\tINF\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBS\tR0,3$\n\tRET\n3$:"+
		"\tPUSHAL\tCTX\n\tPUSHAL\tMAX\n"+
		"\tCALLS\t#2,G^LIB$GET_MAXIMUM_DATE_LENGTH\n\tBLBS\tR0,4$\n\tRET\n4$:"+
		"\tCMPL\tMAX,#27\n\tBNEQ\tFAIL\n"+
		"\tPUSHAL\tLEN\n\tPUSHAL\tCTX\n\tPUSHAQ\tTIME\n\tPUSHAQ\tBUF\n"+
		"\tCALLS\t#4,G^LIB$FORMAT_DATE_TIME\n\tBLBS\tR0,5$\n\tRET\n5$:"+
		"\tMOVW\tLEN,BUF\n"+
		"\tPUSHAQ\tBUF\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVW\t#40,BUF\n"+
		"\tPUSHAL\tCTX\n\tPUSHAQ\tBUF\n"+
		"\tCALLS\t#2,G^LIB$GET_DATE_FORMAT\n\tBLBS\tR0,PUT\n\tRET\n"+
		"PUT:\tPUSHAQ\tBUF\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#42,R0\n\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 42 {
		t.Errorf("R0 = %#x, want 42", r0)
	}

	if want := regexp.MustCompile(`(?m)^SEPTEMBER 21, 1988 11:24 PM\n^MM-DD-YYYY2 HH:MM:SS\.CC2 +\n`); !want.MatchString(out) {
		t.Errorf("output:\n%s", out)
	}
}

// TestLibrtlDateInput: LIB$CONVERT_DATE_STRING through its stub, by a
// context's input format: a 12-hour time read back, which LIB$SYS_ASCTIM
// shows, and the fields it defaulted, the seconds and fraction (0x60).
func TestLibrtlDateInput(t *testing.T) {
	r0, out := runMacroProgram(t, "\t.PSECT\tDATA,NOEXE,WRT,LONG\n"+
		"CTX:\t.LONG\t0\n"+
		"INF:\t.LONG\t2\n"+ // LIB$K_INPUT_FORMAT
		"IFMT:\t.ASCID\t/|!MAU !DD, !Y4 !H02:!M0:!S0.!C2 !MIU|/\n"+
		"LANG:\t.LONG\t4\n"+ // LIB$K_LANGUAGE
		"ENGL:\t.ASCID\t/|ENGLISH|/\n"+
		"STR:\t.ASCID\t/feb 25, 1988 4:39 pm/\n"+
		"TIME:\t.QUAD\t0\n"+
		"DEFS:\t.LONG\t0\n"+
		"BUF:\t.LONG\t^X010E0017,TEXT\n"+ // 23 bytes, fixed
		"TEXT:\t.BLKB\t23\n"+
		"\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n"+
		"\tBRB\tGO\n"+
		"FAIL:\tCLRL\tR0\n"+
		"DONE:\tRET\n"+
		"GO:\tPUSHAQ\tENGL\n\tPUSHAL\tLANG\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBS\tR0,1$\n\tRET\n1$:"+
		"\tPUSHAQ\tIFMT\n\tPUSHAL\tINF\n\tPUSHAL\tCTX\n"+
		"\tCALLS\t#3,G^LIB$INIT_DATE_TIME_CONTEXT\n\tBLBS\tR0,2$\n\tRET\n2$:"+
		"\tPUSHAL\tDEFS\n\tCLRL\t-(SP)\n\tCLRL\t-(SP)\n\tPUSHAL\tCTX\n\tPUSHAQ\tTIME\n\tPUSHAQ\tSTR\n"+
		"\tCALLS\t#6,G^LIB$CONVERT_DATE_STRING\n\tBLBS\tR0,3$\n\tRET\n3$:"+
		"\tCMPL\tDEFS,#^X60\n\tBNEQ\tFAIL\n"+
		"\tCLRL\t-(SP)\n\tPUSHAQ\tTIME\n\tPUSHAQ\tBUF\n\tCLRL\t-(SP)\n"+
		"\tCALLS\t#4,G^LIB$SYS_ASCTIM\n\tBLBS\tR0,4$\n\tRET\n4$:"+
		"\tPUSHAQ\tBUF\n\tCALLS\t#1,G^LIB$PUT_OUTPUT\n"+
		"\tMOVL\t#42,R0\n\tRET\n"+
		"\t.END\tSTART\n")

	if r0 != 42 {
		t.Errorf("R0 = %#x, want 42", r0)
	}

	if !strings.Contains(out, "25-FEB-1988 16:39:00.00\n") {
		t.Errorf("output:\n%s", out)
	}
}
