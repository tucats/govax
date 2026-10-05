package anl

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// kindsImage builds an image with one of each thing no fixture image
// has: a shareable image (IHD$K_LIM) with I/O counts and IHD$V_INISHR,
// two header blocks with the ISD list continued into the second, a patch
// block, a page fault cluster, fixed and PIC private and shareable
// sections, a system space section, global sections with each other
// match control, G^ lists for two images (one longer than a line), a
// .ADDRESS list, two protection changes, IAF$V_SHR, and an extra image.
func kindsImage() []byte {
	const (
		headerBlocks = 2
		fixupVBN     = 6
		fixupVA      = 0x800
	)

	b := make([]byte, fixupVBN*imageBlock)
	le := binary.LittleEndian

	// The fixed header and its blocks.
	const (
		iha = 0x30
		ihs = iha + ihaLength
		ihi = ihs + ihsLength
		ihp = ihi + ihiLength
		isd = ihp + ihpLength
	)

	le.PutUint16(b[ihdISDOffset:], isd)
	le.PutUint16(b[ihdActivOffset:], iha)
	le.PutUint16(b[ihdSymDbgOffset:], ihs)
	le.PutUint16(b[ihdImgIDOffset:], ihi)
	le.PutUint16(b[ihdPatchOffset:], ihp)
	copy(b[ihdMajorIDOffset:], "02")
	copy(b[ihdMinorIDOffset:], "05")
	b[ihdBlockCount] = headerBlocks
	b[ihdImageType] = 2
	le.PutUint16(b[ihdIOChannels:], 16)
	le.PutUint16(b[ihdIOPages:], 8)
	le.PutUint32(b[ihdLinkFlags:], 1<<3|ihdFlagINISHR|1<<7)
	le.PutUint32(b[ihdIAFVA:], fixupVA)

	le.PutUint32(b[iha+ihaInitShare:], 0x210)

	putCounted := func(p int, s string) {
		b[p] = byte(len(s))
		copy(b[p+1:], s)
	}

	putCounted(ihi, "KINDS")
	putCounted(ihi+40, "V2.0")
	le.PutUint64(b[ihi+56:], vmsdef.Time(time.Date(2026, time.October, 5, 8, 7, 6, 540_000_000, time.UTC)))
	putCounted(ihi+64, "V11-39")

	copy(b[ihp:ihp+ihpLength], "patched by hand, for the report")

	// The ISDs: three private ones in the first block, the rest in the
	// second.
	p := isd

	private := func(pages uint16, va uint32, pfc byte, flags uint32, vbn uint32) {
		le.PutUint16(b[p:], isdPrivateLength)
		le.PutUint16(b[p+2:], pages)
		le.PutUint32(b[p+4:], va>>9|uint32(pfc)<<24)
		le.PutUint32(b[p+8:], flags)
		le.PutUint32(b[p+12:], vbn)
		p += isdPrivateLength
	}

	demandZero := func(pages uint16, va uint32, flags uint32) {
		le.PutUint16(b[p:], isdDemandZeroLength)
		le.PutUint16(b[p+2:], pages)
		le.PutUint32(b[p+4:], va>>9)
		le.PutUint32(b[p+8:], flags|isdFlagDZRO)
		p += isdDemandZeroLength
	}

	global := func(match int, name string) {
		size := isdGlobalLength + 1 + len(name)
		le.PutUint16(b[p:], uint16(size))
		le.PutUint16(b[p+2:], 12)
		le.PutUint32(b[p+8:], isdFlagGBL|uint32(match)<<isdMatchBit|3<<24)
		le.PutUint32(b[p+16:], 2<<24|0x31)
		putCounted(p+isdNameOffset, name)
		p += size
	}

	private(2, 0x200, 8, 1<<1|1<<3|1<<7|2<<24, 3) // PRVFXD, CRF, WRT, LASTCLU
	private(1, 0x600, 0, 1<<24, 5)                // SHRFXD
	private(1, fixupVA, 0, isdFlagFIXUPVEC|4<<24, fixupVBN)

	le.PutUint16(b[p:], isdContinue)
	p = imageBlock

	demandZero(4, 0x80000000, 1<<3)
	demandZero(20, 0x7FFFD800, 1<<3|253<<24)
	global(0, "SHR_A_001")
	global(1, "SHR_B_001")
	global(3, "SHR_C_001")

	// The fixup section.
	f := (fixupVBN - 1) * imageBlock

	const (
		gfix = 0x40
		dot  = 0x70
		prt  = 0x88
		shl  = 0xA0
	)

	le.PutUint32(b[f+iafGFixOffset:], gfix)
	le.PutUint32(b[f+iafDotAddrOffset:], dot)
	le.PutUint32(b[f+iafChgPrtOffset:], prt)
	le.PutUint32(b[f+iafShlOffset:], shl)
	le.PutUint32(b[f+iafShrImgCount:], 3)
	le.PutUint32(b[f+iafShlExtra:], 1)
	le.PutUint32(b[f+iafFlags:], 1)

	longs := func(at int, values ...uint32) {
		for i, v := range values {
			le.PutUint32(b[f+at+4*i:], v)
		}
	}

	longs(gfix, 5, 1, 0x410, 0x478, 0x558, 0x7D0, 0x800, 1, 2, 0x20, 0)
	longs(dot, 2, 2, 0x10, 0x14, 0)
	longs(prt, 2, 0x600, 1|0xD<<16, 0x200, 2|0xF<<16)

	putCounted(f+shl+shlEntryLength+shlNameOffset, "SHR_A")
	putCounted(f+shl+2*shlEntryLength+shlNameOffset, "SHR_B")

	return b
}

// TestEveryImageKind analyzes kindsImage, which should have no errors,
// against testdata/imagekinds.txt: the layouts no real ANALYZE/IMAGE
// output settles, in one place for review (go test -update rewrites it).
func TestEveryImageKind(t *testing.T) {
	img, err := ReadImage(kindsImage())
	if err != nil {
		t.Fatal(err)
	}

	rep := AnalyzeImage(img, ImageOptions{})

	var b bytes.Buffer
	if err := WriteText(&b, rep.Lines); err != nil {
		t.Fatal(err)
	}

	if rep.Errors != 0 {
		t.Errorf("%d errors:\n%s", rep.Errors, b.String())
	}

	const golden = "testdata/imagekinds.txt"

	if *update {
		if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}

	if diff := firstDiff(string(want), b.String()); diff != "" {
		t.Errorf("%s: %s", golden, diff)
	}
}
