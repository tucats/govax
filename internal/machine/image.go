package machine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// This handles image_load/image_fixup and the IHD/IHI/ISD/IAF struct definitions:
// read each struct's fields directly at their documented byte
// offsets -- matching the precedent already set by internal/rms's own port
// of FAB/RAB field access (docs/PHASE-22.md), which has the same
// "declarative offset table in C, direct typed reads in Go" relationship.

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
	isdWRT      = 1 << 3
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
	Pages uint16
	VPN   uint32
	// PFC is the section's page fault cluster, the top byte of the ISD's
	// VPN longword (0 for the default, PFCDEFAULT). Only a mapped image
	// (imageMap) reads it; the copying loader's ISDs leave it 0.
	PFC       byte
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
	Name string
	// File is the file the image was loaded from, as RUN or the shareable
	// image search named it. Name is "<MAIN>" for the main image, so File
	// is what the debugger's SHOW IMAGE names it by.
	File     string
	Base     uint32
	End      uint32
	Flags    uint32
	Transfer [4]uint32
	ISDList  []*ISD
	FixupISD *ISD
	IAF      IAF
	SHRList  []*SHR

	// Debug is the image's debug symbol table (Phase 41), read from the
	// image file when it's loaded and relocated by Base: its modules,
	// routines, symbols, and line numbers. Nil for an image linked
	// /NOTRACEBACK, which has none, or when it couldn't be read; then
	// DebugErr says why (nil when there's simply no table).
	Debug    *dbgsym.Program
	DebugErr error

	// Mapped is set for an image mapped from its file (imageMap, with
	// paging on), clear for one copied into memory.
	Mapped bool

	// Spec is where the image's file was found: its full specification
	// on a volume, or its host path. The main image's becomes its
	// process's JPI$_IMAGNAME (ActivateImage).
	Spec imageSpec

	// Ident and Match are a shareable image's global section ident and
	// match control, from its header (IHD$L_IDENT and the top byte of
	// IHD$L_LNKFLAGS), which the images naming it were linked against
	// (identMatches).
	Ident uint32
	Match int

	// Type is the header's image type, IHD$K_EXE or IHD$K_LIM; 0 when
	// the header couldn't be decoded.
	Type byte

	// LinkDebug is the image header's IHD$V_LNKDEBUG: the image was
	// linked /DEBUG, so RUN starts the debugger on it unless told not to.
	LinkDebug bool

	// LegacyStartup is set when the first transfer address is a VMS V1
	// image's image startup stub, a jump to SYS$IMGSTA's entry in the
	// old system vector (legacyStartupStub): the image was linked with
	// traceback, and its main routine is the next transfer address.
	LegacyStartup bool

	// Protections are the fixup section's protection changes (each a
	// run of pages from an address relative to the image's lowest
	// section, imageLow, and a PRT$C_ code), applied after the fixups
	// as VMS's image activator applies them (setImageProtection).
	Protections []vmsimage.Protection

	// Cells are the image's G^ fixup cells, by address, once imageFixup
	// has filled them: what shareable image and transfer-vector offset
	// each one's G^ reference asked for (Phase 41). The disassembler names
	// a reference through one (shared.go).
	Cells map[uint32]fixupCell
}

// ResetICBList discards every loaded image and resets the P0 high-water
// mark, matching reset_icb_list. Unlike the C source, there is no explicit
// per-ISD/per-ICB freemem to port -- Go's GC reclaims them once ICBList is
// replaced.
func (p *ImageProcess) ResetICBList() {
	for _, icb := range p.ICBList {
		p.setImageProtection(icb, false)
	}

	p.ICBList = nil

	p.process().DeleteP0() // what a rundown under the debugger left

	p.process().RegionSize[0] = 0
}

// FindMainICB returns the ICB flagged ICB_MAIN, or nil, matching
// find_main_icb.
func (p *ImageProcess) FindMainICB() *ICB {
	for _, icb := range p.ICBList {
		if icb.Flags&icbMain != 0 {
			return icb
		}
	}

	return nil
}

