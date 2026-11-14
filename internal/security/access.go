package security

import (
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// Access is a set of kinds of access, as the ARM$M_ bits of $ARMDEF
// define them. The same bits mean different things for different
// classes of object: a file's execute bit is a device's physical I/O.
type Access uint32

// The kinds of access. The first four are the bits a protection code's
// field holds; Control is only ever given by an ACE or by being the
// owner or a system user.
var (
	Read     = Access(vmsdef.Symbols["ARM$M_READ"])
	Write    = Access(vmsdef.Symbols["ARM$M_WRITE"])
	Execute  = Access(vmsdef.Symbols["ARM$M_EXECUTE"])
	Delete   = Access(vmsdef.Symbols["ARM$M_DELETE"])
	Control  = Access(vmsdef.Symbols["ARM$M_CONTROL"])
	Physical = Access(vmsdef.Symbols["ARM$M_PHYSICAL"]) // a device's, the same bit as Execute
	Logical  = Access(vmsdef.Symbols["ARM$M_LOGICAL"])  // a device's, the same bit as Delete
)

// codeAccess is every kind of access a protection code can give.
var codeAccess = Read | Write | Execute | Delete

// All is every kind of access, BYPASS's.
var All = codeAccess | Control

// String names a file's access as a protection code's letters, with C
// for control: "RWED", "RC", or "" for none.
func (a Access) String() string {
	var b strings.Builder

	for _, k := range []struct {
		bit    Access
		letter byte
	}{{Read, 'R'}, {Write, 'W'}, {Execute, 'E'}, {Delete, 'D'}, {Control, 'C'}} {
		if a&k.bit != 0 {
			b.WriteByte(k.letter)
		}
	}

	return b.String()
}

// Category is one of a protection code's four categories of user.
type Category int

// The categories, in the order of their fields in the mask.
const (
	System Category = iota
	Owner
	Group
	World
)

// Protection is a protection code: four 4-bit fields, SYSTEM's in bits
// 0-3, then OWNER, GROUP, and WORLD. A set bit *denies* that access, as
// VMS stores it in a file header and a XABPRO.
type Protection uint16

// NoAccess denies everything to everyone, (S,O,G,W).
const NoAccess Protection = 0xFFFF

// Allows returns the access category c's field gives.
func (p Protection) Allows(c Category) Access {
	return Access(^p>>(4*uint(c))) & codeAccess
}

// With returns p with category c's field set to give exactly access a
// (of which only the protection code's four kinds count).
func (p Protection) With(c Category, a Access) Protection {
	shift := 4 * uint(c)

	p &^= 0xF << shift

	return p | Protection(^a&codeAccess)<<shift
}

// Without returns p with access a denied to every category: a new
// directory's protection is its parent's without delete.
func (p Protection) Without(a Access) Protection {
	for c := System; c <= World; c++ {
		p = p.With(c, p.Allows(c)&^a)
	}

	return p
}
