package console_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/machine"
)

// docs/PHASE-70 - traceback.md, subtask 10: the oracle. TestTB70Oracle
// does on govax what the author did on VMS V7.1 (testdata/tb70's
// README): it puts the probe's sources on a fresh volume as DUA1, runs
// @TB70/OUTPUT=TB70.LOG and @TB70B/OUTPUT=TB70B.LOG, which assemble and
// link every image with govax's MACRO and LINK and run every part under
// each, and holds each log to VMS's, command by command. The tests in
// tb70_test.go run VMS's own images; this one runs govax's, built from
// the sources, through the procedures' own DCL.

// tb70Sources are, by procedure, the files the exchange volume got for
// its round (exchange.cmd, then exchange2.cmd).
var tb70Sources = map[string][]string{
	"TB70":  {"tb70.mar", "tb70long.mar", "tb70shr.mar", "tb70.com"},
	"TB70B": {"tb70b.mar", "tb70b.com"},
}

// tb70LogDifferences are the commands whose output in VMS's log govax's
// needn't match, by the command line as the log echoes it (for a RUN,
// with the part after it), and why. The procedures' other commands must
// match: outside an "improperly handled condition" dump line for line,
// and in one by excmsgMatch's rules, with tb70Masked's for PCs outside
// the image; DIRECTORY's listings with tb70Directory's masks.
var tb70LogDifferences = map[string]string{
	`$ WRITE SYS$OUTPUT F$GETSYI("VERSION"), " ", F$GETSYI("HW_NAME")`: "govax is V7.3, and names itself",
}

// tb70SizeDifferences are the files whose sizes in DIRECTORY/SIZE=ALL
// govax's needn't match, and why.
var tb70SizeDifferences = map[string]string{
	"000000.DIR;1": "VMS's directory has a block more, from the volume's history",
	"INDEXF.SYS;1": "VMS's index file had been extended",
	// VMS's header has no end of file while the procedure has its log
	// open, and DCL had allocated 224 blocks; govax writes the end of
	// file through as it goes.
	"TB70.LOG;1":  "the open log's end of file and allocation",
	"TB70B.LOG;1": "the open log's end of file and allocation",
	// The maps name govax's LINK and its own LIBRTL, a shorter map.
	"TB70.MAP;1":    "LINK's maps",
	"TB70NT.MAP;1":  "LINK's maps",
	"TB70D.MAP;1":   "LINK's maps",
	"TB70SHR.MAP;1": "LINK's maps",
	"TB70B.MAP;1":   "LINK's maps",
	"TB70BNT.MAP;1": "LINK's maps",
}

// tb70DirectoryTime is DIRECTORY/DATE's time.
var tb70DirectoryTime = regexp.MustCompile(`\d\d-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d\.\d\d`)

// tb70DirectoryTotal is DIRECTORY's total's blocks.
var tb70DirectoryTotal = regexp.MustCompile(`, \d+/\d+ blocks\.$`)

// tb70Directory masks a DIRECTORY listing's lines: the times, the
// sizes of tb70SizeDifferences, and so the total's blocks.
func tb70Directory(lines []string) []string {
	out := make([]string, len(lines))

	for i, line := range lines {
		line = tb70DirectoryTime.ReplaceAllString(line, "<time>")
		line = tb70DirectoryTotal.ReplaceAllString(line, ", <blocks>.")

		if fields := strings.Fields(line); len(fields) > 1 && tb70SizeDifferences[fields[0]] != "" {
			line = fields[0] + " <size> " + strings.Join(fields[2:], " ")
		}

		out[i] = line
	}

	return out
}

// tb70Log is a log's commands (tb70SplitLog), each the line DCL echoed
// and its output, blank lines and all.
type tb70Log []tb70Command

// tb70SplitLog splits a procedure's log into its commands: each line
// that starts with "$ " (SET VERIFY's echo) begins one.
func tb70SplitLog(text string) tb70Log {
	var log tb70Log

	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r", ""), "\n"), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "$ ") || len(log) == 0 {
			log = append(log, tb70Command{line: line})

			continue
		}

		log[len(log)-1].want = append(log[len(log)-1].want, line)
	}

	return log
}

// tb70Define is a DEFINE of the probe's part, in either round.
var tb70Define = regexp.MustCompile(`^\$ +DEFINE(/NOLOG)? TB70B?_PART (\S+)$`)

// tb70Run is a RUN of one of the probe's images.
var tb70Run = regexp.MustCompile(`^\$ +RUN(/NODEBUG)? (TB70\S*)$`)

