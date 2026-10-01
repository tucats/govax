package vmsdef

import (
	"sort"
	"strings"
)

// Symbols (symbols_generated.go) holds every VMS symbolic constant govax
// knows, in one table. Each VMS definition file names its symbols with
// its own prefix, so the prefix says where a name comes from and no two
// files' names collide:
//
//   - FAB$, RAB$, RMS$_: $FABDEF, $RABDEF, and $RMSDEF's field offsets
//     (FAB$L_STS, ...), bit numbers, bitmask flags, named code values, and
//     RMS$_ completion codes; what .RMSDEF (internal/asm) defines as
//     assembler symbols. FABFields and RABFields describe the same fields
//     for .FAB and .RAB, at the same offsets (TestFields_matchSymbols).
//   - NAM$: $NAMDEF's name block: field offsets, bits, and codes.
//   - XAB$: $XABDEF's extended attribute blocks: the common fields, and
//     those of the XABALL, XABDAT, XABFHC, XABITM, XABKEY, XABPRO, XABRDT,
//     XABSUM, and XABTRM blocks. XABKEY's, XABSUM's, and XABITM's came from
//     real MACRO's output (testdata/mar/rms/defined.txt, docs/PHASE-32.md).
//   - SS$_: $SSDEF's system-service completion codes, exactly as
//     STARLET.OLB's SYS$SSDEF defines them (docs/PHASE-31.md).
//   - LNM$: $LNMDEF's attribute bits, limits, and item codes. LNM$_CHAIN
//     is -1, stored as its 32-bit two's complement.
//   - DEV$: $DEVDEF's device-characteristics bits, for both the DEVCHAR
//     and DEVCHAR2 longwords. The two are separate union members in VMS's
//     definition, so both number their bits from 0 (DEV$M_CLU is 1).
//   - JPI$: $JPIDEF's $GETJPI item codes, JPI$K_ values such as the
//     JPI$_MODE codes, and JPI$C_ and JPI$M_ definitions.
//   - IO$: $IODEF's $QIO function codes and function-modifier bits. The
//     modifiers of different device classes are separate union members,
//     so the same bit can have several names (IO$M_NOECHO and
//     IO$M_CANCTRLO are both bit 6).
//   - SCH$C_: $STATEDEF's scheduling states, what JPI$_STATE returns.
//   - SYI$: $SYIDEF's $GETSYI item codes and SYI$C_ values.
//   - DVI$: $DVIDEF's $GETDVI item codes, item-code flags
//     (DVI$M_SECONDARY), and DVI$C_ values.
//   - TT$, TT2$: $TTDEF's terminal characteristics, in a terminal's
//     DEVDEPEND and DEVDEPEND2 longwords, and TT$C_ values such as the
//     line speeds.
//   - PRT$C_: $PRTDEF's page protection codes, in the VAX page table
//     entry's encoding.
//   - PRV$: $PRVDEF's privilege bit numbers, which may be past bit 31 of
//     the quadword privilege mask; PRV$K_NUMBER_OF_PRIVS is the count.
//   - BRK$: $BRKDEF's $BRKTHRU send types, requestor classes, and flags.
//   - FIB$: $FIBDEF's file information block field offsets, bits, and
//     codes: the disk ACP's $QIO argument.
//   - ATR$: $ATRDEF's disk ACP attribute codes, their sizes, and the
//     layout of an attribute list's entries.
//   - OBJ$, MHD$, GSD$, GPS$, GSY$, SDF$, SRF$, EPM$, TIR$, EOM$, and the
//     rest of the VAX object language ($OBJRECDEF through $TIRDEF): record
//     and subrecord types, TIR commands, flag bits, and field offsets. Only
//     the VAX modules are included, not the Alpha ones. $OBJRECDEF's
//     SDA-only aggregate puts its flag bitfields directly in a union, so
//     every OBJ$V_PSC_ and OBJ$V_SYM_ bit is 0: use the GPS$ and GSY$ bits
//     instead. See docs/PHASE-27.md.

// SymbolNames returns, sorted, the names in Symbols that begin with any of
// prefixes.
func SymbolNames(prefixes ...string) []string {
	var names []string

	for name := range Symbols {
		for _, p := range prefixes {
			if strings.HasPrefix(name, p) {
				names = append(names, name)

				break
			}
		}
	}

	sort.Strings(names)

	return names
}
