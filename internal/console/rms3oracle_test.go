package console

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
)

// The Phase 33 oracle's reconciliation (docs/PHASE-33.md, subtask 6):
// each probe runs under govax on a copy of the container the author's VMS
// 7.3 system ran it on (testdata/mar/rms3/README.md), and what it writes
// is compared with what it wrote there. The container is local-only, so
// the test skips without it.

// rms3VAXDisk is the container, after BUILD.COM and RUN.COM on VMS. The
// environment variable RMS3_VAX_DISK names another.
var rms3VAXDisk = func() string {
	if p := os.Getenv("RMS3_VAX_DISK"); p != "" {
		return p
	}

	return filepath.Join("..", "..", "testdata", "disks", "rms3-vax.dsk")
}()

// rms3Probes are the probes, in RUN.COM's order.
var rms3Probes = []string{"parse", "search", "open", "xab", "namfid", "create"}

// copyFile copies a host file.
func copyFile(t *testing.T, from, to string) {
	t.Helper()

	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}

	defer in.Close()

	out, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}

	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// vaxDevice is the device VMS ran the probes on, from the expanded string
// of PARSE's first case: its name, without "_" or ":".
func vaxDevice(t *testing.T, recs []probeRecord) string {
	t.Helper()

	for _, r := range recs {
		if r.Tag == "ESA_" && r.Step == 1 {
			dev, _, ok := bytes.Cut(r.Data, []byte(":"))
			if !ok {
				t.Fatalf("PARSE case 1's expanded string %q has no device", r.Data)
			}

			return strings.TrimPrefix(string(dev), "_")
		}
	}

	t.Fatal("PARSE has no case 1")

	return ""
}

// readDump reads a probe's highest-version dump from the mounted volume.
func readDump(t *testing.T, c *Console, device, probe string) []probeRecord {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: device + ":[OUT]" + strings.ToUpper(probe) + ".DMP"}, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	return decodeProbe(t, records)
}

// runOracleProbe runs probe under govax on a copy of the VAX container
// mounted as device, as RUN.COM ran it, and returns VMS's records and
// govax's.
func runOracleProbe(t *testing.T, device, probe string) (vms, govax []probeRecord) {
	t.Helper()

	disk := filepath.Join(t.TempDir(), "rms3.dsk")
	copyFile(t, rms3VAXDisk, disk)

	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	if err := c.Mounts.Mount(device, disk, true); err != nil {
		t.Fatal(err)
	}

	vms = readDump(t, c, device, probe)

	// RUN.COM empties [CRE] before the probes run.
	if _, err := c.ContainerSession.Delete(device + ":[CRE]*.*;*"); err != nil && !strings.Contains(err.Error(), "no files") {
		t.Logf("emptying [CRE]: %v", err)
	}

	if err := c.SetDefault(device + ":[000000]"); err != nil {
		t.Fatal(err)
	}

	for n, eqv := range map[string][]string{
		"TST": {device + ":[TEST]"},
		"TSL": {device + ":[TEST.SUB]", device + ":[TEST]"},
	} {
		var e []lnm.Equivalence
		for _, v := range eqv {
			e = append(e, lnm.Equivalence{Value: v})
		}

		if _, err := c.Logicals.Define(lnm.ProcessTableName, n, lnm.Supervisor, 0, e); err != nil {
			t.Fatal(err)
		}
	}

	src := filepath.Join("..", "..", "testdata", "mar", "rms3", probe+".mar")
	dir := t.TempDir()

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, probe+".obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, probe)}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if r0 := runImage(t, c, filepath.Join(dir, probe+".exe")); r0&1 != 1 {
		t.Fatalf("R0 = %#x, want a success status", r0)
	}

	return vms, readDump(t, c, device, probe)
}

// diffRecords lists where govax's records differ from VMS's.
func diffRecords(vms, govax []probeRecord) []string {
	var out []string

	if len(vms) != len(govax) {
		out = append(out, fmt.Sprintf("%d records, VMS wrote %d", len(govax), len(vms)))
	}

	for i := range min(len(vms), len(govax)) {
		v, g := vms[i], govax[i]

		if v.Tag != g.Tag || v.Step != g.Step || v.Op != g.Op {
			out = append(out, fmt.Sprintf("record %d: %s %d op%d, VMS wrote %s %d op%d", i, g.Tag, g.Step, g.Op, v.Tag, v.Step, v.Op))

			break
		}

		if !bytes.Equal(v.Data, g.Data) {
			var at []string

			for j := range min(len(v.Data), len(g.Data)) {
				if v.Data[j] != g.Data[j] {
					at = append(at, fmt.Sprintf("%#x: %02X/%02X", j, g.Data[j], v.Data[j]))
				}
			}

			out = append(out, fmt.Sprintf("%s %d op%d (govax/VMS) %s", v.Tag, v.Step, v.Op, strings.Join(at, " ")))
		}
	}

	return out
}

// TestRMS3Oracle runs each probe under govax against the VAX run's
// container and compares what it wrote with what VMS wrote.
func TestRMS3Oracle(t *testing.T) {
	if _, err := os.Stat(rms3VAXDisk); err != nil {
		t.Skip("no VAX run's container (testdata/disks/rms3-vax.dsk)")
	}

	// The device VMS ran on, from VMS's own PARSE dump.
	c := newBootableConsole(t)
	disk := filepath.Join(t.TempDir(), "rms3.dsk")
	copyFile(t, rms3VAXDisk, disk)

	if err := c.Mounts.Mount("DUA9", disk, false); err != nil {
		t.Fatal(err)
	}

	device := vaxDevice(t, readDump(t, c, "DUA9", "parse"))

	for _, probe := range rms3Probes {
		t.Run(probe, func(t *testing.T) {
			vms, govax := runOracleProbe(t, device, probe)

			for _, d := range diffRecords(vms, govax) {
				t.Error(d)
			}
		})
	}
}
