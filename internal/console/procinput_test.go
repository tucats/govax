package console_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/respath"
)

// readerSource reads lines with LIB$GET_INPUT and writes each with
// LIB$PUT_OUTPUT, until the end of its input (all lines), or after the
// first (one line); then it says so.
const readerSource = `	.title	reader
	.psect	data,noexe,wrt
buf:	.blkb	80
ind:	.word	80
	.byte	14,1
	.address buf
outd:	.word	0
	.byte	14,1
	.address buf
len:	.word	0
endmsg:	.ascid	/end of input/
	.psect	code,exe,nowrt
	.entry	start,^m<>
loop:	pushaw	len
	clrl	-(sp)
	pushaq	ind
	calls	#3,g^lib$get_input
	blbc	r0,done
	movw	len,outd
	pushaq	outd
	calls	#1,g^lib$put_output
	ONCE
	brb	loop
done:	pushaq	endmsg
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
	.end	start
`

// TestProcedureInput_image: an image a command procedure runs reads the
// procedure's data lines as its input, up to the next "$" line (its end
// of file); the data an image leaves unread is skipped, with SKPDAT.
func TestProcedureInput_image(t *testing.T) {
	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)

	// Nothing waits at the terminal: a read of it would be the end of
	// its input.
	c.In = strings.NewReader("")

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v\n%s", err, out.String())
	}

	all := buildImage(t, c, "readall", strings.Replace(readerSource, "ONCE", "", 1))
	one := buildImage(t, c, "readone", strings.Replace(readerSource, "ONCE", "brb done", 1))

	proc := filepath.Join(t.TempDir(), "reader.com")
	text := strings.Join([]string{
		`$ RUN "` + all + `"`,
		"first",
		"second",
		`$ RUN "` + one + `"`,
		"third",
		"fourth",
		"fifth",
		`$ PRINT "done"`,
	}, "\n") + "\n"

	if err := os.WriteFile(proc, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	if err := d.Dispatch(`@"` + proc + `"`); err != nil {
		t.Fatalf("@reader: %v\n%s", err, out.String())
	}

	want := "first\nsecond\nend of input\nthird\nend of input\n" +
		`%DCL-W-SKPDAT, image data (records not beginning with "$") ignored` + "\ndone\n"
	if out.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}
