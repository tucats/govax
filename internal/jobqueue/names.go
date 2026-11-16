package jobqueue

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/security"
)

// This file is the forms the JSON file gives values VMS keeps as numbers:
// a UIC, a queue's protection code, an ACE, a CPU time, a file ID, and
// the states. Each is a Go type with its own JSON form, so the database
// is plain Go to its users and readable text in the file.

// valueError is a value in the file that isn't in its type's form; the
// text lets Load find the line it is on.
type valueError struct {
	text string
	err  error
}

func (e *valueError) Error() string { return e.err.Error() }

func (e *valueError) Unwrap() error { return e.err }

// unmarshalText reads a JSON string and hands it to parse, wrapping
// parse's error so that Load can find its line.
func unmarshalText(data []byte, parse func(string) error) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	if err := parse(s); err != nil {
		return &valueError{text: s, err: err}
	}

	return nil
}

// UIC is a user identification code, the group in the high word and the
// member in the low. In the file it is "[group,member]", in octal.
type UIC security.UIC

// SystemUIC is [1,4], SYSTEM's UIC, a new queue's owner by default.
const SystemUIC = UIC(1<<16 | 4)

// String is the UIC as the file writes it, "[1,4]".
func (u UIC) String() string { return security.UIC(u).String() }

// ParseUIC reads "[group,member]", both numbers octal, as the file has
// it.
func ParseUIC(text string) (UIC, error) {
	bad := fmt.Errorf("%q is not a UIC of the form [group,member], in octal", text)

	inner, ok := strings.CutPrefix(strings.TrimSpace(text), "[")
	if !ok {
		return 0, bad
	}

	inner, ok = strings.CutSuffix(inner, "]")
	if !ok {
		return 0, bad
	}

	g, m, ok := strings.Cut(inner, ",")
	if !ok {
		return 0, bad
	}

	group, err1 := strconv.ParseUint(strings.TrimSpace(g), 8, 16)
	member, err2 := strconv.ParseUint(strings.TrimSpace(m), 8, 16)

	if err1 != nil || err2 != nil {
		return 0, bad
	}

	return UIC(security.NewUIC(uint32(group), uint32(member))), nil
}

// MarshalJSON writes the UIC as its text.
func (u UIC) MarshalJSON() ([]byte, error) { return json.Marshal(u.String()) }

// UnmarshalJSON reads a UIC's text.
func (u *UIC) UnmarshalJSON(data []byte) error {
	return unmarshalText(data, func(s string) error {
		v, err := ParseUIC(s)
		*u = v

		return err
	})
}

// Protection is a queue's protection code: four 4-bit fields, SYSTEM's in
// bits 0-3, then OWNER, GROUP, and WORLD, a set bit denying that access,
// as a file's is (internal/security). A queue's four kinds of access are
// in the bits a file's read, write, execute, and delete are in: Read (see
// the queue's jobs), Submit, Manage, and Delete (a job's owner's, to
// change or delete one's own jobs). In the file and on the screen it is
// "(S:M,O:D,G:R,W:S)", the letters each category is given; a category
// with no letters is given nothing ("W").
//
// Which access a letter implies (whether M also gives R, S, and D) is the
// access check's, not this type's: the code keeps exactly the letters it
// was given.
type Protection uint16

// The kinds of access a queue's protection gives, the security package's
// bits under a queue's names.
var (
	AccessRead   = security.Read
	AccessSubmit = security.Write
	AccessManage = security.Execute
	AccessDelete = security.Delete
)

// DefaultProtection is a new queue's protection, (S:M,O:D,G:R,W:S), as
// VMS 7.1 showed BAT98_B's (the phase doc's A.1 and A.4).
var DefaultProtection = func() Protection {
	p, err := ParseProtection("(S:M,O:D,G:R,W:S)", Protection(security.NoAccess))
	if err != nil {
		panic(err)
	}

	return p
}()

// queueLetters are the letters of a queue's kinds of access, in the
// order the code writes them.
var queueLetters = []struct {
	access security.Access
	letter byte
}{
	{AccessRead, 'R'}, {AccessSubmit, 'S'}, {AccessManage, 'M'}, {AccessDelete, 'D'},
}

