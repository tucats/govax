package uaf

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// Version is the file format's version, the "version" key every file
// has, so that a later format can tell an old file.
const Version = 1

// File is the whole authorization file: the accounts and the rights
// identifiers.
type File struct {
	// Version is the format's version, Version.
	Version int `json:"version"`

	// Accounts are the accounts, one Record each, in the order the file
	// has them (AUTHORIZE adds a new one at the end).
	Accounts []Record `json:"accounts"`

	// Identifiers are the rights database's identifiers, general and
	// UIC both, as RIGHTSLIST.DAT has them, each with its holders. The
	// environmental ones every system has (INTERACTIVE, BATCH, ...) aren't
	// here (Environmental).
	Identifiers []Identifier `json:"identifiers"`
}

// Record is one account, SYSUAF.DAT's record for a username.
type Record struct {
	// Username is the account's name, 1 to 12 letters, digits, "$",
	// and "_", in capitals.
	Username string `json:"username"`

	// Owner is the owner's name (AUTHORIZE's /OWNER, up to 31
	// characters), and Account the account name (/ACCOUNT, up to 8).
	Owner   string `json:"owner"`
	Account string `json:"account"`

	// UIC is the account's UIC.
	UIC UIC `json:"uic"`

	// CLI is the command language interpreter (/CLI), DCL; CLITables its
	// tables (/CLITABLES), DCLTABLES. govax has only DCL.
	CLI       string `json:"cli"`
	CLITables string `json:"clitables"`

	// LGICMD is the login command procedure (/LGICMD); "" means
	// SYS$LOGIN:LOGIN.COM.
	LGICMD string `json:"lgicmd"`

	// Device and Directory are the home device and directory (/DEVICE,
	// /DIRECTORY), SYS$LOGIN at login. They may be logical names
	// (SYSTEM's "SYS$SYSROOT:"), translated at each login.
	Device    string `json:"device"`
	Directory string `json:"directory"`

	// Flags are the login flags (/FLAGS).
	Flags Flags `json:"flags"`

	// Priority is the base priority (/PRIORITY), and QueuePriority the
	// queue priority (/QUEPRIO), kept and shown but used by nothing yet.
	Priority      uint32 `json:"priority"`
	QueuePriority uint32 `json:"queue_priority"`

	// Privileges are the authorized privileges (/PRIVILEGES), and
	// DefaultPrivileges those a process starts with (/DEFPRIVILEGES).
	Privileges        Privileges `json:"privileges"`
	DefaultPrivileges Privileges `json:"default_privileges"`

	// Quotas are the account's limits.
	Quotas Quotas `json:"quotas"`

	// Password is the password's hash (HashPassword), "" for none.
	Password string `json:"password"`

	// Legacy is what files written before holders moved to the
	// identifiers (2026-11-15) kept here: the identifiers the account
	// held. Parse moves them to the identifiers' Holders, by the account's
	// UIC, and the field is never written again.
	Legacy []Held `json:"identifiers,omitempty"`

	// LastInteractive and LastNonInteractive are the times of the last
	// interactive and non-interactive logins, and PasswordChanged of the
	// last password change; the zero time for never.
	LastInteractive    time.Time `json:"last_interactive_login,omitzero"`
	LastNonInteractive time.Time `json:"last_noninteractive_login,omitzero"`
	PasswordChanged    time.Time `json:"password_changed,omitzero"`
}

