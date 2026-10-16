# Phase 63 — working sets, soft faults, and the page lists

**Status:** in progress (2026-10-16): subtask 1, the survey (Part A
below), subtask 2, the working set lists (`internal/paging`'s
`WorkingSet`), subtask 3, faults, replacement, and soft faults in
corevms (`corevms/workingset.go`), and subtask 4, the limits (`$ADJWSL`
on the real list, WSAUTH and WSAUTHEXT, GROWLIM's key, the reset at
image exit), subtask 5, deleting pages in transition, and subtask 6,
the working set services (`$PURGWS`, the locks on the real list,
SS$_LKWSETFUL, the unlock statuses), and subtask 7, the modified page
writer for private file sections (`corevms/mpw.go`, MPW_HILIMIT and
MPW_LOLIMIT, `$UPDSEC` from the modified list), and subtask 8, the
reclaimer and the accounting (`corevms/reclaim.go`, FREELIM and
FREEGOAL; `corevms/accounting.go`, faults by kind, PPGCNT, GPGCNT,
WSPEAK), and subtask 9, DCL's SET WORKING_SET and SHOW WORKING_SET
(`corevms/setworkingset.go`, `dclcmd/workingset.go`), are done. Needs Phase 62 (settings, the
`internal/paging` PFN database and lists, the PTE forms, frame
allocation through `internal/paging`). Part of the paging program laid
out in `PHASE-62 - VMS paging.md` (Parts A and B: the survey and the
accepted decisions, which this doc doesn't repeat).

## Goal

With paging on, each process gets a **working set**, and pages start
leaving memory as they do on VMS:

- every valid P0/P1 page the pager brings in enters the process's
  working set list;
- when the list is full (WSLIMIT, with VMS's growth rules toward
  WSEXTENT), the oldest page leaves (Decision 6's modified FIFO);
- a page that leaves keeps its frame: its PTE takes the *transition*
  form, and the frame goes to the free list (clean) or the modified list
  (dirty, or "effectively modified": every demand-zero and
  copy-on-reference page);
- a fault on a transition page is a **soft fault**: the frame comes off
  its list, and no I/O happens;
- the working set services stop being bookkeeping: `$ADJWSL` changes the
  real limit, `$PURGWS` empties a range, `$LKWSET` locks pages in, and
  `$LCKPAG` locks them in memory;
- DCL's **SET WORKING_SET** (/LIMIT, /QUOTA, /EXTENT, /[NO]ADJUST) changes
  the current process's own values within what it was authorized at
  creation, the first per-process setting after creation (the author,
  2026-10-15), and SHOW WORKING_SET reports them;
- faults are counted per process as soft or hard, and the working set's
  sizes are reported.

There's no page file yet (Phase 64), so a dirty page that belongs in
the page file stays on the modified list. The one writer in this phase
writes modified pages of writable `$CRMPSC` file sections back to their
own files. Eviction in this phase therefore mostly moves pages between
the working set and the lists. Physical memory is freed only by clean
pages being reused and by deletion, as before.

## What earlier phases leave in place

- **Phase 62:** the `Sysgen` settings, copied into each process at
  creation. Only the PQL defaults and MINWSCNT exist so far; GROWLIM,
  FREELIM, FREEGOAL, and TBSKIPWSL are added by the subtasks that first
  read them, as Decision 11 says (the survey's finding 1). The PFN
  database with owners and backing store; the free
  list's head and tail rules; the transition PTE form; the set of frames
  translated since the last instruction boundary.
- **Phase 49:** `releasePTE`, the single exit point for a page leaving
  a PTE; private file sections (`processSection`); `writePage`.
- **Phase 26:** `$ADJWSL` (`process.go`, the `WSLimit` field), the
  page lock maps (`pageprot.go`), `$DELTVA`/`$CRETVA`.
- **Phases 43–44:** each process's `Environment` and address space, and
  the scheduler. A fault on a page of a process that isn't current
  (`TranslateIn`) still belongs to that process
  (`processForSpace`).

## Design notes

- **The working set list** lives in `internal/paging`, one per process:
  a circular list of entries (virtual address, global or private,
  locked), WSNEXT, the limit and the quotas, and PPGCNT and GPGCNT.
  Process handles are PIDs, so the package knows nothing of corevms.
- **Global section pages stay resident in this phase.** A page-file
  section's frames are valid in mapping PTEs from the start, and a file
  section's frames stay once read. They count in GPGCNT but are never
  chosen for replacement until Phase 65 gives the global page table its
  states. Private file section pages do go through the working set.
- **Replacement skips:** entries still valid in the TB (current process
  only: the TB holds no other process's entries), up to TBSKIPWSL;
  locked entries; and frames translated during the current instruction.
- **Image exit** shrinks the working set to WSDEFAULT, removing entries
  oldest first.
- **The reclaimer** (Decision 7) is built here but can only trim to the
  lists. It frees frames only when trimmed pages are clean, until Phase
  64's writer drains the modified list to the page file.
- **Soft faults count.** JPI$_PAGEFLTS counts every fault in paging
  mode, soft and hard. With paging off, the count is today's.

## Open questions

The survey (Part A) answered some of these from the manuals; the rest
go to the probe (subtask 10, the case list under "The probe's cases").

- **JPI$_WSSIZE:** *answered.* It is the working set **limit**, the
  size of the working set list, which `$ADJWSL` changes. VSI's
  `$GETJPI` text says "the current working set limit", and the book's
  16.4.1 says `$ADJWSL`'s effect "is to change the value of the WSSIZE
  working set list counter". The pages in use are PPGCNT + GPGCNT.
  govax's answer (`WSLimit`) stands; the probe confirms it.
- **Page table faults:** VMS's page tables are pageable and their faults
  count too. govax's page tables don't fault (Decision 13). The probe
  shows how much of VMS's counts that accounts for; differences are
  expected and listed, not chased.
- **SET WORKING_SET's limits:** *partly answered* (Part A, "DCL"). Each
  value is capped by an authorized value, not by the PQL_M minimums.
  /QUOTA is capped at WSAUTH, and /EXTENT and /LIMIT at WSAUTHEXT. A
  /LIMIT above the quota raises the quota too. /EXTENT must exceed
  MINWSCNT. The probe settles the rest: the message, the lower bounds,
  and how /NOADJUST shows.
- **Statuses:** *partly answered* (Part A, "The services"). The
  unlock services' WASSET and WASCLR rule is the reverse of the lock
  services', and govax has it wrong today. The SS$_LKWSETFUL threshold
  is the book's "extra dynamic working set" rule, which govax has to
  translate (Part A, finding 6). `$PURGWS` on locked pages, NOPRIV for
  S0, and `$ULKPAG`'s privilege go to the probe.

# Part A — the survey (subtask 1, 2026-10-15)

## Sources

- **VMS 5.0 System Services Reference Manual** (the PDF, extracted with
  `pdftotext -layout` to a scratch file): `$ADJWSL` (SYS-10), `$LCKPAG`
  (SYS-335), `$LKWSET` (SYS-337), `$PURGWS` (SYS-370), `$ULKPAG`
  (SYS-526), `$ULWSET` (SYS-528), and `$GETJPI`'s WS items (SYS-233).
- **VSI's system services text** (`VSI-system-services-1.txt`, `-2.txt`;
  entries under their descriptive names, "Adjust Working Set Limit",
  "Purge Working Set", "Lock Pages in Working Set", "Lock Pages in
  Memory", "Unlock Pages from ..."), and its `$GETJPI` and `$CREPRC`
  items (PQL$_WSDEFAULT, WSQUOTA, WSEXTENT).
- **DCL Dictionary**, VMS 7.3-1, N–Z: SET WORKING_SET (DCLII–287) and
  SHOW WORKING_SET (DCLII–434). The text file runs the examples' lines
  together, so they were read from the PDF with its layout kept.
- ***VAX/VMS Internals and Data Structures***: 14.1.2 (the working set
  list, figures 14-4 and 14-5), 15.2.1 (a page's removal and soft
  faults), 15.4 (replacement), 16.4 (the services, the SET WORKING_SET
  command, automatic adjustment, Tables 16-1 and 16-2), 10.1 (automatic
  adjustment, scheduling side), 20 (the working set at process
  creation), and 21.3 (image rundown).
- **Earlier VMS runs:** `testdata/mp/probe2/vax/probe2.log` (SYSGEN's
  PQL parameters on the VMS 7.3 VAX) and `testdata/mp/probe5` (a
  `$CREPRC` child's DFWSCNT and WSQUOTA, 4 pages over its creator's).

## The services

All six take their arguments as the manuals give them. On a VAX a
pagelet is a page, so VSI's rounding doesn't apply. Each address range
uses only the virtual page numbers of its two addresses.

**`$ADJWSL [pagcnt] ,[wsetlm]`**: adds the signed `pagcnt` to the
working set limit. It never fails for a bound: a limit above WSEXTENT
or below MINWSCNT is quietly clamped. With `pagcnt` 0 or omitted,
nothing changes. `wsetlm` receives the new limit, or with no
adjustment the current one. No privilege or quota is needed. Statuses:
SS$_NORMAL, and SS$_ACCVIO if `wsetlm` can't be written. The book adds
the mechanics (16.4.1):

- Growing the list is bounded by WSEXTENT only. **No free-list check
  (BORROWLIM) is made**: that check belongs to automatic adjustment.
- Shrinking is bounded by MINWSCNT. If WSNEXT points beyond the new
  end, it is moved back to the new end. The book says WSLAST moves back
  "as a side effect of freeing excess working set list entries (above
  the new limit)". That suggests the pages over the new limit are
  removed, but it doesn't say when. The probe checks PPGCNT straight
  after a shrink.

**`$PURGWS inadr`**: removes every page in the range that is in the
working set. A range of 0 to 7FFFFFFF purges all of P0 and P1, and the
image continues, faulting pages back as it needs them. With equal
addresses, only that page goes. No privilege. The 5.0 manual lists
SS$_NORMAL and SS$_ACCVIO (`inadr` unreadable). VSI's text adds
SS$_NOPRIV for a page in system space, which the probe checks on 7.3.
Neither manual says what happens to locked pages in the range. The
book says VMS itself purges the working set at image startup "to
insure that a program starts its execution without unnecessary pages
(such as CLI command processing routines)" (16.4.1).

**`$LKWSET inadr ,[retadr] ,[acmode]`** and **`$LCKPAG`** (same
arguments): fault each page into the working set if it isn't there, and
lock it. `acmode` is maximized with the caller's mode, and must be at
least as privileged as each page's owner. `retadr` receives the range
done; if nothing was done, both longwords are -1. A page locked in the
working set is never replaced. A page locked in memory also stays when
the process is outswapped, and is also locked in the working set.
Statuses:

| Status | `$LKWSET` | `$LCKPAG` |
|---|---|---|
| SS$_WASCLR | success: **every** page was unlocked before | the same |
| SS$_WASSET | success: **at least one** page was already locked | the same |
| SS$_ACCVIO | `inadr` unreadable, `retadr` unwritable, a page missing or inaccessible | the same |
| SS$_NOPRIV | a system page, or a **global page with write access** | no PSWAPM privilege |
| SS$_LKWSETFUL | the locked working set is full (below) | — |
| SS$_LCKPAGFUL | — | the system's limit on pages locked in memory |
| SS$_PAGOWNVIO | `acmode` less privileged than a page's owner | VSI's list only (5.0 omits it) |

VSI also lists SS$_LDWSETFUL for `$LCKPAG`. VMS 7.3's definitions don't
have it (`vmsdef.Symbols` lacks it), so it's an Alpha addition and
govax leaves it out. Writable global pages can't be locked in the
working set because the swapper drops them at outswap (book 16.4.2.1).
`$LCKPAG` may lock them. Locked pages are unlocked at image exit, and
the 5.0 manual's `$ULKPAG` entry says pages locked in memory "are
automatically unlocked and deleted at image exit".

**`$ULWSET`** and **`$ULKPAG`** (same arguments) unlock. Both manuals
give the success statuses **the other way round** from the lock
services:

| Status | Meaning (both unlock services) |
|---|---|
| SS$_WASCLR | at least one page was **unlocked** already |
| SS$_WASSET | **every** page was locked |
| SS$_ACCVIO | as for locking |
| SS$_NOPRIV | `$ULWSET`: a system page. `$ULKPAG`: not listed |

The 5.0 manual's `$ULKPAG` description says the caller must have
PSWAPM, but its status list has no SS$_NOPRIV. VSI's text says the same.
The probe settles what VMS returns without the privilege.

**The LKWSETFUL rule** (book 16.4.2.1): a lock must leave "enough
dynamic working set". The *extra dynamic working set* (PHD$W_EXTDYNWS)
is the dynamic part of the list, less WSFLUID (MINWSCNT) and room for
page table pages. It must stay above zero. Locking moves each locked
entry to the front of the list (between WSLOCK and WSDYN), swapping it
with the entry at WSDYN, so the locked entries come out of the dynamic
part. `$LCKPAG` has no LKWSETFUL of its own in either list.

## DCL: SET WORKING_SET and SHOW WORKING_SET

**SET WORKING_SET** (no parameters) changes the current process's
values within its authorized ones:

- **/LIMIT=n** is the size the working set is reduced to **at image
  exit**: DFWSCNT, the WSDEFAULT that each image starts with. It doesn't
  change the current limit. A value above the current quota raises the
  quota too. Per the book (16.4.1.2), neither /LIMIT nor /EXTENT may
  exceed WSAUTHEXT.
- **/QUOTA=n** is WSQUOTA, the most an image can request with
  `$ADJWSL` without borrowing. A value above WSAUTH is set to WSAUTH.
- **/EXTENT=n** is WSEXTENT, the most `$ADJWSL` can reach. It must
  exceed MINWSCNT. A value above WSAUTHEXT is set to WSAUTHEXT (the
  Dictionary's text says "sets the working set limit at the maximum
  authorized value", read as the extent).
- **/[NO]ADJUST** (default /ADJUST) turns automatic adjustment on or
  off for the process (PCB$V_DISAWS, the book's Table 12-2). The book
  (16.4.1.2) adds that adjustment is off
  anyway for a process whose quota equals its limit.
- **/[NO]LOG** (default /NOLOG) shows a confirmation. But both of the
  Dictionary's examples show `%SET-I-NEWLIMS` without /LOG, so the probe
  checks when VMS shows it.

The examples, as laid out in the PDF:

```
$ SHOW WORKING_SET
  Working Set      /Limit= 150 /Quota= 700  /Extent= 700
  Adjustment enabled    Authorized Quota= 700 Authorized Extent= 700
$ SET WORKING_SET/QUOTA=1000
%SET-I-NEWLIMS, new working set: Limit = 150 Quota = 700 Extent = 700
```

The first example tried a quota above the authorized 700 and got 700.
The second set /LIMIT=100, after which SHOW gave `/Limit= 100`. So
**SHOW WORKING_SET's /Limit is DFWSCNT, not WSSIZE** (an image is not
running when DCL shows it, so after image exit the two agree anyway).
The PDF's column spacing isn't reliable (the 7.3-1 VAX example has
`/Extent=1200` with no blank), so the probe captures the real layout,
and the text of NEWLIMS, which `vmsdef.Messages` doesn't have (no SET
facility messages are generated).

**SHOW WORKING_SET** (/OUTPUT[=file], /NOOUTPUT) shows, on a VAX,
those two lines: the limit, quota, and extent; whether adjustment is
enabled; and the authorized quota and extent.

## The book's mechanics the design needs

**The list** (14.1.2, figure 14-4) is a circular list in the process
header, in three parts: the permanently locked pages (WSLIST to
WSLOCK: the kernel stack, the P1 pointer page, P1 page table pages, the
process header's own pages), the pages locked by request (WSLOCK to
WSDYN), and the dynamic part (WSDYN to WSLAST), where WSNEXT is the
entry added most recently. Each entry (figure 14-5) is the page's
virtual page number with control bits: valid, the page type (process,
system, global read-only, global read/write, page table pages), locked
in memory, locked in the working set, and a saved modify bit.

**Table 16-1's fields**, and govax's:

| VMS field | Meaning | Set by | govax today |
|---|---|---|---|
| PHD$W_WSSIZE / WSLAST | the list's current size (the limit) | `$ADJWSL`, image exit, automatic adjustment | `Process.WSLimit` |
| PHD$W_DFWSCNT | default size, reset to at image exit | LOGINOUT, SET WS/LIMIT | `WSDefault` |
| PHD$W_WSQUOTA | normal limit | LOGINOUT, SET WS/QUOTA | `WSQuota` |
| PHD$W_WSEXTENT | the most the list may reach | LOGINOUT, SET WS/EXTENT | `WSExtent` |
| PHD$W_WSAUTH | upper bound on WSQUOTA | LOGINOUT, never changed | **none** (JPI$_WSAUTH returns `WSQuota`) |
| PHD$W_WSAUTHEXT | upper bound on WSEXTENT | LOGINOUT, never changed | **none** (JPI$_WSAUTHEXT returns `WSExtent`) |
| PHD$W_WSFLUID | least dynamic size (MINWSCNT) | process creation | `MinWSCount` |
| PCB$W_PPGCNT, GPGCNT | process and global pages in the set | the pager | **none** |
| SYSGEN WSMAX | most any list may be; WSEXTENT and WSAUTHEXT are minimized with it | SYSGEN | **none** |

At process creation (chapter 20) WSQUOTA, WSAUTH, WSEXTENT, and
WSAUTHEXT come from the quota list, minimized with WSMAX. DFWSCNT and
WSLAST start at the default. govax's WSAUTH and WSAUTHEXT are
therefore a process's WSQUOTA and WSEXTENT as created, which the plan
already assumed.

**Removal** (15.2.1, item 4): the entry is made available; the PTE's
modify bit is ORed into the frame's saved modify bit; the PTE's valid
and TYP bits are cleared and its PFN left (the transition form); the
reference count drops, and at zero the frame goes to the free or
modified list by the saved modify bit. Pages leave a working set only
when room is needed, when they're deleted, or through `$PURGWS`.

**Replacement** (15.4, MMG$FREWSLE):

1. If the entry at WSNEXT is free, use it (after the growth checks
   below).
2. Otherwise advance WSNEXT, wrapping from WSLAST to WSDYN.
3. Skip entries locked in the dynamic part (only page table pages
   are; pages locked by request were moved out of it).
4. Skip entries still valid in the TB, up to TBSKIPWSL of them; after
   that, reuse the next entry whatever its state (TBSKIPWSL 0 turns the
   check off).

**Growth** (15.4.3), before a free entry may take a new page:

1. If the pages in use (PPGCNT + GPGCNT) equal WSSIZE, the working set
   is full: reuse the next entry. Free entries at the end of the list
   are given back by moving WSLAST down.
2. Below WSQUOTA, a new page is always allowed.
3. At or above WSQUOTA, it is allowed only while the free list has
   more than GROWLIM pages. Otherwise the next entry is reused.

**Automatic adjustment** (16.4.1.3, Table 16-2) runs at quantum end,
for a process that hasn't turned it off and whose DFWSCNT isn't equal to
its WSQUOTA, every AWSTIME of CPU time. If the fault rate is at least
PFRATH, the list grows by WSINC: below WSQUOTA always, above it only
while the free list has more than BORROWLIM pages. If the rate is below
PFRATL, it shrinks by WSDEC, but not below AWSMIN. PFRATL has been 0
since VMS 3.1, so in practice it only grows. The change is made by a
kernel AST that calls `$ADJWSL`.

**Image rundown** (21.3, step 7) runs MMG$IMGRESET: P0 is deleted, as
is P1 below the permanent part. The working set is reset to DFWSCNT,
undoing any `$ADJWSL` and automatic adjustment, and WSLAST is reduced.
Page locks go with the deleted pages.

## What govax has today

- **`$ADJWSL`** (`process.go`): clamps to [MinWSCount, WSExtent] and
  stores `WSLimit`, as the manuals say. **Image rundown doesn't reset
  `WSLimit` to `WSDefault`**: `ImageRundown` has no such step. VMS
  does, with paging on or off, so this is a fix for subtask 4 in both
  modes. It changes what `$GETJPI` WSSIZE shows in a second image, so
  any oracle that depends on it must be checked.
- **`$PURGWS`** isn't registered: a call ends in the services' missing
  path. It has to be added (subtask 6), in both modes: with paging off
  it checks its argument and returns SS$_NORMAL.
- **`$LKWSET`, `$ULWSET`, `$LCKPAG`, `$ULKPAG`** (`pageprot.go`)
  remember locks in two maps, by address, and check ranges, owners, and
  `retadr` as the manuals say, but:
  - the **unlock services' statuses are wrong**: they return WASSET if
    any page was locked. The manuals give WASSET only if every page was
    locked, and WASCLR otherwise. This is a bug in both modes (subtask 6;
    a DEVIATIONS.md entry, since it changes a status a program sees);
  - `$LKWSET` doesn't refuse a writable global page (SS$_NOPRIV), and
    returns no SS$_LKWSETFUL;
  - `$ULKPAG` doesn't check PSWAPM (the probe decides whether it should);
  - `$LCKPAG` checks PSWAPM before reading `inadr`. The order matters
    only when both are wrong; the probe can show it.
- **`$GETJPI`** (`getjpi.go`): WSSIZE is `WSLimit` (right). WSAUTH and
  WSAUTHEXT return WSQUOTA and WSEXTENT, which is right only until SET
  WORKING_SET changes those two. PPGCNT, GPGCNT, and WSPEAK aren't
  items yet (the symbols are in `vmsdef`: JPI$_PPGCNT 0x30D, GPGCNT
  0x30C, WSPEAK 0x201). F$GETJPI's WS keywords use the same table.
- **`$GETSYI`**: MINWSCNT only. The symbols for GROWLIM, BORROWLIM,
  FREELIM, FREEGOAL, TBSKIPWSL, WSMAX, MPW_HILIMIT, PFRATL, and WSINC
  are in `vmsdef`.
- **Fault counts** (`filesec.go`'s `countFault`): only faults the pager
  resolves (demand-zero and section pages). **SHOW SYSTEM's "Page
  flts" is always 0** (`showsys.go` passes 0), and so are LOGOUT's
  "Page faults" and "Peak working set size" (`logout.go`) and the
  termination message's ACC$L_WSPEAK. SHOW SYSTEM's "Pages" counts the
  valid P0/P1 PTEs (`mappedPages`), which stands in for PPGCNT +
  GPGCNT.
- **Quotas**: `applyQuotas` gives a `$CREPRC` process its DFWSCNT and
  WSQUOTA plus 4 pages (`wsCreationPages`, from probe 5: 516 and 1028
  for 512 and 1024). The VMS 7.3 VAX's SYSGEN (probe 2) has PQL_DWSDEFAULT
  294, PQL_MWSDEFAULT 512, PQL_DWSQUOTA 588, PQL_MWSQUOTA 1024,
  PQL_DWSEXTENT and PQL_MWSEXTENT 16400: on that system the minimums
  are above the defaults.
- **No DCL commands**: neither SET WORKING_SET nor SHOW WORKING_SET is
  in `console.dcl`.
- **No STARLET macros** for the six services: `starlet.mar` has no
  `$ADJWSL`, `$PURGWS`, `$LKWSET`, `$ULWSET`, `$LCKPAG`, or `$ULKPAG` in
  any form. The probe can be written with `CALLS G^SYS$...`. Programs
  that use the macros need them, checked against real MACRO's objects
  as `testdata/mp/macros` checked the others. *(Wrong, found in subtask
  6: five of them were there, checked by the macro probes' rounds 3 and
  4, `testdata/mp/macros/ext_*.mar` and `r4_*.mar`. Only `$PURGWS` was
  missing.)*
- **Where P0/P1 pages become valid**: only through the pager
  (`System.PageIn`: demand-zero and section pages), and through
  `$MGBLSC`/`$CRMPSC` of a global section, whose mapping PTEs are valid
  from the start (`gblsec.go`). Image activation writes its pages
  through `StoreIn`, so they fault in through the pager too. The P1
  vector's PTEs (`mapShared`) and VMINIT's S0 are system pages.
- **The modify bit**: every write translation sets it in the PTE
  (`translate.go`, and `resolvePTE` for `TranslateIn`), so a removed
  page's PTE says whether it was written. The TB caches the bit, so a
  removal must read the PTE and invalidate the page's TB entry. That
  empties the STC and with it the instruction-fetch window.
- **The TB** answers no "is this page valid in the TB" question yet.
  `tb.go` needs one for TBSKIPWSL (current process only, P0/P1 slots).

## Findings that change the plan

1. **Phase 62 left only MINWSCNT and the PQL keys in `Sysgen`.**
   GROWLIM (subtask 4), TBSKIPWSL (3), FREELIM and FREEGOAL (8), and
   MPW_HILIMIT and MPW_LOLIMIT (7) come with the subtasks that read
   them, each in `validConfigs` and HELP. **BORROWLIM has no reader in
   this phase**: only automatic adjustment uses it, and `$ADJWSL`
   doesn't check it. It waits for automatic adjustment, if a probe
   ever asks for that (Decision 6). Without it, the list grows past
   WSQUOTA only through `$ADJWSL`, and the working set fills it past
   WSQUOTA only while the free list has more than GROWLIM pages.
2. **WSAUTH and WSAUTHEXT become fields** (`Process.WSAuth`,
   `WSAuthExt`), set wherever WSQUOTA and WSEXTENT are first set
   (`applyQuotas`, `LoginQuotas`) and never changed. SET WORKING_SET
   bounds by them, and `$GETJPI` reports them (subtask 9, or 4 if the
   limit code needs them first).
3. **Image exit resets WSLIMIT to DFWSCNT in both modes** (subtask 4).
   With paging on, the reset also removes the entries above the new
   limit, oldest first, as the doc's design notes already said.
4. **`$PURGWS` walks the working set, not the range.** A purge of 0 to
   7FFFFFFF is the documented "purge everything", 4 million pages: the
   service goes through the list's entries and removes those in the
   range. With paging off it checks `inadr` and returns SS$_NORMAL. A
   range that reaches S0 is SS$_NOPRIV if the probe agrees with VSI.
5. **The unlock services' statuses are a bug to fix in subtask 6**,
   in both modes, with a DEVIATIONS.md entry.
6. **LKWSETFUL in govax's terms.** govax's list has no permanently
   locked part (Decision 13: kernel stacks, the PCB page, and page
   tables are system pages), and no page table pages in it. So the
   book's rule becomes: a lock fails with SS$_LKWSETFUL if, after it,
   fewer than MINWSCNT entries of the current limit would be unlocked.
   This is unconfirmed until the probe finds VMS's threshold (locking
   page by page until it fails, at a known WSSIZE and MINWSCNT). VMS's
   count includes its permanent pages, so an offset is expected.
7. **LCKPAGFUL**: the "system-defined maximum" is no parameter either
   manual names. govax never returns it (unconfirmed), unless locking
   would take the free list below FREELIM. Chosen: never.
8. **Writable global pages can't be locked in the working set**
   (SS$_NOPRIV), in both modes, but can be locked in memory.
9. **Pages that are valid without a fault** (global section mappings)
   enter the working set when they're mapped, counted in GPGCNT, and
   locked against replacement until Phase 65 (the doc's design notes).
   VMS faults each of them softly on first touch, so after a `$MGBLSC`
   govax's fault count is lower than VMS's by the pages touched. That's
   a listed difference until Phase 65.
10. **When nothing can be replaced.** Pages translated during the
    current instruction aren't chosen (Decision 6). A service runs in
    Go inside one instruction (the CHMK or XFC that called it), so a
    `$QIO` read into a buffer larger than the working set would find
    every entry protected. The list then grows past its limit for the
    rest of that instruction, and comes back down at the next fault
    after it (the oldest entries over the limit go first). VMS doesn't
    face this: its services lock I/O buffers and fault page by page.
    Subtask 3 tests it, and changed "at the next fault" to "at the next
    instruction boundary" (`vm.Memory.AtNextInstruction`): the image
    activator writes every page of an image in one Go call, and an image
    that then touched only its own pages never faulted again, so it kept
    them all.
11. **Image startup purge** (book 16.4.1). VMS purges the working set
    when an image starts. govax's DCL is Go and has no pages to drop,
    but the purge also drops the previous image's P1 pages. Whether
    govax should purge at RUN depends on the probe's first PAGEFLTS
    reading in a fresh image; it's a question for subtask 10, and Phase
    66 (mapped activation) is where image start's fault counts matter.
12. **SHOW WORKING_SET's /Limit is DFWSCNT.** SET WORKING_SET/LIMIT
    changes DFWSCNT, never the current limit, and SHOW shows DFWSCNT
    (finding from the Dictionary's second example).
13. **The +4 pages at creation** (`wsCreationPages`) may be VMS's
    permanently locked pages: the kernel stack, the P1 pointer page,
    and two page table pages make four. That's the book's list in
    14.1.2.1, but this is a guess. govax's list has no permanent part,
    so the 4 pages stay as limit bookkeeping. The probe's PPGCNT after
    a full purge shows the residue VMS keeps.

## The probe's cases (subtask 10)

The probe (`testdata/pg63`) is a program, a command procedure, and
`exchange.cmd`, as `testdata/lib5x` were. It reads every count with
`$GETJPI` (WSSIZE, PPGCNT, GPGCNT, WSPEAK, PAGEFLTS, DFWSCNT, WSQUOTA,
WSEXTENT, WSAUTH, WSAUTHEXT) and prints differences where values vary
from run to run.

- **SYSGEN SHOW** of MINWSCNT, WSMAX, GROWLIM, BORROWLIM, FREELIM,
  FREEGOAL, TBSKIPWSL (a SHOW/SPECIAL parameter), MPW_HILIMIT,
  MPW_LOLIMIT, MPW_WRTCLUSTER, PFCDEFAULT, PFRATL, PFRATH, WSINC,
  WSDEC, AWSTIME, AWSMIN, and SWPOUTPGCNT (Decision 11's table), and
  SHOW MEMORY.
- **DCL:** SHOW WORKING_SET at login; SET WORKING_SET with each
  qualifier below, at, and above its bound (/LIMIT above the quota,
  /QUOTA above WSAUTH, /EXTENT above WSAUTHEXT and at or below
  MINWSCNT), with and without /LOG, /NOADJUST then SHOW, and no
  qualifier at all; F$GETJPI of the WS items after each; the same in a
  SPAWNed subprocess (its authorized values).
- **`$ADJWSL`:** 0; +n; a huge +n (WSEXTENT?); a huge -n (MINWSCNT, or a
  higher floor?); PPGCNT straight after a shrink below the pages in use;
  `wsetlm` unwritable; after a grow, a sweep over the new limit's pages
  (is the room used at once, or only once the list wraps? Subtask 2's
  `Resize` chose at once).
- **Patterns** (each after `$PURGWS` of everything, at a limit set with
  `$ADJWSL`): touch N pages of a large array once, read and write,
  for N below the limit, at it, and above it (a cyclic sweep over more
  pages than the limit faults on every touch); sweep again (soft
  faults); report PAGEFLTS deltas and PPGCNT. The residue after a full
  purge (the code page, the stack, the permanent pages).
- **`$PURGWS`:** a single page; a reversed range; a range with locked
  pages (do they stay?); a range into S0 (NOPRIV?); `inadr` unreadable.
- **`$LKWSET`/`$ULWSET`:** lock a page twice (WASCLR, then WASSET);
  lock a range partly locked; lock pages one at a time until
  SS$_LKWSETFUL (the threshold, at two limits); a writable global page
  (NOPRIV); a system page; a missing page (ACCVIO, `retadr` -1); unlock
  all-locked (WASSET), mixed (WASCLR), none locked; PAGOWNVIO with an
  `acmode` of user on a page owned by executive mode, if there is one to
  hand (the P1 vector is kernel).
- **`$LCKPAG`/`$ULKPAG`:** with and without PSWAPM (SET PROCESS/PRIV),
  `$ULKPAG` without it, a writable global page (allowed?).
- **Image exit:** a second image reads WSSIZE (reset to DFWSCNT?) and
  finds its predecessor's locks gone; WSPEAK across images (per image or
  per process?); PAGEFLTS at the second image's first instruction (the
  image startup purge).
- **Macros:** `$ADJWSL`, `$PURGWS`, `$LKWSET`, `$ULWSET`, `$LCKPAG`,
  `$ULKPAG` in their `_S`, argument-list, and `_G` forms, as a round of
  `testdata/mp/macros`, if subtask 6 adds them to govax's STARLET.

## Subtasks (provisional; adjusted by the survey)

1. **Survey** (done): `$ADJWSL`, `$PURGWS`, `$LKWSET`/`$ULWSET`,
   `$LCKPAG`/`$ULKPAG` from the 5.0 manual and VSI's text (arguments,
   statuses, privilege); SHOW WORKING_SET and SET WORKING_SET in the DCL
   Dictionary; the book's 14.1.2, 15.4, and 16.4 details the design
   needs. Part A.
2. **Working set lists** in `internal/paging`, with `Check`: add,
   replace (Decision 6, the book's MMG$FREWSLE steps), remove, lock
   (locked entries out of the dynamic part), and growth past the limit
   when nothing can be replaced (finding 10). Unit tests. (Done
   2026-10-16; the progress log says what it chose.)
3. **corevms:** a resolved fault adds the page to the working set,
   replacement removes one (transition PTE, frame to a list, the TB
   entry invalidated), and a soft fault takes a transition page back.
   TBSKIPWSL (its key, and a `vm` query for a page valid in the TB);
   global mappings enter the list at mapping time (finding 9). (Done
   2026-10-16; the progress log says what it chose.)
4. **Limits:** WSQUOTA, GROWLIM (its key), WSEXTENT, and MINWSCNT;
   `$ADJWSL` on the real list (a shrink removes the excess); WSAUTH and
   WSAUTHEXT as fields (finding 2); the reset to DFWSCNT at image exit,
   in both modes (finding 3). No BORROWLIM (finding 1). (Done
   2026-10-16; the progress log says what it chose.)
5. **Every exit path handles transition pages:** `releasePTE`,
   `TeardownAddressSpace`, `$DELTVA`, `$CRETVA` over existing pages,
   and `unmapSections`. Deleted pages' frames go to the head of the free
   list. (Done 2026-10-16; the progress log says what it chose.)
6. **The working set services:** `$PURGWS` (new, both modes; it walks
   the list, finding 4); real `$LKWSET`/`$ULWSET` and
   `$LCKPAG`/`$ULKPAG`; the unlock statuses fixed in both modes
   (finding 5, DEVIATIONS.md); SS$_LKWSETFUL (finding 6) and NOPRIV for
   a writable global page (finding 8); the six services' macros in
   govax's STARLET. (Done 2026-10-16; the progress log says what it
   chose.)
7. **Private file section pages through the working set:** a clean
   page's PTE returns to its PSTX when its frame is reused, and a dirty
   write-back page is written to its file from the modified list. The
   writer's thresholds are MPW_HILIMIT and MPW_LOLIMIT (their keys), as
   Phase 64's will be. (Done 2026-10-16; the progress log says what it
   chose.)
8. **The reclaimer** (FREELIM, FREEGOAL, their keys), and accounting:
   soft and hard counts; `$GETJPI` PPGCNT, GPGCNT, WSPEAK (WSSIZE stays
   the limit); `$GETSYI`'s paging parameters; SHOW SYSTEM's page faults
   and pages, LOGOUT's page faults and peak working set, and the
   termination message's WSPEAK, all 0 or approximations today. (Done
   2026-10-16; the progress log says what it chose.)
9. **DCL SET WORKING_SET and SHOW WORKING_SET** (the console's grammar
   and `internal/dclshell`), in process 1 and in subprocesses, through
   the same code as `$ADJWSL`; SHOW's /Limit is DFWSCNT (finding 12).
   (Done 2026-10-16; the progress log says what it chose.)
10. **The VMS probe** (`testdata/pg63`), Part A's case list, and a
   macro round for the six services. govax is then changed to its
   answers.
11. **Close-out:** CLAUDE.md, HELP, PLAN.md.

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author put SET WORKING_SET in this phase
  (subtask 9).
- **2026-10-15:** Subtask 1, the survey (Part A): the six services
  from the 5.0 manual and VSI's text, SET and SHOW WORKING_SET from the
  DCL Dictionary, and the book's working set list, replacement, growth,
  adjustment, and image reset. JPI$_WSSIZE is the limit (answered).
  Found: the unlock services' WASSET/WASCLR rule is reversed in govax;
  `$PURGWS` isn't implemented; image exit doesn't reset the limit;
  WSAUTH and WSAUTHEXT have no fields of their own; BORROWLIM has no
  reader without automatic adjustment; SHOW SYSTEM's and LOGOUT's fault
  counts are 0; Phase 62 added only MINWSCNT and the PQL keys to
  `Sysgen`. The subtasks are adjusted to match, and the probe's cases
  are listed.
- **2026-10-16:** Subtask 2, the working set lists
  (`internal/paging/workingset.go`, tests in `workingset_test.go`). A
  `WorkingSet` is one process's list: a slice of entries (page address,
  global or private, lock bits), the locked part before `dyn` (WSDYN)
  and the dynamic part after it, a circular buffer whose newest entry
  is `next` (WSNEXT); a map by address stands in for the PFN database's
  WSLX array. Its size is the limit (WSSIZE), its `Bounds` are WSQUOTA,
  WSEXTENT, and MINWSCNT (WSFLUID), and it counts PPGCNT, GPGCNT, the
  locked entries, and the peak (WSPEAK). What a choice depends on
  outside the list comes in as `Conditions`: the free list's length and
  GROWLIM, TBSKIPWSL, and two questions about a page (`ValidInTB`, and
  `Protected`, which the caller answers from the frames translated this
  instruction; subtask 3 can use it for global pages too). The
  operations:
  - **`Add`** follows the book's 15.4 as written: the scan starts at
    the entry at `next`, if it's available, then the entries after it,
    wrapping to `dyn`. An available entry is used only if the set isn't
    full and is below WSQUOTA, or the free list has more than GROWLIM
    pages. Otherwise it's passed over, and the first entry in use that
    the scan reaches, the oldest, is reused (removed and returned). So
    a list that has wrapped reuses its oldest pages before it reaches
    available entries further on, as the book's steps do. Protected
    pages are always passed over. Pages valid in the TB are passed over
    up to TBSKIPWSL, and a second pass reuses the first unprotected
    page if the first pass skipped them all.
  - **Nothing replaceable** (finding 10): the page takes a new entry
    after `next`, the list grows past its limit, and the next `Add`
    first trims it back, oldest first; a `Remove` from such a list
    takes the entry out too.
  - **`Lock`/`Unlock`** move an entry between the parts by the book's
    exchanges (16.4.2.1, 16.4.2.3): a newly locked entry is exchanged
    with the one at `dyn`, an entry left with no lock with the last
    locked one. That reorders the FIFO a little, as VMS's does. Both
    return the locks the page had, from which subtask 6 makes WASSET
    and WASCLR. A lock in the working set fails with `ErrLockedSetFull`
    (SS$_LKWSETFUL) if it would leave fewer than MINWSCNT unlocked
    entries of the limit (finding 6, unconfirmed); a lock in memory
    has no such check.
  - **`Remove`** (deletion; locked pages too) and **`RemoveRange`**
    (`$PURGWS`, walking the list, finding 4, with or without the locked
    pages, oldest first).
  - **`Resize`** (`$ADJWSL`, image exit's reset) clamps to at most
    WSEXTENT and at least MINWSCNT more than the locked pages (the
    book's EXTDYNWS floor), removes the oldest unprotected pages over a
    smaller limit (no TB skipping), and then lays the dynamic part out
    again, oldest to newest from `dyn` with the available entries
    after. The re-layout is govax's choice (unconfirmed): VMS leaves
    holes and adds room at the list's end (16.4.1), where, by the
    scan's rule above, a wrapped list would use it only after reusing
    the pages before it. The probe's `$ADJWSL` patterns test it.
- **2026-10-16:** Subtask 3, the working set in corevms
  (`corevms/workingset.go`; tests in `internal/console`'s
  `workingset_test.go` and `workingsetglobal_test.go`).
  - **Where the list lives:** `Environment.WorkingSet()`, made at its
    first use with paging on from the process's WSLIMIT, WSQUOTA,
    WSEXTENT, and MINWSCNT (a new PFN database, from a new VMINIT,
    starts it afresh). Its size stays the one it was made with until
    subtask 4.
  - **Entering:** `System.PageIn` puts each P0/P1 page of a known
    process in its working set *before* resolving the fault, as VMS
    frees an entry before it takes a frame (so a clean page leaving can
    give its frame to the page coming in); a fault that then fails takes
    the page out again. A page-file global section's pages, valid from
    mapping, enter at `mapSectionPage` as global pages; a file global
    section's enter at their fault (a copy-on-reference section's
    private copy is a process page). `releasePTE` takes any page leaving
    its PTE out (deletion, not paging out).
  - **Replacement** (`evictPage`): the chosen page's PTE becomes
    `TransitionPTE` (protection and owner kept) through `StorePTEIn`,
    which empties its TB slot; the frame is released with the PTE's
    modify bit (`paging.Database.Release`). Only a frame tied to that
    page, with one reference, is touched. The conditions
    (`replacementConditions`): the free list's length; GROWLIM (a
    `Sysgen.GrowLimit` field, default 63, whose key is subtask 4's);
    TBSKIPWSL and `vm.Memory.ValidInTB` only for the process whose
    address space the CPU holds; and the protected pages
    (`wsProtected`): global pages (resident until Phase 65), pages whose
    frames were translated this instruction, and pages whose PTEs can't
    be read.
  - **Effectively modified:** a demand-zero page's frame and a
    copy-on-reference copy's get the saved modify bit at their fault
    (`effectivelyModified`), so they go to the modified list.
  - **Soft faults:** with paging on, `resolveFault` looks for a
    transition PTE first (`softFault`): the frame must still be tied to
    the faulting page, and `Reclaim` takes it off its list. The new PTE
    is valid and unmodified (the saved modify bit stays in the
    database). Soft faults count in JPI$_PAGEFLTS (the design notes);
    the split into soft and hard is subtask 8's.
  - **Ties:** the System is the database's `TieBreaker` (`BreakTie`,
    installed by `NewSystem`): when a free-list frame is reused, its old
    page, if its PTE is still in transition in that frame, gets its
    backing store's form back (`backingPTE`: section, global, page
    file, or demand-zero). In this phase only a private file section's
    clean pages reach the free list, so the form is the PSTX, and the
    next touch reads the file.
  - **The P1 vector** (Decision 13): `ShareP1` takes its pages out of
    the depositing process's working set and makes their frames untied
    system pages (`paging.Database.MakeSystem`, new).
  - **Growth past the limit** (finding 10, refined there): when `Add`
    leaves a list over its limit, `vm.Memory.AtNextInstruction` (new: a
    routine run once from `BeginInstruction`, with paging on only) runs
    `trimWorkingSets` at the next instruction boundary, which evicts
    what `WorkingSet.Trim` (new, with `Over`) takes out.
  - **`vax.sysgen.tbskipwsl`** (default 8; 0 is allowed, so it's read
    with `ParseSysgenNumber` and audited as a nonnegative number,
    `SysgenNumberKeys`), in `validConfigs` and HELP CONFIG KEYS.
  - **A fix found by running the whole suite with paging forced on and
    25- and 30-page working sets** (an experiment, not committed):
    `setImageProtection` changed only valid pages' protections, so a
    page of the last image that was in transition kept its read-only
    protection, and the next image's activation failed writing it
    (TestLib57Oracle). A page in transition counts as touched now
    (`ImageProcess.touched`). With that, every test passed at both
    sizes, also with the PFN database's and the working set's `Check`
    run after every 32nd fault.
  - **The tests:** an image with a 30-page working set sweeps an 80-page
    array and a 64-page private read-only file section, then spins.
    After the instruction limit stops it, the array's pages that left
    the set are in transition on the modified list and the section's on
    the free list; the array's come back in their own frames (soft
    faults); taking every free frame sends the section's PTEs back to
    their PSTX, and they read back from the file. With TBSKIPWSL 0 every
    touch faults (601 faults); with 8, the four sweeps of 34 pages find
    pages still there (537). Global pages stay in the set while 60 P0
    pages pass through it, and the P1 vector's frames are system pages.
  - **Left for later:** a page deleted while in transition
    (`releasePTE` returns early for an invalid PTE, and
    `TeardownAddressSpace` skips it) keeps its tied frame on its list,
    so a new page at that address makes two frames with one owner:
    subtask 5. A private section's modified page on the modified list
    isn't written by `$UPDSEC`, `$DELTVA`, or image rundown: subtask 7.
