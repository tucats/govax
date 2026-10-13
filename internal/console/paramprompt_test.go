package console_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// TestParameterPrompt checks DCL's prompt for a required parameter a
// command typed at the terminal leaves out (OpenVMS User's Manual,
// 2.2.1): "$ SHOW" asks "_What: " on the terminal, and the reply is
// parsed as if typed on the line. Ctrl/Z at the prompt drops the command
// without a message (2.2.2). A command read from a command procedure
// isn't prompted for: its missing parameter is an error, as on VMS.
func TestParameterPrompt(t *testing.T) {
	var out bytes.Buffer

	c := console.New(&out)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if err := d.Dispatch("SHOW MEMORY"); err != nil {
		t.Fatalf("SHOW MEMORY: %v", err)
	}

	want := out.String()

	// An empty reply asks again; the reply is uppercased with the line.
	out.Reset()
	c.In = strings.NewReader("\nmemory\n")

	if err := d.Dispatch("SHOW"); err != nil {
		t.Fatalf("SHOW: %v", err)
	}

	if got := out.String(); got != "_What: _What: "+want {
		t.Errorf("SHOW, replying MEMORY: output\n%q\nwant\n%q", got, "_What: _What: "+want)
	}

	// Ctrl/Z (the input's end) cancels the command, $STATUS SS$_NORMAL
	// marked as shown, whatever it was (VMS's console).
	_ = d.Dispatch("SHOW NOSUCHTHING")

	out.Reset()
	c.In = strings.NewReader("")

	if err := d.Dispatch("SHOW"); err != nil || out.String() != "_What: " {
		t.Errorf("SHOW, Ctrl/Z: err %v, output %q; want nil, \"_What: \"", err, out.String())
	}

	out.Reset()

	if err := d.Dispatch("SHOW SYMBOL $STATUS"); err != nil || !strings.Contains(out.String(), `$STATUS == "%X10000001"`) {
		t.Errorf("$STATUS after Ctrl/Z: err %v, output %q; want %%X10000001", err, out.String())
	}

	// In a command procedure, no prompt: the parameter is missing.
	proc := filepath.Join(t.TempDir(), "show.com")
	if err := os.WriteFile(proc, []byte("$ SHOW\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	in := strings.NewReader("MEMORY\n")
	c.In = in

	err := d.Dispatch(`@"` + proc + `"`)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER)) && !strings.Contains(out.String(), "MISSINGPARAMETER") {
		t.Errorf("@SHOW.COM: err %v, output %q; want MISSINGPARAMETER", err, out.String())
	}

	if strings.Contains(out.String(), "_What:") || in.Len() == 0 {
		t.Errorf("@SHOW.COM prompted: output %q, unread input %d bytes", out.String(), in.Len())
	}
}
