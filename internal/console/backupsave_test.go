package console

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// backupSaveLines are the saves of VMS's log of the probe (Part 2), each
// with what VMS wrote after it, and the SHOW SYMBOL $STATUS lines after
// them. TODAY, the probe's day, is written as the date it was
// (20-OCT-2026), since the test's clock has a day of its own.
func backupSaveLines(t *testing.T) (commands []string, outputs [][]string) {
	t.Helper()

	data, err := os.ReadFile(backupSets + "backup69.log")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(string(data), "\n")
	command := regexp.MustCompile(`^\$ +(\S.*)$`)
	inPart := false

	for i := 0; i < len(lines); i++ {
		switch {
		case strings.Contains(lines[i], "Part 2: save sets"):
			inPart = true
		case strings.Contains(lines[i], "Part 3:"):
			return commands, outputs
		}

		m := command.FindStringSubmatch(lines[i])
		if !inPart || m == nil || !strings.HasPrefix(m[1], "BACKUP") && m[1] != "SHOW SYMBOL $STATUS" {
			continue
		}

		var out []string

		for i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "$") {
			i++
			out = append(out, lines[i])
		}

		commands = append(commands, strings.ReplaceAll(m[1], "=TODAY", "=20-OCT-2026"))
		outputs = append(outputs, out)
	}

	return commands, outputs
}

// readVolumeSaveSet reads a save set file on the console's volume.
func readVolumeSaveSet(t *testing.T, c *Console, name string) ([]byte, *backup.SaveSet) {
	t.Helper()

	f, err := c.ContainerSession.OpenSaveSet(rms.FileLocation{Name: name})
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}

	defer f.Close()

	data, err := io.ReadAll(io.NewSectionReader(f, 0, f.Size))
	if err != nil {
		t.Fatal(err)
	}

	s, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{CheckCRC: true})
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	return data, s
}

