package vm

import "testing"

// TestAllocatePage_skipsPageZero matches vm.c's own validate_page/
// mapped_pages loops, which both start their search/count at physical page
// index 1 -- physical page 0 is always claimed directly by VMINIT's S0
// identity map (see internal/console/vminit.go), never handed out by the
// demand-paging allocator.
func TestAllocatePage_skipsPageZero(t *testing.T) {
	m := NewMemory(4 * pageSize) // 4 physical pages: 0-3

	pfn, ok := m.AllocatePage()
	if !ok {
		t.Fatal("AllocatePage: want ok, got false")
	}
	if pfn == 0 {
		t.Error("AllocatePage returned physical page 0, want it reserved/unallocatable")
	}
}

func TestAllocatePage_firstFitInOrder(t *testing.T) {
	m := NewMemory(4 * pageSize)

	var got []uint32
	for i := 0; i < 3; i++ {
		pfn, ok := m.AllocatePage()
		if !ok {
			t.Fatalf("AllocatePage call %d: want ok, got false", i)
		}
		got = append(got, pfn)
	}

	want := []uint32{1, 2, 3}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("AllocatePage sequence = %v, want %v", got, want)
			break
		}
	}
}

func TestAllocatePage_exhausted(t *testing.T) {
	m := NewMemory(2 * pageSize) // physical pages 0-1; only page 1 is allocatable

	if _, ok := m.AllocatePage(); !ok {
		t.Fatal("first AllocatePage: want ok, got false")
	}
	if _, ok := m.AllocatePage(); ok {
		t.Error("second AllocatePage: want exhausted (false), got a page")
	}
}

func TestReservePage_excludesFromAllocate(t *testing.T) {
	m := NewMemory(4 * pageSize)

	m.ReservePage(1)

	pfn, ok := m.AllocatePage()
	if !ok {
		t.Fatal("AllocatePage: want ok, got false")
	}
	if pfn != 2 {
		t.Errorf("AllocatePage = %d, want 2 (page 1 already reserved)", pfn)
	}
}

func TestMappedPages_countsReservedAndAllocatedButNotPageZero(t *testing.T) {
	m := NewMemory(4 * pageSize)

	if got := m.MappedPages(); got != 0 {
		t.Errorf("MappedPages on fresh Memory = %d, want 0", got)
	}

	m.ReservePage(0) // page 0 is never counted, matching mapped_pages()
	m.ReservePage(1)

	if _, ok := m.AllocatePage(); !ok {
		t.Fatal("AllocatePage: want ok, got false")
	}

	if got := m.MappedPages(); got != 2 {
		t.Errorf("MappedPages = %d, want 2 (page 1 reserved + one allocated, page 0 excluded)", got)
	}
}
