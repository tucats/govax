package link

import (
	"encoding/binary"
	"fmt"
	"slices"
	"sort"

	"github.com/tucats/govax/internal/obj"
)

// This file is what makes a shareable image different from an executable
// (LINK/SHAREABLE; the Linker manual, chapter 4, and testdata/link68's
// probe, docs/PHASE-68 - link shareable images.md):
//
//   - The image starts at address 0, not 0x200 (origin), since the image
//     activator maps it wherever it finds room, and it has no user stack.
//   - Its header's image type is IHD$K_LIM, its IHD$V_LNKNOTFR flag is
//     set (it has no user transfer address), and its global section ident
//     and match control are GSMATCH='s (image.go).
//   - Two more psect attributes decide which image section a psect goes
//     in, SHR and PIC (sectionKey), and the section's type says which:
//     ISD$K_SHRFXD, PRVFXD, SHRPIC, or PRVPIC (sectionType). Only a
//     private (NOSHR) writable section is copy on reference.
//   - No section is split into demand-zero runs (SHR1's eight untouched
//     pages stayed in the file, even with DZRO_MIN=1), and LASTCLU marks
//     none.
//   - A .ADDRESS of the image's own data is an address relative to image
//     0, the image itself, which the image activator adds the image's
//     base to (referSelf). The fixup section is there only when something
//     needs it, a .ADDRESS or a reference to another shareable image, and
//     is PRVFXD with IAF$V_SHR (hasFixupSection, fixupSectionFlags).
//   - Its global symbol table lists its universal symbols, which other
//     images are linked against (universal.go).
//
// What isn't done yet (docs/PHASE-68, subtask 7) is refused: a
// position-dependent (NOPIC) psect, a based image (BASE= or a based
// CLUSTER=), and a .ADDRESS in a read-only section.

// sectionType returns the ISD$K_ type of image section s: 0 (NORMAL) in
// an executable, and in a shareable image the type its SHR and PIC
// attributes give.
func (l *linker) sectionType(s *section) uint32 {
	if !l.opts.Shareable {
		return 0
	}

	switch s.key & (keyPIC | keyNOSHR) {
	case 0:
		return isdTypeSharedFix
	case keyNOSHR:
		return isdTypePrivateFix
	case keyPIC:
		return isdTypeSharedPIC
	default:
		return isdTypePrivatePIC
	}
}

// copyOnReference says writable section s is copy on reference
// (ISD$V_CRF): each process gets its own copy of a page once it touches
// it. Every writable section of an executable is; in a shareable image,
// only a private one, since a shared writable section is one copy every
// process maps (the Linker manual, Table 6-1).
func (l *linker) copyOnReference(s *section) bool {
	return !l.opts.Shareable || s.key&keyNOSHR != 0
}

// hasFixupSection says the image has a fixup section: an executable
// always does, and a shareable image when it refers to another shareable
// image or has a .ADDRESS to fix up (SHR3, based, with neither, had
// none).
func (l *linker) hasFixupSection() bool {
	return !l.opts.Shareable || len(l.shared) > 0 || l.addressFixups > 0 || len(l.selfAddresses) > 0
}

// fixupSectionFlags are the fixup section's ISD flags: a writable copy
// on reference section, which a shareable image's types PRVFXD.
func (l *linker) fixupSectionFlags() uint32 {
	flags := uint32(isdFIXUPVEC | isdWRT | isdCRF)
	if l.opts.Shareable {
		flags |= isdTypePrivateFix
	}

	return flags
}

// referSelf records a .ADDRESS longword at addr that holds an address in
// the shareable image itself. Its section must be writable, for the image
// activator to fix it up.
func (l *linker) referSelf(addr uint32) error {
	for _, s := range l.sections {
		if addr >= s.base && addr < s.base+s.pages*blockSize && !s.writable() {
			return fmt.Errorf("a .ADDRESS at %%X%08X, in a read-only section of a shareable image, isn't supported yet", addr)
		}
	}

	l.selfAddresses = append(l.selfAddresses, addr)

	return nil
}

