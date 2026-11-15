# Phase 93's VMS round

`docs/planning/PHASE-93 - file protection.md`'s subtask 14. Rounds 1 to 3
(`testdata/prot93`) had VMS judge files VMS made. Here VMS reads files govax
gave owners, protections, and ACLs, and its DIRECTORY/SECURITY, SHOW
SECURITY, and `$CHECK_ACCESS` say whether they agree with govax's. Nothing
on the volume is DIGITAL's.

| File | What it is |
| --- | --- |
| `exchange.cmd` | Makes the volume (label VMS93, RD54, 3-block clusters): `[V93]`, 17 files each with an owner, a protection, and for nine an ACL govax set (identifier ACEs by UIC, a group and a member wildcard, a general identifier by value, two in order, PROTECTED and NOPROPAGATE, an environmental identifier, a file owned by [0,0]); `[V93.DEF]`, a directory with a default protection ACE, a DEFAULT identifier ACE, and a creator ACE, and a file and a directory govax made in it, which inherited them; `[V93.D0]`, a directory only its owner and SYSTEM may look in; `UFO.DAT`; and VMS93C built by govax's MACRO and LINK as `VMS93CG.EXE` |
| `vms93.com` | Runs on VMS as SYSTEM. Makes round 1's accounts again (PROT93A [200,1]; PROT93B [200,2], with READALL, GRPPRV, SYSPRV, and BYPASS authorized; PROT93C [300,1], holding PROT93_TEAM), then DIRECTORY/SECURITY of the tree, SHOW SECURITY and F$FILE_ATTRIBUTES' UIC and PRO of each file, builds VMS93C and VMS93U, runs VMS93C, govax's VMS93CG, and VMS93U, removes the accounts, and ANALYZE/DISK_STRUCTURE of the volume |
| `vms93c.mar` | For each file under `[V93...]` and each of PROT93A, PROT93B, PROT93C, and SYSTEM, `$CHECK_ACCESS` for READ, WRITE, EXECUTE, DELETE, and CONTROL: a line of letters, `-` for refused, `?` for another status |
| `vms93u.mar` | `$OPEN` of `UFO.DAT` with FOP=UFO, each of six FACs with each of seven SHRs, and the status: round 3's case 458 found VMS 7.1 refusing FAC=PUT with SHR=GET (RMS$_SHR), where govax opens it |
| `t.txt` | The files' text |
| `copyout.cmd` | Copies the log and ANALYZE's listing to `vax/` |

`TestVMS93Procedure` (`internal/console/vms93_test.go`) rehearses all of it
on govax, with the accounts in the System's table as AUTHORIZE would make
them, and once `vax/vms93.log` exists holds govax's DIRECTORY, SHOW, CHECK,
CHECKG, and UFO sections to VMS's, line for line, but for
`vms93Differences`.

## Running it

1. From the repository root, the volume (`testdata/disks/vms93-exchange.dsk`):

       go build -o govax ./cmd/govax
       ./govax console < testdata/vms93/exchange.cmd

2. On VMS, as SYSTEM, with the disk attached to simh as DUA1:

       $ MOUNT DUA1: VMS93
       $ SET DEFAULT DUA1:[000000]
       $ @VMS93/OUTPUT=VMS93.LOG
       $ DISMOUNT DUA1:

   Nothing waits for input. No ACE names an alarm journal (round 3's crash).

3. Then:

       mkdir -p testdata/vms93/vax
       ./govax console < testdata/vms93/copyout.cmd

## What each answer is for

- `DIRECTORY` and `SHOW`: whether VMS reads govax's headers (owners, protection codes, ACEs, and the inherited ones) as govax shows them.
- `CHECK` and `CHECKG`: whether VMS's check of each account against each file is govax's (`security.ThirdParty`'s `Interpreted` rules, `$CHECK_ACCESS`), and whether govax's image of the same program runs on VMS.
- `UFO`: which user file opens VMS's RMS refuses with RMS$_SHR, for `rms/ufo.go`'s `ufoSharing`.
- `DISK`: whether VMS's ANALYZE/DISK_STRUCTURE finds anything wrong with a volume of govax's ACL'd headers.
