package rtl

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The logical-name system services (docs/PHASE-25.md subtask 8), all thin
// wrappers over the shared internal/lnm database in env.Logicals:
//
//   - $CRELNM, $DELLNM, $TRNLNM and $CRELNT, as the VMS System Services
//     Reference Manual describes them, item lists (LNM$_CHAIN included)
//     and all;
//   - the pre-V4 $CRELOG, $DELLOG and $TRNLOG, still present in VMS 7.3,
//     whose table numbers 0/1/2 select the system, group and process
//     tables.
//
// govax has no privilege model, so a caller is treated as holding every
// privilege: $CRELNM, $DELLNM and $CRELNT use an explicitly given access
// mode as-is (the SYSNAM rule), and only an omitted one defaults to the
// caller's. The old services take their mode by value, where 0 can't be
// told from "omitted", so they "maximize" it with the caller's mode.

// Status codes these services return themselves (lnm's own failures come
// back as VMSErrors carrying their $SSDEF status).
var (
	ssBufferOvf  = vmsdef.SSConstants["SS$_BUFFEROVF"]
	ssResultOvf  = vmsdef.SSConstants["SS$_RESULTOVF"]
	ssNoTran     = vmsdef.SSConstants["SS$_NOTRAN"]
	ssSupersede  = vmsdef.SSConstants["SS$_SUPERSEDE"]
	ssLnmCreated = vmsdef.SSConstants["SS$_LNMCREATED"]
)

// $LNMDEF item codes.
var (
	lnmIndex      = uint16(vmsdef.LNMConstants["LNM$_INDEX"])
	lnmString     = uint16(vmsdef.LNMConstants["LNM$_STRING"])
	lnmAttributes = uint16(vmsdef.LNMConstants["LNM$_ATTRIBUTES"])
	lnmTable      = uint16(vmsdef.LNMConstants["LNM$_TABLE"])
	lnmLength     = uint16(vmsdef.LNMConstants["LNM$_LENGTH"])
	lnmACMode     = uint16(vmsdef.LNMConstants["LNM$_ACMODE"])
	lnmMaxIndex   = uint16(vmsdef.LNMConstants["LNM$_MAX_INDEX"])
	lnmChain      = uint16(vmsdef.LNMConstants["LNM$_CHAIN"])
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
	if env.cpu.DebugEnabled(vax.DebugLogicals) {
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

	e, err := env.Logicals.Translate(tabnam, lognam, mode, attr)
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

	mode, st := env.lnmMode(argv[3])
	if st != 0 {
		return st, nil
	}

	var (
		eqv        []lnm.Equivalence
		tableItems []itemListEntry
		current    uint32
	)

	status := env.walkItemListChain(argv[4], lnmChain, func(it itemListEntry) uint32 {
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
	if status != 0 {
		return status, nil
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

	mode, st := env.lnmMode(argv[2])
	if st != 0 {
		return st, nil
	}

	if _, err := env.Logicals.Delete(tabnam, lognam, mode); err != nil {
		env.traceLogicals("$DELLNM(%s,%s), %v", tabnam, lognam, err)

		return lnmStatus(err)
	}

	return ssNormal, nil
}

// serviceSysCrelnt is SYS$CRELNT(attr, resnam, reslen, quota, promsk,
// tabnam, partab, acmode): creates a logical name table under partab,
// named tabnam or (when omitted) a unique LNM$xxxx name, returned through
// resnam/reslen. quota and promsk are accepted and ignored: govax has no
// logical-name quotas or table protection.
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

	mode, st := env.lnmMode(argv[7])
	if st != 0 {
		return st, nil
	}

	t, result, err := env.Logicals.CreateTable(tabnam, partab, mode, attr)
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

	eqv := []lnm.Equivalence{{Value: eqlnam}}

	superseded, err := env.Logicals.Define(table, lognam, env.maximizedMode(argv[3]), lnm.AttrCrelog, eqv)
	if err != nil {
		return lnmStatus(err)
	}

	if superseded {
		return ssSupersede, nil
	}

	return ssNormal, nil
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

	if _, err := env.Logicals.Delete(table, lognam, env.maximizedMode(argv[2])); err != nil {
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
