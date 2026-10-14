# Phase 58 — LIB$ callbacks into the program, and file routines

**Status:** in progress (subtasks 1 to 6 done 2026-10-13, 7 and 8 on
2026-10-14). Planned
2026-10-12. Needs Phases 53 and 54. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

Two things that come together here because the file routines are the
first to need the first:

- **Calling back into the program.** Several LIB$ routines call
  routines the program supplies: FILE_SCAN's success and error routines,
  DELETE_FILE's and RENAME_FILE's success, error, and confirm routines,
  and later the tree routines' compare and action routines (Phase 60),
  TPARSE's action routines (Phase 60), and a user zone's allocation
  routines (Phase 59). A LIB$ routine written in Go must be able to call
  a VAX routine, get its R0, and go on, perhaps many times in one call.
  The pattern exists for services: a service returns a
  `corevms.CallRequest`, the engine calls the routine with the service's
  XFC as the return address, and the service runs again to continue
  (`$CMKRNL`, `cmode.go`; exit handlers, `exit.go`; the condition
  dispatcher). This phase generalizes it for shims. A shim keeps its
  state across the calls it makes in a per-call record, found again by
  frame and stack pointer, as `cmodeCall` is. The engine allows a shim
  to return a CallRequest. Nesting works too: a callback that calls the
  same LIB$ routine. The routine's Go code should read as a loop, not
  a hand-written state machine, so the design should look at whether a
  goroutine handoff (as `dclshell/subcli.go` does for the subprocess CLI)
  or an explicit continuation is simpler to keep correct.
- **Files.** FIND_FILE and FIND_FILE_END: wildcard search with a
  context, over RMS's `$PARSE`/`$SEARCH` (Phase 33). FILE_SCAN and
  FILE_SCAN_END: the same through a FAB and NAM the program supplies,
  calling its routines. DELETE_FILE and RENAME_FILE, with wildcards,
  related and default names, and the callbacks. FID_TO_NAME: a file ID to
  a file name, through the directory back-links, which ods2 can read.
  TRIM_FILESPEC: fitting a file name to a width.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Search | FIND_FILE (155), FIND_FILE_END (159), FILE_SCAN (151), FILE_SCAN_END (153) |
| Delete and rename | DELETE_FILE (106), RENAME_FILE (319) |
| Names | FID_TO_NAME (149), TRIM_FILESPEC (461) |

## What earlier phases leave in place

- `CallRequest` and its users (above); `corevms.Environment.Shim`
  (`environment.go`), which today passes a shim's error through without
  looking for a CallRequest.
- RMS: `$PARSE`, `$SEARCH` (with VMS's two kinds of wildcard context),
  `$ERASE`, `$RENAME`, NAM blocks, related names (`internal/rms`,
  Phases 24, 33, 47). DCL's DELETE and RENAME (`internal/dclcmd`) for
  how govax already words their messages.
- `LIB$CREATE_DIR` (Phase 34), the existing LIB$ file routine, for its
  conventions.

## Open questions

- ~~**The callback mechanism's shape**~~ (decided 2026-10-13, subtask
  2): the goroutine handoff (the survey's "The callbacks";
  `corevms/callback.go`).
- ~~**A condition in a callback**~~ (settled 2026-10-13, subtask 2): a
  `$UNWIND` past the stub's frame abandons the routine
  (`finishUnwind`), as image rundown does; what VMS's own routines do
  then (anything left open) is still a probe case.
- ~~**FIND_FILE's flags** (multiple-file context, NODEFAULTS) and the
  context's lifetime; and FID_TO_NAME on a file with no back-link~~
  (settled 2026-10-14, subtask 8): the flags are NOWILD and MULTIPLE;
  RMS$_NMF ends a context (but with MULTIPLE); a file its directory has
  no entry for is `[]NAME`.

## The survey (subtask 1)

Each routine's interface, from the 5.0 LIB$ manual, in the notation of
Phase 53's survey (`name.access-type.mechanism`; `[ ]` optional). Page
numbers are the manual's LIB-nnn where the entry starts. Section 2.7.4,
"File Searching Routines" (pages 2-24 to 2-30), adds what FIND_FILE and
FILE_SCAN share, with a BLISS example of each. Choices the manuals
leave open are marked **unconfirmed**, and each is a case for the probe
(subtask 8).

All eight routines are in VMS 7.3's LIBRTL.EXE transfer vector
(`vmsdef.ImageSymbols`: DELETE_FILE 0x398, FILE_SCAN 0x3E8, FIND_FILE
0x3F0, RENAME_FILE 0x488, TRIM_FILESPEC 0x938, FIND_FILE_END 0xA00,
FILE_SCAN_END 0xA08, FID_TO_NAME 0xA80). Today each is a
missing-routine stub.

### Sources

