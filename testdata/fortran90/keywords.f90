! keywords.f90: the intrinsic procedures' table (docs/planning/PHASE-108 -
! Fortran 90 intrinsic procedures.md, subtask 2): keyword arguments to the
! old functions by the standard's names, ISHFTC's SIZE left out, the bit
! functions of I's kind, ACHAR and IACHAR, SELECTED_INT_KIND and
! SELECTED_REAL_KIND of values, and MVBITS as the standard has it: of any
! integer expression, by keyword, and elemental.
program keywords
  implicit none
  integer :: i, j, n
  integer(2) :: w
  integer(1) :: b
  integer :: a(4), c(4)
  integer(2) :: wa(3)
  integer :: d(4), p(4), r(4)
  data d /1, 4, 7, 40/, p /6, 15, 30, 40/, r /37, 37, 5000, 5000/

  ! Keywords, in any order after those by position.
  i = 12
  j = 10
  write (*, '(1x, 4i6)') iand(j=i, i=j), mod(p=5, a=17), max(a2=3, a1=9), sign(b=-1, a=4)
  write (*, '(1x, 3i6)') ishft(shift=2, i=5), ibits(i=255, pos=2, len=3), &
    index(substring='C', string='ABCABC')
  write (*, '(1x, f8.4, 2i6)') atan2(x=1.0, y=0.0), ichar(c='A'), index('ABCABC', 'C', back=.false.)

  ! ISHFTC's SIZE is the bits of I's kind.
  write (*, '(1x, 2i12)') ishftc(1, -1), ishftc(i=3, shift=1, size=2)
  w = 1_2
  write (*, '(1x, 2i8)') ishftc(w, -1), ishftc(w, 4)
  b = 1_1
  write (*, '(1x, i6)') ishftc(b, -1)
  b = -1_1
  write (*, '(1x, i6)') ishft(b, -4)

  ! The bit functions are of I's kind.
  w = -1_2
  j = -12
  write (*, '(1x, 3i8)') ibits(w, 12, 4), ishft(w, j), ibclr(w, 15)
  write (*, '(1x, 4i3)') kind(ibits(w, 12, 4)), kind(ishftc(w, 1)), kind(ibclr(w, 0)), kind(ishft(b, 1))

  ! ACHAR and IACHAR.
  write (*, '(1x, a, i5)') achar(72) // achar(105), iachar('z')

  ! SELECTED_INT_KIND and SELECTED_REAL_KIND of values (gfortran's
  ! INTEGER(8) and INTEGER(16) would take 10 or 19 digits).
  do n = 1, 4
    write (*, '(1x, 2i4)') d(n), selected_int_kind(d(n))
  end do
  do n = 1, 4
    write (*, '(1x, 5i5)') p(n), r(n), selected_real_kind(p(n)), selected_real_kind(r=r(n)), &
      selected_real_kind(p(n), r(n))
  end do

  ! MVBITS of a constant, of an expression, and by keyword.
  i = 0
  call mvbits(7, 0, 3, i, 4)
  j = 255
  call mvbits(from=i+1, frompos=0, len=4, to=j, topos=0)
  w = 0
  call mvbits(-1_2, 0, 8, w, 4)
  b = 0
  call mvbits(3_1, 0, 2, b, 6)
  write (*, '(1x, 4i6)') i, j, w, b

  ! MVBITS on arrays, an element at a time.
  a = (/1, 2, 3, 4/)
  c = 0
  call mvbits(a, 0, 3, c, (/0, 4, 8, 12/))
  write (*, '(1x, 4i7)') c
  call mvbits(a, 0, 2, a, 2)
  write (*, '(1x, 4i7)') a
  wa = 0
  call mvbits(int(a(1:3), 2), 0, 4, wa, 0)
  write (*, '(1x, 3i7)') wa
  call mvbits(1, 0, 1, c(4:1:-1), (/0, 1, 2, 3/))
  write (*, '(1x, 4i7)') c
end program keywords
