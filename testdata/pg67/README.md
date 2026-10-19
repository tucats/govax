# The Phase 67 probe: the address space

These fixtures are `docs/PHASE-67 - address space under paging.md`'s
subtask 9: the VMS run that settles what the phase's progress log left
unconfirmed (the doc's "The probe's cases" lists the questions). One
MACRO program, linked twice, reads the process's address space items
with `$GETJPIW` around each case; a command procedure asks SYSGEN for
the parameters, shows the items from DCL between the runs, and has the
program run part Q in a detached process.

| File | What it holds |
| ---- | ------------- |
| `pg67.mar` | The program (below), by hand |
| `pg67s.opt` | PG67S.EXE's options: `STACK=50` |
| `pg67.com` | The procedure: SYSGEN's parameters, SHOW PROCESS/QUOTA, the build, the runs, and a `va` line (F$GETJPI's FREP0VA, FREP1VA, FREPTECNT, VIRTPEAK, PAGFILCNT) after each |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back |
| `vax/` | VMS's output: the procedure's log, each part's lines, the maps, the object, and both images (no `pg67l1.log`: the first LINK wrote nothing, so its file was never made) |

## How the program works

It uses only system services and RMS, and prints with `$FAOL` and `$PUT`
to SYS$OUTPUT. A snapshot (`SNAP`) is one `$GETJPIW` of FREP0VA,
FREP1VA, FREPTECNT, VIRTPEAK, PAGFILCNT, PGFLQUOTA, and CURPRIV. A case's
`SPACE` line is the changes between two snapshots, in pages:

    id what: p0= p1= free= peak= charged=

(`p0` is FREP0VA gone up by, `p1` FREP1VA gone down by, `free` and
`peak` FREPTECNT's and VIRTPEAK's changes, `charged` PAGFILCNT gone down
by). Addresses show as offsets from FREP0VA or FREP1VA (`frep0va+`,
`base+`). A reference is made under a condition handler (`TOUCH`): its
line is `ok`, or the condition, its reason mask (1 length violation, 2
PTE reference, 4 write), and its address. `PROBE` lines are PROBER and
PROBEW from user mode. Statuses are in hex. The logical name PG67_PART
picks the part:

