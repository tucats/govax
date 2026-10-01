package rms

import (
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/vmstime"
	"github.com/tucats/ods2/volume"
)

// This file is the extended attribute blocks (docs/PHASE-33.md, subtask
// 5), from the RMS Reference Manual's chapters 9-11, 15, 16, and 18:
// the XAB chain a FAB names at FAB$L_XAB, which $OPEN, $CREATE, and
// $DISPLAY fill from the file header, $CREATE reads (XABDAT, XABPRO,
// XABALL), and $CLOSE reads (XABRDT, XABPRO).

// XAB type codes and the shortest block each may be.
var (
	xabALL = byte(vmsConst("XAB$C_ALL"))
	xabDAT = byte(vmsConst("XAB$C_DAT"))
	xabFHC = byte(vmsConst("XAB$C_FHC"))
	xabITM = byte(vmsConst("XAB$C_ITM"))
	xabKEY = byte(vmsConst("XAB$C_KEY"))
	xabPRO = byte(vmsConst("XAB$C_PRO"))
	xabRDT = byte(vmsConst("XAB$C_RDT"))
	xabSUM = byte(vmsConst("XAB$C_SUM"))
	xabTRM = byte(vmsConst("XAB$C_TRM"))

	xabLengths = map[byte]byte{
		xabALL: byte(vmsConst("XAB$C_ALLLEN")),
		xabDAT: byte(vmsConst("XAB$C_DATLEN")),
		xabFHC: byte(vmsConst("XAB$C_FHCLEN")),
		xabITM: 0,
		xabKEY: 0,
		xabPRO: byte(vmsConst("XAB$C_PROLEN")),
		xabRDT: byte(vmsConst("XAB$C_RDTLEN")),
		xabSUM: byte(vmsConst("XAB$C_SUMLEN")),
		xabTRM: 0,
	}
)

// XAB field offsets.
var (
	xabCOD = vmsConst("XAB$B_COD")
	xabBLN = vmsConst("XAB$B_BLN")
	xabNXT = vmsConst("XAB$L_NXT")

	// XABDAT and XABRDT, which share RDT and RVN's offsets ($XABDEF's).
	xabCDT  = vmsConst("XAB$Q_CDT")
	xabRDTq = vmsConst("XAB$Q_RDT")
	xabEDT  = vmsConst("XAB$Q_EDT")
	xabBDT  = vmsConst("XAB$Q_BDT")
	xabRVN  = vmsConst("XAB$W_RVN")

	// XABFHC.
	xabRFO      = vmsConst("XAB$B_RFO")
	xabATR      = vmsConst("XAB$B_ATR")
	xabLRL      = vmsConst("XAB$W_LRL")
	xabHBK      = vmsConst("XAB$L_HBK")
	xabEBK      = vmsConst("XAB$L_EBK")
	xabFFB      = vmsConst("XAB$W_FFB")
	xabBKZFHC   = vmsConst("XAB$B_BKZ")
	xabHSZ      = vmsConst("XAB$B_HSZ")
	xabMRZ      = vmsConst("XAB$W_MRZ")
	xabDXQ      = vmsConst("XAB$W_DXQ")
	xabGBC      = vmsConst("XAB$W_GBC")
	xabVERLIMIT = vmsConst("XAB$W_VERLIMIT")
	xabSBN      = vmsConst("XAB$L_SBN")

	// XABPRO.
	xabPROw   = vmsConst("XAB$W_PRO")
	xabUIC    = vmsConst("XAB$L_UIC")
	xabMTACC  = vmsConst("XAB$B_MTACC")
	xabACLSTS = vmsConst("XAB$L_ACLSTS")

	// XABALL.
	xabALQ = vmsConst("XAB$L_ALQ")
	xabDEQ = vmsConst("XAB$W_DEQ")
	xabAOP = vmsConst("XAB$B_AOP")

	// XABSUM.
	xabNOA = vmsConst("XAB$B_NOA")
	xabNOK = vmsConst("XAB$B_NOK")
	xabPVN = vmsConst("XAB$W_PVN")

	xabContiguous = byte(vmsConst("XAB$M_CTG"))
)

// maxXABs bounds a chain, so a loop in one can't hang a service.
const maxXABs = 64

// xabEntry is one XAB in a chain: its address and type.
type xabEntry struct {
	Addr uint32
	Cod  byte
}