// Quotas are an account's limits, each AUTHORIZE's qualifier of the same
// name. govax's processes have ASTLM, BIOLM, BYTLM, DIOLM, ENQLM, FILLM,
// JTQUOTA, PGFLQUOTA, PRCLM, TQELM, WSDEFAULT, WSQUOTA, and WSEXTENT, and
// are given them at login; the rest are kept and shown.
type Quotas struct {
	ASTLM       uint32 `json:"astlm"`
	BIOLM       uint32 `json:"biolm"`
	BYTLM       uint32 `json:"bytlm"`
	PBYTLM      uint32 `json:"pbytlm"`
	DIOLM       uint32 `json:"diolm"`
	ENQLM       uint32 `json:"enqlm"`
	FILLM       uint32 `json:"fillm"`
	SHRFILLM    uint32 `json:"shrfillm"`
	JTQUOTA     uint32 `json:"jtquota"`
	PGFLQUOTA   uint32 `json:"pgflquota"`
	PRCLM       uint32 `json:"prclm"`
	TQELM       uint32 `json:"tqelm"`
	WSDEFAULT   uint32 `json:"wsdefault"`
	WSQUOTA     uint32 `json:"wsquota"`
	WSEXTENT    uint32 `json:"wsextent"`
	CPUTIME     uint32 `json:"cputime"` // in 10-millisecond ticks; 0 is no limit
	MAXJOBS     uint32 `json:"maxjobs"`
	MAXACCTJOBS uint32 `json:"maxacctjobs"`
	MAXDETACH   uint32 `json:"maxdetach"`
}

// Held is how files before 2026-11-15 said an account held an
// identifier: its name (or its value, "%X80010001") and the attributes it
// was held with. Parse turns each into a Holder.
type Held struct {
	Name       string     `json:"name"`
	Attributes Attributes `json:"attributes,omitzero"`
}

// Identifier is a rights identifier: its name, value, and attributes, and
// who holds it.
type Identifier struct {
	Name       string          `json:"name"`
	Value      IdentifierValue `json:"value"`
	Attributes Attributes      `json:"attributes,omitzero"`

	// Holders hold the identifier, in the order it was granted to them
	// (AUTHORIZE's SHOW/IDENTIFIER/FULL lists them so).
	Holders []Holder `json:"holders,omitempty"`
}

// Holder is a holder record: a UIC that holds an identifier, and the
// attributes it holds it with. On VMS a holder is a UIC, not an account:
// every account with the UIC holds what it holds, and GRANT/IDENTIFIER
// names the holder by its UIC identifier (VMS 7.1's GRANTUSR, "user
// identifier NOSUCH does not exist", the phase doc's A.5). When a UIC
// identifier's value changes, its holder records change with it
// ($MOD_IDENT), and when it is removed they go ($REM_IDENT).
type Holder struct {
	UIC        UIC        `json:"uic"`
	Attributes Attributes `json:"attributes,omitzero"`
}

// HeldIdentifier is an identifier a UIC holds, and the attributes it
// holds it with: the identifier's own, and the holder record's.
type HeldIdentifier struct {
	Identifier Identifier
	Attributes Attributes
}

// The SYSTEM account's values: what process 1 has always had (corevms's
// NewProcess and its job's nominal quotas), so that logging in as SYSTEM
// from the file changes nothing (the phase doc's Decision 3). They are in
// the range VMS's SYSTEM account has, not 7.1's own (the probe's case 2
// showed WSEXTENT 2048 and ENQLM 300).
func systemQuotas() Quotas {
	return Quotas{
		ASTLM: 50, BIOLM: 40, DIOLM: 40,
		BYTLM: 27392, FILLM: 300, PGFLQUOTA: 40960, TQELM: 30, ENQLM: 200, JTQUOTA: 4096,
		PRCLM:     10,
		WSDEFAULT: 512, WSQUOTA: 1024, WSEXTENT: 16400,
	}
}

// SystemRecord is the SYSTEM account a new file has: owner SYSTEM
// MANAGER, account SYSTEM, UIC [1,4], every privilege, priority 4, home
// SYS$SYSROOT:[SYSMGR] (SYS$MANAGER, as on VMS), DCL, no password.
func SystemRecord() Record {
	return Record{
		Username: "SYSTEM", Owner: "SYSTEM MANAGER", Account: "SYSTEM",
		UIC: NewUIC(1, 4), CLI: "DCL", CLITables: "DCLTABLES",
		Device: "SYS$SYSROOT:", Directory: "[SYSMGR]",
		Priority: 4, Privileges: AllPrivileges, DefaultPrivileges: AllPrivileges,
		Quotas: systemQuotas(),
	}
}

