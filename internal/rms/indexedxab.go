package rms

import (
	"github.com/tucats/govax/internal/isam"
	"github.com/tucats/ods2/volume"
)

// What $OPEN, $DISPLAY, and $CREATE say of an indexed file's keys and areas
// (Phase 88, subtask 7), from its prolog: a XABSUM gets the number of areas
// and keys and the prolog's level, a XABKEY the key its REF names, and a
// XABALL the area its AID names, as VMS 7.1's did (testdata/idx88's shape
// and area cases, each file's $DISPLAY of every key and area).
//
// A XABKEY's FLG has the compression bits inverted, as a program sets them,
// and KEY$M_INITIDX as the descriptor has it; LVL, RVB, and DVB are 0 until
// the key's first record makes its index. KNM, if not 0, gets the key's name
// in 32 bytes, blank filled, or zeros for a key without one. A XABALL's ALQ
// is the area's whole allocation, its extensions too (S66).
//
// A REF past the file's keys is RMS$_REF and an AID past its areas RMS$_AID,
// each with the number as STV; that VMS refuses them so is unconfirmed, no
// case asking. $CREATE fills only the XABSUM: it leaves the XABKEYs and
// XABALLs it was given (as it leaves a sequential file's XABALL).

// fillIndexedXABs fills chain's XABSUM, XABKEYs, and XABALLs from f's prolog.
func (ctx *Context) fillIndexedXABs(chain []xabEntry, f *volume.File, mode xabMode) (uint32, uint32, error) {
	file, err := openIndexed(f, false)
	if err != nil {
		return prologStatus(err), 0, nil
	}

	for _, x := range chain {
		switch {
		case x.Cod == xabSUM:
			if err := ctx.fillSummary(x.Addr, file); err != nil {
				return 0, 0, err
			}

		case mode == xabCreate:

		case x.Cod == xabKEY:
			ref, err := ctx.loadByte(x.Addr + xabKeyREF)
			if err != nil {
				return 0, 0, err
			}

			if int(ref) >= len(file.Keys) {
				return rmsKeyRef, uint32(ref), nil
			}

			if err := ctx.fillKey(x.Addr, file.Keys[ref], file.Version); err != nil {
				return 0, 0, err
			}

		case x.Cod == xabALL:
			aid, err := ctx.loadByte(x.Addr + xabAllAID)
			if err != nil {
				return 0, 0, err
			}

			if int(aid) >= len(file.Areas) {
				return rmsAreaID, uint32(aid), nil
			}

			if err := ctx.fillArea(x.Addr, &file.Areas[aid]); err != nil {
				return 0, 0, err
			}
		}
	}

	return 0, 0, nil
}

// fillSummary fills a XABSUM.
func (ctx *Context) fillSummary(addr uint32, file *isam.File) error {
	if err := ctx.storeByte(addr+xabNOA, byte(len(file.Areas))); err != nil {
		return err
	}

	if err := ctx.storeByte(addr+xabNOK, byte(len(file.Keys))); err != nil {
		return err
	}

	return ctx.storeWord(addr+xabPVN, uint16(file.Version))
}

// fillKey fills a XABKEY from a key descriptor. XAB$L_DVB and XAB$B_PROLOG
// are written only to a block long enough to have them.
func (ctx *Context) fillKey(addr uint32, k *isam.Key, version int) error {
	var rvb, dvb uint32
	if k.HasIndex() {
		rvb, dvb = k.RootVBN, k.FirstData
	}

	bytes := []struct {
		off uint32
		v   byte
	}{
		{xabKeyIAN, byte(k.IndexArea)}, {xabKeyLAN, byte(k.Level1Area)}, {xabKeyDAN, byte(k.DataArea)},
		{xabKeyLVL, byte(k.RootLevel)}, {xabKeyIBS, byte(k.IndexBucketSize)}, {xabKeyDBS, byte(k.DataBucketSize)},
		{xabKeyFLG, xabFlags(k.Flags)}, {xabKeyDTP, byte(k.Type)}, {xabKeyNSG, byte(len(k.Segments))},
		{xabKeyNUL, k.Null}, {xabKeyTKS, byte(k.Size)},
	}

	for i := range keySegments {
		var size byte
		if i < len(k.Segments) {
			size = byte(k.Segments[i].Size)
		}

		bytes = append(bytes, struct {
			off uint32
			v   byte
		}{xabKeySIZ + uint32(i), size})
	}

	for _, b := range bytes {
		if err := ctx.storeByte(addr+b.off, b.v); err != nil {
			return err
		}
	}

	words := []struct {
		off uint32
		v   uint16
	}{
		{xabKeyMRL, uint16(k.MinRecord)}, {xabKeyIFL, uint16(k.IndexFill)}, {xabKeyDFL, uint16(k.DataFill)},
	}

	for i := range keySegments {
		var pos uint16
		if i < len(k.Segments) {
			pos = uint16(k.Segments[i].Position)
		}

		words = append(words, struct {
			off uint32
			v   uint16
		}{xabKeyPOS + uint32(2*i), pos})
	}

	for _, w := range words {
		if err := ctx.storeWord(addr+w.off, w.v); err != nil {
			return err
		}
	}

	// XAB$L_RVB isn't longword aligned (0E): its words go one at a time.
	if err := ctx.storeWord(addr+xabKeyRVB, uint16(rvb)); err != nil {
		return err
	}

	if err := ctx.storeWord(addr+xabKeyRVB+2, uint16(rvb>>16)); err != nil {
		return err
	}

	if err := ctx.fillKeyName(addr, k); err != nil {
		return err
	}

	bln, err := ctx.loadByte(addr + xabBLN)
	if err != nil {
		return err
	}

	if uint32(bln) >= xabKeyDVB+4 {
		if err := ctx.storeLongword(addr+xabKeyDVB, dvb); err != nil {
			return err
		}
	}

	if uint32(bln) > xabKeyPROLOG {
		return ctx.storeByte(addr+xabKeyPROLOG, byte(version))
	}

	return nil
}

// fillKeyName writes a key's name to the buffer XAB$L_KNM names, if any:
// blank filled, or zeros for an unnamed key (S37 and every other file).
func (ctx *Context) fillKeyName(addr uint32, k *isam.Key) error {
	knm, err := ctx.loadLongword(addr + xabKeyKNM)
	if err != nil || knm == 0 {
		return err
	}

	name := make([]byte, keyNameSize)

	if k.Name != "" {
		for i := range name {
			name[i] = ' '
		}

		copy(name, k.Name)
	}

	for i, b := range name {
		if err := ctx.storeByte(knm+uint32(i), b); err != nil {
			return err
		}
	}

	return nil
}

// fillArea fills a XABALL from an area descriptor.
func (ctx *Context) fillArea(addr uint32, a *isam.Area) error {
	for _, s := range []func() error{
		func() error { return ctx.storeByte(addr+xabAOP, a.Options) },
		func() error { return ctx.storeByte(addr+xabAllALN, a.Alignment) },
		func() error { return ctx.storeWord(addr+xabAllVOL, uint16(a.Volume)) },
		func() error { return ctx.storeLongword(addr+xabAllLOC, a.Location) },
		func() error { return ctx.storeLongword(addr+xabALQ, a.TotalAllocation) },
		func() error { return ctx.storeWord(addr+xabDEQ, uint16(a.ExtendSize)) },
		func() error { return ctx.storeByte(addr+xabAllBKZ, byte(a.BucketSize)) },
	} {
		if err := s(); err != nil {
			return err
		}
	}

	return nil
}
