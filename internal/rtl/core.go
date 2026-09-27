package rtl

import (
	"github.com/tucats/govax/internal/vax"
)

// Port of service.c's SYS$ services that don't need internal/io or the RMS
// layer: the exit-handler/AST recording stubs and virtual address region
// expansion. Its event-flag services moved to eventflags.go when common
// event flag clusters arrived, and SYS$GETJPIW to getjpi.go when it grew
// into a real $GETJPI (docs/PHASE-26.md).

// serviceSysSetast is SYS$SETAST: records whether ASTs are enabled. Nothing
// currently delivers an AST — see Environment.astEnabled's doc comment.
func serviceSysSetast(env *Environment, argv []uint32) (uint32, error) {
	env.astEnabled = byte(argv[0]) != 0

	return ssNormal, nil
}

// serviceSysDclexh is SYS$DCLEXH: records the exit-handler address. Nothing
// currently invokes it — see Environment.exitHandler's doc comment.
func serviceSysDclexh(env *Environment, argv []uint32) (uint32, error) {
	env.exitHandler = argv[0]

	return ssNormal, nil
}

// serviceSysExpreg is SYS$EXPREG: expands one of the P0/P1/S0 virtual
// address regions by pagcnt 512-byte pages, matching sys_expreg's own
// region-size bookkeeping (Environment.RegionSize takes the place of
// get_region_size/set_region_size's VAX-memory-cell indirection — see
// docs/PHASE-10.md's open questions).
func serviceSysExpreg(env *Environment, argv []uint32) (uint32, error) {
	pageCount, retAddr, mode, region := argv[0], argv[1], argv[2], argv[3]

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
	t.Register("SYS$SETAST", serviceSysSetast)
	t.Register("SYS$DCLEXH", serviceSysDclexh)
	t.Register("SYS$EXPREG", serviceSysExpreg)
}
