# Phase 93 — file protection

**Status:** planned 2026-11-14; the plan accepted by the author the same
day, with the answers in "The review". Subtask 1 (the survey, Part A) is
done; nothing is implemented yet.

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

To be filled in from the VAX run (subtask 2).

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

**Decision 4 — a new file's profile, in one function.** `rms`'s
`newFileProfile(creds, dir, name, explicit)` is the only place a new
file's owner, protection, and ACL are chosen, by A.1's order:

- **Protection of a data file:** explicit, else the previous version's (the
  highest existing version, unconfirmed), else the directory's default
  protection ACE, else the process default.
- **ACL of a data file:** the previous version's, else the directory's
  DEFAULT ACEs with DEFAULT removed and NOPROPAGATE ones skipped.
- **Owner:** the creator's UIC; with extended privileges (a system UIC,
  SYSPRV, BYPASS, or GRPPRV in that owner's group), the previous version's
  owner, else the directory's. A XABPRO's owner is explicit, allowed by
  `CanChangeOwner`; what a refused one does is A.4's.
- **Directories:** the parent's protection less delete (from the MFD for a
  top-level one), the parent's whole ACL but NOPROPAGATE ACEs, and the
  owner by the same rule, or the UIC a directory named `[g,m]` gives.
  `volume.InheritedDirectoryOptions` already does the protection.
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
| run an image | E (A.4) | R or E |
| `$SEARCH` with a wildcard, DIRECTORY | | R |
| `$SEARCH` of a known name, `$PARSE` with a lookup | | R or E |
| create | | W |
| new version | W on the previous version; D on the oldest past the limit | W |
| supersede | A.4 | W |
| delete | D | W |
| rename | A.4 | W on both (A.4) |
| change protection, ACL | control | |
| change owner | control and `CanChangeOwner` | |
| read attributes (`$DISPLAY`, F$FILE_ATTRIBUTES, DIRECTORY/FULL) | A.4 | R or E |

The levels of a path above the file's directory, and the volume's own
protection, are A.4's and out of scope (the review, answer 4).

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
`RMS$_PRV` with STV `SS$_NOPRIV`; the ACP `SS$_NOPRIV`; the image
activator's error as A.4 finds; each DCL utility its own message over
`-RMS-E-PRV` (`%TYPE-W-OPENIN`, `%DELETE-W-FILNOTDEL`, ...), each held to
the probe's log. Texts come from `vmsdef.Messages`; any missing message is
added through `internal/vmsdef/gen`.

**Decision 8 — ods2 changes, in a tagged release.** In `ods2`:

- `CreateFileVersion` (and `CreateFile`) take an optional profile (owner,
  protection, ACL), by a new `CreateFileVersionWith(..., NewFileProfile)`
  or an options argument; a nil profile keeps today's behavior, so the
  `ods2` command-line tool and its tests don't change.
- `ondisk` learns the ACE layouts it already stores, with encode and
  decode for the identifier, default protection, creator, alarm, and audit
  ACEs and their flags (DEFAULT, PROTECTED, NOPROPAGATE, HIDDEN), and
  `ondisk.Uic`'s wildcard forms for identifier ACEs. The layouts come from
  BACKUP's saved ACLs and the probe's files, as `rms/directory.go`'s
  `aceText` already reads identifier ACEs.
- No access checks in ods2: it knows nothing of processes. Its
  `ParseProtection` and `FormatProtection` stay the shared syntax.

Tagged and pinned as Phase 22's note describes.

**Decision 9 — grammar types `$PROTECTION` and `$UIC`.** `internal/dcl`
gets two value types. `$PROTECTION` reads a protection code as one value
(parenthesized or a single category), checks it at parse time, and hands
the handler a parsed value: the mask *and* which categories were given,
so a handler applies it to a base (`ondisk.ParseProtection`'s rule).
`$UIC` reads `[g,m]` (octal), `[name]`, or `[group,name]` and an
identifier name, resolved through the rights database when the command
runs. A malformed value is a parse error with VMS's message (A.4). Every
qualifier that takes one moves to the type: CREATE/DIRECTORY's, BACKUP's
`/BY_OWNER` and `/PROTECTION`, and the new commands'.

**Decision 10 — the commands.**

- **SET SECURITY** `file[,...]` with `/PROTECTION`, `/OWNER`, `/DEFAULT`,
  `/LIKE`, `/COPY_ATTRIBUTE`, `/LOG`, `/CONFIRM`, `/EXCLUDE`, `/BY_OWNER`,
  `/CLASS=FILE` (other classes refused), and the date selectors, with
  control access, and for `/OWNER` the guide's rule (`CanChangeOwner`,
  which is wider than the Dictionary's "GRPPRV ... SYSPRV": a system UIC,
  BYPASS, and the volume owner's UIC qualify too; A.4 settles it).
- **SET SECURITY/ACL**`=(ace[,...])` with `/AFTER`, `/DELETE[=ALL]`, and
  `/REPLACE`, every ACE form the ACL editor's chapter gives (identifier,
  default protection, creator, alarm, audit; their OPTIONS), protected and
  hidden ACEs' rules; the text of an ACE read and written by `$PARSE_ACL`'s
  and `$FORMAT_ACL`'s code (Decision 13), so DCL and programs share it.
  The ACL editor (`/EDIT`, EDIT/ACL) stays out.
- **SHOW SECURITY** `file[,...]`, VMS's layout, ACEs included.
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
2. **The probe** (written and rehearsed 2026-11-14; waiting for the VAX
   run): `testdata/prot93`, as in Part D, written and rehearsed on govax;
   run on the VAX by the author; A.4 filled in and Part B adjusted to its
   answers. Later rounds as later subtasks find gaps (the
   review, answer 7), each a section of A.4.
3. **The rule**: `internal/security` (Decision 1), the guide's whole
   sequence with ACLs, `CanChangeOwner`, table-driven tests from A.1a and
   A.4; `uicAccess` on it, the mailbox, section, and event flag tests
   unchanged.
4. **ods2**: profiles at creation and the ACE layouts (Decision 8), a
   tagged release, govax pinned to it.
5. **Identity and defaults**: `Process.Credentials`, `DefaultProtection`
   and `vax.sysgen.rms_fileprot` (in `validConfigs` and HELP), inheritance
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
    (Decision 13), the ACE text both ways, held to the probe.
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

**The definitions for round 2.** `def_*.mar` assemble `$CHPDEF`, `$OSSDEF`,
`$ACEDEF`, `$ACLDEF`, `$ARMDEF`, `$KGBDEF`, and `$PRVDEF` `GLOBAL` on the VAX,
and `decode.go` turns the objects into `defined.txt`, values only, as
`testdata/idx88` did: the services' item codes and ACE layouts govax's
`vmsdef` lacks. Round 2 is the security services (Decision 13), written
once these values are in.

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
