package rms

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/ods2/filespec"
)

// This file implements the file-name rules of docs/PHASE-27.md's "File
// specifications: host files and ODS-2 volumes": how a command that takes
// file names (MACRO now, LINK later) decides whether a name means a file
// on the host or a file on a mounted ODS-2 volume. The rules live here so
// every such command agrees, and because only this package can parse and
// resolve a VMS file specification.
//
// A name is classified by its syntax first:
//
//  1. An explicit /HOST (the caller's host argument) means a host path,
//     whatever the name looks like.
//  2. A name that is obviously VMS syntax means a file on a mounted volume:
//     one with a bracketed directory ("[DIR.SUB]" or "<DIR>"), or a
//     "name:" prefix whose name is at least two characters long, and in
//     either case no "/" or "\". A logical name prefix ("SRC:HELLO.MAR")
//     must translate to a mounted device; if it doesn't, that's an error,
//     not a quiet fall back to the host.
//  3. A name that is obviously a host path means a host file: one with a
//     "/" or "\", or a one-letter Windows drive prefix ("C:x").
//  4. Anything else (a bare "HELLO.MAR") means a file in the default
//     directory of a mounted volume when a SET DEFAULT onto one is in
//     effect, and a host file otherwise. A name given alongside another
//     file (an object named with /OBJECT=) instead follows that file: the
//     same side, and on a volume the same device and directory.

// FileLocation is where a file name refers to.
type FileLocation struct {
	// Host is true for a host file and false for a file on a mounted
	// volume.
	Host bool

	// Name is the host path, or the file specification to resolve on a
	// mounted volume.
	Name string
}

// String returns the location's name.
func (l FileLocation) String() string {
	return l.Name
}

// nameKind is what a file name's syntax alone says about it.
type nameKind int

const (
	nameBare   nameKind = iota // neither obviously VMS nor obviously host
	nameVolume                 // obviously a VMS file specification
	nameHost                   // obviously a host path
)

// classifyName applies rules 2 and 3 above to name.
func classifyName(name string) nameKind {
	if strings.ContainsAny(name, `/\`) {
		return nameHost
	}

	if hasBracketDirectory(name) {
		return nameVolume
	}

	switch i := strings.IndexByte(name, ':'); {
	case i >= 2:
		return nameVolume
	case i == 1:
		return nameHost // a Windows drive letter
	}

	return nameBare
}

// hasBracketDirectory reports whether name has a directory in square or
// angle brackets.
func hasBracketDirectory(name string) bool {
	for _, pair := range []string{"[]", "<>"} {
		if open := strings.IndexByte(name, pair[0]); open >= 0 && strings.IndexByte(name[open:], pair[1]) > 0 {
			return true
		}
	}

	return false
}

// Locate decides where name refers to, by the rules above. host is an
// explicit /HOST on the name. A VMS specification must resolve to a
// mounted device (a *NotMountedError otherwise); whether the file exists
// isn't checked.
func (s *Session) Locate(name string, host bool) (FileLocation, error) {
	if name == "" {
		return FileLocation{}, fmt.Errorf("rms: no file name given")
	}

	if host {
		return FileLocation{Host: true, Name: name}, nil
	}

	switch classifyName(name) {
	case nameHost:
		return FileLocation{Host: true, Name: name}, nil

	case nameVolume:
		if err := s.checkMounted(name); err != nil {
			return FileLocation{}, err
		}

		return FileLocation{Name: name}, nil
	}

	if s.DefaultOnVolume() {
		return FileLocation{Name: name}, nil
	}

	return FileLocation{Host: true, Name: name}, nil
}

// LocateRelated is Locate for a name given alongside another file,
// related, which should be the location of the file actually found (as
// ReadRecordFile returns it). Rules 1 to 3 are unchanged, but a bare name
// follows related: on the host it's in related's directory, and on a
// volume it takes related's device and directory.
func (s *Session) LocateRelated(name string, host bool, related FileLocation) (FileLocation, error) {
	if host || classifyName(name) != nameBare {
		return s.Locate(name, host)
	}

	if related.Host {
		return FileLocation{Host: true, Name: filepath.Join(filepath.Dir(related.Name), name)}, nil
	}

	specs, err := expandSpec(s.Logicals, related.Name, s.Default)
	if err != nil {
		return FileLocation{}, err
	}

	base := filespec.Spec{Device: specs[0].Spec.Device, Dirs: specs[0].Spec.Dirs}

	spec, err := filespec.Parse(name, base)
	if err != nil {
		return FileLocation{}, fmt.Errorf("rms: %q: %w", name, err)
	}

	return FileLocation{Name: spec.String()}, nil
}

// DefaultOnVolume reports whether a SET DEFAULT onto a mounted volume is
// in effect: SYS$DISK is defined, and some element of it is a mounted
// device.
func (s *Session) DefaultOnVolume() bool {
	if s.Logicals == nil {
		return false
	}

	if _, err := s.Logicals.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0); err != nil {
		return false
	}

	defaults, err := defaultSpecs(s.Logicals, s.Default)
	if err != nil {
		return false
	}

	for _, d := range defaults {
		if _, ok := s.Mounts.Lookup(d.Device); ok {
			return true
		}
	}

	return false
}

// checkMounted reports a *NotMountedError unless some translation of the
// specification text names a mounted device.
func (s *Session) checkMounted(text string) error {
	specs, err := expandSpec(s.Logicals, text, s.Default)
	if err != nil {
		return err
	}

	for _, r := range specs {
		if _, ok := s.Mounts.Lookup(r.Spec.Device); ok {
			return nil
		}
	}

	if specs[0].Spec.Device == "" {
		return fmt.Errorf("rms: %s: no device given, and no default device", text)
	}

	return &NotMountedError{Device: specs[0].Spec.Device}
}
