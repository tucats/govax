! A linear-equation solver (Phase 104, subtask 9): Gaussian elimination with
! partial pivoting on assumed-shape dummies, the augmented matrix and a row in
! automatic arrays of the system's size, the pivot by MAXLOC of a section, rows
! swapped whole, and back substitution by SUM of sections. The systems are
! allocatable arrays allocated for each size from 2 to 5, their right-hand
! sides MATMUL of a known solution, and each answer checked by its residual;
! then a singular system, and one made a thousand times.
program solver
  implicit none
  interface
    subroutine solve(a, b, x, ok)
      double precision :: a(:, :), b(:), x(:)
      logical :: ok
    end subroutine
  end interface
  double precision, allocatable :: a(:, :), b(:), x(:)
  double precision :: s(2, 2)
  integer :: n, i, j, k, bad
  logical :: ok

  do n = 2, 5
    allocate (a(n, n), b(n), x(n))
    do j = 1, n
      do i = 1, n
        a(i, j) = 1.0d0 / (i + j - 1)
      end do
      a(j, j) = a(j, j) + n
    end do
    x = (/ (dble(i), i = 1, n) /)
    b = matmul(a, x)
    x = 0.0d0
    call solve(a, b, x, ok)
    write (*, '(1x, a, i2, l2, 5f9.5)') 'n =', n, ok, x
    write (*, '(1x, a, l2)') 'residual small:', maxval(abs(matmul(a, x) - b)) < 1.0d-10
    deallocate (a, b, x)
  end do

  s = reshape((/ 1.0d0, 2.0d0, 2.0d0, 4.0d0 /), (/ 2, 2 /))
  allocate (x(2))
  call solve(s, (/ 1.0d0, 2.0d0 /), x, ok)
  write (*, '(1x, a, l2)') 'singular solved:', ok
  deallocate (x)

  bad = 0
  allocate (a(3, 3), b(3), x(3))
  do k = 1, 1000
    a = reshape((/ 4.0d0, 1.0d0, 0.0d0, 1.0d0, 4.0d0, 1.0d0, 0.0d0, 1.0d0, 4.0d0 /), (/ 3, 3 /))
    b = matmul(a, (/ dble(k), 1.0d0, -dble(k) /))
    call solve(a, b, x, ok)
    if (.not. ok .or. abs(x(1) - k) > 1.0d-9 .or. abs(x(3) + k) > 1.0d-9) bad = bad + 1
  end do
  write (*, '(1x, a, i5)') 'bad of 1000:', bad
end program solver

subroutine solve(a, b, x, ok)
  double precision :: a(:, :), b(:), x(:)
  logical :: ok
  double precision :: m(size(a, 1), size(a, 2) + 1), row(size(a, 2) + 1), factor
  integer :: n, k, p, i, at(1)

  n = size(a, 1)
  m(:, 1:n) = a
  m(:, n + 1) = b
  ok = .true.

  do k = 1, n
    at = maxloc(abs(m(k:n, k)))
    p = at(1) + k - 1
    if (abs(m(p, k)) < 1.0d-12) then
      ok = .false.
      return
    end if
    row = m(k, :)
    m(k, :) = m(p, :)
    m(p, :) = row
    do i = k + 1, n
      factor = m(i, k) / m(k, k)
      m(i, k:) = m(i, k:) - factor * m(k, k:)
    end do
  end do

  do k = n, 1, -1
    x(k) = (m(k, n + 1) - sum(m(k, k + 1:n) * x(k + 1:n))) / m(k, k)
  end do
end subroutine solve