// storeBytes writes data starting at virtual address addr, via translation
// forced into kernel mode for the duration (see withKernelMode) regardless
// of whatever mode the CPU was last left in -- this is console-driven
// memory access (ASM's own code deposit, SHIM$ stub synthesis), not an
// access made by a running program, so it must not be gated by that
// program's own PSL. It goes through the current process's mapping; image
// activation, which may be for another process, uses ImageProcess's own
// accessors (images.go) instead.
func (m *Machine) storeBytes(addr uint32, data []byte) error {
	return m.withKernelMode(func() error {
		for i, b := range data {
			if err := m.Mem.StoreByte(m.CPU, addr+uint32(i), b); err != nil {
				return err
			}
		}

		return nil
	})
}

func (m *Machine) loadByte(addr uint32) (byte, error) {
	var v byte

	err := m.withKernelMode(func() (err error) {
		v, err = m.Mem.LoadByte(m.CPU, addr)

		return err
	})

	return v, err
}

func (m *Machine) loadLong(addr uint32) (uint32, error) {
	var v uint32

	err := m.withKernelMode(func() (err error) {
		v, err = m.Mem.LoadLongword(m.CPU, addr)

		return err
	})

	return v, err
}

func (m *Machine) storeLong(addr, v uint32) error {
	return m.withKernelMode(func() error {
		return m.Mem.StoreLongword(m.CPU, addr, v)
	})
}

// readIHDTransferOffset/readIHDIdentOffset read the two IHD fields
// for control flow (init_ihd_maps's OFFSET_TRANSFER @2, OFFSET_IDENT @6);
// HEADER_BLOCKS @16 is read directly from the host-side file buffer instead
// (see imageLoad), since it's needed before any of the header has been written into VAX
// memory at all. The rest of the IHD (size, offset_dst/patch, major/minor
// id, mask, channels, io_pages, flags, section_id, version) and the IHI's
// own image-name field only ever feed this phase's C source's own debug
// printing, with no consumer in this port.
func (p *ImageProcess) readIHDTransferOffset(base uint32) (uint32, error) {
	v, err := p.loadWord(base + 2)

	return uint32(v), err
}

func (p *ImageProcess) readIHDIdentOffset(base uint32) (uint32, error) {
	v, err := p.loadWord(base + 6)

	return uint32(v), err
}

// ihiSize is IHI_SIZE from imgdef.h: the fixed size of the Image
// Identification block, used to locate the first ISD immediately after it.
const ihiSize = 80

// readISD reads one Image Section Descriptor at addr, returning it along
// with its own declared on-disk size (image_load's own loop advances by
// this, not sizeof(ISD), since ISD records are variable-length).
func (p *ImageProcess) readISD(addr uint32) (*ISD, uint32, error) {
	size, err := p.loadWord(addr)
	if err != nil {
		return nil, 0, err
	}

	if size == 0 {
		return nil, 0, nil
	}

	pages, err := p.loadWord(addr + 2)
	if err != nil {
		return nil, 0, err
	}

	vpn, err := p.loadWord(addr + 4)
	if err != nil {
		return nil, 0, err
	}

	flags, err := p.loadLong(addr + 8)
	if err != nil {
		return nil, 0, err
	}

	vbn, err := p.loadLong(addr + 12)
	if err != nil {
		return nil, 0, err
	}

	sectionID, err := p.loadLong(addr + 16)
	if err != nil {
		return nil, 0, err
	}

	count, err := p.loadByte(addr + 20)
	if err != nil {
		return nil, 0, err
	}

	name := "<NONE>"

	if count >= 1 && count <= 39 {
		buf := make([]byte, count)
		for i := range buf {
			b, err := p.loadByte(addr + 21 + uint32(i))
			if err != nil {
				return nil, 0, err
			}

			buf[i] = b
		}

		name = string(buf)
	}

	return &ISD{Pages: pages, VPN: uint32(vpn), Flags: flags, VBN: vbn, SectionID: sectionID, Name: name}, uint32(size), nil
}

