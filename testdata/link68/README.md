# The Phase 68 probe: shareable images

These fixtures are subtask 1 of `docs/PHASE-68 - link shareable
images.md`: a VMS run that shows what real MACRO and LINK write for
shareable images, and what the image activator does with them. The
doc's "The probe's cases" lists the questions. One command procedure,
`LINK68.COM`, assembles the sources, links the images with full maps,
runs ANALYZE/IMAGE on each image as it's made, runs the programs, and
tries the links that should fail.

| File | What it holds |
| ---- | ------------- |
| `xfr1.mar` | SHR1's transfer vector: `.TRANSFER` with `.MASK` (one mask ORed with more registers), a JSB entry, an entry defined later in the module and reached by `BRW`, and room for more |
| `shr1.mar` | SHR1's routines (BASE, ADD, JSB, SHOW), universal data (`SHR1_COUNT`, `SHR1_TABLE`, `SHR1_ORIGIN`), a self-relative `.ADDRESS`, `.ASCID`s, a G^ call to `LIB$PUT_OUTPUT`, and 8 pages nothing is stored in |
| `shr2.mar` | SHR2, which uses SHR1 through G^ (a call and a data read) and `.ADDRESS` |
| `shr3.mar` | NOPIC code with a `.ADDRESS` of its own data, linked based (SHR3) and not (SHR3P) |
| `shr6.mar` | A `.ADDRESS` in a shared, read-only psect |
| `main1.mar` | Calls SHR1 (CALLS and JSB through G^), reads its data through G^, and holds `.ADDRESS`es of a SHR1 routine and its data; linked against SHR1, SHR1A, SHR4, and through a shareable image library |
| `main2.mar`, `main3.mar`, `main6.mar` | Use SHR2, SHR3 (and SHR3P), and SHR6 |
| `exe1.mar`, `exe1b.mar` | An executable laid out by `BASE=`, `CLUSTER=`, `COLLECT=`, `PSECT_ATTR=`, `IOSEGMENT=`, `DZRO_MIN=`, `ISD_MAX=` |
| `shr1.opt` | SHR1's options: the transfer vector's cluster first, the library, `UNIVERSAL=` |
| `link68.com` | The procedure (below) |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back into `vax/` |

## How the programs report

The programs print with `SYS$FAO` and `LIB$PUT_OUTPUT`, through a `SAY`
macro of their own. They use no STARLET macros, so the listings hold
only this probe's code. MAIN1's addresses in SHR1 show as offsets from
`SHR1_ORIGIN`, the first byte of SHR1's code, which `SHR1_BASE`
returns. Its first line shows that origin and MAIN1's entry, absolute,
which is where the activator put the two images. MAIN3 and EXE1 show
absolute addresses, since their layouts are fixed at link time.

## What the procedure does

Every link is followed by `SHOW SYMBOL $STATUS`, and every image it
makes by `ANALYZE/IMAGE` into `NAME.ANI`. Object files are always named
with their type. That way a logical name defined for a shareable image
(`SHR1`, ...) never captures its object file.

| Step | What it asks |
| ---- | ------------ |
| MACRO/LIST | Real MACRO's objects and listings, `XFR1.OBJ`'s `.TRANSFER` above all (and `XFR1.ANL`, its ANALYZE/OBJECT) |
| SHR1 | `LINK/SHAREABLE/MAP/FULL/SYMBOL_TABLE` from two options files (`SHR1.OPT`, and `GSM.OPT` with `GSMATCH=LEQUAL,1,1000`): the header, the ISDs, the GST, `.TRANSFER`'s two values, the fixup section, the map, `SHR1.STB` |
| MAIN1 | Linked against SHR1 from `SYS$INPUT` options |
| R1 to R4 | MAIN1 run with SHR1 found by a full logical name; a logical name without device or type; no logical name and no `SYS$SHARE:SHR1.EXE` (the messages); `SYS$SHARE` a process search list with SHR1.EXE in its first directory |
| R5 | SHR1 relinked with GSMATCH LEQUAL 1,1001, 1,999, and 2,1000; EQUAL 1,1000 and 1,1001; NEVER 1,1000; ALWAYS 3,3; and LEQUAL 1,1000 again, MAIN1 run after each. MAIN1N, linked against the NEVER image, is run against it and against ALWAYS. Each relink's analysis is `SHR1_n.ANI`, n from 1 to 8 |
| R6 | SHR1A: `UNIVERSAL=` only, no GSMATCH, `/NOTRACEBACK`; MAIN1A runs, then SHR1A is relinked unchanged two seconds later (the time-based default ident) and MAIN1A runs again |
| R7 | SHR2 uses SHR1 (`GSMATCH=EQUAL,2,5`); MAIN2 uses SHR2 |
| R8 | SHR3 (`BASE=%X300000`) and SHR3P (not based), MAIN3 and MAIN3P |
| R9 | SHR4: SHR1's objects with `COLLECT=`, `PSECT_ATTR=`, `DZRO_MIN=1`, `ISD_MAX=20`, `GSMATCH=ALWAYS,0,0`; MAIN4 run before and after SHR4 is relinked with ALWAYS,7,7 |
| SHR5 | `PROTECT=YES` around one cluster (linked and analyzed, not run) |
| R10 | SHR6 and MAIN6: does the activator fix up a shared, read-only page? |
| R11 | EXE1, laid out by options |
| R12 | `LIBRARY/CREATE/SHARE MYSHR` of SHR1 and SHR2, `LIBRARY/LIST/FULL/NAMES` (`MYSHR.LIS`); MAIN1L through `MYSHR/LIBRARY`, MAIN1U through `LNK$LIBRARY`, and E9 with `/NOUSERLIBRARY` |
| E1 to E8 | Links that should fail: `UNIVERSAL=` of an undefined symbol; a transfer vector in an executable; a shareable image named on the command line; GSMATCH's bad keyword and a major ident past 255; an executable based on top of SHR3; GSMATCH= twice; a misspelled option; `RUN SHR1.EXE`; `UNIVERSAL=` in an executable |

At the end, `FILES.LST` is a `DIRECTORY/FULL` of everything, and a
`DIRECTORY/SIZE/DATE` of the images goes into the log.

## Running it

1. From the repository root, build the exchange volume:

       govax console < testdata/link68/exchange.cmd

   This makes `testdata/disks/link68-exchange.dsk` (gitignored), an RD51
   labeled LINK68.
2. On the VAX, mount it and run the procedure with its `[000000]` as the
   default directory:

       $ MOUNT DUA1: LINK68
       $ SET DEFAULT DUA1:[000000]
       $ @LINK68/OUTPUT=LINK68.LOG

   The procedure creates `[SHRDIR]` on the volume (R4) and defines only
   process logical names, which it deassigns at the end.
3. Back on the host, copy the results into `vax/`:

       govax console < testdata/link68/copyout.cmd

   The error links that made no image report a missing file there,
   which is expected. Images linked more than once come back as their
   first version (`SHR1.EXE;1`, `SHR1A.EXE;1`, `SHR4.EXE;1`); SHR1A's
   and SHR4's relinks come back too, as `shr1a-2.exe` and `shr4-2.exe`.

Nothing here is DIGITAL's: the images are linked from this probe's own
objects, and the shareable image library holds only their symbol
tables.
