package jobqueue

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

// DefaultPath is the file's place when vax.queue.file names none: beside
// the authorization file, in the profile directory (Decision 3). A
// leading "~/" is the user's home directory.
const DefaultPath = "~/.govax/qman.json"

// fileMode is the host file's mode: only its owner may read or write it,
// as the authorization file's.
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
		return fmt.Sprintf("queue database %s, line %d: %v", e.Path, e.Line, e.Err)
	}

	return fmt.Sprintf("queue database %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

// Load reads and checks the file at path. A missing file is an error
// fs.ErrNotExist matches; one that isn't JSON, has a field govax doesn't
// know, or fails Check is a *ParseError naming the line.
func Load(path string) (*Database, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	d, err := Parse(data)
	if err != nil {
		return nil, &ParseError{Path: path, Line: errorLine(data, err), Err: err}
	}

	return d, nil
}

// Parse reads and checks a file's text.
func Parse(data []byte) (*Database, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	d := &Database{}
	if err := dec.Decode(d); err != nil {
		return nil, err
	}

	if dec.More() {
		return nil, errors.New("there is more after the file's object")
	}

	// A file written by hand may leave the lists out.
	if d.Queues == nil {
		d.Queues = []*Queue{}
	}

	if d.Jobs == nil {
		d.Jobs = []*Job{}
	}

	if hasNil(d.Queues) || hasNil(d.Jobs) {
		return nil, errors.New("a queue or job is null")
	}

	if err := d.Check(); err != nil {
		return nil, err
	}

	d.sortQueues()
	d.sortJobs()

	return d, nil
}

// hasNil reports whether a list read from the file has a null in it.
func hasNil[T any](list []*T) bool {
	for _, p := range list {
		if p == nil {
			return true
		}
	}

	return false
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
func (d *Database) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(data, '\n'), nil
}

// Save checks the database and writes it to path whole: to a temporary
// file in the same directory, then renamed over the old one, so a reader
// sees the old file or the new, never part of one. The file's mode is
// 0600.
func (d *Database) Save(path string) error {
	d.sortQueues()
	d.sortJobs()

	if err := d.Check(); err != nil {
		return err
	}

	data, err := d.Marshal()
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

// managerPath is the file whose host lock the session running the queue
// manager holds (LockManager).
func managerPath(path string) string { return path + ".manager" }

// Update reads the file at path, calls change on it, and writes it back
// if change returns nil, all while holding the file's lock, so that
// sessions changing the file one after another each see the others'
// changes. A missing file is an empty database (New) for change to fill.
// change's error is Update's, and nothing is written.
func Update(path string, change func(*Database) error) error {
	lock, err := hostlock.Wait(lockPath(path))
	if err != nil {
		return err
	}

	defer func() { _ = lock.Close() }()

	d, err := Load(path)

	switch {
	case errors.Is(err, fs.ErrNotExist):
		d = New()
	case err != nil:
		return err
	}

	if err := change(d); err != nil {
		return err
	}

	return d.Save(path)
}

// Read reads the file at path as Update would, under its lock but
// changing nothing: a missing file is an empty database. It is how a
// session not running the manager shows queues (Decision 4).
func Read(path string) (*Database, error) {
	lock, err := hostlock.Wait(lockPath(path))
	if err != nil {
		return nil, err
	}

	defer func() { _ = lock.Close() }()

	d, err := Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		return New(), nil
	}

	return d, err
}

// Ensure checks the file at path, making its directory (mode 0700) if it
// isn't there. A missing file is fine, an empty database until something
// is saved; one that exists is read and checked, and an error means govax
// shouldn't start on it.
func Ensure(path string) error {
	if _, err := Load(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return os.MkdirAll(filepath.Dir(path), 0o700)
}

// LockManager takes the lock the session running the queue manager holds
// for as long as it runs (Decision 4), on a file beside path that it
// makes if need be. Another session holding it is hostlock.ErrLocked:
// that session is the batch initiator, and this one may only show queues.
// The lock goes with Close, or when the session ends however it ends.
func LockManager(path string) (*hostlock.Lock, error) {
	name := managerPath(path)

	f, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, fileMode)
	if err != nil {
		return nil, err
	}

	_ = f.Close()

	return hostlock.TryLock(name)
}