// categoryLetters are the categories' letters, in mask order.
var categoryLetters = [...]string{"SYSTEM", "OWNER", "GROUP", "WORLD"}

// Allows returns the access category c's field gives.
func (p Protection) Allows(c security.Category) security.Access {
	return security.Protection(p).Allows(c)
}

// String is the code as SHOW QUEUE/FULL writes it: "(S:M,O:D,G:R,W:S)",
// a category with no access its letter alone.
func (p Protection) String() string {
	parts := make([]string, len(categoryLetters))

	for c, name := range categoryLetters {
		var b strings.Builder

		b.WriteByte(name[0])

		access := p.Allows(security.Category(c))
		if access != 0 {
			b.WriteByte(':')
		}

		for _, k := range queueLetters {
			if access&k.access != 0 {
				b.WriteByte(k.letter)
			}
		}

		parts[c] = b.String()
	}

	return "(" + strings.Join(parts, ",") + ")"
}

// ParseProtection reads a queue's protection code, with or without its
// parentheses, and returns base with each category the text names set to
// the letters given it. A category's name may be shortened to any
// leading part, the separator may be ":" or "=", and case doesn't matter.
func ParseProtection(text string, base Protection) (Protection, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")

	if s == "" {
		return 0, errors.New("no categories given")
	}

	result := security.Protection(base)

	var seen [len(categoryLetters)]bool

	for _, item := range strings.Split(s, ",") {
		name, letters, _ := strings.Cut(strings.Replace(item, "=", ":", 1), ":")

		c := -1

		for i, full := range categoryLetters {
			if name != "" && strings.HasPrefix(full, name) {
				c = i
			}
		}

		if c < 0 {
			return 0, fmt.Errorf("%q is not SYSTEM, OWNER, GROUP, or WORLD", name)
		}

		if seen[c] {
			return 0, fmt.Errorf("category %s given twice", categoryLetters[c])
		}

		seen[c] = true

		var access security.Access

		for i := range len(letters) {
			found := false

			for _, k := range queueLetters {
				if letters[i] == k.letter {
					access |= k.access
					found = true
				}
			}

			if !found {
				return 0, fmt.Errorf("%q has a letter other than R, S, M, or D", item)
			}
		}

		result = result.With(security.Category(c), access)
	}

	return Protection(result), nil
}

// MarshalJSON writes the code as its text.
func (p Protection) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }

// UnmarshalJSON reads a code's text. Every category is given: one the
// text leaves out gives no access.
func (p *Protection) UnmarshalJSON(data []byte) error {
	return unmarshalText(data, func(s string) error {
		v, err := ParseProtection(s, Protection(security.NoAccess))
		*p = v

		return err
	})
}

// ACE is one access control entry, the bytes VMS keeps (its first byte
// its length), as internal/security reads and writes them. In the file
// it is those bytes in hexadecimal, so that the file needs no table of
// identifier names to be read.
type ACE []byte

// checkACE reports whether a is an ACE's bytes: at least the length and
// type bytes, and as long as its first byte says.
func checkACE(a []byte) error {
	if len(a) < 2 || int(a[0]) != len(a) {
		return fmt.Errorf("an ACE's first byte must be its length (%d bytes)", len(a))
	}

	return nil
}

// MarshalJSON writes the ACE's bytes in hexadecimal.
func (a ACE) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToUpper(hex.EncodeToString(a)))
}

// UnmarshalJSON reads an ACE's hexadecimal bytes.
func (a *ACE) UnmarshalJSON(data []byte) error {
	return unmarshalText(data, func(s string) error {
		b, err := hex.DecodeString(s)
		if err != nil {
			return fmt.Errorf("%q is not an ACE's bytes in hexadecimal", s)
		}

		if err := checkACE(b); err != nil {
			return err
		}

		*a = b

		return nil
	})
}

// CPUTime is a CPU time limit in hundredths of a second, as a queue's
// /CPUDEFAULT and /CPUMAXIMUM and a job's /CPUTIME give it. 0 is no
// limit given (the queue's or the account's applies); CPUInfinite is
// INFINITE. In the file it is "hh:mm:ss" (with ".cc" when the hundredths
// aren't 0) or "INFINITE".
type CPUTime int64

// CPUInfinite is a limit of INFINITE: no limit at all.
const CPUInfinite CPUTime = -1

