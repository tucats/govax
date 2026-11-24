package main

import (
	"fmt"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/rms"
)

// This is a list of all valid govax configuration keys. During startup, if a
// configuration key is found that isn't in this list, a warning is printed.
// It usually means the user misspelled something.
//
// ALWAYS ADD NEW CONFIG ITEMS TO THIS LIST!
var validConfigs = map[string]bool{
	"vax.debug.vm":         true,
	"vax.debug.tb":         true,
	"vax.debug.symbols":    true,
	"vax.debug.exceptions": true,
	"vax.debug.interrupts": true,
	"vax.debug.chm":        true,
	"vax.debug.registers":  true,
	"vax.debug.fulldisasm": true,
	"vax.debug.userhalt":   true,
	"vax.debug.keyboard":   true,
	"vax.debug.images":     true,
	"vax.debug.services":   true,
	"vax.debug.dcl":        true,
	"vax.debug.command":    true,
	"vax.debug.logicals":   true,
	"vax.debug.devices":    true,
	"vax.debug.process":    true,
	"vax.debug.rms":        true,
	"vax.debug.protection": true,
	"vax.debug.privileges": true,
	"vax.debug.userstep":   true,
	"vax.hardware.clock":   true,
	"vax.library":          true,
	"vax.link.library":     true,
	"vax.path":             true,
	"vax.quantum":          true,
	"vax.console.prompt":   true,
	"vax.console.lines":    true,
	"vax.language":         true,
	"vax.runtime.random":   true,
	"vax.editor.command":   true,
	"vax.editor.scratch":   true,

	// Host files need SYSPRV (docs/planning/PHASE-97 - host file
	// access.md): internal/machine reads it into the System.
	"vax.host.sysprv": true,

	// RAM disks (docs/planning/PHASE-96 - RAM disk.md): internal/dclcmd's
	// ramdisk.go reads them.
	"vax.ramdisk.size":  true,
	"vax.ramdisk.limit": true,

	"vax.default.system.file":   true,
	"vax.default.system.device": true,
	"vax.default.system.type":   true,

	// The authorization file (docs/planning/PHASE-95 - authentication.md).
	"vax.authorization.file": true,

	// The queue database (docs/planning/PHASE-98 - batch queues.md).
	"vax.queue.file": true,

	"vax.default.volume.file":      true,
	"vax.default.volume.label":     true,
	"vax.default.volume.device":    true,
	"vax.default.volume.type":      true,
	"vax.default.volume.directory": true,
	"vax.disassemble.symbolic":     true,
	"vax.startup.procedure":        true,

	"vax.process.scheduler": true,
	"vax.process.quantum":   true,
	"vax.process.preempt":   true,

	// The system parameters (docs/planning/PHASE-62 - VMS paging.md, Decision
	// 11): internal/machine's sysgen.go reads them.
	"vax.sysgen.paging.enable":  true,
	"vax.sysgen.paging.file":    true,
	"vax.sysgen.paging.size":    true,
	"vax.sysgen.paging.defer":   true,
	"vax.sysgen.minwscnt":       true,
	"vax.sysgen.tbskipwsl":      true,
	"vax.sysgen.growlim":        true,
	"vax.sysgen.mpw_hilimit":    true,
	"vax.sysgen.mpw_lolimit":    true,
	"vax.sysgen.mpw_wrtcluster": true,
	"vax.sysgen.freelim":        true,
	"vax.sysgen.freegoal":       true,
	"vax.sysgen.pql_dwsdefault": true,
	"vax.sysgen.pql_dwsquota":   true,
	"vax.sysgen.pql_dwsextent":  true,
	"vax.sysgen.pql_dpgflquota": true,
	"vax.sysgen.gblsections":    true,
	"vax.sysgen.gblpages":       true,
	"vax.sysgen.gblpagfil":      true,
	"vax.sysgen.pfcdefault":     true,
	"vax.sysgen.virtualpagecnt": true,
	"vax.sysgen.rms_fileprot":   true,
	"vax.sysgen.maxsysgroup":    true,

	// Read by STARTUP.COM (F$CONFIG): the memory's sizes, in pages.
	"vax.memory.size": true,
	"vax.memory.p0":   true,
	"vax.memory.p1":   true,
	"vax.memory.s0":   true,
	"vax.memory.ksp":  true,
	"vax.memory.esp":  true,
	"vax.memory.ssp":  true,
	"vax.memory.isp":  true,
}

func auditConfig() {
	count := 0
	keys := settings.Keys()

	for _, key := range keys {
		if !validConfigs[key] {
			if count == 0 {
				fmt.Printf("\nWarning, the following invalid or unrecognized config value(s) will be ignored:\n")
			}

			fmt.Printf("   %s\n", key)

			count++
		}
	}

	// A key whose value must be one of a few words is checked too: the
	// console would quietly use the default.
	if _, err := corevms.ParsePreemptMode(settings.Get("vax.process.preempt")); err != nil {
		fmt.Printf("\nWarning, %v; the default, all, will be used\n", err)
	}

	// So is a system parameter that must be a count of pages.
	for _, key := range machine.SysgenCountKeys() {
		if settings.Exists(key) {
			if _, ok := machine.ParseSysgenCount(settings.Get(key)); !ok {
				fmt.Printf("\nWarning, %s is %q; it must be a positive integer, and its default will be used\n",
					key, settings.Get(key))
			}
		}
	}

	for _, key := range machine.SysgenNumberKeys() {
		if settings.Exists(key) {
			if _, ok := machine.ParseSysgenNumber(settings.Get(key)); !ok {
				fmt.Printf("\nWarning, %s is %q; it must be a nonnegative integer, and its default will be used\n",
					key, settings.Get(key))
			}
		}
	}

	if key := machine.WriteClusterSetting; settings.Exists(key) {
		if _, ok := machine.ParseWriteCluster(settings.Get(key)); !ok {
			fmt.Printf("\nWarning, %s is %q; it must be a multiple of 8 from 16 to 120, and its default will be used\n",
				key, settings.Get(key))
		}
	}

	if key := machine.FileProtSetting; settings.Exists(key) {
		if _, ok := machine.ParseFileProt(settings.Get(key)); !ok {
			fmt.Printf("\nWarning, %s is %q; it must be a protection mask from 0 to %%XFFFF, and its default will be used\n",
				key, settings.Get(key))
		}
	}

	if key := machine.MaxSysGroupSetting; settings.Exists(key) {
		if _, ok := machine.ParseMaxSysGroup(settings.Get(key)); !ok {
			fmt.Printf("\nWarning, %s is %q; it must be a group number from 1 to %d, and its default will be used\n",
				key, settings.Get(key), machine.MaxMaxSysGroup)
		}
	}

	// So is a RAM disk key (docs/planning/PHASE-96 - RAM disk.md).
	for _, key := range dclcmd.RAMDiskKeys() {
		if settings.Exists(key) {
			if _, ok := dclcmd.ParseRAMDiskBlocks(settings.Get(key)); !ok {
				fmt.Printf("\nWarning, %s is %q; it must be a number of blocks, at least %d, and its default will be used\n",
					key, settings.Get(key), rms.MinimumMemoryBlocks)
			}
		}
	}

	if key := machine.PFCDefaultSetting; settings.Exists(key) {
		if _, ok := machine.ParsePFCDefault(settings.Get(key)); !ok {
			fmt.Printf("\nWarning, %s is %q; it must be a number from 0 to 127, and its default will be used\n",
				key, settings.Get(key))
		}
	}
}
