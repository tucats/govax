package dclcmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET PROCESS/PRIVILEGES and SHOW PROCESS/PRIVILEGES (docs/planning/PHASE-93 -
// file protection.md, subtask 5). A privilege enabled or disabled with SET
// PROCESS/PRIVILEGES is changed as $SETPRV changes it with its permanent
// flag, for a caller in supervisor mode, DCL's: a privilege the process
// isn't authorized for (or for SETPRV) isn't enabled. The layout of SHOW
// PROCESS/PRIVILEGES is VMS 7.1's, from Phase 93's probe
// (testdata/prot93/vax).

// privilegeDescriptions are SHOW PROCESS/PRIVILEGES' words for each
// privilege, by the name it shows, as VMS 7.1 wrote them.
var privilegeDescriptions = map[string]string{
	"ACNT":        "may suppress accounting messages",
	"ALLSPOOL":    "may allocate spooled device",
	"ALTPRI":      "may set any priority value",
	"AUDIT":       "may direct audit to system security audit log",
	"BUGCHK":      "may make bug check log entries",
	"BYPASS":      "may bypass all object access controls",
	"CMEXEC":      "may change mode to exec",
	"CMKRNL":      "may change mode to kernel",
	"IMPERSONATE": "may impersonate another user",
	"DIAGNOSE":    "may diagnose devices",
	"DOWNGRADE":   "may downgrade object secrecy",
	"EXQUOTA":     "may exceed disk quota",
	"GROUP":       "may affect other processes in same group",
	"GRPNAM":      "may insert in group logical name table",
	"GRPPRV":      "may access group objects via system protection",
	"IMPORT":      "may set classification for unlabeled object",
	"LOG_IO":      "may do logical i/o",
	"MOUNT":       "may execute mount acp function",
	"NETMBX":      "may create network device",
	"OPER":        "may perform operator functions",
	"PFNMAP":      "may map to specific physical pages",
	"PHY_IO":      "may do physical i/o",
	"PRMCEB":      "may create permanent common event clusters",
	"PRMGBL":      "may create permanent global sections",
	"PRMMBX":      "may create permanent mailbox",
	"PSWAPM":      "may change process swap mode",
	"READALL":     "may read anything as the owner",
	"SECURITY":    "may perform security administration functions",
	"SETPRV":      "may set any privilege bit",
	"SHARE":       "may assign channels to non-shared devices",
	"SHMEM":       "may create/delete objects in shared memory",
	"SYSGBL":      "may create system wide global sections",
	"SYSLCK":      "may lock system wide resources",
	"SYSNAM":      "may insert in system logical name table",
	"SYSPRV":      "may access objects via system protection",
	"TMPMBX":      "may create temporary mailbox",
	"UPGRADE":     "may upgrade object integrity",
	"VOLPRO":      "may override volume protection",
	"WORLD":       "may affect other processes in the world",
}

// shownPrivilege is one privilege as SHOW PROCESS/PRIVILEGES lists it.
type shownPrivilege struct {
	bit  uint
	name string
}

// shownPrivileges are the privileges in SHOW PROCESS/PRIVILEGES' order:
// alphabetical by the older names (corevms.PrivilegeNames), DETACH shown
// as IMPERSONATE where DETACH would be, as VMS 7.1 listed them.
var shownPrivileges = func() []shownPrivilege {
	var list []shownPrivilege

	for bit, name := range corevms.PrivilegeNames {
		if name != "" {
			list = append(list, shownPrivilege{bit: uint(bit), name: name})
		}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })

	for i := range list {
		if list[i].name == "DETACH" {
			list[i].name = "IMPERSONATE"
		}
	}

	return list
}()

// privilegesPerLine is how many privileges a line of "Authorized
// privileges" has, after a space, each in 10 columns, cut short if
// longer (IMPERSONATE is IMPERSONAT).
const privilegesPerLine = 8

