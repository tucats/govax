package lck

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestCompatible checks the table against the manual's mode
// descriptions, and that it's symmetric.
func TestCompatible(t *testing.T) {
	// Each row: the modes compatible with the row's mode.
	want := map[Mode]string{
		NL: "NL CR CW PR PW EX",
		CR: "NL CR CW PR PW",
		CW: "NL CR CW",
		PR: "NL CR PR",
		PW: "NL CR",
		EX: "NL",
	}

	for a := NL; a <= EX; a++ {
		var got []string

		for b := NL; b <= EX; b++ {
			if a.Compatible(b) {
				got = append(got, b.String())
			}

			if a.Compatible(b) != b.Compatible(a) {
				t.Errorf("%s/%s isn't symmetric", a, b)
			}
		}

		if strings.Join(got, " ") != want[a] {
			t.Errorf("%s is compatible with %q, want %q", a, strings.Join(got, " "), want[a])
		}
	}
}

// TestNoMoreRestrictive: a conversion to a mode compatible with at least
// everything the old one was is always grantable.
func TestNoMoreRestrictive(t *testing.T) {
	cases := []struct {
		from, to Mode
		want     bool
	}{
		{EX, NL, true}, {EX, PW, true}, {PW, PR, true}, {PW, CW, true},
		{PR, CR, true}, {CW, CR, true}, {CR, NL, true}, {PR, PR, true},
		{NL, CR, false}, {CR, PR, false}, {PR, CW, false}, {CW, PR, false},
		{PR, PW, false}, {PW, EX, false},
	}

	for _, c := range cases {
		if got := c.to.noMoreRestrictive(c.from); got != c.want {
			t.Errorf("%s to %s: %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

// harness drives a Manager in tests, naming locks by a label.
type harness struct {
	t      *testing.T
	m      *Manager
	locks  map[string]*Lock
	events []string
}

func newHarness(t *testing.T) *harness {
	return &harness{t: t, m: NewManager(), locks: map[string]*Lock{}}
}

// record notes events as "kind:label" strings.
func (h *harness) record(events []Event) {
	for _, e := range events {
		kind := [...]string{"granted", "blocking", "aborted", "canceled"}[e.Kind]
		h.events = append(h.events, kind+":"+e.Lock.Data.(string))
	}
}

// takeEvents returns the events since the last call, joined.
func (h *harness) takeEvents() string {
	s := strings.Join(h.events, " ")
	h.events = nil

	return s
}

// enq requests a lock labeled label for owner on resource name.
func (h *harness) enq(label string, owner Owner, name string, mode Mode, opts ...func(*Request)) error {
	h.t.Helper()

	req := Request{Owner: owner, Mode: mode, Name: name, Group: 1, AccessMode: 3, Data: label}
	for _, o := range opts {
		o(&req)
	}

	l, events, err := h.m.Enqueue(req)
	h.record(events)

	if err == nil {
		h.locks[label] = l
	}

	return err
}

// state describes a lock as "MODE" when granted, "MODE->MODE" when
// converting, and "wait:MODE" when waiting; "gone" if it's been
// dequeued.
func (h *harness) state(label string) string {
	l := h.locks[label]
	if _, ok := h.m.Lock(l.ID); !ok {
		return "gone"
	}

	switch l.State {
	case Converting:
		return l.Mode.String() + "->" + l.Requested.String()
	case Waiting:
		return "wait:" + l.Requested.String()
	}

	return l.Mode.String()
}

// states is state for each label, space separated.
func (h *harness) states(labels ...string) string {
	out := make([]string, 0, len(labels))

	for _, l := range labels {
		out = append(out, h.state(l))
	}

	return strings.Join(out, " ")
}

func (h *harness) convert(label string, mode Mode, opts ConvertOptions) error {
	l := h.locks[label]

	if opts.Data == nil {
		opts.Data = label
	}

	_, events, err := h.m.Convert(l.Owner, l.ID, mode, opts)
	h.record(events)

	return err
}

func (h *harness) deq(label string, opts DequeueOptions) error {
	l := h.locks[label]
	events, err := h.m.Dequeue(l.Owner, l.ID, opts)
	h.record(events)

	return err
}

func noQueue(r *Request)  { r.NoQueue = true }
func blocking(r *Request) { r.Blocking = true }
func valblk(r *Request)   { r.ValueBlock = true }
func system(r *Request)   { r.Group = 0 }

func parent(l *Lock) func(*Request) {
	return func(r *Request) { r.Parent = l.ID }
}

// TestNewLocks: compatible requests are granted, incompatible ones wait,
// and the waiting queue is first in, first out: a compatible request
// behind a waiting one waits too.
func TestNewLocks(t *testing.T) {
	cases := []struct {
		modes []Mode
		want  string
	}{
		{[]Mode{PR, PR, CR}, "PR PR CR"}, //nolint:dupword
		{[]Mode{CW, CW, CR}, "CW CW CR"}, //nolint:dupword
		{[]Mode{PR, CW}, "PR wait:CW"},
		{[]Mode{EX, NL, CR}, "EX NL wait:CR"},
		{[]Mode{PW, CR, PR}, "PW CR wait:PR"},
		{[]Mode{PW, PR, CR}, "PW wait:PR wait:CR"}, // FIFO: CR waits behind PR
		{[]Mode{NL, EX}, "NL EX"},
	}

	for _, c := range cases {
		h := newHarness(t)

		var labels []string

		for i, mode := range c.modes {
			label := fmt.Sprint("L", i)
			labels = append(labels, label)

			if err := h.enq(label, Owner(i+1), "RES", mode); err != nil {
				t.Fatal(err)
			}
		}

		if got := h.states(labels...); got != c.want {
			t.Errorf("%v: %s, want %s", c.modes, got, c.want)
		}
	}
}

// TestNoQueue: a request that can't be granted at once fails, leaving
// no lock and no resource behind; a conversion keeps its old mode.
func TestNoQueue(t *testing.T) {
	h := newHarness(t)

	_ = h.enq("A", 1, "RES", EX)

	if err := h.enq("B", 2, "RES", CR, noQueue); !errors.Is(err, ErrNotQueued) {
		t.Fatalf("NOQUEUE request: %v", err)
	}

	if n := len(h.m.locks); n != 1 {
		t.Errorf("%d locks, want 1", n)
	}

	_ = h.enq("C", 2, "OTHER", CR)
	_ = h.enq("D", 3, "OTHER", CR)

	if err := h.convert("C", EX, ConvertOptions{NoQueue: true}); !errors.Is(err, ErrNotQueued) {
		t.Errorf("NOQUEUE conversion: %v", err)
	}

	if got := h.state("C"); got != "CR" {
		t.Errorf("after the failed conversion: %s", got)
	}

	// A request for a resource nobody has is always grantable.
	if err := h.enq("E", 4, "FRESH", EX, noQueue); err != nil {
		t.Errorf("NOQUEUE on a new resource: %v", err)
	}
}

// TestDequeueGrantsWaiters: dequeuing grants the waiting queue from its
// head while it can.
func TestDequeueGrantsWaiters(t *testing.T) {
	h := newHarness(t)

	_ = h.enq("A", 1, "RES", EX)
	_ = h.enq("B", 2, "RES", PR)
	_ = h.enq("C", 3, "RES", PR)
	_ = h.enq("D", 4, "RES", PW)
	_ = h.enq("E", 5, "RES", CR)

	if err := h.deq("A", DequeueOptions{}); err != nil {
		t.Fatal(err)
	}

	if got := h.states("A", "B", "C", "D", "E"); got != "gone PR PR wait:PW wait:CR" { //nolint:dupword
		t.Errorf("after A: %s", got)
	}

	if got := h.takeEvents(); got != "granted:B granted:C" {
		t.Errorf("events: %s", got)
	}

	_ = h.deq("B", DequeueOptions{})
	_ = h.deq("C", DequeueOptions{})

	if got := h.states("D", "E"); got != "PW CR" {
		t.Errorf("after B and C: %s", got)
	}

	if got := h.takeEvents(); got != "granted:D granted:E" {
		t.Errorf("events: %s", got)
	}
}

// TestConversions covers the conversion rules.
func TestConversions(t *testing.T) {
	t.Run("a lock doesn't block its own conversion", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", NL)
		_ = h.enq("B", 2, "RES", PW)

		if err := h.convert("B", EX, ConvertOptions{}); err != nil {
			t.Fatal(err)
		}

		if got := h.states("A", "B"); got != "NL EX" {
			t.Errorf("%s", got)
		}
	})

	t.Run("an incompatible conversion queues; the release grants it", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", PR)
		_ = h.enq("B", 2, "RES", PR)
		_ = h.convert("B", EX, ConvertOptions{})

		if got := h.states("A", "B"); got != "PR PR->EX" {
			t.Fatalf("%s", got)
		}

		_ = h.deq("A", DequeueOptions{})

		if got := h.states("B"); got != "EX" {
			t.Errorf("%s", got)
		}

		if got := h.takeEvents(); got != "granted:B" {
			t.Errorf("events: %s", got)
		}
	})

	t.Run("conversions go before waiting requests", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", PR)
		_ = h.enq("B", 2, "RES", PR)
		_ = h.enq("W", 3, "RES", EX)             // waits
		_ = h.convert("B", PW, ConvertOptions{}) // waits for A

		_ = h.deq("A", DequeueOptions{})

		if got := h.states("B", "W"); got != "PW wait:EX" {
			t.Errorf("%s", got)
		}
	})

	t.Run("a down conversion is granted at once and lets others through", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", EX)
		_ = h.enq("B", 2, "RES", CR)
		_ = h.convert("A", CR, ConvertOptions{})

		if got := h.states("A", "B"); got != "CR CR" {//nolint:dupword
			t.Errorf("%s", got)
		}
	})

	t.Run("an up conversion waits behind a queued one", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", CR)
		_ = h.enq("B", 2, "RES", CR)
		_ = h.enq("C", 3, "RES", PR)
		_ = h.convert("B", EX, ConvertOptions{}) // waits for A and C
		_ = h.convert("A", CW, ConvertOptions{}) // compatible with B and C? no (C is PR): waits

		if got := h.states("A", "B", "C"); got != "CR->CW CR->EX PR" {
			t.Fatalf("%s", got)
		}

		_ = h.deq("C", DequeueOptions{})

		// B is at the head, and A (CR) still blocks EX: nothing moves,
		// though A's CW would now be grantable.
		if got := h.states("A", "B"); got != "CR->CW CR->EX" {
			t.Errorf("%s", got)
		}
	})

	t.Run("only a granted lock converts", func(t *testing.T) {
		h := newHarness(t)
		_ = h.enq("A", 1, "RES", EX)
		_ = h.enq("B", 2, "RES", EX)

		if err := h.convert("B", NL, ConvertOptions{}); !errors.Is(err, ErrNotGranted) {
			t.Errorf("converting a waiting lock: %v", err)
		}
	})
}

