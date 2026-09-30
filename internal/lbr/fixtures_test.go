package lbr

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// The Phase 28 fixtures' libraries (testdata/mar/macros/vax), made by
// real LIBRARIAN on VMS 7.3 from the fixtures' sources and objects. govax's
// Builder, given the same inputs and times, must write the same bytes.

var macrosDir = filepath.Join("..", "..", "testdata", "mar", "macros")

func readFixture(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// withRealTimes gives b, and the entries it's about to insert, the real
// library's times and librarian name.
func withRealTimes(t *testing.T, b *Builder, real *Library, entries []*Entry) {
	t.Helper()

	b.Created, b.Updated, b.Librarian = real.Created, real.Updated, real.Librarian

	for _, e := range entries {
		rfa, ok := real.Lookup(e.Name)
		if !ok {
			t.Fatalf("the real library has no module %s", e.Name)
		}

		h, err := real.Header(rfa)
		if err != nil {
			t.Fatal(err)
		}

		e.Inserted = h.Inserted
	}
}

// requireSameLibrary compares govax's library with real LIBRARIAN's,
// byte for byte, naming the first block that differs.
func requireSameLibrary(t *testing.T, got, want []byte) {
	t.Helper()

	if bytes.Equal(got, want) {
		return
	}

	for vbn := 0; vbn*512 < max(len(got), len(want)); vbn++ {
		g := got[min(vbn*512, len(got)):min(vbn*512+512, len(got))]
		w := want[min(vbn*512, len(want)):min(vbn*512+512, len(want))]

		if !bytes.Equal(g, w) {
			for i := range min(len(g), len(w)) {
				if g[i] != w[i] {
					t.Fatalf("%d bytes, want %d; block %d (VBN %d) first differs at byte %d: %02X, want %02X\ngot:  % X\nwant: % X",
						len(got), len(want), vbn, vbn+1, i, g[i], w[i], g, w)
				}
			}

			t.Fatalf("%d bytes, want %d; block %d (VBN %d) is %d bytes, want %d", len(got), len(want), vbn, vbn+1, len(g), len(w))
		}
	}
}

// TestFixtureMacroLibrary builds LIBMAC.MLB from libmac.mar, as
// LIBRARY/CREATE/MACRO LIBMAC LIBMAC.MAR did.
func TestFixtureMacroLibrary(t *testing.T) {
	want := readFixture(t, filepath.Join(macrosDir, "vax", "libmac.mlb"))

	real, err := Open(want)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Create(TypeMacro)
	if err != nil {
		t.Fatal(err)
	}

	src := strings.TrimSuffix(string(readFixture(t, filepath.Join(macrosDir, "libmac.mar"))), "\n")

	entries, warns, err := b.MacroModules(strings.Split(src, "\n"), true)
	if err != nil || len(warns) > 0 {
		t.Fatalf("MacroModules: %v %v", warns, err)
	}

	withRealTimes(t, b, real, entries)

	for _, e := range entries {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	requireSameLibrary(t, b.Bytes(), want)
}

// TestFixtureObjectLibrary builds LIBOBJ.OLB from real MACRO's LIBSUB1
// and LIBSUB2 objects, as LIBRARY/CREATE LIBOBJ LIBSUB1,LIBSUB2 did.
func TestFixtureObjectLibrary(t *testing.T) {
	want := readFixture(t, filepath.Join(macrosDir, "vax", "libobj.olb"))

	real, err := Open(want)
	if err != nil {
		t.Fatal(err)
	}

	b, err := Create(TypeObject)
	if err != nil {
		t.Fatal(err)
	}

	var all []*Entry

	for _, name := range []string{"libsub1", "libsub2"} {
		f, err := os.Open(filepath.Join(macrosDir, "vax", name+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		records, err := obj.ReadRecords(f)
		f.Close()

		if err != nil {
			t.Fatal(err)
		}

		entries, err := b.ObjectModules(records, false)
		if err != nil {
			t.Fatal(err)
		}

		all = append(all, entries...)
	}

	withRealTimes(t, b, real, all)

	for _, e := range all {
		if err := b.Insert(e); err != nil {
			t.Fatal(err)
		}
	}

	requireSameLibrary(t, b.Bytes(), want)
}

// TestFixtureListings compares List with real LIBRARIAN's listings to a
// file (so 132 columns wide) of the fixture libraries: its own, and
// govax's (GV_*.MLB, GV_*.OLB, and the copies LIBRARIAN changed). The
// first line's time is the listing's own, so only what's before it is
// compared.
func TestFixtureListings(t *testing.T) {
	cases := []struct {
		name        string
		full, names bool
	}{
		{"libmac", true, false},
		{"libobj", true, true},
		{"gv_libmac", true, false},
		{"gv_libmac2", true, false},
		{"gv_libobj", true, true},
		{"gv_libobj2", true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ext := ".mlb"
			if strings.Contains(tc.name, "obj") {
				ext = ".olb"
			}

			l, err := Open(readFixture(t, filepath.Join(macrosDir, "vax", tc.name+ext)))
			if err != nil {
				t.Fatal(err)
			}

			want := strings.Split(strings.TrimRight(string(readFixture(t, filepath.Join(macrosDir, "vax", tc.name+".lls"))), "\n"), "\n")
			name := "DUA1:[000000]" + strings.ToUpper(tc.name+ext) + ";1"

			got, err := l.List(ListOptions{Name: name, Now: l.Updated, Full: tc.full, Names: tc.names, Width: 132})
			if err != nil {
				t.Fatal(err)
			}

			if len(got) > 0 && len(want) > 0 {
				g, _, _ := strings.Cut(got[0], " on ")
				w, _, _ := strings.Cut(want[0], " on ")
				got[0], want[0] = g, w
			}

			g, w := strings.TrimRight(strings.Join(got, "\n"), "\n"), strings.Join(want, "\n")
			if g != w {
				t.Errorf("listing:\n%s\nwant:\n%s", g, w)
			}
		})
	}
}
