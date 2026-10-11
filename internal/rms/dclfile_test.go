package rms

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
)

// TestOpenDCLFile: OPEN/WRITE makes the next version of a file of VFC
// records, two bytes of fixed control (01 8D) and print-file carriage
// control (VMS 7.3's DIRECTORY/FULL, testdata/dcl50's F1), which READ
// reads back without the fixed control; the file's name has its version.
func TestOpenDCLFile(t *testing.T) {
	s, vol := newTypeTestSession(t)

	for _, want := range []string{"DUA0:[000000]NEW.DAT;1", "DUA0:[000000]NEW.DAT;2"} {
		f, err := s.OpenDCLFile(FileLocation{Name: "NEW"}, DCLAccess{Write: true})
		if err != nil {
			t.Fatalf("OpenDCLFile(NEW, /WRITE): %v", err)
		}

		if f.Name != want || f.Device != "DUA0" {
			t.Errorf("the file opened is %s on %s, want %s on DUA0", f.Name, f.Device, want)
		}

		for _, record := range []string{"one", "", "three"} {
			if sts := f.Put([]byte(record)); sts != rmsNormal {
				t.Fatalf("Put(%q) = %08X", record, sts)
			}
		}

		if err := f.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	matches, err := filespec.Glob(vol, filespec.Spec{Name: "NEW", Type: "DAT", Version: "1"})
	if err != nil || len(matches) != 1 {
		t.Fatalf("Glob(NEW.DAT;1) = %v, %v", matches, err)
	}

	file, err := vol.OpenFID(matches[0].Fid)
	if err != nil {
		t.Fatal(err)
	}

	attr := file.Header.RecordAttributes
	if attr.Format != ondisk.RecordFormatVFC || attr.VfcSize != 2 || attr.Attributes != ondisk.AttrPrintCC {
		t.Errorf("NEW.DAT;1's attributes: %+v, want VFC with 2 bytes of control and print-file carriage control", attr)
	}

	// Variable records keep any fixed control (readVolumeRecords drops it
	// for text only).
	raw, err := readVolumeRecords(file, VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) != 3 || string(raw[0]) != "\x01\x8Done" {
		t.Errorf("NEW.DAT;1's records are %q, want each with 01 8D first", raw)
	}

	// TYPE shows the records as lines.
	if text, err := s.Type("NEW.DAT;1"); err != nil || text != "one\n\nthree\n" {
		t.Errorf("Type(NEW.DAT;1) = %q, %v", text, err)
	}

	f, err := s.OpenDCLFile(FileLocation{Name: "NEW.DAT;1"}, DCLAccess{})
	if err != nil {
		t.Fatalf("OpenDCLFile(NEW.DAT;1): %v", err)
	}

	defer f.Close()

	for _, want := range []string{"one", "", "three"} {
		record, sts := f.Get()
		if sts != rmsNormal || string(record) != want {
			t.Errorf("Get = %q, %08X, want %q", record, sts, want)
		}
	}

	if _, sts := f.Get(); sts != rmsEOF {
		t.Errorf("Get at the end = %08X, want RMS$_EOF", sts)
	}
}

// TestOpenDCLFile_failures: the RMS status of each OPEN that fails, with
// the name as expanded.
func TestOpenDCLFile_failures(t *testing.T) {
	s, _ := newTypeTestSession(t)

	for _, tc := range []struct {
		name   string
		access DCLAccess
		want   uint32
		as     string
	}{
		{"NOSUCH", DCLAccess{}, rmsFileNotFound, "DUA0:[000000]NOSUCH.DAT;"},
		{"NOSUCH", DCLAccess{Append: true}, rmsFileNotFound, "DUA0:[000000]NOSUCH.DAT;"},
		{"NOSUCH", DCLAccess{Read: true, Write: true}, rmsFileNotFound, "DUA0:[000000]NOSUCH.DAT;"},
		{"[NODIR]X", DCLAccess{Write: true}, rmsDirNotFound, "DUA0:[NODIR]X.DAT;"},
		{"*.TXT", DCLAccess{}, rmsWildcardError, "DUA0:[000000]*.TXT;"},
	} {
		_, err := s.OpenDCLFile(FileLocation{Name: tc.name}, tc.access)

		var fe *DCLFileError
		if !errors.As(err, &fe) || fe.Status != tc.want || fe.Name != tc.as {
			t.Errorf("OpenDCLFile(%s, %+v) = %v, want status %08X for %s", tc.name, tc.access, err, tc.want, tc.as)
		}
	}
}

// TestOpenDCLFile_sharing: without /SHARE a writer keeps every other
// opener out, a /SHARE reader too, where two /SHARE=READ readers share a
// file and keep a writer out (VMS 7.3, testdata/dcl50's F9).
func TestOpenDCLFile_sharing(t *testing.T) {
	s, _ := newTypeTestSession(t)
	foo := FileLocation{Name: "FOO.TXT"}

	writer, err := s.OpenDCLFile(foo, DCLAccess{Append: true})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.OpenDCLFile(foo, DCLAccess{ShareWrite: true}); !isStatus(err, rmsFileLocked) {
		t.Errorf("a /SHARE reader of a file being written: %v, want RMS$_FLK", err)
	}

	_ = writer.Close()

	for range 2 {
		if _, err := s.OpenDCLFile(foo, DCLAccess{ShareRead: true}); err != nil {
			t.Fatalf("a /SHARE=READ reader: %v", err)
		}
	}

	if _, err := s.OpenDCLFile(foo, DCLAccess{Append: true}); !isStatus(err, rmsFileLocked) {
		t.Errorf("an /APPEND of a file two readers have: %v, want RMS$_FLK", err)
	}
}

// TestOpenDCLFile_host: a host file's records are its lines. /READ/WRITE
// updates a line in place (the same length only), and writes at the end
// only once Get has reached it; /APPEND starts at the end, where Get is
// at the end of the file and there is no record to update.
func TestOpenDCLFile_host(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.dat")
	if err := os.WriteFile(path, []byte("alpha\r\nbeta\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := openHostDCLFile(path, DCLAccess{Read: true, Write: true})
	if err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		what string
		sts  uint32
	}{
		{"get", rmsNormal},
		{"update ALPHA", rmsNormal},
		{"put", rmsNotAtEOF},
		{"get", rmsNormal},
		{"update longer", rmsRecordTooBig},
		{"update", rmsNoCurrent},
		{"get", rmsEOF},
		{"put", rmsNormal},
	}

	for _, step := range steps {
		var sts uint32

		switch step.what {
		case "get":
			_, sts = f.Get()
		case "update ALPHA":
			sts = f.Update([]byte("ALPHA"))
		case "update longer":
			sts = f.Update([]byte("longer"))
		case "update":
			sts = f.Update([]byte("x"))
		case "put":
			sts = f.Put([]byte("gamma"))
		}

		if sts != step.sts {
			t.Errorf("%s = %08X, want %08X", step.what, sts, step.sts)
		}
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	appender, err := openHostDCLFile(path, DCLAccess{Append: true})
	if err != nil {
		t.Fatal(err)
	}

	if sts := appender.Put([]byte("delta")); sts != rmsNormal {
		t.Errorf("Put after /APPEND = %08X", sts)
	}

	if _, sts := appender.Get(); sts != rmsEOF {
		t.Errorf("Get after /APPEND = %08X, want RMS$_EOF", sts)
	}

	if sts := appender.Update([]byte("x")); sts != rmsNoCurrent {
		t.Errorf("Update after /APPEND = %08X, want RMS$_CUR", sts)
	}

	_ = appender.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := "ALPHA\r\nbeta\ngamma" + hostLineEnd + "delta" + hostLineEnd
	if string(data) != want {
		t.Errorf("the file holds %q, want %q", data, want)
	}

	if _, err := openHostDCLFile(filepath.Join(t.TempDir(), "none.dat"), DCLAccess{}); !isStatus(err, rmsFileNotFound) {
		t.Errorf("opening a missing host file: %v, want RMS$_FNF", err)
	}

	if _, err := openHostDCLFile(filepath.Join(t.TempDir(), "nodir", "x.dat"), DCLAccess{Write: true}); !isStatus(err, rmsDirNotFound) {
		t.Errorf("creating a host file in a missing directory: %v, want RMS$_DNF", err)
	}
}

// isStatus reports whether err is a DCLFileError with RMS status sts.
func isStatus(err error, sts uint32) bool {
	var fe *DCLFileError

	return errors.As(err, &fe) && fe.Status == sts
}
