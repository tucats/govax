package librtl

import (
	"encoding/binary"
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$TPARSE and LIB$TABLE_PARSE (docs/PHASE-80, subtask 10), written from the
// 5.0 LIB$ manual (LIB-411 to LIB-452) and VSI's LIB$ manual:
//
//	LIB$TPARSE       argument-block ,state-table ,key-table
//	LIB$TABLE_PARSE  argument-block ,state-table ,key-table
//
// Each is a finite-state parser driven by a table the program builds with the
// $INIT_STATE, $STATE, $TRAN, and $END_STATE macros. A state is its transitions,
// one after another, the last one marked. Each transition names a kind of token
// and what to do on matching it: the parser tries a state's transitions in
// order, takes the first that matches, and goes on from the state it names.
// One may call a subexpression, the same parser on another part of the table;
// if that fails, the input it used is given back and the transition simply
// hasn't matched.
//
// A transition, in memory, with its optional fields in this order:
//
//	type      byte   a character, or (flag 0) a keyword index or a symbol type
//	flags     byte   tpaFlag... below
//	flags2    byte   present with tpaFlagSecond: bit 0, an argument is present
//	subexpr   word   the subexpression's state, self-relative
//	argument  long   for the action routine, as it is
//	action    long   the action routine, self-relative
//	mask      long   ORed into the longword at msk-adr
//	msk-adr   long   self-relative
//	target    word   self-relative, or tpaExit or tpaFail
//
// The two routines differ in how they call an action routine. LIB$TPARSE calls
// it with CALLG, the argument block being its argument list (so it reaches the
// block's fields through AP); LIB$TABLE_PARSE with the block's address as its
// one argument. The parser sets R0's low bit first, so a routine that wants
// to accept the transition need do nothing about R0.
//
// The parser keeps its place in the argument block itself, so an action routine
// sees it, and may change it: the string, the options, and the rest are read
// from memory each time. It runs on a goroutine of its own, to call back
// (corevms's callback.go), and recursion on Go's stack is a subexpression
// call, so the routine reads as the manual describes it.

// The argument block's fields (Figure LIB-6) and its options' bits.
const (
	tpaCount     = 0
	tpaOptions   = 4
	tpaStringCnt = 8
	tpaStringPtr = 12
	tpaTokenCnt  = 16
	tpaTokenPtr  = 20
	tpaChar      = 24
	tpaNumber    = 28
	tpaParam     = 32

	tpaCount0 = 8

	tpaBlanks = 1 << 0
	tpaAbbrev = 1 << 1
	tpaAbbrfm = 1 << 2
	tpaAmbig  = 1 << 16
)

// The bits of a transition's flags byte.
const (
	tpaFlagSymbol  = 1 << 0
	tpaFlagSecond  = 1 << 1
	tpaFlagLast    = 1 << 2
	tpaFlagSubexpr = 1 << 3
	tpaFlagTarget  = 1 << 4
	tpaFlagMask    = 1 << 5
	tpaFlagMskAdr  = 1 << 6
	tpaFlagAction  = 1 << 7

	tpaFlag2Argument = 1 << 0
)

// tpaExit and tpaFail are a target's final states, tpaKeywords how many
// keywords a table can have (their indexes are the type codes below that), and
// tpaSubexpr the type code of a subexpression call.
const (
	tpaExit     = -1
	tpaFail     = -2
	tpaKeywords = 220
	tpaSubexpr  = 248
)

// tpaMaxDepth bounds subexpressions' nesting. A table that recurses without
// using any input would run VMS's parser off the stack: here, too, it ends in
// an access violation, rather than Go's stack growing without end.
const tpaMaxDepth = 16384

var (
	libInvType   = vmsdef.Symbols["LIB$_INVTYPE"]
	libInvArgTPA = vmsdef.Symbols["LIB$_INVARG"]
)

// errTPAInvalidType is a table entry no type code means: LIB$_INVTYPE.
var errTPAInvalidType = errors.New("invalid LIB$TPARSE state table entry")

// tparser is one call of the parser.
type tparser struct {
	env   *corevms.Environment
	mem   *vm.Memory
	cpu   *registers.CPU
	block uint32
	keys  uint32

	// byCalls is LIB$TABLE_PARSE's way of calling an action routine.
	byCalls bool

	// err is the first memory error the input's reader met.
	err error
}

// tpaTransition is a decoded transition.
type tpaTransition struct {
	kind, flags byte
	subexpr     uint32
	argument    uint32
	hasArgument bool
	action      uint32
	mask        uint32
	mskAdr      uint32
	target      uint32
	final       int16 // tpaExit or tpaFail, when the target is one
	end         uint32
}

func (t *tpaTransition) has(flag byte) bool { return t.flags&flag != 0 }

// libTparse is LIB$TPARSE.
func libTparse(env *corevms.Environment, argv []uint32) (uint32, error) {
	return tparse(env, argv, false)
}

// libTableParse is LIB$TABLE_PARSE.
func libTableParse(env *corevms.Environment, argv []uint32) (uint32, error) {
	return tparse(env, argv, true)
}

func tparse(env *corevms.Environment, argv []uint32, byCalls bool) (uint32, error) {
	p := &tparser{
		env: env, mem: env.Memory(), cpu: env.CPU(),
		block: arg(argv, 0), keys: arg(argv, 2), byCalls: byCalls,
	}

	count, err := p.field(tpaCount)
	if err != nil {
		return memoryFault(env, err)
	}

	if int32(count) < tpaCount0 {
		return libInvArgTPA, nil
	}

	ok, status, err := p.run(arg(argv, 1), 0)

	switch {
	case errors.Is(err, errTPAInvalidType):
		return libInvType, nil
	case err != nil:
		return memoryFault(env, err)
	case ok:
		return ssNormal, nil
	}

	return status, nil
}

// field reads a longword of the argument block.
func (p *tparser) field(offset uint32) (uint32, error) {
	return p.mem.LoadLongword(p.cpu, p.block+offset)
}

// setField writes a longword of the argument block.
func (p *tparser) setField(offset, value uint32) error {
	if err := p.mem.StoreLongword(p.cpu, p.block+offset, value); err != nil {
		return writing(err)
	}

	return nil
}

// setString makes the argument block's string the n characters at ptr.
func (p *tparser) setString(n, ptr uint32) error {
	if err := p.setField(tpaStringCnt, n); err != nil {
		return err
	}

	return p.setField(tpaStringPtr, ptr)
}

// setToken makes the argument block's token the n characters at ptr.
func (p *tparser) setToken(n, ptr uint32) error {
	if err := p.setField(tpaTokenCnt, n); err != nil {
		return err
	}

	return p.setField(tpaTokenPtr, ptr)
}

// input returns the string left to parse.
func (p *tparser) input() (tpaInput, error) {
	n, err := p.field(tpaStringCnt)
	if err != nil {
		return tpaInput{}, err
	}

	ptr, err := p.field(tpaStringPtr)
	if err != nil {
		return tpaInput{}, err
	}

	return tpaInput{p: p, ptr: ptr, n: n}, nil
}

// run parses from state until a transition to TPA$_EXIT, which is ok, or a
// failure, whose status is LIB$_SYNTAXERR or an action routine's. A failure
// at main level leaves the argument block as the manual says; a subexpression's
// caller puts the string back.
func (p *tparser) run(state uint32, depth int) (ok bool, status uint32, err error) {
	if depth > tpaMaxDepth {
		return false, 0, &vm.TranslationFault{Kind: vm.AccessViolation, Addr: p.cpu.GPR(registers.SP)}
	}

	for {
		if err := p.enterState(); err != nil {
			return false, 0, err
		}

		next, final, failed, status, err := p.runState(state, depth)

		switch {
		case err != nil:
			return false, 0, err
		case failed:
			return false, status, nil
		case final == tpaExit:
			return true, 0, nil
		case final == tpaFail:
			return false, status, nil
		}

		state = next
	}
}

// enterState does what's done on entering any state: TPA$V_AMBIG is cleared,
// and unless blanks are significant the blanks and tabs in front are removed.
func (p *tparser) enterState() error {
	options, err := p.field(tpaOptions)
	if err != nil {
		return err
	}

	if options&tpaAmbig != 0 {
		if err := p.setField(tpaOptions, options&^tpaAmbig); err != nil {
			return err
		}
	}

	if options&tpaBlanks != 0 {
		return nil
	}

	in, err := p.input()
	if err != nil {
		return err
	}

	n := in.run(0, isBlankChar)
	if p.err != nil {
		return p.err
	}

	if n == 0 {
		return nil
	}

	return p.setString(in.n-n, in.ptr+n)
}

// runState tries state's transitions in order. If one is taken, it returns the
// next state, or final if that's TPA$_EXIT or TPA$_FAIL (with the status of a
// failure). If none is, failed is true, with the status.
func (p *tparser) runState(state uint32, depth int) (next uint32, final int16, failed bool, status uint32, err error) {
	var (
		alternate uint32 // a failing action routine's status
		ambiguous bool   // keywords aren't recognized in this state
	)

	for at := state; ; {
		t, err := p.decode(at)
		if err != nil {
			return 0, 0, false, 0, err
		}

		taken, err := p.try(&t, depth, &alternate, &ambiguous)
		if err != nil {
			return 0, 0, false, 0, err
		}

		if taken {
			switch {
			case t.final != 0:
				return 0, t.final, false, failureStatus(alternate), nil
			case t.has(tpaFlagTarget):
				return t.target, 0, false, 0, nil
			}

			next, err := p.stateEnd(&t)

			return next, 0, false, 0, err
		}

		if t.has(tpaFlagLast) {
			if err := p.failedToken(); err != nil {
				return 0, 0, false, 0, err
			}

			return 0, 0, true, failureStatus(alternate), nil
		}

		at = t.end
	}
}

// failureStatus is a failure's status: an action routine's, if one gave a
// status other than 0, or LIB$_SYNTAXERR.
func failureStatus(alternate uint32) uint32 {
	if alternate != 0 {
		return alternate
	}

	return libSyntaxErr
}

// failedToken is the token when no transition matched: what TPA$_SYMBOL would
// have matched, or else the first character left.
func (p *tparser) failedToken() error {
	in, err := p.input()
	if err != nil {
		return err
	}

	n := in.run(0, isSymbolChar)
	if n == 0 && in.n > 0 {
		n = 1
	}

	if p.err != nil {
		return p.err
	}

	return p.setToken(n, in.ptr)
}

// stateEnd returns the address just past the last transition of t's state: the
// state that follows it.
func (p *tparser) stateEnd(t *tpaTransition) (uint32, error) {
	last := *t

	for !last.has(tpaFlagLast) {
		next, err := p.decode(last.end)
		if err != nil {
			return 0, err
		}

		last = next
	}

	return last.end, nil
}

// decode reads the transition at address at.
func (p *tparser) decode(at uint32) (tpaTransition, error) {
	var t tpaTransition

	head, err := p.bytes(at, 2)
	if err != nil {
		return t, err
	}

	t.kind, t.flags = head[0], head[1]
	pos := at + 2

	if t.has(tpaFlagSecond) {
		second, err := p.bytes(pos, 1)
		if err != nil {
			return t, err
		}

		t.hasArgument = second[0]&tpaFlag2Argument != 0
		pos++
	}

	word := func() (int16, error) {
		b, err := p.bytes(pos, 2)
		if err != nil {
			return 0, err
		}

		pos += 2

		return int16(binary.LittleEndian.Uint16(b)), nil
	}

	long := func() (uint32, error) {
		v, err := p.mem.LoadLongword(p.cpu, pos)
		pos += 4

		return v, err
	}

	if t.has(tpaFlagSubexpr) {
		d, err := word()
		if err != nil {
			return t, err
		}

		t.subexpr = pos + uint32(int32(d))
	}

	if t.hasArgument {
		if t.argument, err = long(); err != nil {
			return t, err
		}
	}

	if t.has(tpaFlagAction) {
		d, err := long()
		if err != nil {
			return t, err
		}

		t.action = pos + d
	}

	if t.has(tpaFlagMask) {
		if t.mask, err = long(); err != nil {
			return t, err
		}
	}

	if t.has(tpaFlagMskAdr) {
		d, err := long()
		if err != nil {
			return t, err
		}

		t.mskAdr = pos + d
	}

	if t.has(tpaFlagTarget) {
		d, err := word()
		if err != nil {
			return t, err
		}

		if d == tpaExit || d == tpaFail {
			t.final = d
		} else {
			t.target = pos + uint32(int32(d))
		}
	}

	t.end = pos

	return t, nil
}

// bytes reads n bytes of the table.
func (p *tparser) bytes(addr uint32, n int) ([]byte, error) {
	out := make([]byte, n)

	for i := range out {
		b, err := p.mem.LoadByte(p.cpu, addr+uint32(i))
		if err != nil {
			return nil, err
		}

		out[i] = b
	}

	return out, nil
}

// try tries one transition, and takes it if it matches and its action routine,
// if any, agrees. A transition that isn't taken leaves the string as it was.
func (p *tparser) try(t *tpaTransition, depth int, alternate *uint32, ambiguous *bool) (bool, error) {
	in, err := p.input()
	if err != nil {
		return false, err
	}

	var m tpaMatch

	if t.has(tpaFlagSymbol) && t.kind == tpaSubexpr {
		ok, status, err := p.run(t.subexpr, depth+1)
		if err != nil {
			return false, err
		}

		if !ok {
			if status != libSyntaxErr {
				*alternate = status
			}

			return false, p.setString(in.n, in.ptr)
		}

		after, err := p.input()
		if err != nil {
			return false, err
		}

		m.n = after.ptr - in.ptr
	} else {
		var matched bool

		m, matched, err = p.match(t, in, ambiguous)
		if err != nil {
			return false, err
		}

		if !matched {
			return false, nil
		}

		if err := p.setString(in.n-m.n, in.ptr+m.n); err != nil {
			return false, err
		}
	}

	if err := p.setToken(m.n, in.ptr); err != nil {
		return false, err
	}

	if t.has(tpaFlagAction) {
		accepted, err := p.callAction(t, &m, alternate)
		if err != nil {
			return false, err
		}

		if !accepted {
			return false, p.setString(in.n, in.ptr)
		}
	}

	return true, p.store(t, &m, in.ptr)
}

// callAction calls t's action routine and reports whether it accepted the
// transition. The fields it may look at are set only now, each only by the
// kind of transition it belongs to.
func (p *tparser) callAction(t *tpaTransition, m *tpaMatch, alternate *uint32) (bool, error) {
	if m.char {
		if err := p.mem.StoreByte(p.cpu, p.block+tpaChar, m.value8); err != nil {
			return false, writing(err)
		}
	}

	if m.number {
		if err := p.setField(tpaNumber, m.value); err != nil {
			return false, err
		}
	}

	if t.hasArgument {
		if err := p.setField(tpaParam, t.argument); err != nil {
			return false, err
		}
	}

	p.cpu.SetGPR(registers.R0, 1)

	var (
		r0  uint32
		err error
	)

	if p.byCalls {
		r0, err = p.env.Callback(t.action, p.block)
	} else {
		r0, err = p.env.CallbackList(t.action, p.block)
	}

	if err != nil {
		return false, err
	}

	if r0&1 != 0 {
		return true, nil
	}

	if r0 != 0 {
		*alternate = r0
	}

	return false, nil
}

// store does t's mask and msk-adr: the mask ORed in, or without one what the
// token is, by its kind.
func (p *tparser) store(t *tpaTransition, m *tpaMatch, tokenPtr uint32) error {
	if !t.has(tpaFlagMskAdr) {
		return nil
	}

	mem, cpu := p.mem, p.cpu

	var err error

	switch {
	case t.has(tpaFlagMask):
		var old uint32

		if old, err = mem.LoadLongword(cpu, t.mskAdr); err == nil {
			err = mem.StoreLongword(cpu, t.mskAdr, old|t.mask)
		}
	case m.char:
		err = mem.StoreByte(cpu, t.mskAdr, m.value8)
	case m.longword:
		err = mem.StoreLongword(cpu, t.mskAdr, m.value)
	default:
		if err = mem.StoreLongword(cpu, t.mskAdr, m.n); err == nil {
			err = mem.StoreLongword(cpu, t.mskAdr+4, tokenPtr)
		}
	}

	if err != nil {
		return writing(err)
	}

	return nil
}
