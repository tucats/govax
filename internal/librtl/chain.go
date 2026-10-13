package librtl

import "github.com/tucats/govax/internal/corevms"

// Chaining (docs/PHASE-57, "Chaining"; the 5.0 manual's sections 2.2.2
// and 2.2.3): LIB$DO_COMMAND and LIB$RUN_PROGRAM end the image and have
// the DCL that ran it run a command, or another image, next. Each leaves
// the text with the process's DCL (corevms.CLI's Chain) and then exits
// the image as $EXIT does, its exit handlers called first, so on success
// neither returns.
//
// The exit is a tail call of $EXIT (corevms.Environment.ExitFromShim),
// which calls the exit handlers one at a time, each returning to $EXIT's
// own entry, until none is left and the image ends.

// maxCommandText is the longest command LIB$DO_COMMAND takes, DCL's
// 1024-byte command buffer (the DCL Dictionary; VMS V7.1 ran a command of
// 256 characters, where both manuals say 255: testdata/lib57; where it
// stops is unconfirmed), and maxProgramText the longest program name
// LIB$RUN_PROGRAM takes (VMS V7.1 refused 256 characters).
const (
	maxCommandText = 1024
	maxProgramText = 255
)

// chain reads the command or program name at addr and, if the process
// has a CLI and the text isn't too long, leaves it with the CLI and
// exits the image with SS$_NORMAL (unconfirmed: the manuals don't say
// which status the image ends with, and the command's own becomes
// $STATUS anyway). It returns LIB$_NOCLI (checked first, as for the
// other CLI routines), LIB$_INVARG for text longer than maxCommandText or
// maxProgramText, or LIB$_INVSTRDES.
func chain(env *corevms.Environment, addr uint32, program bool) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	text, err := readString(env, addr)
	if err != nil {
		return stringFailure(env, err)
	}

	if len(text) > maxCommandText || program && len(text) > maxProgramText {
		return libInvArg, nil
	}

	env.CLI.Chain(text, program)

	return env.ExitFromShim(ssNormal)
}

// libDoCommand is LIB$DO_COMMAND (LIB-124):
//
//	LIB$DO_COMMAND command-string
//
// command-string.rt.dx. It ends the image, and DCL then runs
// command-string as a command line typed at the command level that ran
// the image, parsed by DCL's usual rules: its status becomes $STATUS,
// and in a procedure the level's ON action applies to it.
//
// On success it doesn't return. It returns LIB$_INVARG for a command
// longer than 1024 characters, LIB$_NOCLI, or LIB$_INVSTRDES; an argument
// it can't read is an access violation, signaled (memoryFault).
func libDoCommand(env *corevms.Environment, argv []uint32) (uint32, error) {
	return chain(env, arg(argv, 0), false)
}

// libRunProgram is LIB$RUN_PROGRAM (LIB-332):
//
//	LIB$RUN_PROGRAM program-name
//
// program-name.rt.dx. It ends the image, and DCL then runs the image
// program-name names (its file type .EXE by default), as a foreign
// command would: text after the name, past a blank, is the new image's
// command text for LIB$GET_FOREIGN, by DCL's rules (uppercased), and an
// empty name runs nothing (VMS V7.1, testdata/lib57). An image that
// can't be activated is DCL's failure, shown by DCL.
//
// On success it doesn't return. It returns LIB$_INVARG for a name longer
// than 255 characters, LIB$_NOCLI, or LIB$_INVSTRDES; an argument it
// can't read is an access violation, signaled (memoryFault).
func libRunProgram(env *corevms.Environment, argv []uint32) (uint32, error) {
	return chain(env, arg(argv, 0), true)
}
