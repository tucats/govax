! Array temporaries of a size known only when the program runs (Phase 104,
! subtask 4c): assignments through a temporary (a reversal, a shift read both
! ways), a WHERE whose mask is kept, a vector subscript of the target's own
! array, constructors of a run-time size in expressions and in conditions,
! CHARACTER arrays of a run-time length through temporaries, a CHARACTER
! function's value and a concatenation of a run-time length in array
! expressions, MERGE of run-time lengths, a temporary past 64 KB (the heap),
! and a statement in a loop that mustn't grow the stack.
program rtemps
  implicit none
  integer :: n, i, k
  n = 6
  call reals(n)
  call masks(5)
  call vectors(n)
  call ctors(n)
  call chars(4, 3)
  call bigone(20000)
  call loop(100)
contains
  subroutine reals(m)
    integer :: m
    real :: a(m)
    a = (/ (real(i), i = 1, m) /)
    a(m:1:-1) = a
    print '(6F5.1)', a
    a(2:m) = a(1:m-1) + a(m:2:-1)
    print '(6F5.1)', a
  end subroutine

  subroutine masks(m)
    integer :: m
    integer :: a(m), b(m)
    a = (/ 1, -2, 3, -4, 5 /)
    where (a > 0)
      a = -a
      b = 1
    elsewhere
      a = 0
      b = 2
    end where
    print *, a
    print *, b
  end subroutine

  subroutine vectors(m)
    integer :: m
    integer :: v(m)
    v = (/ 2, 3, 4, 5, 6, 1 /)
    v(v) = (/ (10 * i, i = 1, m) /)
    print *, v
    v = (/ 2, 3, 4, 5, 6, 1 /)
    v(v(1:m)) = v + 100
    print *, v
  end subroutine

  subroutine ctors(m)
    integer :: m, j
    integer :: w(m)
    print *, sum((/ (i, i = 1, m) /)), maxval((/ (i * i, i = -m, 2) /))
    w = 2 * (/ (i, i = m, 1, -1) /) + 1
    print *, w
    if (sum((/ (i, i = 1, m) /)) == 21) print *, 'sum is 21'
    select case (sum((/ (i, i = 1, m) /)))
    case (21)
      print *, 'case 21'
    case default
      print *, 'no case'
    end select
    j = 0
    do while (count((/ (i <= j, i = 1, m) /)) < 3)
      j = j + 1
    end do
    print *, 'j is', j
  end subroutine

  subroutine chars(m, l)
    integer :: m, l
    character(len=l) :: c(m), s
    character(len=2) :: t(m)
    c = (/ 'abc', 'def', 'ghi', 'jkl' /)
    c(m:1:-1) = c
    print *, c
    s = 'xyz'
    t = c(1)(2:3) // 'Q'
    print *, t
    c = (/ s, c(2:m) /)
    print *, c
    c(2:3) = (/ (c(5 - i), i = 1, 2) /)
    print *, c
    c = upper(s) // c
    print *, c
    print *, c(1:2) // c(1)(1:l-1)
    c = (/ 'aaa', 'bbb', 'ccc', 'ddd' /)
    c = merge(c, c(m:1:-1), (/ .true., .false., .true., .false. /))
    print *, c
  end subroutine

  character(len=3) function upper(x)
    character(len=*) :: x
    upper = 'UP' // x(1:1)
  end function

  subroutine bigone(m)
    integer :: m
    integer :: b(m)
    b = 1
    b(m:1:-1) = b + (/ (i, i = 1, m) /)
    print *, b(1), b(m), sum(b)
  end subroutine

  subroutine loop(m)
    integer :: m, j
    real :: a(m)
    a = (/ (real(i), i = 1, m) /)
    do j = 1, 6001
      a(m:1:-1) = a
    end do
    print '(2F7.1)', a(1), a(m)
  end subroutine
end program rtemps
