package dclcmd

import (
	"bytes"
	"errors"
	"io/fs"
	"time"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
)

// This file is what the rest of govax reaches the system volume by
// (docs/PHASE-87 - system volume.md): the two logical names govax defines
// for it (Decision 5), and its files as an fs.FS for govax's resolver
// (Decision 4).

// The logical names govax defines when it mounts the system volume, in the
// system table at executive mode as VMS's boot defines its own. STARTUP.COM
// defines the rest (SYS$SYSTEM, SYS$LIBRARY, ...) from SystemDeviceName.
const (
	// SystemDeviceName is the system volume's device ("_DUA0:").
	SystemDeviceName = "SYS$SYSDEVICE"

	// SystemRootName is the volume's [SYSTEM] directory. VMS's is a rooted
	// directory (DUA0:[SYS0.], so that SYS$SYSROOT:[SYSEXE] is
	// DUA0:[SYS0.SYSEXE]); govax's RMS has no rooted directories, so this
	// is a plain one, and SYS$SYSROOT:[SYSLIB] isn't [SYSTEM.SYSLIB].
	SystemRootName = "SYS$SYSROOT"
)

// defineSystemNames defines SystemDeviceName and SystemRootName for the
// system volume on device. Each is terminal: its equivalence is a device's
// physical name, not translated again.
func (c *Context) defineSystemNames(device string) {
	physical := "_" + device + ":"

	for name, value := range map[string]string{
		SystemDeviceName: physical,
		SystemRootName:   physical + sysvol.Root,
	} {
		eqv := []lnm.Equivalence{{Value: value, Attrs: lnm.AttrTerminal}}
		_, _ = c.Logicals.Define(lnm.SystemTableName, name, lnm.Executive, 0, eqv)
	}
}

// SystemFS returns the system volume's files for govax's resolver
// (respath.Resolver.Volume): each name in sysvol.Resolved opens the
// volume's copy, as text with lines ended by line feeds, and every other
// name, or every name when no system volume is mounted, isn't there. It is
// read when a file is opened, so it follows the volume's current contents.
func (c *Context) SystemFS() fs.FS {
	return systemFS{c: c}
}

// systemFS is SystemFS's fs.FS.
type systemFS struct {
	c *Context
}

// Open opens name's copy on the system volume (fs.FS).
func (s systemFS) Open(name string) (fs.File, error) {
	spec, ok := sysvol.Resolved[name]
	device := s.c.Mounts.System()

	if !ok || device == "" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	records, _, err := s.c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: device + ":" + spec}, rms.TextRecords)

	var notFound *rms.NotFoundError

	switch {
	case errors.As(err, &notFound):
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	case err != nil:
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	return &memoryFile{name: name, Reader: bytes.NewReader(sysvol.Join(records, sysvol.Text))}, nil
}

// memoryFile is an fs.File whose contents are in memory.
type memoryFile struct {
	*bytes.Reader
	name string
}

func (f *memoryFile) Stat() (fs.FileInfo, error) { return memoryInfo{f}, nil }
func (f *memoryFile) Close() error               { return nil }

// memoryInfo is a memoryFile's fs.FileInfo.
type memoryInfo struct{ f *memoryFile }

func (i memoryInfo) Name() string       { return i.f.name }
func (i memoryInfo) Size() int64        { return i.f.Size() }
func (i memoryInfo) Mode() fs.FileMode  { return 0o444 }
func (i memoryInfo) ModTime() time.Time { return time.Time{} }
func (i memoryInfo) IsDir() bool        { return false }
func (i memoryInfo) Sys() any           { return nil }
