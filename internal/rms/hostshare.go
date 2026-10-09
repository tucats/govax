package rms

import (
	"errors"
	"io/fs"
	"os"
)

// Host file sharing (docs/PHASE-49.md, subtask 11): the arbitration
// sharing.go applies to a volume's files, for files on the host that a
// running program opens (the console's EXE$OPEN shim; RMS reaches only
// volumes). The rule is the same, the File Applications guide's per
// operation one: an open asks for the operations its access (FAB$B_FAC)
// names and lets others do those its sharing (FAB$B_SHR) names, and may
// join the file's other openers only if each allows the other's. A host
// file has no file ID, so a file is known by its identity on the host
// (os.SameFile: the same device and inode, whatever path named it).
//
// Only govax's own opens take part: the host, and govax's console
// commands (which run while no program does), aren't arbitrated.

// HostOpeners is the list of host files programs have open, with each
// one's openers. Its zero value is ready to use. Like MountTable, it is
// system state and needs no locking (one goroutine runs every process).
type HostOpeners struct {
	files []*hostOpenFile
}

// hostOpenFile is one host file with openers.
type hostOpenFile struct {
	info    fs.FileInfo
	openers []*opener
}

// HostClaim is one open's place among a host file's openers; Release
// takes it out.
type HostClaim struct {
	t *HostOpeners
	f *hostOpenFile
	o *opener
}

// ErrHostFileLocked is Claim's refusal: another open's access or sharing
// conflicts (RMS$_FLK's condition, for a host file).
var ErrHostFileLocked = errors.New("rms: host file locked by another opener")

// Claim asks to open the host file at path with access fac and sharing
// shr (FAB$B_FAC and FAB$B_SHR bits; shr 0 takes the RMS defaults). It
// is made before the file is opened, so a refused open changes nothing
// (a truncating open doesn't truncate). A file that doesn't exist yet
// can't conflict: the claim is attached to it once it's open (Attach).
func (t *HostOpeners) Claim(path string, fac, shr byte) (*HostClaim, error) {
	c := &HostClaim{t: t, o: &opener{access: accessOps(fac), share: sharingOps(fac, shr)}}

	info, err := os.Stat(path)
	if err != nil {
		return c, nil
	}

	f := t.find(info)
	if f != nil {
		for _, other := range f.openers {
			if !c.o.compatible(*other) {
				return nil, ErrHostFileLocked
			}
		}
	}

	c.attach(f, info)

	return c, nil
}

// Attach records the claim against the file now open as file, if Claim
// couldn't (the open created it).
func (c *HostClaim) Attach(file *os.File) {
	if c == nil || c.f != nil || c.o == nil {
		return
	}

	if info, err := file.Stat(); err == nil {
		c.attach(c.t.find(info), info)
	}
}

// attach adds the claim's opener to f, or to a new entry for the file
// info describes if f is nil.
func (c *HostClaim) attach(f *hostOpenFile, info fs.FileInfo) {
	if f == nil {
		f = &hostOpenFile{info: info}
		c.t.files = append(c.t.files, f)
	}

	f.openers = append(f.openers, c.o)
	c.f = f
}

// find returns the entry for the file info describes, or nil.
func (t *HostOpeners) find(info fs.FileInfo) *hostOpenFile {
	for _, f := range t.files {
		if os.SameFile(f.info, info) {
			return f
		}
	}

	return nil
}

// Release takes the claim out of its file's openers (at close); the
// file leaves the list with its last opener. Releasing again does
// nothing.
func (c *HostClaim) Release() {
	if c == nil || c.f == nil {
		return
	}

	f := c.f
	c.f = nil

	for i, o := range f.openers {
		if o == c.o {
			f.openers = append(f.openers[:i], f.openers[i+1:]...)

			break
		}
	}

	if len(f.openers) > 0 {
		return
	}

	for i, other := range c.t.files {
		if other == f {
			c.t.files = append(c.t.files[:i], c.t.files[i+1:]...)

			break
		}
	}
}

// Count reports how many host files have openers (for tests and
// displays).
func (t *HostOpeners) Count() int { return len(t.files) }
