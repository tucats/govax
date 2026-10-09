package debugger_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// helpText returns what HELP prints for the words, from the help file h.
func helpText(t *testing.T, h *console.Help, words ...string) string {
	t.Helper()

	var out strings.Builder

	c := console.New(&out)
	if err := c.Help(h, words); err != nil {
		t.Fatalf("Help(%v): %v", words, err)
	}

	return out.String()
}

// TestEveryVerbHasHelp: each verb of the debugger's grammar, and each
// verb of the console's, has a topic in its own help file (docs/PHASE-42.md,
// subtask 15, as Phase 37 did for the console's). An alias, such as S for
// STEP, is checked through the topic of the verb it stands for; its own
// name is checked too where it is a word a user might ask about.
func TestEveryVerbHasHelp(t *testing.T) {
	for _, tc := range []struct {
		name    string
		grammar string
		help    string
	}{
		{"debugger", "debug", "debug.help"},
		{"console", "console", "console.help"},
	} {
		g := consoletest.ConsoleGrammar(t)
		if tc.grammar == "debug" {
			g = consoletest.DebugGrammar(t)
		}

		h := consoletest.ParseHelp(t, tc.help)

		names, alias := g.Verbs()
		for i, verb := range names {
			if alias[i] {
				continue
			}

			if got := helpText(t, h, verb); strings.Contains(got, "No help available") {
				t.Errorf("%s: no help topic for the verb %s", tc.name, verb)
			}
		}
	}
}

// TestDebuggerHelpTopics: the debugger's own help covers each SET, SHOW,
// and CANCEL keyword and the qualifiers that need explaining, so a HELP of
// each says something.
func TestDebuggerHelpTopics(t *testing.T) {
	h := consoletest.ParseHelp(t, "debug.help")

	topics := [][]string{
		{"ASM"}, {"ASM", "LABELS"}, {"ASM", "ADDRESSING"}, {"ASM", "PSEUDO", "QUAD"}, {"ASM", "PSEUDO", "BYTE"},
		{"SET", "BREAK"}, {"SET", "TRACE"}, {"SET", "WATCH"}, {"SET", "STEP"},
		{"SET", "SOURCE"}, {"SET", "MODE"}, {"SET", "RADIX"}, {"SET", "MODULE"},
		{"SET", "PSL"}, {"SET", "PTE"}, {"SET", "FAULT"}, {"SET", "VM"}, {"SET", "BASE"},
		{"SHOW", "BREAK"}, {"SHOW", "TRACE"}, {"SHOW", "WATCH"}, {"SHOW", "STEP"},
		{"SHOW", "SOURCE"}, {"SHOW", "CALLS"}, {"SHOW", "STACK"}, {"SHOW", "KSP"},
		{"SHOW", "SP"}, {"SHOW", "REGISTERS"}, {"SHOW", "REGIONS"}, {"SHOW", "PSL"},
		{"SHOW", "CPU_STATUS"}, {"SHOW", "CLOCK"}, {"SHOW", "BASE"}, {"SHOW", "MEMORY"},
		{"SHOW", "MAPS"}, {"SHOW", "TB"}, {"SHOW", "PAGE"}, {"SHOW", "SCB"},
		{"SHOW", "SHIM"}, {"SHOW", "EXCEPTIONS"}, {"SHOW", "MODE"}, {"SHOW", "RADIX"},
		{"SHOW", "IMAGE"}, {"SHOW", "MODULE"}, {"SHOW", "SYMBOL"}, {"SHOW", "SCOPE"},
		{"SHOW", "LANGUAGE"},
		{"CANCEL", "BREAK"}, {"CANCEL", "TRACE"}, {"CANCEL", "WATCH"}, {"CANCEL", "SOURCE"},
		{"CANCEL", "RADIX"}, {"CANCEL", "MODE"}, {"CANCEL", "INTERRUPT"}, {"CANCEL", "TB"},
		{"CANCEL", "MEMORY"}, {"CLEAR", "BREAK"},
		{"EXAMINE", "/INSTRUCTION"}, {"EXAMINE", "/OPERANDS"}, {"EXAMINE", "/BYTE"},
		{"EXAMINE", "/ASCII"}, {"EXAMINE", "/HEXADECIMAL"}, {"EXAMINE", "/PSL"},
		{"EXAMINE", "/PTE"}, {"EXAMINE", "/CONSTANTS"}, {"EXAMINE", "/SHAREABLE"},
		{"EXPRESSIONS"}, {"REGISTERS"}, {"RADIX"}, {"BREAKPOINTS"},
		{"EVALUATE"}, {"SYMBOLIZE"}, {"DEPOSIT"}, {"EX"}, {"D"}, {"S"}, {"ST"},
		{"SH", "BREAK"}, {"SE", "BREAK"},
	}

	for _, words := range topics {
		if got := helpText(t, h, words...); strings.Contains(got, "No help available") {
			t.Errorf("HELP %s: no topic", strings.Join(words, " "))
		}
	}

	if got := helpText(t, h); !strings.Contains(got, "SYMBOLIZE") {
		t.Errorf("HELP alone doesn't list the commands:\n%s", got)
	}
}

// TestDebuggerHelpKeysAreUnique: no two keys of debug.help are the same
// once each word is cut to the four letters HELP matches on (SHOW REGISTERS
// and SHOW REGIONS would both be "SHOW,REGI"): the later topic would
// silently replace the earlier.
func TestDebuggerHelpKeysAreUnique(t *testing.T) {
	data, err := os.ReadFile(consoletest.BootFile(t, "debug.help"))
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]int{}
	inBody := false

	for n, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "$") {
			inBody = true

			continue
		}

		// Consecutive "$" lines are synonyms for one body; a key repeated
		// across different bodies is the problem.
		words := strings.Split(strings.ToUpper(line[1:]), ",")
		for i, w := range words {
			w = strings.TrimSpace(w)
			if len(w) > 4 {
				w = w[:4]
			}

			words[i] = w
		}

		key := strings.Join(words, ",")

		if prev, ok := seen[key]; ok && inBody {
			t.Errorf("debug.help line %d: key %q is also on line %d", n+1, key, prev)
		}

		seen[key] = n + 1
		inBody = false
	}
}
