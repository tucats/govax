package corevms

import (
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// sampleShowSystem is real SHOW SYSTEM output, from the author's VMS 7.1
// system (trailing blanks removed).
const sampleShowSystem = `OpenVMS V7.1  on node SIMVAX   7-OCT-2026 11:37:42.07  Uptime  0 00:00:31
  Pid    Process Name    State  Pri      I/O       CPU       Page flts  Pages
00000101 SWAPPER         HIB     16        0   0 00:00:00.01         0      0
0000010D SECURITY_SERVER HIB     10       27   0 00:00:00.03       952   1028
0000010E DNS$ADVER       LEF      4     1028   0 00:00:00.11      4917   1416
00000114 SYSTEM          CUR      7       95   0 00:00:00.01       649    469`

// TestSystemLayout: SHOW SYSTEM's lines, laid out from the sample's
// values, are the sample's lines.
func TestSystemLayout(t *testing.T) {
	want := strings.Split(sampleShowSystem, "\n")

	now := vmsdef.Time(time.Date(2026, time.October, 7, 11, 37, 42, 70_000_000, time.Local))
	boot := now - 31*10_000_000

	got := []string{
		systemTitle("OpenVMS V7.1", "SIMVAX", now, boot),
		systemHeadings,
		systemLine(0x101, "SWAPPER", "HIB", 16, 0, 1*100_000, 0, 0),
		systemLine(0x10D, "SECURITY_SERVER", "HIB", 10, 27, 3*100_000, 952, 1028),
		systemLine(0x10E, "DNS$ADVER", "LEF", 4, 1028, 11*100_000, 4917, 1416),
		systemLine(0x114, "SYSTEM", "CUR", 7, 95, 1*100_000, 649, 469),
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// TestSystemReport: govax's SHOW SYSTEM lists its processes with their
// scheduling states, leaving out a deleted one.
func TestSystemReport(t *testing.T) {
	env, _ := fixture()
	withScheduler(env)

	env.Process.Name = "SYSTEM"
	two := newProcess(t, env)
	two.Process.Name = "TWO"
	three := newProcess(t, env)
	three.Process.Name = "THREE"

	_ = callWaiting(two, serviceSysHiber)
	env.DeleteProcess(three)

	lines := env.SystemReport("GOVAX 1.0-1", "myhost")
	if len(lines) != 4 {
		t.Fatalf("%d lines, want 4:\n%s", len(lines), strings.Join(lines, "\n"))
	}

	if !strings.HasPrefix(lines[0], "GOVAX 1.0-1  on node myhost  ") || lines[1] != systemHeadings {
		t.Errorf("title and headings:\n%s\n%s", lines[0], lines[1])
	}

	if !strings.HasPrefix(lines[2], "00000301 SYSTEM          COM      4") ||
		!strings.HasPrefix(lines[3], "00000302 TWO             HIB      4") {
		t.Errorf("process lines:\n%s\n%s", lines[2], lines[3])
	}
}

// TestCPUTime: without the scheduler, process 1 has had the CPU since
// boot; $GETJPI's JPI$_CPUTIM is in 10ms units.
func TestCPUTime(t *testing.T) {
	env, _ := fixture()

	var now uint64 = 1_000_000_000
	env.Clock = func() uint64 { return now }
	env.BootTime = now - 25*100_000 // 25 hundredths

	if got := env.CPUTime(env); got != 25*100_000 {
		t.Errorf("CPU time %d, want %d", got, 25*100_000)
	}

	if v := jpiItemsByName["JPI$_CPUTIM"](env); v != itemLong(25) {
		t.Errorf("JPI$_CPUTIM %+v, want 25", v)
	}
}
