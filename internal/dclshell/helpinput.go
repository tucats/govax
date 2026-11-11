package dclshell

// What DCL's HELP needs of the DCL running it (docs/planning/PHASE-90 - VMS help.md,
// subtask 6): where its answers come from, and whether its output goes to
// a file.

// maxHelpResponse is the longest response HELP takes: RMS's largest
// record, as for the other commands that read SYS$INPUT.
const maxHelpResponse = 32767

// ReadHelpResponse reads HELP's response to prompt. VMS 7.1's HELP reads
// SYS$INPUT: in a command procedure the data lines after the command,
// shown by SET VERIFY and not prompted for; with SYS$INPUT defined as a
// file, that file's records; and at the terminal a line typed after the
// prompt (testdata/help90's part D and round 2). ok is false at the
// input's end (Ctrl/Z at the terminal), which ends HELP.
func (c *Interpreter) ReadHelpResponse(prompt string) (string, bool) {
	if c.RTL != nil {
		if line, ok, handled := c.RTL.ReadInputRecord(maxHelpResponse); handled {
			return line, ok
		}
	}

	return c.readTerminal(prompt)
}

// EndHelpInput closes a file of HELP's responses SYS$INPUT named, as an
// image's rundown would.
func (c *Interpreter) EndHelpInput() {
	if c.RTL != nil {
		c.RTL.CloseInputFile()
	}
}

// OutputToFile reports whether the DCL's output goes to a file now: a
// command procedure's /OUTPUT file, at its level or one around it. HELP
// lays its lists out 132 columns wide there, as for a file of its own
// (testdata/help90's logs).
func (c *Interpreter) OutputToFile() bool {
	level := c.currentLevel()

	return level != nil && level.redirected
}
