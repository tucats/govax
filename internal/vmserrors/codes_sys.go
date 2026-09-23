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
)

// SYS facility status codes -- SS_ prefix, matching real VMS's own SS$_
// (system service) status codes.
const (
	SS_STATUS = SYSFacility<<FacilityPosition | sysStatus<<MessagePosition | StatusSuccess
	SS_ACCVIO = SYSFacility<<FacilityPosition | sysAccvio<<MessagePosition | StatusSevere

	// SS_BADPARAM is real VMS's SS$_BADPARAM (20): docs/PHASE-23.md's
	// INITIALIZE/CONTAINER (internal/console/initialize.go) reports this
	// for any failure building the new container/volume -- a size of zero
	// blocks, a container path that can't be created (bad directory,
	// permissions, ...), a volume label too long for its fixed 12-byte
	// on-disk field, or a size too small to hold even the minimal reserved
	// file layout. Real VMS's own INITIALIZE reports failures through a
	// dedicated INIT facility this project doesn't model; SS_BADPARAM is
	// used instead, matching this phase's "operator-console-facing SS$_
	// statuses, the same class MOUNT/DISMOUNT already use" design decision
	// -- ss_def.h has no INIT-specific status to reuse, and every one of
	// this command's own failure modes ultimately traces back to a bad
	// argument value (path/size/label/cluster), the same story SS$_BADPARAM
	// tells for any other system service.
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
)

func init() {
	DefineMessage(SS_STATUS, SYSFacility, "NORMAL", "Completed successfully")
	DefineMessage(SS_ACCVIO, SYSFacility, "ACCVIO", "Access violation at !X")
	DefineMessage(SS_BADPARAM, SYSFacility, "BADPARAM", "Unable to complete INITIALIZE operation on !S")
	DefineMessage(SS_DEVMOUNT, SYSFacility, "DEVMOUNT", "Device !S already mounted")
	DefineMessage(SS_DEVNOTMOUNT, SYSFacility, "DEVNOTMOUNT", "Device !S not mounted")
	DefineMessage(SS_NOMOUNT, SYSFacility, "NOMOUNT", "Unable to complete MOUNT/DISMOUNT operation on device !S")
	DefineMessage(SS_NOSUCHFILE, SYSFacility, "NOSUCHFILE", "File !S not found")
}
