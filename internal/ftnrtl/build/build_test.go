package build

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/link"
)

// TestCommittedImage checks that the committed ftnrtl.exe is what its
// sources build: run "go generate ./internal/ftnrtl" after changing them.
func TestCommittedImage(t *testing.T) {
	built, err := Build(ftnrtl.Files)
	if err != nil {
		t.Fatal(err)
	}

	committed, ok := ftnrtl.Image("ftnrtl")
	if !ok {
		t.Fatal("no built-in FTNRTL")
	}

	if !bytes.Equal(built, committed) {
		t.Error("ftnrtl.exe isn't what its sources build; run go generate ./internal/ftnrtl")
	}
}

// TestImage checks the image's global symbol table: its name, its
// match control, and that each entry is a universal symbol at its
// transfer vector offset, which must never change.
func TestImage(t *testing.T) {
	data, _ := ftnrtl.Image(ftnrtl.Name)

	src, name, err := link.ReadShareableImage(data)
	if err != nil || name != ftnrtl.Name {
		t.Fatalf("ReadShareableImage: %q, %v", name, err)
	}

	image, ok := src.Image(ftnrtl.Name)
	if !ok || image.Match != link.MatchLEQ || image.MajorID != 1 {
		t.Errorf("image %+v", image)
	}

	for symbol, offset := range map[string]uint32{"FTN$VERSION": 0, "FTN$STOP": 8} {
		d, ok, _ := src.Lookup(symbol)
		if !ok || d.Image != ftnrtl.Name || d.Value != offset {
			t.Errorf("%s: %+v, want offset %d in FTNRTL", symbol, d, offset)
		}
	}

	if _, ok := ftnrtl.Image("LIBRTL"); ok {
		t.Error("Image answered for LIBRTL")
	}
}
