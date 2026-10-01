# govax's RMS macros: the specification

This is the specification govax's own RMS macros (`internal/bootdata/files/
starlet.mar`) are written from (docs/PHASE-32.md, subtask 2). It comes
from three sources only:

- **[M]** the *OpenVMS Record Management Services Reference Manual*
  (2001, VMS 7.3 era): Appendix A (each macro's format), Appendix B (VAX
  MACRO rules), Chapters 4–19 (each block's fields), Part III (each
  service's calling format). Section numbers like [M A–2] point there.
- **[V]** values and layouts in govax's own tables: `vmsdef.Symbols`, which
  holds VMS's numbers, captured from STARLET.OLB's definition modules and
  VMS's definition files (Phase 31).
- **[O]** real VAX MACRO's output for govax-written programs (subtask 3's
  oracle): objects and object analyses, never expansion listings.

It is not written from VMS's own macro library, and nothing here may be
taken from it. Each detail the manual leaves open is an **oracle
question**, numbered O1, O2, …. The oracle fixtures (subtask 3) are
written to answer them, and the answers are recorded here before the
macros are written.

Scope: VAX VMS 7.3. `$RAB64`, `$RAB64_STORE`, `$NAML`, and `$NAML_STORE`
are Alpha-only [M A–8, A–14] and left out. `$XABRU` has no macro in
Appendix A and is left out.

## 1. General rules [M B.1.2, B.2]

- **Arguments** are `KEYWORD=value`, in any order, separated by a comma,
  a blank, or tabs. A statement continues onto the next line after a
  trailing `-` (govax's macro facility already does all this).
- **Argument kinds** [M A–2]:
  - *values:* a number or expression, stored in the field;
  - *addresses:* a symbolic address, stored with `.ADDRESS`, so that it's
    relocatable;
  - *keywords:* a mnemonic naming an option.
- **A field the macro has no specific default for is 0** [M A–2].
- **Multiple options** for a bit field go in angle brackets
  (`FAC=<GET,PUT>`), and so do file specifications (`FNM=<A.DAT>`) and
  other lists (`UIC=<377,377>`) [M B.2.1].
- **Field sizes** follow the symbol's letter: `B` byte, `W` word, `L`
  longword, `Q` quadword, `T` text [M 2.2]. Offsets and sizes are [V]'s.

### Keyword encodings [V]

These rules hold for every keyword the manual lists, except where §2
says otherwise. Checked against `Symbols`, Phase 32.

- **One choice** (ORG, RFM, RAC, ALN, DTP, MODE): the field gets the
  value of the `C_` symbol for the keyword, such as `ORG=IDX` →
  `FAB$C_IDX`. The field gets that code; it isn't a bit.
- **Options** (FAC, FOP, RAT, SHR, ROP, NOP, AOP, FLG, PROT_OPT): the
  field gets the OR of the `M_` masks, such as `FAC=<GET,PUT>` →
  `FAB$M_GET!FAB$M_PUT`.
- **SHR** is the exception to the naming: `GET`, `PUT`, `DEL`, and `UPD`
  are `FAB$M_SHRGET` and so on, and `MSE`, `NIL`, and `UPI` are
  `FAB$M_MSE`, `FAB$M_NIL`, and `FAB$M_UPI`.
- **An unknown option** is an assembly error. Real MACRO reports
  `Generated ERROR: UNDEFINED BIT VALUE CODE: name` (observed in Phase
  28). The text of an unknown single-choice keyword's error is **O1**.

### The alignment check [M B.2.1; observed, Phase 28]

An initialization macro placed at an address that isn't longword
aligned displays the informational message
`%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT LONGWORD ALIGNED;`. A
misaligned block still assembles (`fabalign.mar`). govax's macro facility
already supports this kind of check: a conditional on `.`, and `.PRINT`
with its own message prefix.

### Symbols the initialization macros define [M Table B–1]

"$FAB … also defines symbolic offsets for a FAB": each initialization
macro defines its block's symbols, as its `$xxxDEF` macro does.

- **O2:** whether that's by calling the `$xxxDEF` macro (whose second
  call defines nothing new), and whether the symbols are local or global
  in the object.

## 2. Initialization macros

Each allocates its block, `xxx$C_BLN` (or `XAB$C_xxxLEN`) bytes [V], at
the current location. It stores the identification fields, any
specified fields, and the specific defaults; every other byte is 0.

- **O3:** the order fields are stored in, and whether a field is stored
  more than once. govax's assembler can already write a field stored
  twice as real MACRO does (`overwrite.go`, Phase 28): `$FAB` with FNM=
  stores FNA twice.
- **O4:** where FNM=/DNM= strings go, relative to the block.

### $FAB [M A–2, Ch. 4]

- **Identification:** `FAB$B_BID` = `FAB$C_BID`, `FAB$B_BLN` =
  `FAB$C_BLN`.
- **Values:**
  - ALQ → `FAB$L_ALQ`, BKS → `FAB$B_BKS`, BLS → `FAB$W_BLS`;
  - CTX → `FAB$L_CTX`, DEQ → `FAB$W_DEQ`, DNS → `FAB$B_DNS`;
  - FNS → `FAB$B_FNS`, FSZ → `FAB$B_FSZ`, GBC → `FAB$W_GBC`;
  - MRN → `FAB$L_MRN`, MRS → `FAB$W_MRS`, RTV → `FAB$B_RTV`.
- **Access modes:** CHAN_MODE and LNM_MODE go in `FAB$B_ACMODES`, at
  `FAB$V_CHAN_MODE` and `FAB$V_LNM_MODE` [M 4.8, 4.22].
- **Addresses:** DNA → `FAB$L_DNA`, FNA → `FAB$L_FNA`, NAM → `FAB$L_NAM`,
  XAB → `FAB$L_XAB`.
- **Strings:** FNM=<spec> sets `FAB$L_FNA` to the string's address and
  `FAB$B_FNS` to its length, and DNM likewise sets DNA and DNS [M A–2].
- **Keywords:**
  - FAC=<BIO BRO DEL GET PUT TRN UPD> → `FAB$B_FAC`;
  - FOP=<CBT CIF CTG DFW DLT MXV NAM NEF NFS OFP POS RCK RWC RWO SCF SPL
    SQO SUP TEF TMD TMP UFO WCK> → `FAB$L_FOP`;
  - ORG={IDX REL SEQ} → `FAB$B_ORG`;
  - RAT=<BLK CR FTN PRN> → `FAB$B_RAT`;
  - RFM={FIX STM STMCR STMLF UDF VAR VFC} → `FAB$B_RFM`;
  - SHR=<DEL GET MSE NIL PUT UPD UPI NQL> → `FAB$B_SHR`. NQL has no VAX
    7.3 mask in [V]: **O5**.
- **Specific defaults:**
  - RFM: "For the $FAB macro, this is the default value" [M 4.28], of
    the format it names. **O6** confirms which, and the stored byte.
  - ORG=SEQ is `FAB$C_SEQ` = 0.
  - FAC and SHR defaults are RMS's at run time [M 4.14, 4.31]. **O7:**
    whether `$FAB` stores anything for them.
  - FSZ's default of 2 is RMS's at run time when the field is 0 [M 4.18].

### $RAB [M A–11, Ch. 7]

- **Identification:** `RAB$B_BID` = `RAB$C_BID`, `RAB$B_BLN` =
  `RAB$C_BLN`.
- **Values:**
  - BKT → `RAB$L_BKT`, CTX → `RAB$L_CTX`, KRF → `RAB$B_KRF`;
  - KSZ → `RAB$B_KSZ`, MBC → `RAB$B_MBC`, MBF → `RAB$B_MBF`;
  - PSZ → `RAB$B_PSZ`, RSZ → `RAB$W_RSZ`, TMO → `RAB$B_TMO`;
  - USZ → `RAB$W_USZ`.
- **Addresses:** FAB → `RAB$L_FAB`, KBF → `RAB$L_KBF`, PBF →
  `RAB$L_PBF`, RBF → `RAB$L_RBF`, RHB → `RAB$L_RHB`, UBF → `RAB$L_UBF`,
  XAB → `RAB$L_XAB`.
- **Keywords:**
  - RAC={KEY RFA SEQ} → `RAB$B_RAC`;
  - ROP=<ASY BIO CCO CDK CVT EOF EQNXT ETO FDL KGE KGT LIM LOA LOC NLK
    NXR NXT PMT PTA RAH REA REV RLK RNE RNF RRL TMO TPT UIF ULK WAT WBH>
    → `RAB$L_ROP`. KGE and EQNXT are synonyms, as are KGT and NXT
    [M A–11].
  - ROP_2=<NQL NODLCKWT NODLCKBLK> has no VAX 7.3 masks in [V]: **O5**.
- **Specific defaults:** RAC=SEQ is `RAB$C_SEQ` = 0. **O8:** any other
  default `$RAB` stores.

### $NAM [M A–5, Ch. 5]

- **Identification:** `NAM$B_BID` = `NAM$C_BID`, `NAM$B_BLN` =
  `NAM$C_BLN`.
- **Values:** ESS → `NAM$B_ESS`, RSS → `NAM$B_RSS`.
- **Addresses:** ESA → `NAM$L_ESA`, RLF → `NAM$L_RLF`, RSA → `NAM$L_RSA`.
- **Keywords:** NOP=<NOCONCEAL PWD NO_SHORT_UPCASE SRCHXABS SYNCHK> →
  `NAM$B_NOP`. NO_SHORT_UPCASE has no VAX 7.3 mask in [V]: **O5**.

### XABs [M A–18 … A–35, Ch. 9–19]

Every XAB starts with `XAB$B_COD` (its type code) and `XAB$B_BLN` (its
length), and has `XAB$L_NXT`, which NXT= sets (an address).

| Macro | COD, BLN | Arguments → fields |
| ----- | -------- | ------------------ |
| `$XABALL` | `XAB$C_ALL`, `XAB$C_ALLLEN` | AID → `B_AID`; ALN={ANY CYL LBN RFI VBN} → `B_ALN`; ALQ → `L_ALQ`; AOP=<CBT CTG HRD ONC> → `B_AOP`; BKZ → `B_BKZ`; DEQ → `W_DEQ`; LOC → `L_LOC`; RFI=<f1,f2,f3> → `W_RFI` (three words); VOL → `W_VOL` |
| `$XABDAT` | `XAB$C_DAT`, `XAB$C_DATLEN` | EDT → `Q_EDT` (a quadword date) |
| `$XABFHC` | `XAB$C_FHC`, `XAB$C_FHCLEN` | (NXT only) |
| `$XABITM` | `XAB$C_ITM`, `XAB$C_ITMLEN` | ITEMLIST → `L_ITEMLIST` (an address); MODE={SENSEMODE SETMODE}, default sensemode [M A–24] |
| `$XABKEY` | `XAB$C_KEY`, `XAB$C_KEYLEN` | COLTBL (an address); DAN, DFL, IAN, IFL, LAN, NUL, PROLOG, REF (values); DTP={BN2 DBN2 BN4 DBN4 BN8 DBN8 IN2 DIN2 IN4 DIN4 IN8 DIN8 COL DCOL PAC DPAC STG DSTG}; FLG=<CHG DAT_NCMPR DUP IDX_NCMPR KEY_NCMPR NUL>; KNM (an address); POS=p or <p0,…,p7> → `W_POS0`…; SIZ=s or <s0,…,s7> → `B_SIZ0`… |
| `$XABPRO` | `XAB$C_PRO`, `XAB$C_PROLEN` | ACLBUF (an address); ACLCTX=<…>; ACLSIZ; MTACC (a value, usually `^A/x/`); PRO=<sys,own,grp,wld> → `W_PRO`; PROT_OPT=<PROPAGATE> → `B_PROT_OPT`; UIC=<group,member> (octal) → `W_GRP`, `W_MBM` |
| `$XABRDT` | `XAB$C_RDT`, `XAB$C_RDTLEN` | (NXT only) |
| `$XABSUM` | `XAB$C_SUM`, `XAB$C_SUMLEN` | (NXT only) |
| `$XABTRM` | `XAB$C_TRM`, `XAB$C_TRMLEN` | ITMLST → `L_ITMLST` (an address); ITMLST_LEN → `W_ITMLST_LEN` |

- **XABKEY's, XABSUM's, and XABITM's** codes, lengths, and fields aren't
  in [V] yet (PHASE-32 subtask 1). **O9** captures them: a program that
  stores each field name's value, assembled after `$XABKEYDEF`,
  `$XABSUMDEF`, and `$XABITMDEF`.
- **XABKEY's defaults:** with no FLG=, an alternate key (REF ≠ 0) gets
  duplicates and changes allowed; with FLG= given, exactly the named bits
  [M A–25]. **O10:** the stored bytes, and the primary key's default.
  POS and SIZ take one value, or up to eight in brackets.
- **XABPRO's PRO=:** each class is a 4-bit field (`XAB$V_SYS`, `_OWN`,
  `_GRP`, `_WLD`), whose bits *deny* access: `XAB$M_NOREAD`, `NOWRITE`,
  `NOEXE`, `NODEL`. A letter R, W, E, or D grants that access, clearing
  its bit. With no PRO=, the field is 0, which grants everything
  [M 15.13]. **O11:** what an omitted class within the brackets stores,
  and UIC='s packing.

## 3. Store macros [M B.2.3, A–4 …]

`$xxx_STORE` takes the same keywords as its initialization macro, apart
from the differences Appendix A lists, and generates executable code:

- **The block's address** is required, as FAB=, RAB=, NAM=, or XAB=. If
  it isn't given as a register `Rn`, the macro loads it into R0. With no
  address argument at all, R0 already holds it. R0 isn't preserved.
- **Address arguments:** `MOVAx` of the address into the field (usually
  MOVAL). A register `Rn` (R0–R12) means the register holds the address.
- **Value arguments:** `MOVx` into the field. A literal needs `#`, except
  inside angle brackets.
