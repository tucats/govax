package lnm

import "testing"

// TestPermanentFileString: the string is ESC, 0, the IFI low byte first,
// then the device, and reads back; anything else isn't one.
func TestPermanentFileString(t *testing.T) {
	s := PermanentFileString(0x8101, "_SIMVAX$OPA0:")
	if want := "\x1b\x00\x01\x81_SIMVAX$OPA0:"; s != want {
		t.Fatalf("PermanentFileString = %q, want %q", s, want)
	}

	pf, ok := ParsePermanentFile(s)
	if !ok || pf.IFI != 0x8101 || pf.Device != "_SIMVAX$OPA0:" {
		t.Errorf("ParsePermanentFile = %+v, %v", pf, ok)
	}

	if got := DisplayValue(s); got != "_SIMVAX$OPA0:" {
		t.Errorf("DisplayValue = %q", got)
	}

	for _, v := range []string{"", "DUA1:", "\x1b\x00\x01", "\x1b\x01\x01\x81TTA0:"} {
		if _, ok := ParsePermanentFile(v); ok {
			t.Errorf("ParsePermanentFile(%q) is a process-permanent file", v)
		}

		if got := DisplayValue(v); got != v {
			t.Errorf("DisplayValue(%q) = %q", v, got)
		}
	}
}

// TestTranslateFileSpec_permanentFile: translation stops at a
// process-permanent file string, reached directly or through an alias,
// giving the device (with a colon) and the rest of the spec, the file,
// and the process-permanent name; a concealed name is shown by its name.
func TestTranslateFileSpec_permanentFile(t *testing.T) {
	db := NewDatabase(0)
	define := func(name string, mode Mode, attrs uint32, value string) {
		t.Helper()

		if _, err := db.Define(ProcessTableName, name, mode, 0, []Equivalence{{Value: value, Attrs: attrs}}); err != nil {
			t.Fatal(err)
		}
	}

	define("SYS$OUTPUT", Executive, AttrTerminal|AttrConcealed, PermanentFileString(0x8103, "_GOVAX$TTA0:"))
	define("PPF_OUT", Supervisor, AttrTerminal, PermanentFileString(0x8106, "_GOVAX$DUA1"))
	define("PPF_ALIAS", Supervisor, 0, "SYS$OUTPUT")

	tests := []struct {
		spec, want, name, display string
		ifi                       uint16
	}{
		{"SYS$OUTPUT", "_GOVAX$TTA0:", "SYS$OUTPUT", "SYS$OUTPUT", 0x8103},
		{"SYS$OUTPUT:X.DAT", "_GOVAX$TTA0:X.DAT", "SYS$OUTPUT", "SYS$OUTPUT:X.DAT", 0x8103},
		{"PPF_ALIAS:", "_GOVAX$TTA0:", "SYS$OUTPUT", "SYS$OUTPUT", 0x8103},
		{"PPF_OUT:", "_GOVAX$DUA1:", "PPF_OUT", "_GOVAX$DUA1:", 0x8106},
	}

	for _, tt := range tests {
		fs := mustTranslateFileSpec(t, db, tt.spec)
		if len(fs) != 1 {
			t.Fatalf("%s: %d results", tt.spec, len(fs))
		}

		f := fs[0]
		if f.Spec != tt.want || f.Permanent == nil || f.Permanent.IFI != tt.ifi ||
			f.PermanentName != tt.name || f.Display != tt.display {
			t.Errorf("%s: %+v (permanent %+v)", tt.spec, f, f.Permanent)
		}
	}

	if fs := mustTranslateFileSpec(t, db, "_SYS$OUTPUT:"); fs[0].Permanent != nil {
		t.Errorf("_SYS$OUTPUT: translated: %+v", fs[0])
	}
}
