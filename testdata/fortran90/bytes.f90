! bytes.f90: Fortran 90's INTEGER(1) (docs/planning/PHASE-100 - Fortran 90
! declarations and kinds.md, subtask 5), a type VAX FORTRAN lacks: byte
! arithmetic, mixed operations with the wider integers and REAL, constants of
! kind 1, the generic functions of INTEGER(1) arguments (done in INTEGER(2)
! and converted back), powers, a DO loop and subscripts of kind 1, arrays,
! arguments, a function of kind 1, and formatted, list-directed, and internal
! I/O of such values.
program bytes
  implicit none
  integer, parameter :: b = 1
  integer(kind=b) :: i, j, k, n
  integer(b) :: list(5)
  integer(2) :: w
  integer :: l
  real :: r
  integer(b), external :: twice
  character(len=12) :: text
  data list /1_b, -2_b, 3_b, -4_b, 5_b/

  i = 100_b
  j = -27_b
  k = i + j
  write (*, '(1x, 3i5)') i, j, k
  k = i - j - 120_b
  n = k * 2_b
  write (*, '(1x, 2i5)') k, n
  n = i / 7_b
  write (*, '(1x, i5, i5)') n, mod(i, 7_b)

  ! An INTEGER(1) beside a wider type is done in that type.
  w = i * 300_2
  l = j * 100000
  r = i / 8.0
  write (*, '(1x, i7, i10, f8.3)') w, l, r

  ! The generic functions keep the arguments' kind.
  write (*, '(1x, 6i5)') abs(j), max(i, j), min(i, j), iand(i, j), ior(i, j), ieor(i, j)
  write (*, '(1x, 3i5)') ishft(1_b, 6_b), not(i), sign(5_b, j)
  write (*, '(1x, 2i5)') int(j), dim(i, j)
  write (*, '(1x, i5)') 3_b ** 4_b

  ! Comparisons and a loop with a byte variable.
  if (j < i .and. i == 100_b) write (*, '(1x, a)') 'compared'
  l = 0
  do k = -3_b, 120_b, 9_b
    l = l + k
  end do
  write (*, '(1x, 2i6)') k, l

  ! Arrays, subscripts, arguments, and a function of kind 1.
  do k = 1_b, 5_b
    list(k) = list(k) * k
  end do
  write (*, '(1x, 5i4)') list
  call bump(list(2), 10_b)
  write (*, *) list
  write (*, '(1x, i4)') twice(list(5))

  ! Editing: I, O, Z, and list-directed, both ways.
  write (*, '(1x, i4.3, 1x, o3, 1x, z2)') i, i, i
  write (text, '(i4, 1x, i4)') j, -127_b - 1_b
  read (text, '(i4, 1x, i4)') i, n
  write (*, *) i, n
  read (text, *) j, k
  write (*, *) j, k
end program bytes

! bump adds a byte to a byte.
subroutine bump(x, by)
  integer(1), intent(inout) :: x
  integer(1), intent(in) :: by
  x = x + by
end subroutine bump

! twice is its argument doubled, a byte.
integer(1) function twice(x)
  integer(1), intent(in) :: x
  twice = x + x
end function twice
