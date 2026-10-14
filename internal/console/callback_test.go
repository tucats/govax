package console

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// testCallbackCode is TEST$CALLBACK's XFC$SHIM code in
// testdata/asm/callback.asm: one no LIBRTL routine uses (missing
// routines' codes start at 1000).
const testCallbackCode = 999

// callbackCounts counts TEST$CALLBACK's calls: started, finished (it
// returned), and abandoned (a callback never came back to it).
type callbackCounts struct {
	started, finished, abandoned int
}

// testCallback returns TEST$CALLBACK (routine, count, value), a routine
// that calls back as the LIB$ file routines do (docs/PHASE-58, subtask
// 2): it calls routine count times, as CALLG (i, value), and returns the
// sum of the R0s.
func testCallback(counts *callbackCounts) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		counts.started++

		returned := false

		defer func() {
			if returned {
				counts.finished++
			} else {
				counts.abandoned++
			}
		}()

		var sum uint32

		for i := uint32(1); i <= argv[1]; i++ {
			r0, err := env.Callback(argv[0], i, argv[2])
			if err != nil {
				return 0, err
			}

			sum += r0
		}

		returned = true

		return sum, nil
	}
}

// TestCallback_assembledProgram runs testdata/asm/callback.asm: a Go
// routine calling back into the program in a loop, a callback calling
// the routine again, and a callback that a handler unwinds past, after
// which the routine works again (see the fixture's comment).
func TestCallback_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	var counts callbackCounts

	c.ProcessOne().Shims().RegisterCallback(testCallbackCode, "TEST$CALLBACK", testCallback(&counts))

	word := runFixture(t, c, "callback.asm")

	checks := []struct {
		sym  string
		want uint32
	}{
		{"SUM1", 60},
		{"CALLS1", 3 + 4 + 1}, // case 1's three, case 2's two times two, case 4's one
		{"SUM2", 9},
		{"RET3", 0x08018002}, // the condition CB3 signaled, as SUB's return
		{"SUM4", 5},
		{"SPOK", 1},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}

	if got := c.CPU.GPR(registers.R0); got != 1 {
		t.Errorf("R0 = %#x, want main's 1", got)
	}

	// Case 1, case 2's outer call and its two inner ones, case 3, and
	// case 4: six calls, of which case 3's was abandoned.
	if want := (callbackCounts{started: 6, finished: 5, abandoned: 1}); counts != want {
		t.Errorf("TEST$CALLBACK's calls %+v, want %+v", counts, want)
	}

	if out := c.Out.(*bytes.Buffer).String(); out != "" {
		t.Errorf("unexpected output %q", out)
	}
}
