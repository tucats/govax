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

**Status: planning.** No code yet.

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
  `reference/vms/` has no `lnmdef.h` yet. Add one (VMS 7.3 SDL-generated, like
  the FAB/RAB/RMS headers) and feed it through `internal/vmsdef/gen` so the
  `LNM$_*`/`LNM$M_*`/`LNM$C_*` values come from a generator instead of being
  typed in by hand (see subtask 1).

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
over. The `vax.DebugLogicals` debug flag stays and is used inside `internal/lnm`
through a small injected `io.Writer`/enabled hook, not a `vax` import.

### Data model

```text
Database
  tables  map[string]*Table          // keyed by full table name (LNM$PROCESS_TABLE, ...)
  uicGroup uint16                    // for LNM$GROUP_gggggg

Table
  Name        string                 // e.g. "LNM$PROCESS_TABLE"
  Parent      *Table                 // VMS parent-table hierarchy (SHOW LOGICAL/STRUCTURE)
  Directory   bool                   // LNM$PROCESS_DIRECTORY / LNM$SYSTEM_DIRECTORY
  Shareable   bool                   // process-private vs. shareable (group/system)
  AccessMode  Mode
  Attrs       TableAttr              // (future: SUPERSEDE/CONFINE/NO_ALIAS)
  entries     map[key]*Entry         // key = (name, access mode): one name may exist once per mode

Entry
  Name         string                // case preserved; lookup is case-sensitive unless CASE_BLIND
  AccessMode   Mode                  // user / supervisor / executive / kernel
  Attrs        NameAttr              // CONFINE, NO_ALIAS (stored; enforcement deferred)
  Equivalences []Equivalence         // ≥1; len>1 ⇒ search list, index 0..MAX_INDEX

Equivalence
  Value  string
  Attrs  TransAttr                   // CONCEALED, TERMINAL (per equivalence, as in VMS)
```

Mode and attribute bit values are the real `LNM$M_*`/`PSL$C_*` values from the
generated `vmsdef` constants, so what `$TRNLNM`'s `LNM$_ATTRIBUTES` item returns
is exactly what the database stores. No translation layer is needed.

When one table holds the same name in several access modes, the outermost
(least privileged) mode that the caller's access mode allows wins. This matches
the manual's `SYS$OUTPUT [super]` / `SYS$OUTPUT [exec]` example.

### Table names are themselves logical names (VMS directories)

As VMS does, tables are found by *translating a table name through the
directories*. The code never looks up the Go map directly. At startup,
`Database` creates:

| Directory | Name | Translates to |
| --- | --- | --- |
| `LNM$PROCESS_DIRECTORY` | `LNM$PROCESS` | `LNM$PROCESS_TABLE` |
| | `LNM$PROCESS_DIRECTORY` | (the directory itself) |
| `LNM$SYSTEM_DIRECTORY` | `LNM$SYSTEM` | `LNM$SYSTEM_TABLE` |
| | `LNM$GROUP` | `LNM$GROUP_gggggg` |
| | `LNM$FILE_DEV` | `LNM$PROCESS`, `LNM$GROUP`, `LNM$SYSTEM` (search list) |
| | `LNM$DCL_LOGICAL` | `LNM$FILE_DEV` |
| | `LNM$DIRECTORIES` | `LNM$PROCESS_DIRECTORY`, `LNM$SYSTEM_DIRECTORY` |
| | `LNM$SYSTEM_DIRECTORY` | (the directory itself) |

VMS puts `LNM$GROUP` in the process directory; the table above lists it under
the system directory, which is what govax will do. In practice this only
changes what `SHOW LOGICAL/STRUCTURE` prints. Open question 2 covers the fact
that govax has only one process.

A table-name argument (`/TABLE=`, `$TRNLNM`'s `tabnam`, `$CRELNM`'s `tabnam`) is
resolved like this:

1. Look the name up in `LNM$DIRECTORIES` (process directory first, then system)
   and translate it iteratively, keeping only names that resolve to real tables.
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
   - If the spec starts with `_`, strip the `_` and do no translation (the VMS
     rule that marks a physical device name).
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
     the return value is a small struct, not a bare string. Its exact shape is
     settled in subtask 3.

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
  be added". Qualifiers: `/PARENT_TABLE=` (default `LNM$PROCESS_DIRECTORY`,
  so the new table is process-private) and `/USER_MODE`/`/SUPERVISOR_MODE`/
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

1. **`internal/vmsdef` LNM constants.** Add `reference/vms/lnmdef.h` (plus
   `ssdef.h` entries for `SS$_NOLOGNAM`, `NOLOGTAB`, `TOOMANYLNAM`, `IVLOGNAM`,
   `IVLOGTAB`, `SUPERSEDE`, `NORMAL`, `DUPLNAM`, `NOPRIV`, if they aren't
   already available) and extend `internal/vmsdef/gen` to generate `LNM$_*`/
   `LNM$M_*`/`LNM$C_*` (`NAMLENGTH`, `MAXDEPTH`, `TABNAMLEN`). Unit tests pin
   the handful of values the design depends on.
