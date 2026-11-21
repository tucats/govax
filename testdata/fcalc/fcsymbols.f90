! fcalc: the names a user defines, and the constants fcalc defines.
!
! Variables are kept in a binary search tree by name, each entry a node
! with POINTER children, so LIST shows them in name order by walking the
! tree. Entries are added and removed through pointer dummy arguments: a
! routine given the pointer that leads to an entry (the root, or a parent's
! LEFT or RIGHT) can make it lead somewhere else.
module fcsymbols
  use fckinds
  use fcmessages
  use fcvalues
  implicit none
  private

  ! VALUE is public here too: a public procedure's argument may not be of a
  ! type private in this module.
  public :: value
  public :: start_symbols, constant_value, is_constant, find_variable
  public :: set_variable, remove_variable, clear_variables, list_variables

  type variable
    character(len=name_length) :: name
    type(value) :: v
    type(variable), pointer :: left
    type(variable), pointer :: right
  end type variable

  type(variable), pointer, save :: root

  ! fcalc's constants. HUGE is fcalc's largest value, LIMIT, not the
  ! machine's: a VAX's and an IEEE machine's differ.
  character(len=4), parameter :: constant_names(5) = (/ 'pi  ', 'e   ', 'i   ', 'eps ', 'huge' /)

contains

  ! START_SYMBOLS empties the table: a module's pointer starts undefined,
  ! not null, in Fortran 90.
  subroutine start_symbols()
    nullify (root)
  end subroutine start_symbols

  logical function is_constant(name)
    character(len=*), intent(in) :: name

    is_constant = any(constant_names == name)
  end function is_constant

  ! CONSTANT_VALUE: if NAME is a constant, V is its value and the result
  ! is true.
  logical function constant_value(name, v)
    character(len=*), intent(in) :: name
    type(value), intent(out) :: v

    constant_value = .true.
    select case (name)
    case ('pi')
      v = real_value(pi)
    case ('e')
      v = real_value(e_value)
    case ('i')
      v = complex_value(0.0_rk, 1.0_rk)
    case ('eps')
      v = real_value(eps)
    case ('huge')
      v = real_value(limit)
    case default
      v = real_value(0.0_rk)
      constant_value = .false.
    end select
  end function constant_value

  ! FIND_VARIABLE: if NAME is a variable, V is its value and the result is
  ! true.
  logical function find_variable(name, v)
    character(len=*), intent(in) :: name
    type(value), intent(out) :: v
    type(variable), pointer :: p

    p => root
    do while (associated(p))
      if (name == p%name) then
        v = p%v
        find_variable = .true.
        return
      end if
      if (llt(name, p%name)) then
        p => p%left
      else
        p => p%right
      end if
    end do
    v = real_value(0.0_rk)
    find_variable = .false.
  end function find_variable

  subroutine set_variable(name, v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: v

    call insert(root, name, v)
  end subroutine set_variable

  ! INSERT puts NAME in the tree that T leads to, or changes its value.
  recursive subroutine insert(t, name, v)
    type(variable), pointer :: t
    character(len=*), intent(in) :: name
    type(value), intent(in) :: v

    if (.not. associated(t)) then
      allocate (t)
      t%name = name
      t%v = v
      nullify (t%left, t%right)
    else if (name == t%name) then
      t%v = v
    else if (llt(name, t%name)) then
      call insert(t%left, name, v)
    else
      call insert(t%right, name, v)
    end if
  end subroutine insert

  ! REMOVE_VARIABLE takes NAME out of the table; the result is false if it
  ! wasn't there.
  logical function remove_variable(name)
    character(len=*), intent(in) :: name

    remove_variable = .false.
    call remove(root, name, remove_variable)
  end function remove_variable

  ! REMOVE takes NAME out of the tree that T leads to. An entry with two
  ! children takes the place of its successor, the leftmost entry of its
  ! right subtree, which is then removed instead.
  recursive subroutine remove(t, name, found)
    type(variable), pointer :: t
    character(len=*), intent(in) :: name
    logical, intent(inout) :: found
    type(variable), pointer :: gone, successor
    character(len=name_length) :: key

    if (.not. associated(t)) return

    if (llt(name, t%name)) then
      call remove(t%left, name, found)
    else if (lgt(name, t%name)) then
      call remove(t%right, name, found)
    else
      found = .true.
      if (.not. associated(t%left)) then
        gone => t
        t => t%right
        deallocate (gone)
      else if (.not. associated(t%right)) then
        gone => t
        t => t%left
        deallocate (gone)
      else
        successor => t%right
        do while (associated(successor%left))
          successor => successor%left
        end do
        ! The successor's name is copied first: the entry it is in goes.
        key = successor%name
        t%name = key
        t%v = successor%v
        call remove(t%right, key, found)
      end if
    end if
  end subroutine remove

  ! CLEAR_VARIABLES empties the table.
  subroutine clear_variables()
    call chop(root)
  end subroutine clear_variables

  recursive subroutine chop(t)
    type(variable), pointer :: t

    if (.not. associated(t)) return
    call chop(t%left)
    call chop(t%right)
    deallocate (t)
  end subroutine chop

  ! LIST_VARIABLES shows each variable as NAME = VALUE, in name order.
  subroutine list_variables()
    call walk(root)
  end subroutine list_variables

  recursive subroutine walk(t)
    type(variable), pointer :: t

    if (.not. associated(t)) return
    call walk(t%left)
    call say(trim(t%name) // ' = ' // trim(format_value(t%v)))
    call walk(t%right)
  end subroutine walk
end module fcsymbols
