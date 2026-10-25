package dclshell

import (
	"io"
	"strings"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
)

// The debugger's command files (docs/PHASE-73 - trace trap.md, subtask
// 5). The VMS debugger reads its commands from DBG$INPUT, which is the
// terminal unless a logical name of that name says otherwise; a command
// procedure that runs a program under the debugger defines it, often
// /USER_MODE, to a file of debugger commands. The debugger's @ reads a
// command file too. Both are files DCL names, so the debugger asks the
// console's DCL for them.

// DebuggerInput returns the file the logical name DBG$INPUT names, the
// debugger's commands for the image about to be debugged, and false when
// it names none: no such name, or a terminal (SYS$INPUT, SYS$COMMAND, a
// TT or OPA device), which is the debugger's own input already.
func (c *Interpreter) DebuggerInput() (string, bool) {
	if c.Logicals == nil {
		return "", false
	}

	e, err := c.Logicals.Translate("LNM$FILE_DEV", "DBG$INPUT", lnm.User, lnm.AttrCaseBlind)
	if err != nil || len(e.Equivalences) == 0 {
		return "", false
	}

	name := strings.TrimSpace(e.Equivalences[0].Value)
	upper := strings.ToUpper(strings.TrimSuffix(name, ":"))

	switch {
	case name == "", upper == "SYS$INPUT", upper == "SYS$COMMAND",
		strings.HasPrefix(upper, "TT"), strings.HasPrefix(upper, "OPA"), strings.HasPrefix(upper, "_TT"):
		return "", false
	}

	return name, true
}

// ReadDebuggerFile reads the debugger command file file, found as @
// finds a command procedure (openProcedure: a volume or the host, .COM
// when it has no type), and returns its full name and its lines. The file
// isn't kept open.
func (c *Interpreter) ReadDebuggerFile(file string) (string, []string, error) {
	source, err := c.openProcedure(file, false)
	if err != nil {
		return "", nil, err
	}

	if source.ownsFile {
		_ = source.file.Close()
		source.file = nil
	}

	return source.name, source.records[:source.end], nil
}

// CreateDebuggerLog creates the debugger's log file, SET LOG's, and
// returns it with its full name: a text
// file of variable-length records, on a volume or the host as DCL's OPEN
// finds it, with .LOG when the name has no type (the VMS debugger's
// default log is DEBUG.LOG). What is written to it is split into
// records at each newline; a line not yet ended goes out at Close.
func (c *Interpreter) CreateDebuggerLog(file string) (io.WriteCloser, string, error) {
	loc, err := c.ContainerSession.Locate(file, false)
	if err != nil {
		return nil, "", dclcmd.FileFailure(err, file)
	}

	loc = loc.WithDefaultType("LOG")

	f, err := c.ContainerSession.OpenDCLFile(loc, rms.DCLAccess{Write: true, Variable: true})
	if err != nil {
		return nil, "", err
	}

	w := &recordWriter{put: f.Put, done: f.Close}

	return closingWriter{w}, f.Name, nil
}

// closingWriter is a recordWriter as an io.WriteCloser.
type closingWriter struct{ *recordWriter }

func (w closingWriter) Close() error { return w.close() }