// xabChain walks the chain FAB$L_XAB starts, checking each block, as VMS
// 7.3 does (the oracle's XAB probe): an unknown type is RMS$_COD, a block
// too short for its type RMS$_XAB, and a second XAB of a type there can
// be only one of RMS$_IMX, each with the XAB's address as STV.
func (ctx *Context) xabChain(fab uint32) ([]xabEntry, uint32, uint32, error) {
	addr, err := ctx.loadLongword(fab + fabOffset("XAB"))
	if err != nil {
		return nil, 0, 0, err
	}

	var chain []xabEntry

	seen := map[byte]bool{}

	for addr != 0 && len(chain) < maxXABs {
		cod, err := ctx.loadByte(addr + xabCOD)
		if err != nil {
			return nil, 0, 0, err
		}

		bln, err := ctx.loadByte(addr + xabBLN)
		if err != nil {
			return nil, 0, 0, err
		}

		min, ok := xabLengths[cod]
		if !ok {
			return nil, rmsInvalidXABCode, addr, nil
		}

		if bln < min {
			return nil, rmsInvalidXAB, addr, nil
		}

		// XABALL and XABKEY come one per area or key.
		if seen[cod] && cod != xabALL && cod != xabKEY {
			return nil, rmsDuplicateXAB, addr, nil
		}

		seen[cod] = true

		chain = append(chain, xabEntry{Addr: addr, Cod: cod})

		if addr, err = ctx.loadLongword(addr + xabNXT); err != nil {
			return nil, 0, 0, err
		}
	}

	return chain, 0, 0, nil
}

// storeQuad writes a quadword.
func (ctx *Context) storeQuad(addr uint32, v vmstime.VMSTime) error {
	if err := ctx.storeLongword(addr, uint32(uint64(v))); err != nil {
		return err
	}

	return ctx.storeLongword(addr+4, uint32(uint64(v)>>32))
}

// loadQuad reads a quadword.
func (ctx *Context) loadQuad(addr uint32) (vmstime.VMSTime, error) {
	lo, err := ctx.loadLongword(addr)
	if err != nil {
		return 0, err
	}

	hi, err := ctx.loadLongword(addr + 4)

	return vmstime.VMSTime(uint64(hi)<<32 | uint64(lo)), err
}

// xabMode is the service filling the XABs, which VMS fills a little
// differently (the oracle's XAB and CREATE probes).
type xabMode int

const (
	xabOpen xabMode = iota
	xabDisplay
	xabCreate
)

// mtaccDefault is XAB$B_MTACC as $OPEN reports it for a disk file: a
// blank, the magnetic tape accessibility character no tape has set.
const mtaccDefault = ' '

// fillXABs fills each output XAB of chain from f's header. As VMS does:
//
//   - XABPRO's ACL status is SS$_ACLEMPTY from $OPEN and SS$_NORMAL from
//     $DISPLAY; $CREATE sets only the status, leaving the protection and
//     owner it was given;
//   - XABFHC's version limit is 32767 for none from $OPEN, and the
//     header's own value from $DISPLAY;
//   - $CREATE leaves the XABALL it was given.
func (ctx *Context) fillXABs(chain []xabEntry, f *volume.File, mode xabMode) error {
	h := f.Header

	id, err := h.Ident()
	if err != nil {
		id = ondisk.Ident{}
	}

	ra := h.RecordAttributes

	for _, x := range chain {
		a := x.Addr

		var stores []func() error

		switch x.Cod {
		case xabDAT:
			stores = []func() error{
				func() error { return ctx.storeQuad(a+xabCDT, id.CreationDate) },
				func() error { return ctx.storeQuad(a+xabRDTq, id.RevisionDate) },
				func() error { return ctx.storeQuad(a+xabEDT, id.ExpirationDate) },
				func() error { return ctx.storeQuad(a+xabBDT, id.BackupDate) },
				func() error { return ctx.storeWord(a+xabRVN, id.Revision) },
			}

		case xabRDT:
			stores = []func() error{
				func() error { return ctx.storeQuad(a+xabRDTq, id.RevisionDate) },
				func() error { return ctx.storeWord(a+xabRVN, id.Revision) },
			}

		case xabFHC:
			sbn := uint32(0)
			if h.FileCharacteristics&ondisk.FchContig != 0 && len(f.Extents) > 0 {
				sbn = f.Extents[0].StartLBN
			}

			stores = []func() error{
				func() error { return ctx.storeByte(a+xabRFO, byte(ra.Format)) },
				func() error { return ctx.storeByte(a+xabATR, ra.Attributes) },
				func() error { return ctx.storeWord(a+xabLRL, ra.RecordSize) },
				func() error { return ctx.storeLongword(a+xabHBK, ra.HighestBlock) },
				func() error { return ctx.storeLongword(a+xabEBK, ra.EndOfFileBlock) },
				func() error { return ctx.storeWord(a+xabFFB, ra.FirstFreeByte) },
				func() error { return ctx.storeByte(a+xabBKZFHC, ra.BucketSize) },
				func() error { return ctx.storeByte(a+xabHSZ, ra.VfcSize) },
				func() error { return ctx.storeWord(a+xabMRZ, ra.MaxRecordSize) },
				func() error { return ctx.storeWord(a+xabDXQ, ra.DefaultExtend) },
				func() error { return ctx.storeWord(a+xabGBC, ra.GlobalBufferCount) },
				func() error { return ctx.storeWord(a+xabVERLIMIT, verLimit(ra.VersionLimit, mode)) },
				func() error { return ctx.storeLongword(a+xabSBN, sbn) },
			}

		case xabPRO:
			aclsts := ssNormal
			if mode == xabOpen {
				aclsts = ssACLEmpty
			}

			stores = []func() error{
				func() error { return ctx.storeLongword(a+xabACLSTS, aclsts) },
			}

			if mode != xabCreate {
				stores = append(stores,
					func() error { return ctx.storeWord(a+xabPROw, h.FileProtection) },
					func() error { return ctx.storeByte(a+xabMTACC, mtaccDefault) },
					func() error {
						return ctx.storeLongword(a+xabUIC, uint32(h.Owner.Group)<<16|uint32(h.Owner.Member))
					},
				)
			}

		case xabALL:
			if mode == xabCreate {
				continue
			}

			aop := byte(0)
			if h.FileCharacteristics&ondisk.FchContig != 0 {
				aop = xabContiguous
			}

			stores = []func() error{
				func() error { return ctx.storeLongword(a+xabALQ, ra.HighestBlock) },
				func() error { return ctx.storeWord(a+xabDEQ, ra.DefaultExtend) },
				func() error { return ctx.storeByte(a+xabAOP, aop) },
			}

		case xabSUM:
			stores = []func() error{
				func() error { return ctx.storeByte(a+xabNOA, 0) },
				func() error { return ctx.storeByte(a+xabNOK, 0) },
				func() error { return ctx.storeWord(a+xabPVN, 0) },
			}
		}

		for _, s := range stores {
			if err := s(); err != nil {
				return err
			}
		}
	}

	return nil
}

