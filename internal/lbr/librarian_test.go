package lbr

import (
	"bytes"
	"testing"
)

// TestLibrarianHistoryRecord holds LibrarianHistory to the record VMS 7.1's
// LIBRARIAN wrote in round 3's H90T.TLB (testdata/help90: "SYSTEM replaced
// 1 module", T2), and reads it back.
func TestLibrarianHistoryRecord(t *testing.T) {
	const when = 0x00BC5316E68587E0

	want := []byte{
		0xE0, 0x87, 0x85, 0xE6, 0x16, 0x53, 0xBC, 0x00, 0x01, 0x00, 0x03, 0x0C,
		'S', 'Y', 'S', 'T', 'E', 'M', ' ', ' ', ' ', ' ', ' ', ' ', 0x02, 'T', '2',
	}

	got := LibrarianHistory(when, HistoryReplaced, "system", []string{"T2"})
	if !bytes.Equal(got, want) {
		t.Fatalf("record:\n got % X\nwant % X", got, want)
	}

	h, ok := readLibrarianHistory(got)
	if !ok || h.time != when || h.op != HistoryReplaced || h.user != "SYSTEM" || len(h.names) != 1 || h.names[0] != "T2" {
		t.Fatalf("read back %+v, %v", h, ok)
	}

	// A record the LBR$ routines wrote isn't LIBRARIAN's (round 3's W5).
	if _, ok := readLibrarianHistory([]byte("A history record of the routines.")); ok {
		t.Error("a text record read as LIBRARIAN's")
	}
}

// TestHistoryListing makes a library, adds LIBRARIAN's records with an
// Updater, and lists them as LIBRARY/LIST/HISTORY does.
func TestHistoryListing(t *testing.T) {
	u, err := NewUpdater(UpdateOptions{Type: TypeText, KeyLength: 39, Indexes: 1, Entries: 8, History: 20})
	if err != nil {
		t.Fatal(err)
	}

	rfa := u.PutModule(&Entry{Name: "T2", Records: [][]byte{[]byte("Only line of text module T2.")}})

	key, err := u.Key(0, "T2")
	if err != nil {
		t.Fatal(err)
	}

	if err := u.InsertKey(0, key, rfa); err != nil {
		t.Fatal(err)
	}

	const when = 0x00BC5316E68587E0

	for _, rec := range [][]byte{
		LibrarianHistory(when, HistoryReplaced, "SYSTEM", []string{"T2"}),
		LibrarianHistory(when, HistoryDeleted, "SYSTEM", []string{"A", "B"}),
	} {
		if err := u.PutHistory(rec); err != nil {
			t.Fatal(err)
		}
	}

	l, err := u.Library()
	if err != nil {
		t.Fatal(err)
	}

	lines, err := l.List(ListOptions{Name: "H90T.TLB;1", History: true})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"T2",
		"",
		"Library Update History",
		"",
		"SYSTEM       replaced   1 module on  8-NOV-2026 14:35:44",
		"T2",
		"",
		"SYSTEM       deleted    2 modules on  8-NOV-2026 14:35:44",
		"A", // columns of 41 (the key length and 2): one fits in 80
		"B",
		"",
	}

	got := lines[len(lines)-len(want):]
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