- **Keyword arguments:** the keyword, without `#`.
- **Multi-word fields:** `RAB$W_RFA`, `NAM$W_DID`, and `NAM$W_FID` (3
  words), `NAM$T_DVI` (16 bytes), and the XABDAT/XABRDT dates
  (quadwords) are given by symbolic address, or by a register pair (not
  R12).
- **Store-only arguments:** `$NAM_STORE` has DID, DVI, and FID;
  `$RAB_STORE` has RFA; `$XABDAT_STORE` has CDT, RDT, and RVN;
  `$XABRDT_STORE` has RDT and RVN; `$XABKEY_STORE` takes POS0…POS7 and
  SIZ0…SIZ7 separately too. `$FAB_STORE` has no FNM or DNM.
- **O12:** the instructions for each kind: MOVB/MOVW/MOVL/MOVQ by field
  size, and MOVx or BISx for an option field (does FAC=<GET> replace the
  field, or add to it?). Also the multi-word moves, and the register-pair
  forms.

## 4. Definition macros [M B.2.2, Table B–1]

`$FABDEF`, `$RABDEF`, `$NAMDEF`, `$XABDEF`, and `$XABxxxDEF` (one for each
XAB) take no arguments, can go in any psect, and define their block's
symbols. `$RMSDEF` defines the RMS$_ completion codes. The values are
[V]'s, so these macros are generated from `Symbols`, not written by hand.

