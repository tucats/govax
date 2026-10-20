package console

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-69 - backup save sets.md, subtask 12: the second round.
// VMS V7.1 ran testdata/backup69/round2's RUN2.COM on a volume govax
// made (round 1's tree restored by govax, and govax's save sets of it),
// listing, restoring, comparing, and damaging govax's save sets.
// TestBackup69Round2 runs RUN2.COM on govax on the same volume, the save
// sets VMS read (vax/round2/govax), and holds its log and reports to
// VMS's, masked as round 1's are (backup69Mask), but for what VMS
// answered differently by design: HOST.BCK, a save set as fixed 512-byte
// records, which VMS refuses and govax reads (Decision 3), and
// ANALYZE/DISK_STRUCTURE, which govax lacks.

// backup69Round2 holds VMS's results of round 2, and govax's save sets
// as VMS read them in govax/.
var backup69Round2 = filepath.Join(backupSets, "round2")

// backup69Round2Govax is a console with round 2's exchange volume, made
// as exchange.cmd made it: its lines run on an RD54 with 3-block
// clusters in DUA1 (the volume and the host save set in the test's
// directories), so round 1's tree is restored by govax and the save sets
// in [SETS] are govax's, with the attributes govax gives a save set's
// file. The save sets VMS read (vax/round2/govax) go into [VAX], for
// backup69Round2SaveSets.
func backup69Round2Govax(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	path := filepath.Join(t.TempDir(), "round2.dsk")
	if err := c.cmd().InitializeContainer(path, 20000, "BACKUP69", 3, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	host := filepath.Join(t.TempDir(), "backup69-host.bck")
	testdata := filepath.Join("..", "..", "testdata") + string(filepath.Separator)

	for line := range strings.SplitSeq(link68Read(t, filepath.Join(backup69Dir, "round2", "exchange.cmd")), "\n") {
		switch {
		case line == "", strings.HasPrefix(line, "!"), strings.HasPrefix(line, "INITIALIZE"), strings.HasPrefix(line, "MOUNT"),
			strings.HasPrefix(line, "DISMOUNT"), line == "SET DEFAULT DUA0:[000000]":
			continue
		}

		line = strings.ReplaceAll(line, `"testdata/disks/backup69-host.bck"`, `"`+host+`"`)
		line = strings.ReplaceAll(line, `"testdata/`, `"`+testdata)

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	shareFindRun(t, d, "CREATE/DIRECTORY [VAX]")

	for _, name := range backup69SaveSets {
		set := filepath.Join(backup69Round2, "govax", strings.ToLower(name)+".bck")
		shareFindRun(t, d, `COPY/BINARY "`+set+`" [VAX]`+name+".BCK")
	}

	buf.Reset()

	return d, c, buf
}

// backup69Round2Reports are the reports copyout.cmd brought back, by
// their names in [RESULTS].
var backup69Round2Reports = []string{"tree2.lst", "sets2.lst", "vb69.lst", "b69full2.lis"}

// backup69Round2Damaged are the save sets SSCOPY made in Part 5, each
// from its source with one byte of block 2 complemented. govax's save
// sets carry the test's own date, so its copies are held to VMS's by
// that: each differs from its source in that byte alone.
var backup69Round2Damaged = []struct {
	name, source string
	offset       int
}{
	{"b69d.bck", "b69.bck", 5000},
	{"b69crcd.bck", "b69crc.bck", 5000},
	{"b69hdrd.bck", "b69.bck", 100},
}

// backup69Round2TreeDifference is TREE2.LST's VERLIM.TXT: VMS read the
// tree govax restored before a restore gave a file's directory record its
// saved version limit, which is where VMS reads it, so VMS found none;
// govax's DIRECTORY reads the header's (TestRestoreEntryVersionLimit
// checks the record now).
var backup69Round2TreeDifference = link68Difference{
	line: regexp.MustCompile(`^File attributes:    Allocation: 3, Extend: 0, Global buffer count: 0, No version limit$`),
	why:  "VERLIM.TXT's version limit, which govax's restore didn't yet put in the directory record when VMS read it",
}

// backup69ChangedByte says whether damaged is source with the byte at
// offset of block 2 (32256-byte blocks) complemented, and nothing else.
func backup69ChangedByte(damaged, source []byte, offset int) bool {
	at := 32256 + offset
	if len(damaged) != len(source) || at >= len(source) || damaged[at] != ^source[at] {
		return false
	}

	return bytes.Equal(damaged[:at], source[:at]) && bytes.Equal(damaged[at+1:], source[at+1:])
}

// TestBackup69Round2 makes round 2's volume, holds its save sets to the
// ones VMS read, runs RUN2.COM on govax, and holds its log, reports, and
// damaged save sets to VMS's.
func TestBackup69Round2(t *testing.T) {
	d, c, buf := backup69Round2Govax(t)

	// The exchange's save sets, made again, are the ones VMS read, but
	// for the run's own values. HOST.BCK's command names the host file,
	// which the test's run puts elsewhere; its listing is checked below.
	for _, name := range backup69SaveSets {
		t.Run(name, func(t *testing.T) {
			compareSaveSet(t, c, name, "DUA1:[SETS]"+name+".BCK", "DUA1:[VAX]"+name+".BCK")
		})
	}

	if err := d.Dispatch("@RUN2/OUTPUT=RUN2.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@RUN2: %v\n%s", err, buf)
	}

	out := t.TempDir()
	shareFindRun(t, d, `COPY DUA1:[000000]RUN2.LOG "`+filepath.Join(out, "run2.log")+`"/HOST/QUIET`)

	for _, name := range backup69Round2Reports {
		shareFindRun(t, d, `COPY DUA1:[RESULTS]`+strings.ToUpper(name)+` "`+filepath.Join(out, name)+`"/HOST/QUIET`)
	}

	for _, set := range backup69Round2Damaged {
		for _, name := range []string{set.name, set.source} {
			shareFindRun(t, d, `COPY/BINARY DUA1:[SETS]`+strings.ToUpper(name)+` "`+filepath.Join(out, name)+`"/HOST/QUIET`)
		}
	}

	t.Run("log", func(t *testing.T) { backup69Round2Log(t, out) })

	for _, name := range backup69Round2Reports {
		t.Run(name, func(t *testing.T) {
			want := backup69Mask(link68Read(t, filepath.Join(backup69Round2, name)))
			got := backup69Mask(link68Read(t, filepath.Join(out, name)))

			diffs := backup69Differences
			if name == "tree2.lst" {
				diffs = append(append([]link68Difference{}, diffs...), backup69Round2TreeDifference)
			}

			link68CompareLines(t, name, got, want, diffs)
		})
	}

	for _, set := range backup69Round2Damaged {
		t.Run(set.name, func(t *testing.T) {
			vmsSource := link68ReadBytes(t, filepath.Join(backup69Round2, "govax", set.source))
			if !backup69ChangedByte(link68ReadBytes(t, filepath.Join(backup69Round2, set.name)), vmsSource, set.offset) {
				t.Errorf("VMS's %s isn't %s with byte %d of block 2 changed", set.name, set.source, set.offset)
			}

			if !backup69ChangedByte(link68ReadBytes(t, filepath.Join(out, set.name)), link68ReadBytes(t, filepath.Join(out, set.source)), set.offset) {
				t.Errorf("govax's %s isn't its %s with byte %d of block 2 changed", set.name, set.source, set.offset)
			}
		})
	}
}

// The commands of RUN2.COM whose output differs by design, which
// backup69Round2Log cuts out of both logs and checks on their own.
const (
	backup69Round2ListHost    = "$   BACKUP/LIST [SETS]HOST.BCK/SAVE_SET"
	backup69Round2RestoreHost = "$ BACKUP/LOG [SETS]HOST.BCK/SAVE_SET/CRC [VHOST...]/BY_OWNER=ORIGINAL"
	backup69Round2Directory   = "$ DIRECTORY/SIZE=ALL [VB69K8...],[VB69G0...],[VB69G3...],[VB69CRC...],[VHOST...]"
	backup69Round2Analyze     = "$ ANALYZE/DISK_STRUCTURE DUA1:"
)

// backup69Round2Log holds govax's RUN2.LOG to VMS's. HOST.BCK's listing
// and restore are VMS's NOTSAVESET, %X10A38104, where govax reads the
// save set as any other (and its listing names HOST:, the host, as the
// device it was written on); so [VHOST...] is missing from VMS's
// DIRECTORY, whose lines before it are compared. ANALYZE/DISK_STRUCTURE
// is VMS's report (only BADHIGHWATER notices for its own header-only
// restores of NOBACK.TXT) and govax's DCL-E-UNRECOGNIZED.
func backup69Round2Log(t *testing.T, out string) {
	vms := backup69Mask(link68Read(t, filepath.Join(backup69Round2, "run2.log")))
	govax := backup69Mask(link68Read(t, filepath.Join(out, "run2.log")))

	for _, cut := range []struct {
		command string
		vms     func(t *testing.T, lines []string)
		govax   func(t *testing.T, lines []string)
	}{
		{backup69Round2ListHost, backup69NotSaveSet, backup69Listed},
		{backup69Round2RestoreHost, backup69NotSaveSet, backup69Restored},
		{backup69Round2Directory, nil, nil},
		{backup69Round2Analyze, backup69Analyzed, backup69NoAnalyze},
	} {
		var vmsOut, govaxOut []string

		vms, vmsOut = backup69CutCommand(t, vms, cut.command)
		govax, govaxOut = backup69CutCommand(t, govax, cut.command)

		if cut.command == backup69Round2Directory {
			backup69DirectoryBeforeHost(t, govaxOut, vmsOut)

			continue
		}

		t.Run(strings.Fields(cut.command)[1], func(t *testing.T) {
			cut.vms(t, vmsOut)
			cut.govax(t, govaxOut)
		})
	}

	link68CompareLines(t, "run2.log", govax, vms, backup69Differences)
}

// backup69CutCommand takes out of lines the command's output (the lines
// after it up to the next "$" line) and, when a SHOW SYMBOL $STATUS
// follows, its line too, appended to the output; the command line
// stays.
func backup69CutCommand(t *testing.T, lines []string, command string) (rest, output []string) {
	t.Helper()

	for i, line := range lines {
		if line != command {
			continue
		}

		end := i + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "$") {
			end++
		}

		output = append(output, lines[i+1:end]...)
		rest = append(rest, lines[:i+1]...)

		if end+1 < len(lines) && strings.TrimLeft(lines[end], "$ ") == "SHOW SYMBOL $STATUS" {
			output = append(output, lines[end+1])
			rest = append(rest, lines[end])
			end += 2
		}

		return append(rest, lines[end:]...), output
	}

	t.Fatalf("%q not in the log", command)

	return nil, nil
}

