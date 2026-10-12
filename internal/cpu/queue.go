package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// The queue instructions: INSQUE/REMQUE (absolute queues -- link fields
// hold absolute addresses) and INSQHI/INSQTI/REMQHI/REMQTI (self-relative
// queues -- link fields hold byte displacements from the entry or header
// they're in). The self-relative ones are the "interlocked" queue
// instructions; their work is done by InsertInterlocked and
// RemoveInterlocked, which LIBRTL's LIB$INSQHI and its kind share
// (internal/librtl/queue.go).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x0E, emulInsque)
	reg(0x0F, emulRemque)
	reg(0x5C, emulInsqhi)
	reg(0x5D, emulInsqti)
	reg(0x5E, emulRemqhi)
	reg(0x5F, emulRemqti)
}

// emulInsque is INSQUE: entry is inserted into the (absolute-queue) list
// immediately after pred. N/Z/C reflect entry's new forward/backward links
// (pred's old forward link, and pred itself, respectively) per the manual;
// V <- 0. This project doesn't emulate multiple processors, so there is no
// interlock to fail -- matches emul_insque.c, which has no such logic
// either.
func emulInsque(e *Engine, d *Decoded) error {
	entry := d.Operands[0].Addr
	pred := d.Operands[1].Addr

	predForward, err := e.mem.LoadLongword(e.cpu, pred)
	if err != nil {
		return err
	}

	if err := e.mem.StoreLongword(e.cpu, entry, predForward); err != nil {
		return err
	}

	if err := e.mem.StoreLongword(e.cpu, entry+4, pred); err != nil {
		return err
	}

	if err := e.mem.StoreLongword(e.cpu, predForward+4, entry); err != nil {
		return err
	}

	if err := e.mem.StoreLongword(e.cpu, pred, entry); err != nil {
		return err
	}

	result, _, c := subResult(uint64(predForward), uint64(pred), 4)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 4))
	psl.SetZ(isZero(result, 4))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	return nil
}

// emulRemque is REMQUE: entry is removed from the (absolute-queue) list it
// sits in, using its own forward/backward links to find its neighbors. N/Z/C
// come from comparing entry's successor and predecessor (LSS/EQL/LSSU); V <-
// 1 if entry's successor equals entry itself (nothing was actually linked in
// -- "no entry to remove"), matching emul_remque.c.
func emulRemque(e *Engine, d *Decoded) error {
	entry := d.Operands[0].Addr

	pred, err := e.mem.LoadLongword(e.cpu, entry+4)
	if err != nil {
		return err
	}

	succ, err := e.mem.LoadLongword(e.cpu, entry)
	if err != nil {
		return err
	}

	result, _, c := subResult(uint64(succ), uint64(pred), 4)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, 4))
	psl.SetZ(isZero(result, 4))
	psl.SetV(succ == entry)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	if err := e.mem.StoreLongword(e.cpu, pred, succ); err != nil {
		return err
	}

	if err := e.mem.StoreLongword(e.cpu, succ+4, pred); err != nil {
		return err
	}

	return d.Operands[1].Store(e.cpu, e.mem, uint64(entry))
}

// A self-relative queue (the VAX Architecture Reference Manual, the
// INSQHI to REMQTI entries) is a quadword header followed, around a ring,
// by quadword-aligned entries. Each holds two longwords, a forward link
// and a backward link, and each link is a displacement from the start of
// the header or entry it's in (never from the link's own address). An
// empty queue's header is two zeros.
//
// Bit 0 of the header's forward link is the *secondary interlock*: while
// it's set, another processor (or code that set it by hand) owns the
// queue, and an interlocked instruction changes nothing and sets C so
// its caller can try again. govax has one processor, so nothing but a
// program sets it, but the rule is modeled anyway.
//
// Before changing anything an instruction checks its addresses: the
// header, the entry, and the entries it will link must be quadword
// aligned, the header's forward link must have bits 2:1 clear, and an
// entry can't be its own header. Any failure is a reserved operand fault,
// with the queue untouched. Every location the instruction will write is
// read first, so a memory fault also leaves the queue as it was (the
// manual's "validates that the entire operation can be completed").

