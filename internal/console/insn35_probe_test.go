package console

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/rms"
)

// The Phase 35 instruction probes (docs/PHASE-35.md, subtask 3):
// testdata/insn35/*.mar, written by testdata/insn35/gen.go. Each runs a
// table of instruction cases and writes one record per case.
// TestInsn35ProbesRun checks govax's MACRO and LINK build each probe and
// that it runs to the end; TestInsn35Oracle compares every record with
// VMS's run (subtask 14).

// insn35Probes are the probe programs, one per family of instructions.
var insn35Probes = []string{"P35FD", "P35G", "P35H", "P35O", "P35P"}

// insn35RecordSize is a probe record's length (gen.go's comment gives the
// layout).
const insn35RecordSize = 260

// insn35Record is one case's record.
type insn35Record struct {
	Case   uint32
	Name   string
	Flags  uint32 // 1 completed, 2 branch taken, 4 signalled, 8 signalled twice
	PSL    uint32
	Regs   [12]uint32 // R0-R11
	Signal []uint32   // the signal array's longwords, after its count
	DST    []byte
}

// parseInsn35Record decodes one record.
func parseInsn35Record(b []byte) (insn35Record, bool) {
	if len(b) != insn35RecordSize || string(b[:4]) != "I35R" {
		return insn35Record{}, false
	}

	le := binary.LittleEndian
	r := insn35Record{
		Case:  le.Uint32(b[4:]),
		Name:  strings.TrimRight(string(b[8:40]), " "),
		Flags: le.Uint32(b[40:]),
		PSL:   le.Uint32(b[44:]),
		DST:   b[132:260],
	}

	for i := range r.Regs {
		r.Regs[i] = le.Uint32(b[48+4*i:])
	}

	count := min(le.Uint32(b[96:]), 8)
	for i := uint32(0); i < count; i++ {
		r.Signal = append(r.Signal, le.Uint32(b[100+4*i:]))
	}

	return r, true
}

// insn35CaseNames returns a probe source's case names by number, from its
// name strings ("N3:	.ASCII	|MOVF max ...|").
func insn35CaseNames(source string) map[uint32]string {
	re := regexp.MustCompile(`(?m)^N(\d+):\t\.ASCII\t\|([^|]*)\|`)
	names := map[uint32]string{}

	for _, m := range re.FindAllStringSubmatch(source, -1) {
		n, _ := strconv.ParseUint(m[1], 10, 32)
		names[uint32(n)] = strings.TrimRight(m[2], " ")
	}

	return names
}

// runInsn35Probe builds and runs one probe under govax, with a fresh
// volume as the default directory, and returns its records.
func runInsn35Probe(t *testing.T, name string) []insn35Record {
	t.Helper()

	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	disk := filepath.Join(t.TempDir(), "insn35.dsk")
	if err := c.InitializeContainer(disk, 0, "INSN35", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mounts.Mount("DUA1", disk, true); err != nil {
		t.Fatal(err)
	}

	if err := c.SetDefault("DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	lower := strings.ToLower(name)
	src := filepath.Join("..", "..", "testdata", "insn35", lower+".mar")
	dir := t.TempDir()

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, lower+".obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, lower)}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if r0 := runImage(t, c, filepath.Join(dir, lower+".exe")); r0&1 != 1 {
		t.Fatalf("R0 = %#x, want a success status", r0)
	}

	raw, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name + ".DMP"}, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	records := make([]insn35Record, 0, len(raw))

	for i, b := range raw {
		r, ok := parseInsn35Record(b)
		if !ok {
			t.Fatalf("record %d isn't a probe record (%d bytes)", i+1, len(b))
		}

		records = append(records, r)
	}

	return records
}

// TestInsn35ProbesRun builds and runs each probe and checks it wrote one
// record per case, in order, each with its case's name.
func TestInsn35ProbesRun(t *testing.T) {
	for _, name := range insn35Probes {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "insn35", strings.ToLower(name)+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			names := insn35CaseNames(string(source))
			records := runInsn35Probe(t, name)

			if len(records) != len(names) || len(names) == 0 {
				t.Fatalf("%d records, the probe has %d cases", len(records), len(names))
			}

			for i, r := range records {
				if want := uint32(i + 1); r.Case != want || r.Name != names[want] {
					t.Errorf("record %d is case %d %q, want case %d %q", i+1, r.Case, r.Name, want, names[want])
				}
			}
		})
	}
}

// insn35VAXDisk is the exchange volume after INSN35.COM ran on VMS:
// testdata/insn35/vax/insn35-vax.dsk.gz, unless INSN35_VAX_DISK names
// another.
var insn35VAXDisk = func() string {
	if p := os.Getenv("INSN35_VAX_DISK"); p != "" {
		return p
	}

	return filepath.Join("..", "..", "testdata", "insn35", "vax", "insn35-vax.dsk.gz")
}()

// vaxInsn35Records reads each probe's records from VMS's run, by probe
// name, and the probe sources VMS assembled. It skips the test if there's
// no VMS run.
func vaxInsn35Records(t *testing.T) (map[string][]insn35Record, map[string]string) {
	t.Helper()

	if _, err := os.Stat(insn35VAXDisk); err != nil {
		t.Skipf("no VMS run of the Phase 35 probes (%s): see testdata/insn35/README.md", insn35VAXDisk)
	}

	disk := filepath.Join(t.TempDir(), "insn35-vax.dsk")
	copyFile(t, insn35VAXDisk, disk)

	c := newBootableConsole(t)
	if err := c.Mounts.Mount("DUA2", disk, false); err != nil {
		t.Fatal(err)
	}

	records := map[string][]insn35Record{}
	sources := map[string]string{}

	for _, name := range insn35Probes {
		raw, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]" + name + ".DMP"}, rms.VariableRecords)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		for i, b := range raw {
			r, ok := parseInsn35Record(b)
			if !ok {
				t.Fatalf("%s: record %d isn't a probe record (%d bytes)", name, i+1, len(b))
			}

			records[name] = append(records[name], r)
		}

		lines, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]" + name + ".MAR"}, rms.TextRecords)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		text := make([]string, len(lines))
		for i, l := range lines {
			text[i] = string(l)
		}

		sources[name] = strings.Join(text, "\n")
	}

	return records, sources
}

