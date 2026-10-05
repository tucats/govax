# VMS Debug Symbol Table (DST) Records

A clean-room reference to the Debug Symbol Table format that VAX and Alpha
VMS compilers emit and that the VMS debugger and traceback facility consume.
Symbol names and numeric values precise; all other text are newly written
requirements specifications.

**Contents:**

1. Overview
2. Locating the DST
3. Overall structure
4. Record header and code tables
5. Scope records
6. Record types and variants
7. Data symbol records
8. Value specifications and the stack machine
9. Type specifications
10. Enumerations
11. BLISS records
12. Image, PSECT, label and entry records
13. Line-number program
14. Source correlation
15. GEM locators
16. Continuation
17. GOTO/TARGET
18. Fixups
19. Prolog, epilog and register records
20. Ada records
21. C++ records
22. Obsolete records
23. Implementation notes

All multi-byte fields are little-endian. "byte", "word", "long" and "quad"
mean 1, 2, 4 and 8 bytes. Offsets in the layout tables are measured from
the start of the record, at its length byte, unless a table says otherwise.

---

## 1. Overview

### 1.1 What the DST is for

The DST is a symbol table that does not depend on the source language. Every
compiler that supports the debugger writes symbol information in the same
binary form, whatever the source language. The pipeline is:

1. **Compiler**: writes DST data into the object module.
2. **Linker**: relocates the data, resolves global references, and copies it
   into the executable image. Apart from knowing which fields need
   relocation, the linker does not interpret the DST.
3. **Debugger / Traceback**: reads the DST from the image at run time.

The DST can describe:

- program structure (modules, routines, blocks), plus labels and data objects, and how these nest
- line-number to PC mappings, and source file and line correlation
- data types of any complexity: records with variants, enumerations,
  arrays, pointers, and so on
- value and address computations of any complexity, including calls to
  compiler-generated thunks

The design assumes compiled code that runs as native instructions and data
objects that have real addresses. It does not support interpreted
languages, because breakpoints and stepping rely on native code.

### 1.2 Traceback (TBT) vs. symbol (DBT) records

A compiler emits DST data in two kinds of object-language records:

| Object record | Contents | Copied to the image when |
| - | - | - |
| **TBT** (traceback) | Module Begin/End, Routine Begin/End, Block Begin/End, Line Number PC-Correlation, optionally Version Number | default `LINK` and `LINK/DEBUG` |
| **DBT** (debug) | All other DST records | only `LINK/DEBUG` |

`LINK/NOTRACEBACK` drops both kinds. DBT data cannot be kept without TBT
data, because the scope records in the TBT set give the symbol records
their meaning.

Typical compiler qualifiers:

| Qualifier | Effect |
| - | - |
| `/DEBUG` (= `/DEBUG=(TRACEBACK,SYMBOLS)`) | everything |
| `/DEBUG=TRACEBACK` | TBT records only |
| `/DEBUG=(NOTRACE,NOSYMBOL)` | nothing |
| `/DEBUG=(NOTRACE,SYMBOLS)` | everything except line-number PC-correlation |

Scope records (module, routine, block) are always emitted whenever any
symbol information is emitted.

### 1.3 Companion tables: GST and DMT

When you link with `/DEBUG`, the linker writes two more tables:

- **GST (Global Symbol Table)**: one entry per global symbol the linker
  knows about. The debugger falls back on it when no DST information is
  available. It has no type information.
- **DMT (Debug Module Table)**: a compact index into the DST with one entry
  per module (see §2.3). The debugger reads it to build its module table and
  static-address map without scanning the whole DST.

---

## 2. Locating the DST in an image file

### 2.1 Image header pointer

The image header has a word field, `IHD$W_SYMDBGOFF`, at a fixed position:
the low word of the second longword. It holds the byte offset, from the
start of the header, of the **Image Header Symbol Table Descriptor (IHS)**.

### 2.2 IHS layout

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | long | `IHS$L_DSTVBN` | Virtual block number where the DST starts |
| 4 | long | `IHS$L_GSTVBN` | Virtual block number where the GST starts |
| 8 | word | `IHS$W_DSTBLKS` | DST size in 512-byte blocks (16-bit) |
| 10 | word | `IHS$W_GSTRECS` | GST size in records (16-bit) |
| 12 | long | `IHS$L_DMTVBN` | Virtual block number where the DMT starts (0 if none) |
| 16 | long | `IHS$L_DMTBYTES` | DMT size in bytes (0 if none) |
| 20 | long | `IHS$L_DSTBLKS` | DST size in blocks (32-bit) |
| 24 | long | `IHS$L_GSTRECS` | GST record count (32-bit) |

Notes:

- The DMT fields exist only in images from the V4.0 or later linker. Bit 5
  of `IHD$L_LNKFLAGS` marks such an image.
- If the header flag `IHD$V_IHSLONG` is set, use the 32-bit size fields
  (`IHS$L_DSTBLKS`, `IHS$L_GSTRECS`). The 16-bit fields are still filled in
  for compatibility, but they hold truncated values and are unreliable.
- If the last DST block is not full, it is padded with zeros.

### 2.3 Debug Module Table (DMT) entry

Each entry has a fixed header followed by a variable number of PSECT
descriptors.

`DBG$DMT_HEADER` (size `DBG$K_DMT_HEADER_SIZE` = 12):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | long | `dbg$l_dmt_modbeg` | Offset of this module's Module Begin record from the start of the DST |
| 4 | long | `dbg$l_dmt_dst_size` | Size in bytes of this module's DST |
| 8 | word | `dbg$w_dmt_psect_count` | Number of PSECT entries that follow |
| 10 | word | `dbg$w_dmt_mbz` | Reserved, must be zero |

`DBG$DMT_PSECT` (size `DBG$K_DMT_PSECT_SIZE` = 8), repeated
`dbg$w_dmt_psect_count` times:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | long | `dbg$l_dmt_psect_start` | PSECT start address |
| 4 | long | `dbg$l_dmt_psect_length` | PSECT length in bytes |

The PSECT array begins at offset `DBG$K_DMT_HEADER_SIZE` (BLISS:
`dbg$a_dmt_psect_base`). The debugger cannot work out how much symbol-table
memory a module needs from the DMT alone. It estimates that by scaling
`dbg$l_dmt_dst_size`.

---

## 3. Overall structure

### 3.1 Record stream

The DST is one contiguous run of variable-length records. Each record
starts with a length byte and a type byte. Modules follow one another with
no gap:

```text
Module Begin (M1) … symbols for M1 … Module End
Module Begin (M2) … symbols for M2 … Module End
…
Module Begin (Mn) … Module End
Fixup records (optional, for shareable-image-relative addresses)
zero padding to the block boundary
```

### 3.2 Nesting through Begin/End pairs

Scopes and composite types are expressed by bracketing records. A record
belongs to the innermost open Begin/End pair. Bracket types:

| Construct | Opening record | Closing record |
| - | - | - |
| Module | `DST$K_MODBEG` | `DST$K_MODEND` |
| Routine | `DST$K_RTNBEG` | `DST$K_RTNEND` |
| Lexical block | `DST$K_BLKBEG` | `DST$K_BLKEND` |
| Ada package spec | `DST$K_PACK_SPEC_BEG` | `DST$K_PACK_SPEC_END` |
| Ada package body | `DST$K_PACK_BODY_BEG` | `DST$K_PACK_BODY_END` |
| Record type | `DST$K_RECBEG` | `DST$K_RECEND` |
| Variant set (inside a record) | `DST$K_VARBEG` | `DST$K_VAREND` |
| Enumeration type | `DST$K_ENUMBEG` | `DST$K_ENUMEND` |
| BLISS field set | `DST$K_BLIFLDBEG` | `DST$K_BLIFLDEND` |
| Register-save group | `DST$K_REG_SAVE_BEGIN` | `DST$K_REG_SAVE_END` |

Routines and blocks can nest inside each other to any reasonable depth.
Example:

```text
Source                           DST stream
------                           ----------
module M                         Module Begin "M"
  var a: integer                   Data "a"   (DTYPE_L)
  routine R1                       Routine Begin "R1"
    var b: boolean                   Data "b"   (BOOL)
    begin  (anonymous)               Block Begin ""
      var c: word                      Data "c" (DTYPE_W)
      routine R2                       Routine Begin "R2"
        L: begin                         Block Begin "L"
          var d: real                      Data "d" (DTYPE_F)
        end                              Block End
      end R2                           Routine End
    end                              Block End
  end R1                           Routine End
end M                            Module End
```

Some record kinds ignore nesting below module level. Line-number
PC-correlation records and source-file correlation records can appear
anywhere in a module, and the debugger treats them as describing the module
as a whole.

### 3.3 Types and objects

A data object is described by three things: a **name** (counted ASCII), a
**value/address**, and a **type**. Scope comes from where the record sits
in the stream.

- **Value/address.** The common case is a compact 5-byte encoding (a flags
  byte plus a longword, see §6.1). Special flag values escape to longer value
  specifications later in the same record.
- **Type.** The simple case is one type byte: the record's own `DST$B_TYPE`
  holds a standard data-type code. For more complex types:
  - use a descriptor, either in user memory or embedded in the DST, or
  - emit a **Separate Type Specification** record (`DST$K_SEPTYP`) for the
    object. The record that immediately follows it gives the type. That
    record must be a Record Begin, an Enumeration Begin, or a Type
    Specification record (`DST$K_TYPSPEC`). A Type Specification record can
    hold an Indirect Type Spec that points back to an earlier type
    definition.

Example of declaring objects of a record type:

```text
Data REC1 (SEPTYP)            <- REC1's type is the record that follows
Record Begin "RECTYP"  (VFLAGS = NOVAL, i.e. a type, not an object)
  Data COMP1 (DTYPE_L)
  Data COMP2 (DTYPE_F)
Record End
Data REC2 (SEPTYP)
Type Spec (Indirect -> Record Begin "RECTYP")
```

Variant records and enumerations work the same way (see §5.4 and §8).

---

## 4. The common record header

### 4.1 Layout

Every DST record begins with `DST$HEADER`:

| Offset | Size | Field (alias) | Meaning |
| - | - | - | - |
| 0 | byte | `DST$B_LENGTH` (`DST$X_LENGTH`) | Number of bytes in the record **after** this length byte |
| 1 | byte | `DST$B_TYPE` (`DST$X_TYPE`) | Record type code (type `DST$DTYPE`, unsigned byte) |
| 2 | var | `DST$A_NEXT` | Type-specific body |

Header-related constants:

| Constant | Value | Meaning |
| - | - | - |
| `DST$K_DST_HEADER_SIZE` | 2 | Size of the header |
| `DST$K_DST_BASE_FOR_NEXT` | 1 | Offset from which `DST$B_LENGTH` is counted |
| `DST$K_LENGTH_LENGTH` | 1 | Size of the length field |
| `DST$K_LENGTH_TYPE` | 1 | Size of the type field |

**Walking the stream:** `next_record = this_record + DST$K_DST_BASE_FOR_NEXT + DST$B_LENGTH`.
So the total size of a record is `DST$B_LENGTH + 1`. For example, a record
that is nothing but a header has `DST$B_LENGTH = 1`.

> A few newer record kinds (Unallocated Routine, Inline, Discontiguous Range,
> and others noted below) are drawn in the original with word-sized
> length/type cells (`DST$W_LENGTH`, `DST$W_TYPE`). Their SDL aggregates
> still begin with the ordinary byte `DST$HEADER`, so a reader should parse
> the 2-byte header the same way for every record. The word-sized layout
> appears to belong to an Alpha-specific variant of the definitions.

### 4.2 How the type byte is classified

`DST$B_TYPE` is a single unsigned byte, interpreted by range:

| Range | Meaning |
| - | - |
| `DST$K_BLI` (0) | BLISS special-case record (historical; code 0) |
| `DSC$K_DTYPE_LOWEST` (1) … `DBG$K_MAXIMUM_DTYPE` | Data-type code: the record is a data symbol in Standard Data form or one of its variants, and the code is the symbol's type |
| `DST$K_LOWEST` (116) … `DST$K_HIGHEST` | Special DST record kinds, listed in §4.4 |
| everything else | Unsupported; readers skip the record |

The codes between the last dtype and `DST$K_LOWEST` are reserved. New DST
record kinds take their code by *decrementing* `DST$K_LOWEST`; new codes are
never added at the top. Codes 192–255 are nominally reserved for customer use.

### 4.3 Data-type codes (`DSC$K_DTYPE_*`)

The calling standard defines the standard codes. They are listed here so the
reference is complete.

| Value | Symbol | Data type |
| - | - | - |
| 0 | `DSC$K_DTYPE_Z` | Unspecified (never valid in a DST) |
| 1 | `DSC$K_DTYPE_V` | Aligned bit |
| 2 | `DSC$K_DTYPE_BU` | Unsigned byte |
| 3 | `DSC$K_DTYPE_WU` | Unsigned word |
| 4 | `DSC$K_DTYPE_LU` | Unsigned longword |
| 5 | `DSC$K_DTYPE_QU` | Unsigned quadword |
| 6 | `DSC$K_DTYPE_B` | Signed byte |
| 7 | `DSC$K_DTYPE_W` | Signed word |
| 8 | `DSC$K_DTYPE_L` | Signed longword |
| 9 | `DSC$K_DTYPE_Q` | Signed quadword |
| 10 | `DSC$K_DTYPE_F` | F_floating |
| 11 | `DSC$K_DTYPE_D` | D_floating |
| 12 | `DSC$K_DTYPE_FC` | F_floating complex |
| 13 | `DSC$K_DTYPE_DC` | D_floating complex |
| 14 | `DSC$K_DTYPE_T` | Character text |
| 15 | `DSC$K_DTYPE_NU` | Numeric string, unsigned |
| 16 | `DSC$K_DTYPE_NL` | Numeric string, leading separate sign |
| 17 | `DSC$K_DTYPE_NLO` | Numeric string, leading overpunched sign |
| 18 | `DSC$K_DTYPE_NR` | Numeric string, trailing separate sign |
| 19 | `DSC$K_DTYPE_NRO` | Numeric string, trailing overpunched sign |
| 20 | `DSC$K_DTYPE_NZ` | Numeric string, zoned sign |
| 21 | `DSC$K_DTYPE_P` | Packed decimal |
| 22 | `DSC$K_DTYPE_ZI` | Instruction sequence |
| 23 | `DSC$K_DTYPE_ZEM` | Procedure entry mask |
| 24 | `DSC$K_DTYPE_DSC` | Descriptor (e.g. arrays of dynamic strings) |
| 25 | `DSC$K_DTYPE_OU` | Unsigned octaword |
| 26 | `DSC$K_DTYPE_O` | Signed octaword |
| 27 | `DSC$K_DTYPE_G` | G_floating |
| 28 | `DSC$K_DTYPE_H` | H_floating |
| 29 | `DSC$K_DTYPE_GC` | G_floating complex |
| 30 | `DSC$K_DTYPE_HC` | H_floating complex |
| 31 | `DSC$K_DTYPE_CIT` | COBOL intermediate temporary |
| 32 | `DSC$K_DTYPE_BPV` | Bound procedure value |
| 33 | `DSC$K_DTYPE_BLV` | Bound label value |
| 34 | `DSC$K_DTYPE_VU` | Unaligned bit |
| 35 | `DSC$K_DTYPE_ADT` | Absolute date/time |
| 36 | — | Unused; the debugger does not support it |
| 37 | `DSC$K_DTYPE_VT` | Varying text |
| 38 | `DSC$K_DTYPE_T2` | 16-bit character text |
| 39 | `DSC$K_DTYPE_VT2` | 16-bit varying text |

Range constants: `DSC$K_DTYPE_LOWEST` = 1 and `DSC$K_DTYPE_HIGHEST` = 39.

**Debugger extension codes.** These may appear in a DST but must not be
passed in run-time descriptors to other software:

| Value | Symbol | Data type |
| - | - | - |
| 40 | `DSC$K_DTYPE_TF` | Boolean true/false (length in bits) |
| 41 | `DSC$K_DTYPE_SV` | Signed bit field, aligned |
| 42 | `DSC$K_DTYPE_SVU` | Signed bit field, unaligned |
| 43 | `DSC$K_DTYPE_FIXED` | Fixed-point binary (Ada FIXED, PL/I FIXED BINARY) |
| 44 | `DSC$K_DTYPE_TASK` | Ada task |
| 45 | `DSC$K_DTYPE_AC` | Counted ASCII text |
| 46 | `DSC$K_DTYPE_AZ` | Zero-terminated ASCII text |
| 47 | `DSC$K_DTYPE_M68_S` | Motorola 68881 single (32-bit) |
| 48 | `DSC$K_DTYPE_M68_D` | Motorola 68881 double (64-bit) |
| 49 | `DSC$K_DTYPE_M68_X` | Motorola 68881 extended (96-bit) |
| 50 | `DSC$K_DTYPE_1750_S` | MIL-STD-1750 single (32-bit) |
| 51 | `DSC$K_DTYPE_1750_X` | MIL-STD-1750 extended (48-bit) |
| 52 | `DSC$K_DTYPE_FS` | IEEE single *(now in the system definitions)* |
| 53 | `DSC$K_DTYPE_FT` | IEEE double *(system definitions)* |
| 54 | `DSC$K_DTYPE_FSC` | IEEE single complex *(system definitions)* |
| 55 | `DSC$K_DTYPE_FTC` | IEEE double complex *(system definitions)* |
| 56 | `DSC$K_DTYPE_WC` | Wide character (`wchar_t`), locale-dependent |
| 57 | `DSC$K_DTYPE_FX` | IEEE extended (128-bit) *(system definitions)* |
| 58 | `DSC$K_DTYPE_FXC` | IEEE extended complex *(system definitions)* |

Range constants: `DBG$K_MINIMUM_DTYPE` = 0 and `DBG$K_MAXIMUM_DTYPE` = 58.
The calling-standard body must approve any new data-type code.

These codes are used only inside the debugger and never appear in a DST:

| Value | Symbol | Purpose |
| - | - | - |
| 191 | `DSC$K_DTYPE_LITERAL` | Marks a parsed literal during expression evaluation |
| 192 | `DBG$K_DTYPE_AD` | Descriptor-addressed ASCII text (internal) |

