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
	fopCTG = vmsConst("FAB$M_CTG")
	fopCBT = vmsConst("FAB$M_CBT")
)

// diskDevChar is FAB$L_DEV and FAB$L_SDC for a mounted Files-11 disk, as
// VMS 7.3 reports an RA disk (the oracle's ^X1CCD4108): file-oriented,
// directory-structured, shareable, random-access, available and mounted,
// doing input and output, with a revector cache table (RCT), and
// error-logged (ELG) and allocated (ALL).
var diskDevChar = vmsConst("DEV$M_FOD") | vmsConst("DEV$M_DIR") | vmsConst("DEV$M_SHR") |
	vmsConst("DEV$M_AVL") | vmsConst("DEV$M_MNT") | vmsConst("DEV$M_IDV") |
	vmsConst("DEV$M_ODV") | vmsConst("DEV$M_RND") | vmsConst("DEV$M_RCT") |
	vmsConst("DEV$M_ELG") | vmsConst("DEV$M_ALL")

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

// deviceID is NAM$T_DVI's text for a device: its full name, with the
// node's name and no ":" ("_SIMVAX$DUA1", as VMS 7.3 writes it).
func (ctx *Context) deviceID(device string) string {
	return "_" + ctx.NodeName + "$" + strings.ToUpper(device)
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

// namNameFields are the offsets of NAM$B_NODE through NAM$L_VER: the six
// component lengths, then the six pointers.
var namNameFields = struct{ start, end uint32 }{namNODE, vmsConst("NAM$L_VER") + 4}

// clearNAMOutputs clears what name processing writes to a NAM, as VMS
// does before a $PARSE or $OPEN: ESL, RSL, FID, DID, DVI, WCC, FNB, and
// the component lengths and pointers.
func (ctx *Context) clearNAMOutputs(nam uint32) error {
	for _, f := range []struct{ off, size uint32 }{
		{namESL, 1}, {namRSL, 1}, {namFID, 6}, {namDID, 6}, {namDVI, namDVISize},
		{namWCC, 4}, {namFNB, 4}, {namNameFields.start, namNameFields.end - namNameFields.start},
	} {
		for i := range f.size {
			if err := ctx.storeByte(nam+f.off+i, 0); err != nil {
				return err
			}
		}
	}

	return nil
}

// storeNameString writes f's text to a NAM string buffer (buf, of size
// bytes) with its length at lenOff, and points the NAM's components into
// it. A zero buf or size writes nothing. When the text doesn't fit,
// overflow is true, and the buffer is left as VMS leaves it (the oracle's
// PARSE cases 9 and 28, OPEN case 10): the length is the buffer's size,
// as much of the text as fits is written when the device name fits, and
// only the device's component is set.
func (ctx *Context) storeNameString(nam, buf uint32, size byte, lenOff uint32, f fileName) (overflow bool, err error) {
	if buf == 0 || size == 0 {
		return false, nil
	}

	text := f.Dev + f.Dir + f.Name + f.Type + f.Ver

	if len(text) <= int(size) {
		if _, err := ctx.storeString(buf, size, text); err != nil {
			return false, err
		}

		if err := ctx.storeByte(nam+lenOff, byte(len(text))); err != nil {
			return false, err
		}

		return false, ctx.storeComponents(nam, buf, f)
	}

	if err := ctx.storeByte(nam+lenOff, size); err != nil {
		return true, err
	}

	dir := buf

	if len(f.Dev) <= int(size) {
		if _, err := ctx.storeString(buf, size, text[:size]); err != nil {
			return true, err
		}

		dir += uint32(len(f.Dev))
	}

	for i := range uint32(6) {
		at := buf
		if i == 2 {
			at = dir
		}

		if err := ctx.storeLongword(nam+namLNOD+4*i, at); err != nil {
			return true, err
		}

		n := byte(0)
		if i == 1 {
			n = byte(len(f.Dev))
		}

		if err := ctx.storeByte(nam+namNODE+i, n); err != nil {
			return true, err
		}
	}

	return true, nil
}
