package corevms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/security"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The logical-name system services (docs/planning/PHASE-25.md subtask 8), all thin
// wrappers over the shared internal/lnm database in env.Logicals:
//
//   - $CRELNM, $DELLNM, $TRNLNM and $CRELNT, as the VMS System Services
//     Reference Manual describes them, item lists (LNM$_CHAIN included)
//     and all;
//   - the pre-V4 $CRELOG, $DELLOG and $TRNLOG, still present in VMS 7.3,
//     whose table numbers 0/1/2 select the system, group and process
//     tables.
//
// Modes (docs/planning/PHASE-26.md subtask 38): $CRELNM, $DELLNM and
// $CRELNT use an explicitly given access mode as-is only if the caller
// holds SYSNAM; otherwise they "maximize" it with the caller's mode (the
// less privileged of the two), without complaint, and an omitted one is
// the caller's. The old services take their mode by value, where 0 can't
// be told from "omitted", so they always maximize it; but a name they put
// in the system table asks for executive mode (VMS 7.1, testdata/priv94's
// cases 68 to 77).
//
// Tables (docs/planning/PHASE-94 - privileges.md, Decision 4): a
// shareable table has an owner and a protection (lnm's profile.go), and
// putting a name in it or deleting one takes write access by
// internal/security's rule, which gives a process in the system UIC
// category the system's access and SYSPRV that too, BYPASS everything;
// failing that, SYSNAM for the system table or GRPNAM for the process's
// group table (lnmTableAccess). $TRNLNM needs read access to a table it
// searches. Making a shareable table takes write access to
// LNM$SYSTEM_DIRECTORY, where its name goes, whatever its parent.
// A process-private table is the process's own, and isn't checked.

// Status codes these services return themselves (lnm's own failures come
// back as VMSErrors carrying their $SSDEF status).
var (
	ssBufferOvf  = vmsdef.Symbols["SS$_BUFFEROVF"]
	ssResultOvf  = vmsdef.Symbols["SS$_RESULTOVF"]
	ssNoTran     = vmsdef.Symbols["SS$_NOTRAN"]
	ssSupersede  = vmsdef.Symbols["SS$_SUPERSEDE"]
	ssLnmCreated = vmsdef.Symbols["SS$_LNMCREATED"]
	ssExLnmQuota = vmsdef.Symbols["SS$_EXLNMQUOTA"]
)

// $LNMDEF item codes.
var (
	lnmIndex      = uint16(vmsdef.Symbols["LNM$_INDEX"])
	lnmString     = uint16(vmsdef.Symbols["LNM$_STRING"])
	lnmAttributes = uint16(vmsdef.Symbols["LNM$_ATTRIBUTES"])
	lnmTable      = uint16(vmsdef.Symbols["LNM$_TABLE"])
	lnmLength     = uint16(vmsdef.Symbols["LNM$_LENGTH"])
	lnmACMode     = uint16(vmsdef.Symbols["LNM$_ACMODE"])
	lnmMaxIndex   = uint16(vmsdef.Symbols["LNM$_MAX_INDEX"])
	lnmChain      = uint16(vmsdef.Symbols["LNM$_CHAIN"])
)

// oldTable maps $CRELOG/$DELLOG's table number to a table name.
func (env *Environment) oldTable(tblflg uint32) (string, bool) {
	switch tblflg {
	case 0:
		return lnm.SystemTableName, true
	case 1:
		return env.Logicals.GroupTableName, true
	case 2:
		return lnm.ProcessTableName, true
	}

	return "", false
}

// lnmName reads a logical-name or table-name argument: a descriptor of
// 1-255 characters. A missing descriptor is SS$_BADPARAM, a bad length
// SS$_IVLOGNAM.
func lnmName(env *Environment, desc uint32) (string, uint32) {
	if desc == 0 {
		return "", ssBadParam
	}

	s, ok, err := strGet(env, desc, lnm.MaxNameLength)
	if err != nil {
		return "", ssAccVio
	}

	if !ok || s == "" {
		return "", ssIvLogNam
	}

	return s, 0
}

