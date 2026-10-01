package console

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/ods2/ondisk"
)

// This file implements the console's CREATE/DIRECTORY (docs/PHASE-34.md,
// subtask 6). The work is internal/rms.Session.CreateDirectory's; this file
// reads the qualifiers, supplies the process's UIC, and reports each
// directory as VMS's CREATE does.

// CreateDirectoryRequest is CREATE/DIRECTORY's command line: its
// directories and qualifiers, as typed.
type CreateDirectoryRequest struct {
	// Directories are the directory specifications, in order.
	Directories []string

	// OwnerUIC is /OWNER_UIC's value: a UIC, "[g,m]", or PARENT; "" for
	// none.
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

// processUIC is the UIC CREATE/DIRECTORY gives a directory's owner when
// /OWNER_UIC doesn't say otherwise: the emulated process's, or the nominal
// SYSTEM UIC, [1,4], before INIT has made a process.
func (c *Console) processUIC() ondisk.Uic {
	uic := uint32(rtl.NominalUIC)
	if c.RTL != nil && c.RTL.Process != nil {
		uic = c.RTL.Process.UIC
	}

	return ondisk.Uic{Group: uint16(uic >> 16), Member: uint16(uic)}
}

// CreateDirectory implements CREATE/DIRECTORY: each directory in req is
// made, with every missing directory above it. With /LOG, each directory
// made is reported with %CREATE-I-CREATED; a directory that already existed
// is reported with %CREATE-I-EXISTS whether or not /LOG is given. A
// directory that can't be made is reported, and the rest of the list is
// still tried; the command then fails with the first such error.
func (c *Console) CreateDirectory(req CreateDirectoryRequest) error {
	opts, err := c.createDirectoryOptions(req)
	if err != nil {
		return err
	}

	var first error

	for _, spec := range req.Directories {
		created, err := c.ContainerSession.CreateDirectory(spec, opts)

		for i, d := range created {
			switch {
			case d.Created && req.Log:
				c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_CREATED, d.Name))
			case !d.Created && i == len(created)-1 && err == nil:
				c.Printf("%%%s\n", vmserrors.New(vmserrors.CREATE_EXISTS, d.Name))
			}
		}

		if err == nil {
			continue
		}

		failure := c.createDirectoryFailure(err, spec)

		if first == nil {
			first = failure
		}

		if len(req.Directories) > 1 {
			c.Printf("%%%s\n", failure)
		}
	}

	// With several directories, each failure was reported as it happened,
	// so the command's own failure is only for its exit status.
	if first != nil && len(req.Directories) > 1 {
		return vmserrors.InhibitMessage(first)
	}

	return first
}

// createDirectoryOptions turns req's qualifiers into
// rms.CreateDirectoryOptions, checking their values.
func (c *Console) createDirectoryOptions(req CreateDirectoryRequest) (rms.CreateDirectoryOptions, error) {
	opts := rms.CreateDirectoryOptions{ProcessUIC: c.processUIC()}

	switch owner := strings.TrimSpace(req.OwnerUIC); {
	case owner == "":
	case strings.EqualFold(owner, "PARENT"):
		opts.OwnerParent = true
	default:
		uic, err := ondisk.ParseUic(owner)
		if err != nil {
			return opts, vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, "/OWNER_UIC="+owner)
		}

		opts.Owner = &uic
	}

	if req.VersionLimit != nil {
		n := *req.VersionLimit
		if n < 0 || n > rms.MaxVersionLimit {
			return opts, vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER,
				fmt.Errorf("the version limit must be 0 to %d", rms.MaxVersionLimit), fmt.Sprintf("/VERSION_LIMIT=%d", n))
		}

		limit := uint16(n)
		opts.VersionLimit = &limit
	}

	if len(req.Protection) > 0 {
		text := "(" + strings.Join(req.Protection, ",") + ")"
		if _, err := ondisk.ParseProtection(text, 0); err != nil {
			return opts, vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER, err, "/PROTECTION="+text)
		}

		opts.Protection = text
	}

	if req.Allocation < 0 {
		return opts, vmserrors.Wrap(vmserrors.CLI_BADQUALIFIER,
			errors.New("the allocation must be a number of blocks"), fmt.Sprintf("/ALLOCATION=%d", req.Allocation))
	}

	opts.Allocation = uint32(req.Allocation)

	return opts, nil
}

// createDirectoryFailure reports a directory that couldn't be made as
// %CREATE-E-DIRNOTCRE, with its cause; an unmounted device as
// SS_DEVNOTMOUNT, as other file commands report it.
func (c *Console) createDirectoryFailure(err error, spec string) error {
	if lnmErr := logicalNameFailure(err); lnmErr != nil {
		return lnmErr
	}

	var notMounted *rms.NotMountedError
	if errors.As(err, &notMounted) {
		return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
	}

	return vmserrors.Wrap(vmserrors.CREATE_DIRNOTCRE, err, spec)
}
