package rms

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/lnm"
)

// This file implements the file-name rules of docs/planning/PHASE-27.md's "File
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
//     Before rules 2 to 4, a name with no punctuation that is a logical
//     name stands for its translation, as RMS takes it, and the rules
//     place that (locateLogical).
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

	// Physical, in a location a reader found on a volume (ReadRawFile,
	// AccessImage), is the file's full specification with its physical
	// device, where Name shows a concealed device's logical name: what
	// VMS's image activator records as an image's name (JPI$_IMAGNAME).
	Physical string
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

// classify is classifyName, except that a one-letter device that's a
// logical name is one, not a Windows drive letter.
func (s *Session) classify(name string) nameKind {
	kind := classifyName(name)

	if kind == nameHost && s.Logicals != nil && len(name) > 1 && name[1] == ':' && !strings.ContainsAny(name, `/\`) {
		if _, err := s.Logicals.Translate(lnm.FileDevName, strings.ToUpper(name[:1]), lnm.User, 0); err == nil {
			return nameVolume
		}
	}

	return kind
}

// BareName reports whether name's syntax alone says nothing about where it
// is (rule 4 above): no directory, no device or logical name, no host
// path. A command may treat such a name more freely than an explicit one,
// as ASM and LOAD do: a bare name not found on the default volume is
// looked for among the console's host and built-in files.
func BareName(name string) bool {
	return classifyName(name) == nameBare
}

// HostPath reports whether name's syntax alone says it is a host path
// (rule 3 above): one with "/" or "\\", or a Windows drive letter that
// isn't a one-letter logical name. SET DEFAULT refuses such a name, since
// the default directory can only be on a volume.
func (s *Session) HostPath(name string) bool {
	return s.classify(name) == nameHost
}

// DefaultOnHost reports whether no default device is set (SYS$DISK has no
// translation), as when govax runs with neither a system volume nor a work
// volume. A bare name is then a host file in the host's current directory
// (rule 4), which SHOW DEFAULT says rather than showing a directory alone.
func (s *Session) DefaultOnHost() bool {
	if s.Logicals == nil {
		return true
	}

	_, err := s.Logicals.Translate(lnm.FileDevName, sysDiskName, lnm.User, 0)

	return err != nil
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
//
// A name placed on the host is one the session's process is about to
// use, so Locate asks the session's HostAccess whether the process may
// (docs/planning/PHASE-97 - host file access.md, Decision 4): with the
// configuration key vax.host.sysprv on, only a process holding SYSPRV
// may. A refusal is a *DeniedError with Host set, so every command that
// shows a protection refusal shows this one the same way. A caller that
// only asks which side a name is on, and opens nothing, uses Side.
func (s *Session) Locate(name string, host bool) (FileLocation, error) {
	loc, err := s.Side(name, host)
	if err != nil || !loc.Host {
		return loc, err
	}

	if err := s.checkHostAccess(loc.Name); err != nil {
		return FileLocation{}, err
	}

	return loc, nil
}

// checkHostAccess asks the session's HostAccess about the host file
// name: nil when there is no HostAccess or it allows the access, a
// *DeniedError with Host set when it doesn't.
func (s *Session) checkHostAccess(name string) error {
	if s.HostAccess == nil || s.HostAccess() {
		return nil
	}

	return &DeniedError{Name: name, Host: true}
}

// Side is Locate without the question of whether the process may use a
// host file: where name refers to, for a caller that only needs to know
// which side a name is on (a volume's or the host's) and opens nothing
// itself.
func (s *Session) Side(name string, host bool) (FileLocation, error) {
	if name == "" {
		return FileLocation{}, fmt.Errorf("rms: no file name given")
	}

	if host {
		return FileLocation{Host: true, Name: name}, nil
	}

	if loc, ok := s.locateLogical(name); ok {
		return loc, nil
	}

	switch s.classify(name) {
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

// locateLogical is Locate for a name with no punctuation that is a
// logical name: RMS translates such a name whole, before it applies any
// default (the Guide to OpenVMS File Applications, 6.2.2), so it stands
// for its translation, not for a file of its own name. Where the name is
// follows from the translation, not from where a bare name would be (a
// host file when the default directory is on the host).
//
// A single translation is returned as itself (as its user sees it, a
// concealed device by its logical name), so that a default type a caller
// adds (@'s .COM, MACRO's .MAR) goes on the file the name stands for, not
// on the logical name, which with a type added would no longer translate:
// DEFINE SYS$SYLOGIN SYS$MANAGER:SYLOGIN.COM, then @SYS$SYLOGIN. A search
// list is left as the name, on the side of its first element, for RMS to
// search (a default type a caller adds as text is then lost, unconfirmed
// on VMS, where the default name applies to each element). A
// process-permanent file's name (SYS$INPUT) is left alone, as is a name
// whose translation Locate can't place (a terminal), which goes on as a
// bare name.
func (s *Session) locateLogical(name string) (FileLocation, bool) {
	if s.Logicals == nil || !plainName(name) || !s.definesName(strings.ToUpper(name)) {
		return FileLocation{}, false
	}

	fs, err := translateSpec(s.Logicals, name)
	if err != nil || len(fs) == 0 || fs[0].Permanent != nil {
		return FileLocation{}, false
	}

	if len(fs) > 1 {
		if s.classify(fs[0].Spec) != nameVolume || s.checkMounted(name) != nil {
			return FileLocation{}, false
		}

		return FileLocation{Name: name}, true
	}

	eqv := fs[0].Display

	if plainName(eqv) {
		// A name that translates to another bare name, a file's name, is
		// that name, wherever a bare name goes; one that is still a
		// logical name (through a terminal translation) is left alone.
		if s.definesName(strings.ToUpper(eqv)) {
			return FileLocation{}, false
		}

		return FileLocation{Name: eqv, Host: !s.DefaultOnVolume()}, true
	}

	loc, err := s.Locate(eqv, false)
	if err != nil {
		return FileLocation{}, false
	}

	return loc, true
}

// plainName reports whether name has no punctuation: only the characters
// a logical name in a file specification may have (the User's Manual,
// 11.3.3), so that RMS takes it whole as a logical name.
func plainName(name string) bool {
	if name == "" {
		return false
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '$' || c == '_' || c == '-') {
			return false
		}
	}

	return true
}

// LocateRelated is Locate for a name given alongside another file,
// related, which should be the location of the file actually found (as
// ReadRecordFile returns it). Rules 1 to 3 are unchanged, but a bare name
// follows related: on the host it's in related's directory, and on a
// volume it takes related's device and directory.
func (s *Session) LocateRelated(name string, host bool, related FileLocation) (FileLocation, error) {
	if host || s.classify(name) != nameBare {
		return s.Locate(name, host)
	}

	if related.Host {
		return FileLocation{Host: true, Name: filepath.Join(filepath.Dir(related.Name), name)}, nil
	}

	// A bare name that is a logical name stands for its translation,
	// which the related file's device and directory don't override: RMS
	// translates the name before it applies the related file's fields
	// (an options file's FORRTL/SHAREABLE, with FORRTL defined). A
	// single translation is returned as itself, so that a default type
	// the caller adds goes on it, not on the logical name.
	if fs, err := translateSpec(s.Logicals, name); err == nil && len(fs) > 0 && !strings.EqualFold(fs[0].Spec, name) {
		if len(fs) == 1 {
			return s.Locate(fs[0].Spec, host)
		}

		return s.Locate(name, host)
	}

	specs, err := expandSpec(s.Logicals, related.Name, s.Default)
	if err != nil {
		return FileLocation{}, err
	}

	spec, sts := scanSpec(name)
	if sts != 0 {
		return FileLocation{}, &NameError{Spec: name, Status: sts}
	}

	spec.Device, spec.Dirs = specs[0].Spec.Device, specs[0].Spec.Dirs

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