// DefaultRecord is the DEFAULT account a new file has, the record ADD
// copies: VMS 7.1's new file's (the probe's case 1), but for its password
// lifetime and minimum, which govax doesn't keep: UIC [200,200], DISUSER,
// directory [USER], TMPMBX and NETMBX, priority and queue priority 4.
func DefaultRecord() Record {
	tmpmbx, _ := PrivilegeBit("TMPMBX")
	netmbx, _ := PrivilegeBit("NETMBX")
	privs := Privileges(1<<tmpmbx | 1<<netmbx)

	return Record{
		Username: "DEFAULT", UIC: NewUIC(0o200, 0o200), CLI: "DCL", CLITables: "DCLTABLES",
		Directory: "[USER]", Flags: FlagDISUSER,
		Priority: 4, QueuePriority: 4, Privileges: privs, DefaultPrivileges: privs,
		Quotas: Quotas{
			ASTLM: 40, BIOLM: 40, DIOLM: 40, BYTLM: 32768, FILLM: 300, JTQUOTA: 4096,
			PGFLQUOTA: 32768, PRCLM: 2, TQELM: 40, ENQLM: 200,
			WSDEFAULT: 256, WSQUOTA: 512, WSEXTENT: 1024,
		},
	}
}

// Default returns a new file: SYSTEM and DEFAULT, as AUTHORIZE makes a
// new SYSUAF.DAT, and SYSTEM's UIC identifier.
func Default() *File {
	return &File{
		Version:     Version,
		Accounts:    []Record{SystemRecord(), DefaultRecord()},
		Identifiers: []Identifier{{Name: "SYSTEM", Value: IdentifierValue(NewUIC(1, 4))}},
	}
}

// Find returns the account called username (any case, trailing blanks
// ignored), and whether there is one.
func (f *File) Find(username string) (*Record, bool) {
	username = strings.ToUpper(strings.TrimRight(username, " "))

	for i := range f.Accounts {
		if f.Accounts[i].Username == username {
			return &f.Accounts[i], true
		}
	}

	return nil, false
}

// Put puts r in the file, replacing the account of the same name or
// adding it at the end.
func (f *File) Put(r Record) {
	if have, ok := f.Find(r.Username); ok {
		*have = r

		return
	}

	f.Accounts = append(f.Accounts, r)
}

// Remove takes the account called username out, and reports whether
// there was one.
func (f *File) Remove(username string) bool {
	have, ok := f.Find(username)
	if !ok {
		return false
	}

	name := have.Username
	f.Accounts = slices.DeleteFunc(f.Accounts, func(r Record) bool { return r.Username == name })

	return true
}

// IdentifierNamed returns the identifier called name (any case).
func (f *File) IdentifierNamed(name string) (*Identifier, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))

	for i := range f.Identifiers {
		if f.Identifiers[i].Name == name {
			return &f.Identifiers[i], true
		}
	}

	return nil, false
}

// IdentifierWithValue returns the identifier whose value is v.
func (f *File) IdentifierWithValue(v IdentifierValue) (*Identifier, bool) {
	for i := range f.Identifiers {
		if f.Identifiers[i].Value == v {
			return &f.Identifiers[i], true
		}
	}

	return nil, false
}

// NextGeneral is the value ADD/IDENTIFIER gives a new general identifier
// with no value of its own: one past the highest general identifier in
// the file, from FirstGeneral.
func (f *File) NextGeneral() IdentifierValue {
	next := IdentifierValue(FirstGeneral)

	for _, id := range f.Identifiers {
		if uint32(id.Value)&0xFFFF0000 == FirstGeneral && id.Value >= next {
			next = id.Value + 1
		}
	}

	return next
}

// HeldBy returns the identifiers uic holds, in the file's order, each
// with the attributes its holder record gives.
func (f *File) HeldBy(uic UIC) []HeldIdentifier {
	var held []HeldIdentifier

	for _, id := range f.Identifiers {
		for _, h := range id.Holders {
			if h.UIC == uic {
				held = append(held, HeldIdentifier{Identifier: id, Attributes: h.Attributes})

				break
			}
		}
	}

	return held
}

