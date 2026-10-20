package console

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-69 - backup save sets.md, subtask 12: the milestone.
// TestBackup69Oracle runs testdata/backup69's BACKUP69.COM on govax, as
// the author ran it on VMS V7.1: the saves, the listings, the restores,
// the compares, and the errors, SSCOPY's damaged and truncated save sets
// among them. Then it copies the results out as copyout.cmd copied
// VMS's and holds each to VMS's: the log and the reports line for line,
// with what is the run's own masked (backup69Mask), but for
// backup69Differences; and Part 2's save sets byte for byte, as
// TestBackupSaveOracle holds them, once their run's own values are
// taken from govax's.

// backup69Dir holds the probe.
var backup69Dir = filepath.Join("..", "..", "testdata", "backup69")

// backup69Part1Start and backup69Part1End are the first line of Part 1
// that makes the tree and the line after its last: govax's copy of the
// procedure has CREATE/DIRECTORY [SETS] in their place, and the logs are
// compared without them.
const (
	backup69Part1Start = "$ CREATE/DIRECTORY [B69]"
	backup69Part1End   = "$ DIRECTORY/FULL/OUTPUT=[SETS]TREE.LST [B69...]"
	backup69Part1Stand = "$ CREATE/DIRECTORY [SETS]"
)

// backup69Today is the probe's day, which govax's copy of the procedure
// names where VMS's says TODAY, since the test's clock has a day of its
// own.
const backup69Today = "=20-OCT-2026"

