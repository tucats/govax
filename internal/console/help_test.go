package console

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// vaxHelpPath returns the one true copy of vax.help --
// internal/bootdata/files/vax.help, the file cmd/govax's own resolver
// actually reads at runtime (main.go's `resolver.ReadFile("vax.help")`).
// A second, historical copy used to live at testdata/dcl/vax.help (a
// git-archive import from the upstream C repo, like testdata/dcl/evax.dcl);
// it was deleted so there is only one file to keep MOUNT/DISMOUNT's help
// text (and everything else) up to date in, rather than two copies that
// could silently drift apart -- see docs/PHASE-22.md.
func vaxHelpPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "bootdata", "files", "vax.help")
}

func TestLoadHelpFile_realFixture(t *testing.T) {
	h, err := LoadHelpFile(vaxHelpPath(t))
	if err != nil {
		t.Fatalf("LoadHelpFile: %v", err)
	}

	c, buf := newTestConsole(t)
	buf.Reset()
	
	if err := c.Help(h, []string{"SHOW", "LOGICAL"}); err != nil {
		t.Fatalf("Help: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "logical") && !strings.Contains(got, "Logical") {
		t.Errorf("output = %q, want it to mention logical names", got)
	}
}

func TestHelpKey_matchesDocumentedExample(t *testing.T) {
	// The file's own preamble: "HELP SET TRACE" resolves to "$SET ,TRAC".
	got := helpKey([]string{"SET", "TRACE"})
	if got != "SET ,TRAC" {
		t.Errorf("helpKey = %q, want \"SET ,TRAC\"", got)
	}
}
