package vmsdef

import (
	"fmt"
	"strconv"
	"strings"
)

// BACKUP's messages (docs/PHASE-69 - backup save sets.md, subtask 6).
// The system message file govax's tables were made from doesn't have
// them, so they are as VMS V7.1's BACKUP V6.2 showed them in the probe's
// log (testdata/backup69/vax/backup69.log): the texts are its, written as
// $FAO control strings, and so are the condition values of the messages
// whose code $STATUS showed. BACKUP is facility 163 (%XA3): the probe's
// $STATUS of %X10A3800C is OPENIN, severity F, with STS$M_INHIB_MSG (bit
// 28) set because BACKUP had shown it.
//
// The success and informational messages never reached $STATUS (BACKUP
// ended with SS$_NORMAL after them), so their message numbers are
// govax's, counted down from the top of the facility's range, where
// BACKUP's own are least likely to be: unconfirmed. So are the messages
// govax's BACKUP adds for what VMS's would do and govax's doesn't, or
// what the probe didn't try (their texts govax's too).

// BackupFacility is BACKUP's facility number.
const BackupFacility = 0xA3

// backupCode is the condition value of BACKUP's message number n (bit 15,
// facility-specific, included in n) with severity sev.
func backupCode(n, sev uint32) uint32 {
	return BackupFacility<<16 | n<<3 | sev
}

// BACKUP's condition values, by message, each with the severity VMS
// showed it with (a caller may give OPENIN severity E, as a compare's
// missing file does).
var (
	// Codes the probe's $STATUS confirmed.
	BackupOpenIn     = backupCode(0x1001, 4)
	BackupOpenOut    = backupCode(0x1002, 2)
	BackupLisInpSav  = backupCode(0x1011, 4)
	BackupNotSaveSet = backupCode(0x1020, 4)
	BackupVBNMissing = backupCode(0x1045, 2)
	BackupVerifyErr  = backupCode(0x104E, 2)
	BackupNoFiles    = backupCode(0x1080, 0)

	// VMS's texts, govax's codes.
	BackupCreated    = backupCode(0x1FFF, 1)
	BackupCreDir     = backupCode(0x1FFE, 1)
	BackupCopied     = backupCode(0x1FFD, 1)
	BackupCompared   = backupCode(0x1FFC, 1)
	BackupHeadCopied = backupCode(0x1FFB, 1)
	BackupNoBackup   = backupCode(0x1FFA, 3)
	BackupXorErrs    = backupCode(0x1FF9, 3)

	// govax's texts and codes.
	BackupBothSaveSets = backupCode(0x1FF8, 4)
	BackupNoSaveSet    = backupCode(0x1FF7, 4)
	BackupCmpInpSav    = backupCode(0x1FF6, 4)

	// A save's (subtask 11; 0x1FF5 was NOTYET, for a save before it):
	// VMS's texts as the manual quotes them or as
	// VMS's BACKUP is known to word them, unconfirmed against the probe,
	// and govax's codes; the block and group sizes' texts are govax's.
	BackupSearchFail  = backupCode(0x1FF4, 2)
	BackupAccConflict = backupCode(0x1FF3, 0)
	BackupBlockSize   = backupCode(0x1FF2, 4)
	BackupGroupSize   = backupCode(0x1FF1, 4)
)

// backupMessages are BACKUP's messages, keyed as Messages is.
// LookupMessage finds them after observedMessages.
var backupMessages = map[uint32]Message{}

func init() {
	for code, m := range map[uint32]struct{ ident, text string }{
		BackupOpenIn:     {"OPENIN", "error opening !AS as input"},
		BackupOpenOut:    {"OPENOUT", "error opening !AS as output"},
		BackupLisInpSav:  {"LISINPSAV", "/LIST requires save set as input"},
		BackupNotSaveSet: {"NOTSAVESET", "!AS is not a BACKUP save set"},
		BackupVBNMissing: {"VBNMISSING", "!AS has missing blocks !UL through !UL"},
		BackupVerifyErr:  {"VERIFYERR", "verification error for block !UL of !AS"},
		BackupNoFiles:    {"NOFILES", "no files selected from !AS"},

		BackupCreated:    {"CREATED", "created !AS"},
		BackupCreDir:     {"CREDIR", "created directory !AS"},
		BackupCopied:     {"COPIED", "copied !AS"},
		BackupCompared:   {"COMPARED", "compared !AS"},
		BackupHeadCopied: {"HEADCOPIED", "copied !AS header"},
		BackupNoBackup:   {"NOBACKUP", "!AS data not copied, file marked NOBACKUP"},
		// "1 error": the plural is govax's guess.
		BackupXorErrs: {"XORERRS", "!UL error!%S recovered by redundancy group in !AS"},

		BackupBothSaveSets: {"BOTHSAVSET", "input and output cannot both be save sets"},
		BackupNoSaveSet:    {"NOSAVESET", "input or output must be a save set"},
		BackupCmpInpSav:    {"CMPINPSAV", "/COMPARE requires save set as input"},

		BackupSearchFail:  {"SEARCHFAIL", "error searching for !AS"},
		BackupAccConflict: {"ACCONFLICT", "!AS is open for write by another user"},
		BackupBlockSize:   {"BLOCKSIZE", "block size must be from 2048 to 65535 bytes"},
		BackupGroupSize:   {"GROUPSIZE", "group size must be from 0 to 100"},
	} {
		backupMessages[code&messageKeyMask] = Message{
			Facility: "BACKUP", Ident: m.ident, Text: m.text, FAOCount: strings.Count(m.text, "!") - strings.Count(m.text, "!%S"),
		}
	}
}

// FormatMessage is the message for condition value code with its $FAO
// parameters args, without the leading "%", as $PUTMSG shows it:
// "BACKUP-S-CREATED, created DUA1:[R1.B69]ACL.TXT;1". It knows the
// directives BACKUP's messages use: !AS (a string), !UL (an unsigned
// number), and !%S ("s" unless the last number was 1). A code with no
// message is "NONAME-...-NOMSG", as $GETMSG gives it.
func FormatMessage(code uint32, args ...any) string {
	severity := "WSEIF???"[code&7 : code&7+1]

	m, ok := LookupMessage(code)
	if !ok {
		return fmt.Sprintf("NONAME-%s-NOMSG, Message number %08X", severity, code)
	}

	var (
		b    strings.Builder
		last uint64
	)

	text := m.Text
	for text != "" {
		i := strings.IndexByte(text, '!')
		if i < 0 {
			b.WriteString(text)

			break
		}

		b.WriteString(text[:i])
		text = text[i:]

		switch {
		case strings.HasPrefix(text, "!AS") && len(args) > 0:
			fmt.Fprint(&b, args[0])
			args, text = args[1:], text[3:]
		case strings.HasPrefix(text, "!UL") && len(args) > 0:
			last = unsigned(args[0])
			b.WriteString(strconv.FormatUint(last, 10))
			args, text = args[1:], text[3:]
		case strings.HasPrefix(text, "!%S"):
			if last != 1 {
				b.WriteString("s")
			}

			text = text[3:]
		default:
			b.WriteByte('!')
			text = text[1:]
		}
	}

	return m.Facility + "-" + severity + "-" + m.Ident + ", " + b.String()
}

// unsigned is a !UL parameter's value: any of Go's integers.
func unsigned(a any) uint64 {
	switch v := a.(type) {
	case int:
		return uint64(v)
	case int64:
		return uint64(v)
	case uint32:
		return uint64(v)
	case uint64:
		return v
	}

	return 0
}
