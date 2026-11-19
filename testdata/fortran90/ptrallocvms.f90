! DEALLOCATE of pointers as VMS sees it (Phase 107, subtask 4): STAT='s FOR
! numbers for a disassociated pointer (NOTALLOC, 302) and for one whose
! target ALLOCATE didn't make, or isn't the whole of (govax's BADDEALLOC,
! 304): a TARGET variable, an allocatable array, sections of an allocated
! target (a part, every other element, all of it reversed), a target given
! back already through another pointer, a CHARACTER pointer to a part of
! another's; a failed DEALLOCATE leaving the pointer as it was; a section
! that is the whole target, and a scalar pointer to an allocated array's one
! element, given back; the same failures signaled without STAT=, made their
! routine's value by LIB$SIG_TO_RET; LIB$STAT_VM's bytes the same after
! targets of every kind are allocated and given back, and a target lost by a
! second ALLOCATE still counted. VMS's own, so not in compare.py.
program ptrallocvms
  implicit none
  integer, pointer :: p, q, a(:), b(:), grid(:, :)
  integer, target :: n
  integer, allocatable, target :: al(:)
  character(len=8), pointer :: c8
  character(len=5), pointer :: c5
  real(8), pointer :: x
  integer :: ist, before, after
  integer :: outstanding, badtarget, notassoc

  nullify (p)
  deallocate (p, stat=ist)
  print '(1x, a, i4)', 'disassociated:      ', ist

  p => n
  deallocate (p, stat=ist)
  print '(1x, a, i4, l2)', 'a TARGET:           ', ist, associated (p, n)

  allocate (al(4))
  a => al
  deallocate (a, stat=ist)
  print '(1x, a, i4, l2)', 'allocatable array:  ', ist, allocated (al)
  deallocate (al)

  allocate (a(6))
  b => a(2:4)
  deallocate (b, stat=ist)
  print '(1x, a, i4)', 'a part:             ', ist
  b => a(::2)
  deallocate (b, stat=ist)
  print '(1x, a, i4)', 'every other:        ', ist
  b => a(6:1:-1)
  deallocate (b, stat=ist)
  print '(1x, a, i4)', 'reversed:           ', ist
  b => a(:)
  deallocate (b, stat=ist)
  print '(1x, a, i4, l2)', 'the whole section:  ', ist, associated (b)

  allocate (p)
  q => p
  deallocate (p)
  deallocate (q, stat=ist)
  print '(1x, a, i4)', 'given back already: ', ist

  allocate (a(1))
  p => a(1)
  deallocate (p, stat=ist)
  print '(1x, a, i4, l2)', 'one element:        ', ist, associated (p)

  allocate (c8)
  c5 => c8(1:5)
  deallocate (c5, stat=ist)
  print '(1x, a, i4)', 'part of a string:   ', ist
  deallocate (c8)

  print '(1x, a, z8.8)', 'signaled:   %X', badtarget()
  print '(1x, a, z8.8)', 'signaled:   %X', notassoc()

  ! Targets of every kind, allocated and given back: no bytes kept.
  before = outstanding()
  allocate (p, x, c8, a(100), grid(0:2, 3))
  deallocate (grid, a, c8, x, p)
  after = outstanding()
  print '(1x, a, i6)', 'bytes kept:', after - before

  ! A second ALLOCATE loses the first target: an INTEGER's 8 bytes (the
  ! least FTNRTL takes) and its 8-byte header.
  before = outstanding()
  allocate (p)
  allocate (p)
  deallocate (p)
  after = outstanding()
  print '(1x, a, i6)', 'bytes lost:', after - before

end program ptrallocvms

! The bytes LIB$GET_VM has given out and LIB$FREE_VM hasn't had back.
integer function outstanding()
  integer :: lib$stat_vm, st, n
  st = lib$stat_vm(3, n)
  outstanding = n
end function outstanding

! DEALLOCATE of a pointer to a TARGET, with no STAT=: FTNRTL signals
! BADDEALLOC, and LIB$SIG_TO_RET makes it this function's value.
integer function badtarget()
  external lib$sig_to_ret
  integer, target :: t
  integer, pointer :: p
  call lib$establish(lib$sig_to_ret)
  p => t
  deallocate (p)
  badtarget = 1
end function badtarget

integer function notassoc()
  external lib$sig_to_ret
  real, pointer :: p(:)
  call lib$establish(lib$sig_to_ret)
  nullify (p)
  deallocate (p)
  notassoc = 1
end function notassoc
