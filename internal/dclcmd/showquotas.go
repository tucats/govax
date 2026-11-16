package dclcmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/corevms"
)

// SHOW PROCESS/QUOTAS (docs/planning/PHASE-95 - authentication.md): what
// is left of each of the process's quotas, laid out as VMS 7.1 laid
// them out in the probe's logs (testdata/uaf95/vax/uaf95i_*.log):
//
//	Process Quotas:
//	 Account name: GOVAX
//	 CPU limit:                      Infinite  Direct I/O limit:        40
//	 Buffered I/O byte count quota:     32256  Buffered I/O limit:      40
//	 Timer queue entry quota:              40  Open file quota:        298
//	 Paging file quota:                 31278  Subprocess quota:         2
//	 Default page fault cluster:           16  AST quota:               39
//	 Enqueue quota:                       200  Shared file limit:        0
//	 Max detached processes:                0  Max active jobs:          0
//
// Every line is 70 columns: a space, then a cell of 40 columns (the label
// and its value right-aligned), then two spaces and a cell of 27.

// quotaCellWidths are the widths of a quota line's two cells.
const (
	quotaLeftWidth  = 40
	quotaRightWidth = 27
)

// quotaCell is one label and value in a cell width columns wide, the
// value right-aligned; a cell too narrow for both keeps a space between
// them.
func quotaCell(label, value string, width int) string {
	pad := max(width-len(label)-len(value), 1)

	return label + strings.Repeat(" ", pad) + value
}

// processQuotasReport is SHOW PROCESS/QUOTAS' text after the report's
// heading. Each number is the $GETJPI item VMS's numbers matched in the
// probe (the log's F$GETJPI section): what is left of a quota govax
// charges (JPI$_ASTCNT, JPI$_ENQCNT, JPI$_PAGFILCNT, the subprocesses
// left of PRCLM), and the limit itself for a quota govax records but
// doesn't charge (BIOLM, DIOLM, BYTLM, TQELM, FILLM): VMS shows the
// count left, which govax's limits stand for, so VMS's 298 open files
// left of 300 are govax's 300. The default page fault cluster is
// PFCDEFAULT (vax.sysgen.pfcdefault; the probe's VAX showed 16, its
// PFCDEFAULT, testdata/pg64/vax/pg64.log), the shared file limit and the job limits the account's
// (SHRFILLM, MAXDETACH, MAXJOBS). A CPU limit of 0 is "Infinite", and
// another is a delta time (unconfirmed: the probe's accounts had none).
func processQuotasReport(env *corevms.Environment) []string {
	item := func(name string) uint32 {
		data, _ := env.JPIItem("JPI$_" + name)

		return corevms.ItemLongword(data)
	}

	number := func(name string) string { return strconv.FormatUint(uint64(item(name)), 10) }

	// PAGFILCNT goes negative when a job has charged past its quota
	// (pgflquota.go), and VMS's SHOW shows a signed count.
	pageFile := strconv.FormatInt(int64(int32(item("PAGFILCNT"))), 10)

	// The subprocesses left are this process's, as JPI$_PRCCNT counts
	// them, of the job's PRCLM.
	subprocesses := "0"
	if limit, used := item("PRCLM"), item("PRCCNT"); limit > used {
		subprocesses = strconv.FormatUint(uint64(limit-used), 10)
	}

	cpu := "Infinite"
	if limit := item("CPULIM"); limit != 0 {
		cpu = cpuLimitText(limit)
	}

	account, _ := env.JPIItem("JPI$_ACCOUNT")

	rows := [][4]string{
		{"CPU limit:", cpu, "Direct I/O limit:", number("DIOLM")},
		{"Buffered I/O byte count quota:", number("BYTLM"), "Buffered I/O limit:", number("BIOLM")},
		{"Timer queue entry quota:", number("TQLM"), "Open file quota:", number("FILLM")},
		{"Paging file quota:", pageFile, "Subprocess quota:", subprocesses},
		{"Default page fault cluster:", strconv.FormatUint(uint64(env.Sysgen.PFCDefault), 10), "AST quota:", number("ASTCNT")},
		{"Enqueue quota:", number("ENQCNT"), "Shared file limit:", number("SHRFILLM")},
		{"Max detached processes:", number("MAXDETACH"), "Max active jobs:", number("MAXJOBS")},
	}

	lines := []string{"", "Process Quotas:", " Account name: " + account}

	for _, r := range rows {
		lines = append(lines, " "+quotaCell(r[0], r[1], quotaLeftWidth)+"  "+quotaCell(r[2], r[3], quotaRightWidth))
	}

	return lines
}

// cpuLimitText is a CPU limit, in the 10-millisecond units JPI$_CPULIM
// counts, as a VMS delta time: days, then hours, minutes, seconds, and
// hundredths.
func cpuLimitText(ticks uint32) string {
	cs := ticks % 100
	s := ticks / 100

	return fmt.Sprintf("%d %02d:%02d:%02d.%02d", s/86400, s/3600%24, s/60%60, s%60, cs)
}
