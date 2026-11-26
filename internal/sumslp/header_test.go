package sumslp

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHeadedAsVMS holds /HEADER's records to the ones VMS 7.1 wrote (the
// probe's U11 to U14, testdata/sum117/vax/headers.txt), fixed control and
// all: the input line's number or 0, the insert's number or the "-n" count,
// and the audit trail in 8 bytes.
func TestHeadedAsVMS(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join(probeDir, "vax", "headers.txt"))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string][][]byte{}

	var id string

	for line := range strings.SplitSeq(string(data), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, " "):
			b, err := hex.DecodeString(strings.TrimSpace(line))
			if err != nil {
				t.Fatal(err)
			}

			want[id] = append(want[id], b)
		default:
			id = line
		}
	}

	files := probeFiles(t)

	for _, c := range []struct {
		id      string
		updates []string
	}{
		{"U11", []string{"U11"}},
		{"U12", []string{"U02"}},
		{"U13", nil},
		{"U14", []string{"G12A", "G12B"}},
	} {
		var updates []Update
		for _, u := range c.updates {
			updates = append(updates, Update{Name: u, Records: files[u+".UPD"]})
		}

		got := Apply(files["S10.SRC"], updates).Headed()

		if !equalRecords(got, want[c.id]) {
			t.Errorf("%s: govax's records\n%X\nVMS's\n%X", c.id, bytes.Join(got, []byte("\n")), bytes.Join(want[c.id], []byte("\n")))
		}
	}
}
