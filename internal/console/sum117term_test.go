package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// sum117TerminalSkipped are the terminal log's commands the terminal oracle
// doesn't replay, and why.
var sum117TerminalSkipped = map[string]string{
	"SET NOON":            "the procedure's own bookkeeping",
	"SET VERIFY":          "the procedure's own bookkeeping",
	"SET NOVERIFY":        "the procedure's own bookkeeping",
	"EDIT/SUM/OUTPUT=TT:": "a missing parameter is DCL's INSFPRM, which the dispatcher's caller shows",
	"EDIT/SUM/OUTPUT=TT: S10.SRC/UPDATE=SYS$COMMAND": "VMS's SUMSLP reads an update file twice, which a terminal can't give it " +
		"(READERR, RAC); govax reads it once (Decision 8)",
}

// sum117ListingLine is a line of a SUMSLP listing as a terminal shows it,
// its times masked: a heading, or the 16-column field and a line number.
var sum117ListingLine = regexp.MustCompile(`^( {60}dd-MMM-yyyy |.{16}[ .0-9]{6}\d )`)

// TestSum117TerminalOracle replays SUM117T.COM's session at a terminal
// (testdata/sum117/vax/terminal.log, Phase 117's subtask 9): the output and
// the listing at the terminal (/OUTPUT=TT:, /LISTING=TT:, and
// /LISTING=SYS$OUTPUT), a missing parameter, and a syntax error's messages.
// Blank lines are left out (VMS's terminal showed a listing's form feed as
// blank lines), and so are the headings' times. When the output and the
// listing both go to the terminal, VMS's SUMSLP writes each output record
// before its listing line and govax writes the output and then the listing
// (DEVIATIONS.md), so such a command's lines are compared as the output's
// and the listing's apart.
func TestSum117TerminalOracle(t *testing.T) {
	t.Parallel()

	text, err := os.ReadFile(filepath.Join(sum117Dir(t), "vax", "terminal.log"))
	if err != nil {
		t.Fatal(err)
	}

	type exchange struct {
		command string
		output  []string
	}

	var session []*exchange

	for _, line := range strings.Split(strings.ReplaceAll(string(text), "\r", ""), "\n") {
		if rest, ok := strings.CutPrefix(line, "$ "); ok {
			session = append(session, &exchange{command: strings.TrimSpace(rest)})

			continue
		}

		if len(session) > 0 {
			last := session[len(session)-1]
			last.output = append(last.output, line)
		}
	}

	d, _, buf := sum117Volume(t)
	shareFindRun(t, d, "SET DEFAULT SUM$TOP:[SUM117]")

	ran, skipped := 0, false

	for _, x := range session {
		if x.command == "" || strings.HasPrefix(x.command, "!") || strings.HasPrefix(x.command, "@") {
			continue
		}

		// A skipped command's SHOW SYMBOL $STATUS goes with it.
		if _, skip := sum117TerminalSkipped[x.command]; skip || skipped && x.command == "SHOW SYMBOL $STATUS" {
			skipped = skip

			continue
		}

		skipped = false

		buf.Reset()

		if err := d.Dispatch(x.command); err != nil && !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: %v", x.command, err)

			continue
		}

		ran++

		got := sum117TerminalLines(strings.Split(buf.String(), "\n"))
		want := sum117TerminalLines(x.output)

		// Both to the terminal: VMS's lines interleaved, govax's not.
		if strings.Contains(x.command, "/OUTPUT=TT:") && strings.Contains(x.command, "/LISTING=TT:") {
			want = sum117Apart(want)
		}

		if a, b := strings.Join(got, "\n"), strings.Join(want, "\n"); a != b {
			t.Errorf("%s: the output differs\ngovax:\n%s\nVMS:\n%s", x.command, a, b)
		}
	}

	if ran < 6 {
		t.Errorf("only %d commands replayed", ran)
	}
}

// sum117TerminalLines are a terminal's lines without blank ones, form
// feeds, trailing blanks, or the headings' times.
func sum117TerminalLines(lines []string) []string {
	var out []string

	for _, l := range lines {
		l = strings.TrimRight(strings.ReplaceAll(l, "\f", ""), " ")
		if l == "" {
			continue
		}

		out = append(out, sum117Times.ReplaceAllString(l, "dd-MMM-yyyy hh:mm:ss"))
	}

	return out
}

// sum117Apart is a terminal's lines with the output records first and the
// listing's lines after them, each in its order.
func sum117Apart(lines []string) []string {
	var output, listing []string

	for _, l := range lines {
		if sum117ListingLine.MatchString(l) {
			listing = append(listing, l)
		} else {
			output = append(output, l)
		}
	}

	return append(output, listing...)
}
