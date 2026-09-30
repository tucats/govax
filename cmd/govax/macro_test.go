package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
)

const smallSource = "\t.TITLE\tSMALL\n\t.PSECT\tDATA\n\t.LONG\t1,2,3\n\t.END\n"

func TestMacroCommand(t *testing.T) {
	cases := []struct {
		source, object string
		noObject       bool
		want           string
	}{
		{"hello.mar", "", false, `MACRO "hello.mar"`},
		{"/abs/Hello.mar", "", false, `MACRO "/abs/Hello.mar"`},
		{"hello.mar", "out/x.obj", false, `MACRO "hello.mar"/OBJECT="out/x.obj"`},
		{"hello.mar", "", true, `MACRO "hello.mar"/NOOBJECT`},
		{"DUA0:[X]HELLO.MAR", "", false, `MACRO "DUA0:[X]HELLO.MAR"`},
	}

	for _, c := range cases {
		if got := macroCommand(c.source, c.object, c.noObject); got != c.want {
			t.Errorf("macroCommand(%q, %q, %v) = %q, want %q", c.source, c.object, c.noObject, got, c.want)
		}
	}
}

func TestLinkCommand(t *testing.T) {
	cases := []struct {
		objects     []string
		executable  string
		noExe, noTB bool
		want        string
	}{
		{[]string{"a.obj"}, "", false, false, `LINK "a.obj"`},
		{[]string{"a.obj", "/x/b"}, "", false, false, `LINK "a.obj","/x/b"`},
		{[]string{"a"}, "out.exe", false, true, `LINK "a"/EXECUTABLE="out.exe"/NOTRACEBACK`},
		{[]string{"a"}, "", true, false, `LINK "a"/NOEXECUTABLE`},
	}

	for _, c := range cases {
		if got := linkCommand(c.objects, c.executable, c.noExe, c.noTB); got != c.want {
			t.Errorf("linkCommand(%v, %q, %v, %v) = %q, want %q", c.objects, c.executable, c.noExe, c.noTB, got, c.want)
		}
	}
}

// TestRun_macroLinkRunOneShot assembles, links, and runs a program with
// three one-shot commands, as govax macro, link, and run would.
func TestRun_macroLinkRunOneShot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.mar")

	if err := os.WriteFile(src, []byte("\t.PSECT\tC,NOWRT,EXE\n\t.ENTRY\tGO,^M<>\n\tMOVL\t#1,R0\n\tRET\n\t.END\tGO\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{macroCommand(src, "", false), linkCommand([]string{filepath.Join(dir, "prog")}, "", false, false), "RUN " + dclQuote(filepath.Join(dir, "prog.exe"))} {
		var buf bytes.Buffer
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}
	}
}

// TestRun_macroOneShot runs the macro subcommand's console command as a
// one-shot command: a host source, keeping the case of its path.
func TestRun_macroOneShot(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "Small.Mar")

	if err := os.WriteFile(src, []byte(smallSource), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), []string{macroCommand(src, "", false)}); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "Small.Obj")); err != nil {
		t.Errorf("no Small.Obj: %v", err)
	}
}

// TestRun_failedOneShotEndsWithItsError checks that a one-shot command
// that fails ends the session and run returns its failure.
func TestRun_failedOneShotEndsWithItsError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.mar")

	if err := os.WriteFile(src, []byte("\tBOGUS\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer

	// Input that would run if the session went on to the prompt.
	in := strings.NewReader("SHOW VERSION\n")

	err := run(nil, 0, 0, &buf, readCloser{in}, []string{macroCommand(src, "", false)})
	if err == nil || !strings.Contains(err.Error(), "ASMERRORS") {
		t.Fatalf("run error = %v, want the MACRO failure", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "bad.obj")); err == nil {
		t.Error("an object was written despite the error")
	}
}

type readCloser struct{ *strings.Reader }

func (readCloser) Close() error { return nil }

// TestRun_mountWrite mounts a container for a one-shot MACRO whose source
// and object are on it, and checks the volume is dismounted cleanly (its
// allocation bitmaps written) when the session ends, so a later session
// can create files on it.
func TestRun_mountWrite(t *testing.T) {
	disk := filepath.Join(t.TempDir(), "x.dsk")
	if err := rms.InitializeContainer(disk, 1000, "MACVOL", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	mounts := rms.NewMountTable()
	if err := mounts.Mount("DUA0", disk, true); err != nil {
		t.Fatal(err)
	}

	s := rms.NewSession(mounts)
	if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]SMALL.MAR"}, rms.TextRecords,
		[][]byte{[]byte("\t.PSECT\tDATA"), []byte("\t.LONG\t1"), []byte("\t.END")}); err != nil {
		t.Fatal(err)
	}

	if err := mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}

	saved := mountRequests
	defer func() { mountRequests = saved }()

	for i := 1; i <= 2; i++ {
		mountRequests = []mountRequest{{device: "DUA0", path: disk, write: true}}

		var buf bytes.Buffer
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{macroCommand("DUA0:[000000]SMALL", "", false)}); err != nil {
			t.Fatalf("run %d: %v\n%s", i, err, buf.String())
		}
	}

	if err := mounts.Mount("DUA0", disk, false); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"DUA0:[000000]SMALL.OBJ;1", "DUA0:[000000]SMALL.OBJ;2"} {
		records, _, err := s.ReadRecordFile(rms.FileLocation{Name: name}, rms.VariableRecords)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if _, err := obj.Decode(records); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestRun_mountFailure checks that a --mount that fails ends run with
// its error.
func TestRun_mountFailure(t *testing.T) {
	saved := mountRequests
	defer func() { mountRequests = saved }()

	mountRequests = []mountRequest{{device: "DUA0", path: filepath.Join(t.TempDir(), "none.dsk")}}

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err == nil {
		t.Error("run with a missing container succeeded")
	}
}
