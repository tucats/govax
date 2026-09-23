package vmsdef

// RABFields is every named field of the real, 68-byte RAB ($RABDEF$K_BLN),
// derived the same way FABFields was — see that var's own doc comment for
// the method and cross-verification. Both `KBF`/`PBF` (offset 48),
// `KSZ`/`PSZ` (offset 52), and `BKT`/`DCT` (offset 56) are real, separately
// documented union member names for the same bytes (not C-only bitfield
// views, which are omitted the same way FABFields omits them) — both are
// included since either is a genuine field name a program might reference,
// matching this phase's "every known symbol" goal.
var RABFields = []Field{
	{"RAB$B_BID", "BID", 0, 1},
	{"RAB$B_BLN", "BLN", 1, 1},
	{"RAB$W_ISI", "ISI", 2, 2},
	{"RAB$L_ROP", "ROP", 4, 4},
	{"RAB$L_STS", "STS", 8, 4},
	{"RAB$L_STV", "STV", 12, 4},
	{"RAB$W_RFA", "RFA", 16, 6}, // see Field.Size's own doc comment: not keyword-settable.
	{"RAB$L_CTX", "CTX", 24, 4},
	{"RAB$B_RAC", "RAC", 30, 1},
	{"RAB$B_TMO", "TMO", 31, 1},
	{"RAB$W_USZ", "USZ", 32, 2},
	{"RAB$W_RSZ", "RSZ", 34, 2},
	{"RAB$L_UBF", "UBF", 36, 4},
	{"RAB$L_RBF", "RBF", 40, 4},
	{"RAB$L_RHB", "RHB", 44, 4},
	{"RAB$L_KBF", "KBF", 48, 4},
	{"RAB$L_PBF", "PBF", 48, 4},
	{"RAB$B_KSZ", "KSZ", 52, 1},
	{"RAB$B_PSZ", "PSZ", 52, 1},
	{"RAB$B_KRF", "KRF", 53, 1},
	{"RAB$B_MBF", "MBF", 54, 1},
	{"RAB$B_MBC", "MBC", 55, 1},
	{"RAB$L_BKT", "BKT", 56, 4},
	{"RAB$L_DCT", "DCT", 56, 4},
	{"RAB$L_FAB", "FAB", 60, 4},
	{"RAB$L_XAB", "XAB", 64, 4},
}
