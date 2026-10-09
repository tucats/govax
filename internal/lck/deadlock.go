package lck

import "slices"

// Deadlock detection (docs/PHASE-49.md, subtask 8), as the Internals
// book's section 13.3 describes VMS's lock manager doing it.
//
// A deadlock is a circle of locks each waiting for the next: process A
// holds resource 1 and waits for 2 while B holds 2 and waits for 1 (a
// multiple resource deadlock), or two locks converting on one resource
// each held back by the other (a conversion deadlock). Searching for one
// at every request would be slow, so the lock manager searches only when
// a request has waited a while: each queued request (waiting, or
// converting) is given a due time, DEADLOCK_WAIT after it was queued,
// and when one's due time passes (CheckDeadlocks, which the system calls
// as time goes by) the search starts from it.
//
// The search follows the locks that block the request (Blocks), then
// the queued requests of each blocking lock's owner, and theirs, and so
// on; if it comes back to a lock owned by the process it started from,
// there is a deadlock. A process whose requests have been searched once
// isn't searched again (the book's process bitmap). Conversion deadlocks
// need no search of their own: a converting lock is blocked by the
// granted mode of the one behind it, which waits behind it in turn, so
// the same search finds the circle.
//
// One request in the circle, the victim, is refused: a new lock's
// request is removed, a conversion goes back to the lock's granted mode,
// and its owner is told (EventDeadlock: $ENQ completes with
// SS$_DEADLOCK). VMS chooses the participant with the lowest deadlock
// priority (PCB$L_DLCKPRI), taking one of 0 at once; every govax
// process's is 0, and govax takes the request the search started from
// (unconfirmed: which of several zero-priority participants VMS
// refuses). A search that finds no deadlock gives the request a new due
// time, DEADLOCK_WAIT on.

// DefaultDeadlockWait is SYSGEN's DEADLOCK_WAIT default: 10 seconds, in
// VMS's 100-nanosecond units.
const DefaultDeadlockWait = 10 * 10_000_000

// queue gives a request just queued its due time for a deadlock search.
func (m *Manager) queue(l *Lock) {
	if m.Now != nil && m.DeadlockWait != 0 {
		l.due = m.Now() + m.DeadlockWait
	}
}

// Deadlocked reports whether l's request was refused to break a
// deadlock: a new lock's, removed from the database (it isn't granted,
// and never will be).
func (l *Lock) Deadlocked() bool {
	return l.deadlocked
}

// NextDeadlockCheck returns when the next queued request is due for a
// deadlock search, and false if none is (or detection is off).
func (m *Manager) NextDeadlockCheck() (uint64, bool) {
	if t := m.earliestDue(); t != nil {
		return t.due, true
	}

	return 0, false
}

// earliestDue is the queued request with the earliest due time (the
// lowest ID among equals), or nil.
func (m *Manager) earliestDue() *Lock {
	if m.Now == nil || m.DeadlockWait == 0 {
		return nil
	}

	var first *Lock

	for _, l := range m.locks {
		if l.State == Granted || l.due == 0 {
			continue
		}

		if first == nil || l.due < first.due || l.due == first.due && l.ID < first.ID {
			first = l
		}
	}

	return first
}

// CheckDeadlocks searches from every queued request whose due time has
// passed by now, breaking each deadlock it finds, and returns the events
// for the owners to be told (Deliver).
func (m *Manager) CheckDeadlocks(now uint64) []Event {
	var events []Event

	// Each pass either refuses a request or moves one's due time on, so
	// this many passes are enough.
	for range len(m.locks) + 1 {
		t := m.earliestDue()
		if t == nil || t.due > now {
			break
		}

		if m.findDeadlock(t) == nil {
			t.due = now + m.DeadlockWait

			continue
		}

		events = append(events, m.refuse(t)...)
	}

	return events
}

// findDeadlock returns the circle of queued requests that t's request is
// part of, starting with t, or nil if it isn't part of one.
func (m *Manager) findDeadlock(t *Lock) []*Lock {
	searched := map[Owner]bool{t.Owner: true}

	var (
		path   []*Lock
		search func(w *Lock) bool
	)

	search = func(w *Lock) bool {
		path = append(path, w)

		for _, b := range Blockers(w) {
			if b.Owner == t.Owner {
				return true
			}

			if searched[b.Owner] {
				continue
			}

			searched[b.Owner] = true

			for _, next := range m.queued(b.Owner) {
				if search(next) {
					return true
				}
			}
		}

		path = path[:len(path)-1]

		return false
	}

	if search(t) {
		return path
	}

	return nil
}

// queued returns owner's queued requests (waiting or converting), by ID.
func (m *Manager) queued(owner Owner) []*Lock {
	var out []*Lock

	for _, l := range m.locks {
		if l.Owner == owner && l.State != Granted {
			out = append(out, l)
		}
	}

	slices.SortFunc(out, func(a, b *Lock) int { return int(a.ID) - int(b.ID) })

	return out
}

// refuse breaks a deadlock by refusing l's request: a conversion goes
// back to l's granted mode, a new lock is removed. Either way its owner
// gets EventDeadlock, and whatever that lets through is granted.
func (m *Manager) refuse(l *Lock) []Event {
	l.due = 0

	if l.State == Converting {
		res := l.Resource
		res.converting = remove(res.converting, l)
		res.granted = append(res.granted, l)
		l.State, l.Requested = Granted, l.Mode

		events := append([]Event{{EventDeadlock, l}}, m.regrant(res)...)

		return append(events, m.blockingNotices(res)...)
	}

	l.deadlocked = true
	events := m.remove(l)

	for i := range events {
		if events[i].Lock == l && events[i].Kind == EventAborted {
			events[i].Kind = EventDeadlock
		}
	}

	return events
}
