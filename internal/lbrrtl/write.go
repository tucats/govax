package lbrrtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/librtl"
)

// The writing routines (subtask 11): each changes a library opened to
// create or update, through its lbr.Updater, which keeps the library's
// blocks as VMS 7.1's LBRSHR would have them (internal/lbr/update.go has
// the rules round 3 showed). As VMS 7.1's did, they return SS$_NORMAL, and
// LBR$_ILLOP for a library opened only to read.

// writable returns the control a writing routine changes: LBR$_ILLCTL,
// LBR$_LIBNOTOPN, or LBR$_ILLOP if it can't.
func writable(env *corevms.Environment, addr uint32) (*control, error) {
	c, err := openControl(env, addr)
	if err != nil {
		return nil, err
	}

	if c.upd == nil {
		return nil, errStatus(lbrIllOp)
	}

	return c, nil
}

// writeStatus is the status for an lbr.Updater's error.
func writeStatus(err error) uint32 {
	for _, s := range []struct {
		err    error
		status uint32
	}{
		{lbr.ErrDuplicateKey, lbrDupKey},
		{lbr.ErrInvalidKey, lbrInvKey},
		{lbr.ErrKeyNotFound, lbrKeyNotFnd},
		{lbr.ErrInvalidRFA, lbrInvRFA},
		{lbr.ErrPastEOF, lbrRFAPastEOF},
		{lbr.ErrNoRecord, rmsEOF},
		{lbr.ErrRecordLength, lbrRecLng},
		{lbr.ErrStillKeys, lbrStillKeys},
		{lbr.ErrIndexNumber, lbrIllIdxNum},
	} {
		if errors.Is(err, s.err) {
			return s.status
		}
	}

	return lbrIntrnlErr
}

// lbrIntrnlErr is LBR$_INTRNLERR, for an error no status names.
const lbrIntrnlErr = 0x002690CA

// loadRFA reads an RFA as the LBR$ routines take one: the block's
// longword, then the offset's word.
func loadRFA(env *corevms.Environment, addr uint32) (lbr.RFA, error) {
	mem, cpu := env.Memory(), env.CPU()

	vbn, err := mem.LoadLongword(cpu, addr)
	if err != nil {
		return lbr.RFA{}, err
	}

	offset, err := mem.LoadWord(cpu, addr+4)
	if err != nil {
		return lbr.RFA{}, err
	}

	return lbr.RFA{VBN: vbn, Offset: offset}, nil
}

// putRecord is LBR$PUT_RECORD library_index, bufdes, txtrfa: a record of
// the module being written, its header first if it's the module's first,
// and the module's RFA stored at txtrfa. A record of more than
// LBR$C_MAXRECSIZ (2,048) bytes is LBR$_RECLNG.
func putRecord(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := writable(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	rec, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		return librtl.StringFailure(env, err)
	}

	rfa, err := c.upd.PutRecord([]byte(rec))
	if err != nil {
		return writeStatus(err), nil
	}

	c.refresh()

	if arg(argv, 2) != 0 {
		if err := storeRFA(env, arg(argv, 2), rfa); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}
	}

	return ssNormal, nil
}

// putEnd is LBR$PUT_END library_index: the module's end.
func putEnd(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := writable(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	c.upd.PutEnd()
	c.refresh()

	return ssNormal, nil
}

// keyed reads a key routine's control and key: LBR$_UPDURTRAV inside
// LBR$GET_INDEX's routine, and the key as the current index enters it.
func keyed(env *corevms.Environment, argv []uint32) (*control, []byte, uint32, error) {
	c, err := writable(env, arg(argv, 0))
	if err != nil {
		r, err := failure(env, err)

		return nil, nil, r, err
	}

	if c.traversing {
		return nil, nil, lbrUpdurTrav, nil
	}

	text, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		r, err := librtl.StringFailure(env, err)

		return nil, nil, r, err
	}

	key, err := c.upd.Key(c.index-1, text)
	if err != nil {
		return nil, nil, writeStatus(err), nil
	}

	return c, key, 0, nil
}

