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