// TestCancel: $DEQ with LCK$M_CANCEL.
func TestCancel(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", PR)
	_ = h.enq("B", 2, "RES", PR)
	_ = h.enq("W", 3, "RES", EX)
	_ = h.convert("B", EX, ConvertOptions{})

	if err := h.deq("A", DequeueOptions{Cancel: true}); !errors.Is(err, ErrCancelGranted) {
		t.Errorf("canceling a granted lock: %v", err)
	}

	if err := h.deq("B", DequeueOptions{Cancel: true}); err != nil {
		t.Fatal(err)
	}

	if got := h.states("B"); got != "PR" {
		t.Errorf("after canceling the conversion: %s", got)
	}

	if err := h.deq("W", DequeueOptions{Cancel: true}); err != nil {
		t.Fatal(err)
	}

	if got := h.states("W"); got != "gone" {
		t.Errorf("after canceling the waiting request: %s", got)
	}

	if got := h.takeEvents(); got != "canceled:B aborted:W" {
		t.Errorf("events: %s", got)
	}
}

// TestDequeueQueued: dequeuing a converting lock removes it, aborting the
// conversion.
func TestDequeueQueued(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", PR)
	_ = h.enq("B", 2, "RES", PR)
	_ = h.convert("B", EX, ConvertOptions{})
	_ = h.deq("B", DequeueOptions{})

	if got := h.takeEvents(); got != "aborted:B" {
		t.Errorf("events: %s", got)
	}

	if got := h.states("A", "B"); got != "PR gone" {
		t.Errorf("%s", got)
	}
}

