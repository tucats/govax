package console

import (
	"strings"
	"testing"
)

// TestScheduler_oneProcessRun runs the FORTH interpreter with the
// scheduler installed (vax.process.scheduler on, docs/PHASE-44.md) and a
// short quantum, so its one process meets many quantum ends, and checks
// that the session is the same as without the scheduler, and that the
// scheduler charged the run's instructions to process 1.
func TestScheduler_oneProcessRun(t *testing.T) {
	typed := strings.Join([]string{
		": sq dup * ; 7 sq .",
		": count 5 0 do i . loop ; count",
		"1.5 2.25 f+ f.",
		"halt",
	}, "\n") + "\n"

	want, _ := forthSession(t, typed, nil)

	withSettings(t, map[string]string{schedulerSetting: "true", quantumSetting: "997"})

	got, c := forthSession(t, typed, nil)
	if got != want {
		t.Errorf("with the scheduler:\n%s\nwithout:\n%s", got, want)
	}

	sys := c.RTL.System
	if !sys.ProcessSettings.Scheduler {
		t.Fatal("the scheduler setting didn't reach the System")
	}

	if n := sys.CPUInstructions(c.RTL); n < 10_000 {
		t.Errorf("process 1 was charged %d instructions", n)
	}
}
