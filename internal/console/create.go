package console

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
)

// This file implements the console's CREATE/DIRECTORY (docs/PHASE-34.md,
// subtask 6). The work is internal/rms.Session.CreateDirectory's; this file
// reads the qualifiers, supplies the process's UIC, and reports each
// directory as VMS 7.3's CREATE does (checked against its run of the
// Phase 34 oracle, testdata/credir).

// CreateDirectoryRequest is CREATE/DIRECTORY's command line: its
// directories and qualifiers, as typed.
type CreateDirectoryRequest struct {
	// Directories are the directory specifications, in order.
	Directories []string

	// OwnerUIC is /OWNER_UIC's value: a UIC, "[g,m]", or PARENT; "" for
	// none. Without one, or with PARENT, a new directory has its parent's
	// owner.
	OwnerUIC string

	// VersionLimit is /VERSION_LIMIT=n; nil without it.
	VersionLimit *int

	// Protection is /PROTECTION's list of items, "S:RWED" and so on; nil
	// without it.
	Protection []string

	// Allocation is /ALLOCATION=n; 0 without it.
	Allocation int

	// Log is /LOG: report each directory made.
	Log bool
}

// The secondary statuses CREATE/DIRECTORY's failures show, as VMS 7.3
// shows them (testdata/credir): a name too long or a path too deep is
// RMS$_DIR; a file name, wildcard, or "..." is LIB$_INVFILSPE; a device
// that isn't there is SS$_NOSUCHDEV; a bad UIC is SS$_IVIDENT.
var (
	rmsDIR        = vmsdef.Symbols["RMS$_DIR"]
	rmsCRE        = vmsdef.Symbols["RMS$_CRE"]
	ssIVIDENT     = vmsdef.Symbols["SS$_IVIDENT"]
	ssNOSUCHDEV   = vmsdef.Symbols["SS$_NOSUCHDEV"]
	ssDEVNOTMOUNT = vmsdef.Symbols["SS$_DEVNOTMOUNT"]
	ssWRITLCK     = vmsdef.Symbols["SS$_WRITLCK"]

	// libINVFILSPE is LIB$_INVFILSPE (LIB-F-INVFILSPE), which
	// vmsdef.Symbols doesn't carry; its message is in vmsdef.Messages.
	libINVFILSPE = uint32(0x15839C)
)

// CreateDirectory implements CREATE/DIRECTORY: each directory in req is
// made, with every missing directory above it, and reported as VMS 7.3's
// CREATE reports it:
//
//	%CREATE-I-CREATED, DUA1:[DEEP.A.B] created          (with /LOG)
//	%CREATE-I-EXISTS, [PLAIN] already exists
//	%CREATE-E-DIRNOTCRE, [WILD*] directory file not created
//	-LIB-F-INVFILSPE, invalid file specification
//
// /LOG reports only the directory asked for, not the ones made above it;
// EXISTS names it as typed, with or without /LOG. A directory that can't
// be made is reported, and the rest of the list is still tried. A bad
// /OWNER_UIC (CREATE-F-SYNTAX) stops the command before anything is made;
// a /VERSION_LIMIT out of range (CREATE-E-BADVALUE) is reported and then
// ignored, as VMS does. Every message is shown here, so a failure is
// returned with its message inhibited (exit status only).
func (c *Console) CreateDirectory(req CreateDirectoryRequest) error {
	opts, failure := c.createDirectoryOptions(req)
	if opts == nil {
		return failure
	}

	for _, spec := range req.Directories {
		created, err := c.ContainerSession.CreateDirectory(spec, *opts)
		if err != nil {
			c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_DIRNOTCRE, spec))
			c.Printf("%s\n", conditionLine(c.createDirectoryStatus(err)))

			if failure == nil {
				failure = vmserrors.InhibitMessage(vmserrors.Wrap(vmserrors.CREATE_DIRNOTCRE, err, spec))
			}

			continue
		}

		switch last := created[len(created)-1]; {
		case !last.Created:
			c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_EXISTS, spec))
		case req.Log:
			c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_CREATED, last.Name))
		}
	}

	return failure
}

// createDirectoryOptions turns req's qualifiers into
// rms.CreateDirectoryOptions, reporting a bad value as VMS does. A nil
// result means the command stops there; otherwise the error, if any, is a
// value that was reported and ignored, for the command's exit status.
func (c *Console) createDirectoryOptions(req CreateDirectoryRequest) (*rms.CreateDirectoryOptions, error) {
	opts := &rms.CreateDirectoryOptions{}

	syntax := func(value string, secondary uint32) (*rms.CreateDirectoryOptions, error) {
		c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_SYNTAX, value))

		if secondary != 0 {
			c.Printf("%s\n", conditionLine(secondary))
		}

		return nil, vmserrors.InhibitMessage(vmserrors.New(vmserrors.CREATE_SYNTAX, value))
	}

	switch owner := strings.TrimSpace(req.OwnerUIC); {
	case owner == "":
	case strings.EqualFold(owner, "PARENT"):
		// The parent's owner, which is also the default.
	default:
		uic, err := ondisk.ParseUic(owner)
		if err != nil {
			return syntax(owner, ssIVIDENT)
		}

		opts.Owner = &uic
	}

	if len(req.Protection) > 0 {
		text := "(" + strings.Join(req.Protection, ",") + ")"
		if _, err := ondisk.ParseProtection(text, 0); err != nil {
			return syntax(text, 0)
		}

		opts.Protection = text
	}

	var ignored error

	badValue := func(n int) {
		value := fmt.Sprint(n)
		c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_BADVALUE, value))

		if ignored == nil {
			ignored = vmserrors.InhibitMessage(vmserrors.New(vmserrors.CREATE_BADVALUE, value))
		}
	}

	if req.VersionLimit != nil {
		if n := *req.VersionLimit; n < 0 || n > rms.MaxVersionLimit {
			badValue(n)
		} else {
			limit := uint16(n)
			opts.VersionLimit = &limit
		}
	}

	if req.Allocation < 0 {
		badValue(req.Allocation)
	} else {
		opts.Allocation = uint32(req.Allocation)
	}

	return opts, ignored
}

// createDirectoryStatus is the secondary status VMS shows for err, a
// directory CREATE/DIRECTORY couldn't make.
func (c *Console) createDirectoryStatus(err error) uint32 {
	var notMounted *rms.NotMountedError

	switch {
	case errors.Is(err, volume.ErrDirectoryName):
		return rmsDIR
	case errors.Is(err, rms.ErrNotDirectorySpec):
		return libINVFILSPE
	case errors.Is(err, rms.ErrACPWriteLocked):
		return ssWRITLCK
	case errors.As(err, &notMounted):
		if _, known := c.Devices.Find(notMounted.Device); known {
			return ssDEVNOTMOUNT
		}

		return ssNOSUCHDEV
	}

	return rmsCRE
}
