package corevms

import (
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// Logging in (docs/planning/PHASE-95 - authentication.md, Decision 7).
//
// On VMS, LOGINOUT turns a new process into a user's: it reads the
// user's SYSUAF.DAT record, refuses a disabled account, and gives the
// process the record's username, account, UIC, privileges, priority, and
// quotas, and the identifiers the user holds with the environmental ones
// (INTERACTIVE and LOCAL at a terminal). It notes the login's time in the
// record, and tells the user when the last one was.
//
// govax does the same in two steps. BeginLogin is LOGINOUT's look at the
// authorization file: it checks the account, notes the time, and returns
// a Login holding what the process is to get. LogIn gives a process a
// Login. They're apart because process 1 is made again at each VMINIT
// (a new System), and must be the same user again without a second login
// being recorded: the machine keeps the Login and gives it to each new
// process 1. DCL's side (SYS$LOGIN and its kin, the default directory)
// is dclshell's.

// LoginError is a login LOGINOUT refuses: its LGI$_ condition and the
// message's identifier and text ("%LOGIN-F-DISUSER, account is
// disabled", VMS 7.1's, the probe's round 2).
type LoginError struct {
	Status      uint32
	Ident, Text string
}

func (e *LoginError) Error() string { return "LOGIN-F-" + e.Ident + ", " + e.Text }

// The logins LOGINOUT refuses. DISUSER's text is VMS 7.1's; NOSUCHUSER's
// and CLIFAIL's are unconfirmed (a terminal login with a bad username
// says only "User authorization failure", and the probe tried no other
// CLI).
func loginError(ident, text string) *LoginError {
	return &LoginError{Status: vmsdef.LibrarySymbols["LGI$_"+ident], Ident: ident, Text: text}
}

// LoginRecord returns the account username may log in as from f: its
// record, or a LoginError for an account f doesn't have, one with
// DISUSER, or one whose CLI isn't DCL, the only one govax has.
func LoginRecord(f *uaf.File, username string) (uaf.Record, error) {
	r, ok := f.Find(username)

	switch {
	case !ok:
		return uaf.Record{}, loginError("NOSUCHUSER", "no such user")
	case r.Flags&uaf.FlagDISUSER != 0:
		return uaf.Record{}, loginError("DISUSER", "account is disabled")
	case r.CLI != "" && !strings.EqualFold(r.CLI, "DCL"):
		return uaf.Record{}, loginError("CLIFAIL", "error activating command interpreter "+r.CLI)
	}

	return *r, nil
}

// Login is what a process logging in is to get: the account's record as
// it was before this login, whether the login is interactive, its time,
// and the identifiers the account holds.
type Login struct {
	// Record is the account's record, its last-login times those of the
	// login before this one ($GETJPI's LAST_LOGIN_I and LAST_LOGIN_N).
	Record uaf.Record

	// Interactive is whether this is an interactive login (at a
	// terminal) rather than a batch or other non-interactive one.
	Interactive bool

	// Time is the login's time, in VMS time.
	Time uint64

	// Rights are the identifiers the account holds, by value; an
	// identifier the file can't name is left out.
	Rights []security.Identifier
}

// BeginLogin is LOGINOUT's look at the authorization file for a login by
// username: the record (LoginRecord's checks), the identifiers it holds,
// and the login's time, which it notes in the record as the last
// interactive or non-interactive login.
func (s *System) BeginLogin(username string, interactive bool) (*Login, error) {
	f, err := s.Authorization()
	if err != nil {
		return nil, err
	}

	r, err := LoginRecord(f, username)
	if err != nil {
		return nil, err
	}

	l := &Login{Record: r, Interactive: interactive, Time: s.now()}

	l.Rights = heldRights(f, r.UIC)

	when := goTime(l.Time)

	err = s.UpdateAuthorization(func(f *uaf.File) error {
		rec, ok := f.Find(r.Username)
		if !ok {
			return nil // removed meanwhile: nothing to note
		}

		if interactive {
			rec.LastInteractive = when
		} else {
			rec.LastNonInteractive = when
		}

		return nil
	})

	return l, err
}

// now is the System's time, or the host's when it has no clock.
func (s *System) now() uint64 {
	if s.Clock != nil {
		return s.Clock()
	}

	return vmsdef.Time(time.Now())
}

// goTime is the VMS time v as a Go time: VMS keeps local time, and the
// file keeps a moment, so v's wall-clock reading is taken in the host's
// zone. The zero time is VMS's 0.
func goTime(v uint64) time.Time {
	if v == 0 {
		return time.Time{}
	}

	w := vmsdef.GoTime(v)

	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), w.Nanosecond(), time.Local)
}

