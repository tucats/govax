# VMS MACRO's TIR records: a probe

Phase 111's VMS round (`testdata/f90r`, round 2) brought back the objects VMS 7.1's MACRO made of
fcalc's compiled `.MAR` files. govax's MACRO makes 14 of the 16 the same, record for record
(`TestF90RObjects`, after the repetition fix below). The other two differ where VMS stored one
byte of a run of immediate data through the stack (`STA_UB 0`, `STO_B`), not in the run's
`STO_IMM` commands as govax stores it:

| Object | Where | Immediate bytes before it in the run |
| --- | --- | --- |
| FCFUNCS | `$PDATA+^X216`, line 227, in BUILTIN_INDEX's table | 534 |
| FCFUNCS | `$PDATA+^X666`, line 335, in ROW's copy of the table | 498 |
| FCMATRIX | `$PDATA+^X551`, line 356, in APPLY_MATRIX's table | 321 |

FCMATRIX has the same table at `$PDATA` 0 and ^X208 with no such byte, so it isn't the text alone.
This probe varies one thing at a time, a module each, so each object's record packing starts
afresh. `gen.go` writes the modules from fcalc's `.MAR` files in `testdata/f90r/vax/fcalc`.

It also asks two smaller questions. A repetition factor (`.LONG 0[2]`) is `STA_UB 0`, `STA_UB 2`,
`STO_RL` in VMS's object (govax's MACRO now writes that); TIRR checks `.BYTE` and `.WORD`, a count
of 1, wider values, and symbols. And VMS's MACRO refuses `.ASCII` with nothing after it
(DIRSYNX); TIRX checks `.ASCIZ`, `.ASCIC`, and `.ASCID` with nothing, and with `//`.

## The modules

See `modules.md` (written by `gen.go`).

## Running it

1. Build the volume, from the repository root (`testdata/disks/tir111-exchange.dsk`, label TIR111):

       go build -o govax ./cmd/govax
       ./govax --profile tbprobe --system-file none console < testdata/tir111/exchange.cmd

2. On VMS, with the disk attached to simh as DUA1 (no privileges needed):

       $ MOUNT DUA1: TIR111
       $ SET DEFAULT DUA1:[000000]
       $ @TIR111/OUTPUT=TIR111.LOG
       $ DISMOUNT DUA1:

   TIRX's errors are on purpose.

3. Copy the results back:

       mkdir -p testdata/tir111/vax
       ./govax --profile tbprobe --system-file none console < testdata/tir111/copyout.cmd

   If VMS's MACRO wrote no TIRX.OBJ, its COPY says so; that is an answer too.
