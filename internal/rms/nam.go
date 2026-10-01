package rms

import (
	"strings"

	"github.com/tucats/ods2/ondisk"
)

// This file is the NAM block (docs/PHASE-33.md): its field offsets, and
// reading and writing the fields name processing uses. Written from the
// RMS Reference Manual's chapter 5.

// NAM field offsets.
var (
	namBID  = vmsConst("NAM$B_BID")
	namBLN  = vmsConst("NAM$B_BLN")
	namRSS  = vmsConst("NAM$B_RSS")
	namRSL  = vmsConst("NAM$B_RSL")
	namRSA  = vmsConst("NAM$L_RSA")
	namNOP  = vmsConst("NAM$B_NOP")
	namESS  = vmsConst("NAM$B_ESS")
	namESL  = vmsConst("NAM$B_ESL")
	namESA  = vmsConst("NAM$L_ESA")
	namRLF  = vmsConst("NAM$L_RLF")
	namDVI  = vmsConst("NAM$T_DVI")
	namFID  = vmsConst("NAM$W_FID")
	namDID  = vmsConst("NAM$W_DID")
	namWCC  = vmsConst("NAM$L_WCC")
	namFNB  = vmsConst("NAM$L_FNB")
	namNODE = vmsConst("NAM$B_NODE")
	namLNOD = vmsConst("NAM$L_NODE")

	namBIDValue = byte(vmsConst("NAM$C_BID"))
	namBLNValue = byte(vmsConst("NAM$C_BLN"))
	namDVISize  = vmsConst("NAM$C_DVI")

	nopSynChk    = byte(vmsConst("NAM$M_SYNCHK"))
	nopNoConceal = byte(vmsConst("NAM$M_NOCONCEAL"))
)

// FAB fields name processing uses beyond fab.go's.
var (
	fabFOP = fabOffset("FOP")
	fabNAM = fabOffset("NAM")
	fabDNA = fabOffset("DNA")
	fabDNS = fabOffset("DNS")
	fabDEV = fabOffset("DEV")
	fabSDC = fabOffset("SDC")

	fopOFP = vmsConst("FAB$M_OFP")
	fopNAM = vmsConst("FAB$M_NAM")
	fopCIF = vmsConst("FAB$M_CIF")
)

// diskDevChar is FAB$L_DEV and FAB$L_SDC for a mounted Files-11 disk: a
// file-oriented, directory-structured, shareable, random-access device,
// available and mounted, that does input and output.
var diskDevChar = vmsConst("DEV$M_FOD") | vmsConst("DEV$M_DIR") | vmsConst("DEV$M_SHR") |
	vmsConst("DEV$M_AVL") | vmsConst("DEV$M_MNT") | vmsConst("DEV$M_IDV") |
	vmsConst("DEV$M_ODV") | vmsConst("DEV$M_RND")

// checkNAM confirms nam is a NAM block: NAM$B_BID says so and NAM$B_BLN
// is long enough. It returns RMS$_NAM if not. err is a VAX memory access
// failure.
func checkNAM(ctx *Context, nam uint32) (uint32, error) {
	bid, err := ctx.loadByte(nam + namBID)
	if err != nil {
		return 0, err
	}

	bln, err := ctx.loadByte(nam + namBLN)
	if err != nil {
		return 0, err
	}

	if bid != namBIDValue || bln < namBLNValue {
		return rmsInvalidNAM, nil
	}

	return 0, nil
}

// fabNames reads the name processing inputs from a FAB and its NAM (nam
// is 0 when the FAB has none): the file and default names, the related
// file's resultant string, FAB$V_OFP, and NAM$V_NOCONCEAL.
func fabNames(ctx *Context, fab, nam uint32) (nameInputs, error) {
	var in nameInputs

	fn, err := loadFileSpecString(ctx, fab)
	if err != nil {
		return in, err
	}

	dns, err := ctx.loadByte(fab + fabDNS)
	if err != nil {
		return in, err
	}

	dna, err := ctx.loadLongword(fab + fabDNA)
	if err != nil {
		return in, err
	}

	dn := ""
	if dns > 0 {
		if dn, err = ctx.loadFixedString(dna, int(dns)); err != nil {
			return in, err
		}
	}

	fop, err := ctx.loadLongword(fab + fabFOP)
	if err != nil {
		return in, err
	}

	in = nameInputs{Primary: fn, Default: dn, OFP: fop&fopOFP != 0}

	if nam == 0 {
		return in, nil
	}

	nop, err := ctx.loadByte(nam + namNOP)
	if err != nil {
		return in, err
	}

	in.NoConceal = nop&nopNoConceal != 0

	rlf, err := ctx.loadLongword(nam + namRLF)
	if err != nil || rlf == 0 {
		return in, err
	}

	rsl, err := ctx.loadByte(rlf + namRSL)
	if err != nil {
		return in, err
	}

	rsa, err := ctx.loadLongword(rlf + namRSA)
	if err != nil {
		return in, err
	}

	if rsl > 0 {
		in.Related, err = ctx.loadFixedString(rsa, int(rsl))
	}

	return in, err
}