// TestBackupSaveOracle restores VMS's [B69] tree from its save sets
// (B69.BCK, then B69IGN.BCK over NOBACK.TXT for its data), replays the
// probe's saves on it, and holds each save set govax writes up against
// VMS's: the messages and statuses line for line, then the save set
// byte for byte, once what is the run's own is taken from govax's (the
// summary's date, CPU ID, device, and system version; the command where
// the test changed TODAY; the file IDs, backlinks, and high-water marks,
// which a restore makes anew; and of each directory file its dates,
// revision count, and contents, which hold file IDs).
func TestBackupSaveOracle(t *testing.T) {
	d, c, output := mountBackupSets(t)

	for _, line := range []string{
		"BACKUP [SETS]B69.BCK/SAVE_SET [*...]/BY_OWNER=ORIGINAL",
		"BACKUP [SETS]B69IGN.BCK/SAVE_SET [*...]/BY_OWNER=ORIGINAL/OVERLAY",
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	output()

	commands, outputs := backupSaveLines(t)
	if len(commands) < 16 {
		t.Fatalf("%d commands from the log", len(commands))
	}

	saveSet := regexp.MustCompile(`\[SETS\](\w+)\.BCK/SAVE_SET`)

	for i, cmd := range commands {
		err := d.Dispatch(cmd)
		got := output()

		if err != nil && !vmserrors.MessageInhibited(err) {
			got += "%" + err.Error() + "\n"
		}

		want := strings.Join(outputs[i], "\n")
		if diff := firstDifference(strings.TrimRight(got, "\n"), strings.TrimRight(want, "\n")); diff != "" {
			t.Errorf("%s: %s", cmd, diff)
		}

		if m := saveSet.FindStringSubmatch(cmd); m != nil {
			t.Run(m[1], func(t *testing.T) {
				compareSaveSet(t, c, m[1], "DUA1:[SETS]"+m[1]+".BCK;2", "DUA1:[SETS]"+m[1]+".BCK;1")
			})
		}
	}
}

// compareSaveSet holds govax's save set name, the volume file gotFile,
// up against VMS's, wantFile: it rewrites VMS's with govax's values for
// what is the run's own and wants govax's bytes.
func compareSaveSet(t *testing.T, c *Console, name, gotFile, wantFile string) {
	gotData, got := readVolumeSaveSet(t, c, gotFile)
	wantData, want := readVolumeSaveSet(t, c, wantFile)

	if len(got.Files) != len(want.Files) {
		var names []string
		for _, f := range got.Files {
			names = append(names, f.Name)
		}

		t.Fatalf("%d files, want %d: %v", len(got.Files), len(want.Files), names)
	}

	recorded := strings.ReplaceAll(want.Summary.Command, "=TODAY", "=20-OCT-2026")
	if got.Summary.Command != recorded {
		t.Errorf("command %q, want %q", got.Summary.Command, recorded)
	}

	s := want.Summary
	s.Date, s.CPUID, s.Device, s.SystemVersion = got.Summary.Date, got.Summary.CPUID, got.Summary.Device, got.Summary.SystemVersion
	s.Command = recorded

	var out bytes.Buffer

	w, err := backup.NewWriter(&out, s, backup.ParseHeader(wantData).HasCRC())
	if err != nil {
		t.Fatal(err)
	}

	for i, f := range want.Files {
		g := got.Files[i]
		if g.Name != f.Name {
			t.Fatalf("file %d is %s, want %s", i+1, g.Name, f.Name)
		}

		a := f.Attributes
		a.FID, a.BackLink, a.Highwater = g.FID, g.BackLink, g.Highwater
		data := f.Data()

		if a.Directory() {
			a.Created, a.Revised, a.Revision = g.Created, g.Revised, g.Revision
			data = g.Data()
		}

		if name == "LIST" && i == 1 {
			if err := w.EndBlock(); err != nil {
				t.Fatal(err)
			}
		}

		if err := w.Add(a, f.Flags, data); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	expected := out.Bytes()
	if len(gotData) != len(expected) {
		t.Fatalf("%d bytes, want %d", len(gotData), len(expected))
	}

	for i := range gotData {
		if gotData[i] != expected[i] {
			block, at := i/got.BlockSize, i%got.BlockSize
			t.Fatalf("first difference in block %d at offset %d: %02X, want %02X%s", block+1, at, gotData[i], expected[i], whereInBlock(expected[block*got.BlockSize:(block+1)*got.BlockSize], at))
		}
	}
}

// whereInBlock names the record of block that offset at falls in.
func whereInBlock(block []byte, at int) string {
	if at < backup.HeaderSize {
		return " (the block header)"
	}

	records, _ := backup.Records(block)
	pos := backup.HeaderSize

	for _, r := range records {
		end := pos + backup.RecordHeaderSize + len(r.Body)
		if at < end {
			return fmt.Sprintf(" (a type %d record at %d, body offset %d)", r.Type, pos, at-pos-backup.RecordHeaderSize)
		}

		pos = end
	}

	return " (the null record)"
}

// TestBackupSaveMessages: what a save says when an input finds nothing
// (SEARCHFAIL with RMS's reason), when its selection saves nothing
// (NOFILES), and when its save set can't be made (OPENOUT), with the
// statuses; and that a save set written as a host file reads back,
// listed and restored, as one on a volume does.
func TestBackupSaveMessages(t *testing.T) {
	d, _, output := mountBackupSets(t)

	if err := d.Dispatch("BACKUP [SETS]B69.BCK/SAVE_SET [*...]"); err != nil {
		t.Fatal(err)
	}

	output()

	host := t.TempDir() + "/text.bck"

	cases := []struct {
		line, want string
		status     string
	}{
		{"BACKUP [NOPE]*.* [SETS]X.BCK/SAVE_SET",
			"%BACKUP-E-SEARCHFAIL, error searching for DUA1:[NOPE]*.*;\n-RMS-E-DNF, directory not found\n-SYSTEM-W-NOSUCHFILE, no such file",
			"%X10A3FFA2"},
		{"BACKUP [B69]NONE.* [SETS]X.BCK/SAVE_SET",
			"%BACKUP-E-SEARCHFAIL, error searching for DUA1:[B69]NONE.*;\n-RMS-E-FNF, file not found", "%X10A3FFA2"},
		{"BACKUP [B69]*.*/SINCE=TOMORROW [SETS]X.BCK/SAVE_SET",
			"%BACKUP-W-NOFILES, no files selected from DUA1:[B69]*.*;", "%X10A38400"},
		{"BACKUP [B69]*.* [NOPE]X.BCK/SAVE_SET",
			"%BACKUP-E-OPENOUT, error opening DUA1:[NOPE]X.BCK; as output\n-RMS-E-DNF, directory not found\n-SYSTEM-W-NOSUCHFILE, no such file",
			"%X10A38012"},
		{"BACKUP/LOG [B69]TEXT.TXT,EMPTY.DAT \"" + host + "\"/SAVE_SET/CRC",
			"%BACKUP-S-COPIED, copied DUA1:[B69]TEXT.TXT;1\n%BACKUP-S-COPIED, copied DUA1:[B69]EMPTY.DAT;1", "%X10000001"},
		{"BACKUP/LOG \"" + host + "\"/SAVE_SET/CRC [R1]",
			"%BACKUP-S-CREDIR, created directory DUA1:[R1]\n%BACKUP-S-CREATED, created DUA1:[R1]TEXT.TXT;1\n%BACKUP-S-CREATED, created DUA1:[R1]EMPTY.DAT;1",
			"%X10000001"},
		{"BACKUP/COMPARE/LOG [SETS]TEXT.BCK/SAVE_SET [R1]",
			"%BACKUP-S-COMPARED, compared DUA1:[R1]TEXT.TXT;1", "%X10000001"},
	}

	for _, tc := range cases {
		err := d.Dispatch(tc.line)
		got := strings.TrimRight(output(), "\n")

		if err != nil && !vmserrors.MessageInhibited(err) {
			got = strings.TrimLeft(got+"\n%"+err.Error(), "\n")
		}

		if got != tc.want {
			t.Errorf("%s:\n%s\nwant\n%s", tc.line, got, tc.want)
		}

		if err := d.Dispatch("SHOW SYMBOL $STATUS"); err != nil {
			t.Fatal(err)
		}

		if status := strings.TrimSpace(output()); status != `$STATUS == "`+tc.status+`"` {
			t.Errorf("%s: %s, want %s", tc.line, status, tc.status)
		}
	}
}
