package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Things in a report that are the moment's, not the file's: each page heading's
// time, FDL's IDENT, and the file ID, a restored file's being its own.
var (
	analyzePageTime = regexp.MustCompile(`\d?\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d   Page`)
	analyzeIdent    = regexp.MustCompile(`IDENT\t".*`)
	analyzeFileID   = regexp.MustCompile(`File ID: \(\d+,\d+,\d+\)`)
)

// analyzeMasked is a report with those masked.
func analyzeMasked(text string) string {
	text = analyzePageTime.ReplaceAllString(text, "TIME   Page")
	text = analyzeIdent.ReplaceAllString(text, "IDENT")

	return analyzeFileID.ReplaceAllString(text, "File ID: FID")
}

// firstDifference says where two reports first differ.
func firstDifference(want, have string) string {
	w, h := strings.Split(want, "\n"), strings.Split(have, "\n")

	for i := 0; i < max(len(w), len(h)); i++ {
		var wl, hl string
		if i < len(w) {
			wl = w[i]
		}

		if i < len(h) {
			hl = h[i]
		}

		if wl != hl {
			return fmt.Sprintf("line %d:\nVMS:   %q\ngovax: %q", i+1, wl, hl)
		}
	}

	return ""
}

// expectedAnalyzeReports are round 1's files govax doesn't read, Prologs 1 and 2, and
// the ones the save set doesn't have.
var expectedAnalyzeReports = map[string]bool{
	"e92": true, "s34": true, "s59": true, "s60": true, "s61": true,
	"l138": true, "u126": true, "u127": true,
}

