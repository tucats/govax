package rtl

import (
	"fmt"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

// Port of devices.c's sys_assign/sys_getdviw — the two SYS$ services built
// on Phase 09's internal/io.DeviceTable, deferred to this phase per
// docs/PHASE-09.md's own open questions (they need the RTL calling
// convention and a process/PID/UIC concept, neither of which existed yet).

// DVI item codes sys_getdviw recognizes, matching devices.c's own
// DVI__DEVCLASS/DEVTYPE/DEVBUFSIZE.
const (
	dviDevClass   = 4
	dviDevType    = 6
	dviDevBufSize = 8
)

// channel is one SYS$ASSIGN-created channel, the Go equivalent of devices.c's
// struct CHAN. Channel numbers count up by 8 starting at 8 (nextChannel += 8
// before use), matching find_device's own convention.
type channel struct {
	Name    string
	Number  uint16
	Class   iodev.DeviceClass
	Flags   uint32
	Mailbox string
	Device  *iodev.Device
}

// findChannel looks up a channel by number, matching sys_getdviw's own
// linear scan of the channels list.
func (env *Environment) findChannel(number uint32) (*channel, bool) {
	for _, c := range env.channels {
		if uint32(c.Number) == number {
			return c, true
		}
	}
	return nil, false
}

// serviceSysAssign is SYS$ASSIGN: given a device name, creates and returns a
// channel number bound to that device.
//
// devices.c's own sys_assign ignores str_get's "descriptor too large for a
// 64-byte buffer" outcome (str_get leaves its buffer untouched and reports
// failure only via an out-parameter the caller never checks), which would
// read name[-1] a few lines later — a plain out-of-bounds bug, not an ISA
// judgment call (this is RTL/emulator tooling, not VAX ISA behavior, same as
// Phase 09's internal/io — see its own doc.go), so it's fixed here by
// reporting SS_BADPARAM instead of replicating the out-of-bounds access.
func serviceSysAssign(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return ssInsfArg, nil
	}
	if len(argv) > 5 {
		return ssTooManyArgs, nil
	}
	if argv[0] == 0 || argv[1] == 0 {
		return ssInsfArg, nil
	}

	name, ok, err := strGet(env, argv[0], 64)
	if err != nil {
		return ssAccVio, nil
	}
	if !ok {
		return ssBadParam, nil
	}

	dp, found := env.Devices.Find(name)
	if !found {
		return ssIvDevNam, nil
	}

	env.nextChannel += 8
	c := &channel{
		Name:   name,
		Number: uint16(env.nextChannel),
		Device: dp,
		Class:  dp.DevClass,
	}

	if err := env.mem.StoreWord(env.cpu, argv[1], c.Number); err != nil {
		return ssAccVio, nil
	}

	if len(argv) > 3 && argv[3] != 0 {
		mbx, ok, err := strGet(env, argv[3], 255)
		if err != nil {
			return ssAccVio, nil
		}
		if !ok {
			return ssBadParam, nil
		}
		c.Mailbox = mbx
	}
	if len(argv) > 4 {
		c.Flags = argv[4]
	}

	env.channels = append(env.channels, c)
	dp.RefCnt++
	dp.PID = env.pid
	dp.OwnUIC = env.uic

	return ssNormal, nil
}

// serviceSysGetdviw is SYS$GETDVIW: looks up a device (by channel number or
// by name) and returns the item-list-requested attributes this phase
// implements (DEVCLASS/DEVTYPE/DEVBUFSIZE, matching devices.c's own
// sys_getdviw — every other real DVI$ item code is unimplemented there too).
func serviceSysGetdviw(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) != 8 {
		return ssInsfArg, nil
	}

	chanNum := argv[1]
	
	var dp *iodev.Device

	switch {
	case chanNum != 0:
		c, found := env.findChannel(chanNum)
		if !found {
			return ssIvChan, nil
		}
		dp = c.Device

	case argv[2] != 0:
		name, ok, err := strGet(env, argv[2], 64)
		if err != nil {
			return ssAccVio, nil
		}
		if !ok {
			return ssBadParam, nil
		}
		found := false
		dp, found = env.Devices.Find(name)
		if !found {
			return ssNoSuchDev, nil
		}

	default:
		return ssIvDevNam, nil
	}

	if env.cpu.DebugEnabled(vax.DebugDevices) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: SYS$GETDVIW looks up device %s\n", dp.Name)
	}

	status := env.walkItemList(argv[3], func(e itemListEntry) uint32 {
		switch e.ItemCode {
		case dviDevClass:
			if err := env.mem.StoreByte(env.cpu, e.BuffAddr, byte(dp.DevClass)); err != nil {
				return ssAccVio
			}
			return env.setRetLen(e, 1)

		case dviDevType:
			if err := env.mem.StoreByte(env.cpu, e.BuffAddr, byte(dp.DevType)); err != nil {
				return ssAccVio
			}
			return env.setRetLen(e, 1)

		case dviDevBufSize:
			if err := env.mem.StoreLongword(env.cpu, e.BuffAddr, dp.DevBufSize); err != nil {
				return ssAccVio
			}
			return env.setRetLen(e, 4)

		default:
			return ssBadParam
		}
	})
	if status != 0 {
		return status, nil
	}
	return ssNormal, nil
}

func registerDeviceServices(t *ServiceTable) {
	t.Register("SYS$ASSIGN", serviceSysAssign)
	t.Register("SYS$GETDVIW", serviceSysGetdviw)
}