### 4.4 Special DST record type codes (`DST$K_*`)

| Value | Symbol | Record kind |
| - | - | - |
| 0 | `DST$K_BLI` | BLISS special cases |
| 116 | `DST$K_SYMBOL_FIXUP_64` | Symbol fixup (quadword) |
| 117 | `DST$K_FIXUP_64` | DST fixup (quadword) |
| 118 | `DST$K_DIS_RANGE` | Discontiguous address range list |
| 119 | `DST$K_PROLOG_LIST` | Prolog list |
| 120 | `DST$K_RTN_UNALLOC` | Unallocated routine |
| 121 | `DST$K_SYMBOL_FIXUP` | Symbol fixup (longword) |
| 122 | `DST$K_BASE_CLASS` | C++ base class reference |
| 123 | `DST$K_TEMP_DECL` | C++ template declaration |
| 124 | `DST$K_VIRT_FUNC` | C++ virtual function vtable index |
| 125 | `DST$K_EXCEPTION` | Exception number-to-name mapping (XD Ada) |
| 126 | `DST$K_RETURN` | Return-instruction locations (for profiling) |
| 127 | `DST$K_EPILOG` | Routine epilog locations |
| 128 | `DST$K_REG_SAVE_END` | End of register-save group |
| 129 | `DST$K_REG_SAVE` | One saved-register description |
| 130 | `DST$K_REG_SAVE_BEGIN` | Start of register-save group |
| 131 | `DST$K_BLIFLDBEG` | BLISS field set begin |
| 132 | `DST$K_BLIFLDEND` | BLISS field set end |
| 133 | `DST$K_FULFILLS_TYPE` | Fulfills type |
| 134 | `DST$K_FIXUP` | DST fixup (longword) |
| 135 | `DST$K_IMAGE` | Image descriptor (built by the debugger, never by compilers) |
| 136 | `DST$K_INLINE` | Inlined routine instance |
| 137 | `DST$K_EPILOGS` | Routine and inline epilogs |
| 138 | `DST$K_TYPE_SIG` | C++ type signature |
| 139 | `DST$K_EDIT_SOURCE` | Source correlation used by the EDIT command |
| 140 | `DST$K_ALIAS` | Alias |
| 141 | `DST$K_CXX_ATTRIBUTES` | C++ attributes |
| 142 | `DST$K_GOTO` | GOTO |
| 143 | `DST$K_TARGET` | Target |
| 144 | `DST$K_REAL_NAME` | Ada package real name |
| 145 | `DST$K_BODY_SPEC` | Ada package body → spec link |
| 146 | `DST$K_PACK_SPEC_BEG` | Ada package spec begin |
| 147 | `DST$K_PACK_SPEC_END` | Ada package spec end |
| 148 | `DST$K_PACK_BODY_BEG` | Ada package body begin |
| 149 | `DST$K_PACK_BODY_END` | Ada package body end |
| 150 | `DST$K_SUBUNIT` | Ada subunit |
| 151 | `DST$K_SET_MODULE` | Ada WITH clause |
| 152 | `DST$K_USE_CLAUSE` | Ada USE clause |
| 153 | `DST$K_VERSION` | Version number |
| 154 | `DST$K_COBOLGBL` | COBOL global attribute |
| 155 | `DST$K_SOURCE` | Source file correlation |
| 156 | `DST$K_STATLINK` | Static link |
| 157 | `DST$K_VARVAL` | Variant value |
| 158 | `DST$K_BOOL` | Boolean data object (1 byte; the low bit is the value) |
| 159 | `DST$K_EXTRNXT` | External-is-next *(obsolete)* |
| 160 | `DST$K_GLOBNXT` | Global-is-next *(obsolete)* |
| 161 | `DSC$K_DTYPE_UBS` | Unaligned bit string, debugger-internal *(obsolete)* |
| 162 | `DST$K_PROLOG` | Prolog |
| 163 | `DST$K_SEPTYP` | Separate type specification |
| 164 | `DST$K_ENUMELT` | Enumeration element |
| 165 | `DST$K_ENUMBEG` | Enumeration type begin |
| 166 | `DST$K_ENUMEND` | Enumeration type end |
| 167 | `DST$K_VARBEG` | Variant set begin |
| 168 | `DST$K_VAREND` | Variant set end |
| 169 | `DST$K_OVERLOAD` | Overloaded symbol |
| 170 | `DST$K_DEF_LNUM` | Definition line number |
| 171 | `DST$K_RECBEG` | Record begin |
| 172 | `DST$K_RECEND` | Record end |
| 173 | `DST$K_CONTIN` | Continuation |
| 174 | `DST$K_VALSPEC` | Value specification *(obsolete)* |
| 175 | `DST$K_TYPSPEC` | Type specification |
| 176 | `DST$K_BLKBEG` | Block begin |
| 177 | `DST$K_BLKEND` | Block end |
| 178 | `DST$K_COB_HACK` | COBOL special *(obsolete)* |
| 179 | `DST$K_DTYPE_RESERVED_1` | Reserved to the debugger |
| 180 | `DST$K_USING` | C++ using declarations and directives |
| 181 | `DST$K_ENTRY` | Entry point |
| 182 | `DST$K_LINE_NUM_REL_R11` | Threaded-code PC correlation *(obsolete)* |
| 183 | `DST$K_BLIFLD` | BLISS field |
| 184 | `DST$K_PSECT` | PSECT |
| 185 | `DST$K_LINE_NUM` | Line-number PC correlation |
| 186 | `DST$K_LBLORLIT` | Label or literal |
| 187 | `DST$K_LABEL` | Label |
| 188 | `DST$K_MODBEG` | Module begin |
| 189 | `DST$K_MODEND` | Module end |
| 190 | `DST$K_RTNBEG` | Routine begin |
| 191 | `DST$K_RTNEND` | Routine end |
| 192 | `DST$K_PCLOC` | PC / GEM locator correlation |

`DST$K_LOWEST` = 116. `DST$K_HIGHEST` is the last value in this table, which
is 192 because `DST$K_PCLOC` was appended. Some older descriptions give the
top as 191.

### 4.5 Language codes

The Module Begin record stores a language code in a longword (typedef
`DST$LANGUAGE`). The picture and file type specs store one in a single byte
(typedef `DBG$DST_LANGUAGE`). Each `DST$K_x` constant has an identical
`DBG$K_x` alias.

| Value | Symbol | Language |
| - | - | - |
| 0 | `DST$K_MACRO` | MACRO (VAX assembler) |
| 1 | `DST$K_FORTRAN` | Fortran |
| 2 | `DST$K_BLISS` | BLISS |
| 3 | `DST$K_COBOL` | COBOL |
| 4 | `DST$K_BASIC` | BASIC |
| 5 | `DST$K_PLI` | PL/I |
| 6 | `DST$K_PASCAL` | Pascal |
| 7 | `DST$K_C` | C |
| 8 | `DST$K_RPG` | RPG |
| 9 | `DST$K_ADA` | Ada |
| 10 | `DST$K_UNKNOWN` | Unknown / generic |
| 11 | `DST$K_SCAN` | SCAN |
| 12 | `DST$K_DIBOL` | DIBOL |
| 13 | `DST$K_MODULA` | Modula |
| 14 | `DST$K_PILLAR` | Pillar |
| 15 | `DST$K_CXX` | C++ |
| 16 | `DST$K_AMACRO` | AMACRO (Alpha compiled MACRO-32) |
| 17 | `DST$K_MACRO64` | MACRO-64 |

`DST$K_MIN_LANGUAGE` = 0 and `DST$K_MAX_LANGUAGE` = 17.

How the debugger handles language codes:

- `DST$K_UNKNOWN`, or any code outside the range, gets generic support. That
  means identifiers made of `A–Z 0–9 $ _`, subscripts written with `()` or
  `[]`, dotted component selection, and common operators. It is meant for
  compilers that do not yet have dedicated support, and it is not a stable
  interface.
- Internally the code is truncated to its low 8 bits.
- A new language gets the next code after `DST$K_MAX_LANGUAGE`.

---

## 5. Scope records

### 5.1 Module Begin (`DST$K_MODBEG` = 188), `DST$MODULE_BEGIN`

This must be the first record of each compilation unit. Each object module
may contain only one Module Begin/Module End pair. If there are more, the
debugger sees only the first, because the linker reports only that one.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_MODBEG_FLAGS` | Flag bits, listed below |
| 3 | long | `DST$L_MODBEG_LANGUAGE` | Language code (§4.5) |
| 7 | byte | `DST$B_MODBEG_NAME` | Length of the module name |
| 8 | var | — | Module name (ASCII) |

`DST$K_MODBEG_SIZE` = 8 is the fixed part, up to and including the name
count byte.

Flag bits:

| Bit | Field | Meaning |
| - | - | - |
| 0 | `DST$V_MODBEG_HIDE` | Leave this module out of `SHOW MODULE` output |
| 1 | `DST$V_MODBEG_VERSION` | The record carries a version field (Alpha variant only) |
| 2–7 | `DST$V_MODBEG_UNUSED` | Must be zero |

### 5.2 Module End (`DST$K_MODEND` = 189), `DST$MODULE_END`

This record is a header only (`DST$B_LENGTH` = 1) and must be the last record
of the module. `DST$K_MODEND_SIZE` = 2.

### 5.3 Routine Begin (`DST$K_RTNBEG` = 190), `DST$ROUTINE_BEGIN`

Opens a routine scope and gives the routine's name and entry address.
A Routine End must close it, **except in MACRO modules**. MACRO routines
have no well-defined end, so in those modules the Routine End is always
omitted.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_RTNBEG_FLAGS` | Flag bits, listed below |
| 3 | long | `DST$L_RTNBEG_ADDRESS` | Start address, which is also the entry point |
| 7 | byte | `DST$B_RTNBEG_NAME` | Length of the routine name |
| 8 | var | — | Routine name |

`DST$K_RTNBEG_SIZE` = 8.

Flag bits:

| Bit | Field | Meaning |
| - | - | - |
| 0–3 | `DST$V_RTNBEG_UNUSED` | Must be zero |
| 4 | `DST$V_RTNBEG_UNALLOC` | The routine was optimized away. Ignore the address fields, but a Routine End is still required |
| 5 | `DST$V_RTNBEG_PROTOTYPE` | This is a prototype only. Ignore the address and the size in the Routine End |
| 6 | `DST$V_RTNBEG_INLINED` | This is an inlined instance of a routine |
| 7 | `DST$V_RTNBEG_NO_CALL` | The routine is entered by JSB/BSB rather than CALLS/CALLG |

### 5.4 Routine End (`DST$K_RTNEND` = 191), `DST$ROUTINE_END`

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header (`DST$B_LENGTH` = 6) | |
| 2 | byte | `dst$b_rtnend_unused` | Must be zero |
| 3 | long | `DST$L_RTNEND_SIZE` | Length of the routine's code in bytes |

`DST$K_RTNEND_SIZE` = 7.

### 5.5 Unallocated Routine (`DST$K_RTN_UNALLOC` = 120), `DST$ROUTINE_UNALLOC`

A compact record for a routine that was declared but never generated as
code. It carries only the name and, for C++, a type signature.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_RTNUNALLOC_NAME` | Length of the name |
| 3 | var | — | Name |
| … | byte | `DST$B_RTNUNALLOC_TYPE_SIG` | Length of the type signature (C++) |
| … | var | — | Type signature (ASCII) |

`DST$K_RTNUNALLOC_SIZE` = 3 is the fixed part.

### 5.6 Inline Instance (`DST$K_INLINE` = 136), `DST$INLINE`

Marks the scope that encloses it as an inline expansion of some routine.
At most one Inline record may appear directly inside a given Block
Begin/End pair, and that block is the scope of the inlined instance. Nested
inlining uses nested blocks, each with its own Inline record. The prolog
and epilog ranges of the instance are described with the ordinary Prolog
and Epilog records.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | long | `DST$L_INLINE_RTN_DST` | DST offset of the Routine Begin of the inlined routine |
| 6 | long | `DST$L_INLINE_CALLER_LINE` | Line number of the call site |

`DST$K_INLINE_SIZE` = 10.

### 5.7 Block Begin (`DST$K_BLKBEG` = 176) and Block End (`DST$K_BLKEND` = 177)

A *lexical block* is any scope-defining construct that is entered by
falling in or jumping in, not by a call. Examples are BEGIN/END blocks, and
COBOL paragraphs and sections. Blocks and routines may nest inside each
other freely. A block with an empty name cannot be referred to by name, but
its line numbers can still be used.

`DST$BLOCK_BEGIN`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `dst$b_blkbeg_unused` | Must be zero |
| 3 | long | `DST$L_BLKBEG_ADDRESS` | Address where the block's code starts |
| 7 | byte | `DST$B_BLKBEG_NAME` | Length of the name (may be 0) |
| 8 | var | — | Name |

`DST$K_BLKBEG_SIZE` = 8.

`DST$BLOCK_END` (`DST$B_LENGTH` = 6):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 2 | byte | `dst$b_blkend_unused` | Must be zero |
| 3 | long | `DST$L_BLKEND_SIZE` | Length of the block's code in bytes |

`DST$K_BLKEND_SIZE` = 7.

### 5.8 Ada package records

Both the package-spec and package-body Begin/End pairs deliberately use the
**same field positions as Routine Begin/End**, so that traceback and
profiling tools can treat them alike. That layout must be kept.

**Package Spec Begin** (`DST$K_PACK_SPEC_BEG` = 146), `DST$PACKAGE_SPEC_BEGIN`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 2 | byte | `DST$B_PACKSPEC_FLAGS` | Bits 0–1 are `DST$V_PACKSPEC_ELAB`, bit 2 is `DST$V_PACKSPEC_DUPL` (this is a duplicate definition), bits 3–7 are `dst$v_packspec_beg_unused` |
| 3 | long | `DST$L_PACKSPEC_ADDRESS` | Address of the elaboration code |
| 7 | byte | `DST$B_PACKSPEC_NAME` | Length of the package name, which is the source name as written |
| 8 | var | — | Name |

`DST$K_PACKSPEC_BEG_SIZE` = 8.

Elaboration codes, used by both the spec and the body:

| Value | Symbol | Meaning |
| - | - | - |
| 0 | `DST$K_NOELAB` | There is no elaboration code |
| 1 | `DST$K_ELAB_NOCALL` | There is elaboration code with no entry mask, so the address is the first instruction |
| 2 | `DST$K_ELAB_CALL` | There is elaboration code that starts with an entry mask, so the address points to the mask |

**Package Spec End** (`DST$K_PACK_SPEC_END` = 147), `DST$PACKAGE_SPEC_END`.
It has the same layout as Routine End: an unused byte, then
`DST$L_PACKSPEC_END_SIZE`, the size of the elaboration code (0 if there is
none). `DST$K_PACKSPEC_END_SIZE` = 7.

**Package Body Begin** (`DST$K_PACK_BODY_BEG` = 148), `DST$PACKAGE_BODY_BEGIN`:
bits 0–1 of the flags byte are `DST$V_PACKBODY_ELAB` and bits 2–7 are
`dst$v_packbody_mbz`. Then come `DST$L_PACKBODY_ADDRESS` and
`DST$B_PACKBODY_NAME`. The name is generated by the compiler, for example
`P$BODY`. `DST$K_PACKBODY_BEG_SIZE` = 8.

**Package Body End** (`DST$K_PACK_BODY_END` = 149), `DST$PACKAGE_BODY_END`:
an unused byte, then `DST$L_PACKBODY_END_SIZE`. `DST$K_PACKBODY_END_SIZE` = 7.

### 5.9 Discontiguous Range List (`DST$K_DIS_RANGE` = 118)

This record may appear inside any scope: routine, block, package, or inline
instance. At most one may appear in a scope. When present, it **replaces**
the single address range given by that scope's Begin/End records with a
list of ranges.

`DST$DIS_RANGES`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | long | `DST$LU_DISRNG_COUNT` | Number of ranges that follow |

`DST$K_DISRNGS_SIZE` = 6. The ranges start at that offset. The BLISS macro
`dst$a_dis_ranges` refers to `DST$K_DISRNG_SIZE` here, which looks like a
slip in the original.

Each range is a `DST$DIS_RANGE` (`DST$K_DISRNG_SIZE` = 8):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | long | `DST$LU_DISRNG_ADDRESS` | Start address |
| 4 | long | `DST$LU_DISRNG_SIZE` | Length in bytes |

The ranges must not overlap and must be sorted by ascending address.

---

## 6. Record (structure) types and variants

### 6.1 Model

A record type is a **Record Begin**, then one data record per component,
then a **Record End**. Other record kinds, such as type specs that give
component types, may sit between them. Only *data* records count as
components. Each component's value field holds its offset from the start of
the enclosing record, in bits or bytes.

A record definition nested inside another one does not make a nested
component by itself. A component is record-typed only when it refers to a
record type, either through a Separate Type Spec followed by Record Begin or
through an Indirect Type Spec. Each Begin/End pair describes one level of
components.

Record Begin can define either a **type** or an **object**:

- If `DST$B_VFLAGS` = `DST$K_VFLAGS_NOVAL` (128), the record defines a type
  only, and its name is the type name.
- Otherwise it defines both a type and an object of that type, which suits
  languages without named types such as COBOL. The value fields locate the
  object exactly as in a Standard Data record. Other records may still use
  Indirect Type Specs to refer to it as a type.

**Variants** (Pascal variant records, Ada discriminated records):

```text
Record Begin
  <fixed components>
  Variant Set Begin      (names the tag component)
    Variant Value        (tag values for variant 1)
      <components of variant 1>
    Variant Value        (tag values for variant 2)
      <components of variant 2>
  Variant Set End
  <more fixed components / more variant sets>