// TestIdx88ReportsOracle runs ANALYZE/RMS_FILE on each of round 1's files, restored
// from VMS's save set where the VAX's were, and holds its report, /CHECK, and /FDL to
// VMS's, line for line.
func TestIdx88ReportsOracle(t *testing.T) {
	vax := filepath.Join("..", "..", "testdata", "idx88", "vax")

	names, err := filepath.Glob(filepath.Join(vax, "anl", "*.fdl"))
	if err != nil || len(names) == 0 {
		t.Skip("no VMS reports")
	}

	sort.Strings(names)

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.NodeName = "SIMVAX"
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	dispatch := func(line string) {
		t.Helper()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}
	}

	dsk := filepath.Join(t.TempDir(), "idx88.dsk")
	save, err := filepath.Abs(filepath.Join(vax, "idx88.bck"))
	if err != nil {
		t.Fatal(err)
	}

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD54 /CLUSTER=3 IDX88`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")
	dispatch(`BACKUP "` + save + `"/SAVE_SET [000000]`)

	compared := 0

	for _, path := range names {
		base := strings.TrimSuffix(filepath.Base(path), ".fdl")
		if expectedAnalyzeReports[base] {
			continue
		}

		upper := strings.ToUpper(base)

		// E103 has two versions, and the VAX's search found the newer.
		version := ";1"
		if base == "e103" {
			version = ";2"
		}

		for _, form := range []struct{ ext, qualifiers string }{
			{"anl", "/OUTPUT=" + upper + ".ANL"},
			{"chk", "/CHECK/OUTPUT=" + upper + ".CHK"},
			{"fdl", "/FDL/OUTPUT=" + upper + ".FDL"},
		} {
			want, err := os.ReadFile(filepath.Join(vax, "anl", base+"."+form.ext))
			if err != nil {
				t.Fatal(err)
			}

			dispatch("ANALYZE/RMS_FILE" + form.qualifiers + " DUA1:[000000]" + upper + ".IDX" + version)

			records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + upper + "." + strings.ToUpper(form.ext)}, rms.TextRecords)
			if err != nil {
				t.Fatal(err)
			}

			var have strings.Builder
			for _, r := range records {
				have.Write(r)
				have.WriteByte('\n')
			}

			compared++

			if diff := firstDifference(analyzeMasked(string(want)), analyzeMasked(have.String())); diff != "" {
				t.Errorf("%s.%s, %s", base, form.ext, diff)
			}
		}
	}

	t.Logf("%d reports compared", compared)
}

// analyzeDates masks a report's file dates, which are this run's.
var analyzeDates = regexp.MustCompile(`((?:Creation|Revision) Date: +)\d?\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d`)

// analyzeCommand is an ANALYZE command of round 3 and what followed it, up to the
// next command but for the status that follows it.
type analyzeCommand struct {
	command string
	lines   []string
}

// analyzeCommands picks out round 3's ANALYZE/RMS_FILE commands, each with the lines
// it wrote and the $STATUS shown next, if one is.
func analyzeCommands(text string) []analyzeCommand {
	var list []analyzeCommand

	on, status := false, false

	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, "$ ANALYZE/RMS_FILE"):
			list = append(list, analyzeCommand{command: line})
			on, status = true, false

			continue
		case strings.HasPrefix(line, "$ SHOW SYMBOL $STATUS") && on:
			status = true

			continue
		case strings.HasPrefix(line, "$"):
			on, status = false, false

			continue
		}

		if on {
			c := &list[len(list)-1]
			line = analyzeDates.ReplaceAllString(analyzeMasked(line), "${1}DATE")
			c.lines = append(c.lines, line)

			if status {
				on = false
			}
		}
	}

	return list
}

// expectedAnalyzeDifferences are the commands whose report isn't VMS's: files govax
// refuses to make (Prologs 1 and 2) and those made from them, and a directory, which
// ods2 extends by less than VMS does.
var expectedAnalyzeDifferences = map[string]bool{
	"[000000]R3.DIR": true,
	"C3A.IDX": true, "C3B.IDX": true, "C3C.IDX": true, "C93A.IDX": true, "C93B.IDX": true, "C93C.IDX": true,
}

// TestIdx88CAnalyzeOracle holds every ANALYZE/RMS_FILE command of round 3 to VMS's
// log: each organization and format, the four reports, their outputs and statuses,
// the damaged files of C12.
func TestIdx88CAnalyzeOracle(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88c.log"))
	if err != nil {
		t.Skipf("no VMS log: %v", err)
	}

	log, _ := runIdx88C(t)

	want, have := analyzeCommands(string(text)), analyzeCommands(log)
	if len(want) == 0 || len(have) != len(want) {
		t.Fatalf("%d commands on VMS, %d on govax", len(want), len(have))
	}

	for i, w := range want {
		g := have[i]
		if w.command != g.command {
			t.Fatalf("command %d is %q on VMS, %q on govax", i+1, w.command, g.command)
		}

		name := w.command[strings.LastIndex(w.command, " ")+1:]
		if expectedAnalyzeDifferences[name] {
			continue
		}

		if diff := firstDifference(strings.Join(w.lines, "\n"), strings.Join(g.lines, "\n")); diff != "" {
			t.Errorf("%s, %s", w.command, diff)
		}
	}
}

// TestAnalyzeRMSForms is what VMS's logs don't show: /OUTPUT's default names, by the
// manual, a host file, and qualifiers VMS has that govax doesn't.
func TestAnalyzeRMSForms(t *testing.T) {
	_, c := runIdx88C(t)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	host := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(host, []byte("a line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{
		"SET DEFAULT DUA1:[R3]",
		"ANALYZE/RMS_FILE/STATISTICS/OUTPUT=.TXT C31.IDX",
		"ANALYZE/RMS_FILE/OUTPUT=NAMED C31.IDX",
		"ANALYZE/RMS_FILE/FDL/OUTPUT=.XYZ C31.IDX",
		"ANALYZE/RMS_FILE/OUTPUT C31.IDX",
		`ANALYZE/RMS_FILE "` + host + `"/HOST`,
	} {
		if err := d.Dispatch(line); err != nil {
			t.Errorf("%s: %v", line, err)
		}
	}

	for _, name := range []string{"ANALYZE.TXT", "NAMED.ANL", "C31.XYZ"} {
		if _, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: name}); err != nil {
			t.Errorf("%s wasn't written: %v", name, err)
		}
	}

	if err := d.Dispatch("ANALYZE/RMS_FILE/INTERACTIVE C31.IDX"); err == nil {
		t.Error("/INTERACTIVE was accepted")
	}
}
