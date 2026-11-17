! strings.f90: character work in free form (docs/planning/PHASE-99 - Fortran
! 90.md, subtask 6): concatenation, substrings, LEN, INDEX, ICHAR and CHAR,
! comparisons written with == and <, a passed-length dummy, and constants
! between either mark.
program strings
  implicit none
  character*20 name, reversed, upper
  integer vowels

  name = "Fortran" // ' ' // "Ninety"
  write (*, 10) "joined: [" // name(1:14) // "]"
10 format (1x, a, i3)
  write (*, 10) "length:", len(name)
  write (*, 10) "Ninety starts at", index(name, "Ninety")

  call reverse(name(1:14), reversed)
  write (*, 10) "reversed: [" // reversed(1:14) // "]"

  call upcase(name, upper)
  write (*, 10) "upper: [" // upper(1:14) // "]"
  write (*, 10) "vowels:", vowels(name)

  ! Shorter operands are padded with blanks; LLT compares in ASCII's order.
  write (*, 20) "apple" < "banana", "abc" == "abc   ", llt("Z", "a"), "b" /= 'b'
20 format (1x, 4l2)

  write (*, 10) 'He said "it''s"', 3
  write (*, 10) "He said ""it's""", 4
end program strings

! reverse sets r to s backwards, padded with blanks.
subroutine reverse(s, r)
  character*(*) s, r
  integer k, n
  n = len(s)
  r = ' '
  do k = 1, n
     r(k:k) = s(n-k+1:n-k+1)
  end do
end subroutine reverse

! upcase sets u to s with its small letters capitals.
subroutine upcase(s, u)
  character*(*) s, u
  integer k
  character c
  u = s
  do k = 1, len(s)
     c = s(k:k)
     if (c >= "a" .and. c <= "z") u(k:k) = char(ichar(c) - ichar("a") + ichar("A"))
  end do
end subroutine upcase

integer function vowels(s)
  character*(*) s
  integer k
  vowels = 0
  do k = 1, len(s)
     if (index("aeiouAEIOU", s(k:k)) > 0) vowels = vowels + 1
  end do
end function vowels