Record End
```

The tag field has to belong to the enclosing record itself and has to appear **before** the
Variant Set Begin. Variant sets may nest. A Variant Value runs until the
next Variant Value or the Variant Set End.

### 6.2 Record Begin (`DST$K_RECBEG` = 171)

The layout is a Standard Data record (§7.2) followed by a trailer. If the
object's address needs a Trailing Value Spec, that spec follows the name,
and the trailer comes after it.

| Field | Size | Meaning |
| - | - | - |
| header + `DST$B_VFLAGS` + `DST$L_VALUE` + `DST$B_NAME` + name | — | As in Standard Data |
| `DST$L_RECBEG_SIZE` | long | Bit size of objects of this type, or 0 if unknown at compile time |

The trailer aggregate is `DST$RECBEG_TRLR`, with `DST$K_RECBEG_TRAILER_SIZE` = 4.

### 6.3 Record End (`DST$K_RECEND` = 172)

Header only. `DST$K_RECEND_SIZE` = 2.

### 6.4 Variant Set Begin (`DST$K_VARBEG` = 167)

Same layout as Standard Data. The name is normally empty. It is followed by
the trailer `DST$VARBEG_TRAILER` (`DST$K_VARBEG_TRAILER_SIZE` = 8):

| Field | Size | Meaning |
| - | - | - |
| `DST$L_VARBEG_SIZE` | long | Bit size of the largest variant, or 0 |
| `DST$L_VARBEG_TAG_PTR` | long | DST offset of the tag component's data record |

### 6.5 Variant Value (`DST$K_VARVAL` = 157), `DST$VARIANT_VALUE`

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | long | `DST$L_VARVAL_SIZE` | Bit size of this variant, or 0 |
| 6 | word | `DST$W_VARVAL_COUNT` | Number of tag-range specs that follow |
| 8 | var | `DST$A_VARVAL_RNGSPEC` | Tag-range specs |

`DST$K_VARIANT_VALUE_SIZE` = 8.

**Tag value range spec** (`DST$VARVAL_RANGE`, fixed part
`DST$K_VARVAL_RANGE_SIZE` = 1):

| Field | Size | Meaning |
| - | - | - |
| `DST$B_VARVAL_RNGKIND` | byte | Range kind, listed below |
| `DST$A_VARVAL_RNGADDR` | var | One or two value specs, depending on the kind |

| Value | Symbol | What follows |
| - | - | - |
| 1 | `DST$K_VARVAL_SINGLE` | One value spec giving a single tag value |
| 2 | `DST$K_VARVAL_RANGE` | Two value specs giving the lower and upper bounds |

### 6.6 Variant Set End (`DST$K_VAREND` = 168)

Header only. `DST$K_VARSET_END_SIZE` = 2.

---

## 7. Data symbol records

### 7.1 Choosing a form

| Situation | Record to use |
| - | - |
| Simple scalar; address or value fits the 5-byte encoding | **Standard Data**, with the record type set to the dtype or `DST$K_BOOL` |
| Static object whose type needs a descriptor, and none exists in memory | **Descriptor Format**: a descriptor embedded in the record |
| Address or value too complex, or a constant wider than 32 bits | **Trailing Value Spec**: a value spec embedded in the record |
| Type too complex for a dtype or a descriptor | **Separate Type Spec** (`DST$K_SEPTYP`), followed by a type-defining record |
| BLISS structures and fields, enumeration literals, and similar | The dedicated records described later |

A Trailing Value Spec can also produce a whole descriptor at debug time. For
example, it can build the descriptor of an array with dynamic bounds, which
carries both the address and the type.

### 7.2 Standard Data record

`DST$B_TYPE` is a dtype code (1…`DBG$K_MAXIMUM_DTYPE`) or `DST$K_BOOL`.

`DST$DATA_HEADER` (`DST$K_DATA_HEADER_SIZE` = 3) followed by `DST$DATA_DST`
(`DST$K_DATA_SIZE` = 8):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_VFLAGS` | Value flags, see §7.3 |
| 3 | long | `DST$L_VALUE` | Value, address, offset, or register number |
| 7 | byte | `DST$B_NAME` | Length of the name |
| 8 | var | — | Name |

### 7.3 The value-flags byte

Unless the byte holds one of the special values in §7.4, it is split into
bit fields:

| Bits | Field | Meaning |
| - | - | - |
| 0–1 | `DST$V_VALKIND` | Kind of result, listed below |
| 2 | `DST$V_INDIRECT` | Dereference the computed address once |
| 3 | `DST$V_DISP` | Add the contents of register `REGNUM` to `DST$L_VALUE` |
| 4–7 | `DST$V_REGNUM` | Register used for displacement |

Value kinds:

| Value | Symbol | Meaning |
| - | - | - |
| 0 | `DST$K_VALKIND_LITERAL` | `DST$L_VALUE` is the constant itself (up to 32 bits) |
| 1 | `DST$K_VALKIND_ADDR` | The computation yields the object's address |
| 2 | `DST$K_VALKIND_DESC` | The computation yields the address of a descriptor for the object |
| 3 | `DST$K_VALKIND_REG` | `DST$L_VALUE` is a register number, and the object lives in that register |

`DST$K_VALKIND_MIN` = 0 and `DST$K_VALKIND_MAX` = 3.

**How to evaluate:**

1. **LITERAL**: the result is `DST$L_VALUE`.
2. **REG**: `r = DST$L_VALUE`. If INDIRECT is clear, the object *is*
   register `r`. If INDIRECT is set, the object's address is the contents
   of `r`.
3. Otherwise, start with `a = DST$L_VALUE`.
4. If DISP is set, `a += contents(REGNUM)`.
5. If INDIRECT is set, `a = fetch_long(a)`.
6. **ADDR**: `a` is the object's address.
   **DESC**: `a` is the address of a descriptor, and the debugger reads the
   real address, length and type from that descriptor.

*Example (VAX):* a second parameter passed by reference uses
`REGNUM` = 12 (AP), `VALUE` = 8, `DISP` = 1, `INDIRECT` = 1, and
`VALKIND` = ADDR. If the parameter is passed by descriptor, the encoding is
the same except that `VALKIND` = DESC.

*Alpha restrictions:* `DST$V_REGNUM` can encode only R16–R31 (field values
0–15; see §7.5). `DST$L_VALUE` in REG mode can name only integer registers.
Any other case needs an Extended Value Spec (§8.7).

### 7.4 Special value-flags values

When the flags byte holds one of these values, the bit fields above do not
apply and `DST$L_VALUE` takes on the special meaning shown. All of them
except NOVAL have the top four bits set. That cannot happen for an ordinary
flags byte, because REGNUM = 15 (the VAX PC) is never used for displacement.

| Value | Symbol | Meaning |
| - | - | - |
| 128 | `DST$K_VFLAGS_NOVAL` | No value: the record describes a type. Valid only in Record Begin |
| 248 | `DST$K_VFLAGS_NOTACTIVE` | The object has storage but is not live at the current PC, for example an Ada object not yet elaborated. Typically used inside split-lifetime specs |
| 249 | `DST$K_VFLAGS_UNALLOC` | The object never has storage, for example an unreferenced Pascal variable. Valid only in the flags byte of a data record. Not valid in value specs nested in type specs, and not valid for enumeration literals |
| 250 | `DST$K_VFLAGS_DSC` | Descriptor form: `VALUE` is an offset to an embedded descriptor |
| 251 | `DST$K_VFLAGS_TVS` | Trailing-VS form: `VALUE` is an offset to an embedded value spec |
| 253 | `DST$K_VS_FOLLOWS` | A full value spec follows. Valid only as the target of a trailing value spec |
| 255 | `DST$K_VFLAGS_BITOFFS` | `VALUE` holds a bit offset into the enclosing record (components only) |

### 7.5 Register numbering

These register numbers can appear in `DST$L_VALUE` when VALKIND = REG, and in
extended value specs. Only values 0–15 fit in the 4-bit `DST$V_REGNUM`
field. `DST$K_NO_SUCH_REG` = −1 marks an empty range.

**VAX** (`DST$K_REG_VAX_MIN` = 0, `DST$K_REG_VAX_MAX` = 35):

| Value | Symbol(s) |
| - | - |
| 0–11 | `DST$K_REG_VAX_R0` … `DST$K_REG_VAX_R11` |
| 12 | `DST$K_REG_VAX_R12` = `DST$K_REG_VAX_AP` |
| 13 | `DST$K_REG_VAX_R13` = `DST$K_REG_VAX_FP` |
| 14 | `DST$K_REG_VAX_R14` = `DST$K_REG_VAX_SP` |
| 15 | `DST$K_REG_VAX_R15` = `DST$K_REG_VAX_PC` |
| 16 | `DST$K_REG_VAX_PSL` = `DST$K_REG_VAX_PS` |
| 17–32 | `DST$K_REG_VAX_V0` … `DST$K_REG_VAX_V15` (vector registers) |
| 33 | `DST$K_REG_VAX_VCR` |
| 34 | `DST$K_REG_VAX_VLR` |
| 35 | `DST$K_REG_VAX_VMR` |

VAX ranges: scalar R0–R15; float none (−1); real vector V0–V15; vector
V0–VMR (`DST$K_REG_VAX_MIN_SCALAR`, `…_MAX_SCALAR`, `…_MIN_FLOAT`,
`…_MAX_FLOAT`, `…_MIN_REAL_VECTOR`, `…_MAX_REAL_VECTOR`, `…_MIN_VECTOR`,
`…_MAX_VECTOR`).

**Alpha** (`DST$K_REG_ALPHA_MIN` = 0, `DST$K_REG_ALPHA_MAX` = 85). The
integer registers are rotated so that R16–R31 take the values 0–15, which
fit in the 4-bit REGNUM field:

| Value | Symbol(s) |
| - | - |
| 0–15 | `DST$K_REG_ALPHA_R16` … `DST$K_REG_ALPHA_R31` |
| 36–51 | `DST$K_REG_ALPHA_R0` … `DST$K_REG_ALPHA_R15` |
| 52–83 | `DST$K_REG_ALPHA_F0` … `DST$K_REG_ALPHA_F31` |
| 84 | `DST$K_REG_ALPHA_PC` |
| 85 | `DST$K_REG_ALPHA_PS` |

Alpha aliases:

| Alias | Equals | Value |
| - | - | - |
| `DST$K_REG_ALPHA_AI` | R25 (argument information) | 9 |
| `DST$K_REG_ALPHA_RA` | R26 (return address) | 10 |
| `DST$K_REG_ALPHA_PV` | R27 (procedure value) | 11 |
| `DST$K_REG_ALPHA_FP` | R29 (frame pointer) | 13 |
| `DST$K_REG_ALPHA_SP` | R30 (stack pointer) | 14 |
| `DST$K_REG_ALPHA_REGNUM_FP` | `DST$K_REG_VAX_FP` | 13 |
| `DST$K_REG_ALPHA_REGNUM_SP` | `DST$K_REG_VAX_SP` | 14 |

The `…REGNUM_FP/SP` constants let a REGNUM field value of 13 or 14 be read
as the Alpha FP or SP. With the rotated numbering these match R29 and R30
anyway. Alpha ranges: scalar R0–R31; float F0–F31; vector none.

**Motorola 68xxx / 68881** (`DST$K_REG_M68_MIN` = 0, `…_MAX` = 35):

| Value | Symbol(s) |
| - | - |
| 0–7 | `DST$K_REG_M68_A0` … `A7` |
| 8–15 | `DST$K_REG_M68_D0` … `D7` |
| 16–23 | `DST$K_REG_M68_FP0` … `FP7` |
| 24–35 | `PC`, `SR`, `FPCR`, `FPSR`, `FPIAR`, `USP`, `MSP`, `CAAR`, `CACR`, `VBR`, `SFC`, `DFC` (each `DST$K_REG_M68_*`). Compilers must not emit these |

Ranges: scalar A0–D7; float FP0–FP7; vector none.

**MIL-STD-1750** (`DST$K_REG_1750_MIN` = 0, `…_MAX` = 23). The numbering
is reversed:

| Value | Symbol(s) |
| - | - |
| 0–15 | `DST$K_REG_1750_R15` … `DST$K_REG_1750_R0` (R15 = 0, R0 = 15) |
| 16–23 | `PC`, `SW`, `FT`, `MK`, `PI`, `IOIC`, `MFSR`, `PAGE` (each `DST$K_REG_1750_*`). Compilers must not emit these |

Aliases: `DST$K_REG_1750_SP` = R11, `DST$K_REG_1750_FP` = R15, and
`DST$K_REG_1750_IC` = PC. Ranges: scalar R15…R0; index R15…R1; base
R15…R12; float and vector none.

### 7.6 Descriptor Format record

The layout is Standard Data with `DST$B_VFLAGS` = `DST$K_VFLAGS_DSC`. The
value longword becomes `DST$L_DSC_OFFS` (aggregate
`DST$DESCRIPTOR_FORMAT`). `DST$K_DESCRIPTOR_FORMAT_SIZE` = 7 is the offset
of the name count byte, which is also called `DST$A_DSC_BASE`.

```text
descriptor_address = record + DST$K_DESCRIPTOR_FORMAT_SIZE + DST$L_DSC_OFFS
```

The embedded object is a standard descriptor: a word length, a byte dtype, a
byte class, a longword pointer, and then fields that depend on the class.

### 7.7 Trailing Value Specification record

The layout is Standard Data with `DST$B_VFLAGS` = `DST$K_VFLAGS_TVS`. The
value longword becomes `DST$L_TVS_OFFSET` (aggregate
`DST$TRAILING_VALSPEC`, `DST$K_TRAILING_VALSPEC_SIZE` = 7, base
`DST$A_TVS_BASE`).

```text
value_spec_address = record + DST$K_TRAILING_VALSPEC_SIZE + DST$L_TVS_OFFSET
```

### 7.8 Separate Type Specification record (`DST$K_SEPTYP` = 163)

The layout is Standard Data, optionally with a trailing value spec, and
`DST$K_SEPTYP_SIZE` = `DST$K_DATA_SIZE` = 8. The **next** record gives the
symbol's type, and only Continuation records may come between them. That
next record must be one of:

- Type Specification (`DST$K_TYPSPEC`)
- Record Begin (`DST$K_RECBEG`)
- Enumeration Type Begin (`DST$K_ENUMBEG`)

---

## 8. Value specifications

### 8.1 Where they appear

A *value specification* ("value spec") says how to get a value, an address,
or a descriptor. Value specs appear in:

- data records, as the flags byte plus value longword
- the trailing area of TVS-form data records
- type specs, for things like subrange bounds, dynamic descriptors, and
  picture edit patterns
- tag ranges in variant records, binding specs, and biased specs

Every value spec starts with a flags byte, `DST$B_VS_VFLAGS`. The rest of the
spec is chosen by that byte:

| Flags byte | Form |
| - | - |
| ordinary bit fields | Standard (§8.2) |
| `DST$K_VFLAGS_DSC` | Descriptor (§8.3) |
| `DST$K_VFLAGS_TVS` | Trailing (§8.3) |
| `DST$K_VS_FOLLOWS` | VS-Follows (§8.4): materialization, split-lifetime, biased, or extended |
| `DST$K_VFLAGS_UNALLOC` / `NOTACTIVE` | No storage, or not live at this PC |

### 8.2 `DST$VAL_SPEC` layout

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | byte | `DST$B_VS_VFLAGS` | Bit fields `DST$V_VS_VALKIND` (bits 0–1), `DST$V_VS_INDIRECT` (bit 2), `DST$V_VS_DISP` (bit 3), `DST$V_VS_REGNUM` (bits 4–7), or a special value |
| 1 | long | `DST$L_VS_VALUE` | Standard form: the value, address, or bit offset |
| 1 | long | `DST$L_VS_DSC_OFFS` | Descriptor form: offset to the descriptor |
| 1 | long | `DST$L_VS_TVS_OFFSET` | Trailing form: offset to the next value spec |
| 1 | word | `DST$W_VS_LENGTH` | VS-Follows form: length of the rest of the spec, counting from after this word |
| 3 | byte | `DST$B_VS_ALLOC` | VS-Follows form: allocation kind (§8.4) |
| 4 | byte | `DST$B_VS_NUM_BINDINGS` | Split-lifetime form: number of bindings |
| 4 | byte | `DST$B_VS_RESERVED` | Biased form: reserved for the biasing method, must be zero |

`DST$K_VALUE_SPEC_SIZE` = 5. Standard, descriptor, and trailing value specs
are always exactly 5 bytes.

Offset constants (BLISS names in parentheses):

| Constant | Value | Points to |
| - | - | - |
| `DST$K_VS_DSC_BASE` (`DST$A_VS_DSC_BASE`) | 5 | Base for `DST$L_VS_DSC_OFFS` |
| `DST$K_VS_TVS_BASE` (`DST$A_VS_TVS_BASE`) | 5 | Base for `DST$L_VS_TVS_OFFSET` |
| `DST$K_VS_MATSPEC_BASE` (`DST$A_VS_MATSPEC`) | 4 | Start of the materialization spec |
| `DST$K_VS_XVS_BASE` (`DST$A_VS_XVS_BASE`) | 4 | Start of the extended VS body |
| `DST$K_VS_BINDSPEC_BASE` (`DST$A_VS_BINDSPEC`) | 5 | First binding spec |
| `DST$K_VS_BIASING_VS_BASE` (`DST$A_VS_BIASING_VALSPEC`) | 5 | Nested 5-byte value spec that gives the bias |
| `DST$K_VS_BIASED_VS_BASE` (`DST$A_VS_BIASED_VALSPEC`) | 10 | Nested value spec that locates the stored value |

### 8.3 Descriptor and Trailing forms

Both forms are 5 bytes long. Each holds an offset, measured from the byte
just after the 5-byte spec, to more data later **in the same DST record**:

```text
descriptor = vs + DST$K_VS_DSC_BASE + DST$L_VS_DSC_OFFS   ; a standard descriptor
next_vs    = vs + DST$K_VS_TVS_BASE + DST$L_VS_TVS_OFFSET ; usually a VS-Follows spec
```

The trailing form lets a short spec point to a spec of any length.

### 8.4 VS-Follows form and allocation kinds

```text
byte  DST$B_VS_VFLAGS = DST$K_VS_FOLLOWS (253)
word  DST$W_VS_LENGTH
byte  DST$B_VS_ALLOC
var   body chosen by DST$B_VS_ALLOC
```

`DST$B_VS_ALLOC` (typedef `DST$VS_ALLOC_KIND`):

| Value | Symbol | Body |
| - | - | - |
| 1 | `DST$K_VS_ALLOC_STAT` | Materialization spec (§8.5) for a static value |
| 2 | `DST$K_VS_ALLOC_DYN` | Materialization spec for a dynamic value |
| 3 | `DST$K_VS_ALLOC_SPLIT` | Split-lifetime bindings (§8.6) |
| 4 | `DST$K_VS_ALLOC_BIASED` | Biased value (§8.8) |
| 5 | `DST$K_VS_ALLOC_XVS` | Extended value spec (§8.7) |

