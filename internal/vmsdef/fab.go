package vmsdef

// Field is one named field within a FAB or RAB, at its real, fixed VMS byte
// offset — see FABFields/RABFields' own doc comments for how these were
// derived and verified. Not something either consumer gets to choose: a
// real, unmodified VAX program's compiled FAB/RAB layout is exactly this,
// the same kind of fixed hardware/software-interface fact as a VAX
// instruction's own opcode encoding.
type Field struct {
	// Symbol is the real MACRO-32 $FABDEF/$RABDEF offset-symbol name (e.g.
	// "FAB$B_FAC") — what .RMSDEF defines as a permanent assembler symbol,
	// letting a program address the field the real-MACRO-32 way
	// (<label>+FAB$B_FAC) once .RMSDEF has run.
	Symbol string
	// Keyword is what a .FAB/.RAB parameter names this field by (e.g.
	// "FAC"), matching the real $FAB/$RAB macro's own keyword convention —
	// Symbol's own trailing name segment, upcased.
	Keyword string
	Offset  uint32
	// Size is the field's width in bytes. .FAB/.RAB's own KEYWORD=value
	// placement only supports Size 1, 2, or 4 (byte/word/longword, the
	// scales this assembler's expression/fixup machinery already knows how
	// to store) — every field below is one of those three *except*
	// RAB$W_RFA (Size 6, a 3-word array with no single-value keyword form
	// in real $RAB either), included in RABFields purely so .RMSDEF can
	// still define its offset symbol for direct <label>+RAB$W_RFA
	// addressing.
	Size uint32
}

// FABFields is every named field of the real, 80-byte FAB ($FABDEF$K_BLN),
// derived by hand-walking reference/vms/fabdef.h's `struct fabdef`
// declaration field by field under VAX C's natural alignment rules (1-byte
// types need no alignment, a 2-byte type aligns to an even offset, a
// 4-byte type or a union containing one aligns to a multiple of 4) —
// reserved/padding fields (fabdef$$_fill_*) are omitted, since they have no
// real name a program could set by keyword.
//
// Verified two independent ways during docs/PHASE-24.md's planning (neither
// aware of the other's numbers until compared): once by hand, once by a
// research agent working from the same header — both derivations agree on
// every field, and both running totals land exactly on the header's own
// documented FAB$K_BLN/FAB$C_BLN (80). Both also reproduce every offset
// internal/rms/fab.go's own pre-existing, separately-verified (against a
// real VMS 7.3 system) subset exactly, which is itself strong independent
// confirmation of the alignment-derivation method.
var FABFields = []Field{
	{"FAB$B_BID", "BID", 0, 1},
	{"FAB$B_BLN", "BLN", 1, 1},
	{"FAB$W_IFI", "IFI", 2, 2},
	{"FAB$L_FOP", "FOP", 4, 4},
	{"FAB$L_STS", "STS", 8, 4},
	{"FAB$L_STV", "STV", 12, 4},
	{"FAB$L_ALQ", "ALQ", 16, 4},
	{"FAB$W_DEQ", "DEQ", 20, 2},
	{"FAB$B_FAC", "FAC", 22, 1},
	{"FAB$B_SHR", "SHR", 23, 1},
	{"FAB$L_CTX", "CTX", 24, 4},
	{"FAB$B_RTV", "RTV", 28, 1},
	{"FAB$B_ORG", "ORG", 29, 1},
	{"FAB$B_RAT", "RAT", 30, 1},
	{"FAB$B_RFM", "RFM", 31, 1},
	{"FAB$B_JOURNAL", "JOURNAL", 32, 1},
	{"FAB$B_RU_FACILITY", "RU_FACILITY", 33, 1},
	{"FAB$L_XAB", "XAB", 36, 4},
	{"FAB$L_NAM", "NAM", 40, 4},
	{"FAB$L_FNA", "FNA", 44, 4},
	{"FAB$L_DNA", "DNA", 48, 4},
	{"FAB$B_FNS", "FNS", 52, 1},
	{"FAB$B_DNS", "DNS", 53, 1},
	{"FAB$W_MRS", "MRS", 54, 2},
	{"FAB$L_MRN", "MRN", 56, 4},
	{"FAB$W_BLS", "BLS", 60, 2},
	{"FAB$B_BKS", "BKS", 62, 1},
	{"FAB$B_FSZ", "FSZ", 63, 1},
	{"FAB$L_DEV", "DEV", 64, 4},
	{"FAB$L_SDC", "SDC", 68, 4},
	{"FAB$W_GBC", "GBC", 72, 2},
	{"FAB$B_ACMODES", "ACMODES", 74, 1},
	{"FAB$B_RCF", "RCF", 75, 1},
}
