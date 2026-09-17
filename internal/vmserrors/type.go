// This package describes VMS-compatible errors, which emulate the way
// native errors are genearted from VMS runtimes and system services.

package vmserrors

const (
	// Mask values to extract bit fields from a VMS status code.
	// +----------+----------+----------+---------+----------+
	// | Reserved | Facility | Customer | Message | Severity |
	// +----------+----------+----------+---------+----------+
	// | 31..29   | 28..20   | 19       | 18..3   | 2..0     |
	// +----------+----------+----------+---------+----------+
	// The following constants define the bit masks for each field
	// in the status code.

	SeverityPosition = 0
	MessagePosition  = 3
	CustomerPosition = 19
	FacilityPosition = 20
	ReservedPosition = 29

	Severity = 0x7
	Message  = 0xFFFF << MessagePosition
	Customer = 0x1 << CustomerPosition
	Facility = 0x1FF << FacilityPosition
	Reserved = 0x7 << ReservedPosition

	// Well known faciity codes.
	SYSFacility = 0  // System and system service facility
	RMSFacility = 1  // RMS (file I/O) facility
	CLIFacility = 2  // Command line or DCL errors
	DBGFacility = 3  // Messages from the debugger
	LIBFacility = 6  // MEssages from the LIBRTL shims and runtimes
	VAXFacility = 15 // This is the facility used by govax internal messages

	// Well known names for the severity field values. Note that, matching
	// real VMS's $VMS_STATUS_SUCCESS convention, the low bit of the
	// severity field (StatusSuccess and StatusInfo, both odd) marks a
	// status as "successful" -- see VMSError.OK.
	StatusWarning = 0
	StatusSuccess = 1
	StatusError   = 2
	StatusInfo    = 3
	StatusSevere  = 4
)

const (
	// Constant descriptive names for the severify field.
	SYS_Warning = "WARNING"
	SYS_Success = "SUCCESS"
	SYS_Error   = "ERROR"
	SYS_Info    = "INFO"
	SYS_Severe  = "SEVERE"
)

// Well known error codes are constructed from a facility, message, and
// severity, and are catalogued by facility in codes_*.go (codes_sys.go,
// codes_cli.go, codes_vax.go, codes_lib.go, codes_rms.go). Each of those
// files defines its own private (lower-case) block of sequential message
// ID numbers -- an implementation detail used only to build the composite
// codes below and to call DefineMessage -- plus the exported, fully
// constructed uint32 status codes (SS_*, CLI_*, VAX_*, LIB_*, RMS_*) that
// the rest of govax refers to errors by. Only the composite codes are part
// of this package's public API; callers should never need to reason about
// a bare message ID.
//
// Naming follows real VMS convention: the composite constant's prefix
// names the facility that reports it (SS_ for the SYS facility, matching
// VMS's own SS$_ system-service status codes; CLI_/VAX_/LIB_/RMS_
// otherwise), not the message's own mnemonic.

// Structure of an error, which is a 32-bit status code and an optional list of arguments that
// provide additional information about the error. The status code is a 32-bit value that
// encodes the severity, facility, and message ID of the error. The arguments are a list of
// 32-bit values that provide additional context for the error. The error string will indicate
// if the argument value is a hex constant ("!X"), a decimal constant ("!D"), or a string ("!S").
// String vlaues are addresses of a value in memory, which is expected to be a null-terminated
// string.
//
// The string representation of the error will include the facility name, message name, and
// severity, along with the arguments.
//
// Cause, when non-nil, is an underlying Go error this VMSError wraps (for example an I/O
// error encountered while servicing the request the status code describes). It is not part
// of the VMS status-code model itself, but lets callers use errors.Unwrap/errors.As to reach
// the original error while still reporting a VMS-style status code up through the emulator.
type VMSError struct {
	Status    uint32
	Arguments []any
	Cause     error
}

// The following table maps the faiclity names to strings. This will
// eventually be externalized in a file that can be loaded to extend
// or modify the messages.

var FacilityNames = map[uint32]string{
	SYSFacility: "SYSTEM",
	RMSFacility: "RMS",
	CLIFacility: "CLI",
	DBGFacility: "DBG",
	LIBFacility: "LIB",
	VAXFacility: "VAX", // These are the errors used by govax internally
}

var MessageNames = map[uint32]string{}

// Equals tests to see if an error matches a given known status code,
// irrespective of the arguments or wrapped cause.
func (e VMSError) Equals(status uint32) bool {
	return e.Status == status
}

// Is implements the interface errors.Is looks for, so that
// errors.Is(err, vmserrors.New(vmserrors.SS_ACCVIO)) works regardless of
// either side's Arguments/Cause -- two VMSErrors are considered the same
// error if (and only if) they carry the same Status. This is needed
// because VMSError's Arguments field (a []any) makes the type
// uncomparable, so errors.Is can't fall back to its own built-in ==
// comparison the way it does for plain sentinel errors.
func (e VMSError) Is(target error) bool {
	t, ok := target.(VMSError)
	if !ok {
		return false
	}

	return e.Status == t.Status
}

// Unwrap returns the wrapped Cause error, if any, so that errors.Is/
// errors.As can see through a VMSError to whatever underlying Go error
// (if any) it was built from.
func (e VMSError) Unwrap() error {
	return e.Cause
}

// SeverityCode extracts the severity field (StatusWarning/.../StatusSevere)
// from the status code.
func (e VMSError) SeverityCode() uint32 {
	return e.Status & Severity
}

// FacilityCode extracts the facility field (SYSFacility/.../VAXFacility)
// from the status code.
func (e VMSError) FacilityCode() uint32 {
	return (e.Status & Facility) >> FacilityPosition
}

// MessageCode extracts the message-id field from the status code.
func (e VMSError) MessageCode() uint32 {
	return (e.Status & Message) >> MessagePosition
}

// OK reports whether the status code represents success, matching real
// VMS's $VMS_STATUS_SUCCESS test: the severity field's low bit is set for
// StatusSuccess and StatusInfo, clear for StatusWarning/StatusError/
// StatusSevere.
func (e VMSError) OK() bool {
	return e.SeverityCode()&1 == 1
}