// lnmMode reads an access-mode argument passed by reference: the byte at
// ptr, or the caller's mode when ptr is 0.
func (env *Environment) lnmMode(ptr uint32) (lnm.Mode, uint32) {
	if ptr == 0 {
		return lnm.Mode(env.cpu.PSL().CurMod()), 0
	}

	b, err := env.mem.LoadByte(env.cpu, ptr)
	if err != nil {
		return 0, ssAccVio
	}

	return lnm.Mode(b & 3), 0
}

// lnmWriteMode is lnmMode for a service that creates or deletes names or
// tables: without the SYSNAM privilege, the mode is maximized with the
// caller's.
func (env *Environment) lnmWriteMode(ptr uint32) (lnm.Mode, uint32) {
	mode, st := env.lnmMode(ptr)
	if st != 0 {
		return mode, st
	}

	return env.writeMode(mode), 0
}

// writeMode is the mode a name asked for at mode is made or deleted at:
// mode itself if it isn't inner to the caller's or the caller holds
// SYSNAM, else the caller's mode.
func (env *Environment) writeMode(mode lnm.Mode) lnm.Mode {
	return env.writeModeAt(mode, env.callerMode())
}

// writeModeAt is writeMode for a caller in mode caller: DCL's commands
// ask from supervisor mode.
func (env *Environment) writeModeAt(mode lnm.Mode, caller registers.AccessMode) lnm.Mode {
	if uint32(mode) >= uint32(caller) || env.privilegeGranted(ruleLnmInnerMode, caller) {
		return mode
	}

	return lnm.Mode(caller)
}

// lnmTableAccess checks that the process may have want (accessRead or
// accessWrite) to table t: a process-private table always, a shareable
// one by its protection (the system category and SYSPRV get the
// system's access, READALL reads, BYPASS does anything), or, to write
// it, by SYSNAM for the system table or GRPNAM for the process's own
// group table (VMS 7.1, testdata/priv94's cases 4 to 65: SYSNAM doesn't
// write a group table or another shareable one, GRPNAM only the group's).
// It returns SS$_NOPRIV, or 0; a nil t passes, leaving the error to the
// operation.
func (env *Environment) lnmTableAccess(t *lnm.Table, want uint32) uint32 {
	return env.lnmTableAccessAt(t, want, env.callerMode())
}

// lnmTableAccessAt is lnmTableAccess for a caller in mode caller.
func (env *Environment) lnmTableAccessAt(t *lnm.Table, want uint32, caller registers.AccessMode) uint32 {
	if t == nil || !t.Shareable {
		return 0
	}

	// The protection's answer goes in SET DEBUG PRIVILEGES's trace too:
	// a process in the system UIC category writes the system table by
	// its protection, and no privilege is asked (VMS 7.1, testdata/priv94's
	// case 4).
	what := "read"
	if want == accessWrite {
		what = "write"
	}

	d := env.uicDecision(t.Name, t.Owner, t.Protection, want)

	var rule *PrivilegeRule

	if want == accessWrite {
		switch t.Name {
		case lnm.SystemTableName:
			rule = ruleLnmSystemTable
		case env.Logicals.GroupTableName:
			rule = ruleLnmGroupTable
		}
	}

	switch {
	case d.Granted:
		env.tracePrivileges(caller, "logical name: %s %s: by its protection %s, %s; no privilege asked",
			what, t.Name, tableProtectionText(t), tableOutcome(d))

		return 0

	case rule != nil:
		env.tracePrivileges(caller, "logical name: %s %s: by its protection %s, %s; asking the privilege",
			what, t.Name, tableProtectionText(t), tableOutcome(d))

		return env.requirePrivilegeAt(rule, caller)
	}

	env.tracePrivileges(caller, "logical name: %s %s: by its protection %s, %s; no privilege overrides it: refused, SS$_NOPRIV",
		what, t.Name, tableProtectionText(t), tableOutcome(d))

	return ssNoPriv
}

// tableOutcome is a protection check's answer for the trace: "granted,
// has RWC (system)" or "denied, has R (group,world)", in a table's
// letters.
func tableOutcome(d security.Decision) string {
	outcome := "denied"
	if d.Granted {
		outcome = "granted"
	}

	return fmt.Sprintf("%s, has %s (%s)", outcome, tableLetters(uint16(^d.Access)&0xF), d.From())
}

