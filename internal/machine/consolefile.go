package machine

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is how the machine's own commands that read a file **ASM) 
// find it (docs/planning/PHASE-87 - system volume.md,
// Decision 9). They once read only host files, through the console's
// resolver (m.Paths: the host's current directory, the --path
// directories, then govax's built-in files). Now a name is placed as RUN
// and @ place theirs, by rms.Session.Locate's rules:
//
//   - a VMS file specification ("SYS$SOURCES:KERNEL.ASM") is a file on a
// 	   mounted volume;
//   - a host path ("./kernel.asm") is a host file, found
//     through the resolver as before;
//   - a bare name ("kernel.asm") is a file in the default directory when
//     a SET DEFAULT onto a volume is in effect, and a host file otherwise.
//
// One rule is the console's own: a bare name that isn't in the default
// volume directory is then looked for through the resolver, so that
// `ASM "kernel.asm"` still finds govax's built-in
// files when the default is a work volume. RUN and @ don't do this: their
// bare names are the user's files, where these are usually govax's.

// consoleFile is a file ASM reads: its contents, the name to
// report it by, and where it was found (for ASM's .INCLUDE, which looks
// beside it).
type consoleFile struct {
	data []byte
	name string
	loc  rms.FileLocation
}

// readConsoleFile reads the file name for ASM  by the rules above.
// text reads a volume file's records as lines (ASM's source), each ended
// by a line feed; otherwise its bytes are read whole.
func (m *Machine) readConsoleFile(name string, text bool) (consoleFile, error) {
	s := m.Hooks.Session()
	if s == nil {
		return readResolved(m.Paths, name)
	}

	loc, err := s.Side(name, false)
	if err != nil {
		return consoleFile{}, volumeFileFailure(s, err, name)
	}

	if loc.Host {
		if s.MayUseHost() {
			return readResolved(m.Paths, loc.Name)
		}

		return readConfigured(m.Paths, name, loc)
	}

	f, err := readVolumeFile(s, loc, text)
	if err == nil {
		return f, nil
	}

	var notFound *rms.NotFoundError
	if errors.As(err, &notFound) && rms.BareName(name) {
		read := readResolved
		if !s.MayUseHost() {
			read = func(paths *respath.Resolver, name string) (consoleFile, error) {
				return readConfigured(paths, name, rms.FileLocation{Host: true, Name: name})
			}
		}

		if f, rerr := read(m.Paths, name); rerr == nil {
			return f, nil
		}
	}

	return consoleFile{}, volumeFileFailure(s, err, loc.Name)
}

// readResolved reads name through the resolver paths: a host file, or one
// of govax's built-in files.
func readResolved(paths *respath.Resolver, name string) (consoleFile, error) {
	data, err := paths.ReadFile(name)
	if err != nil {
		return consoleFile{}, err
	}

	return consoleFile{data: data, name: name, loc: rms.FileLocation{Host: true, Name: name}}, nil
}

// readConfigured is readResolved for a process that may not use a host
// file (vax.host.sysprv: docs/planning/PHASE-97 - host file access.md,
// Decision 2): a bare name is still looked for in the places the
// configuration named (the --path directories, the system volume, and
// govax's built-in files), but not in the host's current directory, and
// any other host name is refused, SS$_NOPRIV.
func readConfigured(paths *respath.Resolver, name string, loc rms.FileLocation) (consoleFile, error) {
	if rms.BareName(name) {
		if data, err := paths.ReadConfigured(name); err == nil {
			return consoleFile{data: data, name: name, loc: rms.FileLocation{Host: true, Name: name}}, nil
		}
	}

	return consoleFile{}, vmserrors.Wrap(vmserrors.SS_NOPRIV, &rms.DeniedError{Name: loc.Name, Host: true})
}

// readVolumeFile reads the file at loc on a mounted volume.
func readVolumeFile(s *rms.Session, loc rms.FileLocation, text bool) (consoleFile, error) {
	if !text {
		data, found, err := s.ReadRawFile(loc)
		if err != nil {
			return consoleFile{}, err
		}

		return consoleFile{data: data, name: found.Name, loc: found}, nil
	}

	records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return consoleFile{}, err
	}

	var b strings.Builder

	for _, r := range records {
		b.Write(r)
		b.WriteByte('\n')
	}

	return consoleFile{data: []byte(b.String()), name: found.Name, loc: found}, nil
}

// volumeFileFailure turns RMS's error for a volume file into the
// message DCL's commands give for it: a device that isn't mounted, or a
// file that isn't there (named in full, as VMS names it).
func volumeFileFailure(s *rms.Session, err error, spec string) error {
	var notMounted *rms.NotMountedError
	if errors.As(err, &notMounted) {
		return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
	}

	var notFound *rms.NotFoundError
	if errors.As(err, &notFound) {
		if full, xerr := s.ExpandName(spec); xerr == nil {
			spec = full
		}

		return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, spec)
	}

	return err
}

// includeReader returns ASM's .INCLUDE reader for a source found at
// parent. A bare name is looked for beside the including file, on its
// side: in the same device and directory of its volume, or on the host
// in its directory and then through the resolver, as before. A bare name
// not beside a volume file is looked for through the resolver too, as
// readConsoleFile does. An explicit name is placed by Locate's rules, as
// ASM's own file name is. For interactive ASM, with no file, parent is
// a host location with no name: bare names go to the resolver.
func (m *Machine) includeReader(parent rms.FileLocation) func(string) (string, error) {
	paths := m.Paths
	if parent.Host && parent.Name != "" {
		paths = m.Paths.WithDir(filepath.Dir(parent.Name))
	}

	return func(name string) (string, error) {
		s := m.Hooks.Session()

		if s != nil && (!parent.Host || !rms.BareName(name)) {
			loc, err := s.LocateRelated(name, false, parent)
			if err != nil {
				if _, denied := rms.IsDenied(err); denied {
					return "", vmserrors.Wrap(vmserrors.SS_NOPRIV, err)
				}

				return "", volumeFileFailure(s, err, name)
			}

			if !loc.Host {
				f, err := readVolumeFile(s, loc, true)
				if err == nil {
					return string(f.data), nil
				}

				var notFound *rms.NotFoundError
				if !errors.As(err, &notFound) || !rms.BareName(name) {
					return "", volumeFileFailure(s, err, loc.Name)
				}

				read := paths.ReadFile
				if !s.MayUseHost() {
					read = m.Paths.ReadConfigured
				}

				if b, rerr := read(name); rerr == nil {
					return string(b), nil
				}

				return "", volumeFileFailure(s, err, loc.Name)
			}
		}

		// A bare name beside a host file, or for interactive ASM: a
		// process that may not use a host file finds it only where the
		// configuration said to look (readConfigured).
		if s != nil && !s.MayUseHost() {
			b, err := m.Paths.ReadConfigured(name)
			if err != nil {
				return "", vmserrors.Wrap(vmserrors.SS_NOPRIV, &rms.DeniedError{Name: name, Host: true})
			}

			return string(b), nil
		}

		b, err := paths.ReadFile(name)

		return string(b), err
	}
}
