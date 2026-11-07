package console_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/help"
)

// This file holds govax's DCL help library (HELPLIB.HLB, built from
// internal/bootdata/files/help) to the console's grammar (docs/PHASE-90 -
// VMS help.md, Decision 10), and checks that topics an operator asks for
// are there.

// TestHelpTopics: HELP finds each of these topics, by the whole words or
// their abbreviations, and its text says what's expected.
func TestHelpTopics(t *testing.T) {
	lib := consoletest.HelpLibrary(t, bootdata.HelpLibrary)

	cases := []struct {
		words []string
		want  []string
	}{
		{[]string{"SHOW", "LOGICAL"}, []string{"logical names"}},
		{[]string{"SH", "LOGI"}, []string{"logical names"}},
		{[]string{"INITIALIZE"}, []string{"required qualifier"}},
		{[]string{"INIT"}, []string{"required qualifier"}},
		{[]string{"INITIALIZE", "/VAX"}, []string{"INITIALIZE/VAX"}},
		{[]string{"INITIALIZE", "/CONTAINER"}, []string{"INITIALIZE/CONTAINER"}},
		{[]string{"DIRECTORY"}, []string{"DIRECTORY command lists files"}},
		{[]string{"DIR"}, []string{"DIRECTORY command lists files"}},
		{[]string{"DELETE"}, []string{"DELETE command removes a file"}},
		{[]string{"DELETE", "/SYMBOL"}, []string{"DELETE/SYMBOL [/LOCAL | /GLOBAL] /ALL"}},
		{[]string{"PURGE"}, []string{"PURGE command trims old versions"}},
		{[]string{"TYPE"}, []string{"TYPE command writes"}},
		{[]string{"COPY"}, []string{"COPY command copies a file"}},
		{[]string{"SET", "DEFAULT"}, []string{"SET DEFAULT command establishes"}},
		{[]string{"SE", "DEFAULT"}, []string{"SET DEFAULT command establishes"}},
		{[]string{"SHOW", "DEFAULT"}, []string{"SHOW DEFAULT command displays"}},
		{[]string{"CONFIG"}, []string{"govax config set KEY=VALUE"}},
		{[]string{"CONFIGURATION"}, []string{"govax config set KEY=VALUE"}},
		{[]string{"CONFIG", "KEYS"}, []string{"vax.default.volume.file"}},
		{[]string{"CONFIG", "KEY"}, []string{"vax.default.volume.directory"}},
		{[]string{"RUN"}, []string{"SYS$SHARE:.EXE", "IMAGEFNF", "SHRIDMISMAT", "%DCL-E-NOTFR"}},
		{[]string{"LINK"}, []string{
			"/SHAREABLE[=<image>]", "/SYMBOL_TABLE[=<file>]", "/FULL",
			"UNIVERSAL=<name>", "GSMATCH=<match>,<major>,<minor>", "CLUSTER=<name>",
			"COLLECT=<cluster>", "PSECT_ATTR=<psect>", "BASE=<address>",
			"DZRO_MIN=<pages>", "ISD_MAX=<count>", "PROTECT=YES|NO",
			"IOSEGMENT=<pages>", ".TRANSFER", "--shareable", "--symbol-table",
		}},
		{[]string{"LIBRARY"}, []string{"/SHARE", "/HELP"}},
	}

	for _, tc := range cases {
		got, found := consoletest.HelpText(t, lib, tc.words...)
		if !found {
			t.Errorf("HELP %s: no topic", strings.Join(tc.words, " "))

			continue
		}

		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("HELP %s doesn't mention %q", strings.Join(tc.words, " "), want)
			}
		}
	}
}

// TestEveryVerbHasHelp: each verb of the console's grammar has a level-1
// topic of HELPLIB.HLB (Decision 10). An alias shares its verb's topic.
func TestEveryVerbHasHelp(t *testing.T) {
	lib := consoletest.HelpLibrary(t, bootdata.HelpLibrary)

	names, alias := consoletest.ConsoleGrammar(t).Verbs()
	for i, verb := range names {
		if alias[i] {
			continue
		}

		if _, found := consoletest.HelpText(t, lib, verb); !found {
			t.Errorf("no help topic for the verb %s", verb)
		}
	}
}

