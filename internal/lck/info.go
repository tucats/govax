package lck

import (
	"slices"
)

// What the lock database tells about itself (docs/PHASE-49.md): the
// questions $GETLKI asks, and the "who waits for whom" that deadlock
// detection follows.

// All returns every lock in the database, by ID.
func (m *Manager) All() []*Lock {
	out := make([]*Lock, 0, len(m.locks))

	for _, l := range m.locks {
		out = append(out, l)
	}

	slices.SortFunc(out, func(a, b *Lock) int { return int(a.ID) - int(b.ID) })

	return out
}

// Sublocks is how many locks have l as their parent (LKI$_LCKREFCNT).
func (l *Lock) Sublocks() int {
	return l.sublocks
}

// Subresources is how many resources have r as their parent resource
// (LKI$_RSBREFCNT): a resource locked with a parent lock on r.
func (r *Resource) Subresources() int {
	return r.children
}

// QueueLengths returns how many locks are on each of r's queues: granted,
// converting, and waiting (LKI$_GRANTCOUNT, LKI$_CVTCOUNT,
// LKI$_WAITCOUNT).
func (r *Resource) QueueLengths() (granted, converting, waiting int) {
	return len(r.granted), len(r.converting), len(r.waiting)
}

// Blocks reports whether lock b blocks lock w: w is queued (waiting, or
// converting) for a mode that b keeps it from. That is so when b, another
// lock on w's resource, holds a mode incompatible with the one w asks
// for (b is granted, or converting and keeping its granted mode), or is
// itself queued ahead of w (the lock manager grants each queue in order:
// the conversion queue before the waiting queue) asking for an
// incompatible mode. These are the incompatible locks the Internals
// book's deadlock search follows ("lock blocks whose granted or
// requested lock mode is incompatible with that of the waiting lock",
// section 13.3.2.2); counting a queued lock as blocking only those
// behind it is govax's reading (two waiters don't block each other).
func Blocks(b, w *Lock) bool {
	if b == w || b.Resource != w.Resource || w.State == Granted {
		return false
	}

	if b.State != Waiting && !b.Mode.Compatible(w.Requested) {
		return true
	}

	return b.State != Granted && queuedAhead(b, w) && !b.Requested.Compatible(w.Requested)
}

// queuedAhead reports whether queued lock a comes before queued lock b
// on their resource's queues.
func queuedAhead(a, b *Lock) bool {
	queued := slices.Concat(a.Resource.converting, a.Resource.waiting)

	return slices.Index(queued, a) < slices.Index(queued, b)
}

// Blockers returns the locks that block w (LKI$_BLOCKING), in the order
// of their resource's queues.
func Blockers(w *Lock) []*Lock {
	var out []*Lock

	for _, b := range w.Resource.Locks() {
		if Blocks(b, w) {
			out = append(out, b)
		}
	}

	return out
}

// BlockedBy returns the locks b blocks (LKI$_BLOCKEDBY), in the order of
// their resource's queues.
func BlockedBy(b *Lock) []*Lock {
	var out []*Lock

	for _, w := range b.Resource.Locks() {
		if Blocks(b, w) {
			out = append(out, w)
		}
	}

	return out
}