// readIAF reads the fixed part of an Image Attribute/Fixup header at addr,
// matching init_ihd_maps's IAF field table.
func (p *ImageProcess) readIAF(addr uint32) (IAF, error) {
	var (
		iaf IAF
		err error
	)

	if iaf.OffsetGFix, err = p.loadLong(addr + 0x0C); err != nil {
		return iaf, err
	}

	if iaf.OffsetAddr, err = p.loadLong(addr + 0x10); err != nil {
		return iaf, err
	}

	if iaf.OffsetChgprot, err = p.loadLong(addr + 0x14); err != nil {
		return iaf, err
	}

	if iaf.OffsetShl, err = p.loadLong(addr + 0x18); err != nil {
		return iaf, err
	}

	if iaf.ShrImgCnt, err = p.loadLong(addr + 0x1C); err != nil {
		return iaf, err
	}

	return iaf, nil
}

// imageLoad loads fn as a VMS image, matching image_load: an already-loaded
// image of the same name is a no-op success (returning the existing ICB);
// otherwise the file is opened, its header parsed, each section loaded or
// zero-filled into P0 space starting at the current high-water mark
// (p.process().RegionSize[0]), and -- if the image carries a FIXUPVEC section --
// its sharable-image dependency list is built and recursively loaded. A
// secondary image is found by the shareable image rules (findShareable,
// shareable.go): one govax's shims stand for isn't loaded, and one that
// isn't found stops the activation.
func (p *ImageProcess) imageLoad(fn string, flag uint32) (*ICB, error) {
	return p.loadImage(fn, flag, nil)
}

// loadImage is imageLoad, for a shareable image the image naming it maps
// through linked, its first global section ISD (nil for the main image),
// which says where it goes when the link placed it (imageBase).
func (p *ImageProcess) loadImage(fn string, flag uint32, linked *ISD) (*ICB, error) {
	for _, icb := range p.ICBList {
		if icb.Name == fn {
			return icb, nil
		}
	}

	if p.mapsImages() {
		return p.imageMap(fn, flag, linked)
	}

	var (
		data []byte
		spec imageSpec
		err  error
	)

	if flag == icbMain {
		data, spec, err = p.m.readImage(fn, true)
	} else {
		var loc rms.FileLocation
		if loc, err = p.findShareable(fn); err == nil {
			data, spec, err = p.readShareable(loc)
		}
	}

	if err != nil {
		return nil, err
	}

	if len(data) < 512 {
		return nil, vmserrors.New(vmserrors.RMS_SHORTHEADER, fn)
	}

	nblocks := data[16]
	if nblocks == 0 {
		nblocks = 1
	}

	at, err := p.imageBase(data, flag, linked)
	if err != nil {
		return nil, err
	}

	// The C source loads the header one page above the current high-water
	// mark (page zero of P0 can't be written on VMS -- it's the guard
	// page VMInit installs, see vminit.go), then treats everything from
	// there on as belonging to the image itself.
	base := p.process().RegionSize[0] + 0x200

	headerLen := int(nblocks) * 512
	if headerLen > len(data) {
		headerLen = len(data)
	}

	if err := p.storeBytes(base, data[:headerLen]); err != nil {
		return nil, err
	}

	icb := p.newICB(fn, flag)
	icb.Base = at
	icb.Spec = spec

	transferOffset, err := p.readIHDTransferOffset(base)
	if err != nil {
		return nil, err
	}

	identOffset, err := p.readIHDIdentOffset(base)
	if err != nil {
		return nil, err
	}

	for n := uint32(0); n < 4; n++ {
		v, err := p.loadLong(base + transferOffset + n*4)
		if err != nil {
			return nil, err
		}

		p.setTransfer(icb, n, v)
	}

	isdAddr := base + identOffset + ihiSize
	for i := 0; i < 1000; i++ {
		isd, size, err := p.readISD(isdAddr)
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
				if addr >= p.process().RegionSize[0] {
					p.process().RegionSize[0] = addr + 512
				}

				if addr+0x1FF > icb.End {
					icb.End = addr + 0x1FF
				}

				if err := p.storeBytes(addr, make([]byte, 512)); err != nil {
					return nil, err
				}
			}

			continue
		}

		if err := p.loadSection(icb, isd, data); err != nil {
			return nil, err
		}
	}

	return icb, p.finishLoad(icb, data)
}

