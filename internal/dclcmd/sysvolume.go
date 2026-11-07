package dclcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/hostlock"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sysvol"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file mounts the system volume (docs/PHASE-87 - system volume.md):
// the container govax mounts every time it starts, before its startup
// procedure runs, holding the files internal/sysvol lists. It is made the
// first time, from govax's built-in files; a second session finds it in
// use and mounts it read-only; and a volume that can't be used is a
// warning, after which govax starts from its built-in files as it did
// before there was a system volume.

// DefaultSystemFile is the system volume's container when the
// configuration and the command line name none: one per user, beside
// govax's other files in the profile directory (Decision 0).
const DefaultSystemFile = "~/.govax/govax-system.dsk"

// NoSystemVolume, as the container's name, starts govax with no system
// volume at all (--system-file none).
const NoSystemVolume = "none"

// SystemVolume describes the system volume MountSystemVolume mounts. Each
// field may be empty, taking its default.
type SystemVolume struct {
	// File is the container's host path: DefaultSystemFile if empty, no
	// volume if NoSystemVolume. A leading "~/" is the user's home
	// directory.
	File string

	// Device is the device it is mounted on, sysvol.DefaultDevice (DUA0)
	// if empty; a controller with no unit (DUA) is its next free unit
	// (ResolveUnit).
	Device string

	// Type is the disk type a new container is made as, and the device is
	// defined as: sysvol.DefaultType (RD54) if empty.
	Type string

	// Initialize is --system-initialize: the volume is made again even if
	// it exists, the old container kept beside it as FILE.old.
	Initialize bool
}

// SystemVolumeResult says what MountSystemVolume did.
type SystemVolumeResult struct {
	// Mounted is whether a system volume is mounted; the rest are
	// meaningful only if it is.
	Mounted bool

	// Device is the device it is mounted on ("DUA0").
	Device string

	// Path is its container's host path.
	Path string

	// Writable is whether it is mounted for writing; it isn't when
	// another session holds it (Shared) or its file can only be read.
	Writable bool

	// Shared is whether another govax session holds it (Decision 7).
	Shared bool

	// Created is whether it was made by this call (for the first time,
	// or again by Initialize).
	Created bool
}

// MountSystemVolume mounts the system volume v describes, making it
// first if it doesn't exist or v.Initialize asks, and records it as the
// system volume (rms.MountTable.SetSystem). Mounted for writing, the
// volume is filled or refreshed with this govax's files
// (refreshSystemVolume). Nothing it finds stops
// govax's startup: a volume that can't be used is MOUNT_NOSYSVOL's
// warning, and the result says no volume is mounted. Informational
// messages are shown only when the console is verbose.
func (c *Context) MountSystemVolume(v SystemVolume) SystemVolumeResult {
	file := strings.TrimSpace(v.File)
	if file == "" {
		file = DefaultSystemFile
	}

	if strings.EqualFold(file, NoSystemVolume) {
		return SystemVolumeResult{}
	}

	path, err := expandHome(file)
	if err != nil {
		return c.noSystemVolume(file, err)
	}

	devType := strings.ToUpper(strings.TrimSpace(v.Type))
	if devType == "" {
		devType = sysvol.DefaultType
	}

	typeOptions, knownType := iodev.DiskTypeOptions(devType)
	if !knownType {
		return c.noSystemVolume(path, fmt.Errorf("unknown device type %s", devType))
	}

	deviceText := strings.TrimSpace(v.Device)
	if deviceText == "" {
		deviceText = sysvol.DefaultDevice
	}

	device := c.ResolveUnit(deviceText)

	if _, mounted := c.Mounts.Lookup(device); mounted {
		return c.noSystemVolume(path, fmt.Errorf("%s: already mounted", device))
	}

	open, err := c.openSystemContainer(path, devType, v.Initialize)
	if err != nil {
		return c.noSystemVolume(path, err)
	}

	result := SystemVolumeResult{Device: device, Path: path, Shared: open.shared, Created: open.created}

	// The device takes the type unless something already defined it, as
	// the default volume's does (defvolume.go).
	_, predefined := c.Devices.Find(device)

	// Mounted for writing unless another session holds it; a container
	// that can only be read is mounted read-only.
	writeLocked := false
	result.Writable = !open.shared

	if err := c.Mount(device, path, result.Writable); err != nil {
		if !result.Writable || c.Mount(device, path, false) != nil {
			_ = open.lock.Close()

			return c.noSystemVolume(path, err)
		}

		result.Writable, writeLocked = false, true
	}

	if label, _ := c.Mounts.VolumeLabel(device); !strings.EqualFold(strings.TrimSpace(label), sysvol.Label) {
		_ = c.Dismount(device)
		_ = open.lock.Close()

		return c.noSystemVolume(path, fmt.Errorf("volume label is %s, not %s", strings.TrimSpace(label), sysvol.Label))
	}

	if d, found := c.Devices.Find(device); found && !predefined {
		d.DevType = typeOptions.DevType
		d.Cylinders = typeOptions.Cylinders
		d.Sectors = typeOptions.Sectors
		d.MaxBlock = typeOptions.MaxBlock
	}

	// The lock is held while the volume is mounted for writing, and let go
	// when it's dismounted; a read-only mount needs none.
	if result.Writable && open.lock != nil {
		_ = c.Mounts.Hold(device, open.lock)
	} else {
		_ = open.lock.Close()
	}

	c.Mounts.SetSystem(device)
	c.defineSystemNames(device)

	result.Mounted = true

	if open.oldPath != "" {
		c.info(vmserrors.New(vmserrors.MOUNT_SYSINIT, path, open.oldPath))
	} else if open.created {
		c.info(vmserrors.New(vmserrors.MOUNT_CREATED, path, devType, sysvol.Label))
	}

	if open.shared {
		c.info(vmserrors.New(vmserrors.MOUNT_SYSREADONLY, path))
	}

	c.reportMounted(device, writeLocked)

	// A volume mounted for writing is filled, if it's new, or brought up
	// to date with this govax's files (Decision 3); a read-only one is
	// used as it is.
	if result.Writable {
		c.refreshSystemVolume(device, open.created)
	}

	return result
}

