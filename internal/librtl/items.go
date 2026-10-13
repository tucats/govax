package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
)

// The item routines (docs/PHASE-55, the survey's "Item routines"):
// LIB$GETJPI, LIB$GETDVI, and LIB$GETSYI. Each asks $GETJPI, $GETDVI, or
// $GETSYI for one item, named by its code (passed by reference), so a
// program needn't build an item list. A numeric item's value goes to a
// longword or quadword, and its text to a string of any class; a string
// item (a name) goes to the string only.
//
// The text forms are the manual's tables (LIB-4 for devices, LIB-5 for
// processes), which corevms keeps beside DCL's (corevms.ItemText in
// StyleLIB): a PID in hexadecimal, privileges by name, a UIC by
// identifier, the scheduling state and job mode by name, a device's
// protection as "SYSTEM=RWLP,...", a truth as TRUE or FALSE, and every
// other number as an unsigned decimal integer. A string item's trailing
// blanks (JPI$_USERNAME's, SYI$_VERSION's) are left out.
//
// On VMS each routine allocates a local event flag with LIB$GET_EF for
// its synchronous call, and so may return LIB$_INSEF; govax's call the
// services' Go code, which completes at once, and use no event flag
// (the survey's "No event flag of their own").

// maxItemArgs is the most arguments each item routine's format has:
// more is LIB$_WRONUMARG (unconfirmed; the survey's "Argument counts").
// The VSI manual's LIB$GETDVI has a seventh, a multipath device's path
// name, which VMS 7.3 predates.
const maxItemArgs = 6

// wrongItemArgs reports whether argv has no item code or more arguments
// than the format allows.
func wrongItemArgs(argv []uint32) bool {
	return tooFewArgs(argv, 1) || len(argv) > maxItemArgs
}

// optionalName reads the string argument at addr, of any class, and
// whether one was given (addr not 0).
func optionalName(env *corevms.Environment, addr uint32) (name string, named bool, err error) {
	if addr == 0 {
		return "", false, nil
	}

	name, err = readString(env, addr)

	return name, true, err
}

