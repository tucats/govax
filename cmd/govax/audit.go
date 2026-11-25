package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/rms"
)

// configDomain is the configuration keys' reserved domain: every key whose
// name starts with it must be one of configKeys, with a value that suits
// it. gopackages' "govax config set" refuses any other (settings.Reserve),
// and auditConfig reports any the profile holds another way.
const configDomain = "vax."

// configKeys are every configuration key govax reads, each with the form
// its value must take: a kind, and for many a validator that parses the
// value as the code that reads it does, so "config set" refuses what that
// code would quietly ignore. A key read as text (a path, a name, a
// command) is a String, which anything suits.
//
// ALWAYS ADD NEW CONFIG ITEMS TO THIS LIST, and describe each under HELP
// CONFIGURATION KEYS (TestConfigKeysHaveHelp).
var configKeys = func() []settings.Key {
	keys := []settings.Key{
		// The debug switches, read with strconv.ParseBool (internal/cpu's
		// debugConfig), which takes neither "yes" nor "no".
		debugKey("vm"), debugKey("tb"), debugKey("symbols"), debugKey("exceptions"),
		debugKey("interrupts"), debugKey("chm"), debugKey("registers"), debugKey("fulldisasm"),
		debugKey("userhalt"), debugKey("keyboard"), debugKey("images"), debugKey("services"),
		debugKey("dcl"), debugKey("command"), debugKey("logicals"), debugKey("devices"),
		debugKey("process"), debugKey("rms"), debugKey("protection"), debugKey("privileges"),
		debugKey("userstep"),

		{Name: "vax.hardware.clock", Kind: settings.Bool},
		{Name: "vax.library"},
		{Name: "vax.link.library"},
		{Name: "vax.path"},
		{Name: "vax.quantum", Kind: settings.Int, Validate: positive},
		{Name: "vax.console.prompt"},
		{Name: "vax.console.lines", Kind: settings.Int},
		{Name: "vax.language"},
		{Name: "vax.runtime.random", Kind: settings.Bool},
		{Name: "vax.editor.command"},
		{Name: "vax.editor.scratch"},

		// Host files need SYSPRV (docs/planning/PHASE-97 - host file
		// access.md): internal/machine reads it into the System.
		{Name: "vax.host.sysprv", Kind: settings.Bool},

		{Name: "vax.default.system.file"},
		{Name: "vax.default.system.device"},
		{Name: "vax.default.system.type"},

		// The authorization file (docs/planning/PHASE-95 - authentication.md).
		{Name: "vax.authorization.file"},

		// The queue database (docs/planning/PHASE-98 - batch queues.md).
		{Name: "vax.queue.file"},

		{Name: "vax.default.volume.file"},
		{Name: "vax.default.volume.label"},
		{Name: "vax.default.volume.device"},
		{Name: "vax.default.volume.type"},
		{Name: "vax.default.volume.directory"},
		{Name: "vax.disassemble.symbolic", Kind: settings.Bool},
		{Name: "vax.startup.procedure"},

		{Name: "vax.process.scheduler", Kind: settings.Bool},
		{Name: "vax.process.quantum", Kind: settings.Int, Validate: positive},
		// corevms.ParsePreemptMode's modes.
		{Name: "vax.process.preempt", Kind: settings.Enum, Values: []string{"all", "user", "none"}},

		// The system parameters (docs/planning/PHASE-62 - VMS paging.md,
		// Decision 11) that aren't numbers: internal/machine's sysgen.go
		// reads them. The numbers follow, from its lists.
		{Name: "vax.sysgen.paging.enable", Kind: settings.Bool},
		{Name: "vax.sysgen.paging.file"},
		{Name: "vax.sysgen.paging.defer", Kind: settings.Bool},
		{Name: machine.WriteClusterSetting, Validate: parsed(machine.ParseWriteCluster,
			"must be a multiple of 8 from 16 to 120")},
		{Name: machine.FileProtSetting, Validate: parsed(machine.ParseFileProt,
			"must be a protection mask from 0 to %XFFFF")},
		{Name: machine.MaxSysGroupSetting, Validate: parsed(machine.ParseMaxSysGroup,
			fmt.Sprintf("must be a group number from 1 to %d", machine.MaxMaxSysGroup))},
		{Name: machine.PFCDefaultSetting, Validate: parsed(machine.ParsePFCDefault,
			"must be a number from 0 to 127")},

		// Read by STARTUP.COM (F$CONFIG): the memory's sizes, in pages,
		// which INITIALIZE/MEMORY checks.
		{Name: "vax.memory.size"},
		{Name: "vax.memory.p0"},
		{Name: "vax.memory.p1"},
		{Name: "vax.memory.s0"},
		{Name: "vax.memory.ksp"},
		{Name: "vax.memory.esp"},
		{Name: "vax.memory.ssp"},
		{Name: "vax.memory.isp"},
	}

	for _, key := range machine.SysgenCountKeys() {
		keys = append(keys, settings.Key{Name: key, Validate: parsed(machine.ParseSysgenCount,
			"must be a positive integer (decimal, or %X and hexadecimal)")})
	}

	for _, key := range machine.SysgenNumberKeys() {
		keys = append(keys, settings.Key{Name: key, Validate: parsed(machine.ParseSysgenNumber,
			"must be a nonnegative integer (decimal, or %X and hexadecimal)")})
	}

	// RAM disks (docs/planning/PHASE-96 - RAM disk.md): internal/dclcmd's
	// ramdisk.go reads them.
	for _, key := range dclcmd.RAMDiskKeys() {
		keys = append(keys, settings.Key{Name: key, Validate: parsed(dclcmd.ParseRAMDiskBlocks,
			fmt.Sprintf("must be a number of blocks, at least %d", rms.MinimumMemoryBlocks))})
	}

	return keys
}()

