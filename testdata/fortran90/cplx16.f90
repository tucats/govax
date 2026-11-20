! cplx16.f90: Fortran 90's COMPLEX(16) (docs/planning/PHASE-100 - Fortran 90
! declarations and kinds.md, subtask 7), a type VAX FORTRAN lacks: two
! H_floating parts. Constants of kind 16, an initial value, sums,
! differences, products, quotients (FTNRTL's FTN$DIVCH), negation, equality,
! conversions to and from COMPLEX(8), REAL(16), and INTEGER, CMPLX with
! KIND=16, REAL, AIMAG, and CONJG, KIND and PRECISION, arrays and their
! elements with a subscript, a dummy array, and a function of the type. The
! values are printed by their parts, as they were before the type's I/O
! (cplx16io.f90, Phase 109).
program cplx16
  implicit none
  integer, parameter :: q = 16
  complex(q) :: z = (1.5_q, -2.0_q), w, a(4)
  complex(8) :: d
  real(q) :: h
  integer :: i

  write (*, '(1x, a, 2i4)') 'KIND, PRECISION:', kind(z), precision(z)

  w = z + (0.5_q, 1.0_q)
  call show('Z + (0.5, 1)', w)
  call show('Z - W', z - w)
  call show('Z * W', z * w)
  call show('Z / W', z / w)
  call show('-Z', -z)
  call show('CONJG(Z)', conjg(z))

  ! One third, to H_floating's 33 digits.
  w = (1.0_q, 0.0_q) / (3.0_q, 0.0_q)
  write (*, '(1x, a, f36.33)') 'One third:', real(w)

  ! A quotient folded by the compiler and one computed: the same.
  w = (2.0_q, 1.0_q)
  write (*, '(1x, a, l2)') 'Folded = computed:', (1.0_q, 2.0_q) / (2.0_q, 1.0_q) == z / z * (1.0_q, 2.0_q) / w

  ! Conversions.
  d = z
  call show('COMPLEX(8)', cmplx(d, kind=q))
  h = z
  write (*, '(1x, a, f8.3)') 'REAL(16):', h
  write (*, '(1x, a, i4)') 'INTEGER:', int(z)
  call show('CMPLX', cmplx(h, 3, kind=q))
  call show('From an INTEGER', cmplx(7, kind=q))

  ! An array, its elements, and a function of COMPLEX(16) values.
  do i = 1, 4
    a(i) = cmplx(i, -i, kind=q)
  end do
  call turn(a, 4)
  do i = 1, 4
    call show('A(I)', a(i))
  end do
  write (*, '(1x, a, 2l2)') 'Equal:', a(1) == (1.0_q, 1.0_q), a(2) /= a(3)
end program cplx16

! Each part of a value, with 6 decimals.
subroutine show(what, x)
  implicit none
  character(len=*), intent(in) :: what
  complex(16), intent(in) :: x
  write (*, '(1x, a, t20, 2f12.6)') what, real(x), aimag(x)
end subroutine show

! Each element turned a quarter to the left.
subroutine turn(x, n)
  implicit none
  integer, intent(in) :: n
  complex(16), intent(inout) :: x(n)
  complex(16), external :: rotate
  integer :: i
  do i = 1, n
    x(i) = rotate(x(i))
  end do
end subroutine turn

complex(16) function rotate(x)
  implicit none
  complex(16), intent(in) :: x
  rotate = x * (0.0_16, 1.0_16)
end function rotate
