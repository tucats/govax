# The Phase 116 probe: DIFFERENCES

These fixtures are `docs/planning/PHASE-116 - DIFFERENCES.md`'s subtask 2
(its Part D). They ask VMS 7.1 what the DCL Dictionary leaves open about
DIFFERENCES (the phase doc's A.3):

- **The layout, to the byte**, of the merged listing for every shape of
  difference (LAYOUT), and of every other form: `/SEPARATED`,
  `/PARALLEL`, `/MODE=HEX` and `OCTAL`, `/CHANGE_BAR` in each placement,
  `/NONUMBER`, `/WIDTH`, `/SLP`, and several forms together (FORMS).
- **The matching rule**: `/MATCH`, `/WINDOW` (MATCHING), and 540 runs on
  pairs generated from a fixed seed (GENERATED), so the rule can be
  fitted to VMS's answers.
- **Counting** and `/MAXIMUM_DIFFERENCES` (LIMITS).
- **Ignoring**: each `/IGNORE` keyword, EDITED, EXACT, and PRETTY,
  `/COMMENT_DELIMITER`, and the default comment characters of every file
  type the manual names (IGNORING).
- **Names and errors**: the second file's defaults, the next lower
  version, missing and wildcarded files, bad qualifier values, the
  closing echo of long names and many qualifiers (NAMES).
- **The listing file**: where `/OUTPUT` and `/SLP` put it, and its
  attributes and records (OUTPUT).
- **Record formats**: VFC, STREAM_LF, fixed, and records holding every
  byte (FORMATS).
- **The terminal**: the terminal's width, and whether 7.1 has
  `/PAGE` and its companions (`dif116t.com`, by hand).

| File | What it holds |
| ---- | ------------- |
| `gen.go` | Writes everything below but this README: `go run testdata/dif116/gen.go` from the repository root |
| `cases.txt` | Every input file's records and every case: the probe's whole content, which govax's oracle tests read |
| `in/` | The input files as host files, for `exchange.cmd` (gitignored; `gen.go` makes them again) |
| `dif116.com` | The run: 859 cases in sections, each listing to `[DIF116.OUT]case.DIF` (or, for errors, to the log), then `$STATUS` and its message |
| `dif116t.com` | The terminal cases, run by hand |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |

Each case in the log starts with a `#### CASE id: title` line, then the
command (the procedure runs with SET VERIFY), any messages, and a
`STATUS %Xnnnnnnnn text` line. Listings go to files, not the log, so
their bytes (tabs, trailing blanks) come back unchanged.

## What the run changes on the VAX

Nothing outside the exchange volume. It defines two process logical
names, `DIF$TOP` and `DIF$OUT`, and leaves the default directory in
`[DIF116]`. It needs no privileges, but the input files' protection
gives the world no access, so run it as SYSTEM (or any account in the
system group).

## The VAX run

1. **Build the exchange volume**, from the repository root on the host:

       go run testdata/dif116/gen.go
       rm -f testdata/disks/dif116-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/exchange.cmd

   This makes `testdata/disks/dif116-exchange.dsk` (RD53, label DIF116,
   gitignored), about 6,000 blocks used.

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt:

       attach rq1 testdata/disks/dif116-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, mount it and run the probe:

       $ MOUNT DUA1: DIF116
       $ SET DEFAULT DUA1:[DIF116]
       $ @DIF116/OUTPUT=DIF116.LOG

   It writes nothing to the terminal. Its log is `[DIF116]DIF116.LOG`,
   whose last line is `DIF116 FINISHED`. It runs DIFFERENCES 859 times;
   case A21 compares two files of 100,010 records.

4. **The terminal**, as SYSTEM, at a terminal (not a batch job), keeping
   everything the terminal shows from the first command to the last:

       $ SET DEFAULT DUA1:[DIF116]
       $ @DIF116T

   It sets the terminal's width to 80, then 132, and puts it back at the
   end. At `DIFFERENCES` with no parameters, answer the prompts with
   `A02.A` and `A02.B`. The paging qualifiers at the end, if VMS takes
   them, may stop for a key: press Return (or Ctrl/Z) to go on. Paste the
   terminal's text into `testdata/dif116/vax/terminal.log` on the host.

5. **Dismount** and detach:

       $ DISMOUNT DUA1:

       sim> detach rq1

6. **Bring the results back**, from the repository root:

       mkdir -p testdata/dif116/vax/out
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/copyout.cmd

   This copies the log to `vax/dif116.log` and every listing to
   `vax/out/`.

## Rehearsal on govax

`dif116.com` runs to its end on govax (2026-11-25), every DIFFERENCES
an unknown verb until subtask 3: the procedure's other lines work, and
the exchange volume's records are what `cases.txt` says (DUMP/RECORD of
trailing blanks, empty files, form feeds, and the fixed files' every
byte). The one rehearsal-only message is case O09's `%DCL-W-UNDFIL`: with
no image run, its user-mode `SYS$OUTPUT` is still defined at the WRITE
after it.
