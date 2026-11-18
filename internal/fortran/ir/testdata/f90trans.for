      OPTIONS /F90
C     The transformational functions (Phase 104, subtask 8), each computed
C     into a temporary before the statement's loops: SPREAD, CSHIFT, EOSHIFT
C     with BOUNDARY, PACK, UNPACK, RESHAPE of values with a shape known only
C     at run time, and SUM by a DIM= known only at run time, checked under
C     /CHECK=BOUNDS.
      SUBROUTINE TRANS(V, M, L, N, K)
      INTEGER N, K, V(6), M(3, 3), W(3)
      LOGICAL L(3, 3)
      M = SPREAD(V(1:3), 2, N)
      V = CSHIFT(V, -K)
      M = EOSHIFT(M, 1, -1, 2)
      W = PACK(M, L)
      M = UNPACK(W, L, 0)
      PRINT *, RESHAPE(V, (/ 2, N /))
      END SUBROUTINE
      OPTIONS /F90/CHECK=BOUNDS
      SUBROUTINE DIMS(M, K, W)
      INTEGER K, M(3, 3), W(3)
      W = SUM(M, DIM=K)
      END SUBROUTINE