// DropHolder takes uic's holder records out of every identifier, as
// $REM_IDENT does when uic's own identifier is removed.
func (f *File) DropHolder(uic UIC) {
	for i := range f.Identifiers {
		f.Identifiers[i].Holders = slices.DeleteFunc(f.Identifiers[i].Holders, func(h Holder) bool { return h.UIC == uic })
	}
}

// MoveHolder makes every holder record of from one of to, as $MOD_IDENT
// does when a UIC identifier's value changes.
func (f *File) MoveHolder(from, to UIC) {
	for i := range f.Identifiers {
		for j := range f.Identifiers[i].Holders {
			if h := &f.Identifiers[i].Holders[j]; h.UIC == from {
				h.UIC = to
			}
		}
	}
}

// migrate moves what records of an older file held (Legacy) to their
// identifiers' holders, and reports an identifier the file doesn't have.
func (f *File) migrate() error {
	for i := range f.Accounts {
		r := &f.Accounts[i]

		for _, h := range r.Legacy {
			id, ok := f.IdentifierNamed(h.Name)
			if !ok {
				if v, err := ParseIdentifierValue(h.Name); err == nil {
					id, ok = f.IdentifierWithValue(v)
				}
			}

			if !ok {
				return &valueError{text: h.Name, err: fmt.Errorf("account %s holds %s, which is no identifier in the file", r.Username, h.Name)}
			}

			if !slices.ContainsFunc(id.Holders, func(have Holder) bool { return have.UIC == r.UIC }) {
				id.Holders = append(id.Holders, Holder{UIC: r.UIC, Attributes: h.Attributes})
			}
		}

		r.Legacy = nil
	}

	return nil
}

// Clone returns a copy of the file that shares nothing with it.
func (f *File) Clone() *File {
	c := &File{Version: f.Version, Accounts: slices.Clone(f.Accounts), Identifiers: slices.Clone(f.Identifiers)}

	for i := range c.Accounts {
		c.Accounts[i].Legacy = slices.Clone(c.Accounts[i].Legacy)
	}

	for i := range c.Identifiers {
		c.Identifiers[i].Holders = slices.Clone(c.Identifiers[i].Holders)
	}

	return c
}

// Environmental are the identifiers every rights database has, which VMS
// 7.1's CREATE/RIGHTS made in a new one (the probe's case 6): how a
// process logged in, and two its software uses. They aren't in the
// file: the system gives them, and AUTHORIZE shows them among the file's.
var Environmental = []Identifier{
	{Name: "BATCH", Value: IdentifierValue(vmsdef.Symbols["KGB$K_BATCH_ID"])},
	{Name: "DIALUP", Value: IdentifierValue(vmsdef.Symbols["KGB$K_DIALUP_ID"])},
	{Name: "INTERACTIVE", Value: IdentifierValue(vmsdef.Symbols["KGB$K_INTERACTIVE_ID"])},
	{Name: "LOCAL", Value: IdentifierValue(vmsdef.Symbols["KGB$K_LOCAL_ID"])},
	{Name: "NETWORK", Value: IdentifierValue(vmsdef.Symbols["KGB$K_NETWORK_ID"])},
	{Name: "REMOTE", Value: IdentifierValue(vmsdef.Symbols["KGB$K_REMOTE_ID"])},
	{Name: "DECWINDOWS", Value: IdentifierValue(vmsdef.Symbols["KGB$K_DECWINDOWS_ID"])},
	{Name: "VMS$BUFFER_OBJECT_USER", Value: IdentifierValue(vmsdef.Symbols["KGB$K_BOBUSER_ID"])},
}

// EnvironmentalNamed returns the environmental identifier called name, or
// whose value is v when name is "".
func EnvironmentalNamed(name string, v IdentifierValue) (Identifier, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))

	for _, id := range Environmental {
		if name != "" && id.Name == name || name == "" && id.Value == v {
			return id, true
		}
	}

	return Identifier{}, false
}
