package vmserrors

// ConditionError is a failure given as a VMS condition value rather than
// as one of govax's statuses: an image's exit status, a procedure's EXIT
// status, a message an image would have signaled. Status is the value,
// which DCL makes $STATUS, and Text its message.
type ConditionError struct {
	Status uint32
	Text   string
}

func (e ConditionError) Error() string { return e.Text }
