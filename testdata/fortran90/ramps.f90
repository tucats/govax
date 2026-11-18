! Array-valued functions (Phase 104, subtask 7b): values whose shape is known
! only when the program runs, which the function makes. An external one
! through an interface body (a ramp of N values); an internal one whose
! bounds are SIZE of an assumed-shape dummy; one of rank two; one whose lower
! bound isn't 1; one of its host's variable; a recursive one with RESULT that
! uses its own value; values of no elements; values in expressions,
! reductions, WHERE, IF, output lists and their implied DO lists, as
! arguments to old and assumed-shape dummies, assigned to sections, and
! called a thousand times.
program ramps
  implicit none
  interface
    function ramp(n)
      integer :: n
      real :: ramp(n)
    end function
    recursive function counts(n) result(r)
      integer :: n, r(n)
    end function
    subroutine shaped(v)
      real :: v(:, :)
    end subroutine
  end interface
  real :: a(5), b(6), m(2, 3), total
  integer :: i, k(4), width

  a = ramp(5)
  write (*, '(1x, a, 5f5.1)') 'ramp: ', a
  write (*, '(1x, a, 5f5.1)') 'plus: ', ramp(5) + a
  write (*, '(1x, a, f6.1, i3)') 'sum:  ', sum(ramp(4)), size(ramp(7))
  write (*, '(1x, a, i3, f5.1)') 'none: ', size(ramp(0)), sum(ramp(-2))
  write (*, '(1x, a, 4f6.1)') 'loop: ', (sum(ramp(i)), i = 1, 4)

  b = 0.0
  b(2:6) = twice(a)
  write (*, '(1x, a, 6f5.1)') 'twice:', b
  write (*, '(1x, a, 2i3)') 'lbnd: ', lbound(twice(a)), ubound(twice(a))

  m = grid(2, 3)
  write (*, '(1x, a, 6f5.1)') 'grid: ', m
  write (*, '(1x, a, f5.1, 2i3)') 'max:  ', maxval(grid(2, 3)), shape(grid(4, 1))

  write (*, '(1x, a, 3f5.1)') 'shift:', shifted(3)

  width = 3
  write (*, '(1x, a, 3f5.1)') 'host: ', wide()
  width = 1
  write (*, '(1x, a, 3f5.1)') 'host: ', wide(), -wide()

  k = counts(4)
  write (*, '(1x, a, 4i5)') 'count:', k

  where (ramp(5) > 2.5)
    a = 0.0
  end where
  write (*, '(1x, a, 5f5.1)') 'where:', a
  if (any(ramp(3) == 3.0)) write (*, '(1x, a)') 'any:   yes'

  call old(ramp(3), 3)
  call shaped(grid(2, 2))

  total = 0.0
  do i = 1, 1000
    total = total + sum(ramp(mod(i, 7)))
  end do
  write (*, '(1x, a, f9.1)') 'total:', total

contains

  function twice(v)
    real :: v(:)
    real :: twice(size(v))
    twice = 2.0 * v
  end function

  function grid(r, c)
    integer :: r, c
    real :: grid(r, c)
    integer :: i, j
    do j = 1, c
      do i = 1, r
        grid(i, j) = 10.0 * i + j
      end do
    end do
  end function

  function shifted(n)
    integer :: n
    real :: shifted(0:n - 1)
    integer :: i
    shifted = (/ (real(i), i = 0, n - 1) /)
  end function

  function wide()
    real :: wide(width)
    wide = real(width)
  end function

end program ramps

function ramp(n)
  integer :: n
  real :: ramp(n)
  integer :: i
  do i = 1, n
    ramp(i) = i
  end do
end function

recursive function counts(n) result(r)
  integer :: n, r(n)
  r(n) = n
  if (n > 1) r(1:n - 1) = counts(n - 1) * 10
end function

subroutine old(v, n)
  integer :: n
  real :: v(n)
  write (*, '(1x, a, 3f5.1)') 'old:  ', v
end subroutine

subroutine shaped(v)
  real :: v(:, :)
  write (*, '(1x, a, 4f5.1, 2i3)') 'shape:', v, shape(v)
end subroutine
