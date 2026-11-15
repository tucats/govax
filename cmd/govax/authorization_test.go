package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/uaf"
)

// withAuthorizationFile gives one test an authorization file of its own,
// put back when it ends.
func withAuthorizationFile(t *testing.T, path string) {
	t.Helper()

	saved := authorizationFile
	authorizationFile = path

	t.Cleanup(func() { authorizationFile = saved })
}

// TestRun_authorizationFile: the first start makes the file with SYSTEM
// and DEFAULT, mode 0600, and the System reads it; a file govax can't
// read stops startup with the file's name and the line.
func TestRun_authorizationFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "sysuaf.json")
	withAuthorizationFile(t, path)

	var buf bytes.Buffer
	if err := run(nil, 0, 0, &buf, emptyStdin(), nil); err != nil {
		t.Fatalf("run: %v\n%s", err, buf.String())
	}

	f, err := uaf.Load(path)
	if err != nil {
		t.Fatalf("the file wasn't made: %v", err)
	}

	if _, ok := f.Find("SYSTEM"); !ok {
		t.Error("no SYSTEM")
	}

	if _, ok := f.Find("DEFAULT"); !ok {
		t.Error("no DEFAULT")
	}

	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}

	data, _ := os.ReadFile(path)
	bad := strings.Replace(string(data), `"TMPMBX"`, `"NOSUCHPRIV"`, 1)

	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	err = run(nil, 0, 0, &buf, emptyStdin(), nil)

	var pe *uaf.ParseError
	if !errors.As(err, &pe) || pe.Line == 0 || !strings.Contains(err.Error(), path) {
		t.Errorf("run on a bad file = %v, want a ParseError naming %s and a line", err, path)
	}

	if strings.Contains(buf.String(), "STARTUP") {
		t.Errorf("startup went on:\n%s", buf.String())
	}
}
