package lbr

import (
	"bytes"
	"path/filepath"
	"testing"
)

// for83Library reads one of the libraries VMS 7.1's LIBRARY/TEXT made
// (testdata/for83's part D).
func for83Library(t *testing.T, name string) (*Library, []byte) {
	t.Helper()

	data := readFixture(t, filepath.Join("..", "..", "testdata", "for83", "vax", name))

	l, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}

	return l, data
}

// The files part D makes its modules from (testdata/for83/for83.com).
var (
	textA = [][]byte{[]byte("      INTEGER*4 A"), []byte("      PARAMETER (A = 1)")}
	textC = [][]byte{[]byte("      C ONE"), []byte("      C TWO")}
)

// TestFor83NewLibraries makes the libraries one command makes, and matches
// VMS's byte for byte: its times and its librarian's name are all that's
// taken from them.
func TestFor83NewLibraries(t *testing.T) {
	cases := []struct {
		file string
		opts CreateOptions
		mods []*Entry
	}{
		{"s1.tlb", CreateOptions{}, []*Entry{TextModule("TXA", textA)}},
		{"t2.tlb", CreateOptions{Modules: 40, KeySize: 31}, []*Entry{TextModule("TXA", textA), TextModule("TXC", textC)}},
		{"t3.tlb", CreateOptions{}, []*Entry{TextModule("M", textA)}},
	}

	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			real, data := for83Library(t, c.file)

			b, err := CreateWith(TypeText, c.opts)
			if err != nil {
				t.Fatal(err)
			}

			withRealTimes(t, b, real, c.mods)

			for _, e := range c.mods {
				if err := b.Insert(e); err != nil {
					t.Fatal(err)
				}
			}

			requireSameLibrary(t, b.Bytes(), data)
		})
	}
}

// TestFor83Modules reads VMS's text libraries: the module part D's empty file
// made is one empty record, and blanks, tabs, and an empty line stay, here and
// in the data-reduced library.
func TestFor83Modules(t *testing.T) {
	for _, file := range []string{"s7.tlb", "s9.tlb"} {
		l, _ := for83Library(t, file)

		rfa, ok := l.Lookup("TXD")
		if !ok {
			t.Fatalf("%s has no TXD", file)
		}

		m, err := l.Module(rfa)
		if err != nil {
			t.Fatal(err)
		}

		if len(m.Records) != 1 || len(m.Records[0]) != 0 {
			t.Errorf("%s: TXD's records: %q", file, m.Records)
		}

		rfa, _ = l.Lookup("$MOD_B")

		if m, err = l.Module(rfa); err != nil {
			t.Fatal(err)
		}

		if len(m.Records) != 4 || len(m.Records[1]) != 0 || len(m.Records[2]) != 132 || !bytes.Equal(m.Records[3], []byte("\tTAB FIRST")) {
			t.Errorf("%s: $MOD_B's records: %q", file, m.Records)
		}
	}
}

// TestTextModuleEmpty checks an empty file's module against VMS's.
func TestTextModuleEmpty(t *testing.T) {
	if e := TextModule("X", nil); len(e.Records) != 1 || len(e.Records[0]) != 0 {
		t.Errorf("records %q", e.Records)
	}
}

// TestEditKeepsPreallocation checks that a change leaves a library's reserved
// index blocks as /CREATE made them.
func TestEditKeepsPreallocation(t *testing.T) {
	real, _ := for83Library(t, "t2.tlb")

	b, err := Edit(real)
	if err != nil {
		t.Fatal(err)
	}

	if err := b.Insert(TextModule("MORE", textC)); err != nil {
		t.Fatal(err)
	}

	l := checkLayout(t, b.Bytes())
	if l.Preallocated != 3 || l.KeySize() != 31 {
		t.Errorf("%d blocks preallocated, keys of %d", l.Preallocated, l.KeySize())
	}
}
