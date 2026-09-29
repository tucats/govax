package rtl

import (
	"sort"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
)

// Rights identifiers (docs/PHASE-26.md subtask 40): $ASCTOID, $IDTOASC,
// and $FINISH_RDB.
//
// # Identifiers
//
// VMS protects objects (files, devices, queues, ...) not only by the
// owner's UIC but by *identifiers*: named 32-bit values a process may
// hold, which an object's access control list can grant access to. The
// system keeps them in the *rights database* (SYS$SYSTEM:RIGHTSLIST.DAT),
// each with a name, a value, and attribute bits. There are two kinds:
//
//   - UIC identifiers, one per user: the value is the user's UIC (group
//     in the high word, member in the low; bit 31 clear) and the name is
//     the username. SYSTEM's is [1,4], %X00010004.
//   - General identifiers, bit 31 set, for anything else. VMS itself
//     creates the *environmental* ones, which say how a process logged
//     in: BATCH, NETWORK, INTERACTIVE, LOCAL, DIALUP, REMOTE.
//
// $ASCTOID translates a name to its value, $IDTOASC a value to its name
// (or, given -1, lists every identifier in name order, one per call, with
// a context longword to keep its place), and $FINISH_RDB ends such a
// listing early. $FAO's !%I writes an identifier by name.
//
// # How govax does it
//
// govax has no rights database file. The database is built in memory
// from what the emulated process knows: its own UIC identifier (its
// username and UIC) and the six environmental identifiers. None has
// attribute bits (KGB$M_RESOURCE, KGB$M_DYNAMIC) set.

// rightsIdentifier is one rights database record.
type rightsIdentifier struct {
	name   string
	value  uint32
	attrib uint32
}

// environmentalIdentifiers are the general identifiers VMS creates in a
// new rights database, with the values AUTHORIZE gives them.
var environmentalIdentifiers = []rightsIdentifier{
	{name: "BATCH", value: 0x80000001},
	{name: "NETWORK", value: 0x80000002},
	{name: "INTERACTIVE", value: 0x80000003},
	{name: "LOCAL", value: 0x80000004},
	{name: "DIALUP", value: 0x80000005},
	{name: "REMOTE", value: 0x80000006},
}

// maxIdentifierName is the longest identifier name.
const maxIdentifierName = 31

// Status values the identifier services return.
var (
	ssNoSuchID = vmsdef.SSConstants["SS$_NOSUCHID"]
	ssIvIdent  = vmsdef.SSConstants["SS$_IVIDENT"]
)

// rightsDatabase returns the rights database, sorted by name (the order
// $IDTOASC lists it in).
func (env *Environment) rightsDatabase() []rightsIdentifier {
	db := append([]rightsIdentifier{{name: env.Process.Username, value: env.Process.UIC}}, environmentalIdentifiers...)

	sort.Slice(db, func(i, j int) bool { return db[i].name < db[j].name })

	return db
}

// identifierByValue returns the identifier with value id.
func (env *Environment) identifierByValue(id uint32) (rightsIdentifier, bool) {
	for _, r := range env.rightsDatabase() {
		if r.value == id {
			return r, true
		}
	}

	return rightsIdentifier{}, false
}

// validIdentifierName reports whether name has an identifier name's form:
// 1-31 letters, digits, "$" and "_", not all digits.
func validIdentifierName(name string) bool {
	if name == "" || len(name) > maxIdentifierName {
		return false
	}

	digits := true

	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z', c == '$', c == '_':
			digits = false
		case c >= '0' && c <= '9':
		default:
			return false
		}
	}

	return !digits
}

