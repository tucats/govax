# Phase 33 — RMS name blocks and XABs at run time

**Status:** in progress (2026-10-01).

## Goal

Programs that use RMS name blocks (NAM) and extended attribute blocks
(XABs) run under govax as they do on VMS 7.3:

- **`$PARSE`** analyzes a file specification against its defaults (FAB$L_DNA)
  and a related file (NAM$L_RLF). It fills the NAM: the expanded string,
  the component pointers and lengths, the FNB status bits, and DID/DVI.
- **`$SEARCH`** walks the files a parsed (possibly wildcard) specification
  matches. It returns each one's resultant string and FID, then RMS$_NMF.
- **`$OPEN` and `$CREATE`** fill a NAM given at FAB$L_NAM. `$OPEN` can
  also open by file ID (FAB$V_NAM).
- **XABs** chained from FAB$L_XAB are filled by `$OPEN`, `$CREATE`, and the
  new **`$DISPLAY`**: XABDAT, XABRDT, XABFHC, XABPRO, XABALL, and XABSUM.
  `$CREATE` takes XABDAT, XABPRO, and XABALL as input, and `$CLOSE` takes
  XABRDT and XABPRO.

Phase 32 gave govax the macros that build these blocks; this phase gives
them meaning at run time.

## Method: the manual, then the oracle

Behavior is written from the *OpenVMS RMS Reference Manual*:

- chapter 5 (NAM), and chapters 9–11, 15, 16, and 18 (the XABs);
- part III's service descriptions, which list each service's input and
  output fields.

It's then checked against **a runtime oracle**:

- govax-written probe programs call the services with NAM blocks and XABs,
  and write each result (R0, FAB$L_STS/STV, and the bytes of the FAB, NAM,
  XABs, and string buffers) to a file as binary records;
- they run first on the author's VMS 7.3 system, then under govax, against
  the same container, so file IDs, directory IDs, and dates are the same;
- the probes are assembled from govax's macros, which Phase 32 showed
  assemble to real MACRO's objects, and govax's LINK makes real LINK's
  images, so buffer addresses in the NAM are the same too.

Only genuinely variable data, such as the wildcard context (an RMS
internal) or the date a file is created during a probe, is masked.
VMS's RMS source isn't consulted (Phase 32's clean-room hook stays in
place).

## Subtasks

Each ends with `go build`/`go vet`/`go test` and golangci-lint clean, and a
commit, plus `build -i` when it changes behavior.

1. **The runtime oracle.** `testdata/mar/rms3/`:
   - a command procedure that builds a test tree on the exchange volume
     (files and versions in `[TEST]` and `[TEST.SUB]`);
   - probe programs (`PARSE`, `SEARCH`, `OPEN`, `XAB`, `CREATE`, `NAMFID`)
     that write their results to `[OUT]`;
   - a procedure that assembles, links, and runs them;
   - the govax script that builds the volume.

   The author runs it on VMS while subtasks 2–6 proceed.
2. **Name processing and `$PARSE`.** Merge the primary specification, the
   default name, and the related file's resultant string into the expanded
   string. Fill the NAM's components, FNB bits, DID/DVI, and WCC. Report
   errors as RMS does: RMS$_DNF, RMS$_DEV, RMS$_ESS, RMS$_SYN, and the
   rest. `$OPEN`, `$CREATE`, and `$RENAME` use the same name processing,
   which gives them default names.
3. **`$SEARCH`.** Iterate a parse's matches with wildcard context kept in
   govax and named by NAM$L_WCC. Fill RSA/RSL, the components into the
   resultant string, and FID/DID. Return RMS$_NMF, RMS$_FNF, and RMS$_NMF
   for a search list.
4. **NAM on `$OPEN`/`$CREATE`.** Fill the expanded and resultant strings
   and FID/DID/DVI. Set FNB's HIGHVER/LOWVER on `$CREATE`. Support
   FAB$V_NAM: `$OPEN` by NAM$W_FID, or by DID and name.
5. **XABs and `$DISPLAY`.** Walk the XAB chain on `$OPEN`, `$CREATE`, and
   `$DISPLAY`. Fill XABDAT, XABRDT, XABFHC, XABPRO, XABALL, and XABSUM from
   the file header. `$CREATE` takes XABDAT, XABPRO, and XABALL as input,
   and `$CLOSE` takes XABRDT and XABPRO. Unknown XAB codes give
   RMS$_COD.
6. **Reconcile with the oracle.** A test runs each probe under govax on a
   copy of the container and compares its records with VMS's. Each
   difference is fixed, or masked as variable data with the reason
   recorded.
7. **Close-out.** Docs, HELP where relevant, DEVIATIONS.md, CLAUDE.md, and
   PLAN.md.

## Out of scope

- Indexed and relative files, and so XABKEY's meaning. govax's RMS handles
  sequential files only. An XABKEY on a sequential file is handled as the
  oracle shows.
- XABTRM, XABITM, and the NAML block (Alpha).
- DECnet node names in file specifications.

## Progress log

- 2026-10-01: Plan written. At the author's direction, subtasks proceed
  without a separate review, with a commit after each.
- 2026-10-01: Subtask 1 done. `testdata/mar/rms3/gen.go` writes the six
  probes, `BUILD.COM`, `RUN.COM`, and `exchange.cmd`. All six assemble and
  link with govax, and the exchange volume is built and waiting for the
  author's VAX run.
