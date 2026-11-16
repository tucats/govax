package authorize

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tucats/govax/internal/uaf"
)

// This file lays out AUTHORIZE's reports of accounts: SHOW's full report
// of each account and its brief one-line form, which LIST writes into
// SYSUAF.LIS too. Every column is where VMS 7.1 put it in testdata/uaf95's
// runs (the probe's cases 1, 2, 26, 49, 98, and 117 to 119).
//
// The full report has lines for what govax doesn't keep (the phase
// doc's A.3): access times, primary and secondary days, the expiration
// date, the password's minimum length and lifetime, and the login
// failure count. They are shown as VMS shows an account that has none of
// them: every day primary but Saturday and Sunday, no access
// restrictions, no expiration, no minimum or lifetime, no failures.

// briefHeading is the brief report's heading and the blank line after it.
var briefHeading = []string{
	"       Owner         Username           UIC       Account  Privs Pri Directory",
	"",
}

// briefLine is an account's line of the brief report: the owner (its
// first 20 characters, unconfirmed: no probe case had a longer one), the
// username, the UIC with its comma in column 41, the account, the
// privilege category, the priority, and the home, or "Disuser" for a
// disabled account.
func briefLine(r *uaf.Record) string {
	owner := r.Owner
	if len(owner) > 20 {
		owner = owner[:20]
	}

	home := r.Device + r.Directory
	if r.Flags&uaf.FlagDISUSER != 0 {
		home = "Disuser"
	}

	return fmt.Sprintf("%-20s %-12s%8s%-9s%-8s %-6s %2d %s", owner, r.Username,
		fmt.Sprintf("[%o", r.UIC.Group()), fmt.Sprintf(",%o]", r.UIC.Member()),
		r.Account, privilegeCategory(r.Privileges), r.Priority, home)
}

// The privilege categories of the *Guide to System Security*'s Table 5-5,
// from least to most powerful. The brief report shows an account's
// highest. VMS 7.1 showed "Normal" for TMPMBX and NETMBX, "System" with
// OPER, and "All" with SYSPRV; the others' words are the table's,
// unconfirmed.
var privilegeCategories = []struct {
	name       string
	privileges []string
}{
	{"Normal", []string{"NETMBX", "TMPMBX"}},
	{"Group", []string{"GROUP", "GRPPRV"}},
	{"Devour", []string{"ACNT", "ALLSPOOL", "BUGCHK", "EXQUOTA", "GRPNAM", "PRMCEB", "PRMGBL", "PRMMBX", "SHMEM"}},
	{"System", []string{"ALTPRI", "AUDIT", "OPER", "PSWAPM", "WORLD", "SECURITY", "SYSLCK"}},
	{"Objects", []string{"DIAGNOSE", "IMPORT", "MOUNT", "READALL", "SYSGBL", "VOLPRO"}},
	{"All", []string{"BYPASS", "CMEXEC", "CMKRNL", "DETACH", "DOWNGRADE", "LOG_IO", "PFNMAP", "PHY_IO", "SETPRV", "SHARE", "SYSNAM", "SYSPRV", "UPGRADE"}},
}

// privilegeCategory is the category of the most powerful of p's
// privileges, "None" for none. A bit no privilege is named for (ALL sets
// every one) counts as All.
func privilegeCategory(p uaf.Privileges) string {
	named := uaf.Privileges(0)
	best := "None"

	for _, c := range privilegeCategories {
		for _, name := range c.privileges {
			bit, _ := uaf.PrivilegeBit(name)
			named |= 1 << bit

			if p&(1<<bit) != 0 {
				best = c.name
			}
		}
	}

	if p&^named != 0 {
		best = "All"
	}

	return best
}

