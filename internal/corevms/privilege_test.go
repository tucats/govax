package corevms

import (
	"encoding/binary"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
)

// quadAt reads the quadword at addr.
func quadAt(t *testing.T, env *Environment, addr uint32) uint64 {
	t.Helper()

	v, err := env.mem.LoadQuadword(env.cpu, addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// quadArg writes v in a new quadword and returns its address.
func quadArg(a *arena, v uint64) uint32 {
	addr := a.alloc(8)
	putLongword(a.t, a.env, addr, uint32(v))
	putLongword(a.t, a.env, addr+4, uint32(v>>32))

	return addr
}

func TestPrivilegeBits(t *testing.T) {
	cases := map[string]uint64{
		"CMKRNL": 1 << 0, "SYSNAM": 1 << 2, "PRMMBX": 1 << 11, "ALTPRI": 1 << 13,
		"SETPRV": 1 << 14, "TMPMBX": 1 << 15, "SHARE": 1 << 31, "SECURITY": 1 << 38,
	}

	for name, want := range cases {
		if got := privilegeBit(name); got != want {
			t.Errorf("privilegeBit(%s) = %#x, want %#x", name, got, want)
		}
	}

	if allPrivileges != 1<<39-1 {
		t.Errorf("allPrivileges = %#x, want 39 bits", allPrivileges)
	}

	p := NewProcess()
	if p.CurrentPrivileges != allPrivileges || p.ProcessPrivileges != allPrivileges ||
		p.AuthorizedPrivileges != allPrivileges || p.ImagePrivileges != 0 {
		t.Error("the SYSTEM process should hold every privilege")
	}
}

func TestSetprv(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	prvprv := a.alloc(8)

	// Disable CMKRNL for the image only.
	wantR0(t, callLNM(t, env, serviceSysSetprv, 0, quadArg(a, privCMKRNL), 0, prvprv), ssNormal)

	if p.hasPrivilege(privCMKRNL) || p.ProcessPrivileges&privCMKRNL == 0 {
		t.Error("a temporary disable should change only the current mask")
	}

	if quadAt(t, env, prvprv) != allPrivileges {
		t.Errorf("prvprv %#x, want the previous current mask", quadAt(t, env, prvprv))
	}

	// Image rundown restores it.
	env.ImageRundown()

	if !p.hasPrivilege(privCMKRNL) {
		t.Error("rundown should restore the permanent privileges")
	}

	// Permanently: both masks; prvprv is the permanent mask's old value.
	wantR0(t, callLNM(t, env, serviceSysSetprv, 0, quadArg(a, privOPER|privWORLD), 1, prvprv), ssNormal)

	if p.hasAnyPrivilege(privOPER|privWORLD) || p.ProcessPrivileges&(privOPER|privWORLD) != 0 {
		t.Error("a permanent disable should change both masks")
	}

	env.ImageRundown()

	if p.hasAnyPrivilege(privOPER | privWORLD) {
		t.Error("a permanent disable should survive rundown")
	}

	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, quadArg(a, privOPER), 1), ssNormal)

	if !p.hasPrivilege(privOPER) || p.ProcessPrivileges&privOPER == 0 {
		t.Error("a permanent enable should change both masks")
	}

	// No prvadr: nothing changes, prvprv still reported.
	before := p.CurrentPrivileges

	wantR0(t, callLNM(t, env, serviceSysSetprv, 0, 0, 0, prvprv), ssNormal)

	if p.CurrentPrivileges != before || quadAt(t, env, prvprv) != before {
		t.Error("no prvadr should change nothing and report the mask")
	}

	// Bits past the last privilege are ignored.
	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, quadArg(a, ^uint64(0))), ssNormal)

	if p.CurrentPrivileges != allPrivileges {
		t.Errorf("mask %#x, want only the 39 privileges", p.CurrentPrivileges)
	}

	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, 0, 0, badAddr), ssAccVio)
}

