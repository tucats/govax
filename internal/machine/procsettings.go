package machine

import (
	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/corevms"
)

// The multiprocessing settings' keys (docs/PHASE-43.md; HELP CONFIG
// KEYS). Each must also be in cmd/govax/main.go's validConfigs.
const (
	SchedulerSetting = "vax.process.scheduler"
	QuantumSetting   = "vax.process.quantum"
	preemptSetting   = "vax.process.preempt"
)

// processSettings reads the vax.process.* settings, each key that isn't
// set (or, for the quantum, isn't a positive number, or, for preempt,
// isn't one of its values) keeping its default. govax checks the values
// at startup and warns about a bad one (cmd/govax's auditConfig), so a
// mistake isn't silent.
func processSettings() corevms.ProcessSettings {
	ps := corevms.DefaultProcessSettings()

	// The scheduler is on (the default) unless the setting says
	// otherwise (Phase 48, Decision 5 in docs/PHASE-43.md).
	if settings.Exists(SchedulerSetting) {
		ps.Scheduler = settings.GetBool(SchedulerSetting)
	}

	if q := settings.GetInt(QuantumSetting); q > 0 {
		ps.Quantum = q
	}

	if m, err := corevms.ParsePreemptMode(settings.Get(preemptSetting)); err == nil {
		ps.Preempt = m
	}

	return ps
}
