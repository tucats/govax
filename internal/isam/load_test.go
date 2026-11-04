package isam

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/backup"
)

// convertedFiles are round 3's files VMS's CONVERT fast-loaded with no failure,
// and what each file's $CREATE allocated: the input's or the FDL's allocation.
var convertedFiles = []struct {
	name string
	alq  uint32
}{
	{"C62.IDX", 246}, {"C63.IDX", 246}, {"C67A.IDX", 792}, {"C67B.IDX", 21}, {"C68.IDX", 3},
	{"C71.IDX", 246}, {"C73.IDX", 3}, {"C76.IDX", 3}, {"C77A.IDX", 246}, {"C77B.IDX", 3},
	{"C78A.IDX", 3}, {"C78B.IDX", 3}, {"C78C.IDX", 3}, {"C79.IDX", 3}, {"C94.IDX", 15},
	{"C95.IDX", 15}, {"C10E.IDX", 3},
}

// TestFastLoadAsVMS loads each of those files' records, in the primary key's order,
// into a new file made from its descriptors, and holds the result to VMS's:
// its Dump, then every block up to the end of the file's use.
func TestFastLoadAsVMS(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "idx88", "vax", "idx88c.bck"))
	if err != nil {
		t.Skipf("no save set: %v", err)
	}

	set, err := backup.Read(bytes.NewReader(data), int64(len(data)), backup.Options{})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range convertedFiles {
		var file *backup.File

		for _, f := range set.Files {
			if f.Name == "[R3]"+c.name+";1" {
				file = f
			}
		}

		if file == nil {
			t.Fatalf("%s isn't in the save set", c.name)
		}

		attr := attributesOf(file.RecordAttributes)

		vms, err := Open(memoryBlocks(file.Data()), attr)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}

		var records [][]byte

		for _, e := range entriesOf(t, c.name, vms, 0) {
			r, err := vms.Record(e.RFA)
			if err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}

			records = append(records, r)
		}

		out := &growingBlocks{cluster: volumeCluster}

		spec := specOf(vms, c.alq)
		for i := range spec.Keys {
			spec.Keys[i].Named = vms.Keys[i].Named
		}

		f, err := Create(out, spec)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}

		load := f.StartLoad(false)

		for i, r := range records {
			if _, err := load.Add(r); err != nil {
				t.Fatalf("%s record %d: %v", c.name, i, err)
			}
		}

		if err := load.FinishPrimary(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}

		for i := 1; i < len(f.Keys); i++ {
			if err := load.FinishAlternate(i); err != nil {
				t.Fatalf("%s key %d: %v", c.name, i, err)
			}
		}

		load.End()

		var a, b strings.Builder

		if err := vms.Dump(&a); err != nil {
			t.Fatal(err)
		}

		if err := f.Dump(&b); err != nil {
			t.Fatalf("%s: dump: %v", c.name, err)
		}

		if a.String() != b.String() {
			w, g := strings.Split(a.String(), "\n"), strings.Split(b.String(), "\n")

			for i := 0; i < min(len(w), len(g)); i++ {
				if w[i] != g[i] {
					t.Errorf("%s, dump line %d:\nVMS:   %s\ngovax: %s", c.name, i+1, w[i], g[i])

					break
				}
			}

			continue
		}

		if err := f.Check(); err != nil {
			t.Errorf("%s: Check: %v", c.name, err)
		}

		// Past a bucket's free offset VMS leaves what was in its buffer, and the
		// dump has shown everything else: the prolog is compared whole.
		have, want := out.data, file.Data()
		for i := 0; i < int(f.AreaVBN)*BlockSize; i += BlockSize {
			if !bytes.Equal(have[i:i+BlockSize], want[i:i+BlockSize]) {
				at := 0
				for have[i+at] == want[i+at] {
					at++
				}

				t.Errorf("%s: prolog block %d differs at %#x:\nVMS:   %s\ngovax: %s", c.name, i/BlockSize+1, at,
					hex.EncodeToString(want[i+at:min(i+at+24, i+BlockSize)]), hex.EncodeToString(have[i+at:min(i+at+24, i+BlockSize)]))

				break
			}
		}
	}
}