// tableLetters is a protection field's letters, R, W, C (create), and D,
// for the access bits clear in denied; "none" for no access.
func tableLetters(denied uint16) string {
	letters := ""

	for bit, letter := range "RWCD" {
		if denied&(1<<bit) == 0 {
			letters += string(letter)
		}
	}

	if letters == "" {
		return "none"
	}

	return letters
}

// tableProtectionText is t's owner and protection for the trace:
// "[1,4] (S:RWC,O:RWC,G:R,W:R)", with a table's letters (C for create).
func tableProtectionText(t *lnm.Table) string {
	parts := make([]string, 4)

	for c, name := range []string{"S", "O", "G", "W"} {
		parts[c] = name

		if letters := tableLetters(t.Protection >> (4 * c)); letters != "none" {
			parts[c] += ":" + letters
		}
	}

	return fmt.Sprintf("%v (%s)", security.UIC(t.Owner), strings.Join(parts, ","))
}

// firstTable is the first table tabnam designates, or nil when it
// designates none (the operation then reports why).
func (env *Environment) firstTable(tabnam string) *lnm.Table {
	tables, err := env.Logicals.ResolveTables(tabnam, lnm.User)
	if err != nil || len(tables) == 0 {
		return nil
	}

	return tables[0]
}

// lnmWritable is lnmTableAccess's write check of the first table tabnam
// designates, where a new name goes.
func (env *Environment) lnmWritable(tabnam string) uint32 {
	return env.lnmTableAccess(env.firstTable(tabnam), accessWrite)
}

// maximizedMode is the less privileged of mode and the caller's mode.
func (env *Environment) maximizedMode(mode uint32) lnm.Mode {
	return lnm.Mode(max(mode&3, uint32(env.cpu.PSL().CurMod())))
}

// lnmAttr reads a longword attribute mask passed by reference (0 when
// ptr is 0).
func (env *Environment) lnmAttr(ptr uint32) (uint32, uint32) {
	if ptr == 0 {
		return 0, 0
	}

	v, err := env.mem.LoadLongword(env.cpu, ptr)
	if err != nil {
		return 0, ssAccVio
	}

	return v, 0
}

// lnmStatus turns an lnm error into the status to return: its $SSDEF
// status, or the error itself for anything that isn't a VMS status.
func lnmStatus(err error) (uint32, error) {
	var ve vmserrors.VMSError
	if errors.As(err, &ve) {
		return ve.Status, nil
	}

	return 0, err
}

// storeBuffer copies s into the size-byte buffer at addr, returning how
// many bytes were copied and whether s was cut short.
func storeBuffer(env *Environment, addr uint32, size uint16, s string) (uint16, bool, error) {
	n := min(len(s), int(size))

	for i := 0; i < n; i++ {
		if err := env.mem.StoreByte(env.cpu, addr+uint32(i), s[i]); err != nil {
			return 0, false, err
		}
	}

	return uint16(n), n < len(s), nil
}

// storeDescriptor copies s into the buffer the descriptor at desc
// describes (no padding), returning the length copied and whether s was
// cut short.
func storeDescriptor(env *Environment, desc uint32, s string) (uint16, bool, error) {
	dlen, err := env.mem.LoadWord(env.cpu, desc)
	if err != nil {
		return 0, false, err
	}

	daddr, err := env.mem.LoadLongword(env.cpu, desc+4)
	if err != nil {
		return 0, false, err
	}

	return storeBuffer(env, daddr, dlen, s)
}

func (env *Environment) traceLogicals(format string, args ...any) {
	if env.cpu.DebugEnabled(registers.DebugLogicals) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: "+format+"\n", args...)
	}
}

