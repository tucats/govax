! EQUIVALENCE with a TARGET and the optimizer's aliasing rule (Phase 107,
! subtask 7). Fortran 90's 5.5.1 lets a TARGET be in an EQUIVALENCE list
! (Fortran 95 forbids it, so this program is govax's own, not given to
! gfortran), and then a store through a pointer to the TARGET changes what
! shares its storage, whatever that is named or typed: TWIN, an INTEGER, and
! BITS, a REAL, here. Each is read by its own name after each store through P,
! in a loop the optimizer would keep it in a register for if the rule judged
! it by its own attributes alone.
program ptrequiv
  implicit none
  integer, target :: it, jt
  integer twin
  real bits
  equivalence (it, twin), (jt, bits)
  integer, pointer :: p
  integer s, i
  real r

  ! TWIN shares IT's storage, and IT is P's target.
  twin = 1
  p => it
  s = 0
  do i = 1, 3
    p = p * 2
    s = s + twin
  end do
  print *, 'same type', s, twin

  ! BITS shares JT's storage. An INTEGER 0 is the F_floating 0.0.
  p => jt
  r = 0.0
  do i = 1, 3
    bits = 2.5
    p = 0
    r = r + bits
  end do
  print *, 'another type', r
end program ptrequiv
