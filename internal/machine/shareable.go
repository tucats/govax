package machine

import (
	"errors"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/rms"
)

// Finding a program's shareable images (docs/PHASE-68 - link shareable
// images.md, subtask 10): rms.ImageLocator's rules, with govax's shims
// standing for the images they emulate. Both loaders use it, the copying
// one (imageLoad) and the mapping one (imageMap); LINK uses the same
// locator without the shims.

// errEmulated is a shareable image's load when govax's shims stand for
// it: there is no file to load, and its references resolve to the shims
// (resolveFixupTarget).
var errEmulated = errors.New("emulated by govax's shims")

// ImageLocator is the locator of shareable images for a process whose
// volumes and logical names are session's: SET SHARE_PREFIX's prefix,
// the host library directory, the shims of ShimTable, and last the
// images built into govax (FTNRTL).
func (m *Machine) ImageLocator(session *rms.Session) rms.ImageLocator {
	return rms.ImageLocator{
		Session: session, Prefix: m.SharePrefix, Library: m.libraryDir(),
		Emulated: Emulated, Builtin: ftnrtl.Image,
	}
}

// FindShareableImage finds the shareable image name for process 1, as RUN
// does.
func (m *Machine) FindShareableImage(name string) (rms.FoundImage, error) {
	return m.ImageLocator(m.Hooks.Session()).Find(name)
}

// Emulated reports whether govax stands for the shareable image name:
// some entry of ShimTable is in it, or it is one of VMS's run-time
// libraries (runtimeLibraries).
func Emulated(name string) bool {
	name = strings.ToUpper(name)

	if runtimeLibraries[name] {
		return true
	}

	for _, e := range ShimTable {
		if e.Library == name {
			return true
		}
	}

	return false
}

// runtimeLibraries are VMS's run-time libraries, the shareable images in
// SYS$SHARE that DIGITAL's compilers' programs use: govax stands for each
// as it does for LIBRTL, its routines that no shim stands for resolving
// to a stub that signals NOTIMPL (resolveFixupTarget). A DEC C program names MTHRTL
// and CMA$TIS_SHR whether or not it calls them (testdata/exe's
// simple.exe), so a missing file of one of these can't stop activation
// as a missing image of the user's does. The list is of VMS 7's
// libraries, unconfirmed beyond the images testdata names.
var runtimeLibraries = map[string]bool{
	"BASRTL": true, "BASRTL2": true, "CMA$RTL": true, "CMA$OPEN_RTL": true,
	"CMA$TIS_SHR": true, "COBRTL": true, "DECC$SHR": true, "DPML$SHR": true,
	"FORRTL": true, "FORRTL2": true, "LIBRTL": true, "LIBRTL2": true,
	"MTHRTL": true, "PASRTL": true, "PLIRTL": true, "UVMTHRTL": true,
	"VAXCRTL": true, "VAXCRTLG": true, "VMTHRTL": true,
}

// Settings naming the host library directory (dclcmd's LibrarySetting
// and LegacyLibrarySetting).
const (
	librarySetting       = "vax.library"
	legacyLibrarySetting = "vax.link.library"
)

// libraryDir is the host directory of VMS's library files: the hook's,
// or else the vax.library setting's, or the older vax.link.library's.
func (m *Machine) libraryDir() string {
	if m.Hooks.LibraryDir != nil {
		return m.Hooks.LibraryDir()
	}

	if dir := settings.Get(librarySetting); dir != "" {
		return dir
	}

	return settings.Get(legacyLibrarySetting)
}

// session is the session p's process finds files through: its own, or
// else DCL's.
func (p *ImageProcess) session() *rms.Session {
	if env := p.process(); env != nil && env.Session != nil {
		return env.Session
	}

	return p.m.Hooks.Session()
}

// findShareable finds p's process's shareable image fn: errEmulated when
// the shims stand for it, an *rms.ImageNotFoundError when nothing has it.
func (p *ImageProcess) findShareable(fn string) (rms.FoundImage, error) {
	found, err := p.m.ImageLocator(p.session()).Find(fn)

	switch {
	case err != nil:
		return rms.FoundImage{}, err
	case found.Emulated:
		return rms.FoundImage{}, errEmulated
	}

	return found, nil
}

// readShareable reads the shareable image found, returning where it is.
func (p *ImageProcess) readShareable(found rms.FoundImage) ([]byte, imageSpec, error) {
	loc := found.Location

	switch {
	case found.Builtin != nil:
		return found.Builtin, imageSpec{Name: loc.Name}, nil

	case loc.Host:
		data, err := p.m.Paths.ReadFile(loc.Name)

		return data, hostImageSpec(loc.Name), err
	}

	data, located, err := p.session().ReadRawFile(loc)

	return data, imageSpec{Name: located.Physical}, err
}
