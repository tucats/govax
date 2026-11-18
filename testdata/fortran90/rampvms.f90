! Array-valued functions of run-time shape, VMS's side (Phase 104, subtask
! 7b): LIB$STAT_VM's count of bytes, unchanged by thousands of values made on
! the heap by the function and given back by the caller: in assignments, in
! an implied DO list that calls the function at each pass, in a statement's
! own branch (a logical IF's, an arithmetic IF's, a computed GO TO's), in a
! CALL with alternate returns, and in a recursive function's references to
! itself; values of 20,000 elements, past the stack's limit for
! temporaries; and CHARACTER values of a length known only at run time, a
! dynamic string or an array (Phase 104, subtask 7c), one in an implied DO
! list. A dynamic string comes from LIB$SGET1_DD's own zone, which the count
! doesn't include; the arrays' blocks are counted. The first count is taken after the loop's first pass, which
! makes what stays: the heap's zones, at its first block, and the internal
! WRITE's format, kept for its later passes.
program rampvms
  implicit none
  interface
    function ramp(n)
      integer :: n
      real :: ramp(n)
    end function
    recursive function tri(n) result(r)
      integer :: n, r(n)
    end function
    function blanks(n)
      integer :: n
      character(len=n) :: blanks
    end function
    function labels(n, w)
      integer :: n, w
      character(len=w) :: labels(n)
    end function
  end interface
  integer :: before, after, j, k, hits, chars
  real :: x(20000), total
  character(len=80) :: line, text

  hits = 0
  chars = 0
  total = 0.0
  do j = 1, 500
    x(1:j) = ramp(j)
    total = total + x(j)
    write (line, '(10f6.0)') (sum(ramp(mod(j + k, 9))), k = 1, 10)
    if (any(ramp(mod(j, 5)) > 3.0)) hits = hits + 1
    if (size(ramp(mod(j, 3))) - 1) 10, 20, 20
10  hits = hits + 1
20  go to (30, 30, 40) size(ramp(mod(j, 3) + 1))
30  hits = hits + 1
40  call alt(ramp(2), *50)
    hits = -1000
50  if (sum(tri(mod(j, 6) + 1)) < 0) hits = -1000
    if (mod(j, 100) == 0) x = ramp(20000)
    chars = chars + len(blanks(mod(j, 4)))
    write (text, '(5a)') (blanks(k), k = 1, 5)
    if (any(labels(mod(j, 3) + 1, 2) == 'ab')) chars = chars + 1
    if (j == 1) before = bytes()
  end do
  after = bytes()
  print '(1x, a, f10.0, i5)', 'total:', total, hits
  print '(1x, a, a)', 'line: ', line(1:30)
  print '(1x, a, f10.0)', 'big:  ', x(20000)
  print '(1x, a, i6)', 'chars:', chars
  print '(1x, a, l2)', 'heap given back:', before == after
contains
  integer function bytes()
    integer :: lib$stat_vm, st
    st = lib$stat_vm(3, bytes)
  end function
end program rampvms

function ramp(n)
  integer :: n
  real :: ramp(n)
  integer :: i
  ramp = (/ (real(i), i = 1, n) /)
end function

recursive function tri(n) result(r)
  integer :: n, r(n)
  r(n) = n
  if (n > 1) r(1:n - 1) = tri(n - 1)
end function

function blanks(n)
  integer :: n
  character(len=n) :: blanks
  blanks = ' '
end function

function labels(n, w)
  integer :: n, w
  character(len=w) :: labels(n)
  labels = 'ab'
end function

subroutine alt(v, *)
  real :: v(2)
  if (v(2) == 2.0) return 1
end subroutine