- **Observed (Phase 28):** a second call defines nothing new, and the
  definitions are made in `$ABS$`, which real MACRO's objects show.
- **O13:** which names each macro defines. Does `$XABDEF` define every
  XAB's symbols, and `$XABALLDEF` only the common ones plus XABALL's? Are
  `xxx$V_`, `xxx$S_`, and `xxx$K_`/`xxx$C_` all included? Is a global
  form available?
- **The other families** (decided in Phase 32): `$SSDEF`, `$IODEF`,
  `$JPIDEF`, `$DVIDEF`, `$SYIDEF`, `$LNMDEF`, `$DEVDEF`, `$TTDEF`,
  `$PRTDEF`, `$PRVDEF`, `$BRKDEF`, `$FIBDEF`, `$ATRDEF`, and `$STATEDEF`,
  from the same generator. **O13** covers them too.

## 5. Service macros [M B.2.4, Part III]

| Block | Macros | Argument list |
| ----- | ------ | ------------- |
| FAB | `$CLOSE`, `$CREATE`, `$DISPLAY`, `$ENTER`, `$ERASE`, `$EXTEND`, `$OPEN`, `$PARSE`, `$REMOVE`, `$SEARCH` | `fab [,[err] [,suc]]` |
| RAB | `$CONNECT`, `$DELETE`, `$DISCONNECT`, `$FIND`, `$FLUSH`, `$FREE`, `$GET`, `$NXTVOL`, `$PUT`, `$READ`, `$RELEASE`, `$REWIND`, `$SPACE`, `$TRUNCATE`, `$UPDATE`, `$WRITE` | `rab [,[err] [,suc]]` |
| FAB | `$RENAME` | `OLDFAB=, ERR=, SUC=, NEWFAB=`: `old-fab, err, suc, new-fab` |
| RAB | `$WAIT` | `RAB=` only (no ERR or SUC) |

