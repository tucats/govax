package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// TestShowSecurityTables checks SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE
// against what VMS 7.1 showed (testdata/priv94's DCL logs): the standard
// tables' profiles, a table made closed to the world, and that a process
// outside the system category needs read access to see a table's profile,
// which READALL gives and SECURITY doesn't.
func TestShowSecurityTables(t *testing.T) {
	t.Parallel()
	d, c, buf := newCommandDispatcher(t)

	if _, _, err := c.RTL.Logicals.CreateTableWith("PRIV94_PT", lnm.SystemDirectoryName, lnm.User, 0,
		lnm.TableProfile{Owner: lnm.SystemUIC, Protection: 0xFF00}); err != nil {
		t.Fatal(err)
	}

	show := func(table string) (string, error) {
		buf.Reset()
		err := d.Dispatch("SHOW SECURITY/CLASS=LOGICAL_NAME_TABLE " + table)

		return strings.TrimSpace(buf.String()), err
	}

	for _, tc := range []struct{ table, want string }{
		{"LNM$SYSTEM_TABLE", "LNM$SYSTEM_TABLE object of class LOGICAL_NAME_TABLE\n" +
			"     Owner: [SYSTEM]\n" +
			"     Protection: (System: RWC, Owner: RWC, Group: R, World: R)\n" +
			"     Access Control List: <empty>"},
		{"LNM$SYSTEM_DIRECTORY", "LNM$SYSTEM_DIRECTORY object of class LOGICAL_NAME_TABLE\n" +
			"     Owner: [SYSTEM]\n" +
			"     Protection: (System: RWC, Owner: RWC, Group: R, World: R)\n" +
			"     Access Control List: <empty>"},
		{"LNM$GROUP_000001", "LNM$GROUP_000001 object of class LOGICAL_NAME_TABLE\n" +
			"     Owner: [1,*]\n" +
			"     Protection: (System: RWCD, Owner: R, Group: R, World)\n" +
			"     Access Control List: <empty>"},
		{"PRIV94_PT", "PRIV94_PT object of class LOGICAL_NAME_TABLE\n" +
			"     Owner: [SYSTEM]\n" +
			"     Protection: (System: RWCD, Owner: RWCD, Group, World)\n" +
			"     Access Control List: <empty>"},
	} {
		got, err := show(tc.table)
		if err != nil || got != tc.want {
			t.Errorf("SYSTEM, %s:\n got %s (%v)\nwant %s", tc.table, got, err, tc.want)
		}
	}

	// [200,3], holding TMPMBX and NETMBX, then SECURITY too, then READALL.
	p := c.RTL.Process
	p.UIC = 0x00800003
	c.RTL.Logicals.SetUIC(p.UIC)

	for _, privs := range [][]string{{"TMPMBX", "NETMBX"}, {"TMPMBX", "NETMBX", "SECURITY"}} {
		p.CurrentPrivileges = privilegeMask(privs...)

		if got, err := show("PRIV94_PT"); err == nil || !strings.Contains(err.Error(), "NOPRIV") {
			t.Errorf("%v, PRIV94_PT: %q, %v; want NOPRIV", privs, got, err)
		}

		want := "LNM$GROUP_000200 object of class LOGICAL_NAME_TABLE\n" +
			"     Owner: [200,*]\n" +
			"     Protection: (System: RWCD, Owner: R, Group: R, World)\n" +
			"     Access Control List: <empty>"
		if got, err := show("LNM$GROUP_000200"); err != nil || got != want {
			t.Errorf("%v, the group's table:\n got %s (%v)\nwant %s", privs, got, err, want)
		}
	}

	p.CurrentPrivileges = privilegeMask("READALL")

	if _, err := show("PRIV94_PT"); err != nil {
		t.Errorf("READALL, PRIV94_PT: %v", err)
	}
}

// privilegeMask is a privilege mask of the privileges names, by their
// $PRVDEF names.
func privilegeMask(names ...string) uint64 {
	var m uint64

	for _, n := range names {
		m |= 1 << vmsdef.Symbols["PRV$V_"+n]
	}

	return m
}
