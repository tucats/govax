package rms

import (
	"slices"
	"strings"
	"testing"
)

// TestSplitSpecList: commas and plus signs separate, but not inside
// quotes.
func TestSplitSpecList(t *testing.T) {
	for text, want := range map[string][]string{
		"A.LIS":                 {"A.LIS"},
		"A.LIS, B.LIS":          {"A.LIS", "B.LIS"},
		"A.LIS+B.LIS,C":         {"A.LIS", "B.LIS", "C"},
		`"/tmp/a,b+c",D.DAT`:    {`"/tmp/a,b+c"`, "D.DAT"},
		"[R3...],[R4...]":       {"[R3...]", "[R4...]"},
		"DUA2:[000000]*.EXE,*.": {"DUA2:[000000]*.EXE", "*."},
	} {
		if got := SplitSpecList(text); !slices.Equal(got, want) {
			t.Errorf("SplitSpecList(%q) = %q, want %q", text, got, want)
		}
	}
}

// TestTemporaryDefaults: the User's Manual's examples (4.3.3), and the
// cases it doesn't show.
func TestTemporaryDefaults(t *testing.T) {
	for _, tc := range []struct{ list, want string }{
		// The manual's three examples.
		{"[STATS]A.LIS,B.LIS", "[STATS]A.LIS,[STATS]B.LIS"},
		{"BASE:[STATS]A.LIS,[TIME]B.LIS,C.LIS", "BASE:[STATS]A.LIS,BASE:[TIME]B.LIS,BASE:[TIME]C.LIS"},
		{"[ALPHA]TEST.DAT,[]FINAL.DAT", "[ALPHA]TEST.DAT,[]FINAL.DAT"},
		// Phase 72's probe: DUA2:[000000] carries to *.DAT.
		{"DUA2:[000000]*.EXE,*.DAT", "DUA2:[000000]*.EXE,DUA2:[000000]*.DAT"},
		// No name or type is carried.
		{"A.LIS,.DAT", "A.LIS,.DAT"},
		// A new device without a directory carries none.
		{"DUA0:[X]A,DUA1:B,C", "DUA0:[X]A,DUA1:B,DUA1:C"},
		// A node carries its device and directory; a new one replaces them.
		{"NODE::DUA0:[X]A,B,OTHER::C", "NODE::DUA0:[X]A,NODE::DUA0:[X]B,OTHER::C"},
		// Angle brackets are a directory too.
		{"<X.Y>A,B", "<X.Y>A,<X.Y>B"},
		// A host file name carries nothing.
		{`DUA0:[X]A,"/tmp/b",C`, `DUA0:[X]A,"/tmp/b",C`},
	} {
		got := strings.Join(TemporaryDefaults(strings.Split(tc.list, ",")), ",")
		if got != tc.want {
			t.Errorf("TemporaryDefaults(%s) = %s, want %s", tc.list, got, tc.want)
		}
	}
}

// TestDirectoryList: a list's elements in one directory are listed under
// one heading with one total, as VMS's DIRECTORY ERRORS.*,ERREND.* was
// (testdata/mar/list/vax/list.log), each element's files in its own
// order; and the first element's device and directory carry to the next.
func TestDirectoryList(t *testing.T) {
	s, vol := newDirectoryTestSession(t)
	createTestFile(t, vol, "ERRORS.MAR", "a")
	createTestFile(t, vol, "ERRORS.LIS", "b")
	createTestFile(t, vol, "ERREND.MAR", "c")

	text, found, err := s.Directory("DUA0:[000000]ERRORS.*,ERREND.*", DirectoryOptions{})
	if err != nil || !found {
		t.Fatalf("Directory: %v, found %v", err, found)
	}

	want := "\nDirectory DUA0:[000000]\n\nERRORS.LIS;1        ERRORS.MAR;1        ERREND.MAR;1        \n\nTotal of 3 files.\n"
	if text != want {
		t.Errorf("DIRECTORY DUA0:[000000]ERRORS.*,ERREND.* =\n%q\nwant\n%q", text, want)
	}

	// Elements in a row in one directory share it; the same directory
	// again after another is listed again, with a grand total.
	s, vol0, vol1 := newTwoVolumeSession(t)
	createTestFile(t, vol0, "ERRORS.LIS", "a")
	createTestFile(t, vol0, "ERREND.MAR", "b")
	createTestFile(t, vol1, "OTHER.TXT", "c")

	text, _, err = s.Directory("DUA0:[000000]ERRORS.LIS,ERREND.MAR,DUA1:[000000]OTHER.TXT,DUA0:[000000]ERRORS.LIS", DirectoryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	want = "\nDirectory DUA0:[000000]\n\nERRORS.LIS;1        ERREND.MAR;1        \n\nTotal of 2 files.\n" +
		"\nDirectory DUA1:[000000]\n\nOTHER.TXT;1         \n\nTotal of 1 file.\n" +
		"\nDirectory DUA0:[000000]\n\nERRORS.LIS;1        \n\nTotal of 1 file.\n" +
		"\nGrand total of 3 directories, 4 files.\n"
	if text != want {
		t.Errorf("DIRECTORY over two volumes =\n%q\nwant\n%q", text, want)
	}
}
