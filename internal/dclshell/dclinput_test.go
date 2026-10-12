package dclshell

import (
	"testing"
)

// skpdatLine and ivvaluLine are CLI$_SKPDAT's and CLI$_IVVALU's
// messages, as a procedure shows them.
const ()

// TestParseDeck: DECK's record, with and without /DOLLARS.
func TestParseDeck(t *testing.T) {
	for _, tc := range []struct {
		command string
		marker  string
		ok      bool
	}{
		{" DECK", "", true},
		{"deck ! comment", "", true},
		{"DECK/DOLLARS", "", true},
		{"DECK/D=xx", "XX", true},
		{`DECK /DOLLARS="%x"`, "%x", true},
		{"DECK/NOPE", "", false},
		{"DECK foo", "", false},
		{"DECKS", "", false},
		{"EOD", "", false},
	} {
		marker, ok := parseDeck(tc.command)
		if marker != tc.marker || ok != tc.ok {
			t.Errorf("parseDeck(%q) = %q, %v; want %q, %v", tc.command, marker, ok, tc.marker, tc.ok)
		}
	}

	for record, want := range map[string]bool{"$ EOD": true, "$eod ! end": true, "$ EODX": false, "EOD": false} {
		if got := (&procedureDeck{}).ends(record); got != want {
			t.Errorf("ends(%q) = %v, want %v", record, got, want)
		}
	}
}
