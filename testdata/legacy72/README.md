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
