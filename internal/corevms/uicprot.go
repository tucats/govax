package corevms

// UIC-based protection (docs/PHASE-46.md, subtask 3): the check VMS
// makes before letting a process use an object that has an owner and a
// protection mask, such as a mailbox.
//
// # The protection mask
//
// An object's protection is a 16-bit mask of four 4-bit fields, one for
// each category of user:
//
//	bits  0-3   SYSTEM   processes in a system group (MAXSYSGROUP or
//	                     lower), or with the SYSPRV privilege
//	bits  4-7   OWNER    processes whose UIC is the object's owner's
//	bits  8-11  GROUP    processes in the owner's UIC group
//	bits 12-15  WORLD    every process
//
// Within a field, bit 0 is read access, bit 1 write, bit 2 execute (for
// a device, logical I/O), and bit 3 delete (for a device, physical I/O).
// A *set* bit denies that access to that category; a mask of 0 allows
// everything to everyone. A process falls in every category whose test
// it passes, and gets an access if any of its categories allows it.
//
// Privileges: BYPASS allows every access; SYSPRV puts the process in
// the SYSTEM category, and GRPPRV does too for an object owned by its
// group; READALL allows read access. (DIGITAL's *Guide to VMS System
// Security*.) Access control lists aren't modeled.

// The access kinds, as bits within a category's field.
const (
	accessRead uint32 = 1 << iota
	accessWrite
	accessLogical
	accessPhysical
)

// The protection mask's categories: each one's field begins at this bit.
const (
	protSystem = 0
	protOwner  = 4
	protGroup  = 8
	protWorld  = 12
)

// maxSysGroup is the SYSGEN parameter MAXSYSGROUP at its usual value:
// UIC groups up to octal 10 are system groups.
const maxSysGroup = 0o10

// The privileges protection checks look at.
var (
	privBYPASS  = privilegeBit("BYPASS")
	privGRPPRV  = privilegeBit("GRPPRV")
	privREADALL = privilegeBit("READALL")
)

// uicAccess reports whether process p may have every access in want
// (accessRead, accessWrite, ...) to an object owned by owner (a UIC)
// with protection mask mask, by the rules in this file's opening
// comment.
func (p *Process) uicAccess(owner uint32, mask uint16, want uint32) bool {
	if p.hasPrivilege(privBYPASS) {
		return true
	}

	if p.hasPrivilege(privREADALL) {
		want &^= accessRead
	}

	if want == 0 {
		return true
	}

	sameGroup := p.UIC>>16 == owner>>16

	categories := []struct {
		applies bool
		shift   uint
	}{
		{p.UIC>>16 <= maxSysGroup || p.hasPrivilege(privSYSPRV) || (sameGroup && p.hasPrivilege(privGRPPRV)), protSystem},
		{p.UIC == owner, protOwner},
		{sameGroup, protGroup},
		{true, protWorld},
	}

	allowed := uint32(0)

	for _, c := range categories {
		if c.applies {
			allowed |= ^uint32(mask>>c.shift) & 0xF
		}
	}

	return allowed&want == want
}
