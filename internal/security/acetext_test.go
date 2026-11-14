package security

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// probeNames is the rights database of Phase 93's round 3 on VMS 7.1
// (testdata/prot93/vax/round3b/sec93b.log): its accounts, their group's
// identifier, PROT93_TEAM, SYSTEM, and the environmental identifiers.
type probeNames map[string]uint32

func (n probeNames) IdentifierValue(name string) (uint32, bool) {
	v, ok := n[strings.ToUpper(name)]

	return v, ok
}

func (n probeNames) IdentifierName(id uint32) (string, bool) {
	for name, v := range n {
		if v == id {
			return name, true
		}
	}

	return "", false
}

var probe = probeNames{
	"PROT93A": 0x00800001, "PROT93B": 0x00800002, "PROT93C": 0x00C00001,
	"PROT93": 0x0080FFFF, "PROT93_TEAM": 0x80010001, "SYSTEM": 0x00010004,
	"INTERACTIVE": 0x80000003, "LOCAL": 0x80000004,
}

// TestParseACE holds ParseACE to VMS 7.1's $PARSE_ACL (round 3b's part P):
// the bytes it made, or its status and errpos.
func TestParseACE(t *testing.T) {
	cases := []struct {
		text   string
		bytes  string // hex, or "" for a refusal
		status ACEStatus
		pos    int
	}{
		{text: "(IDENTIFIER=[200,1],ACCESS=READ)", bytes: "0c010000010000000100800" + "0"},
		{text: "(IDENTIFIER=[PROT93,PROT93A],ACCESS=READ)", bytes: "0c01000001000000010080" + "00"},
		{text: "(IDENTIFIER=[200,*],ACCESS=READ)", bytes: "0c0100000100000" + "0ffff8000"},
		{text: "(IDENTIFIER=[*,*],ACCESS=READ)", bytes: "0c01000001000000ffffff3f"},
		{text: "(IDENTIFIER=PROT93,ACCESS=READ)", bytes: "0c01000001000000ffff8000"},
		{text: "(IDENTIFIER=prot93_team,ACCESS=READ)", bytes: "0c0100000100000001000180"},
		{text: "(IDENTIFIER=INTERACTIVE+LOCAL,ACCESS=READ)", bytes: "100100000100000003000080" + "04000080"},
		{text: "(IDENTIFIER=[200,1]+[200,2]+[300,1]+PROT93_TEAM,ACCESS=READ)", status: BadACE, pos: 60},
		{text: "(IDENTIFIER=[200,1],ACCESS=CONTROL+READ)", bytes: "0c0100001100000001008000"},
		{text: "(IDENTIFIER=[200,1])", bytes: "0c0100000000000001008000"},
		{text: "(IDENTIFIER=[200,1],ACCESS=BIT_5+BIT_31)", bytes: "0c0100002000008001008000"},
		{text: "(IDENT=[200,1],ACC=READ+WRI)", bytes: "0c0100000300000001008000"},
		{text: "( IDENTIFIER = [200,1] , ACCESS = READ )", bytes: "0c0100000100000001008000"},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ)   ", bytes: "0c0100000100000001008000"},
		{text: "(IDENTIFIER=[200,1],OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ)", bytes: "0c01000a0100000001008000"},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ,OPTIONS=DEFAULT)", bytes: "0c0100010100000001008000"},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ,ACCESS=WRITE)", bytes: "0c0100000300000001008000"},
		{text: "(DEFAULT_PROTECTION,S:RWED,O:RWED,G:RE,W:)", bytes: "180900000000000010000000100000001a0000001f000000"},
		{text: "(DEFAULT_PROTECTION,OPTIONS=PROTECTED,S:RWED,O:RWED,G,W)", bytes: "180900020000000010000000100000001f0000001f000000"},
		{text: "(DEFAULT_PROTECTION,S:RWEDC)", bytes: "1809000000000000000000001f0000001f0000001f000000"},
		{text: "(DEFAULT_PROTECTION,S:RWED,S:R)", bytes: "1809000000000000100000001f0000001f0000001f000000"},
		{text: "(DEFAULT_PROTECTION)", status: BadACE, pos: 19},
		{text: "(DEFAULT_PROTECTION,(S:RWED,O:RWED,G:RE,W))", status: BadACE, pos: 20},
		{text: "(DEFAULT_PROTECTION,X:R)", status: BadACE, pos: 20},
		{text: "(CREATOR)", bytes: "080d000000000000"},
		{text: "(ALARM_JOURNAL=SECURITY,ACCESS=READ+SUCCESS)", bytes: "100601000100000053454355524954" + "59"},
		{text: "(AUDIT=SECURITY,ACCESS=DELETE+FAILURE)", bytes: "10050200080000005345435552495459"},
		{text: "(ALARM=OTHER,ACCESS=READ+SUCCESS)", status: BadACE, pos: 33},
		{text: "(ALARM=SECURITY,ACCESS=READ)", status: BadACE, pos: 28},
		{text: "(SUBSYSTEM,IDENTIFIER=PROT93_TEAM)", bytes: "100b0000000000000100018000000000"},
		{text: "(SUBSYSTEM,IDENTIFIER=PROT93_TEAM,ACCESS=READ)", status: BadACE, pos: 34},
		{text: "(IDENTIFIER=NOSUCH,ACCESS=READ)", status: NoSuchIdentifier, pos: 12},
		{text: "(IDENTIFIER=[200,1],ACCESS=READX)", status: BadACE, pos: 20},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ", status: BadACE, pos: 31},
		{text: "", status: BadACE, pos: 0},
		{text: "()", status: BadACE, pos: 1},
		{text: "(IDENTIFIER=,ACCESS=READ)", status: BadACE, pos: 12},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ)X", status: BadACE, pos: 32},
		{text: "(IDENTIFIER=[200,1],ACCESS=READ),(IDENTIFIER=[200,2],ACCESS=READ)", status: BadACE, pos: 32},
		{text: "(IDENTIFIER=[400000,1],ACCESS=READ)", status: BadIdentifier, pos: 12},
		{text: "(IDENTIFIER=[1,99999],ACCESS=READ)", status: BadIdentifier, pos: 12},
		{text: "(IDENTIFIER=[200],ACCESS=READ)", status: BadIdentifier, pos: 12},
		{text: "(IDENTIFIER=[200,1,2],ACCESS=READ)", status: BadACE, pos: 12},
		{text: "(FOO=1)", status: BadACE, pos: 1},
		{text: "IDENTIFIER=[200,1],ACCESS=READ", status: BadACE, pos: 0},
		{text: "(IDENTIFIER=[200,1],FOO=BAR,ACCESS=READ)", status: BadACE, pos: 20},
		{text: "(IDENTIFIER=[200,1],OPTIONS=NOSUCH,ACCESS=READ)", status: BadACE, pos: 20},
	}

	for _, c := range cases {
		got, err := ParseACE(c.text, probe, nil)

		if c.bytes == "" {
			var bad *ACEError
			if !errors.As(err, &bad) || bad.Status != c.status || bad.Pos != c.pos {
				t.Errorf("%q: got %x, %v; want status %d at %d", c.text, got, err, c.status, c.pos)
			}

			continue
		}

		if err != nil || hex.EncodeToString(got) != c.bytes {
			t.Errorf("%q: got %x, %v; want %s", c.text, got, err, c.bytes)
		}
	}
}

