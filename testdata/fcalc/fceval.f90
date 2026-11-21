! fcalc: the evaluator, which walks a tree to its value.
!
! EVALUATE is recursive: an operation's value is its operands' values,
! combined by FCVALUES' operators. An error in a value (a division by zero,
! an overflow) is reported at the node it arose in, with that node's
! column, the first one found being the one FAIL keeps: so the pointer goes
! to the innermost operation that failed.
!
! A call of a user's function, f(2, 3), evaluates its arguments, puts them
! in a FRAME beside the names of the function's parameters, and evaluates
! the function's tree with that frame: a name in the tree is looked for
! among the frame's parameters first, then among the constants and the
! variables. Each call has a frame of its own, a local variable of the
! recursive CALL_USER, so a function that calls itself,
!
!     fact(n) = if(n < 2, 1, n * fact(n - 1))
!
! has a frame for each level, as a compiled recursive routine has a stack
! frame for each. The depth of such calls is limited to CALL_LIMIT, so a
! function that never stops calling itself is an error, not the end of
! the stack.
!
! The tree of a function was parsed from the line that defined it, so its
! columns are that line's, not the one being evaluated. Inside a function
! an error is reported at the column of the call the user typed: the
! frame carries it (FRAME%COLUMN), and every node evaluated with a frame
! reports there.
module fceval
  use fckinds
  use fcmessages
  use fcvalues
  use fctree
  use fcsymbols
  use fcfuncs
  implicit none
  private

  ! NODE, VALUE, and FRAME are public here too: a public procedure's
  ! argument and value may not be of a type private in this module.
  public :: node, value, frame
  public :: evaluate

  ! The values of one call's parameters, by name.
  type frame
    integer :: count
    integer :: column
    character(len=name_length) :: names(max_params)
    type(value) :: values(max_params)
  end type frame

  ! How deeply calls of users' functions may nest, and how deeply they are
  ! nested now.
  integer, parameter :: call_limit = 200
  integer, save :: calls = 0

