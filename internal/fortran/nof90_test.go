package fortran_test

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/fortran/sysdef"
)

// The Fortran 90 program promises that nothing the compiler does to a VAX FORTRAN
// program changes (docs/planning/PHASE-99 - Fortran 90.md, Decision 16).
// TestNoF90Unchanged holds it to that promise: every fixed-form FORTRAN source
// under testdata (the corpus, the modules, the oracles' programs), and DUNGEON
// when its kit is to hand, is compiled in each of noF90Modes, and the SHA-256 of
// what came out (the MACRO file, then the messages) must be the one recorded in
// testdata/nof90.sum. The baseline was made by the compiler before Phase 99's
// subtask 2 changed anything; a subtask that means to change old programs'
// output must say so in its log and rewrite the file:
//
//	go test ./internal/fortran -run TestNoF90Unchanged -nof90-update
//
// A digest is kept rather than the MACRO files themselves, which would be
// megabytes; a failure names the source and mode, and compiling it by hand on
// two commits shows the difference.

// nof90Update rewrites testdata/nof90.sum from this compiler's output.
var nof90Update = flag.Bool("nof90-update", false, "rewrite testdata/nof90.sum")

// noF90Mode is a way the corpus is compiled, with its name in the baseline.
type noF90Mode struct {
	name string
	opts func(*fortran.Options)
}

// noF90Modes are the settings the corpus's tests compile it in.
var noF90Modes = []noF90Mode{
	{"noopt", func(*fortran.Options) {}},
	{"opt", func(o *fortran.Options) { o.Optimize = true }},
	{"recursive", func(o *fortran.Options) { o.Optimize = true; o.Recursive = true }},
}

// noF90Options are the fixed options every compilation starts from, so that the
// header doesn't change from run to run.
func noF90Options(name string) fortran.Options {
	return fortran.Options{
		Version:     "1.0-1",
		Compiled:    time.Date(2026, time.November, 17, 12, 0, 0, 0, time.UTC),
		CommandLine: "FORTRAN " + name,
	}
}

// hostIncludes is an Opener over host files: an INCLUDE'd file relative to the
// including file's directory (its Handle), as the FORTRAN command looks for it,
// with the type .for if it has none, and a module from govax's definition
// modules.
type hostIncludes struct{}

func (hostIncludes) OpenInclude(name string, from *source.File) (*source.File, error) {
	dir, _ := from.Handle.(string)
	path := filepath.Join(dir, name)

	if filepath.Ext(path) == "" {
		path += ".for"
	}

	lines, err := readSourceLines(path)
	if err != nil {
		return nil, err
	}

	return &source.File{Spec: filepath.Join(filepath.Dir(from.Spec), name), Lines: lines, Handle: filepath.Dir(path)}, nil
}

func (hostIncludes) OpenModule(_, module string, from *source.File) (*source.File, error) {
	if lines, ok := sysdef.Module(module); ok {
		return &source.File{Spec: "FORSYSDEF(" + strings.ToUpper(module) + ")", Lines: lines, Handle: from.Handle}, nil
	}

	return nil, errors.New("no module " + module)
}

// readSourceLines reads a host source file's lines without their terminators.
func readSourceLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), nil
}

// noF90Sources are the fixed-form sources to compile, each as a path relative to
// this package (so the messages name the same file on every machine), and the
// local DUNGEON kit's if it's there.
func noF90Sources(t *testing.T) []string {
	t.Helper()

	var paths []string

	root := filepath.Join("..", "..", "testdata")

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "disks":
			return filepath.SkipDir // local-only volumes and kits
		case d.IsDir() && d.Name() == "fortran90":
			// The Fortran 90 corpus (Phase 99, subtask 6), new with the
			// program: its fixed-form files were never compiled before it.
			return filepath.SkipDir
		case !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".for"):
			paths = append(paths, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	home, _ := os.UserHomeDir()
	kit := filepath.Join(home, "Documents", "Technical Doc", "VMS", "dungeon-3.2A")

	if _, err := os.Stat(filepath.Join(kit, "dungeon.f")); err == nil {
		for _, module := range []string{"dungeon", "game", "gdt", "objects", "parser", "rooms", "subr", "timefnc", "verbs"} {
			paths = append(paths, filepath.Join(kit, module+".f"))
		}
	}

	slices.Sort(paths)

	return paths
}

