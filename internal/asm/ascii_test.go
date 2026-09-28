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
	requireBytes(t, assembleBytes(t, "\t.ascic \"AB\", \"CD\"\n\t.end\n"), 4, 'A', 'B', 'C', 'D')

	// .ASCID: length word, type/class word, address of the text (0x208:
	// eight bytes past the default origin, 0x200), then the text.
	requireBytes(t, assembleBytes(t, "\t.ascid \"AB\"\n\t.end\n"), 2, 0, 0, 0, 8, 2, 0, 0, 'A', 'B')
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
