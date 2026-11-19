! New storage with pointers in it starts disassociated (Phase 107, subtask
! 5a; the author's answer 2 to the survey): a target ALLOCATE makes for a
! pointer to a type with pointer components, an allocatable array of such a
! type, and one whose pointers are a component's components, all cleared by
! FTNRTL; a recursive function's records in the frame, one, three (cleared
! in line), and twenty (by a loop), each pointed somewhere by one call before
! the next call's frame takes the same stack; an automatic array of them.
! The standard leaves such pointers undefined, so this is govax's own, not in
! compare.py.
program ptrclear
  implicit none
  type node
    integer :: value
    type(node), pointer :: next
    character(len=3), pointer :: tag
  end type node
  type holder
    integer :: count
    type(node) :: first, pair(2)
  end type holder
  type(node), pointer :: p
  type(node), allocatable :: arr(:)
  type(holder), pointer :: h
  type(holder), allocatable :: hs(:)
  integer :: i, n, k1, k2

  allocate (p)
  print '(1x, a, 2l2)', 'allocated:  ', associated (p%next), associated (p%tag)

  allocate (arr(40))
  n = 0
  do i = 1, 40
    if (associated (arr(i)%next)) n = n + 1
    if (associated (arr(i)%tag)) n = n + 1
  end do
  print '(1x, a, i4)', 'array:      ', n

  allocate (h, hs(5))
  n = 0
  if (associated (h%first%next) .or. associated (h%pair(2)%tag)) n = n + 1
  do i = 1, 5
    if (associated (hs(i)%pair(1)%next) .or. associated (hs(i)%pair(2)%tag)) n = n + 1
  end do
  print '(1x, a, i4)', 'nested:     ', n

  k1 = level (.true.)
  k2 = level (.false.)
  print '(1x, a, 2i4)', 'frames:     ', k1, k2

  call automatic (30, n)
  print '(1x, a, i4)', 'automatic:  ', n

contains
  ! How many of its records' pointers are associated at its start; then, if
  ! dirty, every one pointed at a record of its own.
  recursive function level (dirty) result (k)
    logical, intent(in) :: dirty
    integer :: k, i
    type(node), target :: mine
    type(node) :: few(3), many(20)
    character(len=3), target :: word
    k = 0
    if (associated (mine%next)) k = k + 1
    if (associated (mine%tag)) k = k + 1
    do i = 1, 3
      if (associated (few(i)%next)) k = k + 1
    end do
    do i = 1, 20
      if (associated (many(i)%next)) k = k + 1
      if (associated (many(i)%tag)) k = k + 1
    end do
    if (dirty) then
      word = 'ABC'
      mine%next => mine
      mine%tag => word
      do i = 1, 3
        few(i)%next => mine
      end do
      do i = 1, 20
        many(i)%next => mine
        many(i)%tag => word
      end do
    end if
  end function level

  subroutine automatic (m, n)
    integer, intent(in) :: m
    integer, intent(out) :: n
    type(node) :: a(m)
    integer :: i
    n = 0
    do i = 1, m
      if (associated (a(i)%next) .or. associated (a(i)%tag)) n = n + 1
    end do
  end subroutine automatic
end program ptrclear
