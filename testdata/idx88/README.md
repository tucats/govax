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
| `idx88d.mar`, `idx88d.com` | Round 4, DCL on indexed files (below); written by hand |
| `exchanged.cmd`, `copyoutd.cmd` | Round 4's exchange volume scripts |
| `idx88c.com`, `idx88p.mar` | Round 3, the utilities (below); written by hand |
| `exchangec.cmd`, `copyoutc.cmd` | Round 3's exchange volume scripts |

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

## Round 4: DCL (`idx88d.com`)

Phase 88's subtask 9. DCL can't make an indexed file, so `idx88d.mar` (written by hand)
makes three with no records: D1.IDX (variable records, four keys: a string primary, a
string alternate with duplicates, a binary one, a descending string), D2.IDX (fixed
records of 20 bytes, a primary key with duplicates), and D3.IDX (one key). `idx88d.com`
then writes, reads, changes, and shows them, in sections D1 to D11: WRITE in any order and
its refusals; READ in sequence; READ's `/KEY`, `/MATCH`, `/INDEX`, `/DELETE`, and
`/NOLOCK`; WRITE/UPDATE; two streams on one file; the fixed-length file; OPEN/APPEND;
TYPE, DUMP/RECORD, DIRECTORY/FULL, and F$FILE_ATTRIBUTES. It runs with SET NOON and SET
VERIFY, each command's `$STATUS` shown, and each READ's symbol set to "-" first. A loop's
READ leaves on `/ERROR` as well as `/END_OF_FILE`, so nothing can go round forever.
MACRO's own output goes to `IDX88DM.LOG`, which stays on the volume.

On the host, from the repository root:

    govax console < testdata/idx88/exchanged.cmd

That makes `testdata/disks/idx88d-exchange.dsk`, mounted there as DUA5. On the VAX, with
its `[000000]` the default directory:

    $ @IDX88D/OUTPUT=IDX88D.LOG

It takes a few seconds and reads nothing from the terminal. Back on the host:

    govax console < testdata/idx88/copyoutd.cmd

which brings back `vax/idx88d.log`, VMS V7.1's run (2026-11-04). `TestIdx88DProbe`
(`internal/console/idx88dcl_test.go`) rehearses it on govax, and `TestIdx88DOracle` holds govax's
log to VMS's line for line, times masked.

## Round 3: the utilities (`idx88c.com`)

Phase 88's subtask 10: CREATE/FDL, CONVERT, and ANALYZE/RMS_FILE, which the
*Record Management Utilities Reference Manual* (VAX 6.0) describes. The round is what the
manual leaves to their behavior. `idx88c.com` (written by hand, its FDL files and text inputs
inline, by DCL's CREATE) runs in sections C0 to C13, each command's `$STATUS` shown:

| Section | What it does |
| ------- | ------------ |
| C0 | Assembles and links `idx88p.mar` (below); round 1's files are already in `[IN]` |
| C1 | HELP/MESSAGE's text for the CONVERT, FDL, and ANLRMS facilities, to `MSG*.TXT` (DIGITAL's text: local only) |
| C2, C3 | CREATE/FDL: sequential and relative files of every format and attribute, indexed files of every key type, segments, flags, areas, prologs, and RMS's refusals |
| C4 | FDL's own refusals, one fault a file |
| C5 | CREATE/FDL from ANALYZE/RMS_FILE/FDL's files of round 1's |
| C6 to C10 | CONVERT: between organizations, `/KEY`, reorganizing, loading (fast or not, fills, `/NOSORT`), exceptions, `/EXIT`, `/STATISTICS`, `/PAD`, `/TRUNCATE`, `/FIXED_CONTROL`, stream formats, `/NOCREATE`, `/MERGE`, `/APPEND`, `/PROLOG`, and the refusals |
| C11 | ANALYZE/RMS_FILE on each organization and format, `/FDL`, `/STATISTICS`, `/SUMMARY`, the output's default name, lists, wildcards, refusals |
| C12 | `/CHECK` on 14 copies of round 1's S35.IDX, each with one byte changed by IDX88P |
| C13 | A save set of everything made, `IDX88C.BCK` |

`idx88p.mar` adds a number to one byte of a file by block I/O, its arguments the DCL symbols
PFILE, PVBN, POFF, and PADD, and prints `patch FILE vbn N offset M: OLD to NEW`.

On the host, from the repository root:

    govax console < testdata/idx88/exchangec.cmd

That makes `testdata/disks/idx88c-exchange.dsk` (mounted there as DUA5), with round 1's
files restored into `[IN]` from `vax/idx88.bck`. On the VAX, with its `[000000]` the default
directory:

    $ @IDX88C/OUTPUT=IDX88C.LOG

It reads nothing from the terminal. Back on the host:

    mkdir -p testdata/disks/idx88c-help
    govax console < testdata/idx88/copyoutc.cmd

which brings back `vax/idx88c.log` (VMS V7.1's run, 2026-11-04), `vax/idx88c.bck` (the files made, of our own data), and
`vax/idx88cbck.txt`, and puts the `MSG*.TXT` files in `testdata/disks/idx88c-help`, which is
never committed. `IDX88CM.LOG`, MACRO's own output, stays on the volume.
`TestIdx88CProbe` (`internal/console/idx88util_test.go`) rehearses the round on govax.

## What's next

`vax/` is VMS V7.1's third run; the phase doc's "What round 1 settled" says what it showed. Round 2 is on FORRTL, in `testdata/for88`, whose README has FOR$OPEN's KEY= and FOR$READ_KU's arguments.