// TestOwnership: a lock is reached only by its owner.
func TestOwnership(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", EX)
	l := h.locks["A"]

	if _, _, err := h.m.Convert(2, l.ID, NL, ConvertOptions{}); !errors.Is(err, ErrInvalidLock) {
		t.Errorf("Convert by another owner: %v", err)
	}

	if _, err := h.m.Dequeue(2, l.ID, DequeueOptions{}); !errors.Is(err, ErrInvalidLock) {
		t.Errorf("Dequeue by another owner: %v", err)
	}

	if _, err := h.m.Dequeue(1, 999, DequeueOptions{}); !errors.Is(err, ErrInvalidLock) {
		t.Errorf("Dequeue of no lock: %v", err)
	}

	if err := h.enq("B", 2, "SUB", EX, parent(l)); !errors.Is(err, ErrInvalidLock) {
		t.Errorf("a sublock of another owner's lock: %v", err)
	}
}

// TestRequestValidation: modes and names.
func TestRequestValidation(t *testing.T) {
	h := newHarness(t)

	if err := h.enq("A", 1, "RES", Mode(6)); !errors.Is(err, ErrBadMode) {
		t.Errorf("mode 6: %v", err)
	}

	if err := h.enq("A", 1, "", NL); !errors.Is(err, ErrBadName) {
		t.Errorf("empty name: %v", err)
	}

	if err := h.enq("A", 1, strings.Repeat("N", 32), NL); !errors.Is(err, ErrBadName) {
		t.Errorf("32-byte name: %v", err)
	}

	if err := h.enq("A", 1, strings.Repeat("N", 31), NL); err != nil {
		t.Errorf("31-byte name: %v", err)
	}
}

