package console

import (
	"fmt"
	"os"
	"strings"
)

// This file is the Go port of console_run.c's image_load/image_fixup and
// the IHD/IHI/ISD/IAF struct definitions from imgdef.h (Phase 13). Unlike
// structure_mapping.c's generic, name-keyed map()/STROFF machinery (built
// for the C source's interactive EXAMINE/DEPOSIT struct support), this
// port reads each struct's fields directly at their documented byte
// offsets -- matching the precedent already set by internal/rtl/rms.go's
// own port of FAB/RAB field access, which has the same "declarative offset
// table in C, direct typed reads in Go" relationship. The offsets below are
// exactly console_run.c's own init_ihd_maps table, not re-derived.
//
// This also answers reference/eVAX/AUDIT.md's own V8 finding, left
// explicitly unresolved there ("read console_run.c's image-loading path
// directly and resolve which case applies before triaging this further"):
// whether IHD/IHI/ISD/IAF get overlaid directly onto raw file bytes as C
// structs (in which case the 32-vs-64-bit LONGWORD bug would misalign
// every field after the first, the same failure mode as V1's ROM format)
// or built up field-by-field through VAX-memory accessors. It's the
// latter -- console_run.c's own image_load reads each field with
// load_memory calls at computed offsets, never a raw struct cast over the
// file buffer -- so V8's worse-case branch doesn't apply, confirmed (not
// assumed) by direct inspection while porting this file in Phase 13 and
// re-confirmed as part of Phase 12's audit cross-check. This port's own
// design (typed field reads through vm.Memory, see the doc comment above)
// has no LONGWORD-width concept to misalign in the first place either way.

// ICB flag bits, matching imgdef.h's ICB_* constants.
const (
	icbMain       = 0x00000001
	icbSecondary  = 0x00000002
	icbFixed      = 0x00000004
	icbIncomplete = 0x80000000
)

// ISD flag bits, matching imgdef.h's ISD_M_* constants.
const (
	isdGBL      = 1 << 0
	isdDZRO     = 1 << 2
	isdFIXUPVEC = 1 << 10
)

// isdUsrStack is ISD_K_USRSTACK, the image-section type value (top byte of
// the flags longword) marking the initial user-stack section. The C
// source's own skip check (`if (n == -3) continue`) compares this unsigned
// byte-extracted type against a value it can never equal -- ISD_B_TYPE's
// mask+shift never produces a negative LONGWORD, so that branch is dead
// code, a plain copy-paste slip against the already-defined ISD_K_USRSTACK
// constant, not an ISA/hardware-fidelity question. Fixed directly here
// (compare against 253, the real constant) per this project's policy on
// clear, obvious logic errors -- this is loader/tooling code, not emulated
// VAX instruction-set behavior, so it's not a docs/DEVIATIONS.md matter
// either way (see Phase 10's own precedent for this category).
const isdUsrStack = 253

func isdType(flags uint32) uint32 { return flags >> 24 }

// ISD is one Image Section Descriptor, the on-disk-variable-length record
// imgdef.h's struct ISD describes. Name is read as a counted string (Count
// bytes at the NAME offset), not padded/truncated to the C struct's
// (already inconsistently-sized, per imgdef.h vs. its own count range)
// 15-byte field -- Go strings have no such fixed-buffer concern.
type ISD struct {
	Pages     uint16
	VPN       uint16
	Flags     uint32
	VBN       uint32
	SectionID uint32
	Name      string
}

// IAF is the Image Attribute/Fixup header (imgdef.h's struct IAF): offsets
// (relative to the IAF's own base address) of the G^ fixup list, the
// .ADDRESS fixup list, and the sharable-image name list.
type IAF struct {
	OffsetGFix    uint32
	OffsetAddr    uint32
	OffsetChgprot uint32
	OffsetShl     uint32
	ShrImgCnt     uint32
	OffsetNames   uint32 // derived, not part of the on-disk IAF table itself -- see readIAFNames
}

// SHR is one entry in an ICB's sharable-image dependency list: id 0 is
// always the image's self-reference (Icb == the image's own ICB, once
// loaded), matching imgdef.h's struct SHR.
type SHR struct {
	Name string
	ID   uint32
	Base uint32
	Icb  *ICB
}

