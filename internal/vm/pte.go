package vm

import "github.com/tucats/govax/internal/vax"

// PTE is a VAX page table entry: a 32-bit longword. Bit layout matches the
// non-BIGENDIAN branch of pte.h's struct PTEBITS (LSB-first declaration
// order, which is what every actually-supported little-endian host uses —
// same convention as vax.PSL):
//
//	31  30-27  26  25   24-23  22-21  20-0
//	V | PROT | M | MBZ | OWN  | S    | PFN
//
// reference/AUDIT.md finding C8 flagged sizeof(union PTE) as 8 bytes instead
// of 4 due to the LONGWORD 32-vs-64-bit bug; that's already fixed in
// reference/eVAX (AUDIT.md's "CONFIRMED FIXED" note), and PTE here is simply
// a plain 32-bit type by construction, with no equivalent possible.
type PTE uint32

const (
	pteMaskPFN = 0x1FFFFF // bits 0-20

	pteShiftS = 21
	pteMaskS  = 0x3 // bits 21-22

	pteShiftOwn = 23
	pteMaskOwn  = 0x3 // bits 23-24

	pteBitZ = 1 << 25 // bit 25, must-be-zero
	pteBitM = 1 << 26 // bit 26, modify

	pteShiftProt = 27
	pteMaskProt  = 0xF // bits 27-30

	pteBitV = 1 << 31 // bit 31, valid
)

// PFN returns the page frame number: the physical page's address is
// PFN() << 9 (VAX pages are 512 bytes).
func (p PTE) PFN() uint32 { return uint32(p) & pteMaskPFN }

// Software returns the two Digital-reserved software bits.
func (p PTE) Software() uint8 { return uint8((p >> pteShiftS) & pteMaskS) }

// Owner returns the two owner bits.
func (p PTE) Owner() uint8 { return uint8((p >> pteShiftOwn) & pteMaskOwn) }

// Modified reports whether the page has been written to.
func (p PTE) Modified() bool { return p&pteBitM != 0 }

// Protection returns the page's protection code.
func (p PTE) Protection() Protection { return Protection((p >> pteShiftProt) & pteMaskProt) }

// Valid reports whether the page is mapped to a physical page frame.
func (p PTE) Valid() bool { return p&pteBitV != 0 }

// SetPFN sets the page frame number.
func (p *PTE) SetPFN(v uint32) { *p = (*p &^ pteMaskPFN) | PTE(v&pteMaskPFN) }

// SetSoftware sets the two Digital-reserved software bits.
func (p *PTE) SetSoftware(v uint8) {
	*p = (*p &^ (pteMaskS << pteShiftS)) | PTE(v&pteMaskS)<<pteShiftS
}

// SetOwner sets the two owner bits.
func (p *PTE) SetOwner(v uint8) {
	*p = (*p &^ (pteMaskOwn << pteShiftOwn)) | PTE(v&pteMaskOwn)<<pteShiftOwn
}

// SetModified sets or clears the modify bit.
func (p *PTE) SetModified(v bool) {
	if v {
		*p |= pteBitM
	} else {
		*p &^= pteBitM
	}
}

// SetProtection sets the page's protection code.
func (p *PTE) SetProtection(pr Protection) {
	*p = (*p &^ (pteMaskProt << pteShiftProt)) | PTE(pr&pteMaskProt)<<pteShiftProt
}

// SetValid sets or clears the valid bit.
func (p *PTE) SetValid(v bool) {
	if v {
		*p |= pteBitV
	} else {
		*p &^= pteBitV
	}
}

// Protection is a VAX page protection code, stored in a PTE's PROT field.
// Named values match pte.h's PTE_K_* constants (from the VAX SRM's page
// protection table); codes with no architected mnemonic (1) are reserved.
type Protection uint8

const (
	ProtNA   Protection = 0
	ProtKW   Protection = 2
	ProtKR   Protection = 3
	ProtUW   Protection = 4 // "ALL": every mode may read and write
	ProtEW   Protection = 5
	ProtERKW Protection = 6
	ProtER   Protection = 7
	ProtSW   Protection = 8
	ProtSREW Protection = 9
	ProtSRKW Protection = 10
	ProtSR   Protection = 11
	ProtURSW Protection = 12
	ProtUREW Protection = 13
	ProtURKW Protection = 14
	ProtUR   Protection = 15
)

// allows reports whether an access mode at the given cur_mod privilege level
// may perform the given access type against a page with this protection
// code. This is a direct port of vm.c's algorithmic protection check (the
// VM_TABLE_ACCESS==0 branch, which is what the C source actually builds with
// — the alternate access_map table-driven branch is VM_TABLE_ACCESS==1 dead
// code and is not ported; spot-checking it against this algorithm by hand
// shows they agree on every architecturally-defined protection code, and
// diverge only on the reserved code 1, which no valid PTE should carry).
func (pr Protection) allows(mode vax.AccessMode, access AccessType) bool {
	code := uint32(pr)
	if code == 0 {
		return false
	}

	rm := (code >> 2) & 0x3
	wm := (^code) & 0x3
	cm := uint32(mode)

	if code == uint32(ProtUW) {
		return true
	}
	if cm < wm {
		return true
	}
	if access == AccessRead && cm <= rm {
		return true
	}
	return false
}

// Allows is the exported form of allows, for callers outside this package
// (SHOW PAGE, internal/console) that need to report whether a page's
// protection code would permit a given access without actually performing
// it.
func (pr Protection) Allows(mode vax.AccessMode, access AccessType) bool {
	return pr.allows(mode, access)
}

// protectionNames matches tracevm's own prot_name[] table (vm.c) — the
// architected PTE$K_* mnemonic for each of the 16 possible protection
// codes (code 1 has no architected mnemonic, hence "RESERVED").
var protectionNames = [16]string{
	"NONE", "RESERVED", "KW", "KR",
	"ALL", "EW", "ERKW", "ER",
	"SW", "SREW", "SRKW", "SR",
	"URSW", "UREW", "URKW", "UR",
}

// String returns the protection code's PTE$K_* mnemonic, matching
// tracevm's own prot_name[] lookup (used by SHOW PAGE).
func (pr Protection) String() string {
	if int(pr) < len(protectionNames) {
		return protectionNames[pr]
	}
	return "?"
}
