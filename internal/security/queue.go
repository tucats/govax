package security

import (
	"fmt"
	"strings"
)

// Queues and the jobs in them (docs/planning/PHASE-98 - batch queues.md,
// Decision 8). A batch queue is an object of the security class QUEUE,
// with an owner, a protection code, and an ACL like a file's, checked by
// the same rule (Check's Figure 4-1). What differs is what the four bits
// of a category mean, and that a queue's protection also protects the
// jobs in it.
//
// The four kinds of access sit in the bits a file's read, write, execute,
// and delete do (the Guide to System Security, 4.8.7.2):
//
//	R  QueueRead    see the security elements of the queue, or of a job
//	S  QueueSubmit  put jobs in the queue
//	M  QueueManage  start, stop, change, or delete the queue, and do
//	                anything to any job in it
//	D  QueueDelete  delete a job, or change its elements
//
// Control (C) is a file's: the right to change the queue's owner,
// protection, and ACL. It is never in a protection code.
//
// Two rules are particular to queues (4.6.3 and 4.8.7.2):
//
//   - Manage implies read, submit, and delete. A category whose letters
//     are M alone (the system category's, by default) may do everything
//     but change the profile.
//   - Read and delete reach jobs in two ways. Given by the protection
//     code, they reach only the process's own jobs: the code is read with
//     the job's owner as the owner, so a code of (O:D) lets each user
//     change their own jobs and nobody else's. Given by an ACE, they reach
//     every job in the queue. Manage, however it is given, reaches every
//     job.
//
// The OPER privilege gives every access to every queue and job; that is
// the privilege gate's (corevms's ruleQueueOperator), not this package's,
// which knows only the privileges protection itself looks at. BYPASS,
// SYSPRV, GRPPRV, and READALL work here as they do for files: BYPASS
// everything, SYSPRV and GRPPRV through the system category (so the
// default (S:M) gives them manage), READALL a request for read alone.
//
// VMS 7.1 behaved this way in all 90 runs of the probe's protection
// matrix (testdata/bat98, A.4; corevms's TestBat98ProtectionMatrix).
// One point the matrix couldn't settle, since in every state where a user
// had a job of their own their categories gave them read or delete on it:
// whether a job's owner may always see it. Here they may (unconfirmed).

// The kinds of access a queue's protection gives, in a file's bits.
var (
	QueueRead   = Read
	QueueSubmit = Write
	QueueManage = Execute
	QueueDelete = Delete
)

// QueueAccessNames are the QUEUE class's names for the access bits, bit
// 0 first, as an ACE's ACCESS= and $GET_SECURITY's ACCESS_NAMES give
// them.
var QueueAccessNames = AccessNames{"READ", "SUBMIT", "MANAGE", "DELETE", "CONTROL"}

// queueImplied is the access Manage brings with it.
var queueImplied = QueueRead | QueueSubmit | QueueDelete

// implyQueue adds what Manage implies to a.
func implyQueue(a Access) Access {
	if a&QueueManage != 0 {
		a |= queueImplied
	}

	return a
}

// QueueLetters names a queue's access as a protection code's letters,
// with C for control: "RSMD", "MC", or "" for none.
func QueueLetters(a Access) string {
	var b strings.Builder

	for _, k := range []struct {
		bit    Access
		letter byte
	}{{QueueRead, 'R'}, {QueueSubmit, 'S'}, {QueueManage, 'M'}, {QueueDelete, 'D'}, {Control, 'C'}} {
		if a&k.bit != 0 {
			b.WriteByte(k.letter)
		}
	}

	return b.String()
}

// CheckQueue decides whether the process with credentials c may have
// every access in want to the queue with profile p: Evaluate's access,
// with what Manage implies, and READALL's read.
func CheckQueue(c Credentials, p Profile, want Access) Decision {
	d := Evaluate(c, p)
	d.Access = implyQueue(d.Access)
	d.Granted = d.Access&want == want

	if !d.Granted && c.Has(PrivREADALL) && want&^QueueRead == 0 {
		d.Granted = true
		d.Sources |= FromREADALL
	}

	if c.Trace != nil {
		c.Trace(queueTraceLine(c, p.Name, p, want, d))
	}

	return d
}

// CheckJob decides whether the process with credentials c may have every
// access in want to a job owned by jobOwner in the queue with profile
// queue. The process has, together:
//
//   - everything, if it has Manage on the queue (CheckQueue's rule);
//   - the access the queue's ACL gives it, for any job;
//   - for its own job, everything the protection code gives the job's
//     owner, read and delete among it (the code read with jobOwner as the
//     owner), and read in any case;
//   - BYPASS's everything, and READALL's read alone.
func CheckJob(c Credentials, queue Profile, jobOwner UIC, want Access) Decision {
	q := Evaluate(c, queue)
	d := Decision{ACE: q.ACE}

	add := func(a Access, from Source) {
		if a != 0 {
			d.Access |= a
			d.Sources |= from
		}
	}

	codeSources := FromSystem | FromOwner | FromGroup | FromWorld | FromUnowned

	switch {
	case implyQueue(q.Access)&QueueManage != 0:
		add(queueImplied|QueueManage, q.Sources&(FromACE|codeSources|FromBYPASS))
	default:
		if q.Sources&FromACE != 0 {
			add(implyQueue(queue.ACL[q.ACE].Access&codeAccess), FromACE)
		}

		if c.UIC == jobOwner {
			own := queue
			own.Owner = jobOwner

			j := Evaluate(c, own)
			add(implyQueue(j.Access&codeAccess), j.Sources&(FromACE|codeSources))
			add(QueueRead, FromOwner)
		}

		if c.Has(PrivBYPASS) {
			add(All&^d.Access, FromBYPASS)
		}
	}

	d.Granted = d.Access&want == want

	if !d.Granted && c.Has(PrivREADALL) && want&^QueueRead == 0 {
		d.Granted = true
		d.Sources |= FromREADALL
	}

	if c.Trace != nil {
		c.Trace(queueTraceLine(c, fmt.Sprintf("job of %v in %s", jobOwner, queue.Name), queue, want, d))
	}

	return d
}

// queueTraceLine is SET DEBUG PROTECTION's line for a queue's or job's
// check, in a queue's letters:
//
//	[200,10] wants M of SYS$BATCH owner [1,4] (S:M,O:D,G:R,W:S) ACL 0: denied, has S (world)
func queueTraceLine(c Credentials, name string, p Profile, want Access, d Decision) string {
	if name == "" {
		name = "queue"
	}

	outcome := "denied"
	if d.Granted {
		outcome = "granted"
	}

	letters := func(a Access) string {
		if s := QueueLetters(a); s != "" {
			return s
		}

		return "none"
	}

	return fmt.Sprintf("%v wants %s of %s owner %v %s ACL %d: %s, has %s (%s)",
		c.UIC, letters(want), name, p.Owner, QueueCodeText(p.Protection), len(p.ACL), outcome, letters(d.Access), sourceText(d))
}

// QueueCodeText is a queue's protection code as SHOW QUEUE/FULL writes
// it, "(S:M,O:D,G:R,W:S)", a category given nothing its letter alone.
func QueueCodeText(p Protection) string {
	parts := make([]string, len(categoryNames))

	for c, n := range categoryNames {
		parts[c] = n[:1]
		if letters := QueueLetters(p.Allows(Category(c))); letters != "" {
			parts[c] += ":" + letters
		}
	}

	return "(" + strings.Join(parts, ",") + ")"
}
