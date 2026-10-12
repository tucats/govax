package dclcmd

import "github.com/tucats/govax/internal/vmsdef"

// ssNormal is SS$_NORMAL, and stsInhibitMsg STS$M_INHIB_MSG: the bit of
// a condition value that says its message has been shown.
var ssNormal = vmsdef.Symbols["SS$_NORMAL"]

const stsInhibitMsg = 0x10000000

// maxInputRecord is the longest line a command reads from SYS$INPUT:
// RMS's largest record.
const maxInputRecord = 32767
