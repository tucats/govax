package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// for88Dir is round 2's directory, from this package's.
var for88Dir = filepath.Join("..", "..", "testdata", "for88")

// runFor88OnFORRTL runs round 2 of the Phase 88 probe on VMS's FORRTL.EXE over
// govax (docs/PHASE-88 - keyed index files.md, subtask 12), and returns its
// output. It skips without FORRTL.EXE on a local volume.
func runFor88OnFORRTL(t *testing.T) string {
	t.Helper()

	disk, file := findFORRTL(t)
	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "for88.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 FOR88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch(`MOUNT/NOWRITE DUA5 "` + disk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch("DEFINE/NOLOG FORRTL DUA5:" + file)

	src, err := filepath.Abs(filepath.Join(for88Dir, "for88.mar"))
	if err != nil {
		t.Fatal(err)
	}

	opt := filepath.Join(t.TempDir(), "for88.opt")
	if err := os.WriteFile(opt, []byte("FORRTL/SHAREABLE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dispatch(`COPY "` + src + `"/HOST FOR88.MAR`)
	dispatch(`COPY "` + opt + `"/HOST FOR88.OPT`)

	if out := dispatch("MACRO/NOLIST FOR88"); out != "" {
		t.Fatalf("MACRO: %s", out)
	}

	if out := dispatch("LINK/NOMAP FOR88,FOR88/OPTIONS"); out != "" {
		t.Fatalf("LINK: %s", out)
	}

	return dispatch("RUN FOR88")
}

// TestFor88Rehearsal runs the probe, as the VAX will, to its end. With
// FOR88_WRITE set it keeps the output in that file.
func TestFor88Rehearsal(t *testing.T) {
	out := runFor88OnFORRTL(t)

	if path := os.Getenv("FOR88_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if !strings.Contains(out, "##### the end of the cases") {
		t.Errorf("the probe stopped before its end; its last lines:\n%s", lastLines(out, 30))
	}
}

// lastLines is the last n lines of a text.
func lastLines(text string, n int) string {
	lines := strings.Split(text, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n")
}
