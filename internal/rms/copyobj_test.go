package rms

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/ods2/ondisk"
)

// These tests check docs/PHASE-27.md subtask 9's requirement that an
// object module survives COPY in both directions: a .OBJ keeps its record
// boundaries on the host (in the ODS-2 variable-length layout), and comes
// back into a volume as a real variable-length record file.

// realObject reads one of the real VAX MACRO objects in testdata/mar/vax.
func realObject(t *testing.T, name string) (path string, records [][]byte) {
	t.Helper()

	path = filepath.Join(disksDir(t), "..", "mar", "vax", name)

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	records, err = obj.ReadRecords(f)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	return path, records
}

func TestCopyObject_hostToVolume(t *testing.T) {
	for _, binary := range []bool{false, true} {
		s, vol := newCopyTestSession(t)
		hostPath, want := realObject(t, "hello.obj")

		if _, err := s.Copy(hostPath, true, "HELLO.OBJ", false, CopyOptions{Binary: binary}); err != nil {
			t.Fatalf("/BINARY=%v: Copy: %v", binary, err)
		}

		attr := fileAttributes(t, vol, "HELLO.OBJ", 1)
		if attr.Format != ondisk.RecordFormatVariable || attr.DefaultExtend != 20 {
			t.Errorf("/BINARY=%v: attributes = %+v, want an object module's", binary, attr)
		}

		got, _, err := s.ReadRecordFile(FileLocation{Name: "HELLO.OBJ"}, VariableRecords)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("/BINARY=%v: records on the volume = %d records, %v; want the host file's %d", binary, len(got), err, len(want))
		}

		if _, err := obj.Decode(got); err != nil {
			t.Errorf("/BINARY=%v: Decode: %v", binary, err)
		}
	}
}

func TestCopyObject_hostNotAnObject(t *testing.T) {
	s, _ := newCopyTestSession(t)
	path := filepath.Join(t.TempDir(), "bad.obj")

	if err := os.WriteFile(path, []byte("not an object\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Copy(path, true, "BAD.OBJ", false, CopyOptions{}); err == nil {
		t.Error("copying a host .obj that isn't the record layout succeeded")
	}
}

func TestCopyObject_volumeToHost(t *testing.T) {
	s, _ := newCopyTestSession(t)
	_, want := realObject(t, "hello.obj")

	if _, err := s.CreateRecordFile(FileLocation{Name: "HELLO.OBJ"}, VariableRecords, want); err != nil {
		t.Fatal(err)
	}

	var layout bytes.Buffer
	if err := obj.WriteRecords(&layout, want); err != nil {
		t.Fatal(err)
	}

	for _, binary := range []bool{false, true} {
		out := filepath.Join(t.TempDir(), "hello.obj")

		if _, err := s.Copy("HELLO.OBJ", false, out, true, CopyOptions{Binary: binary}); err != nil {
			t.Fatalf("/BINARY=%v: Copy: %v", binary, err)
		}

		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, layout.Bytes()) {
			t.Errorf("/BINARY=%v: host file isn't the object's variable-length layout", binary)
		}
	}
}

func TestCopyObject_volumeToVolume(t *testing.T) {
	s, vol := newCopyTestSession(t)
	_, want := realObject(t, "extern.obj")

	if _, err := s.CreateRecordFile(FileLocation{Name: "EXTERN.OBJ"}, VariableRecords, want); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Copy("EXTERN.OBJ", false, "COPY.OBJ", false, CopyOptions{}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	if attr := fileAttributes(t, vol, "COPY.OBJ", 1); attr.Format != ondisk.RecordFormatVariable || attr.DefaultExtend != 20 {
		t.Errorf("copy's attributes = %+v, want the source's", attr)
	}

	got, _, err := s.ReadRecordFile(FileLocation{Name: "COPY.OBJ"}, VariableRecords)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("copy's records = %d, %v; want %d", len(got), err, len(want))
	}
}

// TestCopyBinary_keepsRecordAttributes checks that COPY/BINARY between
// volumes keeps the source's record format rather than making every copy
// an Undefined-format file.
func TestCopyBinary_keepsRecordAttributes(t *testing.T) {
	s, vol := newCopyTestSession(t)
	lines := [][]byte{[]byte("a text line"), []byte("another")}

	if _, err := s.CreateRecordFile(FileLocation{Name: "T.LIS"}, TextRecords, lines); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Copy("T.LIS", false, "U.LIS", false, CopyOptions{Binary: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	src, dst := fileAttributes(t, vol, "T.LIS", 1), fileAttributes(t, vol, "U.LIS", 1)
	if dst.Format != src.Format || dst.Attributes != src.Attributes || dst.RecordSize != src.RecordSize {
		t.Errorf("copy's attributes = %+v, want the source's %+v", dst, src)
	}

	got, _, err := s.ReadRecordFile(FileLocation{Name: "U.LIS"}, TextRecords)
	if err != nil || !reflect.DeepEqual(got, lines) {
		t.Errorf("copy's records = %q, %v", got, err)
	}
}

// TestCopyObject_realVAXVolume copies the objects real VAX MACRO wrote on
// the exchange volume out to the host with a plain COPY, reads them in
// place with ReadRecordFile, and compares both with the fixtures
// COPY/BINARY brought out in subtask 3. Skipped when the (gitignored)
// volume isn't present.
func TestCopyObject_realVAXVolume(t *testing.T) {
	path := skipUnlessDiskPresent(t, "mar-exchange-vms.dsk")

	mounts := NewMountTable()
	if err := mounts.Mount("DUA1", path, false); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	s := NewSession(mounts)

	for _, name := range []string{"empty", "data", "entry", "reloc", "extern", "globals", "branch", "exprs", "hello"} {
		_, want := realObject(t, name+".obj")
		spec := "DUA1:[000000]" + name + ".OBJ"

		got, found, err := s.ReadRecordFile(FileLocation{Name: spec}, VariableRecords)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: ReadRecordFile = %d records, %v; want %d", name, len(got), err, len(want))
		}

		if found.Name != "DUA1:[000000]"+upper(name)+".OBJ;1" {
			t.Errorf("%s: found %q", name, found.Name)
		}

		out := filepath.Join(t.TempDir(), name+".obj")
		if _, err := s.Copy(spec, false, out, true, CopyOptions{}); err != nil {
			t.Fatalf("%s: Copy: %v", name, err)
		}

		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}

		copied, err := obj.ReadRecords(f)
		f.Close()

		if err != nil || !reflect.DeepEqual(copied, want) {
			t.Errorf("%s: copied to the host = %d records, %v; want %d", name, len(copied), err, len(want))
		}
	}
}

func upper(s string) string {
	return string(bytes.ToUpper([]byte(s)))
}
