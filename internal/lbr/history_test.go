package lbr

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
)

// TestUpdateHistoryW5 replays round 3's W5 (testdata/help90): five records
// with LUHMAX 3, an empty one, one too long, and a sixth, and checks the
// records kept and the header's RFAs against VMS 7.1's file.
func TestUpdateHistoryW5(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 96, History: 3})
	if err != nil {
		t.Fatal(err)
	}

	lib, _ := u.Library()
	if recs, err := lib.UpdateHistory(); err != nil || len(recs) != 0 {
		t.Fatalf("a new library's history: %q, %v", recs, err)
	}

	for _, r := range []string{"First history record.", "Second history record.", "Third.", "Fourth.", "Fifth, which leaves three.", ""} {
		if err := u.PutHistory([]byte(r)); err != nil {
			t.Fatal(err)
		}
	}

	if err := u.PutHistory(make([]byte, maxRecSize+1)); !errors.Is(err, ErrRecordLength) {
		t.Fatalf("a long record: %v", err)
	}

	if err := u.PutHistory([]byte("Sixth, after reopening.")); err != nil {
		t.Fatal(err)
	}

	lib, _ = u.Library()

	recs, err := lib.UpdateHistory()
	if err != nil {
		t.Fatal(err)
	}

	if got := fmt.Sprintf("%q", recs); got != `["Fifth, which leaves three." "" "Sixth, after reopening."]` {
		t.Errorf("records %s", got)
	}

	// VMS's W5: the head at block 9, 0x48 from the data, and the tail
	// at 0x85; three records of three.
	h := u.data[:blockSize]
	if got := h[lhdHisHead : lhdHisTail+6]; !bytes.Equal(got, []byte{9, 0, 0, 0, 0x48, 0, 9, 0, 0, 0, 0x85, 0}) {
		t.Errorf("header RFAs % X", got)
	}

	if n := le16(h, lhdNumLUHRec); n != 3 {
		t.Errorf("%d records, want 3", n)
	}
}

// TestUpdateHistoryBlocks fills more than a block with records and holds
// what's read back to a model: the newest LUHMAX records, oldest first.
func TestUpdateHistoryBlocks(t *testing.T) {
	const most = 7

	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 96, History: most})
	if err != nil {
		t.Fatal(err)
	}

	var model [][]byte

	for i := range 200 {
		rec := bytes.Repeat([]byte{byte('A' + i%26)}, (i*37)%300)
		if err := u.PutHistory(rec); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}

		if model = append(model, rec); len(model) > most {
			model = model[1:]
		}

		lib, err := u.Library()
		if err != nil {
			t.Fatal(err)
		}

		recs, err := lib.UpdateHistory()
		if err != nil {
			t.Fatalf("after record %d: %v", i, err)
		}

		if len(recs) != len(model) {
			t.Fatalf("after record %d: %d records, want %d", i, len(recs), len(model))
		}

		for j := range recs {
			if !bytes.Equal(recs[j], model[j]) {
				t.Fatalf("after record %d: record %d differs", i, j)
			}
		}
	}
}

// TestNoHistory: a library made with LUHMAX 0 keeps none.
func TestNoHistory(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 31, Indexes: 1, Entries: 96})
	if err != nil {
		t.Fatal(err)
	}

	if err := u.PutHistory([]byte("x")); !errors.Is(err, ErrNoHistory) {
		t.Errorf("PutHistory: %v", err)
	}

	lib, _ := u.Library()
	if _, err := lib.UpdateHistory(); !errors.Is(err, ErrNoHistory) {
		t.Errorf("UpdateHistory: %v", err)
	}
}
