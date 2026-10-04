$ ! CELLS.COM - a follow-up to Phase 29's VMS round (docs/PHASE-29.md,
$ ! subtask 14): the order of the fixup section's cells. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @CELLS/OUTPUT=CELLS.LOG
$ !
$ SET NOON
$ SET VERIFY
$ MACRO/LIST CELLS
$ MACRO/LIST CELLSB
$ LINK/MAP CELLS
$ LINK/MAP=CELLS2/EXECUTABLE=CELLS2 CELLS,CELLSB
$ ANALYZE/IMAGE/OUTPUT=CELLS.ANI CELLS.EXE
$ ANALYZE/IMAGE/OUTPUT=CELLS2.ANI CELLS2.EXE
$ DIRECTORY/SIZE=ALL/DATE CELLS*.*
$ SET NOVERIFY
$ EXIT
