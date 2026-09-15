package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// ExamSize is a EXAMINE/DEPOSIT data size/format, matching the subset of
// console_exam.c's /BYTE, /WORD, /LONGWORD, /ASCII and /PTE format
// qualifiers this port implements — see doc.go for the ones intentionally
// left out (/F_FLOATING, /D_FLOATING, /DESCRIPTOR, /COUNTED, /ZERO, the
// register-value-only shortcuts) and why.
type ExamSize int

const (
	SizeLongword ExamSize = iota
	SizeWord
	SizeByte
	SizeASCII
	SizePTE
)

// registerNames matches console_exam.c's regnames table: R0-R15 plus the
// AP/FP/SP/PC aliases.
var registerNames = map[string]vax.Reg{
	"R0": vax.R0, "R1": vax.R1, "R2": vax.R2, "R3": vax.R3,
	"R4": vax.R4, "R5": vax.R5, "R6": vax.R6, "R7": vax.R7,
	"R8": vax.R8, "R9": vax.R9, "R10": vax.R10, "R11": vax.R11,
	"R12": vax.R12, "R13": vax.R13, "R14": vax.R14, "R15": vax.R15,
	"AP": vax.AP, "FP": vax.FP, "SP": vax.SP, "PC": vax.PC,
}

func sizeBytes(sz ExamSize) uint32 {
	switch sz {
	case SizeWord:
		return 2
	case SizeByte, SizeASCII:
		return 1
	default:
		return 4
	}
}

// Examine implements EXAMINE: displays either a register (reg != "") or a
// range of memory starting at addr, count units of size sz, matching
// console_exam.c's core loop — minus the special-case data structures
// listed in doc.go's scope note.
func (c *Console) Examine(reg string, addr uint32, count uint32, sz ExamSize) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if reg != "" {
		r, ok := registerNames[strings.ToUpper(reg)]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADREG, reg)
		}

		c.Printf("%-4s: %s\n", strings.ToUpper(reg), c.formatOne(0, sz, c.CPU.GPR(r)))

		return nil
	}

	if count == 0 {
		count = 1
	}

	step := sizeBytes(sz)
	for n := uint32(0); n < count; n++ {
		a := addr + n*step

		v, err := c.loadSized(a, sz)
		if err != nil {
			return err
		}

		c.Printf("%08X: %s\n", a, c.formatOne(a, sz, v))
	}

	c.DepositAddr = addr + count*step

	return nil
}

func (c *Console) loadSized(addr uint32, sz ExamSize) (uint32, error) {
	switch sz {
	case SizeWord:
		v, err := c.Mem.LoadWord(c.CPU, addr)

		return uint32(v), err

	case SizeByte, SizeASCII:
		v, err := c.Mem.LoadByte(c.CPU, addr)

		return uint32(v), err

	default: // SizeLongword, SizePTE
		return c.Mem.LoadLongword(c.CPU, addr)
	}
}

func (c *Console) formatOne(addr uint32, sz ExamSize, v uint32) string {
	switch sz {
	case SizeByte:
		return fmt.Sprintf("%02X", v&0xFF)

	case SizeWord:
		return fmt.Sprintf("%04X", v&0xFFFF)

	case SizeASCII:
		ch := byte(v & 0xFF)
		if ch < ' ' || ch > 127 {
			ch = '.'
		}

		return string(ch)

	case SizePTE:
		valid := 0

		pte := vm.PTE(v)
		if pte.Valid() {
			valid = 1
		}

		modified := 0
		if pte.Modified() {
			modified = 1
		}

		return fmt.Sprintf("%08X   V:%d  PROT:%02d  M:%d  OWN:%d  PFN:%08X",
			uint32(pte), valid, pte.Protection(), modified, pte.Owner(), pte.PFN()<<9)

	default:
		return fmt.Sprintf("%08X", v)
	}
}

// Deposit implements DEPOSIT: stores value at a register or a memory
// address/size, matching Examine's addressing conventions. The C source has
// no standalone DEPOSIT command of its own — memory is normally modified
// through the inline mini-assembler's immediate mode (Phase 11's scope,
// not ported here) — so this is a small, deliberate Go-native addition
// implementing the "DEPOSIT" deliverable docs/PHASE-08.md names explicitly,
// using the same size/register/addressing rules as Examine rather than
// inventing a different convention.
func (c *Console) Deposit(reg string, addr uint32, sz ExamSize, value uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if reg != "" {
		r, ok := registerNames[strings.ToUpper(reg)]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADREG, reg)
		}

		c.CPU.SetGPR(r, value)

		return nil
	}

	switch sz {
	case SizeWord:
		if err := c.Mem.StoreWord(c.CPU, addr, uint16(value)); err != nil {
			return err
		}

	case SizeByte, SizeASCII:
		if err := c.Mem.StoreByte(c.CPU, addr, byte(value)); err != nil {
			return err
		}

	default:
		if err := c.Mem.StoreLongword(c.CPU, addr, value); err != nil {
			return err
		}
	}

	c.DepositAddr = addr + sizeBytes(sz)

	return nil
}