`DST$K_VS_ALLOC_MIN` = 1 and `DST$K_VS_ALLOC_MAX` = 5.

STAT and DYN can be used interchangeably, because the rest of the spec
already shows which applies. The convention is DYN when an address is
computed and STAT when a constant is computed. The debugger ignores the
difference.

### 8.5 Materialization specification

`DST$MATER_SPEC` starts at `DST$A_VS_MATSPEC`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | byte | `DST$B_MS_KIND` | What kind of value is produced |
| 1 | byte | `DST$B_MS_MECH` | How it is produced |
| 2 | byte | `DST$B_MS_FLAGBITS` | Flag bits, listed below |
| 3 | long / var | `DST$L_MS_MECH_RTNADDR` / `DST$A_MS_MECH_SPEC` | Thunk address, or stack-machine code |

`DST$K_MATER_SPEC_HEADER_SIZE` = 3 and `DST$K_MATER_SPEC_SIZE` = 7. The
mechanism-specific part starts at `dst$a_ms_mech_spec`, offset 3.

`DST$B_MS_KIND` (typedef `DST$MS_KIND`):

| Value | Symbol | Result produced |
| - | - | - |
| 1 | `DST$K_MS_BYTADDR` | 32-bit byte address |
| 2 | `DST$K_MS_BITADDR` | Byte address plus a 32-bit bit offset (two longwords) |
| 3 | `DST$K_MS_BITOFFS` | Bit offset from the start of the record (components) |
| 4 | `DST$K_MS_RVAL` | Literal value of any size |
| 5 | `DST$K_MS_REG` | Register number (the object is in that register) |
| 6 | `DST$K_MS_DSC` | A complete standard descriptor |
| 7 | `DST$K_MS_ADDR_DSC` | The address of a standard descriptor |

`DST$K_MS_LOWEST` = 1 and `DST$K_MS_HIGHEST` = 7.

`DST$B_MS_MECH` (typedef `DST$MS_MECH`):

| Value | Symbol | Mechanism |
| - | - | - |
| 1 | `DST$K_MS_MECH_RTNCALL` | Call a compiler-generated thunk whose address is in `DST$L_MS_MECH_RTNADDR` |
| 2 | `DST$K_MS_MECH_STK` | Run a DST stack-machine routine (§8.9) |
| 3 | `DST$K_MS_MECH_RTN_NOFP` | Like 1, but the debugger skips its check for a zero FP |

`DST$K_MS_MECH_MIN` = 1 and `DST$K_MS_MECH_MAX` = 3.

`DST$B_MS_FLAGBITS`:

| Bit | Field | Meaning |
| - | - | - |
| 0 | `DST$V_MS_NOEVAL` | Purpose undocumented |
| 1 | `DST$V_MS_DUMARG` | Pass a 16-byte result buffer as an extra first argument |
| 2 | `DST$V_MS_WRONGBOUNDS` | The array bounds in the produced descriptor may be wrong (see below) |
| 3–7 | `DST$V_MS_MBZ` | Must be zero |

**Thunk calling convention** (mechanism 1 or 3; on the wire,
`DST$W_VS_LENGTH` = 8 and ALLOC = DYN):

- The single argument is the address of a read-only vector of the declaring
  frame's registers: R0–R11, AP, FP, SP, PC, PSL, in that order.
- R1 holds the frame's FP.
- The result is returned in R0.
- If `DST$V_MS_DUMARG` is set, argument 1 is the address of a 16-byte buffer
  for a wider result, and the register vector moves to argument 2.

Thunks are needed when an address depends on user code, for example PL/I
BASED variables.

**WRONGBOUNDS.** The Ada compiler sets this flag in an array descriptor
built inside a type spec when the real bounds are known only from a
run-time descriptor, as with access-to-unconstrained-array types. When the
debugger sees the flag, it gets the bounds from the run-time descriptor
reached through the access value, which is described with
`DST$K_TS_TPTR_D`, and not from the type spec.

### 8.6 Split-lifetime value spec

Use this when a variable's location changes with the PC inside its scope.
For example, it might be unallocated in one range, in a register in
another, and on the stack in a third. A variable that keeps one location
for the whole of its scope does **not** need this form.

```text
byte  DST$K_VS_FOLLOWS
word  DST$W_VS_LENGTH
byte  DST$K_VS_ALLOC_SPLIT
byte  DST$B_VS_NUM_BINDINGS
repeated NUM_BINDINGS times:
    DST$BIND_SPEC
```

`DST$BIND_SPEC` (`DST$K_BIND_SPEC_SIZE` = 8):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | long | `DST$L_BS_LO_PC` | Low end of the PC range |
| 4 | long | `DST$L_BS_HI_PC` | High end of the PC range |
| 8 | var | `DST$A_BS_VALSPEC` | Nested value spec of any kind, including UNALLOC or NOTACTIVE |

The debugger searches the ranges in order and uses the first one that
contains the PC. If the record grows past 255 bytes, continue it with
Continuation records.

### 8.7 Extended value spec (XVS)

This form does the same job as the standard 5-byte spec but has wider
fields, so it can name any register in the architecture, such as Alpha
R0–R15 or the floating-point registers, for displacement or indirection.
It is always wrapped in a VS-Follows spec:

```text
byte  DST$K_VS_FOLLOWS
word  DST$W_VS_LENGTH = 13
byte  DST$K_VS_ALLOC_XVS
DST$XVS_SPEC:
  word  DST$W_XVS_FLAGS
  word  DST$W_XVS_REGNUM
  quad  DST$Q_XVS_VALUE   (DST$L_XVS_LOW_VALUE, DST$L_XVS_HI_VALUE)
```

`DST$K_XVS_SPEC_SIZE` = 12.

`DST$W_XVS_FLAGS` bits. Note that the **bit order is different** from the
standard flags byte:

| Bits | Field |
| - | - |
| 0 | `DST$V_XVS_INDIRECT` |
| 1 | `DST$V_XVS_DISP` |
| 2–3 | `DST$V_XVS_VALKIND` |
| 4–15 | `DST$V_XVS_FILL0` (must be zero) |

The flags word is only ever read as bit fields. It has no special values.

### 8.8 Biased value spec

Some Ada representation clauses store a value in fewer bits than it needs by
subtracting a bias. For example, the range 100..103 can be stored in 2 bits
as 0..3. This spec describes such values.

```text
byte  DST$K_VS_FOLLOWS
word  DST$W_VS_LENGTH
byte  DST$K_VS_ALLOC_BIASED
byte  reserved (MBZ)                       DST$B_VS_RESERVED
5     DST$A_VS_BIASING_VALSPEC             nested 5-byte value spec giving the bias
var   DST$A_VS_BIASED_VALSPEC              nested value spec locating the stored bits
```

To read the value, evaluate the bias as a signed 32-bit number and add it to
the stored representation. To deposit, subtract the bias. The bias is
usually a literal, but because it is a value spec it can also be computed at
run time. The reserved byte leaves room for other encodings later.

*Encoded value specs*, a generalization of biased specs, were proposed but
never supported, and their format is not defined here.

### 8.9 The DST stack machine

```text
byte  DST$K_VS_FOLLOWS
word  DST$W_VS_LENGTH
byte  DST$B_VS_ALLOC          (DYN for addresses, STAT for constants)
byte  DST$B_MS_KIND
byte  DST$B_MS_MECH = DST$K_MS_MECH_STK
byte  DST$B_MS_FLAGBITS
var   DST$A_MS_MECH_SPEC      stack-machine code, ending in STOP
```

**Machine model:**

- The stack holds 256 longword cells and grows toward lower addresses.
- Each instruction is a one-byte opcode followed by zero or more operand
  bytes.
- Execution runs until `STOP`. The value is then on top of the stack. Its
  width depends on `DST$B_MS_KIND`: one longword for an address, two for a
  bit address, any number for a literal, or a complete descriptor.
- *Top* means the most recently pushed cell, and *second* means the one
  below it.
- Register values come from the frame of the symbol's scope.

`DST$K_STK_LOW` = 0 and `DST$K_STK_HIGH` = 86 bound the valid opcodes.

**Push register:**

| Opcode | Symbol | VAX register |
| - | - | - |
| 0–11 | `DST$K_STK_PUSHR0` … `DST$K_STK_PUSHR11` | R0–R11 |
| 12 | `DST$K_STK_PUSHRAP` | AP |
| 13 | `DST$K_STK_PUSHRFP` | FP |
| 14 | `DST$K_STK_PUSHRSP` | SP |
| 15 | `DST$K_STK_PUSHRPC` | PC |

The same opcodes 0–15 serve other architectures under these names:

- 68K: `DST$K_STK_PUSH_M68_A0` … `A7` = 0–7, and `DST$K_STK_PUSH_M68_D0` …
  `D7` = 8–15.
- MIL-STD-1750: `DST$K_STK_PUSH_1750_R0` = 15 down to
  `DST$K_STK_PUSH_1750_R15` = 0 (reversed).

Alpha has its own opcodes: `DST$K_STK_PUSH_ALPHA_R0` … `R31` = 55 … 86.
Aliases: `…_AI` = R25 (80), `…_RA` = R26 (81), `…_PV` = R27 (82),
`…_FP` = R29 (84), `…_SP` = R30 (85).

**Push immediate.** The operand bytes follow the opcode.

| Opcode | Symbol | Operand | Extension |
| - | - | - | - |
| 16 | `DST$K_STK_PUSHIMB` | 1 byte | sign |
| 17 | `DST$K_STK_PUSHIMW` | 2 bytes | sign |
| 18 | `DST$K_STK_PUSHIML` | 4 bytes | — |
| 25 | `DST$K_STK_PUSHIMBU` | 1 byte | zero |
| 26 | `DST$K_STK_PUSHIMWU` | 2 bytes | zero |
| 24 | `DST$K_STK_PUSHIM_VAR` | length byte *n*, then *n* bytes | Zero-padded at the high end to a whole number of longwords, and pushed as a block |

**Push indirect.** Pops an address and pushes the 1, 2 or 4 bytes stored
there, extended to 32 bits.

| Opcode | Symbol | Width | Extension |
| - | - | - | - |
| 20 | `DST$K_STK_PUSHINB` | byte | sign |
| 21 | `DST$K_STK_PUSHINW` | word | sign |
| 22 | `DST$K_STK_PUSHINL` | long | — |
| 27 | `DST$K_STK_PUSHINBU` | byte | zero |
| 28 | `DST$K_STK_PUSHINWU` | word | zero |

**Arithmetic and logic.** Each pops two cells and pushes one. T is the top
cell and S is the second.

| Opcode | Symbol | Result |
| - | - | - |
| 19 | `DST$K_STK_ADD` | T + S |
| 29 | `DST$K_STK_SUB` | T − S (the *second* is subtracted from the *top*) |
| 30 | `DST$K_STK_MULT` | T × S |
| 31 | `DST$K_STK_DIV` | T ÷ S (the *top* is divided by the *second*) |
| 32 | `DST$K_STK_LSH` | S shifted logically by T bits |
| 33 | `DST$K_STK_ROT` | S rotated by T bits |

**Bit-field extraction.** Each pops three cells and pushes one. The first
(top) cell is the bit position, the second is the field size, and the third
is the operand, or for the indirect forms the operand's address.

| Opcode | Symbol | Operand | Extension |
| - | - | - | - |
| 45 | `DST$K_STK_EXTV_IMED` | value | sign |
| 46 | `DST$K_STK_EXTZV_IMED` | value | zero |
| 47 | `DST$K_STK_EXTV_IND` | address | sign |
| 48 | `DST$K_STK_EXTZV_IND` | address | zero |

**Stack manipulation:**

| Opcode | Symbol | Effect |
| - | - | - |
| 34 | `DST$K_STK_COP` | Duplicate the top cell |
| 35 | `DST$K_STK_EXCH` | Swap the top two cells |
| 39 | `DST$K_STK_POP` | Discard the top cell |
| 36 | `DST$K_STK_STO_B` | Operand byte *k*: store the low byte of the top cell at (SP + 4 + *k*), then pop |
| 37 | `DST$K_STK_STO_W` | Same, storing a word |
| 38 | `DST$K_STK_STO_L` | Same, storing a longword |
| 49 | `DST$K_STK_FET_B` | Operand byte *k*: read a byte at (SP + 4 + *k*), measured before the push, and push it. The unused high bits are undefined |
| 50 | `DST$K_STK_FET_W` | Same, reading a word |
| 51 | `DST$K_STK_FET_L` | Same, reading a longword |

The fetch instructions are exact inverses of the store instructions. In
both, *k* is an unsigned byte offset counted from the second cell.

**Control:**

| Opcode | Symbol | Effect |
| - | - | - |
| 23 | `DST$K_STK_STOP` | Stop. The top of the stack is the result |
| 40 | `DST$K_STK_RTNCALL` | Pop a thunk address and call it. Argument 1 is the register vector (R0–R11, AP, FP, SP, PC, PSL; read-only) and argument 2 is the current stack-top address, so the code can push arguments for the thunk first. R1 holds the frame's FP. The R0 result is pushed |
| 41 | `DST$K_STK_RTNCALL_ALT` | Like 40, but argument 1 is a 16-byte result buffer and the register vector and stack pointer move to arguments 2 and 3. The whole buffer is pushed |
| 44 | `DST$K_STK_RTN_NOFP` | Like 40, but the debugger does not check that FP is nonzero. FP is still passed and may be zero |

**Record context.** These are used to build descriptors whose fields depend
on sibling components, such as PL/I arrays with bounds taken from other
fields.

| Opcode | Symbol | Effect |
| - | - | - |
| 42 | `DST$K_STK_PUSH_OUTER_REC` | Push the address of the outermost record that contains the current symbol |
| 43 | `DST$K_STK_PUSH_INNER_REC` | Push the address of the innermost record that contains the current symbol |

**Other:**

| Opcode | Symbol | Effect |
| - | - | - |
| 52 | `DST$K_STK_POS` | Operand: a longword DST offset of an Enumeration Begin or Type Spec record. Replaces the value on top of the stack with its zero-based position in that enumeration. Every enumeration element must have a LITERAL value kind |
| 53 | `DST$K_STK_PUSH_VALSPEC` | Reserved, not implemented. Meant to push the result of another value spec |
| 54 | `DST$K_STK_PUSH_INNER_ARRAY` | Reserved, not implemented |

Opcodes not listed in these tables are unassigned.

---

## 9. Type specifications

### 9.1 Type Specification record (`DST$K_TYPSPEC` = 175)

This is the most general way to define a type. A Type Specification record
appears in one of two places:

- immediately after a Separate Type Spec record, or
- somewhere else, as the target of an Indirect or Novel Length type spec.