// TestNameSpaces: the group, the access mode, and the parent each make a
// separate resource.
func TestNameSpaces(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("G1", 1, "RES", EX)
	_ = h.enq("SYS", 2, "RES", EX, system)
	_ = h.enq("G2", 3, "RES", EX, func(r *Request) { r.Group = 2 })
	_ = h.enq("K", 4, "RES", EX, func(r *Request) { r.AccessMode = 0 })

	if got := h.states("G1", "SYS", "G2", "K"); got != "EX EX EX EX" {//nolint:dupword
		t.Errorf("%s", got)
	}

	_ = h.enq("P1", 5, "FILE1", CR)
	_ = h.enq("P2", 6, "FILE2", CR)
	_ = h.enq("S1", 5, "REC", EX, parent(h.locks["P1"]))
	_ = h.enq("S2", 6, "REC", EX, parent(h.locks["P2"]))
	_ = h.enq("S3", 5, "REC", EX, parent(h.locks["P1"]))

	if got := h.states("S1", "S2", "S3"); got != "EX EX wait:EX" {//nolint:dupword
		t.Errorf("sub-resources: %s", got)
	}
}

// TestSublocks: a parent with sublocks can't be dequeued; DequeueAll on
// it takes them; its sublocks take its access mode; and a waiting parent
// can't have sublocks.
func TestSublocks(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("P", 1, "FILE", CR, func(r *Request) { r.AccessMode = 1 })
	_ = h.enq("S", 1, "REC", EX, parent(h.locks["P"]))
	_ = h.enq("SS", 1, "FIELD", EX, parent(h.locks["S"]))

	if h.locks["S"].AccessMode != 1 {
		t.Errorf("sublock access mode %d, want the parent's (1)", h.locks["S"].AccessMode)
	}

	if err := h.deq("P", DequeueOptions{}); !errors.Is(err, ErrSublocks) {
		t.Errorf("dequeuing a parent: %v", err)
	}

	p := h.locks["P"]
	if _, err := h.m.DequeueAll(1, 0, p.ID); err != nil {
		t.Fatal(err)
	}

	if got := h.states("P", "S", "SS"); got != "CR gone gone" {//nolint:dupword
		t.Errorf("after DequeueAll of P's sublocks: %s", got)
	}

	if err := h.deq("P", DequeueOptions{}); err != nil {
		t.Errorf("dequeuing the parent now: %v", err)
	}

	if n := len(h.m.resources); n != 0 {
		t.Errorf("%d resources left", n)
	}

	_ = h.enq("X", 2, "R", EX)
	_ = h.enq("Y", 3, "R", EX)

	if err := h.enq("Z", 3, "SUB", NL, parent(h.locks["Y"])); !errors.Is(err, ErrParentNotGranted) {
		t.Errorf("a sublock of a waiting lock: %v", err)
	}
}