// backup69Govax is a console with the probe's exchange volume, an RD54
// with VMS's 3-block clusters in DUA1 holding SSCOPY.MAR and govax's
// copy of BACKUP69.COM, the default directory DUA1:[000000]. Part 1's
// tree is VMS's own, restored from its save sets with every saved
// attribute (B69IGN.BCK gives NOBACK.TXT its data): govax has no
// CONVERT, SET FILE, or SET SECURITY to make it. VMS's save sets are in
// [VAX], for the save sets' comparison.
func backup69Govax(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	path := filepath.Join(t.TempDir(), "backup69.dsk")
	if err := c.cmd().InitializeContainer(path, 20000, "BACKUP69", 3, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	host := filepath.Join(t.TempDir(), "backup69.com")
	if err := os.WriteFile(host, []byte(backup69Procedure(t)), 0o644); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY "`+filepath.Join(backup69Dir, "sscopy.mar")+`"/HOST DUA1:[000000]SSCOPY.MAR`)
	shareFindRun(t, d, `COPY "`+host+`"/HOST DUA1:[000000]BACKUP69.COM`)
	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, `BACKUP "`+backupSets+`b69.bck"/SAVE_SET [*...]/BY_OWNER=ORIGINAL`)
	shareFindRun(t, d, `BACKUP "`+backupSets+`b69ign.bck"/SAVE_SET [*...]/BY_OWNER=ORIGINAL/OVERLAY`)
	shareFindRun(t, d, "CREATE/DIRECTORY [VAX]")

	for _, name := range backup69SaveSets {
		lower := strings.ToLower(name)
		shareFindRun(t, d, `COPY/BINARY "`+backupSets+lower+`.bck" [VAX]`+name+".BCK")
	}

	buf.Reset()

	return d, c, buf
}

// backup69SaveSets are Part 2's save sets, in the order it makes them.
var backup69SaveSets = []string{
	"B69", "B69K8", "B69G0", "B69G3", "B69CRC", "B69NOCRC", "B69COM", "B69EXC",
	"B69BYO", "B69TXT", "B69NONE", "TEXT", "B69OWN", "LIST", "B69IGN",
}

// backup69Procedure is BACKUP69.COM as govax runs it: Part 1's files
// left out (the tree is restored) and TODAY the probe's day.
func backup69Procedure(t *testing.T) string {
	t.Helper()

	text := strings.ReplaceAll(link68Read(t, filepath.Join(backup69Dir, "backup69.com")), "=TODAY", backup69Today)

	return strings.Join(backup69CutPart1(t, strings.Split(text, "\n")), "\n")
}

// backup69CutPart1 puts CREATE/DIRECTORY [SETS] in place of the lines
// of Part 1 that make the tree, in the procedure or its log.
func backup69CutPart1(t *testing.T, lines []string) []string {
	t.Helper()

	start, end := -1, -1

	for i, line := range lines {
		switch {
		case line == backup69Part1Start && start < 0:
			start = i
		case line == backup69Part1End && start >= 0:
			end = i
		}

		if end >= 0 {
			break
		}
	}

	if start < 0 || end < 0 {
		t.Fatal("Part 1's lines not found")
	}

	cut := append([]string{}, lines[:start]...)
	cut = append(cut, backup69Part1Stand)

	return append(cut, lines[end:]...)
}

// backup69Differences are the lines of VMS's log and reports, masked
// (backup69Mask), that govax's needn't match.
var backup69Differences = []link68Difference{
	{regexp.MustCompile(`^V7\.1 +VAX 8600$`), "F$GETSYI's VERSION and HW_NAME: govax says V7.3 and names itself"},
	{regexp.MustCompile(`^File organization:  Indexed, Prolog: 3, Using 1 key$`), "DIRECTORY/FULL of INDEXED.IDX: govax doesn't read an indexed file's prolog, so it shows neither its version nor its keys"},
	{regexp.MustCompile(`Revised: +<time> \(0\)$`), "the tree's [B69.SUB1] and [B69.SUB1.SUB2]: VMS's CREATE/DIRECTORY made them with revision 0, the test's restore with 1, as VMS's BACKUP makes a directory"},
}

var (
	// backup69Entry is a line that starts a file's entry: in a
	// DIRECTORY report or a listing, its name (with a directory, a
	// listing's), and the type that says whether the run made it.
	backup69Entry = regexp.MustCompile(`^(\[[A-Z0-9.$_]+\])?[A-Z0-9$_]*\.([A-Z0-9$_]*);\d+`)

	// backup69Times are a date with its time, as DIRECTORY and BACKUP's
	// listings write them.
	backup69Times = regexp.MustCompile(`[ 0-9]\d-[A-Z]{3}-\d{4} \d\d:\d\d(:\d\d\.\d\d)?`)

	// backup69FileIDs are DIRECTORY/FULL's and BACKUP/LIST/FULL's file
	// IDs, which the volume gives.
	backup69FileIDs = regexp.MustCompile(`File ID: +\(\d+,\d+,\d+\) *`)

	// backup69System are the summary's fields that name the system that
	// wrote the save set.
	backup69System = regexp.MustCompile(`^(Operating system:  OpenVMS VAX version|Written on:) .*$`)
)

// backup69CPUID starts the summary's CPU ID line, which a listing leaves
// out for govax's CPU ID, 0, and so is taken out of both runs' lines.
const backup69CPUID = "CPU ID register:"

// backup69RunMade are the types of the files whose times are the run's
// own: the directories (the tree's made by the test's restore, VMS's by
// CREATE/DIRECTORY), the save sets, and the reports.
var backup69RunMade = map[string]bool{"DIR": true, "BCK": true, "LST": true, "LIS": true, "LOG": true}

// backup69Mask masks what differs in every run, VMS's or govax's: the
// times of the files the run made (on an entry's first line and the
// lines that follow it to the next entry or a blank line), a save set's
// date and the system that wrote it (its CPU ID line taken out), and
// file IDs. The restored files' dates are the saved files', so they're
// compared.
func backup69Mask(text string) []string {
	var lines []string

	for line := range strings.SplitSeq(strings.TrimRight(strings.ReplaceAll(text, "\r", ""), "\n"), "\n") {
		if !strings.HasPrefix(line, backup69CPUID) {
			lines = append(lines, line)
		}
	}

	runMade := false

	for i, line := range lines {
		if m := backup69Entry.FindStringSubmatch(line); m != nil {
			runMade = backup69RunMade[m[2]]
		} else if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, " ") && !strings.Contains(line, ":") {
			runMade = false
		}

		if runMade || strings.HasPrefix(line, "Date:  ") {
			line = backup69Times.ReplaceAllString(line, "<time>")
		}

		line = backup69FileIDs.ReplaceAllString(line, "File ID: (masked) ")
		lines[i] = backup69System.ReplaceAllString(line, "$1 <system>")
	}

	return lines
}

// backup69Reports are the files copyout.cmd brought back as text.
var backup69Reports = []string{"tree.lst", "sets.lst", "r1.lst", "r7.lst", "text.lis", "b69full.lis"}

// TestBackup69Oracle runs BACKUP69.COM on govax and holds its log, its
// reports, and Part 2's save sets to VMS's.
func TestBackup69Oracle(t *testing.T) {
	d, c, buf := backup69Govax(t)

	if err := d.Dispatch("@BACKUP69/OUTPUT=BACKUP69.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@BACKUP69: %v\n%s", err, buf)
	}

	out := t.TempDir()
	shareFindRun(t, d, `COPY DUA1:[000000]BACKUP69.LOG "`+filepath.Join(out, "backup69.log")+`"/HOST`)

	for _, name := range backup69Reports {
		shareFindRun(t, d, `COPY DUA1:[SETS]`+strings.ToUpper(name)+` "`+filepath.Join(out, name)+`"/HOST`)
	}

	t.Run("log", func(t *testing.T) {
		vax := strings.ReplaceAll(link68Read(t, backupSets+"backup69.log"), "=TODAY", backup69Today)
		want := backup69Mask(strings.Join(backup69CutPart1(t, strings.Split(vax, "\n")), "\n"))
		got := backup69Mask(link68Read(t, filepath.Join(out, "backup69.log")))

		link68CompareLines(t, "backup69.log", got, want, backup69Differences)
	})

	for _, name := range backup69Reports {
		t.Run(name, func(t *testing.T) {
			want := backup69Mask(link68Read(t, backupSets+name))
			got := backup69Mask(link68Read(t, filepath.Join(out, name)))

			link68CompareLines(t, name, got, want, backup69Differences)
		})
	}

	for _, name := range backup69SaveSets {
		t.Run(name, func(t *testing.T) {
			compareSaveSet(t, c, name, "DUA1:[SETS]"+name+".BCK", "DUA1:[VAX]"+name+".BCK")
		})
	}
}
