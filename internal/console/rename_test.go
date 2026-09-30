package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// newRenameTestDispatcher is a dispatcher on the real console grammar,
// its console's output, and DUA0: mounted with FIRST.TXT and SECOND.TXT
// in its master file directory, which is the default directory.
func newRenameTestDispatcher(t *testing.T) (*Dispatcher, *bytes.Buffer) {
	t.Helper()

	c, out := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	mountFreshContainer(t, c, "DUA0")

	vol, _ := c.Mounts.Lookup("DUA0")
	createConsoleTestFile(t, vol, "FIRST.TXT")
	createConsoleTestFile(t, vol, "SECOND.TXT")

	if err := d.Dispatch("SET DEFAULT DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	return d, out
}

// renameLines runs command and returns what it printed, a line at a
// time, and its error.
func renameLines(t *testing.T, d *Dispatcher, out *bytes.Buffer, command string) ([]string, error) {
	t.Helper()

	out.Reset()
	err := d.Dispatch(command)

	return strings.Split(strings.TrimRight(out.String(), "\n"), "\n"), err
}

// wantLines fails unless got is want, line for line.
func wantLines(t *testing.T, what string, got []string, want ...string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s printed:\n%s\nwant:\n%s", what, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestRename_logAndList: /LOG reports each file of a list, with its new
// name and version.
func TestRename_logAndList(t *testing.T) {
	d, out := newRenameTestDispatcher(t)

	lines, err := renameLines(t, d, out, "RENAME/LOG FIRST.TXT,SECOND.TXT *.OLD")
	if err != nil {
		t.Fatalf("RENAME: %v", err)
	}

	wantLines(t, "RENAME/LOG", lines,
		"%RENAME-I-RENAMED, DUA0:[000000]FIRST.TXT;1 renamed to DUA0:[000000]FIRST.OLD;1",
		"%RENAME-I-RENAMED, DUA0:[000000]SECOND.TXT;1 renamed to DUA0:[000000]SECOND.OLD;1")

	// Without /LOG, nothing is shown.
	lines, err = renameLines(t, d, out, "REN FIRST.OLD FIRST.TXT")
	if err != nil || len(lines) != 1 || lines[0] != "" {
		t.Errorf("RENAME without /LOG: %v, printed %q", err, lines)
	}
}

// TestRename_newVersion: by default a file with no input version gets
// the next version of its new name; /NONEW_VERSION keeps its own.
func TestRename_newVersion(t *testing.T) {
	d, out := newRenameTestDispatcher(t)

	lines, _ := renameLines(t, d, out, "RENAME/LOG SECOND.TXT FIRST.TXT")
	wantLines(t, "RENAME", lines,
		"%RENAME-I-RENAMED, DUA0:[000000]SECOND.TXT;1 renamed to DUA0:[000000]FIRST.TXT;2")

	lines, _ = renameLines(t, d, out, "RENAME/LOG/NONEW_VERSION FIRST.TXT THIRD.TXT")
	wantLines(t, "RENAME/NONEW_VERSION", lines,
		"%RENAME-I-RENAMED, DUA0:[000000]FIRST.TXT;2 renamed to DUA0:[000000]THIRD.TXT;2")
}

// TestRename_failureMessages: each kind of failure shows RENAME.B32's
// message, and the command fails without the console showing it again.
func TestRename_failureMessages(t *testing.T) {
	d, out := newRenameTestDispatcher(t)
	mountFreshContainer(t, d.Console, "DUA1")

	for _, tc := range []struct {
		command string
		want    []string
	}{
		{"RENAME NOPE.TXT X.TXT", []string{
			"%RENAME-E-SEARCHFAIL, error searching for DUA0:[000000]NOPE.TXT;",
			"-RMS-E-FNF, file not found",
		}},
		{"RENAME FIRST.TXT X*.TXT", []string{
			"%RENAME-E-OPENOUT, error opening DUA0:[000000]X*.TXT; as output",
			"-RMS-F-WLD, invalid wildcard operation",
		}},
		{"RENAME FIRST.TXT DUA1:[000000]", []string{
			"%RENAME-E-NOTRENAMED, DUA0:[000000]FIRST.TXT;1 not renamed",
			"-RENAME-E-NOTSAMEDEV, Cannot RENAME to a different device",
		}},
		{"RENAME FIRST.TXT SECOND.TXT;1", []string{
			"%RENAME-E-OPENOUT, error opening DUA0:[000000]SECOND.TXT;1 as output",
			"-RMS-E-ENT, ACP enter function failed",
			"-SYSTEM-W-DUPFILENAME, duplicate file name",
		}},
		{"RENAME FIRST.TXT [NOSUCH]", []string{
			"%RENAME-E-OPENIN, error opening DUA0:[000000]FIRST.TXT;1 as input",
			"-RMS-E-DNF, directory not found",
		}},
	} {
		lines, err := renameLines(t, d, out, tc.command)
		wantLines(t, tc.command, lines, tc.want...)

		if err == nil || !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: error %v, want a failure with its message inhibited", tc.command, err)
		}
	}
}

// TestRename_continuesAfterFailure: a bad item in a list doesn't stop the
// others.
func TestRename_continuesAfterFailure(t *testing.T) {
	d, out := newRenameTestDispatcher(t)

	lines, err := renameLines(t, d, out, "RENAME/LOG NOPE.TXT,FIRST.TXT *.DAT")
	wantLines(t, "RENAME/LOG", lines,
		"%RENAME-E-SEARCHFAIL, error searching for DUA0:[000000]NOPE.TXT;",
		"-RMS-E-FNF, file not found",
		"%RENAME-I-RENAMED, DUA0:[000000]FIRST.TXT;1 renamed to DUA0:[000000]FIRST.DAT;1")

	if err == nil {
		t.Error("a RENAME with one failed file succeeded")
	}
}
