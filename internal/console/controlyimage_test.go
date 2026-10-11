package console_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/respath"
)

// spinSource loops COUNT times (forever when COUNT is 0, since SOBGTR
// counts down from 0 through the negative numbers before it stops), then
// writes "spun".
const spinSource = `	.title	spin
	.psect	data,noexe,wrt
spun:	.ascid	/spun/
	.psect	code,exe,nowrt
	.entry	start,^m<r2>
	movl	#COUNT,r2
	beql	forever
loop:	sobgtr	r2,loop
	pushaq	spun
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
forever: brb	forever
	.end	start
`

// TestControlY_image: Ctrl/Y typed while an image a procedure runs is
// computing (from another goroutine, as the terminal's keys arrive):
// at the Ctrl/Y level STOP ends the procedure, and CONTINUE lets the
// image finish and the procedure go on (13.11.1); with ON CONTROL_Y,
// the image is forced to exit and the command runs (13.12.1). The image
// is run down either way, and no ATTENTION message is shown.
func TestControlY_image(t *testing.T) {
	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)

	c.In = strings.NewReader("")

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v\n%s", err, out.String())
	}

	forever := buildImage(t, c, "forever", strings.Replace(spinSource, "COUNT", "0", 1))
	finite := buildImage(t, c, "finite", strings.Replace(spinSource, "COUNT", "2000000", 1))

	for _, tc := range []struct {
		name, typed string
		lines       []string
		want        string
	}{
		{"STOP", "STOP\n", []string{`$ RUN "` + forever + `"`, `$ PRINT "after"`}, ""},
		{"ON CONTROL_Y", "", []string{
			"$ ON CONTROL_Y THEN GOTO DONE",
			`$ RUN "` + forever + `"`,
			`$ PRINT "after"`,
			`$ DONE: PRINT "done"`,
		}, "done\n"},
		{"CONTINUE", "CONTINUE\n", []string{`$ RUN "` + finite + `"`, `$ PRINT "after"`}, "spun\nafter\n"},
	} {
		out.Reset()

		c.In = strings.NewReader(tc.typed)

		proc := filepath.Join(t.TempDir(), "spin.com")
		if err := os.WriteFile(proc, []byte(strings.Join(tc.lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// The key is typed once the image has had time to start; the
		// finite image may end first, and then DCL takes the key after
		// RUN, with the same result.
		engine := c.Engine

		go func() {
			time.Sleep(20 * time.Millisecond)
			engine.AttentionKey(cpu.AttentionCtrlY)
		}()

		_ = d.Dispatch(`@"` + proc + `"`)

		got := strings.ReplaceAll(out.String(), c.Prompt(), "")
		if got != tc.want || c.CommandLevel() != 0 || c.ImageActive() {
			t.Errorf("%s: output %q, level %d, image active %v; want %q, 0, false", tc.name, got, c.CommandLevel(), c.ImageActive(), tc.want)
		}
	}
}
