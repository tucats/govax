package rtl

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// Port of logical_names.c's sys_trnlnm, now looking names up in the
// shared internal/lnm database (docs/PHASE-25.md subtask 4). Subtask 8
// rewrites this service against the full $TRNLNM definition (LNM$_INDEX,
// a real LNM$_MAX_INDEX, the access-mode filter, and so on); until then
// it keeps the C port's behavior apart from where names are stored.

// LNM attribute/item-code constants, matching logicals.h.
const (
	lnmCaseBlind = 0x02000000

	lnmString     = 2
	lnmAttributes = 3
	lnmTable      = 4
	lnmLength     = 5
	lnmACMode     = 6
	lnmMaxIndex   = 7
)

// serviceSysTrnlnm is SYS$TRNLNM: translates a logical name and, if an item
// list was given, returns the requested attributes of it.
//
// logical_names.c's own sys_trnlnm has no default case in its item-list
// switch — an item code none of LNM__STRING/ATTRIBUTES/TABLE/LENGTH/ACMODE/
// MAX_INDEX match is silently skipped (unlike sys_getjpiw/sys_getdviw's
// explicit SS_BADPARAM), replicated as-is below since nothing marks it a
// mistake rather than deliberate laxity.
func serviceSysTrnlnm(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 5 {
		return ssInsfArg, nil
	}

	if len(argv) > 5 {
		return ssTooManyArgs, nil
	}

	var attr uint32

	if argv[0] != 0 {
		v, err := env.mem.LoadLongword(env.cpu, argv[0])
		if err != nil {
			return ssAccVio, nil
		}

		attr = v
	}

	if argv[1] == 0 {
		return ssIvLogTab, nil
	}

	tabnam, ok, err := strGet(env, argv[1], 63)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok || tabnam == "" {
		return ssIvLogTab, nil
	}

	logname, ok, err := strGet(env, argv[2], 63)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok || logname == "" {
		return ssIvLogNam, nil
	}

	debug := env.cpu.DebugEnabled(vax.DebugLogicals)

	entry, err := env.Logicals.Translate(tabnam, logname, lnm.User, attr&lnmCaseBlind)
	if err != nil {
		var ve vmserrors.VMSError
		if !errors.As(err, &ve) {
			return 0, err
		}

		if debug {
			fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: $TRNLNM(%s,%s), %v\n", tabnam, logname, err)
		}

		return ve.Status, nil
	}

	value := entry.Equivalences[0].Value
	attrs := entry.Attrs | entry.Equivalences[0].Attrs

	if debug {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: $TRNLNM(%s,%s), value=%q\n", tabnam, logname, value)
	}

	accmode := uint32(env.cpu.PSL().CurMod())

	if argv[3] != 0 {
		v, err := env.mem.LoadByte(env.cpu, argv[3])
		if err != nil {
			return ssAccVio, nil
		}

		accmode = uint32(v)
	}

	if argv[4] == 0 {
		return ssNormal, nil
	}

	status := env.walkItemList(argv[4], func(e itemListEntry) uint32 {
		switch e.ItemCode {
		case lnmACMode:
			if err := env.mem.StoreByte(env.cpu, e.BuffAddr, byte(accmode)); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, 1)

		case lnmAttributes:
			if err := env.mem.StoreLongword(env.cpu, e.BuffAddr, attrs); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, 4)

		case lnmLength:
			size := uint16(len(value))
			if err := env.mem.StoreWord(env.cpu, e.BuffAddr, size); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, 4)

		case lnmMaxIndex:
			// This port doesn't support multi-valued (list) logical
			// names, so the max index is always 1, matching
			// sys_trnlnm's own comment.
			if err := env.mem.StoreWord(env.cpu, e.BuffAddr, 1); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, 4)

		case lnmString:
			if err := storeString(env, value, e.BuffAddr, len(value)); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, uint16(len(value)))

		case lnmTable:
			if err := strPut(env, e.BuffAddr, tabnam); err != nil {
				return ssAccVio
			}

			return env.setRetLen(e, uint16(len(tabnam)))

		default:
			return 0
		}
	})

	if status != 0 {
		return status, nil
	}
	
	return ssNormal, nil
}

func registerLogicalServices(t *ServiceTable) {
	t.Register("SYS$TRNLNM", serviceSysTrnlnm)
}
