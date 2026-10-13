package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestItemText: each form's text, and where DCL's and LIB$'s styles
// differ (docs/PHASE-55, subtask 3). None of these forms reads the
// process, so there's no Environment.
func TestItemText(t *testing.T) {
	var env *Environment

	tmpmbx := uint64(1) << vmsdef.Symbols["PRV$V_TMPMBX"]
	netmbx := uint64(1) << vmsdef.Symbols["PRV$V_NETMBX"]
	f11v2 := vmsdef.Symbols["DVI$C_ACP_F11V2"]

	cases := []struct {
		form  ItemForm
		data  string
		style Style
		want  string
	}{
		{FormNumber, itemLong(0xFFFFFFFF).data, StyleDCL, "-1"},
		{FormNumber, itemLong(0xFFFFFFFF).data, StyleLIB, "4294967295"},
		{FormNumber, itemQuad(1 << 40).data, StyleLIB, "1099511627776"},
		{FormNumber, itemByte(1).data, StyleLIB, "1"},
		{FormString, "SYSTEM      ", StyleLIB, "SYSTEM      "},
		{FormHex, itemLong(0x2A).data, StyleLIB, "0000002A"},
		{FormPrivileges, itemQuad(tmpmbx | netmbx).data, StyleLIB, "TMPMBX,NETMBX"},
		{FormPrivileges, itemQuad(0).data, StyleDCL, ""},
		{FormBoolean, itemLong(3).data, StyleLIB, "TRUE"},
		{FormBoolean, itemLong(2).data, StyleDCL, "FALSE"},
		{FormMode, itemLong(vmsdef.Symbols["JPI$K_BATCH"]).data, StyleLIB, "BATCH"},
		{FormMode, itemLong(jpiOther).data, StyleLIB, "OTHER"},
		{FormState, itemLong(vmsdef.Symbols["SCH$C_HIB"]).data, StyleLIB, "HIB"},
		{FormACP, itemLong(f11v2).data, StyleLIB, "F11V2"},
		{FormACP, itemLong(0).data, StyleLIB, "UNKNOWN"},
		{FormACP, itemLong(0).data, StyleDCL, "ILLEGAL"},
		{FormACP, itemLong(vmsdef.Symbols["DVI$C_ACP_JNL"]).data, StyleLIB, "JNL"},
		{FormProtection, itemLong(0).data, StyleLIB, "SYSTEM=RWPL,OWNER=RWPL,GROUP=RWPL,WORLD=RWPL"},
		{FormProtection, itemLong(0xFA80).data, StyleLIB, "SYSTEM=RWPL,OWNER=RWP,GROUP=RP,WORLD"},
		{FormProtection, itemLong(0).data, StyleDCL, "SYSTEM=RWED, OWNER=RWED, GROUP=RWED, WORLD=RWED"},
		{FormProtection, itemLong(0xFA80).data, StyleDCL, "SYSTEM=RWED, OWNER=RWE, GROUP=RE, WORLD"},
		{FormOwner, itemLong(0).data, StyleDCL, ""},
		{FormOwner, itemLong(0).data, StyleLIB, "00000000"},
		{FormOwner, itemLong(0x2A).data, StyleDCL, "0000002A"},
		{FormPrivileges, itemQuad(1 << 5 | 1 << 9 | 1 << 13).data, StyleLIB, "IMPERSONATE,DETACH,NOACNT,SETPRI"},
		{FormPrivileges, itemQuad(1 << 5 | 1 << 9 | 1 << 13).data, StyleDCL, "DETACH,ACNT,ALTPRI"},
	}

	for _, c := range cases {
		if got := env.ItemText(c.form, c.data, c.style); got != c.want {
			t.Errorf("form %d, style %d, % X: %q, want %q", c.form, c.style, c.data, got, c.want)
		}
	}
}

// TestItemForms: the item tables' forms, which the two styles share
// (testdata/lib55: VMS 7.1 writes the owner's PID and F$GETSYI's special
// forms in LIB$'s style too).
func TestItemForms(t *testing.T) {
	cases := []struct {
		got, want ItemForm
		what      string
	}{
		{JPIItemForm("JPI$_PID", StyleLIB), FormHex, "PID"},
		{JPIItemForm("JPI$_OWNER", StyleDCL), FormOwner, "OWNER, DCL"},
		{JPIItemForm("JPI$_OWNER", StyleLIB), FormOwner, "OWNER, LIB"},
		{JPIItemForm("JPI$_USERNAME", StyleLIB), FormString, "USERNAME"},
		{JPIItemForm("JPI$_BUFIO", StyleLIB), FormNumber, "BUFIO"},
		{SYIItemForm("SYI$_BOOTTIME", StyleDCL), FormTime, "BOOTTIME, DCL"},
		{SYIItemForm("SYI$_BOOTTIME", StyleLIB), FormTime, "BOOTTIME, LIB"},
		{SYIItemForm("SYI$_NODE_CSID", StyleLIB), FormHex, "NODE_CSID, LIB"},
		{SYIItemForm("SYI$_NODENAME", StyleLIB), FormString, "NODENAME"},
		{DVIItemForm("DVI$_MNT", StyleLIB), FormBoolean, "MNT"},
		{DVIItemForm("DVI$_TT_NOECHO", StyleLIB), FormBoolean, "TT_NOECHO"},
		{DVIItemForm("DVI$_TT_PAGE", StyleLIB), FormNumber, "TT_PAGE"},
		{DVIItemForm("DVI$_TT_PHYDEVNAM", StyleLIB), FormString, "TT_PHYDEVNAM"},
		{DVIItemForm("DVI$_VPROT", StyleDCL), FormProtection, "VPROT"},
		{DVIItemForm("DVI$_ACPTYPE", StyleLIB), FormACP, "ACPTYPE"},
		{DVIItemForm("DVI$_DEVCLASS", StyleLIB), FormNumber, "DEVCLASS"},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: form %d, want %d", c.what, c.got, c.want)
		}
	}
}

// TestItemLongword: an item's value read from its bytes, short data
// padded with zeros.
func TestItemLongword(t *testing.T) {
	if v := ItemLongword("\x01\x02"); v != 0x0201 {
		t.Errorf("short: %#x", v)
	}

	if v := ItemQuadword(itemQuad(0x0102030405060708).data); v != 0x0102030405060708 {
		t.Errorf("quadword: %#x", v)
	}
}
