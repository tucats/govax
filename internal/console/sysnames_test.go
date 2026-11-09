package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
)

// The system logical names (docs/PHASE-91 - concealed logical names.md,
// Decisions 1 and 6, subtask 8).

// TestSystemNames boots on a system volume and holds what its names do to
// the forms VMS's do in testdata/lnm91/vax/lnm91.log. They can't be compared
// line for line: VMS's SYS$SYSROOT is a search list on [SYS0.] and
// [SYS0.SYSCOMMON.], govax's one root, [SYSTEM.], so here each name shows
// itself, where VMS's show SYS$COMMON for a file found in its second element.
func TestSystemNames(t *testing.T) {
	_, _, run := bootOnSystemVolume(t, filepath.Join(t.TempDir(), "govax-system.dsk"), false, "")

	const rooted = " = \"_DUA0:[SYSTEM.]\" [concealed,terminal] (LNM$SYSTEM_TABLE)\n"

	for _, c := range []struct{ line, want string }{
		// The names themselves.
		{"SHOW LOGICAL/FULL SYS$SYSROOT", `   "SYS$SYSROOT" [exec]` + rooted},
		{"SHOW LOGICAL/FULL SYS$COMMON", `   "SYS$COMMON" [exec]` + rooted},
		{"SHOW LOGICAL/FULL SYS$SPECIFIC", `   "SYS$SPECIFIC" [exec]` + rooted},
		{"SHOW LOGICAL/FULL SYS$SYSDEVICE", `   "SYS$SYSDEVICE" [exec] = "_DUA0:" [concealed,terminal] (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$SYSTEM", `   "SYS$SYSTEM" [exec] = "SYS$SYSROOT:[SYSEXE]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$MANAGER", `   "SYS$MANAGER" [exec] = "SYS$SYSROOT:[SYSMGR]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$HELP", `   "SYS$HELP" [exec] = "SYS$SYSROOT:[SYSHLP]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$LIBRARY", `   "SYS$LIBRARY" [exec] = "SYS$SYSROOT:[SYSLIB]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$SHARE", `   "SYS$SHARE" [exec] = "SYS$SYSROOT:[SYSLIB]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SHOW LOGICAL/FULL SYS$SOURCES", `   "SYS$SOURCES" [exec] = "SYS$SYSROOT:[SOURCES]" (LNM$SYSTEM_TABLE)` + "\n"},
		{`WRITE SYS$OUTPUT F$TRNLNM("SYS$SYSROOT",,,,,"CONCEALED"), " ", F$TRNLNM("SYS$SYSROOT",,,,,"TERMINAL")`, "TRUE TRUE\n"},

		// F$PARSE and F$SEARCH.
		{`WRITE SYS$OUTPUT F$PARSE("SYS$SYSTEM:DEFAULT.ROM")`, "SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;\n"},
		{`WRITE SYS$OUTPUT F$PARSE("SYS$SYSROOT:[SYSEXE]DEFAULT.ROM",,,,"NO_CONCEAL")`, "_DUA0:[SYSTEM.][SYSEXE]DEFAULT.ROM;\n"},
		{`WRITE SYS$OUTPUT F$PARSE("SYS$SYSTEM:DEFAULT.ROM",,,"DEVICE"), " ", F$PARSE("SYS$SYSTEM:DEFAULT.ROM",,,"DIRECTORY")`, "SYS$SYSROOT: [SYSEXE]\n"},
		{`WRITE SYS$OUTPUT F$PARSE("SYS$SYSTEM:DEFAULT.ROM",,,"DEVICE","NO_CONCEAL"), " ", F$PARSE("SYS$SYSTEM:DEFAULT.ROM",,,"DIRECTORY","NO_CONCEAL")`, "_DUA0: [SYSTEM.][SYSEXE]\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$SYSTEM:DEFAULT.ROM")`, "SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;1\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$COMMON:[SYSEXE]DEFAULT.ROM")`, "SYS$COMMON:[SYSEXE]DEFAULT.ROM;1\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$SPECIFIC:[SYSEXE]DEFAULT.ROM")`, "SYS$SPECIFIC:[SYSEXE]DEFAULT.ROM;1\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM")`, "SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM;1\n"},

		// The device services see through them, and say nothing of it.
		{`WRITE SYS$OUTPUT F$GETDVI("SYS$SYSROOT:","DEVNAM"), " ", F$GETDVI("SYS$SYSROOT:","CONCEALED")`, "_DUA0: FALSE\n"},
		{`WRITE SYS$OUTPUT F$GETDVI("SYS$SYSTEM:","DEVNAM"), " ", F$GETDVI("SYS$SYSTEM:","CONCEALED")`, "_DUA0: FALSE\n"},

		// The default.
		{"SET DEFAULT SYS$SYSTEM:", ""},
		{"SHOW DEFAULT", "  SYS$SYSROOT:[SYSEXE]\n"},
		{"SHOW LOGICAL SYS$DISK", `   "SYS$DISK" = "SYS$SYSROOT:" (LNM$PROCESS_TABLE)` + "\n" +
			`1  "SYS$SYSROOT" = "_DUA0:[SYSTEM.]" (LNM$SYSTEM_TABLE)` + "\n"},
		{"SET DEFAULT SYS$COMMON:[SYSEXE]", ""},
		{"SHOW DEFAULT", "  SYS$COMMON:[SYSEXE]\n"},
		{"SET DEFAULT SYS$SYSROOT:[-]", ""},
		{"SHOW DEFAULT", "  SYS$SYSROOT:[000000]\n"},
		{"SET DEFAULT SYS$SYSDEVICE:[000000]", ""},

		// A missing image names the physical form, and a directory with no
		// name of its own the process default below the default's root.
		{"RUN SYS$SYSROOT:[SYSEXE]NOSUCH", "%DCL-W-ACTIMAGE, error activating image SYS$SYSROOT:[SYSEXE]NOSUCH\n" +
			"-CLI-E-IMAGEFNF, image file not found _DUA0:[SYSTEM.][SYSEXE]NOSUCH.EXE;\n"},
		{"RUN SYS$SYSTEM:NOSUCH", "%DCL-W-ACTIMAGE, error activating image SYS$SYSTEM:NOSUCH\n" +
			"-CLI-E-IMAGEFNF, image file not found _DUA0:[SYSTEM.][SYSEXE]NOSUCH.EXE;\n"},
		{"HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=[]H90A ALPS", "%HELP-E-OPENIN, error opening SYS$SYSROOT:[000000]H90A.HLB; as input\n" +
			"-RMS-E-FNF, file not found\n"},
	} {
		if got := run(c.line); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.line, got, c.want)
		}
	}

	// DIRECTORY's heading is the name it went through, the root's own
	// directory [000000].
	for _, c := range []struct{ line, heading, file string }{
		{"DIRECTORY SYS$SYSTEM:DEFAULT.ROM", "Directory SYS$SYSROOT:[SYSEXE]\n", "DEFAULT.ROM;1"},
		{"DIRECTORY SYS$COMMON:[SYSEXE]DEFAULT.ROM", "Directory SYS$COMMON:[SYSEXE]\n", "DEFAULT.ROM;1"},
		{"DIRECTORY SYS$SPECIFIC:[SYSEXE]DEFAULT.ROM", "Directory SYS$SPECIFIC:[SYSEXE]\n", "DEFAULT.ROM;1"},
		{"DIRECTORY SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM", "Directory SYS$SYSDEVICE:[SYSTEM.SYSEXE]\n", "DEFAULT.ROM;1"},
		{"DIRECTORY SYS$SYSROOT:[000000]GOVAX$FILES.DAT", "Directory SYS$SYSROOT:[000000]\n", "GOVAX$FILES.DAT;"},
	} {
		if got := run(c.line); !strings.Contains(got, c.heading) || !strings.Contains(got, c.file) {
			t.Errorf("%s: want %q and %q in:\n%s", c.line, c.heading, c.file, got)
		}
	}
}

