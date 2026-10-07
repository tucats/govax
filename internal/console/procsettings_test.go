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

	if got := processSettings(); got != corevms.DefaultProcessSettings() {
		t.Errorf("with bad values, ProcessSettings = %+v, want the defaults", got)
	}
}
