# The multiprocessing program's last probe

What Phases 45 to 48 chose without a manual or a VMS run to settle it
(docs/DEVIATIONS.md, "[Phases 43–48] Multiprocessing rules chosen
without a manual or probe", and the phases' carry-forward sections). It
runs with round 7 of the macro probes, on one volume
(`../final/README.md`).

| File | What it holds |
| ---- | ------------- |
| `probe5.mar` | The program: steps 1 to 8, each line `n what: result` |
| `p5sleep.mar` | A process that hibernates until it is deleted |
| `p5lock.mar` | A process that takes P5VAL in EX with a value block, then hibernates |
| `p5info.mar` | A process that writes what it inherited to P5_INFO.LOG |
| `probe5.com` | Builds and runs them, then asks DCL steps 9 to 13 |
| `p5args.mar`, `p5args.com` | Step 8 again, for the services the first run didn't reach |
| `vax/` | The VMS run's log, `probe5.log` (2026-10-08), and the second run's, `p5args.log` (not yet run) |

`TestProbe5` (`internal/console`) runs the program under govax;
`go test ./internal/console -run TestProbe5 -v` prints govax's report
and P5_INFO.LOG, to set beside VMS's. Running it already found one bug
(`$EXPREG` called with fewer than four arguments failed in Go).

## The steps

1. A `$CREPRC` subprocess's PRI, PRIB, DFWSCNT, WSQUOTA, and ASTLM, read
   before it has run, and the creator's own (govax: PRI and PRIB 2, the
   manual's default base priority, DFWSCNT 512, WSQUOTA 1024, and ASTLM
   24, the PQL defaults; Phase 45's "+2 above its base" and "+4 pages"
   were seen once on VMS 7.1).
2. The PID of a process made again with the same name after the first
   is deleted (govax: the same slot, its sequence number one higher).
3. What a subprocess inherits (P5_INFO.LOG): the default device and
   directory, SYS$DISK in its process table, and a supervisor-mode name
   its creator defined (govax: the default directory and SYS$DISK, not
   the name).
4. `$ENQW` with LCK$M_NOQUEUE that conflicts: R0 and the LKSB (govax:
   SS$_NOTQUEUED in both, the lock ID left alone).
5. `$ENQW` PR with LCK$M_VALBLK after a process holding the resource in
   EX is deleted (govax: SS$_NORMAL; VMS may say SS$_VALNOTVALID).
6. `$OPEN` of a mailbox: FAB$W_MRS afterwards, 1234 before (govax: left
   alone).
7. `$ERASE` of a file another FAB has open for writing, and whether it
   is still there once the writer has closed it (govax: the erase is
   refused with RMS$_FLK).
8. For each of 41 services harmless with all arguments zero, the
   smallest argument count it doesn't refuse with SS$_INSFARG (govax's
   minimums are the manual's required arguments, or none).
9. DCL: `R` for RUN, `MC` for MCR, `EXI` and `LO` in a subprocess, and
   `$STATUS` after each (govax's subprocess CLI takes these
   abbreviations).
10. SHOW LOGICAL of a name with no translation: `$STATUS` (govax's CLI:
    SS$_NORMAL with STS$M_INHIB_MSG).
11. RUN of a file that isn't an image: DCL's messages and `$STATUS`
    (govax: STS$M_INHIB_MSG on every activation failure).
12. The access modes the copied process logical names keep in a spawned
    subprocess (SHOW LOGICAL/FULL; govax: their own).
13. OPA0:'s and NLA0:'s DEVCHAR, DEVTYPE, and buffer size, SHOW
    DEVICE/FULL of each and of MBA1:, SHOW DEVICE of a prefix (MB) and
    of no device (govax's TTA0: has 0C040007).

## What VMS answered (2026-10-08)

| Step | VMS 7.3 | govax now |
| ---- | ------- | --------- |
| 1 | child PRI 5 (base 2), DFWSCNT 516, WSQUOTA 1028, ASTLM 24; SYSTEM's ASTLM 50 | the working set's 4 pages and ASTLM 50 adopted; the child's priority (3 above its base; probe 1 saw 2) fits no rule, so it stays at its base: unconfirmed |
| 2 | 00000137, then 00000138: the next slot | the next slot after the last one given |
| 3 | default directory and SYS$DISK inherited, the creator's process names not | the same |
| 4 | SS$_NOTQUEUED; the LKSB untouched | the same (it wrote the status before) |
| 5 | SS$_NORMAL: the value block not marked invalid | the same |
| 6 | FAB$W_MRS 0 | the same (it left 1234) |
| 7 | RMS$_FLK; the file still there | the same |
| 8 | $ADJWSL 2, $ALLOC 4, $ASCEFC 4; then $ASCTIM signaled an access violation | the three minimums adopted; the rest is `p5args` |
| 9 | `R`, `MC` (MCR looks in SYS$SYSTEM), `EXI 5` (status 5), `LO`; a spawned LOGOUT writes `  Process SYSTEM_1 logged out at ...` | the logout line adopted |
| 10 | NOTRAN's `$STATUS` %X10788019 | the same, in the subprocess CLI |
| 11 | `-CLI-E-IMGNAME, image file DUA1:[000000]PROBE5.MAR;1`, `-IMGACT-F-BADHDR`, %X104D8C84 | the same, in the subprocess CLI |
| 12 | the subprocess has the supervisor-mode names in their mode, not the executive one; SHOW LOGICAL/FULL's header shows the table's mode and "[no protection information]" | the same; the header adopted |
| 13 | OPA0: DEVCHAR 0C040087 (an operator terminal; TTA0:'s 0C040007 is that less OPR), NLA0: as govax has it; SHOW DEVICE's brief layout; NOSUCHDEV | the brief layout adopted |
