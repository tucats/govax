! The optimizer's fused nests (Phase 110, subtask 8): array assignments one after
! the other, each reading the one before at the same element, done in one nest of
! loops, three of them, in two dimensions, and on explicit-shape dummies whose bounds
! have the same text; a WHERE construct's arms in one nest testing the mask once.
! And the cases that must stay apart, each made so that a nest shared wrongly would
! print other numbers: a section read one element ahead of where the statement before
! wrote it, an element of the array the statement before assigned read as a scalar,
! and a WHERE whose assignments change what its mask reads.
program optfuse
  implicit none
  integer, parameter :: n = 8
  real :: a(n), b(n), c(n), d(n), m(3, 4), p(3, 4)
  real :: x(1000), y(1000), z(1000)
  integer :: i

  b = (/ (real(i), i = 1, n) /)

  ! One nest for three statements.
  a = b + 1.0
  c = a * 2.0
  d = c - b
  write (*, '(1x, a, 8f6.1)') 'a', a
  write (*, '(1x, a, 8f6.1)') 'c', c
  write (*, '(1x, a, 8f6.1)') 'd', d

  ! A(2:N) is read after the statement before wrote it: two nests.
  a(1:n-1) = b(2:n) * 10.0
  c(1:n-1) = a(2:n)
  write (*, '(1x, a, 8f6.1)') 'a', a
  write (*, '(1x, a, 8f6.1)') 'c', c

  ! A(N), a scalar, is read after the statement before set it: two nests.
  a = b
  c = a(n) + b
  write (*, '(1x, a, 8f6.1)') 'c', c

  ! A WHERE construct's arms, one nest.
  b = (/ (real(i - 4), i = 1, n) /)
  where (b > 0.0)
    a = b * b
    c = a + 1.0
  elsewhere
    a = 1.0 - b
    c = 0.0
  end where
  write (*, '(1x, a, 8f6.1)') 'a', a
  write (*, '(1x, a, 8f6.1)') 'c', c

  ! The assignments change what the mask reads: the mask first, into a temporary.
  where (a > 2.0)
    a = 0.0
    c = 1.0
  end where
  write (*, '(1x, a, 8f6.1)') 'a', a
  write (*, '(1x, a, 8f6.1)') 'c', c

  ! Two dimensions.
  m = reshape((/ (real(i), i = 1, 12) /), (/ 3, 4 /))
  p = m + 1.0
  m = p * m
  write (*, '(1x, a, 6f7.1 / 1x, 6f7.1)') 'm', m

  ! Explicit-shape dummies of the same bounds.
  call scaled(b, c, d, n)
  write (*, '(1x, a, 8f6.1)') 'c', c
  write (*, '(1x, a, 8f6.1)') 'd', d

  ! What the gain is measured on: three statements, one nest, fifty times.
  x = 1.0
  do i = 1, 50
    y = x + 1.0
    z = y * x
    x = z - 1.0
  end do
  write (*, '(1x, 3f10.1)') sum(x), sum(y), sum(z)
end program

subroutine scaled(x, y, z, k)
  implicit none
  integer, intent(in) :: k
  real, intent(in) :: x(k)
  real, intent(out) :: y(k), z(k)

  y = x * 3.0
  z = y + x
end subroutine