// checkShareable refuses a shareable image govax's LINK can't make yet:
// one with a base, or with a position-dependent psect.
func (l *linker) checkShareable() error {
	if !l.opts.Shareable {
		return nil
	}

	for _, c := range l.clusters {
		if c.user() && c.based {
			return fmt.Errorf("link: a based shareable image (cluster %s) isn't supported yet", c.name)
		}
	}

	for _, p := range l.order {
		if p.flags&obj.PsectREL != 0 && p.flags&obj.PsectPIC == 0 && len(p.contribs) > 0 {
			return fmt.Errorf("link: psect %s is position dependent (NOPIC), which a shareable image can't have yet", p.name)
		}
	}

	return nil
}

// DST fixup records (docs/DEBUG-RECORDS.md, 18.1): a shareable image's
// debug symbol table holds addresses that are offsets in the image, and
// the debugger adds the image's base to each longword a fixup record
// lists. Real LINK wrote one record after the last module's, at the
// start of the next block (SHR1, whose ten symbol and psect values were
// at offsets 0x0F to 0xC5 of the table's first block). The offsets are
// in the order of the clusters the addresses are in, and in the table's
// order within a cluster: SHR4's SHR1_RO, which COLLECT= moved into
// cluster FIRST, came before the default cluster's routines, though its
// psect's record followed theirs. The record is its length byte,
// its type, DST$K_FIXUP, then a group, a longword count, a counted image
// name, and each offset as a longword. The name was DEFAULT_CLUSTER,
// though two of the offsets were in cluster SHR1_VECTORS's module; govax
// names every group so (unconfirmed). A record holds at most
// maxDSTFixups offsets, its length byte's limit; how real LINK splits a
// longer list is unconfirmed, and govax writes a record for each part.
const (
	dstTypeFixup     = 134 // DST$K_FIXUP
	dstFixupImage    = defaultClusterName
	dstFixupOverhead = 1 + 4 + 1 + len(dstFixupImage) // type, count, name
	maxDSTFixups     = (255 - dstFixupOverhead) / 4
)

// dstFixup is a longword of the debug symbol table that holds an address
// in the image: its offset in the table, and the address.
type dstFixup struct {
	offset, address uint32
}

// withDSTFixups returns dst with a shareable image's fixup records after
// it, from the next block, or dst itself if it needs none.
func (l *linker) withDSTFixups(dst []byte) []byte {
	if !l.opts.Shareable || len(l.dstFixups) == 0 {
		return dst
	}

	// Each address's cluster, by its place in the list; one past them all
	// for an address no cluster holds.
	clusterOf := func(addr uint32) int {
		for i, c := range l.clusters {
			if c.user() && c.size > 0 && addr >= c.base && addr < c.base+c.size {
				return i
			}
		}

		return len(l.clusters)
	}

	fixups := slices.Clone(l.dstFixups)
	sort.SliceStable(fixups, func(i, j int) bool { return clusterOf(fixups[i].address) < clusterOf(fixups[j].address) })

	dst = append(dst, make([]byte, int(pageUp(uint32(len(dst))))-len(dst))...)

	for rest := fixups; len(rest) > 0; {
		part := rest[:min(len(rest), maxDSTFixups)]
		rest = rest[len(part):]

		rec := []byte{byte(dstFixupOverhead + 4*len(part)), dstTypeFixup}
		rec = binary.LittleEndian.AppendUint32(rec, uint32(len(part)))
		rec = append(rec, byte(len(dstFixupImage)))
		rec = append(rec, dstFixupImage...)

		for _, f := range part {
			rec = binary.LittleEndian.AppendUint32(rec, f.offset)
		}

		dst = append(dst, rec...)
	}

	return dst
}