`DST$TYPSPEC`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_TYPSPEC_NAME` | Length of the type name (0 if the type is unnamed) |
| 3 | var | — | Type name |
| … | var | `DST$A_TYPSPEC_TS_ADDR` | The type spec itself |

`DST$K_TYPSPEC_SIZE` = 3. The type spec starts at offset 3 plus the name
length.

### 9.2 Common type-spec header (`DST$TYPE_SPEC`)

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | word | `DST$W_TS_LENGTH` | Number of bytes after this word |
| 2 | byte | `DST$B_TS_KIND` | Kind of type spec (typedef `DST$TS_DTYPE`) |
| 3 | var | — | Body, which depends on the kind |

`DST$K_TS_BASE` = 2. To skip over any type spec, compute
`spec + DST$K_TS_BASE + DST$W_TS_LENGTH`. That is also where data following
the type spec begins.

Type specs can contain other type specs, for example a subrange's parent
type or an array's element type. Record and enumeration types are reached
only through `DST$K_TS_IND` (or, rarely, a Novel Length spec). No other
type spec can name them directly. To save space, several symbols of the
same type should share one Type Specification record and point to it with
Indirect specs.

### 9.3 Type-spec kinds

| Value | Symbol | Kind |
| - | - | - |
| 1 | `DST$K_TS_ATOM` | Atomic |
| 2 | `DST$K_TS_DSC` | Standard descriptor |
| 3 | `DST$K_TS_IND` | Indirect (pointer into the DST) |
| 4 | `DST$K_TS_TPTR` | Typed pointer |
| 5 | `DST$K_TS_PTR` | Untyped pointer |
| 6 | `DST$K_TS_PIC` | Picture |
| 7 | `DST$K_TS_ARRAY` | Array |
| 8 | `DST$K_TS_SET` | Set |
| 9 | `DST$K_TS_SUBRANGE` | Subrange |
| 10 | `DST$K_TS_ADA_DSC` | Ada extended descriptor |
| 11 | `DST$K_TS_FILE` | File |
| 12 | `DST$K_TS_AREA` | PL/I area |
| 13 | `DST$K_TS_OFFSET` | PL/I offset |
| 14 | `DST$K_TS_NOV_LENG` | Novel length |
| 15 | `DST$K_TS_IND_TSPEC` | Debugger-internal pointer to a type spec (never in a DST) |
| 16 | `DST$K_TS_SELF_REL_LABEL` | PL/I self-relative label |
| 17 | `DST$K_TS_RFA` | BASIC record file address |
| 18 | `DST$K_TS_TASK` | Ada task |
| 19 | `DST$K_TS_ADA_ARRAY` | Ada array |
| 20 | `DST$K_TS_XMOD_IND` | Cross-module indirect |
| 21 | `DST$K_TS_CONSTRAINED` | Ada constrained record |
| 22 | `DST$K_TS_MAYBE_CONSTR` | Ada might-be-constrained record |
| 23 | `DST$K_TS_DYN_NOV_LENG` | Dynamic novel length |
| 24 | `DST$K_TS_TPTR_D` | Typed pointer to a descriptor |
| 25 | `DST$K_TS_SCAN_TREE` | SCAN tree |
| 26 | `DST$K_TS_SCAN_TREEPTR` | SCAN tree pointer |
| 27 | `DST$K_TS_INCOMPLETE` | Incomplete (Ada deferred) |
| 28 | `DST$K_TS_BLISS_BLOCK` | BLISS BLOCK |
| 29 | `DST$K_TS_TPTR_64` | 64-bit typed pointer |
| 30 | `DST$K_TS_PTR_64` | 64-bit untyped pointer |
| 31 | `DST$K_TS_REF` | C++ reference |
| 32 | `DST$K_TS_REF_64` | C++ reference, 64-bit |

`DST$K_TS_DTYPE_LOWEST` = 1 and `DST$K_TS_DTYPE_HIGHEST` = 32.
`DST$K_TYPE_SPEC_SIZE` = 11 is the size of the largest fixed-part variant.

### 9.4 Size and offset constants

The `…_LENG` constants give the **total byte size** of the fixed-length
kinds, including the length word. The other constants give the offset,
from the start of the type spec, of a variable-length part.

| Constant | Value | Meaning |
| - | - | - |
| `DST$K_TS_ATOM_LENG` | 4 | Atomic spec size |
| `DST$K_TS_IND_LENG` | 7 | Indirect spec size |
| `DST$K_TS_PTR_LENG` | 3 | Pointer spec size |
| `DST$K_TS_PTR_64_LENG` | 3 | 64-bit pointer spec size |
| `DST$K_TS_FILE_LENG` | 4 | File spec size without the record type |
| `DST$K_TS_AREA_LENG` | 3 | Area spec fixed part |
| `DST$K_TS_OFFSET_LENG` | 3 | Offset spec fixed part |
| `DST$K_TS_NOV_LENG_LENG` | 11 | Novel length spec size |
| `DST$K_TS_TASK_LENG` | 3 | Task spec with no entries |
| `DST$K_TS_INCOMPLETE_LENG` | 7 | Incomplete spec size |
| `DST$K_TS_DSC_VSPEC` | 3 | Descriptor spec's value spec |
| `DST$K_TS_ADA_DSC_VSPEC` | 5 | Ada descriptor spec's value spec |
| `DST$K_TS_TPTR_TSPEC`, `DST$K_TS_TPTR_64_TSPEC`, `DST$K_TS_REF_TSPEC`, `DST$K_TS_REF_64_TSPEC` | 3 | Nested pointee type spec |
| `DST$K_TS_PIC_ADDR` | 6 | Picture encoding |
| `DST$K_TS_ARRAY_FLAGS` | 4 | Array flag bit vector |
| `DST$K_TS_ADA_ARRAY_FLAGS` | 6 | Ada array flag bit vector |
| `DST$K_TS_SET_PAR_TSPEC` | 7 | Set parent type spec |
| `DST$K_TS_SUBR_PAR_TSPEC` | 7 | Subrange parent type spec |
| `DST$K_TS_FILE_RCRD_TYP` | 4 | File record type spec |
| `DST$K_TS_AREA_BYTE_LEN` | 3 | Area length value spec |
| `DST$K_TS_OFFSET_VALSPEC` | 3 | Offset area-base value spec |
| `DST$K_TS_NOV_LENG_VSPEC` | 3 | Dynamic novel length: value spec giving the length |
| `DST$K_TS_NOV_LENG_TSPEC` | 8 | Dynamic novel length: parent type spec |
| `DST$K_TS_TASK_ENTRY` | 5 | First task entry |
| `DST$K_TS_CONSTR_LIST` | 11 | List of constrained components |
| `DST$K_TS_MIGHTBE_VALSPEC` | 7 | Might-be-constrained value spec |
| `DST$K_TS_SCAN_TREE_FLAGS` | 4 | SCAN tree flag bit vector |
| `DST$K_TS_SCAN_TREEPTR_TREE` | 3 | Nested tree type spec |
| `DST$K_TS_FIELD_SET_LIST` | 9 | BLISS block field-set pointer list |

BLISS uses `DST$A_…` macros with matching names for each of these offsets,
for example `DST$A_TS_ARRAY_FLAGS_ADDR`.

### 9.5 Each kind in detail

**Atomic (1).** Body: `DST$B_TS_ATOM_TYP`, one byte holding a dtype code.
`DST$W_TS_LENGTH` = 2.

**Descriptor (2).** Body: a value spec at `DST$A_TS_DSC_VSPEC_ADDR` that
produces a standard descriptor. Use it for types that need length or scale
information, such as packed decimal or fixed-length text.

**Ada descriptor (10).** Ada's *extended* descriptor uses the whole first
longword for the length, which overwrites the class and dtype bytes, so the
type spec supplies them separately.

| Field | Size |
| - | - |
| `DST$B_TS_ADA_DSC_CLASS` | byte |
| `DST$B_TS_ADA_DSC_DTYPE` | byte |
| `DST$A_TS_ADA_DSC_VSPEC` | value spec producing the extended descriptor |

**Indirect (3).** Body: `DST$L_TS_IND_PTR`, the DST offset of a Type Spec,
Record Begin, or Enumeration Begin record. `DST$W_TS_LENGTH` = 5.

**Cross-module indirect (20).** Like Indirect, except that the offset is
counted from the start of another module's DST. Ada uses it to refer to
types in library packages.

| Field | Size |
| - | - |
| `DST$L_TS_XMOD_OFFSET` | long |
| `DST$B_TS_XMOD_MODNAME` | count byte, followed by the module name |

**Typed pointer (4), 64-bit typed pointer (29), pointer to descriptor (24),
C++ reference (31), and 64-bit reference (32).** Body: a nested type spec
for the pointee at `DST$A_TS_TPTR_TSPEC_ADDR`.

- `TPTR_D` means the pointer holds the address of a *descriptor*, and the
  descriptor's pointer field leads to the data. Ada uses this for access
  types that designate unconstrained arrays.
- The 64-bit kinds apply only on Alpha.

**Pointer (5) and 64-bit pointer (30).** Untyped pointers, such as PL/I
POINTER. There is no body, and `DST$W_TS_LENGTH` = 1.

**Picture (6).** Describes COBOL and PL/I picture types. Objects are stored
as ASCII strings.

| Field | Size | Meaning |
| - | - | - |
| `DST$B_TS_PIC_DLENG` | byte | Object length in bytes |
| `DST$B_TS_PIC_LANG` | byte | Language code |
| `DST$B_TS_PIC_PLENG` | byte | Length of the encoding that follows, in bytes |
| `DST$A_TS_PIC_ADDR` | var | Picture encoding, as described below |
| — | var | Optional value spec giving the address of the EDITPC pattern string |
| `DST$B_TS_PIC_SCALE` | byte | Scale factor |
| `DST$B_TS_PIC_DIGITS` | byte | Digit count |

- **Encoding.** The picture is a series of 2-byte pairs. In each pair the
  low byte is the picture character and the high byte is its repeat count.
  For example, `S999.99` is stored as the bytes `'S',1,'9',3,'.',1,'9',2`.
- **Scale and digits.** These two bytes have no SDL symbols; the names in
  the table are informal. They sit at `spec + DST$W_TS_LENGTH`, meaning the
  length word's value is used as an offset from the start of the spec.
- **EDITPC pattern.** The debugger uses it with EDITPC to deposit numbers.
  If the value spec is missing, only character strings can be deposited.

**Array (7).**

| Field | Size | Meaning |
| - | - | - |
| `DST$B_TS_ARRAY_DIM` | byte | Number of dimensions, *n* |
| `DST$A_TS_ARRAY_FLAGS_ADDR` | ⌈(*n*+1)/8⌉ bytes | Presence bit vector |
| — | var | Value spec that produces an array descriptor (class `DSC$K_CLASS_A`, `NCA`, or `UBA`) |
| — | var | Optional element type spec, present if bit 0 is set |
| — | var | Optional type spec for subscript *i*, present if bit *i* is set (*i* = 1…*n*) |

- If the element type spec is missing, the descriptor's dtype gives the
  element type. If the element type spec is present, the descriptor's dtype
  should be 0.
- If a subscript type spec is missing, that subscript is `DTYPE_L`.
  Subscript type specs exist mainly for enumeration-indexed arrays.
- Use the Array spec only when nothing simpler works: either the bounds are
  dynamic and no run-time descriptor exists, or the element or subscript
  types cannot be expressed in a standard descriptor (records, enumerations,
  typed pointers).

**Ada array (19).** Same as Array, but the descriptor is in Ada extended
form, so the class and dtype are given explicitly.

| Field | Size |
| - | - |
| `DST$B_TS_ADA_ARRAY_DIM` | byte |
| `DST$B_TS_ADA_ARRAY_CLASS` | byte (`A`, `NCA`, or `UBA`) |
| `DST$B_TS_ADA_ARRAY_DTYPE` | byte (0 if an element type spec is present) |
| `DST$A_TS_ADA_ARRAY_FLAGS` | bit vector, as for Array |

These are followed by the descriptor value spec and the optional type specs.

**Set (8).** A Pascal-style set, stored as a bit string in which bit *k* is
set exactly when the *k*-th element of the parent type is a member.

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_SET_LENG` | long | Bit length of objects |
| `DST$A_TS_SET_PAR_TSPEC_ADDR` | var | Parent type: integer, an enumeration (through Indirect), or a subrange |

Debugger limits: `DBG$K_SET_SIZE_MAX` = 8192 bytes (2^16 elements) and
`DBG$K_PREDEF_SET_SIZE_MAX` = 32 bytes (256 elements, for predefined
integer sets).

**Subrange (9).**

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_SUBR_LENG` | long | Bit length of objects |
| `DST$A_TS_SUBR_PAR_TSPEC_ADDR` | var | Parent type spec |
| — | var | Value spec for the lower bound |
| — | var | Value spec for the upper bound |

The bounds are expressed as values of the parent type.

**File (11).**

| Field | Size | Meaning |
| - | - | - |
| `DST$B_TS_FILE_LANG` | byte | Language code |
| `DST$A_TS_FILE_RCRD_TYP` | var | Optional type spec for the record type. If missing, a file of characters is assumed |

**Area (12)**, PL/I. Body: a value spec at `DST$A_TS_AREA_BYTE_LEN` giving
the area's length in bytes. Early debugger versions did not support it.

**Offset (13)**, PL/I. Body: a value spec at `DST$A_TS_OFFSET_VALSPEC`
giving the area's base address, then a second value spec giving the offset.
Early debugger versions did not support it either.

**Novel length (14).** A type identical to its parent except for its length,
for example a 1-bit Boolean in a packed record. The debugger widens such
values to the parent's normal length when it accesses them.

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_NOV_LENG` | long | The new length, in bits |
| `DST$L_TS_NOV_LENG_PAR_TSPEC` | long | DST offset of the parent type's Type Spec, Record Begin, or Enumeration Begin record |

`DST$W_TS_LENGTH` = 9.

**Dynamic novel length (23).** Like Novel Length, but the length is computed
at run time.

| Field | Size | Meaning |
| - | - | - |
| `DST$A_TS_NOV_LENG_VSPEC` | 5 | Value spec (`dst$a_dyn_nov_val_spec`) giving the length |
| `DST$A_TS_NOV_LENG_TSPEC` | var | Nested parent type spec (may be Indirect) |

**Self-relative label (16)**, PL/I. An array of longwords, each holding a
label address relative to the start of the array, so the table is
position-independent. There is no body, and `DST$W_TS_LENGTH` = 1.

**RFA (17)**, BASIC record file address. The source defines the code but no
layout.

**Task (18)**, Ada. Task objects are longwords that the tasking kernel
interprets. If `DST$W_TS_LENGTH` = 1 there is no body. Otherwise:

| Field | Size | Meaning |
| - | - | - |
| `DST$WU_TS_TASK_ENTRY_COUNT` | word | Number of entries |
| — | var | That many entry descriptors, starting at `DST$K_TS_TASK_ENTRY` |

Each entry is a `DST$TASK_TS_ENTRY` (fixed part `DST$K_TASK_TS_ENTRY_SIZE` = 2):

| Field | Size | Meaning |
| - | - | - |
| `DST$BU_TS_TASK_ENTRY_FLAGS` | byte | Bit 0 is `DST$V_TS_TASK_ENTRY_FAMILY` (the entry is a family); bits 1–7 are `DST$V_TS_TASK_ENTRY_MBZ` |
| `DST$BU_TS_TASK_ENTRY_NAME` | byte | Length of the entry name. The same byte is also named `DST$BU_TS_TASK_ENTRY_TRLR_OFFS`, because it is also the offset from `dst$a_ts_task_entry_trlr_base` (= 2) to the trailer |
| — | var | Entry name |
| trailer | var | Present only for families, as below |

Family trailer `DST$TASK_TS_FAMILY` (BLISS alias
`DST$TASK_TS_ENTRY_FAMILY`; size `DST$K_TASK_TS_ENTRY_FAMILY_SIZE` = 10):

| Field | Size | Meaning |
| - | - | - |
| `DST$A_TS_ENTRY_FAMILY_LB` | 5 | Value spec for the lower bound |
| `DST$A_TS_ENTRY_FAMILY_UB` | 5 | Value spec for the upper bound |
| `dst$a_ts_entry_family_type` | var | Type spec for the family index, starting at offset 10 |

**Constrained record (21)**, Ada. A record with variants whose
discriminants are fixed.

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_CONSTR_RECORD` | long | DST offset of a Record Begin, a Typed Pointer type spec, or a Cross-Module Indirect type spec |
| `DST$L_TS_CONSTR_COUNT` | long | Number of constrained components |
| `DST$A_TS_CONSTR_LIST` | var | Repeated pairs, described below |

Each pair is a longword component number (0-based), then a value spec giving
that component's required value. The debugger checks these constraints
whenever the record is accessed.

**Might-be-constrained record (22)**, Ada. Used for an unconstrained formal
parameter whose actual argument may be constrained.

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_MIGHTBE_RECORD` | long | DST offset of the Record Begin |
| `DST$A_TS_MIGHTBE_VALSPEC` | var | Value spec whose low bit is TRUE when the actual argument is constrained. Evaluated for `'CONSTRAINED` |

Apart from that, the record is treated as an ordinary record.

**SCAN tree (25).**

| Field | Size | Meaning |
| - | - | - |
| `DST$B_TS_SCAN_TREE_DEPTH` | byte | Number of subscripts, *d* |
| `DST$A_TS_SCAN_TREE_FLAGS` | ⌈(*d*+1)/8⌉ bytes | Bit 0 means a leaf type spec is present; bit *i* means a type spec for subscript *i* is present |
| — | var | Optional leaf type spec, then optional subscript type specs |

- A missing leaf or subscript type defaults to `DTYPE_L`.
- Tree nodes live on the heap, so the DST gives no way to locate them.

**SCAN tree pointer (26).** Body: a nested SCAN tree type spec at
`DST$A_TS_SCAN_TREEPTR_TREE`. It describes the subtree below the level the
pointer refers to, combining that level's subscript type with the type of
what lies beneath it.

*Example:* `TREEPTR (STRING) TO TREE (INTEGER) OF INTEGER` embeds the tree
type `TREE (STRING, INTEGER) OF INTEGER`.

**Incomplete (27)**, Ada. Used when the full type is declared in a package
body that may not be loaded.

- Body: `DST$L_TS_INCOMPLETE_PTR` (long). `DST$W_TS_LENGTH` = 5.
- The compiler writes 0 here. When the debugger later reads a Fulfills Type
  record, it fills in the pointer and changes the spec into an Indirect
  spec.
- For that reason the layout **must stay identical to `DST$K_TS_IND`**.
- While the pointer is still 0, no type information is available.

**BLISS block (28).** Describes a BLISS `BLOCK[n, unit] FIELD(…)`. A REF
BLOCK is described as a typed pointer to a block, and a BLOCKVECTOR as an
array of blocks. This kind replaces the older `DST$K_BLI` record.

| Field | Size | Meaning |
| - | - | - |
| `DST$L_TS_NUMBER_UNITS` | long | *n*, the number of units |
| `DST$B_TS_UNIT_SIZE` | byte | Unit size: 1 = BYTE, 2 = WORD, 4 = LONG |
| `DST$B_TS_FIELD_SET_COUNT` | byte | Number of field-set pointers that follow |
| `DST$A_TS_FIELD_SET_LIST` | var | Longword DST offsets of BLISS Field records |

---

## 10. Enumeration types

An enumeration type is written as one Begin record, then one Element record
per literal, then an End record. Only Continuation records may appear among
them.

- The elements do not have to be in order of their values, although that is
  preferred.
- For languages whose literal values need not increase with position, such
  as Ada, the elements **must** be in order of logical position.
- An object of an enumeration type uses a Separate Type Spec record. When
  several objects share the type, each uses a Separate Type Spec followed by
  an Indirect type spec that points to the Enumeration Begin.

