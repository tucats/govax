# The Phase 94 probe: privileges

These fixtures are `docs/planning/PHASE-94 - privileges.md`'s subtask 2.
They ask VMS 7.1 what each privilege guards: which services refuse
without it, what they answer instead, whether an executive- or
kernel-mode caller needs it, what a process may see and do to others,
how `$SETPRV` and `$CREPRC` treat the masks, what image rundown leaves,
what an installed image's privileges do, and what DCL's commands say
when a privilege is missing.

| File | What it holds |
| ---- | ------------- |
| `gen.go` | Writes everything below: `go run testdata/priv94/gen.go` from the repository root |
| `priv94.mar` | The program: 289 cases, each a routine under a handler, its body run with exactly the case's privileges, from user, executive, or kernel mode |
| `priv94h.mar` | The holder: a process that hibernates, which the cases act on |
| `priv94i.mar` | The image installed with SYSNAM and OPER: shows its masks and puts a name in the system table |
| `priv94.com` | SYSTEM's run: makes the accounts, builds the programs, makes the fixtures and holders, runs SYSTEM's parts, installs PRIV94I |
| `priv94u.com` | Each account's run |
| `priv94p.com` | What every identity runs: the program, the rundown, DCL's side, the holders ended and made again |
| `priv94d.com` | DCL's side: DEFINE and DEASSIGN in each table and mode, CREATE/NAME_TABLE, SHOW SECURITY of tables, SHOW SYSTEM and SHOW PROCESS of others, F$PID, SET PROCESS/PRIORITY and /SUSPEND of others, SET TERMINAL/PERMANENT, RUN/DETACH, STOP, each under one privilege set after another |
| `priv94z.com` | The end: INSTALL REMOVE, the holders and fixtures deleted, the accounts removed |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |

## The program's cases

Each case prints a line

    == n PART MODE [PRIVILEGES] what it does

then `R0`, the body's status and its message, and what the service did:
a logical name's translation, mode, and table (`lnm`), an I/O status
block (`iosb`), a process's priority (`pri`), a new process's base
priority, UIC, and masks (`child`), the masks after a `$SETPRV`. A case
in executive or kernel mode gets there through `$CMEXEC` or `$CMKRNL`,
then turns off CMEXEC or CMKRNL before its body runs (`inner cur` shows
what the body had). Everything around a body runs with every privilege
the process is authorized for, so each case undoes what it did. Lines
beginning `  ~` hold counts that differ from system to system.

`PRIV94_PART` names the parts to run (the program's opening comment
lists them); the default is every part that leaves the system as it
found it. Part K deletes the holders, and part R leaves OPER enabled
permanently, for the procedure to see what image rundown does.

## The identities

| Who | UIC | Authorized | Holds at login |
| --- | --- | ---------- | -------------- |
| SYSTEM | `[1,4]` | every privilege | every privilege |
| PRIV94 | `[200,3]` | every privilege | TMPMBX, NETMBX |
| PRIV94N | `[200,4]` | TMPMBX, NETMBX | TMPMBX, NETMBX |

`PRIV94.COM` makes the two accounts with AUTHORIZE (password `PRIV94PW`,
not expiring, default directory `PRIV94$DISK:[PRIV94LOG]`, account
PRIV94G, which gives group 200 the identifier PRIV94G if it has none),
and `PRIV94Z.COM` removes them. If the VAX already uses UIC group 200,
say so before running it.

The holders are two detached processes the program makes with `$CREPRC`:
PRIV94_G in group 200 (`[200,1]`, PRIV94's and PRIV94N's group) and
PRIV94_W in group 300 (`[300,1]`). Their PIDs are in the system table
as PRIV94_G_PID and PRIV94_W_PID. No account is needed for their UICs.

## The VAX run

The run changes the VAX in four ways, all undone by `PRIV94Z.COM`: the
two accounts in SYSUAF (and the PRIV94G identifier in RIGHTSLIST), the
system logical names `PRIV94$DISK` and the holders' PIDs, two logical
name tables and a permanent mailbox (the fixtures), and PRIV94I in the
known file list. No procedure waits for input. Some cases send OPCOM
messages and broadcasts ("PRIV94 PROBE MESSAGE, PLEASE IGNORE"), which
appear on the console and on terminals; they need no answer.

