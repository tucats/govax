package corevms

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/tucats/govax/internal/sched"
)

// scriptedTerminal is a TerminalSource whose input the test types: a
// read finds only what has been typed, and Ready says whether there is
// any.
type scriptedTerminal struct {
	typed []byte
}

func (s *scriptedTerminal) Read(p []byte) (int, error) {
	if len(s.typed) == 0 {
		return 0, io.EOF
	}

	n := copy(p, s.typed)
	s.typed = s.typed[n:]

	return n, nil
}

func (s *scriptedTerminal) Ready() bool { return len(s.typed) > 0 }

func (s *scriptedTerminal) typeIn(text string) { s.typed = append(s.typed, text...) }

// TestTerminal_queue: two processes read the one terminal, each waiting
// in LEF until its line is there and its turn has come; the second's
// prompt is written only when the first has its line, and each prompt
// only once; what was typed ahead stays for the next read.
func TestTerminal_queue(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	term := &scriptedTerminal{}

	var out bytes.Buffer

	a, b := newProcess(t, env), newProcess(t, env)
	for _, p := range []*Environment{a, b} {
		p.consoleIn, p.consoleOut = term, &out
	}

	read := func(p *Environment, prompt string) (string, error) {
		line, _, err := p.ReadInputLine(prompt, 80)
		p.enterWait(errors.Is(err, ErrWait))

		return line, err
	}

	if _, err := read(a, "A> "); !errors.Is(err, ErrWait) {
		t.Fatalf("A's read with nothing typed: %v, want ErrWait", err)
	}

	wantState(t, a, sched.StateLEF, sched.ResourceNone)

	if _, err := read(b, "B> "); !errors.Is(err, ErrWait) {
		t.Fatalf("B's read: %v, want ErrWait", err)
	}

	if out.String() != "A> " {
		t.Errorf("prompts written %q, want only A's", out.String())
	}

	// A partial line ends no wait; a whole one ends A's (the scheduler's
	// test of its wait), not B's: A's turn is first.
	term.typeIn("hel")
	env.pollEvents()
	wantState(t, a, sched.StateLEF, sched.ResourceNone)

	term.typeIn("lo\rworld\r")
	env.pollEvents()
	wantState(t, a, sched.StateCOM, sched.ResourceNone)
	wantState(t, b, sched.StateLEF, sched.ResourceNone)

	// A's read made again gets its line, without its prompt again; B's
	// turn comes, with its prompt, and the line typed ahead is there.
	if line, err := read(a, "A> "); err != nil || line != "hello" {
		t.Fatalf("A's read: %q, %v; want hello", line, err)
	}

	if out.String() != "A> B> " {
		t.Errorf("prompts written %q, want A's, then B's", out.String())
	}

	env.pollEvents()
	wantState(t, b, sched.StateCOM, sched.ResourceNone)

	if line, err := read(b, "B> "); err != nil || line != "world" {
		t.Fatalf("B's read: %q, %v; want world", line, err)
	}

	if out.String() != "A> B> " || len(env.terminalQueue) != 0 {
		t.Errorf("after both: prompts %q, queue %d; want each once, empty", out.String(), len(env.terminalQueue))
	}
}

// TestTerminal_deletion: a process deleted while its read waits gives up
// its turn, and the next read's comes.
func TestTerminal_deletion(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	term := &scriptedTerminal{}

	var out bytes.Buffer

	a, b := newProcess(t, env), newProcess(t, env)
	for _, p := range []*Environment{a, b} {
		p.consoleIn, p.consoleOut = term, &out
	}

	_, _, _ = a.ReadInputLine("A> ", 80)
	_, _, _ = b.ReadInputLine("B> ", 80)

	a.processRundown()

	if out.String() != "A> B> " || len(env.terminalQueue) != 1 || env.terminalQueue[0].env != b {
		t.Errorf("after A's rundown: prompts %q, queue %d; want B's turn", out.String(), len(env.terminalQueue))
	}
}

// TestTerminal_withoutScheduler: with the scheduler off, a read doesn't
// wait in the scheduler: it reads at once, as it always has.
func TestTerminal_withoutScheduler(t *testing.T) {
	env, _ := fixture()

	term := &scriptedTerminal{}
	term.typeIn("now\r")
	env.consoleIn = term

	if line, ok, err := env.ReadInputLine("", 80); err != nil || !ok || line != "now" {
		t.Errorf("read: %q, %v, %v; want now", line, ok, err)
	}
}

