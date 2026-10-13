package corevms

import "github.com/tucats/govax/internal/vmsdef"

// What LIBRTL's item routines read (docs/PHASE-55, subtask 3):
// LIB$GETJPI, LIB$GETDVI, and LIB$GETSYI ask one item of $GETJPI,
// $GETDVI, or $GETSYI by its code, naming the process, device, or node
// as the service does. These answer the question with the services' own
// code -- the same choice of target, the same privilege check, the same
// item registries -- but without an item list or a completion: no event
// flag, IOSB, or AST, as the routine's synchronous call would leave
// none behind (the survey's "No event flag of their own"). The names are
// strings the routine has already read, of any descriptor class.

// Item is one information item's value: the bytes the service would
// write to the caller's buffer, its $xxxDEF name, and its form as
// LIB$'s routines write it as text (ItemText).
type Item struct {
	Name string
	Data string
	Form ItemForm
}

// itemName returns the name, among a service's registry's (keys of
// registry), of the item with code: the first in sort order if aliases
// share it, "" if none has it. It reads the registry when called, as
// $GETDVI's is finished by an init function.
func itemName[V any](code uint32, registry map[string]V) string {
	found := ""

	for name := range registry {
		if v, ok := vmsdef.Symbols[name]; ok && v == code && (found == "" || name < found) {
			found = name
		}
	}

	return found
}

// ProcessItem returns the $GETJPI item with code for the process
// LIB$GETJPI names: by the PID at pidadr (0 for none; a wildcard -1
// scans, and the PID used is written back, as $GETJPI writes it), else
// by name, if named, else the caller. status is 0, or $GETJPI's:
// SS$_BADPARAM for an item it doesn't support, SS$_NONEXPR, SS$_NOPRIV,
// SS$_NOMOREPROC, SS$_IVLOGNAM, SS$_ACCVIO.
func (env *Environment) ProcessItem(code, pidadr uint32, name string, named bool) (Item, uint32) {
	item, ok := jpiItems[uint16(code)]
	if !ok || code > 0xFFFF {
		return Item{}, ssBadParam
	}

	var readName func() (string, uint32)
	if named {
		readName = func() (string, uint32) { return name, 0 }
	}

	target, st := env.processTargetBy(pidadr, readName, true)
	if st != 0 {
		return Item{}, st
	}

	if st := env.mayAffect(target); st != 0 {
		return Item{}, st
	}

	full := itemName(code, jpiItemsByName)

	return Item{Name: full, Data: item(target).data, Form: JPIItemForm(full, StyleLIB)}, 0
}

// DeviceItem returns the $GETDVI item with code for the device
// LIB$GETDVI names: the one channel chanNum is assigned to, if it isn't
// 0, else the one name names, if named (a device or a logical name for
// one; a colon and what follows are ignored). status is 0 or $GETDVI's:
// SS$_BADPARAM, SS$_IVDEVNAM (neither), SS$_NOSUCHDEV, SS$_NOPRIV, ....
// DVI$M_SECONDARY (code + 1) is the same item, as $GETDVI takes it.
func (env *Environment) DeviceItem(code, chanNum uint32, name string, named bool) (Item, uint32) {
	item, ok := dviItem(uint16(code))
	if !ok || code > 0xFFFF {
		return Item{}, ssBadParam
	}

	d, st := env.dviTargetName(chanNum&0xFFFF, name, named)
	if st != 0 {
		return Item{}, st
	}

	full := itemName(code&^uint32(dviItemFlags), dviItemsByName)

	return Item{Name: full, Data: item(env, d).data, Form: DVIItemForm(full, StyleLIB)}, 0
}

// SystemItem returns the $GETSYI item with code for the node LIB$GETSYI
// names: by the CSID at csidadr (0 for none; -1 a wildcard scan, as
// $GETSYI takes it), and by name, if named. govax is a single node, so
// it's this one or none. status is 0 or $GETSYI's: SS$_BADPARAM,
// SS$_NOSUCHNODE, SS$_NOMORENODE, SS$_IVLOGNAM, SS$_ACCVIO.
func (env *Environment) SystemItem(code, csidadr uint32, name string, named bool) (Item, uint32) {
	item, ok := syiItems[uint16(code)]
	if !ok || code > 0xFFFF {
		return Item{}, ssBadParam
	}

	if st := env.nodeTargetName(csidadr, name, named); st != 0 {
		return Item{}, st
	}

	full := itemName(code, syiItemsByName)

	return Item{Name: full, Data: item(env).data, Form: SYIItemForm(full, StyleLIB)}, 0
}
