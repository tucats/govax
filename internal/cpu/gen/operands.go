package main

// This file is the generator's second source, next to the C reference's
// instruction_table.h: every VAX instruction's operand list, transcribed
// from DIGITAL's VAX Architecture Reference Manual (vax_instr_set.pdf).
//
// # The manual's operand notation
//
// The manual describes each instruction with a "format line" such as
//
//	ADDL3 add1.rl, add2.rl, sum.wl
//
// Each operand is a name, a dot, then two letters:
//
//   - the first letter is the operand's ACCESS: how the instruction uses it.
//     r = read, w = write, m = modify (read, then written back), a = address
//     (the operand's address is used, not its contents), v = variable
//     bit-field base (an address, or a register), b = branch displacement
//     (a signed offset in the instruction stream, added to the PC).
//   - the second letter is the operand's DATA TYPE, which also fixes its
//     size: b = byte (1), w = word (2), l = longword (4), q = quadword (8),
//     o = octaword (16), f = F_floating (4), d = D_floating (8),
//     g = G_floating (8), h = H_floating (16).
//
// For an address operand ("ab", "aq") the data type is the type of the
// thing at that address. It matters to the CPU: indexed mode, "(Rn)[Rx]",
// scales the index register by that size, and autoincrement, "(Rn)+",
// advances Rn by it.
//
// The table below keeps only the access/type pairs, comma-separated, in
// operand order: ADDL3 is "rl,rl,wl". An instruction with no operands
// (HALT, REI, LDPCTX, ...) is "".
//
// # govax's own letter: i
//
// "i" (implicit immediate) is not in the manual. govax uses it for data
// that follows an opcode in the instruction stream but is not an operand
// specifier (no addressing-mode byte): the selector byte after XFC, which
// govax uses for its RTL shims, and the word or longword after BUGW/BUGL.
//
// # How the generator uses this table
//
//   - An instruction_table.h entry with no operands, whose instruction has
//     operands here, is filled in from here. The C header left 49 such
//     entries blank (the instructions eVAX never implemented).
//   - An entry with operands must agree with this table (after the
//     generator's knownTableFixes). If it doesn't, the generator stops,
//     unless manualCorrections names the instruction and says why the C
//     header is wrong. Nothing changes silently.
//   - Instructions here that instruction_table.h lacks altogether are
//     listed in missingInstructions with their opcodes, and are added.
//   - Every instruction's data types come from here.

