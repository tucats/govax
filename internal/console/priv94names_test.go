package console_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// priv94Holders makes the probe's fixtures and holder processes as
// PRIV94.COM does, by running part FH of PRIV94 as SYSTEM: the tables
// PRIV94_OT and PRIV94_PT with a name in each, and PRIV94_G in group 200 and
// PRIV94_W in group 300. It defines GPID and WPID as the procedure does, and
// returns VMS's holders' PIDs in file (its "holders:" line) mapped to
// govax's.
func priv94Holders(t *testing.T, file string) func(*console.Console, func(string) string) map[string]string {
	return func(_ *console.Console, dispatch func(string) string) map[string]string {
		t.Helper()

		buildPriv94(t, dispatch, []string{"PRIV94", "PRIV94H"})
		dispatch(`DEFINE PRIV94_PART "FH"`)
		dispatch("RUN PRIV94")

		var ours []string

		for _, name := range []string{"PRIV94_G_PID", "PRIV94_W_PID"} {
			pid := strings.TrimSpace(dispatch(`WRITE SYS$OUTPUT F$TRNLNM("` + name + `","LNM$SYSTEM_TABLE")`))
			if len(pid) != 8 {
				t.Fatalf("%s is %q: the holders weren't made", name, pid)
			}

			ours = append(ours, pid)
		}

		dispatch(`GPID = "` + ours[0] + `"`)
		dispatch(`WPID = "` + ours[1] + `"`)

		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "priv94", "vax", file))
		if err != nil {
			t.Fatal(err)
		}

		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r", ""), "\n") {
			if f := strings.Fields(line); len(f) == 3 && f[0] == "holders:" {
				return map[string]string{f[1]: ours[0], f[2]: ours[1]}
			}
		}

		t.Fatalf("%s has no holders line", file)

		return nil
	}
}

// TestPriv94Names replays the probe's NAMES and STOP sections (the phase
// doc's subtask 7) as SYSTEM, PRIV94, and PRIV94N: DEFINE, ASSIGN, DEASSIGN,
// SHOW LOGICAL, F$TRNLNM, and CREATE/NAME_TABLE in the system, group,
// process, and fixture tables and modes, under no privilege, SYSNAM, GRPNAM,
// and SYSPRV; and STOP of the holders with none.
func TestPriv94Names(t *testing.T) {
	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94System, "priv94.log"},
		{priv94Account, "priv94u_priv94.log"},
		{priv94Plain, "priv94u_priv94n.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			sections := append(priv94DCLSections(t, run.file, "NAMES"), priv94DCLSections(t, run.file, "STOP")...)

			// F$GETJPI of a holder is subtask 8's (seeing other processes).
			for i := range sections {
				sections[i].commands = slices.DeleteFunc(sections[i].commands, func(c priv94DCLCommand) bool {
					return strings.Contains(c.line, "F$GETJPI(GPID")
				})
			}

			replayPriv94DCLWith(t, run.id, sections, nil, priv94Holders(t, run.file))
		})
	}
}

// TestPriv94Tables replays round 2's TABLES sections (PRIV94E.COM) as SYSTEM
// and PRIV94: DEFINE, DEASSIGN, SHOW LOGICAL, F$TRNLNM, and SHOW SECURITY on
// two supervisor-mode tables SYSTEM made with CREATE/NAME_TABLE/PROTECTION,
// one open to everyone and one closed to the group and the world, under no
// privilege, SYSPRV, SYSNAM, GRPNAM, and READALL.
func TestPriv94Tables(t *testing.T) {
	fixtures := func(_ *console.Console, dispatch func(string) string) map[string]string {
		for _, line := range []string{
			"CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) PRIV94_DO",
			"CREATE/NAME_TABLE/PARENT_TABLE=LNM$SYSTEM_DIRECTORY/PROTECTION=(S:RWCD,O:RWCD,G,W) PRIV94_DP",
			`DEFINE/TABLE=PRIV94_DP PRIV94_DPN "P"`,
		} {
			if out := dispatch(line); out != "" {
				t.Fatalf("%s: %s", line, out)
			}
		}

		return nil
	}

	for _, run := range []struct {
		id   priv94Identity
		file string
	}{
		{priv94System, "round2/priv94r.log"},
		{priv94Account, "round2/priv94v.log"},
	} {
		t.Run(run.id.name, func(t *testing.T) {
			replayPriv94DCLWith(t, run.id, priv94DCLSections(t, run.file, "TABLES"), nil, fixtures)
		})
	}
}