// fullReport is an account's full report, starting with its blank line;
// with held, the identifiers its UIC holds follow the privileges.
func fullReport(f *uaf.File, r *uaf.Record, held bool) []string {
	q := &r.Quotas

	lines := []string{
		"",
		fmt.Sprintf("Username: %-33sOwner:  %s", r.Username, r.Owner),
		fmt.Sprintf("Account:  %-33sUIC:    %s (%s)", r.Account, r.UIC, namedUIC(f, r.UIC)),
		fmt.Sprintf("CLI:      %-33sTables: %s", r.CLI, r.CLITables),
		"Default:  " + r.Device + r.Directory,
		"LGICMD:   " + r.LGICMD,
		"Flags: " + flagWords(r.Flags),
		"Primary days:   Mon Tue Wed Thu Fri        ",
		"Secondary days:                     Sat Sun",
		"No access restrictions",
		fmt.Sprintf("Expiration:  %16s    Pwdminimum: %2d   Login Fails: %5d", "(none)", 0, 0),
		fmt.Sprintf("Pwdlifetime: %16s    Pwdchange:%19s ", "(none)", loginTime(r.PasswordChanged)),
		fmt.Sprintf("Last Login: %17s (interactive), %17s (non-interactive)", loginTime(r.LastInteractive), loginTime(r.LastNonInteractive)),
	}

	quota := func(a string, av uint32, b string, bv uint32, c string, cv uint32) string {
		return fmt.Sprintf("%-*s%d  %-*s%d  %-*s%d", 18-digits(av), a, av, 16-digits(bv), b, bv, 19-digits(cv), c, cv)
	}

	lines = append(lines,
		quota("Maxjobs:", q.MAXJOBS, "Fillm:", q.FILLM, "Bytlm:", q.BYTLM),
		quota("Maxacctjobs:", q.MAXACCTJOBS, "Shrfillm:", q.SHRFILLM, "Pbytlm:", q.PBYTLM),
		quota("Maxdetach:", q.MAXDETACH, "BIOlm:", q.BIOLM, "JTquota:", q.JTQUOTA),
		quota("Prclm:", q.PRCLM, "DIOlm:", q.DIOLM, "WSdef:", q.WSDEFAULT),
		quota("Prio:", r.Priority, "ASTlm:", q.ASTLM, "WSquo:", q.WSQUOTA),
		quota("Queprio:", r.QueuePriority, "TQElm:", q.TQELM, "WSextent:", q.WSEXTENT),
		fmt.Sprintf("CPU:%14s  %-*s%d  %-*s%d", cpuText(q.CPUTIME), 16-digits(q.ENQLM), "Enqlm:", q.ENQLM, 19-digits(q.PGFLQUOTA), "Pgflquo:", q.PGFLQUOTA),
	)

	lines = append(lines, "Authorized Privileges: ")
	lines = append(lines, privilegeLines(r.Privileges)...)
	lines = append(lines, "Default Privileges: ")
	lines = append(lines, privilegeLines(r.DefaultPrivileges)...)

	if held {
		if ids := f.HeldBy(r.UIC); len(ids) > 0 {
			lines = append(lines, rightsHeading)

			for _, h := range ids {
				lines = append(lines, identifierLine(h.Identifier.Name, h.Identifier.Value, h.Attributes))
			}
		}
	}

	return lines
}

// digits is how many digits n has.
func digits(n uint32) int { return len(fmt.Sprint(n)) }

// namedUIC is a UIC as SHOW writes it after the numbers: by its
// identifiers' names, "[CASED,UAFD]" (the group's identifier, [g,177777],
// then the member's), "[UAFA]" when the group has none, and the numbers
// again when the UIC itself has no identifier (the probe's cases 2, 9,
// and 26).
func namedUIC(f *uaf.File, uic uaf.UIC) string {
	member, ok := f.IdentifierWithValue(uaf.IdentifierValue(uic))
	if !ok {
		return uic.String()
	}

	if group, ok := f.IdentifierWithValue(uaf.IdentifierValue(uaf.NewUIC(uic.Group(), 0o177777))); ok {
		return "[" + group.Name + "," + member.Name + "]"
	}

	return "[" + member.Name + "]"
}

// flagDisplay are the flags' words as SHOW writes them, by uaf.FlagNames'
// order. VMS 7.1 showed DisCtlY, DefCLI, LockPwd, DisUser, DisWelcome,
// DisNewMail, and Captive (case 26); the others' are spelled the same way,
// unconfirmed.
var flagDisplay = map[string]string{
	"DISCTLY": "DisCtlY", "DEFCLI": "DefCLI", "LOCKPWD": "LockPwd", "RESTRICTED": "Restricted",
	"DISUSER": "DisUser", "DISWELCOME": "DisWelcome", "DISNEWMAIL": "DisNewMail", "DISMAIL": "DisMail",
	"GENPWD": "GenPwd", "PWD_EXPIRED": "Pwd_Expired", "PWD2_EXPIRED": "Pwd2_Expired", "AUDIT": "Audit",
	"DISREPORT": "DisReport", "DISRECONNECT": "DisReconnect", "AUTOLOGIN": "AutoLogin",
	"DISFORCE_PWD_CHANGE": "DisForce_Pwd_Change", "CAPTIVE": "Captive", "DISIMAGE": "DisImage",
	"DISPWDDIC": "DisPwdDic", "DISPWDHIS": "DisPwdHis",
}

