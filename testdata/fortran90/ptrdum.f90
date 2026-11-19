! Pointer dummy arguments (Phase 107, subtask 6a): each is given the actual
! pointer itself, so what the procedure does to it, =>, NULLIFY, ALLOCATE,
! DEALLOCATE, it does to the caller's. A linked list whose head and whose
! records' links are passed to procedures that push, count, drop, and free
! nodes; an array that a procedure grows by ALLOCATE, a copy, DEALLOCATE,
! and =>, with its new bounds; a CHARACTER(LEN=*) pointer dummy taking its
! actual's length; an OPTIONAL pointer dummy, left out and given; a rank-two
! pointer dummy pointed at a column; a pointer dummy passed on as a pointer
! and as its target; an array pointer component as the actual; an internal
! procedure's pointer dummies, and one reaching its host's; and a COMMON
! array pointer an external procedure allocates, through an interface block.
module stacks
  implicit none
  type node
    integer :: value
    type(node), pointer :: next
  end type node
  type bag
    character(len=4) :: name
    type(node), pointer :: first
    real, pointer :: weights(:)
  end type bag
  character(len=5), target :: words(3) = (/ 'alpha', 'beta ', 'gamma' /)
  real, target :: grid(3, 4)
contains
  subroutine push (head, v)
    type(node), pointer :: head
    integer, intent(in) :: v
    type(node), pointer :: new
    allocate (new)
    new%value = v
    new%next => head
    head => new
  end subroutine push

  integer function count_nodes (head)
    type(node), pointer :: head
    type(node), pointer :: p
    count_nodes = 0
    p => head
    do while (associated (p))
      count_nodes = count_nodes + 1
      p => p%next
    end do
  end function count_nodes

  subroutine show (title, head)
    character(len=*), intent(in) :: title
    type(node), pointer :: head
    type(node), pointer :: p
    integer :: n, vals(10)
    n = 0
    p => head
    do while (associated (p))
      n = n + 1
      vals(n) = p%value
      p => p%next
    end do
    print '(1x, a, i3, a, 10i4)', title, count_nodes (head), ':', vals(1:n)
  end subroutine show

  ! Drops the node after the one link points at, by pointing the link past it.
  subroutine drop_next (link)
    type(node), pointer :: link
    type(node), pointer :: gone
    gone => link%next
    if (.not. associated (gone)) return
    link%next => gone%next
    deallocate (gone)
  end subroutine drop_next

  subroutine free_all (head)
    type(node), pointer :: head
    type(node), pointer :: p
    do while (associated (head))
      p => head
      head => head%next
      deallocate (p)
    end do
  end subroutine free_all

  ! Grows a to upper bound n, keeping its values, the new ones 0.
  subroutine grow (a, n)
    real, pointer :: a(:)
    integer, intent(in) :: n
    real, pointer :: b(:)
    integer :: lo, hi
    lo = lbound (a, 1)
    hi = ubound (a, 1)
    allocate (b(lo:n))
    b = 0.0
    b(lo:hi) = a
    deallocate (a)
    a => b
  end subroutine grow

  subroutine pick (c, k)
    character(len=*), pointer :: c
    integer, intent(in) :: k
    c => words(k)
    print '(1x, a, i2, 3a)', 'pick', len (c), ' [', c, ']'
  end subroutine pick

  ! Points p at k's node of the list, or nullifies p; tells where if asked.
  subroutine find (head, k, p, place)
    type(node), pointer :: head, p
    integer, intent(in) :: k
    integer, pointer, optional :: place
    integer, target, save :: at
    integer :: i
    p => head
    do i = 2, k
      if (.not. associated (p)) exit
      p => p%next
    end do
    if (present (place)) then
      if (associated (p)) then
        at = k
        place => at
      else
        nullify (place)
      end if
    end if
  end subroutine find

  subroutine column (m, j)
    real, pointer :: m(:, :)
    integer, intent(in) :: j
    m => grid(:, j:j)
  end subroutine column

  real function total (x)
    real, intent(in) :: x(:)
    total = sum (x)
  end function total

  ! A pointer dummy passed on as a pointer, to grow, and as its target, to
  ! total; and reached by an internal procedure.
  subroutine double (a)
    real, pointer :: a(:)
    integer :: n
    n = size (a)
    call grow (a, 2 * n)
    a(n + 1:) = a(1:n) * 2.0
    call report
  contains
    subroutine report
      print '(1x, a, i3, f6.1)', 'doubled', size (a), total (a)
    end subroutine report
  end subroutine double
