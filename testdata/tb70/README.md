# The Phase 70 probe: traceback

These fixtures are `docs/PHASE-70 - traceback.md`'s subtask 1. They ask
VMS what an unhandled condition prints, and when:

- the traceback facility's symbolic stack dump
  (`%TRACE-x-TRACEBACK, symbolic stack dump follows`), in an image
  linked with traceback (LINK's default);
- the "Improperly handled condition, image exit forced." dump (the
  signal array, the stack, the registers), in an image linked
  `/NOTRACEBACK`;
- which severities get which; which frames the dump shows, and how it
  names a PC in a JSB subroutine, a psect with no routine, data,
  LIBRTL, a shareable image, a condition handler, an AST routine, an
  exit handler, or no image at all; how deep it goes; how it lays out
  31-character names; and its line numbers in an image linked `/DEBUG`;
- the order of the condition search's last stages: whether a
  last-chance handler the program sets runs before the catch-all, and
  whether a handler that signals while it runs is called again for its
  own condition;
- where the dump goes when SYS$OUTPUT and SYS$ERROR are apart (to both,
  as `$PUTMSG`'s messages do, the author reports; the probe shows each
  file's order), and in a subprocess.

Phase 29's probe (`testdata/mar/list/vax/list.log`) already has VMS
7.3's dumps for an access violation two calls deep and for a signaled
error then a LIB$STOP, with and without traceback; this probe goes
further.

| File | What it holds |
| ---- | ------------- |
| `tb70.mar` | The program: 21 parts, picked by the logical name `TB70_PART` (the source's opening comment lists them). Part S signals conditions the program goes on from (one of each severity, then errors from every kind of place, a severe one last); each other part ends the image with one unhandled condition. Each case writes a line saying what it is first. |
| `tb70long.mar` | A module with a 31-character name and routines with 20- and 31-character names |
| `tb70shr.mar` | TB70SHR, a shareable image with a transfer vector: SHRDO (signal an error, or an access violation) and SHRCALL (call a routine back) |
| `tb70.com` | Assembles everything, with traceback and with `/DEBUG`; links TB70SHR, then the program three ways (TB70 with traceback, TB70NT `/NOTRACEBACK`, TB70D `/DEBUG`); runs every part under each (`RUN/NODEBUG`), showing `$STATUS`; then SYS$OUTPUT and SYS$ERROR apart, a subprocess, and no part at all |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `tb70b.mar`, `tb70b.com` | Round 2: what round 1 left open (the source's opening comment lists its parts, picked by `TB70B_PART`) |
| `exchange2.cmd`, `copyout2.cmd` | Add round 2 to the exchange volume, and copy its results into `vax/` |

`tb70.com` takes an optional P1, the list of parts to run (`"S,A,O"`);
VMS's run gives none. govax's own dry run leaves part H out
(`docs/PHASE-70 - traceback.md`'s side findings: govax loops on it).

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/tb70/exchange.cmd

   This makes `testdata/disks/tb70-exchange.dsk` (RD51 size, label TB70,
   gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the
   default directory, and run:

       @TB70/OUTPUT=TB70.LOG

   Everything goes to `TB70.LOG`. The condition messages also reach the
   terminal (SYS$ERROR), so the terminal shows them as the runs go by.
   Nothing should wait for input. Part X's exit handler, part T's AST,
   and part H's handler each fail once; if any run hangs or loops, a
   Ctrl/Y and `STOP` ends it, and the log up to there is still useful.
3. With simh paused (or the disk detached), copy the results back, from
   the repository root:

       govax console < testdata/tb70/copyout.cmd

   That copies the log and maps as text and the images
   byte for byte into `vax/` (`TestTB70Oracle` will run VMS's own
   images on govax, so the PCs match), and the objects in the host
   variable-length record layout.

None of the sources calls a system macro but `$LNMDEF`, `$TRNLNM_S`,
`$DCLAST_S`, `$DCLEXH_S`, and `$SETEXV_S`, and nothing copied back is a
DIGITAL file, so the volume needs no erasing afterward.

## Round 2

Round 2 asks what round 1 left open: where a routine ends for
traceback (a return PC at routine+3FF and +400), which conditions get
the "improperly handled" dump (SS$_OPCDEC by LIB$SIGNAL, SS$_BADPARAM
with a longer signal array, LIB$STOP of SS$_ACCVIO), where that dump's
stack starts and what its SP is (three longwords pushed), and where the
catch-all writes with SYS$OUTPUT and SYS$ERROR apart (the program
writing nothing to SYS$OUTPUT first, then a line). On the same volume:

1. From the repository root: `govax console < testdata/tb70/exchange2.cmd`
2. On the VAX, in the volume's `[000000]`: `@TB70B/OUTPUT=TB70B.LOG`
3. From the repository root: `govax console < testdata/tb70/copyout2.cmd`

## The runs

Round 1: 2026-10-21, OpenVMS V7.1 on simh's VAX 8600, the volume on
DUA1. MACRO and LINK had no messages; every part ran. Round 2: the same
day, on the same system and volume; every part ran.
`docs/PHASE-70 - traceback.md`'s Part A has the answers ("The probe's
answers").
