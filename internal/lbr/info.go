package lbr

// Where the two header fields HeaderInfo reads that Open doesn't are
// ($LHDDEF): the update history's first block and the first deleted block,
// before LHD$L_FREEBLK in the manual's order. Unconfirmed: every library the
// probes read has both 0.
const (
	lhdUpdHis  = 0x40
	lhdFreeVBN = 0x44
)

// HeaderInfo is the library's header as LBR$GET_HEADER returns it (the
// Utility Routines manual's LHI$ table): 32 longwords, in its order. The
// header block's fields are moved into place; the library's status
// (LHI$L_LIBSTATUS) is 1, since a library govax reads was closed properly.
func (l *Library) HeaderInfo() [32]uint32 {
	h := l.data[:blockSize]

	var info [32]uint32

	info[0] = uint32(h[lhdType])
	info[1] = uint32(h[lhdNIndex])
	info[2] = uint32(le16(h, lhdMajorID))
	info[3] = uint32(le16(h, lhdMinorID))

	for i := range 8 { // LHI$T_LBRVER, 32 bytes
		info[4+i] = le32(h, lhdLbrVer+4*i)
	}

	for i := range 4 { // LHI$L_CREDAT and LHI$L_UPDTIM, quadwords
		info[12+i] = le32(h, lhdCreDat+4*i)
	}

	info[16] = le32(h, lhdUpdHis)
	info[17] = le32(h, lhdFreeVBN)
	info[18] = le32(h, lhdFreeBlk)
	info[19] = le32(h, lhdNextRFA)
	info[20] = uint32(le16(h, lhdNextRFA+4))
	info[21] = le32(h, lhdNextVBN)
	info[22] = le32(h, lhdFreIdxBlk)
	info[23] = le32(h, lhdFreeIdx)
	info[24] = le32(h, lhdHiPreAl)
	info[25] = le32(h, lhdIdxBlks)
	info[26] = le32(h, lhdIdxCnt)
	info[27] = le32(h, lhdModCnt)
	info[28] = uint32(h[lhdMhdUsz])
	info[29] = uint32(le16(h, lhdMaxLUHRec))
	info[30] = uint32(le16(h, lhdNumLUHRec))
	info[31] = 1

	return info
}
