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
| `vax/` | What the VAX run leaves: the five logs, `prot93.exe`, and the definition objects |

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