// manualOperands maps each mnemonic to its operand list in the manual's
// notation (see the file comment). Grouped by opcode, as the manual's
// opcode table is.
var manualOperands = map[string]string{
	// 00-0F: control, queues, and context.
	"HALT": "", "NOP": "", "REI": "", "BPT": "", "RET": "", "RSB": "",
	"LDPCTX": "", "SVPCTX": "",
	"CVTPS": "rw,ab,rw,ab", // srclen, srcaddr, dstlen, dstaddr (leading separate numeric)
	"CVTSP": "rw,ab,rw,ab",
	"INDEX": "rl,rl,rl,rl,rl,wl", // subscript, low, high, size, indexin, indexout
	"CRC":   "ab,rl,rw,ab",       // tbl, inicrc, strlen, stream
	"PROBER": "rb,rw,ab", "PROBEW": "rb,rw,ab", // mode, len, base
	"INSQUE": "ab,ab", // entry, pred
	"REMQUE": "ab,wl", // entry, addr

	// 10-1F: branches and subroutine jumps.
	"BSBB": "bb", "BRB": "bb", "BNEQ": "bb", "BEQL": "bb",
	"BGTR": "bb", "BLEQ": "bb", "JSB": "ab", "JMP": "ab",
	"BGEQ": "bb", "BLSS": "bb", "BGTRU": "bb", "BLEQU": "bb",
	"BVC": "bb", "BVS": "bb", "BCC": "bb", "BCS": "bb",

	// 20-27: packed decimal arithmetic and conversions. A packed decimal
	// string is a length (in digits, 0-31) and the address of its first
	// byte, so these take length/address pairs.
	"ADDP4": "rw,ab,rw,ab",       // addlen, addaddr, sumlen, sumaddr
	"ADDP6": "rw,ab,rw,ab,rw,ab", // add1len, add1addr, add2len, add2addr, sumlen, sumaddr
	"SUBP4": "rw,ab,rw,ab",
	"SUBP6": "rw,ab,rw,ab,rw,ab",
	"CVTPT": "rw,ab,ab,rw,ab", // srclen, srcaddr, tbladdr, dstlen, dstaddr (trailing numeric)
	"MULP":  "rw,ab,rw,ab,rw,ab",
	"CVTTP": "rw,ab,ab,rw,ab",
	"DIVP":  "rw,ab,rw,ab,rw,ab",

	// 28-2F: character strings.
	"MOVC3": "rw,ab,ab", "CMPC3": "rw,ab,ab",
	"SCANC": "rw,ab,ab,rb", "SPANC": "rw,ab,ab,rb", // len, addr, tbladdr, mask
	"MOVC5": "rw,ab,rb,rw,ab", "CMPC5": "rw,ab,rb,rw,ab",
	"MOVTC": "rw,ab,rb,ab,rw,ab", "MOVTUC": "rw,ab,rb,ab,rw,ab",

	// 30-3F.
	"BSBW": "bw", "BRW": "bw", "CVTWL": "rw,wl", "CVTWB": "rw,wb",
	"MOVP":   "rw,ab,ab",    // len, srcaddr, dstaddr
	"CMPP3":  "rw,ab,ab",    // len, src1addr, src2addr
	"CVTPL":  "rw,ab,wl",    // srclen, srcaddr, dst
	"CMPP4":  "rw,ab,rw,ab", // src1len, src1addr, src2len, src2addr
	"EDITPC": "rw,ab,ab,ab", // srclen, srcaddr, pattern, dstaddr
	"MATCHC": "rw,ab,rw,ab", "LOCC": "rb,rw,ab", "SKPC": "rb,rw,ab",
	"MOVZWL": "rw,wl", "ACBW": "rw,rw,mw,bw", "MOVAW": "aw,wl", "PUSHAW": "aw",

	// 40-57: F_floating.
	"ADDF2": "rf,mf", "ADDF3": "rf,rf,wf", "SUBF2": "rf,mf", "SUBF3": "rf,rf,wf",
	"MULF2": "rf,mf", "MULF3": "rf,rf,wf", "DIVF2": "rf,mf", "DIVF3": "rf,rf,wf",
	"CVTFB": "rf,wb", "CVTFW": "rf,ww", "CVTFL": "rf,wl", "CVTRFL": "rf,wl",
	"CVTBF": "rb,wf", "CVTWF": "rw,wf", "CVTLF": "rl,wf", "ACBF": "rf,rf,mf,bw",
	"MOVF": "rf,wf", "CMPF": "rf,rf", "MNEGF": "rf,wf", "TSTF": "rf",
	"EMODF": "rf,rb,rf,wl,wf", // mulr, mulrx (extension byte), muld, int, fract
	"POLYF": "rf,rw,ab",       // arg, degree, tbladdr
	"CVTFD": "rf,wd",

	// 58-5F: ADAWI and the interlocked queue instructions.
	"ADAWI":  "rw,mw",
	"INSQHI": "ab,aq", "INSQTI": "ab,aq", // entry, header
	"REMQHI": "aq,wl", "REMQTI": "aq,wl", // header, addr

	// 60-77: D_floating.
	"ADDD2": "rd,md", "ADDD3": "rd,rd,wd", "SUBD2": "rd,md", "SUBD3": "rd,rd,wd",
	"MULD2": "rd,md", "MULD3": "rd,rd,wd", "DIVD2": "rd,md", "DIVD3": "rd,rd,wd",
	"CVTDB": "rd,wb", "CVTDW": "rd,ww", "CVTDL": "rd,wl", "CVTRDL": "rd,wl",
	"CVTBD": "rb,wd", "CVTWD": "rw,wd", "CVTLD": "rl,wd", "ACBD": "rd,rd,md,bw",
	"MOVD": "rd,wd", "CMPD": "rd,rd", "MNEGD": "rd,wd", "TSTD": "rd",
	"EMODD": "rd,rb,rd,wl,wd",
	"POLYD": "rd,rw,ab",
	"CVTDF": "rd,wf",

	// 78-7F: quadword.
	"ASHL": "rb,rl,wl", "ASHQ": "rb,rq,wq", "EMUL": "rl,rl,rl,wq", "EDIV": "rl,rq,wl,wl",
	"CLRQ": "wq", "MOVQ": "rq,wq", "MOVAQ": "aq,wl", "PUSHAQ": "aq",

	// 80-9F: byte.
	"ADDB2": "rb,mb", "ADDB3": "rb,rb,wb", "SUBB2": "rb,mb", "SUBB3": "rb,rb,wb",
	"MULB2": "rb,mb", "MULB3": "rb,rb,wb", "DIVB2": "rb,mb", "DIVB3": "rb,rb,wb",
	"BISB2": "rb,mb", "BISB3": "rb,rb,wb", "BICB2": "rb,mb", "BICB3": "rb,rb,wb",
	"XORB2": "rb,mb", "XORB3": "rb,rb,wb", "MNEGB": "rb,wb", "CASEB": "rb,rb,rb",
	"MOVB": "rb,wb", "CMPB": "rb,rb", "MCOMB": "rb,wb", "BITB": "rb,rb",
	"CLRB": "wb", "TSTB": "rb", "INCB": "mb", "DECB": "mb",
	"CVTBL": "rb,wl", "CVTBW": "rb,ww", "MOVZBL": "rb,wl", "MOVZBW": "rb,ww",
	"ROTL": "rb,rl,wl", "ACBB": "rb,rb,mb,bw", "MOVAB": "ab,wl", "PUSHAB": "ab",

	// A0-BF: word, PSW, register masks, and change mode.
	"ADDW2": "rw,mw", "ADDW3": "rw,rw,ww", "SUBW2": "rw,mw", "SUBW3": "rw,rw,ww",
	"MULW2": "rw,mw", "MULW3": "rw,rw,ww", "DIVW2": "rw,mw", "DIVW3": "rw,rw,ww",
	"BISW2": "rw,mw", "BISW3": "rw,rw,ww", "BICW2": "rw,mw", "BICW3": "rw,rw,ww",
	"XORW2": "rw,mw", "XORW3": "rw,rw,ww", "MNEGW": "rw,ww", "CASEW": "rw,rw,rw",
	"MOVW": "rw,ww", "CMPW": "rw,rw", "MCOMW": "rw,ww", "BITW": "rw,rw",
	"CLRW": "ww", "TSTW": "rw", "INCW": "mw", "DECW": "mw",
	"BISPSW": "rw", "BICPSW": "rw", "POPR": "rw", "PUSHR": "rw",
	"CHMK": "rw", "CHME": "rw", "CHMS": "rw", "CHMU": "rw",

	// C0-DF: longword, processor registers, and the PSL.
	"ADDL2": "rl,ml", "ADDL3": "rl,rl,wl", "SUBL2": "rl,ml", "SUBL3": "rl,rl,wl",
	"MULL2": "rl,ml", "MULL3": "rl,rl,wl", "DIVL2": "rl,ml", "DIVL3": "rl,rl,wl",
	"BISL2": "rl,ml", "BISL3": "rl,rl,wl", "BICL2": "rl,ml", "BICL3": "rl,rl,wl",
	"XORL2": "rl,ml", "XORL3": "rl,rl,wl", "MNEGL": "rl,wl", "CASEL": "rl,rl,rl",
	"MOVL": "rl,wl", "CMPL": "rl,rl", "MCOML": "rl,wl", "BITL": "rl,rl",
	"CLRL": "wl", "TSTL": "rl", "INCL": "ml", "DECL": "ml",
	"ADWC": "rl,ml", "SBWC": "rl,ml", "MTPR": "rl,rl", "MFPR": "rl,wl",
	"MOVPSL": "wl", "PUSHL": "rl", "MOVAL": "al,wl", "PUSHAL": "al",

	// E0-FF: bit branches, bit fields, loops, calls, and the rest.
	"BBS": "rl,vb,bb", "BBC": "rl,vb,bb", "BBSS": "rl,vb,bb", "BBCS": "rl,vb,bb",
	"BBSC": "rl,vb,bb", "BBCC": "rl,vb,bb", "BBSSI": "rl,vb,bb", "BBCCI": "rl,vb,bb",
	"BLBS": "rl,bb", "BLBC": "rl,bb",
	"FFS": "rl,rb,vb,wl", "FFC": "rl,rb,vb,wl", // startpos, size, base, findpos
	"CMPV": "rl,rb,vb,rl", "CMPZV": "rl,rb,vb,rl", // pos, size, base, src
	"EXTV": "rl,rb,vb,wl", "EXTZV": "rl,rb,vb,wl", // pos, size, base, dst
	"INSV":   "rl,rl,rb,vb", // src, pos, size, base
	"ACBL":   "rl,rl,ml,bw",
	"AOBLSS": "rl,ml,bb", "AOBLEQ": "rl,ml,bb",
	"SOBGEQ": "ml,bb", "SOBGTR": "ml,bb",
	"CVTLB": "rl,wb", "CVTLW": "rl,ww",
	"ASHP":  "rb,rw,ab,rb,rw,ab", // cnt, srclen, srcaddr, round, dstlen, dstaddr
	"CVTLP": "rl,rw,ab",          // src, dstlen, dstaddr
	"CALLG": "ab,ab",             // arglist, dst
	"CALLS": "rl,ab",             // numarg, dst
	"XFC":   "ib",                // govax: the RTL shim selector byte (see the file comment)

	// FD-prefixed (two-byte opcodes): conversions among the four floating
	// formats.
	"CVTDH": "rd,wh", "CVTGF": "rg,wf", "CVTGH": "rg,wh",
	"CVTFH": "rf,wh", "CVTFG": "rf,wg", "CVTHF": "rh,wf", "CVTHD": "rh,wd",

	// FD40-FD56: G_floating.
	"ADDG2": "rg,mg", "ADDG3": "rg,rg,wg", "SUBG2": "rg,mg", "SUBG3": "rg,rg,wg",
	"MULG2": "rg,mg", "MULG3": "rg,rg,wg", "DIVG2": "rg,mg", "DIVG3": "rg,rg,wg",
	"CVTGB": "rg,wb", "CVTGW": "rg,ww", "CVTGL": "rg,wl", "CVTRGL": "rg,wl",
	"CVTBG": "rb,wg", "CVTWG": "rw,wg", "CVTLG": "rl,wg", "ACBG": "rg,rg,mg,bw",
	"MOVG": "rg,wg", "CMPG": "rg,rg", "MNEGG": "rg,wg", "TSTG": "rg",
	"EMODG": "rg,rw,rg,wl,wg", // G and H take a word extension; F and D a byte
	"POLYG": "rg,rw,ab",

	// FD60-FD76: H_floating.
	"ADDH2": "rh,mh", "ADDH3": "rh,rh,wh", "SUBH2": "rh,mh", "SUBH3": "rh,rh,wh",
	"MULH2": "rh,mh", "MULH3": "rh,rh,wh", "DIVH2": "rh,mh", "DIVH3": "rh,rh,wh",
	"CVTHB": "rh,wb", "CVTHW": "rh,ww", "CVTHL": "rh,wl", "CVTRHL": "rh,wl",
	"CVTBH": "rb,wh", "CVTWH": "rw,wh", "CVTLH": "rl,wh", "ACBH": "rh,rh,mh,bw",
	"MOVH": "rh,wh", "CMPH": "rh,rh", "MNEGH": "rh,wh", "TSTH": "rh",
	"EMODH": "rh,rw,rh,wl,wh",
	"POLYH": "rh,rw,ab",
	"CVTHG": "rh,wg",

	// FD7C-FD7F: octaword. The manual gives each opcode two names, one per
	// data type of the same size (CLRO and CLRH, MOVAO and MOVAH, PUSHAO
	// and PUSHAH); the table uses the octaword name, and the assembler
	// accepts the other as an alias.
	"CLRO": "wo", "MOVO": "ro,wo", "MOVAO": "ao,wl", "PUSHAO": "ao",

	// FF-prefixed: the BUGW/BUGL bug checks, followed by a word or
	// longword of data that is not an operand specifier.
	"BUGL": "il", "BUGW": "iw",
}