// itemResult writes item's value as LIB$'s item routines do: for a
// numeric item, its value to the longword at value (a quadword if wide
// and the item is 8 bytes) if value isn't 0; and its text (a string
// item's without trailing blanks) to the string at text if that isn't 0,
// with the characters written to the word at length. A string item with
// no text argument is LIB$_INVARG. Returns SS$_NORMAL or LIB$_STRTRU,
// or the string statuses.
func itemResult(env *corevms.Environment, item corevms.Item, wide bool, value, text, length uint32) (uint32, error) {
	if item.Form == corevms.FormString {
		if text == 0 {
			return libInvArg, nil
		}
	} else if value != 0 {
		mem, cpu := env.Memory(), env.CPU()
		v := corevms.ItemQuadword(item.Data)

		if err := mem.StoreLongword(cpu, value, uint32(v)); err != nil {
			return memoryFault(env, writing(err))
		}

		if wide && len(item.Data) == 8 {
			if err := mem.StoreLongword(cpu, value+4, uint32(v>>32)); err != nil {
				return memoryFault(env, writing(err))
			}
		}
	}

	if text == 0 {
		return ssNormal, nil
	}

	s := env.ItemText(item.Form, item.Data, corevms.StyleLIB)
	if item.Form == corevms.FormString {
		s = strings.TrimRight(s, " ")
	}

	stored, truncated, err := writeString(env, text, s)
	if err != nil {
		return stringFailure(env, err)
	}

	if length != 0 {
		if err := env.Memory().StoreWord(env.CPU(), length, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// libGetjpi is LIB$GETJPI:
//
//	LIB$GETJPI item-code.rl.r [,process-id.mlu.r] [,process-name.rt.dx]
//	           [,resultant-value.wz.r] [,resultant-string.wt.dx]
//	           [,resultant-length.wwu.r]
//
// The process is $GETJPI's choice: process-id's PID if it's nonzero (-1
// a wildcard scan), else process-name (exact, no trailing blanks), else
// the caller; a zero process-id gets the PID used. The value is the
// item's own size: a quadword for the privilege masks and the login
// time. Returns SS$_NORMAL, LIB$_STRTRU, LIB$_INVARG, LIB$_WRONUMARG,
// the string statuses, or $GETJPI's (SS$_BADPARAM for an item it lacks,
// SS$_NONEXPR, SS$_NOPRIV, SS$_NOMOREPROC, ...).
func libGetjpi(env *corevms.Environment, argv []uint32) (uint32, error) {
	if wrongItemArgs(argv) {
		return libWroNumArg, nil
	}

	code, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	name, named, err := optionalName(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	item, status := env.ProcessItem(code, arg(argv, 1), name, named)
	if status != 0 {
		return status, nil
	}

	return itemResult(env, item, true, arg(argv, 3), arg(argv, 4), arg(argv, 5))
}

// libGetdvi is LIB$GETDVI:
//
//	LIB$GETDVI item-code.rl.r [,channel.rwu.r] [,device-name.rt.dx]
//	           [,longword-integer-value.wl.r] [,resultant-string.wt.dx]
//	           [,resultant-length.wwu.r]
//
// The device is the one channel (a word, by reference) is assigned to,
// or the one device-name names (a device or a logical name for one; a
// colon and what follows are ignored); the manual says to give one, not
// both, and with both govax uses the channel (unconfirmed). Neither is
// SS$_IVDEVNAM. The value is always a longword. DVI$M_SECONDARY (code +
// 1) asks for the secondary device's, which is the same device in govax,
// as $GETDVI has it. Returns as LIB$GETJPI does, with $GETDVI's statuses
// (SS$_NOSUCHDEV, SS$_IVDEVNAM, ...).
func libGetdvi(env *corevms.Environment, argv []uint32) (uint32, error) {
	if wrongItemArgs(argv) {
		return libWroNumArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	code, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	var channel uint16

	if addr := arg(argv, 1); addr != 0 {
		if channel, err = mem.LoadWord(cpu, addr); err != nil {
			return memoryFault(env, err)
		}
	}

	name, named, err := optionalName(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	item, status := env.DeviceItem(code, uint32(channel), name, named)
	if status != 0 {
		return status, nil
	}

	return itemResult(env, item, false, arg(argv, 3), arg(argv, 4), arg(argv, 5))
}

// libGetsyi is LIB$GETSYI:
//
//	LIB$GETSYI item-code.rl.r [,resultant-value.wz.r]
//	           [,resultant-string.wt.dx] [,resultant-length.wwu.r]
//	           [,cluster-system-id.mlu.r] [,node-name.rt.dx]
//
// The node is $GETSYI's choice: a nonzero cluster-system-id names it (-1
// a wildcard scan) and node-name isn't used; a zero one, or none, means
// node-name, and with neither the caller's node. govax's system is a
// single node outside any cluster, so it's this node or no such node.
// The value is the item's own size (a quadword for SYI$_BOOTTIME), and
// every numeric item's text an unsigned decimal integer. Returns as
// LIB$GETJPI does, with $GETSYI's statuses (SS$_NOSUCHNODE,
// SS$_NOMORENODE, ...).
func libGetsyi(env *corevms.Environment, argv []uint32) (uint32, error) {
	if wrongItemArgs(argv) {
		return libWroNumArg, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	code, err := mem.LoadLongword(cpu, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	csidadr := arg(argv, 4)

	var csid uint32

	if csidadr != 0 {
		if csid, err = mem.LoadLongword(cpu, csidadr); err != nil {
			return memoryFault(env, err)
		}
	}

	var (
		name  string
		named bool
	)

	if csid == 0 {
		if name, named, err = optionalName(env, arg(argv, 5)); err != nil {
			return stringFailure(env, err)
		}
	}

	item, status := env.SystemItem(code, csidadr, name, named)
	if status != 0 {
		return status, nil
	}

	return itemResult(env, item, true, arg(argv, 1), arg(argv, 2), arg(argv, 3))
}