| Part | Lines | What they ask |
| ---- | ----- | ------------- |
| A | A1 | At the first instruction: FREP0VA (the image's end), SP less FREP1VA (the user stack's size), FREPTECNT, VIRTPEAK, and the pages charged by activation |
| | A2, A3 | `$EXPREG` of 10 P0 and 10 P1 pages: where, and the items' changes (FREP1VA is an address? VIRTPEAK counts both regions?) |
| | A4 | `$DELTVA` of P1's 3 lowest pages: does P1 get shorter? Then `$EXPREG` of 2 P1 pages: where they land (on the freed pages, or below them?) |
| | A5 | `$DELTVA` of P0's last 3 pages: does P0 get shorter? |
| | A6 | `$DELTVA` of a page in the middle of P0: FREP0VA stays? A read of it (a protection violation, mask 0?) and PROBE |
| | A7 | `$CNTREG` of 2 pages of each region |
| | A8 | `$CRETVA` of one page 5 pages past P0's end: P0 grows past the 4 skipped? A read of a skipped page (mask 0?), PROBE of it, and a read of the new page |
| | A9 | Reads and writes at P0's end and a megabyte past it: the masks (1 and 5?); PROBE of each; nothing changes? |
| | A10 | Below P1's low end from user mode: PROBE (no expansion?), a read at FREP1VA (one page?), a read 50 pages below (50 more?) |
| | A11 | A read at FREP1VA from executive mode (`$CMEXEC`): no expansion, mask 1? |
| | A12 | `$CRETVA` of FREPTECNT+100 pages from P0's end, then below FREP1VA: SS$_VASFULL with none made, or pages up to a limit (the quota first?) |
| | A13 | A user-mode read further below FREP1VA than FREPTECNT pages (testdata/lib59's references to 0x55555555): pages made, or none? |
| | A14 | The whole image's changes |
| S | S1-S3 | Recursion 100 levels deep, 512 bytes of locals a level: the stack's growth at the deepest and after the return (the pages stay?), charged a page at a time; then `$EXPREG` of 5 P1 pages: below the deepest point? |
| T | A1 | The first instruction only, from PG67.EXE (LINK's default stack) and PG67S.EXE (`STACK=50`): SP less FREP1VA, and the activation's charges |
| C | C1-C3 | Part Q in a detached process (`$CREPRC`, PRCNAM PG67DET, a PGFLQUOTA of 2048 raised to PQL_MPGFLQUOTA if that's more, a termination mailbox); its lines are `PG67Q.OUT`; the termination message's final status |
| Q | Q1-Q3 | (No PG67_PART: the detached process.) `$EXPREG` of PAGFILCNT less 10 P0 pages, then recursion 30 levels deep under a handler: does the handler see the access violation when the stack can't grow, or does the image end (the book's 16.2.1.3: "the normal condition handler search is bypassed")? Then `$EXPREG` of a P1 page |

PG67.COM runs A, S, T (twice), and C, with a `va` line from DCL after
each, so the items before and after a run show what image rundown gives
back. The procedure's TYPEs put every part's lines at the end of its log.

## Not asked

VIRTUALPAGECNT can't be changed for one process (it's a SYSGEN
parameter that needs a reboot), so SS$_VASFULL at a small one, and deep
recursion at the address space's end, aren't asked; A12 and A13 meet the
address space's end at the VAX's own VIRTUALPAGECNT (65500 on the probe
VAX), with a PGFLQUOTA (40960 for SYSTEM on it) smaller than FREPTECNT,
so the quota and the room are told apart by the status.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/pg67/exchange.cmd

   This makes `testdata/disks/pg67-exchange.dsk` (RD51 size, label PG67,
   gitignored).
2. Attach it to the simh VAX, log in as SYSTEM, and:

       MOUNT DUA1: PG67
       SET DEFAULT DUA1:[000000]
       @PG67/OUTPUT=PG67.LOG

   Part C waits for the detached process (at most two minutes); the
   whole run takes well under a minute.
3. Copy the results back, from the repository root:

       govax console < testdata/pg67/copyout.cmd

   The procedure's log, the runs' outputs, MACRO's and the two LINKs'
   messages and maps, the object, and both images go to `vax/`. MACRO's
   messages (`pg67_build.log`) may quote STARLET macro expansions: the
   clean-room hook refuses them until the author has audited them.

## The VAX run (2026-10-19)

OpenVMS V7.1 on simh's VAX 8600 (`vax/`). That VAX had VIRTUALPAGECNT
65500, CTLPAGES 100, and PQL_DPGFLQUOTA and PQL_MPGFLQUOTA 16400 (so the
detached process's 2048 became 16400); SYSTEM's PGFLQUOTA was 40960.
`TestPG67Oracle` runs VMS's own two images on govax as PG67.COM did and
matches VMS's lines, VIRTPEAK masked (it's the process's peak, which
VMS's DCL had already taken higher), but for `expectedPG67Differences`.
VMS answered:

- **Confirmed as govax had them:** every `$EXPREG`, `$DELTVA`, and
  `$CRETVA` line of part A, its items' changes and charges; `$DELTVA` of
  a region's end pages shortens it, and P1's freed addresses are reused
  (A4); a middle page's deletion leaves the length, and reading it is
  mask 0; `$CRETVA` past P0's end lengthens P0 over the skipped pages,
  which are mask 0; the masks past P0's end (1, and 5 for a write), near
  or far; PROBE never expands; a user-mode read below P1's low end
  creates every page down to it, charged, and an executive-mode one is
  mask 1 with nothing created; a read further below than FREPTECNT
  creates nothing (A13); the recursion's 87 pages, kept after the
  return, with the next P1 `$EXPREG` just below; STACK=50's 30 pages
  more at the first instruction; and part Q's image ending with
  SS$_ACCVIO (`finalsts=1000000C`) when its stack outgrew the quota,
  no handler run, no Q2 or Q3 line. The image's whole changes (A14) are
  the same. After every run DCL's FREP0VA is empty, and FREP1VA,
  FREPTECNT, and PAGFILCNT are back where they were.
- **Changed:**
  - **Only P1 creation is checked for room first:** A12's `$CRETVA` of
    FREPTECNT+100 pages made none in P1 (SS$_VASFULL), but in P0 made
    39708 pages one at a time until the quota stopped it (SS$_EXQUOTA).
    govax's dry run had refused both at once; now P0's pages are made
    one at a time as VMS's were, stopping at the quota or the area's end
    (`regions.go`'s `spaceHolds`).
  - **`$CNTREG`'s retadr** is in `$EXPREG`'s order (P0 low to high, P1
    high to low), the reverse of the deletion's (A7); govax's was the
    deletion's.
  - **No message** when an exception has no stack to be reported on:
    part Q's output has none (`lastChanceExit`).
  - **F$GETJPI's FREP0VA and FREP1VA** are eight hexadecimal digits, and
    FREP0VA "" when P0 is empty (the procedure's `va` lines: `frep0va=
    frep1va=7FED5600`), as DCL writes JPI$_OWNER; govax's were decimal.
- **Differences that stay** (the test's list): the first instruction's
  lines (VMS's activation used 112 bytes of the stack to govax's 88, its
  FREPTECNT is VIRTUALPAGECNT 65500's less DCL's P1 pages, and its job
  had 1188 pages charged to govax's 25: DCL, LIBRTL, P1's image
  pieces); A12's P0 count, which is the quota left; and A13's address,
  which is FREPTECNT's distance below FREP1VA.

## govax's dry run (2026-10-19, before the VAX run)

`TestPG67DryRun` (`internal/console/pg67_test.go`, since replaced by
`TestPG67Oracle`) built PG67.EXE and
PG67S.EXE with govax's MACRO and LINK/NOTRACEBACK on an RD54 volume
(paging on, a 20000-block page file on the volume) and runs parts A, S,
T (both images), and C as PG67.COM does. What govax answers, for
comparison with VMS's:

- **A1:** FREP0VA ^X2600, the image's end; SP is ^X29A8 above FREP1VA,
  the 20-page default stack less 88 bytes the activation's call used;
  25 pages charged by the activation (the stack, DATA).
- **A2, A3:** `$EXPREG`'s pages at FREP0VA and just below FREP1VA (P1's
  retadr high page first); FREPTECNT down by 10 each; VIRTPEAK up by 10
  each; each page charged.
- **A4, A5:** `$DELTVA` of a region's end pages shortens it (FREP1VA up
  by 3, FREP0VA down by 3), the charges refunded; the next P1 `$EXPREG`
  reuses the freed addresses.
- **A6:** a page deleted in the middle leaves P0's length; reading it is
  a protection violation, mask 0; PROBE says neither.
- **A7:** `$CNTREG` shortens each region by 2.
- **A8:** `$CRETVA` 5 pages past P0's end lengthens P0 by 6, charging
  only the page made; a skipped page is a protection violation (mask 0).
- **A9:** past P0's end, a read is mask 1, a write mask 5, near or far;
  nothing changes.
- **A10:** PROBE below P1 doesn't expand; a user-mode read at FREP1VA
  creates that page, and a read 50 pages below creates the 51 down to
  it, each charged.
- **A11:** an executive-mode read at FREP1VA is an access violation,
  mask 1, nothing created.
- **A12:** `$CRETVA` of FREPTECNT+100 pages, in either region, is
  SS$_VASFULL with nothing made (govax checked the room for the whole
  range first; VMS does so only in P1, above).
- **A13:** the far read is mask 1, nothing created.
- **S:** the recursion grows P1 by 87 pages (the default stack's 20
  held the rest), each charged, and the pages stay after the return; the
  P1 `$EXPREG` lands directly below the deepest point.
- **T:** STACK=50 gives SP ^X3C00 (30 pages) more above FREP1VA, and
  charges 30 pages more.
- **C, Q:** with 10 pages of quota left, the recursion's stack can't
  grow past them, and there's no stack left to report the access
  violation on: the handler is passed over, the condition's message goes
  to the console (no longer: VMS writes none), and the image ends with
  SS$_ACCVIO (`finalsts=1000000C`), so no Q2 or Q3 line.
