package vmserrors

// Create a new instance of an error, with optional arguments.
func New(status uint32, args ...any) VMSError {
	return VMSError{
		Status:    status,
		Arguments: args,
	}
}
