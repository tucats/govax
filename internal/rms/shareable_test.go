package rms

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestImageLocatorBuiltin checks rule 4: an image built into govax is
// found last, after the host's files, and only for its own name.
func TestImageLocatorBuiltin(t *testing.T) {
	image := []byte("FTNRTL's bytes")
	builtin := func(name string) ([]byte, bool) {
		if name == "FTNRTL" {
			return image, true
		}

		return nil, false
	}

	dir := t.TempDir()
	l := ImageLocator{Library: dir, Builtin: builtin}

	found, err := l.Find("ftnrtl")
	if err != nil || string(found.Builtin) != string(image) || found.Location.Name != "FTNRTL.EXE (built into govax)" {
		t.Errorf("built-in: %+v, %v", found, err)
	}

	var notFound *ImageNotFoundError
	if _, err := l.Find("OTHER"); !errors.As(err, &notFound) {
		t.Errorf("OTHER: %v, want ImageNotFoundError", err)
	}

	// A file in the host library directory comes first.
	path := filepath.Join(dir, "ftnrtl.exe")
	if err := os.WriteFile(path, []byte("the user's"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err = l.Find("FTNRTL")
	if err != nil || found.Builtin != nil || !found.Location.Host || !strings.EqualFold(filepath.Base(found.Location.Name), "ftnrtl.exe") {
		t.Errorf("host file: %+v, %v", found, err)
	}
}
