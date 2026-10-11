package console

import (
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
)

// TestSetPrompt checks SET PROMPT="text": the prompt and the
// vax.console.prompt setting change, case and blanks kept; "" returns to
// the default; and a name that only abbreviates PROMPT is still a symbol
// assignment.
func TestSetPrompt(t *testing.T) {
	saved, had := settings.Get(promptSetting), settings.Exists(promptSetting)

	t.Cleanup(func() {
		if had {
			settings.Set(promptSetting, saved)
		} else {
			_ = settings.Delete(promptSetting)
		}
	})

	d, c, _ := newCommandDispatcher(t)

	_ = settings.Delete(promptSetting)

	if got := c.Prompt(); got != DefaultPrompt {
		t.Fatalf("Prompt() = %q unset, want %q", got, DefaultPrompt)
	}

	for _, line := range []string{`SET PROMPT="vax$ "`, `set prompt = "vax$ "`, `SET PROMPT "vax$ "`} {
		_ = settings.Delete(promptSetting)

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if got := c.Prompt(); got != "vax$ " {
			t.Errorf("%s: Prompt() = %q, want %q", line, got, "vax$ ")
		}

		if got := settings.Get(promptSetting); got != "vax$ " {
			t.Errorf("%s: %s = %q", line, promptSetting, got)
		}
	}

	if err := d.Dispatch(`SET PROMPT=""`); err != nil {
		t.Fatalf(`SET PROMPT="": %v`, err)
	}

	if got := c.Prompt(); got != DefaultPrompt {
		t.Errorf(`Prompt() = %q after SET PROMPT="", want %q`, got, DefaultPrompt)
	}

	if err := d.Dispatch(`SET PROM="X> "`); err != nil {
		t.Fatalf(`SET PROM="X> ": %v`, err)
	}

	if got := c.Prompt(); got != "X> " {
		t.Errorf(`Prompt() = %q after SET PROM="X> ", want "X> "`, got)
	}
}
