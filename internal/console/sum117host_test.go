package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSum117HostFiles is the manual's worked example on host files (Phase
// 117, subtask 9): DIFFERENCES/SLP writes an update file on the host (it
// used to make the file and then fail to write the listing into it, looking
// for it on a volume), EDIT/SUM applies it to make a host file, and the
// result is the revision. EDIT/SUM's /LISTING=SYS$OUTPUT goes to SYS$OUTPUT,
// not to a host file named SYS$OUTPUT.
func TestSum117HostFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	master := "Dear committee,\nThe meeting is on Tuesday.\nPlease bring the minutes.\n" +
		"We will discuss the budget.\nRegards,\nPat\n"
	revision := "Dear committee,\nThe meeting is on Thursday, at ten.\nPlease bring the minutes.\n" +
		"We will discuss the budget.\nAnd the new building.\nRegards,\nPat\n"

	for name, text := range map[string]string{"memo.txt": master, "memo2.txt": revision} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	d, _, buf := shareFindConsole(t, false)
	host := func(name string) string { return `"` + filepath.Join(dir, name) + `"` }

	buf.Reset()
	shareFindRun(t, d, "DIFFERENCES/SLP/OUTPUT="+host("memo.upd")+" "+host("memo.txt")+" "+host("memo2.txt"))

	if buf.Len() != 0 {
		t.Errorf("DIFFERENCES/SLP said:\n%s", buf.String())
	}

	script, err := os.ReadFile(filepath.Join(dir, "memo.upd"))
	if err != nil {
		t.Fatal(err)
	}

	const want = "-    2,    4\nThe meeting is on Thursday, at ten.\nPlease bring the minutes.\n" +
		"We will discuss the budget.\nAnd the new building.\n/\n"
	if string(script) != want {
		t.Errorf("the update file is\n%s\nwant\n%s", script, want)
	}

	buf.Reset()
	shareFindRun(t, d, "EDIT/SUM/OUTPUT="+host("memo3.txt")+"/LISTING=SYS$OUTPUT "+host("memo.txt")+"/UPDATE="+host("memo.upd"))

	if out, err := os.ReadFile(filepath.Join(dir, "memo3.txt")); err != nil || string(out) != revision {
		t.Errorf("EDIT/SUM made %q (%v), want the revision", out, err)
	}

	if _, err := os.Stat("SYS$OUTPUT"); err == nil {
		t.Errorf("/LISTING=SYS$OUTPUT made a host file")
	}

	if want := "-3                    5 Regards,\n"; !strings.Contains(buf.String(), want) {
		t.Errorf("the listing at SYS$OUTPUT is\n%s\nwant a line %q", buf.String(), want)
	}
}
