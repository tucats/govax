# Phase 93 — file protection

**Status:** planned 2026-11-14; the plan accepted by the author the same
day, with the answers in "The review". Subtasks 1 (the survey, Part A),
2 (the probe), 3 (the rule, `internal/security`), and 4 (ods2) are
done.

## Goal

Files on govax's volumes are protected as VMS protects them. Every file
has an owner UIC, a protection code, and possibly an access control
list. Every access to a file or directory, through RMS's services, the
ACP's `$QIO` functions, the image activator, or a DCL command, is checked
against the identity of the process making it. A refused access fails with
the status and message VMS gives: `RMS$_PRV` with an STV of `SS$_NOPRIV`
from RMS, `SS$_NOPRIV` from the ACP, and each DCL utility's own message
over them.

    $ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G,W) LEDGER.DAT
    $ DIRECTORY/PROTECTION/OWNER LEDGER.DAT

    Directory DUA1:[ACCOUNTS]

    LEDGER.DAT;1         [ACCOUNTS,SMITH]                  (RWED,RWED,,)

    Total of 1 file.

A new file gets its owner, protection, and ACL by VMS's rules: an explicit
protection first, then the previous version's, then the directory's
default protection ACE, then the process's default protection, which SET
PROTECTION/DEFAULT changes. DCL can show and change all of it, with a new
grammar value type for protection codes and one for UICs.

**This phase is a precursor** (the author, 2026-11-14). Later phases are
to give govax processes identities other than SYSTEM. So the checks are
real and always made, even though today every process is SYSTEM, [1,4],
with every privilege, and so passes them. Nothing in this phase may assume
the accessor is SYSTEM; the tests reach other identities through
`$CREPRC`'s UIC argument, SET PROCESS/PRIVILEGES, and credentials built in
Go.

ACLs are in, as VMS has them on files: evaluated in every check,
inherited by new files and directories, shown, and changed by SET
SECURITY/ACL; and so are the security services, `$SETDFPROT`, `$CHKPRO`,
`$CHECK_ACCESS`, `$GET_SECURITY`, `$SET_SECURITY`, and the ACL services
(the review, answers 1 and 2).

