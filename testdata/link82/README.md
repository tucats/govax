# The Phase 82 probe: one program section, different attributes

`docs/PHASE-82 - FORTRAN DEC extensions.md`'s subtask 7 left a gap: two
separately compiled files can give one common block different CDEC$ PSECT
attributes, and govax's LINK says nothing. The Linker manual (VMS 5.0, 6.2.1.1
and 6.3) says only that sections of one name "normally must have the same
attributes" and that LINK "checks for conflicting attributes". This probe asks
real LINK what that means: which differences it reports, in what words, at what
severity, whether it still makes an image, and whose attributes the section
ends up with. There is no VAX FORTRAN to run, so the modules are MACRO.

`gen.go` writes everything here but this README and `vax/`:

    go run testdata/link82/gen.go

| File | What it holds |
| ---- | ------------- |
| `base.mar`, `base2.mar` | Section BLK with FORTRAN's attributes for a common block, 16 and 32 bytes; only BASE defines `BLKSTART` |
| `lbase.mar` | Section LOC with FORTRAN's attributes for `$LOCAL`; defines `LOCSTART` |
| `v*.mar`, `l*.mar` | BLK or LOC with one attribute changed, 32 bytes (`vmulti.mar` three; `va4.mar` is alignment `4`, the form the compiler writes for ALIGN=4, which real MACRO must also take) |
| `mainp.mar`, `mainl.mar`, `mainn.mar` | The transfer address: a store to `BLKSTART` or `LOCSTART`, or none |
| `*.opt` | The PSECT_ATTR=, COLLECT=, and CLUSTER= cases |
| `link82.com` | The cases, in order |
| `link82a.com` | Assembles every module, with MACRO's output in `LINK82.MLG` |
| `link82c.com` | One case: LINK with a full map, `$STATUS`, and, if there is an image, RUN and `$STATUS` |
| `exchange.cmd`, `copyout.cmd` | Build the exchange volume, on DUA2 since the startup procedure may mount DUA1, and copy the results off |

## What each case shows

A case in `LINK82.LOG` starts `=== Cnn: what it asks`, followed by LINK's messages
and `$STATUS`. `Cnn.MAP` gives the section's final attributes, each module's
contribution to it (base and length, so an overlay shows from a concatenation, and
one section from two), and the image sections. RUN's `$STATUS` says whether the
store worked, so whether a WRT and NOWRT conflict left the block writable: 1 is
yes, and an access violation is no.

| Cases | What they ask |
| ----- | ------------- |
| C01 | The same attributes, a control: no message |
| C02 to C33 | BASE and each BLK variant, in both orders: which attributes conflict, and does the first or the second win? |
| C34 to C43 | The same for LOC, where the base is CON and LCL |
| C44 to C47 | Three and four modules: one message or several, compared with what |
| C48 to C52 | PSECT_ATTR= and COLLECT=, which the manual says change a module's attributes before the check |
| C53 to C57 | Clusters: GBL and LCL apart (a control among them), and a conflict across clusters |

## Running it

1. Build the volume, from the repository root:

       govax console < testdata/link82/exchange.cmd

   This makes `testdata/disks/link82-exchange.dsk` (RD54 size, label LINK82,
   gitignored). It needs `SET RQ2 RD54` at the `sim>` prompt before the attach.
2. Attach it to the simh VAX as a spare drive, DUA2 for example, log in as SYSTEM,
   and:

       $ MOUNT DUA2: LINK82
       $ SET DEFAULT DUA2:[000000]
       $ @LINK82/OUTPUT=LINK82.LOG
       $ SET DEFAULT SYS$LOGIN
       $ DISMOUNT DUA2:

3. Copy the results back, from the repository root:

       govax console < testdata/link82/copyout.cmd

No licensed file goes on the volume, so it needs no erasing. The modules use no
system macros, and MACRO has no `/LIST` here, so its log, `vax/link82.mlg`, holds at
most its own messages about these few lines. Audit it as usual before it's read.
