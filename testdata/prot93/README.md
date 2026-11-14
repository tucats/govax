# The Phase 93 probe: file protection

These fixtures are `docs/planning/PHASE-93 - file protection.md`'s subtask
2. They ask VMS 7.1 what file protection does: what each identity may do
to files and directories through RMS, the ACP, and DCL's commands; what
owner, protection, and ACL a new file gets; how SET SECURITY and its older
forms change them; how they are shown; and what the messages are.

| File | What it holds |
| ---- | ------------- |
| `prot93.mar` | The program, by hand: takes one case per line from SYS$INPUT and reports what RMS or the ACP did |
| `gen.go` | Writes everything below but `decode.go`: `go run testdata/prot93/gen.go` from the repository root |
| `prot93.com` | SYSTEM's run: makes the accounts and the tree, then SYSTEM's parts |
| `prot93u.com` | Each account's run |
| `prot93b.com` | Part B, DCL's utilities, called by both of the above |
| `prot93z.com` | The listing at the end, then the accounts removed |
| `prot93c.dat`, `prot93d.dat`, `prot93e.dat` | The program's cases: every pass's, each identity's changes, and the ACL dumps |
| `hello.mar` | A small image Part B runs under different protections |
| `def_*.mar` | `$CHPDEF`, `$OSSDEF`, `$ACEDEF`, `$ACLDEF`, `$ARMDEF`, `$KGBDEF`, `$PRVDEF`, each `GLOBAL`, for their values |
| `decode.go` | Turns the VAX's `def_*.obj` into `defined.txt` (values only), for round 2's services |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |
| `defined.txt` | The definitions' values, from `decode.go` |
| `vax/` | What round 1 left: the five logs, `prot93.exe`, and the definition objects |
| `prot93r.com`, `prot93v.com`, `prot93y.com` | Round 2: SYSTEM's run, each account's, and the end |
| `prot93f.dat`, `prot93g.dat` | Round 2's cases: a directory without execute, and the new changing cases |
| `exchange2.cmd`, `copyout2.cmd` | Round 2's govax scripts: its files onto round 1's volume, and its logs back |
| `vax/round2/` | What round 2 leaves: its five logs |
| `gen3.go` | Round 3's generator, which writes the files below: `go run testdata/prot93/gen3.go` from the repository root |
| `sec93.mar` | Round 3's program: 588 cases of the security services, each a routine of its own under a handler |
| `sec93.com`, `sec93u.com`, `sec93x.com`, `sec93y.com` | Round 3: SYSTEM's run, PROT93C's, the risky cases one at a time, and the end |
| `sec93t.dat` | The text of every file round 3's tree copies |
| `exchange-r3.cmd`, `copyout-r3.cmd` | Round 3's govax scripts: a new volume, and its results back |
| `sec93b.com`, `exchange-r3b.cmd`, `copyout-r3b.cmd` | Round 3b: parts P, F, and C again (README's "Round 3's results, and round 3b") |
| `vax/round3/` | What round 3 leaves: its logs, the program's map, object, and image; `sec93-crash.log`, what the first run's log held when VMS crashed |

## The identities

The run uses three accounts, which `PROT93.COM` makes with AUTHORIZE and
`PROT93Z.COM` removes:

| Account | UIC | Role | Privileges it may turn on |
| ------- | --- | ---- | ------------------------- |
| PROT93A | `[200,1]` | owns most of the tree | none |
| PROT93B | `[200,2]` | the owner's group | READALL, GRPPRV, SYSPRV, BYPASS |
| PROT93C | `[300,1]` | the world | READALL, GRPPRV, SYSPRV, BYPASS |

The password of each is `PROT93PW`, and it doesn't expire. Their default
directory is `PROT93$DISK:[PROT93LOG]`. AUTHORIZE also adds the group
identifiers PROT93G2 (`[200,*]`) and PROT93G3 (`[300,*]`), and a general
identifier, PROT93_TEAM, held by PROT93C. If the VAX already uses UIC
groups 200 or 300, say so before running it.

SYSTEM runs too, in three passes: without READALL, GRPPRV, SYSPRV, and
BYPASS (`M0`); with READALL alone (`M1`); and with all four (`M4`). PROT93B
and PROT93C each run five passes: none, then each of the four alone. Each
pass has a tag (`A0`, `B3`, `M4`, ...), which the program puts in place of
each `@` in a case, so each pass makes files of its own.

## The tree

Everything is on the exchange volume, under `[PROT93]`, owned mostly by
`[200,1]`:

- `[.F]` is the access matrix: a file for each combination of categories,
  access types, owner (UIC 0 too), and ACL.
- `[.DR]`, `[.DE]`, `[.DW]`, `[.DRW]`, `[.DN]`, `[.DWO]` each give everyone
  one kind of access to the directory, and hold an `A.DAT` anyone may use.
- `[.PX1.PY.PZ]` to `[.PX5.PY.PZ]` each take one level's access away.
- `[.NV]`, `[.ER]`, `[.ERX]`, `[.RN]`, `[.RNX]`, `[.CTL]`, `[.OWN]`, `[.DP]`
  are for new versions, deletes, renames, changes of protection and owner,
  ownership of new files, and a directory with a default protection ACE.
- `[.U]` and its neighbors, `[.IMG]`, are Part B's: the utilities' inputs,
  each identity's files to change, the images, and the procedures.
- `[.PRF]`, `[.DEFP]`, `[.SEC]`, `[.ACE]` are SYSTEM's: new files' profiles,
  SET PROTECTION/DEFAULT, SET SECURITY, and one file per kind of ACE.

The procedure also sets the exchange volume's own protection and its
MFD's so the accounts can reach it.

## The program's cases

A line is `OF F1|F2|F3|F4|F5`. `prot93.mar`'s opening comment lists the
operations: `O` opens with the access named (G, P, U, D, T, B, R), `I`
opens by file ID, `A` opens and shows the XABPRO (owner, protection, and
the ACL's bytes), `C` creates (with a XABPRO, a supersede, or CIF), `E`
erases, `R` renames, `P` parses, `S` searches, and `Q` calls the ACP (read
attributes, access for read or write, modify the protection and owner,
delete, create). Each case prints its line after `==`, then the status and
STV of each service, and what it found.

## The VAX run

The run changes the VAX in three ways, all undone by `PROT93Z.COM`: the
three accounts and their identifiers in SYSUAF and RIGHTSLIST, and the
system logical name `PROT93$DISK`. Everything else it does is on the
exchange volume. No procedure waits for input.

1. **Build the exchange volume**, from the repository root on the host:

       rm -f testdata/disks/prot93-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange.cmd

   This makes `testdata/disks/prot93-exchange.dsk` (RD53, label PROT93,
   gitignored).

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt (the full path if simh wasn't started in the repository root):

       attach rq1 testdata/disks/prot93-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, mount it for every user, and run SYSTEM's part:

       $ MOUNT/SYSTEM DUA1: PROT93
       $ @DUA1:[000000]PROT93

   It writes its log to `PROT93$DISK:[PROT93LOG]PROT93.LOG` and ends by
   saying so; this one takes the longest (it makes some 400 files and
   directories and runs the program's cases three times). The log's last
   lines are
   `PROT93 FINISHED`. If MACRO or LINK failed, it stops after the BUILD
   section, and `PROT93.MLG` and `PROT93.LLG` in `[000000]` say why. The
   AUTHORIZE section of the log shows the accounts being made.

4. **Log out, and log in as PROT93A** (password `PROT93PW`), then:

       $ @PROT93$DISK:[000000]PROT93U
       $ LOGOUT

   Its log is `PROT93$DISK:[PROT93LOG]PROT93U_PROT93A.LOG`, ending
   `PROT93U FINISHED`. If VMS asks for a new password at login, give it
   any; nothing depends on it.

5. **The same as PROT93B, then as PROT93C**, in that order (each account's
   run sees what the earlier ones did):

       $ @PROT93$DISK:[000000]PROT93U
       $ LOGOUT

6. **Log in as SYSTEM** and run the end:

       $ @PROT93$DISK:[000000]PROT93Z

   It lists the whole tree (`PROT93Z.LOG`), removes the accounts and the
   identifiers, and deassigns `PROT93$DISK`. Then dismount the volume and
   detach it:

       $ DISMOUNT DUA1:

       sim> detach rq1

7. **Copy the results back**, on the host, from the repository root:

       mkdir -p testdata/prot93/vax
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout.cmd
       go run testdata/prot93/decode.go

   The first writes `vax/prot93.log`, `vax/prot93u_prot93a.log`, `_b`,
   `_c`, `vax/prot93z.log`, `vax/prot93.exe`, and `vax/def_*.obj`; the
   second writes `defined.txt`. Keep `testdata/disks/prot93-exchange.dsk`
   too: its files' headers are what VMS made, and the tests will read
   them.

If a run stops partway, run `@PROT93$DISK:[000000]PROT93Z` as SYSTEM to
remove the accounts, then start again from step 1 with a new volume.

## Round 1's results

Round 1 ran on VMS V7.1 on 2026-11-14, and the phase doc's A.4 is written
from it. Two of the probe's own mistakes, left in its procedures as they
ran: `PROT93.COM` set PROT93.EXE's and HELLO.EXE's protection in one SET
SECURITY with a comma list, which 7.1 refuses (`%DCL-W-NOLIST`), so the
accounts couldn't run the program without a privilege, and their passes
without privileges and with GRPPRV failed at activation; and each
`PROCx.COM` was made by a CREATE whose data the next `$` line ended, so the
procedures are empty. `PROT93Z.COM` also left the group identifier PROT93
(`[200,177777]`, which AUTHORIZE made for PROT93A's group) on the VAX.
Round 2 makes up for all three.

## Round 2

Round 2 runs the lost passes on round 1's volume, whose files for them are
untouched, with cases round 1 left open (the phase doc's Part D). It needs
the accounts again, which `PROT93R.COM` makes and `PROT93Y.COM` removes,
with the PROT93 identifier round 1 left. No procedure waits for input.

1. **Keep a copy of round 1's volume**, then add round 2's files to it,
   from the repository root:

       cp testdata/disks/prot93-exchange.dsk testdata/disks/prot93-round1.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange2.cmd

2. **Attach it** to the simh VAX as before (`attach rq1
   testdata/disks/prot93-exchange.dsk`).

3. **As SYSTEM:**

       $ MOUNT/SYSTEM DUA1: PROT93
       $ @DUA1:[000000]PROT93R

   Its log is `PROT93$DISK:[PROT93LOG]PROT93R.LOG`, ending `PROT93R
   FINISHED`.

4. **Log in as PROT93A** (password `PROT93PW`, as before), then:

       $ @PROT93$DISK:[000000]PROT93V
       $ LOGOUT

   Its log is `PROT93V_PROT93A.LOG`, ending `PROT93V FINISHED`. Log in
   anew for it: its first passes must run in a new process.

5. **The same as PROT93B, then as PROT93C.**

6. **As SYSTEM:**

       $ @PROT93$DISK:[000000]PROT93Y
       $ DISMOUNT DUA1:

       sim> detach rq1

   It lists the tree, then removes the accounts, PROT93_TEAM, and PROT93,
   and deassigns `PROT93$DISK`.

7. **Copy the logs back**, from the repository root:

       mkdir -p testdata/prot93/vax/round2
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout2.cmd

**The first try** stopped at step 4: exchange2.cmd's copies have SYSTEM's
default protection, `(RWED,RWED,RE,)`, which PROT93R.COM didn't open, so
the accounts were refused PROT93V.COM (`insufficient privilege`), and
nothing ran under them. PROT93R.COM's and PROT93Y.COM's logs from it are
kept. To go on from there, without running PROT93R.COM again:

1. Add PROT93S.COM to the volume, from the repository root:

       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange3.cmd

2. Attach it, and as SYSTEM:

       $ MOUNT/SYSTEM DUA1: PROT93
       $ @DUA1:[000000]PROT93S

   It defines `PROT93$DISK` again, makes the accounts again, and gives
   each file the accounts read world RE, showing each. Its log is
   `PROT93S.LOG`, ending `PROT93S FINISHED`.

3. Go on with steps 4 to 7 above (PROT93V.COM as each account, PROT93Y.COM
   as SYSTEM, copyout2.cmd, which also brings PROT93S.LOG back).

**Round 2's results** came back on 2026-11-14 (`vax/round2/`), and the
phase doc's A.4 has its answers. PROT93S.COM opened the `PRO2x.COM`
procedures to world RE along with the files the accounts needed, so they
no longer test an execute-only procedure; that question stays open.

If round 2 stops partway, run `@PROT93$DISK:[000000]PROT93Y` as SYSTEM,
copy `prot93-round1.dsk` back over `prot93-exchange.dsk`, and start again
from step 1's second command.

## Round 3

Round 3 asks VMS 7.1 about the security services (the phase doc's subtasks
10 and 13): `$PARSE_ACL` and `$FORMAT_ACL` (an ACE's text both ways, the
forms each accepts, its errors and their positions, widths, terminators,
indentation, and access names), `$CHANGE_ACL` (adding, finding, matching,
reading, and deleting ACEs, contexts, channels, errors), `$CHKPRO` (a third
party's check by item list: categories, privileges, rights, ACLs, modes,
and what it reports), `$CHECK_ACCESS` (a named user's access to a file),
`$GET_SECURITY` and `$SET_SECURITY` (every item, contexts, handles, class
names, object names), and `$SETDFPROT`. It also asks the question rounds 1
and 2 left open: whether an execute-only command procedure's lines run, and
with what verification (`[SEC93.EXO]`, run by PROT93C). And it runs round 9
of the system service macro probes (`testdata/mp/macros`, the security
services' macros) on the same volume.

`gen3.go` writes it all. The program's cases are data and code it writes
itself, as `testdata/lib53`'s are: each ACE a case gives a service is
written in bytes by the layouts rounds 1 and 2 found, so no case depends on
`$PARSE_ACL` but its own. Each case prints `== n part title`, then each
service's R0 and what it returned: an output's return length and bytes in
hex, an ACL's ACEs each in hex and as `$FORMAT_ACL` writes it, and, after a
change, the file's profile through RMS's `$DISPLAY` (`file owner=...`). A
condition a case signals is caught and shown (`signal`), and the next case
runs. `SEC93_PART` picks the parts, a letter each (`sec93.mar`'s opening
comment lists them), and `SEC93_ONLY` one case by its number.

**The first run crashed VMS.** `SEC93.COM`'s program run took the VAX
down (a dump and a reboot) in part C. Its log's end of file was never
written, but its blocks were on the volume, and `vax/round3/sec93-crash.log`
is what they held: the build, parts P and F whole, and part C to case 211.
The volume showed the rest: `[SEC93.CHG]CA4.DAT` had its first six ACEs and
`CA5.DAT` none of its cases', so the crash came in CA4's last cases, the
addition of an identifier ACE with a reserved longword, of a HIDDEN ACE, or
the read after them. That run also found a mistake of the probe's own:
SET SECURITY/PROTECTION refuses a category written with a colon and no
letters (`W:`, `%DCL-W-VALREQ`), so the files given one kept SYSTEM's
owner and default protection; `gen3.go` now writes such a category without
its colon. Every case that gives a service malformed or unusual input
(21) is now part X, which `SEC93X.COM` runs last, one case to an image,
each with its own log, `SEC93X_n.LOG`, the two the crash points to last.
A case whose log is there already is passed over, so after another crash
`@SEC93X` goes on from the case after the one that crashed, whose log is
there and empty.

**The second run crashed VMS too**, in the same place, and its console
named the cause: a bugcheck, `SECAUDERR, Fatal error attempting to perform
a security audit`, in the XQP, after OPCOM's security alarms for each read
of `CA4.DAT`. CA4's cases put alarm ACEs on it; the one before the crash
names a journal other than SECURITY, `(ALARM=JOURNAL1,ACCESS=READ+SUCCESS)`,
and the first access after it was added (the file's display) brought VMS
down. That case is now part X's last, `SEC93X.COM` running it only when
asked (`@SEC93X CRASH`). The alarm ACEs naming SECURITY stay in part C: they
put a message on the console for each access to `CA4.DAT`, which is
expected.

Round 3 runs on a volume of its own, not round 1's, with the accounts made
again (PROT93_TEAM at round 1's value, which the cases' ACEs hold). The
cases' files are made by `SEC93.COM` on the VAX, each with its owner,
protection, and ACL. No procedure waits for input.

0. **After the first run's crash**, the accounts it made are still on
   the VAX: mount the old volume /SYSTEM and run `@DUA1:[000000]SEC93Y` as
   SYSTEM to remove them (its listing will fail without `PROT93$DISK`;
   the AUTHORIZE part is what matters), then dismount and detach it. Keep
   a copy of it if you like; `vax/round3/sec93-crash.log` already holds
   what it had.

1. **Build the exchange volume**, from the repository root:

       rm -f testdata/disks/sec93-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange-r3.cmd

2. **Attach it** to the simh VAX on a free unit (`attach rq1
   testdata/disks/sec93-exchange.dsk`).

3. **As SYSTEM:**

       $ MOUNT/SYSTEM DUA1: SEC93
       $ @DUA1:[000000]SEC93

   Its log is `PROT93$DISK:[PROT93LOG]SEC93.LOG`, ending `SEC93 FINISHED`.
   It makes the accounts, assembles and links the program (with a full
   map, which shows where each service is: some may be in SECURESHR.EXE
   rather than the system service vector), makes the tree, runs the
   program's parts with every privilege and again without READALL,
   GRPPRV, SYSPRV, and BYPASS, and assembles round 9 of the macro probes in
   `[SEC93MAC]`. The program is long (some 26,000 lines), so MACRO takes a
   while.

4. **Log in as PROT93C** (password `PROT93PW`), a new login, then:

       $ @PROT93$DISK:[000000]SEC93U
       $ LOGOUT

   Its log is `SEC93U.LOG`, ending `SEC93U FINISHED`: the program's part
   U as the world, then the execute-only procedures with verification on
   and off, nested, reading data, failing, with `/OUTPUT`, and read as files.

5. **As SYSTEM**, the risky cases:

       $ @PROT93$DISK:[000000]SEC93X

   Its log is `SEC93X.LOG`, ending `SEC93X FINISHED`, and each case's is
   `SEC93X_n.LOG`. If the VAX crashes, reboot, `MOUNT/SYSTEM DUA1: SEC93`,
   and run `@DUA1:[000000]SEC93X` again (it defines `PROT93$DISK` itself);
   repeat until it finishes. Say which cases crashed it: their logs are
   the empty ones. The last case, the JOURNAL1 alarm, runs only with
   `@PROT93$DISK:[000000]SEC93X CRASH`; it is expected to crash VMS, so
   run it, if at all, after step 6's SEC93Y.COM has run and the logs are
   copied out, then reboot and copy its log too.

6. **As SYSTEM:**

       $ @PROT93$DISK:[000000]SEC93Y
       $ DISMOUNT DUA1:

       sim> detach rq1

   It lists the tree, removes the accounts, PROT93_TEAM, and PROT93, and
   deassigns `PROT93$DISK`.

7. **Copy the results back**, from the repository root:

       mkdir -p testdata/prot93/vax/round3
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout-r3.cmd

   It writes `vax/round3/` (the logs, `sec93.map`, `sec93.obj`,
   `sec93.exe`) and round 9's objects, analyses, and log in
   `testdata/mp/macros/vax/`.

If round 3 stops partway, run `@PROT93$DISK:[000000]SEC93Y` as SYSTEM, then
start again from step 1.

**On govax** the round was rehearsed before the VAX run: the program
assembles and links with govax's MACRO and LINK (`SYS$PARSE_ACL`,
`SYS$FORMAT_ACL`, `SYS$CHANGE_ACL`, and `SYS$CHECK_ACCESS` undefined, so
their cases end in an access violation, and the services govax lacks
signal `SS$_UNSUPPORTED`), every case runs, and each procedure runs to its
end. What fails there is what this phase has yet to do (SET SECURITY, SHOW
SECURITY, SET VOLUME, the services), and AUTHORIZE, SEARCH, and the
security services' macros. The rehearsal found that govax's MACRO writes
its object beside its source where VMS writes it in the current default
directory, so `SEC93.COM` names the object.

## Round 3's results, and round 3b

The third try ran whole (2026-11-14; `vax/round3/`: `sec93.log`,
`sec93u.log`, `sec93x.log` and each part X case's `sec93x_n.log`,
`sec93y.log`; round 9's macro objects are in `testdata/mp/macros/vax/`).
One case of the probe's own spoiled a stretch of it: case 80 gave
`$PARSE_ACL` a descriptor of 65,535 bytes over a 256-byte buffer, and the
service wrote over the 64K of the program's data after it, so cases 81 to
204 (the rest of part P, all of part F, and part C to case 204) ran on
zeros, their titles blank. The first run's recovered log, the same program,
lost the same cases. Everything else is whole: case 205 on, the pass
without privileges, PROT93C's part U, and part X, each its own image. The
other mistake of the probe's own: `$CHANGE_ACL` by channel opened its file
for write with read sharing, which RMS refused (`RMS$_SHR`), so cases 229
and 230 asked nothing; they now open it for read, as part G's handles did.

**Round 3b** runs parts P, F, and C again, with case 80 given a buffer as
big as its descriptor (filled first, to show what the service writes) and
the channel fixed. The case numbers are round 3's. `SEC93B.COM` is
`SEC93.COM`'s accounts, build, and tree, then the three parts.

1. Build a new volume, from the repository root:

       rm -f testdata/disks/sec93-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange-r3b.cmd

2. Attach it, and as SYSTEM:

       $ MOUNT/SYSTEM DUA1: SEC93
       $ @DUA1:[000000]SEC93B
       $ @PROT93$DISK:[000000]SEC93Y
       $ DISMOUNT DUA1:

   `SEC93B.LOG` ends `SEC93B FINISHED`; `SEC93Y.COM` removes the accounts.
   Part C's alarm ACEs put security alarms on the console, as before.

3. Copy the logs back:

       mkdir -p testdata/prot93/vax/round3b
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout-r3b.cmd

## What the run is expected to show

Errors in the logs are the probe's point: most cases are there to see
what VMS refuses, and how it says so. `SET NOON` lets each procedure go on
past them.

## On govax

`gen.go`'s procedures were rehearsed on govax before the VAX run (as
SYSTEM, which is all govax has yet): the program assembles, links, and
runs every case file under each tag, and each procedure runs to its end.
What govax lacked then, and so failed in the rehearsal, is this phase's
work: SET SECURITY, SHOW SECURITY, SET PROTECTION (both forms), SHOW
PROTECTION, SET FILE/PROTECTION, SET ACL, SET PROCESS/PRIVILEGES, SHOW
PROCESS/PRIVILEGES, SET VOLUME, DIRECTORY/SECURITY and /ACL, the ACP's
`ATR$C_ACLLENGTH` and IO$_MODIFY by name, CREATE/PROTECTION and
/OWNER_UIC, and COPY/PROTECTION; and, outside it, AUTHORIZE, APPEND,
SEARCH, DIFFERENCES, ANALYZE/RMS_FILE/OUTPUT=NL:, and the `def_*.mar`
macros (VMS's own, which only the VAX assembles).
