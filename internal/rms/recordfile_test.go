package rms

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// sampleRecords has an empty record, odd and even lengths, and one long
// enough to span a block boundary.
func sampleRecords() [][]byte {
	long := bytes.Repeat([]byte{0xA5}, 700)

	return [][]byte{{1, 2, 3}, {}, {4, 5}, long, []byte("end")}
}

// fileAttributes reads the record attributes of name in vol's MFD.
func fileAttributes(t *testing.T, vol *volume.Volume, name string, version uint16) ondisk.RecAttr {
	t.Helper()

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory: %v", err)
	}

	entry, err := dir.Lookup(name, version)
	if err != nil {
		t.Fatalf("Lookup(%s;%d): %v", name, version, err)
	}

	f, err := vol.OpenFID(entry.Fid)
	if err != nil {
		t.Fatalf("OpenFID: %v", err)
	}

	return f.Header.RecordAttributes
}

func TestRecordFile_volumeVariable(t *testing.T) {
	s, vol := newCopyTestSession(t)
	records := sampleRecords()

	loc, err := s.CreateRecordFile(FileLocation{Name: "T.OBJ"}, VariableRecords, records)
	if err != nil {
		t.Fatalf("CreateRecordFile: %v", err)
	}

	if loc.Host || loc.Name != "DUA0:[000000]T.OBJ;1" {
		t.Errorf("CreateRecordFile location = %+v, want DUA0:[000000]T.OBJ;1", loc)
	}

	// The attributes real VAX MACRO gives an object module.
	attr := fileAttributes(t, vol, "T.OBJ", 1)
	if attr.Format != ondisk.RecordFormatVariable || attr.Attributes != 0 || attr.MaxRecordSize != 0 ||
		attr.DefaultExtend != 20 || attr.RecordSize != 700 {
		t.Errorf("attributes = %+v, want VAR, RAT 0, MRS 0, DEQ 20, longest record 700", attr)
	}

	got, found, err := s.ReadRecordFile(FileLocation{Name: "T.OBJ"}, VariableRecords)
	if err != nil {
		t.Fatalf("ReadRecordFile: %v", err)
	}

	if !reflect.DeepEqual(normalize(got), normalize(records)) {
		t.Errorf("records read back = %v, want %v", got, records)
	}

	if found.Name != "DUA0:[000000]T.OBJ;1" {
		t.Errorf("ReadRecordFile location = %+v", found)
	}

	// Another create is the next version, and the highest is read.
	if loc, err = s.CreateRecordFile(FileLocation{Name: "DUA0:[000000]T.OBJ"}, VariableRecords, [][]byte{{9}}); err != nil {
		t.Fatalf("second CreateRecordFile: %v", err)
	}

	if loc.Name != "DUA0:[000000]T.OBJ;2" {
		t.Errorf("second CreateRecordFile location = %+v, want version 2", loc)
	}

	got, found, err = s.ReadRecordFile(FileLocation{Name: "T.OBJ"}, VariableRecords)
	if err != nil || len(got) != 1 || found.Name != "DUA0:[000000]T.OBJ;2" {
		t.Errorf("ReadRecordFile after a second version = %v, %+v, %v", got, found, err)
	}

	// An explicit version.
	if loc, err = s.CreateRecordFile(FileLocation{Name: "T.OBJ;7"}, VariableRecords, records); err != nil || loc.Name != "DUA0:[000000]T.OBJ;7" {
		t.Errorf("CreateRecordFile(T.OBJ;7) = %+v, %v", loc, err)
	}
}

// normalize makes empty and nil records compare equal.
func normalize(records [][]byte) [][]byte {
	out := make([][]byte, len(records))
	for i, r := range records {
		out[i] = append([]byte{}, r...)
	}

	return out
}

func TestRecordFile_volumeText(t *testing.T) {
	s, vol := newCopyTestSession(t)
	lines := [][]byte{[]byte("first line"), {}, []byte("third")}

	if _, err := s.CreateRecordFile(FileLocation{Name: "T.LIS"}, TextRecords, lines); err != nil {
		t.Fatalf("CreateRecordFile: %v", err)
	}

	attr := fileAttributes(t, vol, "T.LIS", 1)
	if attr.Format != ondisk.RecordFormatVariable || attr.Attributes != ratCarriageReturn || attr.RecordSize != 10 {
		t.Errorf("attributes = %+v, want VAR, RAT=CR, longest record 10", attr)
	}

	got, _, err := s.ReadRecordFile(FileLocation{Name: "T.LIS"}, TextRecords)
	if err != nil || !reflect.DeepEqual(normalize(got), normalize(lines)) {
		t.Errorf("ReadRecordFile = %q, %v; want %q", got, err, lines)
	}

	// A Stream_LF file, as COPY/HOST makes a source, reads as lines too.
	got, _, err = s.ReadRecordFile(FileLocation{Name: "FOO.TXT"}, TextRecords)
	if err != nil || !reflect.DeepEqual(got, [][]byte{[]byte("line one"), []byte("line two")}) {
		t.Errorf("ReadRecordFile(FOO.TXT) = %q, %v", got, err)
	}
}

// TestRecordFile_volumeRawObject reads an object module that an older
// COPY/BINARY left as an Undefined-format file holding the host layout.
func TestRecordFile_volumeRawObject(t *testing.T) {
	s, vol := newCopyTestSession(t)

	var buf bytes.Buffer
	if err := obj.WriteRecords(&buf, [][]byte{{1, 2, 3}, {4}}); err != nil {
		t.Fatal(err)
	}

	createFormattedTestFile(t, vol, "RAW.OBJ", buf.String(), ondisk.RecordFormatUndefined)

	got, _, err := s.ReadRecordFile(FileLocation{Name: "RAW.OBJ"}, VariableRecords)
	if err != nil || !reflect.DeepEqual(got, [][]byte{{1, 2, 3}, {4}}) {
		t.Errorf("ReadRecordFile(RAW.OBJ) = %v, %v", got, err)
	}
}

