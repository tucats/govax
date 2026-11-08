package lbrrtl

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/registers"
)

// lookupKey is LBR$LOOKUP_KEY library_index, key_name, txtrfa: the module
// the key names in the current index, by LBR$LOOKUP_KEY's case rule
// (lbr.Index.Find: "alpha" finds ALPHA in a help library; "ALP" finds
// nothing), its module header's RFA stored at txtrfa (6 bytes: the block
// and the offset), and its records made the ones LBR$GET_RECORD reads. A
// key that isn't there is LBR$_KEYNOTFND, and LBR$GET_RECORD then says
// LBR$_LKPNOTDON. Found, VMS 7.1 returned SS$_NORMAL. Only ASCII keys are
// govax's: a library with binary keys finds none.
func lookupKey(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	c.looked = false

	key, err := librtl.ReadString(env, arg(argv, 1))
	if err != nil {
		return librtl.StringFailure(env, err)
	}

	rfa, ok := c.lib.Indexes[c.index-1].Find(key)
	if !ok {
		return lbrKeyNotFnd, nil
	}

	if err := storeRFA(env, arg(argv, 2), rfa); err != nil {
		return librtl.MemoryFault(env, librtl.Writing(err))
	}

	return readModule(c, rfa)
}

// find is LBR$FIND library_index, txtrfa: the module whose header is at
// the RFA, as LBR$LOOKUP_KEY would find it (an RFA of no module's header
// is LBR$_INVRFA).
func find(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	c.looked = false
	mem, cpu := env.Memory(), env.CPU()

	vbn, err := mem.LoadLongword(cpu, arg(argv, 1))
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	offset, err := mem.LoadWord(cpu, arg(argv, 1)+4)
	if err != nil {
		return librtl.MemoryFault(env, err)
	}

	return readModule(c, lbr.RFA{VBN: vbn, Offset: offset})
}

// readModule makes the module at rfa c's records to read. A module that
// can't be read is LBR$_INVRFA, unless one was read before: then, as VMS
// 7.1's LBR$LOOKUP_KEY did for keys with bad RFAs (round 3's W4), the
// lookup succeeds and the records are that module's again, from its
// start.
func readModule(c *control, rfa lbr.RFA) (uint32, error) {
	m, err := c.lib.Module(rfa)
	if err != nil {
		if c.lastModule == nil {
			return lbrInvRFA, nil
		}

		c.records, c.next, c.looked = c.lastModule, 0, true

		return ssNormal, nil
	}

	if c.upd != nil {
		c.upd.NoteRead(rfa)
	}

	c.records, c.next, c.looked, c.lastModule = m.Records, 0, true, m.Records

	return ssNormal, nil
}

// storeRFA stores an RFA as the LBR$ routines give one: the block's
// longword, then the offset's word.
func storeRFA(env *corevms.Environment, addr uint32, rfa lbr.RFA) error {
	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreLongword(cpu, addr, rfa.VBN); err != nil {
		return err
	}

	return mem.StoreWord(cpu, addr+4, rfa.Offset)
}

// getRecord is LBR$GET_RECORD library_index [,inbufdes] [,outbufdes]: the
// module's next record, after which RMS$_EOF. In move mode (the default)
// the record is copied into inbufdes's buffer, LBR$_RECTRUNC (a warning)
// if it's longer than that; in locate mode it stays in a buffer of the
// routine's own. outbufdes, if given, is set to the record's length and
// where it is. A program that passes no buffer in move mode has its
// missing argument taken for one, and gets an access violation (VMS 7.1's
// came from whatever followed the argument list).
func getRecord(env *corevms.Environment, argv []uint32) (uint32, error) {
	c, err := openControl(env, arg(argv, 0))
	if err != nil {
		return failure(env, err)
	}

	if !c.looked {
		return lbrLkpNotDon, nil
	}

	if c.next >= len(c.records) {
		return rmsEOF, nil
	}

	rec := c.records[c.next]
	c.next++

	status := uint32(ssNormal)

	var at uint32

	if c.locate {
		if at, err = c.locateBuffer(env); err != nil {
			return librtl.MemoryFault(env, err)
		}
	} else {
		text, size, err := librtl.Buffer(env, arg(argv, 1))
		if err != nil {
			return librtl.StringFailure(env, err)
		}

		if uint32(len(rec)) > size {
			rec, status = rec[:size], lbrRecTrunc
		}

		at = text
	}

	mem, cpu := env.Memory(), env.CPU()

	for i, b := range rec {
		if err := mem.StoreByte(cpu, at+uint32(i), b); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}
	}

	if out := arg(argv, 2); out != 0 {
		if err := mem.StoreWord(cpu, out, uint16(len(rec))); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}

		if err := mem.StoreLongword(cpu, out+4, at); err != nil {
			return librtl.MemoryFault(env, librtl.Writing(err))
		}
	}

	return status, nil
}

