package vmserrors

// SYS facility message IDs. Private: only the composite SS_* codes below
// are part of this package's public API.
const (
	sysStatus uint32 = 0
	sysAccvio uint32 = 1
)

// SYS facility status codes -- SS_ prefix, matching real VMS's own SS$_
// (system service) status codes.
const (
	SS_STATUS = SYSFacility<<FacilityPosition | sysStatus<<MessagePosition | StatusSuccess
	SS_ACCVIO = SYSFacility<<FacilityPosition | sysAccvio<<MessagePosition | StatusSevere
)

func init() {
	DefineMessage(SS_STATUS, SYSFacility, "NORMAL", "Completed successfully")
	DefineMessage(SS_ACCVIO, SYSFacility, "ACCVIO", "Access violation at !X")
}