// refreshSystemVolume fills or refreshes the system volume on device
// (sysvol.Refresh) and reports what it did: each file the user changed
// that a newer govax copy didn't replace, always, since each is said only
// once; and, when the console is verbose, how many files were written on
// a volume that was there already (a new one's CREATED says enough). A
// failure is SYSFILES's warning, and the volume stays mounted as it is.
func (c *Context) refreshSystemVolume(device string, created bool) {
	report, err := sysvol.Refresh(c.systemFiles(device))
	if err != nil {
		c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.MOUNT_SYSFILES, err, device+":"))

		return
	}

	for _, change := range report.Changes {
		if change.Notify {
			c.Printf("%%%s\n", vmserrors.New(vmserrors.MOUNT_SYSKEPT, device+":"+change.File.Spec()))
		}

		// A help library's modules the user changed, each by name
		// (docs/PHASE-90 - VMS help.md, Decision 7).
		for _, module := range change.KeptModules {
			c.Printf("%%%s\n", vmserrors.New(vmserrors.MOUNT_SYSKEPT, device+":"+change.File.Spec()+"("+module+")"))
		}
	}

	if written := report.Count(sysvol.Written) + report.Count(sysvol.Updated); written > 0 && !created {
		files := "1 file"
		if written > 1 {
			files = fmt.Sprintf("%d files", written)
		}

		c.info(vmserrors.New(vmserrors.MOUNT_SYSUPDATED, files, device+":"))
	}
}

// systemContainer is what openSystemContainer found or made.
type systemContainer struct {
	// lock is the host lock taken on the container, or nil when another
	// session holds it (shared).
	lock *hostlock.Lock

	// shared is whether another govax session holds it.
	shared bool

	// created is whether the container is new.
	created bool

	// oldPath is where --system-initialize put the old container, or "".
	oldPath string
}

// openSystemContainer finds the system volume's container at path, or
// makes it (of type devType, labeled sysvol.Label), and takes the host
// lock on it. With initialize, an existing container no other session
// holds is renamed FILE.old (replacing an older one) and a new one made;
// one another session holds is left alone, shared, as a second session
// can't make the volume again under the first.
//
// A new container is locked as soon as it's made. Two sessions starting
// at the same moment could both find no container, and the second's
// creation would replace the first's; the lock then makes one of them
// the second session, so neither writes the other's.
func (c *Context) openSystemContainer(path, devType string, initialize bool) (systemContainer, error) {
	var found systemContainer

	_, err := os.Stat(path)

	switch {
	case err == nil:
		lock, err := hostlock.TryLock(path)

		switch {
		case errors.Is(err, hostlock.ErrLocked):
			found.shared = true

			return found, nil
		case err != nil:
			return found, err
		}

		if !initialize {
			found.lock = lock

			return found, nil
		}

		_ = lock.Close()

		found.oldPath = path + ".old"
		if err := os.Rename(path, found.oldPath); err != nil {
			return found, err
		}

	case !errors.Is(err, os.ErrNotExist):
		return found, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return found, err
	}

	if err := c.InitializeContainer(path, 0, sysvol.Label, 0, devType); err != nil {
		return found, err
	}

	found.created = true

	lock, err := hostlock.TryLock(path)

	switch {
	case errors.Is(err, hostlock.ErrLocked):
		found.shared = true
	case err != nil:
		return found, err
	default:
		found.lock = lock
	}

	return found, nil
}

// noSystemVolume reports a system volume that can't be used, and why,
// and returns the result of mounting none.
func (c *Context) noSystemVolume(path string, cause error) SystemVolumeResult {
	c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.MOUNT_NOSYSVOL, cause, path))

	return SystemVolumeResult{}
}

// systemFiles returns the system volume on device as internal/sysvol's
// Volume: its files through RMS, by their full specifications.
func (c *Context) systemFiles(device string) sysvol.Volume {
	return systemFiles{c: c, device: device + ":"}
}

// systemFiles is sysvol.Volume over RMS, for the volume on device.
type systemFiles struct {
	c      *Context
	device string // "DUA0:"
}

// CreateDirectory makes dir on the volume, unless it is there already.
func (v systemFiles) CreateDirectory(dir string) error {
	if exists, err := v.c.Mounts.DirectoryExists(strings.TrimSuffix(v.device, ":"), dir); err == nil && exists {
		return nil
	}

	_, err := v.c.ContainerSession.CreateDirectory(v.device+dir, rms.CreateDirectoryOptions{})

	return err
}

// WriteFile writes a new version of spec: a text file as CREATE makes
// one, a binary one as LINK writes an image.
func (v systemFiles) WriteFile(spec string, records [][]byte, f sysvol.Format) error {
	kind := rms.TextRecords
	if f == sysvol.Binary {
		kind = rms.ImageBlocks
	}

	_, err := v.c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: v.device + spec}, kind, records)

	return err
}

// ReadFile reads the highest version of spec, if there is one.
func (v systemFiles) ReadFile(spec string) ([][]byte, bool, error) {
	records, _, err := v.c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: v.device + spec}, rms.TextRecords)

	var notFound *rms.NotFoundError

	switch {
	case errors.As(err, &notFound):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}

	return records, true, nil
}
