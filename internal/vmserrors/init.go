package vmserrors

// Messages maps a fully constructed status code to its VMS-style message
// text (including the "FAC$NAME, " prefix), as registered by DefineMessage.
// Populated by each codes_*.go file's own init(), grouped by facility for
// ease of locating a particular message (codes_sys.go, codes_cli.go,
// codes_vax.go, codes_lib.go, codes_rms.go).
var Messages = map[uint32]string{}

// DefineMessage registers the display text for a status code: fac is the
// status's own facility (used to render the "FAC$" prefix), id is the
// message's short mnemonic (e.g. "ACCVIO"), and text is the message
// template, using "!X"/"!XB"/"!XW"/"!XL"/"!D"/"!UL"/"!S"/"!Q"/"!C" markers
// for positional argument substitution (see error.go's argFormatters).
func DefineMessage(status uint32, fac uint32, id, text string) {
	Messages[status] = FacilityNames[fac] + "$" + id + ", " + text
}
