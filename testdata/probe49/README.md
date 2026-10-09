# Phase 49's probe

What Phase 49's subtasks chose without a manual to settle it
(`docs/PHASE-49 - record updates and locks.md`, each subtask's
"Unconfirmed" items). One program, `probe6.mar`, in one process: two
streams on one file stand in for two processes where record locks are
asked about, and a process's lock on itself stands in for a deadlock.

| File | What it holds |
| ---- | ------------- |
| `probe6.mar` | The program: steps 1 to 9, each line `n what: result` |
| `probe6.com` | Builds and runs it; MACRO's and LINK's output to their own logs |
| `exchange.cmd`, `copyout.cmd` | Make the exchange volume and copy the logs back |
| `probe6b.mar`, `probe6b.com`, `exchange2.cmd`, `copyout2.cmd` | Round 2: what round 1 left open |
| `vax/` | The VMS runs' logs: `probe6.log` (2026-10-09), and round 2's once it has run |

`TestProbe6` and `TestProbe6b` (`internal/console`) run the programs under govax;
`go test ./internal/console -run TestProbe6 -v` prints govax's report,
to set beside VMS's. Writing it already found one bug: `$OPEN` refused
a FAB whose only access was DEL or TRN.

## The steps

Statuses are hexadecimal; govax's answers (2026-10-09) are in
parentheses.

1. `$UPDATE` of a record to another length, in a variable-length file
   and in a stream file (RMS$_RSZ for both).
2. `$GET` by an RFA one byte into the first record (RMS$_RFA).
3. `$OPEN` and `$GET` in a stream whose only access is UPD, and one
   whose only is TRN (both succeed).
4. Two streams on one shared file, R1 R2 R3:
   - a: B's `$GET` of the record A has locked (RMS$_RLK, the baseline);
   - b: A's `$PUT` with RAB$V_TPT where R2 was, and B's `$GET` of the
     record it put, by the RFA the `$PUT` returned: does `$PUT` lock it?
     (it doesn't);
   - c: A reads R1 and the next record with ULK and `$TRUNCATE`s at the
     second; B's `$GET` of R1: does `$TRUNCATE` keep A's locks? (it
     does);
   - d: `$UPDATE` of a record read with RAB$V_NLK (RMS$_RNL);
   - e: B's `$GET` of a locked record with WAT and TMO of 0 and of 2
     seconds: the status and how long it took (RMS$_TMO, after about
     0 and 2 s).
5. `$GETLKI`:
   - a: from the wildcard lock ID -1, holding two locks: each call's
     status, the context it writes back, and the lock ID (the high bit
     and the last lock ID; SS$_NOMORELOCK at the end);
   - b: LKI$_STATE of an EX request waiting behind another: the granted
     mode a waiting lock reports (NL, 0);
   - c: LKI$_LOCKS of a resource with two locks into a 30-byte buffer:
     the return length longword (one whole 24-byte entry, the entry size
     in bits 16-30, bit 31 for the short buffer: 80180018).
6. A process holding a resource in EX asks for it in EX again: is that
   found as a deadlock, and when? A 20-second timer ends the wait if
   not (SS$_DEADLOCK after DEADLOCK_WAIT, about 10 s).
7. File-backed sections:
   - a: a new file, ALQ=10 and nothing written, mapped whole on a user
     file open's channel: the status and the pages mapped (SS$_NORMAL,
     not SS$_CREATED, for a private section; 10 pages, the allocation,
     not the end of file);
   - b: after a write to page 6 and `$DELTVA`, the file's end of file
     and allocation (XABFHC: EBK and FFB unchanged, 0 and 0);
   - c: `$UPDSEC` with nothing modified: the status, the IOSB, the event
     flag, and whether the AST runs (SS$_NOTMODIFIED, in the IOSB too,
     the flag set, no AST);
   - d: `$CRMPSC` on a disk channel with no file accessed
     (SS$_FILNOTACC);
   - e: a global section of a file opened read-only, then `$MGBLSC`
     with SEC$M_WRT (SS$_CREATED, then SS$_NOWRT);
   - f: `$OPEN` of NLA0: with FAB$V_UFO (RMS$_SUPPORT; VMS may give a
     channel).
8. The buffered and direct I/O counts each of these adds (JPI$_BUFIO,
   JPI$_DIRIO): a `$QIOW` write to NLA0:; a LIB$PUT_OUTPUT line (to
   PROBE6.LOG, a file, on VMS); `$CREATE`, ten 80-byte `$PUT`s, and
   `$CLOSE`; `$OPEN`, `$GET` to the end, `$CLOSE`. govax counts the
   `$QIOW` and a line as one buffered I/O each, and ods2's block
   operations as direct I/Os (29 and 16), which won't match RMS's.
