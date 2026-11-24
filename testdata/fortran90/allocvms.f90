! Allocation status as VMS sees it (Phase 104, subtask 3): STAT='s FOR
! numbers (ALREADYALLOC 301, NOTALLOC 302, and VAX FORTRAN's INSVIRMEM 41, for
! an array past 2**31 - 1 bytes); the same failures without STAT= signaled,
! each turned into its routine's value by LIB$SIG_TO_RET; a routine left by an
! unwind with its array allocated, whose next call still allocates it (each
! entry point marks it unallocated, the block left behind); and LIB$STAT_VM's
! bytes the same before and after a recursion whose levels' arrays are freed
! by their returns. VMS's own, so not in compare.py.
program allocvms
  implicit none
  integer, allocatable :: a(:)
  real, allocatable :: big(:, :)
  integer :: ist, k, before, after
  integer :: outstanding, twice, notalloc, toobig, trygrab, churn

  allocate (a(2), stat=ist)
  allocate (a(3), stat=ist)
  print '(1x, a, i4)', 'ALREADYALLOC: ', ist
  deallocate (a)
  deallocate (a, stat=ist)
  print '(1x, a, i4)', 'NOTALLOC:     ', ist
  allocate (big(65536, 65536), stat=ist)
  print '(1x, a, i4, l2)', 'INSVIRMEM:    ', ist, allocated(big)

  do k = 1, 2
    print '(1x, a, z8.8)', 'signaled:   %X', twice()
  end do
  print '(1x, a, z8.8)', 'signaled:   %X', notalloc()
  print '(1x, a, z8.8)', 'signaled:   %X', toobig()

  before = outstanding()
  do k = 1, 2
    print '(1x, a, z8.8)', 'unwound:    %X', trygrab(10)
  end do
  after = outstanding()
  print '(1x, a, l2)', 'a block left behind:', after > before

  ! The call alone between the counts: a PRINT there would count its format,
  ! parsed at its first use and kept for the next (Phase 113, subtask 8).
  before = outstanding()
  k = churn(6)
  after = outstanding()
  print '(1x, a, i6)', 'churn:', k
  print '(1x, a, i6)', 'bytes kept:', after - before

end program allocvms

! The bytes LIB$GET_VM has given out and LIB$FREE_VM hasn't had back.
integer function outstanding()
  integer :: lib$stat_vm, st, n
  st = lib$stat_vm(3, n)
  outstanding = n
end function outstanding

! ALLOCATE of an allocated array, with no STAT=: FTNRTL signals ALREADYALLOC,
! and LIB$SIG_TO_RET makes it this function's value. The unwind leaves A
! allocated, but the next call's entry marks it unallocated again.
integer function twice()
  external lib$sig_to_ret
  real, allocatable :: a(:)
  call lib$establish(lib$sig_to_ret)
  allocate (a(2))
  allocate (a(3))
  twice = 1
end function twice

integer function notalloc()
  external lib$sig_to_ret
  real, allocatable :: a(:)
  call lib$establish(lib$sig_to_ret)
  deallocate (a)
  notalloc = 1
end function notalloc

integer function toobig()
  external lib$sig_to_ret
  real, allocatable :: a(:, :, :)
  call lib$establish(lib$sig_to_ret)
  allocate (a(2048, 2048, 2048))
  toobig = 1
end function toobig

! GRAB allocates its array, then signals; TRYGRAB's handler unwinds past both,
! so GRAB's return never deallocates it.
integer function trygrab(n)
  integer, intent(in) :: n
  external lib$sig_to_ret
  call lib$establish(lib$sig_to_ret)
  call grab(n)
  trygrab = 1
end function trygrab

subroutine grab(n)
  integer, intent(in) :: n
  integer, allocatable :: w(:)
  integer, parameter :: ss_abort = 44
  allocate (w(n))
  w = n
  call lib$signal(%val(ss_abort))
end subroutine grab

! No I/O, so nothing but the arrays uses the heap.
recursive integer function churn(n) result(r)
  integer, intent(in) :: n
  integer, allocatable :: q(:, :)
  allocate (q(n, n))
  q = 1
  r = sum(q)
  if (n > 1) r = r + churn(n - 1)
end function churn
