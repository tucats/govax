package vmserrors

// CREATE facility message IDs -- the console's CREATE command
// (docs/PHASE-34.md). Private: only the composite CREATE_* codes below are
// part of this package's public API.
const (
	createCreated uint32 = iota + 1
	createExists
	createDirNotCre
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
)

func init() {
	DefineMessage(CREATE_CREATED, CREFacility, "CREATED", "!S created")
	DefineMessage(CREATE_EXISTS, CREFacility, "EXISTS", "!S already exists")
	DefineMessage(CREATE_DIRNOTCRE, CREFacility, "DIRNOTCRE", "unable to create directory !S")
}
