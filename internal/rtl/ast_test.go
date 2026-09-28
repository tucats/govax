package rtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// astFixture returns an Environment ready to deliver ASTs: the CPU in
// mode at IPL 0 with a stack at 0x9000, and a SYS$CLRAST stub (the XFC
// an AST routine returns to) at a low address, since the test memory
// doesn't reach the real P1 vector at 0x7FFE....
func astFixture(t *testing.T, mode vax.AccessMode) *Environment {
	t.Helper()

	env, _ := fixture()

	saved := astExitAddr
	astExitAddr = 0x8000
	t.Cleanup(func() { astExitAddr = saved })

	if err := env.mem.StoreWord(env.cpu, astExitAddr, xfcP1VectorWord); err != nil {
		t.Fatal(err)
	}

	env.cpu.SetPSL(modePSL(mode))
	env.cpu.SetGPR(vax.SP, 0x9000)

	return env
}

// modePSL is a PSL of all zeros but its current mode.
func modePSL(mode vax.AccessMode) vax.PSL {
	var psl vax.PSL
	psl.SetCurMod(mode)

	return psl
}

// wantNoAST checks NextAST declines to deliver anything.
func wantNoAST(t *testing.T, env *Environment, why string) {
	t.Helper()

	if _, _, _, ok, err := env.NextAST(); ok || err != nil {
		t.Errorf("%s: NextAST delivered (ok=%v, err=%v), want nothing", why, ok, err)
	}
}

func TestServiceSysDclastQueues(t *testing.T) {
	env := astFixture(t, vax.Supervisor)

	// acmode is maximized with the caller's mode: kernel (0) becomes
	// supervisor; user (3) stays user.
	wantR0(t, callLNM(t, env, serviceSysDclast, 0x1000, 7, 0), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDclast, 0x2000, 8, 3), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDclast, 0x3000), ssNormal) // astprm and acmode omitted

	want := []astRequest{{0x1000, 7, 2}, {0x2000, 8, 3}, {0x3000, 0, 2}}
	for i, w := range want {
		if got := env.Process.ast.queue[i]; got != w {
			t.Errorf("queue[%d] = %+v, want %+v", i, got, w)
		}
	}
}

// TestNextASTDelivers: NextAST pushes the AST frame, marks the AST
// active, and returns the routine, argument list, and exit address.
func TestNextASTDelivers(t *testing.T) {
	env := astFixture(t, vax.User)
	c := env.cpu

	c.SetGPR(vax.R0, 0xAAAA)
	c.SetGPR(vax.R1, 0xBBBB)
	c.SetGPR(vax.PC, 0x4321)

	psl := c.PSL()
	psl.SetZ(true)
	c.SetPSL(psl)

	env.queueAST(0x1000, 0x55, uint32(vax.User))

	routine, argList, returnPC, ok, err := env.NextAST()
	if err != nil || !ok {
		t.Fatalf("NextAST: ok=%v err=%v, want an AST", ok, err)
	}

	if routine != 0x1000 || returnPC != astExitAddr {
		t.Errorf("routine=%#x returnPC=%#x, want 0x1000 and %#x", routine, returnPC, astExitAddr)
	}

	if argList != 0x9000-astFrameSize || c.GPR(vax.SP) != argList {
		t.Errorf("argList=%#x SP=%#x, want both %#x", argList, c.GPR(vax.SP), 0x9000-astFrameSize)
	}

	// count, astprm, R0, R1, PC, PSL.
	for i, want := range []uint32{5, 0x55, 0xAAAA, 0xBBBB, 0x4321, uint32(psl)} {
		got, _ := env.mem.LoadLongword(c, argList+uint32(i)*4)
		if got != want {
			t.Errorf("frame[%d] = %#x, want %#x", i, got, want)
		}
	}

	if env.PendingASTs() != 0 || !env.Process.ast.active[vax.User] {
		t.Errorf("after delivery: %d queued, active=%v; want 0 and active", env.PendingASTs(), env.Process.ast.active[vax.User])
	}

	// A second AST of the same mode waits for the first to finish.
	env.queueAST(0x2000, 0, uint32(vax.User))
	wantNoAST(t, env, "an AST already active in this mode")
}

