package asm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/obj"
)

// The Phase 28 fixtures (testdata/mar/macros): each assembled by govax,
// with the macro libraries real MACRO had, must give the object real
// MACRO gave (testdata/mar/macros/vax), record for record.

// macrosDir is where the Phase 28 fixtures are.
var macrosDir = filepath.Join("..", "..", "testdata", "mar", "macros")

// readObjectFile reads an object module kept in the host layout.
func readObjectFile(t *testing.T, path string) *obj.Module {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	raw, err := obj.ReadRecords(f)
	if err != nil {
		t.Fatal(err)
	}

	m, err := obj.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}

	return m
}

// openMacroFile opens a macro library file.
func openMacroFile(t *testing.T, path string) MacroLibrary {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	l, err := lbr.Open(data)
	if err != nil {
		t.Fatal(err)
	}

	lib, err := NewMacroLibrary(l)
	if err != nil {
		t.Fatal(err)
	}

	return lib
}

// realStarletLibrary is the real STARLET.MLB as a MacroLibrary, skipping
// the test without it.
func realStarletLibrary(t *testing.T) MacroLibrary {
	t.Helper()

	lib, err := NewMacroLibrary(openStarlet(t))
	if err != nil {
		t.Fatal(err)
	}

	return lib
}

func TestMacroFixtureObjects(t *testing.T) {
	cases := []struct {
		name string
		// libraries returns the libraries MACRO searched on the VAX, or
		// skips the test when one is missing.
		libraries func(t *testing.T) []MacroLibrary
	}{
		{"usermac", nil},
		{"qiow", func(t *testing.T) []MacroLibrary { return []MacroLibrary{govaxStarlet(t)} }},
		{"qiow", func(t *testing.T) []MacroLibrary { return []MacroLibrary{realStarletLibrary(t)} }},
		{"rmscopy", func(t *testing.T) []MacroLibrary { return []MacroLibrary{realStarletLibrary(t)} }},
		{"fabalign", func(t *testing.T) []MacroLibrary { return []MacroLibrary{realStarletLibrary(t)} }},
		{"fabalign", func(t *testing.T) []MacroLibrary { return []MacroLibrary{govaxStarlet(t)} }},
		{"uselib", func(t *testing.T) []MacroLibrary {
			return []MacroLibrary{openMacroFile(t, filepath.Join(macrosDir, "vax", "libmac.mlb"))}
		}},
		{"libsub1", nil},
		{"libsub2", nil},
		{"libmain", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(macrosDir, tc.name+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			a := macroAssembler()
			if tc.libraries != nil {
				a.SetMacroLibraries(tc.libraries(t)...)
			}

			if _, err := a.Assemble(string(src)); err != nil {
				t.Fatalf("assemble: %v", err)
			}

			requireSameObject(t, a, readObjectFile(t, filepath.Join(macrosDir, "vax", tc.name+".obj")))
		})
	}
}
