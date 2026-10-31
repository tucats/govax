C     Alternate returns across files (Phase 79, subtask 7): the routine and
C     its entry point are in another module, and nothing of the specifiers
C     goes in the argument list.
      PROGRAM ALTRETURNS
      REAL X
      X = 2.5
      CALL CHOOSE(2, X, *10, *20)
      PRINT *, 'Went on'
10    PRINT *, 'First'
20    PRINT *, 'Second'
      CALL AGAIN(*30, X, 1)
      PRINT *, 'Went on'
30    PRINT *, 'Again'
      END
