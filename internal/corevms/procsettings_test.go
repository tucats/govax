package corevms

import "testing"

// TestParsePreemptMode: the setting's three values, any case, and the
// default for no value.
func TestParsePreemptMode(t *testing.T) {
	cases := map[string]PreemptMode{"": PreemptAll, "all": PreemptAll, " User ": PreemptUser, "NONE": PreemptNone}

	for text, want := range cases {
		if got, err := ParsePreemptMode(text); err != nil || got != want {
			t.Errorf("ParsePreemptMode(%q) = %v, %v; want %v", text, got, err, want)
		}
	}

	if _, err := ParsePreemptMode("kernel"); err == nil {
		t.Error("ParsePreemptMode(kernel) succeeded")
	}

	if PreemptUser.String() != "user" {
		t.Errorf("PreemptUser.String() = %q", PreemptUser.String())
	}
}

// TestNewSystemProcessSettings: a new System starts with the defaults:
// no scheduler, VMS's preemption rule.
func TestNewSystemProcessSettings(t *testing.T) {
	env, _ := fixture()

	if got := env.ProcessSettings; got != DefaultProcessSettings() || got.Scheduler || got.Quantum != DefaultProcessQuantum {
		t.Errorf("ProcessSettings = %+v", got)
	}
}