- 2026-10-01: Subtask 2, name processing and `$PARSE`.
  - **Name processing** (`internal/rms/name.go`). `scanName` splits a
    specification into its fields, keeping their text, and reports a
    malformed one as RMS does (RMS$_DEV, DIR, FNM, TYP, VER, or SYN).
    `expandName` fills the fields from the primary name (after its logical
    name), the default name, the related file, and the process defaults,
    applying relative directories, and works out the FNB bits.
  - **`$PARSE`** (`parse.go`, `nam.go`) fills the NAM from the manual's
    output list: the expanded string and component pointers, FNB, and,
    unless SYNCHK, DID, DVI, and the FAB's DEV and SDC after checking the
    device and directory. Its search context is kept by NAM address, and
    numbered in WCC (`search.go`).
  - **`$OPEN`, `$CREATE`, and `$RENAME`** now use the same name
    processing (`resolveFAB`), so a FAB's default name and related file
    apply to them too.
  - **Unsettled until the oracle:** the related file's fields without
    OFP, whether EXP_* counts a logical name's fields, the DVI text,
    FAB$L_DEV's bits, what's written on a failure, and WCC's value.
  - **Tests.** `name_test.go` covers scanning, directories, and the
    sources' precedence. `TestRMS3Parse_govaxTree` runs the PARSE probe
    against a govax-built copy of the oracle's tree.
- 2026-10-01: Subtask 3, `$SEARCH` (`internal/rms/search.go`).
  - It continues the context `$PARSE` kept for the NAM, or, when WCC
    doesn't name one, starts a search for the expanded string (RMS$_ESA
    when there is none). A search list's elements are searched in turn.
  - Each file found is reported in the NAM: the resultant string and its
    components (in RSA, RMS$_RSS if it doesn't fit), FID, and the DID of
    its directory. When nothing matched, the status is RMS$_FNF, then
    RMS$_NMF.
  - **Versions.** `$SEARCH` reads a version itself: none or 0 is the
    highest, and -n the version n below it, as VMS has it. ods2's
    `filespec` reads ";-1" as the highest. That's left alone until the
    oracle shows VMS's reading (SEARCH case 10, OPEN case 9).
  - **Tests.** `TestRMS3Search_govaxTree` runs the SEARCH probe against
    the govax-built tree.
- 2026-10-01: Subtask 4, the NAM on `$OPEN` and `$CREATE`
  (`internal/rms/namopen.go`, `namfid.go`).
  - **`$OPEN`** reports the file in its NAM: the expanded and resultant
    strings, the components (into the resultant string), FNB, FID, DID,
    and DVI. A wildcard is RMS$_WLD, a missing directory RMS$_DNF (it was
    RMS$_FNF), and a version of -n opens the one n below the highest.
  - **`$CREATE`** does the same, and sets FNB's HIGHVER and LOWVER. It
    now honors an explicit version (RMS$_FEX when it exists), FAB$V_CIF
    (an existing file is opened, with RMS$_NORMAL), and the default name.
    A specification with no name is RMS$_FNM.
  - **FAB$V_NAM.** `$OPEN` opens by NAM$W_FID on the device NAM$T_DVI
    names, writing no strings; or, with no FID, by the file name in the
    directory NAM$W_DID names, writing the resultant string (its
    directory found from the back links).
  - **Unsettled until the oracle:** `$CREATE`'s success status (govax has
    always returned RMS$_CREATED), FAB$L_STV on success (the manual says
    the channel), what a search list's expanded string shows when a later
    element is opened, and the strings and FNB on a FID open.
  - **Tests.** `TestRMS3OpenCreate_govaxTree` runs the OPEN, NAMFID, and
    CREATE probes on the govax-built tree. Four unit tests now expect the
    VMS statuses above.
- 2026-10-01: Subtask 5, XABs and `$DISPLAY` (`internal/rms/xab.go`,
  `display.go`).
  - **The chain.** `$OPEN`, `$CREATE`, `$DISPLAY`, and `$CLOSE` walk
    FAB$L_XAB's chain first. An unknown type code is RMS$_COD and a block
    shorter than its type's length RMS$_BLN, with the XAB's address in
    STV. XABKEY, XABITM, and XABTRM are accepted and left alone.
  - **Outputs.** XABDAT, XABRDT, XABFHC, XABPRO, XABALL, and XABSUM are
    filled from the file header (a sequential file has no areas, keys, or
    prolog). The FAB gets the file's attributes too: ALQ, DEQ, ORG, RFM,
    RAT, MRS, FSZ, BKS, and GBC.
  - **Inputs.** A new file takes the XABDAT's dates, the XABPRO's
    protection and owner, and the XABALL's extension quantity. A file
    opened for writing takes the XABRDT's revision date and number and
    the XABPRO's protection as it's closed. The XABALL's allocation
    quantity isn't applied: ods2 has no way to preallocate.
  - **`$DISPLAY`** reports an open file again: the FAB, the XABs, and the
    NAM's resultant string, FNB, FID, DID, and DVI.
  - **Tests.** `TestRMS3XAB_govaxTree` runs the XAB probe and checks the
    CREATE probe's read-backs (protection, extension, expiration and
    revision dates, revision number).
- 2026-10-01: Subtask 6's harness, ahead of the VAX run.
  `TestRMS3Oracle` (`internal/console/rms3oracle_test.go`) mounts a copy
  of `testdata/disks/rms3-vax.dsk` under the device name VMS used (from
  VMS's own PARSE dump), sets RUN.COM's default and logical names, empties
  `[CRE]`, runs each probe under govax, and compares its records with
  VMS's byte for byte. It skips without the container; `RMS3_VAX_DISK`
  names another. A dry run against a container govax made and ran itself
  matched exactly, except CREATE's new FIDs and creation dates, which are
  the variable data to mask.
