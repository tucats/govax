# Phase 91 — concealed logical names and rooted directories

**Status:** planned 2026-11-09; the plan and the answers to its questions
accepted by the author the same day ("The review"). Subtask 1 (the survey,
Part A below) and 2 (the probe, two rounds on the VAX; A.4, "The probe's
answers") are done. Nothing is implemented yet.

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

`SYS$COMMON` and `SYS$SPECIFIC` are defined too, for procedures and
programs that assume them, both naming the one root.

Along the way, RMS's two name processors become one, so that this and
later changes to file names have one code path (Decision 4).

Out of scope: the `CONCEAL_DEVICES` SYSGEN parameter and node names. The
`[g,m]` directory form is a future enhancement (the author, 2026-11-09);
one name processor gives it one place to go. (The guide says it isn't
allowed in a rooted directory.)

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
   `scanDir` refuses `[ROOT1.]`, so a rooted name fails outright here:
   the probe's rehearsal on govax had `$PARSE` of `BASE:[SUB]A.DAT` return
   `RMS$_DIR`.
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

### A.4 The probe's answers (2026-11-09)

`testdata/lnm91/vax/lnm91.log`, VMS V7.1, 453 program cases and the DCL
commands, ran to the end. The guide's table (A.1) is right in every row.
What the manuals didn't settle:

**What makes a root.** A directory ending in a period, and nothing else.
Concealment isn't needed, whatever the Dictionary says: `NOCON` (no
attributes), `TERM`, and a typed `LDEV:[LNM91.ROOT1.][SUB]A.DAT` all work.
A root can be typed, with or without a device: `[LNM91.ROOT1.]A.DAT` is
that root with the default directory, `[LNM91.ROOT1.][JONES]`.

**How names show.**

| Name | Expanded and resultant strings | CNCL_DEV | ROOT_DIR |
| --- | --- | --- | --- |
| `BASE:[SUB]A.DAT`, concealed root | `BASE:[SUB]A.DAT;` | 1 | 1 |
| the same with `NOCONCEAL` | `_DUA1:[LNM91.ROOT1.][SUB]A.DAT;` | 1 | 1 |
| `NOCON:[SUB]A.DAT`, root not concealed | `_DUA1:[LNM91.ROOT1.][SUB]A.DAT;` | 0 | 1 |
| `CDISK:[LNM91]A.DAT`, concealed, no root | `CDISK:[LNM91]A.DAT;` | 1 | 0 |

- The device text in `.][` form is the equivalence string's as written
  (`_DUA1:`, `DUA1:` for `PHYS`, `SIMVAX$DUA0:`). `NAM$B_DIR` is the whole
  of `[ROOT.][DIR]`.
- `DIR_LVLS` and the wildcard fields count from the root.
- A name through a plain name shows the concealed one it reaches:
  `A1:A.DAT` (`A1` = `BASE:[SUB]`) is `BASE:[SUB]A.DAT;`. A concealed name
  over a root shows itself: `A3:[SUB]A.DAT;`.
- A list shows each element's name: `LIST:` (`BASE:,BASE2:`) finds
  `BASE:[SUB]A.DAT;1` then `BASE2:[SUB]A.DAT;1`; `LIST2:` (concealed
  itself) shows `LIST2:` for both.
- Brackets are kept as written: `_DUA1:<LNM91.ROOT1.>[SUB]A.DAT;`.

**Errors.**

