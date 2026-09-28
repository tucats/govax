package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// TestOperator_assembledProgram runs testdata/asm/operator.asm
// (docs/PHASE-26.md subtask 39): an operator request with a reply
// mailbox, the reply read from it, and a broadcast.
func TestOperator_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "operator.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a call failed?)", got)
	}

	out := c.Out.(*bytes.Buffer).String()

	for _, want := range []string{
		"%%%%%%%%%%% OPCOM    ",
		"\nRequest 1, from user SYSTEM on GOVAX\nPlease mount tape 17\n",
		"\nShutting down\r",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q\nlacks %q", out, want)
		}
	}

	// The reply: code 4, status 1 at +2, the request's own code at +4,
	// the operator terminal's counted name at +10, then the text.
	reply, _ := c.Symbols.Get("REPLY")
	b := make([]byte, 30)

	if err := c.Mem.Load(c.CPU, reply, b); err != nil {
		t.Fatal(err)
	}

	name := "GOVAX$TTA0:"
	want := string([]byte{4, 0, 1, 0, 0x42, 0, 0, 0, 0, 0, byte(len(name))}) + name + "Mounted"

	if got := string(b[:len(want)]); got != want {
		t.Errorf("reply %q\nwant  %q", got, want)
	}

	if st := word("BIOSB"); st&0xFFFF != 1 || st>>16 == 0 {
		t.Errorf("broadcast IOSB %#x: want SS$_NORMAL and a terminal count", st)
	}
}