Out of scope (the review): a user authorization file and LOGINOUT's use
of it, volume protection (the volume's own protection code; answer 4), the
ACL editor, security auditing and alarms (their ACEs are kept and shown,
and do nothing), and protection of other object classes (devices, queues,
logical name tables) beyond moving the existing mailbox and section checks
onto the new code.

## Part A — the survey (2026-11-14)

### A.1 What VMS's manuals say

Sources: the *OpenVMS User's Manual* (7.3-1), chapter 10, "Controlling
Access to Resources"; the *DCL Dictionary* (7.3-1): COPY, EXCHANGE/NETWORK,
INITIALIZE, SET FILE, SET PROTECTION/DEFAULT, SET SECURITY, SHOW
PROTECTION, SHOW SECURITY, F$ENVIRONMENT, F$FILE_ATTRIBUTES, and Appendix
A; the *System Management Utilities Reference Manual* (the "SysAdmin"
books), the ACL editor's ACE formats and the SYSGEN parameter
RMS_FILEPROT; the *VMS 5.0 System Services Reference Manual*, `$SETDFPROT`
and `$CHKPRO`; and the *OpenVMS VAX Guide to System Security* (VMS 6.0,
1993, `AA-PV5RA-TE`, added to the reference folder by the author
2026-11-14), chapter 4, which A.1a summarizes. The guide is VMS 6.0's, a
little older than the VAX the probe runs on, so where it and 7.1 might
differ the probe has the last word (A.4).

**The categories (User's Manual 10.3).**

| Category | Who |
| --- | --- |
| System | a UIC group from 1 to 10 octal (MAXSYSGROUP), or SYSPRV, or GRPPRV and the object owner's group |
| Owner | the UIC identical to the object's owner |
| Group | the same UIC group as the object's owner |
| World | everyone |

A process is in every category whose test it passes, and gets an access
if any of them grants it. The code is written `(S:RWED,O:RWED,G:RE,W)`: a
category with no colon gets no access, a category not named in a change
keeps what it had (SET PROTECTION/DEFAULT's example), and each category
may be abbreviated to its first letter.

**The access types for files (10.3).**

| Access | On a file | On a directory |
| --- | --- | --- |
| Read | read, print, copy; implies execute | list it, and look files up with wildcards |
| Write | change the contents and the characteristics that describe them, but not delete | insert or remove an entry |
| Execute | run an image or a command procedure | look up files whose names you know |
| Delete | delete the file, *with write access to its directory* | |
| Control | change the security profile; implicit for the system and owner categories (SET SECURITY) | |

**Default protection and inheritance (10.4; COPY; EXCHANGE/NETWORK).**
A new file's protection and ACL come from, in order:

1. an explicit protection (`/PROTECTION` on BACKUP, COPY, CREATE, SET
   FILE; a XABPRO to RMS);
2. the previous version of the file, if there is one: "A new version of an
   existing file receives the UIC-based protection and ACL of the previous
   version";
3. the directory's *default protection ACE*,
   `(DEFAULT_PROTECTION,S:RWED,O:RWED,G:RE,W:)`, which applies to new files
   in that directory "or subdirectory of the directory"; and the
   directory's ACEs that carry the DEFAULT option, copied into the new
   file's ACL without it;
4. the process's default protection, `(S:RWED,O:RWED,G:RE,W)` unless SET
   PROTECTION/DEFAULT (or `$SETDFPROT`) changed it. A process inherits its
   default protection from its creator; the first comes from the SYSGEN
   parameter RMS_FILEPROT, %XFA00.

A renamed file's protection is unchanged.

**INITIALIZE/FILE_PROTECTION is not used on ODS-2.** "OpenVMS systems
always use the default file protection" (the DCL Dictionary). The home
block's default file protection is for RSX-11M.

**The owner of a new file (COPY, EXCHANGE/NETWORK).** "Normally, the owner
of the output file will be the same as the creator of the output file;
however, if a user with extended privileges creates the output file, the
owner will be the owner of the parent directory or of a previous version of
the output file, if one exists." Extended privileges are SYSPRV or BYPASS,
a system UIC, GRPPRV when that owner is in the creator's group, or an
identifier with the resource attribute that is that owner.

A **Creator ACE** on a directory decides what extra ACE a new file gets
when it isn't owned by its creator and the creator has no system
privileges, the case of a resource identifier. Without one, the extra ACE
grants control and the owner's access.

**Changing a profile (SET SECURITY).** It needs control access. `/OWNER`
needs GRPPRV to give a file to another member of one's group and SYSPRV
for any other UIC. `/DEFAULT` remakes the profile as if the file were just
created: protection from the directory's default protection ACE or the
process default, the ACL from the directory's DEFAULT ACEs, and the owner
the directory's; a subdirectory gets the parent directory's owner,
protection, and ACL. `/LIKE` copies another object's elements,
`/COPY_ATTRIBUTE` picks which. "If an object profile is modified while
the object is being accessed, the existing access is unaffected." Protected
ACEs survive `/ACL/DELETE`, `/LIKE`, and `/DEFAULT`; nopropagate ACEs aren't
copied.

**Obsolete forms (DCL Dictionary, Appendix A).** SET PROTECTION (on a
file) is replaced by SET SECURITY/PROTECTION, SET ACL and SHOW ACL by SET
SECURITY/ACL and SHOW SECURITY. SET FILE/PROTECTION and /OWNER_UIC are
"superseded by" SET SECURITY. SET PROTECTION/DEFAULT and SHOW PROTECTION
remain. Whether VMS 7.1 still accepts the obsolete forms is for the probe.

**SHOW PROTECTION** writes `SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD=NO
ACCESS`; **F$ENVIRONMENT("PROTECTION")** the same form;
**SHOW SECURITY** writes

    WORK_DISK$:[GREG]95_FORECAST.TXT;1 object of class FILE
          Owner: [ACCOUNTING,GREG]
          Protection: (System: RWED, Owner: RWED, Group: RE, World)
          Access Control List: <empty>

**The services.** `$SETDFPROT` sets and reads the process's default
protection. `$CHKPRO` (VMS 5.0) answers whether a set of credentials may
access an object with a given owner, protection, and ACL.

### A.1a What the Guide to System Security adds

**How an access is decided (4.3, Figure 4-1).** "The operating system
calls the Check Protection (`$CHKPRO`) system service", which goes in this
order:

1. **The ACL.** The first identifier ACE matching any of the process's
   rights identifiers grants or denies, and the ACL's scan stops. When it
   denies, only the protection code's *system and owner* fields can still
   grant (and privileges, below).
2. **The protection code**, if the ACL didn't grant and the owner UIC isn't
   0: the owner field, then world, then group, then system (4.5.3), stopping
   at the first category the process belongs to that grants the access.
   GRPPRV (in the owner's group) and SYSPRV get the system field.
3. **Privileges.** BYPASS gives every access. READALL gives read "even if
   that access is denied by the ACL and the protection code", and the
   process keeps whatever the code grants.
4. Access overrides of other classes (OPER for queues, SYSNAM for the system
   logical name table); none for files.

An object owned by UIC 0 has no protection code check: everyone has every
access but control, unless the ACL has identifier ACEs, when only the ACL
or privilege grants.

**Control access (4.5.2, 4.6.2).** Never in a protection code. A process
has it if it has the owner's UIC, belongs to the system category (and the
owner isn't 0), holds BYPASS, or holds an identifier an ACE grants CONTROL.
Group and world never get it from the code. Control alone doesn't change a
*file's* owner (4.8.4.2): that also needs a system UIC, SYSPRV, BYPASS, the
volume owner's UIC, GRPPRV in the file owner's group, or both the old and
the new owning identifiers with the resource attribute.

**File access (4.8.4.2, 4.8.4.3).** The access types are A.1's. Writing a
file needs read *and* write access. Opening a file by name needs read or
execute access to "the directory containing the file", and the volume's
access is checked before the directory's or the file's. A file opened by
file ID passes over directory protection.

**Creating a file (4.8.4.4).** It needs write access to the directory,
write access to the previous version, delete access to the oldest version
when the new one would pass the version limit, and read, write, and create
access to the volume.

**A new file's profile (4.8.4.5).**

1. An explicit owner and protection (`/OWNER_UIC`, `/PROTECTION`).
2. A **new version**: "created with you as its owner and the protection
   code and ACL of the earlier version"; "with sufficient privilege, you can
   reassign ownership to the owner of the previous version".
3. A **data file** with no previous version: the directory's default
   protection ACE's code, and every directory ACE with the DEFAULT
   attribute. The owner is the creator's UIC "unless you have privilege
   (SYSPRV, BYPASS, or GRPPRV with the same group UIC)".
4. A **directory file**: the parent's protection less delete (a top-level
   directory's from the MFD), and the parent's whole ACL, DEFAULT ACEs
   included, but for NOPROPAGATE ones; the owner the creator's UIC unless
   privileged; a directory named in UIC form, `[11,304]`, owned by that UIC.
5. Otherwise the process's default protection.
6. Then, for a creator that isn't the new file's owner (a resource
   identifier's directory), the Creator ACE's access, or an ACE of control
   plus the owner field's access.

The guide's privileged owner (the directory's) and the Dictionary's
(the previous version's, else the directory's) agree. Section 4.4.7 and
4.5.6 have examples of DEFAULT ACEs and the default protection ACE.

### A.2 What govax does today

**The process's identity.** `corevms.Process` has `Username`, `UIC`, and
the authorized, process, and current privilege masks
(`corevms/process.go`). Process 1 is SYSTEM, `[1,4]`, holding every
privilege. A `$CREPRC` child gets its creator's UIC, or the UIC argument
(a detached process), with the privilege rules of `checkCreatePrivileges`;
a spawned subprocess has its owner's. `corevms/rights.go` is an in-memory
rights database of the process's UIC identifier and the six environmental
identifiers, which `IdentifierText` uses to show `[1,4]` as `[SYSTEM]`.
There is no default file protection in the process: F$ENVIRONMENT's
PROTECTION is a constant string (`dclshell/dcllexprocess.go`).

**A UIC check exists, for other objects.** `corevms/uicprot.go`'s
`uicAccess` is A.1's category rule with BYPASS, SYSPRV, GRPPRV, and READALL,
used by mailboxes, global sections, and common event flag clusters. RMS
can't use it: `corevms` imports `rms`, so `rms` can't import `corevms`
(`rms/context.go`'s opening comment).

**ods2 stores the profile but decides nothing.** `ondisk` has `Uic`,
`ParseUic`, `ParseProtection` (with a base, so a category not named keeps
its value), `FormatProtection`, and an ACL area returned as raw bytes
(`ondisk/acl.go`: "This package doesn't interpret the ACEs").
`volume.CreateFileVersion` makes every new file with the home block's
owner and default file protection, the field A.1 says ODS-2 doesn't use.
`volume.CreateDirectory` takes an owner and protection, and
`InheritedDirectoryOptions` is the parent's protection less delete.
`UpdateHeader` and `SetACL` change a header.

**RMS.** No service checks protection. `$CREATE` reads a XABPRO
(`rms/xab.go`'s `readXABInputs`, `HasPro`) and never applies it; `$DISPLAY`
fills one from the header. `RMS$_PRV` is returned today only for a write
to a volume mounted read-only. The ACP's IO$_CREATE and IO$_MODIFY take
`ATR$C_UIC` and `ATR$C_FPRO` (`rms/acpattr.go`) unchecked; IO$_DELETE
refuses the reserved files with `SS$_NOPRIV`.

**Where files are reached.** Opens by RMS's services and the ACP go
through ods2's `Volume.Access` (`rms/sharing.go`), but the DCL utilities
open files with `vol.OpenFID` directly: about 20 sites in `type.go`,
`copy.go`, `recordfile.go`, `edit.go`, `purge.go`, `delete.go`, `save.go`,
and others. Directories are reached by `filespec.ResolveDirectory` (25
sites) and `OpenDirectory` (13), entries changed by `Insert`, `Remove`,
and `Rename`, and headers by `UpdateHeader` (14). There is no one place
where an access is decided.

**What the user sees already.** DIRECTORY has `/OWNER`, `/PROTECTION`,
and `/FULL` (owner, `File protection:`, and `Access Cntrl List:` with
identifier ACEs, `rms/directory.go`); F$FILE_ATTRIBUTES has PRO, UIC, GRP,
and MBM; CREATE/DIRECTORY takes `/OWNER_UIC` and `/PROTECTION`; BACKUP
saves and restores owners, protections, and ACLs, and has `/BY_OWNER`.
There is no SET SECURITY, SHOW SECURITY, SET FILE, SET PROTECTION (either
form), SHOW PROTECTION, SET PROCESS, `$SETDFPROT`, or `$CHKPRO`.

**The grammar.** `internal/dcl`'s value types are `$ANY`, `$NAME`,
`$STRING`, `$INTEGER`, `$REST_OF_LINE`, and `$EXPRESSION`
(`dcl/grammar.go`). CREATE/DIRECTORY's `/PROTECTION` is `$any/list`, its
`/OWNER_UIC` `$any`, each parsed by its handler.

**Host files** have no VMS profile. They stay outside this phase.

### A.3 What the manuals leave open

These go to the probe (Part D). The guide (A.1a) answers some in
principle; the probe confirms them on 7.1.

1. Which directory accesses a lookup needs at each level of a path
   (`[A.B.C]X.DAT`). The guide speaks of "the directory containing the
   file"; whether the levels above are looked up with a check is open.
2. What a rename needs: delete on the file? write on both directories?
3. What a supersede (`FAB$V_SUP`) needs: the guide gives a new version's
   needs (write on the directory and the previous version, delete on the
   oldest past the limit); a supersede's are open.
4. What an image activation and a command procedure (`@`) need: execute
   alone, or read.
5. What READALL gives besides read (the guide: nothing but what the code
   grants; whether that includes control is open).
6. Which statuses and STVs each RMS service and ACP function returns when
   refused, and which DCL message each utility writes: TYPE, COPY,
   APPEND, DELETE, PURGE, RENAME, DIRECTORY (of a directory it can't
   read, and of files it can't read with `/FULL`), DUMP, EDIT, OPEN,
   READ, WRITE, `@`, RUN, CREATE, CREATE/DIRECTORY, BACKUP, LIBRARY,
   MACRO, LINK, FORTRAN, ANALYZE, CONVERT.
7. The owner and protection CREATE/DIRECTORY gives with and without
   extended privileges, and `/OWNER_UIC=PARENT`.
8. What a XABPRO's owner does on `$CREATE` without privilege, and what
   `$CREATE` writes back to the XABPRO.
9. What COPY gives its output when the input's protection differs from
   the default (the Dictionary says the output's previous version, the
   directory, the process, never the input), and BACKUP's restore without
   `/BY_OWNER`.
10. The messages for a malformed protection code or UIC in each place
    one is typed (`/PROTECTION=(X:R)`, `(S:RQ)`, `(S:R,S:W)`, `[1,99999]`,
    `[SMITH]` with no such identifier).
11. Whether VMS 7.1 accepts SET PROTECTION=(code) on a file and SET
    FILE/PROTECTION, and SHOW SECURITY's and DIRECTORY/SECURITY's layouts
    with ACEs of each kind.
12. Whether DIRECTORY lists a file it can't read in the brief form and
    what `/FULL` shows of it (VMS reads headers through the directory, not
    the file).

### A.4 The probe's answers

Round 1 ran on VMS V7.1 (a simulated VAX 8600) on 2026-11-14; its logs are
`testdata/prot93/vax/`, and the exchange volume, whose headers VMS wrote,
is kept local. Every procedure ran to its end. Tags: `M0` is SYSTEM
without READALL, GRPPRV, SYSPRV, and BYPASS, `M1` with READALL, `M4` with
all four; `B1`/`C1` are PROT93B/PROT93C with READALL, `x3` SYSPRV, `x4`
BYPASS; `A`, `B`, `C` alone are the accounts' DCL runs without privileges.

**What round 1 lost.** Two of the probe's own mistakes:

- `PROT93.COM` gave PROT93.EXE and HELLO.EXE their protection in one SET
  SECURITY with a comma list, which 7.1's SET SECURITY refuses
  (`%DCL-W-NOLIST`). The image kept SYSTEM's default, `(RWED,RWED,RE,)`, so
  no account could run it without READALL, SYSPRV, or BYPASS (GRPPRV gives
  nothing in SYSTEM's group): the program's passes without privileges
  (`A0`, `B0`, `C0`) and with GRPPRV (`B2`, `C2`) failed at activation. The
  accounts' DCL runs (Part B) did not depend on it and are whole. SHOW
  SECURITY with a list failed the same way in four places, but PROT93Z's
  listing shows those files.
- Each `PROCx.COM` was made by CREATE followed by a `$ WRITE` line, which
  ended CREATE's data, so the procedures are empty: `@` shows whether the
  open is allowed, not whether an execute-only procedure's lines run.

Round 2 (Part D) ran the lost passes on the same volume, whose per-tag
files they had never touched, on 2026-11-14 (`vax/round2/`), with the
cases round 1 left open; its answers are folded in below, marked "round
2". Its fix-up, `PROT93S.COM`, opened the `PRO2x.COM` procedures to world
RE along with the files the accounts needed, so the question of an
execute-only procedure's lines is still open (answer 4).

#### Answers to A.3

1. **Each level of a path is looked up with a check.** M0 (system category,
   no privileges, a new process) is refused `[PROT93.PX1.PY.PZ]X.DAT` and
   `[.PX2.PY.PZ]X.DAT`, where an upper level gives no access, with
   `RMS$_PRV`/`SS$_NOPRIV`; `$PARSE` itself fails when the path can't be
   walked. B3 and C3 (SYSPRV, which gives the system category's empty
   field) were let through PX1 and PX2 but refused PX3 (the file's own
   directory): their READALL passes had run first in the same process, and
   RMS keeps a process's directories in a cache, so the upper levels were
   not looked up again. govax keeps no such cache: every level needs R or E,
   as the guide's rule says. Round 2: each account, its first pass in a new
   process without privileges, is refused PX1 and PX2 too.
2. **Rename** needs D on the file and R and W on both directories (round
   2: refused within `[.DW]`, WE, and allowed within `[.DRWO]`, RW): no D on the
   file (`noDel`, or R alone) or no W on the old directory is `RMS$_RMV`
   with `SS$_NOPRIV`; no W on the new directory is `RMS$_ENT` with
   `SS$_NOPRIV`; DCL's RENAME says `%RENAME-E-OPENIN ... as input`,
   `-RMS-F-RMV`, `-SYSTEM-F-NOPRIV`. A rename keeps the file's profile.
3. **A supersede** of a file without D is refused (`RMS$_PRV`/`SS$_NOPRIV`),
   as is a new version past the version limit whose oldest version lacks D;
   neither creates anything. Round 2: a supersede needs W as well (a file
   with R and D alone is refused, by M0 too), and one allowed returns
   `RMS$_SUPERSEDE` (`%X10631`), the new file the creator's, with the old
   file's protection. A new version needs W on the previous version (round
   2: refused over a file giving R alone, and over one giving RE to the
   group and world, to PROT93B and PROT93C, whose CIF opens are refused too).
4. **RUN and `@`** need R or E: an image or procedure with only E or only R
   runs or opens; one with neither is `%DCL-W-ACTIMAGE`, `-CLI-E-IMGNAME,
   image file DUA1:[PROT93.IMG]HELLON.EXE;`, `-RMS-E-PRV` (`$STATUS`
   `%X1001829A`), and `%DCL-E-OPENIN, error opening ... as input` for `@`.
   READALL runs an image too (B1 ran PROT93.EXE). Whether an execute-only
   procedure's lines run is still open (round 2's procedures were opened by
   its fix-up); govax runs them, unconfirmed.
5. **READALL** gives read and nothing else: B1/C1 read every file, search
   every directory, and are refused every write, every create in a
   directory they can't write, and every change of profile.
6. **Statuses and messages:** "Statuses and messages" below.
7. **CREATE/DIRECTORY** gives the parent's protection (no parent had D to
   take away) and the parent's ACL but its NOPROPAGATE ACEs. `/PROTECTION`
   is merged into the parent's (`(W:RE)` under `(RWE,RWE,RWE,RWE)` is
   `(RWE,RWE,RWE,RE)`). The owner is the creator's UIC, or with privilege
   (SYSTEM's) the parent's owner; `/OWNER_UIC=PARENT` the same;
   `/OWNER_UIC=[300,1]` that UIC. A top-level directory gets the MFD's
   owner and protection. `[300001]` and `[200002.SUB]` are owned by
   `[SYSTEM]`, the MFD's owner: 7.1 gives no UIC-named directory its UIC.
   An account (no privilege) making a directory in another's directory owns
   it, with the parent's protection and ACL (`[.OWN.DB0]`, `[.DP.DB0]`).
8. **A XABPRO's owner** other than the creator's UIC, without the privilege
   to choose one, makes `$CREATE` fail (`RMS$_PRV`/`SS$_NOPRIV`, B1 and C1),
   creating nothing. With privilege (a system UIC, SYSPRV, BYPASS), the file
   gets the owner and protection given, a UIC no identifier names among
   them (`[310,1]`, `[454,1]`). A XABPRO owner of 0 is as if none were
   given; its protection is used. Whether `$CREATE` writes the XABPRO back
   wasn't asked (the program shows `$DISPLAY`'s): the services' round.
   Round 2: GRPPRV is not the privilege to name another group's owner
   (B2 is refused `[310,1]`), and DCL's CREATE/OWNER_UIC of another's UIC
   is `%CREATE-E-OPENOUT ... as output`, `-RMS-E-PRV`, its own UIC
   allowed; CREATE/DIRECTORY/OWNER_UIC of another's, or `=PARENT` under
   another's directory, is `%CREATE-E-DIRNOTCRE`, `-SYSTEM-F-NOPRIV`.
9. **COPY** never gives the output the input's profile: `FROT1.DAT` copied
   is the process default, and a copy of a file with an ACL has none
   (`L3.DAT`). `COPY/PROTECTION=(W:R)` is merged into what the output
   would otherwise get (`(RWED,RWED,RE,R)`). BACKUP's restore: not asked.
10. **Malformed values:** "Forms and their errors" below.
11. **The obsolete forms work:** `SET FILE/PROTECTION=(code)`, `SET
    FILE/OWNER_UIC=uic`, `SET PROTECTION=(code) file`, `SET ACL/ACL=(ace)
    file`, and `SHOW ACL file`. What `SET PROTECTION file`, `SET
    FILE/OWNER_UIC file`, and `SET FILE/PROTECTION file` with no value do
    (round 2): the first and last give the file the process default
    protection, the owner kept; the second makes the process's UIC the
    owner, the protection kept.
    SHOW SECURITY's and DIRECTORY's layouts: "Display" below.
12. **DIRECTORY** lists a file it can't read in the brief form. `/FULL` and
    `/SECURITY` write the file's name, then the message's text, and count
    no blocks (`Total of 1 file, 0/0 blocks.`):

        UNONE.DAT;1         insufficient privilege or object protection violation
        UNONE.DAT;1        insufficient privilege or object protection violation

    (the first is `/FULL`, the name in 20 columns; the second `/SECURITY`,
    in 19). F$SEARCH finds such a file; F$FILE_ATTRIBUTES of it, any item,
    is `%SYSTEM-F-NOPRIV` (it opens the file).

#### The check

Every cell of the `[.F]` matrix in SYSTEM's and the privileged passes is
the guide's `$CHKPRO` sequence (A.1a), and so is every TYPE of an ACL'd
file in the accounts' runs:

- **Categories add.** PROT93B (`[200,2]`, the owner's group) is in the
  group and world categories; SYSTEM `[1,4]` in the system and world.
- **Write needs read** for a `$OPEN` with PUT, and a file with W alone
  opens for neither GET nor PUT (`FW.DAT`); BIO (block I/O for read) is
  allowed where GET is.
- **The first matching identifier ACE decides**, and the code is not
  consulted, but for the system and owner fields after a denial:
  `FACLOWN` (owner `RWED`, ACE `PROT93A` NONE) is read by its owner;
  `FACLSYS` (system `RWED`, ACE `SYSTEM` NONE) is opened by M0;
  `FACLDENY` (everyone `RWED`, ACE `PROT93B` NONE) is refused to PROT93B
  and read by the others; `FACLTWO` (`PROT93C` NONE, then `PROT93_TEAM`
  READ) is refused to PROT93C, who holds both; `FACLCTL` (ACE `PROT93C`
  CONTROL) is refused to PROT93C for reading even after its world field
  was set to RE. An ACE's grant is not limited by the code (`FACLGRANT`,
  no access, ACE R+W: PROT93C writes it).
- **ACE identifiers** match by UIC, a group wildcard `[200,*]`, a general
  identifier held (`PROT93_TEAM`), and an environmental one
  (`INTERACTIVE`).
- **GRPPRV** (round 2) puts its holder in the system category for objects
  owned by its own group: B2 opens `FSYS` and `FROT1` (system R) like M0,
  C2 (group 300) gains nothing on group 200's files.
- **Privileges** after the code: SYSPRV is the system category (no more:
  C3 is refused `FACLGRANT`'s files that give the system nothing), BYPASS
  is everything, READALL is read. A privilege doesn't overrule an ACE's
  denial (B1 is refused writing `FACLDENY`), but SYSPRV's system field
  does (B3 writes it).
- **Owner UIC `[0,0]`** with no ACL: every access to everyone (M0 writes
  `FUIC0.DAT`, protection none). With an ACL (`FUIC0ACL`, ACE `PROT93B`
  READ): the ordinary check (PROT93A and PROT93C refused, PROT93B reads).
- **Control** is the system and owner categories' and an ACE's CONTROL's:
  M0 changes the protection of PROT93A's files; PROT93A its own; PROT93C
  those whose ACL gives it CONTROL (`DCC_C0`, `FACLCTL`); PROT93B (group,
  RWED) none. SHOW SECURITY needs read or control (M0 shows `UNONE`,
  which gives the system nothing; PROT93C is refused `UGRP`).
- **The ACP's read of attributes** by name (`ATR$C_FPRO`, `UIC`,
  `ACLLENGTH`) needs read or control, as SHOW SECURITY does. Round 1 seemed
  to show it free (every identity it had held control); round 2's
  unprivileged passes are refused it (`SS$_NOPRIV`) where the file gives
  them neither: B0, C0, and C2 on `FNONE`, `FE`, `FW`, `FD`, `FSYS`; A0
  (the owner) and B2 (GRPPRV, so the system category for group 200's
  files) never. On `FUIC0ACL` M0, A0, C2, and C3 are refused: an object
  owned by `[0,0]` gives the system category no control (with no ACL it
  needs none).
- **An open by file ID** passes over directories: M0 opens `[.DN]A.DAT`
  and `[.PX1.PY.PZ]X.DAT` by ID, and is refused `FNONE` by ID for the
  file's own protection.

#### Directories

| Directory gives | Wildcard `$SEARCH` | Lookup by name | Create (RMS, ACP) |
| --- | --- | --- | --- |
| R | yes | yes | no |
| E | no (`RMS$_PRV`) | yes | no |
| WE | no | yes | **no** |
| RW (round 2) | yes | yes | yes |
| RWE | yes | yes | yes |
| W | no | no | no |
| none | no | no (`$SEARCH`, open, ACP read) | no |

Creating in a directory with W and E but not R is refused, by RMS and by
the ACP's IO$_CREATE alike, and allowed with R and W but not E (round 2):
a create needs R and W on the directory. So do an erase and a rename
(round 2: both refused in `[.DW]`, WE, both allowed in `[.DRWO]`, RW;
`[.ERX]`, RE, refuses an erase), with D on the file. READALL searches
and looks up in every directory.

#### New files' profiles

- **Protection:** an explicit code is merged into the base; the base is
  the previous version's (`P1.DAT;2` and `;3` take `;1`'s, an account's new
  version of PROT93A's file takes its protection), else the directory's
  default protection ACE (`[.DP]`'s files are `(RWED,RWED,R,E)`;
  `CREATE/PROTECTION=(W:RE)` there is `(RWED,RWED,R,RE)`), else the
  process default. CREATE, COPY, APPEND/NEW_VERSION, `$CREATE`, and the
  ACP's create all follow it.
- **ACL:** the previous version's but its NOPROPAGATE ACEs, protected
  ones too (`L1.DAT;2`; round 2's `NP_A0.DAT;2` keeps only the
  `PROT93_TEAM` ACE of three), else the directory's DEFAULT ACEs, with
  DEFAULT cleared and
  NOPROPAGATE **kept**: `[.DP]`'s DEFAULT+NOPROPAGATE ACE for PROT93B is on
  every file made there, as `OPTIONS=NOPROPAGATE`. A subdirectory gets the
  parent's whole ACL but its NOPROPAGATE ACEs, the DEFAULT ones keeping
  DEFAULT, the default protection ACE among them.
- **Owner:** the creator's UIC; with privilege, the previous version's
  owner (`P1.DAT;2` and `SYS1.DAT;2` are `[300,1]`'s), else the
  directory's. Privilege here is a system UIC (M0, with none of the four,
  and M1), SYSPRV, or BYPASS (B3, B4, C3, C4); READALL is not (B1 and C1 own
  their files). GRPPRV is too, for a directory or version owned in its
  group (round 2: B2's files in `[.DRW]`, `[.OWN]`, and `[.DP]` are
  `[200,1]`'s; C2's its own). A rename keeps the owner and protection.
- **The process default** is `(RWED,RWED,RE,)`, RMS_FILEPROT's `%XFA00`,
  and every account started with it.

#### Changing profiles

- **SET SECURITY** takes one file specification, wildcards allowed (a list
  is `%DCL-W-NOLIST`); `/PROTECTION` merges into the file's code;
  `/OWNER` takes a UIC, an identifier name (`PROT93A`, or the general
  identifier `PROT93_TEAM`, shown bare as the owner), or `[g,m]`;
  `/DEFAULT` sets the process default protection, the process's UIC as the
  owner, and deletes every ACE but the protected ones; `/LIKE=NAME=file`
  copies the protection and the ACL but its NOPROPAGATE ACEs, keeping the
  target's protected ACEs (`L2.DAT`), and `/COPY_ATTRIBUTE=OWNER` the
  owner; `/EXCLUDE`, `/BY_OWNER`, `/CLASS=FILE`, `/LOG`
  (`%SET-I-MODIFIED, DUA1:[PROT93.SEC]S2.DAT;1 modified`). A missing file
  is `%SET-E-OPENOUT, error opening DUA1:[PROT93.SEC]NOSUCH.DAT; as
  output`, `-RMS-E-FNF`; a missing directory adds `-SYSTEM-W-NOSUCHFILE`
  after `-RMS-E-DNF`. Each change raises the file's revision count.
- **The owner:** SYSTEM changes any owner; a file's owner without
  privilege can't give it away (`%SET-F-WRITEERR, error writing ...`,
  `-SYSTEM-F-NOPRIV`) but may set it to its own UIC again. CONTROL through
  an ACE doesn't let an account take another's file (round 2:
  `%SET-F-WRITEERR` for PROT93C on `OT_C0`, where its protection change
  succeeds), and the ACP's IO$_MODIFY of the owner is refused to the
  file's owner (`SS$_NOPRIV`, A0 on `CO_A0`).
- **SET SECURITY/DEFAULT on a directory** (round 2) makes the process's
  UIC the owner and gives it the protection a new directory there would
  get, its parent's (`SD1.DIR` under `SEC.DIR`'s `(RWE,RWE,RWE,RWE)`).
- **SET PROTECTION/DEFAULT** merges into the process default; with no
  code it changes nothing.
- **ACL edits** (`SET SECURITY/ACL`): a new ACE goes first; an ACE for an
  identifier the ACL already has replaces that ACE and goes first;
  `/AFTER=(ace)` puts it after the ACE named; `/REPLACE=(ace)` replaces
  in place; `/DELETE` with an ACE removes it, and with none removes them
  all; `/DELETE=ALL` with `/ACL=(ace)` leaves only that ACE. An ACE with no
  ACCESS is `ACCESS=NONE`. A missing ACE: `%SET-W-NOSUCHACE, ACE for
  object DUA1:[PROT93.ACE]M.DAT;1, does not exist`, the ACE on the next
  line indented four, then `%SET-F-WRITEERR, error writing ...`. HIDDEN
  is refused (`%SET-E-NOHIDDEN, cannot modify hidden ACEs`).

#### Forms and their errors

- **Protection codes:** categories `SYSTEM`, `OWNER`, `GROUP`, `WORLD`,
  abbreviated to one letter, with `:` or `=`, any case, a space after the
  colon, letters repeated (`S:RRR`), a single category without
  parentheses (`/PROTECTION=S:R`). For files the letters are R, W, E, D:
  `C` is refused. A category with no letters is no access. An unknown
  category is DCL's (`%DCL-W-IVKEYW, unrecognized keyword - check validity
  and spelling`, `\X\`, `\SYSTEMS\`); `(S:RWED,)` and `()` are
  `%DCL-W-VALREQ`. The letters are the command's: SET's
  `%SET-F-SYNTAX, error parsing 'S:RQ'`, `-LIB-F-SYNTAXERR, string syntax
  error detected by LIB$TPARSE`; a category twice `%SET-F-SYNTAX, error
  parsing 'S:R,S:W'`, `-LIB-F-INVARG, invalid argument(s)`;
  CREATE/DIRECTORY's `%CREATE-F-SYNTAX, error parsing 'Z'`; but SET
  PROTECTION/DEFAULT's is DCL's own, `%DCL-W-IVPROT, invalid protection
  code`.
- **UICs:** `[1,99999]`, `[400000,1]`, `[1]`, `[NOSUCH]`, `NOSUCH`, and
  `[PROT93G2,*]` are SET's `%SET-F-SYNTAX, error parsing '[1,99999]'` (no
  second line); CREATE/DIRECTORY's adds `-SYSTEM-F-NOSUCHID, unknown rights
  identifier` for a name. `[1,4,5]` and `[200,1` are DCL's
  `%DCL-W-DIRECT, invalid directory syntax - check brackets and other
  delimiters`, `1,4` `%DCL-W-NOLIST`.
- **ACEs:** an unknown identifier `%SET-F-SYNTAX, error parsing
  'NOSUCH,ACCESS=READ)'`, `-SYSTEM-F-NOSUCHID`; an unknown access, keyword,
  or option `-SYSTEM-F-IVACL, invalid access control list entry syntax`,
  the text quoted from the bad item to the ACE's end; an unclosed
  parenthesis `%DCL-W-NOPAREN`.
- **AUTHORIZE** made a group identifier for PROT93A's group, `PROT93`
  `[200,177777]`, and refused the probe's own `PROT93G2` (`%UAF-E-UICERR`);
  ACEs naming `PROT93G2` failed (ACE19), `[200,*]` serves.

#### Display

- **Names of UICs:** `[group,member]` by identifier names when the group
  has one (`[PROT93,PROT93A]`, `[PROT93,*]`), `[member]` when only the
  member has (`[PROT93C]`, `[SYSTEM]`), octal numbers otherwise (`[310,1]`,
  `[0,0]`), `[*,*]`; a general identifier bare (`PROT93_TEAM`). The same
  in SHOW SECURITY, DIRECTORY, ACEs, and F$FILE_ATTRIBUTES' UIC.
- **SHOW SECURITY:** a blank line, then

        DUA1:[PROT93.SEC]S1.DAT;1 object of class FILE
             Owner: [SYSTEM]
             Protection: (System: RWED, Owner: RWED, Group: R, World)
             Access Control List: <empty>

  each ACE on its own line indented ten; a category with no access is its
  name alone (`World`). The volume: `PROT93 object of class VOLUME`.
- **SHOW ACL:** `Object type: FILE,  Object name: DUA1:[PROT93.SEC]S14.DAT;1,
  on 14-NOV-2026 09:05:39.99`, then the ACEs indented ten.
- **DIRECTORY:** `/OWNER` the name in 21 columns and the owner padded to 31;
  `/PROTECTION` `(RWED,RWED,RE,)`; `/SECURITY` both, the owner padded to 33,
  each ACE after on its own line indented ten; `/ACL` the name padded to
  19, then the ACEs; `/FULL` has `Owner:    [PROT93,PROT93A]`, `File
  protection:    System:RWED, Owner:RWED, Group:, World:`, and `Access
  Cntrl List:  None`, or the ACEs one a line from column 21.
- **F$FILE_ATTRIBUTES:** `PRO` is `SYSTEM=RWED, OWNER=RWED, GROUP=RE,
  WORLD=E` (`SYSTEM, OWNER, GROUP, WORLD` for none), `UIC` the named form,
  `GRP` and `MBM` decimal (128 and 1).
- **SHOW PROTECTION:** two spaces, then `SYSTEM=RWED, OWNER=RWED,
  GROUP=RE, WORLD=NO ACCESS`; F$ENVIRONMENT("PROTECTION") has no leading
  spaces and `WORLD` alone for none.
- **DUMP/HEADER**'s owner and protection lines: `File owner UIC:
  [PROT93,PROT93A]`, `File protection:  S:, O:RWED, G:, W:`.

#### ACE layouts (`$DISPLAY`'s XABPRO)

Byte 0 is the ACE's length, byte 1 its type (`ACE$C_KEYID` 1, `AUDIT` 5,
`ALARM` 6, `DIRDEF` 9, `NEW_OWNER` 13), the word at 2 its flags (the
options in the high byte: `DEFAULT` `^X100`, `PROTECTED` `^X200`, `HIDDEN`
`^X400`, `NOPROPAGATE` `^X800`; an alarm's or audit's SUCCESS 1 and
FAILURE 2 in the low), the longword at 4 the access (R 1, W 2, E 4, D 8,
C `^X10`), then by type: an identifier ACE's identifiers, a longword each
(`[200,1]` is `00800001`, `[200,*]` `0080FFFF`, `[*,*]` `3FFFFFFF`,
`PROT93_TEAM` `80010001`, `INTERACTIVE` `80000003`); a default protection
ACE's four longwords at 8, `ACE$L_SYS_PROT` to `WOR_PROT`, each the
category's *denied* bits with C (`S:RWED` `^X10`, `G:RE` `^X1A`, none
`^X1F`); an alarm's or audit's journal name, `SECURITY`. A creator ACE is
8 bytes. An empty ACL is `aclsts` `SS$_ACLEMPTY` (`^X9D0`).
`testdata/prot93/defined.txt` has every `$ACEDEF`, `$ACLDEF`, `$CHPDEF`,
`$OSSDEF`, `$ARMDEF`, `$KGBDEF`, and `$PRVDEF` value.

#### Statuses and messages

- **RMS:** `RMS$_PRV` with STV `SS$_NOPRIV` from `$OPEN`, `$CREATE`,
  `$ERASE`, `$SEARCH`, and `$PARSE` (a path it can't walk); `$RENAME`'s
  `RMS$_RMV` and `RMS$_ENT`, each with `SS$_NOPRIV`. A file that isn't
  there: `RMS$_FNF`, `SS$_NOSUCHFILE`. **The ACP:** `SS$_NOPRIV` in the
  IOSB.
- **DCL's utilities**, each with its RMS or system message under it
  (`-RMS-E-PRV, insufficient privilege or file protection violation` or
  `-SYSTEM-F-NOPRIV, insufficient privilege or object protection
  violation`):

  | Command | Message |
  | --- | --- |
  | TYPE | `%TYPE-W-OPENIN, error opening DUA1:[PROT93.U]UNONE.DAT;1 as input` |
  | TYPE of a file in a directory it can't search | `%TYPE-W-SEARCHFAIL, error searching for DUA1:[PROT93.UDN]A.DAT;` |
  | COPY's input | `%COPY-E-OPENIN, ... as input` |
  | COPY's output | `%COPY-E-OPENOUT, error opening ...CPY_NONE_B0.DAT; as output`, then `%COPY-W-NOTCOPIED, DUA1:[PROT93.U]UWLD.DAT;1 not copied` |
  | APPEND | `%APPEND-E-OPENOUT, ... as output` |
  | DUMP | `%DUMP-E-OPENIN` |
  | SEARCH | `%SEARCH-W-OPENIN`, then `%SEARCH-I-NOMATCHES, no strings matched` |
  | DIFFERENCES | `%DIFF-F-OPENIN` |
  | OPEN/READ, @ | `%DCL-E-OPENIN, error opening DUA1:[PROT93.U]UNONE.DAT; as input` |
  | OPEN/WRITE | `%DCL-E-OPENOUT, ... as output` |
  | OPEN/APPEND | `%DCL-E-OPENIN, ... as input` |
  | F$FILE_ATTRIBUTES | `%SYSTEM-F-NOPRIV` |
  | SHOW SECURITY | `%SHOW-F-READERR, error reading ...`, `-SYSTEM-F-NOPRIV` |
  | ANALYZE/RMS_FILE | `%ANALYZE-E-OPENIN` |
  | CONVERT | `%CONV-F-OPENIN` |
  | BACKUP | `%BACKUP-E-OPENIN`, `-SYSTEM-F-NOPRIV`, then `%BACKUP-W-NOFILES, no files selected from DUA1:[PROT93.U]UNONE.DAT;*` |
  | MACRO, LINK | `%MACRO-E-OPENIN`, `%LINK-F-OPENIN` |
  | LIBRARY | `%LIBRAR-F-OPENIN, error opening DUA1:[PROT93.U]ULIBNONE.OLB; as input` |
  | DELETE | `%DELETE-W-FILNOTDEL, error deleting ...;1` |
  | PURGE | `%PURGE-W-FILNOTPUR, error deleting ...;1` |
  | RENAME | `%RENAME-E-OPENIN, ... as input`, `-RMS-F-RMV, ACP remove function failed`, `-SYSTEM-F-NOPRIV` |
  | DIRECTORY of a directory it can't read | `%DIRECT-E-OPENIN, error opening DUA1:[PROT93.UDN]*.*;* as input` |
  | CREATE | `%CREATE-E-OPENOUT, ... as output` |
  | CREATE/DIRECTORY | `%CREATE-E-DIRNOTCRE, PROT93$DISK:[PROT93.UDN.SC0] directory file not created`, `-SYSTEM-F-NOPRIV` (the name as typed) |
  | SET SECURITY without control | `%SET-E-OPENIN, ... as input`, `-SYSTEM-F-NOPRIV` |
  | SET SECURITY/OWNER without privilege | `%SET-F-WRITEERR, error writing ...` |
  | SET FILE/PROTECTION | `%SET-E-PRONOTCHG, protection on ... not changed`, then `%SET-E-PROERR, error protecting ...`, each with `-SYSTEM-F-NOPRIV` |
  | SET PROTECTION=(code) file | `%SET-E-PRONOTCHG` alone |
  | RUN | `%DCL-W-ACTIMAGE`, `-CLI-E-IMGNAME`, `-RMS-E-PRV` |

#### Open for a later round

Questions rounds 1 and 2 left unanswered, to go into the next VAX round
(the security services' round with subtask 10, Part D) so they aren't
lost; each is marked unconfirmed where govax decides it meanwhile:

1. **Whether an execute-only command procedure's lines run.** Round 1's
   procedures were empty (CREATE's data ended at the next `$` line) and
   round 2's were opened to world RE by its own fix-up, `PROT93S.COM`, so
   neither run answers it; both show only that `@` of a procedure with E
   alone opens it. The case: a procedure written with OPEN/WRITE (a line
   that writes a message), owned by PROT93A with `(S:E,O:RWED,G:E,W:E)`,
   run with `@` by an account without privileges, PROT93C, in a fresh
   login, with SET VERIFY on and off (VMS is said to run such a
   procedure without verification). Until then govax runs its lines,
   with verification as set (unconfirmed). This needs an account, so the
   round makes PROT93C again and removes it after.

## Part B — decisions

**Decision 1 — one rule, in a leaf package.** `internal/security` (plain
Go, standard library and `vmsdef` only) holds the rule A.1 describes, for
every object class: `Credentials` (a UIC, the current privilege mask, the
rights identifiers held, MAXSYSGROUP), a `Profile` (owner, protection mask,
ACL), the access bits (read, write, execute, delete, control, by class),
and `Check(Credentials, Profile, want) Decision`, which says whether the
access is granted and why (which category or privilege or ACE), for the
tests and for `$CHKPRO`. `Check` is the guide's `$CHKPRO` sequence (A.1a):
the ACL's first matching identifier ACE (a denial leaving only the system
and owner fields), then the code's owner, world, group, and system fields,
then BYPASS and READALL; owner UIC 0's special case; control access by
A.1a's rules, never from the group or world field. A separate
`CanChangeOwner` is A.1a's rule for a file's owner. `corevms/uicprot.go`'s `uicAccess` becomes a call
of it, so mailboxes, global sections, and event flag clusters keep their
behavior on the same code. RMS and corevms both import it.

**Decision 2 — identity comes from the process, always.** There is no
user authorization file in this phase (the author's note: later phases
bring other identities). `Process.Credentials()` builds a `Credentials`
from the process's UIC, current privileges, and rights identifiers.
`rms.Context` (for the services) and `rms.Session` (for the DCL
utilities) get a function field, `Credentials func() security.Credentials`,
which corevms and the console fill from the process making the call; nil
in a test that builds RMS alone means SYSTEM's, so existing unit tests
keep working, and new ones pass other credentials. A subprocess's DCL uses
its own process's. Nothing caches a decision: each access asks.

The tests reach other identities four ways: `$CREPRC` with a UIC and a
privilege mask (already supported) runs a program as `[200,1]`; `$CREPRC`
of LOGINOUT with a UIC runs a command procedure's DCL as that UIC, which
replays the probe's account runs (Part D, Part B2); SET
PROCESS/PRIVILEGES (new, Decision 10) lets SYSTEM drop BYPASS, SYSPRV,
GRPPRV, and READALL, though `[1,4]` stays in the system category by its
group; and RMS tests pass `Credentials` directly.

**Decision 3 — the process's default protection.** `Process` gains
`DefaultProtection`, set at process 1's creation from a new key,
`vax.sysgen.rms_fileprot` (default %XFA00, the SYSGEN parameter), and
copied to every process a process creates. `$SETDFPROT`, SET
PROTECTION/DEFAULT, SHOW PROTECTION, and F$ENVIRONMENT("PROTECTION") use
it.

MAXSYSGROUP is configurable the same way (the author, 2026-11-14): a key
`vax.sysgen.maxsysgroup` (default octal 10, the SYSGEN parameter's usual
value), reaching each `System` through `corevms.Sysgen` and every
`Credentials` (`security.Credentials.MaxSysGroup`), so the system category
by UIC group follows it for files and for mailboxes, sections, and event
flag clusters alike.

**Decision 4 — a new file's profile, in one function.** `rms`'s
`newFileProfile(creds, dir, name, explicit)` is the only place a new
file's owner, protection, and ACL are chosen, by A.1's order:

- **Protection of a data file:** a base, the previous version's (the
  highest existing version), else the directory's default protection ACE,
  else the process default; an explicit code (a XABPRO, `/PROTECTION`) is
  merged into the base, changing only the categories it names (A.4).
- **ACL of a data file:** the previous version's but its NOPROPAGATE ACEs
  (protected ones too), else the directory's DEFAULT ACEs with DEFAULT
  removed and NOPROPAGATE kept (A.4: NOPROPAGATE doesn't stop a default
  ACE reaching a file). COPY's output never takes its input's ACL.
- **Owner:** the creator's UIC; with privilege (a system UIC, SYSPRV,
  BYPASS, or GRPPRV in that owner's group, A.4), the previous
  version's owner, else the directory's. A XABPRO's owner is explicit,
  allowed by `CanChangeOwner`; a refused one fails the whole `$CREATE`
  with `RMS$_PRV`/`SS$_NOPRIV`, and an owner of 0 is none given.
- **Directories:** the parent's protection less delete (from the MFD for a
  top-level one), an explicit code merged into it; the parent's whole ACL
  but NOPROPAGATE ACEs, DEFAULT ones keeping DEFAULT; the owner by the same
  rule (with privilege the parent's owner). No UIC from a directory named
  `[g,m]`: 7.1 gives none (A.4). `volume.InheritedDirectoryOptions`
  already does the protection.
- The Creator ACE and its extra ACE need resource identifiers, which no
  govax process has: written so that a later phase can add them, and
  logged as not done.

Every creator calls it: `$CREATE`, the ACP's IO$_CREATE, COPY, APPEND's
new file, CREATE, EDIT's new version, CONVERT, LIBRARY, MACRO, LINK, and
FORTRAN's outputs, BACKUP's restore (whose own rule, the saved profile,
is its `explicit`), and the page file. ods2's `CreateFileVersion` gains a
way to be told the profile (Decision 8), and the home block's default
file protection stops being used for new files.

**Decision 5 — what each operation needs.** One table in `rms`
(`access.go`), from A.1 and the probe:

| Operation | File | Directory |
| --- | --- | --- |
| open for read (FAC GET, BRO/BIO reading) | R | R or E to look it up |
| open for write (PUT, UPD, TRN, DEL, BIO writing) | R and W | R or E |
| open by file ID (`NAM$W_FID`, the ACP) | as above | none |
| run an image, `@` a procedure | R or E | R or E |
| `$SEARCH` with a wildcard, DIRECTORY | | R |
| `$SEARCH` of a known name, `$PARSE` with a lookup | | R or E |
| create | | R and W (E not needed) |
| new version | W on the previous version; D on the oldest past the limit | as create |
| supersede | W and D on the old version | as create |
| delete | D | R and W |
| rename | D | R and W on both: `RMS$_RMV` for the old, `RMS$_ENT` for the new |
| change protection, ACL | control | |
| change owner | control and `CanChangeOwner` | |
| show the profile (SHOW SECURITY, DIRECTORY/FULL and /SECURITY, the ACP's read of attributes) | read or control | R or E |
| F$FILE_ATTRIBUTES, `$DISPLAY` | an open for read | R or E |

Control is the system and owner categories' (GRPPRV making its holder the
system category for its own group's objects), and an ACE's CONTROL; an
object owned by `[0,0]` gives the system category none (A.4).

Every level of a path above the file's directory needs R or E too (A.4,
answer 1; VMS's directory cache, which can skip a level a process has
already looked up, isn't kept). The volume's own protection is out of
scope (the review, answer 4).

An access is checked when it is made, as VMS's file system checks it at
access: an open file isn't checked again for each record, and a change of
protection doesn't affect an access already granted.

**Decision 6 — one checked layer, held by a test.** `rms` gets a small
set of checked operations, `openChecked(purpose)`, `lookupChecked`,
`listChecked`, `enterChecked`, `removeChecked`, `renameChecked`,
`changeProfile`, over ods2's `OpenFID`, `OpenDirectory`,
`ResolveDirectory`, `Insert`, `Remove`, `Rename`, `UpdateHeader`, and
`SetACL`. Every site in A.2 moves onto them. `TestNoUncheckedAccess`, a scan
of `internal/rms`'s source in the manner of `TestEveryInstructionImplemented`,
fails if those ods2 calls appear outside the layer and a short list of
named exceptions (mounting, verification, the page file, BACKUP's reads of
a save set, ANALYZE's raw reads), each with its reason.

**Decision 7 — statuses and messages are VMS's.** RMS's services return
`RMS$_PRV` with STV `SS$_NOPRIV`, and `$RENAME` `RMS$_RMV` or `RMS$_ENT`
with it; the ACP `SS$_NOPRIV`; RUN `%DCL-W-ACTIMAGE` over `-CLI-E-IMGNAME`
and `-RMS-E-PRV`; each DCL utility its own message over `-RMS-E-PRV` or
`-SYSTEM-F-NOPRIV`, as A.4's table has them, each held to the probe's log.
Texts come from `vmsdef.Messages`; any missing message is added through
`internal/vmsdef/gen`.

**Decision 8 — ods2 changes, in a tagged release.** In `ods2`:

- `CreateFileVersion` (and `CreateFile`) take an optional profile (owner,
  protection, ACL), by a new `CreateFileVersionWith(..., NewFileProfile)`
  or an options argument; a nil profile keeps today's behavior, so the
  `ods2` command-line tool and its tests don't change.
- `ondisk` learns the ACE layouts it already stores, with encode and
  decode for the identifier, default protection, creator, alarm, and audit
  ACEs and their flags (DEFAULT, PROTECTED, NOPROPAGATE, HIDDEN), and
  `ondisk.Uic`'s wildcard forms for identifier ACEs. The layouts are A.4's
  ("ACE layouts"), with `defined.txt`'s values, as `rms/directory.go`'s
  `aceText` already reads identifier ACEs.
- No access checks in ods2: it knows nothing of processes. Its
  `ParseProtection` and `FormatProtection` stay the shared syntax.

Tagged and pinned as Phase 22's note describes.

**Decision 9 — grammar types `$PROTECTION` and `$UIC`.** `internal/dcl`
gets two value types, split as VMS splits the work (A.4, "Forms and their
errors"). `$PROTECTION` reads a protection code as one value
(parenthesized or a single category; `:` or `=`; any case) and checks at
parse time what DCL checks: the category names (`%DCL-W-IVKEYW`) and an
empty value (`%DCL-W-VALREQ`). The handler gets each category given with
its letters as typed, and the shared parser (`ondisk.ParseProtection`'s
rule) checks the letters and repeats with the command's own message
(`%SET-F-SYNTAX, error parsing 'S:RQ'`, `-LIB-F-SYNTAXERR`;
`%CREATE-F-SYNTAX`; DCL's `%DCL-W-IVPROT` for SET PROTECTION/DEFAULT), and
merges the mask into a base. `$UIC` checks the brackets and list
(`%DCL-W-DIRECT`, `%DCL-W-NOLIST`) and hands over the text; `[g,m]`
(octal), `[name]`, `[group,name]`, and an identifier name are resolved
through the rights database when the command runs, a bad one the
command's `%SET-F-SYNTAX, error parsing '[1,99999]'` (CREATE adds
`-SYSTEM-F-NOSUCHID`). Every qualifier that takes one moves to the type:
CREATE/DIRECTORY's, BACKUP's `/BY_OWNER` and `/PROTECTION`, and the new
commands'.

**Decision 10 — the commands.**

- **SET SECURITY** `file` (one specification, wildcards allowed; a list is
  `%DCL-W-NOLIST` on 7.1) with `/PROTECTION`, `/OWNER`, `/DEFAULT`,
  `/LIKE`, `/COPY_ATTRIBUTE`, `/LOG`, `/CONFIRM`, `/EXCLUDE`, `/BY_OWNER`,
  `/CLASS=FILE` (other classes refused), and the date selectors, with
  control access, each as A.4's "Changing profiles" has it, and for
  `/OWNER` the guide's rule (`CanChangeOwner`, which is wider than the
  Dictionary's "GRPPRV ... SYSPRV": a system UIC qualifies, A.4; BYPASS and
  the volume owner's UIC as the guide says; one's own UIC again always).
- **SET SECURITY/ACL**`=(ace[,...])` with `/AFTER`, `/DELETE[=ALL]`, and
  `/REPLACE`, every ACE form the ACL editor's chapter gives (identifier,
  default protection, creator, alarm, audit; their OPTIONS), protected and
  hidden ACEs' rules; the text of an ACE read and written by `$PARSE_ACL`'s
  and `$FORMAT_ACL`'s code (Decision 13), so DCL and programs share it.
  The ACL editor (`/EDIT`, EDIT/ACL) stays out.
- **SHOW SECURITY** `file` (one specification), VMS's layout, ACEs
  included.
- **SET FILE/PROTECTION** and **/OWNER_UIC**, **SET PROTECTION=(code)
  file**, and **SET ACL** and **SHOW ACL**, the obsolete forms, exactly as
  VMS 7.1 accepts them (A.4; the review, answer 6), on SET SECURITY's
  code.
- **SET PROTECTION/DEFAULT** and **SHOW PROTECTION**.
- **SET PROCESS/PRIVILEGES** and **SHOW PROCESS/PRIVILEGES**, through
  `$SETPRV`'s code: needed to see a denial as SYSTEM. SET PROCESS's other
  qualifiers stay for a later phase.
- **CREATE/PROTECTION** and **/OWNER_UIC**, **COPY/PROTECTION**, and
  **BACKUP/PROTECTION**, as A.4 finds them in VMS 7.1.
- **DIRECTORY/SECURITY** (and `/ACL` if 7.1 has it), the existing
  `/OWNER`, `/PROTECTION`, and `/FULL` checked against the probe;
  F$FILE_ATTRIBUTES' PRO, UIC, GRP, MBM, and any item the probe shows
  missing; F$ENVIRONMENT("PROTECTION").
- The services are Decision 13.

**Decision 11 — no visible change for SYSTEM's existing work.** SYSTEM
creates files owned by `[1,4]` with `%XFA00`, which is what the home block
gave them, since govax's volumes are initialized with that owner and
default. So no existing oracle's output should change. Where one does, it
is looked into before it is accepted. The one expected difference: a new
version now gets its previous version's protection.

**Decision 12 — host files are unprotected.** A host file is the host's;
govax neither checks it nor gives it a profile. DIRECTORY and
F$FILE_ATTRIBUTES of a host file keep what they show today.

**Decision 13 — the security services** (the review, answer 2), each a
service in corevms over `internal/security` and the checked layer, with
its macros in govax's STARLET (`_S`, `_G`, and argument-list forms,
checked against real MACRO's objects as Phase 45's were), its symbols from
`vmsdef`, and its statuses VMS's:

- `$SETDFPROT`, the process's default protection;
- `$CHKPRO`, the check itself, given credentials and a profile in its
  item list;
- `$CHECK_ACCESS`, whether a user may access a file, by item list;
- `$GET_SECURITY` and `$SET_SECURITY`, an object's profile by name or
  channel, class FILE (other classes `SS$_NOCLASS` or as A.4 finds), with
  their item codes (`OSS$_`) and context;
- `$PARSE_ACL`, `$FORMAT_ACL`, and `$CHANGE_ACL`, an ACE between text and
  binary, and an ACL changed in place.

Sources: the VMS 5.0 System Services manual where it has the service,
and VSI's public system services pages (the clean-room exception of
2026-10-13) for the 6.0 ones, with the probe's cases deciding what they
leave open.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey** (done 2026-11-14): Part A, this plan.
2. **The probe** (done 2026-11-14; rounds 1 and 2, A.4): `testdata/prot93`, as
   in Part D, written and rehearsed on govax; run on the VAX by the author;
   A.4 filled in and Part B adjusted to its answers. Later rounds as later
   subtasks find gaps (the review, answer 7), each a section of A.4; the
   security services' round goes with subtask 10.
3. **The rule** (done 2026-11-14): `internal/security` (Decision 1), the
   guide's whole sequence with ACLs, `CanChangeOwner`, table-driven tests
   from A.1a and A.4; `uicAccess` on it, the mailbox, section, and event
   flag tests unchanged.
4. **ods2** (done 2026-11-14): profiles at creation and the ACE layouts (Decision 8), a
   tagged release, govax pinned to it.
5. **Identity and defaults**: `Process.Credentials` (its rights
   identifiers; subtask 3 made it with the UIC and privileges),
   `DefaultProtection` and `vax.sysgen.rms_fileprot`, and
   `vax.sysgen.maxsysgroup` (Decision 3; both in `validConfigs` and HELP),
   inheritance
   by created and spawned processes, `rms.Context.Credentials` and
   `rms.Session.Credentials` filled in; `$SETDFPROT`, SET
   PROTECTION/DEFAULT, SHOW PROTECTION, F$ENVIRONMENT; SET and SHOW
   PROCESS/PRIVILEGES.
6. **The grammar types**: `$PROTECTION` and `$UIC` (Decision 9), the
   existing qualifiers moved onto them, the parse errors held to the
   probe's.
7. **New files' profiles**: `newFileProfile` (Decision 4) and every
   creator on it, ACLs inherited, XABPRO applied and written back, the home
   block's default no longer used; CREATE/DIRECTORY's owner per A.4.
8. **Checking RMS's services and the ACP**: the checked layer (Decision 6)
   under `$OPEN`, `$CREATE`, `$ERASE`, `$RENAME`, `$SEARCH`, `$PARSE`,
   `$DISPLAY`, and IO$_ACCESS, CREATE, DELETE, and MODIFY; the image
   activator; held to the probe's program runs under each identity.
9. **Checking the DCL utilities**: every `OpenFID` and directory site in
   A.2 moved onto the layer, `TestNoUncheckedAccess`, and each utility's
   messages held to the probe's DCL logs, SYSTEM's and the accounts'.
10. **The ACL services**: `$PARSE_ACL`, `$FORMAT_ACL`, and `$CHANGE_ACL`
    (Decision 13), the ACE text both ways, held to the probe. Its VMS round
    also asks A.4's "Open for a later round" questions.
11. **Changing profiles**: SET SECURITY with `/ACL` and its positional
    qualifiers, SET FILE, SET PROTECTION, SET ACL, CREATE, COPY, and
    BACKUP's `/PROTECTION` (Decision 10).
12. **Showing profiles**: SHOW SECURITY, SHOW ACL, DIRECTORY/SECURITY,
    /ACL, and the others, F$FILE_ATTRIBUTES; held to the probe.
13. **The other security services**: `$CHKPRO`, `$CHECK_ACCESS`,
    `$GET_SECURITY`, and `$SET_SECURITY` (Decision 13), their macros, held to
    the probe.
14. **The VMS round** (the review, answer 7): VMS reads a volume of files
    govax made under several identities, protections, and ACLs, and its
    DIRECTORY/SECURITY, SHOW SECURITY, and accesses agree with govax's.
15. **The close-out**: MANUAL.md (a section on file protection, the new
    commands and services, the configuration key), HELP for every new
    command and qualifier (`TestEveryVerbHasHelp`,
    `TestEveryQualifierHasHelp`), `VAX-VMS-INTERNALS.md`'s RMS and
    processes sections, DEVIATIONS.md, PLAN.md, and CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/prot93/` (its README has the files, the tree, and the steps for
the VAX run). One program, `prot93.mar`, takes cases from SYS$INPUT and
reports what RMS or the ACP did, and four procedures run it and DCL's
commands under each identity. Everything happens on an exchange volume,
mounted `/SYSTEM` so every user can reach it, and named by a system logical
name, `PROT93$DISK`, whatever unit it is on. The run changes nothing else
on the VAX but the accounts, their identifiers, and that name, which the
last procedure removes.

**The identities.** Three accounts (the review, answer 5), made by
`PROT93.COM` with AUTHORIZE: PROT93A `[200,1]`, which owns most of the
tree; PROT93B `[200,2]`, in its group; PROT93C `[300,1]`, the world. Group
identifiers PROT93G2 and PROT93G3, and a general identifier PROT93_TEAM
held by PROT93C, for ACEs. PROT93B and PROT93C may turn on READALL,
GRPPRV, SYSPRV, and BYPASS, and run the program in five passes: none, then
each alone. SYSTEM runs three: without the four (`M0`), READALL (`M1`),
all four (`M4`). The accounts take the place of the `$CREPRC` driver the
plan first had: real logins give real identities, and turning privileges on
and off gives the privilege passes. Each pass has a tag (`B3`), put in place
of each `@` in a case, so each pass makes its own files.

**The tree** (`[PROT93]`, mostly owned by `[200,1]`): the access matrix
`[.F]` (26 files: each category alone, each access alone, rotations, UIC 0,
and ACLs that grant, deny, give control, name a group, a wildcard UIC,
INTERACTIVE, SYSTEM, or two ACEs in order); directories of one access each
(`R`, `E`, `WE`, `RWE`, none, `W`); paths with one level's access taken
away; and each identity's own files to supersede, delete, rename, and
change (new versions over a previous one without W, past a version limit
without D on the oldest).

**The program's cases.** `prot93c.dat`, every pass: each `[.F]` file opened
with GET, PUT, both, and BIO, its XABPRO shown, and the ACP's read of its
attributes and accesses for read and write; each directory searched with a
wildcard and by name, parsed, opened, and created in; the paths; new files
in a directory owned by another (plain, and with XABPROs naming each
owner); a new file in the default protection ACE's directory.
`prot93d.dat`, once per identity: CIF, new versions, a version limit, a
supersede, erases, renames within and across directories, the ACP's
modify of protection and owner, delete, and create. `prot93e.dat`: the
bytes of every kind of ACE, from `$DISPLAY`'s XABPRO, which fix ods2's
layouts (Decision 8). A file of file IDs, written by SYSTEM, opens files by
ID in directories no one may read.

**SYSTEM's DCL** (`PROT93.COM`, every privilege): SHOW PROTECTION,
F$ENVIRONMENT, and SET PROTECTION/DEFAULT in seven forms, a file made after
each; new files' profiles through CREATE, COPY, APPEND, RENAME, and
CREATE/DIRECTORY (`/OWNER_UIC`, `PARENT`, `/PROTECTION`, top-level, a
UIC-named directory, under a default protection ACE), and a new version of
another owner's file; SET SECURITY with each qualifier, SET FILE, SET
PROTECTION on a file, SET ACL and SHOW ACL; F$FILE_ATTRIBUTES' items;
malformed protection codes, UICs, identifiers, and ACEs; every kind of ACE
on a file or directory of its own, an ACL changed step by step (`/AFTER`,
`/REPLACE`, `/DELETE`, `/DELETE=ALL`, protected and nopropagate ACEs through
`/LIKE` and `/DEFAULT`); DIRECTORY's and SHOW SECURITY's layouts.

**Part B, DCL's utilities** (`PROT93B.COM`, by SYSTEM without its four
privileges and by each account): TYPE, COPY, DUMP, SEARCH, DIFFERENCES,
OPEN and READ, F$FILE_ATTRIBUTES, F$SEARCH, DIRECTORY (`/FULL`,
`/SECURITY`), SHOW SECURITY, ANALYZE/RMS_FILE, CONVERT, BACKUP, MACRO, LINK,
and LIBRARY/LIST on files the owner alone, the group, and everyone may use;
DELETE, PURGE, RENAME, APPEND, COPY over, OPEN/WRITE and /APPEND, SET
SECURITY, SET FILE, SET PROTECTION, an ACE, and /OWNER on the identity's
own such files; directories it can't read or can only look up in, or can't
write; RUN of images and `@` of procedures with E, R, or no access.

**Each account's own** (`PROT93U.COM`): the ACL'd files typed; control
through an ACE; SET SECURITY/OWNER of its own file and another's; SET
PROTECTION/DEFAULT then a new file; CREATE/DIRECTORY in others'
directories; new files under the default protection ACE and in another
owner's directory; a new version of another owner's file.

**The end** (`PROT93Z.COM`): DIRECTORY/SECURITY of everything, so what each
identity made is seen as SYSTEM sees it, then the accounts removed.

**The definitions for the services.** `def_*.mar` assemble `$CHPDEF`,
`$OSSDEF`, `$ACEDEF`, `$ACLDEF`, `$ARMDEF`, `$KGBDEF`, and `$PRVDEF`
`GLOBAL` on the VAX, and `decode.go` turns the objects into `defined.txt`,
values only, as `testdata/idx88` did: the services' item codes and ACE
layouts govax's `vmsdef` lacks. The security services (Decision 13) are a
later round, written with subtask 10: SYSTEM's for the services, with one
account (PROT93C) for A.4's "Open for a later round" questions.

**Round 2** (A.4, "What round 1 lost"): `PROT93R.COM` (SYSTEM) makes the
accounts again (PROT93_TEAM with round 1's value, which the tree's ACEs
hold), gives PROT93.EXE and HELLO.EXE their protection one at a time, sets
`FACLCTL` back to round 1's, and adds round 2's files: `[.DRWO]` (RW, no
E), files to erase and rename in `[.DW]` and `[.DRWO]`, a file with
NOPROPAGATE ACEs and supersedes and a new version without W in `[.NV]`,
files to change the owner of in `[.CTL]`, and procedures with a line to
run, `PRO2x.COM`; it shows what SET PROTECTION, SET FILE/OWNER_UIC, and
SET FILE/PROTECTION with no value do, and SET SECURITY/DEFAULT on a
directory, and runs `prot93f.dat` (the new directory's cases) and
`prot93g.dat` (the new changing cases) as M0 and M4. `PROT93V.COM` (each
account) runs the passes round 1 lost, without privileges first, in a new
process (so RMS's directory cache is empty): `prot93c.dat`, `prot93f.dat`,
`prot93d.dat`, `prot93g.dat`, and the file IDs; then with GRPPRV where the
account has it; then owners (its own UIC again on its own file, another's
through CONTROL, CREATE/OWNER_UIC, CREATE/DIRECTORY/OWNER_UIC), a new file
under the default protection ACE, the NOPROPAGATE file's versions, and the
procedures. `PROT93Y.COM` lists the tree and removes the accounts, and the
group identifier PROT93 that round 1 left on the VAX.

**On govax**, the accounts' runs are replayed in detached processes made
by `$CREPRC` of LOGINOUT with the account's UIC (Decision 2), the
username set to the account's, so the logs compare. Part C's program runs
there the same way.

## The review (2026-11-14)

The author accepted the plan and answered its questions:

1. **ACLs:** the fuller support: identifier ACEs evaluated in every check,
   the default protection ACE and DEFAULT ACEs inherited, SET
   SECURITY/ACL with its positional qualifiers. The ACL editor, alarms, and
   audits stay out (Decision 10, subtasks 3, 7, 10, and 11).
2. **Security services:** all in this phase: `$SETDFPROT`, `$CHKPRO`,
   `$CHECK_ACCESS`, `$GET_SECURITY`, `$SET_SECURITY`, and with the ACLs
   `$PARSE_ACL`, `$FORMAT_ACL`, and `$CHANGE_ACL` (Decision 13).
3. **SET PROCESS/PRIVILEGES:** in this phase (subtask 5).
4. **Volume protection:** out of scope, logged for the identities phase.
5. **The probe's identities:** test accounts may be made on the simh VAX
   (Part D's PROT93A, B, and C; Part B2).
6. **Obsolete forms:** accepted exactly as VMS 7.1 accepts them (Decision 10).
7. **More probes:** the author will run further rounds wherever they help
   completeness, and the VMS round is subtask 14.

## Progress log

- **2026-11-14** Subtask 1: read the User's Manual's chapter 10, the DCL
  Dictionary's entries, and the system management books' ACE formats and
  RMS_FILEPROT; surveyed the process, corevms's UIC check, ods2, RMS, the
  DCL utilities, and the grammar (Part A). Found `$CREATE`'s XABPRO read
  and never applied, and the home block's default file protection used for
  every new file. Wrote Parts B to D and the questions. The author noted
  during the survey that this phase precedes identities other than
  SYSTEM, so the checks are always made, and added the VMS 6.0 *Guide to
  System Security* to the reference folder; its chapter 4 (A.1a) settled
  the order of the check, control access, READALL, the owner-change rule,
  a new version's and a directory's profiles, and creation's requirements,
  and Decisions 1, 4, 5, and 10 were revised to it. Stopped for the
  author's review.
- **2026-11-14** The review: see "The review". ACLs and every security
  service are in the phase (Decisions 10 and 13); Part C now has 15
  subtasks; Part D has test accounts on the VAX (Part B2), the replay of
  their runs in detached processes on govax, and an ACL part (Part E).
  Subtask 2, the probe, is next.
- **2026-11-14** Subtask 2, written: `testdata/prot93` (`prot93.mar` by
  hand; `gen.go`'s four procedures, three case files, `hello.mar`, the
  seven definition modules, and the govax scripts; `decode.go` from
  `testdata/idx88`'s). Part D changed as it was written: the accounts run
  the program themselves, privileges turned on and off for the passes, so
  no `$CREPRC` driver; the security services' item codes aren't in
  `vmsdef`, so round 1 brings their values back (`def_*.mar`) and the
  services are round 2. Rehearsed on govax as SYSTEM, from a copy of the
  exchange volume: the program assembles, links, and runs every case file
  under every tag, and each procedure runs to its end. The rehearsal found
  two mistakes of the probe's own, fixed: `$QIOW_S`'s P1 is an address
  (`P1=FIBD`), and F$GETJPI's CURPRIV and AUTHPRIV overflow a DCL token when
  every privilege is held (F$PRIVILEGE instead). What govax lacks is listed
  in the README; among it, the ACP's `ATR$C_ACLLENGTH` (`SS$_BADATTRIB`)
  and IO$_MODIFY by name (`SS$_BADPARAM`), both subtask 8's. A data line
  after a command DCL failed to parse runs as a command on govax; that is
  Phase 50's rule (data after one of DCL's own commands is a command),
  checked, not a bug. Waiting for the VAX run.
- **2026-11-14** Subtask 2, round 1 run by the author on VMS V7.1; every
  procedure reached its end, and the logs, the image, and the definition
  objects are in `testdata/prot93/vax/` (`defined.txt` decoded). A.4 is
  written from them: VMS follows the guide's check throughout, and the
  run settled the directory rules (a create needs R as well as W), the
  path rule, rename's and supersede's needs, the profiles of new files and
  directories (an explicit code merged into a base; NOPROPAGATE keeps a
  default ACE from subdirectories but not from files; no UIC from a
  directory's name), SET SECURITY's qualifiers and ACL edits, every form's
  errors and which part of VMS reports them, every display's layout, the
  ACE bytes, and each utility's message. Part B changed to it: Decisions
  4, 5, 7, 8, 9, and 10. Round 1 lost the accounts' program passes without
  privileges and with GRPPRV (the image's protection was given with a comma
  list, which 7.1's SET SECURITY refuses) and ran empty procedures (CREATE's
  data ended at the `$` line meant for them). Round 2 (Part D) runs those
  passes on the same volume with the cases round 1 left open, written in
  `gen.go` (round 1's procedures as they ran) and rehearsed on govax on a
  copy of the VAX's volume. Round 1 also left the group identifier PROT93
  on the VAX, which round 2's last procedure removes. The security services
  move to a later round with subtask 10. Waiting for round 2's VAX run.
- **2026-11-14** Round 2's first try: PROT93R.COM ran whole (its log is
  `vax/round2/prot93r.log`: the accounts, PROT93_TEAM at round 1's value,
  the images world RE, round 2's files, SYSTEM's passes), but the accounts
  were refused PROT93V.COM: govax's COPY onto the volume (exchange2.cmd)
  gave round 2's files SYSTEM's default protection, and nothing opened
  them as round 1's PROT93.COM had opened its own. PROT93Y.COM then removed
  the accounts. `PROT93S.COM` (gen.go, exchange3.cmd) makes the accounts
  again and opens each file they read, one SET SECURITY each; the accounts'
  runs go on from there.
- **2026-11-14** Subtask 2 done. Round 2 ran with `PROT93S.COM` in place of
  the accounts' first try; every log is in `vax/round2/` (`prot93r`,
  `prot93s`, `prot93v_prot93a`/`b`/`c`, `prot93y`). Its answers are in A.4,
  each marked "round 2": the unprivileged categories' whole matrix; GRPPRV
  as the system category for its own group and as privilege for owners in
  it; directory changes needing R and W (a create E not at all); a
  supersede needing W and D, a new version W; NOPROPAGATE ACEs dropped from
  a new version; the obsolete forms with no value; SET SECURITY/DEFAULT on a
  directory; CONTROL never taking ownership. It corrected one of round 1's
  findings: the ACP's read of attributes needs read or control (round 1's
  identities all held control), which also explains round 1's `FUIC0ACL`
  oddity (`[0,0]` gives the system category no control). Decisions 4 and 5
  changed to it. One question stays open: whether an execute-only
  procedure's lines run, since `PROT93S.COM` opened the procedures with
  the files the accounts needed (a mistake of the fix-up's); govax runs
  them, unconfirmed. Subtask 3, the rule, is next.
- **2026-11-14** Subtask 3 done: `internal/security`, a leaf package over
  `vmsdef`. `Check` and `Evaluate` are the guide's sequence (the first
  identifier ACE whose identifiers the process holds all of, DEFAULT ACEs
  ignored, a denial leaving the SYSTEM and OWNER fields; the categories'
  fields added together; `[0,0]`'s rule; control from ownership, the system
  category, an ACE, or BYPASS); `CanChangeOwner` is 4.8.4.2's list;
  `DecodeACL`, `NewIdentifierACE`, and `NewDefaultProtectionACE` are VMS's
  layouts. `TestProbeMatrix` replays the probe's whole `[.F]` matrix, 2,800
  answers under 14 identities from both rounds' logs (`FACLCTL`, changed
  between runs, is a case of its own), and found one thing A.4 hadn't said:
  READALL answers a request as a whole, giving read alone and adding nothing
  to the code's (a READALL process is refused `FW.DAT`, W for everyone, for
  read and write); BYPASS is the only privilege that adds. `TestProbeACEs`
  makes the probe's ACEs byte for byte as VMS's `$DISPLAY` returned them and
  decodes them back. Unconfirmed: an ACE's grant and the SYSTEM or OWNER
  field's add when each gives part of a request; GRPPRV's owner change needs
  the new owner in its group as well (round 2's B2); a UIC is held as its
  own resource identifier. `corevms`'s `uicAccess` now calls `Check`
  (`Process.Credentials`), so objects owned by `[0,0]` follow the guide's
  rule there too. Two fixes on the way: `uicprot.go` had a device's physical
  and logical I/O bits swapped against `$ARMDEF` (physical is bit 2), and
  `rights.go` NETWORK's and DIALUP's values (`$KGBDEF`: DIALUP `%X80000002`,
  NETWORK `%X80000005`). `vmsdef` gained `$ACEDEF`, `$ACLDEF`, `$CHPDEF`,
  `$ARMDEF`, `$KGBDEF`, and `$OSSDEF` from `defined.txt` (`gen -values`),
  but not its `$PRVDEF` lines, whose display flags share the privilege bit
  numbers. The author asked for `vax.sysgen.maxsysgroup`; it is in Decision
  3 and subtask 5. Subtask 4, ods2, is next.
- **2026-11-14** Subtask 4 done: ods2 v0.1.29, which govax now pins.
  `ondisk/ace.go` is the ACE layouts A.4 found (`ACE`, `DecodeACE` and
  `Encode`, `DecodeACL`/`EncodeACL`, `FileHeader.ACEs`; identifier,
  default protection, creator, alarm, and audit ACEs, an unknown type
  kept whole), `Identifier` (a UIC or general identifier), and the three
  inheritance rules of A.4's "New files' profiles" as ACL filters:
  `ACLForNewVersion` (NOPROPAGATE dropped), `ACLFromDirectoryDefaults`
  (the DEFAULT ACEs, DEFAULT cleared, NOPROPAGATE kept, the default
  protection ACE not copied), and `ACLForSubdirectory`. Its tests hold the
  encoding to the bytes VMS's `$DISPLAY` returned in round 1 (`ACE01` to
  `ACE20`, `DP.DIR`). `ondisk.Uic` has its wildcards (`WildGroup`,
  `WildMember`, `Matches`, `ParseUicPattern`; `ParseUic` still refuses
  them). In `volume`, `NewFileHeader` and `DirectoryOptions` take an ACL,
  and `CreateFileVersionWith` takes a `NewFileProfile` (owner,
  protection, ACL), `CreateFileVersion` being it with nil; and
  `InheritedDirectoryOptions` now gives a new directory its parent's ACL
  less NOPROPAGATE entries, so govax's CREATE/DIRECTORY inherits ACLs from
  now on (the rest of Decision 4 is subtask 7's). `internal/security`'s
  own ACE code stays: it is a leaf over `vmsdef`, and ods2's types are the
  file system's. Subtask 5, identity and defaults, is next.
