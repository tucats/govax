package lbr

import (
	"path/filepath"
	"strings"
	"testing"
)

// testdata/link68's VMS run: LIBRARY/CREATE/SHARE MYSHR SHR1.EXE,SHR2.EXE
// made vax/myshr.olb, and LIBRARY/LIST=MYSHR.LIS/FULL/NAMES/SHARE listed
// it as vax/myshr.lis.
var link68Dir = filepath.Join("..", "..", "testdata", "link68", "vax")

// shareableModules reads the probe's shareable images as LIBRARY/SHARE
// does.
func shareableModules(t *testing.T, b *Builder, images ...string) []*Entry {
	t.Helper()

	return shareableModulesIn(t, b, link68Dir, images...)
}

// shareableModulesIn is shareableModules for the images in dir.
func shareableModulesIn(t *testing.T, b *Builder, dir string, images ...string) []*Entry {
	t.Helper()

	var entries []*Entry

	for _, name := range images {
		e, err := b.ShareableModule(readFixture(t, filepath.Join(dir, name+".exe")))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		entries = append(entries, e)
	}

	return entries
}

// TestCreateShareableLibrary makes MYSHR.OLB from VMS's SHR1.EXE and
// SHR2.EXE, as the probe did, and checks it is real LIBRARIAN's library
// byte for byte, given its times and name: each module the image's global
// symbol table's header and end records, its ident in the module header,
// and its universal symbols in index 2.
func TestCreateShareableLibrary(t *testing.T) {
	want := readFixture(t, filepath.Join(link68Dir, "myshr.olb"))

	real, err := Open(want)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Create(TypeShareable)
	if err != nil {
		t.Fatal(err)
	}

	entries := shareableModules(t, b, "shr1", "shr2")
	withRealTimes(t, b, real, entries)

	for _, e := range entries {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	requireSameLibrary(t, b.Bytes(), want)
}

// TestCreateShareableLibraryOfGovaxImages makes MYSHRV.OLB from govax's
// SHR1.EXE and SHR2.EXE, as VMS's librarian did in testdata/link68's
// round 2, and checks it is VMS's byte for byte, given its times and
// name: real LIBRARIAN reads govax's images' tables as govax does.
func TestCreateShareableLibraryOfGovaxImages(t *testing.T) {
	round2 := filepath.Join(link68Dir, "round2")
	want := readFixture(t, filepath.Join(round2, "myshrv.olb"))

	real, err := Open(want)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Create(TypeShareable)
	if err != nil {
		t.Fatal(err)
	}

	entries := shareableModulesIn(t, b, filepath.Join(round2, "govax"), "shr1", "shr2")
	withRealTimes(t, b, real, entries)

	for _, e := range entries {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	requireSameLibrary(t, b.Bytes(), want)
}

// TestListShareableLibrary lists VMS's MYSHR.OLB, and the one govax
// makes, as LIBRARY/LIST/FULL/NAMES does, and checks the listing is
// LIBRARIAN's.
func TestListShareableLibrary(t *testing.T) {
	want := strings.Split(strings.TrimSuffix(string(readFixture(t, filepath.Join(link68Dir, "myshr.lis"))), "\n"), "\n")

	real, err := Open(readFixture(t, filepath.Join(link68Dir, "myshr.olb")))
	if err != nil {
		t.Fatal(err)
	}

	b, _ := Create(TypeShareable)
	entries := shareableModules(t, b, "shr1", "shr2")
	withRealTimes(t, b, real, entries)

	for _, e := range entries {
		_ = b.Insert(e)
	}

	ours, err := Open(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	for _, l := range []*Library{real, ours} {
		got, err := l.List(ListOptions{Name: "DUA1:[000000]MYSHR.OLB;1", Now: real.Created, Full: true, Names: true, Width: 132})
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != len(want) {
			t.Errorf("%d lines, want %d", len(got), len(want))
		}

		for i := range max(len(got), len(want)) {
			var g, w string
			if i < len(got) {
				g = got[i]
			}

			if i < len(want) {
				w = want[i]
			}

			if g != w {
				t.Errorf("line %d:\n got %q\nwant %q", i+1, g, w)
			}
		}
	}
}

// TestShareableModuleErrors checks what LIBRARY/SHARE refuses: an
// executable image, and a shareable image in another library's type.
func TestShareableModuleErrors(t *testing.T) {
	b, _ := Create(TypeShareable)
	if _, err := b.ShareableModule(readFixture(t, filepath.Join(link68Dir, "main1.exe"))); err == nil {
		t.Error("an executable image was taken")
	}

	o, _ := Create(TypeObject)
	if _, err := o.ShareableModule(readFixture(t, filepath.Join(link68Dir, "shr1.exe"))); err == nil {
		t.Error("an object library took a shareable image")
	}
}
