package authorize

import (
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/uaf"
)

// This file reads the account qualifiers ADD, COPY, MODIFY, and DEFAULT
// share into an edit, the changes they make to a record. Every value is
// read before any record is touched, so a bad one leaves the file as it
// was, as VMS 7.1's AUTHORIZE did (UICERR, BADVALUE, and VALTOOBIG added
// nothing, the probe's cases 15, 31, and 132).

// edit is what a command's qualifiers change.
type edit struct {
	// changes are applied to a record in turn.
	changes []func(r *uaf.Record)

	// uic is /UIC's, if given (the UIC identifier follows it).
	uic    uaf.UIC
	hasUIC bool

	// cli says /CLI was given, which CLIWARNMSG warns of.
	cli bool

	// account is /ACCOUNT's, if given and not blank (ADD makes a group
	// identifier of it).
	account string

	// password says /PASSWORD or /NOPASSWORD was given; noPassword that
	// the account is left with none (PWDLESSMIN).
	password   bool
	noPassword bool
}

// empty reports whether the edit changes nothing (MODIFY with no
// qualifiers says nothing, the probe's case 51).
func (e *edit) empty() bool { return len(e.changes) == 0 }

// apply makes the edit's changes to r.
func (e *edit) apply(r *uaf.Record) {
	for _, change := range e.changes {
		change(r)
	}
}

// quotaFields are the numeric qualifiers and the record's field each
// sets, in the grammar's order.
var quotaFields = []struct {
	name  string
	field func(r *uaf.Record) *uint32
}{
	{"ASTLM", func(r *uaf.Record) *uint32 { return &r.Quotas.ASTLM }},
	{"BIOLM", func(r *uaf.Record) *uint32 { return &r.Quotas.BIOLM }},
	{"BYTLM", func(r *uaf.Record) *uint32 { return &r.Quotas.BYTLM }},
	{"DIOLM", func(r *uaf.Record) *uint32 { return &r.Quotas.DIOLM }},
	{"ENQLM", func(r *uaf.Record) *uint32 { return &r.Quotas.ENQLM }},
	{"FILLM", func(r *uaf.Record) *uint32 { return &r.Quotas.FILLM }},
	{"JTQUOTA", func(r *uaf.Record) *uint32 { return &r.Quotas.JTQUOTA }},
	{"MAXACCTJOBS", func(r *uaf.Record) *uint32 { return &r.Quotas.MAXACCTJOBS }},
	{"MAXDETACH", func(r *uaf.Record) *uint32 { return &r.Quotas.MAXDETACH }},
	{"MAXJOBS", func(r *uaf.Record) *uint32 { return &r.Quotas.MAXJOBS }},
	{"PBYTLM", func(r *uaf.Record) *uint32 { return &r.Quotas.PBYTLM }},
	{"PGFLQUOTA", func(r *uaf.Record) *uint32 { return &r.Quotas.PGFLQUOTA }},
	{"PRCLM", func(r *uaf.Record) *uint32 { return &r.Quotas.PRCLM }},
	{"SHRFILLM", func(r *uaf.Record) *uint32 { return &r.Quotas.SHRFILLM }},
	{"TQELM", func(r *uaf.Record) *uint32 { return &r.Quotas.TQELM }},
	{"WSDEFAULT", func(r *uaf.Record) *uint32 { return &r.Quotas.WSDEFAULT }},
	{"WSEXTENT", func(r *uaf.Record) *uint32 { return &r.Quotas.WSEXTENT }},
	{"WSQUOTA", func(r *uaf.Record) *uint32 { return &r.Quotas.WSQUOTA }},
}

// textFields are the qualifiers whose values are text, the record's
// field each sets, and its longest value (AUTHORIZE's field sizes; a
// longer value is VALTOOBIG, unconfirmed: no probe case gave one).
var textFields = []struct {
	name    string
	longest int
	field   func(r *uaf.Record) *string
}{
	{"ACCOUNT", uaf.MaxAccount, func(r *uaf.Record) *string { return &r.Account }},
	{"CLI", 31, func(r *uaf.Record) *string { return &r.CLI }},
	{"CLITABLES", 31, func(r *uaf.Record) *string { return &r.CLITables }},
	{"DEVICE", 31, func(r *uaf.Record) *string { return &r.Device }},
	{"DIRECTORY", 63, func(r *uaf.Record) *string { return &r.Directory }},
	{"LGICMD", 63, func(r *uaf.Record) *string { return &r.LGICMD }},
	{"OWNER", uaf.MaxOwner, func(r *uaf.Record) *string { return &r.Owner }},
}