1. **Build the exchange volume**, from the repository root on the host:

       rm -f testdata/disks/priv94-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange.cmd

   This makes `testdata/disks/priv94-exchange.dsk` (RD53, label PRIV94,
   gitignored).

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt (the full path if simh wasn't started in the repository root):

       attach rq1 testdata/disks/priv94-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, mount it for every user, and run SYSTEM's part:

       $ MOUNT/SYSTEM DUA1: PRIV94
       $ @DUA1:[000000]PRIV94

   Its log is `PRIV94$DISK:[PRIV94LOG]PRIV94.LOG`, whose last lines are
   `PRIV94 FINISHED`. If MACRO or LINK failed, it stops after the BUILD
   section, and `PRIV94.MLG` and `PRIV94.LLG` in `[000000]` say why.

4. **Log out, and log in as PRIV94** (password `PRIV94PW`), then:

       $ @PRIV94$DISK:[000000]PRIV94U
       $ LOGOUT

   Its log is `PRIV94$DISK:[PRIV94LOG]PRIV94U_PRIV94.LOG`, ending
   `PRIV94U FINISHED`. If VMS asks for a new password at login, give it
   any; nothing depends on it.

5. **The same as PRIV94N** (password `PRIV94PW`):

       $ @PRIV94$DISK:[000000]PRIV94U
       $ LOGOUT

6. **Log in as SYSTEM** and run the end:

       $ @PRIV94$DISK:[000000]PRIV94Z

   Then dismount the volume and detach it:

       $ DISMOUNT DUA1:

       sim> detach rq1

7. **Copy the results back**, on the host, from the repository root:

       mkdir -p testdata/priv94/vax
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout.cmd

   This writes `vax/priv94.log`, `vax/priv94u_priv94.log`,
   `vax/priv94u_priv94n.log`, `vax/priv94z.log`, and the three images.

If a run stops partway, run `@PRIV94$DISK:[000000]PRIV94Z` as SYSTEM to
take everything down, then start again from step 1 with a new volume.
If VMS crashes (a kernel-mode case is the likely cause), note the last
case in the log and say so: the program can run a case alone
(`PRIV94_ONLY`), or skip it.

## The rehearsal

`TestPriv94Probe` (`internal/console/priv94_test.go`) assembles and links
the three programs on govax and runs every part, so the program can't
lose a VAX round to a mistake of its own. The procedures were run on
govax too (as SYSTEM, PRIV94U as SYSTEM, then PRIV94Z), to the end;
what govax lacks of them (AUTHORIZE, INSTALL, RUN/DETACH, SET VOLUME,
SET TERMINAL, SET PROCESS/PRIORITY and /SUSPEND of another process, SHOW
SECURITY of logical name tables, `$SETSWM`) shows as messages, and is
the later subtasks' work.

## Round 1's results

Round 1 ran on VMS V7.1 on 2026-11-15, with no crash; `vax/` holds its
four logs and the three images VMS built. The phase doc's A.4 is written
from them. Its procedures are left as they ran, with two mistakes of
their own: `PRIV94D.COM`'s PRIORITY sections wrote `'P+2'`, which DCL
refuses (`%DCL-W-NOCCAT`), and part Z's sweep deleted names at kernel
mode, which leaves user-mode names alone, so PRIV94_L21, L49, and L51
stayed in the VAX's system table (`$ DEASSIGN/SYSTEM/USER_MODE` removes
them).

## Round 2

Round 2 asks what round 1 left open (the phase doc's A.4, "Open"), on
round 1's exchange volume, as SYSTEM and PRIV94 only. `PRIV94B.MAR`
(written by `gen.go` too, 40 cases on the same machinery) has four parts:

- **M**: permanent mailboxes with PRMMBX, with and without a logical
  name, and with SYSNAM, SYSPRV, or TMPMBX beside it;
- **U**: `$ULKPAG` of a page the program locked, with and without retadr,
  at user mode, in the same body as the lock, with every privilege, and of
  a page never locked;
- **N**: `$CRELNT` under an open shareable table (with quotas 100 and 2000
  and none), a group table, and LNM$SYSTEM_TABLE, under each privilege;
- **D**: which names `$DELLNM` deletes when names of the same name exist at
  several modes, shown by translating at each mode after it.

F makes round 1's fixtures again, and Z removes them, round 1's three
leftover names (PRIV94_L21, L49, L51; NOLOGNAM if they're already gone),
and every name round 2 may have left, at every mode. `PRIV94E.COM` is
DCL's side: DEFINE, SHOW LOGICAL, DEASSIGN, F$TRNLNM, and SHOW SECURITY on
two tables made at supervisor mode (round 1's, made at user mode, DCL
couldn't see), under several privileges, and SET PROCESS/PRIORITY done
right. `TestPriv94BProbe` rehearses the program.

| File | What it holds |
| ---- | ------------- |
| `priv94b.mar` | Round 2's program |
| `priv94r.com` | SYSTEM's run: the account PRIV94 again, the build, the fixtures, SYSTEM's parts, DCL's side |
| `priv94v.com` | PRIV94's run |
| `priv94e.com` | DCL's side |
| `priv94y.com` | The end: the fixtures, the tables, and the account removed |
| `exchange2.cmd`, `copyout2.cmd` | Round 2's files onto round 1's volume, and its logs back into `vax/round2/` |

1. **Add round 2's files** to round 1's volume, from the repository root:

       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange2.cmd

2. **Attach it** (`attach rq1 testdata/disks/priv94-exchange.dsk`) and, **as
   SYSTEM**:

       $ MOUNT/SYSTEM DUA1: PRIV94
       $ @DUA1:[000000]PRIV94R

   Its log is `PRIV94$DISK:[PRIV94LOG]PRIV94R.LOG`, ending `PRIV94R
   FINISHED`.

3. **Log in as PRIV94** (password `PRIV94PW`):

       $ @PRIV94$DISK:[000000]PRIV94V
       $ LOGOUT

4. **As SYSTEM**:

       $ @PRIV94$DISK:[000000]PRIV94Y
       $ DISMOUNT DUA1:

       sim> detach rq1

5. **Copy the results back**:

       mkdir -p testdata/priv94/vax/round2
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout2.cmd
