package vm

import "github.com/tucats/govax/internal/vax"

// PTE is a VAX page table entry: a single 32-bit value that describes
// everything the hardware (and this emulator) needs to know about one
// virtual memory page. "Longword" is VAX terminology for a 32-bit value —
// you'll see it throughout this codebase and the reference C source instead
// of "uint32" or "DWORD".
//
// In Go, `type PTE uint32` declares PTE as a brand-new named type whose
// underlying representation is an unsigned 32-bit integer. It is not just
// an alias: PTE and uint32 are different types as far as the compiler is
// concerned (you must convert explicitly between them, e.g. uint32(somePTE)
// or PTE(someUint32)), which stops PTE values from being accidentally mixed
// up with unrelated uint32s elsewhere in the code, while still letting you
// use all the normal integer bitwise operators (&, |, ^, &^, <<, >>) on it.
// Every method below reads or writes a handful of bits within that single
// 32-bit value — see "How the bit packing works" further down for the
// general technique.
//
// A PTE's 32 bits are split into several fields, each carrying a different
// piece of information about the page it describes (bit 31 is the
// high/most-significant bit, bit 0 is the low/least-significant bit):
//
//	31  30-27  26  25   24-23  22-21  20-0
//	V | PROT | M | MBZ | OWN  | S    | PFN
//
// where:
//
//   - PFN (bits 0-20): the Page Frame Number — which physical page of RAM
//     this virtual page maps to. A VAX page is 512 bytes, so the actual
//     physical byte address is PFN<<9 (equivalently PFN*512); see the PFN
//     method below.
//   - S (bits 21-22): two bits reserved by Digital (VAX's original
//     manufacturer) for software use; the hardware itself ignores them.
//     This emulator does not use them for anything either, but exposes
//     them so console commands like SHOW PAGE/SET PAGE can display and set
//     every bit a real PTE has.
//   - OWN (bits 23-24): two "owner" bits, also mostly a software
//     convention rather than something the hardware protection check
//     inspects.
//   - MBZ (bit 25): "must be zero" — reserved for future use; not modeled
//     as a field here since there's nothing to get or set.
//   - M (bit 26): the Modify bit. Hardware sets this automatically the
//     first time the page is written to (see translate.go's Translate),
//     so the operating system can tell which pages have changed and need
//     to be written back to disk if evicted.
//   - PROT (bits 27-30): the four-bit protection code controlling which
//     privilege modes may read or write this page — see the Protection
//     type below.
//   - V (bit 31): the Valid bit. If clear, this virtual page currently has
//     no physical page behind it at all, and any access to it is a
//     "translation not valid" fault (unless demand-paging kicks in — see
//     validatePage in translate.go).
//
// (Bit layout matches the non-BIGENDIAN branch of the original C source's
// pte.h struct PTEBITS, i.e. its low-bit-first declaration order, which is
// what every little-endian host — including the one this Go port targets —
// actually uses.)
//
// reference/AUDIT.md finding C8 flagged sizeof(union PTE) as 8 bytes instead
// of 4 due to the LONGWORD 32-vs-64-bit bug; that's already fixed in
// reference/eVAX (AUDIT.md's "CONFIRMED FIXED" note), and PTE here is simply
// a plain 32-bit type by construction, with no equivalent possible.
type PTE uint32

// How the bit packing works, once for the whole file:
//
// Rather than defining five separate fields on a Go struct, a PTE packs
// everything into one 32-bit integer, mirroring how the real VAX hardware
// (and the C source this is ported from) lays it out. Each constant below
// is either a "mask" (a value with 1s in exactly the bit positions a field
// occupies, once shifted into place) or a "shift" (how many bit positions
// to slide a field left or right to move it between "packed inside the
// PTE" and "an ordinary small integer starting at bit 0").
//
// The general pattern for reading a field is:
//
//	(value >> shift) & mask
//
// — shift the field down so its own bit 0 lands at bit 0, then mask off
// everything else. For a single-bit flag (like Modified or Valid) there's
// no need to shift or mask a multi-bit field; a plain `value & bit != 0`
// check is enough.
//
// The pattern for writing a field is the mirror image:
//
//	value = (value &^ (mask << shift)) | (newFieldValue & mask) << shift
//
// `&^` is Go's "bit clear" (AND NOT) operator: `a &^ b` clears every bit in
// `a` that is set in `b`, leaving the rest of `a` untouched. So
// `value &^ (mask << shift)` zeroes out just this field's bits without
// disturbing any of the other fields packed into the same 32-bit value,
// and the `| (...)<<shift` afterwards drops the new value into that
// now-empty slot. Masking newFieldValue with `& mask` before shifting
// guards against a caller passing in a value wider than the field (e.g.
// passing 7 for a 2-bit field would otherwise corrupt the neighboring
// bits).
const (
	pteMaskPFN = 0x1FFFFF // bits 0-20

	pteShiftS = 21
	pteMaskS  = 0x3 // bits 21-22

	pteShiftOwn = 23
	pteMaskOwn  = 0x3 // bits 23-24

	// pteBitZ = 1 << 25 // bit 25, must-be-zero.
	pteBitM = 1 << 26 // bit 26, modify

	pteShiftProt = 27
	pteMaskProt  = 0xF // bits 27-30

	pteBitV = 1 << 31 // bit 31, valid
)

