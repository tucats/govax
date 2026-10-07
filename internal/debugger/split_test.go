package debugger_test

import (
	"testing"

	"github.com/tucats/govax/internal/console/consoletest"
)

// Where a command line is understood.
const (
	consoleOnly  = "console"  // only the console's grammar (console.dcl) parses it
	debuggerOnly = "debugger" // only the debugger's (debug.dcl)
	both         = "both"     // each grammar has its own version
	neither      = "neither"  // removed, or never a command
)

// TestGrammarSplit lists the commands that docs/PHASE-42.md's "command
// split" tables assign to the console and to the debugger, and checks each
// line parses in exactly the grammar(s) it should: a command that landed in
// the wrong grammar, or in neither, fails here. The console is the VMS
// command line (files, logical names, MACRO, LINK, RUN, ...); the debugger
// is the machine debugger (memory, registers, breakpoints, stepping).
//
// Only the parse is checked, not what a command does, so a line is written
// with just enough parameters to be complete.
func TestGrammarSplit(t *testing.T) {
	console := consoletest.ConsoleGrammar(t)
	debug := consoletest.DebugGrammar(t)

	cases := []struct {
		line  string
		where string
	}{
		// Verbs the console keeps: the VMS commands and the machine's
		// life cycle.
		{"ABOUT", consoleOnly},
		{"ASM", consoleOnly},
		{"ASSIGN A B", consoleOnly},
		{"BOOT", consoleOnly},
		{"COPY A B", consoleOnly},
		{"CREATE/DIRECTORY [A]", consoleOnly},
		{"DEBUG", consoleOnly},
		{"DEFINE A B", consoleOnly},
		{"DELETE A.B;1", consoleOnly},
		{"DIRECTORY", consoleOnly},
		{"DISMOUNT DUA0", consoleOnly},
		{"ECHO 1", consoleOnly},
		{"IF 1 THEN ECHO 1", consoleOnly},
		{"INITIALIZE/VAX 100", consoleOnly},
		{"LIBRARY A", consoleOnly},
		{"LINK A", consoleOnly},
		{"LOAD/ROM", consoleOnly},
		{"MACRO A", consoleOnly},
		{"MOUNT DUA0 A", consoleOnly},
		{"PRINT 1", consoleOnly},
		{"PURGE", consoleOnly},
		{"RENAME A B", consoleOnly},
		{"RUN A", consoleOnly},
		{"SAVE/ROM A", consoleOnly},
		{"TIME", consoleOnly},
		{"TYPE A", consoleOnly},
		{"VMINIT", consoleOnly},
		{"STOP SOMEONE", consoleOnly},
		{"ZERO", consoleOnly},

		// Verbs both have, each its own: starting and ending a run, help,
		// and command files.
		{"GO", both},
		{"EXECUTE 200", both},
		{"CALL A", both},
		{"EXIT", both},
		{"QUIT", both},
		{"HELP", both},
		{"INCLUDE A", both},
		{"@A", both},

		// Verbs the debugger took from the console.
		{"EXAMINE 200", debuggerOnly},
		{"EX 200", debuggerOnly},
		{"DEPOSIT 200 = 1", debuggerOnly},
		{"STEP", debuggerOnly},
		{"S", debuggerOnly},
		{"EVALUATE 1", debuggerOnly},
		{"SYMBOLIZE 200", debuggerOnly},
		{"EXAMINE/INSTRUCTION 200", debuggerOnly},

		// Verbs removed outright: EXAMINE/INSTRUCTION replaced DISASSEMBLE.
		{"DISASSEMBLE 200", neither},

		// SHOW: the console's keywords.
		{"SHOW DEFAULT", consoleOnly},
		{"SHOW LOGICAL A", consoleOnly},
		{"SHOW TRANSLATION A", consoleOnly},
		{"SHOW DEVICES", consoleOnly},
		{"SHOW VERSION", consoleOnly},
		{"SHOW QUANTUM", consoleOnly},
		{"SHOW DEBUG", consoleOnly},
		{"SHOW INSTRUCTIONS", consoleOnly},
		{"SHOW SHARE_PREFIX", consoleOnly},
		{"SHOW EXPAND", consoleOnly},
		{"SHOW ROM", consoleOnly},
		{"SHOW NVRAM", consoleOnly},
		{"SHOW STRING_POOL", consoleOnly},
		{"SHOW SYMBOLS A", consoleOnly},

		// SHOW: the debugger's.
		{"SHOW REGISTERS", debuggerOnly},
		{"SHOW R0", debuggerOnly},
		{"SHOW PC", debuggerOnly},
		{"SHOW P0BR", debuggerOnly},
		{"SHOW PSL", debuggerOnly},
		{"SHOW CPU_STATUS", debuggerOnly},
		{"SHOW CLOCK", debuggerOnly},
		{"SHOW BASE", debuggerOnly},
		{"SHOW MEMORY", both},
		{"SHOW MAPS", debuggerOnly},
		{"SHOW TB", debuggerOnly},
		{"SHOW REGIONS", debuggerOnly},
		{"SHOW PAGE 200", debuggerOnly},
		{"SHOW SCB", debuggerOnly},
		{"SHOW SHIM", debuggerOnly},
		{"SHOW EXCEPTIONS", debuggerOnly},
		{"SHOW CALLS", debuggerOnly},
		{"SHOW STACK", debuggerOnly},
		{"SHOW KSP", debuggerOnly},
		{"SHOW BREAK", debuggerOnly},
		{"SHOW TRACE", debuggerOnly},
		{"SHOW WATCH", debuggerOnly},
		{"SHOW WATCHPOINTS", debuggerOnly},
		{"SHOW STEP", debuggerOnly},
		{"SHOW MODE", debuggerOnly},
		{"SHOW IMAGE", debuggerOnly},
		{"SHOW MODULE", debuggerOnly},
		{"SHOW SYMBOL A", both}, // the console abbreviates SYMBOLS; the debugger's is VMS's SHOW SYMBOL
		{"SHOW SCOPE", debuggerOnly},
		{"SHOW LANGUAGE", debuggerOnly},
		{"SHOW SOURCE", debuggerOnly},

		// SHOW and SET RADIX: each grammar's own radix (Decision 8).
		{"SHOW RADIX", both},
		{"SET RADIX HEX", both},

		// SHOW entries that parsed and then had no handler, and were removed.
		{"SHOW ERROR", neither},
		{"SHOW ASSEMBLER_FLAGS", neither},
		{"SHOW COMMAND_ARGS", neither},
		{"SHOW SYMBOL/TEMPORARY", neither},

		// SET: the console's.
		{"SET DEFAULT A", consoleOnly},
		{"SET QUANTUM 1", consoleOnly},
		{"SET UIQUANTUM 1", consoleOnly},
		{"SET DEBUG", consoleOnly},
		{"SET VERBOSE", consoleOnly},
		{"SET VERIFY", consoleOnly},

		// SET: the debugger's.
		{"SET BREAK 200", debuggerOnly},
		{"SET TRACE 200", debuggerOnly},
		{"SET WATCH 200", debuggerOnly},
		{"SET STEP OVER", debuggerOnly},
		{"SET SOURCE A", debuggerOnly},
		{"SET MODE SYMBOLIC", debuggerOnly},
		{"SET MODULE A", debuggerOnly},
		{"SET PSL N=1", debuggerOnly},
		{"SET PTE 200 VALID=1", debuggerOnly},
		{"SET FAULT 3", debuggerOnly},
		{"SET VM", debuggerOnly},
		{"SET BASE 200", debuggerOnly},

		// CLEAR and CANCEL: the console clears its own data; the debugger
		// cancels (and takes CLEAR as a synonym for the keywords it has).
		{"CLEAR STRINGS", consoleOnly},
		{"CLEAR SYMBOL/ALL", consoleOnly},
		{"CLEAR MEMORY", both}, // the console's wipes memory; the debugger's has /STATISTICS
		{"CANCEL BREAK/ALL", debuggerOnly},
		{"CANCEL TRACE/ALL", debuggerOnly},
		{"CANCEL WATCH/ALL", debuggerOnly},
		{"CANCEL TB", debuggerOnly},
		{"CANCEL INTERRUPT/ALL", debuggerOnly},
		{"CANCEL RADIX", debuggerOnly},
		{"CLEAR TB", debuggerOnly},
		{"CLEAR BREAK/ALL", debuggerOnly},

		// CLEAR entries that parsed and then had no handler, and were removed.
		{"CLEAR ERROR", neither},
		{"CLEAR PROFILES", neither},
	}

	for _, tc := range cases {
		_, consoleErr := console.Parse(tc.line)
		_, debugErr := debug.Parse(tc.line)

		inConsole, inDebugger := consoleErr == nil, debugErr == nil

		var got string

		switch {
		case inConsole && inDebugger:
			got = both
		case inConsole:
			got = consoleOnly
		case inDebugger:
			got = debuggerOnly
		default:
			got = neither
		}

		if got != tc.where {
			t.Errorf("%-28s parses in: %-8s want: %-8s (console: %v; debugger: %v)",
				tc.line, got, tc.where, consoleErr, debugErr)
		}
	}
}
