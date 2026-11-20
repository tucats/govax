! charfns.f90: Fortran 90's character functions and lengths of zero
! (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md, subtask 3):
! LEN_TRIM, TRIM, ADJUSTL, ADJUSTR, REPEAT, SCAN, VERIFY, and INDEX with BACK,
! of constants (folded, in PARAMETERs too), of variables, on arrays, and by
! keyword; TRIM's value given to an argument and joined; a declared length of
! zero, a substring past its upper bound, and the functions of empty strings.
program charfns
  implicit none
  character(len=*), parameter :: title = trim('  Results   ')
  character(len=*), parameter :: rule = repeat('=-', 4) // '='
  integer, parameter :: where = scan('alpha,beta', ',')
  character(len=12) :: s, t
  character(len=6) :: names(3)
  character(len=0) :: none
  character(len=40) :: line
  integer :: i, n, k(3)
  logical :: back

  write (*, '(1x, a, "|", a, "|", i3)') rule, title, where

  ! A string with blanks before, inside, and after.
  s = '  to be  '
  write (*, '(1x, 2i4, " [", a, "]")') len_trim(s), len(trim(s)), trim(s)
  write (*, '(1x, "[", a, "] [", a, "]")') adjustl(s), adjustr(s)
  write (*, '(1x, "[", a, "]")') trim(adjustl(s)) // '.'

  ! Searching: one character in line, a set through FTNRTL, from either end.
  back = .true.
  write (*, '(1x, 6i4)') scan(s, 'o'), scan(s, 'o', back=.true.), scan(s, 'eb'), &
    scan(string=s, set='eb', back=back), verify(s, ' '), verify(s, ' ', back)
  write (*, '(1x, 6i4)') verify('aabbc', 'ab'), verify('aabbc', 'ab', .true.), &
    verify(s, ' bot'), index('banana', 'an', back), index('banana', 'an', .not. back), &
    index(s, 'xyz', back)

  ! REPEAT of a length known when compiling, and of a count known only now.
  n = 3
  line = repeat('ab', 2) // '|' // repeat(trim(s), n) // '|'
  write (*, '(1x, a)') trim(line)
  write (*, '(1x, i4, " [", a, "]")') len(repeat('xyz', n - 1)), repeat('-', n * 4)

  ! The elemental functions on arrays.
  names = (/ ' fig  ', 'plum  ', '      ' /)
  k = len_trim(names)
  write (*, '(1x, 3i4)') k
  write (*, '(1x, 3i4)') scan(names, 'i'), verify(names, ' ', back=.true.)
  names = adjustr(names)
  write (*, '(1x, 3("[", a, "]"))') names
  write (*, '(1x, 3i4)') index(names, 'l')

  ! TRIM's value as an argument, and a passed-length dummy's own TRIM.
  call show(trim(names(2)))
  call show(trim(names(3)))
  t = 'abc'
  call show(t)

  ! Lengths of zero: a declared one, a substring past its upper bound, and the
  ! functions of an empty string.
  none = 'ignored'
  i = 5
  write (*, '(1x, 4i4)') len(none), len(s(i:i-1)), len(s(i:i-3)), len_trim(none)
  write (*, '(1x, "[", a, "] [", a, "] [", a, "]")') none, s(7:6), repeat(s, 0)
  write (*, '(1x, 4i4, l3)') scan(none, 'a'), verify(none, 'a'), index(none, ''), &
    index('abc', '', back=.true.), none == ''
  write (*, '(1x, "[", a, "]")') adjustl(none) // trim('   ') // adjustr(none)

contains

  subroutine show(text)
    character(len=*), intent(in) :: text
    write (*, '(1x, i4, " [", a, "] [", a, "]")') len(text), text, trim(text) // '<'
  end subroutine show

end program charfns
