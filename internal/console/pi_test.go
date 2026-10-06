package console

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// piRun assembles and links testdata/mar/pi.mar, which prints pi, and
// runs it with command as its command line. It returns what the program
// printed and its final status.
func piRun(t *testing.T, command string) (string, uint32) {
	t.Helper()

	c := newBootableConsole(t)
	dir := t.TempDir()
	src := filepath.Join("..", "..", "testdata", "mar", "pi.mar")

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, "pi.obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "pi")}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	// What the kernel prints as it boots comes before the program's
	// output, so the output is captured only once the image is linked.
	out := &bytes.Buffer{}
	c.Out = out

	r0 := runImageCommand(t, c, filepath.Join(dir, "pi.exe"), command, 50_000_000)

	return out.String(), r0
}

// TestPi_default prints pi's first 100 places, the default.
func TestPi_default(t *testing.T) {
	got, r0 := piRun(t, "")

	if r0 != vmsdef.Symbols["SS$_NORMAL"] {
		t.Errorf("R0 = %#x, want SS$_NORMAL", r0)
	}

	want := "3.1415926535 8979323846 2643383279 5028841971 6939937510\n" +
		"  5820974944 5923078164 0628620899 8628034825 3421170679\n"

	if !strings.HasSuffix(got, want) {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

// TestPi_count prints the count of places the command line gives, which
// may be surrounded by blanks, and ends a partial line and group.
func TestPi_count(t *testing.T) {
	got, _ := piRun(t, "  63 ")

	want := "3.1415926535 8979323846 2643383279 5028841971 6939937510\n" +
		"  5820974944 592\n"

	if !strings.HasSuffix(got, want) {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

// TestPi_badCount reports a command line that isn't a count from 1 to
// 100000, and returns SS$_BADPARAM with its message inhibited.
func TestPi_badCount(t *testing.T) {
	const inhibit = 0x10000000

	for _, command := range []string{"0", "abc", "12x", "1 2", "100001"} {
		t.Run(command, func(t *testing.T) {
			got, r0 := piRun(t, command)

			if want := vmsdef.Symbols["SS$_BADPARAM"] | inhibit; r0 != want {
				t.Errorf("R0 = %#x, want %#x", r0, want)
			}

			if !strings.Contains(got, "%PI-E-BADCOUNT,") || strings.Contains(got, "3.14") {
				t.Errorf("output:\n%s", got)
			}
		})
	}
}
