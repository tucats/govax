! The optimizer's aliasing rule for pointers (Phase 107, subtask 7). A store
! through a pointer may change a TARGET, a part of one, another pointer's
! target, or anything in COMMON, of a type that may meet the pointer's, and
! nothing else. Each case below stores through a pointer, in a loop, into
! something the optimizer would keep in a register, forward, or move out of
! the loop if the rule were too bold, and reads it back by its own name: a
! local TARGET, a record's component, a pointer to a record and one to its
! component, COMMON named a TARGET only in another unit, a module's TARGET, a
! host's TARGET in an internal procedure, a TARGET dummy argument, memory
! ALLOCATE made, a section of a TARGET array, a substring, and a TARGET a
! procedure aimed a pointer at. The last cases
! are the rule's other side, variables without TARGET, which it lets the
! optimizer keep in registers across the stores; they print the same either
! way. EQUIVALENCE with a TARGET, which Fortran 95 forbids, is ptrequiv.f90's.
module shelf
  implicit none
  integer, target :: stock
  integer, pointer :: clerk
  type node
    integer value
    real weight
    type(node), pointer :: next
  end type node
contains
  ! D and DP are associated on entry with the caller's T and its pointer
  ! (12.4.1.1), so a store through DP changes D.
  subroutine twice (d, dp, total)
    integer, target :: d
    integer, pointer :: dp
    integer total, j
    d = 1
    total = 0
    do j = 1, 3
      dp = dp + d
      total = total + d
    end do
  end subroutine twice

  ! CLERK stays associated with the caller's TARGET after the return.
  subroutine remember (d)
    integer, target :: d
    clerk => d
  end subroutine remember
end module shelf

program ptralias
  use shelf
  implicit none
  integer, target :: t, a(6), t14
  type(node), target :: n1
  type(node), pointer :: np
  integer, pointer :: p, q, ip, hp, ap(:)
  integer, target :: ht
  character(len=6), target :: word
  character(len=2), pointer :: cp
  character(len=2) first
  integer s, i, k, total, m
  real r, w
  common /books/ total

  ! A local TARGET read by name after each store through P.
  t = 1
  p => t
  s = 0
  do i = 1, 4
    p = p + 1
    s = s + t
  end do
  print *, 'target', s, t

  ! An INTEGER pointer at a record's component.
  n1%value = 1
  n1%weight = 0.5
  nullify (n1%next)
  ip => n1%value
  s = 0
  do i = 1, 3
    ip = ip + 10
    s = s + n1%value
  end do
  print *, 'component', s, n1%value

  ! A NODE pointer and an INTEGER pointer into the same NODE.
  np => n1
  s = 0
  do i = 1, 3
    np%value = i
    ip = ip * 3
    s = s + np%value
  end do
  np%value = 5
  ip = 80
  m = np%value
  print *, 'two pointers', s, m

  ! TOTAL is in COMMON, a TARGET in AIM, which aims CLERK at it.
  call aim
  total = 1
  s = 0
  do i = 1, 3
    clerk = clerk + 5
    s = s + total
  end do
  total = 2
  clerk = 40
  m = total
  print *, 'common', s, m

  ! A module's TARGET.
  clerk => stock
  stock = 2
  s = 0
  do i = 1, 3
    clerk = clerk * 2
    s = s + stock
  end do
  stock = 3
  clerk = 50
  m = stock
  print *, 'module', s, m

  ! A host's TARGET, in an internal procedure.
  ht = 0
  hp => ht
  call bump (s)
  print *, 'host', s, ht

  ! A TARGET dummy argument and a pointer dummy aimed at it.
  t = 0
  p => t
  call twice (t, p, s)
  print *, 'dummy', s, t

  ! Two pointers to memory ALLOCATE made.
  allocate (p)
  q => p
  p = 1
  s = 0
  do i = 1, 3
    q = q + p
    s = s + p
  end do
  p = 1
  q = 90
  m = p
  print *, 'allocated', s, m
  deallocate (p)

  ! A section of a TARGET array.
  a = 1
  ap => a(2:6:2)
  s = 0
  do i = 1, 3
    ap(i) = 10 * i
    s = s + a(2*i)
  end do
  a(4) = 7
  ap(2) = 70
  m = a(4)
  print *, 'section', s, m
  ap = 0
  print *, 'section', sum (a)

  ! A substring.
  word = 'abcdef'
  cp => word(3:4)
  word(3:4) = 'mn'
  cp = 'XY'
  first = word(3:4)
  print *, 'substring ', first, ' ', word

  ! A TARGET whose address a procedure kept in CLERK.
  call remember (t14)
  t14 = 1
  s = 0
  do i = 1, 2
    clerk = clerk + 100
    s = s + t14
  end do
  print *, 'kept', s, t14

  ! The rule's other side: K, R, and W have no TARGET, so a store through P,
  ! IP, or NP changes none of them, and the optimizer may keep them in
  ! registers. The answers are the same either way.
  t = 0
  p => t
  k = 3
  r = 1.5
  w = 0.0
  do i = 1, 4
    p = p + k
    np%weight = np%weight + r
    w = w + r
  end do
  print *, 'untouched', t, k, nint (10*r), nint (10*w), nint (10*n1%weight)

  ! A REAL pointer's store can't change an INTEGER TARGET.
  m = 0
  do i = 1, 3
    np%weight = np%weight * 2.0
    m = m + n1%value
  end do
  print *, 'other type', m, nint (n1%weight)

contains

  subroutine bump (got)
    integer got, j
    got = 0
    do j = 1, 3
      hp = hp + 4
      got = got + ht
    end do
    ht = 1
    hp = 60
    got = got + ht
  end subroutine bump

end program ptralias

subroutine aim
  use shelf
  implicit none
  integer, target :: total
  common /books/ total
  clerk => total
end subroutine aim
