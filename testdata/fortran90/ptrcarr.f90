! Array pointer components (Phase 107, subtask 5b): a type whose components
! are array pointers, of INTEGER, REAL of rank 2, CHARACTER, and of the type
! itself (a tree whose nodes point at an array of their children); => to whole
! arrays, sections, other pointers, and other components, and a component
! reversed through itself; elements, sections, and whole components read and
! assigned, in expressions, reductions, and an I/O list's implied DO over an
! array of records; LBOUND, UBOUND, SIZE; ALLOCATE with bounds, DEALLOCATE,
! NULLIFY, ASSOCIATED both ways; intrinsic assignment copying a descriptor; a
! component reached through a chain of pointers; a component passed to an
! assumed-shape and an explicit-shape dummy; a statement function's and an
! internal procedure's references, named as the program's are; a constructor
! giving an array pointer component a target; a field of every record an array
! pointer of records points at, through a component and through a named
! pointer; a module whose type has one; a recursive routine's local record and
! an allocated record holding components.
module shelves
  implicit none
  type shelf
    character(len=6) :: label
    integer, pointer :: items(:)
  end type shelf
contains
  integer function total (s)
    type(shelf), intent(in) :: s
    total = 0
    if (associated (s%items)) total = sum (s%items)
  end function total
end module shelves