// vmsTime is goTime's inverse: 0 for the zero time.
func vmsTime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}

	return vmsdef.Time(t.In(time.Local))
}

// LogIn makes the process the user l describes, as LOGINOUT does: its
// username, process name, account, and UIC (and its logical name
// tables'); its authorized privileges, and its process and current
// privileges the account's defaults; its priority; its quotas
// (Sysgen.LoginQuotas, which lays the configured PQL parameters over
// them); its rights, the environmental ones for the kind of login and
// the identifiers the account holds; its login time and the facts
// $GETJPI reports of the account (LastLogin).
func (env *Environment) LogIn(l *Login) {
	p, r := env.Process, l.Record

	p.Username, p.Name, p.Account = r.Username, r.Username, r.Account
	p.UIC = uint32(r.UIC)

	if env.Logicals != nil {
		env.Logicals.SetUIC(p.UIC)
	}

	p.AuthorizedPrivileges = uint64(r.Privileges)
	p.ProcessPrivileges = uint64(r.DefaultPrivileges)
	p.CurrentPrivileges = uint64(r.DefaultPrivileges)
	p.Priority, p.BasePriority, p.AuthorizedPriority = r.Priority, r.Priority, r.Priority

	env.System.Sysgen.LoginQuotas(p, r.Quotas)

	rights := interactiveRights
	if !l.Interactive {
		rights = batchRights
	}

	p.Rights = slices.Clone(rights)

	for _, id := range l.Rights {
		if !slices.ContainsFunc(p.Rights, func(have security.Identifier) bool { return have.Value == id.Value }) {
			p.Rights = append(p.Rights, id)
		}
	}

	p.LoginTime = l.Time
	p.LastLogin = l.Previous()
}

// Previous is what $GETJPI reports of the account: the logins before
// this one, the record's flags, and its limits.
func (l *Login) Previous() LastLogin {
	r := l.Record

	return LastLogin{
		Interactive:    vmsTime(r.LastInteractive),
		NonInteractive: vmsTime(r.LastNonInteractive),
		UAFFlags:       uint32(r.Flags),
		MaxJobs:        r.Quotas.MAXJOBS,
		MaxAcctJobs:    r.Quotas.MAXACCTJOBS,
		MaxDetach:      r.Quotas.MAXDETACH,
		SharedFiles:    r.Quotas.SHRFILLM,
	}
}

// LastLogin is what $GETJPI reports of the account a process logged in
// as, beyond what the process itself has: the times of the last
// interactive and non-interactive logins before this one (0 for none,
// which F$GETJPI writes as 17-NOV-1858, the probe's round 2), the
// record's flags, and the limits govax keeps but doesn't enforce. Login
// failures and the login flags (JPI$_LOGIN_FAILURES, JPI$_LOGIN_FLAGS)
// are always 0: govax asks for no password.
type LastLogin struct {
	Interactive, NonInteractive uint64

	UAFFlags uint32

	MaxJobs, MaxAcctJobs, MaxDetach, SharedFiles uint32
}

// LastLoginText is the login banner's lines about the last logins, as
// VMS 7.1 writes them ("    Last interactive login on Sunday,
// 15-NOV-2026 17:19", the probe's round 2): one for each kind that has
// happened, none for a first login.
func (l LastLogin) LastLoginText() string {
	var b strings.Builder

	line := func(kind string, v uint64) {
		if v != 0 {
			t := vmsdef.GoTime(v)
			b.WriteString("    Last " + kind + " login on " + t.Format("Monday, ") +
				strings.ToUpper(t.Format("02-Jan-2006 15:04")) + "\n")
		}
	}

	line("interactive", l.Interactive)
	line("non-interactive", l.NonInteractive)

	return b.String()
}
