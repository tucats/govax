// Package diag holds the FORTRAN compiler's diagnostic messages: the
// errors and warnings it reports about a source program.
//
// A compiler's messages are its user interface. A good one says what is
// wrong, where, and shows the line, so the programmer needn't hunt. Every
// pass of govax's FORTRAN compiler reports through this package, so
// messages look the same whichever pass found the problem. A message
// looks like this (docs/PHASE-74 - FORTRAN plan.md, Decision 19):
//
//	%FORT-E-UNDEFLBL, statement label 20 is never defined
//	  at line 14 of DUA0:[WORK]SQUARES.FOR;1
//	          GO TO 20
//	          ------^
//
// The first line is in VMS's message form: the facility (FORT, the
// compiler), a severity letter, an identifier, and the text. The second
// says where; the last two quote the source line and point at the column
// the problem starts in.
//
// Each kind of message is an entry in one table (messages.go), keyed by
// its identifier, which fixes its severity and wording. A pass reports a
// message by naming its identifier and giving the arguments its text
// needs; it never writes message text of its own. That keeps the wording
// consistent and lets tests list every message the compiler can give.
//
// The compiler goes on after most errors, so that one compilation reports
// as many problems as it can. Whether to write the MACRO file at the end
// is the List's question: only if nothing worse than a warning was
// reported, as MACRO writes no object file after an error.
package diag

import (
	"fmt"
	"strings"
)

// Severity is how serious a message is. VMS gives every status one of
// five severities; a compiler uses four of them (success is never a
// message).
type Severity int

// The severities, least serious first, so that they compare: Error >
// Warning.
const (
	// Informational is a remark: nothing is wrong, but something is worth
	// knowing (a feature the compiler treats specially, say).
	Informational Severity = iota
	// Warning is something probably wrong that still compiles: the MACRO
	// file is written.
	Warning
	// Error is something wrong: compilation goes on to find more
	// problems, but no MACRO file is written.
	Error
	// Fatal is a problem the compiler can't go past (an INCLUDE file it
	// can't read, say): it stops at once.
	Fatal
)

// Letter is the severity's letter in a VMS message: I, W, E, or F.
func (s Severity) Letter() string {
	return [...]string{"I", "W", "E", "F"}[s]
}

// Status is the severity as the low three bits of a VMS condition value
// (STS$K_WARNING 0, STS$K_SUCCESS 1, STS$K_ERROR 2, STS$K_INFO 3,
// STS$K_SEVERE 4), for a command that sets $STATUS from its worst
// message.
func (s Severity) Status() uint32 {
	return [...]uint32{3, 0, 2, 4}[s]
}

// String is the severity's name, for tests and dumps.
func (s Severity) String() string {
	return [...]string{"informational", "warning", "error", "fatal"}[s]
}

// Position is where in the source a message is about.
type Position struct {
	// File is the source file's full specification, as the user should
	// see it (a volume file's with its version, or a host path).
	File string

	// Line is the line number within File, counted from 1; 0 means the
	// message is about the file as a whole.
	Line int

	// Column is the character within the line, counted from 1; 0 means
	// the whole line, and no pointer is drawn.
	Column int

	// Text is the source line itself, quoted under the message.
	Text string
}

// Message is one reported problem.
type Message struct {
	Severity Severity
	Ident    string // the message's identifier: UNDEFLBL
	Text     string // its text, with the arguments filled in
	Pos      Position
}

// String is the message as the FORTRAN command prints it: the VMS-style
// first line, then, if the message has a line, where it is, the line, and
// a pointer to its column. The lines are separated by newlines, with no
// newline at the end.
func (m Message) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%%FORT-%s-%s, %s", m.Severity.Letter(), m.Ident, m.Text)

	switch {
	case m.Pos.Line > 0:
		fmt.Fprintf(&b, "\n  at line %d of %s", m.Pos.Line, m.Pos.File)
	case m.Pos.File != "":
		fmt.Fprintf(&b, "\n  in %s", m.Pos.File)
	}

	if m.Pos.Line > 0 && m.Pos.Text != "" {
		fmt.Fprintf(&b, "\n  %s", m.Pos.Text)

		if m.Pos.Column > 0 {
			fmt.Fprintf(&b, "\n  %s", pointer(m.Pos.Text, m.Pos.Column))
		}
	}

	return b.String()
}

// pointer is the line drawn under a quoted source line to point at a
// column: dashes up to it, then a caret. Where the source line has a tab
// the pointer has one too, so the caret lines up with the column however
// wide the terminal shows a tab. (FORTRAN's tab-format lines put tabs
// near the start of most lines, so this matters.)
func pointer(text string, column int) string {
	var b strings.Builder

	for i := 0; i < column-1; i++ {
		if i < len(text) && text[i] == '\t' {
			b.WriteByte('\t')
		} else {
			b.WriteByte('-')
		}
	}

	b.WriteByte('^')

	return b.String()
}

// List collects the messages of one compilation, in the order they were
// reported.
type List struct {
	messages []Message
}

// Report records the message whose identifier is ident (an entry of the
// table in messages.go), about the place pos, with args filling in its
// text's format verbs. An identifier not in the table is a mistake in
// the compiler, not in the program being compiled, so it panics.
func (l *List) Report(ident string, pos Position, args ...any) Message {
	kind, ok := messages[ident]
	if !ok {
		panic("diag: no message " + ident)
	}

	m := Message{
		Severity: kind.Severity,
		Ident:    ident,
		Text:     fmt.Sprintf(kind.Format, args...),
		Pos:      pos,
	}

	l.messages = append(l.messages, m)

	return m
}

// Messages returns the messages reported, in order.
func (l *List) Messages() []Message {
	return append([]Message(nil), l.messages...)
}

// Count returns how many messages of severity s were reported.
func (l *List) Count(s Severity) int {
	n := 0

	for _, m := range l.messages {
		if m.Severity == s {
			n++
		}
	}

	return n
}

// Worst returns the most serious severity reported, and false if nothing
// was.
func (l *List) Worst() (Severity, bool) {
	if len(l.messages) == 0 {
		return Informational, false
	}

	worst := Informational

	for _, m := range l.messages {
		worst = max(worst, m.Severity)
	}

	return worst, true
}

// Failed reports whether an error or a fatal error was reported: if so,
// the compilation produces no MACRO file.
func (l *List) Failed() bool {
	worst, reported := l.Worst()

	return reported && worst >= Error
}
