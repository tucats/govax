package rms

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/filespec"
)

// Finding a shareable image's file (docs/PHASE-68 - link shareable
// images.md, Decision 3 and subtask 10). A program's shareable image
// list holds only names ("SHR1", "LIBRTL"); the file each names is found
// the same way by RUN's two loaders (copying and mapping), by LINK for
// the images it reads, and, later, by LIB$FIND_IMAGE_SYMBOL:
//
//  1. When govax's shims stand for the image (LIBRTL, LIBRTL2, ...), the
//     shims, unless a logical name of the image's own name is defined:
//     DEFINE LIBRTL DUA1:[X]LIBRTL.EXE asks for that file instead.
//  2. RMS's parse of the name with the default SYS$SHARE:.EXE, as VMS's
//     image activator parses it (the Linker manual, 4.4): a logical name
//     of the image's name leads to the file, its missing fields taken
//     from SYS$SHARE's translation, else SYS$SHARE does. VMS 7.1's runs
//     (testdata/link68, R1 to R4) showed each of these, and that a
//     search list in SYS$SHARE is searched in order.
//  3. The host: the SET SHARE_PREFIX prefix, the host library directory
//     (vax.library), then the name as given (the current directory).

// ShareableDefault is the default file specification of a shareable
// image's name.
const ShareableDefault = "SYS$SHARE:.EXE"

// ImageLocator finds shareable images' files by the rules above.
type ImageLocator struct {
	// Session gives the mounted volumes and the logical names; nil for
	// neither.
	Session *Session

	// Prefix is prepended to a host file's name (SET SHARE_PREFIX), ""
	// for none; one that starts with ESC (^X1B) is none too.
	Prefix string

	// Library is the host library directory, "" for none.
	Library string

	// Emulated reports whether govax's shims stand for the image name;
	// nil for none, as for LINK, which reads an image's file whenever
	// there is one.
	Emulated func(name string) bool
}

// FoundImage is where a shareable image is: its shims (Emulated), or a
// file on a mounted volume or the host (Location).
type FoundImage struct {
	Emulated bool
	Location FileLocation
}

// ImageNotFoundError is a shareable image no rule found. Spec is the file
// specification RMS tried last on the volumes, as DCL's IMAGEFNF message
// shows it ("DUA0:[SYSLIB]SHR1.EXE;").
type ImageNotFoundError struct {
	Image, Spec string
}

func (e *ImageNotFoundError) Error() string {
	return fmt.Sprintf("image file not found %s", e.Spec)
}

// Unwrap gives govax's status for a missing image, RMS_IMAGENOTFOUND,
// which the process machinery turns into RMS$_FNF.
func (e *ImageNotFoundError) Unwrap() error {
	return vmserrors.New(vmserrors.RMS_IMAGENOTFOUND, e.Image)
}

// Find finds the shareable image name.
func (l ImageLocator) Find(name string) (FoundImage, error) {
	name = strings.TrimSpace(name)
	upper := strings.ToUpper(name)

	if l.Emulated != nil && l.Emulated(upper) && !l.Session.definesName(upper) {
		return FoundImage{Emulated: true}, nil
	}

	tried := upper + ".EXE;"

	if l.Session != nil {
		loc, spec, err := l.Session.FindShareableImage(name)
		if err == nil {
			return FoundImage{Location: loc}, nil
		}

		var notFound *NotFoundError
		if !errors.As(err, &notFound) {
			return FoundImage{}, err
		}

		if spec != "" {
			tried = spec
		}
	}

	if path, ok := l.hostFile(name); ok {
		return FoundImage{Location: FileLocation{Host: true, Name: path}}, nil
	}

	return FoundImage{}, &ImageNotFoundError{Image: upper, Spec: tried}
}

