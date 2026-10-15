package paging

import "fmt"

// Check verifies the database's rules:
//
//   - each list's links agree both ways, end at its head and tail, hold
//     no loop, and add up to its count;
//   - every frame on the free list is Free and every frame on the
//     modified list Modified, and every frame in either state is on its
//     list, so each frame is in exactly one place;
//   - a frame on a list has no references or shares; one on the free
//     list has its saved modify bit clear, and one on the modified list
//     has it set and is tied (its contents must have somewhere to go);
//   - an active frame isn't linked, has at least one reference, and only
//     a global page has shares (each share counting as one reference
//     while there are any);
//   - an untied frame has no owner and no backing store;
//   - no two tied frames of the same type have the same owner;
//   - the active counts by type match the frames.
//
// It returns the first rule broken, or nil.
func (d *Database) Check() error {
	n := len(d.entries)
	if len(d.flink) != n || len(d.blink) != n {
		return fmt.Errorf("paging: %d frames but %d forward and %d backward links", n, len(d.flink), len(d.blink))
	}

	onList := make([]bool, n)

	if err := d.checkList("free", &d.free, Free, onList); err != nil {
		return err
	}

	if err := d.checkList("modified", &d.modified, Modified, onList); err != nil {
		return err
	}

	var active [SystemPage + 1]int

	owners := make(map[ownerKey]PFN)

	for i := range d.entries {
		pfn := PFN(i)
		e := &d.entries[pfn]

		if err := d.checkEntry(pfn, e, onList[pfn]); err != nil {
			return err
		}

		if e.State == Active {
			active[e.Type]++
		}

		if e.Tied {
			key := ownerKey{e.Type, e.Owner}
			if other, dup := owners[key]; dup {
				return fmt.Errorf("paging: frames %d and %d are both tied to %s page %08X of %08X",
					other, pfn, e.Type, e.Owner.VA, e.Owner.Handle)
			}

			owners[key] = pfn
		}
	}

	if active != d.active {
		return fmt.Errorf("paging: active counts %v, but the frames say %v", d.active, active)
	}

	return nil
}

// ownerKey is what makes a tie unique: a virtual page is in one frame at
// a time.
type ownerKey struct {
	typ   Type
	owner Owner
}

// checkList walks one list from its head, checking its links and that
// each frame on it is in state want, and marks the frames in onList.
func (d *Database) checkList(name string, l *pageList, want State, onList []bool) error {
	prev := noFrame
	count := 0

	for pfn := l.head; pfn != noFrame; pfn = d.flink[pfn] {
		if int(pfn) >= len(d.entries) {
			return fmt.Errorf("paging: %s list links to frame %d, beyond the database", name, pfn)
		}

		if onList[pfn] {
			return fmt.Errorf("paging: frame %d is on the lists twice (the %s list loops)", pfn, name)
		}

		if d.blink[pfn] != prev {
			return fmt.Errorf("paging: %s list frame %d links back to %d, not %d", name, pfn, d.blink[pfn], prev)
		}

		if s := d.entries[pfn].State; s != want {
			return fmt.Errorf("paging: %s frame %d is on the %s list", s, pfn, name)
		}

		onList[pfn] = true
		prev = pfn
		count++
	}

	if l.tail != prev {
		return fmt.Errorf("paging: %s list ends at frame %d, but its tail is %d", name, prev, l.tail)
	}

	if l.count != count {
		return fmt.Errorf("paging: %s list holds %d frames, but its count is %d", name, count, l.count)
	}

	return nil
}

// checkEntry checks one frame's record against its state.
func (d *Database) checkEntry(pfn PFN, e *Entry, listed bool) error {
	if e.Type > SystemPage {
		return fmt.Errorf("paging: frame %d has %s", pfn, e.Type)
	}

	if !e.Tied && (e.Owner != Owner{} || e.Backing != Backing{}) {
		return fmt.Errorf("paging: untied frame %d has an owner or backing store", pfn)
	}

	switch e.State {
	case Free, Modified:
		if !listed {
			return fmt.Errorf("paging: %s frame %d isn't on its list", e.State, pfn)
		}

		if e.RefCount != 0 || e.ShareCount != 0 {
			return fmt.Errorf("paging: %s frame %d has %d references, %d shares", e.State, pfn, e.RefCount, e.ShareCount)
		}

		if e.SavedModify != (e.State == Modified) {
			return fmt.Errorf("paging: %s frame %d has its saved modify bit %v", e.State, pfn, e.SavedModify)
		}

		if e.State == Modified && !e.Tied {
			return fmt.Errorf("paging: modified frame %d isn't tied to a page", pfn)
		}

	case Active:
		if d.flink[pfn] != noFrame || d.blink[pfn] != noFrame {
			return fmt.Errorf("paging: active frame %d is linked", pfn)
		}

		if e.RefCount < 1 {
			return fmt.Errorf("paging: active frame %d has %d references", pfn, e.RefCount)
		}

		if e.ShareCount < 0 || (e.ShareCount > 0 && e.Type != GlobalPage) {
			return fmt.Errorf("paging: %s frame %d has %d shares", e.Type, pfn, e.ShareCount)
		}

	default:
		return fmt.Errorf("paging: frame %d is in %s", pfn, e.State)
	}

	return nil
}
