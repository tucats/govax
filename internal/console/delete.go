package console

import (
	"errors"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements the console DELETE command (docs/PHASE-23.md,
// subtask 6) — the operator-facing half of "reclaim a file's storage and
// remove its directory entry". The actual work (glob-matching, the
// required-version check, the DeleteFile/bitmap-flush loop) already lives
// in the sibling internal/rms package's Session.Delete (internal/rms/
// delete.go, this same subtask); this file is a thin, console-specific
// layer on top of it, mirroring directory.go's own Console.Directory.
//
// # A note for a reader new to Go
//
// The method below has the receiver "(c *Console)", meaning it's called as
// c.Delete(...) on a *Console value. Like Console.Directory, it tells
// Session.Delete's different failure modes apart via errors.As (rather than
// inspecting error text) so each is reported as the specific VMS status a
// real operator would see for that exact condition.

// Delete removes every file matching specText from its resolved volume,
// printing one "%DELETE-S-DELETED, NAME.TYPE;version deleted" confirmation
// line per file actually removed — see internal/rms.Session.Delete's own
// doc comment for exactly how specText is resolved, and why it must include
// a specific version.
//
// Session.Delete's failures are told apart here so each is reported as the
// VMS status a real operator would actually see for that specific
// condition, rather than one generic error for all of them:
//
//   - specText naming a device with nothing currently mounted on it
//     surfaces as an *rms.NotMountedError and is reported as
//     SS_DEVNOTMOUNT, matching Console.Directory's own translation for the
//     same underlying condition.
//   - specText with no specific version surfaces as an
//     *rms.VersionRequiredError and is reported as CLI_NEEDVERSION.
//   - specText matching no file at all on its volume surfaces as an
//     *rms.NotFoundError and is reported as SS_NOSUCHFILE.
//   - Anything else (a malformed file specification, an unsupported multi-
//     device volume set, or a genuine I/O failure freeing storage) is
//     reported as CLI_BADFILESPEC, matching Console.Directory's own
//     catch-all bucket.
func (c *Console) Delete(specText string) error {
	deleted, err := c.ContainerSession.Delete(specText)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		var versionRequired *rms.VersionRequiredError
		if errors.As(err, &versionRequired) {
			return vmserrors.Wrap(vmserrors.CLI_NEEDVERSION, err, versionRequired.Spec)
		}

		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) {
			return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, notFound.Spec)
		}

		return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, specText)
	}

	for _, d := range deleted {
		c.Printf("%%DELETE-S-DELETED, %s.%s;%d deleted\n", d.Name, d.Type, d.Version)
	}

	return nil
}