// noF90Key is a source's name in the baseline: its path under testdata, or
// DUNGEON/ and the kit's file name.
func noF90Key(path string) string {
	if rel, err := filepath.Rel(filepath.Join("..", "..", "testdata"), path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}

	return "DUNGEON/" + filepath.Base(path)
}

// noF90Digest compiles the source at path with opts and returns the SHA-256 of
// its MACRO file and messages.
func noF90Digest(t *testing.T, path string, opts fortran.Options) string {
	t.Helper()

	lines, err := readSourceLines(path)
	if err != nil {
		t.Fatal(err)
	}

	// A source is named by its key, so the MACRO file and the messages are the
	// same on every machine and don't show where the author keeps DUNGEON.
	name := strings.ToUpper(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	opts.Includes = hostIncludes{}

	// DUNGEON is built with /CONTINUATIONS=40, which some of its statements need.
	if strings.HasPrefix(noF90Key(path), "DUNGEON/") {
		opts.Continuations = 40
	}

	result := fortran.Compile(opts, fortran.File{Spec: noF90Key(path), Name: name, Lines: lines, Handle: filepath.Dir(path)})

	h := sha256.New()

	for _, line := range result.MACRO {
		fmt.Fprintln(h, line)
	}

	fmt.Fprintln(h, "--")

	for _, m := range result.Messages {
		fmt.Fprintln(h, m.String())
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestNoF90Unchanged compiles every fixed-form source in every mode and holds its
// output to the baseline.
func TestNoF90Unchanged(t *testing.T) {
	baselineFile := filepath.Join("testdata", "nof90.sum")

	got := map[string]string{}

	for _, path := range noF90Sources(t) {
		for _, mode := range noF90Modes {
			opts := noF90Options(filepath.Base(path))
			mode.opts(&opts)
			got[noF90Key(path)+" "+mode.name] = noF90Digest(t, path, opts)
		}
	}

	if *nof90Update {
		writeNoF90Baseline(t, baselineFile, got)

		return
	}

	data, err := os.ReadFile(baselineFile)
	if err != nil {
		t.Fatalf("%v (make it with -nof90-update)", err)
	}

	want := map[string]string{}

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("bad baseline line %q", line)
		}

		want[fields[0]+" "+fields[1]] = fields[2]
	}

	for key, digest := range got {
		switch w, ok := want[key]; {
		case !ok:
			t.Errorf("%s: not in the baseline (a new source? rewrite it with -nof90-update)", key)
		case w != digest:
			t.Errorf("%s: the compiler's output changed", key)
		}
	}

	// A source the baseline names that's gone is a change too, but for
	// DUNGEON's, whose kit isn't on every machine.
	for key := range want {
		if _, ok := got[key]; !ok && !strings.HasPrefix(key, "DUNGEON/") {
			t.Errorf("%s: in the baseline but not compiled (a source removed? rewrite it with -nof90-update)", key)
		}
	}
}

// writeNoF90Baseline writes the digests, sorted, one source and mode a line.
func writeNoF90Baseline(t *testing.T, file string, digests map[string]string) {
	t.Helper()

	keys := make([]string, 0, len(digests))
	for key := range digests {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	var b strings.Builder

	b.WriteString("# TestNoF90Unchanged's baseline: source, mode, and the SHA-256 of its MACRO\n")
	b.WriteString("# file and messages (docs/planning/PHASE-99 - Fortran 90.md, Decision 16).\n")

	for _, key := range keys {
		fmt.Fprintf(&b, "%s %s\n", key, digests[key])
	}

	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