// ICB is an Image Control Block: one loaded image (main or a sharable
// dependency), matching imgdef.h's struct ICB.
type ICB struct {
	Name     string
	Base     uint32
	End      uint32
	Flags    uint32
	Transfer [4]uint32
	ISDList  []*ISD
	FixupISD *ISD
	IAF      IAF
	SHRList  []*SHR
}

// resetICBList discards every loaded image and resets the P0 high-water
// mark, matching reset_icb_list. Unlike the C source, there is no explicit
// per-ISD/per-ICB freemem to port -- Go's GC reclaims them once ICBList is
// replaced.
func (c *Console) resetICBList() {
	c.ICBList = nil
	c.RTL.RegionSize[0] = 0
}

// findMainICB returns the ICB flagged ICB_MAIN, or nil, matching
// find_main_icb.
func (c *Console) findMainICB() *ICB {
	for _, icb := range c.ICBList {
		if icb.Flags&icbMain != 0 {
			return icb
		}
	}
	return nil
}

// storeBytes writes data starting at virtual address addr, through
// whatever translation is currently in effect (kernel mode, VM on, for
// every caller in this file).
func (c *Console) storeBytes(addr uint32, data []byte) error {
	for i, b := range data {
		if err := c.Mem.StoreByte(c.CPU, addr+uint32(i), b); err != nil {
			return err
		}
	}
	return nil
}

func (c *Console) loadByte(addr uint32) (byte, error)   { return c.Mem.LoadByte(c.CPU, addr) }
func (c *Console) loadWord(addr uint32) (uint16, error) { return c.Mem.LoadWord(c.CPU, addr) }
func (c *Console) loadLong(addr uint32) (uint32, error) { return c.Mem.LoadLongword(c.CPU, addr) }
func (c *Console) storeLong(addr, v uint32) error       { return c.Mem.StoreLongword(c.CPU, addr, v) }

// readIHDTransferOffset/readIHDIdentOffset read the two IHD fields
// console_run.c's image_load actually consults for control flow
// (init_ihd_maps's OFFSET_TRANSFER @2, OFFSET_IDENT @6); HEADER_BLOCKS @16
// is read directly from the host-side file buffer instead (see imageLoad),
// since it's needed before any of the header has been written into VAX
// memory at all. The rest of the IHD (size, offset_dst/patch, major/minor
// id, mask, channels, io_pages, flags, section_id, version) and the IHI's
// own image-name field only ever feed this phase's C source's own debug
// printing, with no consumer in this port.
func (c *Console) readIHDTransferOffset(base uint32) (uint32, error) {
	v, err := c.loadWord(base + 2)
	return uint32(v), err
}

func (c *Console) readIHDIdentOffset(base uint32) (uint32, error) {
	v, err := c.loadWord(base + 6)
	return uint32(v), err
}

// ihiSize is IHI_SIZE from imgdef.h: the fixed size of the Image
// Identification block, used to locate the first ISD immediately after it.
const ihiSize = 80

// readISD reads one Image Section Descriptor at addr, returning it along
// with its own declared on-disk size (image_load's own loop advances by
// this, not sizeof(ISD), since ISD records are variable-length).
func (c *Console) readISD(addr uint32) (*ISD, uint32, error) {
	size, err := c.loadWord(addr)
	if err != nil {
		return nil, 0, err
	}
	if size == 0 {
		return nil, 0, nil
	}
	pages, err := c.loadWord(addr + 2)
	if err != nil {
		return nil, 0, err
	}
	vpn, err := c.loadWord(addr + 4)
	if err != nil {
		return nil, 0, err
	}
	flags, err := c.loadLong(addr + 8)
	if err != nil {
		return nil, 0, err
	}
	vbn, err := c.loadLong(addr + 12)
	if err != nil {
		return nil, 0, err
	}
	sectionID, err := c.loadLong(addr + 16)
	if err != nil {
		return nil, 0, err
	}
	count, err := c.loadByte(addr + 20)
	if err != nil {
		return nil, 0, err
	}
	name := "<NONE>"
	if count >= 1 && count <= 39 {
		buf := make([]byte, count)
		for i := range buf {
			b, err := c.loadByte(addr + 21 + uint32(i))
			if err != nil {
				return nil, 0, err
			}
			buf[i] = b
		}
		name = string(buf)
	}

	return &ISD{Pages: pages, VPN: vpn, Flags: flags, VBN: vbn, SectionID: sectionID, Name: name}, uint32(size), nil
}

