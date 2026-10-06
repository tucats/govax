package console

import (
	"os"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"golang.org/x/term"
)

// This file shows long text a screenful at a time, as VMS's TYPE/PAGE
// does: after each screen it prompts "Press RETURN to continue" and waits.
// RETURN alone shows the next screen; anything typed before RETURN ends
// the display there (successfully: it's how the user says "enough").
//
// The screen's size is the real terminal's, when the console's output is
// one; otherwise (output redirected to a file or pipe, or a host where the
// size can't be read) the vax.console.lines setting gives the number of
// lines, and with that unset, a VT100's 24.

// consoleLinesSetting is the configuration key giving the screen's height
// when the terminal can't be asked.
const consoleLinesSetting = "vax.console.lines"

// defaultScreenLines is a VT100's height, the last resort.
const defaultScreenLines = 24

// pagePrompt is what VMS shows at the foot of each screen.
const pagePrompt = "Press RETURN to continue"

// screenSize returns the screen's height in lines and its width in
// columns. width is 0 when it isn't known, in which case long lines
// aren't counted as wrapping.
//
// ScreenSize, when set, overrides the terminal (tests use it).
func (c *Console) screenSize() (height, width int) {
	if c.ScreenSize != nil {
		return c.ScreenSize()
	}

	// c.Out is an *os.File only when it's the process's real standard
	// output (or another real file); term.IsTerminal then says whether
	// it's a terminal whose size can be read.
	if f, ok := c.Out.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if w, h, err := term.GetSize(int(f.Fd())); err == nil && h > 1 {
			return h, w
		}
	}

	if n := settings.GetInt(consoleLinesSetting); n > 1 {
		return n, 0
	}

	return defaultScreenLines, 0
}

// Page writes text to the console's output a screenful at a time. Each
// screen leaves its last line for the prompt. Reaching the end of the
// input while waiting at the prompt ends the display too.
func (c *Console) Page(text string) {
	height, width := c.screenSize()
	perPage := height - 1

	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	used := 0

	for i, line := range lines {
		rows := screenRows(line, width)

		// A screen is full: wait before showing this line. (A line too
		// long for even a whole screen is shown anyway, on a screen of
		// its own.)
		if used > 0 && used+rows > perPage {
			if !c.continuePaging() {
				return
			}

			used = 0
		}

		c.Printf("%s", line)

		used += rows

		// The text's last line may lack its newline; the prompt still
		// belongs on a line of its own.
		if i == len(lines)-1 && !strings.HasSuffix(line, "\n") {
			c.Printf("\n")
		}
	}
}

// screenRows is how many screen rows line takes on a screen width columns
// wide (1 if width is unknown).
func screenRows(line string, width int) int {
	n := len(strings.TrimRight(line, "\r\n"))
	if width <= 0 || n <= width {
		return 1
	}

	return (n + width - 1) / width
}

// continuePaging shows the prompt and reads the user's reply: true to show
// another screen (RETURN alone), false to stop (any text, or no more
// input).
func (c *Console) continuePaging() bool {
	c.Printf("%s", pagePrompt)

	reply, ok := c.readReply()

	// The terminal echoed the RETURN; with no terminal (input from a
	// script or test) nothing did, so end the prompt's line here.
	if f, isFile := c.Out.(*os.File); !isFile || !term.IsTerminal(int(f.Fd())) {
		c.Printf("\n")
	}

	return ok && strings.TrimSpace(reply) == ""
}

// readReply reads one line from the console's input, a byte at a time so
// that nothing past the line is taken from a reader shared with the
// command line. ok is false at the end of the input with nothing read.
func (c *Console) readReply() (line string, ok bool) {
	if c.In == nil {
		return "", false
	}

	var (
		buf  [1]byte
		text []byte
	)

	for {
		n, err := c.In.Read(buf[:])
		if n == 1 {
			if buf[0] == '\n' || buf[0] == '\r' {
				return string(text), true
			}

			text = append(text, buf[0])
		}

		if err != nil {
			return string(text), len(text) > 0
		}
	}
}
