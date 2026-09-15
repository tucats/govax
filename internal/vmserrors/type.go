// This package describes VMS-compatible errors, which emulate the way
// native errors are genearted from VMS runtimes and system services.

package vmserrors

const (
	// Mask values to extract bit fields from a VMS status code.
	Severity = 0x3 << 0
	Facility = 0x1FF << 3
	Customer = 0x1 << 12
	Message  = 0xFFFF << 16

	// Well known faciity codes.
	SYSFacility = 0
	RMSFacility = 1
	CLIFacility = 2
	DBGFacility = 3

	// Well known names for the severity field values.
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

	// Error ID constants by facility. This group covers the
	// SYS facility, which is the most common source of errors.
	SYS_STATUS uint32 = 0
	SYS_ACCVIO uint32 = 1
)

// Well known error codes, constructed from the above constnats.

const (
	SS_STATIS = SYSFacility | SYS_STATUS | StatusSuccess
	SS_ACCVIO = SYSFacility | SYS_ACCVIO | StatusError
)

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
type VMSError struct {
	Status    uint32
	Arguments []any
}

// The following table maps the faiclity names to strings. This will
// eventually be externalized in a file that can be loaded to extend
// or modify the messages.

var FacilityNames = map[uint32]string{
	SYSFacility: "SYS",
	RMSFacility: "RMS",
	CLIFacility: "CLI",
	DBGFacility: "DBG",
}

var MessageNames = map[uint32]map[uint32]string{
	SYSFacility: {
		SYS_STATUS: "STATUS",
		SYS_ACCVIO: "ACCVIO",
	},
}

// Test to see if an error matches a given knwon status code,
// irrespective of the arguments.
func (e VMSError) Equals(status uint32) bool {
	return e.Status == status
}