// verLimit is XABFHC's version limit for a header's: $OPEN reports no
// limit (0) as 32767.
func verLimit(limit uint16, mode xabMode) uint16 {
	if limit == 0 && mode == xabOpen {
		return 32767
	}

	return limit
}

// fillFABAttributes writes the file's attributes into the FAB, as $OPEN
// and $DISPLAY do: ALQ, DEQ, ORG, RFM, RAT, MRS, FSZ, BKS, GBC, BLS (the
// block size), and FOP's CTG and CBT for a contiguous file.
func (ctx *Context) fillFABAttributes(fab uint32, f *volume.File) error {
	ra := f.Header.RecordAttributes

	for _, s := range []func() error{
		func() error { return ctx.storeLongword(fab+fabOffset("ALQ"), ra.HighestBlock) },
		func() error { return ctx.storeWord(fab+fabOffset("DEQ"), ra.DefaultExtend) },
		func() error { return ctx.storeByte(fab+fabORG, byte(ra.Format)&0xf0) },
		func() error { return ctx.storeByte(fab+fabRFM, byte(ra.Format)&0x0f) },
		func() error { return ctx.storeByte(fab+fabRAT, ra.Attributes) },
		func() error { return ctx.storeWord(fab+fabMRS, ra.MaxRecordSize) },
		func() error { return ctx.storeByte(fab+fabOffset("FSZ"), ra.VfcSize) },
		func() error { return ctx.storeByte(fab+fabOffset("BKS"), ra.BucketSize) },
		func() error { return ctx.storeWord(fab+fabOffset("GBC"), ra.GlobalBufferCount) },
		func() error { return ctx.storeWord(fab+fabOffset("BLS"), ondisk.BlockSize) },
		func() error {
			fop, err := ctx.loadLongword(fab + fabFOP)
			if err != nil {
				return err
			}

			fop &^= fopCTG | fopCBT

			switch {
			case f.Header.FileCharacteristics&ondisk.FchContig != 0:
				fop |= fopCTG
			case f.Header.FileCharacteristics&ondisk.FchContigB != 0:
				fop |= fopCBT
			}

			return ctx.storeLongword(fab+fabFOP, fop)
		},
	} {
		if err := s(); err != nil {
			return err
		}
	}

	return nil
}

