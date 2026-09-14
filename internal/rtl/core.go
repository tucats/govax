package rtl

import "github.com/tucats/govax/internal/vax"

// Port of service.c's SYS$ services that don't need internal/io or the RMS
// layer: event flags, the exit-handler/AST recording stubs, virtual address
// region expansion, and SYS$GETJPIW.

// JPI item codes sys_getjpiw recognizes, matching service.c's own
// JPI__ACCOUNT/JPI__CLINAME (their comments: "Always \"USER\""/"Always
// \"EVAX\"", i.e. hardcoded stand-ins with no real process-attribute
// storage behind them, replicated as-is).
const (
	jpiAccount = 515
	jpiCliName = 522
)

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
		mode = curMod
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

// efSlot/efBit split a 0-127 event-flag number into Environment.eventFlags'
// slot index and bit position, matching sys_clref/setef/readef's own
// "n % 0x00FF; slot = n/32; bit = n & 0x1F" arithmetic (SYS$CLREF/SETEF use
// modulo, SYS$READEF uses a bitwise AND against the same 0xFF mask — an
// equivalent operation for any n actually reachable through this argument's
// one-byte encoding, replicated per-caller below to match each service's
// own exact operator).
func efSlotBit(n uint32) (slot, bit uint32) {
	return n / 32, n & 0x1F
}

// serviceSysClref is SYS$CLREF: clears one local event flag.
func serviceSysClref(env *Environment, argv []uint32) (uint32, error) {
	slot, bit := efSlotBit(argv[0] % 0x00FF)
	env.eventFlags[slot] &^= 1 << bit
	return ssNormal, nil
}

// serviceSysSetef is SYS$SETEF: sets one local event flag.
func serviceSysSetef(env *Environment, argv []uint32) (uint32, error) {
	slot, bit := efSlotBit(argv[0] % 0x00FF)
	env.eventFlags[slot] |= 1 << bit
	return ssNormal, nil
}

// serviceSysReadef is SYS$READEF: optionally returns the whole 32-flag word
// containing flag argv[0], reporting whether that flag itself was set.
func serviceSysReadef(env *Environment, argv []uint32) (uint32, error) {
	slot, bit := efSlotBit(argv[0] & 0x00FF)

	if len(argv) == 2 {
		if err := env.mem.StoreLongword(env.cpu, argv[1], env.eventFlags[slot]); err != nil {
			return ssAccVio, nil
		}
	}

	if env.eventFlags[slot]&(1<<bit) != 0 {
		return ssWasSet, nil
	}
	return ssWasClr, nil
}

// serviceSysGetjpiw is SYS$GETJPIW: a minimal process-information lookup
// returning only the two item codes service.c itself recognizes (ACCOUNT,
// CLINAME), both hardcoded stand-ins with no real per-process attribute
// storage — matching the C source, which has none either. argv[2] (an
// optional process-name string descriptor) is read by service.c purely for
// a debug printf and never otherwise consulted; not read here since this
// port has no equivalent trace output to feed.
func serviceSysGetjpiw(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 7 {
		return ssInsfArg, nil
	}

	status := env.walkItemList(argv[3], func(e itemListEntry) uint32 {
		switch e.ItemCode {
		case jpiAccount:
			if err := storeString(env, "USER    ", e.BuffAddr, 8); err != nil {
				return ssAccVio
			}
			return env.setRetLen(e, 4)

		case jpiCliName:
			if err := storeString(env, "DCL\x00", e.BuffAddr, 4); err != nil {
				return ssAccVio
			}
			return env.setRetLen(e, 3)

		default:
			return ssBadParam
		}
	})
	if status != 0 {
		return status, nil
	}
	return ssNormal, nil
}

func registerCoreServices(t *ServiceTable) {
	t.Register("SYS$SETAST", serviceSysSetast)
	t.Register("SYS$DCLEXH", serviceSysDclexh)
	t.Register("SYS$EXPREG", serviceSysExpreg)
	t.Register("SYS$CLREF", serviceSysClref)
	t.Register("SYS$SETEF", serviceSysSetef)
	t.Register("SYS$READEF", serviceSysReadef)
	t.Register("SYS$GETJPIW", serviceSysGetjpiw)
}
