package lnm

// A logical name table's security profile: an owner UIC and a protection
// mask, as a file has. VMS lets a process write names in a shareable
// table when the protection grants it write access, or when it holds a
// privilege that overrides the protection for that table (SYSNAM for the
// system table, GRPNAM for its group's); the check is corevms's, which
// asks internal/security's rule with these fields. This package only
// keeps them.
//
// The defaults are what SHOW SECURITY showed on VMS 7.1
// (testdata/priv94, the phase doc's A.4):
//
//	LNM$SYSTEM_TABLE, LNM$SYSTEM_DIRECTORY   [SYSTEM]   (S:RWC,O:RWC,G:R,W:R)
//	LNM$GROUP_gggggg                         [g,*]      (S:RWCD,O:R,G:R,W)
//	a table $CRELNT makes                    its maker  the promsk it was given
//
// A job table's (owner the job's UIC, (S:RWCD,O:RWCD,G,W)) and a
// process-private table's (the process's, with the same code) are
// govax's choices: the probe didn't show them, and only a shareable
// table's profile is checked.

// SystemUIC is [1,4], SYSTEM's UIC, the owner of the system tables.
const SystemUIC = 0x00010004

// The protection masks the standard tables start with.
const (
	// SystemTableProtection is (S:RWC,O:RWC,G:R,W:R): the system table's,
	// the system directory's, and the date and time format table's.
	SystemTableProtection uint16 = 0xEE88

	// GroupTableProtection is (S:RWCD,O:R,G:R,W): a group table's.
	GroupTableProtection uint16 = 0xFEE0

	// OwnerProtection is (S:RWCD,O:RWCD,G,W): a job table's and a
	// process-private table's (unconfirmed).
	OwnerProtection uint16 = 0xFF00
)

// groupOwner is the owner of group g's table: the group's identifier,
// [g,*], which a UIC shows as its group with member %XFFFF.
func groupOwner(uic uint32) uint32 {
	return uic&0xFFFF0000 | 0xFFFF
}

// TableProfile is what a new table is given beside its name: its owner,
// protection, and quota ($CRELNT's promsk and quota, and its caller's
// UIC).
type TableProfile struct {
	Owner      uint32
	Protection uint16

	// Quota is the table's quota in bytes, 0 for none.
	Quota uint32
}

// UIC is the UIC of the process this view belongs to.
func (db *Database) UIC() uint32 {
	return db.uic
}

// SetUIC gives the process this view belongs to the UIC uic, as a login
// does: LNM$GROUP becomes its group's table (made if its group has none
// yet), and the job's table is owned by uic. Tests use it to make a
// process another user.
func (db *Database) SetUIC(uic uint32) {
	db.uic = uic
	db.GroupTableName = groupTableName(uic)

	if !db.SystemDirectory.hasName(db.GroupTableName) {
		db.newGroupTable(uic)
	}

	if e := db.ProcessDirectory.atMode("LNM$GROUP", Executive); e != nil {
		e.Equivalences = []Equivalence{{Value: db.GroupTableName}}
	}

	for _, t := range db.tables {
		t.Owner = uic
	}

	if e := db.SystemDirectory.atMode(db.JobTableName, Kernel); e != nil && e.Target != nil {
		e.Target.Owner = uic
	}
}

// newGroupTable makes the group table of the process whose UIC is uic.
func (db *Database) newGroupTable(uic uint32) {
	t := db.newPermanentTable(groupTableName(uic), db.SystemDirectory)
	t.Owner, t.Protection = groupOwner(uic), GroupTableProtection
}