// manualCorrections lists the instruction_table.h entries whose operands
// disagree with the manual, where the manual wins. The generator refuses to
// run if an entry disagrees and isn't listed here, so each correction is a
// deliberate, documented choice. (Older corrections, from before this
// table existed, are in main.go's knownTableFixes, which run first.) Each is
// also recorded in docs/DEVIATIONS.md under Phase 35.
var manualCorrections = map[string]string{
	// The C header marks these conversions' destinations as modify
	// (read, then written) rather than write. A modify operand is read
	// before the instruction runs, so a destination that can't be read,
	// or one that a fault interrupts, behaves differently from the manual.
	"CVTWL": "dst is .wl, not .ml",
	"CVTWB": "dst is .wb, not .mb",
	"CVTBL": "dst is .wl, not .ml",
	"CVTBW": "dst is .ww, not .mw",
	"CVTLB": "dst is .wb, not .mb",
	"CVTLW": "dst is .ww, not .mw",

	// The same slip as BISB3 (knownTableFixes): one Bxx3 instruction with a
	// longword destination, where every sibling's is the operand size.
	"BISW3": "dst is .ww, not .wl",

	// Address operands sized as longwords where the manual has bytes.
	// The size of an address operand decides how far "(Rn)+" advances Rn
	// and how "[Rx]" scales the index: CALLG (R1)[R2] must use R1+R2,
	// not R1+4*R2.
	"PROBER": "base is .ab, not .al",
	"PROBEW": "base is .ab, not .al",
	"INSQUE": "entry and pred are .ab, not .al",
	"REMQUE": "entry is .ab, not .al",
	"CALLG":  "arglist and dst are .ab, not .al",
	"CALLS":  "dst is .ab, not .al",

	// ACBF's operands are right, but the C header gives it an integer
	// short-literal type, so "ACBF S^#10.0, S^#1.0, R0, LOOP" read its
	// limit and increment as the integers 40 and 8, not 10.0 and 1.0
	// (and then as floating values those bit patterns don't represent).
	// Its siblings ACBD and ACBG have the floating type.
	"ACBF": "short-literal type is floating, not integer",
}

