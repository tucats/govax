package main

import (
	"bytes"
	"io"
	"sync"
)

// promptOutput is the console's output (Console.Out, and every process's
// terminal output through it) when govax reads its command lines from a
// terminal. Most of the time it writes straight to the terminal. While a
// command line is being read and the other processes run meanwhile
// (docs/PHASE-51, subtask 4: Console.LendWhileReading), what they write
// would land in the middle of the line being typed. So while the line is
// read (begin to end), whole lines go through the line editor's own
// writer (readline's Stdout), which takes the prompt and the line typed
// so far off the screen, writes, and puts them back below. Text with no
// line end yet (a prompt of a subprocess's own) is held until its line
// ends, or the read does: the editor's redraw would erase it otherwise.
type promptOutput struct {
	mu sync.Mutex

	// out is the terminal.
	out io.Writer

	// editor is the line editor's writer while a line is read, else nil.
	editor io.Writer

	// held is text written while a line is read that doesn't end a line
	// yet.
	held []byte
}

// newPromptOutput returns the console's output, writing to out.
func newPromptOutput(out io.Writer) *promptOutput {
	return &promptOutput{out: out}
}

// Write writes b to the terminal, or, while a line is read, its whole
// lines through the line editor (see above). It reports all of b
// written unless the terminal fails.
func (p *promptOutput) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.editor == nil {
		return p.out.Write(b)
	}

	p.held = append(p.held, b...)

	end := bytes.LastIndexByte(p.held, '\n')
	if end < 0 {
		return len(b), nil
	}

	lines := p.held[:end+1]
	p.held = append([]byte(nil), p.held[end+1:]...)

	if _, err := p.editor.Write(lines); err != nil {
		return 0, err
	}

	return len(b), nil
}

// begin says a command line is being read, editor being the line
// editor's writer.
func (p *promptOutput) begin(editor io.Writer) {
	p.mu.Lock()
	p.editor = editor
	p.mu.Unlock()
}

// end says the read is over (the line editor has ended its line), and
// writes what was held.
func (p *promptOutput) end() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.editor = nil

	if len(p.held) > 0 {
		_, _ = p.out.Write(p.held)
		p.held = nil
	}
}

// Fd is the terminal's file descriptor, so the pager can ask its size
// (internal/dclcmd/page.go); ^0, no file, when out has none.
func (p *promptOutput) Fd() uintptr {
	if f, ok := p.out.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}

	return ^uintptr(0)
}
