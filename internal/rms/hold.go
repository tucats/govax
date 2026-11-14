package rms

import (
	"github.com/tucats/ods2/volume"
)

// HoldFile accesses the volume file loc names, as the LBR$ routines hold a
// library they have open (internal/lbrrtl), so that another opener's
// conflicting access waits or fails: to update, it denies others reading
// and writing; to read, it lets others read but not write. A conflicting
// access already there is ErrACPAccessConflict. The caller deaccesses the
// file (ACPFile.Deaccess) to let others in, as before writing it itself.
func (s *Session) HoldFile(loc FileLocation, update bool) (*ACPFile, error) {
	mode := ACPAccessMode{NoWrite: true}
	if update {
		mode = ACPAccessMode{Write: true, NoRead: true, NoWrite: true}
	}

	// The process must be able to find the library and have the access
	// it holds it with (R, and W to update): ErrNoPrivilege if not, as
	// LBR$OPEN's own RMS open would be refused.
	creds := s.Identity()
	mode.Credentials = &creds

	var held *ACPFile

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

		f, err := s.Mounts.ACPAccessWith(r.Spec.Device, fileIDFrom(matches[0].Fid), mode)
		if err != nil {
			return err
		}

		held = f

		return nil
	})

	return held, err
}
