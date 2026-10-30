C     Section 4.3's example, and every rule of its list: numbers converted,
C     character constants padded and cut, octal and hexadecimal constants
C     extended and cut, a code for a CHARACTER*1 item, a Hollerith or
C     character constant for a number.
      PROGRAM DATAS
      INTEGER A(10), B(10)
      CHARACTER BELL, TAB, LF, FF, STARS*6
      DATA A,STARS /10*0,'****'/
      DATA BELL,TAB,LF,FF /7,9,10,12/
      DATA (B(I), I=1,10,2) /5*1/
C     Numbers converted to the item's type.
      REAL X
      DOUBLE PRECISION D
      INTEGER*2 K
      LOGICAL*1 L
      COMPLEX Z
      DATA X /1/, D /0.1/, K /-2/, L /.TRUE./, Z /(1.0, -1.0)/
C     Octal and hexadecimal: extended on the left, cut on the left.
      INTEGER H
      INTEGER*2 HC
      REAL HR
      DATA H /'FF'X/, HC /'12345'X/, HR /'00004080'X/
C     Hollerith and character constants given to numbers.
      INTEGER HH, CC
      DATA HH /2HAB/, CC /'ABCDEF'/
C     Substrings, an element, and a two-dimensional implied-DO.
      CHARACTER*4 SUB, NAMES(2)
      DATA SUB(2:3) /'XY'/, NAMES(2)(1:1) /'Q'/
      INTEGER T(2,3)
      DATA ((T(M,N), M=1,2), N=1,3,2) /1,2,3,4/
C     Initial values in type statements, and a constant's name.
      PARAMETER (NSIX=6, PI=3.14)
      INTEGER V/5/, VV(3)/NSIX, 2*7/
      REAL Y/PI/
      SAVE
      END

C     Section 4.1's example, and an unnamed BLOCK DATA.
      BLOCK DATA BLKDAT
      INTEGER S,X
      LOGICAL T,W
      DOUBLE PRECISION U
      DIMENSION R(3)
      COMMON /AREA1/R,S,T,U /AREA2/W,X,Y
      DATA R/1.0,2*2.0/, T/.FALSE./, U/0.214537D-7/, W/.TRUE./, Y/3.5/
      SAVE /AREA1/
      END

      BLOCK DATA
      COMMON /AREA3/ I
      DATA I /3/
      END

C     EXTERNAL and INTRINSIC.
      SUBROUTINE USES(DUMMY)
      EXTERNAL BLKDAT, DUMMY, SQRT, OTHER
      INTRINSIC SIN
      REAL SQRT
      Y = SQRT(2.0)
      CALL OTHER
      END