// locateBufferSize is the size of locate mode's buffer: the longest record
// a library holds, expanded (LBR_DCX$C_MAXRECSIZ).
const locateBufferSize = 4096

// locateBuffer is the buffer locate mode returns records in, a block of
// the image's heap made on first need and freed by LBR$CLOSE.
func (c *control) locateBuffer(env *corevms.Environment) (uint32, error) {
	if c.buffer == 0 {
		addr, err := env.AllocateVM(locateBufferSize)
		if err != nil {
			return 0, err
		}

		c.buffer = addr
	}

	return c.buffer, nil
}

// getIndex is LBR$GET_INDEX library_index, index_number, routine_name
// [,match_desc]: routine_name called for each key of the index, or each
// match_desc matches (lbr.Index.Select: the pattern taken as it is, so
// "alpha*" matches nothing in a help library), with a descriptor of the
// key and the address of its module's RFA. A routine's failure status
// stops the walk and is returned; no key matching is LBR$_NOMTCHFOU, and an
// empty index LBR$_NULIDX (unconfirmed: the probe's had keys).
func getIndex(env *corevms.Environment, argv []uint32) (uint32, error) {
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

	x := c.lib.Indexes[n-1]
	keys := x.Keys

	if len(keys) == 0 {
		return lbrNulIdx, nil
	}

	if arg(argv, 3) != 0 {
		pattern, err := librtl.ReadString(env, arg(argv, 3))
		if err != nil {
			return librtl.StringFailure(env, err)
		}

		if keys = x.Select(pattern); len(keys) == 0 {
			return lbrNoMtchFou, nil
		}
	}

	c.traversing = true
	defer func() { c.traversing = false }()

	for _, k := range keys {
		rfa := make([]byte, 8)
		binary.LittleEndian.PutUint32(rfa, k.RFA.VBN)
		binary.LittleEndian.PutUint16(rfa[4:], k.RFA.Offset)

		r0, err := callWith(env, arg(argv, 2), func() ([]uint32, error) {
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

	return ssNormal, nil
}

// below puts data below the stack pointer, on a longword boundary, and
// returns its address.
func below(env *corevms.Environment, data []byte) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	at := (cpu.GPR(registers.SP) - uint32(len(data))) &^ 3

	for i, b := range data {
		if err := mem.StoreByte(cpu, at+uint32(i), b); err != nil {
			return 0, err
		}
	}

	cpu.SetGPR(registers.SP, at)

	return at, nil
}

// longword is a longword's bytes, for below.
func longword(v uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, v)
}

// callWith calls the procedure at routine with the arguments args puts
// below the stack pointer, and returns its R0. The stack pointer is put
// back after the call, so a routine that calls back many times (a help
// text's every line) doesn't use the stack up. It isn't put back by a
// deferred call: a callback that never returns (an unwind past the
// routine) abandons the routine, and its deferred calls mustn't change the
// CPU's registers (corevms/callback.go).
func callWith(env *corevms.Environment, routine uint32, args func() ([]uint32, error)) (uint32, error) {
	cpu := env.CPU()
	sp := cpu.GPR(registers.SP)

	list, err := args()
	if err != nil {
		cpu.SetGPR(registers.SP, sp)

		return 0, err
	}

	r0, err := env.Callback(routine, list...)
	cpu.SetGPR(registers.SP, sp)

	return r0, err
}
