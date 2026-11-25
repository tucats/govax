// Package testproc runs a test in a process of its own, so that tests
// which change what a whole process shares (the current directory, the
// environment, gopackages' settings store) can still run in parallel with
// every other test.
//
// Go runs a package's tests in one process. A test that calls t.Chdir,
// t.Setenv, or settings.Set changes that process for every test running
// beside it, so it can't call t.Parallel. Isolate gets around that: in
// the test process it marks the test parallel and runs the same test
// binary again as a child process asked to run only that test, then
// reports what the child reported. In the child, the test runs as usual,
// alone in its own process, so what it changes there touches nothing
// else.
//
// A test uses it as its first statement:
//
//	func TestSomething(t *testing.T) {
//		if testproc.Isolate(t) {
//			return
//		}
//		... the test, which may call t.Chdir, t.Setenv, settings.Set ...
//	}
//
// It works for subtests too (an Isolate call at the top of a t.Run
// function): the child is then asked for that one subtest. Inside a child
// Isolate never starts another process, so a test and its subtests may
// both call it.
//
// Tests in separate processes can't share a Go variable, so a costly
// result several tests use (a whole probe round's log, say) would be
// computed once per process. Shared keeps such a result in a directory
// that every process of one `go test` run sees, made by Main, which a
// package that uses Shared calls from its TestMain.
//
// It is for tests only, and imports only the standard library and
// internal/hostlock.
package testproc

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tucats/govax/internal/hostlock"
)

// childVariable is the environment variable that tells a test process it
// is a child Isolate started; its value is the test's name.
const childVariable = "GOVAX_TEST_ISOLATED"

// sharedVariable names the directory Shared keeps its results in, made by
// Main in the test process and passed on to every child.
const sharedVariable = "GOVAX_TEST_SHARED"

// Main runs a package's tests (m.Run) and returns their exit code, for
// TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testproc.Main(m)) }
//
// In the test process it first makes the directory Shared uses, and
// removes it afterwards; in a child it only runs the tests.
func Main(m *testing.M) int {
	if InChild() || os.Getenv(sharedVariable) != "" {
		return m.Run()
	}

	dir, err := os.MkdirTemp("", "govax-test-shared-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "testproc: %v\n", err)

		return 1
	}

	defer os.RemoveAll(dir)

	// Children inherit the environment (Isolate passes os.Environ on).
	if err := os.Setenv(sharedVariable, dir); err != nil {
		fmt.Fprintf(os.Stderr, "testproc: %v\n", err)

		return 1
	}

	return m.Run()
}

// local is Shared's store when no Main made a directory: within one
// process only.
var local = struct {
	sync.Mutex
	values map[string]string
}{values: map[string]string{}}

// Shared returns the value compute makes for key, computed once in a whole
// `go test` run, whichever of its processes asks first; the others wait for
// it. compute reports whether its value may be kept: when not (its test
// failed, say), nothing is kept, and the next to ask computes it again.
// key must be usable as a file name.
func Shared(t *testing.T, key string, compute func() (string, bool)) string {
	t.Helper()

	dir := os.Getenv(sharedVariable)
	if dir == "" {
		// One process: a lock held while computing would make a parallel
		// test wait just as the file lock does.
		local.Lock()
		defer local.Unlock()

		if v, ok := local.values[key]; ok {
			return v
		}

		v, keep := compute()
		if keep {
			local.values[key] = v
		}

		return v
	}

	lock, err := hostlock.Wait(filepath.Join(dir, key+".lock"))
	if err != nil {
		t.Fatalf("testproc: %v", err)
	}

	defer lock.Close()

	path := filepath.Join(dir, key)
	if data, err := os.ReadFile(path); err == nil {
		return string(data)
	}

	v, keep := compute()
	if keep {
		if err := os.WriteFile(path, []byte(v), 0o600); err != nil {
			t.Fatalf("testproc: %v", err)
		}
	}

	return v
}

// InChild reports whether this process is a child Isolate started.
func InChild() bool {
	return os.Getenv(childVariable) != ""
}

// Parallel is t.Parallel, except in a child, where it does nothing. A test
// whose subtests call Isolate uses it in place of t.Parallel: Go refuses
// t.Chdir and t.Setenv in a test any of whose ancestors called t.Parallel,
// and in the child the subtest's ancestors run too.
func Parallel(t *testing.T) {
	t.Helper()

	if !InChild() {
		t.Parallel()
	}
}

// Isolate runs t in a child process and reports true when it did, in which
// case the caller returns at once: the child has run the test's body, and
// its result is now t's. It reports false in a child, where the caller
// goes on to run the body itself.
func Isolate(t *testing.T) bool {
	t.Helper()

	if InChild() {
		return false
	}

	t.Parallel()

	args := []string{
		"-test.run=" + runPattern(t.Name()),
		"-test.count=1",
		"-test.v", // so the child's result can be read: PASS, FAIL, or SKIP
	}

	if testing.Short() {
		args = append(args, "-test.short")
	}

	// The child has what's left of the parent's time, less a little, so
	// that a child that runs out reports its own stack rather than the
	// parent's timeout killing them both.
	if deadline, ok := t.Deadline(); ok {
		if left := time.Until(deadline) - 5*time.Second; left > 0 {
			args = append(args, "-test.timeout="+left.String())
		}
	}

	// Pass on the flags that change what a test does.
	for _, name := range []string{"test.failfast", "test.benchtime"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != f.DefValue {
			args = append(args, "-"+name+"="+f.Value.String())
		}
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), childVariable+"="+t.Name())

	var out bytes.Buffer

	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	text := out.String()

	switch {
	case strings.Contains(text, "testing: warning: no tests to run"):
		t.Fatalf("the child process found no test named %s:\n%s", t.Name(), text)

	case err != nil || !strings.Contains(text, "--- PASS: "+t.Name()) && !strings.Contains(text, "--- SKIP: "+t.Name()):
		t.Fatalf("in its own process (%v):\n%s", err, text)

	case strings.Contains(text, "--- SKIP: "+t.Name()+" "):
		t.Skipf("skipped in its own process:\n%s", skipReasons(text))

	case testing.Verbose():
		t.Logf("in its own process:\n%s", text)
	}

	return true
}

// runPattern is the -test.run pattern that selects exactly the test or
// subtest named name ("TestX/mode" makes "^TestX$/^mode$").
func runPattern(name string) string {
	parts := strings.Split(name, "/")
	for i, part := range parts {
		parts[i] = "^" + regexp.QuoteMeta(part) + "$"
	}

	return strings.Join(parts, "/")
}

// skipReasons is what a child's verbose output says about why it skipped:
// the lines between the RUN line and the SKIP line, which hold the
// t.Skip text.
func skipReasons(text string) string {
	var b strings.Builder

	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "=== ") || trimmed == "PASS" || strings.HasPrefix(trimmed, "ok ") {
			continue
		}

		fmt.Fprintln(&b, line)
	}

	return b.String()
}
