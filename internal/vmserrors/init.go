package vmserrors

var Messages = map[uint32]string{}

func DefineMessage(facility, id, severity uint32, text string) {
	prefix := FacilityNames[facility] + "$" + MessageNames[facility][id]

	Messages[facility|id|severity] = prefix + ", " + text
}

func init() {
	// Define some example messages
	DefineMessage(SYSFacility, SYS_STATUS, StatusWarning, "Completed with warnings")
	DefineMessage(SYSFacility, SYS_STATUS, StatusSuccess, "Completed successfully")
	DefineMessage(SYSFacility, SYS_STATUS, StatusError, "Failed with errors")
	DefineMessage(SYSFacility, SYS_STATUS, StatusInfo, "Informational message")
	DefineMessage(SYSFacility, SYS_STATUS, StatusSevere, "Severe error occurred")

	DefineMessage(SYSFacility, SYS_ACCVIO, StatusError, "Access violation at !X")
}
