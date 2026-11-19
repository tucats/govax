      OPTIONS /F90
C     ALLOCATE and DEALLOCATE of pointers (Phase 107, subtask 4): a
C     scalar pointer by FTN$ALLOC_SCALAR and FTN$DEALLOC_SCALAR with
C     its target's size, a CHARACTER one's header written first, a
C     record's; an array pointer by FTN$ALLOCATE and FTN$DEALLOCATE
C     with the pointer's flag, a COMMON one's header written first;
C     STAT= on both.
      PROGRAM F90PALLOC
      TYPE PAIR
        INTEGER I, J
      END TYPE
      INTEGER, POINTER :: P, A(:), K(:, :)
      CHARACTER(LEN=6), POINTER :: C
      TYPE(PAIR), POINTER :: R
      REAL, POINTER :: S(:)
      COMMON /BLK/ S
      INTEGER IS, N
      N = 4
      ALLOCATE (P, C, R, A(N), K(0:N, 2))
      ALLOCATE (S(N), STAT=IS)
      P = 1
      DEALLOCATE (P, C, R, A, STAT=IS)
      DEALLOCATE (K, S)
      END
