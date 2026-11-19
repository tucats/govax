      OPTIONS /F90
C     Pointer dummy arguments (Phase 107, subtask 6a): each is the
C     address of the caller's own pointer storage, a longword (IP), a
C     class S descriptor (CP, its length the caller's), or a class NCA
C     descriptor (AP, reached as a located cell at the address its
C     argument holds); =>, an element, ALLOCATE, DEALLOCATE, NULLIFY of
C     an OPTIONAL one, ASSOCIATED, and LEN through them; the pointers
C     passed on to an internal procedure's pointer dummies, which also
C     reach the host's.
      SUBROUTINE F90PDUM(IP, AP, CP, OP)
      INTEGER, POINTER :: IP
      REAL, POINTER :: AP(:)
      CHARACTER(LEN=*), POINTER :: CP
      INTEGER, POINTER, OPTIONAL :: OP
      INTEGER, TARGET :: N
      REAL, TARGET :: V(10)
      CHARACTER(LEN=8), TARGET :: S
      IP => N
      IP = IP + 1
      AP => V(2:10:2)
      AP(2) = AP(1)
      CP => S(1:4)
      ALLOCATE (AP(N))
      DEALLOCATE (AP)
      IF (PRESENT (OP)) NULLIFY (OP)
      IF (ASSOCIATED (AP, V)) N = LEN (CP)
      CALL INNER(IP, AP)
      CONTAINS
      SUBROUTINE INNER(JP, BP)
      INTEGER, POINTER :: JP
      REAL, POINTER :: BP(:)
      JP => IP
      BP => AP
      END SUBROUTINE
      END
