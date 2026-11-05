# The user stack under recursion

`docs/PHASE-89 - reentrant FORTRAN.md`'s subtask 9: FORTRAN programs whose
stack frames are large, or whose recursion never ends, run by
`internal/console/stack89_test.go` in both paging modes (with paging on, on
a page file of the default 32,768 blocks).

| Program | What it shows |
| ------- | ------------- |
| `bigframe.for` | A recursive routine with an 800,000-byte frame, every page written and read back, at depths 1 to 20: 2 levels fit paging off's fixed 2 MB stack, 12 the 10 MB paging on's table area leaves P1 (`TestStack89BigFrames`) |
| `runaway.for` | A recursion with no end: some 87,000 levels of 24 bytes paging off, some 430,000 paging on (`TestStack89Runaway`) |
| `signal.for` | A condition in a routine whose 800,000-byte frame is reserved but untouched: the arrays go on the pages below it, the program's handler continues it, and the second time the traceback reports it; run after `runaway.for`, so it also shows the next image runs as usual |

Each stack that runs out ends its image with no message and `$STATUS`
`%X1000000C`, an access violation marked STS$M_INHIB_MSG: there is no stack
left to report it on, which is VMS 7.1's way (`testdata/pg67`'s part C).
The expected output is written from those rules and the frames' sizes.
