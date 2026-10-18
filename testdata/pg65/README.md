# The Phase 65 probe: sections under the pager

These fixtures are `docs/PHASE-65 - sections and paging.md`'s subtask 7:
the VMS run that settles what the phase's survey and progress log left
unconfirmed (the doc's "The probe's cases" lists the questions). One
MACRO program, in parts, reads the process's paging counts with
`$GETJPIW` and the system's global section and page file counts with
`$GETSYIW` around each case; a command procedure asks SYSGEN for the
parameters, runs the parts, and runs the holder part in a subprocess.

| File | What it holds |
| ---- | ------------- |
| `pg65.mar` | The program (below), by hand |
| `pg65.com` | The procedure: SYSGEN's parameters, SHOW WORKING_SET, SHOW MEMORY/PHYSICAL_PAGES around part D, the parts' runs, and `gbl` lines (F$GETSYI's FREE_GBLSECTS, FREE_GBLPAGES, CONTIG_GBLPAGES, PAGEFILE_FREE) between them |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back |
| `vax/` | VMS's output, after the run |

## How the program works

It uses only system services and RMS, and prints with `$FAOL` and `$PUT`
to SYS$OUTPUT. It locks its own code and data pages in the working set
first (`$LKWSET`), so that the fault counts are the cases' own and not
the image's pages faulting in. A snapshot (`SNAP`) is one `$GETJPIW` of
PAGEFLTS, PAGFILCNT, GPGCNT, PPGCNT, DIRIO, and WSSIZE, and one
`$GETSYIW` of PAGEFILE_FREE, FREE_GBLPAGES, FREE_GBLSECTS, and
CONTIG_GBLPAGES (those three by their codes, `^X110F`, `^X1111`, and
`^X1110`, which not every `$SYIDEF` defines). Most cases print a
`REPORT` line, the changes between two snapshots:

    id what: flts= charged= gpg= ppg= pfused= dirio=

(`charged` is PAGFILCNT gone down by, `pfused` PAGEFILE_FREE gone down
by), and the global section cases a `GREPORT` line, `sects= gblpages=
contig=`, the free counts gone down by. Statuses are in hex. A page's
contents are checked as it's read (`bad=` counts the pages that don't
hold what they should). Sections are mapped with `SEC$M_EXPREG` at P0's
end; PG65.DAT's page n (block n+1) holds n+1 in its first longword.

The logical name PG65_PART picks the part; PG65.COM runs them in this
order, each to its own `PG65x.OUT`:

| Part | Lines | What they ask |
| ---- | ----- | ------------- |
| M | M1 | Makes PG65.DAT: 200 fixed 512-byte records, a block each |
| G | G1 | GBLSECTIONS, GBLPAGES, GBLPAGFIL, PFCDEFAULT, PAGTBLPFC; the sections and global pages in use |
| | G2 | Page-file sections of 1, 2, 3, 4 pages, each made and deleted: GBLPAGES' counting (a page per page plus 2, rounded up to even?), CONTIG_GBLPAGES |
| | G3 | A 3-page global file section: the same rule? |
| | G4 | A page-file section of GBLPAGFIL + 1 pages: SS$_EXGBLPAGFIL? retadr? |
| F | F1 | A permanent 64-page page-file section, PG65_PF: what mapping charges (PGFLQUOTA? GBLPAGFIL?), GPGCNT before any touch |
| | F2 | Every page read (zero? demand-zero faults, GPGCNT), then written |
| | F3 | A second mapping in the same process, read through (global valid faults; GPGCNT counts both?) |
| | F4 | Both mappings deleted: the section stays; page file use then, and a few seconds later |
| D | D1 | MPW_HILIMIT + 1000 pages written at a small working set, so that the modified page writer runs with PG65_PF's pages on the modified list |
| R | R1, R2 | PG65_PF mapped again: its contents kept? faults; the page file's blocks |
| | R3 | `$DGBLSC` while mapped, then `$MGBLSC` (NOSUCHSEC at once?) |
| | R4 | Unmapped: the section gone, its page file blocks and global pages back? |
| P | P1-P4 | A 64-page global file section read at a small working set (pfc 1), its range purged (`$PURGWS`), read again: GPGCNT, faults |
| | P5, P6 | 100 other pages written, then the section read again |
| C | C1-C3 | A writable private copy-on-reference section of PG65.DAT (opened read-only): charged at mapping? at the first touch? at writing? refunded at deletion? |
| | C4 | A read-only CRF mapping: charged? |
| | C5 | A temporary global CRF section, every page written by its only mapper: charged when? does the section survive the faults (FREE_GBLSECTS)? |
| | C6 | `$MGBLSC` of it by name: does it still exist, and are its pages the file's or the first mapping's? |
| U | U1, U2 | A writable global section of PG65.DAT: every page written, the range purged, `$UPDSECW` with UPDFLG 0: status, IOSB, retadr (pages written, as offsets), and how many reached the file (a private mapping of the same blocks reads them back) |
| | U3 | Again at once, with 0 and with 1 (SS$_NOTMODIFIED?) |
| | U4 | Pages 0-31 written and still valid, UPDFLG 1 |
| | U5 | Pages 10-19 written and purged, UPDFLG 1: are pages out of the caller's working set written? |
| | U6 | `$UPDSECW` of a page-file section |
| | U7 | A writable private section, 16 pages written and purged, UPDFLG 0 |
| K | K0 | The working set limit as high as `$ADJWSL` takes it |
| | K1 | A 200-page private section read in order with pfc 0 (PFCDEFAULT), 1, 4, 16, 64, 127, 200: faults (one per cluster?), DIRIO; down with pfc 16 and 0 (the backward scan); a global section with pfc 16 up and down; a read-only CRF section |
| | K2 | pfc 64 at a small working set: where the cluster stops |
| H | H1-H3 | The holder, in the subprocess PG65HLD: maps PG65_SH (64 pages of PG65.DAT, pfc 1) and PG65_SP (a 16-page page-file section, page n holding 2000+n), touches every page, and steps with part S through common event flags 64 to 67 (cluster PG65_EFC); at flag 65 it purges PG65_SH |
| S | S0-S3 | While the holder holds them: PG65_SH mapped and read (global valid faults, GPGCNT), purged and read again; PG65_SP read |
| | S4 | After the holder purged its own: purged and read again here (faults off the lists?) |

Not probed: a copy-on-reference mapping past PGFLQUOTA (SYSTEM's quota
is far more than the file's pages; the program would have to fill it
first, as pg64's detached process did), and read clustering from the
page file (its pages are read from the page file only once their frames
are reused, which a VAX with a large free list doesn't do: pg64's W4
read back pages that had been written, at one fault each).

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/pg65/exchange.cmd

   This makes `testdata/disks/pg65-exchange.dsk` (RD51 size, label PG65,
   gitignored).
2. Attach it to the simh VAX, log in as SYSTEM (part F's permanent
   section needs PRMGBL), and:

       MOUNT DUA1: PG65
       SET DEFAULT DUA1:[000000]
       @PG65/OUTPUT=PG65.LOG

   It takes a minute or two: part D writes a few thousand pages, and
   parts S and H step through their flags.
3. Copy the results back, from the repository root:

       govax console < testdata/pg65/copyout.cmd

   The procedure's log, the parts' outputs, and MACRO's and LINK's
   messages (`pg65_build.log`, which may quote lines of STARLET macro
   expansions: the clean-room hook refuses them until the author has
   audited them) go to `vax/`.

If part S finds the holder isn't ready ("S0 ... the holder ready: 0"),
the subprocess didn't start in time; the procedure STOPs PG65HLD at the
end whatever happened.

## govax's dry run (2026-10-18)

`TestPG65DryRun` (`internal/console/pg65_test.go`) runs every part on
govax (paging on, an RD54 volume with a 20000-block page file on DUA1,
govax's default SYSGEN values), the holder in a spawned subprocess as
PG65.COM runs it. Every page read back held what it should. What govax
answers, for comparison with VMS's:

- **G:** GBLPAGES' rule as the utilities manual gives it (4, 4, 6, 6 for
  1 to 4 pages; 6 for the 3-page file section). G4 is SS$_GPTFULL, not
  EXGBLPAGFIL: govax's GBLPAGES and GBLPAGFIL are both 4096, so the
  GBLPAGES check, made first, fails first (the order is govax's,
  unconfirmed; the VAX's GBLPAGES is likely larger).
- **F:** nothing charged or counted at mapping; 64 demand-zero faults and
  GPGCNT 64 at the first read, nothing more at writing; the second
  mapping 64 global valid faults, GPGCNT 128; unmapped, the section and
  its 66 global pages stay.
- **R:** the contents kept; deleting the section gave back the 64 page
  file blocks part D's writer had given its pages; `$MGBLSC` after
  `$DGBLSC` is SS$_NOSUCHSEC at once.
- **P:** GPGCNT 59 at the small working set; purging takes it to 0, and
  the pages come back with one fault each.
- **C:** a copy-on-reference section is charged at mapping (read-only
  too), refunded at deletion; the temporary global CRF section is gone
  once its only mapper has touched every page (C5's `sects=0`), so C6's
  `$MGBLSC` is SS$_NOSUCHSEC.
- **U:** UPDFLG 0 writes the purged pages (64 on disk); again at once,
  SS$_NOTMODIFIED with either flag; UPDFLG 1 writes the caller's valid
  modified pages, and passes over pages out of its working set (U5:
  SS$_NOTMODIFIED, none on disk; govax's reading of the flag); a
  page-file section is SS$_NOTMODIFIED; a private section's purged pages
  are written.
- **K:** one fault per cluster: pfc 0 (PFCDEFAULT 32) 7, 1 200, 4 50,
  16 13, 64 4, 127 and 200 2 (a pfc above 127 is 127); down the same;
  global and CRF the same; at a small working set 164.
- **S, H:** the holder's pages are global valid faults here (GPGCNT 64),
  purged and back with one fault each, before and after the holder
  purged its own.
- **What govax's DCL lacks:** F$GETSYI's HW_NAME, SYSGEN, and SHOW
  MEMORY/PHYSICAL_PAGES; the procedure goes on past them (SET NOON).
- **Found and fixed:** F$GETSYI didn't know FREE_GBLSECTS, FREE_GBLPAGES,
  CONTIG_GBLPAGES, GBLSECTIONS, GBLPAGES, GBLPAGFIL, or PFCDEFAULT
  (`$GETSYI` did); `TestGetSYIGlobalSectionItems`.
