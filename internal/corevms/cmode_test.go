package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// setMode puts the CPU in mode cur with previous mode prv, with sp as the
// current stack pointer.
func setMode(env *Environment, cur, prv vax.AccessMode, sp uint32) {
	psl := env.cpu.PSL()
	psl.SetCurMod(cur)
	psl.SetPrvMod(prv)
	env.cpu.SetPSL(psl)
	env.cpu.SetGPR(vax.SP, sp)
}

// TestChangeMode steps a $CMKRNL call from user mode through both of its
// runs, standing in for the engine: the switch to kernel mode and the
// routine's call request, then (after the "routine" sets R0) the switch
// back, with its R0 as the status.
func TestChangeMode(t *testing.T) {
	env, _ := fixture()
	c := env.cpu

	const routine, arglst, fp, usp, ksp = 0x5000, 0x6000, 0x7000, 0x8000, 0x9000

	c.SetPR(vax.KSP, ksp)
	c.SetGPR(vax.FP, fp)
	setMode(env, vax.User, vax.User, usp)

	r0, err := serviceSysCmkrnl(env, []uint32{routine, arglst})

	call, ok := err.(*CallRequest)
	if !ok || r0 != 0 || call.Routine != routine || call.ArgList != arglst {
		t.Fatalf("$CMKRNL = %#x, %v; want a call of %#x with %#x", r0, err, routine, arglst)
	}

	if psl := c.PSL(); psl.CurMod() != vax.Kernel || psl.PrvMod() != vax.User {
		t.Errorf("during the call: mode %d, previous %d; want kernel, user", psl.CurMod(), psl.PrvMod())
	}

	if c.GPR(vax.SP) != ksp || c.PR(vax.USP) != usp {
		t.Errorf("during the call: SP %#x, USP %#x; want the kernel stack %#x, USP %#x", c.GPR(vax.SP), c.PR(vax.USP), ksp, usp)
	}

	// The routine returns (RET leaves FP and SP as they were) with a
	// status in R0.
	c.SetGPR(vax.R0, 0x1234)

	r0, err = serviceSysCmkrnl(env, []uint32{routine, arglst})
	if err != nil || r0 != 0x1234 {
		t.Fatalf("the return = %#x, %v; want the routine's R0 0x1234", r0, err)
	}

	if psl := c.PSL(); psl.CurMod() != vax.User || psl.PrvMod() != vax.User {
		t.Errorf("after the call: mode %d, previous %d; want user, user", psl.CurMod(), psl.PrvMod())
	}

	if c.GPR(vax.SP) != usp || c.PR(vax.KSP) != ksp {
		t.Errorf("after the call: SP %#x, KSP %#x; want %#x, %#x", c.GPR(vax.SP), c.PR(vax.KSP), usp, ksp)
	}

	if len(env.Process.cmode) != 0 {
		t.Errorf("%d calls left in progress", len(env.Process.cmode))
	}
}

func TestChangeMode_modes(t *testing.T) {
	cases := []struct {
		name   string
		fn     ServiceFunc
		caller vax.AccessMode
		want   vax.AccessMode
	}{
		{"$CMEXEC from user", serviceSysCmexec, vax.User, vax.Executive},
		{"$CMEXEC from supervisor", serviceSysCmexec, vax.Supervisor, vax.Executive},
		// Never less privileged than the caller.
		{"$CMEXEC from kernel", serviceSysCmexec, vax.Kernel, vax.Kernel},
		{"$CMKRNL from executive", serviceSysCmkrnl, vax.Executive, vax.Kernel},
		{"$CMKRNL from kernel", serviceSysCmkrnl, vax.Kernel, vax.Kernel},
	}

	for _, tc := range cases {
		env, _ := fixture()
		c := env.cpu

		for m := vax.Kernel; m <= vax.User; m++ {
			c.SetPR(vax.PrivReg(m), 0x1000*(uint32(m)+1))
		}

		c.SetGPR(vax.FP, 0x7000)
		setMode(env, tc.caller, vax.Supervisor, 0x1000*(uint32(tc.caller)+1))

		if _, err := tc.fn(env, []uint32{0x5000}); err == nil {
			t.Fatalf("%s: no call request", tc.name)
		}

		if got := c.PSL().CurMod(); got != tc.want {
			t.Errorf("%s: the routine runs in mode %d, want %d", tc.name, got, tc.want)
		}

		wantSP := 0x1000 * (uint32(tc.want) + 1)
		if got := c.GPR(vax.SP); got != wantSP {
			t.Errorf("%s: SP %#x, want %#x", tc.name, got, wantSP)
		}

		// Returning restores the caller's mode and previous mode.
		c.SetGPR(vax.R0, 1)

		if r0, err := tc.fn(env, []uint32{0x5000}); err != nil || r0 != 1 {
			t.Fatalf("%s: return = %#x, %v", tc.name, r0, err)
		}

		if psl := c.PSL(); psl.CurMod() != tc.caller || psl.PrvMod() != vax.Supervisor {
			t.Errorf("%s: back in mode %d, previous %d; want %d, supervisor", tc.name, psl.CurMod(), psl.PrvMod(), tc.caller)
		}
	}
}

// TestChangeMode_nested has the kernel routine call $CMKRNL itself: the
// inner call is a new call (its stub has a different frame), and each
// return ends the right one.
func TestChangeMode_nested(t *testing.T) {
	env, _ := fixture()
	c := env.cpu

	c.SetPR(vax.KSP, 0x9000)
	c.SetGPR(vax.FP, 0x7000)
	setMode(env, vax.User, vax.User, 0x8000)

	if _, err := serviceSysCmkrnl(env, []uint32{0x5000}); err == nil {
		t.Fatal("no outer call")
	}

	// Inside the routine: a new stub frame, lower on the kernel stack.
	c.SetGPR(vax.FP, 0x8F00)
	c.SetGPR(vax.SP, 0x8E00)

	if _, err := serviceSysCmkrnl(env, []uint32{0x5100}); err == nil {
		t.Fatal("no inner call")
	}

	if len(env.Process.cmode) != 2 {
		t.Fatalf("%d calls in progress, want 2", len(env.Process.cmode))
	}

	// The inner routine returns: still kernel mode.
	c.SetGPR(vax.R0, 3)

	if r0, _ := serviceSysCmkrnl(env, []uint32{0x5100}); r0 != 3 || c.PSL().CurMod() != vax.Kernel {
		t.Fatalf("inner return = %#x in mode %d; want 3 in kernel", r0, c.PSL().CurMod())
	}

	// Then the outer one: back to user mode.
	c.SetGPR(vax.FP, 0x7000)
	c.SetGPR(vax.SP, 0x9000)
	c.SetGPR(vax.R0, 5)

	if r0, _ := serviceSysCmkrnl(env, []uint32{0x5000}); r0 != 5 || c.PSL().CurMod() != vax.User {
		t.Fatalf("outer return = %#x in mode %d; want 5 in user", r0, c.PSL().CurMod())
	}

	// Image rundown forgets a call whose routine never returned.
	c.SetGPR(vax.FP, 0x7000)

	if _, err := serviceSysCmkrnl(env, []uint32{0x5000}); err == nil {
		t.Fatal("no call")
	}

	env.ImageRundown()

	if len(env.Process.cmode) != 0 {
		t.Error("image rundown kept a change-mode call")
	}
}
