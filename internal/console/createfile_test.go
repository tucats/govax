package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
)

// createdFileAttributes is the record attributes of name (no version: the
// highest) in DUA0's MFD, [000000].
func createdFileAttributes(t *testing.T, d *Dispatcher, name string) ondisk.RecAttr {
	t.Helper()

	vol := mustVolume(t, d)

	dir, err := filespec.ResolveDirectory(vol, nil)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := dir.Lookup(name, 0)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", name, err)
	}

	f, err := vol.OpenFID(entry.Fid)
	if err != nil {
		t.Fatalf("OpenFID(%s): %v", name, err)
	}

	return f.Header.RecordAttributes
}

// TestCreateFile_volume: CREATE in a procedure makes a volume file of its
// data lines, variable-length records with carriage-return carriage
// control, and /LOG names it with its version.
func TestCreateFile_volume(t *testing.T) {
	d, out := newCreateTestDispatcher(t)
	proc := writeProcedure(t, t.TempDir(), "create.com",
		"$ CREATE/LOG NOTES.TXT", "first line", "", "  third", "$ TYPE NOTES.TXT",
		"$ CREATE NOTES.TXT", "version two", "$ TYPE NOTES.TXT;2")

	if err := d.Dispatch("@" + proc); err != nil {
		t.Fatalf("@create: %v", err)
	}

	want := "%CREATE-I-CREATED, DUA0:[000000]NOTES.TXT;1 created\n" +
		"first line\n\n  third\n" +
		"version two\n"
	if out.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", out.String(), want)
	}

	attr := createdFileAttributes(t, d, "NOTES.TXT")
	if attr.Format != ondisk.RecordFormatVariable || attr.Attributes != 0x02 {
		t.Errorf("attributes: format %v, RAT %#x; want VAR and CR (0x02)", attr.Format, attr.Attributes)
	}
}

// TestCreateFile_host: a host file name makes a host text file, each line
// ending with the host's line ending; with no data lines, the file is
// still made, empty.
func TestCreateFile_host(t *testing.T) {
	d, _, out := newCommandDispatcher(t)
	dir := t.TempDir()
	text, empty := filepath.Join(dir, "text.txt"), filepath.Join(dir, "empty.txt")
	proc := writeProcedure(t, dir, "create.com",
		`$ CREATE "`+text+`"`, "one", "", "two",
		`$ CREATE "`+empty+`"`,
		`$ PRINT "done"`)

	if err := d.Dispatch("@" + proc); err != nil {
		t.Fatalf("@create: %v", err)
	}

	if out.String() != "done\n" {
		t.Errorf("output: %q", out.String())
	}

	got, err := os.ReadFile(text)
	if err != nil {
		t.Fatal(err)
	}

	if want := strings.Join([]string{"one", "", "two", ""}, hostLineEndForTest()); string(got) != want {
		t.Errorf("%s: %q, want %q", text, got, want)
	}

	if got, err := os.ReadFile(empty); err != nil || len(got) != 0 {
		t.Errorf("%s: %q, %v; want an empty file", empty, got, err)
	}
}

// TestCreateFile_terminal: at the terminal, CREATE reads what is typed to
// the end of the input (Ctrl/Z).
func TestCreateFile_terminal(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	c.In = strings.NewReader("typed one\ntyped two\n")
	path := filepath.Join(t.TempDir(), "typed.txt")

	if err := d.Dispatch(`CREATE "` + path + `"`); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if want := "typed one" + hostLineEndForTest() + "typed two" + hostLineEndForTest(); string(got) != want {
		t.Errorf("%s: %q, want %q", path, got, want)
	}
}

// hostLineEndForTest is the line ending a host text file gets.
func hostLineEndForTest() string {
	if filepath.Separator == '\\' {
		return "\r\n"
	}

	return "\n"
}
