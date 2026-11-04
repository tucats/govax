# The Phase 88 probe, round 2: FORRTL on indexed files

These fixtures are `docs/PHASE-88 - keyed index files.md`'s round 2, the start of subtask 12.
The simh VAX has no FORTRAN compiler, so VMS's own FORRTL.EXE is called from MACRO, and what it
does becomes FTNRTL's requirement (only what it does is used, never its code). The form of the
program and its output are `testdata/for81`'s, whose README explains them, and the records are
our own data.

`gen.go` writes everything here but this README, `for88f.mar`, and the output files:

    go run testdata/for88/gen.go

| File | What it holds |
| ---- | ------------- |
| `for88.mar` | 310 cases: KEY='s block, OPEN's keywords around it, an existing file, every match on every key, a keyed READ's arguments and failures, sequential READs after keyed ones, WRITE, REWRITE, DELETE, UNLOCK, positioning |
| `for88.com`, `for88b.com` | Build and run it, then DIRECTORY, ANALYZE/RMS_FILE, and a save set of the files FORRTL made |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |
| `for88f.mar` | By hand: the `FOR$` names `for88.mar` calls, passed on to FTNRTL's `FTN$` routines, so `TestFor88OnFTNRTL` runs the cases on FTNRTL |
| `vax/` | VMS's run: `for88-vax.txt`, `for88.log`, `for88.bck` |

## FORRTL's interface, found by calling it over govax (2026-11-04)

No disassembly was involved: a MACRO program called `FOR$OPEN` and `FOR$READ_KU` with thousands of
shapes, watching the status, what a USEROPEN routine was given, and the RAB.

**`FOR$OPEN`'s KEY=, code 23 by mechanism 4,** is the address of a block: a word no one reads, a word
counting the longwords after it (a multiple of 3; 0 is no key at all), and three longwords per key:

| Longword | Holds |
| -------- | ----- |
| 1 | The type: 0 or 14 a string, 7 or 8 a signed integer (of the key's length, 2 or 4), 3 or 4 an unsigned one; 512 added for descending. Others are FOR$_INVARGFOR |
| 2 | The first byte, from 1 |
| 3 | The last byte |

FORRTL's XABKEY has the key's DTP, POS0, SIZ0, and REF, and FLG 0 for the primary key and DUP and CHG
for an alternate; every other field is 0, for RMS's defaults.

**`FOR$READ_KU(unit, key, keyid, match)`** (and `FOR$READ_KO(unit, format, key, keyid, match)`): the unit
by value, the key's descriptor's address, the key's number by value (-1, or left out, the unit's last),
and the match by value (left out, 0):

| Match | Specifier | ROP | Direction |
| ----- | --------- | --- | --------- |
| 0 | KEY=, KEYEQ= | none | either |
| 1 | KEYGE= | KGE | ascending, else FOR$_INVMATKEY |
| 2 | KEYGT= | KGT | ascending, else FOR$_INVMATKEY |
| 3 | KEYNXT= | KGE | either |
| 4 | KEYNXTNE= | KGT | either |
| 5 | KEYLE= | KGE | descending, else FOR$_INVMATKEY |
| 6 | KEYLT= | KGT | descending, else FOR$_INVMATKEY |
| 7 and up, negative | | | FOR$_INVARGFOR |

VMS 7.1's run confirmed these on its own RMS (2026-11-04): `vax/`, and the phase doc's "What round 2 settled".

## The VAX run

Allow several minutes: MACRO has 25,000 lines to read. If a case hangs, Ctrl/Y and tell me which `#n`
`FOR88-VAX.TXT` reached; the likeliest is "two units on one file", a record locked by another unit.

1. Build the exchange volume, from the repository root:

       govax console < testdata/for88/exchange.cmd

   This makes `testdata/disks/for88-exchange.dsk` (RD54 size, label FOR88).

2. On the VAX, mount it as DUA5, and run the procedure from its top directory:

       $ MOUNT DUA5: FOR88
       $ SET DEFAULT DUA5:[000000]
       $ @FOR88/OUTPUT=FOR88.LOG
       $ DISMOUNT DUA5:

3. Back on the host:

       govax console < testdata/for88/copyout.cmd

   which brings back `vax/for88-vax.txt`, `vax/for88.log`, and `vax/for88.bck`.

`TestFor88Oracle` (`internal/console/for88_test.go`) runs the program on VMS's FORRTL.EXE over
govax's RMS, holding its output and files to VMS's, or skips without one on a local volume. Set
`FOR88_WRITE` to a file to keep its output. `TestFor88OnFTNRTL` (`for88ftn_test.go`) needs no
FORRTL: it links the program with `for88f.mar`, and holds FTNRTL to `#0` to `#148` but `#92`,
on `STD.IDX` as VMS made it.
