package rms

import (
	"bytes"
	"fmt"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// AccessImage opens the volume file at loc as the image activator does
// when it maps an image (docs/PHASE-66 - mapped image activation.md,
// subtask 2): accessed for reading, others allowed to read it but not to
// write it, and held so for as long as the image's sections use it.
// While it's held an RMS open of the file for writing gets RMS$_FLK, an
// IO$_ACCESS for writing SS$_ACCONFLICT, and a DISMOUNT of its volume is
// refused, the file being open. (The Internals book has the activator
// open the image as a process-permanent file without saying with what
// sharing; read sharing only is govax's choice, which testdata/pg66's
// probe checks.)
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
		matches, err := filespec.Glob(vol, r.Spec)
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

		f, err := s.Mounts.ACPAccessWith(r.Spec.Device, fileIDFrom(matches[0].Fid), ACPAccessMode{NoWrite: true})
		if err != nil {
			return err
		}

		if err := copyRawTo(&buf, f.file); err != nil {
			_ = f.Deaccess()

			return err
		}

		a = f
		found = FileLocation{Name: fullSpec(r.Display, matches[0])}

		return nil
	})
	if err != nil {
		return nil, nil, FileLocation{}, fmt.Errorf("rms: accessing %s: %w", loc.Name, err)
	}

	return a, buf.Bytes(), found, nil
}