// TestServiceSysClrastRestores: the AST exit puts back what NextAST
// saved, as the routine's RET would reach it: SP at the frame.
func TestServiceSysClrastRestores(t *testing.T) {
	env := astFixture(t, vax.User)
	c := env.cpu

	c.SetGPR(vax.R0, 0xAAAA)
	c.SetGPR(vax.R1, 0xBBBB)
	c.SetGPR(vax.PC, 0x4321)

	psl := c.PSL()
	psl.SetN(true)
	c.SetPSL(psl)

	env.queueAST(0x1000, 0, uint32(vax.User))
	if _, _, _, ok, _ := env.NextAST(); !ok {
		t.Fatal("NextAST delivered nothing")
	}

	// The AST routine runs and clobbers everything.
	c.SetGPR(vax.R0, 1)
	c.SetGPR(vax.R1, 2)
	c.SetGPR(vax.PC, astExitAddr+2)
	c.SetPSL(modePSL(vax.User))

	r0 := callLNM(t, env, serviceSysClrast)

	if r0 != 0xAAAA || c.GPR(vax.R1) != 0xBBBB || c.GPR(vax.PC) != 0x4321 || c.PSL() != psl {
		t.Errorf("restored R0=%#x R1=%#x PC=%#x PSL=%#x; want 0xAAAA 0xBBBB 0x4321 %#x",
			r0, c.GPR(vax.R1), c.GPR(vax.PC), uint32(c.PSL()), uint32(psl))
	}

	if c.GPR(vax.SP) != 0x9000 {
		t.Errorf("SP = %#x, want 0x9000: the frame removed", c.GPR(vax.SP))
	}

	if env.Process.ast.active[vax.User] {
		t.Error("the AST is still active after its exit")
	}
}

// TestServiceSysClrastNotAnAST: reached with no AST active, or with SP
// somewhere other than the AST's frame, the exit changes nothing.
func TestServiceSysClrastNotAnAST(t *testing.T) {
	env := astFixture(t, vax.User)
	c := env.cpu

	c.SetGPR(vax.R1, 0x77)
	wantR0(t, callLNM(t, env, serviceSysClrast), ssNormal)

	env.queueAST(0x1000, 0, uint32(vax.User))
	env.NextAST()
	c.SetGPR(vax.SP, c.GPR(vax.SP)-4) // unbalanced stack

	wantR0(t, callLNM(t, env, serviceSysClrast), ssNormal)

	if c.GPR(vax.R1) != 0x77 || !env.Process.ast.active[vax.User] {
		t.Errorf("R1=%#x active=%v; want nothing restored and the AST still active", c.GPR(vax.R1), env.Process.ast.active[vax.User])
	}
}

// TestServiceSysClrastKeepsMode: a PSL in the frame can't change the
// mode the AST exit returns to.
func TestServiceSysClrastKeepsMode(t *testing.T) {
	env := astFixture(t, vax.User)
	c := env.cpu

	env.queueAST(0x1000, 0, uint32(vax.User))
	_, argList, _, _, _ := env.NextAST()

	putLongword(t, env, argList+20, uint32(modePSL(vax.Kernel))) // forged: kernel
	callLNM(t, env, serviceSysClrast)

	if c.PSL().CurMod() != vax.User {
		t.Errorf("mode after the AST exit = %v, want user", c.PSL().CurMod())
	}
}

// TestClrastReachedWithoutArgumentList: SystemService doesn't read an
// argument list for SYS$CLRAST — AP is the interrupted code's, and may
// point anywhere.
func TestClrastReachedWithoutArgumentList(t *testing.T) {
	env := astFixture(t, vax.User)
	env.cpu.SetGPR(vax.AP, 0x7FFFFFF0) // unreadable

	var clrast uint32

	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$CLRAST" {
			clrast = e.Addr
		}
	}

	r0, handled, err := env.SystemService(clrast)
	if !handled || err != nil || r0 != ssNormal {
		t.Errorf("SystemService(SYS$CLRAST) = %#x, %v, %v; want SS$_NORMAL, handled, no error", r0, handled, err)
	}
}