// missingInstructions lists the instructions instruction_table.h doesn't
// have at all, with their opcodes. All are two-byte opcodes in the FD page
// (the first byte, 0xFD, selects a second opcode table). Their operands are
// in manualOperands.
var missingInstructions = []struct {
	name     string
	ext, opc int
}{
	{"CVTGF", 0xFD, 0x33},
	{"ADDH2", 0xFD, 0x60}, {"ADDH3", 0xFD, 0x61}, {"SUBH2", 0xFD, 0x62}, {"SUBH3", 0xFD, 0x63},
	{"MULH2", 0xFD, 0x64}, {"MULH3", 0xFD, 0x65}, {"DIVH2", 0xFD, 0x66}, {"DIVH3", 0xFD, 0x67},
	{"CVTHB", 0xFD, 0x68}, {"CVTHW", 0xFD, 0x69}, {"CVTHL", 0xFD, 0x6A}, {"CVTRHL", 0xFD, 0x6B},
	{"CVTBH", 0xFD, 0x6C}, {"CVTWH", 0xFD, 0x6D}, {"CVTLH", 0xFD, 0x6E}, {"ACBH", 0xFD, 0x6F},
	{"MOVH", 0xFD, 0x70}, {"CMPH", 0xFD, 0x71}, {"MNEGH", 0xFD, 0x72}, {"TSTH", 0xFD, 0x73},
	{"EMODH", 0xFD, 0x74}, {"POLYH", 0xFD, 0x75}, {"CVTHG", 0xFD, 0x76},
	{"CLRO", 0xFD, 0x7C}, {"MOVO", 0xFD, 0x7D}, {"MOVAO", 0xFD, 0x7E}, {"PUSHAO", 0xFD, 0x7F},
	{"CVTFG", 0xFD, 0x99},
	{"CVTHD", 0xFD, 0xF7},
}
