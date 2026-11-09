# Phase 91 — concealed logical names and rooted directories

**Status:** planned 2026-11-09; awaiting the author's review. Subtask 1
(the survey, Part A below) is done. Nothing is implemented yet.

## Goal

govax handles concealed logical names as VMS does, including the case it
has never had: a **rooted-device logical name**, whose equivalence string
ends its directory in a period and which makes that directory look like
a volume's master file directory.

    $ DEFINE/TRANSLATION_ATTRIBUTES=(CONCEALED,TERMINAL) BASE _DUA1:[ROOT1.]
    $ DIRECTORY BASE:[SUB]            ! really DUA1:[ROOT1.SUB]

    Directory BASE:[SUB]

Then the system names use one. `SYS$SYSROOT` becomes `_DUA0:[SYSTEM.]`,
concealed, and `SYS$SYSTEM` and the others are `SYS$SYSROOT:[SYSEXE]` and
so on, as VMS's are. VMS's root is `[SYS0.]`; govax's is `[SYSTEM.]`,
because it has one system and no need for `SYS0`, `SYS1`, and the
common root (the author, 2026-11-09). The directories on the system
volume don't change.

Out of scope: the `CONCEAL_DEVICES` SYSGEN parameter, node names, the
`[g,m]` directory form, and `SYS$COMMON`/`SYS$SPECIFIC` unless the review
asks for them (Question 1).

## Part A — the survey (2026-11-09)

### A.1 What VMS's manuals say

*Guide to OpenVMS File Applications* (7.3 text), sections 5.7, 6.2, and
6.3, is the main source; the *RMS Reference Manual*'s NAM chapter, the
*User's Manual*'s 11.3.4 and Table 11-5, and the DCL Dictionary fill it
in.

**Concealed names (5.7, 6.2).** RMS shows the logical name instead of the
device. RMS's translation loop: "if no concealed-device logical names
have been encountered previously", the source string is used as the
device, so the *outermost* concealed name is the one shown; translation
then goes on unless the equivalence is terminal. `SET DEFAULT
USERDISK:[JONES]` then `DIRECTORY` is headed `USERDISK:[JONES]`.

**Rooted names (6.3.3).** A root directory is marked by a trailing period
and "you can only use a trailing period for the root directory". The
named directory appears as the MFD; its subdirectories appear as
top-level directories.

**The guide's table (6.3.4), with `X = DJB3:[SMITH.]` and default
`[JONES]`:**

| Specification | Directory reached |
| --- | --- |
| `X:` | `[SMITH.JONES]` |
| `X:[000000]` | the root, `[SMITH.]` |
| `X:[]` | `[SMITH.JONES]` |
| `X:[-]` | the root, listed as `X:[000000]` |
| `X:[--]` | invalid |
| `X:[name]` | `[SMITH.name]` |
| `X:[.name]` | `[SMITH.JONES.name]` |
| `X:[name.*...]` | every tree below `[SMITH.name]` |
| `X:[*]` | every directory one level below the root |
| `X:[*...]` | every directory below the root |
| `X:[...]` | every directory below `[SMITH.JONES]` |

Other rules from the same place:

- The default directory is applied **below** the root: RMS translates
  the name to device and root first, and merges the rest in after.
- Nothing above the root can be reached. `[-]` from the root is
  `%RMS-E-DIR`.
- Directory concatenation within one specification happens only with a
  rooted name.