// flagWords is the Flags line's text after "Flags: ": a blank before each
// set flag's word.
func flagWords(f uaf.Flags) string {
	var b strings.Builder

	for _, name := range f.Names() {
		b.WriteString(" " + flagDisplay[name])
	}

	return b.String()
}

// loginTime is a time as SHOW writes a login or a password change,
// "15-NOV-2026 17:19" in local time, or "(none)".
func loginTime(t time.Time) string {
	if t.IsZero() {
		return "(none)"
	}

	return strings.ToUpper(t.Local().Format("02-Jan-2006 15:04"))
}

// cpuText is /CPUTIME's limit as SHOW writes it, "0 00:10:00" (case 26),
// or "(none)" for no limit. Hundredths aren't shown (unconfirmed).
func cpuText(ticks uint32) string {
	if ticks == 0 {
		return "(none)"
	}

	s := ticks / 100

	return fmt.Sprintf("%d %02d:%02d:%02d", s/86400, s/3600%24, s/60%60, s%60)
}

// privilegeOrder is the privileges' names in the order SHOW lists them:
// alphabetical by the older name of a bit two names share (DETACH's place
// for IMPERSONATE, case 2), each written as uaf names it.
var privilegeOrder = func() []string {
	type entry struct{ key, name string }

	var entries []entry

	for _, name := range uaf.Privileges(^uint64(0) >> 1).Names() {
		key := name
		if name == "IMPERSONATE" {
			key = "DETACH"
		}

		entries = append(entries, entry{key, name})
	}

	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.key, b.key) })

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}

	return names
}()

// privilegeLines are the privileges' lines under their heading: eight to a
// line in columns of ten, each name cut to ten characters, as VMS 7.1 let
// IMPERSONATE run into the next ("IMPERSONATDIAGNOSE", case 2).
func privilegeLines(p uaf.Privileges) []string {
	var (
		lines []string
		line  strings.Builder
		n     int
	)

	for _, name := range privilegeOrder {
		bit, _ := uaf.PrivilegeBit(name)
		if p&(1<<bit) == 0 {
			continue
		}

		if n == 0 {
			line.WriteString("  ")
		}

		if len(name) > 10 {
			name = name[:10]
		}

		fmt.Fprintf(&line, "%-10s", name)

		if n++; n == 8 {
			lines = append(lines, strings.TrimRight(line.String(), " "))
			line.Reset()
			n = 0
		}
	}

	if n > 0 {
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}

	return lines
}

// The identifier reports' headings: SHOW/IDENTIFIER's, the holders' under
// an identifier, and SHOW/RIGHTS's (and SHOW/FULL's) of what a UIC holds.
const (
	identifierHeading = "  Name                             Value           Attributes"
	holderHeading     = "    Holder                           Attributes"
	rightsHeading     = "Identifier                         Value           Attributes"
)

// identifierLine is an identifier's line: its name, its value as
// AUTHORIZE writes values, and the attributes, each followed by a blank.
func identifierLine(name string, v uaf.IdentifierValue, a uaf.Attributes) string {
	return fmt.Sprintf("  %-33s%-16s%s", name, identifierValue(v), attributeWords(a))
}

// holderLine is a holder's line under its identifier: the name of the
// holder's UIC identifier, or the UIC's numbers if it has none
// (unconfirmed), and the holder record's attributes.
func holderLine(f *uaf.File, h uaf.Holder) string {
	name := identifierValue(uaf.IdentifierValue(h.UIC))
	if id, ok := f.IdentifierWithValue(uaf.IdentifierValue(h.UIC)); ok {
		name = id.Name
	}

	return fmt.Sprintf("    %-33s%s", name, attributeWords(h.Attributes))
}

// attributeWords are attributes' names, each followed by a blank
// ("RESOURCE ", round 3's L8).
func attributeWords(a uaf.Attributes) string {
	var b strings.Builder

	for _, name := range a.Names() {
		b.WriteString(name + " ")
	}

	return b.String()
}
