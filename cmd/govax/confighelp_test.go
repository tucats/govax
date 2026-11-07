package main

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/help"
	"github.com/tucats/govax/internal/lbr"
)

// TestConfigKeysHaveHelp: every configuration key govax reads (validConfigs)
// is described under HELP CONFIGURATION KEYS (docs/PHASE-90 - VMS help.md,
// Decision 10). The debug switches are described together, as
// vax.debug.NAME (SET DEBUG's names).
func TestConfigKeysHaveHelp(t *testing.T) {
	data, err := fs.ReadFile(bootdata.FS, bootdata.HelpLibrary)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	displays, err := help.Lookup(help.NewLBRLibrary(l), []string{"CONFIGURATION", "KEYS"}, help.Options{})
	if err != nil {
		t.Fatal(err)
	}

	text := strings.Join(help.Render(displays, 132), "\n")
	if strings.Contains(text, "Sorry, no documentation") {
		t.Fatal("no CONFIGURATION KEYS topic")
	}

	for key := range validConfigs {
		if strings.HasPrefix(key, "vax.debug.") && strings.Contains(text, "vax.debug.NAME") {
			continue
		}

		if !strings.Contains(text, key) {
			t.Errorf("%s isn't under HELP CONFIGURATION KEYS", key)
		}
	}
}
