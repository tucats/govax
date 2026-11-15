package security

import (
	"encoding/binary"
	"slices"
)

// ACLEdit is an ACL being changed one item at a time, as the security
// services change it ($CHANGE_ACL, $SET_SECURITY) or read it piece by
// piece ($GET_SECURITY): its entries and a position among them
// (docs/planning/PHASE-93 - file protection.md, A.4's "Round 3").
//
// Pos is the position: 1 the first ACE, 0 the top (which is the first ACE
// too), past the last the bottom. An ACE added goes in before the ACE at
// the position, and the position moves past it; a read starts there and
// moves past what it read. Found says a find left the position on the ACE
// it found.
type ACLEdit struct {
	ACL     [][]byte
	Pos     int
	Found   bool
	Changed bool

	// ZeroContext is set by an alarm or audit ACE's addition: VMS 7.1's
	// $CHANGE_ACL left a context of 0 after each (A.4).
	ZeroContext bool
}

// NewACLEdit is an edit of raw, an ACL's entries one after another, at the
// top.
func NewACLEdit(raw []byte) *ACLEdit { return &ACLEdit{ACL: SplitACL(raw)} }

// SplitACL is an ACL's entries, each a copy. An entry of size 0 or one
// running past the end ends the list.
func SplitACL(raw []byte) [][]byte {
	var acl [][]byte

	for off := 0; off < len(raw); {
		n := int(raw[off])
		if n == 0 || off+n > len(raw) {
			break
		}

		acl = append(acl, slices.Clone(raw[off:off+n]))
		off += n
	}

	return acl
}

// Bytes is the ACL's entries one after another.
func (e *ACLEdit) Bytes() []byte { return slices.Concat(e.ACL...) }

// Current is the position as the index of an ACE, 1 for the top.
func (e *ACLEdit) Current() int { return max(e.Pos, 1) }

// insertAt is where an ACE added at the position goes, 1 to len+1.
func (e *ACLEdit) insertAt() int { return min(e.Current(), len(e.ACL)+1) }

// Top moves the position to the top; Bottom past the last ACE.
func (e *ACLEdit) Top()    { e.Pos, e.Found = 0, false }
func (e *ACLEdit) Bottom() { e.Pos, e.Found = len(e.ACL)+1, false }

// Add adds ace at the position, by the services' matching (MatchACE): a
// matching ACE above the position makes the addition do nothing, one at or
// below it is replaced. An alarm or audit ACE replaces a match wherever it
// is (VMS 7.1 dropped (ALARM...READ+SUCCESS) above the bottom for
// (ALARM...WRITE+SUCCESS) added there; unconfirmed elsewhere than the
// bottom).
func (e *ACLEdit) Add(ace []byte) {
	alarm := AlarmLike(ace)
	at := e.insertAt()

	if !alarm {
		for i, have := range e.ACL {
			if i+1 < at && MatchACE(have, ace) {
				return
			}
		}
	}

	for i := len(e.ACL) - 1; i >= 0; i-- {
		if (alarm || i+1 >= at) && MatchACE(e.ACL[i], ace) {
			e.ACL = slices.Delete(e.ACL, i, i+1)
		}
	}

	at = min(e.insertAt(), at)
	e.ACL = slices.Insert(e.ACL, at-1, slices.Clone(ace))
	e.Pos, e.Changed = at+1, true
	e.ZeroContext = e.ZeroContext || alarm
}

// Delete deletes the first ACE matching want (from the top), or with want
// nil the ACE at the position, and reports whether there was one.
func (e *ACLEdit) Delete(want []byte) bool {
	at := -1

	if want == nil {
		if cur := e.Current(); cur <= len(e.ACL) {
			at = cur - 1
		}
	} else {
		at = slices.IndexFunc(e.ACL, func(ace []byte) bool { return MatchACE(ace, want) })
	}

	if at < 0 {
		return false
	}

	e.ACL = slices.Delete(e.ACL, at, at+1)
	e.Changed = true

	if at+1 < e.Pos {
		e.Pos--
	}

	return true
}

// Modify replaces the ACE at the position with ace, the position moving
// past it, and reports whether there was one.
func (e *ACLEdit) Modify(ace []byte) bool {
	cur := e.Current()
	if cur > len(e.ACL) {
		return false
	}

	e.ACL[cur-1] = slices.Clone(ace)
	e.Pos, e.Changed = cur+1, true

	return true
}

// DeleteUnprotected deletes every ACE but the PROTECTED ones, the
// position going to the top.
func (e *ACLEdit) DeleteUnprotected() {
	e.ACL = slices.DeleteFunc(e.ACL, func(ace []byte) bool {
		return binary.LittleEndian.Uint16(ace[2:])&OptionProtected == 0
	})
	e.Pos, e.Found, e.Changed = 0, false, true
}

// DeleteAll deletes every ACE.
func (e *ACLEdit) DeleteAll() { e.ACL, e.Pos, e.Found, e.Changed = nil, 0, false, true }

// Find moves the position to the first ACE from index start (1 the first)
// that match accepts, setting Found, and reports whether there was one;
// if not, the position goes to the top.
func (e *ACLEdit) Find(start int, match func([]byte) bool) bool {
	for i := start - 1; i < len(e.ACL); i++ {
		if i >= 0 && match(e.ACL[i]) {
			e.Pos, e.Found = i+1, true

			return true
		}
	}

	e.Pos, e.Found = 0, false

	return false
}

// FindMatching is Find of the first ACE from start that MatchACE matches
// with want.
func (e *ACLEdit) FindMatching(start int, want []byte) bool {
	return e.Find(start, func(ace []byte) bool { return MatchACE(ace, want) })
}

// FindType is Find of the first ACE from start of type typ.
func (e *ACLEdit) FindType(start int, typ byte) bool {
	return e.Find(start, func(ace []byte) bool { return ace[1] == typ })
}

// ReadFitting is the whole ACEs from the position that fit in size bytes,
// the position moved past them. none is true past the last ACE; short is
// true when the next ACE doesn't fit, which is then returned alone.
func (e *ACLEdit) ReadFitting(size int) (out []byte, none, short bool) {
	cur := e.Current()
	if cur > len(e.ACL) {
		return nil, true, false
	}

	next := cur
	for ; next <= len(e.ACL) && len(out)+len(e.ACL[next-1]) <= size; next++ {
		out = append(out, e.ACL[next-1]...)
	}

	if next == cur {
		return e.ACL[cur-1], false, true
	}

	e.Pos = next

	return out, false, false
}
