package vmserrors

// SYS facility message IDs. Private: only the composite SS_* codes below
// are part of this package's public API.
//
// Unlike the other codes_*.go files (RMS/CLI/LIB/VAX), these message-ID
// numbers aren't just an arbitrary, sequential internal numbering scheme —
// they're chosen so that FacilityPosition/MessagePosition packing them
// with SYSFacility (0) reproduces the real, literal $SSDEF numeric value
// from VMS's own ss_def.h exactly (see each SS_* constant's own doc
// comment below for the specific real value it matches). That's only
// possible for this one facility because SYSFacility is 0: real VMS's own
// status-code encoding gives facility-0 ("system") codes their message
// number directly in bits 3-15 with no facility contribution at all, so
// this package's general packing formula and VMS's own real encoding
// happen to coincide here. A message ID is still just this package's
// internal ordinal (order added, nothing more) — it's the byte value of
// the resulting composite SS_* constant that has to match ss_def.h, not
// the ordinal itself.
const (
	sysStatus      uint32 = 0
	sysAccvio      uint32 = 1
	sysBadParam    uint32 = 2    // real SS$_BADPARAM's message field: 20 >> 3
	sysDevMount    uint32 = 13   // real SS$_DEVMOUNT's message field: 108 >> 3
	sysDevNotMount uint32 = 15   // real SS$_DEVNOTMOUNT's message field: 124 >> 3
	sysNoMount     uint32 = 1297 // real SS$_NOMOUNT's message field: 10380 >> 3
	sysNoSuchFile  uint32 = 290  // real SS$_NOSUCHFILE's message field: 2320 >> 3

	// Logical-name service codes (docs/PHASE-25.md). Values and message
	// texts are from the real VMS 7.3 SYSMSG source listing
	// (vmssrc_archive/v73/msgfil/lis/sysmsg.lis), and agree with
	// VMS 7.3's $SSDEF (see codes_sys_test.go).
	sysNoPriv      uint32 = 4    // SS$_NOPRIV:       36 >> 3
	sysDupLNam     uint32 = 18   // SS$_DUPLNAM:     148 >> 3
	sysIvLogNam    uint32 = 42   // SS$_IVLOGNAM:    340 >> 3
	sysIvLogTab    uint32 = 43   // SS$_IVLOGTAB:    348 >> 3
	sysNoLogNam    uint32 = 55   // SS$_NOLOGNAM:    444 >> 3
	sysTooManyLNam uint32 = 110  // SS$_TOOMANYLNAM: 884 >> 3
	sysSupersede   uint32 = 198  // SS$_SUPERSEDE:  1585 >> 3
	sysLNMCreated  uint32 = 214  // SS$_LNMCREATED: 1713 >> 3
	sysParentDel   uint32 = 1098 // SS$_PARENT_DEL: 8788 >> 3
	sysNoLogTab    uint32 = 1106 // SS$_NOLOGTAB:   8852 >> 3
)

