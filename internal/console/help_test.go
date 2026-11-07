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
		{[]string{"INITIALIZE"}, []string{"chosen by its qualifier, which must be given"}},
		{[]string{"INIT"}, []string{"chosen by its qualifier, which must be given"}},
		{[]string{"INITIALIZE", "/VAX"}, []string{"INITIALIZE/VAX pages"}},
		{[]string{"INITIALIZE", "/CONTAINER"}, []string{"INITIALIZE/CONTAINER container-file"}},
		{[]string{"DIRECTORY"}, []string{"Lists files on a mounted volume"}},
		{[]string{"DIR"}, []string{"Lists files on a mounted volume"}},
		{[]string{"DIRECTORY", "/SIZE"}, []string{"USED (the default), ALLOCATION"}},
		{[]string{"DIR", "Qualifiers", "/VERS"}, []string{"newest versions"}},
		{[]string{"DELETE"}, []string{"Deletes files from a mounted volume"}},
		{[]string{"DELETE", "/SYMBOL"}, []string{"DELETE/SYMBOL [symbol-name]"}},
		{[]string{"PURGE"}, []string{"all but the newest versions"}},
		{[]string{"TYPE"}, []string{"Writes a file to the terminal"}},
		{[]string{"COPY"}, []string{"COPY input-filespec output-filespec"}},
		{[]string{"COPY", "/RECORD_FORMAT"}, []string{"STREAM_LF", "govax's own"}},
		{[]string{"CREATE", "/DIRECTORY", "Qualifiers", "/OWNER"}, []string{"PARENT"}},
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
		{[]string{"SYMBOL"}, []string{"symbol-name :=[=] string"}},
		{[]string{"SYMBOL", "EXPRESSIONS"}, []string{".EQS. .NES."}},
		{[]string{"Symbol_Assign", "Substitution"}, []string{"&NAME"}},
		{[]string{"="}, []string{"symbol-name =[=] expression"}},
		{[]string{":="}, []string{"symbol-name :=[=] string"}},
		{[]string{"LEXICAL"}, []string{"F$name([argument[,...]])"}},
		{[]string{"LEXICALS", "F$TRNLNM"}, []string{"LNM$DCL_LOGICAL"}},
		{[]string{"LEX", "F$CONF"}, []string{"govax's own"}},
		{[]string{"DEFINE", "/DEVICE"}, []string{"DEFINE/DEVICE device-name"}},
		{[]string{"DEFINE", "/DEVICE", "Qualifiers", "/DEVCLASS"}, []string{"MAILBOX"}},
		{[]string{"READ", "/KEY"}, []string{"shorter than the key"}},
		{[]string{"SPAWN", "/[NO]WAIT"}, []string{"beside the console"}},
		{[]string{"@"}, []string{"@filespec [p1 [p2 ... p8]]"}},
		{[]string{"IF"}, []string{"IF expression THEN [$] command"}},
		{[]string{"IF", "Description"}, []string{"INVIFNEST"}},
		{[]string{"STOP", "/IMAGE"}, []string{"FORCEDEXIT"}},
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
	"HELP/*":          "the old help predates HELP's VMS qualifiers (Phase 90, subtask 6)",
	"RUN/INIT":        "the old help never described it",
	"SHOW/RUNTIME":    "the old help never described it",
	"SHOW/STATISTICS": "the old help never described it",
	"VMINIT/DEBUG":    "the old help never described it",
	"VMINIT/LOG":      "the old help never described it",
	"VMINIT/VERIFY":   "the old help never described it",
}

// rewrittenHelp are the verbs whose help subtask 8 has rewritten in VMS's
// shape (docs/PHASE-90 - VMS help.md): each of their qualifiers must be a
// qualifier line ("/NAME" in column 1, under a Qualifiers topic) or a
// "/NAME" key somewhere in the verb's module. When every verb is here,
// the looser check by text and qualifierHelpExceptions go.
var rewrittenHelp = map[string]bool{
	// Subtask 8a: the commands on files and volumes.
	"BACKUP": true, "CONVERT": true, "COPY": true, "CREATE": true,
	"DELETE": true, "DIRECTORY": true, "DISMOUNT": true, "DUMP": true,
	"EDIT": true, "INITIALIZE": true, "MOUNT": true, "PURGE": true,
	"RENAME": true, "TYPE": true,

	// Subtask 8b: DCL itself (logical names, procedures and their flow,
	// file I/O, processes).
	"ASSIGN": true, "ATTACH": true, "CALL": true, "CLOSE": true,
	"CONTINUE": true, "DEASSIGN": true, "DECK": true, "DEFINE": true,
	"ELSE": true, "ENDIF": true, "ENDSUBROUTINE": true, "EOD": true,
	"EXIT": true, "GOSUB": true, "GOTO": true, "IF": true, "INQUIRE": true,
	"LOGOUT": true, "MCR": true, "ON": true, "OPEN": true, "READ": true,
	"RETURN": true, "SPAWN": true, "STOP": true, "SUBROUTINE": true,
	"THEN": true, "WAIT": true, "WRITE": true,
}

// TestEveryQualifierHasHelp: each qualifier a verb's grammar takes (its
// syntaxes' too) is named somewhere in the verb's topic or below it, as
// "/NAME" or "/[NO]NAME" (Decision 10), but for qualifierHelpExceptions;
// a verb of rewrittenHelp's must have a qualifier line or key for it.
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
		keys := qualifierKeys(lib, verb)

		for _, q := range g.Qualifiers(verb) {
			if rewrittenHelp[verb] {
				if !qualifierKeyed(keys, q) {
					missing = append(missing, verb+"/"+q+" (no qualifier line)")
				}

				continue
			}

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

// qualifierKeys is every qualifier line's key and every "/" key in the
// module named name (as a word abbreviating it finds it), upper-cased.
func qualifierKeys(lib help.Library, name string) []string {
	var keys []string

	var walk func(t *help.Topic)

	walk = func(t *help.Topic) {
		if strings.HasPrefix(t.Key, "/") {
			keys = append(keys, strings.ToUpper(t.Key))
		}

		for _, s := range t.Slashes {
			walk(s)
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

	return keys
}

// qualifierKeyed reports whether one of keys, from qualifierKeys, is the
// qualifier q's: "/Q", "/Q=value", "/Q[=value]", "/[NO]Q", or "/NOQ" for
// a negatable Q (CONVERT's "/[NO]CREATE" is CREATE's, and "/NOQ" names Q).
func qualifierKeyed(keys []string, q string) bool {
	for _, k := range keys {
		name := strings.TrimPrefix(k, "/")
		negatable := strings.HasPrefix(name, "[NO]")
		name = strings.TrimPrefix(name, "[NO]")

		if end := strings.IndexFunc(name, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '$')
		}); end >= 0 {
			name = name[:end]
		}

		if name == q || negatable && "NO"+name == q || name == "NO"+q {
			return true
		}
	}

	return false
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

		if !strings.Contains(buf.String(), "DELETE/SYMBOL [symbol-name]") {
			t.Errorf("HELP %s = %q, want the DELETE/SYMBOL topic", arg, buf.String())
		}
	}
}
