C     A unit of the file with a system routine's name is the program's own: no
C     argument need be what the system's takes.
      PROGRAM SYSOWN
      REAL X
      INTEGER RAN
      CALL DATE(1, 2)
      X = SECNDS(3, 4)
      PRINT *, X
      PRINT *, RAN(5.0)
      CALL EXIT
      END

      SUBROUTINE DATE(A, B)
      INTEGER A, B
      PRINT *, 'own DATE', A + B
      END

      REAL FUNCTION SECNDS(A, B)
      INTEGER A, B
      SECNDS = A + B + 0.5
      END

      INTEGER FUNCTION RAN(R)
      RAN = R + 1
      END

C     The program's own EXIT, called with something the system's wouldn't take.
      SUBROUTINE EXIT
      PRINT *, 'own EXIT'
      END
