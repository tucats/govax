package debugger

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// The debugger's output and its log (docs/PHASE-73 - trace trap.md,
// subtask 5), as the VMS 5.5 Debugger Manual's 8.1 and 8.3 describe them:
//
//   - SET OUTPUT [NO]VERIFY: whether the lines of a command procedure (@)
//     are echoed as they are read, each after a blank, between
//     %DEBUG-I-VERIFYICF's "entering" and "exiting" lines.
//   - SET OUTPUT [NO]TERMINAL: whether the debugger's output reaches the
//     terminal (DBG$OUTPUT, govax's console output).
//   - SET OUTPUT [NO]LOG, SET LOG file: whether the session is written to
//     a log file, DEBUG.LOG unless SET LOG names another. The log has
//     each command read from the debugger's input (DBG$INPUT, or the
//     terminal) as it was read, and every line of the debugger's output
//     after a "!", so the log can be run as a command procedure; the
//     program's own output isn't in it.
//   - SHOW OUTPUT and SHOW LOG say how things stand.
//
// The debugger's output is everything that goes through the machine's
// output while a session is in progress (output); the program's goes to
// its terminal another way, so it is never logged.

// outputSettings are SET OUTPUT's and SET LOG's.
type outputSettings struct {
	verify   bool
	terminal bool
	logging  bool

	// logName is SET LOG's file, empty for the default (DEBUG.LOG), and
	// logFile the full name of the file being written, which log writes.
	logName, logFile string
	log              io.WriteCloser

	// midLine says the log's last output line has no newline yet, so the
	// next output continues it rather than starting a new "!" line.
	midLine bool
}

// defaultLogName is the log SET OUTPUT LOG writes when SET LOG has named
// none (the manual: SYS$DISK:[]DEBUG.LOG).
const defaultLogName = "DEBUG.LOG"

// installOutput makes the machine's output, while a session is in
// progress, the debugger's: to the terminal unless SET OUTPUT
// NOTERMINAL, and to the log, if one is being written.
func (d *Debugger) installOutput() {
	terminal := d.Machine.Hooks.Output
	if terminal == nil {
		return
	}

	d.Machine.Hooks.Output = func() io.Writer {
		out := terminal()

		if !d.active && d.running == 0 {
			return out
		}

		// A procedure line's echo comes before the first output of its
		// command (dbginput.go).
		if e := d.pendingEcho; e != "" {
			d.pendingEcho = ""
			_, _ = fmt.Fprintln(d.Machine.Hooks.Output(), e)
		}

		if !d.output.logging && d.output.terminal {
			return out
		}

		return sessionWriter{d: d, terminal: out}
	}
}

// sessionWriter is the debugger's output while it logs or doesn't show it
// at the terminal.
type sessionWriter struct {
	d        *Debugger
	terminal io.Writer
}

func (w sessionWriter) Write(p []byte) (int, error) {
	if w.d.output.terminal && w.terminal != nil {
		if _, err := w.terminal.Write(p); err != nil {
			return 0, err
		}
	}

	if w.d.output.logging {
		w.d.logOutput(p)
	}

	return len(p), nil
}

// logOutput writes the debugger's output p to the log, each line after a
// "!".
func (d *Debugger) logOutput(p []byte) {
	o := &d.output
	if o.log == nil {
		return
	}

	var b bytes.Buffer

	for len(p) > 0 {
		if !o.midLine {
			b.WriteByte('!')
			o.midLine = true
		}

		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			b.Write(p)

			break
		}

		b.Write(p[:i+1])
		p = p[i+1:]
		o.midLine = false
	}

	_, _ = o.log.Write(b.Bytes())
}

// logInput writes a command read from the debugger's input to the log as
// it was read.
func (d *Debugger) logInput(line string) {
	o := &d.output
	if !o.logging || o.log == nil {
		return
	}

	if o.midLine {
		_, _ = io.WriteString(o.log, "\n")
		o.midLine = false
	}

	_, _ = io.WriteString(o.log, line+"\n")
}

// openLog starts writing the log, SET LOG's file or DEBUG.LOG, closing
// one already open.
func (d *Debugger) openLog() error {
	o := &d.output
	d.closeLog()

	name := o.logName
	if name == "" {
		name = defaultLogName
	}

	w, full, err := d.Console.CreateDebuggerLog(name)
	if err != nil {
		return err
	}

	o.log, o.logFile, o.midLine = w, full, false

	return nil
}