// serviceSysAsctoid is SYS$ASCTOID:
//
//	SYS$ASCTOID name ,[id] ,[attrib]
//
// It looks up the identifier the descriptor name names (case doesn't
// matter) and stores its value at id and its attributes at attrib.
// It returns SS$_NORMAL; SS$_IVIDENT for a name that can't be an
// identifier; SS$_NOSUCHID for one the database doesn't have; or
// SS$_ACCVIO.
func serviceSysAsctoid(env *Environment, argv []uint32) (uint32, error) {
	desc := optArg(argv, 0)
	if desc == 0 {
		return ssAccVio, nil
	}

	name, ok, err := strGet(env, desc, 255)

	switch {
	case err != nil:
		return ssAccVio, nil
	case !ok:
		return ssIvIdent, nil
	}

	name = strings.ToUpper(strings.TrimSpace(name))
	if !validIdentifierName(name) {
		return ssIvIdent, nil
	}

	for _, r := range env.rightsDatabase() {
		if r.name != name {
			continue
		}

		if st := env.storeLongIfGiven(optArg(argv, 1), r.value); st != 0 {
			return st, nil
		}

		if st := env.storeLongIfGiven(optArg(argv, 2), r.attrib); st != 0 {
			return st, nil
		}

		return ssNormal, nil
	}

	return ssNoSuchID, nil
}

// storeLongIfGiven stores v at addr unless addr is 0, returning
// SS$_ACCVIO if it can't.
func (env *Environment) storeLongIfGiven(addr, v uint32) uint32 {
	if addr != 0 && env.mem.StoreLongword(env.cpu, addr, v) != nil {
		return ssAccVio
	}

	return 0
}

// serviceSysIdtoasc is SYS$IDTOASC:
//
//	SYS$IDTOASC id ,[namlen] ,[nambuf] ,[resid] ,[attrib] ,[contxt]
//
// It translates identifier id to its name, stored in the buffer the
// descriptor nambuf describes, with its length at namlen (a word), its
// value at resid, and its attributes at attrib.
//
// With id -1 it lists the database: each call returns the next
// identifier in name order, contxt (a longword the caller starts at 0 and
// leaves alone) keeping the place. After the last one, it returns
// SS$_NOSUCHID and clears contxt.
//
// It returns SS$_NORMAL; SS$_BUFFEROVF if the name didn't fit (it's
// truncated); SS$_NOSUCHID for an id the database doesn't have, or the
// end of a listing; or SS$_ACCVIO.
func serviceSysIdtoasc(env *Environment, argv []uint32) (uint32, error) {
	id, namlen, nambuf := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)
	resid, attrib, contxt := optArg(argv, 3), optArg(argv, 4), optArg(argv, 5)

	var (
		r     rightsIdentifier
		found bool
	)

	if id == 0xFFFFFFFF {
		if contxt == 0 {
			return ssAccVio, nil
		}

		n, err := env.mem.LoadLongword(env.cpu, contxt)
		if err != nil {
			return ssAccVio, nil
		}

		db := env.rightsDatabase()
		if int(n) >= len(db) {
			// Past the last one: the listing is over, so the context goes.
			_ = env.mem.StoreLongword(env.cpu, contxt, 0)

			return ssNoSuchID, nil
		}

		if env.mem.StoreLongword(env.cpu, contxt, n+1) != nil {
			return ssAccVio, nil
		}

		r, found = db[n], true
	} else {
		r, found = env.identifierByValue(id)
	}

	if !found {
		return ssNoSuchID, nil
	}

	status := uint32(ssNormal)

	if nambuf != 0 {
		n, truncated, err := storeDescriptor(env, nambuf, r.name)
		if err != nil {
			return ssAccVio, nil
		}

		if truncated {
			status = ssBufferOvf
		}

		if namlen != 0 && env.mem.StoreWord(env.cpu, namlen, n) != nil {
			return ssAccVio, nil
		}
	}

	if st := env.storeLongIfGiven(resid, r.value); st != 0 {
		return st, nil
	}

	if st := env.storeLongIfGiven(attrib, r.attrib); st != 0 {
		return st, nil
	}

	return status, nil
}

// serviceSysFinishRdb is SYS$FINISH_RDB:
//
//	SYS$FINISH_RDB contxt
//
// It ends a listing ($IDTOASC with id -1) before its end, clearing the
// context longword. SS$_ACCVIO if it can't be written.
func serviceSysFinishRdb(env *Environment, argv []uint32) (uint32, error) {
	if contxt := optArg(argv, 0); contxt != 0 && env.mem.StoreLongword(env.cpu, contxt, 0) != nil {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

func registerRightsServices(t *ServiceTable) {
	t.Register("SYS$ASCTOID", serviceSysAsctoid)
	t.Register("SYS$IDTOASC", serviceSysIdtoasc)
	t.Register("SYS$FINISH_RDB", serviceSysFinishRdb)
}