// TestParseACEAccessNames: the caller's names come first, then the
// defaults (round 3b's cases 83 to 85).
func TestParseACEAccessNames(t *testing.T) {
	names := AccessNames{"LOOK", "TOUCH", "RUN", "KILL", "OWN", "X5"}
	for i := 6; i < 32; i++ {
		names = append(names, "X"+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}

	for text, want := range map[string]string{
		"(IDENTIFIER=[200,1],ACCESS=LOOK+TOUCH+OWN)": "0c0100001300000001008000",
		"(IDENTIFIER=[200,1],ACCESS=READ)":           "0c0100000100000001008000",
		"(IDENTIFIER=[200,1],ACCESS=X5+X31)":         "0c0100002000008001008000",
	} {
		got, err := ParseACE(text, probe, names)
		if err != nil || hex.EncodeToString(got) != want {
			t.Errorf("%q: got %x, %v; want %s", text, got, err, want)
		}
	}

	ace, _ := hex.DecodeString("0c0100001f00000001008000")
	if got := FormatACE(ace, probe, names); got != "(IDENTIFIER=[PROT93,PROT93A],ACCESS=LOOK+TOUCH+RUN+KILL+OWN)" {
		t.Errorf("format with names: %s", got)
	}
}

// TestFormatACE holds FormatACE to VMS 7.1's $FORMAT_ACL (round 3b's part
// F).
func TestFormatACE(t *testing.T) {
	cases := map[string]string{
		"0c0100000100000001008000":                              "(IDENTIFIER=[PROT93,PROT93A],ACCESS=READ)",
		"0c01000001000000010000c0":                              "(IDENTIFIER=%XC0000001,ACCESS=READ)",
		"0c0100000100000001000000":                              "(IDENTIFIER=[0,1],ACCESS=READ)",
		"0c010000010000000100c000":                              "(IDENTIFIER=[PROT93C],ACCESS=READ)",
		"0c0100000100000004000100":                              "(IDENTIFIER=[SYSTEM],ACCESS=READ)",
		"0c010000010000000100c800":                              "(IDENTIFIER=[310,1],ACCESS=READ)",
		"0c0100000100000000000000":                              "(IDENTIFIER=[0,0],ACCESS=READ)",
		"0c01000001000000ffff8000":                              "(IDENTIFIER=[PROT93,*],ACCESS=READ)",
		"0c01000001000000ffffff00":                              "(IDENTIFIER=[377,*],ACCESS=READ)",
		"0c01000001000000ffff0180":                              "(IDENTIFIER=%X8001FFFF,ACCESS=READ)",
		"0c0100000100000001000040":                              "(IDENTIFIER=%X40000001,ACCESS=READ)",
		"0a01000001000000" + "0100":                             "(IDENTIFIER=[0,1],ACCESS=READ)",
		"0801000001000000":                                      "(IDENTIFIER,ACCESS=READ)",
		"0c01000f0100000001008000":                              "(IDENTIFIER=[PROT93,PROT93A],OPTIONS=DEFAULT+HIDDEN+PROTECTED+NOPROPAGATE,ACCESS=READ)",
		"0c0100100100000001008000":                              "(IDENTIFIER=[PROT93,PROT93A],OPTIONS=%X1000,ACCESS=READ)",
		"10010100010000007856341201008000":                      "(IDENTIFIER=[PROT93,PROT93A],OPTIONS=%X0001,ACCESS=READ)",
		"18090000000000000000000000000000" + "0a0000000f000000": "(DEFAULT_PROTECTION,SYSTEM:RWEDC,OWNER:RWEDC,GROUP:REC,WORLD:C)",
		"180900020000000010000000100000001f0000001f000000":      "(DEFAULT_PROTECTION,SYSTEM:RWED,OWNER:RWED,GROUP:,WORLD:,OPTIONS=PROTECTED)",
		"10090000000000001000000000000000":                      "(DEFAULT_PROTECTION,SYSTEM:RWED,OWNER:RWEDC,GROUP:RWEDC,WORLD:RWEDC)",
		"080d000100000000":                                      "(CREATOR,OPTIONS=DEFAULT,ACCESS=NONE)",
		"1006030003000000" + "5345435552495459":                 "(ALARM=SECURITY,ACCESS=READ+WRITE+SUCCESS+FAILURE)",
		"0806010001000000":                                      "(ALARM=,ACCESS=READ+SUCCESS)",
		"0c0b00000100000001000180":                              "(SUBSYSTEM)",
		"0c070100cdab000044415441":                              "(APPLICATION,SIZE=%D12,FLAGS=%X0001,ACCESS=%X0000ABCD,DATA=%X41544144)",
		"0c0200000000000000000000":                              "(RMS_BI_JOURNAL,JOURNAL_FILE=DISK$:(0,0,0),JOURNAL_LEVEL=0,JOURNAL_CREATION_DATE=17-NOV-1858:00:00:00.00,JOURNAL_STREAM_INDEX=0,BACKUP_SEQUENCE_NUMBER=0,JOURNAL_CONSISTENCY_DATE=17-NOV-1858:00:00:00.00,JOURNALING_OPTIONS=NONE)",
		"0c0a00000000000000000000":                              "(RMS_RU_JOURNAL,JOURNAL_FILE=DISK$:(0,0,0),JOURNAL_LEVEL=0,JOURNAL_CREATION_DATE=17-NOV-1858:00:00:00.00,JOURNAL_STREAM_INDEX=0,JOURNALING_OPTIONS=NONE)",
		"0c0800000000000000000000":                              "(RMS_RU_DEFAULT,VOLUME_NAME=)",
		"0c0c00000000000000000000":                              "(UNKNOWN=%X0C,SIZE=%D12,FLAGS=%X0000,ACCESS=%X00000000,DATA=%X00000000)",
		"08ff000000000000":                                      "(RESERVED,SIZE=%D8,FLAGS=%X0000,ACCESS=%X00000000,DATA)",
	}

	for in, want := range cases {
		ace, err := hex.DecodeString(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}

		if got := FormatACE(ace, probe, nil); got != want {
			t.Errorf("%s:\n got %s\nwant %s", in, got, want)
		}
	}
}

// TestWrap holds Wrap to VMS 7.1's layouts of the probe's three-identifier
// ACE (round 3b's cases 158 to 165).
func TestWrap(t *testing.T) {
	ace, _ := hex.DecodeString("1401000a1f000000010080000100018003000080")
	pieces := FormatPieces(ace, probe, nil)

	cases := []struct {
		width, indent int
		trm           string
		wrap          bool
		want          string
	}{
		{30, 4, "/", true, "    (IDENTIFIER=/    [PROT93,PROT93A]+/    PROT93_TEAM+INTERACTIVE,/    OPTIONS=PROTECTED+/    NOPROPAGATE,ACCESS=READ+/    WRITE+EXECUTE+DELETE+/    CONTROL)"},
		{20, 0, "..", true, "(IDENTIFIER=..[PROT93,PROT93A]+..PROT93_TEAM+..INTERACTIVE,OPTIONS=..PROTECTED+..NOPROPAGATE,ACCESS=..READ+WRITE+EXECUTE+..DELETE+CONTROL)"},
		{5, 0, "/", true, "(/IDENTIFIER=/[PROT93,PROT93A]/+/PROT93_TEAM/+/INTERACTIVE/,/OPTIONS=/PROTECTED+/NOPROPAGATE,/ACCESS=/READ+/WRITE+/EXECUTE+/DELETE+/CONTROL)"},
		{0, 0, "/", true, "(IDENTIFIER=[PROT93,PROT93A]+PROT93_TEAM+INTERACTIVE,OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ+WRITE+EXECUTE+DELETE+CONTROL)"},
		{30, 0, "", false, "(IDENTIFIER=[PROT93,PROT93A]+PROT93_TEAM+INTERACTIVE,OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ+WRITE+EXECUTE+DELETE+CONTROL)"},
		{30, 2, "", true, "  (IDENTIFIER=[PROT93,PROT93A]  +PROT93_TEAM+INTERACTIVE,  OPTIONS=PROTECTED+  NOPROPAGATE,ACCESS=READ+  WRITE+EXECUTE+DELETE+  CONTROL)"},
		{40, 10, "<>", true, "          (IDENTIFIER=[PROT93,PROT93A]+<>          PROT93_TEAM+INTERACTIVE,<>          OPTIONS=PROTECTED+NOPROPAGATE,<>          ACCESS=READ+WRITE+EXECUTE+<>          DELETE+CONTROL)"},
	}

	for _, c := range cases {
		if got := Wrap(pieces, c.width, c.trm, c.wrap, c.indent); got != c.want {
			t.Errorf("width %d indent %d trm %q:\n got %s\nwant %s", c.width, c.indent, c.trm, got, c.want)
		}
	}

	dirdef, _ := hex.DecodeString("180900000000000010000000100000001a0000001f000000")
	if got := Wrap(FormatPieces(dirdef, probe, nil), 25, "/", true, 3); got != "   (DEFAULT_PROTECTION,/   SYSTEM:RWED,OWNER:RWED/   ,GROUP:RE,WORLD:)" {
		t.Errorf("default protection: %s", got)
	}

	alarm, _ := hex.DecodeString("10060300030000005345435552495459")
	if got := Wrap(FormatPieces(alarm, probe, nil), 20, "/", true, 3); got != "   (ALARM=SECURITY,/   ACCESS=READ+/   WRITE+SUCCESS+/   FAILURE)" {
		t.Errorf("alarm: %s", got)
	}
}

// TestMatchACE: $CHANGE_ACL's matching (round 3b's part C).
func TestMatchACE(t *testing.T) {
	h := func(s string) []byte { b, _ := hex.DecodeString(s); return b }
	cases := []struct {
		a, b string
		want bool
	}{
		{"0c0100000100000001008000", "0c0100000200000001008000", true},  // access differs
		{"0c0100000100000001008000", "0c0100020800000001008000", true},  // PROTECTED aside
		{"0c0100000100000001008000", "0c0100010100000001008000", false}, // DEFAULT counts
		{"0c0100000100000001008000", "0c0100000100000002008000", false},
		{"180900000000000010000000100000001a0000001f000000", "180900000000000010000000100000001f0000001f000000", true},
		{"10060100010000005345435552495459", "10060100020000005345435552495459", true},
		{"10060100010000005345435552495459", "10060200010000005345435552495459", false},
		{"080d000003000000", "080d000001000000", false},
	}

	for _, c := range cases {
		if got := MatchACE(h(c.a), h(c.b)); got != c.want {
			t.Errorf("MatchACE(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
