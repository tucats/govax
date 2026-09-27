# Phase 25: VMS-faithful logical names

## Goal

Replace govax's fledgling logical-name support with a real VAX/VMS logical-name
facility that the console, RMS, and the RTL/system-service layer all share:

- **Multiple named logical-name tables**, organized the VMS way: two
  *directory* tables (`LNM$PROCESS_DIRECTORY`, `LNM$SYSTEM_DIRECTORY`) holding
  table names, and the three default tables the user asked for, reachable by
  their standard logical names:
  - `LNM$PROCESS` → `LNM$PROCESS_TABLE`
  - `LNM$GROUP` → `LNM$GROUP_gggggg` (the process's UIC group, in octal)
  - `LNM$SYSTEM` → `LNM$SYSTEM_TABLE`

  Further tables can be created (`CREATE/NAME_TABLE`, `SYS$CRELNT`).
- **VMS translation semantics**: the default search order is process, then group,
  then system, driven by the `LNM$FILE_DEV` search list (the standard VMS
  mechanism, not a hard-coded order). Translation is **iterative**: each further
  level starts again at the top of the search order, as the VMS manual
  describes. It stops after 10 levels or at a name marked `TERMINAL`, and
  circular definitions are detected. Also supported: multi-valued names
  (**search lists**), per-name **access modes**, and the `CONCEALED`/
  `TERMINAL` translation attributes.
- **File-specification translation** that RMS and the console file commands
  (DIRECTORY, TYPE, COPY, DELETE, PURGE, SET DEFAULT, MOUNT) all use. Only the
  leftmost component is translated, and only when it ends with a colon or is
  the whole spec. A leading underscore turns translation off. RMS tries each
  element of a search list in turn.
- **DCL commands** built on the existing DCL grammar mechanism, with the
  table-selecting forms sharing one handler:
  - `DEFINE[/PROCESS|/GROUP|/SYSTEM|/TABLE=table] logical-name equivalence-name[,...]`
  - `ASSIGN`, `DEASSIGN`, `SHOW LOGICAL`, `SHOW TRANSLATION`, and
    `CREATE/NAME_TABLE`
- **System services**:
  - `SYS$CRELNM`, `SYS$DELLNM`, `SYS$TRNLNM` (rewritten), and `SYS$CRELNT`
  - optionally the older `SYS$CRELOG`/`SYS$DELLOG`/`SYS$TRNLOG`; see the open
    questions

  All of them work on the same shared data the console uses.

**Status: in progress — subtasks 1-3 of 9 done.**

## Why this phase looks different

The existing code is a port of eVAX's `logical_names.c`. **Per the user's
explicit direction while this phase was being planned, the existing govax (and
eVAX) behavior is treated as suspect and the VAX/VMS standard is the goal.** So,
unlike the porting phases (01-12), this phase does **not** replicate C behavior
under CLAUDE.md's "document and defer" policy. As in Phases 22-24, the
correctness references are real VMS documentation:

- *VSI OpenVMS User's Manual*, Chapter 11, "Defining Logical Names for Devices
  and Files" (`~/Documents/Technical Doc/VMS/VSI_USERS_MANUAL.pdf`, pp. 225-254).
  It is the primary behavioral source for tables, directories, search order,
  iterative translation, search lists, access modes, and translation attributes.
- *VAX/VMS Command Language Reference Manual*, §2.2, "Logical Names" (the user
  pasted this excerpt into the planning request). It is the source for
  file-spec translation of the leftmost component, the 10-level limit,
  applying defaults after translation, and temporary defaults in input lists.
- The *VMS System Services Reference* descriptions of `$CRELNM`/`$DELLNM`/
  `$TRNLNM`/`$CRELNT`, for argument lists, item codes, and status codes.
  The numeric values come from two real VMS 7.3 source files copied into
  `reference/vms/` from the user's VMS 7.3 source archive
  (`~/Documents/Technical Doc/VMS/vmssrc_archive/v73`), not typed in by hand:
  - `lnmdef.sdl` (from `starlet_b64/lis/`) is the SDL source of `$LNMDEF`.
    No SDL-generated `lnmdef.h` exists anywhere on disk.
  - `ssdef.txt` (from `vest_dblrtl/lis/`) is a BLISS `LITERAL` listing of all
    406 VAX `$SSDEF` codes.

  `internal/vmsdef/gen` parses both (see subtask 1).

Where the two manuals differ (the V4-era CLRM says names are at most 63
characters and translation is 10 levels deep; the modern VSI manual says 255
characters and "at least nine" levels), the target is **VAX/VMS 7.3**, which
is the version `reference/vms/`'s headers already come from. Details are under
"Open questions".

Each departure from eVAX behavior is still recorded in `docs/DEVIATIONS.md` as a
"fixed in Phase 25" entry, so the reason govax no longer matches `reference/eVAX`
is traceable.

## What exists today (and why it's replaced rather than extended)

| Where | What it does | Problem vs. VMS |
| --- | --- | --- |
| [internal/io/logical.go](../internal/io/logical.go) | `LogicalNameTable`: a map of tables, where each table maps a name to **one** `Value`. `InitLogicals` seeds `LNM$TABLE` and a table called `LNM$FILE_DEV` holding `SYS$INPUT`/`OUTPUT`/`ERROR`/`COMMAND` = `TTA0:`. | `LNM$FILE_DEV` is a **table** here. In VMS it is a *logical name* (a search list of tables). There are no directories, no search order, no iterative translation, no search lists, and no access modes. Attributes are matched by equality, not as a bitmask. `Get` falls back to the table `LNM$ROOT`, which doesn't exist in VMS. |
| [internal/console/device.go:141-175](../internal/console/device.go#L141-L175) + `DEFINE_LOGICAL` binding in [dispatch.go:375](../internal/console/dispatch.go#L375) | `DEFINE/LOGICAL name value [/TABLE=]`, which defaults to the table `LNM_PROCESS` (an underscore, not `$`). `SHOW LOGICAL_NAMES [name] [/TABLE=]`. | The default table name is wrong: `LNM_PROCESS` is never searched, including by RMS, which only looks in `LNM$FILE_DEV`. So **a name defined with the console's own DEFINE command is invisible to RMS today.** `DEFINE/LOGICAL` isn't VMS syntax. The SHOW output doesn't follow the VMS format. |
| [internal/rms/create.go:59](../internal/rms/create.go#L59), [open.go:61](../internal/rms/open.go#L61) | Look up the **entire** file-spec string as a name in the table `LNM$FILE_DEV`, then replace the spec with the value. | This isn't how VMS translates: it doesn't split off the leftmost `name:` component, doesn't iterate, doesn't handle search lists, and doesn't handle `_`. `SCRATCH:PAYROLL.DAT` never translates. |
| [internal/rms/session.go](../internal/rms/session.go) (DIRECTORY/TYPE/COPY/DELETE/PURGE/SET DEFAULT) | Parse the operator's spec directly with `filespec.Parse`. | No logical-name translation at all. |
| [internal/rtl/logicals.go](../internal/rtl/logicals.go) | `SYS$TRNLNM` looks up one table name exactly. `LNM$_MAX_INDEX` is always 1. `LNM$_INDEX` is ignored. | A call like `$TRNLNM(tabnam="LNM$FILE_DEV", ...)`, the most common form in real code, only works by accident of the seeding described above. There's no `$CRELNM`/`$DELLNM`/`$CRELNT`, even though all three are already in the P1 vector ([internal/vmsdef/p1vector.go](../internal/vmsdef/p1vector.go)). |
| `internal/bootdata/files/console.dcl` | `verb define` has only the `/LOGICAL` and `/DEVICE` syntax-redirect qualifiers. | A bare `DEFINE name value` doesn't parse. |

The data model is the core problem (single-valued names and no directories), so
this phase **replaces** `internal/io/logical.go` instead of patching it.

## Design decisions

### New leaf package `internal/lnm`

The logical-name database moves out of `internal/io` into a new package,
**`internal/lnm`**, with no dependencies on other govax packages:

- `internal/io` is the device abstraction. Logical names are a separate VMS
  subsystem that the console, RMS, and RTL all use, so none of those packages
  is the natural owner.
- A leaf package can be imported by `internal/console`, `internal/rms`, and
  `internal/rtl`, and by any later RTL/system-service package, without cycles.
  This matches the user's requirement that the database be available to "the RMS
  package and any peer package to that".
- Ownership stays the same as today. The `Console` constructs one
  `*lnm.Database` in `New`, alongside `Devices`/`Mounts`, and passes the same
  pointer into `rtl.NewEnvironment` and `rms.Context`/`rms.Session`. There's no
  package-level singleton (as required by the PLAN.md architecture rules).
  Logical names survive `INIT`/`ZERO` exactly as the current `Logicals` field
  does.

`internal/io/logical.go` and its tests are deleted once every caller has moved
over. The `vax.DebugLogicals` debug flag stays. `internal/lnm` reports through
an injected `Database.Trace` function (nil means off) instead of importing
`vax`.

### Data model

As built in subtask 2 ([internal/lnm](../internal/lnm)):

```text
Database
  ProcessDirectory, SystemDirectory *Table
  GroupTableName string              // LNM$GROUP_gggggg, from the UIC's group
  Trace func(format, args...)        // nil = off
  tables []*Table                    // every live table, in creation order

Table
  Name       string                  // e.g. "LNM$PROCESS_TABLE"
  Mode       Mode
  Parent     *Table                  // nil only for a directory
  Directory  bool                    // LNM$PROCESS_DIRECTORY / LNM$SYSTEM_DIRECTORY
  Shareable  bool                    // cataloged in the system directory
  Attrs      uint32                  // LNM$M_CONFINE
  names      map[string][]*Entry     // case-sensitive name -> one entry per access mode

Entry
  Name         string
  Mode         Mode                  // Kernel / Executive / Supervisor / User
  Attrs        uint32                // NO_ALIAS, CONFINE, CRELOG; TABLE for a table name
  Equivalences []Equivalence         // index 0..127; none for a table-name entry
  Table        *Table                // the table containing the entry
  Target       *Table                // for a table-name entry, the table it names

Equivalence
  Value  string
  Attrs  uint32                      // CONCEALED, TERMINAL (per equivalence, as in VMS)
```

A table's name is an `Entry` with `AttrTable` in a directory, as in VMS.
Removing that entry deletes the table and all its descendant tables.

Attribute bits and limits come from the generated `vmsdef.LNMConstants`, so what
`$TRNLNM`'s `LNM$_ATTRIBUTES` item returns is exactly what the database stores,
with no translation layer. `Mode` uses the PSL numbering (kernel 0 through
user 3). Status codes come back as `vmserrors.VMSError` values carrying the real
`$SSDEF` numbers.

When one table holds the same name in several access modes, a lookup takes the
outermost (least privileged) one of those at or inside the requested mode. This
matches `$TRNLNM` and the manual's `SYS$OUTPUT [super]` / `SYS$OUTPUT [exec]`
example.

### Table names are themselves logical names (VMS directories)

As VMS does, tables are found by *translating a table name through the
directories*. The code never looks up the Go map directly. At startup,
`Database` creates:

| Directory | Name | Translates to |
| --- | --- | --- |
| `LNM$PROCESS_DIRECTORY` | `LNM$PROCESS_DIRECTORY` | (table name: the directory itself) |
| | `LNM$PROCESS_TABLE` | (table name) |
| | `LNM$PROCESS` | `LNM$PROCESS_TABLE` |
| | `LNM$GROUP` | `LNM$GROUP_gggggg` |
| `LNM$SYSTEM_DIRECTORY` | `LNM$SYSTEM_DIRECTORY` | (table name: the directory itself) |
| | `LNM$SYSTEM_TABLE` | (table name) |
| | `LNM$GROUP_gggggg` | (table name) |
| | `LNM$SYSTEM` | `LNM$SYSTEM_TABLE` |
| | `LNM$FILE_DEV` | `LNM$PROCESS`, `LNM$GROUP`, `LNM$SYSTEM` (search list) |
| | `LNM$DCL_LOGICAL` | `LNM$FILE_DEV` |
| | `LNM$DIRECTORIES` | `LNM$PROCESS_DIRECTORY`, `LNM$SYSTEM_DIRECTORY` |

`LNM$GROUP` is in the process directory, as in the User's Manual's Table 11.2,
because it depends on the process's UIC. The group table itself is shareable and
is cataloged in the system directory. (An earlier draft of this table put
`LNM$GROUP` under the system directory; that was corrected in subtask 2.) The
tables are kernel mode. The directory logical names are executive mode, so every
access mode can see them.

A table-name argument (`/TABLE=`, `$TRNLNM`'s `tabnam`, `$CRELNM`'s `tabnam`) is
resolved like this:

1. Look the name up in the process directory, then the system directory. That
   is the order `LNM$DIRECTORIES` records, but it is fixed in code, because
   resolving `LNM$DIRECTORIES` itself would need a directory to look in.
   Translate the name iteratively, keeping only names that resolve to real
   tables. An element of a search list that names nothing is skipped.
   Iteration is capped at `LNM$C_MAXDEPTH`.
2. The result is an **ordered list of tables**. `$TRNLNM` and `SHOW LOGICAL`
   search the whole list. `$CRELNM`/`DEFINE` use the first table in the list,
   as VMS does.

This means `DEFINE/TABLE=LNM$PROCESS`, `/TABLE=LNM$PROCESS_TABLE`, and `/PROCESS`
all name the same table, which is what the user described ("LNM$PROCESS is the
PROCESS table name"). It also means a user can redefine `LNM$FILE_DEV` or
`LNM$PROCESS` to add their own table to the search order, as §11.9.2 and §11.11
of the manual describe, with no extra code.

The **job table** (`LNM$JOB`) and the cluster tables are left out for now. The
user specified three default tables, and govax has one process and no cluster.
See open question 1.

### Translation engine: the two kinds of translation are kept apart

VMS has two related but different operations, and the package keeps them
separate:

1. **`Translate(tables, name, opts) → Entry`**. This is a single-level lookup,
   what `$TRNLNM` does. Search the resolved table list in order and return the
   first match that the caller's access mode allows. It does **not** iterate:
   `$TRNLNM` and `SHOW TRANSLATION` are one level deep. Options:
   - case-blind matching (`LNM$M_CASE_BLIND`)
   - the caller's access mode
2. **`TranslateFileSpec(spec) → []string`**. This is the RMS/DCL file-spec
   operation from CLRM §2.2.3 and User's Manual §11.5:
   - If the spec starts with `_`, do no translation (the VMS rule that marks a
     physical device name). The `_` is kept in the result, as in a VMS
     resultant string; consumers strip it when they look up the device.
   - Otherwise, pick out the leftmost component. That is either the text before
     the first `:` (and not `::`, since govax has no DECnet), or, when there's
     no `:` at all, the whole spec as long as it's a valid logical-name token.
     Anything ending in `]`, `>`, `.`, or `;` isn't a candidate.
   - Translate that component through `LNM$FILE_DEV`, **starting again at the
     top of the search order at every level**, as the user asked. Substitute the
     equivalence string for `component:` (or for the whole spec) and repeat
     until:
     - no leftmost component translates, or
     - the matched equivalence has the `TERMINAL` attribute, or
     - the depth limit is reached, which fails with `SS$_TOOMANYLNAM`
       (RMS reports it as `RMS$_LNE`).

     A name that comes up a second time in one translation chain is reported
     with the same error, so a circular definition fails deterministically
     instead of just running into the limit.
   - A **search list** fans out: each equivalence is translated recursively, in
     order, so nested lists expand depth-first (§11.7.4's FRED/ETHEL/LUCY/RICKY
     example is a test case). The result is the ordered list of fully translated
     specs.
   - A `CONCEALED` equivalence is translated for access, but the caller also
     gets the concealed logical name so it can be *displayed* instead of the
     physical device (for example SHOW DEFAULT, DIRECTORY headers). This is why
     the return value is a small struct, not a bare string: `lnm.FileSpec`
     has `Spec` (fully translated), `Concealed` (the outermost concealed name
     in the chain), and `Display` (the text at the point that name was about
     to be translated).

   The package only rewrites text. Parsing and applying defaults
   (`filespec.Parse` with the session default) stays in `internal/rms`, which
   is still the only package allowed to import `ods2` (see CLAUDE.md). So
   `internal/lnm` doesn't depend on `ods2`.

### Console commands (DCL grammar)

All of these use the existing `internal/console/dcl` grammar in
`internal/bootdata/files/console.dcl`:

- **`DEFINE logical-name equivalence-name[,...]`**, which by default goes into
  the process table. Qualifiers:
  - choose the table: `/PROCESS`, `/GROUP`, `/SYSTEM`, `/TABLE=name`
  - choose the access mode: `/USER_MODE`, `/SUPERVISOR_MODE` (the default),
    `/EXECUTIVE_MODE`
  - `/TRANSLATION_ATTRIBUTES=(CONCEALED,TERMINAL)`
  - `/LOG`

  The table qualifiers all go to **one handler**, which takes the chosen table
  name as a string (the user's explicit request). `/PROCESS`, `/GROUP`, and
  `/SYSTEM` just mean `LNM$PROCESS`/`LNM$GROUP`/`LNM$SYSTEM`. Following the
  VMS DEFINE rule, a trailing colon on the logical name is **kept**.
- **`ASSIGN equivalence-name[,...] logical-name`**. It takes its parameters in
  the opposite order and **removes** one trailing colon from the logical name,
  per the CLRM excerpt. It uses the same handler.
- **`DEASSIGN logical-name`**, with the same table and mode qualifiers plus
  `/ALL`. It removes one trailing colon, as ASSIGN does.
- **`SHOW LOGICAL [name[,...]]`**. The name may use `*`/`%` wildcards.
  Qualifiers: `/PROCESS`, `/GROUP`, `/SYSTEM`, `/TABLE=`, `/FULL`, `/STRUCTURE`.
  By default it searches `LNM$DCL_LOGICAL` and translates iteratively, printing
  the level number. The output follows the VMS format:
  - one name: `"NAME" = "VALUE" (LNM$PROCESS_TABLE)`
  - further search-list elements: an indented `= "VALUE2"` on each continuation line
  - an iterated level: `1 "WORK4" = "$255$DUA17:" (LNM$SYSTEM_TABLE)`
  - a table listing starts with a `(LNM$PROCESS_TABLE)` header and lists the
    names alphabetically
  - `/FULL` adds `[super]`/`[exec]` and `[concealed,terminal]`
- **`SHOW TRANSLATION name`** does one level only (`$TRNLNM` semantics).
- **`CREATE/NAME_TABLE table`** creates a table, covering "additional tables can
  be added". Qualifiers: `/PARENT_TABLE=` (default `LNM$PROCESS_TABLE`, so the
  new table is process-private. An earlier draft said `LNM$PROCESS_DIRECTORY`;
  subtask 6 will confirm the default against the DCL documentation) and `/USER_MODE`/`/SUPERVISOR_MODE`/
  `/EXECUTIVE_MODE`.
- **`DEASSIGN/TABLE=`** (with no logical name) deletes a table, per §11.12.
- **`DEFINE/DEVICE`** stays exactly as it is.
- **`DEFINE/LOGICAL`** and **`SHOW LOGICAL_NAMES`** are eVAX-isms. Open question
  4 asks whether to keep them as hidden aliases or drop them.

Grammar work these commands need. **Per the user's explicit direction during
planning, extending `internal/console/dcl` is in scope for this phase.** Where
the grammar can't express what real DCL does, it gets fixed properly and in a
general way, not worked around in a handler, because a more complete DCL
package will help every later command-line emulation too.

- `verb define` currently works *only* through syntax-redirect qualifiers
  (`/LOGICAL`, `/DEVICE`). It needs its own parameters (`name`, `value`) plus the
  `/DEVICE` redirect. Subtask 5 first checks that `dcl.Parse` supports a verb
  having both. If it doesn't, subtask 5 adds that support to the grammar.
- Mutually exclusive qualifiers (`/PROCESS` vs `/GROUP` vs `/SYSTEM` vs
  `/TABLE`, and the three `/..._MODE` qualifiers) should be declared in the
  grammar (the CDU-style `DISALLOW` clause, or a simpler equivalent), not
  checked by hand in each handler.
- Keyword-list qualifier values such as `/TRANSLATION_ATTRIBUTES=(CONCEALED,
  TERMINAL)`, which the grammar can already accept as keyword types, need
  parenthesised multi-value support if it isn't there yet.
- **Comma-separated lists.** `readValueToken` in
  [internal/console/dcl/parse.go](../internal/console/dcl/parse.go) stops at
  whitespace, so `DEFINE X A,B` already comes through as the single token
  `"A,B"`. But `DEFINE X A, B` (which real DCL accepts) splits into two
  tokens, and a quoted element containing a comma can't be told apart from
  a separator. The fix is a real list-value type (`$list` or `/list` on a
  parameter) that returns `[]string`, respecting quotes and allowing spaces
  after commas. This also builds the base for later multi-file input lists
  (`TYPE A,B,C`).
- DCL upper-cases names and equivalences unless they're quoted, which the
  tokenizer already does (`upcaseOutsideQuotes`). That matches VMS.
- The legacy `DEFINE_LOGICAL` binding in `dispatch.go` and the
  `ShowLogicals`/`DefineLogical` methods in `device.go` move into a new
  `internal/console/logical.go`.

### SET DEFAULT, SYS$DISK, and process-permanent names

On VMS, `SET DEFAULT dev:[dir]` writes the device part to `SYS$DISK` in the
process table (supervisor mode, **not translated**). The directory is kept as
the process default directory. RMS then gets its default device by translating
`SYS$DISK`. For govax:

- `rms.Session.SetDefault` translates only enough to check that the spec is
  valid. It stores the directory in `Session.Default` and **defines `SYS$DISK`**
  with the untranslated device text (so `SET DEFAULT FIFI` with a search list
  behaves as §11.7.2 describes).
- Default-device application in RMS and the console file commands reads
  `SYS$DISK` through `TranslateFileSpec`, instead of holding a device in
  `Session.Default`.
- The initial process table has these names:
  - `SYS$INPUT`, `SYS$OUTPUT`, `SYS$ERROR`, `SYS$COMMAND` = `_TTA0:`, in
    executive mode with `TERMINAL` set
  - `TT` = `_TTA0:`
  - `SYS$DISK`, once a default is set

  VMS actually stores a process-permanent file's value with a hidden 4-byte
  `ESC`/IFI prefix. That detail is out of scope and noted as a deferred
  deviation. The existing RMS special case for the console device (see
  `create.go`'s `consoleDeviceName`) keeps working, because `SYS$OUTPUT`
  translates to `_TTA0:`.
- The system table starts empty apart from anything a later phase needs, such
  as `SYS$SYSDEVICE` once a system disk exists. Open question 3 covers this.

### RMS and RTL consumers

- **`internal/rms`**:
  - `create.go`/`open.go`: replace the whole-string lookup with
    `TranslateFileSpec`. `$OPEN` tries each search-list result in order and
    uses the first file it finds. If none is found, it returns the *last* error,
    per §11.7. `$CREATE` uses the first result.
  - `Session.resolveVolume`/`Directory`/`Type`/`Delete`/`Purge`/`Copy` all
    translate first. For wildcard commands (DIRECTORY, DELETE, PURGE), each
    search-list element is used in turn, per §11.7.1.
  - `MOUNT`/`DISMOUNT` translate their device argument, with `_` suppressing
    translation. On VMS, `MOUNT` also defines the volume label as a logical
    name. Open question 3 asks whether to do that too.
- **`internal/rtl`**:
  - Rewrite `SYS$TRNLNM` against `internal/lnm`: table-name resolution through
    the directories, `LNM$_INDEX` for search-list elements, a real
    `LNM$_MAX_INDEX`, `LNM$_ATTRIBUTES` with the real `LNM$M_EXISTS`/
    `TERMINAL`/`CONCEALED` bits, and the access-mode filter.
  - Add `SYS$CRELNM`, `SYS$DELLNM`, and `SYS$CRELNT`, registered through
    `ServiceTable.Register` (the table-driven dispatch rule from memory).
  - `SYS$ASSIGN`/`SYS$GETDVI` device names go through the device-name case of
    `TranslateFileSpec`, with `_` suppression, before `Devices.Find`.
  - Names defined in user mode are deleted when an image exits (`RUN`
    completion), per §11.3.5. There is one hook at image rundown in the
    console's run path.
- **Privileges.** Putting a name in the group or system table needs `GRPNAM`/
  `SYSNAM`, but govax has no privilege model yet. The checks are left out and
  the gap is recorded in `docs/DEVIATIONS.md`. They're easy to add later
  because every write goes through one `Database.Define` entry point.

## Subtasks

1. **Done.** **`internal/vmsdef` LNM and SS constants.**
   - Added `reference/vms/lnmdef.sdl` and `reference/vms/ssdef.txt`.
   - Extended `internal/vmsdef/gen` to generate two new maps alongside
     `Constants`:
     - `LNMConstants` (40 entries): `LNM$M_`/`LNM$V_` bits, `LNM$C_` limits,
       and `LNM$_` item codes.
     - `SSConstants` (406 entries).
   - Unit tests pin the values the design depends on.
2. **Done.** **`internal/lnm` core.** `Database`, `Table`, `Entry`, and `Equivalence`, plus
   startup of the directory and default tables (the table in "Table names are
   themselves logical names"). Also:
   - `Define`/`Delete`/`CreateTable`/`DeleteTable`
   - resolving a table name through `LNM$DIRECTORIES`
   - single-level `Translate` with access-mode and case-blind handling
   - name/length validation (`SS$_IVLOGNAM`)
   - wildcard `Match` for SHOW LOGICAL

   Table-driven tests follow the manual's examples. Nothing is wired in yet.
3. **Done.** **`internal/lnm` file-spec translation.** `TranslateFileSpec` covering:
   - the leftmost-component rule and `_` suppression
   - iterative translation that restarts at the top of the search order
   - `TERMINAL`
   - the depth limit and circular-definition detection
   - nested search-list fan-out
   - carrying `CONCEALED` through to the result

   Tests use every example from CLRM §2.2 and User's Manual §11.3-11.7
   (REPORT → `DBA1:WEATHER.SUM`, GO → TEST → `DBA1:`, MEMO, GETTYSBURG, FIFI,
   NESTED, and so on).
4. **Swap consumers over to `internal/lnm`** without changing behavior:
   - `Console` builds a `*lnm.Database`.
   - `rtl.Environment` and `rms.Context`/`Session` take it instead of
     `*iodev.LogicalNameTable`.
   - The console's device names move from the fake `LNM$FILE_DEV` table into
     `LNM$PROCESS_TABLE`.
   - Delete `internal/io/logical.go` and its tests.

   The existing test suite has to pass, with any expectation tied to the old
   table names updated.
5. **DCL grammar**, in scope as general-purpose `internal/console/dcl` work
   (see "Console commands (DCL grammar)").
   - Start with a short gap audit: compare what these commands need with what
     `dcl.Parse` supports, and record the result in this log.
   - Add list-valued parameters and qualifier values, with quoting and spaces
     after commas.
   - Support a verb that has its own parameters and also a syntax-redirect
     qualifier, so `DEFINE` works alongside `DEFINE/DEVICE`.
   - Add declarative mutually exclusive qualifiers.
   - Add parenthesised keyword lists.
   - Add grammar entries for DEFINE/ASSIGN/DEASSIGN/SHOW LOGICAL/SHOW
     TRANSLATION/CREATE/NAME_TABLE.
   - Parser tests go in `internal/console/dcl`. Every existing grammar test has
     to keep passing.
6. **Console commands.** Create `internal/console/logical.go`, with one shared
   define handler (table name as a parameter) for
   `DEFINE[/PROCESS|/GROUP|/SYSTEM|/TABLE]` and `ASSIGN`, plus `DEASSIGN`,
   `SHOW LOGICAL` (VMS output format, iterative levels, `/FULL`,
   `/STRUCTURE`), `SHOW TRANSLATION`, and `CREATE/NAME_TABLE`.
   - Remove the eVAX-only `DEFINE/LOGICAL` syntax and the `SHOW
     LOGICAL_NAMES` keyword; `SHOW LOGICAL` is the VMS form (open question 4).
   - Update `internal/bootdata/files/vax.help` to describe the VMS syntax only.
7. **RMS file-spec translation.**
   - `$OPEN`/`$CREATE`, and the `Session` commands DIRECTORY/TYPE/COPY/
     DELETE/PURGE/SET DEFAULT/MOUNT, go through `TranslateFileSpec`.
   - Search lists work for `$OPEN` (first file found) and for wildcard
     commands (every element).
   - Map the error to `RMS$_LNE`.
   - `MOUNT` defines `DISK$label` in the system table and `DISMOUNT` removes
     it (open question 3).
   - `SYS$DISK`-driven default device.
   - Add end-to-end console tests against a real container, extending
     `internal/console/rms_e2e_test.go`'s pattern.
8. **System services.**
   - Rewrite `SYS$TRNLNM`.
   - Add `SYS$CRELNM`, `SYS$DELLNM`, and `SYS$CRELNT`, with full item-list
     handling (`LNM$_STRING`/`ATTRIBUTES`/`INDEX`/`MAX_INDEX`/`TABLE`/
     `LENGTH`/`ACMODE`, `LNM$_CHAIN`).
   - Add the pre-V4 services `SYS$CRELOG`, `SYS$DELLOG`, and `SYS$TRNLOG` as
     thin wrappers over the same database (open question 6). Their table
     numbers 0/1/2 map to system/group/process, and `$TRNLOG` searches
     process → group → system one level deep.
   - Add device-name translation for `SYS$ASSIGN`/`SYS$GETDVI`.
   - Rundown of user-mode names at image exit.
   - Add a MACRO-32 fixture (`testdata/asm/lnm_roundtrip.asm`) that runs
     `$CRELNM` → `$TRNLNM` → `$DELLNM` and writes through `SYS$OUTPUT` via a
     program-defined logical name, as an acceptance test.
9. **Legacy clean-up and docs.**
   - Sweep for any remaining eVAX-only logical-name syntax in docs, help, and
     tests (open question 4).
   - Add `docs/DEVIATIONS.md` entries for each eVAX behavior this phase
     changes, and for the deliberate gaps (privileges, the job table, the
     process-permanent-file prefix).
   - Add a `docs/PLAN.md` narrative paragraph.
   - Update CLAUDE.md's package list with `internal/lnm`.
   - Close out this progress log.

Subtasks 1-3 are pure library work with no user-visible change. Subtask 4 is the
one step that changes wiring. Subtasks 5-8 can be done in any order after 4, but
7 depends on 3.

## Open questions

1. **Job table.** VMS's `LNM$FILE_DEV` is `PROCESS, JOB, GROUP, SYSTEM`. The
   user asked for three default tables. Should `LNM$JOB` (and `DEFINE/JOB`) be
   added now as a fourth, since govax has one process and it would behave
   exactly like the process table? *Recommendation: leave it out for now. The
   directory-based design makes it a few lines to add later.* **CONFIRMED: leave
   it out for now.**
2. **UIC group number.** `rtl.Environment` already has a nominal UIC
   (`nominalUIC`). The group table should be named from its group field
   (`LNM$GROUP_000001`, or whatever `nominalUIC` holds), which means the UIC has
   to move somewhere the console-owned `lnm.Database` can see it at
   construction. *Recommendation: pass the UIC into `lnm.NewDatabase`, and
   have `rtl.NewEnvironment` take it from the same place.* **CONFIRMED: passing
   the uic into new database.**
3. **Default system-table and mount-time names.** Should `MOUNT` define the
   volume label as a logical name (as VMS does, in the system table for
   `/SYSTEM` mounts and otherwise the job table), and should
   `SYS$SYSDEVICE`/`SYS$SYSTEM` be seeded? *Recommendation: MOUNT defines
   `DISK$label` in the system table. Don't seed `SYS$SYSTEM` etc. until
   something needs them.* **CONFIRMED: you can skip seeding SYS$SYSTEM for now.**
4. **eVAX command aliases.** Should `DEFINE/LOGICAL name value /TABLE=` and
   `SHOW LOGICAL_NAMES` be dropped outright (not VMS syntax), or kept as
   undocumented aliases for existing scripts? *Recommendation: drop them.
   `SHOW LOGICAL` (a prefix of `LOGICAL_NAMES`) keeps working anyway because
   of DCL keyword abbreviation.* **CONFIRMED: goal is to support VAX/VMS CLI
   syntax as much as possible; ignore eVAX-isms in favor of proper VMS emulation.
   Update the bootdata/files/ .help file accordingly.**
5. **Limits.** VMS 7.3 uses `LNM$C_NAMLENGTH` = 255 and `LNM$C_MAXDEPTH` = 10.
   The CLRM excerpt says 63 characters. *Recommendation: use the 7.3 values
   from the generated header.* The user should confirm that 7.3, not V4, is the
   target. **CONFIRMED: VMS 7.3 is the target version to emulate.**
6. **Old-style services.** Should `SYS$CRELOG`/`SYS$DELLOG`/`SYS$TRNLOG` (the
   pre-V4 interfaces, still present in 7.3 and in the P1 vector) be written as
   thin wrappers over the same database now, or later? *Recommendation: later,
   unless a target `.exe` in `testdata/exe` calls them. Subtask 8 checks.* 
   **CONFIMRED: let's implement the pre-V4 interfaces as wrappers; we don't know where a
   user of govax might get their .exe files they want to run.**
7. **Temporary defaults in input lists** (CLRM §2.2.3.3, `TYPE ALPHA,MAL:BETA,
   HIG:GAMMA`). This needs the file commands to accept comma lists, which
   today they don't. *Recommendation: out of scope for this phase. The subtask
   5 list type is the prerequisite, and it can become a later RMS/DCL phase.*
   **CONFIRMED: Agreed, we can defer updating TYPE for now.**

## Progress Log

### 2026-09-27 — Planning

- Requested by the user: make govax's logical-name emulation strong enough to
  support RMS file-name handling, and share it between the console, RMS, and
  the RTL/system-service layer. The request specified: three default tables
  (`LNM$PROCESS`/`LNM$GROUP`/`LNM$SYSTEM`) plus user-created ones; a
  process → group → system search order; iterative translation that restarts at
  the process table on every level; and `DEFINE/PROCESS|/GROUP|/SYSTEM|
  /TABLE=` sharing one handler through the DCL grammar. It pointed to CLRM
  §2.2 (pasted inline) and User's Manual Chapter 11 as the references.
- Mid-planning, the user said: "Assume any code you find in the existing govax
  is suspect; the VAX/VMS standard is our goal." That is why this phase
  replaces `internal/io/logical.go` and doesn't replicate eVAX behavior (see
  "Why this phase looks different").
- Survey of what exists (see the table above). The most important finding is
  that console `DEFINE/LOGICAL` defaults to the table `LNM_PROCESS`, while RMS
  only looks up whole-string names in a *table* called `LNM$FILE_DEV`. So today
  a name the operator defines is never seen by RMS, and `NAME:file` specs never
  translate. Neither `SYS$CRELNM` nor `SYS$DELLNM` exists, although both are in
  the P1 vector.
- Read User's Manual Chapter 11 (§11.1-11.9) for the directory/table structure
  (Tables 11.1-11.4), the `LNM$FILE_DEV` search list, access modes, translation
  attributes, search-list expansion order, and the leftmost-component rule.
  These are the sources for the design section.
- Checked the DCL tokenizer (`readValueToken`): an unquoted `A,B` is already
  one token, but `A, B` isn't. That's why subtask 5 adds a real list type.
- The user then said that extending the DCL package is in scope wherever the
  grammar falls short, since it will help later command-line emulation. That
  made subtask 5 a general grammar extension (lists, verb parameters alongside
  syntax redirects, mutually exclusive qualifiers, keyword lists) instead of
  the smallest possible fix.
- No code written. This document is the planning deliverable.

### 2026-09-27 — Open questions answered

- The user reviewed the plan and answered every open question (marked
  **CONFIRMED** in place):
  - Q1: no job table for now.
  - Q2: the UIC is passed into `lnm.NewDatabase`.
  - Q3: don't seed `SYS$SYSTEM` and the like. The user didn't object to MOUNT
    defining `DISK$label` in the system table, so that is taken as accepted.
  - Q4: drop the eVAX-isms and support only VMS syntax, and update the help
    file to match.
  - Q5: VMS 7.3 is the target.
  - Q6: **implement** `$CRELOG`/`$DELLOG`/`$TRNLOG` as wrappers now, because a
    govax user's `.exe` files could come from anywhere.
  - Q7: temporary defaults in input lists are deferred.
- Subtasks 6-9 were updated to match these answers.

### 2026-09-27 — Subtask 1: `$LNMDEF`/`$SSDEF` constants in `internal/vmsdef`

- **Sources.** The plan assumed an SDL-generated `lnmdef.h` like the FAB/RAB/
  RMS headers, but none exists on disk. A search found the real VMS 7.3 SDL
  *source*, `vmssrc_archive/v73/starlet_b64/lis/lnmdef.sdl` (module `$LNMDEF`,
  version X-3, 1997). For status codes it found
  `vmssrc_archive/v73/vest_dblrtl/lis/ssdef.txt`, a BLISS `LITERAL` listing of
  all 406 VAX `SS$_` codes. The `~/Projects/ods2/ssdef.h` that also turned up
  was rejected: it's a 50-line, third-party subset (Paul Nankervis's ODS2
  tool), not DEC's. Both archive files are copied verbatim into
  `reference/vms/`.
- **Generator.** `internal/vmsdef/gen` gained two parsers:
  - `sdl.go` handles only the SDL subset `$LNMDEF` uses: `aggregate` with
    `bitfield [length N] [mask] [fill]` members, and single or list
    `constant ... equals ... [increment] prefix ... tag ...`. It follows SDL's
    naming (`P$V_`/`P$M_`/`P$S_`, and `P$T_NAME` or `P$_NAME` for an empty
    tag).
  - `bliss.go` reads the `NAME, I, value` literal form.

  Both return an error on any syntax they don't recognise instead of skipping
  it, so a future source file with wider syntax can't silently produce a
  partial table.
- **Separate maps.** Output is two **new** maps, `LNMConstants` and
  `SSConstants`, not additions to `Constants`, because `.RMSDEF` defines every
  `Constants` entry as an assembler symbol. `TestConstants_onlyRMSFamilies`
  guards that separation. The regenerated `Constants` block was checked
  byte-for-byte identical to before.
- **Cross-checks.** The generated `$LNMDEF` values independently match what
  govax already hard-coded:
  - `internal/rtl/logicals.go`: `LNM$M_CASE_BLIND` = `0x2000000`,
    `LNM$_STRING` = 2 through `LNM$_MAX_INDEX` = 7.
  - `internal/io/logical.go`: `LNM$M_TABLE` = `0x8`, `LNM$M_TERMINAL` =
    `0x200`.
  - `LNM$C_NAMLENGTH` = 255 and `LNM$C_MAXDEPTH` = 10, confirming open
    question 5's 7.3 limits.

  `LNM$_CHAIN` (-1) is stored as `0xFFFFFFFF`.
- **Finding for subtask 8.** Three `SS$_` codes in `internal/rtl/status.go` are
  **not in the VAX 7.3 `$SSDEF` at all**: `ssNoSuchFac` = 9276, `ssInvArg` =
  4042, and `ssTooManyArgs` = 10060. No name in `ssdef.txt` has any of those
  values either. They probably come from eVAX or from a later/Alpha VMS.
  `SS$_TOOMANYARGS` matters here because `SYS$TRNLNM` returns it for more than
  5 arguments, so subtask 8 will re-check that path against real 7.3
  behavior. That path probably can't happen at all on VAX (a CALLS with extra
  arguments isn't rejected). `ssdef.txt` also has no `SS$_IVACMODE`, so the
  new services will need a different code for a bad access mode.
- **Pre-existing failure, not from this subtask.** `go test ./...` fails
  `internal/asm` `TestAssembleForth` ("expected non-empty S0 content"). It
  fails the same way with this subtask's changes stashed. It comes from commit
  `a21799c` ("Update forth.asm to run in user space"): the fixture no longer
  deposits into S0, but the test still expects it to. Left for the user.
  Everything else passes.

### 2026-09-27 — Subtask 2: `internal/lnm` core

- **Sources.** The implementation follows the *VMS 5.0 System Services Reference
  Manual* (`AA-LA69A-TE`, in the user's VMS docs folder) for `$CRELNM`,
  `$CRELNT`, `$DELLNM` and `$TRNLNM`: argument rules, access-mode handling,
  and the full lists of returned status codes. Status message texts come from
  DEC's own VMS 7.3 SYSMSG source listing
  (`vmssrc_archive/v73/msgfil/lis/sysmsg.lis`).
- **New package `internal/lnm`.** It provides `Database`, `Table`, `Entry`,
  `Equivalence` and `Mode`, plus:
  - `NewDatabase(uic)`, which lays out the startup directories and tables
    shown in the design section
  - `ResolveTables`, `Translate`, `Define`, `Delete`, `CreateTable`
  - `Match` and `HasWildcards` for SHOW LOGICAL

  Each service entry point is written to be a thin wrapper target for its
  `SYS$` service in subtask 8. The caller passes the access mode actually used;
  the "maximizing" against the caller's mode is left to the wrapper.
- **Behavior implemented from the manual:**
  - access-mode filtering of both names and table names (outermost visible
    entry wins)
  - `LNM$M_CASE_BLIND`
  - `SS$_SUPERSEDE` for a same-mode redefinition
  - `NO_ALIAS` (deletes outer-mode names; an inner-mode `NO_ALIAS` name blocks
    a define with `SS$_DUPLNAM`)
  - `CONFINE` inherited from the table or parent
  - a name can't be more privileged than its table, nor a table than its
    parent (`SS$_NOPRIV`)
  - name syntax in directory tables (1-31 letters, digits, `$`, `_`)
  - `$CRELNT`'s `CREATE_IF`, its supersede rule, and `SS$_PARENT_DEL`
  - deleting a table name deletes all its descendant tables
  - unique default `LNM$xxxx` table names
- **Privileges.** govax has no privilege model, so the caller is treated as
  holding every privilege. The tables created at startup are still protected:
  deleting or superseding them returns `SS$_NOPRIV`, and a refused request
  changes nothing. Either would break the database's structure, and real
  VMS protects them too.
- **One deliberate reading of the manual.** `$DELLNM` with no logical name
  deletes from "the first table whose access mode is equal to or less
  privileged than the caller's". Taken literally, no caller could ever empty
  the kernel-mode process table (`DEASSIGN/ALL`). So the table's own mode isn't
  checked; the per-name mode filter keeps inner-mode names safe. This is
  documented in the `Delete` doc comment.
- **`internal/vmserrors`.** Added `SS_NOPRIV`, `SS_DUPLNAM`, `SS_IVLOGNAM`,
  `SS_IVLOGTAB`, `SS_NOLOGNAM`, `SS_TOOMANYLNAM`, `SS_SUPERSEDE`,
  `SS_LNMCREATED`, `SS_PARENT_DEL` and `SS_NOLOGTAB`, using the real SYSMSG
  texts. The new `codes_sys_test.go` checks every `SS_*` constant against the
  generated `vmsdef.SSConstants`.
- **Finding for later.** The existing `SS_BADPARAM` message text is
  INITIALIZE-specific ("Unable to complete INITIALIZE operation on !S",
  Phase 23). Real VMS says "bad parameter value". `internal/lnm` returns
  `SS$_BADPARAM` for bad arguments, so once subtask 6 prints logical-name
  errors at the console the INITIALIZE wording would show up there. It was not
  changed in this subtask because INITIALIZE's output depends on it; subtask 6
  should decide (probably by giving INITIALIZE its own message).
- **Design corrections made while checking against the sources:**
  - `LNM$GROUP` belongs in the process directory (User's Manual Table 11.2).
    The plan's own table had put it under the system directory.
  - The plan said the `CREATE/NAME_TABLE` default parent was
    `LNM$PROCESS_DIRECTORY`. It is more likely `LNM$PROCESS_TABLE`; this is
    flagged for subtask 6 to confirm.
- **Tests.** `internal/lnm` has 96.4% statement coverage. The tests include
  the manual's `ACCOUNTS` two-mode example and §11.9.2's
  "add APPLICATION_NAMES to LNM$PROCESS" example, which shows that redefining
  `LNM$PROCESS` changes `LNM$FILE_DEV`'s search order. `go test ./...` passes
  except for the pre-existing `TestAssembleForth` failure noted under subtask 1.
  Nothing outside `internal/lnm`/`internal/vmserrors` uses the package yet;
  subtask 4 wires it in.

### 2026-09-27 — Subtask 3: `TranslateFileSpec`

- **New `internal/lnm/filespec.go`.** `Database.TranslateFileSpec(spec, mode)`
  returns `[]FileSpec` in search order. Each `FileSpec` has `Spec`,
  `Concealed`, and `Display`. It follows User's Manual §11.5-11.7:
  - Only the leftmost component is a candidate: the text before the first
    `:`, or the whole spec when there's no `:`. It must consist of letters,
    digits, `$`, `_`, and `-` (§11.3.3), so `[DRYSDALE]PUP` and `PUP.TXT` are
    never translated. A `::` node name and a leading `_` turn translation
    off.
  - The candidate is upper-cased and looked up with `Translate` through
    `LNM$FILE_DEV` (new constant `FileDevName`). Every level starts again at
    the top of the search order.
  - `TERMINAL` is checked per equivalence string, as the data model stores
    it.
  - Search lists fan out depth-first.
  - More than `MaxDepth` (10) translations in one chain, or a name reached
    twice in one chain, fails with `SS$_TOOMANYLNAM`. The same name in two
    *sibling* search-list elements is not circular.
  - An undefined name isn't an error. Any other `Translate` error is
    returned, for example `SS$_NOLOGTAB` if `LNM$FILE_DEV` has been deleted.
- **Two departures from the plan text**, both now reflected in the design
  section above:
  - A leading `_` is **kept** in the result, not stripped. A VMS resultant
    string keeps it (`_TTA0:`). Keeping it also means an equivalence such
    as `SYS$OUTPUT` = `_TTA0:` and an operator-typed `_DUA0:` look the
    same to the consumer. Subtask 7 has to strip it before the device
    lookup: `rms.normalizeDeviceName` doesn't do that today.
  - `Concealed` records the **outermost** concealed name in a chain. This is
    how VMS displays nested concealed names such as `SYS$SYSROOT`.
- **Open point for subtask 7: field merging.** The equivalence string and the
  rest of the spec are joined as text, so the manual's
  `DEFINE PAY_FILE DISK1:[SALES_STAFF]PAYROLL` / `TYPE PAY_FILE:*.DAT` gives
  `DISK1:[SALES_STAFF]PAYROLL*.DAT`. That happens to match the right file,
  but real RMS treats the equivalence's fields as defaults for fields the
  rest of the spec leaves out. That needs `filespec.Parse`, which belongs in
  `internal/rms`. If subtask 7 wants it, the cleanest route is to add a
  `Remainder` field to `FileSpec` so RMS can split the two parts. It was not
  added now, because nothing needs it yet.
- **Tests.** `filespec_test.go` covers every example the plan listed
  (REPORT, GO → TEST, the four PAY forms, PUP/`DISK:PUP`/`[DRYSDALE]PUP`,
  MEMO, GETTYSBURG, FIFI, NESTED). It also covers restarting the search
  order, `_` and `::`, per-equivalence `TERMINAL`, concealed display
  through iteration, the 10/11-level boundary, circular definitions,
  access modes, and a deleted `LNM$FILE_DEV`. `filespec.go` is 100%
  covered, and the package overall is 98.5%. `go test ./...` passes except
  for the pre-existing `TestAssembleForth` failure.
