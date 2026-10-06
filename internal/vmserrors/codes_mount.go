package vmserrors

// MOUNT facility message IDs -- the console's MOUNT command and the
// default volume govax mounts at startup (internal/console/defvolume.go).
// Private: only the composite MOUNT_* codes below are part of this
// package's public API.
const (
	mountMounted uint32 = iota + 1
	mountWriteLock
	mountCreated
	mountNoDefault
	mountNoDir
)

// MOUNT facility status codes.
const (
	// MOUNT_MOUNTED reports a volume mounted on a device, worded as VMS's
	// MOUNT reports it: "WORK mounted on _DUA0:".
	MOUNT_MOUNTED = MOUNTFacility<<FacilityPosition | mountMounted<<MessagePosition | StatusInfo
	// MOUNT_WRITELOCK reports, before MOUNT_MOUNTED, a volume that could
	// only be mounted read-only.
	MOUNT_WRITELOCK = MOUNTFacility<<FacilityPosition | mountWriteLock<<MessagePosition | StatusInfo
	// MOUNT_CREATED reports a container govax created and initialized for
	// the default volume because it didn't exist yet (govax's own).
	MOUNT_CREATED = MOUNTFacility<<FacilityPosition | mountCreated<<MessagePosition | StatusInfo
	// MOUNT_NODEFAULT reports a configured default volume that can't be
	// used; its cause says why. Startup goes on without it.
	MOUNT_NODEFAULT = MOUNTFacility<<FacilityPosition | mountNoDefault<<MessagePosition | StatusWarning
	// MOUNT_NODIR reports a default volume's configured directory that
	// isn't on the volume; the default directory is the MFD instead.
	MOUNT_NODIR = MOUNTFacility<<FacilityPosition | mountNoDir<<MessagePosition | StatusWarning
)

func init() {
	DefineMessage(MOUNT_MOUNTED, MOUNTFacility, "MOUNTED", "!S mounted on !S")
	DefineMessage(MOUNT_WRITELOCK, MOUNTFacility, "WRITELOCK", "volume is write locked")
	DefineMessage(MOUNT_CREATED, MOUNTFacility, "CREATED", "container !S created as an !S volume labeled !S")
	DefineMessage(MOUNT_NODEFAULT, MOUNTFacility, "NODEFAULT", "default volume !S not mounted")
	DefineMessage(MOUNT_NODIR, MOUNTFacility, "NODIR", "directory !S not found on !S; using [000000]")
}
