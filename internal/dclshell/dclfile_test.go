package dclshell

import (
	"strings"
	"testing"
)

// TestSplitDCLList: WRITE's items are split at commas outside quotes and
// parentheses.
func TestSplitDCLList(t *testing.T) {
	got := splitDCLList(`"a,b", F$EXTRACT(0, 1, X) , 3`)
	want := []string{`"a,b"`, `F$EXTRACT(0, 1, X)`, `3`}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("splitDCLList = %q, want %q", got, want)
	}
}
