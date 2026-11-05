C RECURIO.FOR - recursive routines whose I/O and arguments name their own
C frames (Phase 89, subtask 5): each activation's variables written and read
C back as a NAMELIST group, an internal file written while another activation's
C is, ENCODE and DECODE, an adjustable array as an I/O item, %DESCR, dynamic
C strings as arguments and items, an OPEN's KEY= list from variables, and a
C NAMELIST group of an AUTOMATIC variable in a unit that isn't recursive. Each
C routine does its I/O after its recursive call returns, so storage two
C activations shared would print the innermost one's values everywhere.
      PROGRAM RECURIO
      INTEGER A(5), K, L, I, TRI
      NAMELIST /PAIR/ K, L
      DATA A /5, 4, 3, 2, 1/

      CALL NMLOUT(3)

      OPEN (UNIT=1, STATUS='SCRATCH')
      DO 10 I = 1, 3
         K = I
         L = I*I
         WRITE (1, PAIR)
   10 CONTINUE
      REWIND (1)
      CALL NMLIN(3)
      CLOSE (1)

      CALL AUTONM(1.25)

C     Not in the WRITE's list: TRI writes unit 6 too (RECIO_OPE, 7.1.2).
      I = TRI(3)
      WRITE (6, 100) I
  100 FORMAT (' TRI(3) =', I3)

      CALL CODES(3)
      CALL HALVES(A, 5)
      CALL SHOUT('ab', 3)
      CALL KEYS(3)
      END

C Each activation's DEPTH and SQ, written after the deeper ones are.
      RECURSIVE SUBROUTINE NMLOUT(N)
      INTEGER N, DEPTH, SQ(2)
      NAMELIST /LEVEL/ DEPTH, SQ
      DEPTH = N
      SQ(1) = N*N
      SQ(2) = N*N*N
      IF (N .GT. 1) CALL NMLOUT(N - 1)
      WRITE (6, LEVEL)
      END

C Each activation reads the next group into its own K and L, then shows them
C once the deeper ones have read theirs.
      RECURSIVE SUBROUTINE NMLIN(N)
      INTEGER N, K, L
      NAMELIST /PAIR/ K, L
      READ (1, PAIR)
      IF (N .GT. 1) CALL NMLIN(N - 1)
      WRITE (6, 200) N, K, L
  200 FORMAT (' NMLIN', I2, ': K =', I2, ', L =', I2)
      END

C An AUTOMATIC variable in a group, in a unit that isn't recursive.
      SUBROUTINE AUTONM(X)
      REAL X, Y
      AUTOMATIC Y
      NAMELIST /AUTO/ Y
      Y = X*2
      WRITE (6, AUTO)
      END

C N(N+1)/2, each activation writing its own buffer with the next one's value
C among the items, so one internal WRITE is under way while another runs.
      RECURSIVE INTEGER FUNCTION TRI(N)
      INTEGER N, I, J
      CHARACTER*12 BUF
      IF (N .EQ. 0) THEN
         TRI = 0
         RETURN
      END IF
      WRITE (BUF, 300) N, TRI(N - 1)
  300 FORMAT (2I6)
      READ (BUF, 300) I, J
      TRI = I + J
      WRITE (6, 310) BUF
  310 FORMAT (' TRI: "', A, '"')
      END

C ENCODE into each activation's buffer before the deeper ones, DECODE after.
      RECURSIVE SUBROUTINE CODES(N)
      INTEGER N, M
      CHARACTER*6 B
      ENCODE (6, 400, B) N*11
  400 FORMAT (I6)
      IF (N .GT. 1) CALL CODES(N - 1)
      DECODE (6, 400, B) M
      WRITE (6, 410) N, M
  410 FORMAT (' CODES', I2, ':', I4)
      END

C One array, each activation seeing a shorter piece of it.
      RECURSIVE SUBROUTINE HALVES(A, N)
      INTEGER N, A(N)
      IF (N .GT. 1) CALL HALVES(A, N/2)
      WRITE (6, 500) N, A
  500 FORMAT (' HALVES', I2, ':', 5I3)
      END

C A local passed by %DESCR, and concatenations of a passed-length dummy, as
C an argument and as an item: dynamic strings.
      RECURSIVE SUBROUTINE SHOUT(S, N)
      CHARACTER*(*) S
      INTEGER N
      CHARACTER*8 UP
      CALL STR$UPCASE(%DESCR(UP), %DESCR(S))
      IF (N .GT. 1) CALL SHOUT(S//'x', N - 1)
      WRITE (6, 600) N, UP, S//'!'
  600 FORMAT (' SHOUT', I2, ': ', A, ' ', A)
      END

C An indexed file per activation, its key's place given by variables.
      RECURSIVE SUBROUTINE KEYS(N)
      INTEGER N, KF, KL, K, J
      CHARACTER*11 NAME
      KF = 1
      KL = 4
      WRITE (NAME, 700) N
  700 FORMAT ('RECKEY', I1, '.DAT')
      OPEN (UNIT=10+N, FILE=NAME, STATUS='NEW', ORGANIZATION='INDEXED',
     1 ACCESS='KEYED', FORM='UNFORMATTED', RECORDTYPE='FIXED', RECL=2,
     2 KEY=(KF:KL:INTEGER))
      WRITE (10+N) N*100, N
      IF (N .GT. 1) CALL KEYS(N - 1)
      READ (10+N, KEY=N*100) K, J
      CLOSE (10+N, STATUS='DELETE')
      WRITE (6, 710) NAME, K, J
  710 FORMAT (' KEYS: ', A, I4, I2)
      END
