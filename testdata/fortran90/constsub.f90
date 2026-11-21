! Substrings of literal character constants (Phase 111, subtask 8a, found by
! fcalc): Fortran 90's R609 lets a substring's parent string be a scalar
! constant, so 'ABCDEF'(2:4) is 'BCD'. Bounds may be constants, known only
! when the program runs, or left out, and the constant may be a named one's
! value or written in quotes of either kind.
program constsub
  implicit none
  character(len=*), parameter :: digits = '0123456789'
  character(len=12) :: word
  integer :: k, at

  ! Constant bounds: folded when compiling.
  print '(1x, a)', 'abcdef'(2:4)
  print '(1x, a)', "quoted"(:3)
  print '(1x, a)', 'tail'(3:)

  ! Bounds known only when running: a letter at a time.
  word = ' '
  do k = 1, 5
    word(k:k) = 'HELLO'(6 - k:6 - k)
  end do
  print '(1x, a)', trim(word)

  ! The usual use: lowercasing by a letter's place in one constant and
  ! taking the same place in another.
  word = 'Fortran NINE'
  do k = 1, len(word)
    at = index('ABCDEFGHIJKLMNOPQRSTUVWXYZ', word(k:k))
    if (at > 0) word(k:k) = 'abcdefghijklmnopqrstuvwxyz'(at:at)
  end do
  print '(1x, a)', word

  ! A digit by its value, and a substring of a constant joined to another.
  print '(1x, 3a)', '0123456789'(8:8), '0123456789'(1:1), digits(5:5) // '!'(1:1)

  ! An empty substring: no characters.
  print '(1x, i1)', len('abc'(3:2))
end program constsub
