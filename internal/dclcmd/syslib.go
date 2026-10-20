package dclcmd

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
// STARLET.OLB, and the macro library), as VMS finds them through
// SYS$LIBRARY (docs/PHASE-28.md subtask 9); shareable images are found
// as RUN finds them (linksource.go's openSharedImage, rms.ImageLocator),
// the host library directory among the places. Each library is looked
// for through its logical name on a mounted volume first, then in the host
// library directory: Context.HostLibrary, or else the vax.library setting,
// or else the older vax.link.library, which named LINK's directory before
// MACRO shared it.

// Settings naming the host library directory.
const (
	LibrarySetting       = "vax.library"
	LegacyLibrarySetting = "vax.link.library"
)

// HostLibraryDir is the host directory to look in for VMS library files,
// or "" for none.
func (c *Context) HostLibraryDir() string {
	if c.HostLibrary != "" {
		return c.HostLibrary
	}

	if dir := settings.Get(LibrarySetting); dir != "" {
		return dir
	}

	return settings.Get(LegacyLibrarySetting)
}

// readLibraryFile reads a VMS library file: logical:file on a mounted
// volume, or file in the host library directory, and returns where it
// was. It returns nil data, and no error, when neither has it.
func (c *Context) readLibraryFile(logical, file string) ([]byte, rms.FileLocation, error) {
	spec := logical + ":" + file

	data, found, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: spec})

	var (
		notMounted *rms.NotMountedError
		notFound   *rms.NotFoundError
	)

	switch {
	case err == nil:
		return data, found, nil
	case !errors.As(err, &notMounted) && !errors.As(err, &notFound):
		return nil, rms.FileLocation{Name: spec}, err
	}

	dir := c.HostLibraryDir()
	if dir == "" {
		return nil, rms.FileLocation{}, nil
	}

	for _, name := range []string{file, strings.ToLower(file)} {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err == nil {
			return data, rms.FileLocation{Host: true, Name: path}, nil
		}

		if !errors.Is(err, fs.ErrNotExist) {
			return nil, rms.FileLocation{Host: true, Name: path}, err
		}
	}

	return nil, rms.FileLocation{}, nil
}
