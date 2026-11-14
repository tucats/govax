package security

import "fmt"

// UIC is a user identification code: the group number in the high word
// and the member number in the low, as VMS stores it in a longword. As
// an identifier its bit 31 is clear; a general identifier has it set.
type UIC uint32

// Wildcards in an identifier ACE's UIC: [g,*] names every member of group
// g, and [*,*] everyone. The group field of an identifier is 14 bits
// (bits 30 and 31 say what kind of identifier it is), so a wildcard
// group is %X3FFF; a wildcard member is %XFFFF. VMS 7.1 stored [200,*]
// as %X0080FFFF and [*,*] as %X3FFFFFFF (A.4, "ACE layouts").
const (
	WildGroup  = 0x3FFF
	WildMember = 0xFFFF
)

// NewUIC makes a UIC from its group and member numbers.
func NewUIC(group, member uint32) UIC {
	return UIC(group<<16 | member&0xFFFF)
}

// Group is the UIC's group number.
func (u UIC) Group() uint32 { return uint32(u) >> 16 }

// Member is the UIC's member number.
func (u UIC) Member() uint32 { return uint32(u) & 0xFFFF }

// String is the UIC in numeric form, [group,member] in octal.
func (u UIC) String() string {
	return fmt.Sprintf("[%o,%o]", u.Group(), u.Member())
}

// generalBit marks a general identifier: an identifier whose bit 31 is
// set is not a UIC.
const generalBit = 0x80000000

// IsGeneral reports whether identifier id is a general identifier rather
// than a UIC.
func IsGeneral(id uint32) bool { return id&generalBit != 0 }

// matchesUIC reports whether the UIC-format identifier id, which may have
// a wildcard group or member, names the process whose UIC is u.
func matchesUIC(id uint32, u UIC) bool {
	group, member := id>>16&WildGroup, id&0xFFFF

	return (group == WildGroup || group == u.Group()) &&
		(member == WildMember || member == u.Member())
}
