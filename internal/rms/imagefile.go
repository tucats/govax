package rms

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// AccessImage opens the volume file at loc as the image activator does
// when it maps an image (docs/planning/PHASE-66 - mapped image activation.md,
// subtask 2): accessed for reading, others allowed to read it but not to
// write it, and held so for as long as the image's sections use it.
// While it's held an RMS open of the file for writing gets RMS$_FLK, an
// IO$_ACCESS for writing SS$_ACCONFLICT, and a DISMOUNT of its volume is
// refused, the file being open. (The Internals book has the activator
// open the image as a process-permanent file without saying with what
// sharing; read sharing only is govax's choice, which testdata/pg66's
// probe checks.)
//
// The process's protection is checked as VMS's activator checks it: a
// lookup in each directory, and R or E on the image (ErrNoPrivilege).
//
// It returns the accessed file, its bytes up to its end of file (what
// ReadRawFile would read: the header, the ISDs, and the debug symbols
// come from them), and the location of the file found. The caller lets
// the file go with Deaccess when nothing maps it any more. A file not
// there is a *NotFoundError, as for ReadRawFile.
func (s *Session) AccessImage(loc FileLocation) (*ACPFile, []byte, FileLocation, error) {
	if loc.Host {
		return nil, nil, FileLocation{}, fmt.Errorf("rms: %s is a host file", loc.Name)
	}

	var (
		a     *ACPFile
		buf   bytes.Buffer
		found FileLocation
	)

	err := s.firstSpec(loc.Name, func(vol *volume.Volume, r resolvedSpec) error {
		matches, err := s.globChecked(vol, r.Spec)
		if err != nil {
			return err
		}

		switch len(matches) {
		case 0:
			return &NotFoundError{Spec: loc.Name}
		case 1:
		default:
			return &AmbiguousError{Spec: loc.Name, Count: len(matches)}
		}

		// The image's directories must let the process look it up, and
		// the file give it R or E (access.go): VMS 7.1 ran an image with
		// either alone, and refused one with neither (the probe's A.4,
		// answer 4).
		f, err := s.Mounts.ACPAccessWith(r.Spec.Device, fileIDFrom(matches[0].Fid), ACPAccessMode{NoWrite: true})
		if err != nil {
			return err
		}

		if err := s.imageAllowed(vol, matches[0], &f.file.Header); err != nil {
			_ = f.Deaccess()

			return err
		}

		if err := copyRawTo(&buf, f.file); err != nil {
			_ = f.Deaccess()

			return err
		}

		a = f
		found = FileLocation{Name: r.shownFile(matches[0]), Physical: r.name.physicalFile(matches[0].Dirs, matches[0].ShortName(';'))}

		return nil
	})
	if err != nil {
		return nil, nil, FileLocation{}, fmt.Errorf("rms: accessing %s: %w", loc.Name, err)
	}

	return a, buf.Bytes(), found, nil
}

// imageAllowed reports whether the session's process may run the image
// m on vol, whose header is h: a lookup in each of its directories, and
// R or E on the file, else ErrNoPrivilege. VMS 7.1 ran an image giving
// either alone and refused one giving neither (the probe's A.4, answer 4).
func (s *Session) imageAllowed(vol *volume.Volume, m filespec.Match, h *ondisk.FileHeader) error {
	creds := s.Identity()

	if _, err := directoryFor(creds, vol, m.Dirs, needLookup); errors.Is(err, ErrNoPrivilege) {
		return err
	}

	if !mayEither(creds, vol, h, security.Read, security.Execute) {
		return ErrNoPrivilege
	}

	return nil
}
