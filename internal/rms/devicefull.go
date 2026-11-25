package rms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/ods2/volume"
)

// DeviceFullError is an output file a volume had no room to finish: Spec is
// the file's name, as a utility's WRITEERR names it (its device, directory,
// name, type, and the version it was made with), and Err what the volume
// said. VMS reports it as RMS$_FUL, with SS$_DEVICEFULL as its STV
// (unconfirmed: docs/planning/PHASE-113 - Fortran errata.md, subtask 11).
type DeviceFullError struct {
	Spec string
	Err  error
	// Open says the volume had no room for the file as it was made (a COPY
	// output given its input's blocks), and Source names the input.
	Open   bool
	Source string
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

	return errors.Is(err, volume.ErrNoSpace) || strings.Contains(err.Error(), ods2NoSpace)
}

// CreateError is a file the volume couldn't make a header for: RMS$_CRE,
// "ACP file create failed", with STV the volume's reason. VMS 7.1 said
// SS$_HEADERFULL when the index file's header had no room for its next
// growth (testdata/err113, round 2's HDRFULL); SS$_IDXFILEFULL, for a volume
// holding its maximum number of files, is unconfirmed. Spec and Source are
// as DeviceFullError's for a utility's OPENOUT.
type CreateError struct {
	Spec   string
	Source string
	STV    uint32
	Err    error
}

func (e *CreateError) Error() string {
	return fmt.Sprintf("rms: creating %s: %v", e.Spec, e.Err)
}

func (e *CreateError) Unwrap() error { return e.Err }

// rmsCreateFailed is RMS$_CRE, and ssHeaderFull and ssIndexFileFull its
// STVs.
var (
	rmsCreateFailed = vmsConst("RMS$_CRE")
	ssHeaderFull    = vmsConst("SS$_HEADERFULL")
	ssIndexFileFull = vmsConst("SS$_IDXFILEFULL")
)

// Status is RMS$_CRE, the condition a CreateError is reported as.
func (e *CreateError) Status() uint32 { return rmsCreateFailed }

// createFailure says whether err is a header the volume couldn't make, and
// if so its STV: SS$_HEADERFULL or SS$_IDXFILEFULL.
func createFailure(err error) (uint32, bool) {
	switch {
	case errors.Is(err, volume.ErrHeaderFull):
		return ssHeaderFull, true
	case errors.Is(err, volume.ErrIndexFileFull):
		return ssIndexFileFull, true
	}

	return 0, false
}

// AsCreateError is err as a CreateError, if it is a header the volume
// couldn't make, made one when it's ods2's refusal itself.
func AsCreateError(err error) (*CreateError, bool) {
	var ce *CreateError
	if errors.As(err, &ce) {
		return ce, true
	}

	if stv, ok := createFailure(err); ok {
		return &CreateError{STV: stv, Err: err}, true
	}

	return nil, false
}
