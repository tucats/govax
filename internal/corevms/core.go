package corevms

import "github.com/tucats/govax/internal/vmsdef"

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

	// No pages is SS$_ILLPAGCNT, as VMS 7.3 answered it
	// (testdata/mp/probe5/vax/p5args.log).
	if pageCount == 0 {
		return vmsdef.Symbols["SS$_ILLPAGCNT"], nil
	}

	curMod := uint32(env.cpu.PSL().CurMod())
	if mode < curMod {
		mode = curMod
	}

	if region == regionP1 {
		return env.expandP1Pages(pageCount, retAddr, vax.AccessMode(mode&3)), nil
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

// expandP1Pages is $EXPREG of region 1, P1: pageCount new demand-zero
// pages below P1's low end (expandP1), owned by mode and read/write for
// it and the more privileged modes, as $CRETVA makes them. P1 grows
// down, so retadr receives the highest new page's address first and the
// lowest page's last, as the manual describes ("the ending address is
// smaller than the starting address when the control region is
// expanded"); R1 is the starting address. SS$_VASFULL, with -1 in
// retadr, when P1's page table can't hold them.
func (env *Environment) expandP1Pages(pageCount, retAddr uint32, mode vax.AccessMode) uint32 {
	first, st := env.expandP1(pageCount)
	if st != 0 {
		_ = env.storeRetadr(retAddr, nil)

		return st
	}

	done := make([]uint32, 0, pageCount)
	status := uint32(ssNormal)

	for i := pageCount; i > 0; i-- {
		addr := first + (i-1)*pageSize
		if st := env.createPage(addr, mode); st != ssNormal {
			status = st

			break
		}

		done = append(done, addr)
	}

	if !env.storeRetadr(retAddr, done) {
		return ssAccVio
	}

	if len(done) > 0 {
		env.cpu.SetGPR(vax.R1, done[0]|pageMask)
	}

	return status
}

func registerCoreServices(t *ServiceTable) {
	t.Register("SYS$EXPREG", serviceSysExpreg)
}
