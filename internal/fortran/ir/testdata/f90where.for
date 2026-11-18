      OPTIONS /F90
C     WHERE (Phase 102, subtask 10): the mask in each nest, an ELSEWHERE,
C     the mask in a temporary when an assignment changes what it reads, a
C     mask that runs the loop down, and a value through a temporary.
      PROGRAM F90WHR
      REAL A(10), B(10)
      WHERE (A .GT. 0.0) A = SQRT(A)
      WHERE (A .GT. 0.0)
         B = A
      ELSEWHERE
         B = 0.0
      END WHERE
      WHERE (A .GT. 0.0)
         A = -A
         B = 1.0
      END WHERE
      WHERE (A(1:9) .GT. 0.0) A(2:10) = 1.0
      WHERE (B .GT. 0.0) A = A(10:1:-1)
      END
