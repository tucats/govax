package console

import (
	"os"
	"bytes"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
	"github.com/tucats/govax/internal/vmsimage"
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
// the volume on DUA1, and returns the log and govax's VMS93CG.EXE.
func runVMS93(t *testing.T) (string, []byte) {
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

	image := filepath.Join(t.TempDir(), "vms93cg.exe")
	shareFindRun(t, d, `COPY DUA1:[000000]VMS93CG.EXE "`+image+`"/HOST/BINARY/QUIET`)

	data, err := os.ReadFile(image)
	if err != nil {
		t.Fatal(err)
	}

	return ftn86Log(t, d, "VMS93.LOG"), data
}

// vms93Sections splits a log at its "==== NAME" lines, a name being the
// rest of the line ("ACCOUNTS REMOVED"), into each section's text by name.
func vms93Sections(log string) map[string]string {
	header := regexp.MustCompile(`(?m)^==== (.+)\n`)
	parts, names := header.Split(log, -1), header.FindAllStringSubmatch(log, -1)
	sections := map[string]string{}

	for i, name := range names {
		sections[name[1]] = parts[i+1]
	}

	return sections
}

// vms93Files are the files under [V93...], as VMS93C reaches them.
const vms93Files = 24

func TestVMS93Procedure(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	log, image := runVMS93(t)
	sections := vms93Sections(log)

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

	vms := vms93Sections(strings.ReplaceAll(string(data), "\r", ""))

	vms93SameImage(t, image)

	compared := []string{"DIRECTORY", "SHOW", "CHECK", "CHECKG", "UFO"}

	// The round's first run: VMS refused govax's image then (SS$_BADISD), as
	// LINK named SECURESHR with no pages. Until a run of the fixed image, its
	// matching VMS's own VMS93C stands for the section, and CHECKG is held to
	// CHECK above.
	if strings.Contains(vms["CHECKG"], "BADISD") {
		compared = slices.DeleteFunc(compared, func(name string) bool { return name == "CHECKG" })
	}

	for _, name := range compared {
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

// vms93SameImage holds govax's VMS93CG.EXE (govax's MACRO and LINK) to VMS's
// VMS93C.EXE, built by VMS 7.1's MACRO and LINK from the same source: the same
// image sections, global sections and their idents among them, the same
// fixups, and the same pages. Only the header's name, link time, and linker
// differ. The round's first run found govax's image naming SECURESHR with no
// pages, which VMS refused (SS$_BADISD), and its shareable images in the order
// of its code, not its symbols'.
func vms93SameImage(t *testing.T, govax []byte) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "vms93", "vax", "vms93c.exe"))
	if err != nil {
		t.Fatal(err)
	}

	want, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	got, err := vmsimage.ReadImage(govax)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got.ISDs, want.ISDs) {
		t.Errorf("VMS93CG's image sections:\n%+v\nVMS's VMS93C:\n%+v", got.ISDs, want.ISDs)
	}

	if !reflect.DeepEqual(got.Fixups, want.Fixups) {
		t.Errorf("VMS93CG's fixups:\n%+v\nVMS's VMS93C:\n%+v", got.Fixups, want.Fixups)
	}

	const block = 512
	if len(govax) != len(data) || !bytes.Equal(govax[block:], data[block:]) {
		t.Errorf("VMS93CG's pages (%d bytes) differ from VMS's VMS93C's (%d)", len(govax), len(data))
	}
}
