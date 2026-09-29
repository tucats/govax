package obj

import (
	"fmt"

	"github.com/tucats/govax/internal/vmsdef"
)

// objConst returns a generated VAX object language constant. A missing
// name is a programming error in this package, so it panics at package
// initialization rather than quietly using zero.
func objConst(name string) uint32 {
	v, ok := vmsdef.OBJConstants[name]
	if !ok {
		panic("obj: no object language constant " + name)
	}

	return v
}

// RecordType is an object record's first byte (OBJ$C_xxx).
type RecordType byte

// The record types.
var (
	RecHDR  = RecordType(objConst("OBJ$C_HDR"))
	RecGSD  = RecordType(objConst("OBJ$C_GSD"))
	RecTIR  = RecordType(objConst("OBJ$C_TIR"))
	RecEOM  = RecordType(objConst("OBJ$C_EOM"))
	RecDBG  = RecordType(objConst("OBJ$C_DBG"))
	RecTBT  = RecordType(objConst("OBJ$C_TBT"))
	RecLNK  = RecordType(objConst("OBJ$C_LNK"))
	RecEOMW = RecordType(objConst("OBJ$C_EOMW"))
)

// Limits of the object language.
var (
	// MaxRecordSize is the longest record the linker accepts
	// (OBJ$C_MAXRECSIZ), and the usual MHD maximum record size.
	MaxRecordSize = int(objConst("OBJ$C_MAXRECSIZ"))
	// MaxNameLength is the longest symbol, psect, or module name
	// (OBJ$C_SYMSIZ).
	MaxNameLength = int(objConst("OBJ$C_SYMSIZ"))
	// StructureLevel is the MHD structure level (OBJ$C_STRLVL).
	StructureLevel = byte(objConst("OBJ$C_STRLVL"))
	// MaxPsectAlignment is the largest psect alignment, as a power of two
	// (OBJ$C_PSCALILIM: page alignment).
	MaxPsectAlignment = byte(objConst("OBJ$C_PSCALILIM"))
)

func (t RecordType) String() string {
	switch t {
	case RecHDR:
		return "HDR"
	case RecGSD:
		return "GSD"
	case RecTIR:
		return "TIR"
	case RecEOM:
		return "EOM"
	case RecDBG:
		return "DBG"
	case RecTBT:
		return "TBT"
	case RecLNK:
		return "LNK"
	case RecEOMW:
		return "EOMW"
	}

	return fmt.Sprintf("record type %d", byte(t))
}

// HeaderType is a header record's second byte (MHD$C_xxx).
type HeaderType byte

// The header types. Only MHD and LNM are required; the rest hold text the
// linker ignores but ANALYZE/OBJECT displays.
var (
	HdrMHD = HeaderType(objConst("MHD$C_MHD"))
	HdrLNM = HeaderType(objConst("MHD$C_LNM"))
	HdrSRC = HeaderType(objConst("MHD$C_SRC"))
	HdrTTL = HeaderType(objConst("MHD$C_TTL"))
	HdrCPR = HeaderType(objConst("MHD$C_CPR"))
	HdrMTC = HeaderType(objConst("MHD$C_MTC"))
	HdrGTX = HeaderType(objConst("MHD$C_GTX"))
)

func (t HeaderType) String() string {
	switch t {
	case HdrMHD:
		return "MHD"
	case HdrLNM:
		return "LNM"
	case HdrSRC:
		return "SRC"
	case HdrTTL:
		return "TTL"
	case HdrCPR:
		return "CPR"
	case HdrMTC:
		return "MTC"
	case HdrGTX:
		return "GTX"
	}

	return fmt.Sprintf("header type %d", byte(t))
}

// GSDType is a GSD subrecord's first byte (GSD$C_xxx).
type GSDType byte

