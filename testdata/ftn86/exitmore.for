C     EXITMORE - what's left of the exit handlers after one calls $EXIT? VMS ran no
C     more of them in FTN86.COM's EXITS, whose handlers do FORTRAN I/O and where
C     FTNRTL has one of its own, so this has neither: no I/O statement, and
C     LIB$PUT_OUTPUT to say what ran. Handlers run last declared first.
C
      PROGRAM EXITMORE
      INTEGER SYS$DCLEXH, STATUS
      INTEGER B1(4), B2(4), B3(4)
      EXTERNAL FIRST, SECOND, THIRD
      COMMON /ST/ STATUS
      DATA B1 /0, 0, 1, 0/, B2 /0, 0, 1, 0/, B3 /0, 0, 1, 0/
      B1(2) = %LOC(FIRST)
      B1(4) = %LOC(STATUS)
      B2(2) = %LOC(SECOND)
      B2(4) = %LOC(STATUS)
      B3(2) = %LOC(THIRD)
      B3(4) = %LOC(STATUS)
      CALL CHECK(SYS$DCLEXH(B1))
      CALL CHECK(SYS$DCLEXH(B2))
      CALL CHECK(SYS$DCLEXH(B3))
      CALL LIB$PUT_OUTPUT('MAIN ENDS')
      END
C
      SUBROUTINE CHECK(ST)
      INTEGER ST
      IF (.NOT. ST) CALL LIB$PUT_OUTPUT('DECLARE FAILED')
      END
C
      SUBROUTINE THIRD(ST)
      INTEGER ST
      CALL LIB$PUT_OUTPUT('THIRD CALLS $EXIT')
      CALL SYS$EXIT(%VAL(3))
      CALL LIB$PUT_OUTPUT('THIRD GOES ON')
      END
C
      SUBROUTINE SECOND(ST)
      INTEGER ST
      CALL LIB$PUT_OUTPUT('SECOND RAN')
      END
C
      SUBROUTINE FIRST(ST)
      INTEGER ST
      CALL LIB$PUT_OUTPUT('FIRST RAN')
      END
