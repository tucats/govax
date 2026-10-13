package librtl

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestLibChain: LIB$DO_COMMAND and LIB$RUN_PROGRAM leave their text with
// the CLI and end the image by a tail call of $EXIT, status SS$_NORMAL;
// with no CLI they return LIB$_NOCLI, and text past the limit (a
// program name's 255 characters, a command's 1024) is LIB$_INVARG,
// leaving nothing chained.
func TestLibChain(t *testing.T) {
	for _, c := range []struct {
		routine string
		program bool
		limit   int
	}{
		{"LIB$DO_COMMAND", false, 1024},
		{"LIB$RUN_PROGRAM", true, 255},
	} {
		t.Run(c.routine, func(t *testing.T) {
			env := fixture(t)
			env.CPU().SetGPR(registers.SP, 0xF0000)
			a := newArena(t, env)

			if r0 := call(t, env, c.routine, a.desc("SHOW TIME")); r0 != libNoCLI {
				t.Errorf("no CLI: status %#x, want LIB$_NOCLI", r0)
			}

			cli := newFakeCLI()
			env.CLI = cli

			if r0 := call(t, env, c.routine, a.desc(strings.Repeat("X", c.limit+1))); r0 != libInvArg {
				t.Errorf("%d characters: status %#x, want LIB$_INVARG", c.limit+1, r0)
			}

			if len(cli.calls) != 0 {
				t.Errorf("%d characters chained: %v", c.limit+1, cli.calls)
			}

			text := strings.Repeat("Y", c.limit)

			// The image exits by a tail call of $EXIT, with SS$_NORMAL.
			_, err := shim(t, env, c.routine)(env, []uint32{a.desc(text)})

			var call *corevms.CallRequest
			if !errors.As(err, &call) || !call.Tail || call.Routine != exitEntry() {
				t.Fatalf("255 characters: %v, want a tail call of $EXIT", err)
			}

			if got := longword(t, env, call.ArgList+4); got != ssNormal || longword(t, env, call.ArgList) != 1 {
				t.Errorf("$EXIT's status %#x, want SS$_NORMAL", got)
			}

			if cli.chained != text || cli.program != c.program {
				t.Errorf("chained %q (program %v)", cli.chained, cli.program)
			}
		})
	}
}

// exitEntry is the SYS$EXIT P1-vector entry's address.
func exitEntry() uint32 {
	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$EXIT" {
			return e.Addr
		}
	}

	return 0
}
