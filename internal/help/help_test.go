package help

import "testing"

func TestHelpKey_matchesDocumentedExample(t *testing.T) {
	// The file's own preamble: "HELP SET TRACE" resolves to "$SET ,TRAC".
	got := helpKey([]string{"SET", "TRACE"})
	if got != "SET ,TRAC" {
		t.Errorf("helpKey = %q, want \"SET ,TRAC\"", got)
	}
}