2. **`internal/lnm` core.** `Database`, `Table`, `Entry`, and `Equivalence`, plus
   startup of the directory and default tables (the table in "Table names are
   themselves logical names"). Also:
   - `Define`/`Delete`/`CreateTable`/`DeleteTable`
   - resolving a table name through `LNM$DIRECTORIES`
   - single-level `Translate` with access-mode and case-blind handling
   - name/length validation (`SS$_IVLOGNAM`)
   - wildcard `Match` for SHOW LOGICAL

   Table-driven tests follow the manual's examples. Nothing is wired in yet.
3. **`internal/lnm` file-spec translation.** `TranslateFileSpec` covering:
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
   `/STRUCTURE`), `SHOW TRANSLATION`, and `CREATE/NAME_TABLE`. Update the
   console help text in `internal/bootdata/files/vax.help`.
7. **RMS file-spec translation.**
   - `$OPEN`/`$CREATE`, and the `Session` commands DIRECTORY/TYPE/COPY/
     DELETE/PURGE/SET DEFAULT/MOUNT, go through `TranslateFileSpec`.
   - Search lists work for `$OPEN` (first file found) and for wildcard
     commands (every element).
   - Map the error to `RMS$_LNE`.
   - `SYS$DISK`-driven default device.
   - Add end-to-end console tests against a real container, extending
     `internal/console/rms_e2e_test.go`'s pattern.
8. **System services.**
   - Rewrite `SYS$TRNLNM`.
   - Add `SYS$CRELNM`, `SYS$DELLNM`, and `SYS$CRELNT`, with full item-list
     handling (`LNM$_STRING`/`ATTRIBUTES`/`INDEX`/`MAX_INDEX`/`TABLE`/
     `LENGTH`/`ACMODE`, `LNM$_CHAIN`).
   - Add device-name translation for `SYS$ASSIGN`/`SYS$GETDVI`.
   - Rundown of user-mode names at image exit.
   - Add a MACRO-32 fixture (`testdata/asm/lnm_roundtrip.asm`) that runs
     `$CRELNM` → `$TRNLNM` → `$DELLNM` and writes through `SYS$OUTPUT` via a
     program-defined logical name, as an acceptance test.
9. **Legacy clean-up and docs.**
   - Resolve the eVAX-alias question (open question 4).
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
   directory-based design makes it a few lines to add later.*
2. **UIC group number.** `rtl.Environment` already has a nominal UIC
   (`nominalUIC`). The group table should be named from its group field
   (`LNM$GROUP_000001`, or whatever `nominalUIC` holds), which means the UIC has
   to move somewhere the console-owned `lnm.Database` can see it at
   construction. *Recommendation: pass the UIC into `lnm.NewDatabase`, and
   have `rtl.NewEnvironment` take it from the same place.*
3. **Default system-table and mount-time names.** Should `MOUNT` define the
   volume label as a logical name (as VMS does, in the system table for
   `/SYSTEM` mounts and otherwise the job table), and should
   `SYS$SYSDEVICE`/`SYS$SYSTEM` be seeded? *Recommendation: MOUNT defines
   `DISK$label` in the system table. Don't seed `SYS$SYSTEM` etc. until
   something needs them.*
4. **eVAX command aliases.** Should `DEFINE/LOGICAL name value /TABLE=` and
   `SHOW LOGICAL_NAMES` be dropped outright (not VMS syntax), or kept as
   undocumented aliases for existing scripts? *Recommendation: drop them.
   `SHOW LOGICAL` (a prefix of `LOGICAL_NAMES`) keeps working anyway because
   of DCL keyword abbreviation.*
5. **Limits.** VMS 7.3 uses `LNM$C_NAMLENGTH` = 255 and `LNM$C_MAXDEPTH` = 10.
   The CLRM excerpt says 63 characters. *Recommendation: use the 7.3 values
   from the generated header.* The user should confirm that 7.3, not V4, is the
   target.
6. **Old-style services.** Should `SYS$CRELOG`/`SYS$DELLOG`/`SYS$TRNLOG` (the
   pre-V4 interfaces, still present in 7.3 and in the P1 vector) be written as
   thin wrappers over the same database now, or later? *Recommendation: later,
   unless a target `.exe` in `testdata/exe` calls them. Subtask 8 checks.*
7. **Temporary defaults in input lists** (CLRM §2.2.3.3, `TYPE ALPHA,MAL:BETA,
   HIG:GAMMA`). This needs the file commands to accept comma lists, which
   today they don't. *Recommendation: out of scope for this phase. The subtask
   5 list type is the prerequisite, and it can become a later RMS/DCL phase.*

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
