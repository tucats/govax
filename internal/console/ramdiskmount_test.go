package console_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmserrors"
)

// ramDiskSteps runs each line and compares what it shows; a want ending
// in "..." is a prefix.
func ramDiskSteps(t *testing.T, dispatch func(string) string, steps []struct{ line, want string }) {
	t.Helper()

	for _, step := range steps {
		got := trimLines(dispatch(step.line))

		if prefix, ok := strings.CutSuffix(step.want, "..."); ok {
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("%s:\n got %q\nwant %q...", step.line, got, prefix)
			}

			continue
		}

		if got != step.want {
			t.Errorf("%s:\n got %q\nwant %q", step.line, got, step.want)
		}
	}
}

// trimLines takes the blanks off the end of each of text's lines.
func trimLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	return strings.Join(lines, "\n")
}

// TestRAMDisk is the session in the plan's "Goal", and MOUNT, DISMOUNT,
// and SHOW DEVICE of RAM disks around it (docs/planning/PHASE-96 - RAM
// disk.md, Decisions 4, 6, and 7): the unit MOUNT MDA takes, the
// refusals, the label on a first mount and INCVOLLABEL on a later one,
// contents surviving DISMOUNT and lost by a redefinition, the volume's
// owner, SHOW DEVICE in both forms with the memory lines, F$GETDVI, and a
// FORTRAN program writing a file there.
func TestRAMDisk(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c, dispatch := ramDiskConsole(t)

	login := filepath.Join(t.TempDir(), "login.com")
	if err := os.WriteFile(login, []byte("$ WRITE SYS$OUTPUT \"LOGGED IN\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ramDiskSteps(t, dispatch, []struct{ line, want string }{
		// The plan's session.
		{"DEFINE/DEVICE MDA /MAXBLOCK=20000", "%VAX-I-RAMDISK, _MDA0: defined, 20000 blocks"},
		{"MOUNT MDA0: SCRATCH", "%MOUNT-I-MOUNTED, SCRATCH mounted on _MDA0:"},
		{"CREATE/DIRECTORY MDA0:[WORK]", ""},
		{`COPY "` + login + `"/HOST MDA0:[WORK]LOGIN.COM`, ""},
		{"SHOW DEVICE MDA", "Device                  Device           Error    Volume         Free  Trans Mnt\n" +
			" Name                   Status           Count     Label        Blocks Count Cnt\n" +
			"MDA0:                   Mounted              0  SCRATCH      ..."},
		{"DISMOUNT MDA0:", ""},
		{"MOUNT MDA0: SCRATCH", "%MOUNT-I-MOUNTED, SCRATCH mounted on _MDA0:"},
		{"DIRECTORY MDA0:[WORK]", "Directory MDA0:[WORK]\n\nLOGIN.COM;1\n\nTotal of 1 file."},
		{"TYPE MDA0:[WORK]LOGIN.COM", `$ WRITE SYS$OUTPUT "LOGGED IN"`},

		// The volume's name, and $GETDVI's view of it.
		{"SHOW LOGICAL DISK$SCRATCH", `"DISK$SCRATCH" = "_MDA0:" (LNM$SYSTEM_TABLE)`},
		{`WRITE SYS$OUTPUT F$GETDVI("MDA0","MNT"), " ", F$GETDVI("MDA0","VOLNAM"), " ", F$GETDVI("MDA0","DEVTYPE"), " ", F$GETDVI("MDA0","MAXBLOCK")`,
			"TRUE SCRATCH 58 20000"},

		// Refusals: mounted already, an MD unit not defined, the wrong
		// label on a later mount (and the volume left unmounted).
		{"MOUNT MDA0:", "%SYSTEM-F-DEVMOUNT, Device MDA0 already mounted"},
		{"MOUNT MDA7: X", "%SYSTEM-W-NOSUCHDEV, no such device available"},
		{"MOUNT MDA", "%SYSTEM-W-NOSUCHDEV, no such device available"},
		{"DISMOUNT MDA0:", ""},
		{"MOUNT MDA0: OTHER", "%MOUNT-F-INCVOLLABEL, incorrect volume label"},
		{`WRITE SYS$OUTPUT F$GETDVI("MDA0","MNT")`, "FALSE"},

		// With no label any volume mounts; MDA takes the lowest RAM disk
		// with nothing mounted on it.
		{"MOUNT/NOLOG MDA", ""},
		{"DEFINE/DEVICE MDA3 /MAXBLOCK=1000 /VOLNAME=THIRD", "%VAX-I-RAMDISK, _MDA3: defined, 1000 blocks"},
		{"DEFINE/DEVICE MDA /MAXBLOCK=1000", "%VAX-I-RAMDISK, _MDA1: defined, 1000 blocks"},
		{"MOUNT/NOWRITE MDA1:", "%VAX-W-RAMNOWRITE, _MDA1: has no volume yet; its first mount must be writable"},
		{"MOUNT MDA", "%MOUNT-I-MOUNTED, MDA1 mounted on _MDA1:"},
		{"MOUNT MDA", "%MOUNT-I-MOUNTED, THIRD mounted on _MDA3:"},
		{"MOUNT MDA", "%SYSTEM-W-NOSUCHDEV, no such device available"},

		// A later mount may be read-only.
		{"DISMOUNT MDA1:", ""},
		{"MOUNT/NOWRITE MDA1: MDA1", "%MOUNT-I-MOUNTED, MDA1 mounted on _MDA1:"},
		{"CREATE/DIRECTORY MDA1:[X]", "%CREATE-E-DIRNOTCRE, MDA1:[X] directory file not created\n-SYSTEM-F-WRITLCK, write lock error"},
		{"DISMOUNT MDA1:", ""},
		{"DISMOUNT MDA3:", ""},

		// A redefinition starts over with an empty volume.
		{"DEFINE/DEVICE MDA3 /MAXBLOCK=2000", "%VAX-I-RAMDISK, _MDA3: defined, 2000 blocks, previous contents discarded"},
		{"MOUNT MDA3: FRESH", "%MOUNT-I-MOUNTED, FRESH mounted on _MDA3:"},
		{"DIRECTORY MDA3:[000000]", "Directory MDA3:[000000]\n\n000000.DIR;1        BACKUP.SYS;1        BADBLK.SYS;1        BADLOG.SYS;1\n" +
			"BITMAP.SYS;1        CONTIN.SYS;1        CORIMG.SYS;1        INDEXF.SYS;1\nSECURITY.SYS;1      VOLSET.SYS;1\n\nTotal of 10 files."},
	})

	// A read-only mount refuses writes.
	if c.Mounts.Writable("MDA1") {
		t.Errorf("MDA1 is writable after MOUNT/NOWRITE")
	}

	// SHOW DEVICE/FULL names the type and adds the memory lines.
	full := dispatch("SHOW DEVICE MDA0/FULL")
	for _, want := range []string{
		"Disk MDA0:, device type RAM_DISK, is online, mounted (READ/WRITE), file-oriented device.",
		"Total blocks", "20000",
		`Volume label`, `"SCRATCH"`,
		"Host memory in use",
		"Default buffer size                 512",
		"RAM disks total", "23000 of 512000 blocks",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("SHOW DEVICE/FULL lacks %q:\n%s", want, full)
		}
	}

	if inUse := c.Mounts.MemoryInUse("MDA0"); inUse <= 0 || inUse > 20000*512/4 {
		t.Errorf("MDA0 holds %d bytes of host memory", inUse)
	}

	// The volume is the mounting process's, and gives everyone access.
	settings.Set(dclcmd.RAMDiskLimitSetting, "600000")

	if got := dispatch("SHOW DEVICE MDA0/FULL"); !strings.Contains(got, "23000 of 600000 blocks") {
		t.Errorf("SHOW DEVICE/FULL after the limit changed:\n%s", got)
	}

	if got := dispatch("SHOW SECURITY MDA0:/CLASS=VOLUME"); !strings.Contains(got, "Owner: [SYSTEM]") ||
		!strings.Contains(got, "Protection: (System: RWCD, Owner: RWCD, Group: RWCD, World: RWCD)") {
		t.Errorf("the volume's security:\n%s", got)
	}

	// A FORTRAN program writes a file there, which outlives a DISMOUNT.
	source := filepath.Join(t.TempDir(), "writer.for")
	program := "      OPEN (UNIT=1, FILE='OUT.DAT', STATUS='NEW')\n" +
		"      WRITE (1, '(1X, A)') 'WRITTEN ON A RAM DISK'\n" +
		"      CLOSE (UNIT=1)\n" +
		"      END\n"

	if err := os.WriteFile(source, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}

	ramDiskSteps(t, dispatch, []struct{ line, want string }{
		{"SET DEFAULT MDA0:[WORK]", ""},
		{`COPY "` + source + `"/HOST WRITER.FOR`, ""},
		{"FORTRAN WRITER", ""},
		{"MACRO/NOLIST WRITER", ""},
		{"LINK WRITER", ""},
		{"RUN WRITER", ""},
		{"SET DEFAULT DUA0:[000000]", "..."},
		{"DISMOUNT MDA0:", ""},
		{"MOUNT/NOLOG MDA0:", ""},
		{"TYPE MDA0:[WORK]OUT.DAT", "WRITTEN ON A RAM DISK"},
	})
}

