! fcalc: the names a user defines, and the constants fcalc defines.
!
! Variables are kept in a binary search tree by name, each entry a node
! with POINTER children, so LIST shows them in name order by walking the
! tree. Entries are added and removed through pointer dummy arguments: a
! routine given the pointer that leads to an entry (the root, or a parent's
! LEFT or RIGHT) can make it lead somewhere else.
!
! A binary search tree keeps every name in an entry's LEFT subtree before
! the entry's name, and every name in its RIGHT subtree after it. Finding a
! name compares it with the root's and goes left or right, so a table of n
! names takes about log2(n) comparisons when the tree is balanced (more if
! names arrive in order, which a calculator's few names can afford). Names
! are compared by LLT and LGT, which compare by ASCII on every machine,
! where < on strings uses the machine's own character order.
!
! A variable's value is a QUANTITY (module FCARRAYS), whose storage the
! table owns: SET_VARIABLE takes the quantity it is given, and
! FIND_VARIABLE gives the caller a copy of its own.
!
! A dummy argument declared POINTER (as T in INSERT) is the caller's
! pointer itself, not a copy: when INSERT does ALLOCATE (T), the parent's
! LEFT or RIGHT, or ROOT, now points to the new entry. Such a dummy needs an
! explicit interface at the call, as module procedures have.
module fcsymbols
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  ! ONLY names just what this module needs from FCTREE: a tree's NODE type,
  ! and FREE_TREE to give one back when a function is redefined or cleared.
  use fctree, only: node, free_tree
  implicit none
  private

  ! VALUE is public here too: a public procedure's argument may not be of a
  ! type private in this module.
  public :: value, quantity
  public :: start_symbols, constant_value, is_constant, find_variable, holds_array
  public :: set_variable, remove_variable, clear_variables, list_variables
  public :: node, function_entry, max_params
  public :: find_function, define_function, remove_function, clear_functions
  public :: list_functions, heading, each_variable, each_function

  type variable
    character(len=name_length) :: name
    type(quantity) :: q
    type(variable), pointer :: left
    type(variable), pointer :: right
  end type variable

  type(variable), pointer, save :: root

  ! The most parameters a function may have.
  integer, parameter :: max_params = 8

  ! A user's function, f(x, y) = expr: its name, its parameters' names, the
  ! expression as a parsed tree (which the evaluator walks at each call),
  ! and the expression's text as it was typed (which LIST shows). The
  ! functions are a binary search tree of their own, beside the
  ! variables': a name may be both a variable and a function, since a
  ! function's name is always followed by a parenthesis.
  type function_entry
    character(len=name_length) :: name
    integer :: count
    character(len=name_length) :: params(max_params)
    type(node), pointer :: body
    character(len=line_length) :: text
    type(function_entry), pointer :: left
    type(function_entry), pointer :: right
  end type function_entry

  type(function_entry), pointer, save :: functions

  ! fcalc's constants. HUGE is fcalc's largest value, LIMIT, not the
  ! machine's: a VAX's and an IEEE machine's differ.
  character(len=4), parameter :: constant_names(5) = (/ 'pi  ', 'e   ', 'i   ', 'eps ', 'huge' /)

contains

  ! START_SYMBOLS empties the table: a module's pointer starts undefined,
  ! not null, in Fortran 90.
  subroutine start_symbols()
    nullify (root, functions)
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

  ! FIND_VARIABLE: if NAME is a variable, Q is a copy of its value, the
  ! caller's to release, and the result is true.
  logical function find_variable(name, q)
    character(len=*), intent(in) :: name
    type(quantity), intent(out) :: q
    type(variable), pointer :: p

    p => lookup(name)
    find_variable = associated(p)
    if (find_variable) then
      q = duplicate(p%q)
    else
      q = real_quantity(0.0_rk)
    end if
  end function find_variable

  ! HOLDS_ARRAY: NAME is a variable whose value is an array.
  logical function holds_array(name)
    character(len=*), intent(in) :: name
    type(variable), pointer :: p

    p => lookup(name)
    holds_array = .false.
    if (associated(p)) holds_array = is_array(p%q)
  end function holds_array

  ! LOOKUP is NAME's entry, or a null pointer.
  function lookup(name) result(p)
    character(len=*), intent(in) :: name
    type(variable), pointer :: p

    p => root
    do while (associated(p))
      if (name == p%name) return
      if (llt(name, p%name)) then
        p => p%left
      else
        p => p%right
      end if
    end do
  end function lookup

  ! SET_VARIABLE gives NAME the value Q, which the table takes: the caller
  ! mustn't release it.
  subroutine set_variable(name, q)
    character(len=*), intent(in) :: name
    type(quantity), intent(in) :: q

    call insert(root, name, q)
  end subroutine set_variable

  ! INSERT puts NAME in the tree that T leads to, or changes its value,
  ! releasing the old one.
  recursive subroutine insert(t, name, q)
    type(variable), pointer :: t
    character(len=*), intent(in) :: name
    type(quantity), intent(in) :: q

    if (.not. associated(t)) then
      allocate (t)
      t%name = name
      t%q = q
      nullify (t%left, t%right)
    else if (name == t%name) then
      call release(t%q)
      t%q = q
    else if (llt(name, t%name)) then
      call insert(t%left, name, q)
    else
      call insert(t%right, name, q)
    end if
  end subroutine insert

  ! REMOVE_VARIABLE takes NAME out of the table; the result is false if it
  ! wasn't there.
  logical function remove_variable(name)
    character(len=*), intent(in) :: name

    remove_variable = .false.
    call remove(root, name, remove_variable, .true.)
  end function remove_variable

  ! REMOVE takes NAME out of the tree that T leads to, releasing its value
  ! if DROP is true. An entry with two children takes the place of its
  ! successor, the leftmost entry of its right subtree, which is then
  ! removed instead: its value has moved, so it is removed without DROP.
  recursive subroutine remove(t, name, found, drop)
    type(variable), pointer :: t
    character(len=*), intent(in) :: name
    logical, intent(inout) :: found
    logical, intent(in) :: drop
    type(variable), pointer :: gone, successor
    character(len=name_length) :: key

    if (.not. associated(t)) return

    if (llt(name, t%name)) then
      call remove(t%left, name, found, drop)
    else if (lgt(name, t%name)) then
      call remove(t%right, name, found, drop)
    else
      found = .true.
      if (.not. associated(t%left) .or. .not. associated(t%right)) then
        gone => t
        if (associated(t%left)) then
          t => t%left
        else
          t => t%right
        end if
        if (drop) call release(gone%q)
        deallocate (gone)
      else
        successor => t%right
        do while (associated(successor%left))
          successor => successor%left
        end do
        ! The successor's name is copied first: the entry it is in goes.
        key = successor%name
        if (drop) call release(t%q)
        t%name = key
        t%q = successor%q
        call remove(t%right, key, found, .false.)
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
    call release(t%q)
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
    call list_quantity(t%name, t%q)
    call walk(t%right)
  end subroutine walk

  ! EACH_VARIABLE calls ACTION(NAME, Q) for each variable, in name order:
  ! SAVE (module FCSTATE) writes each one to its file this way.
  !
  ! ACTION is a dummy procedure: the caller passes a procedure's name, and
  ! each call of ACTION here calls that procedure. The INTERFACE block says
  ! what arguments it takes, so each call is checked as a module
  ! procedure's is. An interface body is a scope of its own that sees
  ! nothing of the module around it (no host association), so it USEs the
  ! module that defines QUANTITY itself. (Fortran 90 lets a module
  ! procedure or an external one be passed, never an internal one.)
  subroutine each_variable(action)
    interface
      subroutine action(name, q)
        use fcarrays, only: quantity
        character(len=*), intent(in) :: name
        type(quantity), intent(in) :: q
      end subroutine action
    end interface

    call visit(root)

  contains

    recursive subroutine visit(t)
      type(variable), pointer :: t

      if (.not. associated(t)) return
      call visit(t%left)
      call action(trim(t%name), t%q)
      call visit(t%right)
    end subroutine visit
  end subroutine each_variable

  ! ------------------------------------------------------------------
  ! Functions.

  ! FIND_FUNCTION is the entry for the function NAME, or a null pointer if
  ! there is none. A function whose result is a POINTER returns the
  ! address of the entry itself, not a copy, so the caller sees the tree
  ! the entry holds; the caller must use => to keep it:
  !     f => find_function(name)
  function find_function(name) result(f)
    character(len=*), intent(in) :: name
    type(function_entry), pointer :: f

    f => functions
    do while (associated(f))
      if (name == f%name) return
      if (llt(name, f%name)) then
        f => f%left
      else
        f => f%right
      end if
    end do
  end function find_function

  ! DEFINE_FUNCTION adds the function NAME, or replaces its definition. The
  ! tree BODY becomes the entry's: the caller mustn't free it.
  subroutine define_function(name, count, params, body, text)
    character(len=*), intent(in) :: name
    integer, intent(in) :: count
    character(len=*), intent(in) :: params(:)
    type(node), pointer :: body
    character(len=*), intent(in) :: text

    call place(functions)

  contains

    ! PLACE finds where NAME belongs in the tree T leads to. It is an
    ! internal procedure, so it sees DEFINE_FUNCTION's arguments by host
    ! association and needn't be passed them.
    recursive subroutine place(t)
      type(function_entry), pointer :: t

      if (.not. associated(t)) then
        allocate (t)
        nullify (t%left, t%right)
        t%name = name
        call fill(t)
      else if (name == t%name) then
        call free_tree(t%body)
        call fill(t)
      else if (llt(name, t%name)) then
        call place(t%left)
      else
        call place(t%right)
      end if
    end subroutine place

    subroutine fill(t)
      type(function_entry), pointer :: t

      t%count = count
      t%params = ' '
      t%params(1:count) = params(1:count)
      t%body => body
      t%text = text
    end subroutine fill
  end subroutine define_function

  ! REMOVE_FUNCTION takes NAME out of the functions; the result is false if
  ! it wasn't there.
  logical function remove_function(name)
    character(len=*), intent(in) :: name

    remove_function = .false.
    call unlink(functions, name, remove_function)
  end function remove_function

  ! UNLINK takes NAME out of the tree T leads to. Unlike REMOVE (above, for
  ! variables), which copies a successor's contents into the place of the
  ! entry removed, it moves the successor entry itself, so the trees the
  ! entries point to never need copying.
  recursive subroutine unlink(t, name, found)
    type(function_entry), pointer :: t
    character(len=*), intent(in) :: name
    logical, intent(inout) :: found
    type(function_entry), pointer :: gone, successor

    if (.not. associated(t)) return

    if (llt(name, t%name)) then
      call unlink(t%left, name, found)
      return
    else if (lgt(name, t%name)) then
      call unlink(t%right, name, found)
      return
    end if

    found = .true.
    gone => t
    if (.not. associated(t%left)) then
      t => t%right
    else if (.not. associated(t%right)) then
      t => t%left
    else
      ! The successor, the leftmost entry of the right subtree, comes out
      ! of its place and takes the place of the entry removed.
      successor => detach_least(t%right)
      successor%left => t%left
      successor%right => t%right
      t => successor
    end if
    call free_tree(gone%body)
    deallocate (gone)
  end subroutine unlink

  ! DETACH_LEAST takes the leftmost entry out of the tree T leads to, and
  ! returns it.
  recursive function detach_least(t) result(least)
    type(function_entry), pointer :: t
    type(function_entry), pointer :: least

    if (associated(t%left)) then
      least => detach_least(t%left)
    else
      least => t
      t => t%right
    end if
  end function detach_least

  ! CLEAR_FUNCTIONS forgets every function.
  subroutine clear_functions()
    call fell(functions)
  end subroutine clear_functions

  recursive subroutine fell(t)
    type(function_entry), pointer :: t

    if (.not. associated(t)) return
    call fell(t%left)
    call fell(t%right)
    call free_tree(t%body)
    deallocate (t)
  end subroutine fell

  ! LIST_FUNCTIONS shows each function as it was defined, in name order.
  subroutine list_functions()
    call show(functions)
  end subroutine list_functions

  ! HEADING is a function's name and parameters, as f(x, y).
  function heading(t) result(text)
    type(function_entry), pointer :: t
    character(len=name_length * (max_params + 1) + 2 * max_params + 2) :: text
    integer :: k

    text = trim(t%name) // '('
    do k = 1, t%count
      ! (TRIM would take off the blank after a comma, so the comma and
      ! the next name are added together.)
      if (k == 1) then
        text = trim(text) // t%params(k)
      else
        text = trim(text) // ', ' // t%params(k)
      end if
    end do
    text = trim(text) // ')'
  end function heading

  ! EACH_FUNCTION calls ACTION(HEAD, TEXT) for each function, in name
  ! order: HEAD is f(x, y), and TEXT the expression as it was typed.
  subroutine each_function(action)
    interface
      subroutine action(head, text)
        character(len=*), intent(in) :: head, text
      end subroutine action
    end interface

    call visit(functions)

  contains

    recursive subroutine visit(t)
      type(function_entry), pointer :: t

      if (.not. associated(t)) return
      call visit(t%left)
      call action(trim(heading(t)), trim(t%text))
      call visit(t%right)
    end subroutine visit
  end subroutine each_function

  recursive subroutine show(t)
    type(function_entry), pointer :: t

    if (.not. associated(t)) return
    call show(t%left)
    call say(trim(heading(t)) // ' = ' // trim(t%text))
    call show(t%right)
  end subroutine show

end module fcsymbols