// debugKey is the switch vax.debug.name.
func debugKey(name string) settings.Key {
	return settings.Key{Name: "vax.debug." + name, Validate: func(value string) error {
		if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
			return errors.New("must be true or false (or 1 or 0, t or f)")
		}

		return nil
	}}
}

// positive is an Int key's check that its value is above zero: the code
// that reads it ignores anything else.
func positive(value string) error {
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err != nil || n <= 0 {
		return errors.New("must be a positive integer")
	}

	return nil
}

// parsed is a validator from one of govax's parsers, which reports
// whether the value is one it takes; why says what the value must be.
func parsed[T any](parse func(string) (T, bool), why string) func(string) error {
	return func(value string) error {
		if _, ok := parse(value); !ok {
			return errors.New(why)
		}

		return nil
	}
}

// reserveConfigKeys reserves configDomain for configKeys, before
// gopackages parses the command line (so "govax config set" checks them).
func reserveConfigKeys() {
	settings.Reserve(configDomain, configKeys...)
}

// auditConfig warns about every key in the profile that configDomain
// refuses: one govax doesn't read (usually a misspelling), or one whose
// value the code that reads it ignores, using the default instead. "config
// set" refuses both; such a key got into the profile another way, as by
// editing its file.
func auditConfig(w io.Writer) {
	var unknown, invalid []settings.Problem

	for _, p := range settings.Audit() {
		if !strings.HasPrefix(strings.ToLower(p.Key), configDomain) {
			continue
		}

		if p.Unknown {
			unknown = append(unknown, p)
		} else {
			invalid = append(invalid, p)
		}
	}

	if len(unknown) > 0 {
		fmt.Fprintf(w, "\nWarning, the following invalid or unrecognized config value(s) will be ignored:\n")

		for _, p := range unknown {
			fmt.Fprintf(w, "   %s\n", p.Key)
		}
	}

	for _, p := range invalid {
		fmt.Fprintf(w, "\nWarning, %s is %q; it %s, and its default will be used\n", p.Key, p.Value, reason(p.Err))
	}
}

// reason is what a refused value's error says it must be: the text after
// the key and value (settings' "KEY="VALUE": must be ...").
func reason(err error) string {
	text := err.Error()
	if i := strings.LastIndex(text, "\": "); i >= 0 {
		return text[i+3:]
	}

	return text
}
