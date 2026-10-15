package paging

// pageList is one of the page lists: a doubly linked list of frames,
// threaded through the database's forward and backward link arrays (as
// VMS's FLINK and BLINK arrays thread it), so a frame can leave from the
// middle of the list in constant time, as a soft fault takes it.
type pageList struct {
	head, tail PFN
	count      int
}

func newList() pageList {
	return pageList{head: noFrame, tail: noFrame}
}

// pushTail adds pfn at the list's tail, the end frames are released to.
func (d *Database) pushTail(l *pageList, pfn PFN) {
	d.flink[pfn] = noFrame
	d.blink[pfn] = l.tail

	if l.tail == noFrame {
		l.head = pfn
	} else {
		d.flink[l.tail] = pfn
	}

	l.tail = pfn
	l.count++
}

// pushHead adds pfn at the list's head, the end frames are taken from.
func (d *Database) pushHead(l *pageList, pfn PFN) {
	d.blink[pfn] = noFrame
	d.flink[pfn] = l.head

	if l.head == noFrame {
		l.tail = pfn
	} else {
		d.blink[l.head] = pfn
	}

	l.head = pfn
	l.count++
}

// unlink takes pfn off the list, wherever it is in it.
func (d *Database) unlink(l *pageList, pfn PFN) {
	next, prev := d.flink[pfn], d.blink[pfn]

	if prev == noFrame {
		l.head = next
	} else {
		d.flink[prev] = next
	}

	if next == noFrame {
		l.tail = prev
	} else {
		d.blink[next] = prev
	}

	d.flink[pfn], d.blink[pfn] = noFrame, noFrame
	l.count--
}

// listFor is the list a frame in state s is on, or nil for an active
// frame.
func (d *Database) listFor(s State) *pageList {
	switch s {
	case Free:
		return &d.free
	case Modified:
		return &d.modified
	}

	return nil
}