program ptrcarr
  use shelves
  implicit none
  type holder
    integer :: n
    integer, pointer :: v(:)
  end type holder
  type grid
    real, pointer :: m(:,:)
    character(len=3), pointer :: names(:)
  end type grid
  type tree
    integer :: key
    type(tree), pointer :: kids(:)
  end type tree
  type link
    type(holder) :: h
    type(link), pointer :: next
  end type link
  type(holder) :: a, b, hs(3)
  type(grid) :: g
  type(tree) :: root
  type(tree), pointer :: tp(:)
  type(link), target :: l1, l2
  type(link), pointer :: lp
  type(shelf) :: s
  integer, target :: data(10), more(0:4)
  real, target :: plane(3,4)
  character(len=3), target :: words(4)
  integer :: i, j, first

  ! A statement function whose dummy is named as the program's J is.
  first (j) = hs(j)%v(1)

  data = (/ (10*i, i = 1, 10) /)
  more = (/ 5, 6, 7, 8, 9 /)

  ! Whole arrays, with their bounds, and sections, from 1.
  nullify (a%v)
  print '(1x, a, l2)', 'none:', associated (a%v)
  a%v => data
  print '(1x, a, l2, l2, 3i4)', 'whole:', associated (a%v), associated (a%v, data), &
       lbound (a%v), ubound (a%v), size (a%v)
  a%v => more
  print '(1x, a, 3i4, 5i4)', 'more:', lbound (a%v), ubound (a%v), size (a%v), a%v
  print '(1x, a, 2i4)', 'element:', a%v(0), a%v(4)
  a%v => data(2:10:3)
  print '(1x, a, 3i4, 3i5)', 'section:', lbound (a%v), ubound (a%v), size (a%v), a%v
  print '(1x, a, l2, l2)', 'same:', associated (a%v, data(2:10:3)), associated (a%v, data)

  ! Assignment through it changes the target.
  a%v(2) = -1
  a%v = a%v + 1
  print '(1x, a, 10i4)', 'data:', data

  ! Another pointer, another component, and intrinsic assignment.
  b%v => a%v
  print '(1x, a, l2, 3i5)', 'copied:', associated (b%v, a%v), b%v
  b = a
  b%n = 7
  print '(1x, a, l2, 3i5)', 'record:', associated (b%v, data(2:10:3)), b%v
  b%v => data
  a%v => b%v(9:10)
  print '(1x, a, 2i5, i3)', 'from b:', a%v, lbound (a%v)

  ! Reversed through itself.
  a%v => data(1:5)
  a%v => a%v(5:1:-1)
  print '(1x, a, 5i5)', 'reversed:', a%v
  print '(1x, a, 6i5)', 'section of it:', a%v(2:4), a%v(5:1:-2)

  ! ALLOCATE and DEALLOCATE, with bounds.
  allocate (a%v(-2:2))
  a%v = (/ (i*i, i = -2, 2) /)
  print '(1x, a, 2i4, 5i4, i5)', 'allocated:', lbound (a%v), ubound (a%v), a%v, sum (a%v)
  deallocate (a%v)
  print '(1x, a, l2)', 'deallocated:', associated (a%v)

  ! An array of records, in an implied DO.
  do i = 1, 3
    allocate (hs(i)%v(i))
    hs(i)%n = i
    hs(i)%v = (/ (10*i + j, j = 1, i) /)
  end do
  print '(1x, a, 6i4)', 'records:', ((hs(i)%v(j), j = 1, size (hs(i)%v)), i = 1, 3)
  print *, 'list:', hs(3)%v
  j = 3
  print '(1x, a, 2i4)', 'statement function:', first (2), hs(j)%v(1)
  do i = 1, 3
    deallocate (hs(i)%v)
  end do

  ! REAL of rank 2, CHARACTER.
  plane = reshape ((/ (real (i), i = 1, 12) /), (/ 3, 4 /))
  g%m => plane(2:3, 2:4)
  print '(1x, a, 4i3, 6f5.1)', 'rank 2:', lbound (g%m), ubound (g%m), g%m
  g%m(1, :) = 0.
  print '(1x, a, f6.1, 4f5.1)', 'plane:', sum (plane), plane(2, :)
  words = (/ 'ant', 'bee', 'cat', 'dog' /)
  g%names => words(4:1:-1)
  print '(1x, a, 4(1x, a), 1x, a)', 'names:', g%names, g%names(2)(2:3)

  ! Through a chain of pointers.
  l1%next => l2
  nullify (l2%next)
  l2%h%v => data(3:4)
  lp => l1
  print '(1x, a, 2i5)', 'chain:', lp%next%h%v
  lp%next%h%v(1) = 99
  print '(1x, a, i5)', 'data(3):', data(3)

  ! Dummies, assumed-shape and explicit-shape.
  a%v => data(1:9:2)
  call shaped (a%v)
  call explicit (a%v, size (a%v))
  call own ()
  print '(1x, a, 10i4)', 'after calls:', data

  ! A constructor's target.
  b = holder (3, more(1:3))
  print '(1x, a, i3, 3i4, l2)', 'built:', b%n, b%v, associated (b%v, more(1:3))

  ! A tree: each node's children an array pointer of trees.
  root%key = 1
  allocate (root%kids(3))
  do i = 1, 3
    root%kids(i)%key = 10 + i
    allocate (root%kids(i)%kids(i))
    root%kids(i)%kids%key = 100*i
    do j = 1, i
      nullify (root%kids(i)%kids(j)%kids)
    end do
  end do
  print '(1x, a, 3i4)', 'children:', root%kids%key
  print '(1x, a, 6i5)', 'grandchildren:', ((root%kids(i)%kids(j)%key, j = 1, i), i = 1, 3)
  tp => root%kids(3:1:-2)
  print '(1x, a, 2i4, 2i4)', 'every other:', tp%key, size (tp(1)%kids), size (tp(2)%kids)
  tp => root%kids(3)%kids
  print '(1x, a, 3i5)', 'named:', tp%key

  ! A module's type.
  s%label = 'top'
  nullify (s%items)
  print '(1x, a, i4)', 'empty shelf:', total (s)
  allocate (s%items(4))
  s%items = (/ 1, 2, 3, 4 /)
  print '(1x, a, 1x, a, i4)', 'shelf:', s%label, total (s)
  deallocate (s%items)

  ! A recursive routine's records.
  call fresh (3)
contains
  subroutine shaped (x)
    integer :: x(:)
    print '(1x, a, 2i3, 5i4)', 'shaped:', lbound (x), size (x), x
    x = x + 2
  end subroutine shaped

  subroutine explicit (x, n)
    integer :: n, x(n)
    print '(1x, a, 5i4)', 'explicit:', x
    x(n) = 0
  end subroutine explicit

  ! A record of its own named as the host's is.
  subroutine own ()
    type(holder) :: a
    a%v => data(10:9:-1)
    print '(1x, a, 2i4)', 'own:', a%v
  end subroutine own

  recursive subroutine fresh (depth)
    integer, intent(in) :: depth
    type(holder) :: local
    type(holder), pointer :: hp
    allocate (hp)
    allocate (local%v(depth))
    local%v = depth
    hp%v => local%v
    if (depth > 1) call fresh (depth - 1)
    print '(1x, a, i2, 3i3)', 'kept:', depth, hp%v
    deallocate (local%v)
    deallocate (hp)
  end subroutine fresh
end program ptrcarr
