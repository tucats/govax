package console

import (
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/corevms"
)

// withSettings sets each key for the length of the test, putting the old
// values back afterwards.
func withSettings(t *testing.T, values map[string]string) {
	t.Helper()

	for key, v := range values {
		old, had := settings.Get(key), settings.Exists(key)

		t.Cleanup(func() {
			if had {
				settings.Set(key, old)
			} else {
				_ = settings.Delete(key)
			}
		})

		settings.Set(key, v)
	}
}

// TestProcessSettings: the vax.process.* keys reach the System the
// console builds, and bad values keep the defaults.
func TestProcessSettings(t *testing.T) {
	withSettings(t, map[string]string{schedulerSetting: "true", quantumSetting: "5000", preemptSetting: "user"})

	c, _ := newTestConsole(t)

	want := corevms.ProcessSettings{Scheduler: true, Quantum: 5000, Preempt: corevms.PreemptUser}
	if got := c.RTL.ProcessSettings; got != want {
		t.Errorf("ProcessSettings = %+v, want %+v", got, want)
	}

	withSettings(t, map[string]string{schedulerSetting: "false", quantumSetting: "-3", preemptSetting: "sometimes"})

	want = corevms.DefaultProcessSettings()
	want.Scheduler = false

	if got := processSettings(); got != want {
		t.Errorf("with the scheduler off and bad values, ProcessSettings = %+v, want %+v", got, want)
	}
}

// TestProcessSettings_schedulerDefault: with vax.process.scheduler not
// set, the scheduler is on (Phase 48, Decision 5); set to false, it's
// off.
func TestProcessSettings_schedulerDefault(t *testing.T) {
	withSettings(t, map[string]string{schedulerSetting: "true"})

	_ = settings.Delete(schedulerSetting)

	if !processSettings().Scheduler {
		t.Error("with vax.process.scheduler unset, the scheduler is off")
	}

	settings.Set(schedulerSetting, "false")

	if processSettings().Scheduler {
		t.Error("with vax.process.scheduler false, the scheduler is on")
	}
}