// queueInterlock is the secondary interlock, bit 0 of a self-relative
// queue header's forward link.
const queueInterlock = 1

// QueueResult is what an interlocked queue instruction reports, as its
// condition codes: Z, V, and C (N is always 0).
type QueueResult struct {
	// Z: an insertion made the queue's first (only) entry; a removal left
	// the queue empty. 0 when the interlock failed.
	Z bool

	// V: a removal found nothing to remove (the queue was empty, or the
	// interlock failed). Always 0 for an insertion.
	V bool

	// C: the secondary interlock was set, so nothing changed.
	C bool

	// Removed is the address a removal stores in its addr operand: the
	// entry removed, or the header when the queue was empty. Not set
	// when the interlock failed (addr isn't written).
	Removed uint32
}

// setQueueCodes sets the PSL's condition codes from r.
func setQueueCodes(e *Engine, r QueueResult) {
	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(r.Z)
	psl.SetV(r.V)
	psl.SetC(r.C)
	e.cpu.SetPSL(psl)
}

// quadAligned reports whether addr is a multiple of 8.
func quadAligned(addr uint32) bool { return addr&7 == 0 }

// reservedOperandFault is the fault every queue check raises.
func reservedOperandFault() error { return &Fault{Code: ExcReservedOp} }

// queueLink is one longword of a queue: its address, and the value to
// store there.
type queueLink struct{ addr, value uint32 }

// storeLinks reads each link's longword (so a memory fault comes before
// any change), then stores them all.
func storeLinks(cpu *registers.CPU, mem *vm.Memory, links ...queueLink) error {
	for _, l := range links {
		if _, err := mem.LoadLongword(cpu, l.addr); err != nil {
			return err
		}
	}

	for _, l := range links {
		if err := mem.StoreLongword(cpu, l.addr, l.value); err != nil {
			return err
		}
	}

	return nil
}

// headerLink reads a self-relative queue header's forward link and
// checks it: busy is true when the secondary interlock is set, and a
// link whose bits 2:1 aren't clear is a reserved operand fault.
func headerLink(cpu *registers.CPU, mem *vm.Memory, header uint32) (link uint32, busy bool, err error) {
	if !quadAligned(header) {
		return 0, false, reservedOperandFault()
	}

	link, err = mem.LoadLongword(cpu, header)
	if err != nil {
		return 0, false, err
	}

	if link&queueInterlock != 0 {
		return link, true, nil
	}

	if link&6 != 0 {
		return 0, false, reservedOperandFault()
	}

	return link, false, nil
}

// InsertInterlocked is INSQHI (tail false: entry goes first, after the
// header) and INSQTI (tail true: entry goes last, before the header) on
// the self-relative queue at header. Z reports that entry is now the
// queue's only entry; C that the secondary interlock was set and nothing
// changed. A reserved operand fault or memory error is returned as the
// error, the queue unchanged.
func InsertInterlocked(cpu *registers.CPU, mem *vm.Memory, entry, header uint32, tail bool) (QueueResult, error) {
	if !quadAligned(entry) || entry == header {
		return QueueResult{}, reservedOperandFault()
	}

	forward, busy, err := headerLink(cpu, mem, header)
	if err != nil || busy {
		return QueueResult{C: busy}, err
	}

	if !tail {
		// header -> entry -> the old first entry (the header itself in
		// an empty queue).
		succ := header + forward

		err := storeLinks(cpu, mem,
			queueLink{entry, succ - entry},
			queueLink{entry + 4, header - entry},
			queueLink{succ + 4, entry - succ},
			queueLink{header, entry - header})

		return QueueResult{Z: forward == 0}, err
	}

	// The old last entry (the header in an empty queue) -> entry ->
	// header.
	backward, err := mem.LoadLongword(cpu, header+4)
	if err != nil {
		return QueueResult{}, err
	}

	pred := header + backward
	if !quadAligned(pred) {
		return QueueResult{}, reservedOperandFault()
	}

	err = storeLinks(cpu, mem,
		queueLink{entry, header - entry},
		queueLink{entry + 4, pred - entry},
		queueLink{pred, entry - pred},
		queueLink{header + 4, entry - header})

	return QueueResult{Z: pred == header}, err
}

