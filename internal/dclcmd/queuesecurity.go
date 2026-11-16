package dclcmd

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/jobqueue"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW SECURITY/CLASS=QUEUE and SET SECURITY/CLASS=QUEUE
// (docs/planning/PHASE-98 - batch queues.md, subtask 5, Decision 8): a
// batch queue's owner, protection, and ACL, read and changed through
// corevms's queue protection (QueueProfile, ChangeQueueSecurity), as VMS
// 7.1 showed them (testdata/bat98):
//
//	$ SHOW SECURITY/CLASS=QUEUE BAT98_A
//
//	BAT98_A object of class QUEUE
//	     Owner: [SYSTEM]
//	     Protection: (System: M, Owner: D, Group: R, World: S)
//	     Access Control List: <empty>
//
// A queue's letters are R (read), S (submit), M (manage), and D
// (delete); an ACE's access words READ, SUBMIT, MANAGE, DELETE, and
// CONTROL. A refusal is SS$_NOPRIV alone, as VMS 7.1's was for both
// commands, with no SET or SHOW message around it.

// isQueueClass reports whether /CLASS's value names the QUEUE class.
func isQueueClass(class string) bool {
	class = strings.ToUpper(strings.TrimSpace(class))

	return class != "" && strings.HasPrefix("QUEUE", class)
}

// queueCondition is the failure whose condition value is code, its
// message shown and the status's inhibit bit set, as DCL leaves $STATUS
// after showing it (%X10000024 for SS$_NOPRIV, %X1004803A for
// JBC$_NOSUCHQUE).
func queueCondition(code uint32) error {
	return vmserrors.ConditionError{Status: code | stsInhibitMsg, Text: vmsdef.FormatMessage(code)}
}

// queueFailure is err, from a queue operation, as DCL's failure.
func queueFailure(err error) error {
	if st := corevms.QueueErrorStatus(err); st != 0 {
		return queueCondition(st)
	}

	return err
}

// showQueueSecurity is SHOW SECURITY/CLASS=QUEUE name.
func (c *Context) showQueueSecurity(spec string) error {
	if c.RTL == nil {
		return queueCondition(vmsdef.Symbols["JBC$_NOSUCHQUE"])
	}

	name, p, status, err := c.RTL.QueueProfile(spec)
	if err != nil {
		return err
	}

	if status != 0 {
		return queueCondition(status)
	}

	names := rightsNames{c}

	c.Printf("\n%s object of class QUEUE\n", name)
	c.Printf("     Owner: %s\n", security.IdentifierText(uint32(p.Owner), names))
	c.Printf("     Protection: %s\n", ProtectionDisplay(p.Protection, "RSMD"))

	if len(p.ACL) == 0 {
		c.Printf("     Access Control List: <empty>\n")
	} else {
		c.Printf("     Access Control List:\n")

		for _, e := range p.ACL {
			c.Printf("          %s\n", security.FormatACE(e.Data, names, security.QueueAccessNames))
		}
	}

	// VMS 7.1 left $STATUS SS$_NORMAL, %X00000001, after both commands.
	c.SetStatus(ssNormal)

	return nil
}

// setQueueSecurity is SET SECURITY/CLASS=QUEUE name: /PROTECTION (a
// queue's letters), /OWNER, /ACL with /AFTER, /REPLACE, and /DELETE,
// /DEFAULT (the class's template profile, [SYSTEM] and
// (S:M,O:D,G:R,W:S), with no ACL), and /LOG. /LIKE, /EXCLUDE, and
// /BY_OWNER choose among files, and are DCL's CONFLICT here
// (unconfirmed: the probe used none of them on a queue).
func (c *Context) setQueueSecurity(opts SecurityOptions) error {
	if opts.Like != "" || len(opts.Exclude) > 0 || opts.ByOwner != "" {
		return vmserrors.New(vmserrors.CLI_CONFLICT)
	}

	ch := &securityChange{opts: opts, names: rightsNames{c}, access: security.QueueAccessNames, queue: true}
	if c.RTL != nil {
		ch.creds = c.RTL.Credentials()
	}

	if err := ch.read(c); err != nil {
		return err
	}

	if c.RTL == nil {
		return queueCondition(vmsdef.Symbols["JBC$_NOSUCHQUE"])
	}

	report := &securityReport{c: c}

	for _, spec := range opts.Files {
		name := strings.ToUpper(strings.TrimSpace(spec))
		missing := []byte(nil)

		err := c.RTL.ChangeQueueSecurity(name, func(s *corevms.QueueSecurity) error {
			var ok bool

			missing, ok = ch.changeQueue(s)
			if !ok {
				return errACEMissing
			}

			return nil
		})

		switch {
		case errors.Is(err, errACEMissing):
			report.message(setNoSuchACE, "SET-W-NOSUCHACE, ACE for object "+name+", does not exist")
			c.Printf("    %s\n", security.FormatACE(missing, ch.names, ch.access))
			report.message(setWriteErr, "SET-F-WRITEERR, error writing "+name)
		case err != nil:
			return queueFailure(err)
		case opts.Log:
			report.message(setModified, "SET-I-MODIFIED, "+name+" modified")
		}
	}

	if err := report.result(); err != nil {
		return err
	}

	c.SetStatus(ssNormal)

	return nil
}

// errACEMissing stops a queue's change when an ACE /DELETE, /REPLACE, or
// /AFTER names isn't in its ACL.
var errACEMissing = errors.New("dclcmd: an ACE the command names isn't in the queue's ACL")

// changeQueue makes the options' change to a queue's profile s. It
// returns false, with the entry, when an ACE the options name isn't
// there.
func (ch *securityChange) changeQueue(s *corevms.QueueSecurity) ([]byte, bool) {
	opts := ch.opts

	if opts.Default {
		s.Owner = security.UIC(jobqueue.SystemUIC)
		s.Protection = security.Protection(jobqueue.DefaultProtection)
		s.ACL = nil
	}

	if opts.HasProtection {
		code := codeText(opts.Protection)
		if code == "" {
			s.Protection = security.Protection(jobqueue.DefaultProtection)
		} else {
			p, _ := jobqueue.ParseProtection(code, jobqueue.Protection(s.Protection))
			s.Protection = security.Protection(p)
		}
	}

	if opts.HasOwner {
		s.Owner = ch.owner
	}

	if opts.HasACL || opts.HasDelete {
		acl, missing, ok := ch.editACL(s.ACL)
		if !ok {
			return missing, false
		}

		s.ACL = acl
	}

	return nil, true
}
