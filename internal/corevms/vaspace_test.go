package corevms

import (
	"slices"
	"testing"
)

// The services themselves run against real page tables in
// internal/console's vaspace_test.go; these check the range arithmetic.

func TestPageRange(t *testing.T) {
	cases := []struct {
		r    pageRange
		want []uint32
	}{
		{pageRange{0x400, 0x400}, []uint32{0x400}},
		{pageRange{0x400, 0x800}, []uint32{0x400, 0x600, 0x800}},
		{pageRange{0x800, 0x400}, []uint32{0x800, 0x600, 0x400}},
		{pageRange{0, 0}, []uint32{0}},
	}

	for _, tc := range cases {
		if got := tc.r.pages(); !slices.Equal(got, tc.want) {
			t.Errorf("%#x.pages() = %#x, want %#x", tc.r, got, tc.want)
		}
	}
}

func TestReadRange(t *testing.T) {
	env, _ := fixture()

	putLongword(t, env, 0x6000, 0x1234) // the page at 0x1200
	putLongword(t, env, 0x6004, 0x1601) // the page at 0x1600

	if r, ok := env.readRange(0x6000, true); !ok || r.first != 0x1200 || r.last != 0x1600 {
		t.Errorf("forward = %#x, %v", r, ok)
	}

	if r, ok := env.readRange(0x6000, false); !ok || r.first != 0x1600 || r.last != 0x1200 {
		t.Errorf("reverse = %#x, %v", r, ok)
	}

	if _, ok := env.readRange(0x7FFF0000, true); ok {
		t.Error("an unreadable array was read")
	}
}

func TestStoreRetadr(t *testing.T) {
	env, _ := fixture()

	cases := []struct {
		done        []uint32
		first, last uint32
	}{
		{nil, 0xFFFFFFFF, 0xFFFFFFFF},
		{[]uint32{0x400}, 0x400, 0x5FF},
		{[]uint32{0x400, 0x600}, 0x400, 0x7FF},
		{[]uint32{0x600, 0x400}, 0x7FF, 0x400},
	}

	for _, tc := range cases {
		if !env.storeRetadr(0x6000, tc.done) {
			t.Fatal("storeRetadr failed")
		}

		got := longwordsAt(t, env, 0x6000, 2)
		if got[0] != tc.first || got[1] != tc.last {
			t.Errorf("done %#x: retadr %#x, want %#x-%#x", tc.done, got, tc.first, tc.last)
		}
	}

	if !env.storeRetadr(0, []uint32{0x400}) {
		t.Error("omitted retadr should succeed")
	}

	if env.storeRetadr(0x7FFF0000, nil) {
		t.Error("unwritable retadr should fail")
	}
}
