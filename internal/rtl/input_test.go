package rtl

import (
	"bytes"
	"testing"
)

func TestShimDeccGets(t *testing.T) {
	env, _ := fixture()
	env.consoleIn = bytes.NewBufferString("hello world\nsecond line\n")

	bufAddr := uint32(0x1000)

	r0, err := shimDeccGets(env, []uint32{bufAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != bufAddr {
		t.Errorf("r0 = %#x, want %#x (the buffer address)", r0, bufAddr)
	}

	got, err := loadString(env, bufAddr, 64)
	if err != nil {
		t.Fatal(err)
	}

	if got != "hello world\n" {
		t.Errorf("line = %q, want %q (newline kept, unlike EXE$INPUT)", got, "hello world\n")
	}
}

func TestShimExeInputStripsOneTrailingChar(t *testing.T) {
	env, _ := fixture()
	env.consoleIn = bytes.NewBufferString("some input\r\n")

	bufAddr := uint32(0x1000)

	n, err := shimExeInput(env, []uint32{bufAddr, 64})
	if err != nil {
		t.Fatal(err)
	}

	got, err := loadString(env, bufAddr, int(n)+1)
	if err != nil {
		t.Fatal(err)
	}

	// exe_input strips exactly one trailing \r or \n, matching its own
	// single-character check -- not a \r\n pair.
	if got != "some input\r" {
		t.Errorf("input = %q, want %q", got, "some input\r")
	}
}

func TestShimExeReadFromConsole(t *testing.T) {
	env, _ := fixture()
	// exe_read's fid-0 path only strips a trailing '\r', not '\n' (matching
	// exe_read's own "if (bp[count-1] == '\r') count--" — a check that, on
	// a normal Unix '\n'-terminated line, never actually fires; replicated
	// as-is, not "fixed" to strip both).
	env.consoleIn = bytes.NewBufferString("console read\r\n")

	bufAddr := uint32(0x1000)
	
	n, err := shimExeRead(env, []uint32{0, bufAddr, 64})
	if err != nil {
		t.Fatal(err)
	}

	got, err := loadString(env, bufAddr, int(n)+1)
	if err != nil {
		t.Fatal(err)
	}

	if got != "console read\r\n" {
		t.Errorf("data = %q, want %q (trailing \\n left in place)", got, "console read\r\n")
	}
}
