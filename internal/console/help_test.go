package console

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func vaxHelpPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "dcl", "vax.help")
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
