package testproc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunPattern(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"TestX":          "^TestX$",
		"TestX/mode":     "^TestX$/^mode$",
		"TestX/a.b/c(d)": `^TestX$/^a\.b$/^c\(d\)$`,
	}

	for name, want := range cases {
		if got := runPattern(name); got != want {
			t.Errorf("runPattern(%q) = %q, want %q", name, got, want)
		}
	}
}

// TestIsolatedChdir changes the directory and the environment, which only
// a test in a process of its own may do beside parallel tests.
func TestIsolatedChdir(t *testing.T) {
	if Isolate(t) {
		return
	}

	if !InChild() {
		t.Fatal("the body ran in the parent")
	}

	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("TESTPROC_PROBE", "yes")

	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	want, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(here); got != want {
		t.Fatalf("in %s, want %s", got, want)
	}
}

// TestIsolatedSubtests isolates each subtest, not the test.
func TestIsolatedSubtests(t *testing.T) {
	Parallel(t)

	for _, name := range []string{"one", "two words", "three"} {
		t.Run(name, func(t *testing.T) {
			if Isolate(t) {
				return
			}

			t.Setenv("TESTPROC_PROBE", name)
		})
	}
}

// TestIsolatedSkip skips in the child, which the parent reports as its
// own skip.
func TestIsolatedSkip(t *testing.T) {
	if Isolate(t) {
		return
	}

	t.Skip("skipped on purpose")
}

func TestMain(m *testing.M) {
	os.Exit(Main(m))
}

// TestSharedOnce has two isolated subtests ask for one value: it is
// computed once, in whichever asks first, and both get it.
func TestSharedOnce(t *testing.T) {
	Parallel(t)

	counter := filepath.Join(os.Getenv(sharedVariable), "computed")

	t.Run("group", func(t *testing.T) {
		for _, name := range []string{"first", "second"} {
			t.Run(name, func(t *testing.T) {
				if Isolate(t) {
					return
				}

				got := Shared(t, "probe", func() (string, bool) {
					f, err := os.OpenFile(counter, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
					if err != nil {
						t.Fatal(err)
					}

					defer f.Close()

					_, _ = f.WriteString("x")

					return "the value", true
				})

				if got != "the value" {
					t.Fatalf("Shared = %q", got)
				}
			})
		}
	})

	if InChild() {
		return
	}

	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "x" {
		t.Fatalf("computed %d times, want once", len(data))
	}
}
