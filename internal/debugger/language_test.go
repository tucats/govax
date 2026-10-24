package debugger

import "testing"

// TestLanguageRadix: a FORTRAN program's debugger starts in decimal, as
// VMS's does (testdata/legacy72/vax/dun72.dlg), and a MACRO program's in
// hexadecimal.
func TestLanguageRadix(t *testing.T) {
	for language, want := range map[string]int{"FORTRAN": 10, "MACRO": 16, "UNKNOWN": 16} {
		if got := languageRadixOf(language); got != want {
			t.Errorf("languageRadixOf(%q) = %d, want %d", language, got, want)
		}
	}
}