- **The 5.0 LIB$ manual** (`AA-LA76A-TE`, its OCR text) is the
  reference. The OCR's errors that matter here: "LIB$FID_TQ_NAME" for
  FID_TO_NAME, "LIB$DELETEJILE" and "LIB$FINDJILE" for the `_F` names,
  "tab" for `fab` in FILE_SCAN's format, `1` for `I` in condition
  names (`LIB$_1NVFILSPE`), and "LIB$FILE_SCAN-END". FIND_FILE's flag
  table is garbled: the 5.0 text runs the two symbols together ("0
  NOWILD MULTIPLE"); the VSI manual sets it out (bit 0
  LIB$M_FIL_NOWILD, bit 1 LIB$M_FIL_MULTIPLE), and so does the 5.0
  section 2.7.4 ("If bit 1 is set, LIB$FIND_FILE performs temporary
  defaulting for multiple input files"). An error-source table's value
  1 is lost in the OCR ("0 Error searching ..." then "Error deleting
  file"); the VSI manual has 0 and 1.
- **The VSI OpenVMS RTL Library (LIB$) Manual** (local-only). It agrees
  with the 5.0 text but where marked **VSI** below; each difference is
  a probe case, since it may postdate VAX VMS 7.3. Most of its additions
  are for long names (NAML blocks, `LIB$M_FIL_LONG_NAMES`, 4095
  characters) and symbolic links, which the VAX doesn't have.
- **The RMS Reference Manual** and the File Applications guide for
  `$PARSE`, `$SEARCH`, related names, and input file parsing, which
  govax's RMS already follows (Phases 24, 33).
- **The DCL Dictionary**: DELETE and RENAME, which the routines are
  "similar in function to".

### Codes

Values from `vmsdef.LibrarySymbols` (STARLET.OLB's definitions):

| Name | Value | Use |
| --- | --- | --- |
| LIB$_NORMAL | %X158001 | FID_TO_NAME's success (the manual's; SS$_NORMAL elsewhere) |
| LIB$_STRTRU | %X158011 | output string truncated (success) |
| LIB$_ERRROUCAL | %X158029 | success, but the error routine was called |
| LIB$_INVSTRDES | %X158224 | invalid string descriptor |
| LIB$_INVARG | %X158234 | invalid argument |
| LIB$_WRONUMARG | %X15835C | wrong number of arguments |
| LIB$_INVFILSPE | %X15839C | invalid file specification |
| LIB$_NOWILD | %X15112A | SHR$_NOWILD (%X1128) with the LIB facility, error severity |
| RMS$_NMF | %X182CA | no more files |
| RMS$_FNF | %X18292 | file not found |
| RMS$_FEX | %X18282 | file already exists (RENAME's current version) |
| RMS$_FAB | %X1850C | not a valid FAB |
| RMS$_CHN | %X1C0EC | no more channels |

- **The flag masks** are not in govax's definitions: no LIB$M_FIL_ or
  LIB$V_FIL_ symbol is in `vmsdef`, and neither manual names the macro
  that defines them. The values are the bits the manuals give
  (NOWILD 1, MULTIPLE 2; RENAME_FILE's CUR_VER 1, INH_SECUR 2;
  LONG_NAMES 4 for both, **VSI**); the probe program defines them
  itself, as Phase 57's did `$LIBCLIDEF`'s.
- **Shared messages.** `vmsdef.Messages` has the LIB$ facility's own
  texts but none of the SHR$ ones, so LIB$_NOWILD has no text in govax
  yet, nor SHR$_PARSEFAIL (the manual's examples signal it with the CLI
  facility). The probe gets their texts with `$GETMSG`.

### General rules

- **Strings** are read and written by `descriptor.go`'s rules (Phase
  53); every entry says "any string class is supported". An input of
  more than 255 characters is LIB$_INVFILSPE for the file routines
  (DELETE_FILE, RENAME_FILE), and LIB$_INVARG for TRIM_FILESPEC; the
  others don't say (FIND_FILE's "maximum length of a file specification
  is 255 bytes": **unconfirmed** what 256 gives, a probe case).
- **Arguments.** DELETE_FILE, RENAME_FILE, and TRIM_FILESPEC list
  LIB$_WRONUMARG; FID_TO_NAME gives LIB$_INVARG for a "required argument
  omitted". Which counts give WRONUMARG (fewer than the required ones;
  more than the format has) is a probe case. An omitted optional
  argument is 0 (`arg`).
- **Contexts.** FIND_FILE's context is "the address of an internal
  FAB/NAM buffer", allocated with LIB$GET_VM (its condition list names
  GET_VM and FREE_VM), and FIND_FILE_END answers RMS$_FAB for one that
  isn't. So govax's context is a heap block that holds a real FAB and
  NAM (FAB$B_BID, NAM$B_BID set), with the expanded and resultant
  buffers after them, on which the routine calls `$PARSE` and `$SEARCH`
  through `corevms.Environment.CallService`, as the jackets call their
  services (Phase 55). What's kept in Go besides (the last file
  specification, the flags) is per image, found by the block's address,
  as Phase 56's date and time context is. FILE_SCAN's context is "the
  address of a longword" in which it keeps the addresses of "name
  blocks and file specification strings" it allocated: a heap block
  too, holding the related NAM chain (below).