// newICB starts the ICB of fn, an image being loaded with flag (icbMain
// or icbSecondary) at P0's high-water mark, and adds it to the list.
func (p *ImageProcess) newICB(fn string, flag uint32) *ICB {
	icb := &ICB{Base: p.process().RegionSize[0], Flags: flag | icbIncomplete, File: fn}
	if flag == icbMain {
		icb.Name = "<MAIN>"
	} else {
		icb.Name = fn
	}

	p.ICBList = append(p.ICBList, icb)

	return icb
}

// setTransfer records transfer address n of icb, v as the header gives
// it (relative to the image's base; 0 for none), and defines its symbol:
// MAIN for the main image's first, SHARE$name_INITIALIZE and
// SHARE$name_TRANSFER_n for a shareable image's. A shareable image's
// address outside P0 is the system's, not the image's: SYS$IMGSTA, which
// LINK makes a shareable image's first with traceback (testdata/link68's
// SHR1), and which isn't an initialization routine to call. It is kept
// as it is, with no symbol.
func (p *ImageProcess) setTransfer(icb *ICB, n, v uint32) {
	if icb.Flags&icbSecondary != 0 && v >= p1Start {
		icb.Transfer[n] = v

		return
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

		p.setSymbol(sname, v)
	}

	icb.Transfer[n] = v
}

// extend moves P0's high-water mark and icb's end past isd's pages.
func (p *ImageProcess) extend(icb *ICB, isd *ISD) {
	if isd.Pages == 0 {
		return
	}

	last := icb.Base + (isd.VPN+uint32(isd.Pages))<<9

	env := p.process()
	env.RegionSize[0] = max(env.RegionSize[0], last)
	icb.End = max(icb.End, last-1)
}

// finishLoad is the end of loading icb, whose sections are in place,
// from the image file's contents data: its debug symbols and protection
// changes, and, if it has a fixup section, its shareable image list,
// each of which is loaded in turn.
func (p *ImageProcess) finishLoad(icb *ICB, data []byte) error {
	icb.Flags &^= icbIncomplete

	img, err := vmsimage.ReadImage(data)
	if err == nil {
		icb.Ident, icb.Match, icb.Type = img.Ident, int(img.LinkFlags>>24&7), img.Type
		icb.LegacyStartup = img.Legacy() && legacyStartupStub(img, data, icb.Transfer[0]-icb.Base)

		if img.Fixups != nil {
			icb.Protections = img.Fixups.Protections
		}
	}

	icb.Debug, icb.LinkDebug, icb.DebugErr = readDebugSymbols(img, err, data, icb.Base)

	if icb.FixupISD == nil {
		return nil
	}

	if err := p.readSharedImages(icb); err != nil {
		return err
	}

	// The images are activated from the list's end: VMS 7.1 put MAIN1's
	// LIBRTL, last in its list, right after MAIN1, then SHR1
	// (testdata/link68's R1; unconfirmed for two images of the user's).
	for i := len(icb.SHRList) - 1; i >= 0; i-- {
		shr := icb.SHRList[i]
		if shr.ID == 0 {
			continue
		}

		linked := linkedSection(icb, shr.Name)

		// An image the shims stand for has no file (but may have pages
		// the link placed it on); one that isn't found stops the
		// activation (Decision 4).
		dep, err := p.loadImage(shr.Name, icbSecondary, linked)
		if errors.Is(err, errEmulated) {
			if err := p.placeEmulated(icb, shr.Name); err != nil {
				return err
			}

			continue
		}

		if err != nil {
			return err
		}

		if linked != nil && !identMatches(linked, dep) {
			return &IdentMismatchError{Image: dep.Name, File: dep.Spec.Name}
		}

		shr.Icb = dep
		shr.Base = dep.Base
	}

	return nil
}

