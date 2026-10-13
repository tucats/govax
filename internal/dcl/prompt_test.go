package dcl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestParsePrompting checks DCL's prompting for a required parameter a
// command typed at the terminal doesn't give: each prompt is "_" and the
// parameter's /prompt= text, the reply completes the line, an empty reply
// asks again, and Ctrl/Z cancels the command.
func TestParsePrompting(t *testing.T) {
	g := loadEvaxGrammar(t)

	// replies answers each prompt in turn, recording the prompts asked;
	// past its replies it is the terminal's end of file.
	replies := func(asked *[]string, typed ...string) func(string) (string, bool) {
		return func(prompt string) (string, bool) {
			*asked = append(*asked, prompt)
			if len(typed) == 0 {
				return "", false
			}

			reply := typed[0]
			typed = typed[1:]

			return reply, true
		}
	}

	// SET asks for what to set, and SET QUANTUM then for its count.
	var asked []string

	r, err := g.ParsePrompting("SET", replies(&asked, "", "quantum", " 20 "))
	if err != nil {
		t.Fatalf("SET: %v", err)
	}

	if want := []string{"_What: ", "_What: ", "_Quantum: "}; !equalStrings(asked, want) {
		t.Errorf("prompts %q, want %q", asked, want)
	}

	if r.Active != "SET_QUANTUM" || r.Int("COUNT") != 20 {
		t.Errorf("parsed %s COUNT=%d, want SET_QUANTUM COUNT=20", r.Active, r.Int("COUNT"))
	}

	// A line that gives the parameter isn't prompted for.
	asked = nil

	if _, err := g.ParsePrompting("SET QUANTUM 20", replies(&asked)); err != nil || len(asked) != 0 {
		t.Errorf("SET QUANTUM 20: err %v, prompts %q", err, asked)
	}

	// Ctrl/Z at a prompt cancels the command.
	asked = nil

	if _, err := g.ParsePrompting("SET", replies(&asked)); !errors.Is(err, ErrCanceled) {
		t.Errorf("Ctrl/Z: err %v, want ErrCanceled", err)
	}

	// With nothing to ask, the missing parameter is the error Parse gives.
	if _, err := g.ParsePrompting("SET", nil); !errors.Is(err, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER)) {
		t.Errorf("no terminal: err %v, want MISSINGPARAMETER", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
