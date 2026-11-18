! CHARACTER lengths that aren't constant (Phase 104, subtask 4b): a dummy
! argument's, its actual argument's first N characters, whatever N becomes
! after entry; automatic scalars and arrays of the subprogram's own, in both
! forms, CHARACTER(LEN=N) and CHARACTER*(N), from a dummy argument, a common
! block, LEN, and MAX; whole, by element, and by substring, in output lists,
! passed to passed-length dummies, reached by an internal procedure; lengths
! of zero and less; one at each level of a recursion; an ENTRY with the
! dummies in another order; and storage given back at each return, so that ten
! thousand calls don't use up the stack.
program autolen
  implicit none
  integer :: m
  common /sizes/ m
  character(len=12) :: line
  character(len=4) :: rows(3)
  integer :: i, k, total, depth

  line = 'ABCDEFGHIJKL'
  k = 5
  call head(line, k)
  print '(1x, a, a, i3)', 'line now: ', line, k
  k = 0
  call head(line, k)
  k = -3
  call head(line, k)

  call pad('VAX', 3)
  call pad('', 2)

  m = 6
  call shared
  m = 0
  call shared

  rows = (/ 'abcd', 'efgh', 'ijkl' /)
  call tops(rows, 2, 3)
  print '(1x, a, 3(1x, a))', 'rows:', rows

  call words(3)

  print '(1x, a, i4)', 'depth:', depth(4)

  call pair(2, 5)
  call pair2(4, 1)

  m = 1000
  total = 0
  do i = 1, 10000
    call churn(total)
  end do
  print '(1x, a, i8)', 'churned:', total
end program

! A dummy whose length is N: the first N characters of the actual argument.
subroutine head(s, n)
  implicit none
  integer :: n
  character(len=n) :: s

  print '(1x, a, i3, 3a)', 'head:', len(s), ' [', s, ']'
  n = 99
  if (len(s) > 0) then
    s(1:1) = '*'
    s = s // '?'
  end if
  print '(1x, a, i3)', 'still:', len(s)
end subroutine

! Automatic scalars: one of LEN(C)+N, one of the old form.
subroutine pad(c, n)
  implicit none
  character(len=*) :: c
  integer :: n
  character(len=len(c)+n) :: t
  character*(2*n) :: u

  t = c
  u = '<>'
  t(len(t):) = '!'
  print '(1x, a, 2i3, 5a)', 'pad:', len(t), len(u), ' [', t, '] [', u, ']'
  call show(t // u)
end subroutine

subroutine show(s)
  implicit none
  character(len=*) :: s

  print '(1x, a, i3, 3a)', 'show:', len(s), ' [', s, ']'
end subroutine

! Lengths from a common block, MAX, and a zero; an array of them; a host's.
subroutine shared
  implicit none
  integer :: m
  common /sizes/ m
  character(len=m) :: a
  character(len=max(m - 4, 0)) :: b(3)
  integer :: k

  a = 'common'
  do k = 1, 3
    b(k) = char(ichar('0') + k) // char(ichar('0') + k) // '...'
  end do
  print '(1x, a, 2i3, 5a)', 'shared:', len(a), len(b), ' [', a, '] [', b(2), ']'
  print '(1x, 5a)', 'all b: [', b, ']'
  call both
contains
  subroutine both
    print '(1x, a, i3, 3a)', 'inner:', len(a // b(3)), ' [', a // b(3), ']'
  end subroutine
end subroutine

! A dummy array whose elements are N long, and an automatic array of rows.
subroutine tops(rows, n, k)
  implicit none
  integer :: n, k
  character(len=n) :: rows(k)
  character(len=n+1) :: copy(k)
  integer :: i

  do i = 1, k
    copy(i) = rows(i) // '.'
  end do
  rows(2) = 'xy'
  print '(1x, a, i3, 3(1x, a))', 'tops:', len(rows), rows
  print '(1x, a, i3, 3(1x, a))', 'copy:', len(copy), copy
  call each(copy, k)
end subroutine

subroutine each(v, k)
  implicit none
  integer :: k
  character(len=*) :: v(k)

  print '(1x, a, 2i3, 1x, a)', 'each:', len(v), k, v(k)
end subroutine

! An automatic array whose bounds and length are both N.
subroutine words(n)
  implicit none
  integer :: n
  character(len=n) :: w(n)
  integer :: i, j

  do i = 1, n
    do j = 1, n
      w(i)(j:j) = char(ichar('a') + i - 1)
    end do
  end do
  w(2)(2:2) = '-'
  print '(1x, a, i3, 3(1x, a))', 'words:', len(w(1)), w
end subroutine

! One at each level of a recursion.
recursive integer function depth(n) result(r)
  implicit none
  integer :: n
  character(len=n) :: s

  s = '##########'
  if (n > 1) then
    r = depth(n - 1)
  else
    r = 0
  end if
  r = r + len(s) + index(s, '#')
end function

! An ENTRY with the dummies in another order.
subroutine pair(n, k)
  implicit none
  integer :: n, k
  character(len=n) :: a
  character(len=k) :: b

  a = 'pqrstu'
  b = 'pqrstu'
  print '(1x, a, 2i3, 4a)', 'pair: ', len(a), len(b), ' ', a, ' ', b
  return

  entry pair2(k, n)
  a = 'vwxyz'
  b = 'vwxyz'
  print '(1x, a, 2i3, 4a)', 'pair2:', len(a), len(b), ' ', a, ' ', b
end subroutine

subroutine churn(total)
  implicit none
  integer :: total
  integer :: m
  common /sizes/ m
  character(len=m) :: big

  big = 'x'
  total = total + len(big) / 1000 + index(big, ' ') / 2
end subroutine
