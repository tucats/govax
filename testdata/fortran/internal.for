C     INTERNAL.FOR - internal files, ENCODE, and DECODE: section 7.2.4 and
C     7.3.4 of the manual, its appendix A.1's examples, and every kind of
C     place a file can be.
      PROGRAM INTERN
      CHARACTER*12 BUFFER
      CHARACTER*20 STR
      CHARACTER*5 LINES(3)
      CHARACTER*300 LONG
      CHARACTER*8 PART
      INTEGER K(3), L, IOS
      REAL X, Y, VAL
      BYTE BYTES(12)
      INTEGER WORDS(3)
      CHARACTER*80 TEXT
      COMMON /SHARED/ TEXT
  100 FORMAT (2F6.2)
  110 FORMAT (3I4)
  120 FORMAT (A)
C     The two ways of writing to memory, the old and the new.
      X = 1.5
      Y = -2.25
      VAL = 3.14159
      ENCODE (12, 100, BUFFER) X, Y
      PRINT *, '|', BUFFER, '|'
      BUFFER = ' '
      WRITE (BUFFER, 100) X, Y
      PRINT *, '|', BUFFER, '|'
      ENCODE (20, '(F10.2)', STR) VAL
      PRINT *, '|', STR, '|'
      STR = 'XXXXXXXXXXXXXXXXXXXX'
      WRITE (STR, '(F10.2)') VAL
      PRINT *, '|', STR, '|'
C     The same for reading.
      BUFFER = '  1.50 -2.25'
      X = 0.0
      Y = 0.0
      DECODE (12, 100, BUFFER) X, Y
      PRINT *, X, Y
      X = 0.0
      Y = 0.0
      READ (BUFFER, 100) X, Y
      PRINT *, X, Y
C     Appendix A.1's examples, on a numeric buffer.
      BUFFER = '123456789012'
      DECODE (12, 110, BUFFER) K
      PRINT *, K(1), K(2), K(3)
      ENCODE (12, 110, BYTES) K(3), K(2), K(1)
      PRINT '(1X,12A1)', BYTES
      ENCODE (12, 110, WORDS) K(3), K(2), K(1)
      DECODE (12, '(A4,A4,A4)', WORDS) BUFFER(1:4), BUFFER(5:8),
     &    BUFFER(9:12)
      PRINT *, '|', BUFFER, '|'
C     A short ENCODE's rest is blank.
      STR = 'XXXXXXXXXXXXXXXXXXXX'
      ENCODE (8, '(I4)', STR) 7
      PRINT *, '|', STR, '|'
C     An array's elements are records, and those not reached stay.
      LINES(1) = 'AAAAA'
      LINES(2) = 'BBBBB'
      LINES(3) = 'CCCCC'
      WRITE (LINES, '(A,/,A)') 'ONE', 'TWO'
      PRINT *, '|', LINES(1), '|', LINES(2), '|', LINES(3), '|'
C     An element is one record.
      WRITE (LINES(3), 120, IOSTAT=IOS) 'X', 'Y'
      PRINT *, IOS, '|', LINES(3), '|'
C     A substring, whose length is computed.
      STR = 'XXXXXXXXXXXXXXXXXXXX'
      L = 6
      WRITE (STR(3:L), '(I4)') 12
      PRINT *, '|', STR, '|'
C     List-directed, both ways.
      WRITE (TEXT, *) 1, 2.5, 'ABC'
      PRINT *, '|', TEXT(1:40), '|'
      TEXT = '1 2.5 ''ABC'''
      READ (TEXT, *) L, X, PART
      PRINT *, L, X, ' ', PART
C     Too much for the file: what was written stays.
      STR = 'XXXXXXXXXXXXXXXXXXXX'
      WRITE (STR(1:10), '(I5,I6)', ERR=10) 1, 2
      PRINT *, 'NO ERROR'
   10 PRINT *, '|', STR, '|'
C     The end of an internal file is END=.
      L = 99
      READ (BUFFER, 110, END=20) K, L
      PRINT *, 'NO END'
   20 PRINT *, K(3), L
      READ (LINES, '(A)', END=30) PART, PART, PART, PART
      PRINT *, 'NO END'
   30 PRINT *, '|', PART, '|'
C     DECODE's second record is an error.
      DECODE (12, '(I4,/,I4)', BUFFER, IOSTAT=IOS) K(1), K(2)
      PRINT *, IOS
      ENCODE (12, '(I4,/,I4)', STR, ERR=40) K(1), K(2)
      PRINT *, 'NO ERROR'
   40 CONTINUE
C     A record far longer than a unit's.
      WRITE (LONG, '(T290,A,T300,A)') 'END', 'Z'
      PRINT *, '|', LONG(288:300), '|'
      READ (LONG, '(T290,A3,T300,A1)') PART(1:3), PART(4:4)
      PRINT *, PART(1:4)
C     A file inside an I/O statement on a unit, and in a routine, on
C     dummy arguments.
      PRINT *, 'IS ', FN(6), ' DONE'
      CALL SUB(STR, LINES, K)
      PRINT *, '|', STR, '|', LINES(2), '|', K(3)
      END

      FUNCTION FN(I)
      CHARACTER*8 T
      WRITE (T, '(I4)') I
      READ (T, '(F4.0)') FN
      END

      SUBROUTINE SUB(S, A, N)
      CHARACTER*(*) S, A(*)
      INTEGER N(3)
      WRITE (S, '(I3,''!'')') 42
      WRITE (A(2), '(I3)') 43
      ENCODE (4, '(I4)', N(3)) 44
      END