// backup69NotSaveSet: VMS refused HOST.BCK, fixed 512-byte records.
func backup69NotSaveSet(t *testing.T, lines []string) {
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "%BACKUP-F-NOTSAVESET, DUA1:[SETS]HOST.BCK;1 is not a BACKUP save set") ||
		!strings.HasSuffix(text, `$STATUS == "%X10A38104"`) {
		t.Errorf("VMS's lines:\n%s", text)
	}
}

// backup69Listed: govax listed HOST.BCK, all 25 files (its device
// HOST:, the host, masked).
func backup69Listed(t *testing.T, lines []string) {
	text := strings.Join(lines, "\n")
	if !strings.Contains(text, "Total of 25 files, 354 blocks") ||
		!strings.HasSuffix(text, `$STATUS == "%X10000001"`) {
		t.Errorf("govax's lines:\n%s", text)
	}
}

// backup69Restored: govax restored HOST.BCK's 23 files and its two
// directories, CRCs checked.
func backup69Restored(t *testing.T, lines []string) {
	text := strings.Join(lines, "\n")
	if n := strings.Count(text, "%BACKUP-S-CREATED, "); n != 23 || !strings.HasSuffix(text, `$STATUS == "%X10000001"`) {
		t.Errorf("%d files created; govax's lines:\n%s", n, text)
	}
}

