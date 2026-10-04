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
		source string
		flags  macroFlags
		want   string
	}{
		{"hello.mar", macroFlags{}, `MACRO "hello.mar"`},
		{"/abs/Hello.mar", macroFlags{}, `MACRO "/abs/Hello.mar"`},
		{"hello.mar", macroFlags{object: "out/x.obj"}, `MACRO "hello.mar"/OBJECT="out/x.obj"`},
		{"hello.mar", macroFlags{noObject: true}, `MACRO "hello.mar"/NOOBJECT`},
		{"hello.mar", macroFlags{list: true}, `MACRO "hello.mar"/LIST`},
		{"hello.mar", macroFlags{list: true, listFile: "out/x.lis"}, `MACRO "hello.mar"/LIST="out/x.lis"`},
		{"hello.mar", macroFlags{noObject: true, list: true}, `MACRO "hello.mar"/NOOBJECT/LIST`},
		{"hello.mar", macroFlags{list: true, show: []string{"ME", "MEB"}}, `MACRO "hello.mar"/LIST/SHOW=(ME,MEB)`},
		{"hello.mar", macroFlags{list: true, noShow: []string{"CALLS"}}, `MACRO "hello.mar"/LIST/NOSHOW=(CALLS)`},
		{"hello.mar", macroFlags{list: true, xref: true}, `MACRO "hello.mar"/LIST/CROSS_REFERENCE`},
		{"hello.mar", macroFlags{enable: []string{"DEBUG"}, disable: []string{"GLOBAL", "TBK"}}, `MACRO "hello.mar"/ENABLE=(DEBUG)/DISABLE=(GLOBAL,TBK)`},
		{"hello.mar", macroFlags{debug: []string{"TRACEBACK"}}, `MACRO "hello.mar"/DEBUG=(TRACEBACK)`},
		{"hello.mar", macroFlags{noDebug: true}, `MACRO "hello.mar"/NODEBUG`},
		{"hello.mar", macroFlags{list: true, xref: true, xrefKinds: []string{"ALL"}}, `MACRO "hello.mar"/LIST/CROSS_REFERENCE=(ALL)`},
		{"DUA0:[X]HELLO.MAR", macroFlags{}, `MACRO "DUA0:[X]HELLO.MAR"`},
		{"p", macroFlags{libraries: []string{"a.mlb", "/x/B"}}, `MACRO "p"/LIBRARY=("a.mlb","/x/B")`},
	}

	for _, c := range cases {
		if got := macroCommand(c.source, c.flags); got != c.want {
			t.Errorf("macroCommand(%q, %+v) = %q, want %q", c.source, c.flags, got, c.want)
		}
	}
}

func TestLinkCommand(t *testing.T) {
	cases := []struct {
		objects []string
		flags   linkFlags
		want    string
	}{
		{[]string{"a.obj"}, linkFlags{}, `LINK "a.obj"`},
		{[]string{"a.obj", "/x/b"}, linkFlags{}, `LINK "a.obj","/x/b"`},
		{[]string{"a"}, linkFlags{executable: "out.exe", noTraceback: true}, `LINK "a"/EXECUTABLE="out.exe"/NOTRACEBACK`},
		{[]string{"a"}, linkFlags{noExecutable: true}, `LINK "a"/NOEXECUTABLE`},
		{[]string{"a"}, linkFlags{noSysLib: true}, `LINK "a"/NOSYSLIB`},
		{[]string{"a"}, linkFlags{mapWanted: true}, `LINK "a"/MAP`},
		{[]string{"a"}, linkFlags{mapWanted: true, mapFile: "a.lst", brief: true}, `LINK "a"/MAP="a.lst"/BRIEF`},
		{[]string{"a"}, linkFlags{libraries: []string{"x.olb", "y"}, options: []string{"p.opt"}}, `LINK "a","x.olb"/LIBRARY,"y"/LIBRARY,"p.opt"/OPTIONS`},
	}

	for _, c := range cases {
		if got := linkCommand(c.objects, c.flags); got != c.want {
			t.Errorf("linkCommand(%v, %+v) = %q, want %q", c.objects, c.flags, got, c.want)
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

	for _, command := range []string{macroCommand(src, macroFlags{}), linkCommand([]string{filepath.Join(dir, "prog")}, linkFlags{}), "RUN " + dclQuote(filepath.Join(dir, "prog.exe"))} {
		var buf bytes.Buffer
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}
	}
}

// TestRun_macroLibrariesOneShot makes a macro library, assembles a
// program that calls its macro and STARLET.MLB's $EXIT_S, then links and
// runs it, all as one-shot commands.
func TestRun_macroLibrariesOneShot(t *testing.T) {
	dir := t.TempDir()
	macros := filepath.Join(dir, "mine.mar")
	src := filepath.Join(dir, "prog.mar")

	files := map[string]string{
		macros: "\t.MACRO\tSETR0\tVALUE\n\tMOVL\t#VALUE,R0\n\t.ENDM\tSETR0\n",
		src:    "\t.PSECT\tC,NOWRT,EXE\n\t.ENTRY\tGO,^M<>\n\tSETR0\t1\n\t$EXIT_S\tR0\n\t.END\tGO\n",
	}

	for name, text := range files {
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	lib := filepath.Join(dir, "mine.mlb")

	for _, command := range []string{
		libraryCommand(lib, []string{macros}, libraryFlags{create: true, macro: true}),
		macroCommand(src, macroFlags{libraries: []string{lib}}),
		linkCommand([]string{filepath.Join(dir, "prog")}, linkFlags{}),
		"RUN " + dclQuote(filepath.Join(dir, "prog.exe")),
	} {
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
	if err := run(nil, 0, 0, &buf, emptyStdin(), []string{macroCommand(src, macroFlags{})}); err != nil {
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

	err := run(nil, 0, 0, &buf, readCloser{in}, []string{macroCommand(src, macroFlags{})})
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
		if err := run(nil, 0, 0, &buf, emptyStdin(), []string{macroCommand("DUA0:[000000]SMALL", macroFlags{})}); err != nil {
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
