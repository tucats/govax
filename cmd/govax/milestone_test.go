package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/rms"
)

// Phase 48's acceptance test through govax's own command line (docs/
// PHASE-48 - LIB_SPAWN.md, subtask 6): the multiprocessing milestone,
// testdata/mp/msparent.mar and mschild.mar, assembled and linked by the
// macro and link subcommands onto the configured default volume, then
// run by the run subcommand, its command text after the image, under
// --instruction-limit as a guard against a hang. The console package's
// TestMilestone runs the same programs under several quanta.

// setConfig sets a configuration key for the length of the test.
func setConfig(t *testing.T, key, value string) {
	t.Helper()

	old, had := settings.Get(key), settings.Exists(key)

	t.Cleanup(func() {
		if had {
			settings.Set(key, old)
		} else {
			_ = settings.Delete(key)
		}
	})

	settings.Set(key, value)
}

func TestRun_milestone(t *testing.T) {
	dir := t.TempDir()
	disk := filepath.Join(dir, "work.dsk")

	setConfig(t, "vax.process.scheduler", "true")
	setConfig(t, "vax.default.volume.file", disk)

	// Each run's output, the default volume ([WORK] on DUA0, created by
	// the first run) mounted at its start and dismounted at its end.
	govax := func(command, commandText string) string {
		t.Helper()

		console.RunCommandLine = commandText
		t.Cleanup(func() { console.RunCommandLine = "" })

		var buf bytes.Buffer
		if err := run(nil, 50_000_000, 0, &buf, emptyStdin(), []string{command}); err != nil {
			t.Fatalf("%s: %v\n%s", command, err, buf.String())
		}

		return buf.String()
	}

	for _, name := range []string{"mschild", "msparent"} {
		src, err := os.ReadFile(filepath.Join("..", "..", "testdata", "mp", name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		mar, obj := filepath.Join(dir, name+".mar"), filepath.Join(dir, name+".obj")
		if err := os.WriteFile(mar, src, 0o644); err != nil {
			t.Fatal(err)
		}

		govax(macroCommand(mar, macroFlags{object: obj}), "")
		govax(linkCommand([]string{obj}, linkFlags{executable: "DUA0:[WORK]" + strings.ToUpper(name) + ".EXE"}), "")
	}

	for _, how := range []struct{ mode, first string }{
		{"CREPRC", "$CREPRC"},
		{"SPAWN", "LIB$SPAWN"},
	} {
		out := govax(runCommand("MSPARENT.EXE", runFlags{}), how.mode+" MSCHILD.EXE")

		var lines []string

		for _, line := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
			if strings.HasPrefix(line, "Parent:") || strings.HasPrefix(line, "Child:") {
				lines = append(lines, line)
			}
		}

		want := []string{"Parent: starting the child by " + how.first, "Child: running"}
		for n := 1; n <= 8; n++ {
			want = append(want, "Parent: round "+string(rune('0'+n))+" acknowledged")
		}

		want = append(want, "Child: done", "Parent: child ended with status 00000003",
			"Parent: SHARED.DAT has 8 records from the parent and 8 from the child")

		if strings.Join(lines, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s: output:\n%s\nwant:\n%s", how.mode, out, strings.Join(want, "\n"))
		}
	}

	// The volume, after both runs (whose second made new versions of the
	// three files): consistent.
	mounts := rms.NewMountTable()
	if err := mounts.Mount("DUA0", disk, true); err != nil {
		t.Fatal(err)
	}

	problems, err := mounts.VerifyVolume("DUA0")
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range problems {
		t.Errorf("volume: %s", p)
	}

	if err := mounts.DismountAll(); err != nil {
		t.Fatal(err)
	}
}
