package console_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/testproc"
)

// TestScratchRecipe runs the scratch volume as govax supplies it
// (docs/planning/PHASE-96 - RAM disk.md, Decision 9, made the default by
// the author): bootdata's SYSTARTUP_VMS.COM makes and mounts MDA0:, and
// its SYLOGIN.COM gives each account that logs in a directory there and
// points SYS$SCRATCH at it. Before the RAM disk exists, SYLOGIN.COM does
// nothing. Each account can write its own directory and not another's,
// and a subprocess sees the same SYS$SCRATCH.
func TestScratchRecipe(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	for _, key := range dclcmd.RAMDiskKeys() {
		old, had := settings.Get(key), settings.Exists(key)

		t.Cleanup(func() {
			if had {
				settings.Set(key, old)
			} else {
				_ = settings.Delete(key)
			}
		})

		_ = settings.Delete(key)
	}

	s := newLoginSession(t)

	t.Cleanup(func() {
		_ = s.c.Mounts.DismountAll()
		_ = s.c.Mounts.ReleaseMemory()
	})

	// The two procedures as govax supplies them, on DUA1 as the system
	// volume has them in SYS$MANAGER, SYLOGIN readable by everyone.
	dir := t.TempDir()

	for _, name := range []string{"systartup_vms.com", "sylogin.com"} {
		text, err := fs.ReadFile(bootdata.FS, name)
		if err != nil {
			t.Fatal(err)
		}

		host := filepath.Join(dir, name)
		if err := os.WriteFile(host, text, 0o644); err != nil {
			t.Fatal(err)
		}

		if out := s.dispatch(`COPY "` + host + `"/HOST DUA1:[000000]` + strings.ToUpper(name) + "/QUIET"); out != "" {
			t.Fatalf("COPY %s: %s", name, out)
		}
	}

	note := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(note, []byte("a note\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	steps := func(who string, list []struct{ line, want string }) {
		t.Helper()

		for _, step := range list {
			got := strings.TrimSpace(s.dispatch(step.line))

			if prefix, ok := strings.CutSuffix(step.want, "..."); ok {
				if !strings.HasPrefix(got, prefix) {
					t.Errorf("%s: %s:\n got %q\nwant %q...", who, step.line, got, prefix)
				}

				continue
			}

			if got != step.want {
				t.Errorf("%s: %s:\n got %q\nwant %q", who, step.line, got, step.want)
			}
		}
	}

	steps("setup", []struct{ line, want string }{
		{"SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) DUA1:[000000]SYLOGIN.COM", ""},
		{"DEFINE/SYSTEM/EXECUTIVE_MODE/NOLOG SYS$SYLOGIN DUA1:[000000]SYLOGIN.COM", ""},
	})

	// With no RAM disk, SYLOGIN.COM's lines do nothing.
	if out := s.logIn("UAF95"); strings.Contains(out, "%") {
		t.Errorf("UAF95's login with no RAM disk:\n%s", out)
	}

	steps("no RAM disk", []struct{ line, want string }{
		{`WRITE SYS$OUTPUT F$TRNLNM("SYS$SCRATCH")`, "UAF95$DISK:[UAF95]"},
	})

	// The startup procedure makes the scratch volume, silently.
	if out := s.logIn("SYSTEM"); strings.Contains(out, "%") {
		t.Errorf("SYSTEM's login with no RAM disk:\n%s", out)
	}

	steps("startup", []struct{ line, want string }{
		{"@DUA1:[000000]SYSTARTUP_VMS.COM", ""},
		{`WRITE SYS$OUTPUT F$GETDVI("MDA0","MNT"), " ", F$GETDVI("MDA0","VOLNAM"), " ", F$GETDVI("MDA0","MAXBLOCK")`,
			"TRUE SCRATCH 40000"},
	})

	// SYSTEM's login: its directory, and SYS$SCRATCH.
	if out := s.logIn("SYSTEM"); strings.Contains(out, "%") {
		t.Errorf("SYSTEM's login:\n%s", out)
	}

	steps("SYSTEM", []struct{ line, want string }{
		{`WRITE SYS$OUTPUT F$TRNLNM("SYS$SCRATCH")`, "MDA0:[SYSTEM]"},
		{`COPY "` + note + `"/HOST SYS$SCRATCH:SYSTEM.TMP/QUIET`, ""},
		{"DIRECTORY/NOHEAD/NOTRAIL MDA0:[SYSTEM]", "MDA0:[SYSTEM]SYSTEM.TMP;1"},
		{`SPAWN WRITE SYS$OUTPUT F$TRNLNM("SYS$SCRATCH")`, "%DCL-S-SPAWNED, process SYSTEM_1 spawned\n" +
			"%DCL-S-ATTACHED, terminal now attached to process SYSTEM_1\nMDA0:[SYSTEM]\n" +
			"%DCL-S-RETURNED, control returned to process SYSTEM"},
	})

	// UAF95's: a directory of its own, owned by it, that it can write and
	// SYSTEM's directory it can't.
	for round := 1; round <= 2; round++ {
		if out := s.logIn("UAF95"); strings.Contains(out, "%") {
			t.Errorf("UAF95's login %d:\n%s", round, out)
		}
	}

	steps("UAF95", []struct{ line, want string }{
		{`WRITE SYS$OUTPUT F$TRNLNM("SYS$SCRATCH")`, "MDA0:[UAF95]"},
		{`WRITE SYS$OUTPUT F$FILE_ATTRIBUTES("MDA0:[000000]UAF95.DIR","UIC")`, "[UAF95]"},
		{`WRITE SYS$OUTPUT F$FILE_ATTRIBUTES("MDA0:[000000]UAF95.DIR","PRO")`, "SYSTEM=RWED, OWNER=RWED, GROUP, WORLD"},
		{`COPY "` + note + `"/HOST SYS$SCRATCH:MINE.TMP/QUIET`, ""},
		{"DIRECTORY/NOHEAD/NOTRAIL MDA0:[UAF95]", "MDA0:[UAF95]MINE.TMP;1"},
		{`COPY "` + note + `"/HOST MDA0:[SYSTEM]THEIRS.TMP`, "%COPY-E-OPENOUT, ..."},
		{"DIRECTORY/NOHEAD/NOTRAIL MDA0:[SYSTEM]", "..."},
		{`SPAWN WRITE SYS$OUTPUT F$TRNLNM("SYS$SCRATCH")`, "%DCL-S-SPAWNED, process UAF95_1 spawned\n" +
			"%DCL-S-ATTACHED, terminal now attached to process UAF95_1\nMDA0:[UAF95]\n" +
			"%DCL-S-RETURNED, control returned to process UAF95"},
	})

	if got := s.dispatch("DIRECTORY/NOHEAD/NOTRAIL MDA0:[SYSTEM]"); strings.Contains(got, "THEIRS") {
		t.Errorf("UAF95 wrote in MDA0:[SYSTEM]:\n%s", got)
	}
}
