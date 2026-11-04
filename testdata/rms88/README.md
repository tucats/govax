# Programs on indexed files, end to end

`docs/PHASE-88 - keyed index files.md`'s subtask 14. `TestIndexedPhones` and
`TestIndexedLoad` (`internal/console/rms88_test.go`) run these on govax, in both
paging modes, with FORTRAN's `/NOOPTIMIZE` and `/OPTIMIZE`, and compare what they
print with the `.out` files, written by hand from the manuals (`make_load_out.py`
works out `load.out` from a plain model of the statements). None of it is a
VMS run; `testdata/idx88` and `testdata/for88` are those.

| Files | What they do |
| ----- | ------------ |
| `phones.mar`, `phones.out` | A telephone book made by `$CREATE` with four XABKEYs, and every record service on it: `$PUT` and its refusals, each key in sequence, keyed `$GET`s with KGE, KGT, and REV, KSZ's and KRF's and ROP's errors, LIM and CDK, RFA, `$FIND`, `$UPDATE`, `$DELETE`, `$REWIND`, and XABSUM |
| `phonesl.mar`, `phonesc.mar`, `phonesl.out` | That file shared by two processes: a subprocess finds a record locked (RLK, and OK_RRL with RRL), the parent frees it, and what the subprocess writes the parent reads |
| `phonesf.for`, `phonesf.out` | The same file from FORTRAN, with no KEY=, each READ by a key RMS's XABKEYs made |
| `phones.com`, `phonesd.out` | DCL on it: TYPE, READ by each key, and ANALYZE/RMS_FILE/FDL and CONVERT/FDL making a second file |
| `phutil.mar` | What the MACRO programs print with |
| `load.for`, `load.out`, `make_load_out.py` | 2,000 records of fixed length and 500 of variable length, through splits, deletes, moves between duplicates, and reinsertion, every key walked after each stage |

Each test ends with ANALYZE/RMS_FILE/CHECK on every file these leave.