// closeLog closes the log file, if one is open.
func (d *Debugger) closeLog() {
	o := &d.output

	if o.log != nil {
		if o.midLine {
			_, _ = io.WriteString(o.log, "\n")
		}

		_ = o.log.Close()
	}

	o.log, o.logFile, o.midLine = nil, "", false
}

// setLog is SET LOG file: the log's name; if the session is being
// logged, logging goes on in the new file.
func (d *Debugger) setLog(file string) error {
	// A host file's name may be quoted, as at DCL's OPEN.
	file = strings.TrimSpace(file)
	if len(file) >= 2 && file[0] == '"' && file[len(file)-1] == '"' {
		file = file[1 : len(file)-1]
	}

	d.output.logName = file

	if d.output.logging {
		return d.openLog()
	}

	return nil
}

// setOutput is SET OUTPUT keyword[,keyword...]: [NO]LOG, [NO]VERIFY,
// [NO]TERMINAL, and [NO]SCREEN_LOG, which govax accepts and ignores (it
// has no screen mode). A keyword it doesn't know is %DEBUG-E-SYNTAX; the
// ones before it have taken effect.
func (d *Debugger) setOutput(words string) error {
	fields := strings.FieldsFunc(words, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(fields) == 0 {
		return vmserrors.New(vmserrors.DBG_SYNTAX, words)
	}

	for _, word := range fields {
		w := strings.ToUpper(strings.TrimPrefix(word, "/"))
		on := !strings.HasPrefix(w, "NO")
		w = strings.TrimPrefix(w, "NO")

		is := func(full string) bool { return len(w) >= 3 && strings.HasPrefix(full, w) }

		switch {
		case is("LOG"):
			if on && !d.output.logging {
				if err := d.openLog(); err != nil {
					return err
				}
			}

			if !on {
				d.closeLog()
			}

			d.output.logging = on

		case is("VERIFY"):
			d.output.verify = on

		case is("TERMINAL"):
			d.output.terminal = on

		case is("SCREEN_LOG"):

		default:
			return vmserrors.New(vmserrors.DBG_SYNTAX, word)
		}
	}

	return nil
}

// showOutput is SHOW OUTPUT, in the manual's form:
//
//	noverify, terminal, noscreen_log, logging to DSK2:[JONES.P7]DEBUG.LOG;1
func (d *Debugger) showOutput() error {
	o := d.output

	words := []string{onOff(o.verify, "verify"), onOff(o.terminal, "terminal"), "noscreen_log"}

	if o.logging {
		words = append(words, "logging to "+o.logFile)
	} else {
		words = append(words, "nolog")
	}

	d.Machine.Printf("%s\n", strings.Join(words, ", "))

	return nil
}

// showLog is SHOW LOG: whether the session is being logged, and to what
// file (govax's words, unconfirmed).
func (d *Debugger) showLog() error {
	o := d.output

	switch {
	case o.logging:
		d.Machine.Printf("logging to %s\n", o.logFile)
	case o.logName != "":
		d.Machine.Printf("not logging; the log file is %s\n", o.logName)
	default:
		d.Machine.Printf("not logging; the log file is %s\n", defaultLogName)
	}

	return nil
}

// onOff is word, or "no" and word.
func onOff(on bool, word string) string {
	if on {
		return word
	}

	return "no" + word
}

// bindOutput binds SET LOG, SET OUTPUT, SHOW LOG, and SHOW OUTPUT.
func (d *Dispatcher) bindOutput() {
	d.Grammar.Bind("SET_LOG", func(id int64, r *dcl.Result) error {
		return d.Debugger.setLog(r.String("FILE"))
	})
	d.Grammar.Bind("SET_OUTPUT", func(id int64, r *dcl.Result) error {
		return d.Debugger.setOutput(r.String("WORDS"))
	})
	d.Grammar.Bind("SHOW_LOG", func(id int64, r *dcl.Result) error { return d.Debugger.showLog() })
	d.Grammar.Bind("SHOW_OUTPUT", func(id int64, r *dcl.Result) error { return d.Debugger.showOutput() })
}
