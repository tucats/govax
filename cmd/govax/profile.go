package main

import (
	"fmt"
	"os"
	"runtime/pprof"
)

// profileFile is the open --cpu-profile file while Go's CPU profiler is
// running, or nil when it isn't.
var profileFile *os.File

// startCPUProfile starts Go's sampling CPU profiler (runtime/pprof),
// writing to the named host file. The profile covers everything govax does
// from here on: startup, vax.init, and the emulated program. It's for
// performance studies (docs/PERFORMANCE.md):
//
//	govax --cpu-profile cpu.prof run pi 10000
//	go tool pprof -top govax cpu.prof
func startCPUProfile(name string) error {
	if profileFile != nil {
		return nil
	}

	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("--cpu-profile: %w", err)
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()

		return fmt.Errorf("--cpu-profile: %w", err)
	}

	profileFile = f

	return nil
}

// stopCPUProfile stops the profiler, if --cpu-profile started it, and
// closes its file so the profile is complete on disk.
func stopCPUProfile() {
	if profileFile == nil {
		return
	}

	pprof.StopCPUProfile()
	profileFile.Close()
	profileFile = nil
}