// PFN returns the page frame number: the physical page's address is
// PFN() << 9 (VAX pages are 512 bytes, and 512 == 1<<9, so shifting left by
// 9 bits is the same as multiplying by 512 — a common low-level trick for
// converting a "page number" into a byte address).
//
// This and the other plain getter methods below (Software, Owner, Modified,
// Protection, Valid) are declared with a *value* receiver, `(p PTE)`,
// rather than a pointer receiver: they only need to read p, not modify it,
// and PTE is small enough (one machine word) that Go can pass a copy around
// cheaply. Compare with the Set* methods further down, which use a pointer
// receiver `(p *PTE)` because they need to modify the caller's original
// PTE value, not a throwaway copy of it.
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
//
// The receiver here is `p *PTE`, a pointer to the caller's PTE value,
// rather than the plain `p PTE` used by the getters above. This matters:
// if the receiver were a plain PTE, `*p = ...` inside the method would only
// modify a local copy, and the change would vanish the moment the method
// returns. With a pointer receiver, `*p` ("the value p points at") refers
// to the caller's actual variable, so the assignment is visible to the
// caller too — this is exactly why every method here that mutates a PTE
// (every Set* method) needs a pointer receiver, and every method that only
// reads one doesn't.
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

// Protection is a VAX page protection code, stored in a PTE's PROT field
// (bits 27-30, four bits, so 16 possible codes — see the const block
// below). It is its own named type (rather than a plain integer) so that
// the compiler helps catch mistakes like passing a raw protection-code
// number where an AccessType was meant, and so methods like allows/Allows
// and String can be attached to it.
//
// The VAX architecture defines four privilege levels, from most to least
// trusted: Kernel, Executive, Supervisor, and User (see vax.AccessMode).
// Every one of the 16 protection codes below is a *combination* of "which
// of those four levels, and every level more privileged than it, may
// read this page" and "which level, and every level more privileged than
// it, may write this page" — e.g. ProtSREW ("Supervisor Read, Executive
// Write") means kernel and executive mode can both read and write the
// page, while supervisor mode can only read it, and user mode can't touch
// it at all. Kernel mode, being the most trusted, can always do at least
// as much as any less-privileged mode is allowed to.
//
// Named values match pte.h's PTE_K_* constants (from the VAX architecture
// reference manual's page protection table); code 1 has no architected
// meaning and is reserved.
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
// code.
//
// The bit-twiddling here isn't a table lookup; it's a compact arithmetic
// encoding of the 16 protection codes' shared structure, which is why it
// looks more like a puzzle than the 16-case switch you might expect. To
// unpack it:
//
//   - rm (read mode) and wm (write mode) are each a 2-bit privilege level
//     (0=Kernel, 1=Executive, 2=Supervisor, 3=User — see vax.AccessMode)
//     extracted from the 4-bit code: the top two bits give the least-
//     privileged mode allowed to *read*, and the bottom two bits, bitwise
//     inverted, give the least-privileged mode allowed to *write*.
//     Inverting is what makes "fewer bits set" correspond to "more
//     privilege required" for the write side the same way it already does
//     for the read side.
//   - cm (current mode) is simply the caller's privilege level, converted
//     to the same 0-3 numbering.
//   - Remember that on the VAX, *lower* privilege-level numbers mean *more*
//     trusted (Kernel is 0, User is 3), so "cm <= rm" reads as "the
//     current mode is at least as privileged as the minimum needed to
//     read", and likewise for cm < wm on the write side.
//   - ProtUW (code 4, "ALL") is special-cased first since its rm/wm
//     extraction would otherwise come out wrong for the general formula.
//
// This is a direct port of vm.c's algorithmic protection check (the
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
