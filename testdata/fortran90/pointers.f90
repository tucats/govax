! Scalar pointers (Phase 107, subtask 2): POINTER and TARGET as attributes
! and statements; => to a TARGET, an element, a substring, a component, and
! another pointer; NULLIFY; ASSOCIATED with one argument and two; a pointer
! standing for its target in expressions, assignments, READ and WRITE, an
! internal file, and as an actual argument; pointers in a module, in COMMON,
! and in a record's place; an array assignment reading a pointer into its
! own target, which must read it once; and loops whose stores go through a
! pointer to a variable the optimizer would otherwise keep in a register.
module holder
  implicit none
  integer, pointer :: counter
end module holder

program pointers
  use holder
  implicit none
  type pair
    integer first, second
  end type pair
  integer, target :: n, m, v(5), k
  integer, pointer :: p, q
  integer total, i
  integer, target :: tally
  type(pair), target :: a, b
  type(pair), pointer :: pp
  character(len=12), target :: line
  character(len=4), pointer :: part
  integer, pointer :: shared
  common /link/ shared
  pointer :: e
  integer e

  ! Disassociated, then associated with N.
  nullify (p, q)
  print *, associated (p), associated (q)
  n = 1
  m = 2
  p => n
  print *, associated (p), associated (p, n), associated (p, m)

  ! P is N: assigning to P assigns to N.
  p = 5
  print *, n, p

  ! Q takes P's target, N, and keeps it when P moves on to M.
  q => p
  print *, associated (q, p), associated (q, n)
  p => m
  print *, associated (q, p), associated (q, m)
  q = q + p
  print *, n, m

  ! A pointer to an element; the array assignment reads E once, before V(3)
  ! changes.
  v = (/ 10, 20, 30, 40, 50 /)
  e => v(3)
  v = v + e
  print *, v

  ! A pointer to a record, its components through it, and a pointer to a
  ! component.
  a = pair(1, 2)
  b = pair(3, 4)
  pp => a
  pp%first = 100
  print *, a%first, a%second
  pp => b
  e => pp%second
  e = e + 1
  print *, b%first, b%second, associated (e, b%second), associated (e, b%first)

  ! A CHARACTER pointer to a substring, written and read as an internal file.
  line = 'ABCDEFGH1234'
  part => line(5:8)
  write (part, '(I4)') 42
  print *, line, ' ', part, len (part)
  read (part, '(I4)') i
  print *, i

  ! READ into a pointer's target.
  p => k
  read (line(9:12), '(I4)') p
  print *, k

  ! An actual argument is the target.
  p => m
  call triple (p)
  print *, m

  ! Stores through P into K inside a loop, where the optimizer might keep K
  ! in a register.
  k = 0
  p => k
  total = 0
  do i = 1, 5
    k = k + i
    p = p * 2
    total = total + k
  end do
  print *, k, total

  ! A call changes TALLY through the module's pointer.
  counter => tally
  tally = 0
  total = 0
  do i = 1, 3
    call bump
    total = total + tally
  end do
  print *, tally, total

  ! A pointer in COMMON, given its target here and used in another unit.
  shared => n
  call through
  print *, n
end program pointers

subroutine triple (j)
  integer j
  j = j * 3
end subroutine triple

subroutine bump
  use holder
  counter = counter + 1
end subroutine bump

subroutine through
  integer, pointer :: shared
  common /link/ shared
  shared = -shared
end subroutine through