// oldStartup is STARTUP.COM as govax's before Phase 91 had it, each system
// name a directory on SYS$SYSDEVICE.
func oldStartup(t *testing.T) []byte {
	t.Helper()

	f, ok := sysvol.Lookup(sysvol.SysMgr + "STARTUP.COM")
	if !ok {
		t.Fatal("no STARTUP.COM in the table")
	}

	text, err := f.Contents()
	if err != nil {
		t.Fatal(err)
	}

	old := string(text)
	for _, dir := range []string{"sysexe", "sysmgr", "syshlp", "syslib", "sources"} {
		old = strings.ReplaceAll(old, "sys$sysroot:["+dir+"]", "sys$sysdevice:[system."+dir+"]")
	}

	if old == string(text) {
		t.Fatal("STARTUP.COM has no sys$sysroot:[dir] names")
	}

	return sysvol.Canonical([]byte(old), sysvol.Text)
}

// startSystemVolumeWithStartup makes a volume at path whose STARTUP.COM is
// startup and whose manifest records recorded as govax's own copy of it.
func startSystemVolumeWithStartup(t *testing.T, path string, startup, recorded []byte) {
	t.Helper()

	c, _, r := mountTestSystemVolume(t, dclcmd.SystemVolume{File: path})
	if !r.Created || !r.Writable {
		t.Fatalf("first start = %+v", r)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:" + sysvol.ManifestSpec}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	m, err := sysvol.ParseManifest(records)
	if err != nil {
		t.Fatal(err)
	}

	spec := sysvol.SysMgr + "STARTUP.COM"
	writeVolumeFile(t, c, spec, startup, sysvol.Text)
	m.Set(spec, sysvol.SumOf(recorded))
	writeVolumeFile(t, c, sysvol.ManifestSpec, bytes.Join(append(m.Records(), nil), []byte{'\n'}), sysvol.Text)

	if err := c.Mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}
}