- **No arguments:** `CALLG (AP), G^SYS$xxx`. The caller has built the
  argument list at AP [M B.2.4].
- **Keyword arguments:** the argument list is built on the stack and
  called with `CALLS`. `$OPEN FAB=INFAB` is `PUSHAL INFAB` then
  `CALLS #01, G^SYS$OPEN` [M B.2.4].
  - FAB= or RAB= is either a register R0–R11 holding the address, or an
    operand for `PUSHAL`.
  - ERR= and SUC= are `PUSHAL` operands, pushed before the block, so the
    list is block, err, suc.
- **O14:** the argument count and pushes when ERR or SUC is omitted (is
  an omitted ERR pushed as 0 when SUC is given?), and the register form
  (`PUSHL Rn`?). The oracle already has `rmscopy.mar`'s simple forms.

## Oracle questions

| # | Question | Fixture (subtask 3) |
| - | -------- | ------------------- |
| O1 | The error text for an unknown single-choice keyword | an assembly expected to fail; its message only |
| O2 | How an initialization macro defines its block's symbols (via `$xxxDEF`? local or global?) | `$FAB` alone, and `$FAB` after `$FABDEF`; the GSD |
| O3 | The order fields are stored in, and repeated stores | each init macro with defaults, and with every keyword |
| O4 | Where FNM=/DNM= strings go | `$FAB FNM=…, DNM=…` |
| O5 | SHR=NQL, ROP_2=…, and NOP=NO_SHORT_UPCASE on VAX 7.3 | each alone, expected to fail or not |
| O6 | `$FAB`'s RFM default | `$FAB` with no RFM |
| O7 | Whether `$FAB` stores FAC or SHR defaults | `$FAB` with neither |
| O8 | Any other `$RAB` default | `$RAB` with no arguments |
| O9 | XABKEY's, XABSUM's, and XABITM's codes, lengths, and offsets | `.LONG` of each field name after their `$xxxDEF` |
| O10 | `$XABKEY`'s FLG defaults, for primary and alternate keys | `$XABKEY REF=0`, `REF=1`, `REF=1, FLG=CHG` |
| O11 | `$XABPRO`'s omitted protection class, and UIC= | PRO=<RW,,R>, PRO=<,,,>, UIC=<377,377> |
| O12 | Each store macro's instructions | every `_STORE` with each argument kind |
| O13 | Which names each `$xxxDEF` defines, and its global form | `$xxxDEF` then `.LONG` of the names, and object GSDs |
| O14 | Service macros' argument lists with ERR/SUC omitted, and register forms | every service, every form |
