package rms

// This file defines the real, literal RMS$_ completion-status values that
// this package's service handlers write into a FAB's or RAB's status
// fields (fabSTS/fabSTV, rabSTS/rabSTV — see fab.go/rab.go) after every
// call.
//
// # Why these have to be the *real* VMS values
//
// A real, unmodified VAX/VMS program checks the status a system service
// returns against symbolic names like RMS$_NORMAL or RMS$_EOF, which its
// compiler or assembler has already turned into specific fixed numbers
// before govax ever sees the resulting binary. If this package invented
// its own numbering instead of using VMS's real ones, a real program's
// "if status is RMS$_EOF, stop reading" check would silently never match,
// even though this package's own Go-side error handling looked
// internally consistent. So — unlike, say, the small integer file-handle
// numbers ifi.go's FileTable hands out, which are purely this package's
// own bookkeeping and never need to match anything else — these numbers
// are not this project's to choose.
//
// This is exactly the same kind of "must match a fixed external
// contract" situation this package's Environment (internal/rtl) already
// tracks separately in internal/rtl/status.go for VMS's *other* status
// family, SS$_ codes (the general system-service completion codes used
// outside RMS, e.g. for SYS$ASSIGN or SYS$MOUNT). This package's own
// RMS$_ table is kept here instead of merged into that one, matching
// docs/PHASE-22.md's design: RMS$_ values live in a different numbering
// space (VMS's "RMS" facility) from internal/rtl's SS$_ values (VMS's
// "system services" facility), and mixing the two tables would make it
// easy to accidentally write an SS$_ number into an RMS$_ field or vice
// versa.
//
// # Where these numbers come from
//
// The values below are the real, literal $RMSDEF completion codes. Read
// from internal/vmsdef.Constants (Phase 24, docs/PHASE-24.md — the complete
// 267-entry RMS$_ table, machine-generated from reference/vms/rmsdef.h)
// rather than their own literal copies; cross-checked one-for-one against
// the values this file carried before that migration, which were
// themselves cross-checked directly against a real VAX/VMS 7.3 system's
// own rmsdef.h during Phase 22's implementation (docs/PHASE-22.md) — not
// derived from reference/eVAX (which has no RMS$_ status table at all;
// its rms.c only ever returns the generic SS$_NORMAL) and not guessed.
// Only the subset this phase's SYS$CREATE/SYS$CONNECT/SYS$OPEN/
// SYS$CLOSE/SYS$GET/SYS$PUT handlers actually need is named here; like
// internal/rtl/status.go's own SS$_ table, more can be named later as
// later handlers need them — the full set is already in
// internal/vmsdef.Constants regardless, this is just this package's own
// "the subset I actually use" convenience list.
var (
	// rmsNormal is RMS$_NORMAL (== RMS$_SUC): ordinary success. This is
	// the value storeStatus (below) writes for every call that completed
	// with nothing to report.
	rmsNormal = vmsConst("RMS$_NORMAL")

	// rmsCreated is RMS$_CREATED: an alternate *success* status (not an
	// error!) SYS$CREATE returns to specifically say "a new file was
	// created" — as opposed to, say, SYS$OPEN's ordinary RMS$_NORMAL for
	// opening one that already existed. A real VAX program can tell
	// these apart if it cares to.
	rmsCreated = vmsConst("RMS$_CREATED")

	// rmsEOF is RMS$_EOF: SYS$GET returns this once a sequential file's
	// records have all been read — the RMS equivalent of Go's io.EOF.
	rmsEOF = vmsConst("RMS$_EOF")

	// rmsFileExists is RMS$_FEX: the target of SYS$CREATE already
	// exists and the caller didn't ask to supersede it.
	rmsFileExists = vmsConst("RMS$_FEX")

	// rmsFileLocked is RMS$_FLK: the target file is already open in a
	// way that conflicts with the requested access (real VMS's file-
	// sharing rules — see FAB$B_SHR in fab.h/the RMS manual).
	rmsFileLocked = vmsConst("RMS$_FLK")

	// rmsFileNotFound is RMS$_FNF: SYS$OPEN's target file spec doesn't
	// exist. Also the natural error for looking a name up in an
	// ods2 volume.Directory and not finding it.
	rmsFileNotFound = vmsConst("RMS$_FNF")

	// rmsPrivilegeViolation is RMS$_PRV: the caller lacks the access
	// (real VMS file protection, or in this phase's simpler terms:
	// asking to write through a device mounted read-only) needed for
	// the requested operation.
	rmsPrivilegeViolation = vmsConst("RMS$_PRV")

	// rmsRecordNotFound is RMS$_RNF: a requested record doesn't exist
	// (never was in the file, or has been deleted). Not reachable by
	// this phase's sequential-only SYS$GET (which only ever advances
	// forward until EOF), but named here since a well-formed status
	// table should include it alongside rmsEOF for future indexed/
	// keyed-access work.
	rmsRecordNotFound = vmsConst("RMS$_RNF")

	// rmsDeviceNotReady is RMS$_DNR: the target device isn't ready —
	// this phase's handlers use it when a file spec names a device that
	// has no volume currently mounted on it at all (see mount.go's
	// MountTable.Lookup).
	rmsDeviceNotReady = vmsConst("RMS$_DNR")

	// rmsDeviceError is RMS$_DEV: a generic device error, for problems
	// this phase's handlers can detect but that don't fit one of the
	// more specific codes above (for instance, an ods2 volume/diskimage
	// call failing for a reason that isn't "not found" or "locked").
	rmsDeviceError = vmsConst("RMS$_DEV")

	// rmsInvalidIFI is RMS$_IFI: the IFI (internal file index) a call
	// supplied doesn't identify a currently open file — this package's
	// own ifi.go FileTable.Lookup failing is reported this way.
	rmsInvalidIFI = vmsConst("RMS$_IFI")

	// rmsInvalidOrg is RMS$_ORG: the FAB's FAB$B_ORG (fab.go's fabORG)
	// named a file organization this phase doesn't implement (anything
	// other than orgSeq).
	rmsInvalidOrg = vmsConst("RMS$_ORG")

	// rmsInvalidRAC is RMS$_RAC: the RAB's RAB$B_RAC (rab.go's rabRAC)
	// named a record access mode this phase doesn't implement (anything
	// other than racSeq).
	rmsInvalidRAC = vmsConst("RMS$_RAC")

	// rmsInvalidRFM is RMS$_RFM: the FAB's FAB$B_RFM (fab.go's fabRFM)
	// named a record format SYS$CREATE doesn't recognize.
	rmsInvalidRFM = vmsConst("RMS$_RFM")

	// rmsInvalidVersion is RMS$_VER: a file specification's version field
	// (the text after ';' — filespec.Spec's own Version) named something
	// SYS$OPEN (open.go) doesn't know how to resolve to a single file —
	// this phase only supports an explicit positive version number, or no
	// version at all (meaning "the highest existing version", matching
	// ods2's own volume.Directory.Lookup convention for a 0 argument).
	// Real VMS's fuller version syntax (";*" for every version, ";-1" for
	// "N versions back from the highest") is a filespec.Glob-level wildcard
	// concern this phase's plain-file SYS$OPEN doesn't need.
	rmsInvalidVersion = vmsConst("RMS$_VER")

	// rmsRecordTooBig is RMS$_RSZ: a SYS$PUT record was longer than the
	// file's own declared maximum record size (fab.go's fabMRS), or a
	// SYS$GET record was longer than the caller's supplied buffer.
	rmsRecordTooBig = vmsConst("RMS$_RSZ")

	// rmsSystemError is RMS$_SYS: RMS's catch-all "the underlying
	// system/device reported an error" code, used when an ods2 call
	// fails for a reason this package's handlers can't map onto any
	// more specific RMS$_ value above.
	rmsSystemError = vmsConst("RMS$_SYS")
)

// storeStatus writes sts into both of a control block's status fields —
// fabSTS/fabSTV for a FAB, rabSTS/rabSTV for a RAB (fab.go/rab.go); base
// is the block's own VAX address, stsOffset/stvOffset are whichever pair
// applies — and returns sts unchanged as its own first result. That lets
// a handler end a failing branch with a single line like
//
//	return storeStatus(ctx, fabAddr, fabSTS, fabSTV, rmsFileNotFound)
//
// and have sts become both the FAB/RAB's own recorded completion status
// AND the value the calling VAX program sees in R0 (this package's
// handlers return that same (uint32, error) shape services.ServiceFunc
// expects — see docs/PHASE-22.md's subtask 11 for how internal/rtl wires
// that up). This mirrors real RMS, where a call's completion code is
// always both of those things at once, never just one: an unmodified VAX
// program is free to check either R0 right after the call, or FAB$L_STS
// later, and both always agree.
func storeStatus(ctx *Context, base, stsOffset, stvOffset uint32, sts uint32) (uint32, error) {
	if err := ctx.storeLongword(base+stsOffset, sts); err != nil {
		return 0, err
	}

	if err := ctx.storeLongword(base+stvOffset, sts); err != nil {
		return 0, err
	}

	return sts, nil
}
