package cpu

// This is the Go port of emul_misc.c's queue instructions: INSQUE/REMQUE
// (absolute-queue format -- link fields hold absolute addresses) and
// INSQHI/INSQTI/REMQHI/REMQTI (self-relative-queue format -- link fields
// hold byte displacements from the link field's own address). Only these
// six handlers are pulled from emul_misc.c into this phase; see
// docs/PHASE-06.md's design notes for why (the rest of that file is Phase
// 07 territory).

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

// resolveLink resolves the self-relative link field at node+fieldOffset (0
// for the forward link, 4 for backward) to the absolute address it points
// to. The stored displacement is always relative to node itself, never to
// node+fieldOffset -- e.g. a solo node's forward and backward fields hold
// the *same* displacement (both point back to whatever it's linked to),
// even though one is read from node and the other from node+4.
func resolveLink(e *Engine, node, fieldOffset uint32) (uint32, error) {
	off, err := e.mem.LoadLongword(e.cpu, node+fieldOffset)
	if err != nil {
		return 0, err
	}

	return node + off, nil
}

// storeLink is resolveLink's write-side counterpart: stores the
// self-relative displacement from node to target at node+fieldOffset.
func storeLink(e *Engine, node, fieldOffset, target uint32) error {
	return e.mem.StoreLongword(e.cpu, node+fieldOffset, target-node)
}

