package console

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/ods2/filespec"
	"github.com/tucats/ods2/volume"
)

// The Phase 34 oracle's reconciliation (docs/PHASE-34.md, subtask 8):
// credir.com's commands run under govax on a freshly built exchange
// volume, and every directory they make is compared with the one VMS 7.3
// made for the same command (testdata/credir/README.md).

// credirVAXDisk is the container after CREDIR.COM ran on VMS:
// testdata/credir/vax/credir-vax.dsk.gz, unless the environment variable
// CREDIR_VAX_DISK names another.
var credirVAXDisk = func() string {
	if p := os.Getenv("CREDIR_VAX_DISK"); p != "" {
		return p
	}

	return filepath.Join("..", "..", "testdata", "credir", "vax", "credir-vax.dsk.gz")
}()

// credirProcedure is the command procedure VMS ran.
var credirProcedure = filepath.Join("..", "..", "testdata", "credir", "credir.com")

// credirCommands returns the commands of credir.com that govax replays, in
// order, for a volume mounted as device: every DCL command but the ones
// that only set up the run or list its result (SET NOON, SET VERIFY, the
// DEV symbol, WRITE, DIRECTORY), and the CREATE of a file from the data
// lines that follow it, which govax's CREATE doesn't do. 'DEV' becomes
// device.
func credirCommands(t *testing.T, device string) []string {
	t.Helper()

	data, err := os.ReadFile(credirProcedure)
	if err != nil {
		t.Fatal(err)
	}

	var commands []string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "$ ")
		if !ok {
			continue // a data line
		}

		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)

		switch {
		case line == "", strings.HasPrefix(line, "!"),
			strings.HasPrefix(upper, "SET NOON"), strings.HasPrefix(upper, "SET VERIFY"),
			strings.HasPrefix(upper, "DEV ="), strings.HasPrefix(upper, "WRITE "),
			strings.HasPrefix(upper, "DIRECTORY"),
			strings.HasPrefix(upper, "CREATE ") && !strings.HasPrefix(upper, "CREATE/"):
			continue
		}

		commands = append(commands, strings.ReplaceAll(line, "'DEV'", device+":"))
	}

	return commands
}

// credirDirectory is what's compared of one directory.
type credirDirectory struct {
	header volume.File
	entry  uint16 // the version limit on its entry in its parent
}

// credirDirectories returns every directory on the volume mounted as
// device but the MFD, by path ("A.B"), with what's compared of it.
func credirDirectories(t *testing.T, c *Console, device string) map[string]credirDirectory {
	t.Helper()

	vol, ok := c.Mounts.Lookup(device)
	if !ok {
		t.Fatalf("%s isn't mounted", device)
	}

	spec, err := filespec.Parse("[000000...]*.DIR;*", filespec.Spec{})
	if err != nil {
		t.Fatal(err)
	}

	matches, err := filespec.Glob(vol, spec)
	if err != nil {
		t.Fatal(err)
	}

	out := map[string]credirDirectory{}

	for _, m := range matches {
		if len(m.Dirs) == 0 && m.Name == "000000" {
			continue
		}

		f, err := vol.OpenFID(m.Fid)
		if err != nil {
			t.Fatalf("%s: %v", m, err)
		}

		parent, err := filespec.ResolveDirectory(vol, m.Dirs)
		if err != nil {
			t.Fatalf("%s's parent: %v", m, err)
		}

		entry, err := parent.Lookup(m.Name+".DIR", m.Version)
		if err != nil {
			t.Fatalf("%s's entry: %v", m, err)
		}

		out[strings.Join(append(append([]string{}, m.Dirs...), m.Name), ".")] = credirDirectory{header: *f, entry: entry.VersionLimit}
	}

	return out
}

// credirFields renders the compared fields of d, one "name=value" per
// field, so a difference names the field. File IDs, LBNs, and dates are
// left out: they depend on the order things were allocated in and when.
func credirFields(d credirDirectory) []string {
	h := d.header.Header
	ra := h.RecordAttributes

	revision := "?"
	name := "?"

	if ident, err := h.Ident(); err == nil {
		revision = fmt.Sprint(ident.Revision)
		name = strings.TrimSpace(ident.Filename + ident.FilenameExtension)
	}

	return []string{
		"name=" + name,
		"revision=" + revision,
		fmt.Sprintf("characteristics=%#x", h.FileCharacteristics),
		fmt.Sprintf("owner=%v", h.Owner),
		fmt.Sprintf("protection=%#04x", h.FileProtection),
		fmt.Sprintf("format=%v", ra.Format),
		fmt.Sprintf("attributes=%#x", ra.Attributes),
		fmt.Sprintf("rsz=%d", ra.RecordSize),
		fmt.Sprintf("mrs=%d", ra.MaxRecordSize),
		fmt.Sprintf("hiblk=%d", ra.HighestBlock),
		fmt.Sprintf("efblk=%d", ra.EndOfFileBlock),
		fmt.Sprintf("ffb=%d", ra.FirstFreeByte),
		fmt.Sprintf("hwm=%d", h.HighWaterMark),
		fmt.Sprintf("versions=%d", ra.VersionLimit),
		fmt.Sprintf("entry-versions=%d", d.entry),
		fmt.Sprintf("extents=%d", len(d.header.Extents)),
	}
}

// credirMasked are fields that differ between VMS's directories and
// govax's for reasons outside CREATE/DIRECTORY, by directory path and
// field name, each with its reason.
var credirMasked = map[string]map[string]string{}

func TestCreateDirectoryOracle(t *testing.T) {
	if _, err := os.Stat(credirVAXDisk); err != nil {
		t.Skipf("no VMS run of the Phase 34 oracle (%s): see testdata/credir/README.md", credirVAXDisk)
	}

	vaxDisk := filepath.Join(t.TempDir(), "credir-vax.dsk")
	copyFile(t, credirVAXDisk, vaxDisk)

	govaxDisk := filepath.Join(t.TempDir(), "credir-govax.dsk")

	c, out := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	if err := c.InitializeContainer(govaxDisk, 0, "CREDIR", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA1", govaxDisk, true); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA2", vaxDisk, false); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("SET DEFAULT DUA1:[000000]"); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range credirCommands(t, "DUA1") {
		// Some commands fail on purpose, as on VMS.
		_ = d.Dispatch(cmd)
	}

	t.Logf("govax's messages:\n%s", out.String())

	vms := credirDirectories(t, c, "DUA2")
	govax := credirDirectories(t, c, "DUA1")

	paths := map[string]bool{}
	for p := range vms {
		paths[p] = true
	}

	for p := range govax {
		paths[p] = true
	}

	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}

	sort.Strings(sorted)

	for _, p := range sorted {
		v, inVMS := vms[p]
		g, inGovax := govax[p]

		switch {
		case !inGovax:
			t.Errorf("[%s]: VMS made it, govax didn't", p)

			continue
		case !inVMS:
			t.Errorf("[%s]: govax made it, VMS didn't", p)

			continue
		}

		vf, gf := credirFields(v), credirFields(g)
		for i := range vf {
			if vf[i] == gf[i] {
				continue
			}

			field, _, _ := strings.Cut(vf[i], "=")
			if credirMasked[p][field] != "" {
				continue
			}

			t.Errorf("[%s] %s: VMS %s, govax %s", p, field, strings.TrimPrefix(vf[i], field+"="), strings.TrimPrefix(gf[i], field+"="))
		}
	}
}
