package main

import (
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
)

// TestConfigKeysChecked: with configDomain reserved, "config set" (which
// asks settings.Check) takes govax's keys with values their readers take,
// and refuses an unknown key or a value its reader would ignore.
func TestConfigKeysChecked(t *testing.T) {
	reserveConfigKeys()

	cases := []struct {
		key, value string
		ok         bool
		says       string
	}{
		{"vax.library", "/some/where", true, ""},
		{"vax.libary", "/some/where", false, "invalid configuration name"},
		{"VAX.HARDWARE.CLOCK", "yes", true, ""},
		{"vax.hardware.clock", "sometimes", false, "must be true or false"},
		{"vax.debug.rms", "1", true, ""},
		{"vax.debug.rms", "yes", false, "must be true or false (or 1 or 0, t or f)"},
		{"vax.quantum", "20", true, ""},
		{"vax.quantum", "0", false, "must be a positive integer"},
		{"vax.console.lines", "x", false, "must be an integer"},
		{"vax.process.preempt", "User", true, ""},
		{"vax.process.preempt", "sometimes", false, "must be one of all, user, none"},
		{"vax.sysgen.minwscnt", "%X40", true, ""},
		{"vax.sysgen.minwscnt", "0", false, "must be a positive integer"},
		{"vax.sysgen.growlim", "0", true, ""},
		{"vax.sysgen.mpw_wrtcluster", "20", false, "multiple of 8 from 16 to 120"},
		{"vax.sysgen.pfcdefault", "128", false, "from 0 to 127"},
		{"vax.ramdisk.size", "10", false, "must be a number of blocks"},
		{"other.app.key", "anything", true, ""},
	}

	for _, c := range cases {
		err := settings.Check(c.key, c.value)
		if (err == nil) != c.ok {
			t.Errorf("Check(%q, %q) = %v, want ok %v", c.key, c.value, err, c.ok)

			continue
		}

		if err != nil && !strings.Contains(err.Error(), c.says) {
			t.Errorf("Check(%q, %q) = %q, want it to say %q", c.key, c.value, err, c.says)
		}
	}

	if err := settings.CheckDelete("vax.libary"); err != nil {
		t.Errorf("CheckDelete of a misspelled key: %v", err)
	}
}

// TestAuditConfig: a profile's unknown key and a key with a value its reader
// ignores are each reported at the session's end, as before the keys were
// reserved.
func TestAuditConfig(t *testing.T) {
	reserveConfigKeys()

	values := map[string]string{
		"vax.colour":          "blue",
		"vax.process.preempt": "sometimes",
		"vax.debug.rms":       "yes",
		"vax.library":         "/fine",
	}

	for key, value := range values {
		old, had := settings.Get(key), settings.Exists(key)

		t.Cleanup(func() {
			if had {
				settings.Set(key, old)
			} else {
				_ = settings.Delete(key)
			}
		})

		settings.Set(key, value)
	}

	var out strings.Builder

	auditConfig(&out)

	for _, want := range []string{
		"\nWarning, the following invalid or unrecognized config value(s) will be ignored:\n   vax.colour\n",
		"\nWarning, vax.debug.rms is \"yes\"; it must be true or false (or 1 or 0, t or f), and its default will be used\n",
		"\nWarning, vax.process.preempt is \"sometimes\"; it must be one of all, user, none, and its default will be used\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the audit lacks %q:\n%s", want, out.String())
		}
	}

	if strings.Contains(out.String(), "vax.library") {
		t.Errorf("the audit reports a good key:\n%s", out.String())
	}
}
