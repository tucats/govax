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
		{[]string{"SET", "DEFAULT"}, []string{"SET DEFAULT [device:][directory]"}},
		{[]string{"SE", "DEFAULT"}, []string{"SET DEFAULT [device:][directory]"}},
		{[]string{"SHOW", "DEFAULT"}, []string{"Shows the default device and directory"}},
		{[]string{"SET", "RADIX"}, []string{"HEXADECIMAL, DECIMAL, or OCTAL"}},
		{[]string{"SET", "DEBUG"}, []string{"LIBINIT", "USERSTEP"}},
		{[]string{"SHOW", "STRING_POOL"}, []string{"CLEAR STRINGS"}},
		{[]string{"SHOW", "MEMORY", "Qualifiers", "/FILES"}, []string{"PGFLQUOTA"}},
		{[]string{"HELP", "Qualifiers", "/USERLIBRARY"}, []string{"HLP$LIBRARY"}},
		{[]string{"INSTRUCTIONS"}, []string{"Ctrl/Z"}},
		{[]string{"HINTS", "Programs"}, []string{"FORTRAN"}},
		{[]string{"SPECIFY", "Host_files"}, []string{"/HOST"}},
		{[]string{"VMINIT", "Qualifiers", "/PAGING"}, []string{"vax.sysgen.paging.enable"}},
		{[]string{"LOAD", "Qualifiers", "/ERROR"}, []string{"/NOERROR"}},
		{[]string{"XFC"}, []string{"XFC$CONSOLE_PUT"}},
		{[]string{"SYSTEM_VOLUME", "Contents"}, []string{"HELPLIB.HLB", "BUILDHELP.COM"}},
		{[]string{"CONFIG"}, []string{"govax config set KEY=VALUE"}},
		{[]string{"CONFIGURATION"}, []string{"govax config set KEY=VALUE"}},
		{[]string{"CONFIG", "KEYS"}, []string{"vax.default.volume.file"}},
		{[]string{"CONFIG", "KEY"}, []string{"vax.default.volume.directory"}},
		{[]string{"RUN", "Shareable_images"}, []string{"SYS$SHARE:.EXE", "IMAGEFNF", "SHRIDMISMAT"}},
		{[]string{"RUN", "Description"}, []string{"%DCL-E-NOTFR"}},
		{[]string{"RUN", "Qualifiers", "/INIT"}, []string{"LIB$INITIALIZE"}},
		{[]string{"LINK", "Qualifiers", "/SHAREABLE"}, []string{"universal symbols"}},
		{[]string{"LINK", "Qualifiers", "/SYMBOL_TABLE"}, []string{"STB"}},
		{[]string{"LINK", "Options_file"}, []string{
			"UNIVERSAL=name", "GSMATCH=match,major,minor", "CLUSTER=name",
			"COLLECT=cluster", "PSECT_ATTR=psect", "BASE=address",
			"DZRO_MIN=pages", "ISD_MAX=count", "PROTECT=YES|NO",
			"IOSEGMENT=pages",
		}},
		{[]string{"LINK", "Shareable_images"}, []string{".TRANSFER"}},
		{[]string{"LINK", "Description"}, []string{"--shareable", "--symbol-table"}},
		{[]string{"MACRO", "Qualifiers", "/SHOW"}, []string{"EXPANSIONS (ME)"}},
		{[]string{"ANALYZE", "/RMS_FILE", "Qualifiers", "/FDL"}, []string{"CREATE/FDL"}},
		{[]string{"ANALYZE", "/OBJECT"}, []string{"ANALYZE/OBJECT filespec"}},
		{[]string{"FORTRAN", "FTNRTL"}, []string{"BUILDFTN.COM"}},
		{[]string{"FORTRAN", "Qualifiers", "/RECURSIVE"}, []string{"stack frame"}},
		{[]string{"LIBRARY", "Qualifiers", "/HELP"}, []string{"help library"}},
		{[]string{"LIBRARY"}, []string{"shareable image libraries", "help libraries"}},
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
		{[]string{"SPAWN", "/WAIT"}, []string{"beside the console"}},
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

// TestEveryQualifierHasHelp: each qualifier a verb's grammar takes (its
// syntaxes' too) has a qualifier line ("/NAME" in column 1, under a
// Qualifiers topic) or a "/NAME" key somewhere in the verb's module
// (Decision 10; Phase 90's subtask 8 rewrote every verb's help so).
func TestEveryQualifierHasHelp(t *testing.T) {
	lib := consoletest.HelpLibrary(t, bootdata.HelpLibrary)
	g := consoletest.ConsoleGrammar(t)

	names, alias := g.Verbs()

	var missing []string

	for i, verb := range names {
		if alias[i] {
			continue
		}

		keys := qualifierKeys(lib, verb)

		for _, q := range g.Qualifiers(verb) {
			if !qualifierKeyed(keys, q) {
				missing = append(missing, verb+"/"+q)
			}
		}
	}

	sort.Strings(missing)

	for _, m := range missing {
		t.Errorf("%s has no qualifier line in its verb's help", m)
	}
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
