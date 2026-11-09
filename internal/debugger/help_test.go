package debugger_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/help"
)

// debugHelp is the debugger's help library, as govax builds it in.
func debugHelp(t *testing.T) help.Library {
	return consoletest.HelpLibrary(t, bootdata.DebugHelpLibrary)
}

// found reports whether the debugger's HELP of words finds a topic: the
// words looked up below the library's DEBUG module, as the debugger looks
// them up.
func found(t *testing.T, lib help.Library, words ...string) bool {
	t.Helper()

	_, ok := consoletest.HelpText(t, lib, append([]string{"DEBUG"}, words...)...)

	return ok
}

// TestEveryVerbHasHelp: each verb of the debugger's grammar has a topic
// under DBG$HELP.HLB's DEBUG module (docs/PHASE-90 - VMS help.md, Decision
// 10; the console's are internal/console's help_test.go). An alias, such
// as S for STEP, shares its verb's topic.
func TestEveryVerbHasHelp(t *testing.T) {
	lib := debugHelp(t)

	names, alias := consoletest.DebugGrammar(t).Verbs()
	for i, verb := range names {
		if alias[i] {
			continue
		}

		if !found(t, lib, verb) {
			t.Errorf("no help topic for the debugger's verb %s", verb)
		}
	}
}

// TestEveryQualifierHasHelp: each qualifier a debugger verb's grammar
// takes (its syntaxes' too) has a qualifier line ("/NAME" in column 1,
// under a Qualifiers topic) or a "/NAME" key somewhere in the verb's
// topic under DEBUG (Decision 10, as the console's test in
// internal/console's help_test.go holds DCL's verbs; subtask 9 rewrote the
// debugger's help so).
func TestEveryQualifierHasHelp(t *testing.T) {
	lib := debugHelp(t)
	g := consoletest.DebugGrammar(t)
	debug := lib.Module("DEBUG")

	names, alias := g.Verbs()

	var missing []string

	for i, verb := range names {
		if alias[i] {
			continue
		}

		var topic *help.Topic

		for _, s := range debug.Subtopics {
			if strings.HasPrefix(strings.ToUpper(s.Key), verb) {
				topic = s

				break
			}
		}

		keys := map[string]bool{}

		var walk func(t *help.Topic)

		walk = func(t *help.Topic) {
			if strings.HasPrefix(t.Key, "/") {
				keys[strings.ToUpper(t.Key[1:])] = true
			}

			for _, s := range t.Slashes {
				walk(s)
			}

			for _, s := range t.Subtopics {
				walk(s)
			}
		}

		if topic != nil {
			walk(topic)
		}

		for _, q := range g.Qualifiers(verb) {
			if !keys[q] {
				missing = append(missing, verb+"/"+q)
			}
		}
	}

	sort.Strings(missing)

	for _, m := range missing {
		t.Errorf("%s has no qualifier line in its verb's help", m)
	}
}

// TestDebuggerHelpTopics: the debugger's own help covers each SET, SHOW,
// and CANCEL keyword and the qualifiers that need explaining, so a HELP of
// each says something.
func TestDebuggerHelpTopics(t *testing.T) {
	lib := debugHelp(t)

	topics := [][]string{
		{"@"}, {"ASM"}, {"ASM", "LABELS"}, {"ASM", "ADDRESSING"}, {"ASM", "DIRECTIVES", "QUAD"},
		{"ASM", "DIRECTIVES", "BYTE"}, {"ASM", "DIR", "IF_FALSE"}, {"ASM", "DIRECTIVES", "SHIM"},
		{"SET", "BREAK"}, {"SET", "TRACE"}, {"SET", "WATCH"}, {"SET", "STEP"},
		{"SET", "SOURCE"}, {"SET", "MODE"}, {"SET", "RADIX"}, {"SET", "MODULE"},
		{"SET", "PSL"}, {"SET", "PTE"}, {"SET", "FAULT"}, {"SET", "VM"}, {"SET", "BASE"},
		{"SHOW", "BREAK"}, {"SHOW", "TRACE"}, {"SHOW", "WATCH"}, {"SHOW", "STEP"},
		{"SHOW", "SOURCE"}, {"SHOW", "CALLS"}, {"SHOW", "STACK"}, {"SHOW", "KSP"},
		{"SHOW", "SP"}, {"SHOW", "REGISTERS"}, {"SHOW", "REGIONS"}, {"SHOW", "PSL"},
		{"SHOW", "CPU_STATUS"}, {"SHOW", "CLOCK"}, {"SHOW", "BASE"}, {"SHOW", "MEMORY"},
{"SHOW", "TB"}, {"SHOW", "PAGE"}, {"SHOW", "SCB"},
		{"SHOW", "SHIM"}, {"SHOW", "EXCEPTIONS"}, {"SHOW", "MODE"}, {"SHOW", "RADIX"},
		{"SHOW", "IMAGE"}, {"SHOW", "MODULE"}, {"SHOW", "SYMBOL"}, {"SHOW", "SCOPE"},
		{"SHOW", "LANGUAGE"}, {"SET", "TYPE"}, {"SHOW", "TYPE"}, {"CANCEL", "TYPE"},
		{"EXAMINE", "/FLOAT"}, {"EXAMINE", "/PACKED"}, {"EXAMINE", "/DATE_TIME"}, {"EXAMINE", "/TYPE"},
		{"CANCEL", "BREAK"}, {"CANCEL", "TRACE"}, {"CANCEL", "WATCH"}, {"CANCEL", "SOURCE"},
		{"CANCEL", "RADIX"}, {"CANCEL", "MODE"}, {"CANCEL", "INTERRUPT"}, {"CANCEL", "TB"},
		{"CANCEL", "STRINGS"}, {"CLEAR"}, {"SHOW", "STRING_POOL"}, {"PSL"},
		{"EXAMINE", "/INSTRUCTION"}, {"EXAMINE", "/OPERANDS"}, {"EXAMINE", "/BYTE"},
		{"EXAMINE", "/ASCII"}, {"EXAMINE", "/HEXADECIMAL"}, {"EXAMINE", "/PSL"},
		{"EXAMINE", "/PTE"}, {"EXAMINE", "/CONSTANTS"}, {"EXAMINE", "/SHAREABLE"},
		{"EXPRESSIONS"}, {"REGISTERS"}, {"RADIX"}, {"EVENTPOINTS"}, {"BUILT_IN_SYMBOLS"},
		{"PATH_NAMES"}, {"FORTRAN_PROGRAMS"}, {"SET", "BREAK", "QUALIFIERS", "/AFTER"},
		{"EVALUATE"}, {"SYMBOLIZE"}, {"DEPOSIT"}, {"EX"}, {"D"}, {"S"}, {"ST"},
		{"SH", "BREAK"}, {"SE", "BREAK"},
	}

	for _, words := range topics {
		if !found(t, lib, words...) {
			t.Errorf("HELP %s: no topic", strings.Join(words, " "))
		}
	}

	if got, _ := consoletest.HelpText(t, lib, "DEBUG"); !strings.Contains(got, "SYMBOLIZE") {
		t.Errorf("HELP alone doesn't list the commands:\n%s", got)
	}
}
