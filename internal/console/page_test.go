package console

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
)

// numberedLines is n lines of text, "line 1" through "line n".
func numberedLines(n int) string {
	var b strings.Builder

	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}

	return b.String()
}

// pagedConsole is a test console with a screen height lines tall and
// width columns wide, whose input is replies.
func pagedConsole(t *testing.T, height, width int, replies string) (*Console, *bytes.Buffer) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.ScreenSize = func() (int, int) { return height, width }
	c.In = strings.NewReader(replies)

	return c, buf
}

// TestPage_returnShowsEveryScreen: RETURN at each prompt shows the whole
// text, each screen holding height-1 lines and the prompt.
func TestPage_returnShowsEveryScreen(t *testing.T) {
	c, buf := pagedConsole(t, 5, 0, "\n\n\n")
	c.Page(numberedLines(10))

	want := "line 1\nline 2\nline 3\nline 4\n" + pagePrompt + "\n" +
		"line 5\nline 6\nline 7\nline 8\n" + pagePrompt + "\n" +
		"line 9\nline 10\n"
	if buf.String() != want {
		t.Errorf("output =\n%q\nwant\n%q", buf.String(), want)
	}
}

// TestPage_textEndsDisplay: anything typed before RETURN stops the display.
func TestPage_textEndsDisplay(t *testing.T) {
	c, buf := pagedConsole(t, 5, 0, "q\n\n")
	c.Page(numberedLines(10))

	out := buf.String()
	if !strings.Contains(out, "line 4\n") || strings.Contains(out, "line 5") {
		t.Errorf("output = %q, want it to stop after the first screen", out)
	}
}

// TestPage_endOfInputEndsDisplay: no input at the prompt ends the display.
func TestPage_endOfInputEndsDisplay(t *testing.T) {
	c, buf := pagedConsole(t, 5, 0, "")
	c.Page(numberedLines(10))

	if strings.Contains(buf.String(), "line 5") {
		t.Errorf("output = %q, want it to stop at the first prompt", buf.String())
	}
}

// TestPage_fitsOneScreen: text that fits on a screen gets no prompt.
func TestPage_fitsOneScreen(t *testing.T) {
	c, buf := pagedConsole(t, 24, 0, "")
	c.Page("one\ntwo")

	if buf.String() != "one\ntwo\n" {
		t.Errorf("output = %q, want %q", buf.String(), "one\ntwo\n")
	}
}

// TestPage_longLinesWrap: with the width known, a line longer than the
// screen is wide counts as the rows it wraps onto.
func TestPage_longLinesWrap(t *testing.T) {
	c, buf := pagedConsole(t, 5, 10, "\n")
	c.Page("short\n" + strings.Repeat("x", 25) + "\nafter\n")

	// "short" (1 row) + the long line (3 rows) fill the 4 rows; "after"
	// starts the next screen.
	want := "short\n" + strings.Repeat("x", 25) + "\n" + pagePrompt + "\nafter\n"
	if buf.String() != want {
		t.Errorf("output =\n%q\nwant\n%q", buf.String(), want)
	}
}

// TestScreenSize_setting: with no terminal, vax.console.lines gives the
// height, and 24 with it unset.
func TestScreenSize_setting(t *testing.T) {
	old, had := settings.Get(consoleLinesSetting), settings.Exists(consoleLinesSetting)

	t.Cleanup(func() {
		if had {
			settings.Set(consoleLinesSetting, old)
		} else {
			_ = settings.Delete(consoleLinesSetting)
		}
	})

	c, _ := newTestConsole(t)

	_ = settings.Delete(consoleLinesSetting)

	if h, _ := c.screenSize(); h != defaultScreenLines {
		t.Errorf("unset: height = %d, want %d", h, defaultScreenLines)
	}

	settings.Set(consoleLinesSetting, "40")

	if h, _ := c.screenSize(); h != 40 {
		t.Errorf("vax.console.lines=40: height = %d, want 40", h)
	}
}

// TestDispatch_typePage: TYPE/PAGE through the grammar pages the file.
func TestDispatch_typePage(t *testing.T) {
	d, c := newTestDispatcher(t)

	var buf bytes.Buffer

	c.Out = &buf
	c.ScreenSize = func() (int, int) { return 5, 0 }
	c.In = strings.NewReader("stop\n")

	mountFreshContainer(t, c, "DUA0")

	if err := c.SetDefault("DUA0:"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("Lookup(DUA0) after mount = not found")
	}

	createConsoleTestFileWithContent(t, vol, "FOO.TXT", numberedLines(10))

	if err := d.Dispatch("TYPE/PAGE FOO.TXT"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, pagePrompt) || strings.Contains(out, "line 5") {
		t.Errorf("output = %q, want one screen and the prompt", out)
	}
}
