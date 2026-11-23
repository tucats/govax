! NULL() (Phase 112, subtask 4): Fortran 95's disassociated pointer, which takes
! its type, kind, and rank from the pointer it is given to. It stands after => in
! a pointer assignment or a declaration, in DATA for a pointer, for a pointer
! component in a structure constructor, and as a pointer dummy's argument; MOLD
! names a pointer of the kind wanted, which a generic name's choice may need.
module lists
  implicit none
  type node
    integer :: value
    type(node), pointer :: next
  end type node
  type holder
    character(len=4), pointer :: tag
    real, pointer :: v(:)
  end type holder
  ! A module's pointer, disassociated before anything runs.
  integer, pointer :: shared => null()
  ! A generic name whose specifics differ only in their pointer dummies:
  ! NULL() given to it must have a MOLD to say which.
  interface describe
    module procedure describe_scalar, describe_array
  end interface
contains
  subroutine describe_scalar(p)
    integer, pointer :: p
    if (associated(p)) then
      print '(1X, A, I3)', 'scalar', p
    else
      print '(1X, A)', 'scalar, none'
    end if
  end subroutine describe_scalar

  subroutine describe_array(p)
    real, pointer :: p(:)
    if (associated(p)) then
      print '(1X, A, 3F5.1)', 'array', p
    else
      print '(1X, A)', 'array, none'
    end if
  end subroutine describe_array

  ! The length of a list, 0 for a disassociated head.
  subroutine walk(p, n)
    type(node), pointer :: p
    integer, intent(out) :: n
    type(node), pointer :: q
    n = 0
    q => p
    do while (associated(q))
      n = n + 1
      q => q%next
    end do
  end subroutine walk

  subroutine show(v)
    real, pointer :: v(:)
    if (associated(v)) then
      print '(1X, A, I2)', 'size', size(v)
    else
      print '(1X, A)', 'no array'
    end if
  end subroutine show

  subroutine label(c)
    character(len=4), pointer :: c
    print '(1X, A, L2)', 'label', associated(c)
  end subroutine label

  ! => NULL() gives LAST the SAVE attribute, so even in a recursive
  ! subroutine it is one pointer, which each level sees as the last left it.
  recursive subroutine remember(k, t)
    integer, intent(in) :: k
    integer, target :: t(:)
    integer, pointer :: last => null()
    print '(1X, A, I2, L2)', 'level', k, associated(last)
    last => t(k)
    if (k < size(t)) call remember(k + 1, t)
  end subroutine remember
end module lists

program nulls
  use lists
  implicit none
  integer, pointer :: p => null(), q
  real, pointer :: a(:) => null()
  character(len=4), pointer :: s => null()
  real, target :: t(3) = (/ 1.0, 2.0, 3.0 /)
  integer, target :: n = 7, levels(3) = (/ 10, 20, 30 /)
  integer, pointer :: d1, d2
  type(node), target :: first, second
  type(node), pointer :: head => null()
  type(holder) :: h
  integer :: count
  data d1, d2 /2*null()/

  ! Every pointer given NULL() starts disassociated.
  print '(1X, A, 7L2)', 'start', associated(p), associated(a), associated(s), &
    associated(shared), associated(d1), associated(d2), associated(head)

  ! P => NULL() is NULLIFY (P); MOLD may name a pointer of the same kind.
  q => n
  print '(1X, A, L2)', 'q => n', associated(q)
  q => null(q)
  print '(1X, A, L2)', 'q => null(q)', associated(q)
  a => t
  print '(1X, A, L2)', 'a => t', associated(a)
  a => null()
  print '(1X, A, L2)', 'a => null()', associated(a)

  ! A pointer component's value in a structure constructor.
  first = node(1, null())
  print '(1X, A, L2)', 'first%next', associated(first%next)
  second = node(2, null())
  first%next => second
  head => first

  ! A pointer dummy given NULL(): a disassociated pointer of its kind.
  call walk(head, count)
  print '(1X, A, I2)', 'list of', count
  call walk(null(), count)
  print '(1X, A, I2)', 'list of', count
  call show(null())
  a => t
  call show(a)
  call label(null())

  ! A CHARACTER pointer component and an array pointer component.
  h = holder(null(), null())
  print '(1X, A, 2L2)', 'holder', associated(h%tag), associated(h%v)
  h = holder(null(), t)
  print '(1X, A, 2L2, I2)', 'holder', associated(h%tag), associated(h%v), size(h%v)

  call remember(1, levels)
  call remember(1, levels)

  ! A generic name: the pointer itself is passed, and NULL(MOLD) picks a specific.
  q => n
  call describe(q)
  call describe(a)
  call describe(null(q))
  call describe(null(a))
end program nulls
