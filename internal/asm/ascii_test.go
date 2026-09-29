package asm

import (
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestAsciiDirectives pins the four string directives' layouts. .ASCIC's
// count is one byte, as MACRO-32 defines it (eVAX stored a word).
func TestAsciiDirectives(t *testing.T) {
	requireBytes(t, assembleBytes(t, "\t.ascii \"AB\"\n\t.end\n"), 'A', 'B')
	requireBytes(t, assembleBytes(t, "\t.asciz \"AB\"\n\t.end\n"), 'A', 'B', 0)
	requireBytes(t, assembleBytes(t, "\t.ascic \"ABC\"\n\t.end\n"), 3, 'A', 'B', 'C')
	requireBytes(t, assembleBytes(t, "\t.ascic \"AB\" \"CD\"\n\t.end\n"), 4, 'A', 'B', 'C', 'D')

	// .ASCID: length word, information word (class S, type T: 010E), the
	// address of the text (0x208: eight bytes past the default origin,
	// 0x200), then the text.
	requireBytes(t, assembleBytes(t, "\t.ascid \"AB\"\n\t.end\n"), 2, 0, 0x0E, 0x01, 8, 2, 0, 0, 'A', 'B')
}

// TestAsciiMacro32Strings covers MACRO-32's string syntax: any delimiter,
// case kept, <expression> bytes between strings, and no escapes.
func TestAsciiMacro32Strings(t *testing.T) {
	tests := []struct {
		src  string
		want []byte
	}{
		{"\t.ascii /eof/<^X0D><^X0A>", []byte{'e', 'o', 'f', 0x0D, 0x0A}},
		{"\t.asciz /A/<0C>/B/", []byte{'A', 0x0C, 'B', 0}},
		{"\t.ascic #hi#<^D13>", []byte{3, 'h', 'i', 13}},
		{"\t.ascii @a;b@ ; a comment", []byte{'a', ';', 'b'}},
		{"\t.ascii ,a b,", []byte{'a', ' ', 'b'}},
		{"\t.ascii \"don't\" ; comment", []byte{'d', 'o', 'n', '\'', 't'}},
		{"\t.ascii \"a\\nb\"", []byte{'a', '\\', 'n', 'b'}},
		{"\t.ascii <^X41+<1>>", []byte{'B'}},
		{"\t.ascii <'a'>", []byte{'a'}},
		{"LF = 0A\n\t.ascii \"x\"<LF>", []byte{'x', 0x0A}},
		{"\t.ascii <X>/y/\nX = 7", []byte{7, 'y'}},
		{"\tascii \"ab\"", []byte{'a', 'b'}},
	}

	for _, tc := range tests {
		requireBytes(t, assembleBytes(t, tc.src+"\n\t.end\n"), tc.want...)
	}
}

func TestAsciiErrors(t *testing.T) {
	requireCode(t, assembleErr(t, "\t.ascii \"abc"), vmserrors.VAX_NOCLOSE)
	requireCode(t, assembleErr(t, "\t.ascii =abc="), vmserrors.VAX_BADSTRING)
	requireCode(t, assembleErr(t, "\t.ascii <1"), vmserrors.VAX_NOCLOSE)
	requireCode(t, assembleErr(t, "\t.ascii <100>"), vmserrors.VAX_DATARANGE)
}

// TestAscicTooLong: a counted string can't be longer than its count byte
// can say.
func TestAscicTooLong(t *testing.T) {
	requireBytes(t, assembleBytes(t, "\t.ascic \""+strings.Repeat("x", 255)+"\"\n\t.end\n")[:1], 255)

	_, err := New(true).Assemble("\t.ascic \"" + strings.Repeat("x", 256) + "\"\n\t.end\n")
	if !errors.Is(err, vmserrors.New(vmserrors.VAX_DATARANGE)) {
		t.Errorf("a 256-character .ASCIC: err = %v, want VAX_DATARANGE", err)
	}
}
