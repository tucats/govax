C     Entry points called from another file (Phase 79, subtask 6): LINK finds
C     SETVAL, GETVAL, and TWICE in the module of the units KEEP and ONCE,
C     however the modules are ordered.
      PROGRAM ENTRIES
      REAL GETVAL, ONCE
      REAL*8 TWICE
      CALL SETVAL(4.5)
      PRINT *, GETVAL()
      PRINT *, ONCE(1.5)
      PRINT *, TWICE(1.5)
      END
