package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vax"
)

func TestDefaultRunInits(t *testing.T) {
	c := newRunnableConsole(t)
	if !c.DefaultRunInits() {
		t.Error("DefaultRunInits() = false, want true (DebugLibinit is on by default)")
	}

	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugLibinit)

	if c.DefaultRunInits() {
		t.Error("DefaultRunInits() = true, want false once DebugLibinit is cleared")
	}
}

// TestRunOptions_defaultAndOverride checks RUN's qualifiers as the
// grammar reads them (docs/PHASE-37.md): /INIT and /NOINIT override the
// default, and the qualifiers combine.
func TestRunOptions_defaultAndOverride(t *testing.T) {
	g := loadEvaxGrammar(t)

	cases := []struct {
		line string
		def  bool
		want machine.RunOptions
	}{
		{"RUN FOO.EXE", true, machine.RunOptions{RunInits: true}},
		{"RUN/NOINIT FOO.EXE", true, machine.RunOptions{}},
		{"RUN/INIT FOO.EXE", false, machine.RunOptions{RunInits: true}},
		{`R/NOEXECUTE/DEBUG "foo.exe"/HOST`, false, machine.RunOptions{Debug: machine.DebugOn, NoExecute: true, Host: true}},
		{"RUN FOO.EXE /BREAK /NOINIT", true, machine.RunOptions{Debug: machine.DebugOn}},
		{"RUN/STEP FOO.EXE", false, machine.RunOptions{Debug: machine.DebugOn}},
		{"RUN/NODEBUG FOO.EXE", false, machine.RunOptions{Debug: machine.DebugOff}},
	}

	for _, c := range cases {
		r, err := g.Parse(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.line, err)

			continue
		}

		if got := runOptions(r, c.def); got != c.want {
			t.Errorf("%s: %+v, want %+v", c.line, got, c.want)
		}
	}
}

func TestRun_debugImagesTrace(t *testing.T) {
	c := newRunnableConsole(t)
	c.CPU.SetDebug(vax.DebugImages)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.Run(exeFixturePath(t, "simple.exe"), machine.RunOptions{NoExecute: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := c.Out.(*bytes.Buffer).String()
	if !strings.Contains(out, "Main image is") {
		t.Errorf("output = %q, want a \"Main image is\" trace", out)
	}
}
