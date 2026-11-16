package sysvol

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/security"
)

// The system volume's protection (docs/planning/PHASE-95 -
// authentication.md, Decision 9). On VMS the system's own files are
// owned by [1,4], SYSTEM's UIC, and anyone may read them, but only their
// owner (and the system category, which counts as the owner for these
// files) may change them. Once govax has accounts other than SYSTEM,
// the same must hold on its system volume: a user can read
// SYS$MANAGER:SYLOGIN.COM to run it, or SYS$LIBRARY:STARLET.MLB to
// assemble with it, but not delete SYS$SYSTEM:DEFAULT.ROM.
//
// A protection code has four categories, SYSTEM, OWNER, GROUP, and WORLD,
// each given some of read (R), write (W), execute (E), and delete (D)
// access. Directories get no D for anyone: a directory is deleted by
// first changing its protection, as on VMS.
//
// Refresh sets each file's and directory's profile and records it in the
// manifest (an OWNER= and a PROTECTION= word on its record). A profile
// the manifest already records isn't set again, so a protection the user
// changes with SET SECURITY afterwards stays; a later govax that wants
// another profile sets it once more. A new version of a file takes its
// previous version's protection (RMS's rule), so an update keeps the
// user's change too; a file made where there was none gets govax's.

// Profile is a file's or directory's owner and protection code.
type Profile struct {
	Owner      security.UIC
	Protection security.Protection
}

// SystemOwner is [1,4], SYSTEM's UIC, the owner of everything on the
// system volume.
var SystemOwner = security.NewUIC(1, 4)

// The profiles Decision 9 gives: files (S:RWED,O:RWED,G:RE,W:RE), and
// directories (S:RWE,O:RWE,G:RE,W:RE), as VMS's own system files and
// directories have. The page file, which internal/rms makes, keeps its
// own (S:RWED,O:RWED,G,W): nobody but SYSTEM may read what was paged.
var (
	FileProfile      = Profile{Owner: SystemOwner, Protection: mustProtection("(S:RWED,O:RWED,G:RE,W:RE)")}
	DirectoryProfile = Profile{Owner: SystemOwner, Protection: mustProtection("(S:RWE,O:RWE,G:RE,W:RE)")}
)

// mustProtection reads a protection code's text, which names all four
// categories, for the profiles above.
func mustProtection(text string) security.Protection {
	p, err := security.ParseProtection(text, security.NoAccess)
	if err != nil {
		panic("sysvol: " + text + ": " + err.Error())
	}

	return p
}

// Profile is the owner and protection the file has on the volume.
func (f File) Profile() Profile {
	return FileProfile
}

// DirectoryFile is the specification of the directory file that holds
// the directory dir: "[SYSTEM.SYSLIB]" is "[SYSTEM]SYSLIB.DIR", and
// "[SYSTEM]", a top-level directory, "[000000]SYSTEM.DIR", in the
// volume's master file directory. A directory's protection is its
// directory file's.
func DirectoryFile(dir string) string {
	inner := strings.TrimSuffix(strings.TrimPrefix(dir, "["), "]")

	parent, name := "000000", inner
	if i := strings.LastIndex(inner, "."); i >= 0 {
		parent, name = inner[:i], inner[i+1:]
	}

	return "[" + parent + "]" + name + ".DIR"
}

// String writes the profile as the manifest's two words:
// "OWNER=[1,4] PROTECTION=(S:RWED,O:RWED,G:RE,W:RE)".
func (p Profile) String() string {
	return ownerWord + p.Owner.String() + " " + protectionWord + protectionText(p.Protection)
}

// protectionText writes a protection code as SET SECURITY/PROTECTION
// reads it, every category named: "(S:RWED,O:RWED,G:RE,W)".
func protectionText(p security.Protection) string {
	parts := make([]string, 0, 4)

	for c, letter := range []string{"S", "O", "G", "W"} {
		access := p.Allows(security.Category(c)).String()
		if access == "" {
			parts = append(parts, letter)
		} else {
			parts = append(parts, letter+":"+access)
		}
	}

	return "(" + strings.Join(parts, ",") + ")"
}

// parseProfile reads a profile from the manifest's two words, the owner's
// and the protection's, each with its keyword.
func parseProfile(owner, protection string) (Profile, error) {
	uic, err := security.ParseUIC(owner[len(ownerWord):], nil)
	if err != nil {
		return Profile{}, fmt.Errorf("%q: %w", owner, err)
	}

	code, err := security.ParseProtection(protection[len(protectionWord):], security.NoAccess)
	if err != nil {
		return Profile{}, fmt.Errorf("%q: %w", protection, err)
	}

	return Profile{Owner: uic, Protection: code}, nil
}

// applyProfile gives the file spec (which exists) the profile want, if
// the manifest m doesn't already record it there, or if made is set: the
// file is new, made where there was none, and so has the profile RMS gave
// it, not govax's. Every version of the file gets it, so an existing
// volume's older versions are corrected too.
func applyProfile(v Volume, m *Manifest, spec string, want Profile, made bool) error {
	if recorded, ok := m.Profile(spec); ok && recorded == want && !made {
		return nil
	}

	if err := v.SetProfile(spec, want); err != nil {
		return fmt.Errorf("%s: %w", spec, err)
	}

	m.SetProfile(spec, want)

	return nil
}