// TestSystemNamesRefresh is a volume made by a govax with the old names. Its
// STARTUP.COM, unchanged, gets the new copy at the next start, and the system
// names are rooted from then on.
func TestSystemNamesRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")
	old := oldStartup(t)

	startSystemVolumeWithStartup(t, path, old, old)

	_, buf, run := bootOnSystemVolume(t, path, false, "")
	if strings.Contains(buf.String(), "SYSKEPT") {
		t.Errorf("a STARTUP.COM of govax's was kept:\n%s", buf)
	}

	if got, want := run(`WRITE SYS$OUTPUT F$TRNLNM("SYS$LIBRARY"), " ", F$SEARCH("SYS$SYSTEM:DEFAULT.ROM")`),
		"SYS$SYSROOT:[SYSLIB] SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;1\n"; got != want {
		t.Errorf("after the refresh: %q, want %q", got, want)
	}
}

// TestSystemNamesKeptStartup is the same volume with a STARTUP.COM the user
// changed: it is kept, with SYSKEPT, and its names of directories on
// SYS$SYSDEVICE go on working beside the rooted names govax defines itself.
func TestSystemNamesKeptStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "govax-system.dsk")
	mine := append(oldStartup(t), "$ write sys$output \"my startup\"\n"...)

	startSystemVolumeWithStartup(t, path, mine, []byte("an older govax's STARTUP.COM\n"))

	_, buf, run := bootOnSystemVolume(t, path, false, "")
	if !strings.Contains(buf.String(), "%MOUNT-I-SYSKEPT, DUA0:"+sysvol.SysMgr+"STARTUP.COM") || !strings.Contains(buf.String(), "my startup\n") {
		t.Errorf("the user's STARTUP.COM wasn't kept and run:\n%s", buf)
	}

	for _, c := range []struct{ line, want string }{
		{`WRITE SYS$OUTPUT F$TRNLNM("SYS$LIBRARY")`, "SYS$SYSDEVICE:[SYSTEM.SYSLIB]\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$SYSTEM:DEFAULT.ROM")`, "SYS$SYSDEVICE:[SYSTEM.SYSEXE]DEFAULT.ROM;1\n"},
		{`WRITE SYS$OUTPUT F$SEARCH("SYS$SYSROOT:[SYSEXE]DEFAULT.ROM")`, "SYS$SYSROOT:[SYSEXE]DEFAULT.ROM;1\n"},
	} {
		if got := run(c.line); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.line, got, c.want)
		}
	}
}
