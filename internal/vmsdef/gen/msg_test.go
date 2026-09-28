package main

import (
	"strings"
	"testing"
)

// TestParseMessages covers each form of message-listing line parseMessages
// reads: a facility, a plain message, /FAO and /ID qualifiers, a quoted
// text, and a line cut before its /FAO qualifier. Comments, directives,
// and page headers are skipped.
func TestParseMessages(t *testing.T) {
	src := strings.Join([]string{
		"SYSMSG                          Message definitions          17-MAR-2001 03:15:25",
		"                         00000000  **** \t.FACILITY\tSYSTEM,0 /SHARED /SYSTEM /PREFIX=SS$_",
		"                                   **** \t.SEVERITY\tFATAL",
		"                                   **** ! a comment <with brackets>",
		"                         0000000C  **** \tACCVIO\t\t<access violation, reason mask=!XB, virtual address=!XH> /FAO=2",
		"                         00000014  **** \tBADPARAM\t<bad parameter value>",
		"                         00015803  5004 \t.FACILITY\tLIB,21 /SYSTEM",
		"                         0015C048  5243 ILLRECLN2\t<illegal record length (!UL)>/FAO=1/ID=ILLRECLEN",
		"                         0015C050  5244 QUOTED\t\"text in !AS quotes\" /FAO=1",
		"                         0015C058  5245 CUT\t<a long text !AD with !XL cut off by the listi",
		"                         0015C060  5246 CUTFAO\t<ends with !UL> /F",
	}, "\n")

	msgs, facilities, err := parseMessages(src)
	if err != nil {
		t.Fatal(err)
	}

	want := []message{
		{code: 0xC, facility: "SYSTEM", ident: "ACCVIO", text: "access violation, reason mask=!XB, virtual address=!XH", faoCount: 2},
		{code: 0x14, facility: "SYSTEM", ident: "BADPARAM", text: "bad parameter value"},
		{code: 0x15C048, facility: "LIB", ident: "ILLRECLEN", text: "illegal record length (!UL)", faoCount: 1},
		{code: 0x15C050, facility: "LIB", ident: "QUOTED", text: "text in !AS quotes", faoCount: 1},
		{code: 0x15C058, facility: "LIB", ident: "CUT", text: "a long text !AD with !XL cut off by the listi", faoCount: 3},
		{code: 0x15C060, facility: "LIB", ident: "CUTFAO", text: "ends with !UL", faoCount: 1},
	}

	if len(msgs) != len(want) {
		t.Fatalf("got %d messages, want %d: %+v", len(msgs), len(want), msgs)
	}

	for i := range want {
		if msgs[i] != want[i] {
			t.Errorf("message %d = %+v, want %+v", i, msgs[i], want[i])
		}
	}

	if facilities[0] != "SYSTEM" || facilities[21] != "LIB" || len(facilities) != 2 {
		t.Errorf("facilities = %v, want SYSTEM (0) and LIB (21)", facilities)
	}
}

func TestParseMessages_errors(t *testing.T) {
	for name, src := range map[string]string{
		"a message before any facility": "                         0000000C  **** \tACCVIO\t<text>",
		"a duplicate message number": "                         00000000  **** \t.FACILITY\tSYSTEM,0\n" +
			"                         0000000C  **** \tONE\t<text>\n" +
			"                         0000000A  **** \tTWO\t<text>",
	} {
		if _, _, err := parseMessages(src); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestFAOParamCount(t *testing.T) {
	for text, want := range map[string]int{
		"no directives":            0,
		"!UL and !XL":              2,
		"!AD takes two":            2,
		"!/ !_ !! !%S !5*- layout": 0,
		"!%T and !%D, !8AS":        3,
	} {
		if got := faoParamCount(text); got != want {
			t.Errorf("faoParamCount(%q) = %d, want %d", text, got, want)
		}
	}
}
