package backup

import (
	"bytes"
	"fmt"
	"slices"
	"testing"
)

// rewrite writes s again: its summary, then each file's attributes and
// data, ending a block before each file named in breaks.
func rewrite(t *testing.T, s *SaveSet, crc bool, breaks []string, saved func(string)) []byte {
	t.Helper()

	var out bytes.Buffer

	w, err := NewWriter(&out, s.Summary, crc)
	if err != nil {
		t.Fatal(err)
	}

	w.Saved = saved

	for _, f := range s.Files {
		if slices.Contains(breaks, f.Name) {
			if err := w.EndBlock(); err != nil {
				t.Fatal(err)
			}
		}

		if err := w.Add(f.Attributes, f.Flags, f.Data()); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if w.Blocks() != s.Blocks {
		t.Errorf("%d blocks written, want %d", w.Blocks(), s.Blocks)
	}

	return out.Bytes()
}

// TestRewriteVAXSaveSets writes each of VMS's save sets again from what
// Read made of it, and wants VMS's bytes back: the summary, every
// attribute record, the data records split where VMS split them, the
// padding and null records, the block headers' current files, the
// redundancy blocks, the data CRCs, and the header checksums.
func TestRewriteVAXSaveSets(t *testing.T) {
	sets := []struct {
		name   string
		breaks []string
	}{
		{"pi", nil}, {"b69", nil}, {"b69k8", nil}, {"b69g0", nil}, {"b69g3", nil},
		{"b69crc", nil}, {"b69nocrc", nil}, {"b69com", nil}, {"b69exc", nil},
		{"b69byo", nil}, {"b69txt", nil}, {"b69none", nil}, {"text", nil},
		{"b69own", nil}, {"b69ign", nil},
		// An input list: each element starts a block.
		{"list", []string{"[B69.SUB1]A.TXT;1"}},
	}

	for _, set := range sets {
		t.Run(set.name, func(t *testing.T) {
			want := load(t, set.name+".bck")
			s := readSet(t, set.name, Options{CheckCRC: true})
			got := rewrite(t, s, ParseHeader(want).HasCRC(), set.breaks, nil)
			if len(got) != len(want) {
				t.Fatalf("%d bytes, want %d", len(got), len(want))
			}

			for i := range got {
				if got[i] != want[i] {
					block := i / s.BlockSize
					t.Fatalf("first difference in block %d at offset %d: %02X, want %02X", block+1, i%s.BlockSize, got[i], want[i])
				}
			}
		})
	}
}

// TestWriterSaved checks when Writer reports a file saved: once the
// block holding its last record leaves the buffers, so with B69's eight
// buffers every file is reported at Close, in order, and with one buffer
// each as its block is written.
func TestWriterSaved(t *testing.T) {
	s := readSet(t, "b69", Options{})

	var order []string

	rewrite(t, s, false, nil, func(name string) { order = append(order, name) })

	var names []string
	for _, f := range s.Files {
		names = append(names, f.Name)
	}

	if !slices.Equal(order, names) {
		t.Errorf("saved %v, want %v", order, names)
	}

	// With one buffer, a file is reported when its block is written: the
	// second block's start pushes the first out.
	s.Summary.Buffers = 1

	var (
		out    bytes.Buffer
		events []string
	)

	w, err := NewWriter(&out, s.Summary, false)
	if err != nil {
		t.Fatal(err)
	}

	w.Saved = func(name string) { events = append(events, fmt.Sprintf("%s@%d", name, out.Len()/s.BlockSize)) }

	for _, f := range s.Files[:3] {
		if err := w.Add(f.Attributes, f.Flags, f.Data()); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// ACL.TXT is in block 1, CONTIG.DAT ends in block 2, EMPTY.DAT too.
	want := []string{"[B69]ACL.TXT;1@1", "[B69]CONTIG.DAT;1@2", "[B69]EMPTY.DAT;1@2"}
	if !slices.Equal(events, want) {
		t.Errorf("events %v, want %v", events, want)
	}
}

// TestWriterBlockSize checks NewWriter's limits.
func TestWriterBlockSize(t *testing.T) {
	for _, size := range []uint32{0, MinBlockSize - 1, MaxBlockSize + 1} {
		if _, err := NewWriter(&bytes.Buffer{}, Summary{BlockSize: size}, false); err == nil {
			t.Errorf("block size %d: no error", size)
		}
	}
}
