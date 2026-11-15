package dclcmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW SECURITY and SHOW ACL (docs/planning/PHASE-93 - file
// protection.md, subtask 12): a file's owner, protection, and access
// control list, as VMS 7.1 showed them (A.4, "Display"):
//
//	$ SHOW SECURITY DUA1:[PROT93.SEC]S1.DAT
//
//	DUA1:[PROT93.SEC]S1.DAT;1 object of class FILE
//	     Owner: [SYSTEM]
//	     Protection: (System: RWED, Owner: RWED, Group: R, World)
//	     Access Control List: <empty>
//
// SHOW ACL is VMS's older form, the entries alone under a heading with
// the time. Both find their files as SET SECURITY does (wildcards, a name
// with no version its highest version), and need read or control access
// to each, as the ACP's read of a file's attributes does.

// showFacility is the SHOW utility's facility number, which its condition
// values carry (unconfirmed: the probe's logs show SHOW's messages, not
// their values).
const showFacility = 120

// SHOW's messages: shared messages (SHR$) in SHOW's facility, at the
// logs' severities (READERR F; OPENIN E, unconfirmed: no probe showed
// SHOW SECURITY of a missing file).
var (
	showReadErr = showFacility<<16 | vmsdef.LibrarySymbols["SHR$_READERR"]&^7 | 4
	showOpenIn  = showFacility<<16 | vmsdef.LibrarySymbols["SHR$_OPENIN"]&^7 | 2
)

// ShowSecurity is SHOW SECURITY spec, with /CLASS's object class: FILE
// (the default) or VOLUME; another is DCL's IVKEYW (VMS 7.1 has more
// classes, which govax doesn't).
func (c *Context) ShowSecurity(spec, class string) error {
	if c.ContainerSession == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	switch class = strings.ToUpper(strings.TrimSpace(class)); {
	case class == "" || strings.HasPrefix("FILE", class):
	case strings.HasPrefix("VOLUME", class):
		return c.showVolumeSecurity(spec)
	default:
		return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, class)
	}

	return c.eachShownObject(spec, func(o *rms.SecurityObject) {
		names := rightsNames{c}
		p := o.Profile()

		c.Printf("\n%s object of class FILE\n", o.Name)
		c.Printf("     Owner: %s\n", security.IdentifierText(uint32(p.Owner), names))
		c.Printf("     Protection: %s\n", ProtectionDisplay(p.Protection, "RWED"))

		entries, _ := security.DecodeACL(o.ACL())
		if len(entries) == 0 {
			c.Printf("     Access Control List: <empty>\n")

			return
		}

		c.Printf("     Access Control List:\n")

		for _, e := range entries {
			c.Printf("          %s\n", security.FormatACE(e.Data, names, nil))
		}
	})
}

// ShowACL is SHOW ACL spec: each file's heading, its class, name, and the
// time, and its entries indented ten, as VMS 7.1's:
//
//	Object type: FILE,  Object name: DUA1:[PROT93.SEC]S14.DAT;1,  on 14-NOV-2026 09:05:39.99
//	          (IDENTIFIER=[PROT93,PROT93A],ACCESS=READ)
//
// A file with no entries has the heading alone (unconfirmed: the probe's
// file had one).
func (c *Context) ShowACL(spec string) error {
	if c.ContainerSession == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	return c.eachShownObject(spec, func(o *rms.SecurityObject) {
		c.Printf("Object type: FILE,  Object name: %s,  on %s\n", o.Name, vmsTimeText(time.Now()))

		entries, _ := security.DecodeACL(o.ACL())
		for _, e := range entries {
			c.Printf("          %s\n", security.FormatACE(e.Data, rightsNames{c}, nil))
		}
	})
}

// eachShownObject calls show for each file spec names that the process may
// read the profile of. A file it may not is SHOW's READERR over
// SS$_NOPRIV, which ends the command, as its severity says (VMS 7.1: the
// probe's accounts' runs; whether VMS went on to the next file of a
// wildcard is unconfirmed). A file not found is SHOW's OPENIN over RMS's
// status (unconfirmed, after SET SECURITY's OPENOUT).
func (c *Context) eachShownObject(spec string, show func(*rms.SecurityObject)) error {
	report := &securityReport{c: c}

	objects, err := c.ContainerSession.SecurityObjects(spec)
	if err != nil {
		var fileErr *rms.DCLFileError

		switch denied, ok := rms.IsDenied(err); {
		case errors.As(err, &fileErr):
			secondary := []uint32{fileErr.Status}
			if fileErr.STV != 0 {
				secondary = append(secondary, fileErr.STV)
			}

			report.message(showOpenIn, "SHOW-E-OPENIN, error opening "+fileErr.Name+" as input", secondary...)
		case ok:
			name := c.expandedName(spec)
			if denied != nil && denied.Name != "" {
				name = denied.Name
			}

			report.message(showOpenIn, "SHOW-E-OPENIN, error opening "+name+" as input", rmsPRV)
		default:
			return err
		}

		return report.result()
	}

	for _, o := range objects {
		if !o.MayRead() {
			report.message(showReadErr, "SHOW-F-READERR, error reading "+o.Name, ssNOPRIV)

			break
		}

		show(o)
	}

	return report.result()
}

// showVolumeSecurity is SHOW SECURITY/CLASS=VOLUME device: the volume's
// label, owner, and protection, its letters a volume's (C, create, in
// place of a file's E), as VMS 7.1's:
//
//	PROT93 object of class VOLUME
//	     Owner: [SYSTEM]
//	     Protection: (System: RWCD, Owner: RWCD, Group: RWCD, World: RWCD)
//	     Access Control List: <empty>
//
// govax keeps no ACL for a volume.
func (c *Context) showVolumeSecurity(spec string) error {
	label, owner, protection, err := c.ContainerSession.VolumeProfile(spec)
	if err != nil {
		var notMounted *rms.NotMountedError
		if errors.As(err, &notMounted) {
			return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
		}

		return FileFailure(err, spec)
	}

	c.Printf("\n%s object of class VOLUME\n", label)
	c.Printf("     Owner: %s\n", security.IdentifierText(uint32(owner), rightsNames{c}))
	c.Printf("     Protection: %s\n", ProtectionDisplay(protection, "RWCD"))
	c.Printf("     Access Control List: <empty>\n")

	return nil
}

// ProtectionDisplay is a protection code as SHOW SECURITY writes it,
// "(System: RWED, Owner: RWED, Group: R, World)", a category with no
// access its name alone; letters are the class's names for the code's
// four bits in order (a file's "RWED", a volume's "RWCD").
func ProtectionDisplay(p security.Protection, letters string) string {
	bits := []security.Access{security.Read, security.Write, security.Execute, security.Delete}
	parts := make([]string, 0, 4)

	for c, name := range []string{"System", "Owner", "Group", "World"} {
		allowed := p.Allows(security.Category(c))

		var b strings.Builder

		for i, bit := range bits {
			if allowed&bit != 0 {
				b.WriteByte(letters[i])
			}
		}

		if b.Len() == 0 {
			parts = append(parts, name)
		} else {
			parts = append(parts, name+": "+b.String())
		}
	}

	return "(" + strings.Join(parts, ", ") + ")"
}

// vmsTimeText is a time as $ASCTIM writes it, "14-NOV-2026 09:05:39.99",
// a one-digit day after a blank.
func vmsTimeText(t time.Time) string {
	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d.%02d", t.Day(), strings.ToUpper(t.Month().String()[:3]),
		t.Year(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/10_000_000)
}