- **Search contexts in RMS.** A wildcard directory or a search list
  makes `$PARSE` keep a search context (`rms/search.go`, NAM$L_WCC); a
  search that runs out frees it, and so does a `$PARSE` of the null
  string, which is what FIND_FILE_END and FILE_SCAN_END do ("parses the
  null string to deallocate any saved RMS context"). RMS$_CHN can't
  happen in govax ($SEARCH opens no channel that outlives the call); the
  manual's advice about calling FIND_FILE_END stands anyway.
- **Success** is SS$_NORMAL (FIND_FILE: RMS$_NORMAL in the VSI manual's
  list, SS$_NORMAL in the 5.0's; **unconfirmed**, a probe case) but for
  FID_TO_NAME's LIB$_NORMAL and the end routines' RMS$_NORMAL (5.0;
  SS$_NORMAL, **VSI**).

### The callbacks

Three routines call the program: FILE_SCAN its success and error
routines, DELETE_FILE and RENAME_FILE their success, error, and confirm
routines. Each is an ordinary CALL of a procedure entry mask (passed
"by value": the argument is the entry mask's address), and the routine
looks at what it returns in R0 (bit 0) for the error and confirm
routines. Later phases' callers (the tree routines, TPARSE, a user zone)
have the same needs: a call, perhaps many in one LIB$ call, whose R0 the
routine reads before going on.

**What the engine does today.** A shim (a Go routine behind a stub,
`entry mask; MOVL #code,R0; XFC #XFC$SHIM; RET`) can return a
`corevms.CallRequest`, and the engine builds the call's frame as CALLG
would (`cpu.Engine.callForService`). Only two forms are of use to a
shim:

- **Tail** (LIB$SHOW_TIMER's action routine, `librtl/timers.go`): the
  procedure returns past the XFC, to the stub's RET, so its R0 is the
  routine's status. One call, the last thing the routine does.
- **Image** (the subprocess CLI's RUN): the call is an image's start.

The plain form, which returns to the XFC to run the shim again, works
for services (`$CMKRNL`, `$EXIT`'s handlers, the condition dispatcher)
because a service is found by the XFC's address. A shim is found by R0,
and when the procedure returns R0 is the procedure's own result: the XFC
would run whatever shim that number names. That is the gap.

**What's needed.**

1. The XFC, run again after a callback, must find the shim call it
   belongs to, not the one R0 names, and hand that call the callback's
   R0. A call is found as `$CMKRNL`'s are (`cmodeCall`): by the stub's
   frame pointer and the stack pointer when the callback was called,
   both the same again after the callback's RET, and the XFC's address.
   No other code can execute that XFC with that FP: a nested call of
   the same routine (a callback calling LIB$FILE_SCAN again) runs in a
   new stub frame.
2. The callback's argument list and the strings it is given (a class S
   descriptor and its text) must survive the call: they go on the stack
   below SP, in the stub's frame, with SP lowered past them before the
   frame is built (`callForService` builds it at SP). The stub's RET
   takes them away with the frame when the routine returns, as for
   SHOW_TIMER's tail call.
3. The routine's Go code should read as a loop: search, call confirm,
   delete, call success, until RMS$_NMF.
4. If the callback never returns to the routine (a handler's `$UNWIND`
   past the stub's frame, `$EXIT`, LIB$STOP's unwind, the image ending),
   the routine's state must go, at once or the next time the routine is
   entered. The `$UNWIND` code knows the frames it removes
   (`unwindRequest.frames`), and image rundown (`ImageRundown`) can
   cancel whatever is left.

**Two shapes.**

- **An explicit continuation.** The routine keeps a state record (where
  it is in its loop, its RMS state) and is entered again with the
  callback's R0, switching on the record's state. Nothing but the
  record to clean up, but every routine is a hand-written state machine:
  DELETE_FILE has four places it calls out (confirm, success, error
  after a search, error after a delete), RENAME_FILE five, TPARSE's
  action routines are inside its state-table walk, and a tree
  traversal's action routine is inside a recursion.
- **A goroutine handoff**, as `dclshell/subcli.go` runs the subprocess
  CLI: the routine runs on a goroutine of its own, and a callback is a
  Go call (`r0, err := cb.Call(routine, args...)`) that hands the
  engine the request and waits. The engine runs the procedure; the XFC,
  run again, hands the routine the R0 and waits for the routine's next
  request or its result. Only one of the two goroutines runs at a time
  (a strict handoff over channels), so the engine stays deterministic
  and nothing needs a lock. A routine whose call is abandoned (point 4)
  has its goroutine told so: its pending `Call` panics with a sentinel
  the goroutine's wrapper recovers, so the routine's own code needn't
  check, and its deferred cleanups run.

**Proposed: the handoff.** It keeps each routine's code as the manual
describes it, a loop, and serves the later phases' recursive callers
(TRAVERSE_TREE) without making their recursion explicit. The cost is
the goroutine's lifetime, which point 4 bounds. Only routines registered
as calling back run on a goroutine; every other shim stays a plain Go
call. A routine that would wait (ErrWait) while it has a callback in
progress is out of scope for now (none of this phase's does; `$SEARCH`
and `$ERASE` don't wait), and is refused loudly rather than restarted.
Subtask 2 builds it, with a test routine that calls back, a callback
that calls the same routine again, and a callback that unwinds past it.

**The arguments the routines pass.** The user-specified-argument is
passed "using the same passing mechanism that was used to pass it to"
the routine: the longword in the routine's argument list is copied into
the callback's as it is (a value, or an address, unread). Strings are a
class S, type T descriptor in the stub's frame, or, when the program
gave resultant-name (or old- and new-resultant-name), that descriptor,
with the name written into it first ("used to pass the string ...
instead of a default class S, type T string"). The rms-sts, rms-stv,
and error-source arguments are by reference: longwords in the frame.

### FIND_FILE (155) and FIND_FILE_END (159)

| Routine (page) | Format |
| --- | --- |
| FIND_FILE (155) | `filespec.rt.dx, resultant-filespec.mt.dx, context.ml.r [,default-filespec.rt.dx] [,related-filespec.rt.dx] [,status-value.wlc.r] [,flags.rlu.r]` |
| FIND_FILE_END (159) | `context.rl.r` |

- **One file per call.** The first call (context 0) allocates the
  context, writes its address to `context`, and parses filespec with
  default-filespec and related-filespec (input file parsing: the
  related name's fields fill what filespec and the default leave out,
  but not its file name and type when filespec has a wildcard,
  **unconfirmed**); each call then returns the next match in
  resultant-filespec, until RMS$_NMF. A call with the same context and a
  different filespec starts a new search (the BLISS example's outer
  loop does that without calling FIND_FILE_END); whether "different" is
  a comparison of the text, and what a call after RMS$_NMF with the
  same filespec returns (the search again, or NMF again), are probe
  cases.
- **No wildcard, no NOWILD**: the file on the first call, RMS$_NMF on
  the next. **NOWILD set**: a wildcard in the specification is
  LIB$_NOWILD; with the same non-wildcard filespec, the file on every
  call. A missing file is RMS$_FNF (on the first call; then?).
- **MULTIPLE set**: the related name is ignored, and "each time
  LIB$FIND_FILE is called with a different file specification, the
  specification from the previous call is automatically used as a
  related file specification" (temporary defaulting: "[SMITH]A,B" makes
  B [SMITH]B). The VSI manual adds that the filespec is saved only with
  MULTIPLE. Which part of the previous call is the related name, its
  resultant string or its expanded one, is a probe case.
- **status-value** gets FAB$L_STV "from a failing RMS operation" (and
  after a success? **unconfirmed**: the probe sets it to a marker
  first).
- **resultant-filespec** is "modify" access: written by its class. A
  short fixed string: LIB$_STRTRU, or LIB$SCOPY_R_DX's status? A
  probe case.
- **FIND_FILE_END** frees the context (after a `$PARSE` of the null
  string releases RMS's search context) and, per the VSI manual, the
  context value is then no longer valid; whether it writes 0 to the
  longword is a probe case. A context of 0 is SS$_NORMAL (it says "zero
  or the address"); one that isn't a FAB is RMS$_FAB.
- **LONG_NAMES** (bit 2) is **VSI** only (NAML blocks, not on the
  VAX): what VMS 7.x does with an undefined flag bit, ignore it or
  LIB$_INVARG, is a probe case.

### FILE_SCAN (151) and FILE_SCAN_END (153)

| Routine (page) | Format |
| --- | --- |
| FILE_SCAN (151) | `fab.r.r, user-success-procedure.fzem.v, user-error-procedure.fzem.v [,context.ml.r]` |
| FILE_SCAN_END (153) | `[fab.m.r] [,context.ml.r]` |

- **The program's FAB and NAM.** The FAB "referencing a valid NAM
  block" holds the specification (FNA/FNS, DNA/DNS), and its NAM the
  buffers (ESA/ESS, RSA/RSS) and, optionally, a related NAM (RLF), as in
  the manual's second BLISS example. FILE_SCAN does `$PARSE`, then
  `$SEARCH` until RMS$_NMF, on the program's own FAB, calling the
  success routine for each file found and the error routine for each
  failure (a `$PARSE` failure, RMS$_FNF, an access error on one file
  while the search goes on); each is called with one argument, the
  FAB's address (FAB$L_CTX carries the program's own value; FAB$L_STS
  and STV say what failed). So the routines read the name from the NAM
  (NAM$B_RSL and NAM$L_RSA, as the example's do).
- Not said, all probe cases: whether the callbacks' R0 matters (an
  error routine returning failure: does the search stop?); FILE_SCAN's
  own status after the last file (RMS$_NMF, SS$_NORMAL, or the last
  error's); whether a success or error routine may be 0 (the format
  has no brackets, the VSI manual neither); which errors call the error
  routine (RMS$_NMF must not; RMS$_FNF does, in the example); and what
  is in the NAM's resultant string when the error routine runs.
- **Defaulting between calls.** "If this routine is called once for
  each file specification argument in a command line, portions of the
  file specifications which are not specified by the user are inherited
  from the last files processed." With context, FILE_SCAN keeps its own
  related NAMs (the "temporary default context ... for multiple input
  files", as DCL's `COPY [SMITH]A,B,C *`); without it, "unspecified
  portions ... will be inherited from the previous file specification
  processed": presumably through the program's own NAM's RLF, which
  FILE_SCAN points at a NAM of its own holding the last result
  (**unconfirmed**; the probe calls FILE_SCAN twice, "[dir]A.TXT" then
  "B.TXT", with and without a context, and shows NAM$L_RLF after).
- **FILE_SCAN_END**: with fab, a `$PARSE` of the null string on that
  FAB, freeing RMS's search context (the probe shows the FAB's FNS
  and the NAM after: does it restore them?); with context, frees the
  related-name storage, "replacing the context value passed to it with a
  temporary context value that your program passes to LIB$FILE_SCAN the
  next time" (0?); with neither, nothing (SS$_NORMAL). A fab that isn't
  one is RMS$_FAB.

### DELETE_FILE (106)

`filespec.rt.dx [,default-filespec.rt.dx] [,related-filespec.rt.dx]
[,user-success-procedure.flc.v] [,user-error-procedure.flc.v]
[,user-confirm-procedure.flc.v] [,user-specified-argument.rz.v]
[,resultant-name.wt.dx] [,file-scan-context.ml.r]` (the VSI manual adds
`[,flags.rlu.r]` tenth: bits 0 and 1 "reserved", bit 2 LONG_NAMES, and
LIB$_INVARG for undefined bits, **VSI**).

- **Each file the specification matches is deleted** (wildcards
  allowed), found through FILE_SCAN with file-scan-context, which "need
  only be specified ... with multiple input files, as the DCL command
  DELETE does". Related-filespec uses input file parsing.
- **For each file**: the confirm routine, if given, is called
  (`filespec, fab [,user-specified-argument]`); a failure status (bit 0
  clear) skips the file. The FAB "describes the file being deleted":
  the routine's own FAB, which the confirm routine may `$OPEN` (and
  must `$CLOSE`) to look at the file's attributes. Then the file is
  deleted (`$ERASE`), and the success routine is called
  (`filespec [,user-specified-argument]`).
- **Errors**: the error routine (`filespec, rms-sts, rms-stv,
  error-source [,user-specified-argument]`), error-source 0 for an
  error searching and 1 for an error deleting. Its success status goes
  on to the next file, its failure "ceases immediately and
  LIB$DELETE_FILE will return with the error status" (the RMS error,
  or what the error routine returned? a probe case). With an error
  routine and any error, the status is LIB$_ERRROUCAL; without one,
  "the most severe error status encountered" (severity order: the
  probe deletes a missing file and a protected one in one call). The
  error routine "is not called for errors related to string copying".
- **Versions**: the manual doesn't say whether a name without a version
  deletes the highest version only (`$ERASE`'s rule) or is an error as
  DCL's DELETE makes it; the Pascal example passes `*;` as the default,
  which suggests the former, with no default version. A probe case.
- **Condition values**: SS$_NORMAL, LIB$_ERRROUCAL, LIB$_INVFILSPE
  (filespec or default-filespec over 255), LIB$_INVSTRDES,
  LIB$_WRONUMARG, LIB$SCOPY's (not truncation), RMS's.
- **In govax** the search is `$PARSE`/`$SEARCH` on a FAB and NAM in the
  stub's frame or a heap block, and the delete `$ERASE` of the
  resultant name, through `CallService`, so a confirm routine's `$OPEN`
  of the same FAB works. `rms.Session.Delete` (DCL's DELETE) deletes a
  whole list at once, with no room for a confirm between files, so it
  isn't the routine's way in; whether DCL's DELETE later moves onto the
  routine (as on VMS) is outside this phase.

### RENAME_FILE (319)

`old-filespec.rt.dx, new-filespec.rt.dx [,default-filespec.rt.dx]
[,related-filespec.rt.dx] [,flags.rlu.r]
[,user-success-procedure.flc.v] [,user-error-procedure.flc.v]
[,user-confirm-procedure.flc.v] [,user-specified-argument.rz.v]
[,old-resultant-name.wt.dx] [,new-resultant-name.wt.dx]
[,file-scan-context.ml.r]`

- **Each file old-filespec matches is renamed** to new-filespec, whose
  missing or `*` fields come from the old file's name "using the same
  rules as for the DCL command RENAME": what `rms.Session.Rename`
  already does for DCL's RENAME (`rms/renamecmd.go`: the output parsed
  with the old name as its related file, `*` fields the old file's, the
  version rules, and its stages, `RenameStage`, which are this
  routine's error sources). The routine needs it a file at a time, with
  the callbacks between; so the per-file work (`renameTarget`,
  `renameOne`) is shared, and the search is the routine's own.
- **flags** bit 0 (LIB$M_FIL_CUR_VER, **VSI**'s name): without a new
  version, keep the file's version (RMS$_FEX if that name exists), as
  /NONEW_VERSION; bit 1 (LIB$M_FIL_INH_SECUR, **VSI**: /INHERIT_SECURITY)
  and bit 2 (LONG_NAMES, **VSI**) postdate the 5.0 manual, where any
  undefined bit is LIB$_INVARG; which bits VMS 7.x refuses is a probe
  case. govax has no security inheritance to do, so bit 1, if VMS 7.x
  takes it, is accepted and changes nothing (**unconfirmed**).
- **Callbacks**: success `old-filespec, new-filespec
  [,user-specified-argument]`; error `old-filespec, new-filespec,
  rms-sts, rms-stv, error-source, user-specified-argument` (0 searching
  for old-filespec, 1 parsing new-filespec, 2 renaming; note the
  user-specified-argument has no brackets here, but "if omitted, zero is
  passed by value"); confirm `old-filespec, new-filespec, old-fab
  [,user-specified-argument]`. The same R0 rules as DELETE_FILE's, the
  same statuses, and LIB$_INVARG for undefined flags. "If the
  user-error-procedure argument was not specified, the default is to
  continue processing."
- What new-filespec the error routine gets at error source 0 (before a
  new name is known), and whether old-filespec there is the expanded
  name, are probe cases; so are the strings when old- or
  new-resultant-name is a dynamic descriptor.

### FID_TO_NAME (149)

`device-name.rt.dx, file-id.rwu.ra, filespec.wt.dx
[,filespec-length.ww.r] [,directory-id.rwu.ra] [,acp-status.wlu.r]`

- A disk device name (at most 64 characters, from NAM$T_DVI, so
  possibly with a leading `_`) and a file ID (three words, NAM$W_FID) to
  a file specification, "by requesting the ACP file specification
  attribute" (ATR$C_FILE_SPEC, %X2E), which follows the file header's
  back-link, directory by directory, to the MFD. directory-id is for
  structure level 1 disks (ignored on VSI's systems, and govax's volumes
  are all ODS-2).
- Outputs are stored "only if the routine successfully finishes".
  acp-status is "the status resulting from traversing the backward
  links": the probe shows it for a good file, a file whose back-link
  names no directory (made by an `$CREATE` with no directory entry, or
  a directory deleted after), and a file ID that names no file
  (SS$_NOSUCHFILE?), and the filespec it returns for each.
- Statuses: LIB$_NORMAL, LIB$_STRTRU (filespec-length then the
  truncated length), LIB$_INVARG (a required argument omitted, a
  device name over 64), LIB$_INVFILSPE (not a disk), and `$ASSIGN`'s,
  `$QIO`'s, `$DASSGN`'s (a device that doesn't exist: SS$_NOSUCHDEV;
  not mounted: SS$_DEVNOTMOUNT?).
- **The file specification's form** (the device as given or its full
  name, `_DUA1:` or `DUA1:` or the volume label's logical name, the
  directory `[A.B]`, the version) is what the probe has to show.
- **In govax** the disk ACP has no ATR$C_FILE_SPEC
  (`corevms/diskattr.go` reads ATR$C_BACKLINK and the rest but not the
  name), and `$QIO` with an attribute that isn't there is
  SS$_BADATTRIB. The routine walks the back-links itself (ods2 has
  `Volume.OpenFID` and each header's `Backlink`, as `rename.go`'s
  `checkNotAncestor` walks them), in `internal/rms`, the one package
  that may import ods2. Adding the attribute to the ACP as well, for a
  program that asks for it, waits for the probe to show its layout
  (ATR$S_FILE_SPEC isn't in govax's definitions either).

### TRIM_FILESPEC (461)

`old-filespec.rt.dx, new-filespec.wt.dx [,word-integer-width.rwu.r]
[,resultant-length.wwu.r]`

- Fits a file specification ("should be an RMS resultant name string")
  into a width, by removing, in order, the node (with its access
  control), the device, the directory, the version, and the type, as
  long as it's still too long; then truncating the name, LIB$_STRTRU.
  Whether each step is taken only while the result is still too long
  (the reading here) or whole steps at a time is a probe case, as are
  a specification that already fits (unchanged?), and one with no
  type or version.
- The width is word-integer-width, else new-filespec's current length
  (so a dynamic string of length 0 trims to nothing: a probe case, as is
  width 0). resultant-length excludes padding and truncated characters.
- More than 255 characters is LIB$_INVARG (5.0; dropped by **VSI**).
  It splits the name with `$FILESCAN`, whose statuses it may return;
  govax has no `$FILESCAN` (`vmsdef.P1VectorTable` has the entry, no
  service implements it), so the routine splits the name in Go (ods2's
  `filespec`, through `internal/rms`), and a later phase can add the
  service.

### For the probe (subtask 8)

A MACRO program (`testdata/lib58/`, run on a volume with a directory
tree the program makes and removes) with each case under a handler
that tells a signaled condition from a returned status, as Phases 53 to
56's probes; callbacks that print their arguments (the descriptor's
class and text, the FAB's FNS/FNA and STS/STV, the NAM's RSL and RLF,
error-source, the user argument's longword) and return what the case
says. What it should settle:

- **FIND_FILE**: wildcard and non-wildcard names; NOWILD with each;
  MULTIPLE across "[dir]A,B"; a search list logical name; a different
  filespec on the same context; a call after RMS$_NMF; status-value on
  success and failure; a short fixed resultant; a 256-character name;
  FIND_FILE_END of 0, of a good context (and the longword after), and of
  a longword that isn't one; flags bit 2 and bit 3.
- **FILE_SCAN**: success and error routines' arguments and order; an
  error routine returning failure; 0 for either routine; the status
  after the last file; two calls with and without a context (NAM$L_RLF
  after each); FILE_SCAN_END with fab, context, both, neither (the FAB
  and context afterwards).
- **DELETE_FILE**: one file, wildcards, no version, a missing file, a
  protected one; confirm refusing some; error routine returning success
  and failure; no error routine (the most severe status); a dynamic
  resultant-name; the confirm routine's `$OPEN` of its FAB; a nested
  call (DELETE_FILE from a confirm routine); the tenth argument.
- **RENAME_FILE**: `*` fields, a new directory, versions with and
  without CUR_VER (RMS$_FEX); error sources 0, 1 (a wildcard the output
  can't take), and 2; confirm and success arguments; flags 2, 4, 8.
- **FID_TO_NAME**: a file in a subdirectory (FID from a `$OPEN`'s NAM),
  `_DUA1:` and `DUA1:`, a short output, a lost file, a bad file ID, a
  non-disk device (NLA0:), a device name of 65 characters.
- **TRIM_FILESPEC**: the manual's example, widths at each step's edge,
  width omitted with a fixed and a dynamic output, width 0, a
  257-character input.
- **A condition in a callback**: a confirm routine that signals, under
  a handler in the main program that unwinds; then another call of the
  routine, to show nothing was left behind.
- The texts of LIB$_NOWILD and SHR$_PARSEFAIL (`$GETMSG`).

### The shape of the work

`corevms`: the callback mechanism (a `ShimTable` registration for
routines that call back, the goroutine and its handoff, the pending
calls found by FP and SP, cleanup on `$UNWIND` and image rundown; a new
file, `callback.go`), with the engine's side in `cpu/xfc.go` if the
XFC needs to tell a continuation from a new call (it shouldn't: the
shim dispatch can look for a pending call before the code). `librtl`:
`findfile.go` (FIND_FILE, FIND_FILE_END), `filescan.go` (FILE_SCAN,
FILE_SCAN_END), `deletefile.go`, `renamefile.go`, `fidtoname.go`, and
`trimfilespec.go`. `rms`: the per-file rename step shared with DCL's
RENAME, the back-link walk for FID_TO_NAME, and the name split for
TRIM_FILESPEC.

## Subtasks (provisional)

1. **Survey.** **Done** 2026-10-13.
2. **Callbacks from a shim**: design, the engine's side, nesting,
   unwinding; a test routine that calls back. **Done** 2026-10-13.
3. **FIND_FILE, FIND_FILE_END.** **Done** 2026-10-13.
4. **FILE_SCAN, FILE_SCAN_END.** **Done** 2026-10-13.
5. **DELETE_FILE.** **Done** 2026-10-13.
6. **RENAME_FILE.** **Done** 2026-10-13.
7. **FID_TO_NAME, TRIM_FILESPEC.** **Done** 2026-10-14.
8. **The program** (`testdata/lib58/`, on a volume), its console test,
   and the VMS probe. **Done** 2026-10-14.
9. **Close-out**: Phase 53's Part B gets the callback convention.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
- **2026-10-13** — Subtask 1, the survey. Each routine's interface
  from the 5.0 manual's entries and its section 2.7.4, with the VSI
  manual's differences (mostly long names, NAML blocks, and symbolic
  links, which the VAX lacks; DELETE_FILE's tenth argument, RENAME_FILE's
  INH_SECUR flag). All eight routines are in the transfer vector and
  their conditions in `vmsdef.LibrarySymbols`; the LIB$M_FIL_ flag masks
  and the SHR$ message texts aren't in govax's definitions. FIND_FILE's
  flags are NOWILD and MULTIPLE (there is no NODEFAULTS). The gap in
  the engine: a shim's plain CallRequest returns to the XFC with the
  callback's R0, which then selects the wrong shim; the fix is to find
  the call in progress by the stub's FP and SP first, as `$CMKRNL`
  does. The survey proposes the goroutine handoff over an explicit
  continuation, so the routines read as the manual's loops. FID_TO_NAME
  walks the back-links in Go (the ACP has no ATR$C_FILE_SPEC yet), and
  TRIM_FILESPEC splits names in Go (govax has no `$FILESCAN`). Nothing
  needed the author's decision.
- **2026-10-13** — Subtask 2, callbacks from a shim
  (`internal/corevms/callback.go`). A routine registered with
  `ShimTable.RegisterCallback` runs on a goroutine of its own, and
  `Environment.Callback(routine, args...)` is a Go call returning the
  procedure's R0: it writes the argument list below SP (in the stub's
  frame), hands the dispatch a plain `CallRequest`, and waits. The
  goroutine and the engine take turns over two channels (`handOff`), so
  only one runs at a time. When the procedure's RET lands on the XFC
  again, `Environment.Shim` looks first for a routine waiting for a
  callback whose stub FP, SP, PC, and mode match (`returningShimCall`),
  as `$CMKRNL` finds its call, and continues it whatever R0 selects, so
  the engine needed no change (only `cpu/xfc.go`'s comment). Nesting
  needs nothing more: a callback calling the routine again runs in a new
  stub frame. A routine whose callback never returns is abandoned (its
  pending Callback panics with a sentinel the goroutine recovers, so its
  deferred code runs): by `$UNWIND` for the stub frames it removes
  (`finishUnwind`, `abandonShimCallsBelow`), by image rundown
  (`cancelShimCalls`, so process deletion too), and, as a backstop, at
  the next shim call once the stack has been popped past it. A routine
  that waits (ErrWait) after calling back fails rather than starting
  over; one that waits before is run again as any shim. Tests:
  `testdata/asm/callback.asm` run by internal/console's
  `TestCallback_assembledProgram` (TEST$CALLBACK, a test-only routine at
  code 999: a loop of callbacks, a callback calling it again, a callback
  that LIB$SIG_TO_RET unwinds past, then a fresh call, with SP checked),
  and corevms's `TestCallback_*` (the argument list, continuing whatever
  R0 is, image rundown, a popped frame, the wait rule, Callback outside a
  routine); race-clean under `go test -race`. `librtl.Routine` gets its
  way to ask for `RegisterCallback` with the first routine that needs
  it (subtask 4).
- **2026-10-13** — Subtask 3, FIND_FILE and FIND_FILE_END
  (`internal/librtl/findfile.go`, codes 163 and 164). The context is a
  heap block holding a FAB, a NAM, a related NAM (NAM$L_RLF, for
  related-filespec or, with MULTIPLE, the last file found), and their
  buffers; each call runs `$PARSE` and `$SEARCH` on it through
  `CallService`, so default, related, and logical names, search lists,
  and both kinds of wildcard context are RMS's own. In Go, per image, by
  the block's address: the filespec being searched and the last file
  found. Choices made, each **unconfirmed** and a probe case: a new
  search starts on the first call, when filespec's text differs from the
  last call's, and on every call with NOWILD (which, after `$PARSE`,
  makes a wildcard LIB$_NOWILD); otherwise a call after RMS$_NMF gets
  `$SEARCH`'s RMS$_NMF again. Success is `$SEARCH`'s RMS$_NORMAL (the
  VSI manual's) or LIB$_STRTRU for a short resultant; status-value gets
  FAB$L_STV only when a service fails; a name over 255 characters is
  LIB$_INVFILSPE; flag bits past MULTIPLE are ignored; a context
  longword that names no context is RMS$_FAB to both routines.
  FIND_FILE_END parses the null string, frees the block, and zeroes the
  longword (0 is RMS$_NORMAL, nothing to end). An RMS defect found on the
  way: a `$PARSE` cleared NAM$L_WCC without freeing the wildcard context
  it named, so a search abandoned before it ran out (and the null-string
  parse meant to end it) left its context behind; `$PARSE` now frees it
  first (`rms/search.go`'s `releaseSearch`). Tests: librtl's
  `TestFindFile*` (wildcards, a wildcard directory, a search list, a new
  filespec on the same context, NOWILD, a missing file and status-value,
  default and related names, MULTIPLE, short and dynamic resultants, the
  context's checks) and rms's `TestParse_releasesSearch`.
- **2026-10-13** — Subtask 4, FILE_SCAN and FILE_SCAN_END
  (`internal/librtl/filescan.go`, codes 165 and 166), the first routine
  that calls back. `librtl`'s `callsBack` set names the routines
  `Register` installs with `RegisterCallback`. FILE_SCAN runs `$PARSE`,
  then `$SEARCH` until RMS$_NMF, on the program's own FAB through
  `CallService`, calling the success routine for each file and the error
  routine for each failure with the FAB's address. The context is a heap
  block holding a NAM whose resultant string is the last file found.
  Choices made, each **unconfirmed** and a probe case: a routine of 0
  isn't called; the routines' R0 is ignored (the scan always goes on);
  the status is the last RMS service's (RMS$_NMF after a full scan, the
  `$PARSE` error after calling the error routine); a failed `$SEARCH`
  that leaves NAM$L_WCC unchanged (a missing directory in a search
  without a wildcard context, which would fail the same way again) ends
  the scan with its status, while RMS$_FNF (WCC moved on, next RMS$_NMF)
  doesn't; with a context, the program's NAM$L_RLF points at the
  context's NAM for the call when the program gave no related NAM of its
  own, then goes back to 0; without one, FILE_SCAN does no defaulting
  of its own; a context longword that names no context is LIB$_INVARG.
  FILE_SCAN_END parses the null string on fab (FNS and DNS 0, put back
  after; RMS$_FAB, nothing changed, for a block that isn't a FAB), frees
  the context and clears its longword, and returns SS$_NORMAL. Tests:
  librtl's `TestFileScan*`, driving the routine through `env.Shim` as
  the XFC does with each callback answered in Go (`callingBack`):
  wildcards and a wildcard directory, a missing file, a missing
  directory, a bad default name, routines of 0, temporary defaulting
  through a context, and FILE_SCAN_END's arguments (including a
  wildcard context left part way, which it frees). The program run on
  the machine is subtask 8's.
- **2026-10-13** — Subtask 5, DELETE_FILE
  (`internal/librtl/deletefile.go`, code 167, in `callsBack`). A
  `$PARSE`, then `$SEARCH` until RMS$_NMF, on a search block (FIND_FILE's
  layout, `layoutSearchBlock`) below SP in the stub's frame; for each file
  the confirm routine (`filespec, fab [,user]`), `$ERASE`, then the
  success routine (`filespec [,user]`); an error searching (source 0) or
  deleting (source 1) calls the error routine (`filespec, rms-sts,
  rms-stv, error-source [,user]`, the three longwords by reference below
  SP). The FAB the confirm routine gets, and the one `$ERASE` deletes, is
  a second FAB and NAM of the file's own (its file name the resultant
  name), so a confirm routine's `$OPEN` can't clear the search NAM's
  wildcard context. Names go by resultant-name when given (written
  first), else a class S descriptor below SP; SP goes back to its place
  after each file. file-scan-context is FILE_SCAN's context (the last
  file deleted is the next call's related name; FILE_SCAN_END frees it).
  Choices made, each **unconfirmed** and a probe case: no arguments or
  more than ten is LIB$_WRONUMARG; flag bits past the VSI manual's three
  are LIB$_INVARG, those three change nothing; the user argument is
  passed only when the program's list reaches it; the status is
  SS$_NORMAL without errors, LIB$_ERRROUCAL after errors with an error
  routine, the RMS status when the error routine returns failure (which
  stops at once), and the most severe error met (the first of that
  severity) without one; the error routine's name is the NAM's resultant
  string, else its expanded string, else filespec; a related-filespec
  wins over the context's name; a `$SEARCH` that can't go on (WCC
  unchanged) ends the search, as in FILE_SCAN; a name without a version
  deletes the highest version. An RMS defect found on the way: a search
  with no context (`searchPlain`) went on by its entry's position alone,
  so deleting the file just returned made it skip the next one (DCL's
  DELETE *.TXT on VMS deletes each file found between searches); it now
  checks that the entry at the position is still the one the NAM's
  resultant string names, and otherwise goes on after that name in
  directory order (`plainResume`, **unconfirmed** as to VMS's own
  mechanism). Tests: librtl's `TestDeleteFile*` (wildcards with a
  refusing confirm and the user argument, one file with a default name,
  a missing file and directory with and without an error routine, an
  error routine that stops, an `$ERASE` failure (the confirm routine
  leaves the FAB open: RMS$_IFI, source 1), a confirm routine's
  `$OPEN`/`$CLOSE` during a wildcard-directory search, resultant-name,
  the context, the argument checks) and rms's
  `TestSearch_plainAfterDelete`.
- **2026-10-13** — Subtask 6, RENAME_FILE
  (`internal/librtl/renamefile.go`, code 168, in `callsBack`). What it
  shares with DELETE_FILE moved to `filewalk.go`'s `fileWalk`: the
  names, the file-scan context, the search block and file block below
  SP, the loop over `$SEARCH`, the status rule, and the name
  descriptors; each routine gives its per-file step and its report of
  an error searching. For each file found: the new name, worked out by
  `rms.Session.RenameTarget` (DCL RENAME's `renameTarget`: new-filespec
  parsed with the old name as its related file, `*` fields the old
  file's, the version rules; RMS$_WLD and the rest are error source 1);
  the confirm routine (`old, new, old-fab [,user]`, the new name before
  its version is known); the rename, `rms.Session.RenameFile`
  (`$RENAME`'s `renameText`, returning the version the file got; its
  failures are source 2); the success routine (`old, new [,user]`, the
  new name with its version). An error searching is source 0. The error
  routine always gets six arguments, the user argument 0 when the
  program gave none (the manual's format, without brackets). Choices
  made, each **unconfirmed** and a probe case: fewer than two arguments
  or more than twelve is LIB$_WRONUMARG; flag bits past CUR_VER,
  INH_SECUR, and LONG_NAMES are LIB$_INVARG, the last two changing
  nothing; a new-filespec over 255 characters is LIB$_INVFILSPE; at
  source 0 the new name is new-filespec as given, at source 1 as far as
  it was worked out, status value 0; the input's version for the
  version rules is the version field of the `$PARSE`'s expanded string
  (a default name's version counts); with CUR_VER, a name that exists is
  `$RENAME`'s RMS$_ENT with SS$_DUPFILENAME (the manual says RMS$_FEX);
  a file renamed to its own name keeps version 1, since `$RENAME`
  removes the old entry before working out the next version. Tests:
  librtl's `TestRenameFile*` (wildcards with a refusing confirm and the
  user argument, a new directory with a default name, the version rules
  and CUR_VER, errors at sources 0, 1, and 2 with and without an error
  routine and one that stops, the resultant names, the context, the
  argument checks, files renamed to their own names not found again).
- **2026-10-14** — Subtask 7, FID_TO_NAME and TRIM_FILESPEC
  (`internal/librtl/fidtoname.go` and `trimfilespec.go`, codes 169 and
  170). FID_TO_NAME's walk up the back links is RMS's
  (`rms.MountTable.ACPFileSpec`, `rms/acpfilespec.go`: each directory's
  and the file's name that of its entry in the directory its back link
  names, else its header's; the master file directory is
  `[000000]000000.DIR;1`), reached through
  `corevms.Environment.FileSpecByID` (`corevms/fidname.go`), which finds
  the device as `$ASSIGN` does and maps the ACP's errors as the disk
  driver does. `$ASSIGN`'s device names now take a full name with this
  node's name in front, as NAM$T_DVI holds it (`_GOVAX$DUA1`;
  `corevms/devices.go`'s `deviceName`). TRIM_FILESPEC splits the name
  in Go (`splitFileSpec`: node with access control, device, one or more
  bracketed directories, name, type, version), leaving parts out in the
  manual's order. Choices made, each **unconfirmed** and a probe case:
  FID_TO_NAME's specification starts with the device's full name
  (`_GOVAX$DUA1:`) whatever form was given; a file whose back links
  don't reach the master file directory is named without a directory,
  acp-status SS$_NOSUCHFILE, the routine still LIB$_NORMAL; a file ID
  that names no file is SS$_NOSUCHFILE, a device that isn't mounted
  SS$_DEVNOTMOUNT, one that doesn't exist SS$_NOSUCHDEV, one that isn't
  a disk LIB$_INVFILSPE; fewer than three arguments, or a 0 for one of
  them, LIB$_INVARG. TRIM_FILESPEC leaves out each part only while
  what's left is too wide; width omitted is new-filespec's current
  length; width 0 leaves nothing (LIB$_STRTRU); fewer than two arguments
  or more than four is LIB$_WRONUMARG. Tests: librtl's `TestFidToName*`
  (files at three depths, the MFD and a directory, each form of the
  device name, a short filespec, the failures with nothing written,
  broken back links) and `TestTrimFilespec*`/`TestSplitFileSpec` (the
  manual's example, each part's edge, width 0 and omitted, a short
  new-filespec, 255 and 256 characters, the argument counts).
- **2026-10-14** — Subtask 8, part 1: the program and its console
  test. `testdata/lib58/gen.go` writes `lib58.mar` (332 cases),
  `lib58.com`, and `exchange.cmd` (README.md has the layout and the VAX
  run). The program makes its own tree under `[LIB58]` (so it needs a
  fresh volume), and its own routines for FILE_SCAN, DELETE_FILE, and
  RENAME_FILE log each call and argument (descriptors, FABs and their
  NAMs, longwords by reference, values) to a buffer printed under the
  case's line; their answers come a bit a call from a cell, and a
  confirm or success routine can also open its FAB, leave it open,
  signal, or call DELETE_FILE again. LISTF shows what a delete or
  rename left. A file with no directory entry is made with the disk
  ACP's IO$_DELETE without IO$M_DELETE; the cases that pass junk
  contexts, a non-FAB, or too few arguments come last, so damage they
  might do on VMS can't spoil the rest. `TestLib58Oracle`
  (`internal/console/lib58_test.go`) runs it on a fresh RD51 volume as
  DUA1 with `[000000]` the default directory, and skips until
  `vax/lib58-vax.txt` exists. An RMS defect found on the way: a search
  that returns one version of each name (no version, or a numeric or
  relative one) came back to the same name when the version it returned
  was deleted before the next `$SEARCH`, since the name's highest version
  was then another, so LIB$DELETE_FILE of `D2.TXT` deleted every version;
  it now goes on past the name it returned (`rms/search.go`'s
  `resumeAfter`, with and without a search context;
  `TestSearch_versionsAfterDelete`; **unconfirmed** as to VMS's
  mechanism, but the probe shows what DELETE_FILE does). Seen in
  govax's run, for the VMS run to settle: `$ERASE` of a directory that
  still has an entry is RMS$_DEV with SS$_NOSUCHDEV (ods2's
  ErrDirectoryNotEmpty unmapped; the ACP's IO$_DELETE says
  SS$_DIRNOTEMPTY), and NAM$T_DVI and FID_TO_NAME's device are
  `_GOVAX$DUA1`, where VMS 7.1 on that VAX (not in a cluster) gave
  DEVNAM `_DUA1:` in Phase 55's run.
- **2026-10-14** — Subtask 8, part 2: the VMS run (OpenVMS V7.1 on
  simh's VAX 8600; `vax/lib58-vax.txt`, `vax/lib58.log`), and govax
  changed to its answers. `TestLib58Oracle` now matches all 1907 lines
  but for three in `expectedLib58Differences` (an access violation's
  address read from the stack; a header slot inside VMS's index file,
  extended to 829 blocks, but past govax's). What VMS settled:
  - **FIND_FILE**: a name with no wildcard (a search list included) gets
    one `$SEARCH`; the next call is RMS$_NMF with nothing written.
    RMS$_NMF ends the context (block freed, longword 0), so the next call
    starts again, except with MULTIPLE. A short resultant is still
    `$SEARCH`'s status (no STRTRU); a failure writes the NAM's resultant
    string, else its expanded string, else the filespec; status-value is
    written after each `$PARSE` and after a failing `$SEARCH`. MULTIPLE's
    related name is the last call's name, a failed call's included. FNS
    and DNS are bytes: a 256-character name is an empty one (no
    INVFILSPE). FIND_FILE_END's success is SS$_NORMAL. Flag bits past
    MULTIPLE are ignored; a text comparison decides "a different
    filespec".
  - **FILE_SCAN**: the routines get FILE_SCAN's own argument list. Its
    first act is a write of the NAM's WCC (a FAB with no NAM, or a block
    that isn't a FAB, is an access violation at NAM$L_WCC+3). A failed
    `$PARSE` leaves the expanded string, else the filespec, in the
    resultant string. One `$SEARCH` for a name with no wildcard; RMS$_FNF
    and RMS$_DNF end a scan; the status is the last service's. After a
    successful `$PARSE`, DNS is 0. With a context, the related name is a
    chain of name blocks, newest first, each RLF the next (temporary
    defaulting through every earlier call: `librtl/filescan.go`'s
    `scanChain`, and RMS's `relatedName` follows a related NAM's own
    RLF, **unconfirmed** which of the two VMS does); a call's filespec
    joins the chain at the start of the next call, and the context is made
    when the call ends (a call abandoned in a callback leaves the longword
    0). An unknown context is used unchecked (its related NAM is
    RMS$_RLF). FILE_SCAN_END leaves FNS, DNS, and the NAM's ESS 0
    (**unconfirmed** which VMS sets; a FILE_SCAN after is RMS$_ESL), and
    clears an unknown context's longword with SS$_NORMAL.
  - **DELETE_FILE**: at most nine arguments (no flags on VMS 7.1). The
    confirm routine gets the search's FAB (FAB$V_NAM set), whose related
    NAM holds the expanded name; govax puts the search NAM back after it,
    and a FAB left open makes the delete RMS$_IFI (source 1), then the
    search (source 0), which ends it. The user argument is always passed,
    0 when not given. An error routine that stops makes the status severe
    (%X18292 returns %X18294). A wildcard filespec with no default gets
    ";*" as its default (**unconfirmed** for one without a wildcard:
    DELETE of `D2.TXT` deleted only the highest version). A directory
    that still has entries is RMS$_MKD with SS$_DIRNOTEMPTY.
  - **RENAME_FILE**: the new name is worked out, its syntax checked by
    RMS's rules and its directory looked up, before the confirm routine;
    a failure there is source 1 (`;Q` RMS$_SYN with new-filespec as
    given; a missing directory RMS$_DNF, SS$_NOSUCHFILE; `A*B` RMS$_FNM).
    An input that named a version doesn't keep it: the next version of
    the new name (the DCL Dictionary's RENAME rules; DCL RENAME changes
    with it). LONG_NAMES (bit 2) is LIB$_INVARG; INH_SECUR is taken. A
    rename to another device is source 2 with STV 0.
  - **FID_TO_NAME**: SS$_NORMAL; the device is the volume's logical
    name, `DISK$label:` (`DISK$LIB58:`); a failure writes the device alone and the ACP's
    status as acp-status, which is the routine's status (a header slot
    never used SS$_FILESTRUCT, others SS$_NOSUCHFILE); directory-id is
    used when given (a directory without the file, or an ID of 0, is
    SS$_NOSUCHFILE); a file its back link's directory has no entry for is
    `[]NAME.TYP;V`; a device that isn't a disk is LIB$_INVFILSPE with
    filespec-length 0 and acp-status SS$_NORMAL; an empty device name is
    SS$_IVLOGNAM; a file-id of 0 is an access violation.
  - **TRIM_FILESPEC**: as govax did, but more than 255 characters is
    trimmed (no INVARG).
  - **Messages**: the shared (SHR$) messages, a code whose bit 15 is
    clear shown with its own facility's name: `%LIB-E-NOWILD, no
    wildcard permitted`, `PARSEFAIL, error parsing !AS`
    (`vmsdef/messages.go`'s `sharedMessages`, from VMS's output).
  - **RMS**, beyond the routines: `;Q` and a directory with no closing
    bracket are RMS$_SYN (were RMS$_VER and RMS$_DIR; unconfirmed choices
    of Phase 24's); RMS$_DNF writes the expanded name as the resultant
    string; a wildcard-directory search whose directory above the
    wildcard is missing is RMS$_DNF at once; RMS$_FNF from a
    wildcard-directory search has STV 0; and a `[DIR...]` search that
    runs out leaves `[DIR]*.DIR;1`, DID [DIR]'s, and FNB as for its
    subdirectories (VMS 7.3's RMS3 SEARCH case 3 shows the same, so that
    case is now compared rather than masked).
  - **The assembler**: comparing the VAX's LIB58.OBJ with govax's showed
    real MACRO assembling `FAB$B_BID(R2)` (a known 0) as `(R2)`, register
    deferred; govax's MACRO dialect now does (`asm/operand.go`'s
    `registerDisplacement`; `TestMACROZeroDisplacement`; a literal 0 is
    taken the same way, **unconfirmed**), and the objects match byte for
    byte but for the headers that name the assembler.

