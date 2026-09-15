// Package respath implements the file-lookup policy docs/PHASE-15.md's first
// sub-phase describes: resolving a name that may be unqualified (typed on a
// command line or named by a startup script, e.g. "vax.init" or
// "kernel.asm") by trying, in order, (1) the name exactly as given, relative
// to the process's current working directory or as an absolute path, (2)
// each of a configured list of search directories (the "-path" flag's -PATH-
// style directory list), and (3) an embedded filesystem fallback so a
// standalone govax binary with no search directories configured still finds
// its required startup files.
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
	Dirs     []string
	Fallback fs.FS
}

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
	var fallback fs.FS
	if r != nil {
		dirs = append(dirs, r.Dirs...)
		fallback = r.Fallback
	}
	return &Resolver{Dirs: dirs, Fallback: fallback}
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
// search directory, then the fallback filesystem.
func (r *Resolver) Open(name string) (fs.File, error) {
	var tried []string

	if f, err := os.Open(name); err == nil {
		return f, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	} else {
		tried = append(tried, name)
	}

	if r != nil {
		for _, dir := range r.Dirs {
			p := filepath.Join(dir, name)
			if f, err := os.Open(p); err == nil {
				return f, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			} else {
				tried = append(tried, p)
			}
		}

		if r.Fallback != nil {
			if f, err := r.Fallback.Open(name); err == nil {
				return f, nil
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			} else {
				tried = append(tried, "embedded:"+name)
			}
		}
	}

	return nil, &NotFoundError{Name: name, Tried: tried}
}

// ReadFile resolves name and returns its full contents, using the same
// search order as Open.
func (r *Resolver) ReadFile(name string) ([]byte, error) {
	f, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
