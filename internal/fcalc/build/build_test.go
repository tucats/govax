package build

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/fcalc"
	"github.com/tucats/govax/internal/vmsimage"
)

// TestCommittedImage checks that the committed fcalc.exe is what the
// embedded sources build: run "go generate ./internal/fcalc" after
// changing fcalc's sources, or after a change to the compiler, MACRO, or
// LINK that changes what they make of them.
func TestCommittedImage(t *testing.T) {
	built, err := Build(fcalc.Files)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(built, fcalc.Image()) {
		t.Fatalf("internal/fcalc/fcalc.exe (%d bytes) isn't what its sources build (%d bytes): run go generate ./internal/fcalc",
			len(fcalc.Image()), len(built))
	}
}

// TestImageShape checks the image is a program named FCALC, and that its
// shareable image list (each a counted name in its fixup section) names the
// run-time libraries a Fortran 90 program with fcalc's calls needs.
func TestImageShape(t *testing.T) {
	data := fcalc.Image()

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		t.Fatal(err)
	}

	if img.Name != ImageName {
		t.Errorf("image name %q, want %q", img.Name, ImageName)
	}

	for _, want := range []string{"FTNRTL", "MTHRTL", "LIBRTL", "LBRSHR"} {
		if !bytes.Contains(data, append([]byte{byte(len(want))}, want...)) {
			t.Errorf("FCALC.EXE doesn't use %s", want)
		}
	}
}