// TestTerminal_inputShims: EXE$INPUT and DECC$GETS read the shared
// terminal as LIB$GET_INPUT does: with no whole line typed, the shim
// waits in LEF (ErrWait) rather than stopping the machine in the host's
// read, and once the line is there it's read, EXE$INPUT's without its
// newline, DECC$GETS's with it.
func TestTerminal_inputShims(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	term := &scriptedTerminal{}
	env.consoleIn = term

	a := newArena(t, env)
	buf := a.alloc(80)

	if _, err := shimExeInput(env, []uint32{buf, 80}); !errors.Is(err, ErrWait) {
		t.Fatalf("EXE$INPUT with nothing typed: %v, want ErrWait", err)
	}

	env.enterWait(true)
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	term.typeIn("half")
	env.pollEvents()
	wantState(t, env, sched.StateLEF, sched.ResourceNone)

	term.typeIn(" a line\nnext\n")
	env.pollEvents()
	wantState(t, env, sched.StateCOM, sched.ResourceNone)

	n, err := shimExeInput(env, []uint32{buf, 80})
	if err != nil || a.readString(buf, uint16(n)) != "half a line" {
		t.Errorf("EXE$INPUT: %d, %v; want the line without its newline", n, err)
	}

	if addr, err := shimDeccGets(env, []uint32{buf}); err != nil || addr != buf || a.readString(buf, 5) != "next\n" {
		t.Errorf("DECC$GETS: %08X, %v, %q; want the line with its newline", addr, err, a.readString(buf, 5))
	}

	if len(env.terminalQueue) != 0 {
		t.Errorf("the terminal's queue has %d reads, want none", len(env.terminalQueue))
	}
}

// pendingFixture is an Environment with the scheduler on, its console a
// scriptedTerminal, a terminal TTA0 and a user-mode channel to it.
func pendingFixture(t *testing.T) (*Environment, *scriptedTerminal, *bytes.Buffer, *arena, uint32) {
	t.Helper()

	env, out, a, ch := qioFixture(t, "")
	withScheduler(env)

	term := &scriptedTerminal{}
	env.consoleIn = term

	return env, term, out, a, ch
}

// TestTerminal_pendingQIO: a $QIO read with nothing typed returns
// SS$_NORMAL at once, the read pending (its event flag clear, its IOSB
// untouched), and the process goes on; its prompt is written once; a
// partial line completes nothing; the whole line completes the read at
// the scheduler's next look: the buffer, the IOSB, the event flag, the
// AST.
func TestTerminal_pendingQIO(t *testing.T) {
	env, term, out, a, ch := pendingFixture(t)
	iosb, buf := a.alloc(8), a.alloc(20)

	wantR0(t, callQIO(t, env, qioArgs{
		efn: 5, channel: ch, function: fnReadPrompt, iosb: iosb, astadr: 0x4000, astprm: 9,
		p: [6]uint32{buf, 20, 0, 0, a.str("> "), 2},
	}), ssNormal)

	if env.PendingIO() != 1 || flagSet(env, 5) || a.readLong(iosb) != 0 {
		t.Fatalf("after $QIO: pending %d, flag %v, IOSB %08X; want the read pending", env.PendingIO(), flagSet(env, 5), a.readLong(iosb))
	}

	term.typeIn("ab")
	env.pollEvents()

	if env.PendingIO() != 1 {
		t.Fatal("a partial line completed the read")
	}

	term.typeIn("c\r")
	env.pollEvents()

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 3 || info != 0x1000D {
		t.Errorf("IOSB = %d, %d, %#x; want SS$_NORMAL, 3, 0x1000D", st, n, info)
	}

	if got := a.readString(buf, 3); got != "abc" {
		t.Errorf("buffer %q, want abc", got)
	}

	if !flagSet(env, 5) || len(env.Process.ast.queue) != 1 || env.PendingIO() != 0 || len(env.terminalQueue) != 0 {
		t.Errorf("flag %v, ASTs %d, pending %d, queue %d; want set, 1, 0, 0",
			flagSet(env, 5), len(env.Process.ast.queue), env.PendingIO(), len(env.terminalQueue))
	}

	if out.String() != "> " {
		t.Errorf("output %q, want the prompt once", out.String())
	}
}