### 10.1 Enumeration Type Begin (`DST$K_ENUMBEG` = 165), `DST$ENUM_BEGIN`

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_ENUMBEG_LENG` | Bit length of objects of the type |
| 3 | byte | `DST$B_ENUMBEG_NAME` | Length of the type name |
| 4 | var | — | Type name |

`DST$K_ENUM_BEGIN_SIZE` = 4.

### 10.2 Enumeration Type Element (`DST$K_ENUMELT` = 164)

Uses the Standard Data layout: flags byte, value longword, then the name.
The values are treated as unsigned. Normally `VALKIND` = LITERAL and
`DST$L_VALUE` holds the literal's value. (The stack-machine `POS` operator
requires LITERAL.)

### 10.3 Enumeration Type End (`DST$K_ENUMEND` = 166)

Header only. `DST$K_ENUM_END_SIZE` = 2.

---

## 11. BLISS-specific records

### 11.1 Field sets

A BLISS field set is written as a Field Set Begin record, then BLISS Field
records, then a Field Set End record.

- **Field Set Begin** (`DST$K_BLIFLDBEG` = 131), `DST$BLISS_FIELD_BEGIN`:
  the header, then `DST$B_BLIFLDBEG_NAME` (a count byte), then the name.
  `DST$K_BLISS_FIELD_BEGIN_SIZE` = 3.
- **Field Set End** (`DST$K_BLIFLDEND` = 132), `DST$BLISS_FIELD_END`: header
  only. `DST$K_BLISS_FIELD_END_SIZE` = 2.

### 11.2 BLISS Field (`DST$K_BLIFLD` = 183), `DST$BLISS_FIELD`

Binds a BLISS field name to a tuple of integers. The tuple is usually
(offset, bit position, size, sign-extend), but it can have any length. The
debugger uses these names as BLOCK and BLOCKVECTOR subscripts and can also
EVALUATE them.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `dst$b_blifld_unused` | Must be zero |
| 3 | long | `DST$L_BLIFLD_COMPS` | Number of components in the tuple |
| 7 | byte | `DST$B_BLIFLD_NAME` | Length of the name |
| 8 | var | — | Name |
| … | 4 × COMPS | — | Component values, one longword each |

`DST$K_BLISS_FIELD_SIZE` = 8.

A Field record outside any Begin/End pair is accepted for compatibility with
older compilers. It defines a field visible throughout the module rather
than one that belongs to a set. Only BLISS compilers should emit this
record.

### 11.3 BLISS Data record (`DST$K_BLI` = 0), obsolete

This older form describes BLISS VECTOR, BITVECTOR, BLOCK and BLOCKVECTOR
objects, and REFs to them. It is still accepted, but new compilers should
use these forms instead:

| Object | Records to emit |
| - | - |
| VECTOR, BITVECTOR, BLOCKVECTOR | Separate Type Spec, then an Array type spec |
| BLOCK | Separate Type Spec, then a BLISS Block type spec |
| REF … | Separate Type Spec, then a Typed Pointer type spec |

Layout, using `DST$BLI_FIELDS` followed by two trailers:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_BLI_LNG` | Byte count from offset 3 up to the first trailer (3…12) |
| 3 | byte | `DST$B_BLI_FORMAL` | Nonzero if the symbol is a routine formal parameter |
| 4 | byte | `DST$B_BLI_VFLAGS` | Value flags, as in Standard Data |
| 5 | byte | `DST$B_BLI_SYM_TYPE` | Bits 0–2 are `DST$V_BLI_STRUC`, bits 3–6 are `DST$V_BLI_MBZ`, and bit 7 is `DST$V_BLI_REF` (set for a REF) |
| 6 | var | `DST$A_BLI_SYM_ATTR` | Attributes that depend on the structure kind, listed below |

`DST$K_BLI_TRLR1` = 3: the first trailer starts at `3 + DST$B_BLI_LNG`.
`DST$K_BLI_SYM_ATTR` = 6. `DST$K_BLI_FIELDS_SIZE` = 15 is the largest form.

Structure kinds (`DST$V_BLI_STRUC`):

| Value | Symbol | Attributes |
| - | - | - |
| 0 | `DST$K_BLI_NOSTRUC` | None |
| 1 | `DST$K_BLI_VEC` | `DST$L_BLI_VEC_UNITS` (long); then one byte holding `DST$V_BLI_VEC_UNIT_SIZE` (bits 0–3: 1, 2 or 4) and `DST$V_BLI_VEC_SIGN_EXT` (bits 4–7) |
| 2 | `DST$K_BLI_BITVEC` | `DST$L_BLI_BITVEC_SIZE` (long, number of bits) |
| 3 | `DST$K_BLI_BLOCK` | `DST$L_BLI_BLOCK_UNITS` (long); then one byte holding `DST$V_BLI_BLOCK_UNIT_SIZE` (bits 0–3) and `DST$V_BLI_BLOCK_MBZ` (bits 4–7) |
| 4 | `DST$K_BLI_BLKVEC` | `DST$L_BLI_BLKVEC_BLOCKS` (long), `DST$L_BLI_BLKVEC_UNITS` (long), `DST$B_BLI_BLKVEC_UNIT_SIZE` (byte) |

`DST$K_BLI_STRUC_MIN` = 0 and `DST$K_BLI_STRUC_MAX` = 4.

Trailer 1, `DST$BLI_TRAILER1` (`DST$K_BLI_TRAILER1_SIZE` = 5):
`DST$L_BLI_VALUE` (long, interpreted according to `DST$B_BLI_VFLAGS`), then
`DST$B_BLI_NAME` (a count byte), then the name.

Trailer 2, `DST$BLI_TRAILER2` (`DST$K_BLI_TRAILER2_SIZE` = 4), at
`trailer1 + 5 + name length`: `DST$L_BLI_SIZE`, the object's size in bytes.

---

## 12. Image, PSECT, label and entry records

### 12.1 Image (`DST$K_IMAGE` = 135)

Compilers never emit this record. The debugger builds it internally to
represent shareable images. It uses the Standard Data layout, with the flags
byte and value longword unused and the name field holding the image name.
`DST$K_IMAGE_SIZE` = 8.

### 12.2 PSECT (`DST$K_PSECT` = 184)

Gives a program section's name, start address and length. The debugger
pays attention to it **only in MACRO modules**, where code or data can begin
at a PSECT base without any other label. It ignores this record for all
other languages.

`DST$PSECT` (`DST$K_PSECT_HEADER_SIZE` = 8):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `dst$b_psect_unused` | Must be zero |
| 3 | long | `DST$L_PSECT_VALUE` | Start address of the PSECT |
| 7 | byte | `DST$B_PSECT_NAME` | Length of the name. The same byte is also named `DST$B_PSECT_TRLR_OFFS`, because it is also the offset to the trailer |
| 8 | var | — | Name |

Trailer `DST$PSECT_TRAILER` (`DST$K_PSECT_TRAILER_SIZE` = 4), at
`record + DST$K_PSECT_HEADER_SIZE + DST$B_PSECT_TRLR_OFFS`:
`DST$L_PSECT_SIZE`, the PSECT length in bytes.

### 12.3 Label (`DST$K_LABEL` = 187)

Names a code address. This does not cover routines, blocks, or entry
points, which have their own records. The layout is Standard Data with the
flags byte set to zero, `DST$L_VALUE` holding the code address, and then the
name. `DST$K_LABEL_SIZE` = 8. High-level languages should use this record
for labels.

### 12.4 Label-or-Literal (`DST$K_LBLORLIT` = 186)

Intended for MACRO, where labels and literals are hard to tell apart. The
layout is Standard Data. Only `DST$V_VALKIND` (bits 0–1) is used, and the
other flag bits must be zero:

- `DST$K_VALKIND_ADDR`: the symbol is a label at `DST$L_VALUE`.
- `DST$K_VALKIND_LITERAL`: the symbol is a constant equal to `DST$L_VALUE`.

`DST$K_LBL_OR_LIT_SIZE` = 8. High-level languages should use Label or
Standard Data records instead.

### 12.5 Entry Point (`DST$K_ENTRY` = 181), `DST$ENTRY`

Describes a **secondary** entry point, such as a FORTRAN or PL/I ENTRY,
into the routine that encloses this record. It must not be emitted for the
routine's main entry point, which the Routine Begin already gives. Entry
points are always assumed to use CALLS/CALLG linkage.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_ENTRY_FLAGS` | Bits 0–7 are `DST$V_ENTRY_MBZ`, all zero |
| 3 | long | `DST$L_ENTRY_ADDRESS` | Entry address |
| 7 | byte | `DST$B_ENTRY_NAME` | Length of the name |
| 8 | var | — | Name |

`DST$K_ENTRY_SIZE` = 8. On this platform the fields sit in the same places
as in Standard Data, so the data-record field names also work. The separate
`ENTRY` names exist because the layout differs on other platforms.

---

## 13. Line-number to PC correlation (`DST$K_LINE_NUM` = 185)

### 13.1 Record and stream rules

```text
header  (DST$LINE_NUM_HEADER, DST$K_LINE_NUM_HEADER_SIZE = 2)
commands...  (starting at dst$a_line_num_data)
```

- The records may appear anywhere between a module's Begin and End records,
  and the debugger treats them as describing the whole module. They do not
  have to be nested inside the routines they describe.
- All line-number records in a module together form **one continuous
  command stream**. A single command must not be split across two records.
- Continuation records must **not** be used to extend line-number records.

### 13.2 Command encoding

Each command is a signed command byte (`DST$B_PCLINE_COMMAND`), optionally
followed by one parameter: an unsigned byte (`DST$B_PCLINE_UNSBYTE`), an
unsigned word (`DST$W_PCLINE_UNSWORD`), or a longword
(`DST$L_PCLINE_UNSLONG`). The aggregate is `DST$PCLINE_COMMANDS`, and
`DST$K_PCLINE_COMMANDS_SIZE` = `DST$K_PCLINE_COMMANDS_SIZE_MAX` = 5.

- A command byte that is **zero or negative**, in the range
  `DST$K_DELTA_PC_LOW` (−128) to `DST$K_DELTA_PC_HIGH` (0), is a one-byte
  **Delta-PC** command. The PC advances by the byte's absolute value.
- A **positive** byte is one of the commands in this table:

| Code | Symbol | Parameter |
| - | - | - |
| 1 | `DST$K_DELTA_PC_W` | word |
| 2 | `DST$K_INCR_LINUM` | byte |
| 3 | `DST$K_INCR_LINUM_W` | word |
| 4 | `DST$K_SET_LINUM_INCR` | byte |
| 5 | `DST$K_SET_LINUM_INCR_W` | word |
| 6 | `DST$K_RESET_LINUM_INCR` | — |
| 7 | `DST$K_BEG_STMT_MODE` | — |
| 8 | `DST$K_END_STMT_MODE` | — |
| 9 | `DST$K_SET_LINUM` | word |
| 10 | `DST$K_SET_PC` | byte |
| 11 | `DST$K_SET_PC_W` | word |
| 12 | `DST$K_SET_PC_L` | long |
| 13 | `DST$K_SET_STMTNUM` | see note |
| 14 | `DST$K_TERM` | byte |
| 15 | `DST$K_TERM_W` | word |
| 16 | `DST$K_SET_ABS_PC` | long |
| 17 | `DST$K_DELTA_PC_L` | long |
| 18 | `DST$K_INCR_LINUM_L` | long |
| 19 | `DST$K_SET_LINUM_B` | byte |
| 20 | `DST$K_SET_LINUM_L` | long |
| 21 | `DST$K_TERM_L` | long |

`DST$K_PCCOR_LOW` = −128 and `DST$K_PCCOR_HIGH` = 21.

> **Note on `DST$K_SET_STMTNUM`.** The code list calls it a byte command,
> but the description of what it does reads its operand as an unsigned
> word. Implementations should check against real compiler output. Reading
> a **word** follows the description of its behavior.

### 13.3 Interpreter state

| Variable | Initial value | Meaning |
| - | - | - |
| `CURRENT_LINE` | 0 | Listing line number |
| `CURRENT_STMT` | 1 | Statement number within the line |
| `CURRENT_INCR` | 1 | Amount a Delta-PC adds to the line number |
| `CURRENT_STMT_MODE` | false | When true, Delta-PC advances the statement number instead of the line |
| `START_PC` | lowest routine start address in the module | Base for relative Set-PC |
| `CURRENT_PC` | `START_PC` | Current code address |
| `CURRENT_MARK` | `LINE_CLOSED` | Whether a line is currently open |

### 13.4 Command semantics

*Delta-PC family* (an inline negative byte, `DELTA_PC_W`, `DELTA_PC_L`).
Moves to the start of the next line or statement:

```text
if STMT_MODE: STMT += 1  else: LINE += INCR
PC   += delta             ; for the inline form, delta = -command_byte
MARK  = LINE_OPEN
; now (LINE, STMT) begins at PC
```

*Line-number adjustments:*

| Command | Effect |
| - | - |
| `INCR_LINUM`, `_W`, `_L` | `LINE += n`. If in statement mode, `STMT = 1` |
| `SET_LINUM_B`, `SET_LINUM` (word), `SET_LINUM_L` | `LINE = n` |
| `SET_LINUM_INCR`, `_W` | `INCR = n`. If in statement mode, `STMT = 1` |
| `RESET_LINUM_INCR` | `INCR = 1`. If in statement mode, `STMT = 1` |

*Statement mode* is for languages that allow several statements on one
line:

| Command | Effect |
| - | - |
| `BEG_STMT_MODE` | Valid only while a line is open. Sets `STMT_MODE = true` and `STMT = 1` |
| `END_STMT_MODE` | Sets `STMT_MODE = false` and `STMT = 1` |
| `SET_STMTNUM` | `STMT = n`. Meaningful only in statement mode |

*PC positioning* is valid only while lines are closed, that is, between
program units:

| Command | Effect |
| - | - |
| `SET_PC`, `_W`, `_L` | `PC = START_PC + n` |
| `SET_ABS_PC` | `PC = n` |

*Termination:*

| Command | Effect |
| - | - |
| `TERM`, `TERM_W`, `TERM_L` | `PC += n`, then `MARK = LINE_CLOSED` |

`n` is the length in bytes of the last line defined. A Delta-PC only gives
where a line *starts*, so this is what fixes where the last line ends.
Nothing should come between the last Delta-PC and its TERM, or the debugger
may compute the final range wrongly.

The length of a line is the difference between consecutive PCs at which
lines start, or up to the PC reached by a TERM.

---

## 14. Source file correlation (`DST$K_SOURCE` = 155, `DST$K_EDIT_SOURCE` = 139)

### 14.1 Purpose and record structure

These records map listing line numbers, the same numbers the line-number
records use, to *(source file, record number)* pairs. With them, the
debugger can show source text.

Each record is a header (`DST$SOURCE_CORR`, `DST$K_SOURCE_CORR_HEADER_SIZE`
= 2) followed by a series of commands that starts at `dst$a_src_first_cmd`.
Rules for the command stream:

- A module's commands may be split across any number of records. They are
  processed in the order the records appear.
- Commands may be scattered anywhere in the module.
- Continuation records must not be used for these records.

`DST$K_EDIT_SOURCE` records have exactly the same format. They supply a
separate mapping used only by the debugger's EDIT command. Ada uses this so
that EDIT opens the original source file rather than the compiled form. If
any EDIT records are present, they must describe the **complete** mapping
on their own. They are not merged with the normal `DST$K_SOURCE` records.

### 14.2 Model

| State variable | Initial value | Meaning |
| - | - | - |
| `LINE_NUM` | 1 | Listing line number |
| `SRC_FILE` | undefined | File ID of the current source file |
| `SRC_REC` | undefined | Record number within that file |

The single operation is `DEFINE(LINE_NUM, SRC_FILE, SRC_REC)`, which binds
a listing line to a source record.

- Lines must be DEFINEd in **ascending order of line number**. The source
  records they map to can be in any order.
- One module may draw its source from several files, through plus-lists or
  INCLUDE, and some of those files may be modules in source libraries.

**Form-feed records.** A record that holds only a form feed (^L) is
normally **ignored**: it is not shown and gets no record number. Issue
`DST$K_SRC_FORMFEED` to count such records like any other. That command
must come before any command that defines a line, so the first command of
the first source record is the best place for it.

### 14.3 Command codes

| Code | Symbol | Operand | Effect |
| - | - | - | - |
| 1 | `DST$K_SRC_DECLFILE` | Declare File structure (§14.4) | Declares a source file and assigns its File ID |
| 2 | `DST$K_SRC_SETFILE` | word File ID | `SRC_FILE = id`. `SRC_REC` becomes the current record position remembered for that file |
| 3 | `DST$K_SRC_SETREC_L` | long | `SRC_REC = n` |
| 4 | `DST$K_SRC_SETREC_W` | word | `SRC_REC = n` |
| 5 | `DST$K_SRC_SETLNUM_L` | long | `LINE_NUM = n` |
| 6 | `DST$K_SRC_SETLNUM_W` | word | `LINE_NUM = n` |
| 7 | `DST$K_SRC_INCRLNUM_B` | byte | `LINE_NUM += n` |
| 8 | `DST$K_SRC_UNUSED1` | — | Reserved |
| 9 | `DST$K_SRC_UNUSED2` | — | Reserved |
| 10 | `DST$K_SRC_DEFLINES_W` | word *n* | Repeat *n* times: `DEFINE`, then `LINE_NUM++` and `SRC_REC++` |
| 11 | `DST$K_SRC_DEFLINES_B` | byte *n* | Same as code 10 |
| 12–15 | `DST$K_SRC_UNUSED3` … `DST$K_SRC_UNUSED6` | — | Reserved |
| 16 | `DST$K_SRC_FORMFEED` | — | Count form-feed-only records as source lines |

`DST$K_SRC_MIN_CMD` = 1 and `DST$K_SRC_MAX_CMD` = 16.

The generic aggregate is `DST$SRC_COMMAND`. Its command byte is
`DST$B_SRC_COMMAND`, and its operand overlays are `DST$L_SRC_UNSLONG`,
`DST$W_SRC_UNSWORD` and `DST$B_SRC_UNSBYTE`. `DST$K_SRC_COMMAND_SIZE` = 21,
the size of the largest fixed form.

### 14.4 Declare Source File command

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | byte | `DST$B_SRC_COMMAND` | = 1 |
| 1 | byte | `DST$B_SRC_DF_LENGTH` | Number of bytes left in this command after this byte |
| 2 | byte | `DST$B_SRC_DF_FLAGS` | Reserved, must be zero |
| 3 | word | `DST$W_SRC_DF_FILEID` | File ID chosen by the compiler, unique within the module |
| 5 | quad | `DST$Q_SRC_DF_RMS_CDT` | File creation date and time, or the module's insertion time for a library module |
| 13 | long | `DST$L_SRC_DF_RMS_EBK` | End-of-file block (0 for a library module) |
| 17 | word | `DST$W_SRC_DF_RMS_FFB` | First free byte in the EOF block (0 for a library module) |
| 19 | byte | `DST$B_SRC_DF_RMS_RFO` | Record format and file organization (0 for a library module) |
| 20 | byte + var | `DST$B_SRC_DF_FILENAME` | Count byte, then the fully resolved file specification. For a library module this is the library's file name |
| … | byte + var | `DST$B_SRC_DF_LIBMODNAME` | Count byte, then the library module name, or an empty string |

`DST$K_SRC_DF_FILENAME_BASE` = 21 is where the file name text starts.

The library-module name sits in the trailer aggregate `DST$SRC_CMDTRLR`
(`DST$K_SRC_CMDTRLR_SIZE` = 1). It begins right after the file name text,
and its text starts at `dst$a_src_df_libmodname`.

The compiler should fill in the date, EOF block, first free byte and record
format from the file system's attributes when it opens the file. The
debugger uses those values to confirm it has found the right file version.
If the EBK, FFB or RFO field is −1, meaning all bits set, the debugger
skips that check.

---

## 15. GEM locator PC correlation (`DST$K_PCLOC` = 192)

### 15.1 Purpose

This is a finer-grained alternative to the line-number program, used by
GEM-based compilers. It tags individual instructions with a source location
(a point or a range of line and column) and with semantic *events*. The
debugger uses the events to make stepping through optimized code smoother.

The placement rules match the line-number records: anywhere inside the
module, one continuous command stream per module, no command split across
records, and no Continuation records. The header is `DST$GEM_LOC_HEADER`
(`DST$K_GEM_LOC_HEADER_SIZE` = 2), and the data starts at
`dst$a_gem_loc_data`.

### 15.2 Commands

The interpreter keeps one state variable, `CURRENT_PC`, which starts at 0.
Commands that end in `_INCR` add 4 to `CURRENT_PC` after recording, since 4
bytes is one instruction.

| Code | Symbol | Operands (after the command byte) | Effect |
| - | - | - | - |
| 0 | `DST$K_PCLOC_END` | — | End of the table |
| 1 | `DST$K_PCLOC_PNTS_INCR` | short point | Record the point at PC, then PC += 4 |
| 2 | `DST$K_PCLOC_PNTL_INCR` | long point | Same, using the long form |
| 3 | `DST$K_PCLOC_RNGS_INCR` | short range | Record the range at PC, then PC += 4 |
| 4 | `DST$K_PCLOC_RNGL_INCR` | long range | Same, using the long form |
| 5 | `DST$K_PCLOC_PNTS` | short point | Record the point at PC (PC unchanged) |
| 6 | `DST$K_PCLOC_PNTL` | long point | Same, using the long form |
| 7 | `DST$K_PCLOC_RNGS` | short range | Record the range at PC (PC unchanged) |
| 8 | `DST$K_PCLOC_RNGL` | long range | Same, using the long form |
| 9 | `DST$K_PCLOC_INCR` | — | PC += 4 |
| 10 | `DST$K_PCLOC_SETPC64` | quad | PC = value. Only the low 32 bits are honored |
| 11 | `DST$K_PCLOC_SETPC32` | long | PC = value |
| 12 | `DST$K_PCLOC_EVENT_INST` | — | Instruction-level event, see below |
| 13 | `DST$K_PCLOC_EVENT_READ` | long sym1, long sym2 | Data-read event |
| 14 | `DST$K_PCLOC_EVENT_WRITE` | long sym | Complete-write event |
| 15 | `DST$K_PCLOC_EVENT_CTRL` | — | Control-flow event |
| 16 | `DST$K_PCLOC_EVENT_CALL` | — | Call or return event |
| 17 | `DST$K_PCLOC_LINS` | same-line range | Record the range at PC (PC unchanged) |
| 18 | `DST$K_PCLOC_LINS_INCR` | same-line range | Record the range at PC, then PC += 4 |
| 19 | `DST$K_PCLOC_EVENT_PWRIT` | not specified | Partial-write event |
| 20 | `DST$K_PCLOC_EVENT_LABEL` | not specified | The instruction is the target of a label |

`DST$K_PCLOC_LOW` = 0 and `DST$K_PCLOC_HIGH` = 20.

### 15.3 Operand layouts (`DST$PCLOC_COMMANDS`)

The command byte is `DST$B_PCLOC_COMMAND`. The operands are:

| Form | Fields |
| - | - |
| Short point | `DST$W_PCLOC_PNTS_LINE` (word), `DST$B_PCLOC_PNTS_COLUMN` (byte) |
| Long point | `DST$L_PCLOC_PNTL_LINE` (long), `DST$W_PCLOC_PNTL_COLUMN` (word) |
| Short range | `DST$W_PCLOC_RNGS_LOW_LINE`, `DST$B_PCLOC_RNGS_LOW_COLUMN`, `DST$W_PCLOC_RNGS_HIGH_LINE`, `DST$B_PCLOC_RNGS_HIGH_COLUMN` |
| Long range | `DST$L_PCLOC_RNGL_LOW_LINE`, `DST$W_PCLOC_RNGL_LOW_COLUMN`, `DST$L_PCLOC_RNGL_HIGH_LINE`, `DST$W_PCLOC_RNGL_HIGH_COLUMN` |
| Same-line range | `DST$W_PCLOC_LINS_LINE` (word), `DST$B_PCLOC_LINS_LOW_COLUMN` (byte), `DST$B_PCLOC_LINS_HIGH_COLUMN` (byte) |
| 64-bit set PC | `DST$Q_PCLOC_SETPC64_VALUE` |
| 32-bit set PC | `DST$L_PCLOC_SETPC32_VALUE` |
| Read event | `DST$L_PCLOC_EVENT_READ_SYM1`, `DST$L_PCLOC_EVENT_READ_SYM2` |
| Write event | `DST$L_PCLOC_EVENT_WRITE_SYM` |

For every range, the low end must not come after the high end. That is,
`low_line ≤ high_line`, and when the lines are equal,
`low_col ≤ high_col`.

Total command sizes, including the command byte:

| Constant | Value |
| - | - |
| `DST$K_PCLOC_CMD_SIZE_END`, `DST$K_PCLOC_CMD_SIZE_INCR`, `DST$K_PCLOC_CMD_SIZE_EVENT` | 1 |
| `DST$K_PCLOC_CMD_SIZE_PNTS` | 4 |
| `DST$K_PCLOC_CMD_SIZE_PNTL` | 7 |
| `DST$K_PCLOC_CMD_SIZE_RNGS` | 7 |
| `DST$K_PCLOC_CMD_SIZE_RNGL` | 13 |
| `DST$K_PCLOC_CMD_SIZE_LINS` | 5 |
| `DST$K_PCLOC_CMD_SIZE_SETPC64` | 9 |
| `DST$K_PCLOC_CMD_SIZE_SETPC32` | 5 |
| `DST$K_PCLOC_CMD_SIZE_EVENT_RD` | 9 |
| `DST$K_PCLOC_CMD_SIZE_EVENT_WR` | 5 |

### 15.4 Event meanings

| Event | Meaning |
| - | - |
| **INST** | The location at this PC matters only when stepping by instruction. Line-level stepping skips it, which suits address-constant loads and similar housekeeping instructions |
| **READ** | The instruction reads one or two user variables. `SYM1` is the DST offset of one variable's definition and must be nonzero. `SYM2` is the second variable's offset, or 0 if only one is read |
| **WRITE** | The instruction completely overwrites a user variable. `SYM` is its DST offset and must be nonzero |
| **CTRL** | A conditional or unconditional branch. Calls and returns use CALL instead |
| **CALL** | A routine call (JSB) or a return (RET) |

Instructions that touch only registers or temporaries carry no READ or
WRITE events. Only accesses to user-defined symbols do.

---

## 16. Continuation records (`DST$K_CONTIN` = 173)

`DST$B_LENGTH` is one byte, so a single record can hold at most 255 bytes.
To write a longer logical record:

1. Emit the first part in an ordinary record, holding between 100 and 255
   bytes of record text.
2. Follow it with as many Continuation records as needed. Each one is a
   2-byte header (`DST$CONTINUATION`, `DST$K_CONTINUATION_HEADER_SIZE` = 2)
   followed by the next chunk of text, which starts at `dst$a_contin`.

The reader rebuilds the record by joining the first record's text with the
text of each Continuation record, dropping the Continuation headers. All
further parsing works on the joined copy.

**Records that may be continued:**

- Standard Data and its variants (Descriptor Format, Trailing VS)
- Separate Type Spec
- Type Specification
- Split-lifetime value specs, along with the other kinds above
- Records inside an enumeration that are allowed there

**Records that may not be continued:**

- Module Begin, Routine Begin, Block Begin
- Label, Label-or-Literal, Entry, PSECT
- Line Number PC-Correlation and Source Correlation. These are split by
  emitting several records instead
- Every fixed-size record, such as Module End and Routine End

---

## 17. GOTO and TARGET records

These records let a compiler write DST records out of order, for example
when it generates code for an inner routine before the outer one.

- **GOTO** (`DST$K_GOTO` = 142), `DST$GOTO` (`DST$K_GOTO_SIZE` = 6): the
  header, then `DST$L_GOTO_PTR`, the DST offset of a TARGET record. When the
  debugger reaches a GOTO while scanning a module, it continues from the
  target.
- **TARGET** (`DST$K_TARGET` = 143): header only. The debugger skips it and
  uses it only to check that a GOTO points somewhere valid.

Example: emitting an inner routine B first, then its enclosing routine A,
while keeping the logical nesting.

```text
GOTO T2
T1: TARGET
    Routine Begin B … Routine End B
    GOTO T3
