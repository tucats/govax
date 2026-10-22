package corevms

import "strings"

// Where a message goes (docs/PHASE-70 - traceback.md, Decision 3 and
// subtask 8).
//
// $PUTMSG writes a message "to SYS$ERROR by an RMS $PUT request. If
// SYS$OUTPUT is different from SYS$ERROR, then the formatted message is
// also written to SYS$OUTPUT" (the Internals book, 30.2.2), and the
// catch-all and traceback handlers write their messages and dumps through
// it. VMS 7.1's runs with the two names defined to different files
// (testdata/tb70's split parts, round 2's N and M) added two rules:
//
//   - A success message goes to SYS$OUTPUT only (TB70BOUTN.LOG has
//     %SYSTEM-S-NORMAL, TB70BERRN.LOG doesn't). An informational one goes
//     to both.
//   - A SYS$OUTPUT file the image itself has open (it wrote to it, by
//     LIB$PUT_OUTPUT's $CREATE) gets none of them: $PUTMSG's own open of
//     it fails, and the message goes to SYS$ERROR alone (TB70OUTS.LOG
//     holds only the program's lines; its success message went nowhere).
//
// govax applies the rules to every message $PUTMSG writes; that a
// program's own $PUTMSG of a success message follows the first rule is
// unconfirmed (the probe saw only the catch-all's).

// severitySuccess is a condition value's severity field for success
// (STS$K_SUCCESS); an informational one's is 3.
const severitySuccess = 1

// putMessage writes lines, a message (and whatever follows it, a dump)
// for the condition cond, where $PUTMSG writes it (this file's opening
// comment). Two names are "different" when their process logical names
// translate to different strings (two process-permanent file strings are
// the same when they name one entry, as SYS$ERROR shares SYS$OUTPUT's at
// the terminal); a SYS$ERROR that isn't defined is SYS$OUTPUT. Each
// message line is a record put through the name's entry, when it leads
// to one (recdevice.go's putRecord).
func (env *Environment) putMessage(cond uint32, lines []string) {
	if len(lines) == 0 {
		return
	}

	output := strings.ToUpper(env.processValue("SYS$OUTPUT"))
	errorOutput := strings.ToUpper(env.processValue("SYS$ERROR"))

	if cond&7 == severitySuccess || errorOutput == "" || errorOutput == output {
		env.putMessageLines(&env.output, lines)

		return
	}

	env.putMessageLines(&env.errorOutput, lines)
	env.putMessageLines(&env.output, lines)
}

// putMessageLines writes lines to out's name, as records (putRecord), but
// not to a file the image holds (outputFile.held, outfile.go). A full
// mailbox doesn't wait: the line is left pending on its stream, to go
// when there's room, and the lines after it are lost (VMS's $PUTMSG
// likely waits for room; unconfirmed).
func (env *Environment) putMessageLines(out *processOutput, lines []string) {
	if f := env.openRecordFile(out); f != nil && f.held {
		return
	}

	for _, line := range lines {
		_, _ = env.putRecord(out, line, true)
	}
}
