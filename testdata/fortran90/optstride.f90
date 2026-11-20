! The optimizer's stepped addresses (Phase 110, subtask 8): an assumed-shape dummy's
! element is A0 plus each subscript times its stride, and in a loop that address is
! stepped by the stride, not computed again. Assumed-shape dummies given whole arrays
! and sections with strides, in one dimension and two, and an array pointer to a section
! with a stride.
program optstride
  implicit none
  interface
    subroutine axpy(y, x, s)
      real, intent(inout) :: y(:)
      real, intent(in) :: x(:)
      real, intent(in) :: s
    end subroutine
    subroutine scale2(m, s)
      real, intent(inout) :: m(:, :)
      real, intent(in) :: s
    end subroutine
  end interface
  real :: u(10), v(10), g(4, 6), big(3000), w(1000)
  real, target :: t(12)
  real, pointer :: q(:)
  integer :: i, j

  u = (/ (real(i), i = 1, 10) /)
  v = 1.0
  call axpy(v, u, 2.0)
  write (*, '(1x, 10f5.1)') v
  call axpy(v(1:10:3), u(2:8:2), 1.0)
  write (*, '(1x, 10f5.1)') v

  g = reshape((/ ((real(10 * i + j), i = 1, 4), j = 1, 6) /), (/ 4, 6 /))
  call scale2(g(2:4:2, 1:6:2), 0.5)
  write (*, '(1x, 4f7.1)') g

  t = (/ (real(i), i = 1, 12) /)
  q => t(3:12:3)
  q = q * q + 1.0
  write (*, '(1x, 12f6.1)') t

  ! What the gain is measured on: every third element, fifty times.
  big = 1.0
  w = 0.0
  do i = 1, 50
    call axpy(w, big(1:3000:3), 1.0)
  end do
  write (*, '(1x, f10.1)') sum(w)
end program

subroutine axpy(y, x, s)
  implicit none
  real, intent(inout) :: y(:)
  real, intent(in) :: x(:)
  real, intent(in) :: s

  y = y + s * x
end subroutine

subroutine scale2(m, s)
  implicit none
  real, intent(inout) :: m(:, :)
  real, intent(in) :: s

  m = m * s
end subroutine