contains

  ! EVALUATE is the value of the tree P. F, the frame of the function call
  ! being evaluated, is an OPTIONAL argument: the main program leaves it
  ! out, and PRESENT(F) says whether it was given. An absent optional
  ! argument may be passed on to another procedure's optional argument,
  ! where it is absent too, so EVALUATE passes F to everything it calls
  ! without asking.
  recursive function evaluate(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(value) :: v
    integer :: at

    at = p%column
    if (present(f)) at = f%column

    select case (p%kind)
    case (n_number)
      v = p%number

    case (n_name)
      if (.not. parameter_value(p%name, v, f)) then
        if (.not. constant_value(p%name, v)) then
          if (.not. find_variable(p%name, v)) then
            call fail(e_undefined, at, p%name)
            v = error_value(e_undefined)
          end if
        end if
      end if

    case (n_call)
      v = call_function(p, at, f)

    case default
      v = operate(p, f)
    end select

    if (v%err /= 0) call fail(v%err, at)
  end function evaluate

  ! PARAMETER_VALUE: if F is given and NAME is one of its parameters, V is
  ! the parameter's value and the result is true.
  logical function parameter_value(name, v, f)
    character(len=*), intent(in) :: name
    type(value), intent(out) :: v
    type(frame), intent(in), optional :: f
    integer :: k

    parameter_value = .false.
    v = real_value(0.0_rk)
    if (.not. present(f)) return
    do k = 1, f%count
      if (f%names(k) == name) then
        v = f%values(k)
        parameter_value = .true.
        return
      end if
    end do
  end function parameter_value

  ! OPERATE is an operation's value. AND and OR evaluate their right
  ! operand only if they must, so x <> 0 and 1/x > 2 is safe.
  recursive function operate(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(value) :: v
    type(value) :: a, b

    a = evaluate(p%kid, f)
    if (a%err /= 0) then
      v = a
      return
    end if

    select case (p%op)
    case (op_neg)
      v = -a
    case (op_plus)
      v = +a
    case (op_not)
      v = logical_value(.not. truth(a))
    case (op_factorial)
      v = factorial_value(a)
    case (op_and)
      if (truth(a)) then
        v = right()
        if (v%err == 0) v = logical_value(truth(v))
      else
        v = logical_value(.false.)
      end if
    case (op_or)
      if (truth(a)) then
        v = logical_value(.true.)
      else
        v = right()
        if (v%err == 0) v = logical_value(truth(v))
      end if
    case default
      b = right()
      if (b%err /= 0) then
        v = b
        return
      end if
      select case (p%op)
      case (op_add)
        v = a + b
      case (op_sub)
        v = a - b
      case (op_mul)
        v = a * b
      case (op_div)
        v = a / b
      case (op_mod)
        v = v_mod(a, b)
      case (op_power)
        v = a ** b
      case default
        v = v_compare(a, b, p%op - op_compare)
      end select
    end select

  contains

    ! RIGHT is the right operand's value. As an internal procedure it sees
    ! OPERATE's P and F by host association, F even when it is absent.
    recursive function right() result(r)
      type(value) :: r

      r = evaluate(p%kid%next, f)
    end function right
  end function operate

  ! ARGUMENT_COUNT is how many arguments the call P has: the length of the
  ! list KID starts.
  integer function argument_count(p)
    type(node), pointer :: p
    type(node), pointer :: a

    argument_count = 0
    a => p%kid
    do while (associated(a))
      argument_count = argument_count + 1
      a => a%next
    end do
  end function argument_count

  ! CALL_FUNCTION is the value of the call P: of IF, a built-in function,
  ! or a user's function, in that order.
  recursive function call_function(p, at, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: at
    type(frame), intent(in), optional :: f
    type(value) :: v
    type(function_entry), pointer :: fn
    integer :: n, id

    n = argument_count(p)
    id = builtin_index(p%name)

    if (id > 0) then
      if (.not. arity_ok(id, n)) then
        call fail(e_argcount, at, text=p%name)
        v = error_value(e_argcount)
      else if (p%name == 'if') then
        v = choose(p, f)
      else
        v = call_builtin(p, id, n, f)
      end if
      return
    end if

    fn => find_function(p%name)
    if (.not. associated(fn)) then
      call fail(e_nofunc, at, text=p%name)
      v = error_value(e_nofunc)
    else if (fn%count /= n) then
      call fail(e_argcount, at, text=p%name)
      v = error_value(e_argcount)
    else
      v = call_user(p, fn, at, f)
    end if
  end function call_function

  ! CHOOSE is if(c, a, b): A's value if C is true (not zero), and B's if
  ! not. Only the arm chosen is evaluated, so a recursive function can
  ! stop: in if(n < 2, 1, n * fact(n - 1)) the call is made only while n
  ! is 2 or more.
  recursive function choose(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(value) :: v
    type(node), pointer :: arm

    v = evaluate(p%kid, f)
    if (v%err /= 0) return
    if (truth(v)) then
      arm => p%kid%next
    else
      arm => p%kid%next%next
    end if
    v = evaluate(arm, f)
  end function choose

  ! CALL_BUILTIN evaluates the call's N arguments into ARGS, then has
  ! FCFUNCS apply the function to them. ARGS is an automatic array: its
  ! size, N, is a dummy argument, so each call has an array of the size
  ! it needs, made on entry and gone on return.
  recursive function call_builtin(p, id, n, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: id, n
    type(frame), intent(in), optional :: f
    type(value) :: v
    type(value) :: args(n)
    type(node), pointer :: a
    integer :: k

    a => p%kid
    do k = 1, n
      args(k) = evaluate(a, f)
      if (args(k)%err /= 0) then
        v = args(k)
        return
      end if
      a => a%next
    end do
    v = apply_builtin(id, args)
  end function call_builtin

  ! CALL_USER is the call P of the user's function FN: its arguments,
  ! evaluated in the caller's frame F, become a new frame, INNER, in which
  ! FN's tree is evaluated.
  recursive function call_user(p, fn, at, f) result(v)
    type(node), pointer :: p
    type(function_entry), pointer :: fn
    integer, intent(in) :: at
    type(frame), intent(in), optional :: f
    type(value) :: v
    type(frame) :: inner
    type(node), pointer :: a
    integer :: k

    inner%count = fn%count
    inner%column = at
    a => p%kid
    do k = 1, fn%count
      inner%names(k) = fn%params(k)
      inner%values(k) = evaluate(a, f)
      if (inner%values(k)%err /= 0) then
        v = inner%values(k)
        return
      end if
      a => a%next
    end do

    if (calls >= call_limit) then
      v = error_value(e_recursion)
      return
    end if
    calls = calls + 1
    v = evaluate(fn%body, inner)
    calls = calls - 1
  end function call_user
end module fceval
