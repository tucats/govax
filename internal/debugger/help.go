package debugger

import (
	"fmt"

	"github.com/tucats/govax/internal/help"
)

// helpWidth is the width the debugger's HELP lays its lists out at.
const helpWidth = 80

// help is the debugger's HELP: the topic looked up in the debugger's help
// library (SYS$HELP:DBG$HELP.HLB, docs/PHASE-90 - VMS help.md), whose one
// module, DEBUG, holds every command as a level-2 topic, so "HELP SET
// BREAK" is the library's DEBUG SET BREAK. It answers the one request and
// doesn't prompt: VMS's debugger's prompts, and its help rewritten, are
// the phase's subtask 9.
func (d *Dispatcher) help(topic string) error {
	out := d.Debugger.Machine.Hooks.Output()

	lib, err := d.Debugger.Console.DebuggerHelp()
	if err != nil {
		fmt.Fprintf(out, "%%DEBUG-W-NOHELP, no help library: %v\n", err)

		return nil
	}

	s := &help.Session{
		Library: lib,
		Write:   func(line string) { fmt.Fprintf(out, "%s\n", line) },
		Width:   helpWidth,
		Options: help.Options{Instructions: true},
	}

	return s.Run(append([]string{"DEBUG"}, help.Split(topic)...))
}
