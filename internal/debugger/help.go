package debugger

import (
	"fmt"

	"github.com/tucats/govax/internal/help"
)

// helpWidth is the width the debugger's HELP lays its lists out at.
const helpWidth = 80

// help is the debugger's HELP: the topic looked up in the debugger's help
// library (DBG$HELP, or SYS$HELP:DBG$HELP.HLB; docs/planning/PHASE-90 - VMS
// help.md), whose one module, DEBUG, holds every command as a level-2
// topic, so "HELP SET BREAK" is the library's DEBUG SET BREAK. It prompts
// for more, as VMS 7.1's debugger does, its prompts naming DEBUG ("DEBUG
// SET Subtopic?") until they climb to "Topic?", where the words are the
// library's level-1 keys (testdata/help90's round 2). The responses are
// read from the debugger's input (readResponse). Its screen isn't
// cleared: VMS's debugger didn't clear it.
func (d *Dispatcher) help(topic string) error {
	machine := d.Debugger.Machine

	lib, err := d.Debugger.Console.DebuggerHelp()
	if err != nil {
		fmt.Fprintf(machine.Hooks.Output(), "%%DEBUG-W-NOHELP, no help library: %v\n", err)

		return nil
	}

	s := &help.Session{
		Library: lib,
		Write:   func(line string) { fmt.Fprintf(machine.Hooks.Output(), "%s\n", line) },
		Read:    d.Debugger.readResponse,
		Width:   helpWidth,
		Options: help.Options{Instructions: true},
		Prompt:  true,
	}

	return s.Run(append([]string{"DEBUG"}, help.Split(topic)...))
}
