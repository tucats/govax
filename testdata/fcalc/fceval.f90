! fcalc: the evaluator, which walks a tree to its value.
!
! EVALUATE is recursive: an operation's value is its operands' values,
! combined by FCVALUES' operators. An error in a value (a division by zero,
! an overflow) is reported at the node it arose in, with that node's
! column, the first one found being the one FAIL keeps: so the pointer goes
! to the innermost operation that failed.
module fceval
  use fckinds
  use fcmessages
  use fcvalues
  use fctree
  use fcsymbols
  implicit none
  private

  ! NODE and VALUE are public here too: a public procedure's argument and
  ! value may not be of a type private in this module.
  public :: node, value
  public :: evaluate

contains

  recursive function evaluate(p) result(v)
    type(node), pointer :: p
    type(value) :: v

    select case (p%kind)
    case (n_number)
      v = p%number

    case (n_name)
      if (.not. constant_value(p%name, v)) then
        if (.not. find_variable(p%name, v)) then
          call fail(e_undefined, p%column, p%name)
          v = error_value(e_undefined)
        end if
      end if

    case (n_call)
      call fail(e_nofunc, p%column, p%name)
      v = error_value(e_nofunc)

    case default
      v = operate(p)
    end select

    if (v%err /= 0) call fail(v%err, p%column)
  end function evaluate

  ! OPERATE is an operation's value. AND and OR evaluate their right
  ! operand only if they must, so x <> 0 and 1/x > 2 is safe.
  recursive function operate(p) result(v)
    type(node), pointer :: p
    type(value) :: v
    type(value) :: a, b

    a = evaluate(p%kid)
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
      v = v_factorial(a)
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

    ! RIGHT is the right operand's value.
    recursive function right() result(r)
      type(value) :: r

      r = evaluate(p%kid%next)
    end function right
  end function operate
end module fceval