// TestInsn35VAXRun checks VMS's run of the probes is complete: a record
// for every case of the probe VMS assembled, in order, and that the
// probe VMS assembled is the one in the repository, so subtask 14's
// comparison compares like with like.
func TestInsn35VAXRun(t *testing.T) {
	records, sources := vaxInsn35Records(t)

	for _, name := range insn35Probes {
		names := insn35CaseNames(sources[name])

		if len(records[name]) != len(names) || len(names) == 0 {
			t.Errorf("%s: VMS wrote %d records, its probe has %d cases", name, len(records[name]), len(names))

			continue
		}

		for i, r := range records[name] {
			if want := uint32(i + 1); r.Case != want || r.Name != names[want] {
				t.Errorf("%s: record %d is case %d %q, want case %d %q", name, i+1, r.Case, r.Name, want, names[want])
			}
		}

		ours, err := os.ReadFile(filepath.Join("..", "..", "testdata", "insn35", strings.ToLower(name)+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		if strings.TrimRight(string(ours), "\n") != strings.TrimRight(sources[name], "\n") {
			t.Errorf("%s: the probe VMS ran differs from testdata/insn35/%s.mar: run gen.go's output on VMS again", name, strings.ToLower(name))
		}
	}
}

// insn35Mask is a field of one case's record that differs between VMS's
// run and govax's for a reason the architecture allows, with the reason
// (each also in docs/DEVIATIONS.md).
type insn35Mask struct {
	dst       bool   // the destination bytes
	signalFPD bool   // PSL<FPD> in the signal array's saved PSL
	reason    string // why
}

// insn35Masks are the masked fields, by probe and case name.
var insn35Masks = map[string]map[string]insn35Mask{
	"P35P": {
		"DIVP bad divisor digit": {dst: true,
			reason: "the manual makes the quotient UNPREDICTABLE for an invalid digit; VMS divided by 8 where govax uses the nibble's value, 14"},
		"EDITPC digits left over": {signalFPD: true,
			reason: "govax doesn't model PSL<FPD>, which VMS sets on an EDITPC abort (the manual's note 11)"},
		"EDITPC digits run out": {signalFPD: true,
			reason: "govax doesn't model PSL<FPD>, which VMS sets on an EDITPC abort (the manual's note 11)"},
		"EDITPC reserved operator": {signalFPD: true,
			reason: "govax doesn't model PSL<FPD>, which VMS sets on an EDITPC abort (the manual's note 11)"},
	},
}

// pslFPD is PSL<FPD>, first part done (bit 27).
const pslFPD = 1 << 27

// TestInsn35Oracle runs each Phase 35 probe under govax and compares
// every record with VMS's run (testdata/insn35/vax): whether the
// instruction completed or branched, the PSL after it, R0-R11, the
// signal array of any condition (its PC included, since govax's MACRO
// and LINK lay the image out as VMS's did), and the destination bytes.
// The fields insn35Masks lists are skipped, with their reasons.
func TestInsn35Oracle(t *testing.T) {
	vms, sources := vaxInsn35Records(t)

	for _, name := range insn35Probes {
		t.Run(name, func(t *testing.T) {
			ours, err := os.ReadFile(filepath.Join("..", "..", "testdata", "insn35", strings.ToLower(name)+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			if strings.TrimRight(string(ours), "\n") != strings.TrimRight(sources[name], "\n") {
				t.Skipf("VMS ran an earlier %s; run the probes on VMS again (testdata/insn35/README.md)", name)
			}

			govax := runInsn35Probe(t, name)
			if len(govax) != len(vms[name]) {
				t.Fatalf("govax wrote %d records, VMS %d", len(govax), len(vms[name]))
			}

			for i, v := range vms[name] {
				g := govax[i]
				mask := insn35Masks[name][v.Name]

				vSignal, gSignal := append([]uint32{}, v.Signal...), append([]uint32{}, g.Signal...)
				if mask.signalFPD && len(vSignal) > 0 && len(gSignal) > 0 {
					vSignal[len(vSignal)-1] &^= pslFPD
					gSignal[len(gSignal)-1] &^= pslFPD
				}

				var diffs []string

				if v.Flags != g.Flags {
					diffs = append(diffs, fmt.Sprintf("flags VMS %d govax %d", v.Flags, g.Flags))
				}

				if v.PSL != g.PSL {
					diffs = append(diffs, fmt.Sprintf("PSL VMS %08X govax %08X", v.PSL, g.PSL))
				}

				if v.Regs != g.Regs {
					diffs = append(diffs, fmt.Sprintf("R0-R11 VMS %X govax %X", v.Regs, g.Regs))
				}

				if fmt.Sprint(vSignal) != fmt.Sprint(gSignal) {
					diffs = append(diffs, fmt.Sprintf("signal VMS %X govax %X", v.Signal, g.Signal))
				}

				if !mask.dst && string(v.DST) != string(g.DST) {
					diffs = append(diffs, fmt.Sprintf("DST VMS % X\n\tgovax % X", v.DST, g.DST))
				}

				if len(diffs) > 0 {
					t.Errorf("case %d, %s:\n\t%s", v.Case, v.Name, strings.Join(diffs, "\n\t"))
				}
			}
		})
	}
}
