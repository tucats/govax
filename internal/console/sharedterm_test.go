package console_test

import (
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 7: the shared terminal. A process waiting for a
// terminal line waits in the scheduler, so another process runs; CTRL/C
// goes to the process that enabled an AST for it, though another is the
// one reading.

// typedInput is the console's input in these tests: what the test has
// "typed", and a terminal source that says whether there is any.
type typedInput struct {
	mu    sync.Mutex
	typed []byte
}

func (s *typedInput) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.typed) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.typed)
	s.typed = s.typed[n:]

	return n, nil
}

func (s *typedInput) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.typed) > 0
}

func (s *typedInput) typeIn(text string) {
	s.mu.Lock()
	s.typed = append(s.typed, text...)
	s.mu.Unlock()
}

// ttAssign assigns the terminal, its channel in R6.
const ttAssign = `
	clrq	-(sp)			; mbxnam, acmode
	pushal	@#chanw			; chan
	pushaq	@#ttnam			; devnam
	calls	#4, @#sys$assign
	movzwl	@#chanw, r6
`

// TestSharedTerminal: process 1 reads a line with $QIOW and waits, in
// LEF, while process 2 counts; CTRL/C runs process 2's CTRL/C AST, not
// stopping the machine; the line typed completes process 1's read.
func TestSharedTerminal(t *testing.T) {
	one, _ := assembleAt(t, mbxSymbols()+ttAssign+`
	clrl	r1			; efn
	movl	#IO$_READVBLK, r2
	moval	@#data+16, r3		; iosb
	moval	@#data+32, r4		; buffer
	movl	#20, r5			; size
	jsb	@#qiow
	movl	r0, @#data+12
	brw	fin
ttnam:	.ascid	"TTA0:"
`+mbxCommon)

	two, _ := assembleAt(t, mbxSymbols()+ttAssign+fmt.Sprintf(`
	clrl	r1
	movl	#IO$_SETMODE!^X%X, r2
	clrl	r3
	moval	@#ast, r4		; p1: the AST routine
	clrl	r5
	jsb	@#qiow
	movl	r0, @#data+12
count:	incl	@#data+40
	brb	count
ast:	.word	0
	movl	#1, @#data+44
	ret
ttnam:	.ascid	"TTA0:"
`, vmsdef.Symbols["IO$M_CTRLCAST"])+mbxCommon)

	c, _ := scheduledConsole(t, "200", one)
	term := &typedInput{}
	c.In = term

	p := &mbxPair{t: t, c: c, one: c.RTL}
	p.two = handBuiltProcess(t, c, two)

	stepN := func(n int) {
		for range n {
			step(t, c, 1)
		}
	}

	// Process 1 waits for its line; process 2 runs.
	stepN(5000)

	if s := stateOf(p.one); s != sched.StateLEF || p.at(p.one, 12) != 0 {
		t.Fatalf("process 1: %s, $QIOW %08X; want waiting in LEF", s, p.at(p.one, 12))
	}

	before := p.at(p.two, 40)
	stepN(2000)

	if p.at(p.two, 40) == before {
		t.Fatal("process 2 doesn't run while process 1 waits for the terminal")
	}

	// CTRL/C: process 2's AST, though process 1 is the one reading.
	if !c.HandleAttention(corevms.AttentionCtrlC) {
		t.Fatal("CTRL/C wasn't taken: the machine would stop")
	}

	stepN(500)

	if p.at(p.two, 44) != 1 {
		t.Error("process 2's CTRL/C AST didn't run")
	}

	if s := stateOf(p.one); s != sched.StateLEF {
		t.Errorf("process 1 after CTRL/C: %s, want still waiting", s)
	}

	// The line.
	term.typeIn("hello\r")

	for range 5000 {
		if p.at(p.one, 0) == 1 {
			break
		}

		step(t, c, 1)
	}

	if p.at(p.one, 12) != 1 || p.text(p.one, 32, 5) != "hello" {
		t.Fatalf("process 1's read: %08X, %q; want SS$_NORMAL, hello", p.at(p.one, 12), p.text(p.one, 32, 5))
	}

	p.iosb(p.one, 16, "read", 1, 5, 0x1000D) // ended by RETURN, a 1-byte terminator
}

// TestPendingTerminalRead (docs/PHASE-49.md, subtask 10): a $QIO read
// with nothing typed returns SS$_NORMAL at once and the program goes on
// working, counting until its IOSB is written; the line typed then
// completes the read, and the program sees it.
func TestPendingTerminalRead(t *testing.T) {
	one, _ := assembleAt(t, mbxSymbols()+ttAssign+`
	movl	#4, r1			; efn
	movl	#IO$_READVBLK, r2
	moval	@#data+16, r3		; iosb
	moval	@#data+32, r4		; buffer
	movl	#20, r5			; size
	jsb	@#qio
	movl	r0, @#data+12
work:	incl	@#data+60
	tstw	@#data+16
	beql	work
	brw	fin
ttnam:	.ascid	"TTA0:"
`+mbxCommon)

	c, _ := scheduledConsole(t, "200", one)
	term := &typedInput{}
	c.In = term

	p := &mbxPair{t: t, c: c, one: c.RTL}

	for range 3000 {
		step(t, c, 1)
	}

	if p.at(p.one, 12) != 1 || p.at(p.one, 16) != 0 || p.at(p.one, 60) < 100 {
		t.Fatalf("$QIO %08X, IOSB %08X, count %d; want SS$_NORMAL, the read pending, the program working",
			p.at(p.one, 12), p.at(p.one, 16), p.at(p.one, 60))
	}

	term.typeIn("typed\r")

	for range 3000 {
		if p.at(p.one, 0) == 1 {
			break
		}

		step(t, c, 1)
	}

	if p.at(p.one, 0) != 1 || p.text(p.one, 32, 5) != "typed" {
		t.Fatalf("finished %d, buffer %q; want the read completed with typed", p.at(p.one, 0), p.text(p.one, 32, 5))
	}

	p.iosb(p.one, 16, "read", 1, 5, 0x1000D)
}
