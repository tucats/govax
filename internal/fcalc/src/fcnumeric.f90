! fcalc: the numerical commands, SOLVE, INTEGRATE, DIFF, and ROOTS.
!
!     SOLVE expr [= expr] FOR x [FROM a [TO b]]
!     INTEGRATE expr FOR x FROM a TO b
!     DIFF expr FOR x AT a
!     ROOTS c1, c2, ..., cn
!
! Each of the first three treats an expression as a function of the
! variable named after FOR and evaluates it at the points its method
! chooses, through FCEVAL's EVALUATE_WITH: the name stands for the point,
! hiding any variable of that name, which the command doesn't change. Its
! answer is shown and kept as ANS, as an expression's value is.
!
! - SOLVE finds a root of the expression (of the difference of the two
!   sides, if there is an =). From one starting point A (0 if none is
!   given) it uses the secant method, which may find a complex root from a
!   complex start; from a bracket, A to B, where the expression is real and
!   changes sign, Brent's method, which cannot fail to converge.
! - INTEGRATE is the definite integral from A to B, by adaptive
!   Gauss-Kronrod quadrature.
! - DIFF is the derivative at A, by Ridders' method: central differences
!   with smaller and smaller steps, extrapolated to a step of zero.
! - ROOTS is every root of the polynomial with the coefficients given, the
!   highest power's first (module FCPOLY), as the function roots() is.
!
! Each method's numbers are computed by FCVALUES' guarded operations, as
! everything in fcalc is, so nothing traps, whatever the expression's
! values; an error in evaluating the expression is reported at the node of
! the expression it arose in, as for any expression.
!
! Two answers depend on the machine's last bits more than others, and each
! command deals with that itself, so that a VAX and an IEEE machine show
! the same digits:
!
! - A root, integral, or derivative that is 0 is computed as a tiny number
!   made of rounding errors. Each method knows how large its own error may
!   be, and a part of its answer smaller than that is set to 0.
! - Each method's error estimate is a rounding error's size when the answer
!   is good. INTEGRATE and DIFF show it only when it is large, in a
!   warning, %FCALC-W-INACCURATE, to two significant digits.
!
! The procedures that read the commands are written in one pattern: a DO
! construct with no loop control and a name, as a block that can be left.
! Each step that fails does EXIT out of it, and what was built is freed
! after the block, once, whichever step failed (as a C program might use
! GO TO to a cleanup label).
module fcnumeric
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  use fclexer
  use fctree
  use fcsymbols
  use fceval
  use fcmatrix
  implicit none
  private

  public :: solve_command, integrate_command, diff_command, roots_command

  ! The readers of the commands' parts, which module FCPLOT's commands use
  ! too. VALUE is public here too: READ_BOUND's argument is one, and a
  ! public procedure's argument may not be of a type private in this module.
  public :: value
  public :: at_word, read_word, read_name, read_bound, at_end_of_line

  ! How many steps the secant method and Brent's method may take.
  integer, parameter :: step_limit = 200

  ! How many pieces INTEGRATE may cut its interval into, at 15 values of
  ! the expression for each piece and 30 for each cut. (A singularity at
  ! an end, as 1/SQRT(X)'s at 0, takes about 100.)
  integer, parameter :: max_pieces = 200

  ! The Gauss-Kronrod rule of 7 and 15 points on [-1, 1] (QUADPACK's QK15):
  ! the Kronrod rule's nodes, plus and minus each of NODES(1:7), and 0; its
  ! weights for them, the last 0's; and the Gauss rule's weights for its
  ! nodes, NODES(2), NODES(4), NODES(6), and 0, the last 0's.
  real(rk), parameter :: nodes(7) = (/ 0.991455371120812639206854697526329_rk, &
    0.949107912342758524526189684047851_rk, 0.864864423359769072789712788640926_rk, &
    0.741531185599394439863864773280788_rk, 0.586087235467691130294144845693013_rk, &
    0.405845151377397166906606412076961_rk, 0.207784955007898467600689403773245_rk /)
  real(rk), parameter :: kronrod_weights(8) = (/ 0.022935322010529224963732008058970_rk, &
    0.063092092629978553290700663189204_rk, 0.104790010322250183839876322541518_rk, &
    0.140653259715525918745189590510238_rk, 0.169004726639267902826583426598550_rk, &
    0.190350578064785409913256402421014_rk, 0.204432940075298892414161999234649_rk, &
    0.209482141084727828012999174891714_rk /)
  real(rk), parameter :: gauss_weights(4) = (/ 0.129484966168869693270611432679082_rk, &
    0.279705391489276667901467771423780_rk, 0.381830050505118944950369775488975_rk, &
    0.417959183673469387755102040816327_rk /)

  ! Ridders' method: each step is the last divided by RIDDERS_STEP, at most
  ! TABLE_SIZE steps, stopping when the error grows SAFE times past its
  ! smallest.
  real(rk), parameter :: ridders_step = 1.4_rk, safe = 2.0_rk
  integer, parameter :: table_size = 10

contains

  ! ------------------------------------------------------------------
  ! Reading the commands.

  ! AT_WORD: the current token is the name WORD.
  logical function at_word(word)
    character(len=*), intent(in) :: word
    type(token) :: t

    t = token_at(position())
    at_word = t%kind == t_name .and. t%text == word
  end function at_word

  ! READ_WORD: the current token is the name WORD, and is passed; if not,
  ! KEYWORD (with WORD in capitals, as the message shows it).
  logical function read_word(word, shown)
    character(len=*), intent(in) :: word, shown

    read_word = at_word(word)
    if (read_word) then
      call advance(1)
    else
      call fail(e_keyword, current_column(), shown)
    end if
  end function read_word

  ! READ_NAME: the current token is a name a variable could have, NAME,
  ! and is passed.
  logical function read_name(name)
    character(len=*), intent(out) :: name
    type(token) :: t

    read_name = .false.
    name = ' '
    t = token_at(position())
    if (t%kind /= t_name) then
      call fail(e_namereq, t%column)
    else if (t%text == 'or' .or. t%text == 'and' .or. t%text == 'not' .or. &
        t%text == 'mod') then
      call fail(e_reserved, t%column, t%text)
    else if (is_constant(t%text)) then
      call fail(e_constant, t%column, t%text)
    else
      name = t%text
      call advance(1)
      read_name = .true.
    end if
  end function read_name

  ! READ_BOUND: an expression, read and evaluated (with no variable of the
  ! command's), is a scalar, X; if REAL_ONLY, a real one. COLUMN is where it
  ! starts.
  logical function read_bound(x, real_only, column)
    type(value), intent(out) :: x
    logical, intent(in) :: real_only
    integer, intent(out) :: column
    type(node), pointer :: tree
    type(quantity) :: q

    read_bound = .false.
    x = real_value(0.0_rk)
    column = current_column()
    tree => parse_expression()
    if (failed()) return
    q = evaluate(tree)
    call free_tree(tree)
    if (failed()) then
      call release(q)
      return
    end if
    if (is_array(q)) then
      call fail(e_notscalar, column)
      call release(q)
      return
    end if
    if (real_only .and. .not. is_real(q%s)) then
      call fail(e_notreal, column)
      return
    end if
    x = q%s
    read_bound = .true.
  end function read_bound

  ! AT_END_OF_LINE: nothing is left of the line; if something is, EXTRA.
  logical function at_end_of_line()
    at_end_of_line = at_end()
    if (.not. at_end_of_line) call fail(e_extra, current_column())
  end function at_end_of_line

  ! ------------------------------------------------------------------
  ! Evaluating the expression.

  ! VALUE_AT is the scalar value of the tree P with NAME standing for X,
  ! and of P less Q if Q is associated (SOLVE's two sides); its error is
  ! set if evaluating either failed (when FAIL has recorded where), or if
  ! the value is an array (NOTSCALAR, recorded here at COLUMN, the
  ! expression's start).
  function value_at(p, q, name, x, column) result(v)
    type(node), pointer :: p, q
    character(len=*), intent(in) :: name
    type(value), intent(in) :: x
    integer, intent(in) :: column
    type(value) :: v
    type(quantity) :: a, b

    a = evaluate_with(p, name, x)
    if (.not. scalar_of(a, v)) return
    if (associated(q)) then
      b = evaluate_with(q, name, x)
      if (.not. scalar_of(b, v)) return
      v = a%s - b%s
      if (v%err /= 0) call fail(v%err, column)
    end if

  contains

    ! SCALAR_OF: the quantity Y is a scalar without an error, V; if not,
    ! V is a value with the error, and Y is released. (An internal
    ! procedure of an internal procedure isn't allowed: SCALAR_OF is one of
    ! VALUE_AT, a module procedure, and sees COLUMN by host association.)
    logical function scalar_of(y, v)
      type(quantity), intent(inout) :: y
      type(value), intent(out) :: v

      scalar_of = .false.
      if (quantity_error(y) /= 0) then
        v = error_value(quantity_error(y))
      else if (is_array(y)) then
        call fail(e_notscalar, column)
        call release(y)
        v = error_value(e_notscalar)
      else
        v = y%s
        scalar_of = .true.
      end if
    end function scalar_of
  end function value_at

  ! SHOW_ANSWER shows the answer V, after NAME = if NAME isn't blank, and
  ! keeps it as ANS.
  subroutine show_answer(v, name)
    type(value), intent(in) :: v
    character(len=*), intent(in) :: name
    type(quantity) :: q

    q = scalar_quantity(v)
    call show_quantity(q, name)
    call set_variable('ans', q)
  end subroutine show_answer

  ! WITHOUT_NOISE is V with each part no larger than LEVEL set to 0.
  function without_noise(v, level) result(w)
    type(value), intent(in) :: v
    real(rk), intent(in) :: level
    type(value) :: w
    real(rk) :: x, y

    x = real(v%z, rk)
    y = aimag(v%z)
    if (abs(x) <= level) x = 0
    if (abs(y) <= level) y = 0
    w = complex_value(x, y)
  end function without_noise

  ! WARN_INACCURATE gives the warning INACCURATE with the estimate ERROR, to
  ! two significant digits, at COLUMN.
  subroutine warn_inaccurate(error, column)
    real(rk), intent(in) :: error
    integer, intent(in) :: column
    integer :: kept

    kept = digits
    digits = 2
    call fail(w_inaccurate, column, trim(format_value(real_value(error))))
    digits = kept
  end subroutine warn_inaccurate

  ! SCALED is X times Y, or 0 if that is smaller than SMALL: R_MUL, with
  ! an overflow (which tolerances and steps never reach) ignored.
  real(rk) function scaled(x, y)
    real(rk), intent(in) :: x, y
    integer :: err

    err = 0
    scaled = r_mul(x, y, err)
  end function scaled

  ! ------------------------------------------------------------------
  ! SOLVE.

  subroutine solve_command()
    type(node), pointer :: left, right
    character(len=name_length) :: name
    type(value) :: a, b, root
    integer :: start, from_column, column
    logical :: bracket, found

    nullify (left, right)
    bracket = .false.
    found = .false.
    a = real_value(0.0_rk)
    b = a
    call advance(1)
    start = current_column()

    ! The command, read: a block left by EXIT at the first error.
    read: do
      left => parse_expression()
      if (failed()) exit read
      if (is_operator('=')) then
        call advance(1)
        right => parse_expression()
        if (failed()) exit read
      end if
      if (.not. read_word('for', 'FOR')) exit read
      if (.not. read_name(name)) exit read
      from_column = current_column()
      if (at_word('from')) then
        call advance(1)
        if (.not. read_bound(a, .false., column)) exit read
        if (at_word('to')) then
          call advance(1)
          bracket = .true.
          if (.not. is_real(a)) then
            call fail(e_notreal, column)
            exit read
          end if
          if (.not. read_bound(b, .true., column)) exit read
        end if
      end if
      if (.not. at_end_of_line()) exit read

      if (bracket) then
        found = brent(left, right, name, real(a%z, rk), real(b%z, rk), start, from_column, root)
      else
        found = secant(left, right, name, a, start, root)
      end if
      exit read
    end do read

    call free_tree(left)
    call free_tree(right)
    if (found) call show_answer(root, name)
  end subroutine solve_command

  ! SECANT: ROOT is a root of the expression found by the secant method
  ! from A, false if none was. Each step draws the line through the last
  ! two points, (x0, f0) and (x1, f1), and takes where it crosses zero as
  ! the next point:
  !
  !     x2 = x1 - f1 (x1 - x0) / (f1 - f0)
  !
  ! Near a simple root each step about squares the error (strictly, raises
  ! it to the power 1.618, the golden ratio), so a dozen steps reach the
  ! machine's precision. In complex arithmetic the same step finds complex
  ! roots, from a complex start; from a real start, a real expression stays
  ! on the real line.
  logical function secant(left, right, name, a, start, root) result(found)
    type(node), pointer :: left, right
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    integer, intent(in) :: start
    type(value), intent(out) :: root
    type(value) :: x0, x1, f0, f1, step
    integer :: k

    found = .false.
    root = a

    ! The second point is a little way from the first.
    x0 = a
    x1 = a + real_value(scaled(1.0e-3_rk, max(c_abs(a%z), 1.0_rk)))
    f0 = value_at(left, right, name, x0, start)
    if (f0%err /= 0) return
    if (f0%z == (0.0_rk, 0.0_rk)) then
      found = .true.
      return
    end if
    f1 = value_at(left, right, name, x1, start)
    if (f1%err /= 0) return

    do k = 1, step_limit
      if (f1%z == (0.0_rk, 0.0_rk)) then
        root = x1
        found = .true.
        return
      end if
      ! A line parallel to the axis crosses it nowhere: no root this way.
      if (f1%z == f0%z) exit

      step = f1 * (x1 - x0) / (f1 - f0)
      if (step%err /= 0) exit
      x0 = x1
      f0 = f1
      x1 = x1 - step
      if (x1%err /= 0) exit
      f1 = value_at(left, right, name, x1, start)
      if (f1%err /= 0) return

      ! Done when the step is a rounding error's size beside the root.
      if (c_abs(step%z) <= scaled(4 * eps, c_abs(x1%z))) then
        root = x1
        found = .true.
        return
      end if
    end do
    call fail(e_noroot, start)
  end function secant

  ! BRENT: ROOT is a root of the expression between A and B, where it must
  ! be real and change sign, found by Brent's method (R. P. Brent,
  ! "Algorithms for Minimization without Derivatives", 1973), in the form
  ! many libraries give it.
  !
  ! The method keeps a bracket, B and C, the root between them, B the end
  ! whose value is smaller, and A the last B. Each step tries to guess the
  ! root from the last points, by inverse quadratic interpolation through
  ! (A, fA), (B, fB), (C, fC) (x as a quadratic in f, evaluated at f = 0),
  ! or the secant through two; and takes the guess only if it lands well
  ! inside the bracket and shrinks it quickly enough, bisecting the bracket
  ! otherwise. So it is as fast as the secant method near a simple root, yet
  ! never slower than bisection, which always converges: a bracket of width
  ! w is narrowed below t in at most about log2(w/t) bisections, and the
  ! steps between them are faster still.
  !
  ! Every operation is guarded: near a root the values are tiny, and their
  ! products could underflow.
  logical function brent(left, right, name, a0, b0, start, from_column, root) result(found)
    type(node), pointer :: left, right
    character(len=*), intent(in) :: name
    real(rk), intent(in) :: a0, b0
    integer, intent(in) :: start, from_column
    type(value), intent(out) :: root
    real(rk) :: a, b, c, d, e, fa, fb, fc, p, q, r, s, tol, xm, width
    integer :: k, err

    found = .false.
    root = real_value(0.0_rk)
    err = 0
    a = a0
    b = b0
    if (.not. real_value_at(a, fa)) return
    if (.not. real_value_at(b, fb)) return
    if (fa == 0) then
      root = real_value(a)
      found = .true.
      return
    end if
    if (fb == 0) then
      root = real_value(b)
      found = .true.
      return
    end if
    if ((fa > 0) .eqv. (fb > 0)) then
      call fail(e_samesign, from_column)
      return
    end if

    ! WIDTH, a rounding error's size beside the bracket's ends: a root
    ! closer to 0 than that is 0, as far as the bracket can tell.
    width = scaled(eps, max(abs(a), abs(b)))

    c = b
    fc = fb
    d = r_add(b, -a, err)
    e = d
    do k = 1, step_limit
      ! C is the end of the bracket opposite B.
      if ((fb > 0) .eqv. (fc > 0)) then
        c = a
        fc = fa
        d = r_add(b, -a, err)
        e = d
      end if
      ! B is the end whose value is smaller.
      if (abs(fc) < abs(fb)) then
        a = b
        b = c
        c = a
        fa = fb
        fb = fc
        fc = fa
      end if

      ! Done when the bracket's half width, XM, is within TOL, two rounding
      ! errors beside B (and half of WIDTH, for a root at 0).
      tol = r_add(scaled(2 * eps, abs(b)), scaled(0.5_rk, width), err)
      xm = scaled(0.5_rk, r_add(c, -b, err))
      if (abs(xm) <= tol .or. fb == 0) then
        if (abs(b) <= width) b = 0
        root = real_value(b)
        found = err == 0
        if (.not. found) call fail(e_noroot, start)
        return
      end if

      if (abs(e) >= tol .and. abs(fa) > abs(fb)) then
        ! Interpolate: P / Q is the step from B.
        s = r_div(fb, fa, err)
        if (a == c) then
          ! Two points: the secant.
          p = r_mul(scaled(2.0_rk, xm), s, err)
          q = 1 - s
        else
          ! Three: inverse quadratic interpolation.
          q = r_div(fa, fc, err)
          r = r_div(fb, fc, err)
          p = r_mul(s, r_add(r_mul(r_mul(scaled(2.0_rk, xm), q, err), r_add(q, -r, err), err), &
            -r_mul(r_add(b, -a, err), r - 1, err), err), err)
          q = r_mul(r_mul(q - 1, r - 1, err), s - 1, err)
        end if
        if (p > 0) q = -q
        p = abs(p)
        ! Take the step only if it lands within three quarters of the way
        ! across the bracket, and is less than half the step before last
        ! (else bisection would do better).
        if (scaled(2.0_rk, p) < min(r_add(scaled(3.0_rk, r_mul(xm, q, err)), &
            -abs(scaled(tol, q)), err), abs(r_mul(e, q, err)))) then
          e = d
          d = r_div(p, q, err)
        else
          d = xm
          e = d
        end if
      else
        ! Bisect.
        d = xm
        e = d
      end if
      if (err /= 0) exit

      a = b
      fa = fb
      ! A step at least TOL long, so B moves.
      if (abs(d) > tol) then
        b = r_add(b, d, err)
      else
        b = r_add(b, sign(tol, xm), err)
      end if
      if (.not. real_value_at(b, fb)) return
    end do
    call fail(e_noroot, start)

  contains

    ! REAL_VALUE_AT: the expression's value at X is real, FX; NOTREAL if
    ! not (recorded at the expression's start).
    logical function real_value_at(x, fx) result(ok)
      real(rk), intent(in) :: x
      real(rk), intent(out) :: fx
      type(value) :: v

      ok = .false.
      fx = 0
      v = value_at(left, right, name, real_value(x), start)
      if (v%err /= 0) return
      if (.not. is_real(v)) then
        call fail(e_notreal, start)
        return
      end if
      fx = real(v%z, rk)
      ok = .true.
    end function real_value_at
  end function brent

  ! ------------------------------------------------------------------
  ! INTEGRATE.

  subroutine integrate_command()
    type(node), pointer :: tree
    character(len=name_length) :: name
    type(value) :: a, b, result
    integer :: start, column
    logical :: done

    nullify (tree)
    done = .false.
    call advance(1)
    start = current_column()

    read: do
      tree => parse_expression()
      if (failed()) exit read
      if (.not. read_word('for', 'FOR')) exit read
      if (.not. read_name(name)) exit read
      if (.not. read_word('from', 'FROM')) exit read
      if (.not. read_bound(a, .true., column)) exit read
      if (.not. read_word('to', 'TO')) exit read
      if (.not. read_bound(b, .true., column)) exit read
      if (.not. at_end_of_line()) exit read
      done = integral(tree, name, a, b, start, result)
      exit read
    end do read

    call free_tree(tree)
    if (done) call show_answer(result, '')
  end subroutine integrate_command

  ! INTEGRAL: RESULT is the integral of the expression from A to B, by
  ! adaptive Gauss-Kronrod quadrature, as QUADPACK's QAG routine does it
  ! (R. Piessens and others, "QUADPACK", 1983).
  !
  ! A Gauss rule of n points integrates a polynomial of degree 2n - 1
  ! exactly, the best n points can do, so for a smooth function it is very
  ! accurate. Kronrod's extension adds n + 1 points to the Gauss rule's n,
  ! for a rule of 2n + 1 points exact to degree 3n + 1; the two rules share
  ! the Gauss points, so both come from the same 2n + 1 values, and their
  ! difference is an estimate (a pessimistic one) of the Gauss rule's error,
  ! and so a bound on the Kronrod rule's. Here n is 7: 15 values a piece.
  !
  ! The interval starts as one piece. While the pieces' errors add up to
  ! more than the tolerance, the piece with the largest error is cut in two,
  ! so the work goes where the function is hard: near a kink, a jump, or a
  ! singularity at an end, where the pieces shrink, and nowhere else. The
  ! tolerance is relative, 10 to the -(DIGITS+1) of the integral of |f|
  ! (which the same values estimate): one relative to the integral itself
  ! would never be met by an integral of 0. No piece's error counts as less
  ! than a rounding error's size, 50 EPS times its integral of |f|, which is
  ! all the arithmetic can promise.
  !
  ! The rule never evaluates the function at a piece's ends, so it can
  ! integrate a function infinite at an end, 1/SQRT(X) from 0, as long as
  ! the integral is finite. If the pieces run out (MAX_PIECES) first, the
  ! answer is the best there is, and the warning INACCURATE gives the error.
  !
  ! The pieces are kept in allocatable arrays, made the size of the most
  ! there may be: LO and HI, each piece's ends; PART, its integral; ERROR,
  ! its error; and MAGNITUDE, its integral of |f|. SUM, MAXLOC, and array
  ! sections do the bookkeeping.
  logical function integral(tree, name, a, b, start, result) result(done)
    type(node), pointer :: tree
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a, b
    integer, intent(in) :: start
    type(value), intent(out) :: result
    type(node), pointer :: none
    real(rk), allocatable :: lo(:), hi(:), error(:), magnitude(:)
    type(value), allocatable :: part(:)
    real(rk) :: tol, middle
    integer :: n, k, worst(1)

    ! NONE points to no tree: VALUE_AT's second side, for an expression
    ! that has only one. (Fortran 95's NULL() would do; in Fortran 90 a
    ! pointer is made to point nowhere by NULLIFY, and a pointer argument
    ! must be a pointer variable, not a function's value.)
    nullify (none)
    done = .false.
    result = real_value(0.0_rk)
    if (a%z == b%z) then
      done = .true.
      return
    end if

    allocate (lo(max_pieces), hi(max_pieces), error(max_pieces), &
      magnitude(max_pieces), part(max_pieces))
    n = 1
    lo(1) = real(a%z, rk)
    hi(1) = real(b%z, rk)
    if (.not. kronrod(1)) goto 90

    do
      tol = scaled(10.0_rk ** (-digits - 1), sum(magnitude(1:n)))
      if (sum(error(1:n)) <= tol .or. n == max_pieces) exit

      ! MAXLOC is the place of the largest element, as an array of one
      ! element per dimension: WORST(1) here.
      worst = maxloc(error(1:n))
      k = worst(1)
      middle = scaled(0.5_rk, lo(k)) + scaled(0.5_rk, hi(k))
      ! A piece too narrow to cut leaves the answer as it is.
      if (middle <= min(lo(k), hi(k)) .or. middle >= max(lo(k), hi(k))) exit
      n = n + 1
      lo(n) = middle
      hi(n) = hi(k)
      hi(k) = middle
      if (.not. kronrod(k)) goto 90
      if (.not. kronrod(n)) goto 90
    end do

    result = real_value(0.0_rk)
    do k = 1, n
      result = result + part(k)
    end do
    if (result%err /= 0) then
      call fail(result%err, start)
      goto 90
    end if
    result = without_noise(result, tol)
    done = .true.
    if (sum(error(1:n)) > tol) call warn_inaccurate(sum(error(1:n)), start)

    ! Allocatable arrays local to a procedure must be deallocated before it
    ! returns: in Fortran 90 their status is otherwise undefined afterwards
    ! (Fortran 95 deallocates them itself). Every way out comes here.
90  deallocate (lo, hi, error, magnitude, part)

  contains

    ! KRONROD sets PART(K), ERROR(K), and MAGNITUDE(K) from the rules on the
    ! piece from LO(K) to HI(K); false if evaluating the expression failed.
    ! The piece's center is C and its half width H: the rules' points are C
    ! and C plus and minus H times each of NODES(1:7).
    logical function kronrod(k) result(ok)
      integer, intent(in) :: k
      type(value) :: f, f1, f2, sum15, sum7
      real(rk) :: c, h, size15
      integer :: i, err

      ok = .false.
      err = 0
      c = scaled(0.5_rk, lo(k)) + scaled(0.5_rk, hi(k))
      h = scaled(0.5_rk, hi(k)) - scaled(0.5_rk, lo(k))

      f = value_at(tree, none, name, real_value(c), start)
      if (f%err /= 0) return
      sum15 = f * real_value(kronrod_weights(8))
      sum7 = f * real_value(gauss_weights(4))
      size15 = c_abs(f%z) * kronrod_weights(8)
      do i = 1, 7
        f1 = value_at(tree, none, name, real_value(r_add(c, -scaled(h, nodes(i)), err)), start)
        if (f1%err /= 0) return
        f2 = value_at(tree, none, name, real_value(r_add(c, scaled(h, nodes(i)), err)), start)
        if (f2%err /= 0) return
        sum15 = sum15 + (f1 + f2) * real_value(kronrod_weights(i))
        size15 = size15 + (c_abs(f1%z) + c_abs(f2%z)) * kronrod_weights(i)
        ! The Gauss points are the Kronrod rule's even ones.
        if (mod(i, 2) == 0) sum7 = sum7 + (f1 + f2) * real_value(gauss_weights(i / 2))
      end do

      part(k) = sum15 * real_value(h)
      f = (sum15 - sum7) * real_value(h)
      if (part(k)%err /= 0 .or. f%err /= 0) then
        call fail(e_overflow, start)
        return
      end if
      magnitude(k) = r_mul(size15, abs(h), err)
      error(k) = max(c_abs(f%z), scaled(50 * eps, magnitude(k)))
      ok = err == 0
      if (.not. ok) call fail(e_overflow, start)
    end function kronrod
  end function integral

  ! ------------------------------------------------------------------
  ! DIFF.

  subroutine diff_command()
    type(node), pointer :: tree
    character(len=name_length) :: name
    type(value) :: a, result
    integer :: start, column
    logical :: done

    nullify (tree)
    done = .false.
    call advance(1)
    start = current_column()

    read: do
      tree => parse_expression()
      if (failed()) exit read
      if (.not. read_word('for', 'FOR')) exit read
      if (.not. read_name(name)) exit read
      if (.not. read_word('at', 'AT')) exit read
      if (.not. read_bound(a, .false., column)) exit read
      if (.not. at_end_of_line()) exit read
      done = derivative(tree, name, a, start, result)
      exit read
    end do read

    call free_tree(tree)
    if (done) call show_answer(result, '')
  end subroutine diff_command

  ! DERIVATIVE: RESULT is the derivative of the expression at X, by Ridders'
  ! method (C. J. F. Ridders, 1982, as "Numerical Recipes" gives it).
  !
  ! The central difference (f(x + h) - f(x - h)) / 2h is wrong by a series
  ! in even powers of h, c1 h^2 + c2 h^4 + ...; a smaller h makes it more
  ! nearly right, until rounding in f's values, divided by a smaller h,
  ! makes it worse. So instead the differences for a few h, each the last
  ! divided by 1.4, are extrapolated to h = 0 (Neville's scheme on the
  ! table T: T(1, i) is the difference for the ith h, and T(j, i) combines
  ! T(j-1, i) and T(j-1, i-1) to take away the h^(2j-2) term). Each new
  ! entry's difference from its neighbors estimates its error; the entry
  ! with the smallest is the answer, and the table stops growing once the
  ! diagonal's error is SAFE times larger than that.
  logical function derivative(tree, name, x, start, result) result(done)
    type(node), pointer :: tree
    character(len=*), intent(in) :: name
    type(value), intent(in) :: x
    integer, intent(in) :: start
    type(value), intent(out) :: result
    type(node), pointer :: none
    type(value) :: t(table_size, table_size), factor
    real(rk) :: h, error, e1
    integer :: i, j

    nullify (none)
    done = .false.
    result = real_value(0.0_rk)
    h = scaled(0.1_rk, max(c_abs(x%z), 1.0_rk))
    if (.not. difference(h, t(1, 1))) return
    error = huge(1.0_rk)
    result = t(1, 1)

    table: do i = 2, table_size
      h = h / ridders_step
      if (.not. difference(h, t(1, i))) return
      factor = real_value(ridders_step ** 2)
      do j = 2, i
        t(j, i) = (t(j - 1, i) * factor - t(j - 1, i - 1)) / (factor - real_value(1.0_rk))
        factor = factor * real_value(ridders_step ** 2)
        if (t(j, i)%err /= 0) then
          call fail(t(j, i)%err, start)
          return
        end if
        e1 = max(apart(t(j, i), t(j - 1, i)), apart(t(j, i), t(j - 1, i - 1)))
        if (e1 <= error) then
          error = e1
          result = t(j, i)
        end if
      end do
      if (apart(t(i, i), t(i - 1, i - 1)) >= scaled(safe, error)) exit table
    end do table

    result = without_noise(result, error)
    done = .true.
    if (error > scaled(sqrt(eps), max(c_abs(result%z), 1.0_rk))) call warn_inaccurate(error, start)

  contains

    ! DIFFERENCE: D is the central difference with the step H; false if
    ! evaluating the expression failed.
    logical function difference(h, d) result(ok)
      real(rk), intent(in) :: h
      type(value), intent(out) :: d
      type(value) :: up, down, step

      ok = .false.
      step = real_value(h)
      up = value_at(tree, none, name, x + step, start)
      if (up%err /= 0) return
      down = value_at(tree, none, name, x - step, start)
      if (down%err /= 0) return
      d = (up - down) / (real_value(2.0_rk) * step)
      if (d%err /= 0) then
        call fail(d%err, start)
        return
      end if
      ok = .true.
    end function difference

    ! APART is |U - V|. (A component of an expression's value can't be
    ! selected, (U - V)%Z: the difference goes in a variable first.)
    real(rk) function apart(u, v)
      type(value), intent(in) :: u, v
      type(value) :: w

      w = u - v
      apart = c_abs(w%z)
    end function apart
  end function derivative

  ! ------------------------------------------------------------------
  ! ROOTS.

  ! ROOTS c1, c2, ..., cn: the roots of the polynomial with these
  ! coefficients, highest power first. Each may be a vector, its elements
  ! joined to the list, so ROOTS p takes a vector p of coefficients. The
  ! roots are the function roots()'s (module FCPOLY), a column.
  subroutine roots_command()
    type(node), pointer :: tree
    type(quantity) :: list(1), q
    complex(rk), pointer :: c(:), more(:)
    type(value) :: v
    integer :: first, column, n, k

    nullify (c)
    n = 0
    call advance(1)
    first = current_column()

    read: do
      column = current_column()
      tree => parse_expression()
      if (failed()) exit read
      q = evaluate(tree)
      call free_tree(tree)
      if (failed()) then
        call release(q)
        exit read
      end if

      if (is_array(q) .and. .not. is_vector(q)) then
        call fail(e_notvector, column)
        call release(q)
        exit read
      end if

      ! C grows to hold Q's elements too: a new array, the old copied in.
      allocate (more(n + numel(q)))
      if (n > 0) more(1:n) = c
      do k = 1, numel(q)
        v = element_at(q, k)
        more(n + k) = v%z
      end do
      n = n + numel(q)
      if (associated(c)) deallocate (c)
      c => more
      call release(q)

      if (is_operator(',')) then
        call advance(1)
        cycle read
      end if
      if (.not. at_end_of_line()) exit read

      list(1) = array_of(reshape(c, (/ 1, n /)))
      q = apply_matrix(matrix_index('roots'), list)
      call release(list(1))
      if (quantity_error(q) /= 0) then
        call fail(quantity_error(q), first)
        exit read
      end if
      call show_quantity(q, '')
      call set_variable('ans', q)
      exit read
    end do read

    if (associated(c)) deallocate (c)
  end subroutine roots_command
end module fcnumeric
