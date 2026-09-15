package vmserrors

// New creates a new instance of an error, with optional arguments.
func New(status uint32, args ...any) VMSError {
	return VMSError{
		Status:    status,
		Arguments: args,
	}
}

// Wrap creates a new instance of an error that also carries cause, an
// underlying Go error this status code is being reported in place of (for
// example an *os.PathError encountered while servicing an RMS-facility
// request). cause is reachable afterwards via errors.Unwrap/errors.As.
func Wrap(status uint32, cause error, args ...any) VMSError {
	return VMSError{
		Status:    status,
		Arguments: args,
		Cause:     cause,
	}
}