- On a VAX, the depth is 8 directory levels, "with rooted logical, 16"
  (the guide's table of limits).

**The NAM block.** FNB has `NAM$V_ROOT_DIR`, "device name incorporates a
root directory". `NAM$V_NOCONCEAL` puts "its physical device name (and
directory, if so defined)" in the resultant string. `DIR_LVLS` and the
wildcard fields count from "the device or root directory". The manual
writes a name with a root as `DKB100:[ROOT1.ROOT2.][*.SUBDIR1.SUBDIR2]`.

**F$PARSE.** `NO_CONCEAL` "ignores the conceal attribute"; translation
doesn't end at a concealed name.

**The system names (User's Manual, Table 11-5).** `SYS$SYSTEM`,
`SYS$HELP`, `SYS$LIBRARY`, `SYS$MANAGER`, and `SYS$SHARE` are each
`SYS$SYSROOT:[dir]`.

**Where the manuals disagree or stop:**

- The Dictionary says a concealed name's equivalence "must be a physical
  device name", yet its SET DEFAULT example is `DEFINE/TRANS=CONCEALED
  XYZ WORK:[INVOICES.]`, and the guide has rooted names with no
  `/TRANSLATION_ATTRIBUTES` at all (`DEFINE BASE DUA7:[ROOT1.]`).
- Nothing says whether `[ROOT.][DIR]` can be typed, only that RMS
  produces it, or what a non-concealed `[ROOT.]` does.

The probe settles these (Part D).

### A.2 What VMS output is already in the repository

- `testdata/link68/vax/link68.log` and `testdata/mp/probe5/vax/probe5.log`
  show VMS's image activator naming a missing image
  `SIMVAX$DUA0:[SYS0.SYSCOMMON.][SYSLIB]SHR1.EXE;` and
  `SIMVAX$DUA0:[SYS0.SYSCOMMON.][000000]SHR1.EXE;`: the physical device,
  the root, and the directory, with `.][` between them.
- `testdata/tpa80` has `$FILESCAN` accepting `[ROOT.][DIR]A` and
  `[ROOT.]A`, and `corevms/filescan.go` already reports the root as its
  own field (Phase 80). That's syntax only: `$FILESCAN` looks at no
  logical names.
- `TestLink68Oracle` masks `SIMVAX$DUA0:[SYS0.SYSCOMMON.]` to `DUA0:`
  because govax has no rooted `SYS$SYSROOT`. That mask should go.

### A.3 What govax does today

Translation itself is sound: `lnm.TranslateFileSpec` follows the guide's
loop, keeps the outermost concealed name, stops at a terminal
equivalence, fans out search lists, and joins equivalence and remainder
as text. Joined text is exactly VMS's form: `SYS$SYSTEM:X` with `SYS$SYSTEM
= SYS$SYSROOT:[SYSEXE]` and `SYS$SYSROOT = _DUA0:[SYSTEM.]` becomes
`_DUA0:[SYSTEM.][SYSEXE]X`. **`internal/lnm` needs no change for roots.**

The gap is in what reads that text. There are two name processors, and
neither knows about roots:

1. **`rms/name.go`'s `expandName`**, for the system services: `$PARSE`,
   `$SEARCH`, `$OPEN`, `$CREATE`, the image locator, DCL's OPEN, F$SEARCH.
   `scanDir` refuses `[ROOT1.]` as `RMS$_DIR`, so by reading the code a
   rooted name fails outright here.
2. **`rms/logicals.go`'s `expandSpec`**, for the DCL utilities: DIRECTORY,
   TYPE, COPY, DELETE, PURGE, RENAME, CREATE/DIRECTORY, BACKUP, ANALYZE,
   DUMP, F$PARSE. It goes through ods2's `filespec.Parse`, which is more
   tolerant, and that is the dangerous part: it **quietly reaches the
   wrong directory**.

Run on a scratch volume, today's build
(`BASE` = `_DUA2:[ROOT1.]` concealed and terminal; `CDISK` = `DUA2:`
concealed):

| Command | Today | VMS, per A.1 |
| --- | --- | --- |
| `DIRECTORY BASE:[SUB]` | reads top-level `DUA2:[SUB]`, shown as `BASE:[SUB]` | `DUA2:[ROOT1.SUB]` |
| `DIRECTORY BASE:[000000]` | lists the volume's real MFD | lists `[ROOT1]` |
| `CREATE/DIRECTORY BASE:[NEWD]` | **makes `DUA2:[000000]NEWD.DIR`** | makes `[ROOT1.NEWD]` |
| `F$PARSE("BASE:[SUB]A.DAT")` | `BASE:[SUB]A.DAT;` (right text, wrong directory checked) | the same text |
| `F$PARSE("BASE:A.DAT")` | empty | `BASE:[default]A.DAT;` |
| `F$PARSE("DUA2:[ROOT1.][SUB]A.DAT")` | empty | probe |
| `F$PARSE("BASE:[SUB]A.DAT",,,,"NO_CONCEAL")` | `DUA2:[SUB]A.DAT;` | probe; root is lost today |
| `SET DEFAULT CDISK:[ROOT1]`, `DIRECTORY` | **headed `DUA2:[ROOT1]`** | headed `CDISK:[ROOT1]` |

The last row is a bug in **plain** concealed names, with no root
involved: `expandSpec` takes the display name only from the primary
name's translation, and loses the concealed name of `SYS$DISK` when the
device comes from there. `expandName` handles the same case correctly
(`disk.Concealed`).

Also missing or affected:

- **`NAM$V_ROOT_DIR`** is defined in `vmsdef` and never set.
- **`DVI$_CONCEALED`** is defined and not answered by `$GETDVI` or
  F$GETDVI (`corevms/getdvi.go`, `itemtext.go`).
- **Printing names.** About a dozen places build a shown name as
  `r.Display + ":" + dirSpec{Elems: m.Dirs}.String()`: `directory.go`,
  `delete.go`, `dump.go`, `analyze.go`, `fileattr.go`, `lexical.go`,
  `recordfile.go`, `saveset.go`, `imagefile.go`, `edit.go`, and
  `renamecmd.go`'s and `createdir.go`'s `display`. Each has to show the
  directory *relative to the root*.
- **`$SEARCH`** walks directories from the MFD (`searchDirs`,
  `walkSearch`) and writes resultant strings from the paths it finds.
- **Text fed back in.** `FindImageFile` (`shareable.go`) passes a
  translated `f.Spec` as a default name, and `LocateRelated`
  (`location.go`) recurses on one. So the scanner must accept `.][`
  whether or not VMS lets a user type it.
- **`ods2`**: `filespec.CreateDirectoryPath` stops at
  `MaxDirectoryDepth = 8`. A root plus directories can go deeper on VMS.
- **`PhysicalDevice`** (`$ASSIGN`, `$GETDVI`, MOUNT, DISMOUNT) and
  `Session.SetDefault` also parse translated text.
- **`sysvol`'s constants** (`[SYSTEM.SYSEXE]` and the rest) are physical
  paths used with `DUA0:`. They stay as they are, and nothing about the
  volume's layout or the manifest changes.

## Part B — decisions (proposed)

**Decision 1 — the system root is `[SYSTEM.]`** (the author, 2026-11-09).
`SYS$SYSROOT` is `_DUA0:[SYSTEM.]`, concealed and terminal, in the system
table at executive mode. It is one name, not VMS's search list of
`[SYS0.]` and `SYS$COMMON`.

**Decision 2 — one model: device, root, directory.** A name is a device,
a root (a list of directory names, usually empty), and a directory
*relative to the root*. Both processors parse the logical directory
exactly as they do now, against a logical default directory, and only
prepend the root when they go to the volume. That makes "you can't get
above the root" automatic, since `applyDir` and `filespec.Parse` never see
it, and `[-]` at the top fails with the error they already give.

- In `expandName`: `fileName` gains `Root`, `scanName` recognizes
  `[ROOT.]` and `[ROOT.][DIR]`, `parsedName.DirSpec` stays logical, and
  `parsedName.spec()` returns root + directory.
- In `expandSpec`: `resolvedSpec` gains `Root`, and `Spec.Dirs` is root +
  directory, so everything below it (ods2, the volume walks) works
  unchanged. `filespec.Spec` is not changed.

**Decision 3 — no change to `internal/lnm`** beyond whatever the probe
finds wrong. Roots are a matter of reading the translated text.

**Decision 4 — keep two name processors.** Merging `expandSpec` into
`expandName` is the right long-term shape, but it would touch every DCL
utility and isn't needed here. Both call one shared function that splits
a root off directory text, so the syntax rules exist once. If the probe
shows the two can't be kept in step cheaply, I'll raise it.

**Decision 5 — one helper for showing a directory.** A `resolvedSpec`
method gives the shown directory from a physical path, by removing the
root and showing the root itself as `[000000]`. All the sites in A.3 use
it, so a new command can't get it wrong by copying the old expression.

**Decision 6 — the system names, as VMS's.** `defineSystemNames` defines
the rooted `SYS$SYSROOT`. STARTUP.COM's names become
`define/system/executive_mode sys$system sys$sysroot:[sysexe]` and so on.
Consequences:

- Listings of system files are headed `SYS$SYSROOT:[SYSEXE]`, not
  `DUA0:[SYSTEM.SYSEXE]`.
- `[SYSTEM]`'s own files are `SYS$SYSROOT:[000000]GOVAX$FILES.DAT`. The
  help's `SYS$SYSROOT:GOVAX$UPDATE.LOG` would now mean the *default*
  directory under the root, so the text changes.
- STARTUP.COM is a supplied file, so existing volumes get the new copy
  at their next start, by the refresh. A STARTUP.COM the user changed is
  kept, with `SYSKEPT`, and its `sys$sysdevice:[system.sysexe]` names go
  on working.
- `SYS$SYSDEVICE` keeps its name and its use; its attributes follow what
  the VAX shows (Phase 87 left them unconfirmed).

**Decision 7 — depth.** RMS's check applies to the root and the directory
separately, by whatever limits the probe finds. If VMS really allows more
than 8 physical levels, `ods2`'s `MaxDirectoryDepth` is raised in a
tagged `ods2` release, as Phase 22's note on `go.work` describes.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey** (done 2026-11-09): Part A.
2. **The probe**: `testdata/lnm91`, as in Part D; run on the simh VAX by
   the author; "The probe's answers" added to Part A and any decision it
   changes revised.
3. **Syntax and `$PARSE`**: `.][` and `[ROOT.]` in `scanName`, the root
   through `expandName`, the guide's table, errors, `NAM$V_ROOT_DIR`,
   `DIR_LVLS`, `NOCONCEAL`. Held to the probe's `$PARSE` cases, in an
   oracle test.
4. **Files through the services**: `$OPEN`, `$CREATE`, `$SEARCH` with
   wildcards (`[*]`, `[*...]`, `[...]`), `$ERASE`, `$RENAME`, resultant
   strings, related names, a search list of roots.
5. **The DCL utilities**: `expandSpec` and the display helper, every
   command in A.3, SET and SHOW DEFAULT with a rooted device, F$PARSE,
   F$SEARCH, and the `SYS$DISK` concealment bug. Held to the probe's DCL
   log.
6. **Devices and the rest**: `PhysicalDevice`, `$ASSIGN`, `$GETDVI` and
   `DVI$_CONCEALED`, the image activator's message and
   `JPI$_IMAGNAME`, `LIB$FID_TO_NAME`, BACKUP, and the depth (with an
   `ods2` release if Decision 7 needs one).
7. **The system names**: `defineSystemNames`, STARTUP.COM, the refresh
   on an existing volume, removing `TestLink68Oracle`'s mask and the
   tests' plain `SYS$SYSROOT`, and running the system volume's tests.
8. **The close-out**: MANUAL.md (the rooted-names paragraph, "The system
   volume", the "no rooted names" line in the list of gaps), HELP DEFINE,
   ASSIGN, SET DEFAULT, F$PARSE, and SYSTEM_VOLUME, `sysnames.go`'s and
   Phase 87's notes, PLAN.md, and CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/lnm91/`, in the usual layout: `gen.go` writes `lnm91.mar`,
`lnm91.com`, `exchange.cmd`, and `copyout.cmd`, with the README giving
the steps. It runs on an exchange volume on DUA5, makes its own tree
under `[LNM91]`, and needs no privilege beyond what `DEFINE/PROCESS`
wants, so the VAX's system tables aren't changed. Results go in `vax/`.

**`lnm91.mar`, the services.** A table of cases, each reporting status,
STV, expanded and resultant strings, FNB in hex with `ROOT_DIR` and
`CNCL_DEV` named, each component's length, and DVI, DID, and FID. Every
case runs with and without `NAM$V_NOCONCEAL`, and with `NAM$V_SYNCHK`
where that differs.

| Group | Cases |
| --- | --- |
| Guide's table | each row of A.1, against `$PARSE` and `$SEARCH` |
| Forms of the name | concealed and terminal; concealed only; **not concealed**; terminal only; `_DEV:` and `DEV:`; logical over physical |
| Chains | `A = ROOT:[SUB]`; `A = ROOT:[SUB.]` (a root on a root); concealed over rooted; a list of roots |
| Syntax | typed `DEV:[ROOT.][DIR]F`, `DEV:[ROOT.]F`, `[ROOT.][000000]F`, `[ROOT.][.DIR]`, `[ROOT.][-]`, `<ROOT.>[DIR]`, `[A.][B.]`, `[A...][B]`, `[R*.]`, `[A..B.]` |
| Defaults | rooted as default name, as related file, and as `SYS$DISK`; a physical default directory with a rooted primary |
| Depth | root and directory sums of 7, 8, 9, 15, 16, 17 levels; CREATE/DIRECTORY at each |
| Rendering | `[000000]`, `[000000.SUB]`, `[.SUB]` from the root, `[-]` from a child; `DIR_LVLS` |
| Files | `$CREATE`, `$OPEN`, `$ENTER`, `$REMOVE`, `$ERASE`, `$RENAME` across roots, wildcard `$SEARCH` over `[*]`, `[*...]`, `[...]`, including where each walk ends |
| Devices | `$ASSIGN` and `$GETDVI` of a rooted name: DEVNAM, FULLDEVNAM, `CONCEALED` |
| Names back | `LIB$FID_TO_NAME`, `JPI$_IMAGNAME`, for a file reached through a root |

**`lnm91.com`, DCL under SET VERIFY:**

- SHOW LOGICAL/FULL of `SYS$SYSROOT`, `SYS$SYSDEVICE`, `SYS$COMMON`,
  `SYS$SPECIFIC`, `SYS$SYSTEM`, `SYS$LIBRARY`, `SYS$SHARE`, and a
  `DISK$` name.
- DIRECTORY (headings, `/FULL`, `/TOTAL`) of a rooted device, of
  `SYS$SYSTEM:`, and of a plain concealed device after SET DEFAULT.
- SET DEFAULT and SHOW DEFAULT: rooted with a directory, device only,
  `[-]` up to the root, `[-]` past it, a list of roots.
- CREATE, CREATE/DIRECTORY, COPY (the messages), RENAME, DELETE/LOG,
  PURGE/LOG, TYPE, OPEN, DUMP's heading, ANALYZE/RMS_FILE's.
- F$PARSE with every field and `NO_CONCEAL`, F$SEARCH, F$TRNLNM with
  `CONCEALED`/`TERMINAL`, F$GETDVI, F$FILE_ATTRIBUTES.
- BACKUP/LIST of a save made through a rooted input.
- RUN of a missing image through a root (the IMAGEFNF text).

The probe prints a status at every step, resets NAM fields between
cases, and has nothing to wait on.

## Questions for the review

1. **`SYS$COMMON` and `SYS$SPECIFIC`.** VMS has them and govax needn't.
   VMS procedures sometimes name `SYS$COMMON:[SYSMGR]`, and both could be
   defined equal to `SYS$SYSROOT` for that one reason. *Recommended:
   leave them out, and add them if a real procedure needs them.*
2. **`SYS$SOURCES`**, govax's own name, would become
   `SYS$SYSROOT:[SOURCES]`, like the others. *Recommended: yes.*
3. **Merging the two name processors** (Decision 4). *Recommended: not
   in this phase.*
4. **`CONCEAL_DEVICES`** is left out. *Recommended: leave it out; VMS's
   default is on, which is the only behavior govax would have.*

## Progress log

- **2026-11-09** Subtask 1: read the manuals and the VMS logs in the
  repository, ran today's build against a concealed root on a scratch
  volume, and wrote this plan. The run found that a rooted name is
  silently taken as its directory part alone
  (`CREATE/DIRECTORY BASE:[NEWD]` makes a top-level directory), and that
  a concealed `SYS$DISK` is shown by its physical device in DIRECTORY's
  heading. Stopped for the author's review. The author set the root to
  `[SYSTEM.]` during the survey.
