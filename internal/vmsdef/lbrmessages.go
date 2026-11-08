package vmsdef

// lbrMessages are the LBR facility's messages (the librarian's, LBRSHR.EXE's
// LBR$_ conditions), keyed as Messages is, without the severity bits. Each text
// is one VMS 7.1 showed: $GETMSG's for the statuses testdata/help90's LBR90.MAR
// printed (round 1b's log, part E), and LIBRARY's and HELP's messages in the
// probe's other logs (INVKEY, LKPNOTDON, DUPKEY, INVRFA). The condition values
// themselves are LBRSHR's universal symbols (ImageSymbols). The others' texts
// aren't known, and $GETMSG shows them as NOMSG, as it would on a VMS without
// the librarian's messages.
var lbrMessages = map[uint32]Message{
	0x00268000: {Facility: "LBR", Ident: "NORMAL", Text: "success"},
	0x00268830: {Facility: "LBR", Ident: "TYPMISMCH", Text: "library type mismatch"},
	0x00268838: {Facility: "LBR", Ident: "NOMTCHFOU", Text: "no match found"},
	0x00269008: {Facility: "LBR", Ident: "DUPKEY", Text: "duplicate key in index"},
	0x00269010: {Facility: "LBR", Ident: "ILLCTL", Text: "illegal control index"},
	0x00269020: {Facility: "LBR", Ident: "ILLIDXNUM", Text: "illegal index number"},
	0x00269048: {Facility: "LBR", Ident: "INVKEY", Text: "invalid key"},
	0x00269050: {Facility: "LBR", Ident: "INVNAM", Text: "invalid NAM block"},
	0x00269058: {Facility: "LBR", Ident: "INVRFA", Text: "invalid RFA"},
	0x00269060: {Facility: "LBR", Ident: "KEYNOTFND", Text: "key not found"},
	0x00269068: {Facility: "LBR", Ident: "LIBNOTOPN", Text: "library not open"},
	0x00269070: {Facility: "LBR", Ident: "LKPNOTDON", Text: "lookup has not been done"},
	0x00269090: {Facility: "LBR", Ident: "NOTHLPLIB", Text: "not a HELP library"},
	0x002690D8: {Facility: "LBR", Ident: "ILLOUTROU", Text: "output routine improperly specified or missing"},
	0x002690E8: {Facility: "LBR", Ident: "ILLINROU", Text: "input routine improperly specified or missing"},
	0x002690F0: {Facility: "LBR", Ident: "TOOMNYARG", Text: "too many arguments"},
	0x002690F8: {Facility: "LBR", Ident: "USRINPERR", Text: "user input routine returned error status"},
}