// fabNAM returns the FAB's NAM block address, 0 if it has none, or
// RMS$_NAM if the block there isn't a NAM.
func fabNAMBlock(ctx *Context, fab uint32) (uint32, uint32, error) {
	nam, err := ctx.loadLongword(fab + fabNAM)
	if err != nil || nam == 0 {
		return 0, 0, err
	}

	sts, err := checkNAM(ctx, nam)

	return nam, sts, err
}

// storeString writes s to buf, at most size bytes, returning RMS$_ESS-
// style failure (overflow true) when it doesn't fit. A zero buf or size
// writes nothing.
func (ctx *Context) storeString(buf uint32, size byte, s string) (overflow bool, err error) {
	if buf == 0 || size == 0 {
		return false, nil
	}

	if len(s) > int(size) {
		return true, nil
	}

	for i := range len(s) {
		if err := ctx.storeByte(buf+uint32(i), s[i]); err != nil {
			return false, err
		}
	}

	return false, nil
}

// storeFid writes a file ID at addr in its NAM layout: number, sequence,
// RVN, and the number's high byte.
func (ctx *Context) storeFid(addr uint32, f ondisk.Fid) error {
	if err := ctx.storeWord(addr, f.Num); err != nil {
		return err
	}

	if err := ctx.storeWord(addr+2, f.Seq); err != nil {
		return err
	}

	if err := ctx.storeByte(addr+4, f.Rvn); err != nil {
		return err
	}

	return ctx.storeByte(addr+5, f.Nmx)
}

// deviceID is NAM$T_DVI's text for a device: its physical name, with
// "_" and ":".
func deviceID(device string) string {
	return "_" + strings.ToUpper(device) + ":"
}

// storeDVI writes NAM$T_DVI: a counted string, zero-filled.
func (ctx *Context) storeDVI(nam uint32, dvi string) error {
	for i := range namDVISize {
		var c byte

		switch {
		case i == 0 && dvi != "":
			c = byte(len(dvi))
		case i > 0 && int(i) <= len(dvi):
			c = dvi[i-1]
		}

		if err := ctx.storeByte(nam+namDVI+i, c); err != nil {
			return err
		}
	}

	return nil
}

// storeComponents writes the NAM's component pointers and lengths (node,
// device, directory, name, type, version) for p's fields laid out at buf.
func (ctx *Context) storeComponents(nam, buf uint32, p fileName) error {
	fields := []string{"", p.Dev, p.Dir, p.Name, p.Type, p.Ver}
	at := buf

	for i, f := range fields {
		if err := ctx.storeByte(nam+namNODE+uint32(i), byte(len(f))); err != nil {
			return err
		}

		if err := ctx.storeLongword(nam+namLNOD+uint32(4*i), at); err != nil {
			return err
		}

		at += uint32(len(f))
	}

	return nil
}

// resolveFAB is resolveFileSpec for a FAB's file name: name processing
// (name.go) with the FAB's default name and, when it has a NAM, the
// related file. It returns the names in search order, or the RMS status
// to fail with. err is a VAX memory access failure.
func (ctx *Context) resolveFAB(fab uint32) ([]resolvedSpec, uint32, error) {
	nam, sts, err := fabNAMBlock(ctx, fab)
	if err != nil || sts != 0 {
		return nil, sts, err
	}

	in, err := fabNames(ctx, fab, nam)
	if err != nil {
		return nil, 0, err
	}

	names, sts := ctx.expandName(in)
	if sts != 0 {
		return nil, sts, nil
	}

	out := make([]resolvedSpec, len(names))
	for i, p := range names {
		out[i] = resolvedSpec{Spec: p.spec(), Display: p.Lookup}
	}

	return out, 0, nil
}