// String is the limit as SHOW QUEUE/FULL writes it, "00:05:00".
func (t CPUTime) String() string {
	if t == CPUInfinite {
		return "INFINITE"
	}

	cs := int64(t)
	h, m, s, c := cs/360000, cs/6000%60, cs/100%60, cs%100

	if c != 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%02d", h, m, s, c)
	}

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// ParseCPUTime reads a limit as String writes it.
func ParseCPUTime(text string) (CPUTime, error) {
	text = strings.ToUpper(strings.TrimSpace(text))
	if text == "INFINITE" {
		return CPUInfinite, nil
	}

	bad := fmt.Errorf("%q is not a CPU time of the form hh:mm:ss[.cc] or INFINITE", text)

	whole, frac, hasFrac := strings.Cut(text, ".")

	parts := strings.Split(whole, ":")
	if len(parts) != 3 {
		return 0, bad
	}

	var total int64

	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil || i > 0 && n > 59 {
			return 0, bad
		}

		total = total*60 + int64(n)
	}

	total *= 100

	if hasFrac {
		n, err := strconv.ParseUint(frac, 10, 8)
		if err != nil || len(frac) != 2 {
			return 0, bad
		}

		total += int64(n)
	}

	return CPUTime(total), nil
}

// MarshalJSON writes the limit as its text.
func (t CPUTime) MarshalJSON() ([]byte, error) { return json.Marshal(t.String()) }

// UnmarshalJSON reads a limit's text.
func (t *CPUTime) UnmarshalJSON(data []byte) error {
	return unmarshalText(data, func(s string) error {
		v, err := ParseCPUTime(s)
		*t = v

		return err
	})
}

// FileID is an ODS-2 file identifier: the file's number, its sequence
// number, and the relative volume number, as a job holds each of its
// files (so a later version doesn't replace it). In the file it is
// "(num,seq,rvn)", as DIRECTORY/FULL shows it.
type FileID struct {
	Number   uint32
	Sequence uint16
	RVN      uint16
}

// String is the ID as the file writes it, "(23,1,0)".
func (f FileID) String() string { return fmt.Sprintf("(%d,%d,%d)", f.Number, f.Sequence, f.RVN) }

// MarshalJSON writes the ID as its text.
func (f FileID) MarshalJSON() ([]byte, error) { return json.Marshal(f.String()) }

// UnmarshalJSON reads an ID's text.
func (f *FileID) UnmarshalJSON(data []byte) error {
	return unmarshalText(data, func(s string) error {
		var v FileID

		inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "("), ")")

		n, err := fmt.Sscanf(inner, "%d,%d,%d", &v.Number, &v.Sequence, &v.RVN)
		if err != nil || n != 3 {
			return fmt.Errorf("%q is not a file ID of the form (num,seq,rvn)", s)
		}

		*f = v

		return nil
	})
}

// enumText writes and reads an enumeration as names: each type below is
// a small integer in Go and its name in the file.
func enumText[T ~int](v T, names []string) ([]byte, error) {
	if int(v) < 0 || int(v) >= len(names) {
		return nil, fmt.Errorf("value %d out of range", v)
	}

	return json.Marshal(names[v])
}

func enumParse[T ~int](data []byte, names []string, v *T, what string) error {
	return unmarshalText(data, func(s string) error {
		for i, n := range names {
			if strings.EqualFold(n, s) {
				*v = T(i)

				return nil
			}
		}

		return fmt.Errorf("%q is not a %s (%s)", s, what, strings.Join(names, ", "))
	})
}

// QueueState is a queue's state, as the database keeps it. SHOW QUEUE's
// words (idle, available, busy) come from a started queue's executing
// jobs (Database.StateText).
type QueueState int

// The queue states.
const (
	// QueueStopped is a stopped queue: it starts no jobs and runs none.
	QueueStopped QueueState = iota
	// QueueStarted is a started queue, running jobs while it has room.
	QueueStarted
	// QueuePaused is a queue STOP/QUEUE paused: its executing jobs are
	// suspended, and it starts no more.
	QueuePaused
	// QueueStopping is a queue STOP/QUEUE/NEXT left running the jobs it
	// had: it starts no more, and stops when the last ends.
	QueueStopping
)

