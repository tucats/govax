package librtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// foreignFixture is an RTL environment whose foreign command text is
// command and whose terminal input is typed. It returns the environment
// and what it writes to the terminal.
func foreignFixture(t *testing.T, command, typed string) (*corevms.Environment, *bytes.Buffer) {
	t.Helper()

	out := &bytes.Buffer{}
	env, err := corevms.NewEnvironment(corevms.NewSystem(vax.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
		rms.NewMountTable()), lnm.NewDatabase(corevms.NominalUIC), strings.NewReader(typed), out)

	if err != nil {
		t.Fatal(err)
	}

	Register(env.Shims())
	env.CommandLine = command

	return env, out
}

// descText reads the string a descriptor at addr describes.
func descText(t *testing.T, env *corevms.Environment, addr uint32) string {
	t.Helper()

	s, _, err := env.StringDescriptor(addr, 65535)
	if err != nil {
		t.Fatal(err)
	}

	return s
}

func word(t *testing.T, env *corevms.Environment, addr uint32) uint16 {
	t.Helper()

	v, err := env.Memory().LoadWord(env.CPU(), addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// TestLibGetForeign returns the command's text in a fixed-length string,
// padded with blanks, with its length, and doesn't prompt.
func TestLibGetForeign(t *testing.T) {
	env, out := foreignFixture(t, "2 3 + .", "typed\n")
	a := newArena(t, env)
	result, length := a.desc(strings.Repeat("x", 10)), a.word(0)

	if r0 := call(t, env, "LIB$GET_FOREIGN", result, a.desc("? "), length); r0 != ssNormal {
		t.Errorf("status = %#x, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "2 3 + .   " {
		t.Errorf("result = %q", got)
	}

	if got := word(t, env, length); got != 7 {
		t.Errorf("length = %d, want 7", got)
	}

	if out.Len() != 0 {
		t.Errorf("prompted %q with a command line", out.String())
	}
}

// TestLibGetForeign_truncated is LIB$_INPSTRTRU for text longer than the
// fixed-length result.
func TestLibGetForeign_truncated(t *testing.T) {
	env, _ := foreignFixture(t, "ABCDEFG", "")
	a := newArena(t, env)
	result, length := a.desc("xxxx"), a.word(0)

	if r0 := call(t, env, "LIB$GET_FOREIGN", result, 0, length); r0 != libInpStrTru {
		t.Errorf("status = %#x, want LIB$_INPSTRTRU", r0)
	}

	if got, n := descText(t, env, result), word(t, env, length); got != "ABCD" || n != 4 {
		t.Errorf("result = %q, length %d; want ABCD, 4", got, n)
	}
}

// TestLibGetForeign_prompts reads the text from the terminal when there's
// no command line and a prompt is given, or when flags asks it to; flags
// is then 1.
func TestLibGetForeign_prompts(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		flags         uint32
	}{
		{"no command line", "", 0},
		{"forced", "IGNORED", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, out := foreignFixture(t, tc.command, "typed text\r\nmore\n")
			a := newArena(t, env)
			result, length, flags := a.desc(strings.Repeat(" ", 20)), a.word(0), a.long(tc.flags)

			if r0 := call(t, env, "LIB$GET_FOREIGN", result, a.desc("Text: "), length, flags); r0 != ssNormal {
				t.Errorf("status = %#x, want SS$_NORMAL", r0)
			}

			if got := descText(t, env, result)[:word(t, env, length)]; got != "typed text" {
				t.Errorf("result = %q, want the typed line", got)
			}

			if out.String() != "Text: " {
				t.Errorf("prompt = %q", out.String())
			}

			if got := longword(t, env, flags); got != 1 {
				t.Errorf("flags = %d, want 1", got)
			}
		})
	}
}

// TestLibGetForeign_noText returns an empty string, without prompting,
// when there's neither a command line nor a prompt; and RMS$_EOF when
// it prompts at the end of the input.
func TestLibGetForeign_noText(t *testing.T) {
	env, _ := foreignFixture(t, "", "")
	a := newArena(t, env)
	result, length := a.desc("xx"), a.word(9)

	if r0 := call(t, env, "LIB$GET_FOREIGN", result, 0, length); r0 != ssNormal {
		t.Errorf("status = %#x, want SS$_NORMAL", r0)
	}

	if got, n := descText(t, env, result), word(t, env, length); got != "  " || n != 0 {
		t.Errorf("result = %q, length %d; want blanks, 0", got, n)
	}

	if r0 := call(t, env, "LIB$GET_FOREIGN", result, a.desc("> "), length); r0 != rmsEOF {
		t.Errorf("status at end of input = %#x, want RMS$_EOF", r0)
	}
}

// TestLibGetForeign_dynamic allocates a dynamic string's storage to fit
// the text.
func TestLibGetForeign_dynamic(t *testing.T) {
	env, _ := foreignFixture(t, "HELLO THERE", "")
	a := newArena(t, env)
	result := a.long(0)                     // length 0, then...
	putLongword(t, env, result, 0x020E0000) // ...DSC$K_DTYPE_T, DSC$K_CLASS_D
	putLongword(t, env, result+4, 0)        // no storage yet

	if r0 := call(t, env, "LIB$GET_FOREIGN", result); r0 != ssNormal {
		t.Errorf("status = %#x, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "HELLO THERE" {
		t.Errorf("result = %q", got)
	}

	if longword(t, env, result+4) == 0 {
		t.Error("no storage allocated")
	}
}

// TestLibGetForeign_noResult is LIB$_INVARG without a result string.
func TestLibGetForeign_noResult(t *testing.T) {
	env, _ := foreignFixture(t, "X", "")

	if r0 := call(t, env, "LIB$GET_FOREIGN"); r0 != libInvArg {
		t.Errorf("status = %#x, want LIB$_INVARG", r0)
	}
}
