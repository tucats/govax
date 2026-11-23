! fcalc: expressions as trees, and the parser that builds them.
!
! A NODE is a number, a name, a function's call, or an operation. Its
! operands, or a call's arguments, are a list: KID points to the first, and
! each one's NEXT to the one after it. The parser is recursive descent, a
! function for each level of the grammar, loosest first:
!
!     or-expr     and-expr { or and-expr }
!     and-expr    not-expr { and not-expr }
!     not-expr    not not-expr | comparison
!     comparison  sum [ (== | <> | < | <= | > | >=) sum ]
!     sum         term { (+ | -) term }
!     term        unary { (* | / | mod | .* | ./ | \) unary }
!     unary       (- | +) unary | power
!     power       postfix [ (^ | .^) unary ]
!     postfix     primary { ! | ' | .' }
!     primary     number | name | name ( [ argument { , argument } ] )
!                 | ( or-expr ) | [ row { ; row } ]
!     row         or-expr { , or-expr }
!     argument    or-expr | [ or-expr ] : [ or-expr ] [ : or-expr ]
!
! so ^ binds tighter than a sign (-2^2 is -4) but takes one after it (2^-1),
! and is right-associative (2^3^2 is 2^9). Each function returns a tree, or
! a null pointer after calling FAIL, having freed what it had built.
!
! A bracket makes a matrix, its rows parted by semicolons and the elements
! of a row by commas: [1, 2; 3, 4]. An argument may be a range, lo:hi or
! lo:hi:stride with any part left out, which only an index may have:
! m(2, :) is the second row of m, v(1:5:2) every other element of v's first
! five.
!
! Recursive descent, briefly: each rule of the grammar above becomes a
! function that parses that rule, calling the functions of the rules it is
! made of. The order of the rules gives the precedence: PARSE_SUM calls
! PARSE_TERM for its operands, so in 1 + 2 * 3 the 2 * 3 is a term, parsed
! first and placed lower in the tree. A loop (the { } of a rule) makes an
! operator left-associative: 2 - 3 - 4 is (2 - 3) - 4, each new operation
! taking the tree so far as its left operand. A rule that calls itself for
! its right operand, as POWER does through UNARY, makes it right-
! associative. A parenthesis starts again at the loosest rule, OR-EXPR.
!
! Pointers in Fortran 90. A variable declared POINTER doesn't hold a value
! itself: it is associated with (points to) a target, or is null.
! ALLOCATE (p) makes a new target and points P at it; DEALLOCATE (p) gives
! it back; NULLIFY (p) makes P null; ASSOCIATED(p) asks whether P points
! anywhere; p => q makes P point where Q does (pointer assignment), while
! p = q copies Q's value into P's target. A derived type may have
! components that are pointers to its own type, which is how linked lists
! and trees are built: a NODE here points to its first operand (KID) and to
! the next node in its list (NEXT). Fortran 90 has no garbage collector:
! each node ALLOCATEd must be DEALLOCATEd when it is no longer needed, by
! FREE_TREE, or the memory is lost (a leak) until the program ends.
module fctree
  use fckinds
  use fcmessages
  use fcvalues
  use fclexer
  implicit none
  private

  ! VALUE and TOKEN are public here too: a public type's component, or a
  ! public function's value, may not be of a type private in this module,
  ! as the default PRIVATE would make them.
  public :: value, token
  public :: node, scan_line, parse_expression, free_tree, copy_tree
  public :: token_at, token_count, kind_at, current_column, position, advance, at_end, is_operator
  public :: n_number, n_name, n_call, n_operation, n_matrix, n_row, n_range
  public :: op_add, op_sub, op_mul, op_div, op_mod, op_power, op_neg, op_plus, &
    op_not, op_and, op_or, op_compare, op_factorial, op_emul, op_ediv, &
    op_epower, op_solve, op_transpose, op_ctranspose
  public :: range_lo, range_hi, range_stride, nodes_in_use

  ! The kinds of node. A matrix's operands are its rows, and a row's its
  ! elements. A range's operands are the parts it was given, in order, and
  ! its OP says which those are: RANGE_LO, RANGE_HI, and RANGE_STRIDE added
  ! together, so 1:5 is 3 and :: 2 is 4.
  integer, parameter :: n_number = 1, n_name = 2, n_call = 3, n_operation = 4, &
    n_matrix = 5, n_row = 6, n_range = 7
  integer, parameter :: range_lo = 1, range_hi = 2, range_stride = 4

  ! The operations. A comparison is OP_COMPARE plus its relation, 1 to 6
  ! (V_COMPARE's).
  integer, parameter :: op_add = 1, op_sub = 2, op_mul = 3, op_div = 4, &
    op_mod = 5, op_power = 6, op_neg = 7, op_plus = 8, op_not = 9, &
    op_and = 10, op_or = 11, op_factorial = 12, op_emul = 13, op_ediv = 14, &
    op_epower = 15, op_solve = 16, op_transpose = 17, op_ctranspose = 18, &
    op_compare = 20

  ! A node: what it is (KIND), its operation (OP, for an operation), the
  ! column it came from, a name's or call's NAME, a number's value, and its
  ! operands or arguments (KID, then each one's NEXT). In Fortran 90 a
  ! component can't be given an initial value (that came in Fortran 95), so
  ! NEW_NODE sets every component, the pointers to null, as it makes one.
  type node
    integer :: kind
    integer :: op
    integer :: column
    character(len=name_length) :: name
    type(value) :: number
    type(node), pointer :: kid
    type(node), pointer :: next
  end type node

  ! How deeply expressions may nest.
  integer, parameter :: depth_limit = 100

  ! The line's tokens, and the parser's place in them.
  type(token), save :: tokens(line_length + 1)
  integer, save :: count = 0, pos = 1, depth = 0

  ! The nodes made and not yet freed, for SHOW MEMORY.
  integer, save :: nodes = 0

  character(len=2), parameter :: relations(6) = (/ '==', '<>', '< ', '<=', '> ', '>=' /)

contains

  ! ------------------------------------------------------------------
  ! The tokens.

  ! SCAN_LINE tokenizes a line for the parser, which starts at its first
  ! token.
  subroutine scan_line(line)
    character(len=*), intent(in) :: line

    call tokenize(line, tokens, count)
    pos = 1
    depth = 0
  end subroutine scan_line

  function token_at(k) result(t)
    integer, intent(in) :: k
    type(token) :: t

    t = tokens(min(k, count))
  end function token_at

  ! KIND_AT is the kind of the Kth token.
  integer function kind_at(k)
    integer, intent(in) :: k

    kind_at = tokens(min(k, count))%kind
  end function kind_at

  ! CURRENT_COLUMN is the column of the parser's current token.
  integer function current_column()
    current_column = tokens(pos)%column
  end function current_column

  integer function token_count()
    token_count = count
  end function token_count

  integer function position()
    position = pos
  end function position

  subroutine advance(k)
    integer, intent(in) :: k

    pos = min(pos + k, count)
  end subroutine advance

  logical function at_end()
    at_end = tokens(pos)%kind == t_end
  end function at_end

  ! IS_OPERATOR: the current token is the operator TEXT.
  logical function is_operator(text)
    character(len=*), intent(in) :: text

    is_operator = tokens(pos)%kind == t_operator .and. tokens(pos)%text == text
  end function is_operator

  ! IS_WORD: the current token is the name TEXT (or, and, not, mod).
  logical function is_word(text)
    character(len=*), intent(in) :: text

    is_word = tokens(pos)%kind == t_name .and. tokens(pos)%text == text
  end function is_word

  ! ------------------------------------------------------------------
  ! Nodes.

  integer function nodes_in_use()
    nodes_in_use = nodes
  end function nodes_in_use

  function new_node(kind, column) result(p)
    integer, intent(in) :: kind, column
    type(node), pointer :: p

    allocate (p)
    nodes = nodes + 1
    p%kind = kind
    p%op = 0
    p%column = column
    p%name = ' '
    p%number = real_value(0.0_rk)
    nullify (p%kid, p%next)
  end function new_node

  ! OPERATION makes a node of an operation on LEFT (and RIGHT, if it is
  ! given). If the parse has failed, it frees them instead and returns a
  ! null pointer.
  function operation(op, column, left, right) result(p)
    integer, intent(in) :: op, column
    type(node), pointer :: left
    type(node), pointer, optional :: right
    type(node), pointer :: p

    if (failed()) then
      call free_tree(left)
      if (present(right)) call free_tree(right)
      nullify (p)
      return
    end if

    p => new_node(n_operation, column)
    p%op = op
    p%kid => left
    if (present(right)) left%next => right
  end function operation

  ! FREE_TREE frees a tree and the trees after it in its list: each node's
  ! operands first (recursively), then the node. NEXT is saved before the
  ! node is deallocated, since the node's components are gone after. P is
  ! null when it returns.
  recursive subroutine free_tree(p)
    type(node), pointer :: p
    type(node), pointer :: following

    do while (associated(p))
      call free_tree(p%kid)
      following => p%next
      deallocate (p)
      nodes = nodes - 1
      p => following
    end do
  end subroutine free_tree

  ! COPY_TREE is a copy of the tree P (but not the trees after it). Q = P
  ! copies every component, the pointers too, so the copy's KID and NEXT are
  ! nullified at once and the operands copied one by one: a copy must share
  ! no node with the original, or freeing one would free part of the other.
  ! A function whose result is a POINTER is used with =>, as q => copy_tree(p).
  recursive function copy_tree(p) result(q)
    type(node), pointer :: p
    type(node), pointer :: q
    type(node), pointer :: from, last, made

    nullify (q)
    if (.not. associated(p)) return

    allocate (q)
    nodes = nodes + 1
    q = p
    nullify (q%kid, q%next)

    nullify (last)
    from => p%kid
    do while (associated(from))
      made => copy_tree(from)
      if (associated(last)) then
        last%next => made
      else
        q%kid => made
      end if
      last => made
      from => from%next
    end do
  end function copy_tree

  ! ------------------------------------------------------------------
  ! The parser.

  ! PARSE_EXPRESSION parses an expression from the current token.
  function parse_expression() result(p)
    type(node), pointer :: p

    p => parse_or()
  end function parse_expression

  recursive function parse_or() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column

    p => parse_and()
    do while (.not. failed() .and. is_word('or'))
      column = tokens(pos)%column
      call advance(1)
      right => parse_and()
      p => operation(op_or, column, p, right)
    end do
  end function parse_or

  recursive function parse_and() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column

    p => parse_not()
    do while (.not. failed() .and. is_word('and'))
      column = tokens(pos)%column
      call advance(1)
      right => parse_not()
      p => operation(op_and, column, p, right)
    end do
  end function parse_and

  recursive function parse_not() result(p)
    type(node), pointer :: p
    type(node), pointer :: operand
    integer :: column

    if (is_word('not')) then
      column = tokens(pos)%column
      call advance(1)
      operand => parse_not()
      p => operation(op_not, column, operand)
    else
      p => parse_comparison()
    end if
  end function parse_not

  recursive function parse_comparison() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column, k

    p => parse_sum()
    if (failed()) return
    do k = 1, size(relations)
      if (is_operator(trim(relations(k)))) then
        column = tokens(pos)%column
        call advance(1)
        right => parse_sum()
        p => operation(op_compare + k, column, p, right)
        return
      end if
    end do
  end function parse_comparison

  recursive function parse_sum() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column, op

    p => parse_term()
    do while (.not. failed())
      if (is_operator('+')) then
        op = op_add
      else if (is_operator('-')) then
        op = op_sub
      else
        exit
      end if
      column = tokens(pos)%column
      call advance(1)
      right => parse_term()
      p => operation(op, column, p, right)
    end do
  end function parse_sum

  recursive function parse_term() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column, op

    p => parse_unary()
    do while (.not. failed())
      if (is_operator('*')) then
        op = op_mul
      else if (is_operator('/')) then
        op = op_div
      else if (is_word('mod')) then
        op = op_mod
      else if (is_operator('.*')) then
        op = op_emul
      else if (is_operator('./')) then
        op = op_ediv
      else if (is_operator(achar(92))) then
        op = op_solve
      else
        exit
      end if
      column = tokens(pos)%column
      call advance(1)
      right => parse_unary()
      p => operation(op, column, p, right)
    end do
  end function parse_term

  ! PARSE_UNARY is where the parser counts how deeply it has gone: every
  ! parenthesis and every sign passes through here.
  recursive function parse_unary() result(p)
    type(node), pointer :: p
    type(node), pointer :: operand
    integer :: column, op

    nullify (p)
    depth = depth + 1
    if (depth > depth_limit) then
      call fail(e_toodeep, tokens(pos)%column)
    else if (is_operator('-') .or. is_operator('+')) then
      op = op_neg
      if (is_operator('+')) op = op_plus
      column = tokens(pos)%column
      call advance(1)
      operand => parse_unary()
      p => operation(op, column, operand)
    else
      p => parse_power()
    end if
    depth = depth - 1
  end function parse_unary

  recursive function parse_power() result(p)
    type(node), pointer :: p
    type(node), pointer :: right
    integer :: column

    integer :: op

    p => parse_postfix()
    if (failed()) return
    if (is_operator('^') .or. is_operator('.^')) then
      op = op_power
      if (is_operator('.^')) op = op_epower
      column = tokens(pos)%column
      call advance(1)
      right => parse_unary()
      p => operation(op, column, p, right)
    end if
  end function parse_power

  recursive function parse_postfix() result(p)
    type(node), pointer :: p
    integer :: op

    p => parse_primary()
    do while (.not. failed())
      if (is_operator('!')) then
        op = op_factorial
      else if (is_operator("'")) then
        op = op_ctranspose
      else if (is_operator(".'")) then
        op = op_transpose
      else
        exit
      end if
      p => operation(op, tokens(pos)%column, p)
      call advance(1)
    end do
  end function parse_postfix

  recursive function parse_primary() result(p)
    type(node), pointer :: p
    type(token) :: t

    nullify (p)
    t = tokens(pos)

    select case (t%kind)
    case (t_number)
      p => new_node(n_number, t%column)
      if (t%imaginary) then
        p%number = complex_value(0.0_rk, t%number)
      else
        p%number = real_value(t%number)
      end if
      call advance(1)

    case (t_name)
      if (t%text == 'or' .or. t%text == 'and' .or. t%text == 'not' .or. &
          t%text == 'mod') then
        call fail(e_operand, t%column)
        return
      end if
      call advance(1)
      if (is_operator('(')) then
        p => parse_call(t)
      else
        p => new_node(n_name, t%column)
        p%name = t%text
      end if

    case (t_operator)
      if (t%text == '[') then
        p => parse_matrix()
        return
      end if
      if (t%text /= '(') then
        call fail(e_operand, t%column)
        return
      end if
      call advance(1)
      p => parse_or()
      if (failed()) return
      if (.not. is_operator(')')) then
        call fail(e_rparen, tokens(pos)%column)
        call free_tree(p)
        return
      end if
      call advance(1)

    case default
      call fail(e_operand, t%column)
    end select
  end function parse_primary

  ! PARSE_CALL parses a call's arguments, the current token its (.
  recursive function parse_call(t) result(p)
    type(token), intent(in) :: t
    type(node), pointer :: p
    type(node), pointer :: argument, last

    p => new_node(n_call, t%column)
    p%name = t%text
    call advance(1)
    if (is_operator(')')) then
      call advance(1)
      return
    end if

    nullify (last)
    do
      argument => parse_argument()
      if (failed()) then
        call free_tree(p)
        return
      end if
      if (associated(last)) then
        last%next => argument
      else
        p%kid => argument
      end if
      last => argument
      if (.not. is_operator(',')) exit
      call advance(1)
    end do

    if (.not. is_operator(')')) then
      call fail(e_rparen, tokens(pos)%column)
      call free_tree(p)
      return
    end if
    call advance(1)
  end function parse_call

  ! PARSE_ARGUMENT parses one of a call's arguments: an expression, or a
  ! range, whose parts are each an expression or nothing.
  recursive function parse_argument() result(p)
    type(node), pointer :: p
    type(node), pointer :: r, part, last
    integer :: column

    nullify (p)
    column = tokens(pos)%column
    if (.not. is_operator(':')) then
      p => parse_or()
      if (failed() .or. .not. is_operator(':')) return
    end if

    ! A range: P, if there is one, is its first part.
    r => new_node(n_range, column)
    if (associated(p)) then
      r%kid => p
      r%op = range_lo
    end if
    last => p
    call advance(1)

    if (.not. (is_operator(',') .or. is_operator(')') .or. is_operator(':'))) then
      part => parse_or()
      call attach(range_hi)
      if (failed()) return
    end if
    if (is_operator(':')) then
      call advance(1)
      part => parse_or()
      call attach(range_stride)
      if (failed()) return
    end if
    p => r

  contains

    ! ATTACH adds PART after LAST in R's operands, or frees R if the parse
    ! has failed.
    subroutine attach(which)
      integer, intent(in) :: which

      if (failed()) then
        call free_tree(r)
        nullify (p)
        return
      end if
      if (associated(last)) then
        last%next => part
      else
        r%kid => part
      end if
      last => part
      r%op = r%op + which
    end subroutine attach
  end function parse_argument

  ! PARSE_MATRIX parses [ row { ; row } ], the current token its [: a node
  ! N_MATRIX whose operands are N_ROW nodes, each with its elements as its
  ! operands.
  recursive function parse_matrix() result(p)
    type(node), pointer :: p
    type(node), pointer :: row, element, last_row, last

    p => new_node(n_matrix, tokens(pos)%column)
    call advance(1)
    nullify (last_row)
    do
      row => new_node(n_row, tokens(pos)%column)
      if (associated(last_row)) then
        last_row%next => row
      else
        p%kid => row
      end if
      last_row => row

      nullify (last)
      do
        element => parse_or()
        if (failed()) then
          call free_tree(p)
          return
        end if
        if (associated(last)) then
          last%next => element
        else
          row%kid => element
        end if
        last => element
        if (.not. is_operator(',')) exit
        call advance(1)
      end do

      if (.not. is_operator(';')) exit
      call advance(1)
    end do

    if (.not. is_operator(']')) then
      call fail(e_rbracket, tokens(pos)%column)
      call free_tree(p)
      return
    end if
    call advance(1)
  end function parse_matrix
end module fctree
