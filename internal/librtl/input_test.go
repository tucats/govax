package librtl

import (
	"strings"
	"testing"
)

// TestLibGetInput reads lines from the terminal, prompting for each, into
// a fixed-length string padded with blanks, with their lengths. A line
// ends at a carriage return, a line feed, or a "\r\n" pair.
func TestLibGetInput(t *testing.T) {
	env, out := foreignFixture(t, "IGNORED", "first line\r\nsecond\nthird\r")
	a := newArena(t, env)
	result, length, prompt := a.desc(strings.Repeat("x", 12)), a.word(0), a.desc("Input: ")

	for _, want := range []string{"first line", "second", "third"} {
		if r0 := call(t, env, "LIB$GET_INPUT", result, prompt, length); r0 != ssNormal {
			t.Fatalf("status = %#x, want SS$_NORMAL", r0)
		}

		if got := descText(t, env, result); got != want+strings.Repeat(" ", 12-len(want)) {
			t.Errorf("result = %q, want %q padded", got, want)
		}

		if got := word(t, env, length); int(got) != len(want) {
			t.Errorf("length = %d, want %d", got, len(want))
		}
	}

	if got := out.String(); got != strings.Repeat("Input: ", 3) {
		t.Errorf("prompts = %q", got)
	}
}

// TestLibGetInput_ctrlZ: CTRL/Z on an empty line is RMS$_EOF, leaving the
// string and length alone; typed after some text it ends that line, and
// the second CTRL/Z govax's terminal sends after it is the next read's end
// of file.
func TestLibGetInput_ctrlZ(t *testing.T) {
	env, _ := foreignFixture(t, "", "abc\x1A\x1A\x1Aafter\n")
	a := newArena(t, env)
	result, length := a.desc("xxxxx"), a.word(0)

	if r0 := call(t, env, "LIB$GET_INPUT", result, 0, length); r0 != ssNormal {
		t.Fatalf("status = %#x, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "abc  " {
		t.Errorf("result = %q, want the text before CTRL/Z", got)
	}

	for range 2 {
		if r0 := call(t, env, "LIB$GET_INPUT", result, 0, length); r0 != rmsEOF {
			t.Errorf("status = %#x at CTRL/Z, want RMS$_EOF", r0)
		}
	}

	if got, n := descText(t, env, result), word(t, env, length); got != "abc  " || n != 3 {
		t.Errorf("result = %q, length %d after RMS$_EOF; want them unchanged", got, n)
	}

	if r0 := call(t, env, "LIB$GET_INPUT", result, 0, length); r0 != ssNormal {
		t.Errorf("status = %#x after the end of file, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "after" {
		t.Errorf("result = %q after the end of file", got)
	}
}

// TestLibGetInput_endOfInput is RMS$_EOF at the end of the host's input.
func TestLibGetInput_endOfInput(t *testing.T) {
	env, _ := foreignFixture(t, "", "")
	a := newArena(t, env)

	if r0 := call(t, env, "LIB$GET_INPUT", a.desc("xx")); r0 != rmsEOF {
		t.Errorf("status = %#x, want RMS$_EOF", r0)
	}
}

// TestLibGetInput_truncated is LIB$_INPSTRTRU for a line longer than the
// fixed-length string; the rest of the line isn't read by the next call.
func TestLibGetInput_truncated(t *testing.T) {
	env, _ := foreignFixture(t, "", "ABCDEFG\nNEXT\n")
	a := newArena(t, env)
	result, length := a.desc("xxxx"), a.word(0)

	if r0 := call(t, env, "LIB$GET_INPUT", result, 0, length); r0 != libInpStrTru {
		t.Errorf("status = %#x, want LIB$_INPSTRTRU", r0)
	}

	if got, n := descText(t, env, result), word(t, env, length); got != "ABCD" || n != 4 {
		t.Errorf("result = %q, length %d; want ABCD, 4", got, n)
	}

	if r0 := call(t, env, "LIB$GET_INPUT", result, 0, length); r0 != ssNormal || descText(t, env, result) != "NEXT" {
		t.Errorf("next call = %#x, %q; want the next line", r0, descText(t, env, result))
	}
}

// TestLibGetInput_dynamic allocates a dynamic string's storage to fit the
// line.
func TestLibGetInput_dynamic(t *testing.T) {
	env, _ := foreignFixture(t, "", "HELLO THERE\n")
	a := newArena(t, env)
	result := a.long(0)                     // length 0, then...
	putLongword(t, env, result, 0x020E0000) // ...DSC$K_DTYPE_T, DSC$K_CLASS_D
	putLongword(t, env, result+4, 0)        // no storage yet

	if r0 := call(t, env, "LIB$GET_INPUT", result); r0 != ssNormal {
		t.Errorf("status = %#x, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "HELLO THERE" {
		t.Errorf("result = %q", got)
	}
}

// TestLibGetInput_noResult is LIB$_INVARG without a get-string.
func TestLibGetInput_noResult(t *testing.T) {
	env, _ := foreignFixture(t, "", "x\n")

	if r0 := call(t, env, "LIB$GET_INPUT", 0); r0 != libInvArg {
		t.Errorf("status = %#x, want LIB$_INVARG", r0)
	}
}
