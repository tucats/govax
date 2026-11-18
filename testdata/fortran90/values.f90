! Array-valued functions (Phase 104, subtask 7a): values of a shape known when
! compiling. An external one through an interface body (a cross product); an
! internal one of an assumed-shape dummy (a unit vector) and one of rank two;
! a recursive one with RESULT (a pair of Fibonacci numbers); a CHARACTER one;
! values in expressions, reductions, MATMUL, WHERE, IF, output lists, as
! arguments to old and assumed-shape dummies, assigned to their own argument,
! through a dummy procedure and a generic name, and called a thousand times.
program values
  implicit none
  interface
    function cross(a, b)
      real :: a(3), b(3), cross(3)
    end function
    recursive function fib(n) result(r)
      integer :: n, r(2)
    end function
    function iota(n)
      integer :: n, iota(4)
    end function
  end interface
  interface pair
    function ipair(i)
      integer :: i, ipair(2)
    end function
    function rpair(x)
      real :: x, rpair(2)
    end function
  end interface
  real :: x(3), y(3), z(3), m(2, 3), total
  integer :: k(2), i
  character(len=4) :: tags(2)

  x = (/ 1.0, 0.0, 0.0 /)
  y = (/ 0.0, 1.0, 0.0 /)
  z = cross(x, y)
  write (*, '(1x, a, 3f6.2)') 'cross:', z
  write (*, '(1x, a, 3f6.2)') 'back: ', cross(y, x) + 1.0
  write (*, '(1x, a, f6.2)') 'dot:  ', sum(cross(x, y) * z)

  z = (/ 3.0, 0.0, 4.0 /)
  write (*, '(1x, a, 3f6.2)') 'unit: ', unit(z)
  z = unit(z)
  write (*, '(1x, a, 3f6.2)') 'self: ', z

  k = fib(20)
  write (*, '(1x, a, 2i8)') 'fib:  ', k
  write (*, '(1x, a, 4i4)') 'iota: ', iota(3)
  write (*, '(1x, a, i4)') 'max:  ', maxval(iota(5) - 2 * iota(1))

  m = grid(10.0)
  write (*, '(1x, a, 6f6.1)') 'grid: ', m
  write (*, '(1x, a, 2f6.1)') 'mul:  ', matmul(grid(1.0), (/ 1.0, 1.0, 1.0 /))

  x = (/ -1.0, 2.0, -3.0 /)
  where (unit(x) > 0.0)
    y = 1.0
  elsewhere
    y = -1.0
  end where
  write (*, '(1x, a, 3f6.2)') 'where:', y
  if (all(iota(2) > 1)) write (*, '(1x, a)') 'all:   yes'

  tags = labels('ab')
  write (*, '(1x, a, 2(1x, a))') 'tags: ', tags, labels('xyz')

  call old(cross(x, y))
  call shaped(unit(x))
  call apply(ipair)
  k = pair(7)
  write (*, '(1x, a, 2i4, 2f6.2)') 'pair: ', k, pair(0.5)

  total = 0.0
  do i = 1, 1000
    total = total + sum(cross(x, (/ 0.0, 0.0, real(i) /)))
  end do
  write (*, '(1x, a, f12.1)') 'total:', total

contains

  function unit(v)
    real :: v(:), unit(3)
    unit = v / sqrt(sum(v * v))
  end function

  function grid(s)
    real :: s, grid(2, 3)
    integer :: i, j
    do j = 1, 3
      do i = 1, 2
        grid(i, j) = s * i + j
      end do
    end do
  end function

  function labels(s)
    character(len=*) :: s
    character(len=4) :: labels(2)
    labels(1) = s // '-'
    labels(2) = '+' // s
  end function

  subroutine old(v)
    real :: v(3)
    write (*, '(1x, a, 3f6.2)') 'old:  ', v
  end subroutine

  subroutine shaped(v)
    real :: v(:)
    write (*, '(1x, a, i2, 3f6.2)') 'shape:', size(v), v
  end subroutine
end program values

function cross(a, b)
  real :: a(3), b(3), cross(3)
  cross(1) = a(2) * b(3) - a(3) * b(2)
  cross(2) = a(3) * b(1) - a(1) * b(3)
  cross(3) = a(1) * b(2) - a(2) * b(1)
end function

recursive function fib(n) result(r)
  integer :: n, r(2)
  if (n <= 1) then
    r = (/ 0, 1 /)
  else
    r = fib(n - 1)
    r = (/ r(2), r(1) + r(2) /)
  end if
end function

function iota(n)
  integer :: n, iota(4), i
  iota = (/ (n * i, i = 1, 4) /)
end function

function ipair(i)
  integer :: i, ipair(2)
  ipair = (/ i, -i /)
end function

function rpair(x)
  real :: x, rpair(2)
  rpair = (/ x, x * x /)
end function

subroutine apply(fn)
  interface
    function fn(i)
      integer :: i, fn(2)
    end function
  end interface
  write (*, '(1x, a, 2i4)') 'apply:', fn(3) * 10
end subroutine