// insertKey is LBR$INSERT_KEY library_index, key_name, txtrfa: the key
// entered in the current index, pointing at the module at txtrfa, which
// counts it. A key already there is LBR$_DUPKEY; one with an RFA of no
// module is entered all the same, with RMS$_EOF (an RFA in no data block)
// or LBR$_RFAPASTEOF (past the file's end), as VMS 7.1's was.
func insertKey(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, key, r, err := keyed(env, argv)
	if c == nil {
		return r, err
	}

	rfa, err := loadRFA(env, arg(argv, 2))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	err = c.upd.InsertKey(c.index-1, key, rfa)
	c.refresh()

	if err != nil {
		return writeStatus(err), nil
	}

	return ssNormal, nil
}

// deleteKey is LBR$DELETE_KEY library_index, key_name: the key taken out
// of the current index, and its module's count of it (LBR$_KEYNOTFND if
// it isn't there).
func deleteKey(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, key, r, err := keyed(env, argv)
	if c == nil {
		return r, err
	}

	if err := c.upd.DeleteKey(c.index-1, key); err != nil {
		return writeStatus(err), nil
	}

	c.refresh()

	return ssNormal, nil
}

// replaceKey is LBR$REPLACE_KEY library_index, key_name, oldrfa, newrfa:
// the key pointed at the module at newrfa where it pointed at oldrfa
// (LBR$_INVRFA if it didn't), or entered if the index hasn't it.
func replaceKey(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, key, r, err := keyed(env, argv)
	if c == nil {
		return r, err
	}

	old, err := loadRFA(env, arg(argv, 2))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	rfa, err := loadRFA(env, arg(argv, 3))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	err = c.upd.ReplaceKey(c.index-1, key, old, rfa)
	c.refresh()

	if err != nil {
		return writeStatus(err), nil
	}

	return ssNormal, nil
}

// deleteData is LBR$DELETE_DATA library_index, txtrfa: the module at
// txtrfa deleted, which no key may point at (LBR$_STILLKEYS).
func deleteData(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := writable(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	rfa, err := loadRFA(env, arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	if err := c.upd.DeleteData(rfa); err != nil {
		return writeStatus(err), nil
	}

	c.refresh()

	return ssNormal, nil
}

// setModule is LBR$SET_MODULE library_index, rfa [,bufdesc] [,buflen]
// [,updatedesc]: the module header at rfa copied into bufdesc's buffer
// (LBR$_HDRTRUNC, a warning, if it's longer), its length stored at buflen,
// and its user data replaced by updatedesc's text, cut or zero-filled to
// the library's size for it (LBR$_ILLOP in a library opened only to
// read). As VMS 7.1's did, it returns SS$_NORMAL.
func setModule(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	update := arg(argv, 4)
	if update != 0 && c.upd == nil {
		return lbrIllOp, nil
	}

	rfa, err := loadRFA(env, arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	var hdr []byte

	if c.upd != nil {
		hdr, err = c.upd.ModuleHeader(rfa)
		c.upd.NoteRead(rfa)
	} else {
		hdr, err = c.lib.HeaderBytes(rfa)
	}

	if err != nil {
		return writeStatus(err), nil
	}

	status := uint32(ssNormal)
	mem, cpu := env.Memory(), env.CPU()

	if arg(argv, 2) != 0 {
		at, size, err := librtl.Buffer(env, arg(argv, 2))
		if err != nil {
			return librtl.StringFailure(env, err)
		}

		out := hdr
		if uint32(len(out)) > size {
			out, status = out[:size], lbrHdrTrunc
		}

		for i, b := range out {
			if err := mem.StoreByte(cpu, at+uint32(i), b); err != nil {
				return librtl.MemoryFault(env, librtl.Writing(err))
			}
		}
	}

	if arg(argv, 3) != 0 {
		if err := mem.StoreLongword(cpu, arg(argv, 3), uint32(len(hdr))); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}
	}

	if update != 0 {
		text, err := librtl.ReadString(env, update)
		if err != nil {
			return librtl.StringFailure(env, err)
		}

		if err := c.upd.SetUserData(rfa, []byte(text)); err != nil {
			return writeStatus(err), nil
		}

		c.refresh()
	}

	return status, nil
}

// lbrHdrTrunc is LBR$_HDRTRUNC.
const lbrHdrTrunc = 0x00268800