| Case | Status |
| --- | --- |
| A directory after a name whose translation already has one: `A1:[OTHER]A.DAT`, `A1:[.DEEP]A.DAT` (`LIST3`'s `CDISK:[LNM91]` element too) | `RMS$_DIR` |
| `[--]` from one level below the root, `[-]` at the root | `RMS$_DIR` |
| A wildcard in a root (`[R*.]`), or a relative one with `-` (`[-.A.][B]`) | `RMS$_DIR` |
| A second root after a real one, typed: `[A.][B.]`; `[A...][B]`; `[A..B.]` | `RMS$_SYN` |
| A second root reached through a concealed name: `BASE:[A.]X.DAT`, and `A2` = `BASE:[SUB.]` | `RMS$_DEV` |
| A concealed name that translates to more than a device and root: `A4` = `BASE:[SUB]` | `RMS$_DEV` |
| `[.A.][B]`, `[.][SUB]` | parse, keep the text, and end `RMS$_DNF` |
| `[000000.][SUB]` | a root; `_DUA1:[000000.][SUB]A.DAT;` |

**Depth.** A root may have at most **7** levels (`DEEP8`, 8 levels, is
`RMS$_DEV`), and the directory after it at most 8 (a ninth is `RMS$_DIR`).
So up to 15 physical levels: the guide's 16 is one too many, and Phase 58's
"seven more in a root" was right.

**Defaults.** The root goes with the device. `LDEV:A.DAT` with default
name `BASE:[SUB]` is `_DUA1:[SUB]A.DAT;`, no root; `A.DAT` with that
default is `BASE:[SUB]A.DAT;`. A related file's root comes with its
device likewise. A relative directory applies to the process default,
under whichever root the device gives (`[.K]A.DAT` with `BASE:[SUB]` is
`BASE:[JONES.K]A.DAT;`), as it does without roots.

**SYS$DISK on a root** behaves as the guide says, `[.SUB]` from the root
being `BASE:[000000.SUB]`, as `applyDir` already writes it.

**Wildcards.** `$SEARCH` walks below the root as it walks below an MFD,
and ends with the same `RMS$_NMF` marks (`BASE:[.......]*.DIR;1`,
`BASE:[SUB]*.DIR;1`). `[000000...]` shows `[000000]`, `[000000.JONES]`,
and so on.

**Devices.** `$ASSIGN` and `$GETDVI` of every form, even `A2` and `A4`,
which RMS refuses, reach the device. **`DVI$_CONCEALED` and F$GETDVI's
`CONCEALED` are FALSE for every name here**, concealed or not.

**Names back.** `JPI$_IMAGNAME` is physical with the root:
`_DUA1:[LNM91.ROOT1.][SUB]LNM91.EXE;1`. `LIB$FID_TO_NAME` is the whole
path, `DISK$LNM91:[LNM91.ROOT1.SUB]A.DAT;1`. `IMAGEFNF` names the physical
form: `_DUA1:[LNM91.ROOT1.][SUB]NOSUCH.EXE;`.

**DCL.**

- Every utility shows the name RMS gives: `Directory BASE:[SUB]`,
  `Directory _DUA1:[LNM91.ROOT1.][SUB]` for a root not concealed,
  `%COPY-S-COPIED, BASE:[NEWD]C.DAT;1 copied to _DUA1:[LNM91]E.DAT;1`,
  `File Spec: BASE:[SUB]A.DAT;1` (ANALYZE).
- `SET DEFAULT CDISK:[LNM91.ROOT1]` then `DIRECTORY` is headed `CDISK:`,
  as the guide says (the `SYS$DISK` bug of A.3).
- F$PARSE follows `$PARSE`. With `NO_CONCEAL`, `DEVICE` is `_DUA1:` and
  `DIRECTORY` is `[LNM91.ROOT1.][SUB]`.
- SET DEFAULT keeps the concealed name (`BASE:[SUB]`, `SYS$DISK` =
  `BASE:`), follows a plain name to it (`A1:` gives `BASE:[SUB]`), and
  **refuses a root that isn't concealed**: `SET DEFAULT NOCON:[SUB]` is
  `%DCL-W-DIRECT`, and the default doesn't change. A typed root is
  `%RMS-F-DIR`, though `SYS$DISK` had by then changed to `LDEV:`.
- `SET DEFAULT BASE:[NOSUCH]` succeeds, and SHOW DEFAULT then adds
  `%DCL-I-INVDEF, BASE:[NOSUCH] does not exist`.
- **BACKUP saves paths relative to the root**: a save of `BASE:[SUB...]`
  lists `[SUB]A.DAT;1` and `[SUB.DEEP]A.DAT;1`.

**The system names on VMS.** `SYS$SYSROOT` is `[concealed,terminal]`
(`SIMVAX$DUA0:[SYS0.]`, then `SYS$COMMON:`), `SYS$COMMON`, `SYS$SPECIFIC`,
and **`SYS$SYSDEVICE` are `[concealed,terminal]`**, all `[exec]` in the
system table, and `SYS$SYSTEM` and the others are plain
`SYS$SYSROOT:[dir]`.

#### Round 2 (`vax/lnm91b.log`)

**A field on both sides of a translation is an error, for every logical
name, rooted or not.** `P` = `_DUA1:[LNM91]` and `N` = `_DUA1:[LNM91]A.DAT`:

| Case | Status |
| --- | --- |
| `P:A.DAT`, `N:`, `N:;5` (fields that don't overlap) | fine: `_DUA1:[LNM91]A.DAT;`, `...;5` |
| `P:[SUB]A.DAT`, `P:[.ROOT1]`, `P:[-]`, `P:[]`, `P:[000000]`, `P:[*]`, `P:[...]`, `N:[SUB]` | `RMS$_DIR` |
| `N:B.DAT` | `RMS$_FNM` |
| `N:.TXT` | `RMS$_TYP` |

The User's Manual's `PAY_FILE:*.DAT`, with `PAY_FILE` ending in a file
name (11.3.3), is the same shape as `N:B.DAT` but with a wildcard name. The
probe didn't try it; **unconfirmed**. govax keeps today's result for a
`*` name there.

This holds through chains (`P2` = `P:`, `P3` = `LDEV:[LNM91]`) and search
list elements. It's only the *translation* that conflicts: defaults
behave as before (`[SUB]A.DAT` with default `P:` is `_DUA1:[SUB]A.DAT;`,
`B.DAT` with default `N:` is `_DUA1:[LNM91]B.DAT;`, and `P:B.DAT` with
default `[SUB]` is `_DUA1:[LNM91]B.DAT;`).

**A concealed name must be a device, or a device and a root, or RMS$_DEV.**
`C` = `_DUA1:[LNM91]`, concealed, terminal or not, or through `LDEV:`, is
`RMS$_DEV` wherever it's used: as the primary name, the default name, the
related file, or `SYS$DISK`.

**`SYS$DISK` with a directory** (plain) gives the directory only to a name
with no directory of its own: `A.DAT` is `_DUA1:[LNM91]A.DAT;`, `[SUB]A.DAT`
is `_DUA1:[SUB]A.DAT;`. Relative ones, and `[]`, use the process default,
`[JONES]`. SHOW DEFAULT shows `_DUA1:[LNM91]` and `  [] =  [JONES]`.

**Fifteen levels are real.** A 7-level root and 8 levels below it:
CREATE/DIRECTORY, COPY, `$CREATE`, `$OPEN`, `$ERASE`, `$SEARCH`, and SET
DEFAULT all work, and `LIB$FID_TO_NAME` gives
`DISK$LNM91B:[R1.R2.R3.R4.R5.R6.R7.E1.E2.E3.E4.E5.E6.E7.E8]X.DAT;1`. But
an ellipsis walks 8 levels from its MFD or root: `LDEV:[R1...]*.DAT` and
`LDEV:[R1.R2.R3.R4.R5.R6.R7...]*.DAT` find nothing, `D7:[E1...]*.DAT`
finds the files, and `LDEV:[R1.R2.R3.R4.R5.R6.R7.E1.E2]` is `RMS$_DIR`.

**Angle brackets are kept** in `$PARSE`'s and `$SEARCH`'s strings:
`_DUA1:<SUB>A.DAT;`, `_DUA1:<JONES.K>A.DAT;`, `_DUA1:<000000>A.DAT;`,
from the primary name, default name, or related file. A bracketed default
wins: `<SUB>A.DAT` with default `[OTHER]` is `DUA1:[SUB]A.DAT;`.

**SET DEFAULT:**

- A plain name with a directory (`P:`) becomes `SYS$DISK` `DUA1:` and
  default `[LNM91]`. `P:[SUB]` is `%DCL-W-DIRECT`, nothing changed; `N:`
  (with a file name) is `%RMS-F-DIR`, nothing changed.
- A concealed name with a plain directory is taken without complaint
  (`SET DEFAULT C:`, `SYS$DISK` = `C:`; SHOW DEFAULT adds INVDEF). `C:[SUB]`
  is `%RMS-F-DEV`, with `SYS$DISK` already changed to `C:`.

## Part B — decisions

**Decision 1 — the system root is `[SYSTEM.]`** (the author, 2026-11-09).
`SYS$SYSROOT` is `_DUA0:[SYSTEM.]`, concealed and terminal, in the system
table at executive mode. It is one name, not VMS's search list of
`[SYS0.]` and `SYS$COMMON`, since a search list of two identical places
would list every file twice. `SYS$COMMON` and `SYS$SPECIFIC` (the author,
2026-11-09) are defined at the same time, as the same root, so
`SYS$COMMON:[SYSMGR]X.COM` and `SYS$SPECIFIC:[SYSEXE]` work. Each is
shown by its own name.

**Decision 2 — one model: device, root, directory.** A name is a device,
a root (a list of directory names, usually empty), and a directory
*relative to the root*. The processor parses the logical directory as it
does now, against a logical default directory, and only prepends the root
when it goes to the volume. That makes "you can't get above the root"
automatic, since `applyDir` never sees it, and `[-]` at the top fails with
the error it already gives. `fileName` gains `Root`, `scanName` recognizes
`[ROOT.]` and `[ROOT.][DIR]`, `parsedName.DirSpec` stays logical, and
`parsedName.spec()` returns root + directory, so everything below it
(ods2, the volume walks) works unchanged. The root keeps its text as
written, brackets and all, for `.][` strings. A.4's errors are this
decision's rules.

The existing "fields the program wrote win over the translation's" goes
(A.4, Round 2). In `translateName`, a directory, name, or type on both
sides is `RMS$_DIR`, `RMS$_FNM`, or `RMS$_TYP`, for every logical name,
and a concealed name that gives more than a device and root is
`RMS$_DEV`. A `*` name over a translation's name keeps today's result,
for the User's Manual's example, unconfirmed. `mergeName` still merges
defaults, related files, and the process default as it does now.

Directory text keeps its brackets, `<` or `[`, as written, a bracketed
default winning. `dirSpec.String()` always writes `[` today.

**Decision 3 — no change to `internal/lnm`** beyond whatever the probe
finds wrong. Roots are a matter of reading the translated text.

**Decision 4 — one name processor** (the author, 2026-11-09, on the
condition that later changes benefit; they do). `expandName` becomes the
only one, and `expandSpec` a thin adapter that returns its `parsedName`s
as `resolvedSpec`s. The reasons:

- Roots would otherwise be written twice, and the survey's `SYS$DISK` bug
  is what comes of two copies: `expandName` has it right, `expandSpec`
  doesn't.
- F$PARSE carries a third partial copy (`lexical.go`'s `Parse` and
  `mfdRelative`), and SET DEFAULT, `LocateRelated`, and RENAME call
  `filespec.Parse` for themselves.

Measured 2026-11-09 by replacing `expandSpec` with a call of `expandName`
in a throwaway worktree, with `internal/rms`, `dclcmd`, `dclshell`, and
`console` tested: 7 tests failed. 5 were how the stand-in reported an
error (a logical name loop's `SS$_TOOMANYLNAM`, CREATE/DIRECTORY's
second message line), and 2 needed `.out` files that are gitignored and
so weren't in the worktree. None showed a name resolving differently.
The merge is its own subtask, with no change in behavior but the
`SYS$DISK` fix, before roots go in.

**Decision 5 — one helper for showing a directory.** A `resolvedSpec`
method gives the shown directory from a physical path, by removing the
root and showing the root itself as `[000000]`. All the sites in A.3 use
it, so a new command can't get it wrong by copying the old expression.

**Decision 6 — the system names, as VMS's.** `defineSystemNames` defines
the rooted `SYS$SYSROOT`, `SYS$COMMON`, and `SYS$SPECIFIC`. STARTUP.COM's
names become `define/system/executive_mode sys$system
sys$sysroot:[sysexe]` and so on, `SYS$SOURCES` as
`SYS$SYSROOT:[SOURCES]`. Consequences:

- Listings of system files are headed `SYS$SYSROOT:[SYSEXE]`, not
  `DUA0:[SYSTEM.SYSEXE]`.
- `[SYSTEM]`'s own files are `SYS$SYSROOT:[000000]GOVAX$FILES.DAT`. The
  help's `SYS$SYSROOT:GOVAX$UPDATE.LOG` would now mean the *default*
  directory under the root, so the text changes.
- STARTUP.COM is a supplied file, so existing volumes get the new copy
  at their next start, by the refresh. A STARTUP.COM the user changed is
  kept, with `SYSKEPT`, and its `sys$sysdevice:[system.sysexe]` names go
  on working.
- `SYS$SYSDEVICE` becomes `[concealed,terminal]`, as VMS's is (A.4), so
  `DIRECTORY SYS$SYSDEVICE:[X]` is headed `SYS$SYSDEVICE:[X]`. The four
  names' equivalences keep govax's `_DUA0:` form.
- Messages that give the physical name read
  `_DUA0:[SYSTEM.][SYSLIB]X.EXE;`, as VMS's do.

**Decision 7 — depth.** RMS's check is on the root (at most 7 levels,
`RMS$_DEV` past it) and the directory (at most 8, `RMS$_DIR`) separately.
Round 2 shows VMS makes them, so `ods2`'s `MaxDirectoryDepth` is raised to
15 in a tagged `ods2` release, as Phase 22's note on `go.work` describes.
RMS's own limits stay in `scanDir`. An ellipsis walks 8 levels below its
MFD or root.

**Decision 8 — what names show.** Everything a user reads follows A.4.
`DVI$_CONCEALED` is added and answers FALSE, as VMS's did. SET DEFAULT
takes A.4's cases as VMS does, including a concealed root's and refusing a
plain root. BACKUP records paths relative to the root.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey** (done 2026-11-09): Part A.
2. **The probe**: `testdata/lnm91`, as in Part D; run on the simh VAX by
   the author (done 2026-11-09, A.4).
3. **One name processor** (Decision 4): `expandSpec` over `expandName`,
   errors reported as before, F$PARSE, SET DEFAULT, `LocateRelated`, and
   RENAME's own parsing moved onto it, `mfdRelative` gone. No change in
   behavior but the `SYS$DISK` fix; the whole test suite is the check.
4. **Syntax and `$PARSE`**: `.][` and `[ROOT.]` in `scanName`, the root
   through `expandName`, the guide's table, errors, the field conflicts
   of Round 2, angle brackets kept, `NAM$V_ROOT_DIR`, `DIR_LVLS`,
   `NOCONCEAL`. Held to the probe's `$PARSE` cases, in an
   oracle test.
5. **Files through the services**: `$OPEN`, `$CREATE`, `$SEARCH` with
   wildcards (`[*]`, `[*...]`, `[...]`, 8 levels from the root),
   `$ERASE`, `$RENAME`, resultant
   strings, related names, a search list of roots.
6. **The DCL utilities**: the display helper, every command in A.3, SET
   and SHOW DEFAULT with a rooted device, F$PARSE, and F$SEARCH. Held to
   the probe's DCL log.
7. **Devices and the rest**: `PhysicalDevice`, `$ASSIGN`, `$GETDVI` and
   `DVI$_CONCEALED`, the image activator's message and
   `JPI$_IMAGNAME`, `LIB$FID_TO_NAME`, BACKUP, and the depth (with an
   `ods2` release if Decision 7 needs one).
8. **The system names**: `defineSystemNames` with `SYS$COMMON` and
   `SYS$SPECIFIC`, STARTUP.COM, the refresh on an existing volume,
   removing `TestLink68Oracle`'s mask and the tests' plain `SYS$SYSROOT`,
   and running the system volume's tests.
9. **The close-out**: MANUAL.md (the rooted-names paragraph, "The system
   volume", the "no rooted names" line in the list of gaps), HELP DEFINE,
   ASSIGN, SET DEFAULT, F$PARSE, and SYSTEM_VOLUME, `sysnames.go`'s and
   Phase 87's notes, PLAN.md, and CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/lnm91/`: `lnm91.mar` is one small program that takes its cases
from a command procedure's data lines, and `gen.go` writes `lnm91.com`
(the table of cases), `exchange.cmd`, and `copyout.cmd`, with the README
giving the steps. It runs on an exchange volume on DUA5, makes its own
tree on that volume, and defines only process logical names, which it
deassigns at the end, so the VAX's system tables aren't changed. Results
go in `vax/`.

**`lnm91.mar`, the services.** Each case reports status, STV, expanded
and resultant strings, FNB in hex with `ROOT_DIR`, `CNCL_DEV`, and
`DIR_LVLS` picked out, each component's length, and DVI, DID, and FID.
The `$PARSE` and `$SEARCH` cases run with and without `NAM$V_NOCONCEAL`,
and with `NAM$V_SYNCHK`.

| Group | Cases |
| --- | --- |
| Guide's table | each row of A.1, against `$PARSE` and `$SEARCH` |
| Forms of the name | concealed and terminal; concealed only; **not concealed**; terminal only; `_DEV:` and `DEV:`; logical over physical |
| Chains | `A = ROOT:[SUB]`; `A = ROOT:[SUB.]` (a root on a root); concealed over rooted; a list of roots |
| Syntax | typed `DEV:[ROOT.][DIR]F`, `DEV:[ROOT.]F`, `[ROOT.][000000]F`, `[ROOT.][.DIR]`, `[ROOT.][-]`, `<ROOT.>[DIR]`, `[A.][B.]`, `[A...][B]`, `[R*.]`, `[A..B.]` |
| Defaults | rooted as default name, as related file, and as `SYS$DISK`; a physical default directory with a rooted primary |
| Depth | root and directory sums of 7, 8, 9, 15, 16, 17 levels; CREATE/DIRECTORY at each |
| Rendering | `[000000]`, `[000000.SUB]`, `[.SUB]` from the root, `[-]` from a child; `DIR_LVLS` |
| Files | `$CREATE`, `$OPEN`, `$ERASE`, `$RENAME` across roots, wildcard `$SEARCH` over `[*]`, `[*...]`, `[...]`, including where each walk ends (govax has no `$ENTER` or `$REMOVE`, so they aren't probed) |
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

## Round 2

Accepted by the author 2026-11-09, and run the same day (A.4):
`lnm91b.com` on its own exchange volume (README.md), the same program. It
covers:

| Case | Why |
| --- | --- |
| `P = _DUA1:[LNM91]` plain; `P:[SUB]A.DAT`, `P:[.ROOT1]A.DAT`, `P:A.DAT` | Is `mergeName`'s directory rule wrong for plain names too? govax lets these through. |
| `C = _DUA1:[LNM91]` concealed; `C:A.DAT`, `C:[SUB]A.DAT` | The Dictionary's "must be a physical device name". |
| A 7-level root and an 8-level directory: CREATE/DIRECTORY, `$CREATE`, `$OPEN`, `$SEARCH` | Whether 15 physical levels really exist on disk (Decision 7). |
| `<A>` plain directories: `$PARSE` and `$SEARCH` | Whether brackets are kept without a root as well. |
| `SYS$DISK` defined with a directory, plain and concealed | How a directory there meets the default directory and one in the name. |

## The review (2026-11-09)

The author accepted the plan and answered its questions:

1. **`SYS$COMMON` and `SYS$SPECIFIC`:** define them, because a user's
   procedure or program might reasonably assume they are there
   (Decision 1).
2. **`SYS$SOURCES`** becomes `SYS$SYSROOT:[SOURCES]` like the others.
3. **The two name processors:** merge them if later changes to file names
   would benefit from a single code path. They would, and it measured
   cheap (Decision 4).
4. **`CONCEAL_DEVICES`:** not supported.

## Progress log

- **2026-11-09** Subtask 1: read the manuals and the VMS logs in the
  repository, ran today's build against a concealed root on a scratch
  volume, and wrote this plan. The run found that a rooted name is
  silently taken as its directory part alone
  (`CREATE/DIRECTORY BASE:[NEWD]` makes a top-level directory), and that
  a concealed `SYS$DISK` is shown by its physical device in DIRECTORY's
  heading. Stopped for the author's review. The author set the root to
  `[SYSTEM.]` during the survey.
- **2026-11-09** The review: see "The review". Measured the merge of the
  name processors (Decision 4) and reordered Part C around it.
- **2026-11-09** Subtask 2, written: `testdata/lnm91` (`lnm91.mar`, and
  `gen.go`'s `lnm91.com`, 453 program cases and the DCL commands). Part
  D's `gen.go` writes the procedure only; the program is by hand, taking
  its cases from data lines. Rehearsed on govax from start to finish, so
  the program assembles, links, and runs. Waiting for the VAX run.
  The rehearsal also shows what govax doesn't accept yet, to be added or
  left out of the oracle once VMS's log is in: `COPY/LOG`, `PURGE/LOG`,
  `DIRECTORY/TOTAL`, `SET PROTECTION`, and F$GETDVI's `CONCEALED`.
- **2026-11-09** Subtask 2's first round ran on VMS V7.1 (`vax/lnm91.log`,
  `vax/lnm91.exe`). A.4 is its answers. Decisions 2, 6, and 7 changed by
  them, and Decision 8 is new.
- **2026-11-09** Round 2 written (`lnm91b.com`, 275 program cases),
  rehearsed on govax, and run on VMS V7.1 (`vax/lnm91b.log`; A.4, "Round
  2"). A field on both sides of a translation is an error for every
  logical name, which changes behavior Phase 33 assumed; a concealed name
  must be a device or a device and root; 15 physical levels exist on disk.
  Decisions 2, 7, and 8 revised. Subtask 2 done.
