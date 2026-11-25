package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/dclcmd"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmserrors"
)

// ramDiskConsole is a console with a process, verbose, and the RAM disk
// keys unset for the test, with a dispatch that returns what a line
// shows, its error's message too.
func ramDiskConsole(t *testing.T) (*console.Console, func(string) string) {
	t.Helper()

	for _, key := range dclcmd.RAMDiskKeys() {
		old, had := settings.Get(key), settings.Exists(key)

		t.Cleanup(func() {
			if had {
				settings.Set(key, old)
			} else {
				_ = settings.Delete(key)
			}
		})

		_ = settings.Delete(key)
	}

	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	c.Verbose = true

	t.Cleanup(func() {
		_ = c.Mounts.DismountAll()
		_ = c.Mounts.ReleaseMemory()
	})

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	return c, func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r", ""))
	}
}

// TestDefineRAMDisk is DEFINE/DEVICE of MD devices (docs/planning/PHASE-96
// - RAM disk.md, Decision 5): the unit a controller alone gets, the size
// and its minimum, the limit on the total, the conflicting qualifiers, a
// mounted unit refused, a redefinition discarding a volume, /NOLOG, and the
// device's record as F$GETDVI and SHOW DEVICE see it.
func TestDefineRAMDisk(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c, dispatch := ramDiskConsole(t)

	steps := []struct{ line, want string }{
		{"DEFINE/DEVICE MDA /MAXBLOCK=20000", "%VAX-I-RAMDISK, _MDA0: defined, 20000 blocks"},
		{"DEFINE/DEVICE MDA", "%VAX-I-RAMDISK, _MDA1: defined, 65536 blocks"},
		{"DEFINE/DEVICE/NOLOG _MDA3: /MAXBLOCK=1000", ""},
		{"DEFINE/DEVICE MDA /MAXBLOCK=50", "%VAX-E-RAMSIZE, a RAM disk must have at least 100 blocks, not 50"},
		{"DEFINE/DEVICE MDA /MAXBLOCK=500000",
			"%VAX-E-RAMLIMIT, RAM disks would total 586536 blocks, over the limit of 512000\n" +
				"-VAX-I-RAMKEY, the limit is configuration key vax.ramdisk.limit"},
		{"DEFINE/DEVICE MDA /DEVCLASS=TAPE",
			"%DCL-W-CONFLICT, illegal combination of command elements - check documentation"},
		{"DEFINE/DEVICE MDA /DEVTYPE=RD54", "%DCL-W-CONFLICT, illegal combination of command elements - check documentation"},
		{"DEFINE/DEVICE MDA2 /DEVCLASS=DISK /DEVTYPE=RAM_DISK /MAXBLOCK=400",
			"%VAX-I-RAMDISK, _MDA2: defined, 400 blocks"},
		{`WRITE SYS$OUTPUT F$GETDVI("MDA1","DEVTYPE"), " ", F$GETDVI("MDA1","MAXBLOCK"), " ", F$GETDVI("MDA1","FOD")`,
			"58 65536 TRUE"},
		{"DEFINE/DEVICE MDA0 /MAXBLOCK=30000", "%VAX-I-RAMDISK, _MDA0: defined, 30000 blocks"},
	}

	for _, step := range steps {
		if got := dispatch(step.line); got != step.want {
			t.Errorf("%s:\n got %q\nwant %q", step.line, got, step.want)
		}
	}

	// The total is 30000+65536+1000+400; a limit set lower stops a new
	// disk, and one set higher lets it be made.
	settings.Set(dclcmd.RAMDiskLimitSetting, "100000")

	if got, want := dispatch("DEFINE/DEVICE MDB /MAXBLOCK=5000"),
		"%VAX-E-RAMLIMIT, RAM disks would total 101936 blocks, over the limit of 100000"; !strings.HasPrefix(got, want) {
		t.Errorf("over a lower limit:\n got %q\nwant %q...", got, want)
	}

	settings.Set(dclcmd.RAMDiskLimitSetting, "%X30D40") // 200000

	if got, want := dispatch("DEFINE/DEVICE MDB /MAXBLOCK=5000"), "%VAX-I-RAMDISK, _MDB0: defined, 5000 blocks"; got != want {
		t.Errorf("under a higher limit:\n got %q\nwant %q", got, want)
	}

	settings.Set(dclcmd.RAMDiskSizeSetting, "1234")

	if got, want := dispatch("DEFINE/DEVICE MDB"), "%VAX-I-RAMDISK, _MDB1: defined, 1234 blocks"; got != want {
		t.Errorf("vax.ramdisk.size:\n got %q\nwant %q", got, want)
	}

	// A mounted unit can't be defined again; once dismounted, defining it
	// again throws its volume away.
	if err := c.Mounts.MountMemory("MDA0", true, rms.MemoryVolume{Label: "SCRATCH"}); err != nil {
		t.Fatalf("MountMemory: %v", err)
	}

	if got, want := dispatch("DEFINE/DEVICE MDA0"), "%SYSTEM-F-DEVMOUNT, Device MDA0 already mounted"; got != want {
		t.Errorf("a mounted unit:\n got %q\nwant %q", got, want)
	}

	if err := c.Mounts.Dismount("MDA0"); err != nil {
		t.Fatalf("Dismount: %v", err)
	}

	if got, want := dispatch("DEFINE/DEVICE MDA0 /MAXBLOCK=25000"),
		"%VAX-I-RAMDISK, _MDA0: defined, 25000 blocks, previous contents discarded"; got != want {
		t.Errorf("a redefinition:\n got %q\nwant %q", got, want)
	}

	if c.Mounts.MemoryHasVolume("MDA0") {
		t.Errorf("MDA0 still has a volume after being defined again")
	}

	// One record per unit, a disk of type RAM_DISK.
	count := 0

	for _, d := range c.Devices.All() {
		if d.Name == "MDA0" {
			count++

			if d.DevClass != iodev.DeviceClassDisk || d.DevType != iodev.DeviceTypeRAMDisk || d.MaxBlock != 25000 {
				t.Errorf("MDA0's record = %+v", d)
			}
		}
	}

	if count != 1 {
		t.Errorf("MDA0 has %d records, want 1", count)
	}

	if got := dispatch("SHOW DEVICE MDA0/FULL"); !strings.Contains(got, "Disk MDA0:, device type RAM_DISK, is online") {
		t.Errorf("SHOW DEVICE/FULL:\n%s", got)
	}

	// Another device's DEFINE/DEVICE is as it was: silent, any class.
	if got := dispatch("DEFINE/DEVICE MTA0 /DEVCLASS=TAPE"); got != "" {
		t.Errorf("DEFINE/DEVICE MTA0 said %q", got)
	}
}

// TestDefineRAMDiskPrivilege: an MD device needs CMKRNL, other devices
// nothing (Decision 5).
func TestDefineRAMDiskPrivilege(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	c, dispatch := ramDiskConsole(t)

	priv94Account.become(c)

	nopriv := "%SYSTEM-F-NOPRIV, insufficient privilege or object protection violation"

	for _, step := range []struct{ line, want string }{
		{"DEFINE/DEVICE MDA /MAXBLOCK=1000", nopriv},
		{"DEFINE/DEVICE DKA7 /DEVCLASS=DISK", ""},
		{"SET PROCESS/PRIVILEGES=CMKRNL", ""},
		{"DEFINE/DEVICE MDA /MAXBLOCK=1000", "%VAX-I-RAMDISK, _MDA0: defined, 1000 blocks"},
	} {
		if got := dispatch(step.line); got != step.want {
			t.Errorf("%s:\n got %q\nwant %q", step.line, got, step.want)
		}
	}
}
