package obj

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRealObjects reads every object module MACRO produced on the user's
// VAX (testdata/mar/vax/*.obj, in the host variable-length record layout;
// see docs/PHASE-27.md) and checks that it decodes, that encoding it again
// reproduces it byte for byte, and that Check finds nothing wrong. It
// skips when there are none yet.
func TestRealObjects(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "mar", "vax", "*"))
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, path := range paths {
		if !strings.EqualFold(filepath.Ext(path), ".obj") {
			continue
		}

		found = true

		t.Run(filepath.Base(path), func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			raw, err := ReadRecords(f)
			if err != nil {
				t.Fatal(err)
			}

			m, err := Decode(raw)
			if err != nil {
				t.Fatal(err)
			}

			again, err := Encode(m)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(raw, again) {
				for i := range min(len(raw), len(again)) {
					if !bytes.Equal(raw[i], again[i]) {
						t.Errorf("record %d re-encodes as\n% x\nnot\n% x", i+1, again[i], raw[i])

						break
					}
				}
			}

			for _, p := range Check(m) {
				t.Errorf("Check: %s", p)
			}
		})
	}

	if !found {
		t.Skip("no real VAX object modules in testdata/mar/vax yet")
	}
}
