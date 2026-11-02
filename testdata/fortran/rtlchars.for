      PROGRAM RTLCHR
C     The edges of FTNRTL's character work (Phase 84, subtask 11), held on
C     VMS's FORRTL's rules and FTNRTL's before it was tuned: integer fields
C     at the limits of a longword, leading blanks and tabs, a record's gaps,
C     and characters past a record's end.
      CHARACTER*20 F
      CHARACTER*12 C12
      CHARACTER*30 OUT
      CHARACTER*5 C5
      CHARACTER*2 C2
      CHARACTER TAB
      INTEGER V, W, E
      INTEGER*2 HW
      BYTE BB
      TAB = CHAR(9)
C     ------ integers in, by the field's limits ------
      CALL RD('(I12)', '  2147483647')
      CALL RD('(I12)', '  2147483648')
      CALL RD('(I12)', ' -2147483648')
      CALL RD('(I12)', ' -2147483649')
      CALL RD('(I12)', ' 214748364')
      CALL RD('(I12)', ' 214748365')
      CALL RD('(I12)', '000000000012')
      CALL RD('(I12)', '999999999999')
C     Signs, and what isn't a digit.
      CALL RD('(I5)', '  +42')
      CALL RD('(I5)', '  -42')
      CALL RD('(I5)', '4+2')
      CALL RD('(I5)', '12A')
      CALL RD('(I5)', '+')
C     Nothing there, blanks, and blanks among digits, both ways.
      CALL RD('(I5)', ' ')
      CALL RD('(I5)', '     ')
      CALL RD('(I5)', ' 1 2 ')
      CALL RD('(BZ,I5)', ' 1 2 ')
      CALL RD('(BZ,I5)', '- 1  ')
C     Tabs before the digits, and only tabs.
      CALL RD('(I5)', TAB // '7')
      CALL RD('(I5)', ' ' // TAB // ' 8')
      CALL RD('(I5)', TAB // TAB)
C     A comma ends a field; a short width cuts the digits.
      CALL RD('(I5)', '12,34')
      CALL RD('(I2)', '1234')
      CALL RD('(T3,I2)', '1234')
      CALL RD('(T9,I2)', '1234')
C     A short item.
      C12 = '  300'
      READ (C12, '(I5)', IOSTAT=E) HW
      PRINT 900, 'WORD', HW, E
      READ (C12, '(I5)', IOSTAT=E) BB
      PRINT 900, 'BYTE', BB, E
      C12 = ' 70000'
      READ (C12, '(I6)', IOSTAT=E) HW
      PRINT 900, 'WORD BIG', E
      C12 = '  -12'
      READ (C12, '(I5)', IOSTAT=E) BB
      PRINT 900, 'BYTE NEG', BB, E
  900 FORMAT (1X, A8, 2I12)
C     ------ characters out ------
      CALL WR('(A)', 'ABC')
      CALL WR('(A5)', 'ABC')
      CALL WR('(A2)', 'ABC')
      CALL WR('(''X'', A)', 'ABC')
      CALL WR('(T10, A)', 'ABC')
      CALL WR('(10X, A)', 'ABC')
      CALL WR('(''12345'', T3, A)', 'ABC')
      CALL WR('(''12345'', T2, A)', 'ABC')
      CALL WR('(''1234567'', TL4, A)', 'ABC')
      CALL WR('(''12'', TR3, A)', 'ABC')
      CALL WR('(''12'', 3X)', 'ABC')
      CALL WR('(''AB'', T9, A, T1, A)', 'CD')
      CALL WR('(T28, A)', 'ABC')
      CALL WR('(T29, A)', 'ABC')
      CALL WR('(T31, A)', 'ABC')
      CALL WR('(A32)', 'ABC')
      CALL WR('(A30)', 'ABC')
      CALL WR('(I0)', 'ABC')
C     Before the first column.
      CALL WR('(T0, A)', 'ABC')
      CALL WR('(''12'', TL5, A)', 'ABC')
      CALL WR('(TL1, A)', 'ABC')
C     ------ characters in ------
      CALL RC('(A5)', 'ABCDE')
      CALL RC('(A5)', 'ABC')
      CALL RC('(A)', 'AB')
      CALL RC('(A7)', 'ABCDEFG')
      CALL RC('(A2)', 'ABCD')
      CALL RC('(T3, A5)', 'ABCDEFG')
      CALL RC('(T3, A5)', 'ABCD')
      CALL RC('(T7, A5)', 'ABCD')
      CALL RC('(5X, A5)', 'ABCD')
      CALL RC('(''XX'', A5)', 'ABCDEFG')
      CALL RC('(TL1, A5)', 'ABCDEFG')
      CALL RC('(T0, A5)', 'ABCDEFG')
      C12 = 'ABCD'
      READ (C12, '(A2, TL1, A2)', IOSTAT=E) C2, C5
      PRINT 901, 'TWO', C2, C5, E
  901 FORMAT (1X, A8, '[', A, '][', A, ']', I12)
C     ------ to a unit, and back ------
      OPEN (1, FILE='RTLCHARS.DAT', STATUS='NEW')
      WRITE (1, '(A, T9, A, T5, I3)') 'FIRST', 'LAST', 42
      WRITE (1, '(I12)') -2147483647 - 1
      WRITE (1, '(5X)')
      REWIND 1
      READ (1, '(A)') OUT
      PRINT 902, 'BACK', OUT
  902 FORMAT (1X, A8, '[', A, ']')
      READ (1, '(I12)') V
      PRINT 900, 'BACK', V
      READ (1, '(I8)') V
      PRINT 900, 'BACK', V
      CLOSE (1, STATUS='DELETE')
      END

      SUBROUTINE RD(FMT, TEXT)
      CHARACTER*(*) FMT, TEXT
      INTEGER V, E
      V = -1
      READ (TEXT, FMT, IOSTAT=E) V
      PRINT 900, FMT, V, E
  900 FORMAT (1X, A10, 2I12)
      END

      SUBROUTINE WR(FMT, TEXT)
      CHARACTER*(*) FMT, TEXT
      CHARACTER*30 OUT
      INTEGER E
      OUT = '******************************'
      WRITE (OUT, FMT, IOSTAT=E) TEXT
      PRINT 900, FMT, OUT, E
  900 FORMAT (1X, A24, '[', A, ']', I5)
      END

      SUBROUTINE RC(FMT, TEXT)
      CHARACTER*(*) FMT, TEXT
      CHARACTER*5 C
      INTEGER E
      C = '*****'
      READ (TEXT, FMT, IOSTAT=E) C
      PRINT 900, FMT, C, E
  900 FORMAT (1X, A14, '[', A, ']', I5)
      END
