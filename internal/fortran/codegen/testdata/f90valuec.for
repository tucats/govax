      OPTIONS /F90
C     Functions whose CHARACTER value's length is known only when the program
C     runs (Phase 104, subtask 7c): a scalar, a dynamic string the function
C     gets with LIB$SGET1_DD in the caller's class D descriptor, freed by the
C     caller with LIB$SFREE1_DD; an array, its element length the first word
C     of the caller's class NCA descriptor; LEN of each; ENTRY in a function
C     whose value is an array of a known shape, and in one whose value it
C     makes.
      FUNCTION REP(C, N)
      CHARACTER*(*) C
      INTEGER N
      CHARACTER(LEN=N*LEN(C)) REP
      REP = C
      END FUNCTION
      OPTIONS /F90
      FUNCTION TAGS(N, W)
      INTEGER N, W
      CHARACTER(LEN=W) TAGS(N)
      TAGS = '#'
      END FUNCTION
      OPTIONS /F90
      SUBROUTINE USE(S)
      CHARACTER*(*) S
      INTERFACE
        FUNCTION REP(C, N)
        CHARACTER*(*) C
        INTEGER N
        CHARACTER(LEN=N*LEN(C)) REP
        END FUNCTION
        FUNCTION TAGS(N, W)
        INTEGER N, W
        CHARACTER(LEN=W) TAGS(N)
        END FUNCTION
      END INTERFACE
      S = REP('AB', 2) // '!'
      PRINT *, LEN(REP('X', 3)), TAGS(2, 3), LEN(TAGS(1, 4))
      END SUBROUTINE
      OPTIONS /F90
      FUNCTION SQ(N)
      INTEGER N, SQ(3), CU(3)
      SQ = N * N
      RETURN
      ENTRY CU(N)
      CU = N * N * N
      END FUNCTION
      OPTIONS /F90
      FUNCTION UP(N, K)
      INTEGER N, K
      REAL UP(N), DOWN(N)
      ENTRY DOWN(K, N)
      UP = K
      END FUNCTION
