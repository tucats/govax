package asm

import "github.com/tucats/govax/internal/cpu"

// builtinSymbols is the fixed set of predefined, permanent system symbols
// every assembly starts with — the literal name/value table half of
// init_symbols.c's init_system_symbols(). The other half (one
// "OPC$_<mnemonic>" symbol per defined single-byte opcode) is generated
// from the live instruction table instead, in seedBuiltinSymbols below.
var builtinSymbols = map[string]uint32{
	// XFC function codes.
	"XFC$CONSOLE_WRITE": 1, "XFC$CONSOLE_READ": 2, "XFC$CONSOLE_CMD": 3,
	"XFC$VMR": 0x7F, "XFC$VMW": 0x7E, "XFC$SHIM": 0x7D, "XFC$HALT": 0x7C,
	"XFC$HALT_SILENT": 0x7B, "XFC$P1VECTOR": 0x7A, "XFC$DCL": 0x79,
	"XFC$QUIT_EMULATION": 0x78,

	// DCL console callbacks.
	"DCL$_QUALIFIER": 'Q', "DCL$_VERB": 'V', "DCL$_PARAMETER": 'P',

	// PTE protection flags.
	"PTE$K_NA": 0, "PTE$K_RESERVED": 1, "PTE$K_KW": 2, "PTE$K_KR": 3,
	"PTE$K_UW": 4, "PTE$K_EW": 5, "PTE$K_ERKW": 6, "PTE$K_ER": 7,
	"PTE$K_SW": 8, "PTE$K_SREW": 9, "PTE$K_SRKW": 10, "PTE$K_SR": 11,
	"PTE$K_URSW": 12, "PTE$K_UREW": 13, "PTE$K_URKW": 14, "PTE$K_UR": 15,
	"PTE$K_ALL": 4, "PTE$K_NONE": 0, "PTE$K_READONLY": 15,

	// Privileged register numbers.
	"VAX$PR_KSP": 0, "VAX$PR_ESP": 1, "VAX$PR_SSP": 2, "VAX$PR_USP": 3, "VAX$PR_ISP": 4,
	"VAX$PR_P0BR": 8, "VAX$PR_P0LR": 9, "VAX$PR_P1BR": 10, "VAX$PR_P1LR": 11,
	"VAX$PR_SBR": 12, "VAX$PR_SLR": 13,
	"VAX$PR_PCBB": 16, "VAX$PR_SCBB": 17, "VAX$PR_IPL": 18, "VAX$PR_ASTLVL": 19,
	"VAX$PR_SIRR": 20, "VAX$PR_SISR": 21,
	"VAX$PR_ICCS": 24, "VAX$PR_NICR": 25, "VAX$PR_ICR": 26, "VAX$PR_TODR": 27,
	"VAX$PR_RXCS": 32, "VAX$PR_RXDB": 33, "VAX$PR_TXCS": 34, "VAX$PR_TXDB": 35, "VAX$PR_TBDR": 36,
	"VAX$PR_MAPEN": 56, "VAX$PR_TBIA": 57, "VAX$PR_TBIS": 58,
	"VAX$PR_PMR": 61, "VAX$PR_SID": 62, "VAX$PR_TBCHK": 63,

	// EDITPC operators.
	"EO$INSERT": 0x44, "EO$STORE_SIGN": 0x04, "EO$FILL": 0x80, "EO$MOVE": 0x90,
	"EO$FLOAT": 0xA0, "EO$END_FLOAT": 0x01, "EO$BLANK_ZERO": 0x45, "EO$REPLACE_SIGN": 0x46,
	"EO$LOAD_FILL": 0x40, "EO$LOAD_SIGN": 0x41, "EO$LOAD_PLUS": 0x42, "EO$LOAD_MINUS": 0x43,
	"EO$CLEAR_SIGNIF": 0x02, "EO$SET_SIGNIF": 0x03, "EO$ADJUST_INPUT": 0x47, "EO$END": 0x00,

	// Arithmetic traps and faults.
	"SRM$K_INT_OVF_T": 0x01, "SRM$K_INT_DIV_T": 0x02, "SRM$K_FLT_OVF_T": 0x03,
	"SRM$K_FLT_DIV_T": 0x04, "SRM$K_FLT_UND_T": 0x05, "SRM$K_DEC_OVF_T": 0x06,
	"SRM$K_SUB_RNG_T": 0x07,
	"SRM$K_FLT_OVF_F": 0x08, "SRM$K_FLT_DIV_F": 0x09, "SRM$K_FLT_UND_F": 0x0A,

	// Exception vectors (SCBB offsets), matching vax.h's EXC_* constants —
	// the subset internal/cpu's Exception type doesn't already define
	// (Phases 04-07 only needed the CPU-fault-raised subset; these also
	// include interrupt-only and console vectors kernel.asm's .SCB table
	// references).
	"EXC$UNUSED": 0x00, "EXC$CHECK": 0x04, "EXC$KSNV": 0x08, "EXC$POWER": 0x0C,
	"EXC$PRIV": 0x10, "EXC$CUSTOMER": 0x14, "EXC$RESOP": 0x18, "EXC$RESADDR": 0x1C,
	"EXC$ACCVIO": 0x20, "EXC$TNV": 0x24, "EXC$TP": 0x28, "EXC$BPT": 0x2C,
	"EXC$COMPAT": 0x30, "EXC$ARITH": 0x34,
	"EXC$CHMK": 0x40, "EXC$CHME": 0x44, "EXC$CHMS": 0x48, "EXC$CHMU": 0x4C,
	"EXC$SBI": 0x50, "EXC$CMRD": 0x54, "EXC$SBIALERT": 0x58, "EXC$SBIFAULT": 0x5C,
	"EXC$MWT":       0x60,
	"EXC$SOFTWARE1": 0x84, "EXC$SOFTWARE2": 0x88, "EXC$SOFTWARE3": 0x8C, "EXC$SOFTWARE4": 0x90,
	"EXC$SOFTWARE5": 0x94, "EXC$SOFTWARE6": 0x98, "EXC$SOFTWARE7": 0x9C, "EXC$SOFTWARE8": 0xA0,
	"EXC$SOFTWARE9": 0xA4, "EXC$SOFTWARE10": 0xA8, "EXC$SOFTWARE11": 0xAC, "EXC$SOFTWARE12": 0xB0,
	"EXC$SOFTWARE13": 0xB4, "EXC$SOFTWARE14": 0xB8, "EXC$SOFTWARE15": 0xBC,
	"EXC$INTERVAL": 0xC0, "EXC$CONREAD": 0xF8, "EXC$CONWRITE": 0xFC,

	"CONSOLE$HANDLER": 0xFFFFFFFF,
	"$STATUS":         0,
}

// seedBuiltinSymbols populates a fresh Assembler's symbol table with the
// permanent system symbols the reference tool defines at startup —
// init_symbols.c's init_system_symbols(). Every testdata/asm fixture that
// references one of these (kernel.asm's .SCB table, for instance, uses
// EXC$CHMK/EXC$PRIV/... as vector codes) expects it to already exist
// without a corresponding .SET in the source, the same way it would in a
// freshly booted reference-tool console session — see docs/PHASE-11.md.
func (a *Assembler) seedBuiltinSymbols() {
	for name, value := range builtinSymbols {
		sym := a.symbols.create(name)
		sym.value = value
		sym.flags |= SymPermanent | SymBuiltin
	}

	table := cpu.Instructions()
	for i := 0; i < 256; i++ {
		inst := table.Lookup(cpu.Opcode{Function: byte(i)})
		if inst == nil {
			continue
		}
		
		sym := a.symbols.create("OPC$_" + inst.Name)
		sym.value = uint32(i)
		sym.flags |= SymPermanent | SymBuiltin
	}
}