end module stacks

program ptrdum
  use stacks
  implicit none
  type(node), pointer :: list, p
  type(bag) :: b
  real, pointer :: r(:), m(:, :)
  character(len=5), pointer :: w
  integer, pointer :: place
  integer :: i, j
  real, pointer :: shared(:)
  common /pool/ shared
  interface
    subroutine fill (a)
      real, pointer :: a(:)
    end subroutine fill
  end interface

  ! A linked list through its head.
  nullify (list)
  do i = 1, 5
    call push (list, 10 * i)
  end do
  call show ('pushed', list)
  call drop_next (list)
  call show ('dropped 2nd', list)
  call drop_next (list%next%next)
  call show ('dropped 4th', list)
  call find (list, 2, p, place)
  print '(1x, a, i4, i3)', 'found', p%value, place
  call find (list, 7, p, place)
  print '(1x, a, 2l2)', 'found 7', associated (p), associated (place)
  call find (list, 3, p)
  print '(1x, a, i4)', 'found 3', p%value
  call drop_next (p)
  call show ('none after last', list)
  call free_all (list)
  print '(1x, a, l2)', 'freed', associated (list)

  ! A list in a record.
  b%name = 'bag1'
  nullify (b%first)
  call push (b%first, 7)
  call push (b%first, 8)
  call push (b%first%next, 9)
  call show (b%name, b%first)
  call free_all (b%first)

  ! Growing arrays.
  allocate (r(0:2))
  r = (/ 1.0, 2.0, 3.0 /)
  call grow (r, 5)
  print '(1x, a, 2i3, 6f5.1)', 'grown', lbound (r, 1), ubound (r, 1), r
  deallocate (r)
  allocate (r(3))
  r = (/ 1.0, 2.0, 3.0 /)
  call double (r)
  print '(1x, a, 2i3, 6f5.1)', 'doubled', lbound (r, 1), ubound (r, 1), r
  deallocate (r)
  allocate (b%weights(2))
  b%weights = (/ 0.5, 1.5 /)
  call grow (b%weights, 4)
  print '(1x, a, i3, 4f5.1)', 'weights', size (b%weights), b%weights
  deallocate (b%weights)

  ! CHARACTER(LEN=*) and rank two.
  do i = 3, 1, -1
    call pick (w, i)
    print '(1x, 3a)', '[', w, ']'
  end do
  w = 'delta'
  print '(1x, 3(1x, a))', words
  grid = reshape ((/ (real (i), i = 1, 12) /), (/ 3, 4 /))
  do j = 1, 4, 3
    call column (m, j)
    print '(1x, a, 2i2, 3f5.1)', 'column', shape (m), m
  end do
  m = 0.0
  print '(1x, a, f6.1)', 'grid', sum (grid)

  ! An internal procedure's pointer dummies.
  allocate (r(3))
  r = 1.0
  call swap (r, m)
  print '(1x, a, i2, i3)', 'swapped', shape (r), size (m)
  deallocate (m)

  ! A COMMON array pointer, allocated by a procedure.
  nullify (shared)
  call fill (shared)
  print '(1x, a, 4f5.1)', 'shared', shared
  call fill (shared)
  print '(1x, a, 4f5.1)', 'shared again', shared
  deallocate (shared)

contains
  ! x is given new storage and y a row of the grid; the host's r and m
  ! are their actuals.
  subroutine swap (x, y)
    real, pointer :: x(:), y(:, :)
    real, pointer :: keep(:)
    keep => x
    allocate (y(2, 3))
    y = 4.0
    x => grid(1, :)
    print '(1x, a, 2l2, f5.1)', 'swap', associated (keep, x), associated (x, grid(1, :)), sum (y)
    deallocate (keep)
  end subroutine swap
end program ptrdum

! Each call makes a new array, the last value its count of calls so far.
subroutine fill (a)
  implicit none
  real, pointer :: a(:)
  integer, save :: calls = 0
  integer :: i
  calls = calls + 1
  if (associated (a)) deallocate (a)
  allocate (a(4))
  do i = 1, 4
    a(i) = real (i * calls)
  end do
end subroutine fill
