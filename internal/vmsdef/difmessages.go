package vmsdef

// DIFFERENCES' condition values (docs/planning/PHASE-116 - DIFFERENCES.md),
// as VMS 7.1's $STATUS showed them in the phase's probe (testdata/dif116/vax).
// The facility is 108 (%X6C). Its own messages (STS$V_FAC_SP set) show as
// %DIF-..., the shared ones (OPENIN, NOWILD, BADVALUE) as %DIFF-..., the name
// VMS gives the facility.
//
// VMS's F$MESSAGE knew none of DIFFERENCES' own messages ("%DIFF-I-NOMSG,
// Message number 006C8013"): their texts are in the utility's image, not the
// system message file. So they aren't in LookupMessage's tables, and govax's
// F$MESSAGE answers as VMS's did; DIFFERENCES writes their texts itself
// (DifferencesMessages). The shared ones F$MESSAGE did know, through
// sharedMessage.
const DIFFFacility = 108

// DIFFERENCES' statuses. The F ones VMS showed with STS$M_INHIB_MSG set
// (the utility had written its message); the shared ones are the SHR$
// messages in DIF's facility. DIFNoWild's and DIFBadValue's values are
// reckoned from the shared messages', unconfirmed: the probe printed their
// $STATUS before it printed values.
const (
	DIFIdentical  = 0x006C8009 // %DIFF-S: the files are the same
	DIFDifferent  = 0x006C8013 // %DIFF-I: they differ
	DIFMaxDif     = 0x006C8018 // %DIF-W-MAXDIF
	DIFCmdSyntax  = 0x106C8054 // %DIF-F-CMDSYNTAX
	DIFNoLowVer   = 0x106C8064 // %DIF-F-NOLOWVER
	DIFTooMnyDlm  = 0x106C807C // %DIF-F-TOOMNYDLM
	DIFOpenIn     = 0x106C109C // %DIFF-F-OPENIN (SHR$_OPENIN)
	DIFNoWild     = 0x106C112C // %DIFF-F-NOWILD (SHR$_NOWILD)
	DIFBadValue   = 0x106C1114 // %DIFF-F-BADVALUE (SHR$_BADVALUE)
)

// DifferencesMessages are the texts of DIFFERENCES' own messages, by
// identifier, as VMS 7.1 wrote them (the probe's log). MRGGTMTCH,
// PARGTMTCH, and CMDLIN set no status, so their numbers weren't seen.
var DifferencesMessages = map[string]string{
	"MAXDIF":    "maximum number of difference records encountered - DIF aborted",
	"CMDSYNTAX": "command syntax error",
	"CMDLIN":    "command line segment in error",
	"NOLOWVER":  "no lower version of file !AS found",
	"TOOMNYDLM": "more than maximum of 32 comment delimiters specified",
	"MRGGTMTCH": "value of !UL for /MERGE is greater than /MATCH value of !UL",
	"PARGTMTCH": "value of !UL for /PARALLEL is greater than /MATCH value of !UL",
}
