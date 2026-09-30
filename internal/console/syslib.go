package console

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/rms"
)

// This file finds the VMS library files LINK and MACRO use (IMAGELIB.OLB,
// STARLET.OLB, shareable images, STARLET.MLB), as VMS finds them through
// SYS$LIBRARY and SYS$SHARE (docs/PHASE-28.md subtask 9). Each is looked
// for through its logical name on a mounted volume first, then in the host
// library directory: Console.HostLibrary, or else the vax.library setting,
// or else the older vax.link.library, which named LINK's directory before
// MACRO shared it.

// Settings naming the host library directory.
const (
	librarySetting       = "vax.library"
	legacyLibrarySetting = "vax.link.library"
)

// hostLibraryDir is the host directory to look in for VMS library files,
// or "" for none.
func (c *Console) hostLibraryDir() string {
	if c.HostLibrary != "" {
		return c.HostLibrary
	}

	if dir := settings.Get(librarySetting); dir != "" {
		return dir
	}

	return settings.Get(legacyLibrarySetting)
}

// readLibraryFile reads a VMS library file: logical:file on a mounted
// volume, or file in the host library directory. It returns nil data, and
// no error, when neither has it.
func (c *Console) readLibraryFile(logical, file string) ([]byte, string, error) {
	spec := logical + ":" + file

	data, found, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: spec})

	var (
		notMounted *rms.NotMountedError
		notFound   *rms.NotFoundError
	)

	switch {
	case err == nil:
		return data, found.Name, nil
	case !errors.As(err, &notMounted) && !errors.As(err, &notFound):
		return nil, spec, err
	}

	dir := c.hostLibraryDir()
	if dir == "" {
		return nil, "", nil
	}

	for _, name := range []string{file, strings.ToLower(file)} {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err == nil {
			return data, path, nil
		}

		if !errors.Is(err, fs.ErrNotExist) {
			return nil, path, err
		}
	}

	return nil, "", nil
}
