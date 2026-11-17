! spellings.f90: a FORTRAN-77 program in free form, written with Fortran 90's
! spellings (docs/planning/PHASE-99 - Fortran 90.md, subtask 6): the relational
! operators ==, /=, <, <=, >, and >=; character constants between quotation
! marks; ! comments; several statements on a line with ;; statements continued
! with &; binary, octal, and hexadecimal constants in DATA; and named ENDs.
program spellings
  implicit none
  integer i, j, n, mask, octal, hex
  character*12 word
  logical same
  integer twice
  data mask /b'101'/, octal /o'17'/, hex /z'1F'/   ! 5, 15, and 31

  i = 3; j = 4; n = i + j            ! three statements on one line
  write (*, 10) i == 3, i /= 3, i < j, i <= j, i > j, i >= j
10 format (1x, 6l2)

  ! The old spellings still mean the same.
  write (*, 10) i .eq. 3, i .ne. 3, i .lt. j, i .le. j, i .gt. j, i .ge. j

  write (*, 20) mask, octal, hex, n
20 format (1x, 4i5)

  ! Quotation marks or apostrophes, each doubled inside its own kind.
  write (*, 30) "it's", 'say "hi"', "say ""hi""", 'it''s'
30 format (1x, a, 3(:, '|', a))   ! : ends the record with the items

  ! A character constant continued: & ends the first line and starts the next.
  word = "contin&
         &ued"
  write (*, 30) word(1:9)

  ! A name split across two lines the same way.
  n = 12&
      &34
  write (*, 20) n

  ! A statement continued over a comment line.
  n = i + &
      ! (this line is a comment, and doesn't end the statement)
      j * 100
  write (*, 20) n

  ! An ! inside a constant is the constant's, and so is an &.
  write (*, 30) "not ! a comment", "not & continued"

  same = word == "continued"
  write (*, 10) same, word /= "continued", "abc" < "abd", "b" > "abc"

  call show(twice(21))
end program spellings

! The function's END names it, as the subroutine's does.
integer function twice(k)
  integer k
  twice = k + k
end function twice

subroutine show(k)
  integer k
  write (*, 10) k
10 format (1x, "twice 21 is ", i3)
end subroutine show