// TestMountPrompt: MOUNT asks for a container file itself, now the
// grammar doesn't (Decision 6), as DCL asked for it: again while the reply
// is empty, the command dropped by Ctrl/Z, and in a procedure not at all
// (INSFPRM). A RAM disk needs no file, so isn't asked for one.
func TestMountPrompt(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	c := console.New(&out)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	t.Cleanup(func() { _ = c.Mounts.DismountAll() })

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	dsk := filepath.Join(t.TempDir(), "prompt.dsk")
	if err := d.Dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 PROMPTED`); err != nil {
		t.Fatalf("INITIALIZE/CONTAINER: %v", err)
	}

	out.Reset()

	c.In = strings.NewReader("\n\"" + dsk + "\"\n")

	if err := d.Dispatch("MOUNT/NOLOG DUA5"); err != nil {
		t.Fatalf("MOUNT DUA5: %v", err)
	}

	if got, want := out.String(), "_Container file: _Container file: "; got != want {
		t.Errorf("MOUNT DUA5: output %q, want %q", got, want)
	}

	if label, ok := c.Mounts.VolumeLabel("DUA5"); !ok || strings.TrimSpace(label) != "PROMPTED" {
		t.Errorf("DUA5's volume = %q, %v", label, ok)
	}

	// Ctrl/Z drops the command with no message.
	out.Reset()

	c.In = strings.NewReader("")

	if err := d.Dispatch("MOUNT DUA6"); err != nil || out.String() != "_Container file: " {
		t.Errorf("MOUNT, Ctrl/Z: err %v, output %q", err, out.String())
	}

	if _, mounted := c.Mounts.Lookup("DUA6"); mounted {
		t.Errorf("DUA6 mounted after Ctrl/Z")
	}

	// In a procedure, the parameter is missing.
	proc := filepath.Join(t.TempDir(), "mount.com")
	if err := os.WriteFile(proc, []byte("$ MOUNT DUA6\n$ WRITE SYS$OUTPUT \"Done\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	c.In = strings.NewReader("\"" + dsk + "\"\n")

	if err := d.Dispatch(`@"` + proc + `"`); err != nil && !errors.Is(err, vmserrors.New(vmserrors.CLI_INSFPRM)) {
		t.Errorf("@MOUNT.COM: err %v", err)
	}

	if want := "%DCL-W-INSFPRM, missing command parameters - supply all required parameters\nDone\n"; out.String() != want {
		t.Errorf("@MOUNT.COM: output %q, want %q", out.String(), want)
	}
}