// serviceSysTrnlnm is SYS$TRNLNM(attr, tabnam, lognam, acmode, itmlst):
// one level of translation of lognam in the tables tabnam designates,
// returning what the item list asks for about the equivalence string at
// the current LNM$_INDEX (0 until an LNM$_INDEX item sets it).
//
// Without acmode every name is considered and the outermost of a name's
// modes wins; with it, names and tables at less privileged modes are
// ignored. A string or table name longer than its buffer is cut short and
// the service returns SS$_BUFFEROVF, which is a success status.
func serviceSysTrnlnm(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 5 {
		return ssInsfArg, nil
	}

	attr, st := env.lnmAttr(argv[0])
	if st != 0 {
		return st, nil
	}

	tabnam, st := lnmName(env, argv[1])
	if st != 0 {
		return st, nil
	}

	lognam, st := lnmName(env, argv[2])
	if st != 0 {
		return st, nil
	}

	mode := lnm.User

	if argv[3] != 0 {
		if mode, st = env.lnmMode(argv[3]); st != 0 {
			return st, nil
		}
	}

	e, err := env.Logicals.TranslateWhere(tabnam, lognam, mode, attr, func(t *lnm.Table) bool {
		return env.lnmTableAccess(t, accessRead) == 0
	})
	if err != nil {
		env.traceLogicals("$TRNLNM(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	if len(e.Equivalences) > 0 {
		env.traceLogicals("$TRNLNM(%s,%s), value=%q", tabnam, lognam, e.Equivalences[0].Value)
	}

	index := 0
	overflow := false

	current := func() (lnm.Equivalence, bool) {
		if index < len(e.Equivalences) {
			return e.Equivalences[index], true
		}

		return lnm.Equivalence{}, false
	}

	status := env.walkItemListChain(argv[4], lnmChain, func(it itemListEntry) uint32 {
		switch it.ItemCode {
		case lnmIndex:
			v, err := env.mem.LoadLongword(env.cpu, it.BuffAddr)
			if err != nil {
				return ssAccVio
			}

			if v >= lnm.MaxEquivalences {
				return ssBadParam
			}

			index = int(v)

			return 0

		case lnmString:
			q, ok := current()
			if !ok {
				return env.setRetLen(it, 0)
			}

			n, short, err := storeBuffer(env, it.BuffAddr, it.BuffLen, q.Value)
			if err != nil {
				return ssAccVio
			}

			overflow = overflow || short

			return env.setRetLen(it, n)

		case lnmLength:
			q, _ := current()

			return env.storeItemLongword(it, uint32(len(q.Value)))

		case lnmAttributes:
			attrs := e.Attrs
			if q, ok := current(); ok {
				attrs |= q.Attrs | lnm.AttrExists
			}

			return env.storeItemLongword(it, attrs)

		case lnmMaxIndex:
			return env.storeItemLongword(it, uint32(len(e.Equivalences)-1))

		case lnmACMode:
			if err := env.mem.StoreByte(env.cpu, it.BuffAddr, byte(e.Mode)); err != nil {
				return ssAccVio
			}

			return env.setRetLen(it, 1)

		case lnmTable:
			n, short, err := storeBuffer(env, it.BuffAddr, it.BuffLen, e.Table.Name)
			if err != nil {
				return ssAccVio
			}

			overflow = overflow || short

			return env.setRetLen(it, n)
		}

		return ssBadParam
	})

	switch {
	case status != 0:
		return status, nil
	case overflow:
		return ssBufferOvf, nil
	}

	return ssNormal, nil
}

// storeItemLongword stores v at an item's buffer address and 4 as its
// return length.
func (env *Environment) storeItemLongword(it itemListEntry, v uint32) uint32 {
	if err := env.mem.StoreLongword(env.cpu, it.BuffAddr, v); err != nil {
		return ssAccVio
	}

	return env.setRetLen(it, 4)
}

// serviceSysCrelnm is SYS$CRELNM(attr, tabnam, lognam, acmode, itmlst):
// creates lognam in the first table tabnam designates. The item list
// gives the equivalence strings (LNM$_STRING), each taking the
// translation attributes of the most recent LNM$_ATTRIBUTES item, and may
// ask for the table's name back (LNM$_TABLE). attr may hold
// LNM$M_CONFINE and LNM$M_NO_ALIAS.
func serviceSysCrelnm(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 5 {
		return ssInsfArg, nil
	}

	attr, st := env.lnmAttr(argv[0])
	if st != 0 {
		return st, nil
	}

	if attr&^(lnm.AttrConfine|lnm.AttrNoAlias) != 0 {
		return ssBadParam, nil
	}

	tabnam, st := lnmName(env, argv[1])
	if st != 0 {
		return st, nil
	}

	lognam, st := lnmName(env, argv[2])
	if st != 0 {
		return st, nil
	}

	mode, st := env.lnmWriteMode(argv[3])
	if st != 0 {
		return st, nil
	}

	eqv, tableItems, st := env.crelnmItems(argv[4])
	if st != 0 {
		return st, nil
	}

	return env.createName(tabnam, lognam, mode, attr, eqv, tableItems)
}

// crelnmItems reads $CRELNM's item list at itmlst: the equivalence
// strings (LNM$_STRING), each with the translation attributes of the
// most recent LNM$_ATTRIBUTES item, and the LNM$_TABLE items that ask
// for the table's name back. A status other than 0 is the service's.
func (env *Environment) crelnmItems(itmlst uint32) ([]lnm.Equivalence, []itemListEntry, uint32) {
	var (
		eqv        []lnm.Equivalence
		tableItems []itemListEntry
		current    uint32
	)

	status := env.walkItemListChain(itmlst, lnmChain, func(it itemListEntry) uint32 {
		switch it.ItemCode {
		case lnmAttributes:
			v, err := env.mem.LoadLongword(env.cpu, it.BuffAddr)
			if err != nil {
				return ssAccVio
			}

			current = v

			return 0

		case lnmString:
			s, err := loadBytes(env, it.BuffAddr, int(it.BuffLen))
			if err != nil {
				return ssAccVio
			}

			eqv = append(eqv, lnm.Equivalence{Value: s, Attrs: current})

			return 0

		case lnmTable:
			tableItems = append(tableItems, it)

			return 0
		}

		return ssBadParam
	})

	return eqv, tableItems, status
}

// createName is $CRELNM's work once its arguments are read: lognam is
// defined at mode in the first table tabnam designates, if the process
// may put names there (lnmWritable), and tableItems get that
// table's name. It returns SS$_NORMAL, SS$_SUPERSEDE, SS$_BUFFEROVF (a
// table name cut short), or the failure.
func (env *Environment) createName(tabnam, lognam string, mode lnm.Mode, attr uint32,
	eqv []lnm.Equivalence, tableItems []itemListEntry) (uint32, error) {
	if st := env.lnmWritable(tabnam); st != 0 {
		return st, nil
	}

	superseded, err := env.Logicals.Define(tabnam, lognam, mode, attr, eqv)
	if err != nil {
		env.traceLogicals("$CRELNM(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	env.traceLogicals("$CRELNM(%s,%s) [%s] = %d string(s)", tabnam, lognam, mode, len(eqv))

	overflow, st := env.returnTableName(tabnam, tableItems)
	if st != 0 {
		return st, nil
	}

	switch {
	case superseded:
		return ssSupersede, nil
	case overflow:
		return ssBufferOvf, nil
	}

	return ssNormal, nil
}

// returnTableName answers $CRELNM's LNM$_TABLE items with the name of
// the table tabnam designates first (where the name was created).
func (env *Environment) returnTableName(tabnam string, items []itemListEntry) (bool, uint32) {
	if len(items) == 0 {
		return false, 0
	}

	tables, err := env.Logicals.ResolveTables(tabnam, lnm.User)
	if err != nil {
		return false, 0
	}

	overflow := false

	for _, it := range items {
		n, short, err := storeBuffer(env, it.BuffAddr, it.BuffLen, tables[0].Name)
		if err != nil {
			return false, ssAccVio
		}

		overflow = overflow || short

		if st := env.setRetLen(it, n); st != 0 {
			return false, st
		}
	}

	return overflow, 0
}

// loadBytes reads n bytes starting at addr as a string (unlike
// loadString, NULs included).
func loadBytes(env *Environment, addr uint32, n int) (string, error) {
	buf := make([]byte, n)

	for i := range buf {
		b, err := env.mem.LoadByte(env.cpu, addr+uint32(i))
		if err != nil {
			return "", err
		}

		buf[i] = b
	}

	return string(buf), nil
}

// serviceSysDellnm is SYS$DELLNM(tabnam, lognam, acmode): deletes lognam
// at acmode and less privileged modes from the first table tabnam
// designates that has it, or, with no lognam, every such name in the
// first table.
func serviceSysDellnm(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 3 {
		return ssInsfArg, nil
	}

	tabnam, st := lnmName(env, argv[0])
	if st != 0 {
		return st, nil
	}

	lognam := ""

	if argv[1] != 0 {
		if lognam, st = lnmName(env, argv[1]); st != 0 {
			return st, nil
		}
	}

	mode, st := env.lnmWriteMode(argv[2])
	if st != 0 {
		return st, nil
	}

	return env.deleteNames(tabnam, lognam, mode)
}

// deleteNames is $DELLNM's work once its arguments are read: lognam (or,
// when it's empty, every name) at mode and outer modes is deleted from
// the table of those tabnam designates that has it at mode, if the
// process may delete names there. A name not at mode is SS$_NOLOGNAM
// before the table's protection is asked (VMS 7.1, testdata/priv94's
// case 76: a process that can't write the system table got NOLOGNAM).
func (env *Environment) deleteNames(tabnam, lognam string, mode lnm.Mode) (uint32, error) {
	if st, err := env.deletable(tabnam, lognam, mode); st != 0 || err != nil {
		return st, err
	}

	if _, err := env.Logicals.Delete(tabnam, lognam, mode); err != nil {
		env.traceLogicals("$DELLNM(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	return ssNormal, nil
}

// deletable checks a deletion of lognam at mode from the tables tabnam
// designates: the table lnm's DeletionTable finds, and the process's
// write access to it. It returns the failure's status, or 0.
func (env *Environment) deletable(tabnam, lognam string, mode lnm.Mode) (uint32, error) {
	t, err := env.Logicals.DeletionTable(tabnam, lognam, mode)
	if err != nil {
		env.traceLogicals("$DELLNM(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	return env.lnmTableAccess(t, accessWrite), nil
}

// SetSupervisorLogical defines lognam in the first table tabnam
// designates at supervisor mode, whatever the caller's mode and
// privileges (LIB$SET_LOGICAL, docs/planning/PHASE-57: "does not require the
// caller to be executing in supervisor mode"). Its value is the item
// list at itmlst, $CRELNM's, when itmlst isn't 0; then attr may hold
// only the name's attributes (LNM$M_CONFINE, LNM$M_NO_ALIAS), since each
// value's are in the list. Otherwise it is the one string value, and
// attr may also hold that value's (LNM$M_CONCEALED, LNM$M_TERMINAL). Any
// other bit is SS$_BADPARAM. The statuses are $CRELNM's.
func (env *Environment) SetSupervisorLogical(tabnam, lognam string, attr uint32, value string, itmlst uint32) (uint32, error) {
	nameAttrs := attr & (lnm.AttrConfine | lnm.AttrNoAlias)
	valueAttrs := attr & (lnm.AttrConcealed | lnm.AttrTerminal)

	if itmlst != 0 {
		if attr != nameAttrs {
			return ssBadParam, nil
		}

		eqv, tableItems, st := env.crelnmItems(itmlst)
		if st != 0 {
			return st, nil
		}

		return env.createName(tabnam, lognam, lnm.Supervisor, nameAttrs, eqv, tableItems)
	}

	if attr != nameAttrs|valueAttrs {
		return ssBadParam, nil
	}

	eqv := []lnm.Equivalence{{Value: value, Attrs: valueAttrs}}

	return env.createName(tabnam, lognam, lnm.Supervisor, nameAttrs, eqv, nil)
}

// DeleteSupervisorLogical deletes lognam's supervisor-mode definition
// alone from the tables tabnam designates, whatever the caller's mode
// and privileges (LIB$DELETE_LOGICAL): a user-mode one of the same name
// stays, and a name defined only at user mode is SS$_NOLOGNAM (VMS V7.1,
// testdata/lib57). An empty lognam is SS$_IVLOGNAM: unlike $DELLNM's, it
// doesn't mean every name. The statuses are $DELLNM's.
func (env *Environment) DeleteSupervisorLogical(tabnam, lognam string) (uint32, error) {
	if lognam == "" {
		return ssIvLogNam, nil
	}

	if st, err := env.deletable(tabnam, lognam, lnm.Supervisor); st != 0 || err != nil {
		return st, err
	}

	if _, err := env.Logicals.DeleteExact(tabnam, lognam, lnm.Supervisor); err != nil {
		env.traceLogicals("LIB$DELETE_LOGICAL(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	return ssNormal, nil
}

// serviceSysCrelnt is SYS$CRELNT(attr, resnam, reslen, quota, promsk,
// tabnam, partab, acmode): creates a logical name table under partab,
// named tabnam or (when omitted) a unique LNM$xxxx name, returned through
// resnam/reslen. quota (a longword) is the table's quota in bytes, 0 or
// omitted for none, charged to the parent's (lnm's quota.go); promsk (a
// word) is its protection, 0 or omitted for every access to everyone.
// The caller owns it.
func serviceSysCrelnt(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 8 {
		return ssInsfArg, nil
	}

	attr, st := env.lnmAttr(argv[0])
	if st != 0 {
		return st, nil
	}

	tabnam := ""

	if argv[5] != 0 {
		if tabnam, st = lnmName(env, argv[5]); st != 0 {
			return st, nil
		}
	}

	partab, st := lnmName(env, argv[6])
	if st != 0 {
		return st, nil
	}

	mode, st := env.lnmWriteMode(argv[7])
	if st != 0 {
		return st, nil
	}

	quota, st := env.lnmAttr(argv[3])
	if st != 0 {
		return st, nil
	}

	var promsk uint16

	if argv[4] != 0 {
		w, err := env.mem.LoadWord(env.cpu, argv[4])
		if err != nil {
			return ssAccVio, nil
		}

		promsk = w
	}

	// A table under a shareable parent is shareable, and its name goes in
	// the system directory, whatever the parent: making one takes write
	// access to the directory (testdata/priv94, round 2's cases 18 to 29:
	// SYSNAM and GRPNAM didn't help, SYSPRV did).
	if parent := env.firstTable(partab); parent != nil && parent.Shareable {
		if st := env.lnmTableAccess(env.Logicals.SystemDirectory, accessWrite); st != 0 {
			return st, nil
		}
	}

	t, result, err := env.Logicals.CreateTableWith(tabnam, partab, mode, attr,
		lnm.TableProfile{Owner: env.Process.UIC, Protection: promsk, Quota: quota})
	if err != nil {
		env.traceLogicals("$CRELNT(%s,%s), %v", tabnam, partab, err)

		return lnmStatus(err)
	}

	status := uint32(ssNormal)

	switch result {
	case lnm.TableCreated:
		status = ssLnmCreated
	case lnm.TableSuperseded:
		status = ssSupersede
	}

	name := tabnam
	if t != nil {
		name = t.Name
	}

	if argv[1] != 0 {
		n, short, err := storeDescriptor(env, argv[1], name)
		if err != nil {
			return ssAccVio, nil
		}

		if argv[2] != 0 {
			if err := env.mem.StoreWord(env.cpu, argv[2], n); err != nil {
				return ssAccVio, nil
			}
		}

		if short {
			return ssResultOvf, nil
		}
	}

	return status, nil
}

// serviceSysCrelog is the pre-V4 SYS$CRELOG(tblflg, lognam, eqlnam,
// acmode): defines lognam = eqlnam in table tblflg (0 system, 1 group, 2
// process), with LNM$M_CRELOG set as $TRNLNM will report. tblflg and
// acmode are passed by value.
func serviceSysCrelog(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 4 {
		return ssInsfArg, nil
	}

	table, ok := env.oldTable(argv[0])
	if !ok {
		return ssIvLogTab, nil
	}

	lognam, st := lnmName(env, argv[1])
	if st != 0 {
		return st, nil
	}

	eqlnam, st := lnmName(env, argv[2])
	if st != 0 {
		return st, nil
	}

	if st := env.lnmWritable(table); st != 0 {
		return st, nil
	}

	eqv := []lnm.Equivalence{{Value: eqlnam}}

	superseded, err := env.Logicals.Define(table, lognam, env.oldMode(table, argv[3]), lnm.AttrCrelog, eqv)
	if err != nil {
		return lnmStatus(err)
	}

	if superseded {
		return ssSupersede, nil
	}

	return ssNormal, nil
}

// oldMode is the mode $CRELOG and $DELLOG work at in table, given the
// acmode argument: in the system table executive mode, kept with SYSNAM
// and otherwise maximized with the caller's (VMS 7.1 made a user-mode
// caller's name at user mode, and with SYSNAM, or from kernel mode, at
// executive: testdata/priv94's cases 68, 69, 75); elsewhere acmode
// maximized, since by value it can't be told from omitted.
func (env *Environment) oldMode(table string, acmode uint32) lnm.Mode {
	if table == lnm.SystemTableName {
		return env.writeMode(lnm.Executive)
	}

	return env.maximizedMode(acmode)
}

// serviceSysDellog is the pre-V4 SYS$DELLOG(tblflg, lognam, acmode):
// $DELLNM against table tblflg, with tblflg and acmode passed by value.
func serviceSysDellog(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 3 {
		return ssInsfArg, nil
	}

	table, ok := env.oldTable(argv[0])
	if !ok {
		return ssIvLogTab, nil
	}

	lognam := ""

	if argv[1] != 0 {
		var st uint32
		if lognam, st = lnmName(env, argv[1]); st != 0 {
			return st, nil
		}
	}

	mode := env.oldMode(table, argv[2])

	if st, err := env.deletable(table, lognam, mode); st != 0 || err != nil {
		return st, err
	}

	if _, err := env.Logicals.Delete(table, lognam, mode); err != nil {
		return lnmStatus(err)
	}

	return ssNormal, nil
}

// serviceSysTrnlog is the pre-V4 SYS$TRNLOG(lognam, rsllen, rslbuf, table,
// acmode, dsbmsk): one level of translation of lognam, searching the
// process, group and system tables in that order, except those dsbmsk
// disables (bit 0 system, bit 1 group, bit 2 process; by value). The
// first equivalence string goes to the rslbuf descriptor, its length to
// the word at rsllen, and the table's number (0/1/2) and the name's
// access mode to the bytes at table and acmode.
//
// A name that begins with "_" isn't translated, and neither is one that
// isn't found: either way lognam itself (less the "_") is returned, with
// the success status SS$_NOTRAN. A result longer than rslbuf is cut short
// with SS$_RESULTOVF.
func serviceSysTrnlog(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 6 {
		return ssInsfArg, nil
	}

	lognam, st := lnmName(env, argv[0])
	if st != 0 {
		return st, nil
	}

	if argv[2] == 0 {
		return ssBadParam, nil
	}

	result, status := lognam, ssNoTran

	var (
		tableNum uint32
		mode     lnm.Mode
	)

	if lognam[0] == '_' {
		result = lognam[1:]
	} else {
		for _, t := range []struct {
			num, disable uint32
			name         string
		}{
			{2, 4, lnm.ProcessTableName},
			{1, 2, env.Logicals.GroupTableName},
			{0, 1, lnm.SystemTableName},
		} {
			if argv[5]&t.disable != 0 {
				continue
			}

			e, err := env.Logicals.Translate(t.name, lognam, lnm.User, 0)
			if err != nil || len(e.Equivalences) == 0 {
				continue
			}

			result, status, tableNum, mode = e.Equivalences[0].Value, ssNormal, t.num, e.Mode

			break
		}
	}

	n, short, err := storeDescriptor(env, argv[2], result)
	if err != nil {
		return ssAccVio, nil
	}

	if argv[1] != 0 {
		if err := env.mem.StoreWord(env.cpu, argv[1], n); err != nil {
			return ssAccVio, nil
		}
	}

	if status == ssNormal {
		if argv[3] != 0 {
			if err := env.mem.StoreByte(env.cpu, argv[3], byte(tableNum)); err != nil {
				return ssAccVio, nil
			}
		}

		if argv[4] != 0 {
			if err := env.mem.StoreByte(env.cpu, argv[4], byte(mode)); err != nil {
				return ssAccVio, nil
			}
		}
	}

	if short {
		return ssResultOvf, nil
	}

	return status, nil
}

func registerLogicalServices(t *ServiceTable) {
	t.Register("SYS$TRNLNM", serviceSysTrnlnm)
	t.Register("SYS$CRELNM", serviceSysCrelnm)
	t.Register("SYS$DELLNM", serviceSysDellnm)
	t.Register("SYS$CRELNT", serviceSysCrelnt)
	t.Register("SYS$CRELOG", serviceSysCrelog)
	t.Register("SYS$DELLOG", serviceSysDellog)
	t.Register("SYS$TRNLOG", serviceSysTrnlog)
}
