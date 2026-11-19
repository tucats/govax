! Pointer components (Phase 107, subtask 5a): a linked list of a type that
! points at itself, in a module whose procedures push, reverse, and drop its
! nodes; components through chains of pointers, read and assigned; a node of
! the program's own linked in and out; ASSOCIATED, NULLIFY, ALLOCATE, and
! DEALLOCATE of a component; intrinsic assignment copying a pointer;
! INTEGER and CHARACTER pointer components, a substring through one;
! structure constructors whose pointer components are given targets, a
! pointer among them; an array of records with pointers; an array assignment
! reading a pointer component into its own target once; a binary search tree
! built by an internal procedure and walked by recursive ones, each given a
! pointer component's target as an ordinary dummy.
module lists
  implicit none
  type node
    integer :: value
    type(node), pointer :: next
  end type node
  type(node), pointer :: head
  integer :: made = 0
contains
  subroutine push (v)
    integer, intent(in) :: v
    type(node), pointer :: new
    allocate (new)
    new%value = v
    new%next => head
    head => new
    made = made + 1
  end subroutine push

  subroutine show (title)
    character(len=*), intent(in) :: title
    type(node), pointer :: p
    integer :: n, vals(20)
    n = 0
    p => head
    do while (associated (p))
      n = n + 1
      vals(n) = p%value
      p => p%next
    end do
    print '(1x, a, i3, a, 20i4)', title, n, ':', vals(1:n)
  end subroutine show

  subroutine reverse ()
    type(node), pointer :: prev, cur, nxt
    nullify (prev)
    cur => head
    do while (associated (cur))
      nxt => cur%next
      cur%next => prev
      prev => cur
      cur => nxt
    end do
    head => prev
  end subroutine reverse

  subroutine drop_all ()
    type(node), pointer :: p
    do while (associated (head))
      p => head
      head => head%next
      deallocate (p)
    end do
  end subroutine drop_all
end module lists

program ptrcomps
  use lists
  implicit none
  type ref
    integer, pointer :: p
    character(len=4), pointer :: c
  end type ref
  type tnode
    integer :: key
    type(tnode), pointer :: left, right
  end type tnode
  type(ref) :: r1, r2, rs(3)
  integer, target :: v(5), w
  character(len=4), target :: word
  type(tnode), pointer :: root
  type(node), target :: fixed
  type(node) :: copy
  type(node), pointer :: q
  integer :: i, ns, sorted(9), keys(9)
  data keys /50, 30, 70, 20, 40, 60, 80, 35, 65/

  ! A list in a module.
  nullify (head)
  call show ('empty:   ')
  do i = 1, 5
    call push (i * 10)
  end do
  call show ('pushed:  ')
  call reverse ()
  call show ('reversed:')

  ! Chains of pointers.
  print '(1x, a, 3i4)', 'chain:   ', head%value, head%next%value, head%next%next%value
  head%next%next%value = -30
  call show ('changed: ')

  ! A node of the program's own, linked in after the second and out again.
  fixed%value = 25
  fixed%next => head%next%next
  head%next%next => fixed
  call show ('inserted:')
  print '(1x, a, 2l2)', 'fixed:   ', associated (head%next%next, fixed), &
    associated (fixed%next, head%next%next%next)
  head%next%next => fixed%next
  call show ('removed: ')

  ! Intrinsic assignment copies the pointer, not its target.
  copy = head
  copy%value = 99
  print '(1x, a, 2i4, l2)', 'copy:    ', copy%value, head%value, associated (copy%next, head%next)

  ! ALLOCATE, DEALLOCATE, and NULLIFY of components.
  q => head%next%next
  allocate (head%next%next)
  head%next%next%value = 33
  head%next%next%next => q
  call show ('grown:   ')
  deallocate (head%next%next)
  print '(1x, a, l2)', 'given:   ', associated (head%next%next)
  head%next%next => q
  call show ('back:    ')
  q => head%next%next%next%next
  nullify (head%next%next%next%next)
  call show ('cut:     ')
  head%next%next%next%next => q
  call show ('joined:  ')

  ! INTEGER and CHARACTER pointer components.
  v = (/ (i, i = 1, 5) /)
  w = 7
  word = 'ABCD'
  r1%p => v(2)
  r1%c => word
  r2 = r1
  r2%p => w
  r2%p = r2%p + r1%p
  r1%c(2:3) = 'xy'
  print '(1x, a, 7i4, 1x, a, 1x, a)', 'refs:    ', v, w, r1%p, word, r2%c

  ! Constructors give pointer components targets.
  r1 = ref(w, word)
  print '(1x, a, i4, 1x, a, 2l2)', 'built:   ', r1%p, r1%c, associated (r1%p, w), associated (r1%c, word)
  copy = node(5, head)
  print '(1x, a, 2i4, l2)', 'node():  ', copy%value, copy%next%value, associated (copy%next, head)

  ! An array of records with pointers, and an array assignment reading one
  ! into its own target.
  do i = 1, 3
    rs(i)%p => v(6 - i)
  end do
  print '(1x, a, 3i4)', 'array:   ', (rs(i)%p, i = 1, 3)
  v = v + rs(3)%p
  print '(1x, a, 5i4)', 'overlap: ', v

  ! A binary search tree.
  nullify (root)
  do i = 1, 9
    call insert (keys(i))
  end do
  ns = 0
  call walk (root)
  print '(1x, a, 9i4)', 'sorted:  ', sorted
  print '(1x, a, 3i4)', 'tree:    ', depth (root), root%left%right%left%key, root%right%left%right%key

  call drop_all ()
  call show ('dropped: ')
  print '(1x, a, i4)', 'made:    ', made

contains
  subroutine insert (k)
    integer, intent(in) :: k
    type(tnode), pointer :: p, new
    allocate (new)
    new%key = k
    nullify (new%left, new%right)
    if (.not. associated (root)) then
      root => new
      return
    end if
    p => root
    do
      if (k < p%key) then
        if (.not. associated (p%left)) then
          p%left => new
          return
        end if
        p => p%left
      else
        if (.not. associated (p%right)) then
          p%right => new
          return
        end if
        p => p%right
      end if
    end do
  end subroutine insert

  recursive subroutine walk (t)
    type(tnode) :: t
    if (associated (t%left)) call walk (t%left)
    ns = ns + 1
    sorted(ns) = t%key
    if (associated (t%right)) call walk (t%right)
  end subroutine walk

  recursive function depth (t) result (d)
    type(tnode) :: t
    integer :: d
    d = 0
    if (associated (t%left)) d = depth (t%left)
    if (associated (t%right)) d = max (d, depth (t%right))
    d = d + 1
  end function depth
end program ptrcomps