// readEdit reads r's account qualifiers. A bad value is reported, with
// the status the command leaves, and ok false. The flag and privilege
// names are checked first: on VMS the command line's parser refuses an
// unknown keyword before AUTHORIZE sees the command (CLI's IVKEYW, the
// probe's cases 29 and 30).
func (u *Utility) readEdit(r *dcl.Result) (e *edit, ok bool) {
	e = &edit{}

	if !u.readKeywords(r, e) {
		return nil, false
	}

	if r.Present("UIC") {
		text := r.String("UIC")

		uic, err := uaf.ParseUIC(text)
		if err != nil {
			u.message("UICERR", text)

			return nil, false
		}

		e.uic, e.hasUIC = uic, true
		e.changes = append(e.changes, func(rec *uaf.Record) { rec.UIC = uic })
	}

	for _, f := range textFields {
		if !r.Present(f.name) {
			continue
		}

		text := r.String(f.name)
		if len(text) > f.longest {
			u.message("VALTOOBIG", text)

			return nil, false
		}

		field := f.field

		e.changes = append(e.changes, func(rec *uaf.Record) { *field(rec) = text })

		switch f.name {
		case "CLI":
			e.cli = true
		case "ACCOUNT":
			e.account = strings.TrimSpace(text)
		}
	}

	for _, q := range []struct {
		name    string
		largest uint64
		field   func(r *uaf.Record) *uint32
	}{
		{"PRIORITY", 31, func(r *uaf.Record) *uint32 { return &r.Priority }},
		{"QUEPRIO", 255, func(r *uaf.Record) *uint32 { return &r.QueuePriority }},
	} {
		if !r.Present(q.name) {
			continue
		}

		n, ok := u.number(r.String(q.name), q.largest)
		if !ok {
			return nil, false
		}

		field := q.field

		e.changes = append(e.changes, func(rec *uaf.Record) { *field(rec) = n })
	}

	for _, q := range quotaFields {
		if !r.Present(q.name) {
			continue
		}

		n, ok := u.number(r.String(q.name), 1<<32-1)
		if !ok {
			return nil, false
		}

		field := q.field

		e.changes = append(e.changes, func(rec *uaf.Record) { *field(rec) = n })
	}

	if r.Present("CPUTIME") {
		text := r.String("CPUTIME")

		ticks, ok := cpuTime(text)
		if !ok {
			u.message("BADVALUE", text)

			return nil, false
		}

		e.changes = append(e.changes, func(rec *uaf.Record) { rec.Quotas.CPUTIME = ticks })
	}

	if !u.readPassword(r, e) {
		return nil, false
	}

	return e, true
}

// readKeywords reads /FLAGS, /PRIVILEGES, and /DEFPRIVILEGES: lists of
// names, each setting its bit or with NO in front clearing it, ALL and
// NOALL every privilege. An unknown name is CLI's IVKEYW.
func (u *Utility) readKeywords(r *dcl.Result, e *edit) bool {
	if r.Present("FLAGS") {
		set, isClear, ok := u.keywordBits(r.List("FLAGS"), func(name string) (uint64, bool) {
			bit, ok := uaf.FlagBit(name)
			if !ok {
				return 0, false
			}

			return 1 << bit, true
		})
		if !ok {
			return false
		}

		e.changes = append(e.changes, func(rec *uaf.Record) {
			rec.Flags = uaf.Flags((uint64(rec.Flags) | set) &^ isClear)
		})
	}

	for _, q := range []struct {
		name  string
		field func(r *uaf.Record) *uaf.Privileges
	}{
		{"PRIVILEGES", func(r *uaf.Record) *uaf.Privileges { return &r.Privileges }},
		{"DEFPRIVILEGES", func(r *uaf.Record) *uaf.Privileges { return &r.DefaultPrivileges }},
	} {
		if !r.Present(q.name) {
			continue
		}

		set, isClear, ok := u.keywordBits(r.List(q.name), privilegeMask)
		if !ok {
			return false
		}

		field := q.field

		e.changes = append(e.changes, func(rec *uaf.Record) {
			*field(rec) = uaf.Privileges((uint64(*field(rec)) | set) &^ isClear)
		})
	}

	return true
}

// privilegeMask is the mask of the privilege called name, or every bit
// for ALL.
func privilegeMask(name string) (uint64, bool) {
	if name == "ALL" {
		return uint64(uaf.AllPrivileges), true
	}

	bit, ok := uaf.PrivilegeBit(name)
	if !ok {
		return 0, false
	}

	return 1 << bit, true
}

