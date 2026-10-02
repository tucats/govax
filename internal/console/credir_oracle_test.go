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

	"github.com/tucats/govax/internal/rms"
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

// credirHostFile is replaced by a host file's name in a replayed command
// (see credirCommands).
const credirHostFile = "@HOSTFILE@"

// credirCommands returns the commands of credir.com that govax replays, in
// order, for a volume mounted as device: every DCL command but the ones
// that only set up the run or list its result (SET NOON, SET VERIFY, the
// DEV symbol, WRITE, DIRECTORY). The CREATE of a file from the data lines
// that follow it, which govax's CREATE doesn't do, becomes a COPY from a
// host file (credirHostFile). 'DEV' becomes device.
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
			strings.HasPrefix(upper, "DIRECTORY"):
			continue

		case strings.HasPrefix(upper, "CREATE ") && !strings.HasPrefix(upper, "CREATE/"):
			// A file from the data lines that follow, which govax's
			// CREATE doesn't do: COPY makes it from a host file holding
			// the same text, so the directory gets the same entry.
			commands = append(commands, "COPY "+credirHostFile+"/HOST "+strings.TrimSpace(line[len("CREATE "):]))

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

	hostFile := filepath.Join(t.TempDir(), "first.dat")
	if err := os.WriteFile(hostFile, []byte("FIRST.DAT\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var govaxMessages []credirMessages

	for _, cmd := range credirCommands(t, "DUA1") {
		out.Reset()

		// Some commands fail on purpose, as on VMS.
		_ = d.Dispatch(strings.ReplaceAll(cmd, credirHostFile, `"`+hostFile+`"`))

		if strings.HasPrefix(strings.ToUpper(cmd), "CREATE/DIRECTORY") {
			govaxMessages = append(govaxMessages, credirMessages{command: cmd, lines: messageLines(out.String())})
		}
	}

	compareCredirMessages(t, credirLogMessages(t, c), govaxMessages)

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

// credirMessages is one CREATE/DIRECTORY command and the message lines it
// showed.
type credirMessages struct {
	command string
	lines   []string
}

// messageLines returns text's message lines: those starting "%" or "-".
func messageLines(text string) []string {
	var lines []string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r ")
		if strings.HasPrefix(line, "%") || strings.HasPrefix(line, "-") {
			lines = append(lines, line)
		}
	}

	return lines
}

// credirLogMessages reads CREDIR.LOG from the VMS volume (DUA2) and
// returns each CREATE/DIRECTORY command in it with its messages.
func credirLogMessages(t *testing.T, c *Console) []credirMessages {
	t.Helper()

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA2:[000000]CREDIR.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	var (
		out     []credirMessages
		current *credirMessages
	)

	for _, r := range records {
		line := strings.TrimRight(string(r), "\r ")

		if command, ok := strings.CutPrefix(line, "$ "); ok {
			current = nil

			if strings.HasPrefix(strings.ToUpper(command), "CREATE/DIRECTORY") {
				out = append(out, credirMessages{command: command})
				current = &out[len(out)-1]
			}

			continue
		}

		if current != nil && (strings.HasPrefix(line, "%") || strings.HasPrefix(line, "-")) {
			current.lines = append(current.lines, line)
		}
	}

	return out
}

// compareCredirMessages compares VMS's messages for each CREATE/DIRECTORY
// with govax's, command by command. VMS's log shows a translated 'DEV' as
// the device itself, so commands are matched by position.
func compareCredirMessages(t *testing.T, vms, govax []credirMessages) {
	t.Helper()

	if len(vms) == 0 {
		t.Fatal("found no CREATE/DIRECTORY commands in CREDIR.LOG")
	}

	lines := 0
	for _, v := range vms {
		lines += len(v.lines)
	}

	t.Logf("compared %d commands, %d VMS message lines", len(vms), lines)

	if len(vms) != len(govax) {
		t.Errorf("VMS's log has %d CREATE/DIRECTORY commands, govax ran %d", len(vms), len(govax))
	}

	for i := range min(len(vms), len(govax)) {
		v, g := vms[i], govax[i]

		if strings.Join(v.lines, "\n") != strings.Join(g.lines, "\n") {
			t.Errorf("$ %s\nVMS:\n  %s\ngovax:\n  %s", v.command, strings.Join(v.lines, "\n  "), strings.Join(g.lines, "\n  "))
		}
	}
}