// backup69Analyzed: VMS's ANALYZE/DISK_STRUCTURE found nothing wrong
// but QUOTA.SYS missing and the high-water marks of the twelve header-
// only NOBACK.TXT files its own restores made.
func backup69Analyzed(t *testing.T, lines []string) {
	for _, line := range lines[1:] {
		switch {
		case line == "", line == "%ANALDISK-I-OPENQUOTA, error opening QUOTA.SYS", line == "-SYSTEM-W-NOSUCHFILE, no such file",
			strings.HasPrefix(line, "%ANALDISK-I-BADHIGHWATER, file ") && strings.HasSuffix(line, " NOBACK.TXT;1"),
			line == "\tinconsistent highwater mark and EFBLK", line == `  $STATUS == "%X00000001"`:
		default:
			t.Errorf("VMS's line %q", line)
		}
	}
}

// backup69NoAnalyze: govax has no ANALYZE/DISK_STRUCTURE.
func backup69NoAnalyze(t *testing.T, lines []string) {
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "%DCL-E-UNRECOGNIZED, ") {
		t.Errorf("govax's lines %q", lines)
	}
}

// backup69DirectoryBeforeHost holds govax's DIRECTORY of the restored
// trees to VMS's up to [VHOST...], which VMS didn't make.
func backup69DirectoryBeforeHost(t *testing.T, got, want []string) {
	t.Helper()

	before := func(lines []string, last string) []string {
		for i, line := range lines {
			if strings.HasPrefix(line, last) {
				// A blank comes before a heading, not before a message.
				for i > 0 && lines[i-1] == "" {
					i--
				}

				return lines[:i]
			}
		}

		t.Fatalf("no %q in DIRECTORY's lines", last)

		return nil
	}

	link68CompareLines(t, "DIRECTORY", before(got, "Directory DUA1:[VHOST]"), before(want, "%DIRECT-E-OPENIN, "), nil)
}
