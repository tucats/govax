package lck

import "testing"

// take enqueues a lock on name for owner at mode, failing the test on an
// error.
func take(t *testing.T, m *Manager, owner Owner, name string, mode Mode) *Lock {
	t.Helper()

	l, _, err := m.Enqueue(Request{Owner: owner, Mode: mode, Name: name})
	if err != nil {
		t.Fatal(err)
	}

	return l
}

// ids is the IDs of locks, for messages.
func ids(locks []*Lock) []ID {
	out := make([]ID, len(locks))
	for i, l := range locks {
		out[i] = l.ID
	}

	return out
}

// TestBlocks: a granted lock blocks an incompatible request; a queued
// one blocks an incompatible request behind it, not one ahead; the
// queue counts and the lists follow.
func TestBlocks(t *testing.T) {
	m := NewManager()

	pr := take(t, m, 1, "R", PR)  // granted
	ex := take(t, m, 2, "R", EX)  // waits for pr
	cr := take(t, m, 3, "R", CR)  // waits (behind ex), compatible with pr
	ex2 := take(t, m, 4, "R", EX) // waits, behind both

	if ex.State != Waiting || cr.State != Waiting {
		t.Fatalf("states %s %s", ex.State, cr.State)
	}

	cases := []struct {
		b, w *Lock
		want bool
	}{
		{pr, ex, true}, {ex, pr, false}, // a granted lock isn't blocked
		{pr, cr, false},  // compatible
		{ex, cr, true},   // queued ahead, incompatible
		{cr, ex, false},  // behind
		{ex, ex2, true},  // ahead, EX/EX
		{cr, ex2, true},  // ahead, CR/EX
		{ex2, ex, false}, // behind
	}

	for _, c := range cases {
		if got := Blocks(c.b, c.w); got != c.want {
			t.Errorf("Blocks(%d, %d) = %v, want %v", c.b.ID, c.w.ID, got, c.want)
		}
	}

	if got := ids(Blockers(ex2)); len(got) != 3 {
		t.Errorf("Blockers(ex2) = %v, want pr, ex, cr", got)
	}

	if got := ids(BlockedBy(pr)); len(got) != 2 || got[0] != ex.ID || got[1] != ex2.ID {
		t.Errorf("BlockedBy(pr) = %v, want ex, ex2", got)
	}

	if g, c, w := pr.Resource.QueueLengths(); g != 1 || c != 0 || w != 3 {
		t.Errorf("queues %d %d %d, want 1 0 3", g, c, w)
	}

	if got := ids(m.All()); len(got) != 4 || got[0] != pr.ID || got[3] != ex2.ID {
		t.Errorf("All = %v", got)
	}
}

// TestBlocks_conversion: the Internals book's conversion deadlock, two
// PR locks each converting to EX, block each other: the first is held
// back by the second's PR, the second waits behind the first.
func TestBlocks_conversion(t *testing.T) {
	m := NewManager()

	a := take(t, m, 1, "R", PR)
	b := take(t, m, 2, "R", PR)

	for _, l := range []*Lock{a, b} {
		if _, _, err := m.Convert(l.Owner, l.ID, EX, ConvertOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	if !Blocks(b, a) || !Blocks(a, b) {
		t.Errorf("Blocks(b, a) = %v, Blocks(a, b) = %v; want both", Blocks(b, a), Blocks(a, b))
	}
}

// TestSublocksAndSubresources: a lock's sublocks and a resource's
// subresources are counted.
func TestSublocksAndSubresources(t *testing.T) {
	m := NewManager()

	parent := take(t, m, 1, "P", EX)

	for _, name := range []string{"C1", "C2"} {
		if _, _, err := m.Enqueue(Request{Owner: 1, Mode: NL, Name: name, Parent: parent.ID}); err != nil {
			t.Fatal(err)
		}
	}

	if parent.Sublocks() != 2 || parent.Resource.Subresources() != 2 {
		t.Errorf("sublocks %d, subresources %d; want 2, 2", parent.Sublocks(), parent.Resource.Subresources())
	}
}
