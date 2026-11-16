package vmsdef

// UAFFacility is the UAF facility's number, AUTHORIZE's: bits 16 to 27 of
// the conditions VMS 7.1's AUTHORIZE left in $STATUS (UAEERR was
// %X13F1943A, the phase doc's A.5).
const UAFFacility = 0x3F1

// UAFMessage is one of AUTHORIZE's messages: its severity letter and its
// text, "!AS" standing for each value the message names, in order.
type UAFMessage struct {
	Severity byte
	Text     string

	// Status is the message's condition value where VMS 7.1 showed it in
	// $STATUS (with the inhibit bit, as AUTHORIZE leaves it), 0 where no
	// run has shown it.
	Status uint32
}

// UAFMessages are AUTHORIZE's messages by identifier, as VMS 7.1's
// AUTHORIZE printed them in testdata/uaf95's runs (vax/uaf95.log and the
// rounds' logs). Their condition values are known only for the few that
// ended a one-command run with AUTHORIZE's own status; nothing govax
// may read lists the rest.
var UAFMessages = map[string]UAFMessage{
	"ADDMSG":      {'I', "user record successfully added", 0},
	"BADSPC":      {'W', "no user matches specification", 0x13F18C80},
	"BADUSR":      {'W', "user name does not exist", 0},
	"BADVALUE":    {'E', "error in value specification", 0},
	"CLIWARNMSG":  {'W', "/CLITABLES field may need to reflect changes to /CLI field", 0},
	"COPMSG":      {'I', "user record copied", 0},
	"DEFPWD":      {'W', "copied or renamed records must receive new password", 0},
	"DONEMSG":     {'I', "system authorization file modified", 0},
	"GRANTERR":    {'E', "unable to grant identifier !AS to !AS", 0},
	"GRANTMSG":    {'I', "identifier !AS granted to !AS", 0},
	"GRANTUSR":    {'E', "user identifier !AS does not exist; !AS could not be granted", 0},
	"INVUSERNAME": {'E', "user name syntax error", 0x13F1931A},
	"LSTERR":      {'E', "error generating listing file", 0},
	"LSTMSG1":     {'I', "writing listing file", 0},
	"LSTMSG2":     {'I', "listing file SYSUAF.LIS complete", 0},
	"MDFYMSG":     {'I', "user record(s) updated", 0},
	"NAMETOOBIG":  {'E', "user name too long", 0x13F19352},
	"NAOFIL":      {'E', "unable to open system authorization file (SYSUAF.DAT)", 0},
	"NOIDNAME":    {'I', "no ID name was specified", 0},
	"NOMODS":      {'I', "no modifications made to system authorization file", 0},
	"PWDLESSMIN":  {'I', "new password is shorter than minimum password length", 0},
	"RDBADDERRU":  {'E', "unable to add !AS value !AS to rights database", 0},
	"RDBADDMSG":   {'I', "identifier !AS value !AS added to rights database", 0},
	"RDBADDMSGU":  {'I', "identifier !AS value !AS added to rights database", 0},
	"RDBCREERR":   {'E', "unable to create rights database (RIGHTSLIST.DAT)", 0},
	"RDBCREMSG":   {'I', "rights database created", 0},
	"RDBDONEMSG":  {'I', "rights database modified", 0},
	"RDBMDFYERR":  {'E', "unable to modify identifier !AS", 0},
	"RDBMDFYMSG":  {'I', "identifier !AS modified", 0},
	"RDBNOMODS":   {'I', "no modifications made to rights database", 0},
	"RDBREMERR":   {'E', "unable to remove !AS from rights database", 0},
	"RDBREMMSG":   {'I', "identifier !AS value !AS removed from rights database", 0},
	"RDBREMMSGU":  {'I', "identifier !AS value !AS removed from rights database", 0},
	"REMMSG":      {'I', "record removed from system authorization file", 0},
	"RENMSG":      {'I', "user record renamed", 0},
	"REVOKEERR":   {'E', "unable to revoke identifier !AS from !AS", 0},
	"REVOKEMSG":   {'I', "identifier !AS revoked from !AS", 0},
	"RLSTMSG":     {'I', "listing file RIGHTSLIST.LIS complete", 0},
	"SAMEUIC":     {'W', "new user record has the same UIC as the source record", 0},
	"SHOWERR":     {'E', "unable to complete SHOW command", 0},
	"UAEERR":      {'E', "invalid user name, user name already exists", 0x13F1943A},
	"UICERR":      {'E', "error in UIC specification", 0},
	"VALTOOBIG":   {'E', "value too large for field", 0},
}
