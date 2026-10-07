package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/sched"
)

// SHOW SYSTEM and SHOW PROCESS (docs/PHASE-44.md, subtask 10): the
// console's reports of the process table, built here, where the data is.
//
// SHOW SYSTEM's layout is real VMS's, column for column, from the author's
// VMS 7.1 SIMH system's output:
//
//	OpenVMS V7.1  on node SIMVAX   7-OCT-2026 11:37:42.07  Uptime  0 00:00:31
//	  Pid    Process Name    State  Pri      I/O       CPU       Page flts  Pages
//	00000101 SWAPPER         HIB     16        0   0 00:00:00.01         0      0
//
// govax's own title differs from VMS's in its words, by the author's
// choice: the system is "GOVAX <govax's version>" rather than "OpenVMS
// V7.3", and the node is the host's short name, so the display says
// what the system really is (the console supplies both). $GETSYI's node
// name is unchanged. Unconfirmed: how a node name shorter than six
// characters is padded (to six, here), and how the uptime's day count
// grows past 9.

// CPU time accounting. A process is charged the emulated time that
// passes while it holds the CPU: at each scheduling call (and before a
// switch), the time since the last charge goes to the process the CPU
// held. Time the scheduler spends idle (idle.go) is nobody's. In
// quantum-clock mode that is the process's instructions at the clock's
// rate; with the host's clock, real time.

// accountTime charges the time since the last charge to the process the
// CPU holds. It does nothing when the scheduler isn't installed.
func (sys *System) accountTime() {
	if sys.engine == nil {
		return
	}

	now := sys.Clock()

	if cur := sys.Current(); cur != nil && now > sys.lastCharge {
		cur.cpuTime += now - sys.lastCharge
	}

	sys.lastCharge = now
}

// CPUTime returns the CPU time env's process has used, in VMS time units
// (100ns): $GETJPI's JPI$_CPUTIM (in 10ms units) and SHOW SYSTEM's CPU
// column. Without the scheduler, process 1 is the only process and has
// held the CPU since the system booted.
func (sys *System) CPUTime(env *Environment) uint64 {
	if sys.engine == nil {
		if env == sys.Current() {
			return sys.Clock() - sys.BootTime
		}

		return env.cpuTime
	}

	sys.accountTime()

	return env.cpuTime
}

// mappedPages counts the valid pages in a process's P0 and P1 regions:
// SHOW SYSTEM's "Pages" (VMS's is the physical pages in the working set;
// govax has no paging, so a valid page is a resident one).
func (sys *System) mappedPages(env *Environment) int {
	if env.Space == nil {
		return 0
	}

	as := env.Space.AddressSpace
	count := func(base uint32, pages uint32) int {
		n := 0

		for i := range pages {
			if pte, err := sys.mem.LookupPTEIn(sys.cpu, as, base+i*pageSize); err == nil && pte.Valid() {
				n++
			}
		}

		return n
	}

	return count(0, env.Space.P0Pages) + count(0x80000000-env.Space.P1Pages*pageSize, env.Space.P1Pages)
}

// processState is SHOW SYSTEM's state for env: the scheduler's, or, with
// no scheduler installed, CUR for the process the CPU holds and COM for
// the rest.
func (sys *System) processState(env *Environment) (string, int) {
	if info, ok := sys.sched.Info(handle(env)); ok && sys.engine != nil {
		return sched.StateName(info.State, info.Resource), info.Priority
	}

	pri := int(env.currentPriority())

	if env == sys.Current() {
		return sched.StateCUR.String(), pri
	}

	return sched.StateCOM.String(), pri
}

// systemTitle is SHOW SYSTEM's first line: the system ("OpenVMS V7.1"
// on VMS), the node, the time now, and how long since the system booted
// (now and boot are VMS times).
func systemTitle(system, node string, now, boot uint64) string {
	date, _ := formatVMSTime(now, false)
	up := (now - boot) / 10_000_000 // seconds

	return fmt.Sprintf("%s  on node %-6s  %s  Uptime %2d %02d:%02d:%02d",
		system, node, date, up/86400, up/3600%24, up/60%60, up%60)
}

// systemHeadings is SHOW SYSTEM's second line.
const systemHeadings = "  Pid    Process Name    State  Pri      I/O       CPU       Page flts  Pages"

// systemLine is one process's line of SHOW SYSTEM: cpu is its CPU time
// in VMS time units, shown as a delta time ("   0 00:00:00.01").
func systemLine(pid uint32, name, state string, pri int, io uint32, cpu uint64, faults, pages int) string {
	cpuText, ok := formatVMSTime(uint64(-int64(cpu)), false)
	if !ok {
		cpuText = "****************"
	}

	return fmt.Sprintf("%08X %-15s %-6s%4d%9d%s%10d%7d", pid, name, state, pri, io, cpuText, faults, pages)
}

// SystemReport is SHOW SYSTEM's display, a line per process, in PID
// order, after the title (naming system and node) and the column
// headings.
func (sys *System) SystemReport(system, node string) []string {
	lines := []string{systemTitle(system, node, sys.Clock(), sys.BootTime), systemHeadings}

	for _, env := range sys.Processes() {
		state, pri := sys.processState(env)
		lines = append(lines, systemLine(env.Process.PID, env.Process.Name, state, pri, 0, sys.CPUTime(env), 0, sys.mappedPages(env)))
	}

	return lines
}

// ProcessReport is SHOW PROCESS's display for env. The layout is VMS's
// as best remembered (unconfirmed; a VMS 7.3 probe would settle it):
//
//	 7-OCT-2026 11:37:42.07   User: SYSTEM           Process ID:   00000301
//	                          Node: mymac            Process name: "SYSTEM"
//
//	Terminal:           TTA0:
//	User Identifier:    [1,4]
//	Base priority:      4
//	Default file spec:  DUA0:[WORK]
//
// The UIC is shown numerically ([group,member], octal): govax has no
// rights database to name it. node is the node to name, as for
// SystemReport.
func (env *Environment) ProcessReport(node string) []string {
	date, _ := formatVMSTime(env.Clock(), false)
	p := env.Process

	defaultSpec := ""
	if env.Session != nil {
		defaultSpec = env.Session.DefaultString()
	}

	return []string{
		"",
		fmt.Sprintf("%s   User: %-16s Process ID:   %08X", date, p.Username, p.PID),
		fmt.Sprintf("%26sNode: %-16s Process name: %q", "", node, p.Name),
		"",
		fmt.Sprintf("Terminal:           %s", p.Terminal),
		fmt.Sprintf("User Identifier:    [%o,%o]", p.UIC>>16, p.UIC&0xFFFF),
		fmt.Sprintf("Base priority:      %d", p.BasePriority),
		fmt.Sprintf("Default file spec:  %s", defaultSpec),
		"",
	}
}
