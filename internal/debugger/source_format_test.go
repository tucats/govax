package debugger

import "testing"

// TestFormatSource: the layout of a source line, from the VMS debugger's
// own output (testdata/dbg/vax/dbgdis.dlg): tabs expand to every eighth
// column of the text, and a line that overflows 80 columns continues on a
// line numbered "-".
func TestFormatSource(t *testing.T) {
	for _, tc := range []struct {
		n    int
		text string
		want string
	}{
		{43, "\tMOVL\t#BIG, R3", "    43:         MOVL    #BIG, R3\n"},
		{
			110, "JSBRTN:\tINCL\tCOUNT\t\t\t; a JSB subroutine, not a routine",
			"   110: JSBRTN: INCL    COUNT                   ; a JSB subroutine, not a routin\n     -: e\n",
		},
		{7, "", "     7: \n"},
	} {
		if got := formatSource(tc.n, tc.text); got != tc.want {
			t.Errorf("formatSource(%d, %q):\n got %q\nwant %q", tc.n, tc.text, got, tc.want)
		}
	}
}
