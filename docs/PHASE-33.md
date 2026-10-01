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