T2: TARGET
    Routine Begin A … (A's symbols)
    GOTO T1
T3: TARGET
    Routine End A
```

This mechanism works, but writing records inside-out can make the
debugger's view of the program worse, so compilers should avoid it.

---

## 18. Fixup records

When a DST belongs to a shareable image, some of its addresses are relative
to the base of that image or some other image. The linker writes fixup
records **after the last Module End**. Zero bytes of padding may come
between that Module End and the first fixup record.

### 18.1 DST Fixup (`DST$K_FIXUP` = 134) and DST Fixup-64 (`DST$K_FIXUP_64` = 117)

The body of every fixup record is one or more groups, and the groups from
all fixup records are joined into a single table:

```text
group:
    count of offsets
    counted-ASCII image name
    offset[count]       ; each one is a byte offset from the start of the DST
```

For each listed offset, the reader adds the run-time base address of the
named image to the field at that offset. `DST$K_FIXUP` patches longword
fields and `DST$K_FIXUP_64` patches quadword fields. Otherwise the two
records are identical.

> The source shows this layout only as a diagram. It gives no SDL aggregate
> and does not state how wide the count and offset fields are. Longwords
> are the natural assumption. Check against real images.

### 18.2 Symbol Fixup (`DST$K_SYMBOL_FIXUP` = 121) and Symbol Fixup-64 (`DST$K_SYMBOL_FIXUP_64` = 116)

These exist on Alpha to support FORTRAN shared COMMON. The value added
comes from an entry in a shareable image's **symbol vector**, not from the
image's base address.

```text
group:
    count of offsets
    counted-ASCII image name
    offset of the entry within that image's symbol vector
    offset[count]       ; DST locations to patch
```

All Symbol Fixup records come after all ordinary Fixup records, with no
padding in between. The 64-bit variant patches quadword fields.

The original text names `DST$K_SYMBOL_FIXUP` where it means the quadword
variant. It is clearly referring to `DST$K_SYMBOL_FIXUP_64`.

---

## 19. Routine-related auxiliary records

### 19.1 Prolog (`DST$K_PROLOG` = 162), `DST$PROLOG`

Gives the address where routine breakpoints should go, after the prolog
has set up locals and parameters.

- The record is optional. Without it, the debugger breaks at the routine's
  start address.
- It belongs to the nearest preceding Routine Begin or Entry record,
  ignoring nested routines. Best practice is to place it immediately after
  that record.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header (`DST$B_LENGTH` = 5) |
| 2 | long | `DST$L_PROLOG_BKPT_ADDR` |

`DST$K_PROLOG_SIZE` = 6.

### 19.2 Prolog List (`DST$K_PROLOG_LIST` = 119), `DST$PROLIST`

The same as Prolog, but it gives several breakpoint addresses.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | long | `DST$LU_PROLIST_COUNT` |
| 6 | 4 × count | `DST$PROLIST_ENTRY` entries, each holding `DST$LU_PROLIST_BKPT_ADDR` |

`DST$K_PROLIST_SIZE` = 6 and `DST$K_PROLIST_ENTRY_SIZE` = 4. The list
starts at `dst$a_prolist`. The addresses must be distinct and in ascending
order.

### 19.3 Epilog (`DST$K_EPILOGS` = 137, legacy `DST$K_EPILOG` = 127), `DST$EPILOG`

Lists where a routine's epilog sequences begin, or the address ranges they
cover. Breaking there stops the routine just before it returns, while its
local variables can still be read.

- The record goes inside Routine Begin/End, or, with inlining, inside Block
  Begin/End.
- Compilers should use `DST$K_EPILOGS`. It means the same as `DST$K_EPILOG`,
  but older debuggers rejected `DST$K_EPILOG` inside blocks. They simply
  ignore the newer code, so it is safe to emit.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_EPILOG_FLAGS` | Bit 0 is `DST$V_EPILOG_ADDR_PAIRS_FLAG` (the list holds pairs, not single addresses); bits 1–7 are `DST$V_EPILOG_MBZ` |
| 3 | long | `DST$LU_EPILOG_COUNT` | Number of entries |
| 7 | var | `dst$a_epilogs` | The entries |

`DST$K_EPILOG_SIZE` = 7. The entry formats are:

- Single address, `DST$EPI_SINGLETON` (`DST$K_EPILOG_SINGLETON_SIZE` = 4):
  `DST$LU_EPILOG_SINGLE_ADDR`.
- Pair, `DST$EPI_PAIR` (`DST$K_EPILOG_PAIR_SIZE` = 8):
  `DST$LU_EPILOG_PAIR_LOW_ADDR` and `DST$LU_EPILOG_PAIR_HIGH_ADDR`.

These addresses are where the epilog code begins, which is not the same as
the address of the return instruction.

### 19.4 Return (`DST$K_RETURN` = 126), `DST$RETURN`

Lists a routine's return-instruction addresses for execution profilers. It
is placed the same way as Epilog records.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | byte | `dst$b_return_mbz` (must be zero) |
| 3 | long | `DST$LU_RETURN_COUNT` |
| 7 | var | `dst$a_returns`: the addresses |

`DST$K_RETURN_SIZE` = 7. No aggregate is defined for the list elements.
Longword addresses, matching the Epilog singletons, are the natural
reading.

### 19.5 Static Link (`DST$K_STATLINK` = 156), `DST$STATLINK`

Gives the routine's *static link*: the frame pointer of the lexically
enclosing routine's activation, used for up-level addressing.

- The record is optional.
- It belongs to the innermost enclosing routine.
- It matters only in unusual cases, such as recursive routines that are
  passed as parameters.

Layout: the header (`DST$K_STATLINK_SIZE` = 2), then a value spec at
`dst$a_sl_valspec` that yields the up-level FP.

### 19.6 Register save records

These describe where registers are saved in routines that do not use the
CALLx register-save mask. The required order is:

```text
Register Save Begin
  Register Save    (one per saved register, in ascending register number)
  ...
Register Save End
```

- Only Register Save records may sit between Begin and End.
- The group may appear anywhere inside a Routine, Package Spec, or Package
  Body scope, and belongs to the nearest unmatched Begin of one of those.
- An Entry record must not come between the Register Save Begin and the
  routine or package it belongs to.
- A routine that saves no registers needs no group.

**Register Save Begin** (`DST$K_REG_SAVE_BEGIN` = 130), `DST$REG_SAVE_BEGIN`
(BLISS alias `DST$REGISTER_SAVE_BEGIN`):

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$BU_REGBEG_FLAGS` | Bit 0 is `DST$V_REGBEG_SAVE_MASK_FLAG` (a mask follows); bits 1–7 are `DST$V_REGBEG_MBZ` |
| 3 | byte | `DST$BU_REGBEG_SAVE_MASK_LENGTH` | Length of the mask in bytes |
| 4 | var | `DST$V_SAVE_MASK` | Bit vector, starting at `dst$v_regbeg_save_mask_base` |

`DST$K_REGISTER_SAVE_BEGIN_SIZE` = 4.

- The mask is required whenever any Register Save record follows, and it
  must have a bit set for every register described.
- Bits are numbered from 0 upward, starting at the least significant bit
  of the lowest-addressed byte.
- Each bit position stands for a fixed register on the target, which also
  fixes that register's size. On 68K, A0–A7 are 0–7 (32-bit), D0–D7 are
  8–15 (32-bit), and FP0–FP7 are 16–23 (96-bit).

**Register Save** (`DST$K_REG_SAVE` = 129), `DST$REGISTER_SAVE`:

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | 5 | `DST$A_REG_SAVE_VALSPEC` | Value spec giving where the register is saved |
| 7 | byte | `DST$BU_REG_SAVE_REGNUM` | Register number, using the same numbering as the mask |

`DST$K_REGISTER_SAVE_SIZE` = 8.

- The value spec sits at the same offset as the flags and value fields of
  a Standard Data record, so code that reads data records can read it too.
- If the value spec is a split-lifetime spec, the register is saved only
  within those PC ranges.

**Register Save End** (`DST$K_REG_SAVE_END` = 128): header only.
`DST$K_REGISTER_SAVE_END_SIZE` = 2.

### 19.7 Definition Line Number (`DST$K_DEF_LNUM` = 170), `DST$DEF_LNUM`

Gives the listing line where the object in the **immediately preceding**
data record is declared. The debugger ignores this record, but compilers
may emit it.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header (`DST$B_LENGTH` = 6) |
| 2 | byte | `dst$b_def_lnum_mbz` |
| 3 | long | `DST$L_DEF_LNUM_LINE` |

`DST$K_DEF_LNUM_SIZE` = 7.

### 19.8 Version Number (`DST$K_VERSION` = 153), `DST$VERSION`

Gives the version of the compiler that produced the module. It must sit
inside the module. The debugger reads it only when it needs to tell old
compiler output from new.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header (`DST$B_LENGTH` = 3) |
| 2 | byte | `DST$B_VERSION_MAJOR` |
| 3 | byte | `DST$B_VERSION_MINOR` |

`DST$K_VERSION_SIZE` = 4.

### 19.9 COBOL Global Attribute (`DST$K_COBOLGBL` = 154)

Header only (`DST$B_LENGTH` = 1). It marks the symbol in the **next**
record as COBOL GLOBAL, meaning the symbol is also visible in nested
scopes. Only COBOL emits it, and the debugger ignores it for other
languages.

---

## 20. Ada-specific records

### 20.1 Overloaded Symbol (`DST$K_OVERLOAD` = 169)

Records that one name stands for several entities in the same scope.

- The compiler emits each overloaded entity under an invented unique name,
  such as `R__1` or `R__2`.
- It then emits this record under the real name `R`, listing the DST offset
  of each invented-name record.
- C++ uses this record the same way for overloaded functions.

| Field | Size | Meaning |
| - | - | - |
| header | 2 | |
| `DST$B_OL_NAME` | byte + var | The real name |
| `DST$W_OL_COUNT` | word | Number of instances |
| `DST$A_OL_VECTOR` | 4 × count | DST offsets of the instances |

The fixed part is `DST$OVERLOAD_HEADER`, with `DST$K_OVERLOAD_HEADER_SIZE`
= 3. The trailer `DST$OVERLOAD_TRLR` starts at `3 + name length`, and its
vector starts at `DST$K_OVERLOAD_VECTOR_BASE` (2) within the trailer.

### 20.2 Inlined Routine (not supported)

This record was proposed but never supported. It would list every inline
expansion of a routine under invented names. Its layout copies the
Overloaded Symbol record:

- `DST$INLINED_HEADER` holds `DST$B_IL_NAME`, with
  `DST$K_INLINED_HEADER_SIZE` = 3.
- The trailer `DST$INLINE_TRLR` holds `DST$W_IL_COUNT`, then the vector
  `DST$A_IL_VECTOR` at `DST$K_INLINE_VECTOR_BASE` (2).

The type codes in its picture (`DST$K_INLINE_BY_COMPILER`,
`DST$K_INLINE_BY_REQUEST`) are not defined anywhere. Use the Inline
Instance record (§5.6) instead.

### 20.3 Subunit (`DST$K_SUBUNIT` = 150), `DST$SUBUNIT`

Placed immediately after the Routine Begin of an Ada separate subunit.
It names the scope where the subunit logically belongs.

| Field | Size | Meaning |
| - | - | - |
| header | 2 | |
| `DST$B_SUBUNIT_PATHNAME_COUNT` | byte | Number of path elements plus one |
| — | var | Counted-ASCII strings, as described below |

`DST$K_SUBUNIT_SIZE` = 3.

The strings are:

1. The **parent module** (the module to SET when this subunit is SET).
2. The path from the outermost scope inward. It begins with the module
   that contains the stub, then each enclosing routine.

*Example:* subunit R3 is a stub inside routine R2, which is inside R1, and
the stub is in module R1. The count is 4, and the strings are `R1`, `R1`,
`R1`, `R2`. If R2 is itself a subunit, the parent module becomes R2: `R2`,
`R1`, `R1`, `R2`.

The parent should also declare the subunit's name as a data item of type
`DSC$K_DTYPE_ZEM` (entry mask), so the subunit stays reachable when only
the parent is SET. Setting the subunit's module automatically sets the
parent as well.

### 20.4 Set Module / WITH clause (`DST$K_SET_MODULE` = 151), `DST$SET_MODULE`

Setting the module that contains this record also sets the named module,
and cancelling it cancels both. Ada uses this for WITH clauses (one record
per item withed) and to pull in a generic's defining module so source
lookup works. The record must be inside the requesting module's
Begin/End.

Layout: the header, then `DST$B_SET_MODULE_NAME` (a count byte), then the
module name. `DST$K_SET_MODULE_SIZE` = 3.

### 20.5 USE clause (`DST$K_USE_CLAUSE` = 152), `DST$USE_CLAUSE`

Makes a package's contents directly visible within a scope.

- Place it immediately after the Routine, Block, or Package Begin of that
  scope.
- Emit one record per package used.

Layout: the header, then `DST$B_USE_PATHNAME_COUNT`, then that many
counted-ASCII path elements. The first element is the module that contains
the package, followed by any enclosing units, ending with the package name.
`DST$K_USE_CLAUSE_SIZE` = 3.

*Example:* `use P1.P2` (P2 nested in library package P1) gives count 3:
`P1`, `P1`, `P2`.

### 20.6 Package Real Name (`DST$K_REAL_NAME` = 144), `DST$REAL_NAME`

Placed after Package Spec Begin. It gives the package's real name when the
spec's module or package name was altered to tell it apart from the body,
for example `P_` versus `P`. This frees the debugger from guessing naming
conventions.

Layout: the header, then `DST$B_REAL_NAME` (a count byte), then the name.
`DST$K_REAL_NAME_SIZE` = 3.

### 20.7 Package Body→Spec (`DST$K_BODY_SPEC` = 145), `DST$BODY_SPEC`

Placed immediately after Package Body Begin. It names the spec that the
body implements, using the same path encoding as USE, for example `P`, `P`.
The debugger then treats the body as if it had both WITHed and USEd its
spec, so the compiler does not need to emit those records.

Layout: the header, then `DST$B_BODY_SPEC_PATHNAME_COUNT`, then the path
elements. `DST$K_BODY_SPEC_SIZE` = 3.

### 20.8 Alias (`DST$K_ALIAS` = 140), `DST$ALIAS`

Gives another name for a symbol, valid in the scope where the record
appears. Ada renaming uses it, and so do C++ namespace aliases.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | long | `DST$L_ALIAS_MOD_OFFSET` | Offset of the target record, counted from the start of the named module's DST |
| 6 | byte + var | `DST$B_ALIAS_NAME` | The alias |
| … | byte + var | — | Counted-ASCII name of the module the offset refers to. Empty means the current module |

`DST$K_ALIAS_SIZE` = 7.

### 20.9 Fulfills Type (`DST$K_FULFILLS_TYPE` = 133), `DST$FULFILLS_TYPE`

Works together with the Incomplete type spec (§9.5). It is emitted in the
unit that finally defines a deferred type.

1. Header (`DST$K_FULFILLS_TYPE_SIZE` = 2).
2. At `DST$A_FF_INCOMPLETE_TS`: an Indirect or Cross-Module Indirect type
   spec. It leads, possibly through a chain of further indirections, to the
   Incomplete type spec being fulfilled.
3. The **next** record, as with a Separate Type Spec, is a Type Spec,
   Record Begin, or Enumeration Begin record that defines the real type.
   It may itself be indirect.

When the debugger reads this record, it writes into the Incomplete spec's
pointer the DST offset of the record that follows the Fulfills record.
From then on, the Incomplete spec acts as an Indirect spec. The fill-in is
permanent for the session, even if the module that supplied it is later
cancelled.

### 20.10 Exception (`DST$K_EXCEPTION` = 125), `DST$EXCEPTION`

XD Ada identifies exceptions by number at run time. These records map each
number to a name, so tools can show names in call and stack displays.

- Emit one record per exception, sorted by ascending number. Keep the
  numbers small and dense, because the debugger builds a table indexed by
  number.
- Put them in a module that the debugger sets at startup. The usual way is
  a dummy module with a reserved name, flagged `DST$V_MODBEG_HIDE`, that the
  main unit references through a Set Module record.
- A duplicate number is an error only if its name is different. Names are
  compared case-sensitively.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$BU_EXCEPTION_FLAGS` | Bit 0 is `DST$V_EXCEPTION_MOD_NAME_FLAG` (a module-name trailer is present); bits 1–7 are `DST$V_EXCEPTION_MBZ` |
| 3 | long | `DST$LU_EXCEPTION_VALUE` | Exception number |
| 7 | byte | `DST$B_EXCEPTION_NAME` | Length of the name. The same byte is also named `DST$B_EXCEPTION_TRLR_OFFS`, because it is also the offset to the trailer |
| 8 | var | — | Name |

`DST$K_EXCEPTION_SIZE` = 8. The optional trailer is `DST$EXCEPTION_TRAILER`
(`DST$K_EXCEPTION_TRAILER_SIZE` = 1), at
`record + 8 + DST$B_EXCEPTION_TRLR_OFFS`. It holds `DST$B_EXCEPTION_MOD_NAME`
(a count byte), then the name of the defining module.

---

## 21. C++ support

### 21.1 How C++ constructs are written

A C++ program is written with the ordinary records, plus a few rules that
relax what may appear inside a Record Begin/End:

- **Classes, structs and unions** are written as Record Begin/End. A
  preceding C++ Attributes record tells struct and union apart from class,
  which is the default. `DST$L_RECBEG_SIZE` is the size of a complete
  object, including inherited subobjects.
- **Non-static data members** are data records with
  `DST$K_VFLAGS_BITOFFS`. The offset is measured from the most-derived
  non-virtual base, so it already includes the sizes of preceding
  non-virtual bases.
- **Static data members** are data records with ordinary static addresses.
- **Member functions** are Routine Begin/End pairs placed at the class's
  nesting level. A virtual function has a Virtual Function record inside
  its routine.
- Inherited members are **not** repeated in the derived class. Base Class
  records describe the inheritance instead.
- Enumerations and typedefs may appear inside a class. They are scoped like
  members.
- **The `this` pointer** is passed as the first argument, or second after
  the hidden return-buffer pointer when a function returns a struct. The
  debugger identifies it as the first parameter named `this`. There is no
  dedicated record for it (a proposed `This` record was dropped).
- **Overloaded functions** use the Overloaded Symbol record (§20.1) with
  invented names `name__1`, `name__2`, and so on. The debugger does not
  want mangled names.
- **Namespaces** are written as Record Begin/End preceded by a C++
  Attributes record with the namespace bit set.
  - All extensions of a namespace are merged into one definition.
  - An unnamed namespace has an empty name, plus an explicit Using
    directive record.
  - A member that is declared and defined separately is described as a
    single definition.
  - A namespace alias is an Alias record with an empty module name. Only
    one alias record per alias per scope.
- **Templates.** Each template gets a Template Declaration record in the
  scope where it is declared, so typed names like `Array<int>` resolve.
  Instances appear under their ordinary template names, with no explicit
  link to the declaration.

### 21.2 Base Class (`DST$K_BASE_CLASS` = 122), `DST$BASE_CLASS`

Place these inside the derived class's Record Begin/End, after the Record
Begin and before any members, in the order the bases are declared.

| Offset | Size | Field | Meaning |
| - | - | - | - |
| 0 | 2 | header | |
| 2 | byte | `DST$B_BASE_CLASS_FLAGS` | Bit 0 is `DST$V_BASE_CLASS_VIRTUAL`; bits 1–7 are `DST$V_BASE_CLASS_UNUSED` |
| 3 | long | `DST$L_BASE_CLASS_DST` | DST offset of the base class's Record Begin |
| 7 | long | `DST$L_BASE_CLASS_VALUE` | Non-virtual base: offset in bytes of the base subobject. Virtual base: index into the class's btbl, where the offset is found |

`DST$K_BASE_CLASS_SIZE` = 11.

### 21.3 Virtual Function (`DST$K_VIRT_FUNC` = 124), `DST$VIRT_FUNC`

- The record is optional. A routine without one is non-virtual.
- It must be inside the routine's Begin/End, preferably right after the
  Routine Begin.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | byte | `DST$B_VIRT_FUNC_FLAGS` (bits 0–7 are `DST$V_VIRT_FUNC_UNUSED`) |
| 3 | long | `DST$L_VIRT_FUNC_INDEX`: index into the vtbl |

`DST$K_VIRT_FUNC_SIZE` = 7.

**Hidden table pointers.** The vtbl pointer is described as a data member
named `__vptr`, whose type is a one-dimensional array. The function index
selects an element of that array. In the same way, `__bptr` describes the
virtual-base table, and a virtual base's `DST$L_BASE_CLASS_VALUE` indexes
it.

### 21.4 Type Signature (`DST$K_TYPE_SIG` = 138), `DST$TYPE_SIG`

Gives a function's argument-type signature, which the debugger shows when
it needs to tell overloaded functions apart. It goes inside the routine,
preferably right after the Routine Begin. Every function should have one,
even if it is not overloaded.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | byte | `DST$B_TYPE_SIG_FLAGS` (bits 0–7 are `DST$V_TYPE_SIG_UNUSED`) |
| 3 | byte + var | `DST$B_TYPE_SIG_STRING` (`DST$A_TYPE_SIG_STRING`): counted ASCII |

`DST$K_TYPE_SIG_SIZE` = 4.

- The compiler chooses the string format. It only needs to be consistent,
  including whitespace, `const` and `volatile`, and distinct for each
  distinct signature.
- To save space, `@` stands for the enclosing class name and `#` stands
  for the function name.
- The debugger only compares and displays these strings.
- Signatures exist because VMS truncates mangled names to 31 characters,
  which loses the type information.

### 21.5 C++ Attributes (`DST$K_CXX_ATTRIBUTES` = 141), `DST$CXX_ATTRIBUTES`

Applies to the symbol in the **next** record. Set the bits that apply. Bits
left clear mean the default.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | word | `DST$W_CXXA_FLAGS` |

`DST$K_CXX_ATTRIBUTES_SIZE` = 4.

| Bit | Field | Meaning |
| - | - | - |
| 0–2 | `DST$V_CXXA_UNUSED1`, `…2`, `…3` | Must be zero |
| 3 | `DST$V_CXXA_NAMESPACE` | The record that follows is a namespace |
| 4 | `DST$V_CXXA_STRUCT` | The record that follows is a struct |
| 5 | `DST$V_CXXA_UNION` | The record that follows is a union (the original comment is wrong here) |
| 6 | `DST$V_CXXA_STATIC` | The member is static |
| 7–15 | `DST$V_CXXA_UNUSED` | Must be zero |

With no Attributes record, a Record Begin is a class and members are
non-static.

### 21.6 Template Declaration (`DST$K_TEMP_DECL` = 123), `DST$TEMP_DECL`

Layout: the header, then `DST$B_TEMP_DECL_NAME` (a count byte), then the
template name, for example `Array`. `DST$K_TEMP_DECL_SIZE` = 3.

### 21.7 Using (`DST$K_USING` = 180), `DST$USING`

Represents a using-declaration or a using-directive. Place it where the
corresponding scope is.

| Offset | Size | Field |
| - | - | - |
| 0 | 2 | header |
| 2 | long | `DST$L_USING_DST`: DST offset of the namespace or namespace member being used |

`DST$K_USING_SIZE` = 6.

- A using-declaration that matches several symbols needs one Using record
  per symbol.
- When a using-declaration brings in base-class members, the members that
  the derived class hides must **not** get Using records.

---

## 22. Obsolete records

New compilers must not emit these. The debugger may or may not still
accept them.

| Code | Symbol | Notes |
| - | - | - |
| 160 | `DST$K_GLOBNXT` | Global-is-next. Header only. Never implemented |
| 159 | `DST$K_EXTRNXT` | External-is-next. Header only. Never implemented |
| 182 | `DST$K_LINE_NUM_REL_R11` | Threaded-code PC correlation for an old COBOL compiler. Same layout as `DST$K_LINE_NUM`, but the "PC" values are offsets from R11 into a threaded-code vector. No longer supported |
| 178 | `DST$K_COB_HACK` | An old way to describe COBOL formal arguments; see below |
| 174 | `DST$K_VALSPEC` | Holds only a value spec: the header (`DST$VALSPEC`, `DST$K_VALSPEC_SIZE` = 2), then the spec at `dst$a_vs_valspec_addr`. Ignored |
| 161 | `DSC$K_DTYPE_UBS` | An old debugger-internal type code for unaligned bit strings |

**COBOL Hack layout.**

- The record starts with the Standard Data fields (`DST$COB_HACK`,
  `DST$K_COB_HACK_SIZE` = 8).
- The trailer `DST$CH_TRLR` (`DST$K_CH_TRLR_SIZE` = 1) follows. It holds
  `DST$B_CH_TYPE` (a dtype byte), then a stack-machine routine at
  `DST$A_CH_STKRTN_ADDR` that computes the object's address.

If there is no descriptor, the flags byte and value longword are both 0.
If there is one, they locate it, and the computed
address is written into the descriptor's pointer field. For an array
descriptor, the result is also added to the A0 field. The descriptor's
dtype must match `DST$B_CH_TYPE`.

---

## 23. Implementation notes

### 23.1 Reading a DST

1. Find the DST, and the DMT if there is one, through the image header
   (§2).
2. If the DMT is present, take each module's starting offset and size
   from it. Otherwise read the records one after another, starting a new
   module at each Module Begin.
3. For each record:
   - Read the length and type, and step forward by `length + 1`.
   - If continuation is allowed for this type, join any Continuation
     records that follow.
   - Choose a parser by the type byte, using the ranges in §4.2.
   - Skip types you do not know.
4. Keep a stack of open Begin records to know the current scope, including
   records and enumerations.
5. Gather all line-number records of a module into one command stream.
   Gather source-correlation records the same way, and GEM locator records
   the same way.
6. Treat DST offsets in Indirect specs, GOTO, tag pointers and similar
   fields as offsets from the start of the whole DST. Cross-module
   indirection and Alias are the exceptions: their offsets are from the
   start of the named module's DST.
7. After the last Module End, skip any zero bytes and then apply the Fixup
   and Symbol Fixup records.

### 23.2 Conventions used throughout

- Every multi-byte integer is little-endian.
- Names are *counted ASCII*: a length byte followed by that many
  characters, with no terminator.
- A field documented as "must be zero" should be written as zero and may
  be ignored when reading.
- Constants named `…_SIZE` give the size of a record's **fixed** part, up
  to and including the count byte of the first variable-length field.
- When one byte has both a `…_NAME` alias and a `…_TRLR_OFFS` alias, as in
  PSECT, Exception, and task entries, it is the name length and also the
  distance from the end of the fixed part to the trailer. In other words,
  the trailer begins right after the name.

### 23.3 Where the original definitions disagree with themselves

| Topic | What the source says | Recommended reading |
| - | - | - |
| `DST$K_HIGHEST` | The prose says 191, but the code list ends at 192 (`DST$K_PCLOC`) | 192 |
| `DST$K_SET_STMTNUM` operand | The code list says byte, the semantics say word | Word; check against real compiler output |
| Word-sized header drawings | Several newer records are drawn with a word length and word type | The SDL uses the byte `DST$HEADER` for all of them |
| `dst$a_dis_ranges` macro | Refers to `DST$K_DISRNG_SIZE` (one range entry) | Offset `DST$K_DISRNGS_SIZE` (6) |
| Symbol Fixup-64 | The prose names `DST$K_SYMBOL_FIXUP` | `DST$K_SYMBOL_FIXUP_64` |
| `DST$V_CXXA_UNION` | The comment says "class" | Union |
| Alpha REGNUM range | One place says R16–R30, another R16–R31 | All 16 encodings are available (R16–R31), but don't rely on R31 (it always reads as zero) |
| Fixup field widths | Not specified | Probably longwords; check real images |
