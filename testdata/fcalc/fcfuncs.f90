! fcalc: the built-in functions.
!
! BUILTINS is the table of every built-in function: its name and how many
! arguments it takes, so adding a function is a row here and a CASE in
! APPLY_BUILTIN. The evaluator (module FCEVAL) evaluates a call's arguments
! into an array and gives it to APPLY_BUILTIN; IF, whose arms are evaluated
! only when chosen, is the evaluator's own, and is in the table only so its
! name is known.
!
! Every function takes complex values. A real argument in a function's real
! domain takes the real path, the Fortran intrinsic (MTHRTL's routine on a
! VAX), so sqrt(2) and asin(0.5) are as exact as the machine can make them;
! anything else takes the principal branch of the complex function, built
! from FCVALUES' guarded arithmetic. As there, nothing traps: each function
! checks its argument before the intrinsic sees it (an EXP's argument, a
! sine's angle, a pole) and reports OVERFLOW, DOMAIN, or DIVZERO instead.
!
! The trigonometric functions take and give angles in the angle mode
! (SET ANGLE): radians, degrees, or grads. In degrees and grads an angle is
! first reduced exactly to a turn, and a multiple of a quarter turn has its
! exact sine and cosine, so sin(180) is 0 and tan(90) a division by zero.
!
! Some vocabulary of numerical programming used in the comments below:
!
! - Cancellation. Subtracting two nearly equal numbers loses the digits
!   they share: 1.0000001 - 1.0000000 keeps one significant digit of eight.
!   A formula that does that for some arguments is rewritten for them (as
!   LOG1P, ERF's series, and SIN_PI do).
! - Principal branch. Functions like the square root and the logarithm
!   have several values for a complex argument; the principal one is a
!   fixed, conventional choice, and the line where the choice jumps is the
!   branch cut (the negative real axis for SQRT and LN).
! - Range reduction. A periodic function's argument is brought into one
!   period before it is evaluated; done in floating point it loses the
!   argument's low digits, so fcalc does it exactly where it can (in
!   degrees, by MOD of whole numbers) and refuses angles so large in radians
!   that no digit of their sine is meaningful.
! - Series and continued fractions. A function is often computed by summing
!   a series until the terms stop mattering (ERF below 1.5), or by a
!   continued fraction evaluated from a fixed depth back up (ERFC above it);
!   each converges quickly in its own region, so a function switches
!   between them at a crossover chosen by testing.
module fcfuncs
  use fckinds
  use fcmessages
  use fcvalues
  implicit none
  private

  ! VALUE is public here too: a public procedure's argument may not be of a
  ! type private in this module.
  public :: value
  public :: builtin_index, arity_ok, apply_builtin, factorial_value, takes_list
  public :: builtin, many
  public :: angle_mode, a_radians, a_degrees, a_grads

  integer, parameter :: a_radians = 1, a_degrees = 2, a_grads = 3
  integer, save :: angle_mode = a_radians

  ! SET SEED's number, once one has been given (SHOW SETTINGS shows it).
  public :: seed_random, seed_given, seed_number
  logical, save :: seed_given = .false.
  integer, save :: seed_number = 0

  ! (The type is public, as is MANY, so module FCMATRIX's table of the
  ! matrix functions is made the same way.)
  type builtin
    character(len=12) :: name
    integer :: least
    integer :: most
  end type builtin

  ! MOST for a function of any number of arguments.
  integer, parameter :: many = 1000

  ! The table of built-in functions, a constant array of BUILTINs made by
  ! an array constructor of structure constructors (several to a line:
  ! Fortran 90 allows a statement 39 continuation lines).
  type(builtin), parameter :: builtins(50) = (/ &
    builtin('sqrt', 1, 1), builtin('cbrt', 1, 1), builtin('exp', 1, 1), &
    builtin('ln', 1, 1), builtin('log10', 1, 1), builtin('log2', 1, 1), &
    builtin('log', 2, 2), &
    builtin('sin', 1, 1), builtin('cos', 1, 1), builtin('tan', 1, 1), &
    builtin('sec', 1, 1), builtin('csc', 1, 1), builtin('cot', 1, 1), &
    builtin('asin', 1, 1), builtin('acos', 1, 1), builtin('atan', 1, 1), &
    builtin('atan2', 2, 2), &
    builtin('sinh', 1, 1), builtin('cosh', 1, 1), builtin('tanh', 1, 1), &
    builtin('asinh', 1, 1), builtin('acosh', 1, 1), builtin('atanh', 1, 1), &
    builtin('re', 1, 1), builtin('im', 1, 1), builtin('abs', 1, 1), &
    builtin('arg', 1, 1), builtin('conj', 1, 1), builtin('polar', 2, 2), &
    builtin('cis', 1, 1), &
    builtin('floor', 1, 1), builtin('ceil', 1, 1), builtin('round', 1, 1), &
    builtin('trunc', 1, 1), builtin('frac', 1, 1), builtin('sign', 1, 1), &
    builtin('gcd', 2, many), builtin('lcm', 2, many), builtin('isprime', 1, 1), &
    builtin('ncr', 2, 2), builtin('npr', 2, 2), &
    builtin('gamma', 1, 1), builtin('lgamma', 1, 1), builtin('erf', 1, 1), &
    builtin('erfc', 1, 1), builtin('beta', 2, 2), &
    builtin('min', 1, many), builtin('max', 1, many), builtin('sum', 1, many), &
    builtin('prod', 1, many) /)

  ! The list functions and the rest, a second table: Fortran 90 allows a
  ! statement 39 continuation lines, and one table would need more.
  type(builtin), parameter :: more_builtins(7) = (/ &
    builtin('mean', 1, many), builtin('median', 1, many), &
    builtin('var', 2, many), builtin('stdev', 2, many), &
    builtin('if', 3, 3), builtin('rand', 0, 2), builtin('hypot', 2, 2) /)

  ! The largest magnitude of which every real number is whole, and below
  ! which fcalc counts exactly: 4E15, under 2**52.
  real(rk), parameter :: counting = 4.0e15_rk

  ! The largest angle in radians whose sine has significant digits.
  real(rk), parameter :: angle_limit = 1.0e15_rk

  ! Past this a hyperbolic sine or cosine is past LIMIT.
  real(rk), parameter :: hyper_limit = 690.0_rk

  real(rk), parameter :: ln2 = 0.693147180559945309417232121458_rk
  real(rk), parameter :: ln10 = 2.30258509299404568401799145468_rk
  real(rk), parameter :: half_log_2pi = 0.918938533204672741780329736406_rk
  real(rk), parameter :: sqrt_pi = 1.77245385090551602729816748334_rk

  ! Lanczos's approximation of the gamma function, g = 7 and 9 terms, good
  ! to about 15 digits for arguments from 0.5 up.
  real(rk), parameter :: lanczos_g = 7.0_rk
  real(rk), parameter :: lanczos(0:8) = (/ 0.99999999999980993_rk, &
    676.5203681218851_rk, -1259.1392167224028_rk, 771.32342877765313_rk, &
    -176.61502916214059_rk, 12.507343278686905_rk, -0.13857109526572012_rk, &
    9.9843695780195716e-6_rk, 1.5056327351493116e-7_rk /)

contains

  ! ------------------------------------------------------------------
  ! The table.

  ! BUILTIN_INDEX is NAME's place in the tables (the second's after the
  ! first's), or 0 if it isn't a built-in function.
  integer function builtin_index(name)
    character(len=*), intent(in) :: name
    integer :: k

    builtin_index = 0
    do k = 1, size(builtins)
      if (builtins(k)%name == name) then
        builtin_index = k
        return
      end if
    end do
    do k = 1, size(more_builtins)
      if (more_builtins(k)%name == name) then
        builtin_index = size(builtins) + k
        return
      end if
    end do
  end function builtin_index

  function row(id) result(b)
    integer, intent(in) :: id
    type(builtin) :: b

    if (id <= size(builtins)) then
      b = builtins(id)
    else
      b = more_builtins(id - size(builtins))
    end if
  end function row

  ! TAKES_LIST: the function ID takes a list of any length, so the
  ! evaluator gives it every element of an array argument (sum([1, 2, 3])
  ! is sum(1, 2, 3)), where it applies any other function to each element.
  logical function takes_list(id)
    integer, intent(in) :: id
    type(builtin) :: b

    b = row(id)
    takes_list = b%most == many
  end function takes_list

  ! ARITY_OK: the function ID takes N arguments.
  logical function arity_ok(id, n)
    integer, intent(in) :: id, n
    type(builtin) :: b

    b = row(id)
    arity_ok = n >= b%least .and. n <= b%most
  end function arity_ok

  ! APPLY_BUILTIN is the function ID of ARGS, none with an error. ARGS is
  ! an assumed-shape array, as long as the call's argument list; the
  ! function is chosen by SELECT CASE on its name, a CASE listing several
  ! names where one routine serves them all.
  function apply_builtin(id, args) result(v)
    integer, intent(in) :: id
    type(value), intent(in) :: args(:)
    type(value) :: v
    type(builtin) :: b

    b = row(id)
    select case (b%name)
    case ('sqrt')
      v = sqrt_value(args(1))
    case ('cbrt')
      v = cbrt_value(args(1))
    case ('exp')
      v = exp_value(args(1))
    case ('ln')
      v = ln_value(args(1))
    case ('log10')
      v = log10_value(args(1))
    case ('log2')
      v = log2_value(args(1))
    case ('log')
      v = ln_value(args(1))
      if (v%err == 0) v = v / ln_value(args(2))
    case ('sin', 'cos', 'tan', 'sec', 'csc', 'cot')
      v = trig(b%name, args(1))
    case ('asin', 'acos', 'atan')
      v = inverse_trig(b%name, args(1))
    case ('atan2')
      v = atan2_value(args(1), args(2))
    case ('sinh', 'cosh', 'tanh')
      v = hyperbolic(b%name, args(1))
    case ('asinh', 'acosh', 'atanh')
      v = inverse_hyperbolic(b%name, args(1))
    case ('re')
      v = real_value(real(args(1)%z, rk))
    case ('im')
      v = real_value(aimag(args(1)%z))
    case ('abs')
      v = v_abs(args(1))
    case ('arg')
      v = arg_value(args(1))
    case ('conj')
      v = complex_value(real(args(1)%z, rk), -aimag(args(1)%z))
    case ('polar')
      v = cis_value(args(2))
      if (v%err == 0) v = args(1) * v
    case ('cis')
      v = cis_value(args(1))
    case ('floor', 'ceil', 'round', 'trunc', 'frac')
      v = complex_value(rounded(b%name, real(args(1)%z, rk)), rounded(b%name, aimag(args(1)%z)))
    case ('sign')
      v = sign_value(args(1))
    case ('gcd')
      v = gcd_value(args)
    case ('lcm')
      v = lcm_value(args)
    case ('isprime')
      v = isprime_value(args(1))
    case ('ncr', 'npr')
      v = choices(b%name, args(1), args(2))
    case ('gamma', 'lgamma', 'erf', 'erfc')
      v = special(b%name, args(1))
    case ('beta')
      v = beta_value(args(1), args(2))
    case ('min', 'max', 'median')
      v = order_statistic(b%name, args)
    case ('sum')
      v = sum_value(args)
    case ('prod')
      v = product_value(args)
    case ('mean')
      v = sum_value(args)
      if (v%err == 0) v = v / real_value(real(size(args), rk))
    case ('var', 'stdev')
      v = spread_value(b%name, args)
    case ('rand')
      v = random_value(args)
    case ('hypot')
      v = hypot_value(args(1), args(2))
    case default
      v = error_value(e_nofunc)
    end select
  end function apply_builtin

  ! ------------------------------------------------------------------
  ! Arguments.

  ! REAL_ARGUMENT: A is real, and X its value; otherwise V is NOTREAL.
  logical function real_argument(a, x, v)
    type(value), intent(in) :: a
    real(rk), intent(out) :: x
    type(value), intent(inout) :: v

    x = real(a%z, rk)
    real_argument = is_real(a)
    if (.not. real_argument) v = error_value(e_notreal)
  end function real_argument

  ! WHOLE_ARGUMENT: A is a whole number small enough to count with exactly,
  ! and X its value; otherwise V says why not.
  logical function whole_argument(a, x, v)
    type(value), intent(in) :: a
    real(rk), intent(out) :: x
    type(value), intent(inout) :: v

    whole_argument = .false.
    if (.not. real_argument(a, x, v)) return
    if (abs(x) >= counting) then
      v = error_value(e_domain)
    else if (aint(x) /= x) then
      v = error_value(e_notwhole)
    else
      whole_argument = .true.
    end if
  end function whole_argument

  ! ------------------------------------------------------------------
  ! Roots, exponentials, and logarithms.

  ! SQRT_VALUE is the principal square root, the one with a positive real
  ! part, by the stable formula: T = SQRT((|x| + |z|) / 2) is the larger
  ! part's magnitude, and the other is y / 2T.
  function sqrt_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x, y, t, q
    integer :: err

    x = real(a%z, rk)
    y = aimag(a%z)
    if (y == 0) then
      if (x >= 0) then
        v = real_value(sqrt(x))
      else
        v = complex_value(0.0_rk, sqrt(-x))
      end if
      return
    end if

    t = sqrt((abs(x) + c_abs(a%z)) / 2)
    err = 0
    q = r_div(abs(y), 2 * t, err)
    if (x >= 0) then
      if (y < 0) q = -q
      v = complex_value(t, q)
    else if (y > 0) then
      v = complex_value(q, t)
    else
      v = complex_value(q, -t)
    end if
  end function sqrt_value

  ! CBRT_VALUE is a real number's real cube root, and a complex number's
  ! principal one. The real root is EXP(LOG|x| / 3) made exact by a step
  ! of Newton's method, so cbrt(27) is 3.
  function cbrt_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x, r

    x = real(a%z, rk)
    if (.not. is_real(a)) then
      v = exp_value(ln_value(a) / real_value(3.0_rk))
      return
    end if
    if (x == 0) then
      v = real_value(0.0_rk)
      return
    end if

    r = exp(log(abs(x)) / 3)
    r = r - (r - abs(x) / (r * r)) / 3
    if (x < 0) r = -r
    v = real_value(r)
  end function cbrt_value

  ! LN_VALUE is the principal natural logarithm: the real one of a positive
  ! number, the imaginary part in (-pi, pi] otherwise.
  function ln_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    x = real(a%z, rk)
    if (.not. truth(a)) then
      v = error_value(e_domain)
    else if (is_real(a) .and. x > 0) then
      v = real_value(log(x))
    else
      v = log_value(a)
    end if
  end function ln_value

  function log10_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    x = real(a%z, rk)
    if (is_real(a) .and. x > 0) then
      v = real_value(log10(x))
    else
      v = ln_value(a)
      if (v%err == 0) v = v / real_value(ln10)
    end if
  end function log10_value

  ! LOG2_VALUE: a power of two's logarithm is its exponent, exactly, by
  ! the model functions; other numbers' are LN(x) / LN(2).
  function log2_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    x = real(a%z, rk)
    if (is_real(a) .and. x > 0) then
      if (fraction(x) == 0.5_rk) then
        v = real_value(real(exponent(x) - 1, rk))
      else
        v = real_value(log(x) / ln2)
      end if
    else
      v = ln_value(a)
      if (v%err == 0) v = v / real_value(ln2)
    end if
  end function log2_value

  ! LOG1P is LOG(1 + U) for U > -1, accurate when U is small: the error 1 + U
  ! makes in rounding is put back by the ratio U / ((1 + U) - 1).
  real(rk) function log1p(u)
    real(rk), intent(in) :: u
    real(rk) :: w

    w = 1 + u
    if (w == 1) then
      log1p = u
    else
      log1p = log(w) * (u / (w - 1))
    end if
  end function log1p

  ! HYPOT_VALUE is SQRT(x**2 + y**2) of two real numbers, never squaring one.
  function hypot_value(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: x, y

    if (.not. real_argument(a, x, v)) return
    if (.not. real_argument(b, y, v)) return
    v = real_value(c_abs(cmplx(x, y, rk)))
  end function hypot_value

  ! ------------------------------------------------------------------
  ! Angles.

  ! TURN is a whole turn in the angle mode.
  real(rk) function turn()
    select case (angle_mode)
    case (a_degrees)
      turn = 360
    case (a_grads)
      turn = 400
    case default
      turn = 2 * pi
    end select
  end function turn

  ! TO_RADIANS and FROM_RADIANS convert an angle between the angle mode and
  ! radians.
  function to_radians(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    if (angle_mode == a_radians) then
      v = a
    else
      v = a * real_value(2 * pi / turn())
    end if
  end function to_radians

  function from_radians(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    if (angle_mode == a_radians .or. a%err /= 0) then
      v = a
    else
      v = complex_value(real(a%z, rk) * turn() / (2 * pi), aimag(a%z) * turn() / (2 * pi))
    end if
  end function from_radians

  ! SINE_COSINE gives a real angle X's sine and cosine. In degrees and
  ! grads the angle is reduced exactly to [0, a turn), and a multiple of a
  ! quarter turn has its exact values. In radians an angle past
  ! ANGLE_LIMIT has no significant digits, and is out of the domain.
  subroutine sine_cosine(x, s, c, err)
    real(rk), intent(in) :: x
    real(rk), intent(out) :: s, c
    integer, intent(inout) :: err
    real(rk), parameter :: sines(0:3) = (/ 0.0_rk, 1.0_rk, 0.0_rk, -1.0_rk /)
    real(rk), parameter :: cosines(0:3) = (/ 1.0_rk, 0.0_rk, -1.0_rk, 0.0_rk /)
    type(value) :: r
    real(rk) :: t, quarter
    integer :: k

    s = 0
    c = 1
    if (angle_mode == a_radians) then
      if (abs(x) > angle_limit) then
        if (err == 0) err = e_domain
        return
      end if
      s = sin(x)
      c = cos(x)
      return
    end if

    r = v_mod(real_value(x), real_value(turn()))
    t = real(r%z, rk)
    quarter = turn() / 4
    if (aint(t / quarter) * quarter == t) then
      k = nint(t / quarter)
      s = sines(k)
      c = cosines(k)
    else
      t = t * (2 * pi / turn())
      s = sin(t)
      c = cos(t)
    end if
  end subroutine sine_cosine

  ! TRIG is a trigonometric function. A real angle's are the real sine and
  ! cosine's; a complex angle x + iy's come from
  !     sin(x + iy) = sin x cosh y + i cos x sinh y
  !     cos(x + iy) = cos x cosh y - i sin x sinh y
  ! and the other four are quotients of the two.
  function trig(name, a) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    type(value) :: v
    type(value) :: s, c, angle
    real(rk) :: x, y, sx, cx
    integer :: err

    err = 0
    if (is_real(a)) then
      call sine_cosine(real(a%z, rk), sx, cx, err)
      if (err /= 0) then
        v = error_value(err)
        return
      end if
      s = real_value(sx)
      c = real_value(cx)
    else
      angle = to_radians(a)
      if (angle%err /= 0) then
        v = angle
        return
      end if
      x = real(angle%z, rk)
      y = aimag(angle%z)
      if (abs(x) > angle_limit) then
        v = error_value(e_domain)
        return
      end if
      if (abs(y) > hyper_limit) then
        v = error_value(e_overflow)
        return
      end if
      s = complex_value(r_mul(sin(x), cosh(y), err), r_mul(cos(x), sinh(y), err))
      c = complex_value(r_mul(cos(x), cosh(y), err), -r_mul(sin(x), sinh(y), err))
    end if

    select case (name)
    case ('sin')
      v = s
    case ('cos')
      v = c
    case ('tan')
      v = s / c
    case ('sec')
      v = real_value(1.0_rk) / c
    case ('csc')
      v = real_value(1.0_rk) / s
    case default
      v = c / s
    end select
  end function trig

  ! INVERSE_TRIG: a real argument in the real domain has the real inverse;
  ! any other the principal value of
  !     asin z = -i ln(iz + sqrt(1 - z**2))
  !     acos z = pi/2 - asin z
  !     atan z = (i/2) ln((i + z) / (i - z))
  function inverse_trig(name, a) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    type(value) :: v
    type(value) :: i, one
    real(rk) :: x

    x = real(a%z, rk)
    i = complex_value(0.0_rk, 1.0_rk)
    one = real_value(1.0_rk)

    if (is_real(a) .and. (abs(x) <= 1 .or. name == 'atan')) then
      select case (name)
      case ('asin')
        v = real_value(asin(x))
      case ('acos')
        v = real_value(acos(x))
      case default
        v = real_value(atan(x))
      end select
    else
      select case (name)
      case ('asin', 'acos')
        v = -i * ln_value(i * a + sqrt_value(one - a * a))
        if (name == 'acos' .and. v%err == 0) v = real_value(pi / 2) - v
      case default
        if (a%z == (0.0_rk, 1.0_rk) .or. a%z == (0.0_rk, -1.0_rk)) then
          v = error_value(e_domain)
        else
          v = complex_value(0.0_rk, 0.5_rk) * ln_value((i + a) / (i - a))
        end if
      end select
    end if
    v = from_radians(v)
  end function inverse_trig

  ! ATAN2_VALUE is the angle of the point (x, y), from -pi to pi.
  function atan2_value(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: y, x

    if (.not. real_argument(a, y, v)) return
    if (.not. real_argument(b, x, v)) return
    if (x == 0 .and. y == 0) then
      v = error_value(e_domain)
      return
    end if
    v = from_radians(real_value(angle_of(y, x)))
  end function atan2_value

  ! ARG_VALUE is a complex number's angle, from -pi to pi (0 for zero).
  function arg_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    if (.not. truth(a)) then
      v = real_value(0.0_rk)
    else
      v = from_radians(real_value(angle_of(aimag(a%z), real(a%z, rk))))
    end if
  end function arg_value

  ! CIS_VALUE is cos t + i sin t: EXP(it).
  function cis_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: s, c
    integer :: err

    if (is_real(a)) then
      err = 0
      call sine_cosine(real(a%z, rk), s, c, err)
      if (err /= 0) then
        v = error_value(err)
      else
        v = complex_value(c, s)
      end if
    else
      v = exp_value(complex_value(0.0_rk, 1.0_rk) * to_radians(a))
    end if
  end function cis_value

  ! ------------------------------------------------------------------
  ! Hyperbolic functions.

  ! HYPERBOLIC: a real argument's are the intrinsics' (TANH is 1 to the
  ! digits past 20); a complex one's come from
  !     sinh(x + iy) = sinh x cos y + i cosh x sin y
  !     cosh(x + iy) = cosh x cos y + i sinh x sin y
  function hyperbolic(name, a) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    type(value) :: v
    type(value) :: s, c
    real(rk) :: x, y
    integer :: err

    x = real(a%z, rk)
    y = aimag(a%z)

    if (name == 'tanh' .and. y == 0) then
      if (abs(x) > 20) then
        v = real_value(sign(1.0_rk, x))
      else
        v = real_value(tanh(x))
      end if
      return
    end if

    if (abs(x) > hyper_limit) then
      v = error_value(e_overflow)
      return
    end if
    if (abs(y) > angle_limit) then
      v = error_value(e_domain)
      return
    end if

    if (y == 0) then
      s = real_value(sinh(x))
      c = real_value(cosh(x))
    else
      err = 0
      s = complex_value(r_mul(sinh(x), cos(y), err), r_mul(cosh(x), sin(y), err))
      c = complex_value(r_mul(cosh(x), cos(y), err), r_mul(sinh(x), sin(y), err))
    end if

    select case (name)
    case ('sinh')
      v = s
    case ('cosh')
      v = c
    case default
      v = s / c
    end select
  end function hyperbolic

  ! INVERSE_HYPERBOLIC: in the real domain by LOG1P, so a small argument
  ! loses nothing, and past 1E150 by the logarithm alone (x**2 would be
  ! too large); elsewhere the principal value of
  !     asinh z = ln(z + sqrt(z**2 + 1))
  !     acosh z = ln(z + sqrt(z + 1) sqrt(z - 1))
  !     atanh z = ln((1 + z) / (1 - z)) / 2
  function inverse_hyperbolic(name, a) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    type(value) :: v
    type(value) :: one
    real(rk) :: x, ax, r

    x = real(a%z, rk)
    ax = abs(x)
    one = real_value(1.0_rk)

    if (is_real(a)) then
      select case (name)
      case ('asinh')
        if (ax > 1.0e150_rk) then
          r = log(ax) + ln2
        else if (ax < 1.0e-8_rk) then
          ! asinh x = x - x**3/6 + ..., the rest past the digits (and x**2
          ! might underflow).
          r = ax
        else
          r = log1p(ax + ax * ax / (1 + sqrt(1 + ax * ax)))
        end if
        if (x < 0) r = -r
        v = real_value(r)
        return
      case ('acosh')
        if (x >= 1) then
          if (x > 1.0e150_rk) then
            v = real_value(log(x) + ln2)
          else
            v = real_value(log1p((x - 1) + sqrt((x - 1) * (x + 1))))
          end if
          return
        end if
      case default
        if (ax == 1) then
          v = error_value(e_domain)
          return
        end if
        if (ax < 1) then
          v = real_value(log1p(2 * x / (1 - x)) / 2)
          return
        end if
      end select
    end if

    select case (name)
    case ('asinh')
      v = ln_value(a + sqrt_value(a * a + one))
    case ('acosh')
      v = ln_value(a + sqrt_value(a + one) * sqrt_value(a - one))
    case default
      if (a%z == (1.0_rk, 0.0_rk) .or. a%z == (-1.0_rk, 0.0_rk)) then
        v = error_value(e_domain)
      else
        v = ln_value((one + a) / (one - a)) / real_value(2.0_rk)
      end if
    end select
  end function inverse_hyperbolic

  ! ------------------------------------------------------------------
  ! Rounding and whole numbers.

  ! ROUNDED rounds X as HOW says. A number past COUNTING is whole already
  ! (and AINT isn't asked about it).
  real(rk) function rounded(how, x)
    character(len=*), intent(in) :: how
    real(rk), intent(in) :: x
    real(rk) :: t

    if (abs(x) >= counting) then
      rounded = x
      if (how == 'frac') rounded = 0
      return
    end if

    t = aint(x)
    select case (how)
    case ('floor')
      if (t > x) t = t - 1
      rounded = t
    case ('ceil')
      if (t < x) t = t + 1
      rounded = t
    case ('round')
      rounded = anint(x)
    case ('trunc')
      rounded = t
    case default
      rounded = x - t
    end select
  end function rounded

  ! SIGN_VALUE is -1, 0, or 1 for a real number, and z / |z| for another.
  function sign_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    x = real(a%z, rk)
    if (.not. is_real(a)) then
      v = a / v_abs(a)
    else if (x > 0) then
      v = real_value(1.0_rk)
    else if (x < 0) then
      v = real_value(-1.0_rk)
    else
      v = real_value(0.0_rk)
    end if
  end function sign_value

  ! REMAINDER is X mod Y for whole X and Y from 0 to COUNTING: X / Y may be
  ! rounded up to the next whole number, but Y * AINT(X / Y) is exact, so
  ! one correction puts it right.
  real(rk) function remainder(x, y)
    real(rk), intent(in) :: x, y

    remainder = x - y * aint(x / y)
    if (remainder < 0) remainder = remainder + y
    if (remainder >= y) remainder = remainder - y
  end function remainder

  ! GCD2 is the greatest common divisor of whole X and Y, by Euclid.
  real(rk) function gcd2(x, y)
    real(rk), intent(in) :: x, y
    real(rk) :: a, b, r

    a = abs(x)
    b = abs(y)
    do while (b /= 0)
      r = remainder(a, b)
      a = b
      b = r
    end do
    gcd2 = a
  end function gcd2

  function gcd_value(args) result(v)
    type(value), intent(in) :: args(:)
    type(value) :: v
    real(rk) :: g, x
    integer :: k

    g = 0
    do k = 1, size(args)
      if (.not. whole_argument(args(k), x, v)) return
      g = gcd2(g, x)
    end do
    v = real_value(g)
  end function gcd_value

  ! LCM_VALUE is the least common multiple; one past COUNTING couldn't be
  ! counted exactly, and is too large.
  function lcm_value(args) result(v)
    type(value), intent(in) :: args(:)
    type(value) :: v
    real(rk) :: l, x
    integer :: k

    l = 1
    do k = 1, size(args)
      if (.not. whole_argument(args(k), x, v)) return
      x = abs(x)
      if (x == 0 .or. l == 0) then
        l = 0
      else
        l = l / gcd2(l, x)
        if (l > counting / x) then
          v = error_value(e_overflow)
          return
        end if
        l = l * x
      end if
    end do
    v = real_value(l)
  end function lcm_value

  ! ISPRIME_VALUE is 1 for a prime and 0 for anything else, by trial
  ! division by 2 and the odd numbers up to the square root.
  function isprime_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: n, d
    logical :: prime

    if (.not. whole_argument(a, n, v)) return
    if (n < 2) then
      prime = .false.
    else if (n < 4) then
      prime = .true.
    else if (remainder(n, 2.0_rk) == 0) then
      prime = .false.
    else
      prime = .true.
      d = 3
      do while (d * d <= n)
        if (remainder(n, d) == 0) then
          prime = .false.
          exit
        end if
        d = d + 2
      end do
    end if
    v = logical_value(prime)
  end function isprime_value

  ! CHOICES is ncr(n, k), the ways to choose K of N, or npr(n, k), the ways
  ! to arrange them, each a product of K factors (ncr's dividing as it goes,
  ! so every partial product is whole). Each product stops at an overflow,
  ! which comes within a thousand factors, each at least 2.
  function choices(name, a, b) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: n, k, j, c
    integer :: err

    if (.not. whole_argument(a, n, v)) return
    if (.not. whole_argument(b, k, v)) return
    if (n < 0 .or. k < 0) then
      v = error_value(e_domain)
      return
    end if
    if (k > n) then
      v = real_value(0.0_rk)
      return
    end if

    err = 0
    c = 1
    if (name == 'ncr') then
      k = min(k, n - k)
      j = 1
      do while (j <= k .and. err == 0)
        if (c <= limit / (n - k + j)) then
          ! Multiplying first keeps the partial product whole and exact.
          c = c * (n - k + j) / j
        else
          ! The product would pass LIMIT before the division brought it
          ! back: past 1E290 nothing is exact anyway, so divide first.
          c = r_mul(c, (n - k + j) / j, err)
        end if
        j = j + 1
      end do
    else
      j = 0
      do while (j < k .and. err == 0)
        c = r_mul(c, n - j, err)
        j = j + 1
      end do
    end if

    if (err /= 0) then
      v = error_value(err)
    else
      v = real_value(c)
    end if
  end function choices

  ! ------------------------------------------------------------------
  ! Special functions, of real arguments.

  ! FACTORIAL_VALUE is fcalc's !: a whole number's factorial, and gamma(x +
  ! 1) off the whole numbers.
  function factorial_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    if (.not. real_argument(a, x, v)) return
    if (is_whole(a)) then
      v = v_factorial(a)
    else
      v = gamma_value(x + 1)
    end if
  end function factorial_value

  function special(name, a) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: x

    if (.not. real_argument(a, x, v)) return
    select case (name)
    case ('gamma')
      v = gamma_value(x)
    case ('lgamma')
      v = lgamma_value(x)
    case ('erf')
      v = real_value(erf_real(x))
    case default
      v = real_value(erfc_real(x))
    end select
  end function special

  ! LANCZOS_LOG is LOG(GAMMA(X)) for X from 0.5 up, by Lanczos's formula
  !     gamma(y + 1) = sqrt(2 pi) t**(y + 1/2) e**-t A(y),  t = y + g + 1/2
  ! in logarithms, so it never overflows.
  !
  ! The gamma function extends the factorial to real numbers: gamma(n + 1)
  ! = n!, and gamma(1/2) = sqrt(pi). Lanczos's approximation is Stirling's
  ! formula (the t**(y+1/2) e**-t part) times a correction A(y), a short sum
  ! of fractions c(k) / (y + k) whose coefficients were fitted once (with
  ! g = 7 and nine terms they give about 15 digits); they are the widely
  ! published values. Its error is relative, so a value far from 1 is still
  ! good to its 15th digit, but LOG(GAMMA(x)) near x = 1 or 2, where it is
  ! near 0, is only as good as that absolute error: lgamma gives exact 0s
  ! at 1 and 2 for that reason.
  real(rk) function lanczos_log(x)
    real(rk), intent(in) :: x
    real(rk) :: y, a, t
    integer :: k

    y = x - 1
    a = lanczos(0)
    do k = 1, 8
      a = a + lanczos(k) / (y + k)
    end do
    t = y + lanczos_g + 0.5_rk
    lanczos_log = half_log_2pi + (y + 0.5_rk) * log(t) - t + log(a)
  end function lanczos_log

  ! SIN_PI is SIN(PI * X) for a number X that isn't whole, X reduced
  ! exactly to [-1/2, 1/2] first, so the sine near a whole number keeps its
  ! digits.
  real(rk) function sin_pi(x)
    real(rk), intent(in) :: x
    real(rk) :: r

    r = x - 2 * anint(x / 2)
    if (r > 0.5_rk) then
      r = 1 - r
    else if (r < -0.5_rk) then
      r = -1 - r
    end if
    sin_pi = sin(pi * r)
  end function sin_pi

  ! GAMMA_VALUE is the gamma function: a factorial at a whole number, and
  ! Lanczos's formula elsewhere, below 1/2 through the reflection
  !     gamma(x) = pi / (sin(pi x) gamma(1 - x))
  ! The whole numbers from 0 down are its poles (it goes to infinity
  ! there), out of its domain.
  !
  ! The reflection formula carries a value from the left half, where
  ! Lanczos's sum would divide by numbers near zero, to the right, where it
  ! is accurate. For 1 - x past 160, gamma(1 - x) is past the machines'
  ! range though the answer isn't, so the formula is used in logarithms.
  recursive function gamma_value(x) result(v)
    real(rk), intent(in) :: x
    type(value) :: v
    real(rk) :: s, d
    type(value) :: g
    integer :: err

    if (abs(x) < counting .and. aint(x) == x) then
      if (x <= 0) then
        v = error_value(e_domain)
      else
        v = v_factorial(real_value(x - 1))
      end if
      return
    end if
    if (x > 171) then
      v = error_value(e_overflow)
      return
    end if
    if (x <= -counting) then
      v = error_value(e_domain)
      return
    end if

    if (x >= 0.5_rk) then
      v = exp_real(lanczos_log(x))
      return
    end if

    s = sin_pi(x)
    if (1 - x <= 160) then
      g = gamma_value(1 - x)
      err = 0
      d = r_mul(s, real(g%z, rk), err)
      v = real_value(r_div(pi, d, err))
      if (err /= 0) v = error_value(err)
    else
      v = exp_real(log(pi / abs(s)) - lanczos_log(1 - x))
      if (s < 0) v = -v
    end if
  end function gamma_value

  ! LGAMMA_VALUE is LOG|GAMMA(X)|: exactly 0 at 1 and 2, and the logarithm
  ! of the factorial at the other whole numbers it can count.
  function lgamma_value(x) result(v)
    real(rk), intent(in) :: x
    type(value) :: v
    type(value) :: f

    if (abs(x) < counting .and. aint(x) == x) then
      if (x <= 0) then
        v = error_value(e_domain)
      else if (x <= 2) then
        v = real_value(0.0_rk)
      else if (x <= 171) then
        f = v_factorial(real_value(x - 1))
        v = real_value(log(real(f%z, rk)))
        if (f%err /= 0) v = real_value(lanczos_log(x))
      else
        v = real_value(lanczos_log(x))
      end if
      return
    end if
    if (x <= -counting) then
      v = error_value(e_domain)
    else if (x >= 0.5_rk) then
      v = real_value(lanczos_log(x))
    else
      v = real_value(log(pi / abs(sin_pi(x))) - lanczos_log(1 - x))
    end if
  end function lgamma_value

  ! ERF_REAL is the error function: below 1.5 by its series of positive
  ! terms,
  !     erf x = 2/sqrt(pi) e**(-x**2) sum 2**n x**(2n+1) / (1 3 5 ... (2n+1))
  ! which loses nothing to cancellation, and from 1.5 as 1 - erfc x.
  real(rk) function erf_real(x)
    real(rk), intent(in) :: x
    real(rk) :: ax, term, total
    integer :: n

    ax = abs(x)
    if (ax >= 1.5_rk) then
      erf_real = 1 - erfc_real(ax)
    else if (ax < 1.0e-8_rk) then
      ! erf x = 2/sqrt(pi) (x - x**3/3 + ...), the rest past the digits.
      erf_real = 2 / sqrt_pi * ax
    else
      term = ax
      total = ax
      n = 0
      do
        n = n + 1
        term = term * 2 * ax * ax / (2 * n + 1)
        total = total + term
        if (term <= total * 1.0e-17_rk) exit
      end do
      erf_real = 2 / sqrt_pi * exp(-ax * ax) * total
    end if
    if (x < 0) erf_real = -erf_real
  end function erf_real

  ! ERFC_REAL is 1 - erf x: from 1.5 by its continued fraction,
  !     erfc x = e**(-x**2) / sqrt(pi) / (x + (1/2) / (x + 1 / (x + (3/2) / (x + ...))))
  ! evaluated from 200 terms deep up; below as 1 - erf x. Past 30 it is 0
  ! (and x**2 is never too large).
  real(rk) function erfc_real(x)
    real(rk), intent(in) :: x
    real(rk) :: f
    integer :: k

    if (x < 1.5_rk) then
      if (x <= -30) then
        erfc_real = 2
      else
        erfc_real = 1 - erf_real(x)
      end if
      return
    end if
    if (x >= 30) then
      erfc_real = 0
      return
    end if

    f = x
    do k = 200, 1, -1
      f = x + (k / 2.0_rk) / f
    end do
    erfc_real = 0
    if (-x * x > log_small) erfc_real = exp(-x * x) / (sqrt_pi * f)
    if (erfc_real < small) erfc_real = 0
  end function erfc_real

  ! BETA_VALUE is gamma(a) gamma(b) / gamma(a + b), for positive A and B:
  ! by the gamma function itself while a + b is under 100, and by its
  ! logarithms past that, where gamma(a + b) could be too large.
  function beta_value(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: x, y

    if (.not. real_argument(a, x, v)) return
    if (.not. real_argument(b, y, v)) return
    if (x <= 0 .or. y <= 0) then
      v = error_value(e_domain)
      return
    end if
    if (x + y < 100) then
      v = gamma_value(x) * gamma_value(y) / gamma_value(x + y)
    else
      v = lgamma_value(x) + lgamma_value(y) - lgamma_value(x + y)
      if (v%err == 0) v = exp_real(real(v%z, rk))
    end if
  end function beta_value

  ! ------------------------------------------------------------------
  ! Lists.

  ! ORDER_STATISTIC is the least, the greatest, or the median of a list of
  ! real numbers; the median of an even number of them is the mean of the
  ! middle two.
  function order_statistic(name, args) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: args(:)
    type(value) :: v
    real(rk) :: x(size(args))
    integer :: n

    if (.not. all_real(args, v)) return
    x = real(args%z, rk)
    n = size(x)
    select case (name)
    case ('min')
      v = real_value(minval(x))
    case ('max')
      v = real_value(maxval(x))
    case default
      if (mod(n, 2) == 1) then
        v = real_value(kth(x, (n + 1) / 2))
      else
        v = real_value(kth(x, n / 2) / 2 + kth(x, n / 2 + 1) / 2)
      end if
    end select
  end function order_statistic

  ! ALL_REAL: every value in ARGS is real; otherwise V is NOTREAL.
  logical function all_real(args, v)
    type(value), intent(in) :: args(:)
    type(value), intent(inout) :: v

    all_real = all(aimag(args%z) == 0)
    if (.not. all_real) v = error_value(e_notreal)
  end function all_real

  ! KTH is the Kth smallest of X, by quickselect: the values below a pivot,
  ! those equal to it, and those above, each part PACKed from X.
  !
  ! COUNT(x < p) counts the elements for which the logical array X < P is
  ! true; PACK(x, x < p) is a new array of just those elements, in order.
  ! Passing PACK's result straight to KTH's assumed-shape dummy makes a
  ! temporary array of the right size for each call. Quickselect is
  ! quicksort that recurses into only the part holding the Kth value, so
  ! it takes time in proportion to n on average, rather than sorting all.
  recursive function kth(x, k) result(r)
    real(rk), intent(in) :: x(:)
    integer, intent(in) :: k
    real(rk) :: r
    real(rk) :: p
    integer :: below, equal

    p = x((size(x) + 1) / 2)
    below = count(x < p)
    equal = count(x == p)
    if (k <= below) then
      r = kth(pack(x, x < p), k)
    else if (k <= below + equal) then
      r = p
    else
      r = kth(pack(x, x > p), k - below - equal)
    end if
  end function kth

  ! SUM_VALUE and PRODUCT_VALUE add and multiply the values in turn, by
  ! FCVALUES' operators, so a sum is the same as the + of its terms.
  function sum_value(args) result(v)
    type(value), intent(in) :: args(:)
    type(value) :: v
    integer :: k

    v = args(1)
    do k = 2, size(args)
      v = v + args(k)
    end do
  end function sum_value

  function product_value(args) result(v)
    type(value), intent(in) :: args(:)
    type(value) :: v
    integer :: k

    v = args(1)
    do k = 2, size(args)
      v = v * args(k)
    end do
  end function product_value

  ! SPREAD_VALUE is the sample variance of real numbers, or the standard
  ! deviation, its square root. The deviations from the mean are scaled by
  ! the largest, S, so no square is too large or too small:
  !     var = S**2 sum(y**2) / (n - 1),   y = (x - mean) / S
  !
  ! X is an automatic array: its size, SIZE(ARGS), is known only when the
  ! function is called, and it is made on entry and gone on return. X =
  ! X - M is an array expression: Fortran 90 applies it to every element.
  ! WHERE assigns only to the elements where its mask is true, and
  ! ELSEWHERE to the rest. The two-pass method (the mean first, then the
  ! deviations from it) is the accurate one: the one-pass formula
  ! sum(x**2) - n mean**2 cancels badly when the values are close together.
  function spread_value(name, args) result(v)
    character(len=*), intent(in) :: name
    type(value), intent(in) :: args(:)
    type(value) :: v
    real(rk) :: x(size(args)), m, s, q, f, r
    integer :: n, err

    if (.not. all_real(args, v)) return
    n = size(args)
    x = real(args%z, rk)

    ! Numbers this small could leave differences below a VAX's smallest:
    ! scale them up, and the result down.
    f = 1
    if (maxval(abs(x)) < 1.0e-200_rk) f = 1.0e100_rk
    x = x * f

    m = sum(x) / n
    x = x - m
    s = maxval(abs(x))
    if (s == 0) then
      v = real_value(0.0_rk)
      return
    end if
    ! A deviation this much smaller than the largest adds nothing to the
    ! sum of squares, and its square might underflow.
    where (abs(x) < s * 1.0e-100_rk)
      x = 0
    elsewhere
      x = x / s
    end where
    q = sum(x * x) / (n - 1)

    err = 0
    if (name == 'var') then
      r = r_div(r_div(r_mul(r_mul(s, s, err), q, err), f, err), f, err)
    else
      r = r_div(r_mul(s, sqrt(q), err), f, err)
    end if
    if (err /= 0) then
      v = error_value(err)
    else
      v = real_value(r)
    end if
  end function spread_value

  ! RANDOM_VALUE is rand(), a number in [0, 1), or rand(a, b), one in
  ! [a, b), from RANDOM_NUMBER; rand(n) is a whole number from 1 to n.
  function random_value(args) result(v)
    type(value), intent(in) :: args(:)
    type(value) :: v
    real(rk) :: r, a, b

    call random_number(r)
    select case (size(args))
    case (0)
      v = real_value(r)
    case (1)
      if (.not. whole_argument(args(1), b, v)) return
      if (b < 1) then
        v = error_value(e_domain)
      else
        v = real_value(min(aint(r * b) + 1, b))
      end if
    case default
      if (.not. real_argument(args(1), a, v)) return
      if (.not. real_argument(args(2), b, v)) return
      v = real_value(a) + real_value(b - a) * real_value(r)
    end select
  end function random_value

  ! SEED_RANDOM is SET SEED N: RANDOM_NUMBER's sequence started again from
  ! a seed made of N, so the same N gives the same numbers after it.
  !
  ! RANDOM_SEED is the intrinsic subroutine that reads or sets the random
  ! number generator's state, a vector of default integers whose length is
  ! the compiler's own choice (one on govax, eight under gfortran), which
  ! SIZE= asks for. So the seed is an allocatable array of that size: each
  ! element N plus a different amount, since some generators do poorly on
  ! a seed whose elements are all alike. The sequences themselves differ
  ! between compilers; only the repetition is the same everywhere.
  subroutine seed_random(n)
    integer, intent(in) :: n
    integer, allocatable :: seed(:)
    integer :: count, k

    call random_seed(size=count)
    allocate (seed(count))
    seed = (/ (n + 7919 * k, k = 1, count) /)
    call random_seed(put=seed)
    deallocate (seed)
    seed_given = .true.
    seed_number = n
  end subroutine seed_random
end module fcfuncs
