! Pointer function results (Phase 107, subtask 6b): a function whose value
! is a pointer gives its caller an association, not a value. A scalar
! pointer's target read and written through the caller's pointer; the value
! in an expression, as an argument, and in an output list; ASSOCIATED with
! such a reference as its target; a CHARACTER pointer value, and one left
! disassociated; a COMPLEX one; array pointer values, a section of a TARGET
! dummy and a block the function allocates, with its bounds; an array
! assignment whose value may overlap its target through one; a recursive
! search of a linked list returning the node it finds; an internal function
! whose value is a pointer; and an external function with an ENTRY, both
! names pointers, through an interface block.
module lists
  implicit none
  type node
    integer :: value
    type(node), pointer :: next
  end type node
  real, target :: table(6) = (/ 3.0, 9.0, 4.0, 1.0, 7.0, 2.0 /)
  character(len=5), target :: words(3) = (/ 'red  ', 'green', 'blue ' /)
  complex, target :: roots(2) = (/ (1.0, 2.0), (3.0, -4.0) /)
contains
  function largest (a)
    real, target :: a(:)
    real, pointer :: largest
    integer :: i
    largest => a(1)
    do i = 2, size (a)
      if (a(i) > largest) largest => a(i)
    end do
  end function largest

  function word (i)
    integer, intent(in) :: i
    character(len=5), pointer :: word
    if (i >= 1 .and. i <= 3) word => words(i)
  end function word

  function root (k)
    integer, intent(in) :: k
    complex, pointer :: root
    root => roots(k)
  end function root

  function evens (a)
    real, target :: a(:)
    real, pointer :: evens(:)
    evens => a(2::2)
  end function evens

  function ramp (n)
    integer, intent(in) :: n
    integer, pointer :: ramp(:)
    integer :: i
    allocate (ramp(0:n-1))
    do i = 0, n - 1
      ramp(i) = i * i
    end do
  end function ramp

  function newnode (v, next)
    integer, intent(in) :: v
    type(node), pointer :: next
    type(node), pointer :: newnode
    allocate (newnode)
    newnode%value = v
    newnode%next => next
  end function newnode

  recursive function find (head, v) result (hit)
    type(node), pointer :: head
    integer, intent(in) :: v
    type(node), pointer :: hit
    nullify (hit)
    if (.not. associated (head)) return
    if (head%value == v) then
      hit => head
    else
      hit => find (head%next, v)
    end if
  end function find
end module lists

program ptrres
  use lists
  implicit none
  interface
    function first (a)
      integer, target :: a(:)
      integer, pointer :: first
    end function first
    function final (a)
      integer, target :: a(:)
      integer, pointer :: final
    end function final
  end interface
  real, pointer :: big, ev(:)
  integer, pointer :: sq(:), k
  character(len=5), pointer :: c
  type(node), pointer :: head, none, hit
  real, target :: x(6)
  integer, target :: n(4) = (/ 10, 20, 30, 40 /)
  real :: r
  integer :: i

  ! A scalar pointer's target.
  big => largest (table)
  print '(1x, a, f5.1)', 'largest', big
  big = 0.0
  print '(1x, a, 6f5.1)', 'table  ', table
  print '(1x, a, f5.1)', 'twice  ', 2.0 * largest (table)
  print '(1x, a, 2l2)', 'same   ', associated (big, table(2)), associated (big, largest (table))
  r = largest (table)
  call show (largest (table))

  ! CHARACTER and COMPLEX.
  c => word (2)
  print '(1x, a, a)', 'word   ', c
  c = 'GREEN'
  print '(1x, a, 3(1x, a))', 'words  ', words
  print '(1x, a, 2l2)', 'assoc  ', associated (c, word (2)), associated (c, word (3))
  c => word (7)
  print '(1x, a, l2)', 'outside', associated (c)
  print '(1x, a, a, i2)', 'third  ', word (3) // '!', len (word (3))
  print '(1x, a, 2f5.1)', 'root   ', root (2) * (0.0, 1.0)

  ! Array pointers.
  x = (/ 1.0, 2.0, 3.0, 4.0, 5.0, 6.0 /)
  print '(1x, a, 3f5.1)', 'evens  ', evens (x)
  print '(1x, a, f5.1, i2)', 'sum    ', sum (evens (x)), size (evens (x))
  ev => evens (x)
  ev = ev * 10.0
  print '(1x, a, 6f5.1)', 'x      ', x
  print '(1x, a, 2i3)', 'bounds ', lbound (ev, 1), ubound (ev, 1)
  x = (/ 1.0, 2.0, 3.0, 4.0, 5.0, 6.0 /)
  x(4:6) = evens (x)
  print '(1x, a, 6f5.1)', 'overlap', x
  sq => ramp (4)
  print '(1x, a, 2i3, 4i4)', 'ramp   ', lbound (sq, 1), ubound (sq, 1), sq
  deallocate (sq)
  print '(1x, a, l2)', 'freed  ', associated (sq)

  ! A linked list.
  nullify (none)
  head => none
  do i = 1, 4
    head => newnode (i * 11, head)
  end do
  hit => find (head, 22)
  print '(1x, a, l2, i4)', 'found  ', associated (hit), hit%value
  hit%value = 99
  hit => find (head, 22)
  print '(1x, a, l2)', 'gone   ', associated (hit)
  print '(1x, a, i4)', 'second ', head%next%next%value

  ! An internal function, and an external one with an ENTRY.
  print '(1x, a, i4)', 'inner  ', pick (3)
  k => first (n)
  k = -k
  k => final (n)
  k = k + 1
  print '(1x, a, 4i4)', 'n      ', n
contains
  function pick (i)
    integer, intent(in) :: i
    integer, pointer :: pick
    pick => n(i)
  end function pick

  subroutine show (v)
    real, intent(in) :: v
    print '(1x, a, f5.1, l2)', 'shown  ', v, v == r
  end subroutine show
end program ptrres

function first (a)
  integer, target :: a(:)
  integer, pointer :: first, final
  first => a(1)
  return
entry final (a)
  final => a(size (a))
end function first