// processPrivilegesReport is SHOW PROCESS/PRIVILEGES' text for env's
// process after its first lines (ProcessReport's heading): the
// privileges it is authorized for, the ones enabled now with their
// descriptions, and the rights it holds (processRightsReport). Its blank
// lines are a space, as VMS's were.
func processPrivilegesReport(env *corevms.Environment) []string {
	p := env.Process
	lines := []string{" ", "Authorized privileges:"}

	var row strings.Builder

	n := 0

	for _, sp := range shownPrivileges {
		if p.AuthorizedPrivileges&(1<<sp.bit) == 0 {
			continue
		}

		if row.Len() == 0 {
			row.WriteByte(' ')
		}

		fmt.Fprintf(&row, "%-10.10s", sp.name)

		if n++; n%privilegesPerLine == 0 {
			lines = append(lines, strings.TrimRight(row.String(), " "))
			row.Reset()
		}
	}

	if row.Len() > 0 {
		lines = append(lines, strings.TrimRight(row.String(), " "))
	}

	lines = append(lines, " ", "Process privileges:")

	for _, sp := range shownPrivileges {
		if p.CurrentPrivileges&(1<<sp.bit) != 0 {
			lines = append(lines, fmt.Sprintf(" %-20s %s", sp.name, privilegeDescriptions[sp.name]))
		}
	}

	return append(lines, processRightsReport(env)...)
}

// processRightsReport is the "Process rights" section SHOW PROCESS's
// /PRIVILEGES and /RIGHTS end with: a space line, the heading, and each
// identifier the process holds, by name (or %X and its value when the
// rights database has no name for it), in 34 columns after a space, as
// VMS 7.1 wrote them (testdata/uaf95). A process holding no rights has
// no section (unconfirmed). VMS 7.1 then listed "System rights" (its
// SYS$NODE_ identifier), which govax has none of, so that section is
// left out.
func processRightsReport(env *corevms.Environment) []string {
	p := env.Process
	if len(p.Rights) == 0 {
		return nil
	}

	lines := []string{" ", "Process rights:"}

	for _, r := range p.Rights {
		name, ok := env.IdentifierName(r.Value)
		if !ok {
			name = fmt.Sprintf("%%X%08X", r.Value)
		}

		lines = append(lines, fmt.Sprintf(" %-34s", name))
	}

	return lines
}

// SetProcessPrivileges is SET PROCESS/PRIVILEGES=(name[,...]): each
// privilege named is enabled, and each named with NO before it disabled,
// in the process whose DCL runs the command; ALL and NOALL are every
// privilege. A name that isn't a privilege's is %DCL-W-IVKEYW, and
// nothing changes. A privilege the process may not enable is left
// disabled, with VMS's warning, %SYSTEM-W-NOTALLPRIV, after the others
// are changed (unconfirmed: the $STATUS VMS leaves after it; govax's is
// success).
func (c *Context) SetProcessPrivileges(items []string) error {
	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	type change struct {
		mask   uint64
		enable bool
	}

	var changes []change

	for _, item := range items {
		for _, key := range strings.Split(strings.Trim(item, "()"), ",") {
			key = strings.ToUpper(strings.TrimSpace(key))

			switch {
			case key == "":
				continue
			case key == "ALL":
				changes = append(changes, change{corevms.AllPrivileges, true})
			case key == "NOALL":
				changes = append(changes, change{corevms.AllPrivileges, false})
			default:
				if mask, ok := corevms.PrivilegeMask(key); ok {
					changes = append(changes, change{mask, true})

					continue
				}

				rest, negated := strings.CutPrefix(key, "NO")
				mask, ok := corevms.PrivilegeMask(rest)

				if !negated || !ok {
					return vmserrors.NewSegment(vmserrors.CLI_IVKEYW, key)
				}

				changes = append(changes, change{mask, false})
			}
		}
	}

	notAll := false

	for _, ch := range changes {
		if _, status := c.RTL.SetProcessPrivileges(ch.mask, ch.enable); status == ssNotAllPriv {
			notAll = true
		}
	}

	if notAll {
		c.Printf("%%%s\n", vmsdef.FormatMessage(ssNotAllPriv&^7))
	}

	return nil
}

// ssNotAllPriv is SS$_NOTALLPRIV, $SETPRV's success that left some
// privileges out. Its message shows as a warning, its severity bits
// cleared.
var ssNotAllPriv = vmsdef.Symbols["SS$_NOTALLPRIV"]
