package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// TestFortranPause runs a program with PAUSE statements (Phase 79,
// subtask 4): FTN$PAUSE writes the display, or "FORTRAN PAUSE", and
// LIB$PAUSE reads DCL's commands with the program suspended. CONTINUE
// goes on at the next statement; EXIT and STOP end the program there, its
// status SS$_NORMAL; so does the end of the terminal's input. A command
// typed at the pause runs with the program suspended.
func TestFortranPause(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)

		lines := []string{
			"      PROGRAM PAUSES",
			"      PRINT *, 'BEFORE'",
			"      PAUSE",
			"      PRINT *, 'BETWEEN'",
			"      PAUSE 'CHECK THE TAPE'",
			"      PRINT *, 'AFTER'",
			"      END",
		}
		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]PAUSES.FOR"},
			rms.TextRecords, splitSource(strings.Join(lines, "\n")+"\n")); err != nil {
			t.Fatal(err)
		}

		deleteFilesAfter(t, d, "PAUSES")

		for _, command := range []string{"FORTRAN PAUSES", "MACRO PAUSES", "LINK PAUSES"} {
			shareFindRun(t, d, command)

			if out := buf.String(); out != "" {
				t.Fatalf("%s said:\n%s", command, out)
			}
		}

		prompt := c.Prompt()

		// Each run starts from the assignment's status, %X00030001. A
		// program that was continued at command level 0 leaves $STATUS
		// as it was (LIB$PAUSE's rule, internal/dclshell/dclpause.go); one
		// ended at a pause leaves SS$_NORMAL.
		for _, tc := range []struct{ name, typed, want, status string }{
			{"CONTINUE", "CONTINUE\nCONTINUE\n",
				"BEFORE\nFORTRAN PAUSE\n" + prompt + "BETWEEN\nCHECK THE TAPE\n" + prompt + "AFTER\n", "00030001"},
			{"a command, then CONTINUE", "WRITE SYS$OUTPUT \"PAUSED\"\nCONTINUE\nCONTINUE\n",
				"BEFORE\nFORTRAN PAUSE\n" + prompt + "PAUSED\n" + prompt + "BETWEEN\nCHECK THE TAPE\n" + prompt + "AFTER\n", "00010001"},
			{"EXIT at the second", "CONTINUE\nEXIT\n",
				"BEFORE\nFORTRAN PAUSE\n" + prompt + "BETWEEN\nCHECK THE TAPE\n" + prompt, "00000001"},
			{"STOP at the first", "STOP\n",
				"BEFORE\nFORTRAN PAUSE\n" + prompt, "00000001"},
			{"end of input", "",
				"BEFORE\nFORTRAN PAUSE\n" + prompt, "00000001"},
		} {
			shareFindRun(t, d, "X = 1")
			c.In = strings.NewReader(tc.typed)

			want := tc.want + "  $STATUS == \"%X" + tc.status + "\"\n"
			if got := shareRun(t, d, buf, "PAUSES"); got != want {
				t.Errorf("%s: RUN PAUSES printed:\n%q\nwant:\n%q", tc.name, got, want)
			}
		}
	})
}
