# The Phase 68 probe, round 2: VMS runs govax's images

Subtask 13 of `docs/PHASE-68 - link shareable images.md`. Round 1
(`testdata/link68`) had VMS make the probe's objects, images, maps, and
library, and `TestLink68Oracle` holds govax's run of the same
`LINK68.COM` to them. This round goes the other way: govax runs
`LINK68.COM`, and VMS analyzes, runs, links against, and librarian-lists
what govax made. It asks:

- Does VMS's ANALYZE/IMAGE find 0 errors in each of govax's images? A
  shareable image's file ends at a block's end in govax and at the global
  symbol table's end in VMS (Phase 30's Decision 7).
- Do govax's programs run on VMS against govax's shareable images, with
  VMS's output (round 1's), including the ident mismatch MAIN1A meets
  and `RUN SHR1.EXE`'s NOTFR? Each names LIBRTL with the ident govax's
  tables give it, which VMS 7.1's LIBRTL must match.
- Does VMS's LINK read govax's shareable images' global symbol tables
  (MAIN1V against SHR1, MAIN2V against SHR2, which uses SHR1), and its
  librarian govax's images (MYSHRV.OLB) and govax's library (MAIN1W
  through MYSHR.OLB)?

| File | What it is |
| ---- | ---------- |
| `exchange.cmd` | The govax console script that makes the exchange volume and runs `LINK68.COM` on it under govax |
| `run2.com` | The procedure VMS runs |
| `copyout.cmd` | The govax console script that copies the results into `../vax/round2/` |

## Running it

1. From the repository root, build the exchange volume (govax's own run
   of LINK68.COM, with its log in `LINK68.LOG` on the volume):

       govax console < testdata/link68/round2/exchange.cmd

   This makes `testdata/disks/link68-round2.dsk` (gitignored), an RD51
   labeled LINK68.
2. On the VAX, mount it and run the procedure with its `[000000]` as the
   default directory:

       $ MOUNT DUA1: LINK68
       $ SET DEFAULT DUA1:[000000]
       $ @RUN2/OUTPUT=RUN2.LOG

3. Back on the host, copy the results into `testdata/link68/vax/round2/`:

       govax console < testdata/link68/round2/copyout.cmd

Nothing here is DIGITAL's: the images are govax's, linked from this
probe's own sources, and VMS's links of them use only those images and
VMS's own LIBRTL by name.