// tb70SourceConsole is a console with a fresh RD51 as DUA1, the
// default directory, as the exchange volume was made.
func tb70SourceConsole(t *testing.T) func(string) string {
	t.Helper()

	_, dispatch := wsDispatcher(t)
	dsk := filepath.Join(t.TempDir(), "tb70.dsk")

	dispatch(`INITIALIZE/CONTAINER "` + dsk + `" /DEVICE=RD51 TB70`)
	dispatch(`MOUNT/WRITE DUA1 "` + dsk + `"`)
	dispatch("SET DEFAULT DUA1:[000000]")

	return dispatch
}

// tb70CopySources copies the files of procedure's round to DUA1's
// [000000].
func tb70CopySources(t *testing.T, dispatch func(string) string, procedure string) {
	t.Helper()

	for _, name := range tb70Sources[procedure] {
		src, err := filepath.Abs(filepath.Join(tb70Dir, name))
		if err != nil {
			t.Fatal(err)
		}

		dispatch(`COPY "` + src + `"/HOST DUA1:[000000]` + strings.ToUpper(name))
	}
}

// TestTB70Oracle runs TB70.COM and TB70B.COM on govax, with paging off
// (the copying loader) and on (the mapping one), and holds their logs
// to VMS's.
func TestTB70Oracle(t *testing.T) {
	for _, paging := range []string{"false", "true"} {
		t.Run("paging="+paging, func(t *testing.T) {
			setSetting(t, machine.PagingSetting, paging)
			tb70Oracle(t)
		})
	}
}

// tb70Oracle is TestTB70Oracle in one paging mode.
func tb70Oracle(t *testing.T) {
	dispatch := tb70SourceConsole(t)
	dir := t.TempDir()

	for _, name := range []string{"TB70", "TB70B"} {
		tb70CopySources(t, dispatch, name)

		if out := dispatch("@" + name + "/OUTPUT=" + name + ".LOG"); out != "" {
			t.Errorf("@%s wrote to the terminal: %q", name, out)
		}

		host := filepath.Join(dir, strings.ToLower(name)+".log")
		dispatch("COPY DUA1:[000000]" + name + `.LOG "` + host + `"/HOST/QUIET`)

		got, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		want, err := os.ReadFile(filepath.Join(tb70Dir, "vax", strings.ToLower(name)+".log"))
		if err != nil {
			t.Fatal(err)
		}

		for _, problem := range tb70CompareLogs(tb70SplitLog(string(got)), tb70SplitLog(string(want))) {
			t.Errorf("%s.LOG: %s", name, problem)
		}
	}
}

// tb70CompareLogs compares govax's log, got, with VMS's, want, command
// by command, and returns what differs. The part a RUN runs is the last
// one the log defined; a run in tb70DumpDifferences (but X, whose dump
// excmsgMatch compares) isn't compared, nor are tb70LogDifferences.
func tb70CompareLogs(got, want tb70Log) []string {
	if len(got) != len(want) {
		return []string{fmt.Sprintf("%d commands, want %d", len(got), len(want))}
	}

	var problems []string

	part := ""

	for i, w := range want {
		g := got[i]

		if g.line != w.line {
			return append(problems, fmt.Sprintf("command %d is %q, want %q: the logs part", i+1, g.line, w.line))
		}

		if m := tb70Define.FindStringSubmatch(w.line); m != nil {
			part = m[2]
		}

		if _, ok := tb70LogDifferences[w.line]; ok {
			continue
		}

		gotLines, wantLines := g.want, w.want
		if strings.HasPrefix(w.line, "$ DIRECTORY") {
			gotLines, wantLines = tb70Directory(gotLines), tb70Directory(wantLines)
		}

		loose, unset, what := false, []string(nil), w.line

		if m := tb70Run.FindStringSubmatch(w.line); m != nil {
			what = fmt.Sprintf("%s (part %s)", w.line, part)

			if _, ok := tb70DumpDifferences[part]; ok && part != "X" {
				continue
			}

			loose, unset = excmsgLoose[part] != "", excmsgUnset[m[2]+" "+part]
		}

		for _, problem := range excmsgMatch(tb70Masked(gotLines), tb70Masked(wantLines), loose, unset) {
			problems = append(problems, fmt.Sprintf("%s: %s\ngot:\n%s\nwant:\n%s", what, problem, strings.Join(g.want, "\n"), strings.Join(w.want, "\n")))
		}
	}

	return problems
}
