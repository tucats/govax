package console

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
)

// The LIB$CREATE_DIR probe (docs/PHASE-34.md, subtasks 13-14):
// testdata/credir/libcrd.mar runs under govax on a fresh exchange volume,
// and what it writes, and the directories it makes, are compared with VMS
// 7.3's run of the same probe (testdata/credir/README.md).

// libcrdVAXDisk is the container after LIBCRD.COM ran on VMS:
// testdata/credir/vax/libcrd-vax.dsk.gz, unless LIBCRD_VAX_DISK names
// another.
var libcrdVAXDisk = func() string {
	if p := os.Getenv("LIBCRD_VAX_DISK"); p != "" {
		return p
	}

	return filepath.Join("..", "..", "testdata", "credir", "vax", "libcrd-vax.dsk.gz")
}()

// libcrdCaseNames are the cases of a probe source, from its comments
// ("; 3: several levels"), by number.
func libcrdCaseNames(source string) map[uint32]string {
	names := map[uint32]string{}

	for _, line := range strings.Split(source, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "; ")
		if !ok {
			continue
		}

		num, text, ok := strings.Cut(rest, ": ")
		if !ok || num == "" || strings.Trim(num, "0123456789") != "" {
			continue
		}

		var n uint32
		if _, err := fmt.Sscan(num, &n); err == nil {
			names[n] = text
		}
	}

	return names
}

// libcrdMasked are fields of the directories the probe makes that differ
// between VMS's and govax's for reasons the documentation doesn't give, by
// path and field, each with its reason (also in docs/DEVIATIONS.md).
var libcrdMasked = map[string]map[string]string{
	"SUBREL": {
		// VMS 7.3 gave [SUBREL], made by LIB$CREATE_DIR("[.SUBREL]")
		// with [000000] the default, an MFD entry with a version limit
		// of 1, where every directory made from an absolute spec (and
		// every one CREATE/DIRECTORY made relative to a default) got
		// none. Neither manual says why; govax writes none.
		"entry-versions": "a VMS quirk the manuals don't explain",
	},
}

// libcrdStatuses reads LIBCRD.DMP from device: R0 by case number.
func libcrdStatuses(t *testing.T, c *Console, device string) map[uint32]uint32 {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: device + ":[000000]LIBCRD.DMP"}, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	out := map[uint32]uint32{}

	for _, r := range records {
		if len(r) != 12 || string(r[:4]) != "STAT" {
			t.Fatalf("LIBCRD.DMP record % x isn't STAT, case, R0", r)
		}

		out[binary.LittleEndian.Uint32(r[4:])] = binary.LittleEndian.Uint32(r[8:])
	}

	return out
}

// runLibcrdProbe builds a fresh exchange volume on DUA1, and assembles,
// links, and runs the probe there as LIBCRD.COM does on VMS.
func runLibcrdProbe(t *testing.T) *Console {
	t.Helper()

	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	disk := filepath.Join(t.TempDir(), "libcrd.dsk")
	if err := c.InitializeContainer(disk, 0, "LIBCRD", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mounts.Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	if err := c.SetDefault("DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	for name, value := range map[string]string{"CRDDEV": "DUA1:", "CRDLOG": "DUA1:[PLOG]"} {
		if _, err := c.Logicals.Define(lnm.ProcessTableName, name, lnm.Supervisor, 0, []lnm.Equivalence{{Value: value}}); err != nil {
			t.Fatal(err)
		}
	}

	src := filepath.Join("..", "..", "testdata", "credir", "libcrd.mar")
	dir := t.TempDir()

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, "libcrd.obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "libcrd")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if r0 := runImage(t, c, filepath.Join(dir, "libcrd.exe")); r0&1 != 1 {
		t.Fatalf("R0 = %#x, want a success status", r0)
	}

	return c
}

func TestLibCreateDirOracle(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "credir", "libcrd.mar"))
	if err != nil {
		t.Fatal(err)
	}

	names := libcrdCaseNames(string(source))

	c := runLibcrdProbe(t)
	govax := libcrdStatuses(t, c, "DUA1")

	if len(govax) != len(names) {
		t.Fatalf("govax's run wrote %d cases, the probe has %d", len(govax), len(names))
	}

	if _, err := os.Stat(libcrdVAXDisk); err != nil {
		for n := uint32(1); n <= uint32(len(names)); n++ {
			t.Logf("case %d, %s: govax %#x", n, names[n], govax[n])
		}

		t.Skipf("no VMS run of the LIB$CREATE_DIR probe (%s): see testdata/credir/README.md", libcrdVAXDisk)
	}

	vaxDisk := filepath.Join(t.TempDir(), "libcrd-vax.dsk")
	copyFile(t, libcrdVAXDisk, vaxDisk)

	if err := c.Mounts.Mount("DUA2", vaxDisk, false); err != nil {
		t.Fatal(err)
	}

	// VMS's run may be of an earlier version of the probe, with its cases
	// in another order: they're matched by name, from the probe source VMS
	// assembled, which is on its volume. A case VMS never reached (its
	// first run ended at an unhandled access violation) is reported, not
	// compared.
	vmsSource, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]LIBCRD.MAR"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	lines := make([]string, len(vmsSource))
	for i, r := range vmsSource {
		lines[i] = string(r)
	}

	vmsByName := map[string]uint32{}

	vmsStatuses := libcrdStatuses(t, c, "DUA2")
	for n, name := range libcrdCaseNames(strings.Join(lines, "\n")) {
		if st, ran := vmsStatuses[n]; ran {
			vmsByName[name] = st
		}
	}

	compared := 0

	for n := uint32(1); n <= uint32(len(names)); n++ {
		v, ran := vmsByName[names[n]]
		if !ran {
			t.Logf("case %d, %s: not run on VMS; govax %#x", n, names[n], govax[n])

			continue
		}

		compared++

		if v != govax[n] {
			t.Errorf("case %d, %s: VMS %#x, govax %#x", n, names[n], v, govax[n])
		}
	}

	t.Logf("compared %d of %d cases with VMS", compared, len(names))

	compareDirectories(t, credirDirectories(t, c, "DUA2"), credirDirectories(t, c, "DUA1"), libcrdMasked)
}