// The GSD subrecord types.
var (
	GSDPsect         = GSDType(objConst("GSD$C_PSC"))
	GSDSymbol        = GSDType(objConst("GSD$C_SYM"))
	GSDEntry         = GSDType(objConst("GSD$C_EPM"))
	GSDProcedure     = GSDType(objConst("GSD$C_PRO"))
	GSDSymbolW       = GSDType(objConst("GSD$C_SYMW"))
	GSDEntryW        = GSDType(objConst("GSD$C_EPMW"))
	GSDProcedureW    = GSDType(objConst("GSD$C_PROW"))
	GSDIdentCheck    = GSDType(objConst("GSD$C_IDC"))
	GSDEnvironment   = GSDType(objConst("GSD$C_ENV"))
	GSDLocalSymbol   = GSDType(objConst("GSD$C_LSY"))
	GSDLocalEntry    = GSDType(objConst("GSD$C_LEPM"))
	GSDLocalProc     = GSDType(objConst("GSD$C_LPRO"))
	GSDSharedPsect   = GSDType(objConst("GSD$C_SPSC"))
	GSDSymbolV       = GSDType(objConst("GSD$C_SYMV"))
	GSDEntryV        = GSDType(objConst("GSD$C_EPMV"))
	GSDProcedureV    = GSDType(objConst("GSD$C_PROV"))
	GSDSymbolM       = GSDType(objConst("GSD$C_SYMM"))
	GSDEntryM        = GSDType(objConst("GSD$C_EPMM"))
	GSDProcedureM    = GSDType(objConst("GSD$C_PROM"))
	gsdTypeNames     = map[GSDType]string{}
	gsdTypeNameOrder = []struct {
		t    GSDType
		name string
	}{
		{GSDPsect, "PSC"}, {GSDSymbol, "SYM"}, {GSDEntry, "EPM"},
		{GSDProcedure, "PRO"}, {GSDSymbolW, "SYMW"}, {GSDEntryW, "EPMW"},
		{GSDProcedureW, "PROW"}, {GSDIdentCheck, "IDC"},
		{GSDEnvironment, "ENV"}, {GSDLocalSymbol, "LSY"},
		{GSDLocalEntry, "LEPM"}, {GSDLocalProc, "LPRO"},
		{GSDSharedPsect, "SPSC"}, {GSDSymbolV, "SYMV"}, {GSDEntryV, "EPMV"},
		{GSDProcedureV, "PROV"}, {GSDSymbolM, "SYMM"}, {GSDEntryM, "EPMM"},
		{GSDProcedureM, "PROM"},
	}
)

func init() {
	for _, e := range gsdTypeNameOrder {
		gsdTypeNames[e.t] = e.name
	}
}

func (t GSDType) String() string {
	if name, ok := gsdTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("GSD type %d", byte(t))
}

// Psect flags (GPS$M_xxx), in a psect definition's flags word.
var (
	PsectPIC   = uint16(objConst("GPS$M_PIC"))   // position independent
	PsectLIB   = uint16(objConst("GPS$M_LIB"))   // from a shareable image
	PsectOVR   = uint16(objConst("GPS$M_OVR"))   // overlaid (else concatenated)
	PsectREL   = uint16(objConst("GPS$M_REL"))   // relocatable (else absolute)
	PsectGBL   = uint16(objConst("GPS$M_GBL"))   // global (else local)
	PsectSHR   = uint16(objConst("GPS$M_SHR"))   // shareable
	PsectEXE   = uint16(objConst("GPS$M_EXE"))   // executable
	PsectRD    = uint16(objConst("GPS$M_RD"))    // readable
	PsectWRT   = uint16(objConst("GPS$M_WRT"))   // writable
	PsectVEC   = uint16(objConst("GPS$M_VEC"))   // contains a change-mode vector
	PsectNOMOD = uint16(objConst("GPS$M_NOMOD")) // not stored into (demand zero)
	PsectCOM   = uint16(objConst("GPS$M_COM"))   // a C common block

	psectFlagNames = []struct {
		bit        *uint16
		set, clear string
	}{
		{&PsectPIC, "PIC", "NOPIC"}, {&PsectLIB, "LIB", ""},
		{&PsectOVR, "OVR", "CON"}, {&PsectREL, "REL", "ABS"},
		{&PsectGBL, "GBL", "LCL"}, {&PsectSHR, "SHR", "NOSHR"},
		{&PsectEXE, "EXE", "NOEXE"}, {&PsectRD, "RD", "NORD"},
		{&PsectWRT, "WRT", "NOWRT"}, {&PsectVEC, "VEC", "NOVEC"},
		{&PsectNOMOD, "NOMOD", ""}, {&PsectCOM, "COM", ""},
	}
)

// Symbol flags (GSY$M_xxx), in a symbol subrecord's flags word.
var (
	SymWEAK = uint16(objConst("GSY$M_WEAK")) // weak definition or reference
	SymDEF  = uint16(objConst("GSY$M_DEF"))  // a definition (else a reference)
	SymUNI  = uint16(objConst("GSY$M_UNI"))  // universal (for shareable images)
	SymREL  = uint16(objConst("GSY$M_REL"))  // relocatable (else absolute)
	SymCOMM = uint16(objConst("GSY$M_COMM")) // a C common globaldef

	symFlagNames = []struct {
		bit  *uint16
		name string
	}{
		{&SymWEAK, "WEAK"}, {&SymDEF, "DEF"}, {&SymUNI, "UNI"},
		{&SymREL, "REL"}, {&SymCOMM, "COMM"},
	}
)

// EOM completion codes (EOM$C_xxx): the worst error the language processor
// found.
var (
	SeveritySuccess = byte(objConst("EOM$C_SUCCESS"))
	SeverityWarning = byte(objConst("EOM$C_WARNING"))
	SeverityError   = byte(objConst("EOM$C_ERROR"))
	SeverityAbort   = byte(objConst("EOM$C_ABORT"))
)

// EOMWeakTransfer is the EOM transfer flag bit marking a weak transfer
// address (EOM$M_WKTFR).
var EOMWeakTransfer = byte(objConst("EOM$M_WKTFR"))
