package rms

import "testing"

// TestRMSStatusValuesAreDistinctAndPositive is a structural regression
// test over status.go's rms* constants: every one of them is meant to be
// a distinct, real, positive VMS status-code value (see status.go's own
// doc comment on why these have to match VMS's real numbering rather than
// being this project's own invented scheme). A duplicate here would mean
// two different named errors had accidentally been given the same
// number — most likely a copy/paste mistake while transcribing them —
// which would make this package's handlers unable to tell those two
// conditions apart from the status code alone.
func TestRMSStatusValuesAreDistinctAndPositive(t *testing.T) {
	values := map[string]uint32{
		"rmsNormal":             rmsNormal,
		"rmsCreated":            rmsCreated,
		"rmsEOF":                rmsEOF,
		"rmsFileExists":         rmsFileExists,
		"rmsFileLocked":         rmsFileLocked,
		"rmsFileNotFound":       rmsFileNotFound,
		"rmsPrivilegeViolation": rmsPrivilegeViolation,
		"rmsRecordNotFound":     rmsRecordNotFound,
		"rmsDeviceNotReady":     rmsDeviceNotReady,
		"rmsDeviceError":        rmsDeviceError,
		"rmsInvalidIFI":         rmsInvalidIFI,
		"rmsInvalidOrg":         rmsInvalidOrg,
		"rmsInvalidRAC":         rmsInvalidRAC,
		"rmsInvalidRFM":         rmsInvalidRFM,
		"rmsRecordTooBig":       rmsRecordTooBig,
		"rmsSystemError":        rmsSystemError,
	}

	seen := make(map[uint32]string, len(values))

	for name, v := range values {
		if v <= 0 {
			t.Errorf("%s = %d, want a positive value", name, v)
		}

		if other, dup := seen[v]; dup {
			t.Errorf("%s and %s both have value %d, want distinct values", name, other, v)
		}

		seen[v] = name
	}
}

// TestRMSNormalIsSuccess confirms rmsNormal specifically has VMS's
// well-known "success" bit pattern: real VMS condition codes encode
// severity in the low 3 bits, and a value of 1 (STS$K_SUCCESS) there
// means success — this is what every VAX program's own "did the call
// succeed?" check (conventionally something like "BLBC R0, error_label",
// branch-on-low-bit-clear) actually tests. Getting this specific bit
// wrong would mean a real VAX program calling into this package's
// handlers would treat ordinary success as a failure, or vice versa —
// about as fundamental a fidelity requirement as this package has.
func TestRMSNormalIsSuccess(t *testing.T) {
	const severityMask = 0x7
	const success = 1

	if got := rmsNormal & severityMask; got != success {
		t.Errorf("rmsNormal severity bits = %d, want %d (STS$K_SUCCESS)", got, success)
	}
}
