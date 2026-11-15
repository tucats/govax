package lnm

import "github.com/tucats/govax/internal/vmserrors"

// Logical name table quotas. A table made with a quota ($CRELNT's quota
// argument) may hold that many bytes: each name in it is charged its
// size, and each child table made with a quota of its own is charged that
// quota, as VMS charges a shareable table's quota to its parent (the phase
// doc's A.4: a table asking for its parent's whole quota was
// SS$_EXLNMQUOTA, one asking for less was made). A table with no quota (0)
// holds as much as it's given, and a child with no quota charges its
// parent nothing (VMS 7.1 made one under a table with a quota). The tables
// made at startup have none.
//
// A name's size is govax's model of VMS's logical name block, not VMS's
// own count (unconfirmed): a 32-byte header, the name, 8 bytes and the
// text for each equivalence string, rounded up to 16 bytes.

// nameCharge is the bytes a name of lognam with eqv takes from its
// table's quota.
func nameCharge(lognam string, eqv []Equivalence) uint32 {
	n := 32 + len(lognam)
	for _, q := range eqv {
		n += 8 + len(q.Value)
	}

	return uint32(n+15) &^ 15
}

// room reports whether t can take charge more bytes once freed bytes
// (an entry being superseded) are given back: SS$_EXLNMQUOTA if not.
func (t *Table) room(charge, freed uint32) error {
	if t.Quota == 0 || charge == 0 {
		return nil
	}

	if t.Used-min(freed, t.Used)+charge > t.Quota {
		return status(vmserrors.SS_EXLNMQUOTA)
	}

	return nil
}

// chargeEntry charges n bytes of t's quota to e, if t has a quota.
func chargeEntry(e *Entry, t *Table, n uint32) {
	if t == nil || t.Quota == 0 || n == 0 {
		return
	}

	t.Used += n
	e.charge, e.chargedTo = n, t
}

// refund gives back what e was charged.
func refund(e *Entry) {
	if e.chargedTo == nil {
		return
	}

	e.chargedTo.Used -= min(e.charge, e.chargedTo.Used)
	e.charge, e.chargedTo = 0, nil
}

// chargeAt is what the entry for name at exactly mode in t holds of
// table's quota: what superseding it would give back.
func (t *Table) chargeAt(name string, mode Mode, table *Table) uint32 {
	if e := t.atMode(name, mode); e != nil && e.chargedTo == table {
		return e.charge
	}

	return 0
}