// readSharedImages reads icb's fixup section header (IAF) and its
// shareable image list from the image's memory, into icb.IAF and
// icb.SHRList (the image itself first, as image 0).
func (p *ImageProcess) readSharedImages(icb *ICB) error {
	addr := icb.Base + (uint32(icb.FixupISD.VPN) << 9)

	iaf, err := p.readIAF(addr)
	if err != nil {
		return err
	}

	namesOffset, err := p.loadLong(addr + iaf.OffsetShl + 16)
	if err != nil {
		return err
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

		nameLen, err := p.loadByte(nameAddr)
		if err != nil {
			return err
		}

		buf := make([]byte, nameLen)

		for i := range buf {
			b, err := p.loadByte(nameAddr + 1 + uint32(i))
			if err != nil {
				return err
			}

			buf[i] = b
		}

		imageID++
		icb.SHRList = append(icb.SHRList, &SHR{Name: string(buf), ID: imageID})
	}

	return nil
}

// readDebugSymbols reads the debug symbol table of the image file data,
// loaded at base (see dbgsym.Read), whose decoding is img (or the error
// err that vmsimage.ReadImage gave). An image without one gives nil and no
// error. A table that can't be read doesn't stop the image from loading
// or running, since only the disassembler's names depend on it, so the
// error is returned for SHOW IMAGES/FULL to report rather than for
// imageLoad to fail on. It also reports whether the image header says the
// image was linked /DEBUG (IHD$V_LNKDEBUG).
func readDebugSymbols(img *vmsimage.Image, err error, data []byte, base uint32) (*dbgsym.Program, bool, error) {
	if err != nil {
		return nil, false, err
	}

	linkDebug := img.LinkFlags&vmsimage.IHDFlagLNKDEBUG != 0

	prog, err := dbgsym.Read(img, data, base)
	if errors.Is(err, dbgsym.ErrNoDST) {
		return nil, linkDebug, nil
	}

	return prog, linkDebug, err
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
func (p *ImageProcess) findLoadedICB(name string) *ICB {
	for _, icb := range p.ICBList {
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
func (p *ImageProcess) resolveFixupTarget(shr *SHR, offset uint32) (uint32, error) {
	if dep := p.findLoadedICB(shr.Name); dep != nil {
		return dep.Base + offset, nil
	}

	name := fmt.Sprintf("SHIM$%s_%08X", shr.Name, offset)

	v, ok := p.m.Symbols.Get(name)
	if !ok {
		return 0, vmserrors.New(vmserrors.LIB_UNRESOLVED, name)
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
func (p *ImageProcess) imageFixup(icb *ICB) error {
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

		fixupCount, err := p.loadLong(naddr)
		if err != nil {
			return err
		}

		for fixupCount != 0 {
			naddr += 4

			imageID, err := p.loadLong(naddr)
			if err != nil {
				return err
			}

			shr := findSHRByID(icb, imageID)
			if shr == nil {
				fixupCount = 0 //nolint:ineffassign // holdover from C code port
			} else {
				for n := uint32(0); n < fixupCount; n++ {
					naddr += 4

					offset, err := p.loadLong(naddr)
					if err != nil {
						return err
					}

					value, err := p.resolveFixupTarget(shr, offset)
					if err != nil {
						return err
					}

					if err := p.storeLong(naddr, value); err != nil {
						return err
					}

					icb.addCell(naddr, shr.Name, offset)
				}
			}

			naddr += 4
			if fixupCount, err = p.loadLong(naddr); err != nil {
				return err
			}
		}
	}

	if iaf.OffsetAddr != 0 {
		naddr := addr + iaf.OffsetAddr

		fixupCount, err := p.loadLong(naddr)
		if err != nil {
			return err
		}

		for fixupCount != 0 {
			naddr += 4

			imageID, err := p.loadLong(naddr)
			if err != nil {
				return err
			}

			shr := findSHRByID(icb, imageID)
			if shr == nil {
				fixupCount = 0 //nolint:ineffassign // holdover from C code port
			} else {
				for n := uint32(0); n < fixupCount; n++ {
					naddr += 4

					offset, err := p.loadLong(naddr)
					if err != nil {
						return err
					}

					// The longword's address is relative to the image's
					// base, its lowest section: ^X200 in an executable
					// (ANALYZE/IMAGE: "relative to %X'00000200'"). Its
					// offset in the shareable image resolves as a G^
					// cell's does, to the loaded image or else its shim.
					// eVAX added the offset to the load base alone and
					// rebased by the image's base, so a real image's
					// .ADDRESS failed (docs/DEVIATIONS.md).
					vaddr := imageLow(icb) + offset

					target, err := p.loadLong(vaddr)
					if err != nil {
						return err
					}

					value, err := p.resolveFixupTarget(shr, target)
					if err != nil {
						return err
					}

					if err := p.storeLong(vaddr, value); err != nil {
						return err
					}
				}
			}

			naddr += 4
			if fixupCount, err = p.loadLong(naddr); err != nil {
				return err
			}
		}
	}

	icb.Flags |= icbFixed

	return nil
}

// ReadImage reads the image file fn. The main image (main) is found by the
// file name rules MACRO and LINK follow (rms.Session.Locate): a VMS
// specification, or a bare name while the default directory is on a
// volume, is a file on a mounted volume, with the type EXE if it has none;
// anything else, or any name with /HOST, is a host file, found as
// findImage finds it. A shareable image (main false) is found as the
// image activator finds it (ImageLocator): an image govax's shims stand
// for has no file to read (errEmulated).
func (m *Machine) ReadImage(fn string, main bool) ([]byte, error) {
	data, _, err := m.readImage(fn, main)

	return data, err
}

// readImage is ReadImage, also returning where the image was found
// (imageSpec).
func (m *Machine) readImage(fn string, main bool) ([]byte, imageSpec, error) {
	if !main {
		p := m.images()

		loc, err := p.findShareable(fn)
		if err != nil {
			return nil, imageSpec{}, err
		}

		return p.readShareable(loc)
	}

	if s := m.Hooks.Session(); main && s != nil {
		loc, err := s.Locate(fn, m.runHost)
		if err != nil {
			return nil, imageSpec{}, err
		}

		if !loc.Host {
			data, found, err := s.ReadRawFile(loc.WithDefaultType("EXE"))

			var notFound *rms.NotFoundError
			if errors.As(err, &notFound) {
				return nil, imageSpec{}, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND, fn)
			}

			return data, imageSpec{Name: found.Physical}, err
		}
	}

	path, ok := m.findImage(fn)
	if !ok {
		return nil, imageSpec{}, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND, fn)
	}

	data, err := m.Paths.ReadFile(path)

	return data, hostImageSpec(path), err
}

// imageSpec is where an image file was found: the full specification of
// a volume file with its physical device (rms.FileLocation.Physical), or
// the absolute path of a host file (Host).
type imageSpec struct {
	Name string
	Host bool
}

// hostImageSpec is the imageSpec of the host file at path.
func hostImageSpec(path string) imageSpec {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	return imageSpec{Name: path, Host: true}
}

// loadSection copies a section of an image, isd, from the image file's
// contents, data, into its pages, moving P0's high-water mark and the
// image's end past them. This is the copying loader, which runs only when
// images aren't mapped (paging off, or before VMINIT turns memory
// mapping on), so nothing it writes is charged against the paging file
// quota.
func (p *ImageProcess) loadSection(icb *ICB, isd *ISD, data []byte) error {
	fileAddr := int64(isd.VBN-1) * 512

	for n := uint32(0); n < uint32(isd.Pages); n++ {
		addr := icb.Base + ((n + uint32(isd.VPN)) << 9)
		if addr >= p.process().RegionSize[0] {
			p.process().RegionSize[0] = addr + 512
		}

		if addr+0x1FF > icb.End {
			icb.End = addr + 0x1FF
		}

		block := make([]byte, 512)

		off := fileAddr + int64(n)*512
		if off >= 0 && off < int64(len(data)) {
			copy(block, data[off:])
		}

		if err := p.storeBytes(addr, block); err != nil {
			return err
		}
	}

	return nil
}

// imageProtection is the page protection VMS's image activator gives a
// process-private image section: UW (user write) for a writable section,
// UR (read-only to every mode, kernel included) for the rest. VAX pages
// have no execute protection, so EXE plays no part.
func imageProtection(isd *ISD) vm.Protection {
	if isd.Flags&isdWRT != 0 {
		return vm.ProtUW
	}

	return vm.ProtUR
}

// setImageProtection sets the protection of every page icb's sections
// occupy: each section's own (imageProtection) when protect is true, as
// RUN does once the image is loaded and fixed up, then the protection
// changes the fixup section lists (the fixup section's own page UREW,
// user-read and executive-write, so a program can't overwrite its G^
// cells), as VMS's image activator applies them after its fixups; or UW
// when it's false, as ResetICBList does so the next RUN can load over the
// old image's read-only pages. Every page that exists is set, whatever
// its PTE's form: valid, in transition, still mapped to its section in
// the image file, demand-zero, or in the page file (a mapped image's
// pages are mostly not yet valid; docs/PHASE-66, subtask 3). A page
// deleted since, or outside the page tables (VMINIT shrank P0 since), is
// skipped. The C source never sets image page protections, so every page
// stayed at P0's default (UW); see docs/DEVIATIONS.md.
func (p *ImageProcess) setImageProtection(icb *ICB, protect bool) {
	if p.m.CPU == nil || p.m.Mem == nil || p.m.CPU.PR(registers.MAPEN) == 0 {
		return
	}

	for _, isd := range icb.ISDList {
		if isdType(isd.Flags) == isdUsrStack || isd.Flags&isdGBL != 0 {
			continue
		}

		prot := vm.ProtUW
		if protect {
			prot = imageProtection(isd)
		}

		p.protectPages(icb.Base+isd.VPN<<9, uint32(isd.Pages), prot)
	}

	if !protect {
		return
	}

	low := imageLow(icb)

	for _, c := range icb.Protections {
		p.protectPages(low+c.Address, uint32(c.Pages), vm.Protection(c.Code&0xF))
	}
}

// protectPages gives the pages pages from addr protection prot, each that
// exists (setImageProtection).
func (p *ImageProcess) protectPages(addr, pages uint32, prot vm.Protection) {
	space := p.space()

	for n := range pages {
		at := addr&^0x1FF + n<<9

		pte, err := p.m.Mem.LookupPTEIn(p.m.CPU, space, at)
		if err != nil || !pageExists(pte) || pte.Protection() == prot {
			continue
		}

		pte.SetProtection(prot)
		_ = p.m.Mem.StorePTEIn(p.m.CPU, space, at, pte)
	}
}

// pageExists reports whether pte describes a page of the address space:
// one that is valid or has some protection. A deleted page (the all-zero
// PTE) has neither.
func pageExists(pte vm.PTE) bool {
	return pte.Valid() || pte.Protection() != vm.ProtNA
}

// imageLow is an image's base for its fixup lists: the address of its
// lowest section (the user stack and global sections aside), which is
// ^X200 for an executable and the load base for a shareable image.
func imageLow(icb *ICB) uint32 {
	low, found := uint32(0), false

	for _, isd := range icb.ISDList {
		if isdType(isd.Flags) == isdUsrStack || isd.Flags&isdGBL != 0 {
			continue
		}

		if vpn := uint32(isd.VPN); !found || vpn < low {
			low, found = vpn, true
		}
	}

	return icb.Base + low<<9
}

// findImage locates the main image fn on the host, matching find_image: try
// it verbatim (adding a .exe suffix if it doesn't already have one), then
// with SharePrefix prepended, then lowercased -- stopping at the first
// candidate that exists. A literal ESC (0x1B) SharePrefix disables the
// prefixed attempt entirely, matching the C source's own check.
func (m *Machine) findImage(fn string) (string, bool) {
	suffix := ""
	if !strings.HasSuffix(strings.ToLower(fn), ".exe") {
		suffix = ".exe"
	}

	candidates := []string{fn + suffix}
	if m.SharePrefix != "" && m.SharePrefix[0] != 0x1B {
		candidates = append(candidates, m.SharePrefix+fn+suffix)
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
