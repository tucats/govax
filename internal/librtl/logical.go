package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
)

// The logical-name routines of the 5.0 manual's section 2.2.4
// (docs/PHASE-57, "Logical names"): DCL's DEFINE and DEASSIGN for a
// program. Each works at supervisor mode whatever the caller's mode, so
// the names outlive the image, and defaults the table to LNM$PROCESS.
// The work is $CRELNM's and $DELLNM's (corevms.Environment's
// SetSupervisorLogical and DeleteSupervisorLogical); DCL is only checked
// for, since without one these routines return LIB$_NOCLI.
//
// Names and values are taken as they are: logical names are case
// sensitive, so nothing is uppercased or trimmed (unconfirmed for VMS
// 7.x, a probe case; the VSI manual says so).

// defaultLogicalTable is the table both routines use when none is given.
const defaultLogicalTable = "LNM$PROCESS"

// logicalString reads a string argument of the logical-name routines.
// Both manual entries list SS$_ACCVIO among the statuses returned ("The
// logical name could not be read"), so, unlike most LIB$ routines, these
// return it rather than signal it (unconfirmed, a probe case). A bad
// descriptor is LIB$_INVSTRDES. status is 0 when s was read.
func logicalString(env *corevms.Environment, addr uint32) (s string, status uint32, err error) {
	s, err = readString(env, addr)

	switch {
	case err == nil:
		return s, 0, nil
	case errors.Is(err, errInvStrDes):
		return "", libInvStrDes, nil
	case errors.Is(err, errInsVirMem):
		return "", libInsVirMem, nil
	}

	if isMemoryError(err) {
		return "", ssAccVio, nil
	}

	return "", 0, err
}

// logicalTable reads the optional table-name argument at addr:
// LNM$PROCESS when it's omitted.
func logicalTable(env *corevms.Environment, addr uint32) (string, uint32, error) {
	if addr == 0 {
		return defaultLogicalTable, 0, nil
	}

	return logicalString(env, addr)
}

// libSetLogical is LIB$SET_LOGICAL (LIB-339):
//
//	LIB$SET_LOGICAL logical-name [,value-string] [,table] [,attributes] [,item-list]
//
// logical-name.rt.dx, value-string.rt.dx, table.rt.dx,
// attributes.rlu.r, item-list.rz.ra. It defines or redefines the
// supervisor-mode logical name in table (LNM$PROCESS by default), as
// DCL's DEFINE does. Its value is value-string, or, when item-list is
// given, the equivalence strings of that list ($CRELNM's, read as
// $CRELNM reads it; unconfirmed which form VMS 7.x takes), value-string
// then being ignored. attributes may hold LNM$M_CONFINE and
// LNM$M_NO_ALIAS for the name, and, without an item list, LNM$M_CONCEALED
// and LNM$M_TERMINAL for the value.
//
// It returns SS$_NORMAL; SS$_SUPERSEDE when it replaced a definition;
// LIB$_INVARG when neither value-string nor item-list is given;
// LIB$_NOCLI; LIB$_INVSTRDES; SS$_ACCVIO for an argument it can't read;
// or $CRELNM's failures (SS$_IVLOGNAM, SS$_IVLOGTAB, SS$_NOPRIV,
// SS$_BADPARAM, ...).
func libSetLogical(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	name, status, err := logicalString(env, arg(argv, 0))
	if err != nil || status != 0 {
		return status, err
	}

	valueAddr, itemList := arg(argv, 1), arg(argv, 4)
	if valueAddr == 0 && itemList == 0 {
		return libInvArg, nil
	}

	var value string

	if itemList == 0 {
		if value, status, err = logicalString(env, valueAddr); err != nil || status != 0 {
			return status, err
		}
	}

	table, status, err := logicalTable(env, arg(argv, 2))
	if err != nil || status != 0 {
		return status, err
	}

	var attributes uint32

	if a := arg(argv, 3); a != 0 {
		if attributes, err = env.Memory().LoadLongword(env.CPU(), a); err != nil {
			if isMemoryError(err) {
				return ssAccVio, nil
			}

			return 0, err
		}
	}

	return env.SetSupervisorLogical(table, name, attributes, value, itemList)
}

// libDeleteLogical is LIB$DELETE_LOGICAL (LIB-113):
//
//	LIB$DELETE_LOGICAL logical-name [,table-name]
//
// logical-name.rt.dx, table-name.rt.dx. It deletes the supervisor-mode
// logical name from table-name (LNM$PROCESS by default), as DCL's
// DEASSIGN does, which deletes a user-mode name of the same name too
// (the DCL Dictionary's DEASSIGN; so a name defined only at user mode is
// deleted rather than SS$_NOLOGNAM, unconfirmed, a probe case).
//
// It returns SS$_NORMAL; SS$_NOLOGNAM when there's no such name;
// LIB$_NOCLI; LIB$_INVSTRDES; SS$_ACCVIO for an argument it can't read;
// or $DELLNM's failures (SS$_IVLOGNAM, SS$_IVLOGTAB, SS$_NOPRIV, ...).
func libDeleteLogical(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return libNoCLI, nil
	}

	name, status, err := logicalString(env, arg(argv, 0))
	if err != nil || status != 0 {
		return status, err
	}

	table, status, err := logicalTable(env, arg(argv, 1))
	if err != nil || status != 0 {
		return status, err
	}

	return env.DeleteSupervisorLogical(table, name)
}
