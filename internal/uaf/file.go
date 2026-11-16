package uaf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/hostlock"
)

// DefaultPath is the file's place when vax.authorization.file names
// none: beside the system volume, in the profile directory. A leading
// "~/" is the user's home directory (ExpandHome).
const DefaultPath = "~/.govax/sysuaf.json"

// fileMode is the host file's mode: only its owner may read or write
// it, since it holds password hashes.
const fileMode = 0o600

// ParseError is a file Load couldn't read: the file, the line (0 when
// the error can't be placed), and why.
type ParseError struct {
	Path string
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("authorization file %s, line %d: %v", e.Path, e.Line, e.Err)
	}

	return fmt.Sprintf("authorization file %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

// ExpandHome replaces a leading "~/" (or a path that is just "~") with
// the user's home directory.
func ExpandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, strings.TrimPrefix(path[1:], "/")), nil
}

// Load reads and checks the file at path. A missing file is an error
// fs.ErrNotExist matches; one that isn't JSON, has a field govax doesn't
// know, or fails Check is a *ParseError naming the line.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	f, err := Parse(data)
	if err != nil {
		return nil, &ParseError{Path: path, Line: errorLine(data, err), Err: err}
	}

	return f, nil
}

// Parse reads and checks a file's text.
func Parse(data []byte) (*File, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	f := &File{}
	if err := dec.Decode(f); err != nil {
		return nil, err
	}

	if dec.More() {
		return nil, errors.New("there is more after the file's object")
	}

	// A file written by hand may leave the lists out.
	if f.Accounts == nil {
		f.Accounts = []Record{}
	}

	if f.Identifiers == nil {
		f.Identifiers = []Identifier{}
	}

	if err := f.migrate(); err != nil {
		return nil, err
	}

	if err := f.Check(); err != nil {
		return nil, err
	}

	return f, nil
}

// errorLine finds the line of the file's text that err is about: the
// offset encoding/json reports, or the first line with the bad value's
// text in quotes. 0 when neither says.
func errorLine(data []byte, err error) int {
	var (
		syntax *json.SyntaxError
		typ    *json.UnmarshalTypeError
		value  *valueError
		offset = int64(-1)
	)

	switch {
	case errors.As(err, &syntax):
		offset = syntax.Offset
	case errors.As(err, &typ):
		offset = typ.Offset
	case errors.As(err, &value):
		if i := bytes.Index(data, []byte(`"`+value.text+`"`)); i >= 0 {
			offset = int64(i) + 1
		}
	default:
		// "json: unknown field \"name\"" names the field.
		if _, field, ok := strings.Cut(err.Error(), "unknown field "); ok {
			if i := bytes.Index(data, []byte(field)); i >= 0 {
				offset = int64(i) + 1
			}
		}
	}

	if offset < 0 {
		return 0
	}

	offset = min(offset, int64(len(data)))

	return bytes.Count(data[:offset], []byte("\n")) + 1
}

// Marshal returns the file's text: indented JSON, ending with a newline.
func (f *File) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(data, '\n'), nil
}

// Save checks the file and writes it to path whole: to a temporary file
// in the same directory, then renamed over the old one, so a reader sees
// the old file or the new, never part of one. The file's mode is 0600.
func (f *File) Save(path string) error {
	if err := f.Check(); err != nil {
		return err
	}

	data, err := f.Marshal()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	name := tmp.Name()

	defer func() { _ = os.Remove(name) }() // nothing left behind if the rename doesn't happen

	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()

		return err
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()

		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(name, path)
}

// lockPath is the file whose host lock guards path's changes. The lock
// can't be on the file itself, which Save replaces with a new one.
func lockPath(path string) string { return path + ".lock" }

// Update reads the file at path, calls change on it, and writes it back
// if change returns nil, all while holding the file's lock, so that
// sessions changing the file one after another each see the others'
// changes. A missing file is change's to make: it gets Default's.
func Update(path string, change func(*File) error) error {
	lock, err := hostlock.Wait(lockPath(path))
	if err != nil {
		return err
	}

	defer func() { _ = lock.Close() }()

	f, err := Load(path)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		f = Default()
	case err != nil:
		return err
	}

	if err := change(f); err != nil {
		return err
	}

	return f.Save(path)
}

// Ensure makes the file at path, with Default's accounts, if it doesn't
// exist, making its directory (mode 0700) too, and reports whether it
// made it. A file that exists is read and checked: an error means govax
// shouldn't start on it.
func Ensure(path string) (bool, error) {
	if _, err := Load(path); !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false, err
	}

	made := false

	err := Update(path, func(*File) error {
		// Another session may have made it since the Load above; Update
		// read it if so, and writing it back changes nothing.
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			made = true
		}

		return nil
	})

	return made, err
}
