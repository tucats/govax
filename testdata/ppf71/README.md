# The Phase 71 probe: process-permanent files

These fixtures are `docs/PHASE-71 - process-permanent files.md`'s
subtask 1. They ask VMS how a process-permanent file looks from DCL and
from an image:

- the equivalence strings of SYS$INPUT, SYS$OUTPUT, SYS$ERROR, and
  SYS$COMMAND, and of the names DCL's OPEN defines: their length, their
  first four bytes (ESC, a zero byte, and the internal file index, IFI,
  as a word, if the *Guide to OpenVMS File Applications* (6.6) and the
  System Services Reference are right), the rest, and their attributes,
  at executive, supervisor, and user mode; at level 1 under `/OUTPUT`, in
  a nested procedure with an output file of its own, and in a
  subprocess;
- what SHOW LOGICAL, SHOW TRANSLATION, F$TRNLNM, F$PARSE, F$SEARCH,
  F$GETDVI, `$GETDVIW`, and `$ASSIGN` make of those strings, and of a
  copy of one under another name (by DCL's DEFINE, and by `$CRELNM`);
- RMS's indirect access (the guide's 6.6): what `$OPEN` and `$CREATE` of
  such a name return in the FAB (status, STV, IFI, FOP, device
  characteristics, FAC, SHR, ORG, RFM, RAT, MRS, ALQ, FSZ) and the NAM
  (expanded and resultant strings, FNB, DVI, FID); whether two opens
  share one record position, with each other and with DCL's own READ
  and WRITE; which operations are refused (`$REWIND`, `$FIND`, `$GET` of
  an output stream, `$ERASE`, `$DISPLAY`, FOP's PPF, UFO, and NFS, a
  `$PUT` to an input stream); FAC ignored; a name with a file
  specification after its colon; `_SYS$OUTPUT:`; `$PARSE` and `$SEARCH`;
  and what image rundown does with a stream the image left open.

| File | What it holds |
| ---- | ------------- |
| `ppf71.mar` | The program: parts T (translations), D (devices), O (SYS$OUTPUT through RMS), I (SYS$INPUT, from a procedure's data lines), F (files DCL's OPEN opened), and X (SYS$OUTPUT left open), picked by the logical name `PPF71_PART` (the source's opening comment lists them). Each case writes a line saying what it is first, then a report of each RMS operation. |
| `ppf71.com` | Assembles and links the program; DCL's view (a `LEX` subroutine: F$TRNLNM's bytes and items, F$PARSE, F$GETDVI) at level 1; every part; the subprocess and the nested procedure |
| `ppf71s.com` | The nested procedure, `@PPF71S/OUTPUT=PPF71S.LOG`: DCL's view, parts T and O |
| `ppf71p.com` | The subprocess's procedure, `SPAWN/OUTPUT=PPF71P.LOG/INPUT=NL: @PPF71P`, spawned while PPF71_IN and PPF71_OUT are open: DCL's view, parts T and D, a READ of the parent's PPF71_IN |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/ppf71/exchange.cmd

   This makes `testdata/disks/ppf71-exchange.dsk` (RD51 size, label
   PPF71, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the
   default directory, and run, at the terminal:

       @PPF71/OUTPUT=PPF71.LOG

   Nothing should wait for input: part I reads the procedure's own data
   lines, and the subprocess's input is NL:. O14's record goes to
   SYS$ERROR, so it shows on the terminal. Part F's `$PUT` to PPF71_IN
   (F7) and part O's FOP cases (O5 to O7) are expected to fail; if a run
   hangs, a Ctrl/Y and `STOP` ends it, and the logs up to there are
   still useful.
3. **The terminal case** (optional, but the only way to see the strings
   with no `/OUTPUT` in the way): at the `$` prompt, with the same
   default directory,

       DEFINE/USER_MODE PPF71_PART T
       RUN PPF71
       DEFINE/USER_MODE PPF71_PART D
       RUN PPF71
       DEFINE/USER_MODE PPF71_PART O
       RUN PPF71

   and save the screen as `testdata/ppf71/vax/ppf71-term.log`.
4. With simh paused (or the disk detached), copy the results back, from
   the repository root:

       govax console < testdata/ppf71/copyout.cmd

   That copies the three logs, PPF71B.DAT (the file DCL and the image
   both wrote), and the map as text, and the image byte for byte into
   `vax/`, so an oracle can run VMS's own image on govax.

None of the sources calls a system macro but the RMS macros, `$FABDEF`,
`$RABDEF`, `$NAMDEF`, `$LNMDEF`, `$DVIDEF`, `$TRNLNM_S`, `$CRELNM_S`,
`$GETDVIW_S`, `$ASSIGN_S`, `$DASSGN_S`, and `$FAOL_S`, and nothing copied
back is a DIGITAL file, so the volume needs no erasing afterward.

## govax's dry run

govax runs the whole procedure (2026-10-21), and its answers are the
phase doc's Part A, "What govax does today": no prefix anywhere, `/OUTPUT`
leaving SYS$OUTPUT naming the terminal, and an image's `$CREATE` of a
name DCL opened making a new file called `_GOVAX$DUA1.;1`.
