package cpu

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// MATCHC (Match Characters).
//
//	MATCHC objlen.rw, objaddr.ab, srclen.rw, srcaddr.ab
//
// The source string is searched for the object string. Found: R0 is 0, R1
// the address just past the object, R2 the source bytes after the match,
// and R3 the address just past the match. Not found: R0 is the object's
// length, R1 its address, R2 0, and R3 the address just past the source.
// Z is set exactly when it was found (an empty object is found at once);
// N, V, and C are cleared.

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0x39}), emulMatchc)
}

// MatchCharacters is MATCHC's search for the objLen-byte object at obj in
// the srcLen-byte source at src: the offset in the source of its first
// occurrence, and true; or false when there's none. An empty object
// matches at offset 0. LIBRTL's LIB$MATCHC shares it.
func MatchCharacters(cpu *registers.CPU, mem *vm.Memory, objLen uint16, obj uint32, srcLen uint16, src uint32) (uint16, bool, error) {
	object := make([]byte, objLen)

	for i := range object {
		b, err := mem.LoadByte(cpu, obj+uint32(i))
		if err != nil {
			return 0, false, err
		}

		object[i] = b
	}

	for start := uint32(0); start+uint32(objLen) <= uint32(srcLen); start++ {
		matched := true

		for i, want := range object {
			b, err := mem.LoadByte(cpu, src+start+uint32(i))
			if err != nil {
				return 0, false, err
			}

			if b != want {
				matched = false

				break
			}
		}

		if matched {
			return uint16(start), true, nil
		}
	}

	return 0, false, nil
}

func emulMatchc(e *Engine, d *Decoded) error {
	l1v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	l2v, err := d.Operands[2].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	objLen, obj := uint16(l1v), d.Operands[1].Addr
	srcLen, src := uint16(l2v), d.Operands[3].Addr

	at, found, err := MatchCharacters(e.cpu, e.mem, objLen, obj, srcLen, src)
	if err != nil {
		return err
	}

	if found {
		end := uint32(at) + uint32(objLen)
		e.cpu.SetGPR(registers.R0, 0)
		e.cpu.SetGPR(registers.R1, obj+uint32(objLen))
		e.cpu.SetGPR(registers.R2, uint32(srcLen)-end)
		e.cpu.SetGPR(registers.R3, src+end)
	} else {
		e.cpu.SetGPR(registers.R0, uint32(objLen))
		e.cpu.SetGPR(registers.R1, obj)
		e.cpu.SetGPR(registers.R2, 0)
		e.cpu.SetGPR(registers.R3, src+uint32(srcLen))
	}

	psl := e.cpu.PSL()
	psl.SetN(false)
	psl.SetZ(found)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}