// TestDequeueAllByMode: DEQALL with no lock dequeues the owner's locks
// at the access mode and less privileged ones.
func TestDequeueAllByMode(t *testing.T) {
	h := newHarness(t)

	for mode := uint8(0); mode < 4; mode++ {
		label := fmt.Sprint("M", mode)
		_ = h.enq(label, 1, label, EX, func(r *Request) { r.AccessMode = mode })
	}

	_ = h.enq("OTHER", 2, "M3x", EX)

	if _, err := h.m.DequeueAll(1, 2, 0); err != nil {
		t.Fatal(err)
	}

	if got := h.states("M0", "M1", "M2", "M3", "OTHER"); got != "EX EX gone gone EX" {//nolint:dupword
		t.Errorf("%s", got)
	}
}

// TestRelease: a rundown takes every lock the owner has, granting what
// waited for them; an abnormal one invalidates PW and EX value blocks.
func TestRelease(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", EX, valblk)
	_ = h.enq("A2", 1, "OTHER", PR)
	_ = h.enq("B", 2, "RES", PR, valblk)
	_ = h.enq("C", 1, "RES", CR) // waits behind B

	events := h.m.Release(1, true)
	h.record(events)

	if got := h.states("A", "A2", "B", "C"); got != "gone gone PR gone" {//nolint:dupword
		t.Errorf("%s", got)
	}

	if got := h.takeEvents(); got != "aborted:C granted:B" {
		t.Errorf("events: %s", got)
	}

	if !h.locks["B"].ValueNotValid {
		t.Error("B's value block is valid; A was EX when its owner was run down")
	}

	if n := len(h.m.Locks(1)); n != 0 {
		t.Errorf("owner 1 still has %d locks", n)
	}
}

