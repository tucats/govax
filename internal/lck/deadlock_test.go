package lck

import "testing"

// clocked returns a manager with deadlock detection on, and its clock.
func clocked() (*Manager, *uint64) {
	now := uint64(1_000_000)
	m := NewManager()
	m.Now = func() uint64 { return now }
	m.DeadlockWait = 100

	return m, &now
}

// TestDeadlock_multipleResource: A holds 1 and waits for 2, B holds 2
// and waits for 1. Nothing happens before DEADLOCK_WAIT; then the
// request searched from first (A's) is refused, and B's is granted.
func TestDeadlock_multipleResource(t *testing.T) {
	m, now := clocked()

	take(t, m, 1, "R1", EX)
	take(t, m, 2, "R2", EX)
	a := take(t, m, 1, "R2", EX)
	b := take(t, m, 2, "R1", EX)

	if due, ok := m.NextDeadlockCheck(); !ok || due != *now+100 {
		t.Fatalf("next check %d, %v", due, ok)
	}

	if ev := m.CheckDeadlocks(*now + 99); len(ev) != 0 {
		t.Fatalf("events before the wait: %v", ev)
	}

	*now += 100
	ev := m.CheckDeadlocks(*now)

	if len(ev) == 0 || ev[0].Kind != EventDeadlock || ev[0].Lock != a || !a.Deadlocked() {
		t.Fatalf("events %v, want A's request refused", ev)
	}

	if _, ok := m.Lock(a.ID); ok {
		t.Error("the refused request is still in the database")
	}

	// B's request is still blocked by A's lock on R1 (A only lost its
	// request for R2); no deadlock is left.
	if b.State != Waiting {
		t.Errorf("B's request is %s", b.State)
	}

	if ev := m.CheckDeadlocks(*now + 1000); len(ev) != 0 {
		t.Errorf("a second deadlock: %v", ev)
	}
}

// TestDeadlock_conversion: two PR locks both converting to EX (the
// book's conversion deadlock): the first's conversion is refused, back
// to PR; the second stays converting (the first's PR still holds it
// back), but no longer in a circle.
func TestDeadlock_conversion(t *testing.T) {
	m, now := clocked()

	a := take(t, m, 1, "R", PR)
	b := take(t, m, 2, "R", PR)

	for _, l := range []*Lock{a, b} {
		if _, _, err := m.Convert(l.Owner, l.ID, EX, ConvertOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	*now += 100
	ev := m.CheckDeadlocks(*now)

	if len(ev) != 1 || ev[0].Kind != EventDeadlock || ev[0].Lock != a {
		t.Fatalf("events %v, want A's conversion refused", ev)
	}

	if a.State != Granted || a.Mode != PR || a.Deadlocked() {
		t.Errorf("A: %s at %s", a.State, a.Mode)
	}

	if b.State != Converting {
		t.Errorf("B: %s", b.State)
	}
}

// TestDeadlock_none: a long wait that isn't a deadlock is searched and
// given a new due time, and nothing is refused.
func TestDeadlock_none(t *testing.T) {
	m, now := clocked()

	take(t, m, 1, "R", EX)
	w := take(t, m, 2, "R", EX)

	*now += 500

	if ev := m.CheckDeadlocks(*now); len(ev) != 0 {
		t.Fatalf("events %v", ev)
	}

	if due, ok := m.NextDeadlockCheck(); !ok || due != *now+100 || w.State != Waiting {
		t.Errorf("next check %d, %v; want %d", due, ok, *now+100)
	}
}