// TestSetprv_authorization checks that only authorized privileges can be
// enabled from user mode without SETPRV.
func TestSetprv_authorization(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	p.AuthorizedPrivileges = privTMPMBX | privOPER
	p.CurrentPrivileges, p.ProcessPrivileges = 0, 0

	setMode(env, vax.User, vax.User, 0x9000)

	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, quadArg(a, privTMPMBX|privCMKRNL)), ssNotAllPriv)

	if p.CurrentPrivileges != privTMPMBX {
		t.Errorf("mask %#x, want TMPMBX only", p.CurrentPrivileges)
	}

	// Executive mode may enable anything.
	setMode(env, vax.Executive, vax.User, 0x9000)
	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, quadArg(a, privCMKRNL)), ssNormal)

	// So may a process authorized for SETPRV.
	setMode(env, vax.User, vax.User, 0x9000)

	p.AuthorizedPrivileges |= privSETPRV

	wantR0(t, callLNM(t, env, serviceSysSetprv, 1, quadArg(a, privSYSNAM)), ssNormal)

	if !p.hasPrivilege(privCMKRNL | privSYSNAM | privTMPMBX) {
		t.Errorf("mask %#x", p.CurrentPrivileges)
	}
}

func TestGetjpi_privileges(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	p.CurrentPrivileges &^= privCMKRNL | privSECURITYForTest
	p.AuthorizedPriority = 6

	bufs := map[string]uint32{}

	list := make([]item, 0, 5)

	for _, name := range []string{"JPI$_CURPRIV", "JPI$_PROCPRIV", "JPI$_AUTHPRIV", "JPI$_IMAGPRIV", "JPI$_AUTHPRI"} {
		bufs[name] = a.alloc(8)
		list = append(list, item{code: jpiCode(t, name), buflen: 8, buf: bufs[name]})
	}

	wantR0(t, getjpi(t, env, 0, 0, 0, a.items(list...), 0), ssNormal)

	quad := func(name string) uint64 {
		b := readBytes(t, env, bufs[name], 8)

		return binary.LittleEndian.Uint64(b)
	}

	if quad("JPI$_CURPRIV") != p.CurrentPrivileges || quad("JPI$_PROCPRIV") != allPrivileges ||
		quad("JPI$_AUTHPRIV") != allPrivileges || quad("JPI$_IMAGPRIV") != 0 {
		t.Errorf("privilege items %#x %#x %#x %#x", quad("JPI$_CURPRIV"), quad("JPI$_PROCPRIV"), quad("JPI$_AUTHPRIV"), quad("JPI$_IMAGPRIV"))
	}

	if got := a.readLong(bufs["JPI$_AUTHPRI"]); got != 6 {
		t.Errorf("JPI$_AUTHPRI = %d, want 6", got)
	}
}

// privSECURITYForTest is a privilege above bit 31, to show the quadword
// items carry the high longword.
var privSECURITYForTest = privilegeBit("SECURITY")

// TestPrivilegeChecks checks each service that needs a privilege fails
// with SS$_NOPRIV once it's disabled.
func TestPrivilegeChecks(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process

	without := func(mask uint64) { p.CurrentPrivileges = allPrivileges &^ mask }

	// $CMKRNL/$CMEXEC from user mode; not from executive mode.
	setMode(env, vax.User, vax.User, 0x9000)
	without(privCMKRNL | privCMEXEC)
	wantR0(t, callLNM(t, env, serviceSysCmkrnl, 0x5000, 0), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysCmexec, 0x5000, 0), ssNoPriv)

	setMode(env, vax.Executive, vax.User, 0x9000)

	if _, err := serviceSysCmkrnl(env, []uint32{0x5000, 0}); err == nil {
		t.Error("$CMKRNL from executive mode should call its routine without CMKRNL")
	}

	env.Process.cmode = nil
	setMode(env, vax.Kernel, vax.Kernel, 0x9000)

	// Mailboxes.
	without(privTMPMBX)

	r0, _ := crembx(t, env, a, 0, 0, 0, "")
	wantR0(t, r0, ssNoPriv)

	without(privPRMMBX)

	r0, _ = crembx(t, env, a, 1, 0, 0, "")

	wantR0(t, r0, ssNoPriv)

	without(privSYSNAM)

	r0, _ = crembx(t, env, a, 1, 0, 0, "PERMBOX")

	wantR0(t, r0, ssNoPriv)

	r0, ch := crembx(t, env, a, 1, 0, 0, "") // permanent, no name: fine
	wantR0(t, r0, ssNormal)

	without(privPRMMBX)
	wantR0(t, callLNM(t, env, serviceSysDelmbx, ch), ssNoPriv)

	// Common event flag clusters.
	without(privPRMCEB)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 70, a.desc("PERM"), 0, 1), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysAscefc, 70, a.desc("TEMP"), 0, 0), ssNormal)
	env.EventFlagClusters.clusters[clusterKey{p.UICGroup(), "TEMP"}].CreatorUIC = 0x00020001
	wantR0(t, callLNM(t, env, serviceSysDlcefc, a.desc("TEMP")), ssNoPriv)

	// $LCKPAG.
	without(privPSWAPM)
	wantR0(t, callLNM(t, env, serviceSysLckpag, a.alloc(8)), ssNoPriv)

	// $SETPRI: no higher than the authorized priority without ALTPRI.
	without(privALTPRI)
	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, 0, 10), ssNormal)

	if p.BasePriority != p.AuthorizedPriority {
		t.Errorf("priority %d, want the authorized %d", p.BasePriority, p.AuthorizedPriority)
	}

	wantR0(t, callLNM(t, env, serviceSysSetpri, 0, 0, 2), ssNormal) // lower is fine

	if p.BasePriority != 2 {
		t.Errorf("priority %d, want 2", p.BasePriority)
	}
}