// xabInputs are what $CREATE takes from its XABs.
type xabInputs struct {
	// Dates from the XABDAT; a zero one isn't given.
	Created, Revised, Expires, Backup vmstime.VMSTime

	// Protection and Owner from the XABPRO, when HasPro.
	HasPro     bool
	Protection uint16
	Owner      ondisk.Uic

	// Alloc and Extend from the XABALL, when HasAll.
	HasAll bool
	Alloc  uint32
	Extend uint16

	// RDT and RVN from a XABRDT ($CLOSE), when HasRDT.
	HasRDT bool
	RDT    vmstime.VMSTime
	RVN    uint16
}

// readXABInputs reads the input fields of chain's XABDAT, XABPRO,
// XABALL, and XABRDT.
func (ctx *Context) readXABInputs(chain []xabEntry) (xabInputs, error) {
	var (
		in  xabInputs
		err error
	)

	for _, x := range chain {
		a := x.Addr

		switch x.Cod {
		case xabDAT:
			for _, q := range []struct {
				off uint32
				to  *vmstime.VMSTime
			}{{xabCDT, &in.Created}, {xabRDTq, &in.Revised}, {xabEDT, &in.Expires}, {xabBDT, &in.Backup}} {
				if *q.to, err = ctx.loadQuad(a + q.off); err != nil {
					return in, err
				}
			}

		case xabPRO:
			in.HasPro = true

			if in.Protection, err = ctx.loadWord(a + xabPROw); err != nil {
				return in, err
			}

			uic, err := ctx.loadLongword(a + xabUIC)
			if err != nil {
				return in, err
			}

			in.Owner = ondisk.Uic{Member: uint16(uic), Group: uint16(uic >> 16)}

		case xabALL:
			in.HasAll = true

			if in.Alloc, err = ctx.loadLongword(a + xabALQ); err != nil {
				return in, err
			}

			if in.Extend, err = ctx.loadWord(a + xabDEQ); err != nil {
				return in, err
			}

		case xabRDT:
			in.HasRDT = true

			if in.RDT, err = ctx.loadQuad(a + xabRDTq); err != nil {
				return in, err
			}

			if in.RVN, err = ctx.loadWord(a + xabRVN); err != nil {
				return in, err
			}
		}
	}

	return in, nil
}

// applyCreateXABs writes $CREATE's inputs into a new file's header: the
// XABDAT's dates (a zero creation or revision date keeps the file
// system's), the XABPRO's protection and owner (a zero owner keeps the
// file system's), the XABALL's extension quantity, and alq blocks
// allocated (the XABALL's quantity, or else the FAB's). As VMS has it, a
// new file's revision number is 0 until it's first closed.
func applyCreateXABs(f *volume.File, in xabInputs, alq uint32) error {
	if alq > f.Blocks() {
		bm, err := f.Device.Bitmap()
		if err != nil {
			return err
		}

		ib, err := f.Device.IndexBitmap()
		if err != nil {
			return err
		}

		if err := volume.Extend(f, bm, ib, alq-f.Blocks()); err != nil {
			return err
		}
	}

	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, id *ondisk.Ident) {
		id.Revision = 0

		if in.Created != 0 {
			id.CreationDate = in.Created
		}

		if in.Revised != 0 {
			id.RevisionDate = in.Revised
		}

		id.ExpirationDate = in.Expires
		id.BackupDate = in.Backup

		if in.HasPro {
			h.FileProtection = in.Protection

			if in.Owner != (ondisk.Uic{}) {
				h.Owner = in.Owner
			}
		}

		if in.HasAll {
			h.RecordAttributes.DefaultExtend = in.Extend
		}
	})
}

// applyCloseXABs updates the header of a file opened for writing as it's
// closed (the manual's XABRDT chapter, and the oracle's CREATE case 7). An
// empty file's end of file is block 1, byte 0, as VMS has it. Then:
// the revision date and number are the XABRDT's when it gives a date, and
// otherwise now and one more than they were; a XABPRO gives the
// protection and owner.
func applyCloseXABs(f *volume.File, in xabInputs, now vmstime.VMSTime) error {
	return volume.UpdateHeader(f, func(h *ondisk.FileHeader, id *ondisk.Ident) {
		// An empty file ends at the start of its first block, as VMS
		// writes it, not at block 0 (the oracle's CREATE case 7).
		if h.RecordAttributes.EndOfFileBlock == 0 {
			h.RecordAttributes.EndOfFileBlock = 1
			h.RecordAttributes.FirstFreeByte = 0
		}

		if in.HasRDT && in.RDT != 0 {
			id.RevisionDate = in.RDT
			id.Revision = in.RVN
		} else {
			id.RevisionDate = now
			id.Revision++
		}

		if in.HasPro {
			h.FileProtection = in.Protection

			if in.Owner != (ondisk.Uic{}) {
				h.Owner = in.Owner
			}
		}
	})
}