func TestNextASTConditions(t *testing.T) {
	env := astFixture(t, vax.Supervisor)
	c := env.cpu
	p := env.Process

	wantNoAST(t, env, "nothing queued")

	// Queued for another mode: kernel and user ASTs don't run in
	// supervisor mode.
	env.queueAST(0x1000, 0, uint32(vax.Kernel))
	env.queueAST(0x1000, 0, uint32(vax.User))
	wantNoAST(t, env, "only other modes' ASTs queued")

	env.queueAST(0x2000, 0, uint32(vax.Supervisor))

	// IPL 2 or above.
	psl := c.PSL()
	psl.SetIPL(2)
	c.SetPSL(psl)
	wantNoAST(t, env, "IPL 2")

	// On the interrupt stack.
	psl.SetIPL(0)
	psl.SetIS(true)
	c.SetPSL(psl)
	wantNoAST(t, env, "on the interrupt stack")

	psl.SetIS(false)
	c.SetPSL(psl)

	// Disabled in this mode, or a more privileged one.
	p.ast.enabled[vax.Supervisor] = false
	wantNoAST(t, env, "ASTs disabled in supervisor mode")

	p.ast.enabled[vax.Supervisor] = true
	p.ast.enabled[vax.Executive] = false
	wantNoAST(t, env, "ASTs disabled in executive mode")

	p.ast.enabled[vax.Executive] = true

	// Now it goes: the supervisor AST, skipping the others in the queue.
	routine, _, _, ok, err := env.NextAST()
	if !ok || err != nil || routine != 0x2000 {
		t.Fatalf("NextAST = %#x, %v, %v; want the supervisor AST", routine, ok, err)
	}

	if env.PendingASTs() != 2 {
		t.Errorf("%d ASTs left, want the kernel and user ones", env.PendingASTs())
	}
}

func TestNextASTInOrder(t *testing.T) {
	env := astFixture(t, vax.User)

	for _, r := range []uint32{0x1000, 0x2000, 0x3000} {
		env.queueAST(r, 0, uint32(vax.User))
	}

	for _, want := range []uint32{0x1000, 0x2000, 0x3000} {
		routine, _, _, ok, _ := env.NextAST()
		if !ok || routine != want {
			t.Fatalf("NextAST = %#x (ok=%v), want %#x", routine, ok, want)
		}

		callLNM(t, env, serviceSysClrast) // the routine returns
	}
}

func TestNextASTErrors(t *testing.T) {
	// No stub to return through.
	env := astFixture(t, vax.User)
	putLongword(t, env, astExitAddr, 0)
	env.queueAST(0x1000, 0, uint32(vax.User))

	if _, _, _, ok, err := env.NextAST(); ok || err == nil || !strings.Contains(err.Error(), ".P1VECTOR") {
		t.Errorf("NextAST without the stub: ok=%v err=%v, want a .P1VECTOR error", ok, err)
	}

	// A stack that can't take the frame: SP is left alone.
	env = astFixture(t, vax.User)
	env.cpu.SetGPR(vax.SP, 0x7FFFFFF0)
	env.queueAST(0x1000, 0, uint32(vax.User))

	if _, _, _, ok, err := env.NextAST(); ok || err == nil {
		t.Errorf("NextAST with a bad stack: ok=%v err=%v, want an error", ok, err)
	}

	if env.cpu.GPR(vax.SP) != 0x7FFFFFF0 {
		t.Errorf("SP = %#x after a failed push, want it unchanged", env.cpu.GPR(vax.SP))
	}
}

func TestServiceSysSetast(t *testing.T) {
	env := astFixture(t, vax.User)
	p := env.Process

	// Enabled to start with; each call reports the previous state.
	wantR0(t, callLNM(t, env, serviceSysSetast, 0), ssWasSet)
	wantR0(t, callLNM(t, env, serviceSysSetast, 0), ssWasClr)

	env.queueAST(0x1000, 0, uint32(vax.User))
	wantNoAST(t, env, "ASTs disabled")

	// Only the low byte counts: 0x100 disables.
	wantR0(t, callLNM(t, env, serviceSysSetast, 0x100), ssWasClr)
	wantR0(t, callLNM(t, env, serviceSysSetast, 1), ssWasClr)

	if _, _, _, ok, _ := env.NextAST(); !ok {
		t.Error("the queued AST wasn't delivered once ASTs were enabled again")
	}

	// Only the caller's mode is affected.
	for m, on := range p.ast.enabled {
		if !on {
			t.Errorf("mode %d disabled, want only user mode touched and re-enabled", m)
		}
	}
}

func TestASTImageRundown(t *testing.T) {
	env := astFixture(t, vax.User)
	p := env.Process

	env.queueAST(0x1000, 0, uint32(vax.User))
	env.queueAST(0x2000, 0, uint32(vax.Kernel))
	p.ast.enabled[vax.User] = false
	p.ast.active[vax.User] = true

	env.ImageRundown()

	if env.PendingASTs() != 1 || p.ast.queue[0].mode != uint32(vax.Kernel) {
		t.Errorf("queue after rundown = %+v, want just the kernel AST", p.ast.queue)
	}

	if !p.ast.enabled[vax.User] || p.ast.active[vax.User] {
		t.Error("user mode not reset to ASTs enabled, none active")
	}
}