// TestPrivilegeChecks_logicalNames checks the logical-name rules: the
// tables that need SYSNAM, GRPNAM, or SYSPRV, and SYSNAM's access mode
// rule.
func TestPrivilegeChecks_logicalNames(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	one := a.items(item{code: lnmString, buflen: 1, buf: a.str("X")})

	crelnm := func(table string, mode byte) uint32 {
		return callLNM(t, env, serviceSysCrelnm, 0, a.desc(table), a.desc("NAME"), a.byteArg(mode), one)
	}

	p.CurrentPrivileges = allPrivileges &^ (privSYSNAM | privSYSPRV | privGRPNAM)

	wantR0(t, crelnm("LNM$SYSTEM", 0), ssNoPriv)
	wantR0(t, crelnm("LNM$GROUP", 0), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysCrelog, 0, a.desc("NAME"), a.desc("X"), 0), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysDellnm, a.desc("LNM$SYSTEM"), a.desc("NAME"), 0), ssNoPriv)

	// Either of two privileges will do.
	p.CurrentPrivileges |= privSYSPRV

	wantR0(t, crelnm("LNM$SYSTEM", 0), ssNormal)

	p.CurrentPrivileges = allPrivileges &^ (privSYSNAM | privSYSPRV)
	p.CurrentPrivileges |= privGRPNAM

	wantR0(t, crelnm("LNM$GROUP", 0), ssNormal)

	// Without SYSNAM, a user-mode caller's kernel-mode name is user mode.
	setMode(env, vax.User, vax.User, 0x9000)
	wantR0(t, crelnm("LNM$PROCESS", 0), ssNormal)

	e, err := env.Logicals.Translate("LNM$PROCESS", "NAME", lnm.User, 0)
	if err != nil || e.Mode != lnm.User {
		t.Errorf("mode %v, %v; want user (maximized)", e, err)
	}

	// With it, as given.
	setMode(env, vax.Kernel, vax.Kernel, 0x9000)

	p.CurrentPrivileges = allPrivileges

	setMode(env, vax.User, vax.User, 0x9000)
	wantR0(t, crelnm("LNM$PROCESS", 1), ssNormal) // alongside the user-mode one

	if e, err := env.Logicals.Translate("LNM$PROCESS", "NAME", lnm.Executive, 0); err != nil || e.Mode != lnm.Executive {
		t.Errorf("mode %v, %v; want executive (SYSNAM)", e, err)
	}

	// A shareable table takes SYSPRV.
	p.CurrentPrivileges = allPrivileges &^ privSYSPRV
	r0 := callLNM(t, env, serviceSysCrelnt, 0, 0, 0, 0, 0, a.desc("SHARED_TAB"), a.desc("LNM$SYSTEM_DIRECTORY"), 0)
	wantR0(t, r0, ssNoPriv)
}