- **2026-10-16:** Subtask 4, the limits (`corevms/workingset.go`'s
  `setWorkingSetLimit` and `resetWorkingSet`; tests in
  `internal/console/workingsetlimit_test.go`, and corevms's
  `TestImageRundownResetsWorkingSet`).
  - **`$ADJWSL`** clamps the new limit to [MINWSCNT, WSEXTENT] as before,
    and with paging on resizes the list (`WorkingSet.Resize`, its bounds
    refreshed from the process first): a smaller limit takes the oldest
    pages over it out (`evictPage`, into transition), and the result is
    the list's size, which with pages locked in the working set may be
    above the one asked for (the list's floor is MINWSCNT more than the
    locked pages). Pages the calling instruction has translated can't
    leave, so a list left over its limit is trimmed at the next
    instruction boundary, as finding 10's growth is. `wsetlm` is checked
    for writing before anything changes, so an unwritable one still
    leaves the limit alone (SS$_ACCVIO, as before; unconfirmed which way
    VMS orders it). `pagcnt` 0 changes nothing.
  - **Image exit** (`ImageRundown`'s `resetWorkingSet`) sets the limit
    back to DFWSCNT through the same code, in both modes (finding 3).
    No oracle depended on a second image's WSSIZE: the whole suite
    passed unchanged. Process deletion runs image rundown too, so a
    process being deleted shrinks its list before its address space is
    torn down; the pages that leaves in transition are subtask 5's,
    with the others.
  - **WSAUTH and WSAUTHEXT** are `Process.WSAuth` and `WSAuthExt`
    (finding 2): set to the quota and extent in `NewProcess`,
    `applyQuotas` (after `$CREPRC`'s 4 pages, so a child's WSAUTH is its
    WSQUOTA, 1028 by default), and `LoginQuotas`, and never changed.
    `$GETJPI`'s JPI$_WSAUTH and WSAUTHEXT report them.
  - **`vax.sysgen.growlim`** (`Sysgen.GrowLimit`, default 63; 0 is
    allowed, a number key like TBSKIPWSL), in `validConfigs` and HELP
    CONFIG KEYS. WSQUOTA, WSEXTENT, and MINWSCNT were already the list's
    `Bounds` from subtask 3.
  - **The test:** an image with a 60-page working set touches 80 pages
    (at least 20 in transition), shrinks by 30 (`wsetlm` 30, the list 30
    long and not over it, at least 50 in transition), grows by 100 and
    sweeps again (130; none in transition), sweeps once more (no faults),
    and returns, after which the limit and the list are 60 again.
- **2026-10-16:** Subtask 5, deleting pages in transition
  (`corevms/workingset.go`'s `heldFrame`; tests in
  `internal/console/workingsetdelete_test.go`).
  - **One rule for every exit path:** `heldFrame` says which frame a
    page being deleted holds, by its PTE: a valid page's, or, with
    paging on, a transition page's, if that frame is on a list and still
    tied to the page (the process's PID and the page's address). Any
    other PTE holds none. `releasePTE` (which `$DELTVA`, `$CRETVA` over
    existing pages, `$CNTREG`, and `unmapSections` all reach) and
    `TeardownAddressSpace` use it, so a transition page's frame is
    released as a valid page's is: `FreePage`, which with paging on is
    `paging.Database.Delete`, taking the frame off its list to the free
    list's head, untied.
  - **The PID in the address space:** `TeardownAddressSpace` runs after
    the process has left the table, so `ProcessSpace` keeps the PID it
    was built for (`PID`). The tie check matters there: a free-list
    frame reused after the process left the table can't have its old
    PTE changed (`BreakTie` doesn't find the process), so the PTE still
    says transition while the frame is someone else's; that frame is
    left alone.
  - **The saved modify bit counts as modified** for a deleted page: a
    transition page is modified by its saved modify bit, and a valid
    page by its PTE's bit or, if its frame is tied to it, the saved one.
    A page of a writable private section written, taken out of the
    working set, and soft-faulted back by a read has a clean PTE and the
    bit only in the database; without this, image rundown's unmapping
    lost the write. So `$DELTVA` and image rundown now write back a
    section's pages on the modified list too; `$UPDSEC` doesn't yet
    (subtask 7).
  - **The tests:** an image with a 30-page working set touches an
    80-page array, recreates the first 40 pages with `$CRETVA` and
    deletes the other 40 with `$DELTVA`: each frame the pages held in
    transition is free and untied, and the next frame allocated is one
    of them (the free list's head). It then maps a 64-page file as a
    writable private section, adds 1000 to each page, reads the first 10
    again (soft faults, clean PTEs), and returns: every block of the
    file holds its new value. A second process, built in Go with a
    30-page working set, has 80 pages touched and is removed: no frame
    stays tied to it and the frame counts are as before it was made.
    Each test fails without the change, and the write-back one without
    the saved modify bit.
- **2026-10-16:** Subtask 6, the working set services
  (`corevms/pageprot.go`; tests in `internal/console`'s
  `pageprot_test.go`, paging off, and `workingsetservices_test.go`,
  paging on).
  - **The locks:** the process's maps (`memoryLocks`,
    `workingSetLocks`) still record the locks in both modes and decide
    the statuses, so the two modes answer alike. With paging on the
    working set list holds them too (`lockInWorkingSet`): a page not in
    the list is faulted in first (`TranslateIn` with kernel access, which
    goes through the pager; SS$_ACCVIO if it can't be), then locked with
    `WorkingSet.Lock`, which moves it out of the dynamic part. A valid
    page in no working set (the P1 vector's) is only recorded. An unlock
    takes the lock off in the list too, and image rundown's
    `cancelPageLocks` unlocks every page in it before the working set is
    reset, so the reset can shrink it.
  - **The statuses** (finding 5): locking is SS$_WASSET if any page was
    locked, unlocking SS$_WASSET only if every page was; SS$_WASCLR
    otherwise. Fixed in both modes; DEVIATIONS.md has the entry.
  - **SS$_LKWSETFUL** (finding 6, unconfirmed): with paging on,
    `WorkingSet.Lock`'s rule (fewer than MINWSCNT unlocked entries of the
    limit would be left); with paging off the same rule, counted from the
    maps (pages locked either way, once each). A page already locked in
    memory can be locked in the working set without the check, as in the
    list. `$LCKPAG` has no limit, and never returns SS$_LCKPAGFUL
    (finding 7). The service stops at the failing page; `retadr` holds
    the pages done before it.
  - **SS$_NOPRIV for a writable global page** (finding 8), in `$LKWSET`
    only, both modes (`writableGlobalPage`): a page `mapSectionPage`
    mapped, whose protection lets kernel mode write, and whose PTE is a
    global page table index of a section that isn't copy on reference,
    or a valid PTE whose frame is a section's (a copy-on-reference copy
    is the process's own). `$LCKPAG` locks such a page.
  - **`$PURGWS`** (new, both modes; finding 4): reads `inadr` (SS$_ACCVIO),
    refuses a range reaching S0 (SS$_NOPRIV, VSI's list, unconfirmed),
    and with paging on takes the range's pages out of the working set
    with `WorkingSet.RemoveRange` (which now takes `Conditions`), each
    into transition (`evictPage`). Locked pages stay (unconfirmed), and
    so do the pages replacement passes over: global pages (resident until
    Phase 65) and those the calling instruction translated (its code,
    its argument list, `inadr`), which the image would fault back at once
    anyway. In the test, a purge of everything left 8 pages: the 5 locked
    and 3 of the call's. With paging off it returns SS$_NORMAL. Its
    argument count is 1 (the manual's), and it's in govax's STARLET
    (`$PURGWS_S`, `$PURGWS`, `$PURGWS_G`, by the other services' rules,
    unchecked until the macro round). The other five services' macros
    were already there, checked against real MACRO (Part A's "What govax
    has today" was wrong about that).
  - **The tests:** with paging off, the statuses of mixed ranges (the
    unlock of a half-locked range is SS$_WASCLR), SS$_LKWSETFUL at
    MINWSCNT + 2 with a page locked in memory counted, a writable and a
    read-only global page, and `$PURGWS`'s statuses. With paging on, an
    image with a 30-page working set (MINWSCNT 20) locks 5 pages, sweeps
    80 others past them (the locked ones stay, the rest of their array
    goes into transition), purges all of P0 and P1 (the locked pages
    stay, the swept array leaves the set), unlocks (SS$_WASSET), locks
    purged pages one at a time (each faulted in; 10 succeed, the 11th is
    SS$_LKWSETFUL), locks that page in memory (SS$_WASCLR), and returns,
    after which no page is locked.
- **2026-10-16:** Subtask 7, the modified page writer for private file
  sections (`corevms/mpw.go`; tests in `internal/console`'s
  `workingsetwriter_test.go`, `paging`'s `TestModifiedFrames` and
  `TestClearModified`, `machine`'s `TestSysgenModifiedPageWriter`).
  - **A clean page's PTE returning to its PSTX** was already subtask 3's
    (`BreakTie`); a page the writer has written takes the same path, and
    the test reads every page back from the file after taking every free
    frame.
  - **The writer** (`wakeModifiedPageWriter`, `writeModifiedPages`):
    `evictPage` wakes it after each frame it sends to the modified list.
    Over MPW_HILIMIT it takes the list head first
    (`paging.Database.ModifiedFrames`, new) until the list holds
    MPW_LOLIMIT, and writes each frame it can to its block
    (`writableSectionPage`: a tied process page backed by a block of a
    private section that writes back, still mapped at that address), then
    moves it to the free list's tail (`Cleaned`), still tied, so a touch
    is a soft fault. Synchronous, inside the operation that crossed the
    threshold (Decision 7), and unclustered: the book's clustering
    (15.5.2.2) and MPW_WRTCLUSTER change only the I/O count, and wait for
    Phase 64.
  - **Pages it can't write** (demand-zero and copy-on-reference pages,
    whose place is the page file) stay on the list. So that a list full
    of them isn't walked at every eviction, the writer remembers how many
    it passed over (`System.mpwStuck`) and wakes again only when the list
    is over MPW_HILIMIT by more than that; the count is forgotten once the
    list is back to MPW_HILIMIT. govax's rule (VMS always has a page
    file), to revisit in Phase 64, where a full page file leaves pages
    stuck the same way.
  - **From inside a fault:** the writes go through the file's block I/O
    (`rms.ACPFile.WritePage`), as `releasePTE`'s did already; ods2 keeps
    no locks a write could wait on, so a fault in the middle of an RMS
    operation can run the writer. (Decision 9's audit for the page file
    is still Phase 64's.)
  - **`$UPDSEC`** (subtask 5's leftover): a private section's page out of
    the working set whose frame waits on the modified list is written
    from there and moved to the free list (`modifiedTransitionPage`); a
    valid page with a clean PTE but its modify bit saved in its frame (a
    soft fault off the modified list) is written too, and the saved bit
    cleared (`paging.Database.ClearModified`, new). A page whose frame is
    on the free list is clean, and isn't written.
  - **`vax.sysgen.mpw_hilimit`** (500) and **`vax.sysgen.mpw_lolimit`**
    (120), number keys (0 allowed: 0 and 0 write every page they can
    whenever one is modified), `Sysgen.MPWHiLimit` and `MPWLoLimit`, in
    `validConfigs` and HELP CONFIG KEYS. Nothing checks that the low limit
    is below the high one; if it isn't, a run writes nothing.
  - **The test:** an image with a 30-page working set and the thresholds
    at 10 and 2 writes 80 demand-zero pages (at least 50 stuck on the
    modified list), then adds 1000 to each of a 64-page writable private
    section's pages and reads 14 of them again. At most 10 of the
    section's pages wait on the modified list, the rest out of the set
    are on the free list with their blocks written, and at least one page
    is back with its modify bit saved. `$UPDSEC` then leaves none on the
    modified list or saved, and every block written; after every free
    frame is taken, the written pages' PTEs are their PSTX again, and the
    program reads every new value back. The test fails with the writer
    off, and with `$UPDSEC`'s two additions off.
- **2026-10-16:** Subtask 8, the reclaimer and the accounting
  (`corevms/reclaim.go`, `corevms/accounting.go`; tests in
  `internal/console/workingsetreclaim_test.go`, `paging`'s
  `TestWorkingSetShrink`, `vm`'s `TestSetFreeLimit`, `machine`'s
  `TestSysgenReclaimer`, corevms's `TestSYIPagingParameters`).
  - **When it wakes:** `vm.Memory.SetFreeLimit` (new) calls a routine
    whenever an allocation leaves the free list below a limit; the System
    installs `wakeReclaimer` there with FREELIM (`installReclaimer`, from
    `NewSystem` and `SetSysgen`). A soft fault off the free list wakes it
    too (`wakeReclaimerIfLow`); a soft fault off the modified list
    doesn't, since the free list is no shorter. It runs at the next
    instruction boundary, not inside the fault: `atInstructionBoundary`
    (new) is the one routine `AtNextInstruction` keeps, running
    `trimWorkingSets` and then `reclaim`, so the two don't replace each
    other.
  - **What it does** (the book's 17.1.1 and 17.2.2, Decision 7): if the
    free list is still below FREELIM, the modified page writer's run down
    to MPW_LOLIMIT (the book's MFYLOLIM; no MPW_THRESH), then the first
    pass of the swapper's selection table: each process whose working set
    holds more than its WSQUOTA loses its oldest pages down to it
    (`WorkingSet.Shrink`, new: locked and protected pages stay, TB-valid
    pages don't count as a reason to keep one), until the free list
    reaches FREEGOAL; then the writer again, for the pages trimming sent
    to the modified list. Outswapping, process headers, and the
    SWPOUTPGCNT pass need a swapper govax doesn't have.
  - **Choices (unconfirmed):** the processes' order is the book's Table
    17-2 by scheduling state alone (SUSP, LEF, HIB, CEF, FPG, COLPG,
    MWAIT, PFW, then the computable ones, the current process last), by
    PID within a state (the book scans each state's queue from its newest
    entry, which `internal/sched` doesn't expose); trimming leaves the
    limit (WSSIZE) alone, so a trimmed set grows back past its quota only
    while the free list has more than GROWLIM pages; the reclaimer's
    writer remembers the pages it couldn't write (`reclaimStuck`) and
    runs only when the modified list is over MPW_LOLIMIT by more than
    that, forgetting the count when the free list is back to FREELIM (as
    the writer's own `mpwStuck`). A set trimmed to its quota can take its
    own pages back off the modified list without waking the reclaimer, so
    it may be a few pages over its quota until its next allocation (the
    test allows 4).
  - **`vax.sysgen.freelim`** (32; 0 turns the reclaimer off) and
    **`vax.sysgen.freegoal`** (200), number keys (`Sysgen.FreeLimit`,
    `FreeGoal`), in `validConfigs` and HELP CONFIG KEYS.
  - **Faults by kind** (`FaultCounts`): soft off the free list, soft off
    the modified list, global valid (a global section page already in
    memory, which `pageInGlobal` now tells from a read), demand-zero, and
    read (hard). `resolveFault` returns the kind, and `countFault` counts
    it in the process (`Process.Faults`, beside `PageFaults`, which stays
    JPI$_PAGEFLTS) and the system (`System.PageCounts`, with the pages the
    writer wrote and the reclaimer's runs and trimmed pages). SHOW MEMORY
    shows the system's counts in paging mode, as govax's own lines (VMS
    shows rates in MONITOR PAGE).
  - **Pages in the working set** (`residentPages`): with paging on the
    working set's counts (PPGCNT, GPGCNT, its peak); with it off, the
    valid P0 and P1 PTEs, global if the frame is a global section's
    (`Sections.byFrame`). `Process.WSPeak` is the most any count has
    seen; with paging off that's a lower bound, counted when something
    asks and at the start of image rundown, before the image's pages go.
  - **Reported:** `$GETJPI`'s JPI$_PPGCNT, GPGCNT, WSPEAK (and F$GETJPI's
    keywords); SHOW SYSTEM's page faults (JPI$_PAGEFLTS) and pages
    (PPGCNT + GPGCNT; `mappedPages` is gone); LOGOUT's page faults and
    peak working set; the termination message's ACC$L_WSPEAK. `$GETSYI`
    (and F$GETSYI) has GROWLIM, FREELIM, FREEGOAL, TBSKIPWSL,
    MPW_HILIMIT, MPW_LOLIMIT, and the PQL_DWSDEFAULT, DWSQUOTA,
    DWSEXTENT, and DPGFLQUOTA parameters (`sysgenPQL`, the defaults
    `$CREPRC` gives). BORROWLIM, WSMAX, PFCDEFAULT, and MPW_WRTCLUSTER
    have no value in govax and stay SS$_BADPARAM.
  - **The test:** an image with a 30-page default, a 40-page quota, and
    `$ADJWSL` to 100, FREELIM and FREEGOAL above any free list, and
    MPW_LOLIMIT 0, touches 80 `$EXPREG` pages (at most 44 in the set, the
    rest in transition on the modified list), adds 1000 to each page of
    a 64-page writable private section (every page out of the set on the
    free list, its block written), sweeps the array again (soft faults
    off the modified list), and reads PPGCNT, GPGCNT, WSPEAK, and
    PAGEFLTS with `$GETJPIW` (the working set's counts). With FREELIM 0
    the whole array stays in the set and nothing is written. It fails
    without the trimming, and without the reclaimer's writes. SHOW
    SYSTEM's and LOGOUT's tests now check the counts in place of 0.
  - **An experiment** (not committed): the whole suite with paging
    forced on and FREELIM and FREEGOAL at 2^30, so the reclaimer trims
    every set to its quota after every allocation. Only five tests
    failed, each for the forced settings: two paging-off tests, two of
    the settings' defaults, and `TestModifiedPageWriter`, whose section
    pages the reclaimer had written before they came back.
- **2026-10-16:** Subtask 9, SET WORKING_SET and SHOW WORKING_SET
  (`corevms/setworkingset.go`, `dclcmd/workingset.go`; the grammar's
  `set_working_set` and `show_working_set`, bound in
  `console/dispatch.go`; HELP SET WORKING_SET and SHOW WORKING_SET; tests
  in `internal/console/setworkingset_test.go`).
  - **Where:** the commands are the console's utility commands
    (`dclcmd.Context`), on `Context.RTL`, the process whose DCL runs the
    line, so a subprocess's SET and SHOW are its own (a SPAWNed
    subprocess's default and quota are its creator's current ones and 4
    pages more, `applyQuotas`, so its authorized quota is that too).
  - **The rules** (`Environment.SetWorkingSet`, the Dictionary's and the
    book's 16.4.1.2): /EXTENT at most WSAUTHEXT; /QUOTA at most WSAUTH
    and the extent; /LIMIT (DFWSCNT) at most WSAUTHEXT, a limit above the
    quota raising the quota (as far as WSAUTH), and then at most the
    quota; a smaller extent or quota bringing the values under it down.
    Unconfirmed choices, for the probe: an /EXTENT at or below MINWSCNT
    is raised to MINWSCNT + 1 (the Dictionary says only that it must be
    greater; VMS may refuse it with a message instead); /QUOTA and /LIMIT
    are at least MINWSCNT; a /LIMIT above WSAUTH leaves the limit at the
    quota.
  - **The current limit** (WSLIMIT, JPI$_WSSIZE) is set to the new
    default at once, through `setWorkingSetLimit` (`$ADJWSL`'s and image
    exit's code): DCL runs between images, when the limit is the default
    anyway. With paging on, a smaller list gives up its oldest pages.
    Unconfirmed: VMS's DCL is an image of the process, and its WSSIZE
    after SET is the probe's.
  - **/NOADJUST** is `Process.WSAdjustOff` (PCB$V_DISAWS), kept and shown
    as "Adjustment disabled"; govax has no automatic adjustment.
  - **The message:** `%SET-I-NEWLIMS, new working set: Limit = n Quota =
    n Extent = n`, printed with /LOG, or when a value asked for was cut
    back to its bound (unconfirmed: both of the Dictionary's examples
    show it without /LOG, one of them for a value within bounds; the
    probe settles when VMS shows it).
  - **SHOW's layout** is the Dictionary's VAX example's (`/Limit=%4d
    /Quota=%4d`, eleven blanks, `/Extent=%4d`; `Adjustment enabled`,
    four blanks, `Authorized Quota=%4d Authorized Extent=%4d`), whose
    spacing the PDF doesn't pin down: the probe captures the real one.
    SHOW's /OUTPUT isn't implemented (no other SHOW in govax has it).
  - **The tests:** each qualifier within and beyond its bound, /LOG,
    /NOADJUST, F$GETJPI's DFWSCNT, WSSIZE, and WSQUOTA after, a SPAWNed
    subprocess's own values, unchanged in process 1 after it; with paging
    on, the list's size following SET's /LIMIT and /EXTENT.
