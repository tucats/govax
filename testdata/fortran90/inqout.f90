! INTENT(OUT) dummy arguments asked about before they are given a value
! (Phase 111, subtask 5a, found by fcalc): SIZE, LBOUND, UBOUND, SHAPE,
! LEN, KIND, DIGITS, and HUGE ask about a dummy's shape, length, or type,
! not its value, which is undefined until the routine gives it one. The
! compiler must say nothing of them.
program inqout
  implicit none
  real :: v(3), w(-1:2, 2)
  character(len=5) :: c
  double precision :: d

  call fill(v)
  print '(1x, 3f6.1)', v
  call corners(w)
  print '(1x, 4f6.1)', w
  call stars(c)
  print '(1x, a)', c
  call biggest(d)
  print '(1x, l1)', d == huge(d)
contains

  ! Each element its index, counted by SIZE.
  subroutine fill(a)
    real, intent(out) :: a(:)
    integer :: k

    do k = 1, size(a)
      a(k) = k
    end do
  end subroutine fill

  ! Each element the sum of its subscripts, from the bounds and the shape.
  subroutine corners(a)
    real, intent(out) :: a(:, :)
    integer :: i, j, extents(2)

    extents = shape(a)
    do j = lbound(a, 2), ubound(a, 2)
      do i = 1, extents(1)
        a(i, j) = i + j
      end do
    end do
  end subroutine corners

  ! As many stars as the text holds.
  subroutine stars(text)
    character(len=*), intent(out) :: text

    text = repeat('*', len(text))
  end subroutine stars

  ! The largest number of the argument's kind, which has the digits asked.
  subroutine biggest(x)
    double precision, intent(out) :: x

    if (kind(x) == kind(0d0) .and. digits(x) > 50) then
      x = huge(x)
    else
      x = 0
    end if
  end subroutine biggest
end program inqout
