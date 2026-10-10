package corevms

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// What DCL's lexical functions read (docs/PHASE-50 - DCL command
// procedures.md, subtask 14). The console's command interpreter answers
// F$GETJPI, F$GETSYI, F$GETDVI, F$USER, F$PRIVILEGE, F$SETPRV, F$FAO,
// F$TIME, and F$CVTIME with the same code the system services use, so a
// procedure and a program asking the same question get the same answer.
// These are the doors into that code: an item by its $JPIDEF, $SYIDEF, or
// $DVIDEF name, the privilege names, $SETPRV's rule, $FAO with DCL's
// values in place of addresses, and $ASCTIM's conversion.

// JPIItem returns the value $GETJPI gives for the item called name (its
// $JPIDEF name, "JPI$_PRCNAM") for env's process: the bytes the service
// would write to the caller's buffer (a longword little-endian, a string
// as it is). ok is false for an item $GETJPI doesn't support.
func (env *Environment) JPIItem(name string) (data string, ok bool) {
	item, ok := jpiItemsByName[name]
	if !ok {
		return "", false
	}

	return item(env).data, true
}

// SYIItem returns the value $GETSYI gives for the item called name (its
// $SYIDEF name, "SYI$_NODENAME"), as JPIItem does for $GETJPI.
func (env *Environment) SYIItem(name string) (data string, ok bool) {
	item, ok := syiItemsByName[name]
	if !ok {
		return "", false
	}

	return item(env).data, true
}

// DVIItem returns the value $GETDVI gives for the item called name (its
// $DVIDEF name, "DVI$_DEVNAM") for the device device names: a device
// name or a logical name for one, translated as $GETDVI translates its
// devnam. status is 0, SS$_NOSUCHDEV for a name that is no device, the
// translation's status for one that can't be translated, or SS$_BADPARAM
// for an item $GETDVI doesn't support.
func (env *Environment) DVIItem(device, name string) (data string, status uint32) {
	item, ok := dviItemsByName[name]
	if !ok {
		return "", ssBadParam
	}

	physical, st := env.deviceName(device)
	if st != 0 {
		return "", st
	}

	d, found := env.Devices.Find(physical)
	if !found {
		return "", ssNoSuchDev
	}

	return item(env, d).data, 0
}

// IdentifierText is a rights identifier's name, as $FAO's !%I writes it:
// a UIC identifier in brackets ("[SYSTEM]"), a general one as its name,
// and one the rights database doesn't have as a UIC ("[1,4]") or, for a
// general identifier, "%X" and eight hexadecimal digits.
func (env *Environment) IdentifierText(id uint32) string {
	r, found := env.identifierByValue(id)

	switch {
	case found && id&0x80000000 == 0:
		return "[" + r.name + "]"
	case found:
		return r.name
	case id&0x80000000 == 0:
		return formatUIC(id)
	default:
		return fmt.Sprintf("%%X%08X", id)
	}
}

// PrivilegeNames are the privileges' names ($PRVDEF's, without PRV$V_),
// indexed by their bit numbers in a privilege mask. Where two names
// share a bit, the older one is used (DETACH, not IMPERSONATE; NOACNT,
// not ACNT; unconfirmed which VMS 7.3's DCL writes).
var PrivilegeNames = func() []string {
	preferred := map[string]bool{"DETACH": true, "NOACNT": true}

	var names []string

	for name, bit := range vmsdef.Symbols {
		p, ok := strings.CutPrefix(name, "PRV$V_")
		if !ok {
			continue
		}

		for int(bit) >= len(names) {
			names = append(names, "")
		}

		if old := names[bit]; old == "" || (!preferred[old] && (preferred[p] || p < old)) {
			names[bit] = p
		}
	}

	return names
}()

// PrivilegeMask returns the mask bit of the privilege called name
// (CMKRNL, in any case), and whether there is one.
func PrivilegeMask(name string) (uint64, bool) {
	n, ok := vmsdef.Symbols["PRV$V_"+strings.ToUpper(name)]
	if !ok {
		return 0, false
	}

	return 1 << n, true
}

// AllPrivileges is every privilege's bit.
const AllPrivileges = ^uint64(0)

// SetProcessPrivileges enables (or, without enable, disables) the
// privileges in mask, both current and permanent, as $SETPRV with prmflg
// set does for a caller in supervisor mode: DCL's SET PROCESS/PRIVILEGES
// and F$SETPRV. It returns the current privileges before the change, and
// SS$_NORMAL, or SS$_NOTALLPRIV if some could not be enabled.
func (env *Environment) SetProcessPrivileges(mask uint64, enable bool) (previous uint64, status uint32) {
	previous = env.Process.CurrentPrivileges

	return previous, env.setPrivileges(mask&allPrivileges, enable, true, false)
}

// FAOValue is one of F$FAO's arguments: an integer or a string. Num is
// the value a numeric directive formats: the integer, or the string's
// value as DCL converts a string to an integer.
type FAOValue struct {
	IsString bool
	Num      uint32
	Str      string
}

// FormatFAOValues formats ctrl with params, as F$FAO does: $FAO's
// directives, with DCL's values as the parameters, so a string directive
// (!AS, !AC, and the string of !AD and !AF) takes a string argument
// itself rather than an address. An integer given to one is written in
// decimal. !%D and !%T take 0, the current time; any other value is
// SS$_BADPARAM, as there is no quadword in memory to point to. It
// returns the output and 0, or the output up to the failure and its
// status: SS$_BADPARAM for an unknown directive, SS$_ACCVIO for too few
// arguments.
func (env *Environment) FormatFAOValues(ctrl string, params []FAOValue) (string, uint32) {
	f := &faoFormatter{env: env, ctrl: ctrl, fieldWidth: -1, values: params}
	f.param = func(i int) (uint32, bool) {
		if i >= len(params) {
			return 0, false
		}

		return params[i].Num, true
	}

	f.format()

	return string(f.out), f.status
}

// FormatTime is $ASCTIM's conversion of the VMS time v: "dd-mmm-yyyy
// hh:mm:ss.cc" for an absolute time, "dddd hh:mm:ss.cc" for a delta
// time (negative), and "hh:mm:ss.cc" alone with timeOnly. ok is false
// for a time too far away to write.
func FormatTime(v uint64, timeOnly bool) (string, bool) { return formatVMSTime(v, timeOnly) }

// DVIItemNames are the names of the items $GETDVI supports ($DVIDEF's,
// "DVI$_DEVNAM"), sorted.
func DVIItemNames() []string {
	names := make([]string, 0, len(dviItemsByName))
	for name := range dviItemsByName {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
