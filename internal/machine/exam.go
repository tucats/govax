package machine

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// ExamSize is a EXAMINE/DEPOSIT data size/format, supporting the subset
// of /BYTE, /WORD, /LONGWORD, /ASCII and /PTE format
// qualifiers — see doc.go for the ones intentionally
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

// registerNames R0-R15 plus the AP/FP/SP/PC aliases.
var registerNames = map[string]registers.Reg{
	"R0": registers.R0, "R1": registers.R1, "R2": registers.R2, "R3": registers.R3,
	"R4": registers.R4, "R5": registers.R5, "R6": registers.R6, "R7": registers.R7,
	"R8": registers.R8, "R9": registers.R9, "R10": registers.R10, "R11": registers.R11,
	"R12": registers.R12, "R13": registers.R13, "R14": registers.R14, "R15": registers.R15,
	"AP": registers.AP, "FP": registers.FP, "SP": registers.SP, "PC": registers.PC,
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
// range of memory starting at addr, count units of size sz.
func (m *Machine) Examine(reg string, addr uint32, count uint32, sz ExamSize) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if reg != "" {
		r, ok := registerNames[strings.ToUpper(reg)]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADREG, reg)
		}

		m.Printf("%-4s: %s\n", strings.ToUpper(reg), m.formatOne(0, sz, m.CPU.GPR(r)))

		return nil
	}

	if count == 0 {
		count = 1
	}

	step := sizeBytes(sz)
	for n := uint32(0); n < count; n++ {
		a := addr + n*step

		v, err := m.loadSized(a, sz)
		if err != nil {
			return err
		}

		m.Printf("%08X: %s\n", a, m.formatOne(a, sz, v))
	}

	m.DepositAddr = addr + count*step

	return nil
}

// loadSized reads through translation forced into kernel mode (see
// withKernelMode): EXAMINE is console-driven memory access, on real VAX
// hardware independent of whatever mode the CPU was last running in, so it
// must not be gated by that program's own PSL (see kernel.asm's own
// EXE$INITIALIZE, which legitimately drops to user mode and halts there as
// its normal, designed completion).
func (m *Machine) loadSized(addr uint32, sz ExamSize) (v uint32, err error) {
	err = m.withKernelMode(func() error {
		switch sz {
		case SizeWord:
			w, err := m.Mem.LoadWord(m.CPU, addr)
			v = uint32(w)

			return err

		case SizeByte, SizeASCII:
			b, err := m.Mem.LoadByte(m.CPU, addr)
			v = uint32(b)

			return err

		default: // SizeLongword, SizePTE
			var err error
			v, err = m.Mem.LoadLongword(m.CPU, addr)

			return err
		}
	})

	return v, err
}

func (m *Machine) formatOne(addr uint32, sz ExamSize, v uint32) string {
	_ = addr // ignored for now

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
func (m *Machine) Deposit(reg string, addr uint32, sz ExamSize, value uint32) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if reg != "" {
		r, ok := registerNames[strings.ToUpper(reg)]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADREG, reg)
		}

		m.CPU.SetGPR(r, value)

		return nil
	}

	if err := m.ensureConsolePages(addr, sizeBytes(sz)); err != nil {
		return err
	}

	// See loadSized's doc comment: DEPOSIT is console-driven memory access
	// and must not be gated by whatever mode the CPU was last running in.
	err := m.withKernelMode(func() error {
		switch sz {
		case SizeWord:
			return m.Mem.StoreWord(m.CPU, addr, uint16(value))

		case SizeByte, SizeASCII:
			return m.Mem.StoreByte(m.CPU, addr, byte(value))

		default:
			return m.Mem.StoreLongword(m.CPU, addr, value)
		}
	})
	if err != nil {
		return err
	}

	m.DepositAddr = addr + sizeBytes(sz)

	return nil
}

// ensureConsolePages creates the P0 pages the console is about to write
// length bytes of at addr (ASM's program, DEPOSIT's value) when they
// don't exist, so interactive use works as it does with paging off
// (docs/PHASE-67 - address space under paging.md, subtask 7). With paging
// on, P0's length register describes only the pages that exist, and a
// write past P0's end would be an access violation; this extends P0 to
// cover the pages, each created as $CRETVA creates a user-mode page
// (corevms.Environment.EnsurePages: demand-zero, read/write, charged
// against the paging file quota). It's process 1's convenience only, and
// only P0's: a write into another process (after the debugger's SET
// PROCESS), into P1, or into S0 gets the access check a program would.
// With paging off it does nothing, since every page of the tables exists.
func (m *Machine) ensureConsolePages(addr, length uint32) error {
	env := m.processOne
	if env == nil || length == 0 || m.RunningProcess() != env {
		return nil
	}

	// Only the part that lies in P0 (addresses below 0x40000000).
	const p1Base = 0x40000000
	if addr >= p1Base {
		return nil
	}

	length = min(length, p1Base-addr)

	return env.EnsurePages(addr, length)
}
