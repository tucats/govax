package paging

import (
	"errors"
	"math/rand"
	"testing"
)

// tieLog is a TieBreaker that records each tie it's asked to break.
type tieLog struct {
	ties []brokenTie
}

type brokenTie struct {
	pfn   PFN
	typ   Type
	owner Owner
	back  Backing
}

func (t *tieLog) BreakTie(pfn PFN, typ Type, owner Owner, back Backing) {
	t.ties = append(t.ties, brokenTie{pfn, typ, owner, back})
}

// check fails the test if the database breaks a rule.
func check(t *testing.T, d *Database) {
	t.Helper()

	if err := d.Check(); err != nil {
		t.Fatal(err)
	}
}

// must fails the test on an unexpected error, then checks the database.
func must(t *testing.T, d *Database, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}

	check(t, d)
}

// allocate takes a frame for process page va of process 1, demand-zero.
func allocate(t *testing.T, d *Database, va uint32) PFN {
	t.Helper()

	pfn, err := d.Allocate(ProcessPage, Owner{Handle: 1, VA: va}, Backing{})
	must(t, d, err)

	return pfn
}

// release releases a frame, expecting it to land in state want.
func release(t *testing.T, d *Database, pfn PFN, modified bool, want State) {
	t.Helper()

	got, err := d.Release(pfn, modified)
	must(t, d, err)

	if got != want {
		t.Fatalf("frame %d released to %s, want %s", pfn, got, want)
	}
}

// entry returns a frame's record.
func entry(t *testing.T, d *Database, pfn PFN) Entry {
	t.Helper()

	e, err := d.Entry(pfn)
	if err != nil {
		t.Fatal(err)
	}

	return e
}

// freeOrder is the free list from head to tail.
func freeOrder(d *Database) []PFN {
	var out []PFN
	for pfn := d.free.head; pfn != noFrame; pfn = d.flink[pfn] {
		out = append(out, pfn)
	}

	return out
}