// TestVolumeCommandPrivileges: govax's MOUNT and DISMOUNT need SYSNAM, and
// INITIALIZE/CONTAINER of a container that exists VOLPRO, a new one nothing
// (the phase doc's Decision 10), for PRIV94, who holds neither until it
// enables them.
func TestVolumeCommandPrivileges(t *testing.T) {
	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))
	dispatch := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return strings.TrimSpace(strings.ReplaceAll(buf.String(), "\r", ""))
	}

	priv94Account.become(c)

	dsk := filepath.Join(t.TempDir(), "vol94.dsk")
	nopriv := "%SYSTEM-F-NOPRIV, insufficient privilege or object protection violation"

	for _, step := range []struct{ line, want string }{
		{`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 VOL94`, ""},
		{`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 VOL94`, nopriv},
		{"SET PROCESS/PRIVILEGES=VOLPRO", ""},
		{`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 VOL94`, ""},
		{`MOUNT/NOLOG DUA3 "` + dsk + `"`, nopriv},
		{"SET PROCESS/PRIVILEGES=SYSNAM", ""},
		{`MOUNT/NOLOG DUA3 "` + dsk + `"`, ""},
		{"SET PROCESS/PRIVILEGES=NOSYSNAM", ""},
		{"DISMOUNT DUA3", nopriv},
		{"SET PROCESS/PRIVILEGES=SYSNAM", ""},
		{"DISMOUNT DUA3", ""},
	} {
		if got := dispatch(step.line); got != step.want {
			t.Errorf("%s:\n got %q\nwant %q", step.line, got, step.want)
		}
	}
}

// TestPrivilegeTraceOfTables: SET DEBUG PRIVILEGES shows a logical name
// table's protection deciding before any privilege is asked: SYSTEM, with no
// privileges, writes the system table by its protection (the system
// category), and PRIV94 is refused by it and then asked for SYSNAM.
func TestPrivilegeTraceOfTables(t *testing.T) {
	c, buf := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	var trace strings.Builder

	c.CPU.SetDebugWriter(&trace)
	c.CPU.SetDebug(registers.DebugPrivileges)

	run := func(id priv94Identity, line string) string {
		t.Helper()
		id.become(c)
		trace.Reset()
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			buf.WriteString("%" + err.Error() + "\n")
		}

		return trace.String() + buf.String()
	}

	systemDown := priv94System
	systemDown.holds = priv94Mask("TMPMBX", "NETMBX")

	got := run(systemDown, `DEFINE/SYSTEM PRIV94_TR "X"`)
	if !strings.Contains(got, "logical name: write LNM$SYSTEM_TABLE: by its protection [1,4] (S:RWC,O:RWC,G:R,W:R), "+
		"granted, has RWC (system,owner,group,world,ownership); no privilege asked") {
		t.Errorf("SYSTEM's DEFINE/SYSTEM:\n%s", got)
	}

	got = run(priv94Account, `DEFINE/SYSTEM PRIV94_TS "X"`)
	for _, want := range []string{
		"logical name: write LNM$SYSTEM_TABLE: by its protection [1,4] (S:RWC,O:RWC,G:R,W:R), denied, has R (world); asking the privilege",
		"logical name: write the system table past its protection (needs SYSNAM): refused, SS$_NOPRIV",
		"%SYSTEM-F-NOPRIV",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("PRIV94's DEFINE/SYSTEM: no %q in:\n%s", want, got)
		}
	}
}
