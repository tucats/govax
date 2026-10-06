package console

import "github.com/tucats/gopackages/app-cli/settings"

// promptSetting is the configuration key holding the console's prompt,
// which SET PROMPT changes.
const promptSetting = "vax.console.prompt"

// DefaultPrompt is the console's prompt when vax.console.prompt isn't set.
const DefaultPrompt = "VAX> "

// Prompt returns the console's prompt: the vax.console.prompt setting, or
// DefaultPrompt when it isn't set. The run loop (cmd/govax) asks for it
// before each line it reads, so a SET PROMPT takes effect at once.
func (c *Console) Prompt() string {
	if s := settings.Get(promptSetting); s != "" {
		return s
	}

	return DefaultPrompt
}

// SetPrompt implements SET PROMPT="text": text becomes the console's
// prompt and the vax.console.prompt setting, which govax writes back to
// its configuration when it exits, so later sessions keep it. An empty
// text ("") returns to the default prompt.
func (c *Console) SetPrompt(text string) error {
	if text == "" {
		_ = settings.Delete(promptSetting)

		return nil
	}

	settings.Set(promptSetting, text)

	return nil
}
