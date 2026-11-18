C     shortlist.for: VMS's shorter argument list (docs/planning/PHASE-103 -
C     Fortran 90 procedures and interfaces.md, the author's answer 4). This
C     FORTRAN-77 caller, in a file of its own, knows nothing of OPTS's
C     OPTIONAL dummies, and leaves the last ones out altogether, as VMS's
C     convention allows: the argument count says they are absent, so PRESENT
C     says so, and OPTS passes them on to SHOW as absent, not as whatever is
C     past the end of the list. The null argument in the last call is
C     absent too.
      PROGRAM SHORTLIST
      CALL OPTS(1.0)
      CALL OPTS(2.0, 3)
      CALL OPTS(3.0, 4, 'four')
      CALL OPTS(4.0, , 'null')
      END