// TestTerminal_typedAhead: a $QIO read whose line is already typed, with
// no read ahead of it, completes during the $QIO.
func TestTerminal_typedAhead(t *testing.T) {
	env, term, _, a, ch := pendingFixture(t)
	iosb, buf := a.alloc(8), a.alloc(20)

	term.typeIn("now\r")

	wantR0(t, callQIO(t, env, qioArgs{efn: 1, channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 20}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != ssNormal || n != 3 || env.PendingIO() != 0 || len(env.terminalQueue) != 0 {
		t.Errorf("IOSB %d, %d; pending %d, queue %d; want completed at once", st, n, env.PendingIO(), len(env.terminalQueue))
	}
}

// TestTerminal_pendingOrder: reads complete in the order they were made.
// A synchronous read made after a pending $QIO read waits behind it,
// though a line is there, and gets the second line; a second $QIO read
// gets the third.
func TestTerminal_pendingOrder(t *testing.T) {
	env, term, _, a, ch := pendingFixture(t)
	other := newProcess(t, env)
	other.consoleIn = term

	iosb1, buf1 := a.alloc(8), a.alloc(20)
	iosb2, buf2 := a.alloc(8), a.alloc(20)

	wantR0(t, callQIO(t, env, qioArgs{efn: 1, channel: ch, function: fnReadVBlk, iosb: iosb1, p: [6]uint32{buf1, 20}}), ssNormal)

	term.typeIn("one\rtwo\rthree\r")

	if _, _, err := other.ReadInputLine("", 80); !errors.Is(err, ErrWait) {
		t.Fatalf("the read behind the $QIO's: %v, want ErrWait", err)
	}

	other.enterWait(true)

	wantR0(t, callQIO(t, env, qioArgs{efn: 2, channel: ch, function: fnReadVBlk, iosb: iosb2, p: [6]uint32{buf2, 20}}), ssNormal)

	// The scheduler's look completes the first $QIO read and ends the
	// synchronous read's wait; the second $QIO read is behind it.
	env.pollEvents()

	if got := a.readString(buf1, 3); got != "one" || !flagSet(env, 1) || flagSet(env, 2) {
		t.Fatalf("first read %q, flags 1 %v, 2 %v; want one, set, clear", got, flagSet(env, 1), flagSet(env, 2))
	}

	wantState(t, other, sched.StateCOM, sched.ResourceNone)

	if line, _, err := other.ReadInputLine("", 80); err != nil || line != "two" {
		t.Fatalf("synchronous read %q, %v; want two", line, err)
	}

	// Its turn over, the second $QIO read completes with the line there.
	if st, n, _ := readIOSB(a, iosb2); st != ssNormal || n != 5 || a.readString(buf2, 5) != "three" || len(env.terminalQueue) != 0 {
		t.Errorf("second read %d, %d, %q, queue %d; want SS$_NORMAL, 5, three, empty", st, n, a.readString(buf2, 5), len(env.terminalQueue))
	}
}

// TestTerminal_cancelPending: $CANCEL completes a pending read with
// SS$_CANCEL and takes it out of the terminal's queue, so what is typed
// next goes to the next read.
func TestTerminal_cancelPending(t *testing.T) {
	env, term, _, a, ch := pendingFixture(t)
	iosb, buf := a.alloc(8), a.alloc(20)

	wantR0(t, callQIO(t, env, qioArgs{efn: 1, channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 20}}), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)

	if st, _, _ := readIOSB(a, iosb); st != uint16(ssCancel) || !flagSet(env, 1) || len(env.terminalQueue) != 0 {
		t.Fatalf("IOSB %d, flag %v, queue %d; want SS$_CANCEL, set, empty", st, flagSet(env, 1), len(env.terminalQueue))
	}

	term.typeIn("later\r")

	if line, _, err := env.ReadInputLine("", 80); err != nil || line != "later" {
		t.Errorf("next read %q, %v; want later", line, err)
	}
}
