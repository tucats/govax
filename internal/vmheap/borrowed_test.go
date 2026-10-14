package vmheap

import (
	"errors"
	"testing"
)

// fakeRoutines stands in for a program's get-page and free-page
// routines: pages handed out upward from base, every call recorded.
type fakeRoutines struct {
	next  uint32
	gets  []uint32 // each Get's page count
	puts  []span   // each Put's pages
	fail  error    // Get's error, when set
	give  uint32   // Get's address instead of the next pages, when set
	refus error    // Put's error, when set
}

func (f *fakeRoutines) borrowed() *Borrowed {
	return &Borrowed{
		Get: func(n uint32) (uint32, error) {
			f.gets = append(f.gets, n)
			if f.fail != nil {
				return 0, f.fail
			}

			if f.give != 0 {
				return f.give, nil
			}

			addr := f.next
			f.next += n * PageSize

			return addr, nil
		},
		Put: func(addr, n uint32) error {
			f.puts = append(f.puts, span{addr, addr + n*PageSize})

			return f.refus
		},
	}
}

// A zone on borrowed pages takes each area from Get, and Delete gives
// each back through Put, an area that grew by ExtendArea as one call.
func TestBorrowedPages(t *testing.T) {
	f := &fakeRoutines{next: 0x20000}
	b := f.borrowed()

	z, err := NewZone(b, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 2, InitialPages: 1, ExtendArea: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(f.gets) != 1 || f.gets[0] != 1 {
		t.Fatalf("Get calls %v, want one of 1 page (the initial area)", f.gets)
	}

	mustCheck(t, z)

	// More than the first page holds: a second Get, whose pages adjoin
	// the first area and join it.
	a := mustAllocate(t, z, 400)
	c := mustAllocate(t, z, 400)

	if len(f.gets) != 2 || f.gets[1] != 2 || z.Areas() != 1 || b.LentPages() != 3 {
		t.Fatalf("Get calls %v, %d areas, %d pages lent; want [1 2], 1, 3", f.gets, z.Areas(), b.LentPages())
	}

	mustCheck(t, z)

	freed, err := z.Delete()
	if err != nil || len(freed) != 2 || freed[0].Addr != a || freed[1].Addr != c {
		t.Fatalf("Delete = %v, %v; want blocks %X and %X", freed, err, a, c)
	}

	if len(f.puts) != 1 || f.puts[0] != (span{0x20000, 0x20000 + 3*PageSize}) || b.LentPages() != 0 {
		t.Errorf("Put calls %v, %d pages still lent; want one of 3 pages at 20000, none", f.puts, b.LentPages())
	}
}

// Get's failure is the allocation's, with nothing changed; pages a
// zone can't use are ErrBadPages; Put's failure is Delete's, every area
// given back all the same.
func TestBorrowedFailures(t *testing.T) {
	refused := errors.New("refused")

	f := &fakeRoutines{next: 0x20000}
	b := f.borrowed()

	z, err := NewZone(b, Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1})
	if err != nil {
		t.Fatal(err)
	}

	f.fail = refused

	if _, err := z.Allocate(8); !errors.Is(err, refused) {
		t.Errorf("Allocate with Get failing = %v, want its error", err)
	}

	if z.Areas() != 0 || b.LentPages() != 0 {
		t.Errorf("%d areas, %d pages after a failed Get; want none", z.Areas(), b.LentPages())
	}

	f.fail = nil
	a := mustAllocate(t, z, 8)

	for _, give := range []uint32{0x20000 + 0x10, 0x20000, 0xFFFFFE00} {
		f.give = give

		if _, err := z.Allocate(PageSize); !errors.Is(err, ErrBadPages) {
			t.Errorf("Allocate with Get giving %X = %v, want ErrBadPages", give, err)
		}
	}

	f.give = 0

	mustAllocate(t, z, PageSize) // a second area
	mustCheck(t, z)

	f.refus = refused

	if _, err := z.Delete(); !errors.Is(err, refused) {
		t.Errorf("Delete with Put failing = %v, want its error", err)
	}

	if len(f.puts) != 2 || b.LentPages() != 0 {
		t.Errorf("%d Put calls, %d pages lent after Delete; want 2, 0", len(f.puts), b.LentPages())
	}

	if err := b.Reclaim(a&^(PageSize-1), 1); !errors.Is(err, ErrBadAddress) {
		t.Errorf("Reclaim of pages given back = %v, want ErrBadAddress", err)
	}
}
