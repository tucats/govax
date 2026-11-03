# The Phase 88 probe: RMS's indexed files

These fixtures are `docs/PHASE-88 - keyed index files.md`'s subtask 2. An RMS program in
MACRO makes indexed files of every kind on VMS and uses them every way, and VMS's statuses,
records, and blocks become the requirement for `internal/isam` and `internal/rms`. The
records are our own data, made from a number, so the files can be kept.

`gen.go` writes everything here but this README, `decode.go`, and the output files:

    go run testdata/idx88/gen.go

| File | What it holds |
| ---- | ------------- |
| `idx88.mar` | RMS, 154 cases: key shapes, areas, `$CREATE`'s refusals, growth a step at a time, finding, sequence, changing, locks |
| `idx88.com` | Builds and runs it; then ANALYZE/RMS_FILE's report, `/CHECK`, and `/FDL` on every file, three scripted interactive walks, and a save set of the files |
| `idx88w.dat` | The commands of ANALYZE/RMS_FILE/INTERACTIVE's walks, whose stops print VMS's names for each structure's fields |
| `def_bkt.mar`, `def_irc.mar`, `def_key.mar`, `def_area.mar`, `def_plg.mar` | One internal definition macro each, `GLOBAL`: its values from the object, no listing (the author's answer 5) |
| `decode.go` | Turns `vax/def_*.obj` into `defined.txt`, names and values only |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |

## Running it

On the host, from the repository root:

    go run testdata/idx88/gen.go
    govax console < testdata/idx88/exchange.cmd

That makes `testdata/disks/idx88-exchange.dsk`, mounted there as DUA5: the author's startup
procedure has XFER on DUA2, so a script's `MOUNT DUA2` would fail and its COPYs land on XFER.
Attach it to the VAX, mount it, and with its
`[000000]` the default directory:

    $ @IDX88/OUTPUT=IDX88.LOG

The scattered and three-level cases write about 4,000 records, so it may take a few minutes.
Nothing here waits on a lock: the lock cases are last, and use no ROP WAT, so nothing should
hang. If one does, Ctrl/Y and `IDX88-VAX.TXT` so far are still useful.

Back on the host:

    mkdir -p testdata/idx88/vax/anl
    govax console < testdata/idx88/copyout.cmd
    go run testdata/idx88/decode.go

**`DEFS*.LOG` and `IDX88M.LOG`, MACRO's own output, stay on the volume** until the author has
read them, as the clean-room notes ask: an error there could quote a macro. SYS$ERROR is NL:
for those commands, so `IDX88.LOG` can't hold such a line. With `MACRO/NOLIST` they should be
empty but for an error message, if a macro isn't in `LIB.MLB` under that name; then that one
object is missing, and `decode.go` goes on without it.

## How the program works

A case is a routine of its own under a handler that prints a condition and unwinds to the
driver, so a fault costs that case. The driver closes every file after each case.

    #n  title
    =WHAT r=x            a service, and its R0
     fab ... / rab ...   the control block after it
     got n= rfa= |text|  a record read: its size, RFA, and first 60 bytes
     sum / key / area    what $OPEN and $DISPLAY say of the file's keys and areas
     vbn n               a file's blocks, by block I/O
       ooo hhhh...       one line of 32 bytes, in hex; all-zero lines left out
     puts ok= fail= ...  a quiet run of $PUTs; a failed one shows its own line
     walk key ...        a run of $GETs in sequence, each record's first 8 bytes and RFA
     mark text           a note from the case, saying what the next lines show

**The common record,** made by `MKREC` from a number n, so a line's bytes say which it is:

| Offset | Size | Holds |
| ------ | ---- | ----- |
| 0 | 8 | string, `P` and 7 digits of n: the usual primary key |
| 8 | 4 | unsigned longword n |
| 12 | 4 | signed longword -n |
| 16 | 8 | string, `A` and 7 digits of n/3: three records share each value |
| 24 | 8 | string, `B` and 7 digits of 9999999-n: runs the other way |
| 32 | 4 | packed decimal of n, 7 digits |
| 36 | 8 | unsigned quadword n |
| 44 | 8 | signed quadword -n |
| 52 | to 33000 | letters, with a run of blanks at 60 to 99, for data compression |

Records go in by RAC KEY unless a case says otherwise. Numbers in the ROP field are written as numbers: `RAB$V_REV` and `CDK` may not be names VMS
7.1's macros know, and an undefined symbol there would cost a round.

`TestIdx88Probe` (`internal/console/idx88_test.go`) rehearses the program on govax: it must
assemble, link, and reach its end with no condition. Until govax has indexed files, nearly
every case there fails with RMS$_ORG or what follows from it; the first case, on a sequential
file, exercises the helpers.

## What's next

`vax/` is VMS V7.1's third run; the phase doc's "What round 1 settled" says what it showed. Round 2 is on FORRTL, as Part C says, once this round's files can be read. What's known so far of
FOR$OPEN's KEY= (code 23, by trial over govax, 2026-11-03): the value is the address of a block
whose word at +2 is a count that must be a multiple of 3 (0 is accepted and makes no key, a
number that isn't a multiple is FOR$_INVARGFOR, and any other multiple is FOR$_INVKEYSPE so far).
The items' form isn't found yet, and no shape tried has put a XABKEY in the chain. Round 2 can
make its files by RMS and open them without KEY= in any case.