// qualifierHelpExceptions are the console's qualifiers whose verb's topic
// doesn't name them yet, each with the reason: "VERB/QUALIFIER", or
// "VERB/*" for all of a verb's. The help is still the old help's text
// converted (Phase 90's subtask 7); subtask 8 rewrites it with a "/" line
// for every qualifier, and empties this list.
var qualifierHelpExceptions = map[string]string{
	"ASSIGN/*":        "its topic says it takes DEFINE's qualifiers, which DEFINE's topic describes",
	"DEASSIGN/LOG":    "the old help never described it",
	"DEFINE/*":        "DEFINE/DEVICE's pseudodevice qualifiers were never described",
	"HELP/*":          "the old help predates HELP's VMS qualifiers (Phase 90, subtask 6)",
	"OPEN/HOST":       "the old help never described it",
	"RUN/INIT":        "the old help never described it",
	"SHOW/RUNTIME":    "the old help never described it",
	"SHOW/STATISTICS": "the old help never described it",
	"VMINIT/DEBUG":    "the old help never described it",
	"VMINIT/LOG":      "the old help never described it",
	"VMINIT/VERIFY":   "the old help never described it",
}

// TestEveryQualifierHasHelp: each qualifier a verb's grammar takes (its
// syntaxes' too) is named somewhere in the verb's topic or below it, as
// "/NAME" or "/[NO]NAME" (Decision 10), but for qualifierHelpExceptions.
func TestEveryQualifierHasHelp(t *testing.T) {
	lib := consoletest.HelpLibrary(t, bootdata.HelpLibrary)
	g := consoletest.ConsoleGrammar(t)

	names, alias := g.Verbs()

	var missing []string

	used := map[string]bool{}

	for i, verb := range names {
		if alias[i] {
			continue
		}

		text := strings.ToUpper(topicText(lib, verb))

		for _, q := range g.Qualifiers(verb) {
			if qualifierNamed(text, q) {
				continue
			}

			key := verb + "/" + q

			switch {
			case qualifierHelpExceptions[key] != "":
				used[key] = true
			case qualifierHelpExceptions[verb+"/*"] != "":
				used[verb+"/*"] = true
			default:
				missing = append(missing, key)
			}
		}
	}

	sort.Strings(missing)

	for _, m := range missing {
		t.Errorf("%s isn't in its verb's help", m)
	}

	for key := range qualifierHelpExceptions {
		if !used[key] {
			t.Errorf("qualifierHelpExceptions' %s is no longer needed", key)
		}
	}
}

// qualifierNamed reports whether text, upper-cased help, names the
// qualifier q: "/Q", "/NOQ", or "/[NO]Q".
func qualifierNamed(text, q string) bool {
	for _, form := range []string{"/" + q, "/NO" + q, "/[NO]" + q} {
		if strings.Contains(text, form) {
			return true
		}
	}

	return strings.HasPrefix(q, "NO") && strings.Contains(text, "/[NO]"+q[2:])
}

// topicText is every key and line of text in the module named name (as
// a word abbreviating it finds it) and below.
func topicText(lib help.Library, name string) string {
	var b strings.Builder

	var walk func(t *help.Topic)

	walk = func(t *help.Topic) {
		b.WriteString(t.Key + "\n")

		for _, l := range t.Text {
			b.WriteString(l + "\n")
		}

		for _, s := range t.Subtopics {
			walk(s)
		}
	}

	for _, n := range lib.Names() {
		if strings.HasPrefix(strings.ToUpper(n), strings.ToUpper(name)) {
			if t := lib.Module(n); t != nil {
				walk(t)
			}
		}
	}

	return b.String()
}

// TestCmdHelp_qualifierWithoutSpace: HELP splits a qualifier off the word
// it's attached to, as VMS's HELP does: HELP DELETE/SYMBOL is HELP DELETE
// /SYMBOL.
func TestCmdHelp_qualifierWithoutSpace(t *testing.T) {
	var buf strings.Builder

	c := console.New(&buf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t))

	for _, arg := range []string{"DELETE/SYMBOL", "DELETE /SYMBOL", "DEL/SYM"} {
		buf.Reset()

		if err := d.Dispatch("HELP/NOPROMPT " + arg); err != nil {
			t.Fatalf("HELP %s: %v", arg, err)
		}

		if !strings.Contains(buf.String(), "DELETE/SYMBOL [/LOCAL | /GLOBAL] /ALL") {
			t.Errorf("HELP %s = %q, want the DELETE/SYMBOL topic", arg, buf.String())
		}
	}
}