// TestValueBlock: the value block's copies, as the manual describes.
func TestValueBlock(t *testing.T) {
	h := newHarness(t)
	v := ValueBlock{1, 2, 3}

	// EX writes it on its way down.
	_ = h.enq("A", 1, "RES", EX, valblk)
	_ = h.convert("A", NL, ConvertOptions{ValueBlock: true, Value: v})

	// A new lock with VALBLK gets it.
	_ = h.enq("B", 2, "RES", PR, valblk)

	if got := h.locks["B"].Value; got != v {
		t.Errorf("B's copy = %v, want %v", got, v)
	}

	// Without VALBLK, nothing is copied.
	_ = h.enq("C", 3, "RES", PR)

	if got := h.locks["C"].Value; got != (ValueBlock{}) {
		t.Errorf("C's copy = %v", got)
	}

	// A PR dequeue doesn't write it.
	w := ValueBlock{9}
	_ = h.deq("B", DequeueOptions{Value: &w})
	_ = h.deq("C", DequeueOptions{})

	// An EX dequeue does.
	_ = h.convert("A", EX, ConvertOptions{ValueBlock: true})

	if got := h.locks["A"].Value; got != v {
		t.Errorf("A's copy after converting up = %v, want %v", got, v)
	}

	v2 := ValueBlock{4, 5, 6}
	_ = h.enq("D", 4, "RES", NL) // keeps the resource
	_ = h.deq("A", DequeueOptions{Value: &v2})

	_ = h.enq("E", 5, "RES", CR, valblk)

	if got := h.locks["E"].Value; got != v2 {
		t.Errorf("E's copy = %v, want %v", got, v2)
	}

	// INVVALBLK marks it invalid until it's written again.
	_ = h.convert("E", EX, ConvertOptions{ValueBlock: true})
	_ = h.deq("E", DequeueOptions{Invalidate: true})
	_ = h.enq("F", 6, "RES", EX, valblk)

	if !h.locks["F"].ValueNotValid {
		t.Error("F's value block is valid after INVVALBLK")
	}

	_ = h.convert("F", NL, ConvertOptions{ValueBlock: true, Value: v})
	_ = h.convert("F", PR, ConvertOptions{ValueBlock: true})

	if f := h.locks["F"]; f.ValueNotValid || f.Value != v {
		t.Errorf("after writing it again: %v, not valid %v", f.Value, f.ValueNotValid)
	}

	// The value block goes with the resource.
	_ = h.deq("D", DequeueOptions{})
	_ = h.deq("F", DequeueOptions{})
	_ = h.enq("G", 7, "RES", PR, valblk)

	if got := h.locks["G"].Value; got != (ValueBlock{}) {
		t.Errorf("a new resource's value block = %v", got)
	}
}

// TestBlockingNotices: holders that asked are told, once per grant, when
// they block a request.
func TestBlockingNotices(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", PR, blocking)
	_ = h.enq("B", 2, "RES", CR, blocking)
	_ = h.enq("C", 3, "RES", PR) // compatible: no notices

	if got := h.takeEvents(); got != "" {
		t.Errorf("events for compatible requests: %s", got)
	}

	_ = h.enq("W", 4, "RES", PW) // blocked by A and C, not B

	if got := h.takeEvents(); got != "blocking:A" {
		t.Errorf("events: %s", got)
	}

	_ = h.enq("W2", 5, "RES", EX) // blocked by A, B, C; A was told already

	if got := h.takeEvents(); got != "blocking:B" {
		t.Errorf("events: %s", got)
	}

	// A converts to NL (re-granted, re-armed); nothing it blocks now.
	_ = h.convert("A", NL, ConvertOptions{Blocking: true})
	_ = h.takeEvents()
	_ = h.convert("A", PR, ConvertOptions{Blocking: true})

	if got := h.takeEvents(); got != "blocking:A" {
		t.Errorf("events after A's regrant at PR: %s", got)
	}
}

// TestResourcesFreed: a resource goes when its last lock does.
func TestResourcesFreed(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A", 1, "RES", EX)
	_ = h.enq("B", 2, "RES", EX)
	_ = h.deq("B", DequeueOptions{})
	_ = h.deq("A", DequeueOptions{})

	if n := len(h.m.resources); n != 0 {
		t.Errorf("%d resources left", n)
	}

	if n := len(h.m.locks); n != 0 {
		t.Errorf("%d locks left", n)
	}
}

// TestDeadlockWaits: there is no deadlock detection. Two owners each
// holding one resource and waiting for the other's wait forever, until
// one's locks go (as its rundown, after Ctrl-C, takes them).
func TestDeadlockWaits(t *testing.T) {
	h := newHarness(t)
	_ = h.enq("A1", 1, "ONE", EX)
	_ = h.enq("B2", 2, "TWO", EX)
	_ = h.enq("A2", 1, "TWO", EX)
	_ = h.enq("B1", 2, "ONE", EX)

	if got := h.states("A2", "B1"); got != "wait:EX wait:EX" {//nolint:dupword
		t.Fatalf("%s", got)
	}

	h.record(h.m.Release(1, true))

	if got := h.states("B1"); got != "EX" {
		t.Errorf("after owner 1's rundown: %s", got)
	}
}
