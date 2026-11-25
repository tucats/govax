# The Phase 117 probe: SUMSLP (EDIT/SUM) and MACRO/UPDATE

These fixtures are `docs/planning/PHASE-117 - SUMSLP.md`'s subtask 2 (its
Part D). They ask VMS 7.1 what the *OpenVMS SUMSLP Utility Manual*
(AA-PS6EA-TE) leaves open about SUMSLP, the batch editor behind EDIT/SUM
and MACRO/UPDATE (the phase doc's A.3):

- **HELP**: VMS's online help for EDIT/SUM, SUMSLP, and MACRO/UPDATE, and
  HELP/MESSAGE for the SUM facility, written to the log first.
- **COMMAND**: `/UPDATE`'s defaults (`.UPD`, a bare `/UPDATE`, each file's
  defaults from the one before), qualifiers in the wrong place, the
  output's and listing's default names (in a scratch directory), `/NOOUTPUT`,
  missing and wildcarded files, `EDIT/SLP`, `/UPDATE=SYS$INPUT` with the
  update in the procedure.
- **BASICS**: every edit command on one update file: replace, insert,
  delete at the start, middle, and end; a bare `-` and `-0`; the
  terminator missing, alone, or followed by more; the operators `<`, `\`,
  and `%`; the period locator; comments; long and empty records.
- **SYNTAX**: malformed commands, each with its message and status.
- **ORDER**: commands out of order and overlapping in one file.
- **MERGE**: two and three update files meeting at one line, in both
  orders (the manual: the last listed has the highest precedence).
- **AUDIT**: the audit trail in the listing and the output, changed,
  suppressed, long, on records of every width; `/HEADER`'s VFC records,
  dumped.
- **LISTING**: the listing's layout, line numbers past 999 and 9999, an
  empty file; 32,767 and 32,768 lines (the manual's limit).
- **FORMATS**: VFC, STREAM_LF, and fixed inputs and update files, records
  holding every byte, and the outputs' attributes.
- **ROUNDTRIP**: `DIFFERENCES/SLP` of a pair, `EDIT/SUM` of its script, and
  `DIFFERENCES` of the result with the revision: 16 cases by hand (records
  starting with an operator among them) and 120 generated from a fixed
  seed. VMS's `.DIF` scripts come back too, more evidence for Phase 116's
  `/SLP`.
- **MACRO**: `MACRO/UPDATE` with the default, named, and several update
  files, on a list and a concatenation, an update that makes an error, a
  missing update file, `/NOUPDATE`, the listing's audit trail.
- **The terminal**: `sum117t.com`, by hand.

| File | What it holds |
| ---- | ------------- |
| `gen.go` | Writes everything below but this README: `go run testdata/sum117/gen.go` from the repository root |
| `cases.txt` | Every input file's records and every case: the probe's whole content, which govax's oracle tests read |
| `in/` | The input files as host files, for `exchange.cmd` (gitignored; `gen.go` makes them again) |
| `sum117.com` | The run: 346 cases in sections, each step's `$STATUS` and its message in the log, outputs in `[SUM117.OUT]` |
| `sum117t.com` | The terminal cases, run by hand |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts that build the exchange volume and bring the results back |
| `vax/` | VMS 7.1's answers, once run: `sum117.log`, `terminal.log`, `out/` |

Each case in the log starts with a `#### CASE id: title` line, then its
commands (the procedure runs with SET VERIFY), any messages, and a
`STATUS %Xnnnnnnnn text` line after each step. Outputs go to files, not
the log, so their bytes come back unchanged: `ID.OUT` the edited file,
`ID.LIS` the listing, `ID.DIF` and `ID.CMP` a round trip's script and
comparison, `ID.OBJ` an object, `ID.Kn` a file kept from the scratch
directory.

## What the run changes on the VAX

Nothing outside the exchange volume. It defines three process logical
names, `SUM$TOP`, `SUM$OUT`, and `SUM$SCR`, and leaves the default
directory in `[SUM117]`. Run it as SYSTEM: the input files' protection
gives the world no access.

## The VAX run

1. **Build the exchange volume**, from the repository root on the host:

       go run testdata/sum117/gen.go
       rm -f testdata/disks/sum117-exchange.dsk
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/exchange.cmd

   This makes `testdata/disks/sum117-exchange.dsk` (RD53, label SUM117,
   gitignored), about 2,600 blocks used.

2. **Attach it** to the simh VAX on a free disk unit, at the `sim>`
   prompt:

       attach rq1 testdata/disks/sum117-exchange.dsk

   The steps below say `DUA1:`; use the unit you attached.

3. **As SYSTEM**, mount it and run the probe:

       $ MOUNT DUA1: SUM117
       $ SET DEFAULT DUA1:[SUM117]
       $ @SUM117/OUTPUT=SUM117.LOG

   It writes nothing to the terminal. Its log is `[SUM117]SUM117.LOG`,
   whose last line is `SUM117 FINISHED`. Cases L09 and L10 edit files of
   32,767 and 32,768 lines.

4. **The terminal**, as SYSTEM, at a terminal (not a batch job), keeping
   everything the terminal shows from the first command to the last:

       $ SET DEFAULT DUA1:[SUM117]
       $ @SUM117T

   Answer the prompts as its comments say: at `EDIT/SUM` with no
   parameter, `S10.SRC/UPDATE=C01`; at the first `/UPDATE=SYS$COMMAND`,
   the three lines `-3`, `typed after line 3`, `/`, then Ctrl/Z if it is
   still reading; at the second, the first two lines and Ctrl/Z. Paste the
   terminal's text into `testdata/sum117/vax/terminal.log` on the host.

5. **Dismount** and detach:

       $ DISMOUNT DUA1:

       sim> detach rq1

6. **Bring the results back**, from the repository root:

       mkdir -p testdata/sum117/vax/out
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/copyout.cmd

   This copies the log to `vax/sum117.log` and every output to `vax/out/`.

## The rehearsal

`sum117.com` was run on govax before the VAX run (2026-11-25), on a copy of
the exchange volume, to find mistakes in the procedure itself. govax has no
EDIT/SUM or MACRO/UPDATE yet, so those steps fail with `%DCL-W-IVQUAL`, and
nothing else did.
