! Assumed-shape dummies given values (Phase 104, subtask 5b): an array
! expression, a vector-subscripted section, a section in parentheses, an
! elemental function's value, and a constructor, each computed into a
! temporary and passed by a descriptor of it; one of a size known only when the
! program runs; a dummy's own lower bound on an expression; substrings of a
! section described where they are, and of a vector-subscripted section and a
! concatenation through temporaries; OPTIONAL assumed-shape and explicit-shape
! dummies passed on, absent or not; a generic name whose specifics have
! assumed-shape dummies of ranks 1 and 2; a function given an expression inside
! an expression, a thousand times in a loop.
program shapeargs
  implicit none
  interface norm
    real function norm1(v)
      real, intent(in) :: v(:)
    end function
    real function norm2(m)
      real, intent(in) :: m(:, :)
    end function
  end interface
  real :: a(6), b(2, 3), s
  integer :: iv(3), i, j, n
  character(len=4) :: c(5)

  a = (/ (real(i), i = 1, 6) /)
  iv = (/ 6, 1, 3 /)
  do j = 1, 3
    do i = 1, 2
      b(i, j) = i + 10 * j
    end do
  end do
  c = (/ 'abcd', 'efgh', 'ijkl', 'mnop', 'qrst' /)
  n = 4

  call show(a * 2.0)
  call show(a(iv))
  call show((a(2:6:2)))
  call show(sqrt(a(1:4)) + a(3:6))
  call show((/ 9.0, 8.0 /))
  call show(a(1:n) * a(n:1:-1))
  call show(a(n:1:-2) + 0.5)
  call lowest(a + 1.0)

  call words(c(2:4)(2:3))
  call words(c(5:1:-2))
  call words(c(iv(2:3))(3:4))
  call words(c(1:3) // '!')

  call outer()
  call outer(a(4:6))
  call outer(a * 0.5)
  call outer3()
  call outer3(a)

  print '(1x, a, 2f8.3)', 'norm:', norm(a(1:3) - 1.0), norm(b * 0.1)
  print '(1x, a, 2f8.3)', 'norm:', norm(b(:, 2)), norm(b(:, 2:3) - b(:, 1:2))

  s = total(a * 2.0) + total(a(iv))
  print '(1x, a, f9.1)', 'total:', s
  s = 0.0
  do j = 1, 1000
    s = s + total(a(1:n) * j)
  end do
  print '(1x, a, f11.1)', 'loop:', s

contains

  subroutine show(v)
    real :: v(:)
    print '(1x, i2, i3, 6f7.2)', size(v), lbound(v, 1), v
  end subroutine show

  subroutine lowest(v)
    real :: v(0:)
    print '(1x, a, 2i3, 2f6.1)', 'lowest:', lbound(v, 1), ubound(v, 1), v(0), v(ubound(v, 1))
  end subroutine lowest

  subroutine words(w)
    character(len=*) :: w(:)
    print '(1x, i2, i3, 1x, 5(a, :, "|"))', size(w), len(w), w
  end subroutine words

  subroutine outer(o)
    real, optional :: o(:)
    call inner(o)
  end subroutine outer

  subroutine outer3(o)
    real, optional :: o(3)
    call inner(o)
  end subroutine outer3

  subroutine inner(o)
    real, optional :: o(:)
    if (present(o)) then
      print '(1x, a, i2, f7.2)', 'present:', size(o), sum(o)
    else
      print '(1x, a)', 'absent'
    end if
  end subroutine inner

  real function total(v)
    real :: v(:)
    total = sum(v)
  end function total
end program shapeargs

real function norm1(v)
  real, intent(in) :: v(:)
  norm1 = sqrt(sum(v * v))
end function norm1

real function norm2(m)
  real, intent(in) :: m(:, :)
  norm2 = sum(m) / size(m)
end function norm2