// keywordBits reads names, each a name maskOf knows or one with NO in
// front, into the bits they set and clear, applied in order (a later
// name undoing an earlier one). A name maskOf knows as it is (the
// privilege NOACNT) is taken as it is.
func (u *Utility) keywordBits(names []string, maskOf func(string) (uint64, bool)) (set, isClear uint64, ok bool) {
	for _, name := range names {
		word := strings.ToUpper(strings.TrimSpace(name))

		if mask, ok := maskOf(word); ok {
			set, isClear = set|mask, isClear&^mask

			continue
		}

		if bare, negated := strings.CutPrefix(word, "NO"); negated {
			if mask, ok := maskOf(bare); ok {
				set, isClear = set&^mask, isClear|mask

				continue
			}
		}

		u.status = u.cliError("IVKEYW", word)

		return 0, 0, false
	}

	return set, isClear, true
}

// readPassword reads /PASSWORD and /NOPASSWORD. A password is kept as
// uaf's hash; none (/NOPASSWORD, or an empty one) is shorter than any
// minimum, which VMS 7.1 said with PWDLESSMIN (case 45), and leaves the
// account with no password.
func (u *Utility) readPassword(r *dcl.Result, e *edit) bool {
	if !r.Present("PASSWORD") {
		return true
	}

	e.password = true

	text := ""
	if !r.Negated("PASSWORD") {
		text = r.String("PASSWORD")
	}

	if strings.TrimSpace(text) == "" {
		e.noPassword = true
		e.changes = append(e.changes, func(rec *uaf.Record) {
			rec.Password, rec.PasswordChanged = "", time.Time{}
		})

		return true
	}

	hash, err := uaf.HashPassword(text)
	if err != nil {
		u.message("BADVALUE", "")

		return false
	}

	now := time.Now().UTC().Truncate(time.Second)
	
	e.changes = append(e.changes, func(rec *uaf.Record) { rec.Password, rec.PasswordChanged = hash, now })

	return true
}

// number reads a qualifier's number: decimal, or with DCL's radix
// prefixes %D, %O, and %X. Text that isn't one is BADVALUE, and a number
// past largest VALTOOBIG ("/PRIORITY=99", case 31), each with the text
// as typed.
func (u *Utility) number(text string, largest uint64) (uint32, bool) {
	t, base := strings.ToUpper(strings.TrimSpace(text)), 10

	switch {
	case strings.HasPrefix(t, "%D"):
		t = t[2:]
	case strings.HasPrefix(t, "%O"):
		t, base = t[2:], 8
	case strings.HasPrefix(t, "%X"):
		t, base = t[2:], 16
	}

	n, err := strconv.ParseUint(t, base, 64)
	if err != nil {
		u.message("BADVALUE", text)

		return 0, false
	}

	if n > largest {
		u.message("VALTOOBIG", text)

		return 0, false
	}

	return uint32(n), true
}

// cpuTime reads /CPUTIME's delta time, "[d-]hh[:mm[:ss[.cc]]]" ("0-00:10:00",
// case 25), or NONE, into 10-millisecond ticks (0, no limit).
func cpuTime(text string) (uint32, bool) {
	t := strings.ToUpper(strings.TrimSpace(text))
	if t == "NONE" {
		return 0, true
	}

	days := uint64(0)

	if d, rest, ok := strings.Cut(t, "-"); ok {
		n, err := strconv.ParseUint(d, 10, 32)
		if err != nil {
			return 0, false
		}

		days, t = n, rest
	}

	hundredths := uint64(0)

	if whole, frac, ok := strings.Cut(t, "."); ok {
		if len(frac) == 0 || len(frac) > 2 {
			return 0, false
		}

		n, err := strconv.ParseUint(frac, 10, 32)
		if err != nil {
			return 0, false
		}

		if len(frac) == 1 {
			n *= 10
		}

		hundredths, t = n, whole
	}

	parts := strings.Split(t, ":")
	if len(parts) > 3 {
		return 0, false
	}

	limits := []uint64{23, 59, 59}
	values := []uint64{0, 0, 0}

	for i, p := range parts {
		if p == "" && t == "" {
			break
		}

		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil || n > limits[i] {
			return 0, false
		}

		values[i] = n
	}

	total := (((days*24+values[0])*60+values[1])*60+values[2])*100 + hundredths
	if total > 1<<32-1 {
		return 0, false
	}

	return uint32(total), true
}
