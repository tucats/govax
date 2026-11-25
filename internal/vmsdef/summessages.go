package vmsdef

// SUMSLP's messages (docs/planning/PHASE-117 - SUMSLP.md), as VMS 7.1 wrote
// them in the phase's probe (testdata/sum117/vax). Their facility is SUM,
// "SUMSLP Batch Editing Utility" (HELP/MESSAGE/FACILITY=SUM).
//
// None of their numbers was ever seen: SUMSLP leaves $STATUS an RMS status
// with STS$M_INHIB_MSG set, RMS's success (%X10010001) after any run that
// finished, warnings and all, and the failing RMS status (FNF, WLD, RAC)
// after one that didn't (the phase doc's A.4.1). So govax gives them no
// numbers: EDIT/SUM writes their texts itself, and F$MESSAGE knows none of
// them, as VMS's F$MESSAGE knew only RMS's statuses.
const SUMFacilityName = "SUM"

// SUMMessages are the texts of SUMSLP's messages, by identifier. OPENIN and
// READERR are the shared messages (SHR$_OPENIN, SHR$_READERR), their !AS a
// file's name; the four warnings are SUMSLP's own, each written with a line
// per edit command it is about.
var SUMMessages = map[string]string{
	"OPENIN":    "error opening !AS as input",
	"READERR":   "error reading !AS",
	"EDITSCLSH": "edits clash",
	"EDOUTSEQ":  "edits out of sequence",
	"PRMEOF":    "premature end-of-file",
	"SLPSYNERR": "SLP command syntax error",
}