// hostFile is the absolute path of the host file of the image name, and
// true, or false when there is none: name after the prefix, in the
// library directory, then as given, each with the type .exe added when
// it has none, as written and then in lowercase.
func (l ImageLocator) hostFile(name string) (string, bool) {
	file := name
	if !strings.HasSuffix(strings.ToLower(file), ".exe") {
		file += ".exe"
	}

	var candidates []string

	if l.Prefix != "" && l.Prefix[0] != 0x1B {
		candidates = append(candidates, l.Prefix+file)
	}

	if l.Library != "" {
		candidates = append(candidates, filepath.Join(l.Library, file))
	}

	candidates = append(candidates, file)

	for _, c := range candidates {
		for _, path := range []string{c, filepath.Join(filepath.Dir(c), strings.ToLower(filepath.Base(c)))} {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if abs, err := filepath.Abs(path); err == nil {
					path = abs
				}

				return path, true
			}
		}
	}

	return "", false
}

// definesName reports whether name is a logical name in the tables file
// names are translated through (LNM$FILE_DEV). A nil Session has none.
func (s *Session) definesName(name string) bool {
	if s == nil || s.Logicals == nil {
		return false
	}

	_, err := s.Logicals.Translate(lnm.FileDevName, name, lnm.User, 0)

	return err == nil
}

// FindShareableImage finds the file of the shareable image name on the
// mounted volumes: RMS's parse of name with the default ShareableDefault,
// each element of SYS$SHARE's search list in turn (and each of name's, if
// its logical name is one), the highest version, as the image activator
// opens it. It returns the file's location (Name its resultant
// specification, Physical with its physical device) and the last
// specification tried; when none is there, a *NotFoundError, with that
// specification. A name RMS can't parse (a host path) finds nothing.
func (s *Session) FindShareableImage(name string) (FileLocation, string, error) {
	return s.FindImageFile(name, ShareableDefault)
}

// FindImageFile is FindShareableImage with another default file
// specification than SYS$SHARE:.EXE: LINK looks for a shareable image a
// library names in the library's own directory first
// ("DUA1:[000000].EXE"), then in SYS$LIBRARY (the Linker manual, 6.3.3.2).
func (s *Session) FindImageFile(name, defaultSpec string) (FileLocation, string, error) {
	ctx := &Context{Mounts: s.Mounts, Logicals: s.Logicals, Session: s}

	defaults := []string{defaultSpec}
	if fs, err := translateSpec(s.Logicals, defaultSpec); err == nil && len(fs) > 0 {
		defaults = defaults[:0]
		for _, f := range fs {
			defaults = append(defaults, f.Spec)
		}
	}

	tried := ""

	for _, def := range defaults {
		names, sts := ctx.expandName(nameInputs{Primary: name, Default: def})
		if sts != 0 {
			continue
		}

		for _, p := range names {
			spec := p.spec()
			tried = spec.Device + ":" + p.Dir + p.Name + p.Type + p.Ver

			if p.FNB&fnbWildcard != 0 || spec.Name == "" || s.Mounts == nil {
				continue
			}

			if loc, ok := s.findVersion(p, spec); ok {
				return loc, tried, nil
			}
		}
	}

	return FileLocation{}, tried, &NotFoundError{Spec: tried}
}

// findVersion is the file p (spec, for the volume layer) names, if it's
// there: its location, and true.
func (s *Session) findVersion(p parsedName, spec filespec.Spec) (FileLocation, bool) {
	vol, ok := s.Mounts.Lookup(spec.Device)
	if !ok {
		return FileLocation{}, false
	}

	dir, err := filespec.ResolveDirectory(vol, spec.Dirs)
	if err != nil {
		return FileLocation{}, false
	}

	entry, sts := lookupVersion(dir, specFileName(spec), spec.Version)
	if sts != 0 {
		return FileLocation{}, false
	}

	n, fileType := splitEntryName(entry.Name)
	version := strconv.Itoa(int(entry.Version))
	found := fileName{Dev: p.Dev, Dir: dirSpec{Elems: spec.Dirs}.String(), Name: n, Type: "." + fileType, Ver: ";" + version}
	physical := filespec.Spec{Device: spec.Device, Dirs: spec.Dirs, Name: n, Type: fileType, Version: version}

	return FileLocation{Name: found.String(), Physical: physical.String()}, true
}
