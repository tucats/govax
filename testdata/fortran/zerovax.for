      PROGRAM ZEROVAX
C     Fortran 95's zero widths are for Fortran 90 statements only (Phase
C     112, subtask 12). Compiled /NOF90, as a .FOR file is, a zero width is
C     VAX FORTRAN's: a field of no columns, which no value fits, so the
C     record has nothing there and the statement fails with OUTCONERR
C     (FORRTL's number 63) after writing it.
      INTEGER IOS
      WRITE (6, '(1X,A,I0,A)', IOSTAT=IOS) '[', 42, ']'
      WRITE (6, '(1X,I3)') IOS
      WRITE (6, '(1X,A,Z0,A)', IOSTAT=IOS) '[', 255, ']'
      WRITE (6, '(1X,I3)') IOS
      WRITE (6, '(1X,A,F0.2,A)', IOSTAT=IOS) '[', 3.14159, ']'
      WRITE (6, '(1X,I3)') IOS
      END
