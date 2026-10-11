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
// records, two bytes of fixed control (zeros) and carriage-return
// carriage control, which READ reads back without the fixed control; the
// file's name has its version, and its device is the physical name.
func TestOpenDCLFile(t *testing.T) {
	s, vol := newTypeTestSession(t)

	for _, want := range []string{"DUA0:[000000]NEW.DAT;1", "DUA0:[000000]NEW.DAT;2"} {
		f, err := s.OpenDCLFile(FileLocation{Name: "NEW"}, DCLAccess{Write: true})
		if err != nil {
			t.Fatalf("OpenDCLFile(NEW, /WRITE): %v", err)
		}

		if f.Name != want || f.Device != "_DUA0:" {
			t.Errorf("the file opened is %s on %s, want %s on _DUA0:", f.Name, f.Device, want)
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
	if attr.Format != ondisk.RecordFormatVFC || attr.VfcSize != 2 || attr.Attributes != ondisk.AttrImpliedCC {
		t.Errorf("NEW.DAT;1's attributes: %+v, want VFC with 2 bytes of control and CR carriage control", attr)
	}

	// Variable records keep any fixed control (readVolumeRecords drops it
	// for text only).
	raw, err := readVolumeRecords(file, VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) != 3 || string(raw[0]) != "\x00\x00one" {
		t.Errorf("NEW.DAT;1's records are %q, want each with two zero bytes first", raw)
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

// TestOpenDCLFile_host: a host file's records are its lines. /READ/WRITE
// updates a line in place (the same length only), and writes at the end
// only once Get has reached it; /APPEND writes at the end.
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

	if sts := appender.Update([]byte("x")); sts != rmsFACNotAllowed {
		t.Errorf("Update after /APPEND = %08X, want RMS$_FAC", sts)
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
