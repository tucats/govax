# The Phase 72 probe: a VMS V1 image

These fixtures are `docs/PHASE-72 - legacy images.md`'s subtask 2. They
ask VMS how it handles DUNGEON.EXE, a FORTRAN program linked by VMS V1's
linker on 7-DEC-1978 (image format 02.01), which runs on VMS V7:

- what ANALYZE/IMAGE reports for a 02.01 header, and for VMSRTL.EXE,
  the shareable image DUNGEON's three global section ISDs name
  (VMSRTL_001, _002, _003, at fixed addresses from ^X1A600);
- whether VMSRTL is installed, and under which global section names;
- where the image activator puts VMSRTL, and the images it uses, for
  such an image (the debugger's SHOW IMAGE), and whether the old
  VMSRTL_002 and _003 ranges are mapped at all;
- the old system service vector in S0 (^X80000000): what is at
  SYS$IMGSTA's entry (^X80000168), which DUNGEON's startup stub jumps
  to, and SYS$NUMTIM's (^X800001B8), which DUNGEON calls;
- VMSRTL's transfer vector entries where DUNGEON calls them (^X1A600
  to ^X1A830);
- the debugger on a V1 image: its INITIAL message, language, radix,
  SHOW CALLS, SHOW MODULE, and STEP;
- the game's output for a few moves, from DUNGEON and from
  DUNGEON_V552 (the same game linked on VMS 5.5-2), for govax to be
  checked against.

| File | What it holds |
| ---- | ------------- |
| `legacy72.com` | The procedure: ANALYZE/IMAGE of both images, INSTALL LIST, the debugger session, and the two games |
| `dun72.dbg` | The debugger's commands, logged to `DUN72.DLG` |
| `exchange.cmd` | The govax console script that builds the exchange volume, copying the game from the work volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `dungeon-hdr.bin` | DUNGEON.EXE's header block alone (no code), for `internal/anl`'s `TestLegacyImagePages` |
| `legacy72b.com`, `dun72b.dbg` | Round 2: INSTALL's global sections, and the debugger's EXAMINEs with the radix set to hexadecimal |
| `exchange2.cmd`, `copyout2.cmd` | Round 2's govax console scripts |

## The VAX run

1. Build the exchange volume, from the repository root, with govax's
   default configuration (its work volume, DUA0, holds
   `[WORK.DUNGEON]`):

       govax console < testdata/legacy72/exchange.cmd

   This makes `testdata/disks/legacy72-exchange.dsk` (RD51 size, label
   LEGACY72, gitignored) holding `LEGACY72.COM`, `DUN72.DBG`,
   `DUNGEON.EXE`, `DUNGEON_V552.EXE`, `DINDX.DAT`, and `DTEXT.DAT`.
2. Attach it to the simh VAX and mount it. Log in to a privileged
   account (SYSTEM: INSTALL LIST needs CMKRNL), set the volume's
   `[000000]` as the default directory, and run:

       @LEGACY72/OUTPUT=LEGACY72.LOG

   Nothing should wait for input. The debugger reads its commands from
   a file (`DBG$INPUT`), and the games read their moves from the
   procedure's data lines. If a game hangs, a Ctrl/Y and `STOP` ends
   it, and the logs up to there are still useful. Expect some commands
   to fail (`SHOW LOGICAL VMSRTL` when there's no such name, and
   EXAMINEs of pages that aren't mapped): those failures are answers
   too.
3. With simh paused (or the disk detached), copy the results back,
   from the repository root:

       govax console < testdata/legacy72/copyout.cmd

   That copies `LEGACY72.LOG`, `DUN72.DLG`, `DUNGEON.ANI`,
   `VMSRTL.ANI`, and `GLOBAL.LIS` into `vax/`. VMSRTL.EXE and the
   game's images don't come back: they aren't the probe's.

Nothing here is DIGITAL's: the procedure and the debugger's commands
are written for the probe, and the game's files come from the author's
volume.

## Round 1's results (2026-10-24, VMS V7.1, VAX 8600)

`vax/legacy72.log`, `vax/dun72.dlg`, `vax/dungeon.ani`, and
`vax/vmsrtl.ani`. What went wrong:

- `INSTALL LIST` has no `/OUTPUT`, so `GLOBAL.LIS` wasn't made.
- The VMS debugger refused every `%X` address (`%DEBUG-E-MISOPEMIS`):
  its hexadecimal prefix is `%HEX`.
- Both games wrote to FORTRAN unit 5, which in a procedure is
  SYS$INPUT, and stopped with `%FOR-F-ERRDURWRI` (RMS-F-FAC) at their
  first message. The games have to be played at a terminal.

## Round 2

1. With the volume detached from simh, add round 2's files, from the
   repository root:

       govax console < testdata/legacy72/exchange2.cmd

2. On the VAX, logged in to SYSTEM, with the volume's `[000000]` the
   default directory:

       @LEGACY72B/OUTPUT=LEGACY72B.LOG

3. **The games, at the terminal**, with the same default directory:

       DEFINE DINDX DINDX.DAT
       DEFINE DTEXT DTEXT.DAT
       RUN DUNGEON

   then type these moves, one per prompt:

       LOOK
       OPEN MAILBOX
       TAKE LEAFLET
       READ LEAFLET
       INVENTORY
       SCORE
       QUIT
       Y

   Then `RUN DUNGEON_V552` and the same moves. Save the screen, from
   the first `RUN` to the last prompt, as
   `testdata/legacy72/vax/dungeon-term.log`. If a move isn't
   understood or the game asks something else, answer as you like and
   note it.
4. Detach the volume and copy the results back:

       govax console < testdata/legacy72/copyout2.cmd

## Round 2's results (2026-10-24)

`vax/legacy72b.log`, `vax/dun72b.dlg`, `vax/global.lis` (INSTALL's
global sections), and `vax/dungeon-term.log` (both games played at the
terminal, the author's screen). `docs/PHASE-72 - legacy images.md`,
Part A, answers 8 to 12, says what they show.