9. A wildcard directory search, `[P6*...]P6F.DAT`, of [P6A] and [P6C];
   after its first file, [P6] (behind it in the walk), [P6A.B] and
   [P6B] (ahead) are made: which it finds ([P6A], [P6A.B], [P6B],
   [P6C], then RMS$_NMF).

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/probe49/exchange.cmd

   This makes `testdata/disks/probe49.dsk` (RD53 size, label PROBE49,
   gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the
   default directory, and run, from the SYSTEM account:

       @PROBE6

   Nothing should wait longer than step 6's 20 seconds (step 4e's two
   seconds and step 7c's one aside).
3. Dismount, copy the container back, and run:

       govax console < testdata/probe49/copyout.cmd

   The logs go to `vax/`: `probe6.log` (the report), `p6link.log`, and
   MACRO's.

**Audit MACRO's log before anything reads it.** A MACRO error message
can quote a line of a macro's expansion, so Claude's clean-room hook
(`.claude/hooks/cleanroom.sh`) refuses any tool call that names it
until the author has checked it and taken it off the hook's
`unaudited` list. The report and LINK's log are fine to read.

If MACRO or LINK failed, PROBE6.LOG won't be there: the build logs say
why.

## After the run

`go test ./internal/console -run TestProbe6 -v` prints govax's side;
each difference settles one of the phase doc's unconfirmed items.

## What VMS answered (2026-10-09)

| Step | VMS 7.3 | govax now |
| ---- | ------- | --------- |
| 1 | RMS$_RSZ for the variable-length file; SS$_NORMAL for the stream file | the variable-length case as VMS; the stream case asked again (round 2) |
| 2 | RMS$_EOF: no record-boundary check; a length word read from the byte the RFA names ran past the end | the same |
| 3, 4a to 4d | as govax | unchanged |
| 4e | TMO=0: RMS$_TMO at once; TMO=2: RMS$_RFA at once (the probe didn't set the RFA again after the failure) | asked again (round 2) |
| 5a | context FE0000E6 after lock 1A0000E6: ^XFE over the lock's index; more locks than the probe's own | context ^XFE over the lock ID |
| 5b, 5c, 6 | as govax (granted NL; 80180018; SS$_DEADLOCK after 9 s) | unchanged |
| 7a, 7b, 7c, 7e | SS$_IVSECFLG, then failures that follow from it: 7a's `$CREATE` must have failed (no channel; the probe didn't print its status) | asked again (round 2) |
| 7d | SS$_IVCHNLSEC | the same |
| 7f | RMS$_NORMAL, a channel to NLA0: | the same |
| 8 | NLA0: write: direct; a line to a log file: nothing; create/10 puts/close: 3 buffered, 7 direct; open/gets/close: 2 and 1 | NL: is direct; RMS counts by a model fitted to these totals (`internal/rms/iocount.go`) |
| 9 | as govax | unchanged |

## Round 2

`probe6b.mar` asks again what round 1 left open, printing every status:

1. A stream file's `$UPDATE` of R1 to a longer and to a shorter record,
   then the file read back (govax: RMS$_RSZ, the file unchanged).
4. B's `$GET` by RFA of a record A has locked, with WAT and TMO=0, then
   with its RFA set again and TMO=2: the status, the time, and
   RAB$W_RFA and RAB$B_RAC afterwards (govax: RMS$_TMO, at once and
   after 2 s; the RAB unchanged).
7. a: `$CREATE` with UFO and ALQ=10, shared (and, if that fails, not
   shared): the status and FAB$L_STV; b: `$CRMPSC` of it (govax: 10
   pages); c: after writing page 6, `$DELTVA`, `$DASSGN`, and the end
   of file (govax: EBK 0, FFB 0, HBK 10); d: a file of ten blocks with
   three written, opened UFO and mapped (govax: 10 pages, the
   allocation); e: `$UPDSEC` with nothing modified, then with a page
   modified (govax: SS$_NOTMODIFIED, no AST; SS$_NORMAL, the AST); f:
   the file opened read-only, a global section of it, and `$MGBLSC`
   writable (govax: SS$_CREATED, SS$_NOWRT).

Run it as round 1, with its own volume:

    govax console < testdata/probe49/exchange2.cmd

attach `testdata/disks/probe49b.dsk`, set its `[000000]` as the default,
`@PROBE6B`, then

    govax console < testdata/probe49/copyout2.cmd

The report goes to `vax/probe6b.log`. MACRO's log comes back too; audit
it, as round 1's, before anything reads it.