// emulInsqhi is INSQHI: entry is inserted into the self-relative queue
// headed by header, immediately after the header (at the queue's head).
// Z <- 1 if entry is the first (only) entry in the queue afterward, else 0;
// N/V <- 0. C is always 0 (secondary interlock failure, per the manual,
// never happens in this single-processor emulation -- matching
// emul_insqhi.c, which has no interlock logic at all).
//
// Fixes two bugs found while porting emul_insqhi.c: its emptiness test
// compares the header's resolved forward and backward links to *each
// other* (`hf == hb`) rather than to the header itself, which is also true
// -- wrongly -- whenever the queue already holds exactly one entry (that
// entry's address, not the header's, then equals both hf and hb), routing
// a second insertion through the "first entry" path and silently
// discarding the existing entry; and it never explicitly clears Z on the
// normal (non-empty) insertion path, leaving it at whatever it held before
// the instruction ran. See docs/DEVIATIONS.md.
func emulInsqhi(e *Engine, d *Decoded) error {
	entry := d.Operands[0].Addr
	header := d.Operands[1].Addr

	hf, err := resolveLink(e, header, 0)
	if err != nil {
		return err
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetV(false)
	psl.SetC(false)

	if hf == header {
		if err := storeLink(e, header, 0, entry); err != nil {
			return err
		}

		if err := storeLink(e, header, 4, entry); err != nil {
			return err
		}

		if err := storeLink(e, entry, 0, header); err != nil {
			return err
		}

		if err := storeLink(e, entry, 4, header); err != nil {
			return err
		}

		psl.SetZ(true)
		e.cpu.SetPSL(psl)

		return nil
	}

	if err := storeLink(e, hf, 4, entry); err != nil {
		return err
	}

	if err := storeLink(e, entry, 4, header); err != nil {
		return err
	}

	if err := storeLink(e, header, 0, entry); err != nil {
		return err
	}

	if err := storeLink(e, entry, 0, hf); err != nil {
		return err
	}

	psl.SetZ(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulInsqti is INSQTI: like INSQHI, but entry is inserted at the queue's
// tail (immediately before the header, wrapping around). Shares emulInsqhi's
// two fixes (emptiness test, missing Z clear), plus one more found only
// here: emul_insqti.c's non-empty branch updates the previous last entry's
// *backward* link (`store(hb+4, entry-hb)`) where its own comment
// ("Fix current last entry's forward link to be new entry") -- and the
// actual semantics of a tail insertion -- call for updating its *forward*
// link instead (`hb`, not `hb+4`). Confirmed by direct simulation of both
// forms: the literal C form loses every entry past the second insertion,
// exactly like the emptiness-test bug it compounds; the corrected form
// produces a proper forward and backward traversal for three or more
// entries. See docs/DEVIATIONS.md.
func emulInsqti(e *Engine, d *Decoded) error {
	entry := d.Operands[0].Addr
	header := d.Operands[1].Addr

	hf, err := resolveLink(e, header, 0)
	if err != nil {
		return err
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetV(false)
	psl.SetC(false)

	if hf == header {
		if err := storeLink(e, header, 0, entry); err != nil {
			return err
		}

		if err := storeLink(e, header, 4, entry); err != nil {
			return err
		}

		if err := storeLink(e, entry, 0, header); err != nil {
			return err
		}

		if err := storeLink(e, entry, 4, header); err != nil {
			return err
		}

		psl.SetZ(true)
		e.cpu.SetPSL(psl)

		return nil
	}

	hb, err := resolveLink(e, header, 4)
	if err != nil {
		return err
	}

	if err := storeLink(e, hb, 0, entry); err != nil {
		return err
	}

	if err := storeLink(e, entry, 0, header); err != nil {
		return err
	}

	if err := storeLink(e, header, 4, entry); err != nil {
		return err
	}

	if err := storeLink(e, entry, 4, hb); err != nil {
		return err
	}

	psl.SetZ(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulRemqhi is REMQHI: the entry following header (the queue's head) is
// removed and its address written to the second operand. V <- 1 if the
// queue was already empty (nothing removed); Z <- 1 if the queue is empty
// afterward; N and C are always 0 (again, no multiprocessor interlock to
// fail). Verified correct against the manual as ported -- unlike INSQHI/
// INSQTI, emul_remqhi.c's emptiness test already compares the header's raw
// forward-link offset to literal 0, which is unambiguous.
func emulRemqhi(e *Engine, d *Decoded) error {
	header := d.Operands[0].Addr

	fwdOff, err := e.mem.LoadLongword(e.cpu, header)
	if err != nil {
		return err
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetC(false)

	if fwdOff == 0 {
		psl.SetZ(true)
		psl.SetV(true)
		e.cpu.SetPSL(psl)

		return d.Operands[1].Store(e.cpu, e.mem, uint64(header))
	}

	backOff, err := e.mem.LoadLongword(e.cpu, header+4)
	if err != nil {
		return err
	}

	removed := header + fwdOff

	if fwdOff == backOff {
		if err := d.Operands[1].Store(e.cpu, e.mem, uint64(removed)); err != nil {
			return err
		}

		if err := e.mem.StoreLongword(e.cpu, header, 0); err != nil {
			return err
		}

		if err := e.mem.StoreLongword(e.cpu, header+4, 0); err != nil {
			return err
		}

		psl.SetZ(true)
		psl.SetV(false)
		e.cpu.SetPSL(psl)

		return nil
	}

	nextFwdOff, err := e.mem.LoadLongword(e.cpu, removed)
	if err != nil {
		return err
	}

	newFirstOff := fwdOff + nextFwdOff
	if err := e.mem.StoreLongword(e.cpu, header, newFirstOff); err != nil {
		return err
	}

	newFirst := header + newFirstOff
	if err := e.mem.StoreLongword(e.cpu, newFirst+4, -newFirstOff); err != nil {
		return err
	}

	if err := d.Operands[1].Store(e.cpu, e.mem, uint64(removed)); err != nil {
		return err
	}

	psl.SetZ(false)
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulRemqti is REMQTI: like REMQHI, but removes the entry preceding header
// (the queue's tail).
func emulRemqti(e *Engine, d *Decoded) error {
	header := d.Operands[0].Addr

	backOff, err := e.mem.LoadLongword(e.cpu, header+4)
	if err != nil {
		return err
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetC(false)

	if backOff == 0 {
		psl.SetZ(true)
		psl.SetV(true)
		e.cpu.SetPSL(psl)

		return nil
	}

	fwdOff, err := e.mem.LoadLongword(e.cpu, header)
	if err != nil {
		return err
	}

	removed := header + backOff

	if backOff == fwdOff {
		if err := d.Operands[1].Store(e.cpu, e.mem, uint64(removed)); err != nil {
			return err
		}

		if err := e.mem.StoreLongword(e.cpu, header, 0); err != nil {
			return err
		}
		
		if err := e.mem.StoreLongword(e.cpu, header+4, 0); err != nil {
			return err
		}

		psl.SetZ(true)
		psl.SetV(false)
		e.cpu.SetPSL(psl)

		return nil
	}

	prevBackOff, err := e.mem.LoadLongword(e.cpu, removed+4)
	if err != nil {
		return err
	}

	newLastOff := backOff + prevBackOff
	if err := e.mem.StoreLongword(e.cpu, header+4, newLastOff); err != nil {
		return err
	}

	newLast := header + newLastOff
	if err := e.mem.StoreLongword(e.cpu, newLast, -newLastOff); err != nil {
		return err
	}

	if err := d.Operands[1].Store(e.cpu, e.mem, uint64(removed)); err != nil {
		return err
	}

	psl.SetZ(false)
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return nil
}