// readIAF reads the fixed part of an Image Attribute/Fixup header at addr,
// matching init_ihd_maps's IAF field table.
func (c *Console) readIAF(addr uint32) (IAF, error) {
	var (
		iaf IAF
		err error
	)

	if iaf.OffsetGFix, err = c.loadLong(addr + 0x0C); err != nil {
		return iaf, err
	}

	if iaf.OffsetAddr, err = c.loadLong(addr + 0x10); err != nil {
		return iaf, err
	}

	if iaf.OffsetChgprot, err = c.loadLong(addr + 0x14); err != nil {
		return iaf, err
	}

	if iaf.OffsetShl, err = c.loadLong(addr + 0x18); err != nil {
		return iaf, err
	}

	if iaf.ShrImgCnt, err = c.loadLong(addr + 0x1C); err != nil {
		return iaf, err
	}
	
	return iaf, nil
}

// imageLoad loads fn as a VMS image, matching image_load: an already-loaded
// image of the same name is a no-op success (returning the existing ICB);
// otherwise the file is opened, its header parsed, each section loaded or
// zero-filled into P0 space starting at the current high-water mark
// (c.RTL.RegionSize[0]), and -- if the image carries a FIXUPVEC section --
// its sharable-image dependency list is built and recursively loaded
// (secondary images only; a missing secondary image file is tolerated, not
// fatal, matching image_load's own VAX_FNF handling one level up in
// console_run/here).
func (c *Console) imageLoad(fn string, flag uint32) (*ICB, error) {
	for _, icb := range c.ICBList {
		if icb.Name == fn {
			return icb, nil
		}
	}

	path, ok := c.findImage(fn)
	if !ok {
		return nil, fmt.Errorf("console: image %s not found", fn)
	}

	data, err := c.Paths.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 512 {
		return nil, fmt.Errorf("console: image %s: file too short for a header", fn)
	}

	nblocks := data[16]
	if nblocks == 0 {
		nblocks = 1
	}

	// The C source loads the header one page above the current high-water
	// mark (page zero of P0 can't be written on VMS -- it's the guard
	// page VMInit installs, see vminit.go), then treats everything from
	// there on as belonging to the image itself.
	base := c.RTL.RegionSize[0] + 0x200
	headerLen := int(nblocks) * 512
	if headerLen > len(data) {
		headerLen = len(data)
	}
	if err := c.storeBytes(base, data[:headerLen]); err != nil {
		return nil, err
	}

	icb := &ICB{Base: c.RTL.RegionSize[0], Flags: flag | icbIncomplete}
	if flag == icbMain {
		icb.Name = "<MAIN>"
	} else {
		icb.Name = fn
	}

	c.ICBList = append(c.ICBList, icb)

	transferOffset, err := c.readIHDTransferOffset(base)
	if err != nil {
		return nil, err
	}

	identOffset, err := c.readIHDIdentOffset(base)
	if err != nil {
		return nil, err
	}

	for n := uint32(0); n < 4; n++ {
		v, err := c.loadLong(base + transferOffset + n*4)
		if err != nil {
			return nil, err
		}

		if v != 0 {
			var sname string

			v += icb.Base
			if icb.Name != "<MAIN>" {
				if n == 0 {
					sname = fmt.Sprintf("SHARE$%s_INITIALIZE", icb.Name)
				} else {
					sname = fmt.Sprintf("SHARE$%s_TRANSFER_%d", icb.Name, n)
				}
			} else {
				sname = "MAIN"
			}

			c.Symbols.Set(sname, v, SymbolSystem)
		}

		icb.Transfer[n] = v
	}

	isdAddr := base + identOffset + ihiSize
	for i := 0; i < 1000; i++ {
		isd, size, err := c.readISD(isdAddr)
		if err != nil {
			return nil, err
		}

		if isd == nil {
			break
		}

		isdAddr += size
		icb.ISDList = append(icb.ISDList, isd)
	}

	for _, isd := range icb.ISDList {
		if isd.Flags&isdFIXUPVEC != 0 {
			icb.FixupISD = isd
		}

		if isdType(isd.Flags) == isdUsrStack {
			continue
		}
		if isd.Flags&isdGBL != 0 {
			continue
		}

		if isd.Flags&isdDZRO != 0 {
			for n := uint32(0); n < uint32(isd.Pages); n++ {
				addr := icb.Base + ((n + uint32(isd.VPN)) << 9)
				if addr >= c.RTL.RegionSize[0] {
					c.RTL.RegionSize[0] = addr + 512
				}

				if addr+0x1FF > icb.End {
					icb.End = addr + 0x1FF
				}

				if err := c.storeBytes(addr, make([]byte, 512)); err != nil {
					return nil, err
				}
			}

			continue
		}

		fileAddr := int64(isd.VBN-1) * 512

		for n := uint32(0); n < uint32(isd.Pages); n++ {
			addr := icb.Base + ((n + uint32(isd.VPN)) << 9)
			if addr >= c.RTL.RegionSize[0] {
				c.RTL.RegionSize[0] = addr + 512
			}

			if addr+0x1FF > icb.End {
				icb.End = addr + 0x1FF
			}

			block := make([]byte, 512)

			off := fileAddr + int64(n)*512
			if off >= 0 && off < int64(len(data)) {
				copy(block, data[off:])
			}

			if err := c.storeBytes(addr, block); err != nil {
				return nil, err
			}
		}
	}

	icb.Flags &^= icbIncomplete

	if icb.FixupISD != nil {
		addr := icb.Base + (uint32(icb.FixupISD.VPN) << 9)

		iaf, err := c.readIAF(addr)
		if err != nil {
			return nil, err
		}

		namesOffset, err := c.loadLong(addr + iaf.OffsetShl + 16)
		if err != nil {
			return nil, err
		}
		iaf.OffsetNames = namesOffset
		icb.IAF = iaf

		self := &SHR{Name: icb.Name, ID: 0, Base: icb.Base, Icb: icb}
		icb.SHRList = append(icb.SHRList, self)

		naddr := addr + iaf.OffsetShl + 16 + iaf.OffsetNames

		var imageID uint32

		for n := uint32(1); n < iaf.ShrImgCnt; n++ {
			nameAddr := naddr + 0x08
			naddr += 0x40

			nameLen, err := c.loadByte(nameAddr)
			if err != nil {
				return nil, err
			}
			buf := make([]byte, nameLen)
			for i := range buf {
				b, err := c.loadByte(nameAddr + 1 + uint32(i))
				if err != nil {
					return nil, err
				}
				buf[i] = b
			}

			imageID++
			icb.SHRList = append(icb.SHRList, &SHR{Name: string(buf), ID: imageID})
		}

		for _, shr := range icb.SHRList {
			if shr.ID == 0 {
				continue
			}
			dep, err := c.imageLoad(shr.Name, icbSecondary)
			if err != nil {
				continue // matches image_load's VAX_FNF-is-tolerated handling
			}
			shr.Icb = dep
			shr.Base = dep.Base
		}
	}

	return icb, nil
}

