package console

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 93's VMS round (docs/planning/PHASE-93 - file protection.md, subtask 14;
// testdata/vms93's README): VMS's DIRECTORY/SECURITY, SHOW SECURITY, and
// $CHECK_ACCESS on files govax gave owners, protections, and ACLs, and which user
// file opens VMS's RMS refuses. TestVMS93Procedure rehearses it on govax, with the
// round's accounts in the System's table as AUTHORIZE makes them on the VAX, and once
// VMS's log is in vax/ holds govax's run to it.

// vms93Differences are the sections' lines that differ from VMS's, by section,
// each with why; none yet, VMS not having run the round.
var vms93Differences = map[string]string{}

// runVMS93 makes the exchange volume as exchange.cmd does and runs VMS93.COM on it,
// the volume on DUA1, and returns the log.
func runVMS93(t *testing.T) string {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	c.RTL.NodeName = "SIMVAX"

	for _, id := range sec93Identifiers {
		if status := c.RTL.AddIdentifier(id.name, id.value, 0); status != 1 {
			t.Fatalf("AddIdentifier %s: %#x", id.name, status)
		}
	}

	for _, u := range sec93Users {
		// PROT93C has no privileges of its own in this round.
		if u.Name == "PROT93C" {
			u.Privileges = 0
		}

		if status := c.RTL.AddUser(u); status != 1 {
			t.Fatalf("AddUser %s: %#x", u.Name, status)
		}
	}

	path := filepath.Join(t.TempDir(), "vms93.dsk")
	if err := c.cmd().InitializeContainer(path, 0, "VMS93", 3, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA5:", path, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "vms93", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "INITIALIZE/") ||
			strings.HasPrefix(line, "MOUNT/") || strings.HasPrefix(line, "DISMOUNT") {
			continue
		}

		err := d.Dispatch(line)
		if out := buf.String(); err != nil || out != "" {
			t.Fatalf("%s returned %v and said:\n%.600s", line, err, out)
		}
	}

	// On the VAX it's DUA1.
	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, "@VMS93/OUTPUT=VMS93.LOG")
	buf.Reset()

	return ftn86Log(t, d, "VMS93.LOG")
}

// vms93Files are the files under [V93...], as VMS93C reaches them.
const vms93Files = 24

func TestVMS93Procedure(t *testing.T) {
	log := runVMS93(t)
	sections := ftn86Sections(log)

	for _, name := range []string{"ENVIRONMENT", "ACCOUNTS", "DIRECTORY", "SHOW", "BUILD", "CHECK", "CHECKG", "UFO", "DISK", "END"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("no section %s in the log:\n%s", name, log)
		}
	}

	// The access matrix: four lines a file, every answer granted or refused, and
	// govax's image's the same as the one built by the procedure.
	check := strings.Split(strings.TrimSpace(sections["CHECK"]), "\n")
	if len(check) != 4*vms93Files {
		t.Errorf("CHECK has %d lines, want %d:\n%s", len(check), 4*vms93Files, sections["CHECK"])
	}

	for _, l := range check {
		if strings.Contains(l, "?") {
			t.Errorf("CHECK: %s", l)
		}
	}

	if sections["CHECK"] != sections["CHECKG"] {
		t.Errorf("CHECKG differs from CHECK:\n%s\nCHECK:\n%s", sections["CHECKG"], sections["CHECK"])
	}

	if n := strings.Count(sections["UFO"], "fac="); n != 42 {
		t.Errorf("UFO has %d lines, want 42:\n%s", n, sections["UFO"])
	}

	for _, name := range []string{"BUILD"} {
		for _, l := range strings.Split(sections[name], "\n") {
			if strings.Contains(l, "$STATUS") && !strings.Contains(l, `"%X00000001"`) && !strings.Contains(l, `"%X10000001"`) {
				t.Errorf("%s: %s", name, l)
			}
		}
	}

	// VMS's answers, once the round has run.
	data, err := os.ReadFile(filepath.Join("testdata", "vms93", "vax", "vms93.log"))
	if os.IsNotExist(err) {
		t.Logf("no VMS log yet; govax's:\n%s", log)

		return
	} else if err != nil {
		t.Fatal(err)
	}

	vms := ftn86Sections(strings.ReplaceAll(string(data), "\r", ""))

	for _, name := range []string{"DIRECTORY", "SHOW", "CHECK", "CHECKG", "UFO"} {
		got, want := strings.Split(sections[name], "\n"), strings.Split(vms[name], "\n")

		for i := range max(len(got), len(want)) {
			var g, w string
			if i < len(got) {
				g = got[i]
			}

			if i < len(want) {
				w = want[i]
			}

			if g != w {
				if _, ok := vms93Differences[name+": "+w]; !ok {
					t.Errorf("%s line %d:\n got %q\nwant %q", name, i+1, g, w)
				}
			}
		}
	}
}
