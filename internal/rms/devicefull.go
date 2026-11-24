package rms

import (
	"errors"
	"fmt"
	"strings"
)

// DeviceFullError is an output file a volume had no room to finish: Spec is
// the file's name, as a utility's WRITEERR names it (its device, directory,
// name, type, and the version it was made with), and Err what the volume
// said. VMS reports it as RMS$_FUL, with SS$_DEVICEFULL as its STV
// (unconfirmed: docs/planning/PHASE-113 - Fortran errata.md, subtask 11).
type DeviceFullError struct {
	Spec string
	Err  error
}

func (e *DeviceFullError) Error() string {
	return fmt.Sprintf("rms: writing %s: device full: %v", e.Spec, e.Err)
}

func (e *DeviceFullError) Unwrap() error { return e.Err }

// ods2NoSpace is the words ods2's volume package uses when an allocation
// finds too few free clusters ("not enough free space: 3 cluster(s)
// needed, 0 free"). ods2 has no error value for it to be tested against,
// so its text is what DeviceFull looks for; TestDeviceFullFromODS2 fills a
// volume, so a change of wording there fails a test here.
const ods2NoSpace = "not enough free space"

// DeviceFull reports whether err is a volume with no room for what was
// being written: govax's own ErrACPDeviceFull, or ods2's allocation failure
// beneath a record or block write.
func DeviceFull(err error) bool {
	if err == nil {
		return false
	}

	var full *DeviceFullError
	if errors.As(err, &full) || errors.Is(err, ErrACPDeviceFull) {
		return true
	}

	return strings.Contains(err.Error(), ods2NoSpace)
}
