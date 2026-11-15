package security

import "github.com/tucats/govax/internal/vmsdef"

// Credentials are what a process brings to an access check: its UIC, its
// current privileges, and the identifiers it holds.
type Credentials struct {
	UIC UIC

	// Privileges is the process's current privilege mask: bit n is the
	// privilege whose PRV$V_ value is n.
	Privileges uint64

	// Rights are the general identifiers the process holds besides its
	// UIC: the environmental ones (INTERACTIVE, LOCAL, ...) and any
	// granted to it.
	Rights []Identifier

	// MaxSysGroup is the SYSGEN parameter MAXSYSGROUP: a UIC group up to
	// it is a system group. 0 means its usual value, DefaultMaxSysGroup.
	MaxSysGroup uint32

	// DefaultProtection is the process's default file protection (SET
	// PROTECTION/DEFAULT, $SETDFPROT): what a new file gets when neither
	// a previous version nor its directory gives one. A zero code gives
	// everyone every access, as it would on VMS.
	DefaultProtection Protection

	// Trace, if not nil, is told of every access check made with these
	// credentials: a line saying who asked for what of which object, and
	// whether it was granted and by what (Check). It is SET DEBUG
	// PROTECTION's, which corevms fills in while the class is on.
	Trace func(line string)
}

// Identifier is a rights identifier a process holds, with the attributes
// it holds it with (KGB$M_RESOURCE, ...).
type Identifier struct {
	Value      uint32
	Attributes uint32
}

// DefaultFileProtection is the SYSGEN parameter RMS_FILEPROT's usual
// value, %XFA00, (S:RWED,O:RWED,G:RE,W): the default protection every
// account in Phase 93's probe started with.
const DefaultFileProtection Protection = 0xFA00

// DefaultMaxSysGroup is MAXSYSGROUP's usual value: groups up to octal 10
// are system groups.
const DefaultMaxSysGroup = 0o10

// The privileges protection looks at, as Credentials.Privileges bits.
var (
	PrivBYPASS  = privilege("BYPASS")
	PrivSYSPRV  = privilege("SYSPRV")
	PrivGRPPRV  = privilege("GRPPRV")
	PrivREADALL = privilege("READALL")
)

// AllPrivileges is every bit of a privilege mask: VMS's ALL is all 64
// bits, though only the first PRV$K_NUMBER_OF_PRIVS name privileges.
const AllPrivileges = ^uint64(0)

// SystemUIC is the SYSTEM account's UIC, [1,4].
var SystemUIC = NewUIC(1, 4)

// SystemCredentials are the SYSTEM account's: UIC [1,4] with every
// privilege, what govax's processes have unless something lowers them.
// Code given no credentials (an RMS call a test makes with none) acts
// with these.
func SystemCredentials() Credentials {
	return Credentials{UIC: SystemUIC, Privileges: AllPrivileges, DefaultProtection: DefaultFileProtection}
}

// AttributeResource is KGB$M_RESOURCE: the holder of an identifier with
// it may change the owner of objects the identifier owns.
var AttributeResource = uint32(vmsdef.Symbols["KGB$M_RESOURCE"])

// privilege is the mask bit of the $PRVDEF privilege called name.
func privilege(name string) uint64 {
	n, ok := vmsdef.Symbols["PRV$V_"+name]
	if !ok {
		panic("security: no $PRVDEF privilege " + name)
	}

	return 1 << n
}

// Has reports whether every privilege in mask is enabled.
func (c Credentials) Has(mask uint64) bool { return c.Privileges&mask == mask }

// InSystemGroup reports whether the process's UIC group is a system
// group.
func (c Credentials) InSystemGroup() bool {
	limit := c.MaxSysGroup
	if limit == 0 {
		limit = DefaultMaxSysGroup
	}

	return c.UIC.Group() <= limit
}

// Holds reports whether the process holds identifier id: a UIC-format
// one (which may have wildcards) by its UIC, a general one by its rights.
func (c Credentials) Holds(id uint32) bool {
	if !IsGeneral(id) {
		return matchesUIC(id, c.UIC)
	}

	_, ok := c.right(id)

	return ok
}

// HoldsAsResource reports whether the process holds id with the resource
// attribute. A UIC identifier is its own: a process holds its UIC as a
// resource (unconfirmed: the guide speaks of the attribute only for
// general identifiers, 4.8.4.3).
func (c Credentials) HoldsAsResource(id uint32) bool {
	if !IsGeneral(id) {
		return uint32(c.UIC) == id
	}

	r, ok := c.right(id)

	return ok && r.Attributes&AttributeResource != 0
}

// right finds the general identifier id among the process's rights.
func (c Credentials) right(id uint32) (Identifier, bool) {
	for _, r := range c.Rights {
		if r.Value == id {
			return r, true
		}
	}

	return Identifier{}, false
}

// holdsAll reports whether the process holds every identifier of an
// identifier ACE: an ACE naming several, written ID1+ID2, matches only a
// process holding them all.
func (c Credentials) holdsAll(ids []uint32) bool {
	if len(ids) == 0 {
		return false
	}

	for _, id := range ids {
		if !c.Holds(id) {
			return false
		}
	}

	return true
}
