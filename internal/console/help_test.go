package console

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// vaxHelpPath returns the one true copy of vax.help --
// internal/bootdata/files/vax.help, the file cmd/govax's own resolver
// actually reads at runtime (main.go's `resolver.ReadFile("vax.help")`).
// A second, historical copy used to live at testdata/dcl/vax.help (a
// git-archive import from the upstream C repo, like testdata/dcl/evax.dcl);
// it was deleted so there is only one file to keep MOUNT/DISMOUNT's help
// text (and everything else) up to date in, rather than two copies that
// could silently drift apart -- see docs/PHASE-22.md.
func vaxHelpPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "bootdata", "files", "vax.help")
}

func TestLoadHelpFile_realFixture(t *testing.T) {
	h, err := LoadHelpFile(vaxHelpPath(t))
	if err != nil {
		t.Fatalf("LoadHelpFile: %v", err)
	}

	c, buf := newTestConsole(t)
	buf.Reset()
	
	if err := c.Help(h, []string{"SHOW", "LOGICAL"}); err != nil {
		t.Fatalf("Help: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "logical") && !strings.Contains(got, "Logical") {
		t.Errorf("output = %q, want it to mention logical names", got)
	}
}

func TestHelpKey_matchesDocumentedExample(t *testing.T) {
	// The file's own preamble: "HELP SET TRACE" resolves to "$SET ,TRAC".
	got := helpKey([]string{"SET", "TRACE"})
	if got != "SET ,TRAC" {
		t.Errorf("helpKey = %q, want \"SET ,TRAC\"", got)
	}
}

// TestLoadHelpFile_phase23Topics is docs/PHASE-23.md subtask 12's own
// regression check for the vax.help topics it added: every HELP command
// words combination an operator would actually type for this phase's new
// console commands (both the fully spelled-out verb/qualifier and, where
// this project's DCL grammar accepts one, its unambiguous abbreviation)
// resolves to real body text -- not "No help available for that topic",
// the easy failure mode for this file's own hand-maintained, exact-4-
// character-padded "$"-key format (see help.go's own helpKey doc comment).
func TestLoadHelpFile_phase23Topics(t *testing.T) {
	h, err := LoadHelpFile(vaxHelpPath(t))
	if err != nil {
		t.Fatalf("LoadHelpFile: %v", err)
	}

	cases := []struct {
		words []string
		want  string
	}{
		{[]string{"INITIALIZE"}, "required qualifier"},
		{[]string{"INIT"}, "required qualifier"},
		{[]string{"INITIALIZE", "/VAX"}, "INITIALIZE/VAX"},
		{[]string{"INITIALIZE", "/CONTAINER"}, "INITIALIZE/CONTAINER"},
		{[]string{"DIRECTORY"}, "DIRECTORY command lists files"},
		{[]string{"DIR"}, "DIRECTORY command lists files"},
		{[]string{"DELETE"}, "DELETE command removes a file"},
		{[]string{"DEL"}, "DELETE command removes a file"},
		{[]string{"PURGE"}, "PURGE command trims old versions"},
		{[]string{"TYPE"}, "TYPE command writes"},
		{[]string{"TYP"}, "TYPE command writes"},
		{[]string{"COPY"}, "COPY command copies a file"},
		{[]string{"SET", "DEFAULT"}, "SET DEFAULT command establishes"},
		{[]string{"SE", "DEFAULT"}, "SET DEFAULT command establishes"},
		{[]string{"SHOW", "DEFAULT"}, "SHOW DEFAULT command displays"},
		{[]string{"SH", "DEFAULT"}, "SHOW DEFAULT command displays"},
	}

	c, buf := newTestConsole(t)

	for _, tc := range cases {
		buf.Reset()

		if err := c.Help(h, tc.words); err != nil {
			t.Fatalf("Help(%v): %v", tc.words, err)
		}

		got := buf.String()
		if strings.Contains(got, "No help available") {
			t.Errorf("Help(%v) = %q, want real help text (got the \"no help\" fallback -- check this topic's $-key padding/spelling)", tc.words, got)

			continue
		}

		if !strings.Contains(got, tc.want) {
			t.Errorf("Help(%v) = %q, want it to contain %q", tc.words, got, tc.want)
		}
	}
}

// TestLoadHelpFile_everyKeyResolves is a whole-file regression check for
// the class of bug TestLoadHelpFile_phase23Topics originally caught: every
// "$"-prefixed key line in the real vax.help file, reconstructed into the
// HELP argument words an operator would actually type (each comma-
// separated component of the raw key, trimmed of any hand-typed padding),
// must resolve through Console.Help/helpKey back to real body text --
// never the "No help available" fallback. Before this phase's fix to
// normalizeHelpKey/normalizeHelpToken (help.go), roughly forty existing
// topics in this file -- every one whose last key component was a bare
// word under four characters (RUN, GO, DO, VM, ASM, PSL, XFC, SH, ST, EX,
// among others) -- silently failed this exact check, because ParseHelp's
// old plain strings.TrimSpace call discarded the very padding the file's
// own preamble documents as required. This test parses the file itself
// (independently of ParseHelp, deliberately -- it must not share that
// function's own normalization logic, or a bug reintroduced into both
// would go undetected) so it keeps checking real, independently-derived
// queries against whatever normalizeHelpKey and helpKey resolve to.
func TestLoadHelpFile_everyKeyResolves(t *testing.T) {
	h, err := LoadHelpFile(vaxHelpPath(t))
	if err != nil {
		t.Fatalf("LoadHelpFile: %v", err)
	}

	f, err := os.Open(vaxHelpPath(t))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	c, buf := newTestConsole(t)

	seen := map[string]bool{}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 4096), 1<<20)

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "$") {
			continue
		}

		raw := line[1:]
		if seen[raw] {
			continue
		}

		seen[raw] = true

		var words []string
		for _, tok := range strings.Split(raw, ",") {
			words = append(words, strings.TrimSpace(tok))
		}

		buf.Reset()

		if err := c.Help(h, words); err != nil {
			t.Fatalf("Help(%v) (from key %q): %v", words, raw, err)
		}

		if strings.Contains(buf.String(), "No help available") {
			t.Errorf("Help(%v) (from key %q) = %q, want real help text", words, raw, buf.String())
		}
	}

	if err := sc.Err(); err != nil {
		t.Fatalf("scanning vax.help: %v", err)
	}
}
