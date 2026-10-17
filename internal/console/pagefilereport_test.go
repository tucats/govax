package console_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
)

// Phase 64's subtask 7 (docs/PHASE-64 - page file.md): reporting. With
// the modified page writer using the page file, SHOW MEMORY's page file
// section shows its name, size, and the blocks in use, the writer's
// write I/Os, and the pages committed; F$GETSYI's PAGEFILE_PAGE and
// PAGEFILE_FREE are the page file's size and free blocks, and its
// MPW_WRTCLUSTER and PQL_MPGFLQUOTA the parameters.
func TestPageFileReporting(t *testing.T) {
	pagefile := filepath.Join(t.TempDir(), "pagefile.sys")

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.mpw_hilimit", "10")
	setSetting(t, "vax.sysgen.mpw_lolimit", "2")
	setSetting(t, machine.WriteClusterSetting, "16")
	setSetting(t, "vax.sysgen.paging.file", `"`+pagefile+`"`)
	setSetting(t, "vax.sysgen.paging.size", "1000")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	exe := buildImage(t, c, "wspf", pfSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	// Checkpoint 1: the array written, and most of it written out to the
	// page file.
	runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("no page file open\n%s", out.String())
	}

	used, free := pf.Map.UsedBlocks(), pf.Map.FreeBlocks()
	if used == 0 {
		t.Fatal("nothing written to the page file")
	}

	out.Reset()

	for _, line := range []string{
		"SHOW MEMORY",
		`WRITE SYS$OUTPUT "PAGE=", F$GETSYI("PAGEFILE_PAGE"), " FREE=", F$GETSYI("PAGEFILE_FREE")`,
		`WRITE SYS$OUTPUT "SWAP=", F$GETSYI("SWAPFILE_PAGE"), " ", F$GETSYI("SWAPFILE_FREE")`,
		`WRITE SYS$OUTPUT "CLUSTER=", F$GETSYI("MPW_WRTCLUSTER"), " MPGFL=", F$GETSYI("PQL_MPGFLQUOTA")`,
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	counts := c.RTL.PageCounts
	report := c.RTL.PageFileReport()

	for _, want := range []string{
		fmt.Sprintf("        Write I/Os         %8d\n", counts.WriteIOs),
		"    Page file: " + pagefile + "\n",
		fmt.Sprintf("        Size               %8d\n", 1000),
		fmt.Sprintf("        Free               %8d\n", free),
		fmt.Sprintf("        In use             %8d\n", used),
		fmt.Sprintf("    Page file committed%12d\n", report.Charged),
		fmt.Sprintf("PAGE=1000 FREE=%d\n", free),
		"SWAP=0 0\n",
		"CLUSTER=16 MPGFL=16400\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output doesn't contain %q:\n%s", want, out.String())
		}
	}

	if counts.WriteIOs == 0 || report.Charged < pfPages {
		t.Errorf("%d write I/Os and %d pages committed, want writes and at least the array's %d pages",
			counts.WriteIOs, report.Charged, pfPages)
	}

	for n := uint32(1); n <= 4; n++ {
		c.CPU.SetGPR(10, n)

		if n < 4 {
			runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == n+1 })
		}
	}

	finishImage(t, c)
}