// SYS facility status codes -- SS_ prefix, matching real VMS's own SS$_
// (system service) status codes.
const (
	SS_STATUS = SYSFacility<<FacilityPosition | sysStatus<<MessagePosition | StatusSuccess
	SS_ACCVIO = SYSFacility<<FacilityPosition | sysAccvio<<MessagePosition | StatusSevere

	// SS_BADPARAM is real VMS's SS$_BADPARAM (20), "bad parameter value":
	// a service argument that is out of range or malformed (for example
	// internal/lnm's attribute and equivalence-count checks). Until
	// docs/PHASE-25.md subtask 6 it also carried INITIALIZE/CONTAINER's
	// failure text; that now has its own code, CLI_INITFAIL.
	SS_BADPARAM = SYSFacility<<FacilityPosition | sysBadParam<<MessagePosition | StatusSevere

	// SS_DEVMOUNT is real VMS's SS$_DEVMOUNT (108, per
	// reference/eVAX/eVAX/Headers/ss_def.h): docs/PHASE-22.md's MOUNT
	// command reports this when asked to mount a device that already has
	// a volume mounted on it (internal/console/mount.go), matching real
	// MOUNT's refusal to implicitly swap an already-mounted device's
	// volume out from under it.
	SS_DEVMOUNT = SYSFacility<<FacilityPosition | sysDevMount<<MessagePosition | StatusSevere

	// SS_DEVNOTMOUNT is real VMS's SS$_DEVNOTMOUNT (124): reported by
	// DISMOUNT (internal/console/mount.go) when asked to dismount a
	// device with nothing currently mounted on it.
	SS_DEVNOTMOUNT = SYSFacility<<FacilityPosition | sysDevNotMount<<MessagePosition | StatusSevere

	// SS_NOMOUNT is real VMS's SS$_NOMOUNT (10380): the generic "the MOUNT
	// (or DISMOUNT) operation itself could not be completed" status
	// (internal/console/mount.go) for every other failure — a container
	// file that doesn't exist or isn't a valid ODS-2 volume, or an I/O
	// error while dismounting — as opposed to SS_DEVMOUNT/SS_DEVNOTMOUNT's
	// more specific "wrong state" conditions above.
	SS_NOMOUNT = SYSFacility<<FacilityPosition | sysNoMount<<MessagePosition | StatusSevere

	// SS_NOSUCHFILE is real VMS's SS$_NOSUCHFILE (2320, per
	// reference/eVAX/eVAX/Headers/ss_def.h): docs/PHASE-23.md's DELETE
	// (internal/console/delete.go) reports this when a file specification's
	// name/type/version pattern matches nothing on its resolved volume --
	// the operator-console-facing "not found" status the design section's
	// "Status-code / error translation" section earmarked for DELETE/TYPE/
	// COPY. Real ss_def.h encodes this at warning severity (2320's low 3
	// bits are 0), not error/severe -- matching real VMS's own long-standing
	// convention that a "file not found" condition (its RMS-facility
	// cousin, RMS$_FNF, is likewise a W-severity message) is a normal,
	// expected outcome rather than a hard failure.
	SS_NOSUCHFILE = SYSFacility<<FacilityPosition | sysNoSuchFile<<MessagePosition | StatusWarning

	// Logical-name service codes (internal/lnm, docs/PHASE-25.md). The two
	// success codes are returned by $CRELNM/$CRELNT rather than as errors,
	// but are defined here so their messages are registered too.
	SS_NOPRIV      = SYSFacility<<FacilityPosition | sysNoPriv<<MessagePosition | StatusSevere
	SS_DUPLNAM     = SYSFacility<<FacilityPosition | sysDupLNam<<MessagePosition | StatusSevere
	SS_IVLOGNAM    = SYSFacility<<FacilityPosition | sysIvLogNam<<MessagePosition | StatusSevere
	SS_IVLOGTAB    = SYSFacility<<FacilityPosition | sysIvLogTab<<MessagePosition | StatusSevere
	SS_NOLOGNAM    = SYSFacility<<FacilityPosition | sysNoLogNam<<MessagePosition | StatusSevere
	SS_TOOMANYLNAM = SYSFacility<<FacilityPosition | sysTooManyLNam<<MessagePosition | StatusSevere
	SS_SUPERSEDE   = SYSFacility<<FacilityPosition | sysSupersede<<MessagePosition | StatusSuccess
	SS_LNMCREATED  = SYSFacility<<FacilityPosition | sysLNMCreated<<MessagePosition | StatusSuccess
	SS_PARENT_DEL  = SYSFacility<<FacilityPosition | sysParentDel<<MessagePosition | StatusSevere
	SS_NOLOGTAB    = SYSFacility<<FacilityPosition | sysNoLogTab<<MessagePosition | StatusSevere
)

func init() {
	DefineMessage(SS_STATUS, SYSFacility, "NORMAL", "Completed successfully")
	DefineMessage(SS_ACCVIO, SYSFacility, "ACCVIO", "Access violation at !X")
	DefineMessage(SS_BADPARAM, SYSFacility, "BADPARAM", "bad parameter value")
	DefineMessage(SS_DEVMOUNT, SYSFacility, "DEVMOUNT", "Device !S already mounted")
	DefineMessage(SS_DEVNOTMOUNT, SYSFacility, "DEVNOTMOUNT", "Device !S not mounted")
	DefineMessage(SS_NOMOUNT, SYSFacility, "NOMOUNT", "Unable to complete MOUNT/DISMOUNT operation on device !S")
	DefineMessage(SS_NOSUCHFILE, SYSFacility, "NOSUCHFILE", "File !S not found")
	DefineMessage(SS_NOPRIV, SYSFacility, "NOPRIV", "insufficient privilege or object protection violation")
	DefineMessage(SS_DUPLNAM, SYSFacility, "DUPLNAM", "duplicate name")
	DefineMessage(SS_IVLOGNAM, SYSFacility, "IVLOGNAM", "invalid logical name")
	DefineMessage(SS_IVLOGTAB, SYSFacility, "IVLOGTAB", "invalid logical name table")
	DefineMessage(SS_NOLOGNAM, SYSFacility, "NOLOGNAM", "no logical name match")
	DefineMessage(SS_TOOMANYLNAM, SYSFacility, "TOOMANYLNAM", "logical name translation exceeded allowed depth")
	DefineMessage(SS_SUPERSEDE, SYSFacility, "SUPERSEDE", "logical name superseded")
	DefineMessage(SS_LNMCREATED, SYSFacility, "LNMCREATED", "logical name table did not exist; has been created")
	DefineMessage(SS_PARENT_DEL, SYSFacility, "PARENT_DEL", "illegal attempt to delete parent logical name table")
	DefineMessage(SS_NOLOGTAB, SYSFacility, "NOLOGTAB", "no logical name table name match")
}
