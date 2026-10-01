package vmserrors

// CREATE facility message IDs -- the console's CREATE command
// (docs/PHASE-34.md). Private: only the composite CREATE_* codes below are
// part of this package's public API.
const (
	createCreated uint32 = iota + 1
	createExists
	createDirNotCre
	createSyntax
	createBadValue
)

// CREATE facility status codes, named as VMS's CREATE utility names its
// messages.
const (
	// CREATE_CREATED reports, with /LOG, a directory CREATE/DIRECTORY
	// made.
	CREATE_CREATED = CREFacility<<FacilityPosition | createCreated<<MessagePosition | StatusInfo
	// CREATE_EXISTS reports a directory CREATE/DIRECTORY was asked for
	// that already exists -- not a failure.
	CREATE_EXISTS = CREFacility<<FacilityPosition | createExists<<MessagePosition | StatusInfo
	// CREATE_DIRNOTCRE reports a directory CREATE/DIRECTORY couldn't make;
	// its cause says why.
	CREATE_DIRNOTCRE = CREFacility<<FacilityPosition | createDirNotCre<<MessagePosition | StatusError
	// CREATE_SYNTAX reports a qualifier value CREATE can't parse, such as
	// a malformed /OWNER_UIC; nothing is created.
	CREATE_SYNTAX = CREFacility<<FacilityPosition | createSyntax<<MessagePosition | StatusSevere
	// CREATE_BADVALUE reports a qualifier value out of range, such as
	// /VERSION_LIMIT=40000. As on VMS, the qualifier is then ignored and
	// the directory is still created.
	CREATE_BADVALUE = CREFacility<<FacilityPosition | createBadValue<<MessagePosition | StatusError
)

func init() {
	DefineMessage(CREATE_CREATED, CREFacility, "CREATED", "!S created")
	DefineMessage(CREATE_EXISTS, CREFacility, "EXISTS", "!S already exists")
	// The texts are VMS 7.3's, from its run of govax's Phase 34 oracle
	// (testdata/credir).
	DefineMessage(CREATE_DIRNOTCRE, CREFacility, "DIRNOTCRE", "!S directory file not created")
	DefineMessage(CREATE_SYNTAX, CREFacility, "SYNTAX", "error parsing '!S'")
	DefineMessage(CREATE_BADVALUE, CREFacility, "BADVALUE", "'!S' is an invalid keyword value")
}
