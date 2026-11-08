package lbrrtl

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/librtl"
)

// The routines subtask 11c added to the writing routines: the update
// history (LBR$PUT_HISTORY, LBR$GET_HISTORY), LBR$SEARCH, and LBR$FLUSH,
// each as VMS 7.1's behaved in round 3 (docs/PHASE-90 - VMS help.md,
// "What the probe settled").

// More statuses (round 3's definitions).
const (
	lbrNoHistory = 0x00268403 // LBR$_NOHISTORY, informational
	lbrEmptyHist = 0x0026840B // LBR$_EMPTYHIST, informational
	lbrBadParam  = 0x002690C2 // LBR$_BADPARAM
	libBadBloSiz = 0x0015826C // LIB$_BADBLOSIZ
	rmsFAC       = 0x00018514 // RMS$_FAC
)

// The block types LBR$FLUSH takes (LBR$C_FLUSHALL, LBR$C_FLUSHDATA).
const (
	flushAll  = 0
	flushData = 1
)

// putHistory is LBR$PUT_HISTORY library_index, record_desc: the record
// added to the library's update history, the oldest taken off first if
// the history is full (lbr.Updater.PutHistory). A library that keeps no
// history is LBR$_NOHISTORY, a record longer than LBR$C_MAXRECSIZ
// LBR$_RECLNG. In a library opened only to read, VMS 7.1's returned
// LBR$_NORMAL and wrote nothing, but took the library for changed: LBR$FLUSH
// was then LBR$_WRITEERR and LBR$CLOSE RMS$_FAC (round 3's case 13).
func putHistory(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if c.lib.History == 0 {
		return lbrNoHistory, nil
	}

	rec, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		return librtl.StringFailure(env, err)
	}

	if c.upd == nil {
		c.unwritable = true

		return lbrNormal, nil
	}

	err = c.upd.PutHistory([]byte(rec))
	c.refresh()

	switch {
	case errors.Is(err, lbr.ErrNoHistory):
		return lbrNoHistory, nil
	case err != nil:
		return writeStatus(err), nil
	}

	return lbrNormal, nil
}

// getHistory is LBR$GET_HISTORY library_index, action_routine: the routine
// called with a descriptor of each update history record, the oldest
// first. A routine's failure status stops the walk and is returned. A
// library that keeps no history is LBR$_NOHISTORY, one with no records
// LBR$_EMPTYHIST. As VMS 7.1's did (round 3's W5), an empty record stops
// the walk with LIB$_BADBLOSIZ: VMS's routine asks for a block of the
// record's length for it, and LIB$GET_VM refuses a block of 0 bytes.
func getHistory(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	recs, err := c.lib.UpdateHistory()

	switch {
	case errors.Is(err, lbr.ErrNoHistory):
		return lbrNoHistory, nil
	case err != nil:
		return lbrIntrnlErr, nil
	case len(recs) == 0:
		return lbrEmptyHist, nil
	}

	for _, rec := range recs {
		if len(rec) == 0 {
			return libBadBloSiz, nil
		}

		r0, err := callWith(env, arg(argv, 1), func() ([]uint32, error) {
			desc, err := env.StackString(string(rec))

			return []uint32{desc}, err
		})
		if err != nil {
			return librtl.MemoryFault(env, err)
		}

		if r0&1 == 0 {
			return r0, nil
		}
	}

	return lbrNormal, nil
}

// search is LBR$SEARCH library_index, index_number, rfa_to_find,
// routine_name: the routine called, as LBR$GET_INDEX calls its routine,
// for each key of the index that points at the module at rfa_to_find, in
// the index's order. The routine may not change the index (LBR$_UPDURTRAV),
// and its failure status stops the search and is returned. No key there is
// LBR$_KEYNOTFND (the manual's; round 3 had none). Found, VMS 7.1 returned
// SS$_NORMAL. An RFA no module is at is looked for all the same: a key
// entered with it is found (round 3's W4).
func search(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	n, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	if n < 1 || int(n) > len(c.lib.Indexes) {
		return lbrIllIdxNum, nil
	}

	want, err := loadRFA(env, arg(argv, 2))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	c.traversing = true
	defer func() { c.traversing = false }()

	found := false

	for _, k := range c.lib.Indexes[n-1].Keys {
		if k.RFA != want {
			continue
		}

		found = true
		rfa := make([]byte, 8)
		binary.LittleEndian.PutUint32(rfa, k.RFA.VBN)
		binary.LittleEndian.PutUint16(rfa[4:], k.RFA.Offset)

		r0, err := callWith(env, arg(argv, 3), func() ([]uint32, error) {
			desc, err := env.StackString(k.Name)
			if err != nil {
				return nil, err
			}

			at, err := below(env, rfa)

			return []uint32{desc, at}, err
		})
		if err != nil {
			return librtl.MemoryFault(env, err)
		}

		if r0&1 == 0 {
			return r0, nil
		}
	}

	if !found {
		return lbrKeyNotFnd, nil
	}

	return ssNormal, nil
}

// flush is LBR$FLUSH library_index, block_type: the library's changed
// blocks written to its file now, rather than at LBR$CLOSE (which then
// writes over the same file, not a new one). block_type is
// LBR$C_FLUSHDATA or LBR$C_FLUSHALL, by value; govax writes the whole
// library for either. Any other is LBR$_BADPARAM, and a library opened
// only to read that a routine took for changed is LBR$_WRITEERR (round 3's
// case 13).
func flush(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if t := arg(argv, 1); t != flushAll && t != flushData {
		return lbrBadParam, nil
	}

	if c.unwritable {
		return lbrWriteErr, nil
	}

	if c.upd != nil && c.changed {
		if err := writeLibrary(env, c); err != nil {
			return lbrWriteErr, nil
		}

		c.written, c.changed = true, false
	}

	return lbrNormal, nil
}
