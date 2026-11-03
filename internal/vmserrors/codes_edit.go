package vmserrors

// EDIT facility message IDs: the EDIT command and SET EDITOR
// (internal/dclcmd/edit.go), which are govax's own. Private: only the
// composite EDIT_* codes below are part of this package's public API.
const (
	editCommand uint32 = iota + 1
	editScratch
	editEditor
	editNotText
	editReadOnly
	editTooLong
	editNotSaved
)

// EDIT facility status codes.
const (
	// EDIT_COMMAND reports an editor's command that can't be run as written.
	EDIT_COMMAND = EDITFacility<<FacilityPosition | editCommand<<MessagePosition | StatusError
	// EDIT_SCRATCH reports a scratch directory that can't be used.
	EDIT_SCRATCH = EDITFacility<<FacilityPosition | editScratch<<MessagePosition | StatusError
	// EDIT_EDITOR reports an editor that couldn't be run or ended with a
	// failure, which leaves the file as it was.
	EDIT_EDITOR = EDITFacility<<FacilityPosition | editEditor<<MessagePosition | StatusError
	// EDIT_NOTTEXT reports a file that can't be edited as lines of text.
	EDIT_NOTTEXT = EDITFacility<<FacilityPosition | editNotText<<MessagePosition | StatusError
	// EDIT_READONLY reports a file on a volume that can't take a new version.
	EDIT_READONLY = EDITFacility<<FacilityPosition | editReadOnly<<MessagePosition | StatusError
	// EDIT_TOOLONG reports an edited line too long for the file's records.
	EDIT_TOOLONG = EDITFacility<<FacilityPosition | editTooLong<<MessagePosition | StatusError
	// EDIT_NOTSAVED reports a new version that couldn't be written.
	EDIT_NOTSAVED = EDITFacility<<FacilityPosition | editNotSaved<<MessagePosition | StatusError
)

func init() {
	DefineMessage(EDIT_COMMAND, EDITFacility, "COMMAND", "the editor's command !S can't be used: !S")
	DefineMessage(EDIT_SCRATCH, EDITFacility, "SCRATCH", "the scratch directory !S can't be used: !S")
	DefineMessage(EDIT_EDITOR, EDITFacility, "EDITOR", "the editor !S failed: !S; !S is unchanged")
	DefineMessage(EDIT_NOTTEXT, EDITFacility, "NOTTEXT", "!S can't be edited as text: !S")
	DefineMessage(EDIT_READONLY, EDITFacility, "READONLY", "!S can't be edited: its volume is mounted read-only")
	DefineMessage(EDIT_TOOLONG, EDITFacility, "TOOLONG", "line !S is !S characters long, too long for !S, whose records hold !S; it is unchanged")
	DefineMessage(EDIT_NOTSAVED, EDITFacility, "NOTSAVED", "!S is unchanged: !S")
}
