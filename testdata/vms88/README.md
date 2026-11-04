# Phase 88's VMS round

`docs/PHASE-88 - keyed index files.md`'s subtask 15. Until now VMS has judged only files it made
itself. Here its ANALYZE and RMS get the files govax made, and govax's programs run under VMS's RMS,
FTNRTL built there from its sources. Everything on the volume is our own data.

| File | What it is |
| --- | --- |
| `exchange.cmd` | Makes the volume: runs `testdata/idx88`'s program in `[G88]`, `testdata/for88`'s on FTNRTL in `[F88]`, and `testdata/rms88`'s in `[R88]`, so every `.IDX` there is govax's; `[P88]` and `[FTN]` are sources for VMS to build; `[ODD]` is names with no type and no name |
| `vms88.com` | Runs on VMS. Each file: ANALYZE/RMS_FILE, `/CHECK`, `/FDL`; every key walked; a copy changed by RMS, walked, and checked again. Then `vms88l.mar`, FTNRTL, `testdata/rms88`'s programs, `[ODD]`, and two save sets |
| `vms88w.mar` | Walks every key of a named file: how many records, a sum that ignores their order, and one that doesn't |
| `vms88m.mar` | Changes a named file with no change to its records: each third deleted and put back, each fifth updated |
| `vms88l.mar` | What the phase doc left open: where a keyed `$FIND` leaves the stream, what `$PUT` in sequence remembers, `$TRUNCATE`, a value's last SIDR pointers, a stream asking again for a record it holds, and two streams' locks on each organization by what each may do |
| `copyout.cmd` | Copies the results to `vax/` |

`TestVMS88Procedure` rehearses all of it on govax.

## Running it

1. From the repository root, the volume (`testdata/disks/vms88-exchange.dsk`, label VMS88, RD54, 3-block clusters):

       go build -o govax ./cmd/govax
       ./govax console < testdata/vms88/exchange.cmd

2. On VMS, with the disk attached to simh as DUA1 (no privileges needed but `ANALYZE/DISK_STRUCTURE`'s, the last command):

       $ MOUNT DUA1: VMS88
       $ SET DEFAULT DUA1:[000000]
       $ @VMS88/OUTPUT=VMS88.LOG

   Nothing waits for input, and the lock cases use no ROP WAT, so nothing should hang. It should take some minutes.

3. Dismount it or pause simh, then:

       mkdir -p testdata/vms88/vax
       ./govax console < testdata/vms88/copyout.cmd

4. And `testdata/ftn86`'s round again, which now has FTN_KEYED, `parts.for`, and `shared.for`: rename `testdata/disks/ftn86-exchange.dsk` and `ftn86d.log` first, then its README's steps. Its `vax/` is what `TestFTN86Oracle` reads, so keep the old run (`vax/round3/`, by that README's custom) when the new one comes back.

## What each answer is for

- `results.bck`: VMS's reports. For `[G88]`, VMS's own on VMS's own files are `testdata/idx88/vax/anl`, so the two can be compared.
- `vms88.log`: for each file, whether VMS's walk is govax's on the same bytes, and whether its changes went in. `==== PHONES` and the sections after it say whether testdata/rms88's expected files are right.
- `changed.bck`: the files VMS's RMS changed, for govax to read back, and to compare with what govax's RMS makes of the same changes.
