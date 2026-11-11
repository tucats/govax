// Package respath implements the file-lookup policy docs/planning/PHASE-15.md's first
// sub-phase describes: resolving a name that may be unqualified (typed on a
// command line or named by a startup script, e.g. "console.dcl" or
// "kernel.asm") by trying, in order, (1) the name exactly as given, relative
// to the process's current working directory or as an absolute path, (2)
// each of a configured list of search directories (the "-path" flag's -PATH-
// style directory list), (3) the system volume's copy of the file, when
// there is one (docs/planning/PHASE-87 - system volume.md, Decision 4), and (4) an
// embedded filesystem fallback so a standalone govax binary with no search
// directories configured still finds its required startup files.
//
// A nil *Resolver is a legal, useful zero value: every method behaves as a
// plain, unwrapped os.ReadFile/os.Open call, with no search directories and
// no embedded fallback. This lets existing callers/tests that construct a
// Console without assigning a Resolver keep working exactly as before.
package respath

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Resolver holds one file-lookup policy: a list of directories searched in
// order after the literal as-given attempt, and an optional fallback
// filesystem (typically an embed.FS) tried last.
type Resolver struct {
	Dirs []string

	// Volume, if not nil, is the system volume's files by the names the
	// fallback has them under ("console.dcl" for SYS$MANAGER:CONSOLE.DCL),
	// tried after Dirs and before Fallback: the user's copies, which come
	// before govax's own.
	Volume fs.FS

	Fallback fs.FS
}

// Origin is where a Resolver found a file.
type Origin int

const (
	// Host is a host file: the name as given, or in a search directory.
	Host Origin = iota

	// Volume is the system volume's copy (Resolver.Volume).
	Volume

	// Embedded is the fallback's, govax's own built-in copy.
	Embedded
)

// VolumeError is a failure reading the system volume's copy of a file,
// other than its not being there: a damaged volume, say. A caller that can
// do without the volume's copy may read the fallback's instead.
type VolumeError struct {
	Name string
	Err  error
}

func (e *VolumeError) Error() string {
	return fmt.Sprintf("%s on the system volume: %v", e.Name, e.Err)
}

func (e *VolumeError) Unwrap() error { return e.Err }

// New returns a Resolver searching dirs (in order) and falling back to
// fallback (nil for no fallback).
func New(dirs []string, fallback fs.FS) *Resolver {
	return &Resolver{Dirs: append([]string(nil), dirs...), Fallback: fallback}
}

// WithDir returns a Resolver that additionally searches dir, ahead of r's
// own search directories — used to look alongside a file that referenced
// this name (e.g. an assembler .INCLUDE resolved relative to the including
// file's own directory) without losing the wider search list or fallback.
func (r *Resolver) WithDir(dir string) *Resolver {
	dirs := []string{dir}

	if r == nil {
		return &Resolver{Dirs: dirs}
	}

	return &Resolver{Dirs: append(dirs, r.Dirs...), Volume: r.Volume, Fallback: r.Fallback}
}

// NotFoundError reports every location a Resolver tried before giving up.
type NotFoundError struct {
	Name  string
	Tried []string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s: not found (tried %s)", e.Name, strings.Join(e.Tried, ", "))
}

// Open resolves name and opens it, trying the name as given, then each
// search directory, then the system volume, then the fallback filesystem.
func (r *Resolver) Open(name string) (fs.File, error) {
	f, _, err := r.open(name)

	return f, err
}

// OpenFrom is Open, also saying where the file was found.
func (r *Resolver) OpenFrom(name string) (fs.File, Origin, error) {
	return r.open(name)
}

// open is Open's search, returning the file's origin with it.
func (r *Resolver) open(name string) (fs.File, Origin, error) {
	var tried []string

	if f, err := os.Open(name); err == nil {
		return f, Host, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, Host, err
	} else {
		tried = append(tried, name)
	}

	if r != nil {
		for _, dir := range r.Dirs {
			p := filepath.Join(dir, name)
			if f, err := os.Open(p); err == nil {
				return f, Host, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, Host, err
			} else {
				tried = append(tried, p)
			}
		}

		if r.Volume != nil {
			if f, err := r.Volume.Open(name); err == nil {
				return f, Volume, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, Volume, &VolumeError{Name: name, Err: err}
			} else {
				tried = append(tried, "system volume:"+name)
			}
		}

		if r.Fallback != nil {
			if f, err := r.Fallback.Open(name); err == nil {
				return f, Embedded, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, Embedded, err
			} else {
				tried = append(tried, "embedded:"+name)
			}
		}
	}

	return nil, Host, &NotFoundError{Name: name, Tried: tried}
}

// ReadFile resolves name and returns its full contents, using the same
// search order as Open.
func (r *Resolver) ReadFile(name string) ([]byte, error) {
	data, _, err := r.ReadFileFrom(name)

	return data, err
}

// ReadFileFrom is ReadFile, also saying where the file was found.
func (r *Resolver) ReadFileFrom(name string) ([]byte, Origin, error) {
	f, origin, err := r.open(name)
	if err != nil {
		return nil, origin, err
	}

	defer f.Close()

	data, err := io.ReadAll(f)

	return data, origin, err
}

// ReadEmbedded returns the fallback's copy of name, govax's own, whatever
// the other places hold: what a caller uses when the copy it found can't
// be used (a system volume's grammar that doesn't parse).
func (r *Resolver) ReadEmbedded(name string) ([]byte, error) {
	if r == nil || r.Fallback == nil {
		return nil, &NotFoundError{Name: name, Tried: []string{"embedded:" + name}}
	}

	return fs.ReadFile(r.Fallback, name)
}