// RemoveInterlocked is REMQHI (tail false: the first entry) and REMQTI
// (tail true: the last entry) on the self-relative queue at header. The
// entry removed is the result's Removed (the header, with V, when the
// queue was empty); Z reports that the queue is empty now; C (with V)
// that the secondary interlock was set and nothing changed. addr is the
// location the instruction's addr operand stores to, or 0 for a
// register: a header at that same address is a reserved operand fault.
func RemoveInterlocked(cpu *registers.CPU, mem *vm.Memory, header, addr uint32, tail bool) (QueueResult, error) {
	if addr != 0 && addr == header {
		return QueueResult{}, reservedOperandFault()
	}

	forward, busy, err := headerLink(cpu, mem, header)
	if err != nil || busy {
		return QueueResult{V: busy, C: busy}, err
	}

	link := forward
	if tail {
		if link, err = mem.LoadLongword(cpu, header+4); err != nil {
			return QueueResult{}, err
		}
	}

	if link == 0 {
		return QueueResult{Z: true, V: true, Removed: header}, nil
	}

	entry := header + link
	if !quadAligned(entry) {
		return QueueResult{}, reservedOperandFault()
	}

	if !tail {
		// header -> entry -> succ becomes header -> succ.
		next, err := mem.LoadLongword(cpu, entry)
		if err != nil {
			return QueueResult{}, err
		}

		succ := entry + next
		if !quadAligned(succ) {
			return QueueResult{}, reservedOperandFault()
		}

		err = storeLinks(cpu, mem,
			queueLink{succ + 4, header - succ},
			queueLink{header, succ - header})

		return QueueResult{Z: succ == header, Removed: entry}, err
	}

	// pred -> entry -> header becomes pred -> header.
	prev, err := mem.LoadLongword(cpu, entry+4)
	if err != nil {
		return QueueResult{}, err
	}

	pred := entry + prev
	if !quadAligned(pred) {
		return QueueResult{}, reservedOperandFault()
	}

	err = storeLinks(cpu, mem,
		queueLink{pred, header - pred},
		queueLink{header + 4, pred - header})

	return QueueResult{Z: pred == header, Removed: entry}, err
}

// emulInsqhi is INSQHI entry.ab, header.aq; emulInsqti is INSQTI.
func emulInsqhi(e *Engine, d *Decoded) error { return insertInterlocked(e, d, false) }
func emulInsqti(e *Engine, d *Decoded) error { return insertInterlocked(e, d, true) }

func insertInterlocked(e *Engine, d *Decoded, tail bool) error {
	r, err := InsertInterlocked(e.cpu, e.mem, d.Operands[0].Addr, d.Operands[1].Addr, tail)
	if err != nil {
		return err
	}

	setQueueCodes(e, r)

	return nil
}

// emulRemqhi is REMQHI header.aq, addr.wl; emulRemqti is REMQTI.
func emulRemqhi(e *Engine, d *Decoded) error { return removeInterlocked(e, d, false) }
func emulRemqti(e *Engine, d *Decoded) error { return removeInterlocked(e, d, true) }

func removeInterlocked(e *Engine, d *Decoded, tail bool) error {
	var addr uint32
	if d.Operands[1].Kind == OperandMemory {
		addr = d.Operands[1].Addr
	}

	r, err := RemoveInterlocked(e.cpu, e.mem, d.Operands[0].Addr, addr, tail)
	if err != nil {
		return err
	}

	if !r.C {
		if err := d.Operands[1].Store(e.cpu, e.mem, uint64(r.Removed)); err != nil {
			return err
		}
	}

	setQueueCodes(e, r)

	return nil
}