func TestRecordFile_volumeErrors(t *testing.T) {
	s, _ := newCopyTestSession(t)

	var notFound *NotFoundError
	if _, _, err := s.ReadRecordFile(FileLocation{Name: "NONE.OBJ"}, VariableRecords); !errors.As(err, &notFound) {
		t.Errorf("reading a missing file: error = %v, want a *NotFoundError", err)
	}

	var ambiguous *AmbiguousError
	if _, _, err := s.ReadRecordFile(FileLocation{Name: "DUP.TXT;*"}, TextRecords); !errors.As(err, &ambiguous) {
		t.Errorf("reading a wildcard: error = %v, want an *AmbiguousError", err)
	}

	for _, name := range []string{"*.OBJ", "T.*", "T.OBJ;*", "[000000]", "T.OBJ;X"} {
		if _, err := s.CreateRecordFile(FileLocation{Name: name}, VariableRecords, nil); err == nil {
			t.Errorf("CreateRecordFile(%s) succeeded, want an error", name)
		}
	}

	var notMounted *NotMountedError
	if _, err := s.CreateRecordFile(FileLocation{Name: "DUB0:T.OBJ"}, VariableRecords, nil); !errors.As(err, &notMounted) {
		t.Errorf("creating on an unmounted device: error = %v, want a *NotMountedError", err)
	}

	// A read-only volume.
	mounts := NewMountTable()
	if err := mounts.Mount("DUA1", newTestVolumeFile(t, "ROVOL"), false); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	ro := NewSession(mounts)
	if _, err := ro.CreateRecordFile(FileLocation{Name: "DUA1:[000000]T.OBJ"}, VariableRecords, nil); err == nil {
		t.Error("creating on a read-only volume succeeded")
	}
}

func TestRecordFile_hostVariable(t *testing.T) {
	s := NewSession(NewMountTable())
	path := filepath.Join(t.TempDir(), "t.obj")
	records := sampleRecords()

	loc, err := s.CreateRecordFile(FileLocation{Host: true, Name: path}, VariableRecords, records)
	if err != nil || loc != (FileLocation{Host: true, Name: path}) {
		t.Fatalf("CreateRecordFile = %+v, %v", loc, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var want bytes.Buffer
	if err := obj.WriteRecords(&want, records); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, want.Bytes()) {
		t.Error("host file isn't the ODS-2 variable-length layout")
	}

	got, found, err := s.ReadRecordFile(FileLocation{Host: true, Name: path}, VariableRecords)
	if err != nil || found.Name != path || !reflect.DeepEqual(normalize(got), normalize(records)) {
		t.Errorf("ReadRecordFile = %v, %+v, %v", got, found, err)
	}

	// Bytes that aren't the layout.
	if err := os.WriteFile(path, []byte{9, 0, 1}, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.ReadRecordFile(FileLocation{Host: true, Name: path}, VariableRecords); err == nil {
		t.Error("ReadRecordFile of a truncated record succeeded")
	}
}

func TestRecordFile_hostText(t *testing.T) {
	s := NewSession(NewMountTable())
	dir := t.TempDir()
	path := filepath.Join(dir, "t.mar")

	if err := os.WriteFile(path, []byte("one\r\n\ttwo\n\nlast"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, _, err := s.ReadRecordFile(FileLocation{Host: true, Name: path}, TextRecords)
	want := [][]byte{[]byte("one"), []byte("\ttwo"), {}, []byte("last")}

	if err != nil || !reflect.DeepEqual(normalize(got), normalize(want)) {
		t.Errorf("ReadRecordFile = %q, %v; want %q", got, err, want)
	}

	out := filepath.Join(dir, "t.lis")
	if _, err := s.CreateRecordFile(FileLocation{Host: true, Name: out}, TextRecords, want); err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(out); string(data) != "one\n\ttwo\n\nlast\n" {
		t.Errorf("host text file = %q", data)
	}
}

// TestRecordFile_hostWriteIsAllOrNothing checks that a host file is only
// replaced once it's written in full, and that a failure leaves nothing
// behind.
func TestRecordFile_hostWriteIsAllOrNothing(t *testing.T) {
	s := NewSession(NewMountTable())
	dir := t.TempDir()
	path := filepath.Join(dir, "t.obj")

	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A record too long for the layout fails before anything is written.
	if _, err := s.CreateRecordFile(FileLocation{Host: true, Name: path}, VariableRecords, [][]byte{make([]byte, 0x10000)}); err == nil {
		t.Error("CreateRecordFile with a 64KB record succeeded")
	}

	if data, _ := os.ReadFile(path); string(data) != "old" {
		t.Errorf("failed write changed the old file to %q", data)
	}

	if _, err := s.CreateRecordFile(FileLocation{Host: true, Name: path}, VariableRecords, [][]byte{{1}}); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("directory holds %d files after writing, want only t.obj", len(entries))
	}

	missing := filepath.Join(dir, "no", "such", "t.obj")
	if _, err := s.CreateRecordFile(FileLocation{Host: true, Name: missing}, VariableRecords, nil); err == nil {
		t.Error("CreateRecordFile into a missing directory succeeded")
	}
}
