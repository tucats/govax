! layout.f90: free source form's layout (docs/planning/PHASE-99 - Fortran
! 90.md, subtask 6; the standard's 3.3.1): continuation over comment lines and
! blank lines, an & starting the next line or not, a character constant continued
! with its blanks, statements and labels after a ;, keywords with their blank
! or without it, either case, and a line of 132 characters.
PROGRAM Layout
  implicit none
  integer table(12), k, total
  character*40 line
  doubleprecision half
  double precision third

  ! A DATA statement over several lines, with a comment line and a blank
  ! line among them.
  data table / 1, 2, 3, &
               4, 5, 6, &   ! the second row
               ! the third row follows

               7, 8, 9, &
    10, 11, 12 /

  total = 0
  do k = 1, 12; total = total + table(k); enddo
  write (*, 10) "sum of 1 to 12:", total
10 format (1x, a, i4)

  ! A label after a ;, and GO TO with its blank and without it.
  k = 0; 20 k = k + 1
  if (k < 3) goto 20
  write (*, 10) "counted to", k
  k = 0
30 k = k + 2
  IF (K < 7) GO TO 30
  Write (*, 10) "stepped to", k

  ! The blanks after the second & are the constant's.
  line = "two&
         &  blanks"
  write (*, 10) line(1:11)

  ! An & between two tokens, and the next line taken up at its start.
  total = table(1) + &
table(12)
  write (*, 10) "first and last:", total

  half = 0.5d0; third = 1d0/3d0
  if (half < third) then
     write (*, 10) "half is less"
  elseif (half == third) then
     write (*, 10) "the same"
  else if (half > third) then
     write (*, 10) "half is more"
  endif

  k = 0; k = k + 1; k = k + 2; k = k + 3; k = k + 4; k = k + 5; k = k + 6; k = k + 7; k = k + 8; k = k + 9; k = k + 10; k = k + 1111
  write (*, 10) "a full line:", k
END PROGRAM LAYOUT
