package vmsdef

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestJBCMessagesAsVMS holds the JBC facility's messages to what VMS 7.1
// printed in Phase 98's probe: every "%JBC-s-IDENT, text" (or "-JBC-...")
// line in testdata/bat98/vax's logs whose identifier $JBCMSGDEF defines
// must be what LookupMessage gives for that condition value. The lines of
// BAT98Q's $GETMSG dump are numbered by condition value instead, and are
// checked by TestJBCSharedMessages.
func TestJBCMessagesAsVMS(t *testing.T) {
	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "bat98", "vax", "*.log"))
	if err != nil || len(logs) == 0 {
		t.Fatalf("no logs: %v", err)
	}

	line := regexp.MustCompile(`[%-]JBC-[WSEIF]-([A-Z0-9_]+), (.*)$`)
	checked := map[string]bool{}

	for _, path := range logs {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		for _, text := range strings.Split(string(data), "\n") {
			// The dump's lines start with the condition value.
			if regexp.MustCompile(`^  [0-9A-F]{8} %`).MatchString(text) {
				continue
			}

			m := line.FindStringSubmatch(strings.TrimRight(text, "\r"))
			if m == nil {
				continue
			}

			code, ok := Symbols["JBC$_"+m[1]]
			if !ok {
				continue
			}

			msg, ok := LookupMessage(uint32(code))
			if !ok {
				t.Errorf("%s: JBC$_%s (%%X%X) has no message, VMS printed %q", filepath.Base(path), m[1], code, m[2])

				continue
			}

			if msg.Facility != "JBC" || msg.Ident != m[1] || msg.Text != m[2] {
				t.Errorf("%s: JBC$_%s is %s-%s %q, VMS printed %q", filepath.Base(path), m[1], msg.Facility, msg.Ident, msg.Text, m[2])
			}

			checked[m[1]] = true
		}
	}

	if len(checked) < 23 {
		t.Errorf("only %d JBC messages checked", len(checked))
	}
}

// TestJBCMessageValues checks that each of the JBC messages govax has is
// keyed by its $JBCMSGDEF value.
func TestJBCMessageValues(t *testing.T) {
	for key, m := range jbcMessages {
		code, ok := Symbols["JBC$_"+m.Ident]
		if !ok {
			t.Errorf("%s: no JBC$_%s", m.Text, m.Ident)

			continue
		}

		if uint32(code)&messageKeyMask != key {
			t.Errorf("JBC$_%s is %%X%X, keyed %%X%X", m.Ident, code, key)
		}
	}
}

// TestJBCSharedMessages checks a few lines of BAT98Q's $GETMSG dump of
// the JBC facility's shared message numbers: the system message of the
// number, under JBC's name, whatever the severity. (The dump has every
// number from 0 to 1,023; govax's system message texts are VMS 7.3's
// file's, which differ from the VAX's in places, as vaxMessages says.)
func TestJBCSharedMessages(t *testing.T) {
	cases := map[uint32]string{
		0x00040001: "JBC-NORMAL normal successful completion",
		0x00040000: "JBC-NORMAL normal successful completion",
		0x0004000C: "JBC-ACCVIO access violation, reason mask=!XB, virtual address=!XL, PC=!XL, PSL=!XL",
		0x00040012: "JBC-BADPARAM bad parameter value",
		0x00040022: "JBC-NOPRIV insufficient privilege or object protection violation",
		0x00042802: "JBC-NOCMKRNL operation requires CMKRNL privilege",
		0x00042812: "JBC-NOSYSNAM operation requires SYSNAM privilege",
		0x00042892: "JBC-NOOPER operation requires OPER privilege",
	}

	for code, want := range cases {
		m, ok := LookupMessage(code)
		got := m.Facility + "-" + m.Ident + " " + m.Text

		if !ok || got != want {
			t.Errorf("%%X%08X: %q (%v), want %q", code, got, ok, want)
		}
	}
}
