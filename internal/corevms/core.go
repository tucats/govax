package corevms

import (
	"github.com/tucats/govax/internal/vax"
)

// serviceSysExpreg is SYS$EXPREG: expands one of the P0/P1/S0 virtual
// address regions by pagcnt 512-byte pages, matching sys_expreg's own
// region-size bookkeeping (Environment.RegionSize takes the place of
// get_region_size/set_region_size's VAX-memory-cell indirection — see
// docs/PHASE-10.md's open questions).
func serviceSysExpreg(env *Environment, argv []uint32) (uint32, error) {
	// An argument the caller didn't pass is 0 (testdata/mp/probe5 calls
	// it with fewer than four: it mustn't fail in Go).
	pageCount, retAddr, mode, region := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)

	if region > 2 {
		return ssInvArg, nil
	}

	curMod := uint32(env.cpu.PSL().CurMod())
	if mode < curMod {
		mode = curMod //nolint:ineffassign // leftover from C port to Go
	}

	size := pageCount * 512
	start := env.RegionSize[region]
	end := start + size - 1
	env.RegionSize[region] = end + 1

	if retAddr != 0 {
		if err := env.mem.StoreLongword(env.cpu, retAddr, start); err != nil {
			return ssAccVio, nil
		}

		if err := env.mem.StoreLongword(env.cpu, retAddr+4, end); err != nil {
			return ssAccVio, nil
		}
	}

	env.cpu.SetGPR(vax.R1, start) // "expected side effect", matching sys_expreg's own comment

	return ssNormal, nil
}

func registerCoreServices(t *ServiceTable) {
	t.Register("SYS$EXPREG", serviceSysExpreg)
}
