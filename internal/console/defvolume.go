package console

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file mounts the default volume: an ODS-2 container govax mounts
// (creating it first if need be) when it starts, and makes the default
// device and directory, so file commands work on the volume rather than
// the host's file system. cmd/govax fills a DefaultVolume in from the
// vax.default.volume.* configuration settings; with no container file
// configured, nothing happens and names go to the host as before.

// Defaults for the DefaultVolume fields a configuration leaves empty.
const (
	DefaultVolumeLabel     = "WORK"
	DefaultVolumeDevice    = "DUA0:"
	DefaultVolumeType      = "RD54"
	DefaultVolumeDirectory = "[WORK]"
)

// DefaultVolume describes the volume MountDefaultVolume mounts. Every
// field but File may be empty, taking the default above.
type DefaultVolume struct {
	// File is the container's host path. A leading "~/" is the user's
	// home directory.
	File string

	// Label is the volume label: a new container's label, and the label
	// an existing container's volume must have. An empty Label means
	// "WORK" for a new container and no check on an existing one.
	Label string

	// Device is the device the volume is mounted on, such as "DUA0:".
	Device string

	// Type is the disk type (RD54, RA81, ...) that sizes a new container
	// and that a device not yet defined is defined as.
	Type string

	// Directory is the default directory, such as "[WORK]", which a new
	// container is given. One that isn't on the volume falls back to
	// the master file directory, [000000].
	Directory string
}

// MountDefaultVolume mounts v's container, creating and initializing it
// if it doesn't exist, and does a SET DEFAULT to its device and
// directory. A volume that can't be used is a warning, not a failure:
// startup goes on without it. Informational messages (created, mounted)
// are shown only when the console is verbose, so a one-shot command's
// output isn't cluttered with them.
func (c *Console) MountDefaultVolume(v DefaultVolume) {
	if strings.TrimSpace(v.File) == "" {
		return
	}

	path, err := expandHome(strings.TrimSpace(v.File))
	if err != nil {
		c.warnDefaultVolume(v.File, err)

		return
	}

	labelRequired := strings.TrimSpace(v.Label) != ""
	label := strings.ToUpper(strings.TrimSpace(v.Label))

	if label == "" {
		label = DefaultVolumeLabel
	}

	device := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(v.Device), ":"))
	if device == "" {
		device = strings.TrimSuffix(DefaultVolumeDevice, ":")
	}

	devType := strings.ToUpper(strings.TrimSpace(v.Type))
	if devType == "" {
		devType = DefaultVolumeType
	}

	typeOptions, knownType := iodev.DiskTypeOptions(devType)
	if !knownType {
		c.warnDefaultVolume(path, fmt.Errorf("unknown device type %s", devType))

		return
	}

	dir := strings.TrimSpace(v.Directory)
	if dir == "" {
		dir = DefaultVolumeDirectory
	} else if !strings.HasPrefix(dir, "[") && !strings.HasPrefix(dir, "<") {
		dir = "[" + strings.ToUpper(dir) + "]"
	}

	if _, mounted := c.Mounts.Lookup(device); mounted {
		c.warnDefaultVolume(path, fmt.Errorf("%s: already mounted", device))

		return
	}

	// A container that doesn't exist yet is created, with the configured
	// directory in it once it's mounted.
	created := false

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			c.warnDefaultVolume(path, err)

			return
		}

		if err := c.InitializeContainer(path, 0, label, 0, devType); err != nil {
			c.warnDefaultVolume(path, err)

			return
		}

		created = true

		c.info(vmserrors.New(vmserrors.MOUNT_CREATED, path, devType, label))
	} else if err != nil {
		c.warnDefaultVolume(path, err)

		return
	}

	// The device takes the configured type unless something (an earlier
	// DEFINE/DEVICE) has already defined it. Mount defines it as a plain
	// disk, so the type is filled in once the mount has succeeded.
	_, predefined := c.Devices.Find(device)

	// Mount for writing if possible; a container that can only be read
	// (a read-only host file, say) is mounted read-only instead.
	writable := true

	if err := c.Mount(device, path, true); err != nil {
		if roErr := c.Mount(device, path, false); roErr != nil {
			c.warnDefaultVolume(path, err)

			return
		}

		writable = false
	}

	if d, found := c.Devices.Find(device); found && !predefined {
		d.DevType = typeOptions.DevType
		d.Cylinders = typeOptions.Cylinders
		d.Sectors = typeOptions.Sectors
		d.MaxBlock = typeOptions.MaxBlock
	}

	volLabel, _ := c.Mounts.VolumeLabel(device)
	volLabel = strings.ToUpper(strings.TrimSpace(volLabel))

	if labelRequired && volLabel != label {
		_ = c.Dismount(device)

		c.warnDefaultVolume(path, fmt.Errorf("volume label is %s, not %s", volLabel, label))

		return
	}

	c.reportMounted(device, !writable)

	if created && writable {
		if _, err := c.ContainerSession.CreateDirectory(device+":"+dir, rms.CreateDirectoryOptions{}); err != nil {
			c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CREATE_DIRNOTCRE, err, device+":"+dir))
		}
	}

	if exists, err := c.Mounts.DirectoryExists(device, dir); err != nil || !exists {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.MOUNT_NODIR, dir, device+":"))

		dir = "[000000]"
	}

	if err := c.SetDefault(device + ":" + dir); err != nil {
		c.Printf("%%%s\n", err)
	}
}

// warnDefaultVolume reports a default volume that can't be used, and why.
func (c *Console) warnDefaultVolume(path string, cause error) {
	c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.MOUNT_NODEFAULT, cause, path))
}

// info shows an informational message when the console is verbose.
func (c *Console) info(msg error) {
	if c.Verbose {
		c.Printf("%%%s\n", msg)
	}
}

// expandHome replaces a leading "~/" (or a path that is just "~") with
// the user's home directory, as a shell would.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, strings.TrimPrefix(path, "~")), nil
}