func samePFNs(a, b []PFN) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestNewIsAllFreeInOrder(t *testing.T) {
	d := New(8)
	check(t, d)

	if got := (Counts{Free: 8}); d.Counts() != got {
		t.Fatalf("counts %+v, want %+v", d.Counts(), got)
	}

	if got := freeOrder(d); !samePFNs(got, []PFN{0, 1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("free list %v", got)
	}

	for want := range PFN(3) {
		if got := allocate(t, d, 0x200*uint32(want)); got != want {
			t.Fatalf("allocated frame %d, want %d", got, want)
		}
	}

	e := entry(t, d, 1)
	if e.State != Active || !e.Tied || e.Owner != (Owner{1, 0x200}) || e.RefCount != 1 || e.SavedModify {
		t.Fatalf("allocated frame's record %+v", e)
	}

	if c := d.Counts(); c.Free != 5 || c.Process != 3 || c.Active() != 3 {
		t.Fatalf("counts %+v", c)
	}
}

func TestReserve(t *testing.T) {
	d := New(6)

	for _, pfn := range []PFN{0, 1, 4} {
		must(t, d, d.Reserve(pfn))
	}

	if err := d.Reserve(1); !errors.Is(err, ErrWrongState) {
		t.Fatalf("second reserve: %v", err)
	}

	if err := d.Reserve(6); !errors.Is(err, ErrBadFrame) {
		t.Fatalf("reserve beyond the frames: %v", err)
	}

	e := entry(t, d, 4)
	if e.State != Active || e.Type != SystemPage || e.Tied || e.RefCount != 1 {
		t.Fatalf("reserved frame's record %+v", e)
	}

	if got := []PFN{allocate(t, d, 0), allocate(t, d, 0x200), allocate(t, d, 0x400)}; !samePFNs(got, []PFN{2, 3, 5}) {
		t.Fatalf("allocations after reserving took %v", got)
	}

	if _, err := d.Allocate(ProcessPage, Owner{1, 0x600}, Backing{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("allocation from an empty free list: %v", err)
	}

	if c := d.Counts(); c.System != 3 || c.Process != 3 || c.Free != 0 {
		t.Fatalf("counts %+v", c)
	}

	// A reserved frame has no page to fault it back, so it can't be
	// released to a list, only given back.
	if _, err := d.Release(4, true); !errors.Is(err, ErrWrongState) {
		t.Fatalf("release of a reserved frame: %v", err)
	}

	must(t, d, d.Delete(4))

	if got := freeOrder(d); !samePFNs(got, []PFN{4}) || d.Counts().System != 2 {
		t.Fatalf("free list %v, counts %+v", got, d.Counts())
	}
}

// A released frame goes to the tail of the list its saved modify bit
// chooses, keeping its tie; frames are taken from the free list's head,
// so the first released is the first reused.
func TestReleaseToListTails(t *testing.T) {
	d := New(4)
	a, b, c, e := allocate(t, d, 0), allocate(t, d, 0x200), allocate(t, d, 0x400), allocate(t, d, 0x600)

	release(t, d, b, false, Free)
	release(t, d, a, true, Modified)
	release(t, d, e, false, Free)
	release(t, d, c, true, Modified)

	if got := freeOrder(d); !samePFNs(got, []PFN{b, e}) {
		t.Fatalf("free list %v, want [%d %d]", got, b, e)
	}

	if first, ok := d.FirstModified(); !ok || first != a {
		t.Fatalf("modified list's head %d %v, want %d", first, ok, a)
	}

	rec := entry(t, d, a)
	if rec.State != Modified || !rec.Tied || rec.Owner != (Owner{1, 0}) || !rec.SavedModify || rec.RefCount != 0 {
		t.Fatalf("modified frame's record %+v", rec)
	}

	if c := d.Counts(); c.Free != 2 || c.Modified != 2 || c.Active() != 0 {
		t.Fatalf("counts %+v", c)
	}

	if d.FreeCount() != 2 || d.ModifiedCount() != 2 {
		t.Fatalf("free %d, modified %d", d.FreeCount(), d.ModifiedCount())
	}

	if _, err := d.Release(a, false); !errors.Is(err, ErrWrongState) {
		t.Fatalf("release of a frame on a list: %v", err)
	}
}

// The saved modify bit, once set, stays set through a release with the
// PTE's bit clear: SetModified's "effectively modified" pages.
func TestSavedModifySticks(t *testing.T) {
	d := New(2)
	pfn := allocate(t, d, 0)
	must(t, d, d.SetModified(pfn))
	release(t, d, pfn, false, Modified)

	if err := d.SetModified(pfn); !errors.Is(err, ErrWrongState) {
		t.Fatalf("set modified on a listed frame: %v", err)
	}
}

// Reusing a tied frame breaks its old tie, telling the TieBreaker where
// the page's contents are, and starts the frame's record afresh.
func TestAllocateBreaksTie(t *testing.T) {
	d := New(1)
	log := &tieLog{}
	d.SetTieBreaker(log)

	back := Backing{Kind: SectionFile, Index: 3, Block: 17}

	pfn, err := d.Allocate(ProcessPage, Owner{7, 0x1000}, back)
	must(t, d, err)

	if len(log.ties) != 0 {
		t.Fatalf("first allocation broke ties %v", log.ties)
	}

	release(t, d, pfn, false, Free)

	again, err := d.Allocate(GlobalPage, Owner{0, 5}, Backing{Kind: GlobalSection, Index: 5})
	must(t, d, err)

	want := []brokenTie{{pfn, ProcessPage, Owner{7, 0x1000}, back}}
	if again != pfn || len(log.ties) != 1 || log.ties[0] != want[0] {
		t.Fatalf("reuse of frame %d (got %d) broke ties %v, want %v", pfn, again, log.ties, want)
	}

	if e := entry(t, d, pfn); e.Type != GlobalPage || e.Owner != (Owner{0, 5}) || e.Backing.Kind != GlobalSection {
		t.Fatalf("reused frame's record %+v", e)
	}
}

// Reserving a tied free frame breaks its tie too.
func TestReserveBreaksTie(t *testing.T) {
	d := New(1)
	log := &tieLog{}
	d.SetTieBreaker(log)

	release(t, d, allocate(t, d, 0x400), false, Free)
	must(t, d, d.Reserve(0))

	if len(log.ties) != 1 || log.ties[0].owner != (Owner{1, 0x400}) {
		t.Fatalf("ties broken %v", log.ties)
	}
}

// A deleted page's frame loses its tie (with no TieBreaker call: the page
// is gone) and goes to the free list's head, to be reused first.
func TestDeleteToFreeHead(t *testing.T) {
	d := New(5)
	log := &tieLog{}
	d.SetTieBreaker(log)

	a, b, c := allocate(t, d, 0), allocate(t, d, 0x200), allocate(t, d, 0x400)
	release(t, d, b, true, Modified)
	release(t, d, c, false, Free)

	must(t, d, d.Delete(a)) // active
	must(t, d, d.Delete(b)) // on the modified list

	if got := freeOrder(d); !samePFNs(got, []PFN{b, a, 3, 4, c}) {
		t.Fatalf("free list %v", got)
	}

	if e := entry(t, d, b); e.Tied || e.SavedModify || e.Backing != (Backing{}) {
		t.Fatalf("deleted frame's record %+v", e)
	}

	if got := allocate(t, d, 0x600); got != b {
		t.Fatalf("allocation after delete took %d, want %d", got, b)
	}

	if len(log.ties) != 0 {
		t.Fatalf("deleted frames broke ties %v", log.ties)
	}

	must(t, d, d.Delete(c)) // on the free list, from the tail to the head

	if got := freeOrder(d); !samePFNs(got, []PFN{c, a, 3, 4}) {
		t.Fatalf("free list %v", got)
	}

	held := allocate(t, d, 0x800)
	must(t, d, d.Hold(held))

	if err := d.Delete(held); !errors.Is(err, ErrWrongState) {
		t.Fatalf("delete of a held frame: %v", err)
	}
}

// A soft fault takes a frame back from the middle of either list, keeping
// its saved modify bit.
func TestReclaim(t *testing.T) {
	d := New(6)

	var pfns [6]PFN
	for i := range pfns {
		pfns[i] = allocate(t, d, uint32(i)*0x200)
	}

	for i, pfn := range pfns {
		release(t, d, pfn, i%2 == 1, []State{Free, Modified}[i%2])
	}

	must(t, d, d.Reclaim(pfns[2])) // the middle of the free list
	must(t, d, d.Reclaim(pfns[1])) // the head of the modified list
	must(t, d, d.Reclaim(pfns[5])) // the tail of the modified list

	if got := freeOrder(d); !samePFNs(got, []PFN{pfns[0], pfns[4]}) {
		t.Fatalf("free list %v", got)
	}

	if e := entry(t, d, pfns[1]); e.State != Active || e.RefCount != 1 || !e.SavedModify || e.Owner.VA != 0x200 {
		t.Fatalf("reclaimed modified frame's record %+v", e)
	}

	release(t, d, pfns[1], false, Modified) // still effectively modified
	release(t, d, pfns[2], false, Free)

	if err := d.Reclaim(pfns[5]); !errors.Is(err, ErrWrongState) {
		t.Fatalf("reclaim of an active frame: %v", err)
	}

	must(t, d, d.Delete(pfns[0]))

	if err := d.Reclaim(pfns[0]); !errors.Is(err, ErrWrongState) {
		t.Fatalf("reclaim of an untied frame: %v", err)
	}
}

func TestHold(t *testing.T) {
	d := New(2)
	pfn := allocate(t, d, 0)
	must(t, d, d.Hold(pfn))

	release(t, d, pfn, true, Active) // the hold keeps it
	release(t, d, pfn, false, Modified)

	if err := d.Hold(pfn); !errors.Is(err, ErrWrongState) {
		t.Fatalf("hold of a listed frame: %v", err)
	}
}

// A global page's shares count as one reference while there are any.
func TestShare(t *testing.T) {
	d := New(3)

	pfn, err := d.Allocate(GlobalPage, Owner{0, 9}, Backing{Kind: GlobalSection, Index: 9})
	must(t, d, err)

	for range 3 {
		must(t, d, d.Share(pfn))
	}

	if e := entry(t, d, pfn); e.ShareCount != 3 || e.RefCount != 2 {
		t.Fatalf("shared frame's record %+v", e)
	}

	release(t, d, pfn, false, Active) // the owner's reference goes

	if _, err := d.Release(pfn, false); !errors.Is(err, ErrWrongState) {
		t.Fatalf("release of a shared frame's last reference: %v", err)
	}

	for _, modified := range []bool{false, true} {
		got, err := d.Unshare(pfn, modified)
		must(t, d, err)

		if got != Active {
			t.Fatalf("unshare left the frame %s", got)
		}
	}

	got, err := d.Unshare(pfn, false)
	must(t, d, err)

	if got != Modified {
		t.Fatalf("last unshare left the frame %s, want modified (one process wrote it)", got)
	}

	if c := d.Counts(); c.Global != 0 || c.Modified != 1 {
		t.Fatalf("counts %+v", c)
	}

	must(t, d, d.Reclaim(pfn))

	if _, err := d.Unshare(pfn, false); !errors.Is(err, ErrNotShared) {
		t.Fatalf("unshare of an unshared frame: %v", err)
	}

	if err := d.Share(allocate(t, d, 0)); !errors.Is(err, ErrWrongState) {
		t.Fatalf("share of a process page: %v", err)
	}
}

// A global page's first share takes over the reference its allocation
// (or a reclaim) gave it, so the frame goes to a list with its last
// share, and comes back with the next first share.
func TestShareFirst(t *testing.T) {
	d := New(3)

	pfn, err := d.Allocate(GlobalPage, Owner{1, 0}, Backing{Kind: GlobalSection, Index: 4})
	must(t, d, err)
	must(t, d, d.ShareFirst(pfn))

	if e := entry(t, d, pfn); e.ShareCount != 1 || e.RefCount != 1 {
		t.Fatalf("first share's record %+v", e)
	}

	if err := d.ShareFirst(pfn); !errors.Is(err, ErrWrongState) {
		t.Fatalf("a second first share: %v", err)
	}

	must(t, d, d.Share(pfn))

	if e := entry(t, d, pfn); e.ShareCount != 2 || e.RefCount != 1 {
		t.Fatalf("second share's record %+v", e)
	}

	for _, want := range []State{Active, Modified} {
		got, err := d.Unshare(pfn, want == Active)
		must(t, d, err)

		if got != want {
			t.Fatalf("unshare left the frame %s, want %s", got, want)
		}
	}

	must(t, d, d.Reclaim(pfn))
	must(t, d, d.ShareFirst(pfn))

	if e := entry(t, d, pfn); e.State != Active || e.ShareCount != 1 || e.RefCount != 1 || !e.SavedModify {
		t.Fatalf("reclaimed frame's record %+v", e)
	}

	if err := d.ShareFirst(allocate(t, d, 0)); !errors.Is(err, ErrWrongState) {
		t.Fatalf("first share of a process page: %v", err)
	}

	if err := d.ShareFirst(9); !errors.Is(err, ErrBadFrame) {
		t.Fatalf("first share of no frame: %v", err)
	}
}

// The writer's path: the modified list's head is the oldest, and a
// cleaned frame goes to the free list's tail, still tied and faultable.
func TestCleaned(t *testing.T) {
	d := New(3)
	a, b, c := allocate(t, d, 0), allocate(t, d, 0x200), allocate(t, d, 0x400)
	release(t, d, c, true, Modified)
	release(t, d, a, true, Modified)
	release(t, d, b, false, Free)

	if _, err := d.Allocate(ProcessPage, Owner{1, 0x600}, Backing{}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Allocate(ProcessPage, Owner{1, 0x800}, Backing{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("allocation with only modified frames left: %v", err)
	}

	first, ok := d.FirstModified()
	if !ok || first != c {
		t.Fatalf("first modified %d %v, want %d", first, ok, c)
	}

	must(t, d, d.SetBacking(c, Backing{Kind: PageFile, Block: 40}))
	must(t, d, d.Cleaned(c))

	if first, _ = d.FirstModified(); first != a {
		t.Fatalf("first modified %d, want %d", first, a)
	}

	e := entry(t, d, c)
	if e.State != Free || e.SavedModify || !e.Tied || e.Backing != (Backing{Kind: PageFile, Block: 40}) {
		t.Fatalf("cleaned frame's record %+v", e)
	}

	must(t, d, d.Cleaned(a))

	if _, ok := d.FirstModified(); ok {
		t.Fatal("modified list not empty")
	}

	if got := freeOrder(d); !samePFNs(got, []PFN{c, a}) {
		t.Fatalf("free list %v", got)
	}

	if err := d.Cleaned(a); !errors.Is(err, ErrWrongState) {
		t.Fatalf("clean of a free frame: %v", err)
	}
}

// ModifiedFrames lists the modified list head first, and is a copy: a
// frame cleaned while going through it doesn't disturb the walk.
func TestModifiedFrames(t *testing.T) {
	d := New(4)
	a, b, c := allocate(t, d, 0), allocate(t, d, 0x200), allocate(t, d, 0x400)
	release(t, d, b, true, Modified)
	release(t, d, c, false, Free)
	release(t, d, a, true, Modified)

	frames := d.ModifiedFrames()
	if !samePFNs(frames, []PFN{b, a}) {
		t.Fatalf("modified frames %v, want %v", frames, []PFN{b, a})
	}

	for _, pfn := range frames {
		must(t, d, d.Cleaned(pfn))
	}

	if got := d.ModifiedFrames(); len(got) != 0 {
		t.Fatalf("modified frames %v after cleaning, want none", got)
	}
}

// ClearModified clears an active frame's saved modify bit, so its
// release goes to the free list; a frame on a list is refused.
func TestClearModified(t *testing.T) {
	d := New(2)
	a := allocate(t, d, 0)
	must(t, d, d.SetModified(a))
	must(t, d, d.ClearModified(a))

	if e := entry(t, d, a); e.SavedModify {
		t.Fatalf("saved modify bit still set: %+v", e)
	}

	release(t, d, a, false, Free)

	if err := d.ClearModified(a); !errors.Is(err, ErrWrongState) {
		t.Fatalf("clear modified of a free frame: %v", err)
	}
}

func TestSetBacking(t *testing.T) {
	d := New(2)
	pfn := allocate(t, d, 0)
	must(t, d, d.SetBacking(pfn, Backing{Kind: PageFile}))

	if err := d.SetBacking(1, Backing{Kind: PageFile}); !errors.Is(err, ErrWrongState) {
		t.Fatalf("backing for an untied frame: %v", err)
	}
}

func TestBadFrame(t *testing.T) {
	d := New(2)

	for name, err := range map[string]error{
		"Reserve":     d.Reserve(2),
		"Hold":        d.Hold(2),
		"Share":       d.Share(9),
		"Reclaim":     d.Reclaim(2),
		"Delete":      d.Delete(2),
		"SetBacking":  d.SetBacking(2, Backing{}),
		"SetModified": d.SetModified(2),
		"Cleaned":     d.Cleaned(2),
	} {
		if !errors.Is(err, ErrBadFrame) {
			t.Errorf("%s: %v", name, err)
		}
	}

	if _, err := d.Release(2, false); !errors.Is(err, ErrBadFrame) {
		t.Errorf("Release: %v", err)
	}

	if _, err := d.Unshare(2, false); !errors.Is(err, ErrBadFrame) {
		t.Errorf("Unshare: %v", err)
	}

	if _, err := d.Entry(2); !errors.Is(err, ErrBadFrame) {
		t.Errorf("Entry: %v", err)
	}

	check(t, d)
}

// Check finds each kind of damage.
func TestCheckFindsDamage(t *testing.T) {
	setup := func() (*Database, PFN, PFN) {
		d := New(6)
		a, b := allocate(t, d, 0), allocate(t, d, 0x200)
		release(t, d, b, true, Modified)

		return d, a, b
	}

	for name, damage := range map[string]func(d *Database, active, modified PFN){
		"back link":        func(d *Database, _, _ PFN) { d.blink[3] = 5 },
		"loop":             func(d *Database, _, _ PFN) { d.flink[4] = 3 },
		"tail":             func(d *Database, _, _ PFN) { d.free.tail = 4 },
		"count":            func(d *Database, _, _ PFN) { d.free.count++ },
		"wrong list":       func(d *Database, _, _ PFN) { d.entries[3].State = Modified },
		"active listed":    func(d *Database, a, _ PFN) { d.entries[2].State = Active; d.entries[2].RefCount = 1 },
		"unlisted":         func(d *Database, a, _ PFN) { d.entries[a].State = Free; d.entries[a].RefCount = 0 },
		"listed refs":      func(d *Database, _, m PFN) { d.entries[m].RefCount = 1 },
		"free modified":    func(d *Database, _, _ PFN) { d.entries[3].SavedModify = true },
		"modified clean":   func(d *Database, _, m PFN) { d.entries[m].SavedModify = false },
		"modified untied":  func(d *Database, _, m PFN) { d.entries[m] = Entry{State: Modified, SavedModify: true} },
		"no references":    func(d *Database, a, _ PFN) { d.entries[a].RefCount = 0 },
		"process shares":   func(d *Database, a, _ PFN) { d.entries[a].ShareCount = 1 },
		"untied owner":     func(d *Database, _, _ PFN) { d.entries[3].Owner = Owner{1, 0x400} },
		"duplicate owner":  func(d *Database, a, m PFN) { d.entries[m].Owner = d.entries[a].Owner },
		"active count":     func(d *Database, _, _ PFN) { d.active[GlobalPage]++ },
		"active linked":    func(d *Database, a, _ PFN) { d.flink[a] = 3 },
		"bad type":         func(d *Database, a, _ PFN) { d.entries[a].Type = SystemPage + 1 },
		"link beyond":      func(d *Database, _, _ PFN) { d.flink[5] = 99 },
		"bad state":        func(d *Database, a, _ PFN) { d.entries[a].State = Active + 1 },
		"missing links":    func(d *Database, _, _ PFN) { d.flink = d.flink[:5] },
		"untied backing":   func(d *Database, _, _ PFN) { d.entries[4].Backing.Kind = PageFile },
		"negative shares":  func(d *Database, a, _ PFN) { d.entries[a].ShareCount = -1 },
		"modified as free": func(d *Database, _, m PFN) { d.entries[m].State = Free; d.entries[m].SavedModify = false },
	} {
		d, a, m := setup()
		damage(d, a, m)

		if err := d.Check(); err == nil {
			t.Errorf("%s: Check found nothing", name)
		}
	}
}

// Random operations, with Check after each, and a model of where each
// tied page's frame is.
func TestRandomOperations(t *testing.T) {
	rng := rand.New(rand.NewSource(62))
	d := New(64)
	d.SetTieBreaker(&tieLog{})

	for pfn := range PFN(4) {
		must(t, d, d.Reserve(pfn))
	}

	nextVA := uint32(0)

	for range 20000 {
		pfn := PFN(rng.Intn(d.Frames()))
		e := entry(t, d, pfn)

		var err error

		switch op := rng.Intn(9); op {
		case 0, 1:
			typ := []Type{ProcessPage, GlobalPage}[rng.Intn(2)]
			nextVA += 0x200
			_, err = d.Allocate(typ, Owner{1, nextVA}, Backing{Kind: PageFile})
		case 2:
			_, err = d.Release(pfn, rng.Intn(2) == 0)
		case 3:
			err = d.Reclaim(pfn)
		case 4:
			if e.Type != SystemPage {
				err = d.Delete(pfn)
			}
		case 5:
			err = d.Hold(pfn)
		case 6:
			err = d.Share(pfn)
		case 7:
			_, err = d.Unshare(pfn, rng.Intn(2) == 0)
		case 8:
			if first, ok := d.FirstModified(); ok {
				err = d.Cleaned(first)
			}
		}

		// An operation may be refused; it must leave the database whole
		// either way.
		if err != nil && !errors.Is(err, ErrWrongState) && !errors.Is(err, ErrNoFrame) && !errors.Is(err, ErrNotShared) {
			t.Fatal(err)
		}

		check(t, d)
	}

	c := d.Counts()
	if c.Free+c.Modified+c.Active() != d.Frames() {
		t.Fatalf("counts %+v don't add up to %d frames", c, d.Frames())
	}
}

// AllocateSystem takes the free list's head, as Allocate does, for an
// untied system frame, breaking the frame's old tie first.
func TestAllocateSystem(t *testing.T) {
	d := New(3)
	log := &tieLog{}
	d.SetTieBreaker(log)

	release(t, d, allocate(t, d, 0x400), false, Free)

	pfn, err := d.AllocateSystem()
	must(t, d, err)

	if pfn != 1 {
		t.Fatalf("system frame %d, want the free list's head, 1", pfn)
	}

	e := entry(t, d, pfn)
	if e.State != Active || e.Type != SystemPage || e.Tied || e.RefCount != 1 {
		t.Fatalf("system frame's record %+v", e)
	}

	pfn, err = d.AllocateSystem()
	must(t, d, err)

	if pfn != 2 || len(log.ties) != 0 {
		t.Fatalf("second system frame %d, ties broken %v", pfn, log.ties)
	}

	pfn, err = d.AllocateSystem()
	must(t, d, err)

	if pfn != 0 || len(log.ties) != 1 || log.ties[0].owner != (Owner{1, 0x400}) {
		t.Fatalf("third system frame %d, ties broken %v", pfn, log.ties)
	}

	if _, err := d.AllocateSystem(); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("system frame from an empty free list: %v", err)
	}

	must(t, d, d.Delete(2))

	if c := d.Counts(); c.System != 2 || c.Free != 1 {
		t.Fatalf("counts %+v", c)
	}
}

// TestMakeSystem: a process page's frame becomes an untied system page,
// counted as one; making it a system page again changes nothing; a frame
// with a hold, a share, or on a list can't be made one.
func TestMakeSystem(t *testing.T) {
	d := New(4)

	pfn := allocate(t, d, 0x7FFE0000)
	must(t, d, d.MakeSystem(pfn))

	if e := entry(t, d, pfn); e.State != Active || e.Type != SystemPage || e.Tied || e.RefCount != 1 {
		t.Fatalf("the frame's record %+v", e)
	}

	if c := d.Counts(); c.System != 1 || c.Process != 0 {
		t.Fatalf("counts %+v", c)
	}

	must(t, d, d.MakeSystem(pfn))

	held := allocate(t, d, 0x200)
	must(t, d, d.Hold(held))

	if err := d.MakeSystem(held); !errors.Is(err, ErrWrongState) {
		t.Errorf("make system of a held frame: %v", err)
	}

	listed := allocate(t, d, 0x400)
	release(t, d, listed, false, Free)

	if err := d.MakeSystem(listed); !errors.Is(err, ErrWrongState) {
		t.Errorf("make system of a free frame: %v", err)
	}

	global, err := d.Allocate(GlobalPage, Owner{Handle: 9}, Backing{Kind: PageFile})
	must(t, d, err)
	must(t, d, d.Share(global))

	if err := d.MakeSystem(global); !errors.Is(err, ErrWrongState) {
		t.Errorf("make system of a shared frame: %v", err)
	}

	check(t, d)
}
