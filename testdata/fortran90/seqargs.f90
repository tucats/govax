! Sections and array expressions given to dummies that aren't assumed-shape
! (Phase 104, subtask 6): sections known to be contiguous, passed where they
! are; strided sections, rows, and reversed sections copied into a temporary and
! back; expressions and vector-subscripted sections, never copied back;
! CHARACTER sections, substrings of them, and a concatenation; an assumed-shape
! array and its sections tested when the program runs, both ways; an
! allocatable array's row; an alternate return after a copy; a function's copy
! back before the next statement; a row of 20,000 elements, its temporary on
! the heap, many times; an assumed-shape array as a format and as an internal
! file; a dynamic string passed beside a copied section.
program seqargs
  implicit none
  interface
    subroutine shaped(w, f, c)
      real :: w(:, :)
      character(len=*) :: f(:), c(:)
    end subroutine
  end interface
  real :: a(4, 3), big(2, 20000), t
  real, allocatable :: al(:, :)
  character(len=4) :: c(6)
  character(len=12) :: f(3), g(5)
  character(len=8) :: rec(4)
  integer :: iv(3), i, j, n
  real, external :: total, bump

  do j = 1, 3
    do i = 1, 4
      a(i, j) = i + 10 * j
    end do
  end do

  ! Contiguous: passed where they are.
  call twice(a(:, 2), 4)
  call twice(a(2:3, 3), 2)
  print '(1x, a, 4f6.1)', 'column 2:', a(:, 2)
  print '(1x, a, 4f6.1)', 'column 3:', a(:, 3)

  ! Strided, a row, reversed: copied in and back.
  call twice(a(1:4:3, 1), 2)
  call twice(a(4, :), 3)
  call twice(a(3:1:-1, 1), 3)
  print '(1x, a, 12f6.1)', 'a:', a

  ! Expressions and vector subscripts: copied in only.
  iv = (/ 3, 1, 2 /)
  call twice(a(1, :) + 0.5, 3)
  call twice(a(1, iv), 3)
  print '(1x, a, 3f6.1)', 'row 1:', a(1, :)
  print '(1x, a, f7.1, f7.1)', 'totals:', total(a(1, :) * 2.0, 3), total(a(2, iv), 3)

  ! CHARACTER sections.
  c = (/ 'abcd', 'efgh', 'ijkl', 'mnop', 'qrst', 'uvwx' /)
  call words(c(2:4), 3)
  call words(c(1:6:2), 3)
  call words(c(5:6)(2:3), 2)
  call words(c(iv), 3)
  call words(c(5:6) // '!', 2)
  print '(1x, 5(a, 1x), a)', c

  ! An assumed-shape array and its sections, tested when the program runs.
  f(1) = '(1x, '
  f(2) = "'fmt:', i3"
  f(3) = ')'
  g = 'junk'
  g(1) = '(1x, '
  g(3) = "'FMT:', i4"
  g(5) = ')'
  rec = 'oldx'
  call shaped(a, f, rec)
  call shaped(a(1:4:3, :), g(1:5:2), rec(4:1:-1))
  print '(1x, 3(a, 1x), a)', (rec(i)(1:4), i = 1, 4)
  print '(1x, a, 12f6.1)', 'a:', a

  ! An allocatable array's row.
  allocate (al(3, 2))
  al = 1.0
  call twice(al(2, :), 2)
  print '(1x, a, 6f5.1)', 'al:', al

  ! A copy back before an alternate return.
  call alt(a(1, :), *10)
  print '(1x, a)', 'not taken'
10 print '(1x, a, 3f7.1)', 'alt:', a(1, :)

  ! A function's copy back, done when its call returns.
  t = bump(a(2, :), 3)
  print '(1x, a, f7.1, 3f7.1)', 'bump:', t, a(2, :)

  ! A row of 20,000 elements, its temporary from the heap each time.
  n = 20000
  big = 1.0
  t = 0.0
  do j = 1, 10
    call twice(big(1, 1:n), n)
    t = t + total(big(2, 1:n), n)
  end do
  print '(1x, a, f12.1, 2f10.1)', 'big:', t, big(1, 1), big(1, n)

  call strings('ab')
end program seqargs

subroutine twice(v, n)
  integer :: n
  real :: v(n)
  v = v * 2.0
end subroutine twice

real function total(v, n)
  integer :: n
  real :: v(n)
  total = sum(v)
end function total

real function bump(v, n)
  integer :: n
  real :: v(n)
  v = v + 1.0
  bump = sum(v)
end function bump

subroutine words(w, n)
  integer :: n
  character(len=*) :: w(n)
  integer :: i
  print '(1x, i2, i3, 1x, 6(a, :, "|"))', n, len(w), (w(i), i = 1, n)
  w(1) = 'ZZZZZZ'
end subroutine words

subroutine shaped(w, f, c)
  real :: w(:, :)
  character(len=*) :: f(:), c(:)
  integer :: k
  character(len=4) :: got(2)
  real, external :: total
  print '(1x, a, f8.1)', 'whole:', total(w, size(w))
  print '(1x, a, f8.1)', 'column:', total(w(:, 2), size(w, 1))
  call twice(w(2, :), size(w, 2))
  write (*, f) size(w)
  read (c, '(a4)') got
  print '(1x, a, 2(1x, a))', 'got:', got
  write (c, '(a, i1)') ('rec', k, k = 1, size(c))
end subroutine shaped

subroutine alt(v, *)
  real :: v(3)
  v(1) = -v(1)
  return 1
end subroutine alt

subroutine strings(s)
  character(len=*) :: s
  real :: x(5)
  integer :: i
  x = (/ (real(i), i = 1, 5) /)
  call pair(s // 'cdef', x(1:5:2))
end subroutine strings

subroutine pair(q, v)
  character(len=*) :: q
  real :: v(3)
  character(len=40) :: t
  t = q // q // q
  print '(1x, a, 1x, a, 3f5.1)', q, t(1:12), v
end subroutine pair