// findSHRByID returns the SHR entry with the given id from icb's dependency
// list, or nil.
func findSHRByID(icb *ICB, id uint32) *SHR {
	for _, shr := range icb.SHRList {
		if shr.ID == id {
			return shr
		}
	}
	return nil
}

// findLoadedICB returns the already-loaded ICB with the given name, or nil,
// matching image_fixup's own inline "is it on an ICB list we already have"
// scan.
func (c *Console) findLoadedICB(name string) *ICB {
	for _, icb := range c.ICBList {
		if icb.Name == name {
			return icb
		}
	}
	return nil
}

// resolveFixupTarget returns the absolute address a G^ fixup naming shr and
// requesting offset resolves to: either shr's own already-loaded base plus
// offset (if shr is itself a real loaded image, found by name on ICBList),
// or the SHIM$<shr.Name>_<offset> stub address ensureShims registered,
// matching image_fixup's own "on the ICB list, else look for a shim" order.
func (c *Console) resolveFixupTarget(shr *SHR, offset uint32) (uint32, error) {
	if dep := c.findLoadedICB(shr.Name); dep != nil {
		return dep.Base + offset, nil
	}
	name := fmt.Sprintf("SHIM$%s_%08X", shr.Name, offset)
	v, ok := c.Symbols.Get(name)
	if !ok {
		return 0, fmt.Errorf("console: unresolved shim symbol %s", name)
	}
	return v, nil
}