var queueStateNames = []string{"STOPPED", "STARTED", "PAUSED", "STOPPING"}

// String is the state's name in the file.
func (s QueueState) String() string {
	if int(s) < len(queueStateNames) && s >= 0 {
		return queueStateNames[s]
	}

	return fmt.Sprintf("QueueState(%d)", int(s))
}

// MarshalJSON writes the state's name.
func (s QueueState) MarshalJSON() ([]byte, error) { return enumText(s, queueStateNames) }

// UnmarshalJSON reads a state's name.
func (s *QueueState) UnmarshalJSON(data []byte) error {
	return enumParse(data, queueStateNames, s, "queue state")
}

// JobStatus is a job's state.
type JobStatus int

// The job states.
const (
	// JobPending is a job waiting for its queue to have room.
	JobPending JobStatus = iota
	// JobHolding is a job held (/HOLD) until released.
	JobHolding
	// JobTimed is a job held until its /AFTER time, "Holding until".
	JobTimed
	// JobExecuting is a job whose batch process is running.
	JobExecuting
	// JobRetained is a job that ended and was kept by /RETAIN.
	JobRetained
)

var jobStatusNames = []string{"PENDING", "HOLDING", "TIMED", "EXECUTING", "RETAINED"}

// String is the status's name in the file.
func (s JobStatus) String() string {
	if int(s) < len(jobStatusNames) && s >= 0 {
		return jobStatusNames[s]
	}

	return fmt.Sprintf("JobStatus(%d)", int(s))
}

// MarshalJSON writes the status's name.
func (s JobStatus) MarshalJSON() ([]byte, error) { return enumText(s, jobStatusNames) }

// UnmarshalJSON reads a status's name.
func (s *JobStatus) UnmarshalJSON(data []byte) error {
	return enumParse(data, jobStatusNames, s, "job status")
}

// Retain is which ended jobs a queue or a job's /RETAIN keeps.
type Retain int

// The /RETAIN choices.
const (
	// RetainDefault is no /RETAIN given: for a job, its queue's choice;
	// for a queue, keep nothing.
	RetainDefault Retain = iota
	// RetainNever is /NORETAIN on a job: keep it whatever its queue says.
	RetainNever
	// RetainAlways is /RETAIN=ALWAYS: keep every job that ends.
	RetainAlways
	// RetainError is /RETAIN=ERROR: keep a job that didn't succeed.
	RetainError
)

var retainNames = []string{"DEFAULT", "NEVER", "ALWAYS", "ERROR"}

// String is the choice as /RETAIN writes it.
func (r Retain) String() string {
	if int(r) < len(retainNames) && r >= 0 {
		return retainNames[r]
	}

	return fmt.Sprintf("Retain(%d)", int(r))
}

// IsZero lets omitzero leave RetainDefault out of the file.
func (r Retain) IsZero() bool { return r == RetainDefault }

// MarshalJSON writes the choice's name.
func (r Retain) MarshalJSON() ([]byte, error) { return enumText(r, retainNames) }

// UnmarshalJSON reads a choice's name.
func (r *Retain) UnmarshalJSON(data []byte) error {
	return enumParse(data, retainNames, r, "retain choice")
}

// ManagerState is the queue manager's state: whether it was last started
// or stopped, which decides whether a new session starts it (Decision 11).
type ManagerState int

// The manager states.
const (
	// ManagerStopped is a manager never started, or stopped by
	// STOP/QUEUE/MANAGER.
	ManagerStopped ManagerState = iota
	// ManagerStarted is a manager started by START/QUEUE/MANAGER and not
	// stopped since: a session starting runs it.
	ManagerStarted
)

var managerStateNames = []string{"STOPPED", "STARTED"}

// String is the state's name in the file.
func (s ManagerState) String() string {
	if int(s) < len(managerStateNames) && s >= 0 {
		return managerStateNames[s]
	}

	return fmt.Sprintf("ManagerState(%d)", int(s))
}

// MarshalJSON writes the state's name.
func (s ManagerState) MarshalJSON() ([]byte, error) { return enumText(s, managerStateNames) }

// UnmarshalJSON reads a state's name.
func (s *ManagerState) UnmarshalJSON(data []byte) error {
	return enumParse(data, managerStateNames, s, "manager state")
}
