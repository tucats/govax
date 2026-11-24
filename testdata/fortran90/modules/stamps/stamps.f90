! stamps.f90: records of stampmod's structure and type, made where the module
! isn't compiled, each starting with the initial values its text carries.
program stamps
  use stampmod
  implicit none
  record /stamp/ s
  type(hollow) :: h

  write (*, '(1x, i3, f6.2, 1x, a, 3l2, i4, 2f6.1)') s.code, s.when, s.tag, s.flags, s.whole, s.both
  write (*, '(1x, 2i7)') s.halves
  last.code = last.code + 1
  write (*, '(1x, i3, 1x, a)') last.code, last.tag
  h%nothing = 'ignored'
  write (*, '(1x, a, i2, i3, 1x, a, a)') '[', len(h%nothing), h%n, h%pair, ']'
  write (*, '(1x, 2a, i3, 1x, 2a)') '[', h, ']'
end program stamps
