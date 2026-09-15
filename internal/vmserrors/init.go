package vmserrors

var Messages = map[uint32]string{}

func DefineMessage(status uint32, fac uint32, id, text string) {
	Messages[status] = FacilityNames[fac] + "$" + id + ", " + text
}

// Define the messages. These should be filled in as each new
// message is created in the emulator. Additionally, these
// should include the standard VMS SS$_* message codes (to be
// done).  These should be grouped by facility for ease of
// locating a particular message.
//
// Note, the init() function is called exactly once during package
// initialization by the Go runtime.
func init() {
	// System messages
	DefineMessage(SS_STATUS, SYSFacility, "NORMAL", "Completed successfully")
	DefineMessage(SS_ACCVIO, SYSFacility, "ACCVIO", "Access violation at !X")

	// CLI Messages
	DefineMessage(CLI_UNKVERB, CLIFacility, "UNKVERB", "Unknown verb: !S")
}
