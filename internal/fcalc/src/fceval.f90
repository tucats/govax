! fcalc: the evaluator, which walks a tree to its value.
!
! EVALUATE is recursive: an operation's value is its operands' values,
! combined by FCVALUES' operators. An error in a value (a division by zero,
! an overflow) is reported at the node it arose in, with that node's
! column, the first one found being the one FAIL keeps: so the pointer goes
! to the innermost operation that failed.
!
! Values are QUANTITYs (module FCARRAYS), scalars or arrays, and EVALUATE
! keeps FCARRAYS' rule of ownership: every quantity it returns is new, the
! caller's to release, and each operation releases its operands once it
! has its result. An operation on arrays is done element by element (+, -,
! .*, ./, .^, mod, the comparisons, and, or, not, and every function of
! FCFUNCS but those of lists), a scalar operand combining with every
! element of the other; or is a matrix operation of FCMATRIX's (*, /, \,
! ^, and the transposes ' and .').
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
! NAME(...) is, in this order: a built-in function's call, an index of a
! parameter, a user's function's call, an index of a variable. So a
! variable named like a function is reached by its name alone, and v(2)
! is an element of v unless v is also a function.
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
  use fcarrays
  use fctree
  use fcsymbols
  use fcfuncs
  use fcmatrix
  implicit none
  private

  ! NODE, VALUE, QUANTITY, and FRAME are public here too: a public
  ! procedure's argument and value may not be of a type private in this
  ! module.
  public :: node, value, quantity, frame
  public :: evaluate, evaluate_with, assign_into, known_function

  ! The values of one call's parameters, by name. VALUES(1:COUNT) are the
  ! frame's to release when the call ends. COLUMN is where an error inside
  ! the call is reported, or 0 to report it at the node it arose in (as
  ! EVALUATE_WITH's frames do).
  type frame
    integer :: count
    integer :: column
    character(len=name_length) :: names(max_params)
    type(quantity) :: values(max_params)
  end type frame

  ! How deeply calls of users' functions may nest, and how deeply they are
  ! nested now.
  integer, parameter :: call_limit = 200
  integer, save :: calls = 0

contains

  ! KNOWN_FUNCTION: NAME is a built-in function, of FCFUNCS' or FCMATRIX's.
  logical function known_function(name)
    character(len=*), intent(in) :: name

    known_function = builtin_index(name) > 0 .or. matrix_index(name) > 0
  end function known_function

  ! COLUMN_OF is the column an error at the node P is reported at: P's own,
  ! or, inside a user's function, the call's.
  !
  ! (F is tested and used in two IF statements, not one: Fortran doesn't
  ! promise to skip .AND.'s right operand when its left is false, so
  ! PRESENT(F) .AND. F%COLUMN > 0 could touch an absent F.)
  integer function column_of(p, f)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f

    column_of = p%column
    if (present(f)) then
      if (f%column > 0) column_of = f%column
    end if
  end function column_of

  ! EVALUATE_WITH is the value of the tree P with NAME standing for X: the
  ! numerical commands (module FCNUMERIC) evaluate an expression in a
  ! variable they choose the values of, SOLVE X^2 - 2 FOR X, this way. The
  ! name is a frame's one parameter, so it hides a variable of the same
  ! name, and an error is reported at the node it arose in, in the line the
  ! user typed (the frame's column is 0).
  function evaluate_with(p, name, x) result(v)
    type(node), pointer :: p
    character(len=*), intent(in) :: name
    type(value), intent(in) :: x
    type(quantity) :: v
    type(frame) :: f

    f%count = 1
    f%column = 0
    f%names(1) = name
    f%values(1) = scalar_quantity(x)
    v = evaluate(p, f)
    call release(f%values(1))
  end function evaluate_with

  ! EVALUATE is the value of the tree P. F, the frame of the function call
  ! being evaluated, is an OPTIONAL argument: the main program leaves it
  ! out, and PRESENT(F) says whether it was given. An absent optional
  ! argument may be passed on to another procedure's optional argument,
  ! where it is absent too, so EVALUATE passes F to everything it calls
  ! without asking.
  recursive function evaluate(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(value) :: c
    integer :: at

    at = column_of(p, f)

    select case (p%kind)
    case (n_number)
      v = scalar_quantity(p%number)

    case (n_name)
      if (.not. parameter_value(p%name, v, f)) then
        if (constant_value(p%name, c)) then
          v = scalar_quantity(c)
        else if (.not. find_variable(p%name, v)) then
          call fail(e_undefined, at, p%name)
          v = failure(e_undefined)
        end if
      end if

    case (n_call)
      v = call_function(p, at, f)

    case (n_matrix)
      v = matrix_literal(p, f)

    case (n_range)
      v = failure(e_badrange)

    case default
      v = operate(p, f)
    end select

    if (quantity_error(v) /= 0) call fail(quantity_error(v), at)
  end function evaluate

  ! PARAMETER_VALUE: if F is given and NAME is one of its parameters, V is
  ! a copy of the parameter's value and the result is true.
  logical function parameter_value(name, v, f)
    character(len=*), intent(in) :: name
    type(quantity), intent(out) :: v
    type(frame), intent(in), optional :: f
    integer :: k

    parameter_value = .false.
    v = real_quantity(0.0_rk)
    if (.not. present(f)) return
    do k = 1, f%count
      if (f%names(k) == name) then
        v = duplicate(f%values(k))
        parameter_value = .true.
        return
      end if
    end do
  end function parameter_value

  ! ------------------------------------------------------------------
  ! Operations.

  ! OPERATE is an operation's value. AND and OR evaluate their right
  ! operand only if they must, so x <> 0 and 1/x > 2 is safe: a scalar
  ! left operand that settles the result alone ends it.
  recursive function operate(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(quantity) :: a, b

    a = evaluate(p%kid, f)
    if (quantity_error(a) /= 0) then
      v = a
      return
    end if

    select case (p%op)
    case (op_neg, op_plus, op_not, op_factorial)
      v = unary(a, p%op)
      call release(a)
      return
    case (op_transpose, op_ctranspose)
      v = transposed(a, p%op == op_ctranspose)
      call release(a)
      return
    case (op_and, op_or)
      if (.not. is_array(a)) then
        if (truth(a%s) .eqv. p%op == op_or) then
          v = scalar_quantity(logical_value(p%op == op_or))
          return
        end if
      end if
    end select

    b = evaluate(p%kid%next, f)
    if (quantity_error(b) /= 0) then
      call release(a)
      v = b
      return
    end if

    select case (p%op)
    case (op_mul)
      if (is_array(a) .and. is_array(b)) then
        v = matrix_product(a, b)
      else
        v = combine(a, b, op_mul)
      end if
    case (op_div)
      if (is_array(b)) then
        v = right_divide(a, b)
      else
        v = combine(a, b, op_div)
      end if
    case (op_solve)
      ! A \ B with a scalar A is B divided by it.
      if (is_array(a)) then
        v = solve(a, b)
      else
        v = combine(b, a, op_div)
      end if
    case (op_power)
      if (is_array(a)) then
        v = matrix_power(a, b)
      else if (is_array(b)) then
        v = failure(e_notscalar)
      else
        v = combine(a, b, op_power)
      end if
    case (op_emul)
      v = combine(a, b, op_mul)
    case (op_ediv)
      v = combine(a, b, op_div)
    case (op_epower)
      v = combine(a, b, op_power)
    case default
      v = combine(a, b, p%op)
    end select
    call release(a)
    call release(b)
  end function operate

  ! SCALAR_OP is X op Y for two scalar values.
  function scalar_op(x, y, op) result(v)
    type(value), intent(in) :: x, y
    integer, intent(in) :: op
    type(value) :: v

    select case (op)
    case (op_add)
      v = x + y
    case (op_sub)
      v = x - y
    case (op_mul)
      v = x * y
    case (op_div)
      v = x / y
    case (op_mod)
      v = v_mod(x, y)
    case (op_power)
      v = x ** y
    case (op_and)
      v = logical_value(truth(x) .and. truth(y))
    case (op_or)
      v = logical_value(truth(x) .or. truth(y))
    case default
      v = v_compare(x, y, op - op_compare)
    end select
  end function scalar_op

  ! COMBINE is A op B, element by element: two arrays of one shape element
  ! with element, a scalar with each element of an array. The first
  ! element's error is the whole result's.
  function combine(a, b, op) result(q)
    type(quantity), intent(in) :: a, b
    integer, intent(in) :: op
    type(quantity) :: q
    type(value) :: r
    integer :: i, j

    if (.not. is_array(a) .and. .not. is_array(b)) then
      q = scalar_quantity(scalar_op(a%s, b%s, op))
      return
    end if
    if (is_array(a) .and. is_array(b) .and. .not. same_shape(a, b)) then
      q = failure(e_notconform)
      return
    end if
    if (is_array(a)) then
      q = new_array(a%rows, a%cols)
    else
      q = new_array(b%rows, b%cols)
    end if
    if (quantity_error(q) /= 0) return
    do j = 1, q%cols
      do i = 1, q%rows
        r = scalar_op(element(a, i, j), element(b, i, j), op)
        if (r%err /= 0) then
          call release(q)
          q = failure(r%err)
          return
        end if
        q%e(i, j) = r%z
      end do
    end do
  end function combine

  ! UNARY is -A, +A, not A, or A!, element by element.
  function unary(a, op) result(q)
    type(quantity), intent(in) :: a
    integer, intent(in) :: op
    type(quantity) :: q
    type(value) :: r
    integer :: i, j

    if (.not. is_array(a)) then
      q = scalar_quantity(scalar_unary(a%s, op))
      return
    end if
    q = new_array(a%rows, a%cols)
    if (quantity_error(q) /= 0) return
    do j = 1, q%cols
      do i = 1, q%rows
        r = scalar_unary(element(a, i, j), op)
        if (r%err /= 0) then
          call release(q)
          q = failure(r%err)
          return
        end if
        q%e(i, j) = r%z
      end do
    end do
  end function unary

  function scalar_unary(x, op) result(v)
    type(value), intent(in) :: x
    integer, intent(in) :: op
    type(value) :: v

    select case (op)
    case (op_neg)
      v = -x
    case (op_plus)
      v = +x
    case (op_not)
      v = logical_value(.not. truth(x))
    case default
      v = factorial_value(x)
    end select
  end function scalar_unary

  ! ------------------------------------------------------------------
  ! Matrices written out.

  ! MATRIX_LITERAL is [a, b; c, d]. Each element is evaluated, and may
  ! itself be an array: the elements of a row are put side by side, so
  ! each must have as many rows as the row's first, and the rows one under
  ! another, so each must be as wide as the first. [v, 4] is v with a 4
  ! after it; [m; 0, 0] is m with a row of zeros under it.
  recursive function matrix_literal(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(node), pointer :: row, e
    integer :: total

    total = 0
    row => p%kid
    do while (associated(row))
      e => row%kid
      do while (associated(e))
        total = total + 1
        e => e%next
      end do
      row => row%next
    end do
    v = literal_parts(p, total, f)
  end function matrix_literal

  ! LITERAL_PARTS evaluates the literal P's TOTAL elements into PARTS, an
  ! automatic array of quantities, then puts them in place. Each part is
  ! released once its elements are copied.
  recursive function literal_parts(p, total, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: total
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(quantity) :: parts(total)
    type(node), pointer :: row, e
    integer :: k, h, w, width, height, r0, c0

    ! The elements, in order.
    k = 0
    row => p%kid
    do while (associated(row))
      e => row%kid
      do while (associated(e))
        k = k + 1
        parts(k) = evaluate(e, f)
        if (quantity_error(parts(k)) /= 0) then
          v = parts(k)
          call release_parts(k - 1)
          return
        end if
        e => e%next
      end do
      row => row%next
    end do

    ! The shape: each row's height is its first element's, and its width
    ! the sum of its elements'.
    k = 0
    width = -1
    height = 0
    row => p%kid
    do while (associated(row))
      e => row%kid
      h = parts(k + 1)%rows
      w = 0
      do while (associated(e))
        k = k + 1
        if (parts(k)%rows /= h) then
          call mismatch(e)
          return
        end if
        w = w + parts(k)%cols
        e => e%next
      end do
      if (width >= 0 .and. w /= width) then
        call mismatch(row%kid)
        return
      end if
      width = w
      height = height + h
      row => row%next
    end do

    if (height * width == 1) then
      v = parts(1)
      return
    end if
    if (height > max_elements / width) then
      v = failure(e_toobig)
      call release_parts(total)
      return
    end if

    ! Each part's elements into their place: a block of the result,
    ! assigned as an array section.
    v = new_array(height, width)
    k = 0
    r0 = 0
    row => p%kid
    do while (associated(row))
      e => row%kid
      h = parts(k + 1)%rows
      c0 = 0
      do while (associated(e))
        k = k + 1
        v%e(r0 + 1:r0 + h, c0 + 1:c0 + parts(k)%cols) = elements_of(parts(k))
        c0 = c0 + parts(k)%cols
        e => e%next
      end do
      r0 = r0 + h
      row => row%next
    end do
    call release_parts(total)

  contains

    subroutine release_parts(n)
      integer, intent(in) :: n
      integer :: j

      do j = 1, n
        call release(parts(j))
      end do
    end subroutine release_parts

    ! MISMATCH reports the element E's shape as not fitting.
    subroutine mismatch(e)
      type(node), pointer :: e

      call fail(e_notconform, column_of(e, f))
      v = failure(e_notconform)
      call release_parts(total)
    end subroutine mismatch
  end function literal_parts

  ! ------------------------------------------------------------------
  ! Calls.

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
  ! an index of a parameter, a user's function, or an index of a variable,
  ! in that order.
  recursive function call_function(p, at, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: at
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(quantity) :: x
    type(function_entry), pointer :: fn
    integer :: n, id

    n = argument_count(p)

    id = builtin_index(p%name)
    if (id > 0) then
      if (.not. takes_list(id) .and. .not. arity_ok(id, n)) then
        v = wrong_count(p, at)
      else if (p%name == 'if') then
        v = choose(p, f)
      else
        v = call_builtin(p, id, n, at, f)
      end if
      return
    end if

    id = matrix_index(p%name)
    if (id > 0) then
      if (.not. matrix_arity_ok(id, n)) then
        v = wrong_count(p, at)
      else
        v = call_matrix(p, id, n, f)
      end if
      return
    end if

    if (parameter_value(p%name, x, f)) then
      v = index_of(x, p, at, f)
      call release(x)
      return
    end if

    fn => find_function(p%name)
    if (associated(fn)) then
      if (fn%count /= n) then
        v = wrong_count(p, at)
      else
        v = call_user(p, fn, at, f)
      end if
      return
    end if

    if (find_variable(p%name, x)) then
      v = index_of(x, p, at, f)
      call release(x)
      return
    end if

    call fail(e_nofunc, at, text=p%name)
    v = failure(e_nofunc)
  end function call_function

  function wrong_count(p, at) result(v)
    type(node), pointer :: p
    integer, intent(in) :: at
    type(quantity) :: v

    call fail(e_argcount, at, text=p%name)
    v = failure(e_argcount)
  end function wrong_count

  ! CHOOSE is if(c, a, b): A's value if C is true (not zero; an array is
  ! true if no element is zero), and B's if not. Only the arm chosen is
  ! evaluated, so a recursive function can stop: in if(n < 2, 1, n *
  ! fact(n - 1)) the call is made only while n is 2 or more.
  recursive function choose(p, f) result(v)
    type(node), pointer :: p
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(node), pointer :: arm
    logical :: chosen

    v = evaluate(p%kid, f)
    if (quantity_error(v) /= 0) return
    chosen = quantity_truth(v)
    call release(v)
    if (chosen) then
      arm => p%kid%next
    else
      arm => p%kid%next%next
    end if
    v = evaluate(arm, f)
  end function choose

  ! EVALUATE_ARGUMENTS evaluates the call P's N arguments into ARGS; if one
  ! fails, those before it are released and the result is false, BAD the
  ! failure.
  recursive logical function evaluate_arguments(p, n, args, bad, f) result(ok)
    type(node), pointer :: p
    integer, intent(in) :: n
    type(quantity), intent(out) :: args(n)
    type(quantity), intent(out) :: bad
    type(frame), intent(in), optional :: f
    type(node), pointer :: a
    integer :: k, j

    ok = .true.
    bad = real_quantity(0.0_rk)
    a => p%kid
    do k = 1, n
      args(k) = evaluate(a, f)
      if (quantity_error(args(k)) /= 0) then
        bad = args(k)
        do j = 1, k - 1
          call release(args(j))
        end do
        ok = .false.
        return
      end if
      a => a%next
    end do
  end function evaluate_arguments

  subroutine release_all(args)
    type(quantity), intent(inout) :: args(:)
    integer :: k

    do k = 1, size(args)
      call release(args(k))
    end do
  end subroutine release_all

  ! CALL_BUILTIN evaluates the call's N arguments into ARGS, then has
  ! FCFUNCS apply the function to them: to every element of every argument
  ! together, for a function of a list; to the scalars as they are; or to
  ! each element of the arrays in turn (with the scalars), making an array
  ! of the results. ARGS is an automatic array: its size, N, is a dummy
  ! argument, so each call has an array of the size it needs, made on
  ! entry and gone on return.
  recursive function call_builtin(p, id, n, at, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: id, n, at
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(quantity) :: args(n)
    type(value) :: scalars(n)
    integer :: k, total

    if (.not. evaluate_arguments(p, n, args, v, f)) return

    if (takes_list(id)) then
      total = 0
      do k = 1, n
        total = total + numel(args(k))
      end do
      if (.not. arity_ok(id, total)) then
        v = wrong_count(p, at)
      else
        v = apply_list(id, args, total)
      end if
    else if (any(args%rows * args%cols > 1)) then
      v = map_builtin(id, args)
    else
      do k = 1, n
        scalars(k) = args(k)%s
      end do
      v = scalar_quantity(apply_builtin(id, scalars))
    end if
    call release_all(args)
  end function call_builtin

  ! APPLY_LIST applies the list function ID to every element of ARGS, in
  ! order, TOTAL in all.
  function apply_list(id, args, total) result(v)
    integer, intent(in) :: id, total
    type(quantity), intent(in) :: args(:)
    type(quantity) :: v
    type(value) :: list(total)
    integer :: j, k, n

    n = 0
    do k = 1, size(args)
      do j = 1, numel(args(k))
        n = n + 1
        list(n) = element_at(args(k), j)
      end do
    end do
    v = scalar_quantity(apply_builtin(id, list))
  end function apply_list

  ! MAP_BUILTIN applies the function ID to each element of ARGS' arrays,
  ! which must all have one shape, with each scalar argument as it is.
  function map_builtin(id, args) result(q)
    integer, intent(in) :: id
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    type(value) :: these(size(args)), r
    integer :: i, j, k, shape_of

    shape_of = 0
    do k = 1, size(args)
      if (.not. is_array(args(k))) cycle
      if (shape_of == 0) then
        shape_of = k
      else if (.not. same_shape(args(k), args(shape_of))) then
        q = failure(e_notconform)
        return
      end if
    end do

    q = new_array(args(shape_of)%rows, args(shape_of)%cols)
    if (quantity_error(q) /= 0) return
    do j = 1, q%cols
      do i = 1, q%rows
        do k = 1, size(args)
          these(k) = element(args(k), i, j)
        end do
        r = apply_builtin(id, these)
        if (r%err /= 0) then
          call release(q)
          q = failure(r%err)
          return
        end if
        q%e(i, j) = r%z
      end do
    end do
  end function map_builtin

  ! CALL_MATRIX is a call of one of FCMATRIX's functions.
  recursive function call_matrix(p, id, n, f) result(v)
    type(node), pointer :: p
    integer, intent(in) :: id, n
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(quantity) :: args(n)

    if (.not. evaluate_arguments(p, n, args, v, f)) return
    v = apply_matrix(id, args)
    call release_all(args)
  end function call_matrix

  ! CALL_USER is the call P of the user's function FN: its arguments,
  ! evaluated in the caller's frame F, become a new frame, INNER, in which
  ! FN's tree is evaluated. INNER owns its values, and releases them when
  ! the call is done.
  recursive function call_user(p, fn, at, f) result(v)
    type(node), pointer :: p
    type(function_entry), pointer :: fn
    integer, intent(in) :: at
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    type(frame) :: inner

    inner%count = fn%count
    inner%column = at
    inner%names(1:fn%count) = fn%params(1:fn%count)
    if (.not. evaluate_arguments(p, fn%count, inner%values, v, f)) return

    if (calls >= call_limit) then
      v = failure(e_recursion)
    else
      calls = calls + 1
      v = evaluate(fn%body, inner)
      calls = calls - 1
    end if
    call release_all(inner%values(1:fn%count))
  end function call_user

  ! ------------------------------------------------------------------
  ! Indexes.

  ! INDEX_OF is X(...), the call P's arguments its indexes: one, which
  ! counts X's elements in array element order (down each column in turn),
  ! or two, a row and a column. Each index is a number, an array of
  ! numbers (a vector subscript, picking those elements in that order), or
  ! a range. A single index gives a row of the elements it picks, or a
  ! column if X is a column.
  recursive function index_of(x, p, at, f) result(v)
    type(quantity), intent(in) :: x
    type(node), pointer :: p
    integer, intent(in) :: at
    type(frame), intent(in), optional :: f
    type(quantity) :: v
    integer, pointer :: list(:), rows(:), cols(:)
    integer :: err

    select case (argument_count(p))
    case (1)
      if (.not. index_list(p%kid, numel(x), list, err, f)) then
        v = index_failure(err, at)
        return
      end if
      v = picked(x, list)
      deallocate (list)
    case (2)
      if (.not. index_list(p%kid, x%rows, rows, err, f)) then
        v = index_failure(err, at)
        return
      end if
      if (.not. index_list(p%kid%next, x%cols, cols, err, f)) then
        deallocate (rows)
        v = index_failure(err, at)
        return
      end if
      v = section(x, rows, cols)
      deallocate (rows, cols)
    case default
      v = wrong_count(p, at)
    end select
  end function index_of

  function index_failure(err, at) result(v)
    integer, intent(in) :: err, at
    type(quantity) :: v

    call fail(err, at)
    v = failure(err)
  end function index_failure

  ! PICKED is the elements of X that LIST names, counted in array element
  ! order: X laid out flat by RESHAPE, then picked by LIST as a vector
  ! subscript, FLAT(LIST).
  function picked(x, list) result(v)
    type(quantity), intent(in) :: x
    integer, intent(in) :: list(:)
    type(quantity) :: v
    complex(rk) :: flat(x%rows * x%cols)

    flat = reshape(elements_of(x), (/ numel(x) /))
    if (x%cols == 1 .and. x%rows > 1) then
      v = array_of(reshape(flat(list), (/ size(list), 1 /)))
    else
      v = array_of(reshape(flat(list), (/ 1, size(list) /)))
    end if
  end function picked

  ! SECTION is X(ROWS, COLS): with two vector subscripts, the elements of
  ! every row named in ROWS and column named in COLS.
  function section(x, rows, cols) result(v)
    type(quantity), intent(in) :: x
    integer, intent(in) :: rows(:), cols(:)
    type(quantity) :: v
    complex(rk) :: z(size(rows), size(cols))

    if (is_array(x)) then
      v = array_of(x%e(rows, cols))
    else
      z = x%s%z
      v = array_of(z)
    end if
  end function section

  ! INDEX_LIST makes LIST the positions the index P names, each from 1 to
  ! EXTENT: a range's (lo:hi:stride, lo 1, hi EXTENT, and stride 1 if left
  ! out, as in a Fortran array section), or an expression's values. The
  ! result is false if an index is out of range or not a whole number, ERR
  ! the reason; LIST is then not allocated.
  !
  ! LIST is a pointer dummy argument: Fortran 90 doesn't let a dummy be
  ! ALLOCATABLE (Fortran 2003 does), so a routine that allocates an array
  ! for its caller allocates a pointer's target, which the caller must
  ! DEALLOCATE.
  recursive logical function index_list(p, extent, list, err, f) result(ok)
    type(node), pointer :: p
    integer, intent(in) :: extent
    integer, pointer :: list(:)
    integer, intent(out) :: err
    type(frame), intent(in), optional :: f
    type(node), pointer :: part
    type(quantity) :: x
    type(value) :: v
    integer :: lo, hi, stride, n, k

    ok = .false.
    err = 0
    nullify (list)

    if (p%kind == n_range) then
      lo = 1
      hi = extent
      stride = 1
      part => p%kid
      if (iand(p%op, range_lo) /= 0) then
        if (.not. index_number(part, lo, err, f)) return
        part => part%next
      end if
      if (iand(p%op, range_hi) /= 0) then
        if (.not. index_number(part, hi, err, f)) return
        part => part%next
      end if
      if (iand(p%op, range_stride) /= 0) then
        if (.not. index_number(part, stride, err, f)) return
      end if

      ! The count, as a DO loop's: (hi - lo + stride) / stride, or none.
      if (stride == 0) then
        err = e_domain
        return
      end if
      n = (hi - lo + stride) / stride
      err = e_index
      if (n < 1) return
      if (lo < 1 .or. lo > extent) return
      if (lo + (n - 1) * stride < 1 .or. lo + (n - 1) * stride > extent) return
      err = 0
      ! An array constructor with an implied DO: the N values
      ! LO, LO + STRIDE, ..., made as the DO would make them.
      allocate (list(n))
      list = (/ (lo + (k - 1) * stride, k = 1, n) /)
      ok = .true.
      return
    end if

    x = evaluate(p, f)
    if (quantity_error(x) /= 0) then
      err = quantity_error(x)
      return
    end if
    allocate (list(numel(x)))
    do k = 1, numel(x)
      v = element_at(x, k)
      if (.not. whole_index(v, list(k), err)) exit
      if (list(k) < 1 .or. list(k) > extent) err = e_index
      if (err /= 0) exit
    end do
    call release(x)
    if (err /= 0) then
      deallocate (list)
      return
    end if
    ok = .true.
  end function index_list

  ! INDEX_NUMBER is one part of a range, a whole number K.
  recursive logical function index_number(p, k, err, f) result(ok)
    type(node), pointer :: p
    integer, intent(out) :: k
    integer, intent(out) :: err
    type(frame), intent(in), optional :: f
    type(quantity) :: x

    ok = .false.
    k = 0
    x = evaluate(p, f)
    err = quantity_error(x)
    if (err /= 0) return
    if (is_array(x)) then
      err = e_notscalar
    else
      ok = whole_index(x%s, k, err)
    end if
    call release(x)
  end function index_number

  ! WHOLE_INDEX: V is a whole number small enough to be an index, K.
  logical function whole_index(v, k, err)
    type(value), intent(in) :: v
    integer, intent(out) :: k
    integer, intent(inout) :: err
    real(rk) :: x

    whole_index = .false.
    k = 0
    x = real(v%z, rk)
    if (.not. is_real(v)) then
      err = e_notreal
    else if (aint(x) /= x) then
      err = e_notwhole
    else if (abs(x) > 1.0e9_rk) then
      err = e_index
    else
      k = nint(x)
      whole_index = .true.
    end if
  end function whole_index

  ! ASSIGN_INTO is X(...) = Y, the call P's arguments the indexes, as
  ! INDEX_OF reads them; Y is a scalar, given to every element named, or an
  ! array with as many elements as are named, given to them in order. The
  ! result is false after an error, which is reported.
  logical function assign_into(x, p, y) result(ok)
    type(quantity), intent(inout) :: x
    type(node), pointer :: p
    type(quantity), intent(in) :: y
    integer, pointer :: list(:), rows(:), cols(:)
    integer :: err, a, b, k, count

    ok = .false.
    select case (argument_count(p))
    case (1)
      if (.not. index_list(p%kid, numel(x), list, err)) then
        call fail(err, p%column)
        return
      end if
      count = size(list)
      if (is_array(y) .and. numel(y) /= count) then
        call fail(e_notconform, p%column)
      else
        do k = 1, count
          call put(list(k), k)
        end do
        ok = .true.
      end if
      deallocate (list)
    case (2)
      if (.not. index_list(p%kid, x%rows, rows, err)) then
        call fail(err, p%column)
        return
      end if
      if (.not. index_list(p%kid%next, x%cols, cols, err)) then
        call fail(err, p%column)
        deallocate (rows)
        return
      end if
      count = size(rows) * size(cols)
      if (is_array(y) .and. numel(y) /= count) then
        call fail(e_notconform, p%column)
      else
        k = 0
        do b = 1, size(cols)
          do a = 1, size(rows)
            k = k + 1
            call put(rows(a) + (cols(b) - 1) * x%rows, k)
          end do
        end do
        ok = .true.
      end if
      deallocate (rows, cols)
    case default
      call fail(e_argcount, p%column, p%name)
    end select

  contains

    ! PUT gives X's Lth element (in array element order) Y's Kth.
    subroutine put(l, k)
      integer, intent(in) :: l, k
      type(value) :: v

      v = element_at(y, k)
      if (is_array(x)) then
        x%e(mod(l - 1, x%rows) + 1, (l - 1) / x%rows + 1) = v%z
      else
        x%s = v
      end if
    end subroutine put
  end function assign_into
end module fceval