// imageFixup performs load-time linking for icb: G^ fixups (each fixup
// vector slot at naddr initially holds an offset into the target sharable
// image and is overwritten in place with the resolved absolute address --
// this is what a G^ reference in the compiled code actually indirects
// through) and .ADDRESS fixups (each slot names a *different* longword
// elsewhere in icb's own memory whose current offset-into-the-dependency
// value gets rebased by the dependency's load base). Matches image_fixup;
// requires ensureShims to have already run (see runImage in run.go) since
// an unresolved G^ target falls back to a SHIM$ symbol lookup.
func (c *Console) imageFixup(icb *ICB) error {
	if icb.FixupISD == nil {
		return nil
	}
	if icb.Flags&icbFixed != 0 {
		return nil
	}

	addr := icb.Base + (uint32(icb.FixupISD.VPN) << 9)
	iaf := icb.IAF

	if iaf.OffsetGFix != 0 {
		naddr := addr + iaf.OffsetGFix
		fixupCount, err := c.loadLong(naddr)
		if err != nil {
			return err
		}

		for fixupCount != 0 {
			naddr += 4
			imageID, err := c.loadLong(naddr)
			if err != nil {
				return err
			}

			shr := findSHRByID(icb, imageID)
			if shr == nil {
				fixupCount = 0
			} else {
				for n := uint32(0); n < fixupCount; n++ {
					naddr += 4
					offset, err := c.loadLong(naddr)
					if err != nil {
						return err
					}

					value, err := c.resolveFixupTarget(shr, offset)
					if err != nil {
						return err
					}

					if err := c.storeLong(naddr, value); err != nil {
						return err
					}
				}
			}

			naddr += 4
			if fixupCount, err = c.loadLong(naddr); err != nil {
				return err
			}
		}
	}

	if iaf.OffsetAddr != 0 {
		naddr := addr + iaf.OffsetAddr
		fixupCount, err := c.loadLong(naddr)
		if err != nil {
			return err
		}

		for fixupCount != 0 {
			naddr += 4
			imageID, err := c.loadLong(naddr)
			if err != nil {
				return err
			}
			shr := findSHRByID(icb, imageID)
			if shr == nil {
				fixupCount = 0
			} else {
				for n := uint32(0); n < fixupCount; n++ {
					naddr += 4
					offset, err := c.loadLong(naddr)
					if err != nil {
						return err
					}

					vaddr := icb.Base + offset
					target, err := c.loadLong(vaddr)
					if err != nil {
						return err
					}

					if err := c.storeLong(vaddr, target+shr.Base); err != nil {
						return err
					}
				}
			}

			naddr += 4
			if fixupCount, err = c.loadLong(naddr); err != nil {
				return err
			}
		}
	}

	icb.Flags |= icbFixed
	return nil
}

// findImage locates fn on the native filesystem, matching find_image: try
// it verbatim (adding a .exe suffix if it doesn't already have one), then
// with SharePrefix prepended, then lowercased -- stopping at the first
// candidate that exists. A literal ESC (0x1B) SharePrefix disables the
// prefixed attempt entirely, matching the C source's own check.
func (c *Console) findImage(fn string) (string, bool) {
	suffix := ""
	if !strings.HasSuffix(strings.ToLower(fn), ".exe") {
		suffix = ".exe"
	}

	candidates := []string{fn + suffix}
	if c.SharePrefix != "" && c.SharePrefix[0] != 0x1B {
		candidates = append(candidates, c.SharePrefix+fn+suffix)
	}
	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand, true
		}
		lower := strings.ToLower(cand)
		if _, err := os.Stat(lower); err == nil {
			return lower, true
		}
	}
	return "", false
}
