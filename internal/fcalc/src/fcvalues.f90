! fcalc: values and their arithmetic.
!
! A VALUE is a complex number and an error code: an operation that fails
! gives a value with its error set, and an operation on such a value passes
! the error on, as an IEEE NaN would, so the evaluator need only look at the
! last value it gets to know whether everything before it went well.
!
! Arithmetic never goes near the machine's limits (module FCKINDS): each
! operation on the real parts is done by R_ADD, R_MUL, or R_DIV, which check
! their operands first, report a result past LIMIT as an overflow, and give
! zero for one below SMALL, without the machine ever overflowing or
! underflowing. So the same expression gives the same value, or the same
! error, on a VAX and on an IEEE machine, and nothing fcalc computes traps.
!
! The checks are cheap comparisons made before an operation, never after:
! on a VAX the operation itself would already have stopped the program.
! For a product x*y, |x*y| > LIMIT exactly when |y| > LIMIT/|x| (for |x| of
! 1 or more), and LIMIT/|x| can't overflow; that rewriting of a test so it
! is safe to compute is the main trick of the module.
!
! Fortran 90 lets a program give its own meaning to an operator for its
! own types: INTERFACE OPERATOR(+) below names the functions + means for
! VALUEs, so the evaluator writes a + b, and the compiler calls V_ADD(a, b).
! Several functions may share one operator (or one generic name), each for
! different argument types; the compiler picks by the arguments, as it
! picks real or integer addition.
module fcvalues
  use fckinds
  use fcmessages
  implicit none
  private

  public :: value, real_value, complex_value, error_value, format_value
  public :: format_real, is_real, is_whole, truth, logical_value
  public :: v_mod, v_factorial, v_compare, v_abs, c_abs
  public :: operator(+), operator(-), operator(*), operator(/), operator(**)
  public :: digits, r_add, r_mul, r_div, c_add, c_sub, c_mul, c_div, near_to
  public :: exp_real, exp_value, log_value, clean, angle_of
  public :: number_format, number_base, f_auto, f_fixed, f_scientific, f_engineering

  ! A VALUE: Z, the number, and ERR, 0 or the code of the error that made
  ! it. A function whose result is a VALUE returns both at once.
  type value
    complex(rk) :: z
    integer :: err
  end type value

  ! The significant digits a value is shown with (SET DIGITS), or under
  ! FIXED the places after the point.
  integer, save :: digits = 12

  ! How a value is shown (SET FORMAT), and the base a whole number is shown
  ! in (SET BASE): 10, 16, 8, or 2.
  integer, parameter :: f_auto = 1, f_fixed = 2, f_scientific = 3, f_engineering = 4
  integer, save :: number_format = f_auto
  integer, save :: number_base = 10

  interface operator(+)
    module procedure v_add, v_plus
  end interface

  interface operator(-)
    module procedure v_sub, v_neg
  end interface

  interface operator(*)
    module procedure v_mul
  end interface

  interface operator(/)
    module procedure v_div
  end interface

  interface operator(**)
    module procedure v_power
  end interface

  ! Values whose difference is no more than this part of the larger are
  ! equal (V_COMPARE): 0.1 + 0.2 == 0.3, whatever the machine's rounding.
  real(rk), parameter :: tolerance = 64 * eps

contains

  ! ------------------------------------------------------------------
  ! Making values.

  ! A FUNCTION's RESULT clause names the variable that holds its value
  ! (V here), so the function's own name needn't be used as one: needed
  ! for a recursive function, and clearer anywhere. CMPLX(x, y, rk) makes a
  ! complex number of kind RK; without the kind, CMPLX's result would be
  ! the default kind, single precision, and digits would be lost.
  function real_value(x) result(v)
    real(rk), intent(in) :: x
    type(value) :: v

    v = clean(cmplx(x, 0.0_rk, rk))
  end function real_value

  function complex_value(x, y) result(v)
    real(rk), intent(in) :: x, y
    type(value) :: v

    v = clean(cmplx(x, y, rk))
  end function complex_value

  function error_value(code) result(v)
    integer, intent(in) :: code
    type(value) :: v

    v%z = (0.0_rk, 0.0_rk)
    v%err = code
  end function error_value

  ! CLEAN makes a computed number a value: a part past LIMIT is an
  ! overflow, and one below SMALL zero (a positive zero: an IEEE machine's
  ! -0 shows as 0).
  function clean(z) result(v)
    complex(rk), intent(in) :: z
    type(value) :: v
    real(rk) :: x, y

    x = real(z, rk)
    y = aimag(z)
    v%err = 0
    if (abs(x) > limit .or. abs(y) > limit) then
      v = error_value(e_overflow)
      return
    end if
    if (abs(x) < small) x = 0
    if (abs(y) < small) y = 0
    v%z = cmplx(x, y, rk)
  end function clean

  ! ------------------------------------------------------------------
  ! What a value is.

  logical function is_real(v)
    type(value), intent(in) :: v

    is_real = aimag(v%z) == 0
  end function is_real

  ! IS_WHOLE: a real value that is a whole number.
  logical function is_whole(v)
    type(value), intent(in) :: v
    real(rk) :: x

    x = real(v%z, rk)
    is_whole = is_real(v) .and. abs(x) < whole_limit
    if (is_whole) is_whole = aint(x) == x
  end function is_whole

  ! TRUTH: a value is true if it isn't zero.
  logical function truth(v)
    type(value), intent(in) :: v

    truth = v%z /= (0.0_rk, 0.0_rk)
  end function truth

  function logical_value(b) result(v)
    logical, intent(in) :: b
    type(value) :: v

    if (b) then
      v = real_value(1.0_rk)
    else
      v = real_value(0.0_rk)
    end if
  end function logical_value

  ! ------------------------------------------------------------------
  ! The guarded operations on real numbers. Each takes ERR, which it sets
  ! to E_OVERFLOW or E_DIVZERO if it is zero and the operation fails, and
  ! gives a result whose magnitude is zero or between SMALL and LIMIT.
  ! Only the first error is kept (IF (ERR == 0)), so a chain of operations
  ! can share one ERR and report what went wrong first.
  !
  ! A result smaller than SMALL becomes 0, as IEEE's gradual underflow and
  ! a VAX's quiet zero would both nearly do, but alike on both. The
  ! comparisons are arranged so the machine itself never computes a value
  ! below its own smallest number: a VAX compiled /CHECK=UNDERFLOW faults
  ! on that.

  real(rk) function r_add(x, y, err)
    real(rk), intent(in) :: x, y
    integer, intent(inout) :: err
    real(rk), parameter :: scale = 1.0e20_rk
    real(rk) :: s

    r_add = 0
    if (max(abs(x), abs(y)) < 1.0e-280_rk) then
      ! Two values this small could leave a difference below the VAX's
      ! smallest number: add them scaled up instead.
      s = x * scale + y * scale
      if (abs(s) >= small * scale) r_add = s / scale
      return
    end if

    s = x + y
    if (abs(s) > limit) then
      if (err == 0) err = e_overflow
      return
    end if
    if (abs(s) >= small) r_add = s
  end function r_add

  real(rk) function r_mul(x, y, err)
    real(rk), intent(in) :: x, y
    integer, intent(inout) :: err
    real(rk) :: ax, ay

    r_mul = 0
    ax = abs(x)
    ay = abs(y)
    if (ax == 0 .or. ay == 0) return

    if (ax >= 1) then
      if (ay > limit / ax) then
        if (err == 0) err = e_overflow
        return
      end if
    else if (ay < small / ax) then
      return
    end if

    r_mul = x * y
  end function r_mul

  real(rk) function r_div(x, y, err)
    real(rk), intent(in) :: x, y
    integer, intent(inout) :: err
    real(rk) :: ax, ay

    r_div = 0
    ax = abs(x)
    ay = abs(y)
    if (ay == 0) then
      if (err == 0) err = e_divzero
      return
    end if
    if (ax == 0) return

    if (ay >= 1) then
      if (ax < small * ay) return
    else if (ax > limit * ay) then
      if (err == 0) err = e_overflow
      return
    end if

    r_div = x / y
  end function r_div

  ! C_ABS is a complex number's magnitude, never squaring a part that could
  ! overflow or underflow.
  real(rk) function c_abs(z)
    complex(rk), intent(in) :: z
    real(rk) :: m, n, q
    logical :: small_part

    m = max(abs(real(z, rk)), abs(aimag(z)))
    n = min(abs(real(z, rk)), abs(aimag(z)))
    ! N is negligible beside M when N <= M * 1E-17; for M below 1 that
    ! product could underflow, so the test is made as N * 1E17 <= M there.
    if (m >= 1) then
      small_part = n <= m * 1.0e-17_rk
    else
      small_part = n * 1.0e17_rk <= m
    end if
    if (small_part) then
      c_abs = m
    else
      q = n / m
      c_abs = m * sqrt(1 + q * q)
    end if
  end function c_abs

  ! ------------------------------------------------------------------
  ! The guarded operations on complex numbers, built on those on real
  ! numbers. Each takes ERR as R_ADD does. The operators below apply them
  ! to VALUEs; module FCMATRIX applies them to the elements of matrices,
  ! where making a VALUE of each element would only be in the way.

  complex(rk) function c_add(x, y, err)
    complex(rk), intent(in) :: x, y
    integer, intent(inout) :: err

    c_add = cmplx(r_add(real(x, rk), real(y, rk), err), r_add(aimag(x), aimag(y), err), rk)
  end function c_add

  ! (Negating a number is exact, so X - Y is X + (-Y).)
  complex(rk) function c_sub(x, y, err)
    complex(rk), intent(in) :: x, y
    integer, intent(inout) :: err

    c_sub = c_add(x, -y, err)
  end function c_sub

  ! C_MUL: (p + qi)(r + si) = (pr - qs) + (ps + qr)i.
  complex(rk) function c_mul(x, y, err)
    complex(rk), intent(in) :: x, y
    integer, intent(inout) :: err
    real(rk) :: p, q, r, s

    p = real(x, rk)
    q = aimag(x)
    r = real(y, rk)
    s = aimag(y)
    c_mul = cmplx(r_add(r_mul(p, r, err), -r_mul(q, s, err), err), &
      r_add(r_mul(p, s, err), r_mul(q, r, err), err), rk)
  end function c_mul

  ! C_DIV divides by Smith's method, which scales by the divisor's larger
  ! part so that no product is larger than it must be. The textbook
  !     (p + qi) / (r + si) = ((pr + qs) + (qr - ps)i) / (r**2 + s**2)
  ! squares the divisor's parts, which overflows for parts near 1E154 and
  ! underflows near 1E-154, although the quotient may be ordinary. Smith
  ! divides through by the larger of r and s first: with |r| >= |s| and
  ! t = s/r (at most 1 in size), the denominator is r + s t, never larger
  ! than 2|r|, and the numerators are p + q t and q - p t.
  complex(rk) function c_div(x, y, err)
    complex(rk), intent(in) :: x, y
    integer, intent(inout) :: err
    real(rk) :: p, q, r, s, ratio, den, a, b

    p = real(x, rk)
    q = aimag(x)
    r = real(y, rk)
    s = aimag(y)

    if (s == 0) then
      a = r_div(p, r, err)
      b = r_div(q, r, err)
    else if (r == 0) then
      a = r_div(q, s, err)
      b = -r_div(p, s, err)
    else if (abs(r) >= abs(s)) then
      ratio = s / r
      den = r_add(r, r_mul(s, ratio, err), err)
      a = r_div(r_add(p, r_mul(q, ratio, err), err), den, err)
      b = r_div(r_add(q, -r_mul(p, ratio, err), err), den, err)
    else
      ratio = r / s
      den = r_add(s, r_mul(r, ratio, err), err)
      a = r_div(r_add(r_mul(p, ratio, err), q, err), den, err)
      b = r_div(r_add(r_mul(q, ratio, err), -p, err), den, err)
    end if

    ! A part negated above may be an IEEE machine's -0, which a VAX hasn't:
    ! assigning 0 makes it +0, so later functions (ATAN2's branch) agree on
    ! both.
    if (a == 0) a = 0
    if (b == 0) b = 0
    c_div = cmplx(a, b, rk)
  end function c_div

  ! ------------------------------------------------------------------
  ! The operators.

  function v_plus(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    v = a
  end function v_plus

  function v_neg(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    if (a%err /= 0) then
      v = a
    else
      v = clean(-a%z)
    end if
  end function v_neg

  function v_add(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    integer :: err

    if (pass_error(a, b, v)) return
    err = 0
    v%z = c_add(a%z, b%z, err)
    v%err = err
  end function v_add

  function v_sub(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v

    v = a + (-b)
  end function v_sub

  function v_mul(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    integer :: err

    if (pass_error(a, b, v)) return
    err = 0
    v%z = c_mul(a%z, b%z, err)
    v%err = err
  end function v_mul

  ! V_DIV is A / B, by C_DIV.
  function v_div(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    integer :: err

    if (pass_error(a, b, v)) return
    err = 0
    v%z = c_div(a%z, b%z, err)
    if (err == 0) then
      v = clean(v%z)
    else
      v%err = err
    end if
  end function v_div

  ! V_POWER is A ** B, fcalc's ^: a whole power by repeated squaring (so
  ! 2^10 is exactly 1024), a positive real number's real power by EXP and
  ! LOG, and anything else as EXP(B * LOG(A)) with LOG's principal branch,
  ! so (-8)^(1/3) is 1 + 1.73205080757i.
  !
  ! Repeated squaring computes a**n in about log2(n) multiplications rather
  ! than n: n's binary digits are read from the lowest, BASE is squared at
  ! each step (a, a**2, a**4, a**8, ...), and multiplied into the result
  ! where a digit is 1. For n = 10 (binary 1010): a**2 * a**8.
  !
  ! RECURSIVE: V_POWER uses the operators *, /, which are other functions,
  ! and (through EXP_VALUE and LOG_VALUE) may come back to itself; a
  ! procedure that may be active twice at once must say RECURSIVE in
  ! Fortran 90, so the compiler gives each activation its own variables.
  recursive function v_power(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    type(value) :: base
    integer :: n
    real(rk) :: x, t

    if (pass_error(a, b, v)) return

    if (is_whole(b) .and. abs(real(b%z, rk)) <= 1.0e9_rk) then
      n = nint(real(b%z, rk))
      v = real_value(1.0_rk)
      if (n == 0) return
      base = a
      if (n < 0) then
        if (.not. truth(a)) then
          v = error_value(e_divzero)
          return
        end if
        base = real_value(1.0_rk) / a
        n = -n
      end if
      do
        if (mod(n, 2) == 1) v = v * base
        n = n / 2
        if (n == 0 .or. v%err /= 0) exit
        base = base * base
        if (base%err /= 0) then
          v = base
          exit
        end if
      end do
      return
    end if

    if (.not. truth(a)) then
      ! Zero to a power that isn't whole: zero if the power's real part
      ! is positive.
      if (real(b%z, rk) > 0) then
        v = real_value(0.0_rk)
      else
        v = error_value(e_domain)
      end if
      return
    end if

    x = real(a%z, rk)
    if (is_real(a) .and. x > 0 .and. is_real(b)) then
      v%err = 0
      t = r_mul(real(b%z, rk), log(x), v%err)
      if (v%err == 0) v = exp_real(t)
      return
    end if

    v = exp_value(b * log_value(a))
  end function v_power

  ! EXP_REAL is EXP(T) as a value: an overflow past LOG_LIMIT, zero below
  ! LOG_SMALL.
  function exp_real(t) result(v)
    real(rk), intent(in) :: t
    type(value) :: v

    if (t > log_limit) then
      v = error_value(e_overflow)
    else if (t < log_small) then
      v = real_value(0.0_rk)
    else
      v = real_value(exp(t))
    end if
  end function exp_real

  ! EXP_VALUE is a complex value's exponential, by Euler's formula:
  !     exp(x + iy) = e**x (cos y + i sin y)
  function exp_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    real(rk) :: m, y
    integer :: err

    if (a%err /= 0) then
      v = a
      return
    end if
    y = aimag(a%z)
    if (abs(y) > 1.0e15_rk) then
      ! An angle so large that its sine has no significant digits.
      v = error_value(e_domain)
      return
    end if
    v = exp_real(real(a%z, rk))
    if (v%err /= 0 .or. y == 0) return
    m = real(v%z, rk)
    err = 0
    v%z = cmplx(r_mul(m, cos(y), err), r_mul(m, sin(y), err), rk)
    v%err = err
  end function exp_value

  ! LOG_VALUE is a nonzero value's principal logarithm,
  !     ln z = ln |z| + i arg z,   arg z in (-pi, pi]
  ! A complex number has infinitely many logarithms, 2 pi i apart; the
  ! principal one takes the angle nearest zero, and the negative real axis,
  ! where the angle jumps from pi to -pi, is its branch cut.
  function log_value(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    v = complex_value(log(c_abs(a%z)), angle_of(aimag(a%z), real(a%z, rk)))
  end function log_value

  ! ANGLE_OF is ATAN2(Y, X) for a point other than the origin, where one
  ! coordinate is so much smaller than the other that ATAN2 would
  ! underflow, the angle to the axis it is on.
  real(rk) function angle_of(y, x)
    real(rk), intent(in) :: y, x

    if (abs(y) < abs(x) * 1.0e-200_rk) then
      angle_of = 0
      if (x < 0) then
        angle_of = pi
        if (y < 0) angle_of = -pi
      end if
    else if (abs(x) < abs(y) * 1.0e-200_rk) then
      angle_of = pi / 2
      if (y < 0) angle_of = -pi / 2
    else
      angle_of = atan2(y, x)
    end if
  end function angle_of

  ! V_ABS is a value's magnitude.
  function v_abs(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v

    if (a%err /= 0) then
      v = a
    else
      v = real_value(c_abs(a%z))
    end if
  end function v_abs

  ! V_MOD is A mod B, for real values: the remainder with the divisor's
  ! sign, as MODULO's. It is computed exactly, by subtracting the divisor
  ! doubled as often as fits, so a large quotient loses nothing.
  function v_mod(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: r, d, t

    if (pass_error(a, b, v)) return
    if (.not. (is_real(a) .and. is_real(b))) then
      v = error_value(e_notreal)
      return
    end if
    d = abs(real(b%z, rk))
    if (d == 0) then
      v = error_value(e_divzero)
      return
    end if

    r = abs(real(a%z, rk))
    do while (r >= d)
      t = d
      do while (t <= r / 2)
        t = t * 2
      end do
      r = r - t
    end do

    ! R is |A| mod |B|; give it the divisor's sign.
    if (r /= 0 .and. real(a%z, rk) < 0) r = d - r
    if (real(b%z, rk) < 0 .and. r /= 0) r = r - d
    v = real_value(r)
  end function v_mod

  ! V_FACTORIAL is N!, for a whole N from 0 up.
  function v_factorial(a) result(v)
    type(value), intent(in) :: a
    type(value) :: v
    integer :: n, k
    real(rk) :: x

    if (a%err /= 0) then
      v = a
      return
    end if
    if (.not. is_real(a)) then
      v = error_value(e_notreal)
      return
    end if
    if (.not. is_whole(a)) then
      v = error_value(e_notwhole)
      return
    end if
    x = real(a%z, rk)
    if (x < 0) then
      v = error_value(e_domain)
      return
    end if
    if (x > 200) then
      v = error_value(e_overflow)
      return
    end if

    n = nint(x)
    v%err = 0
    x = 1
    do k = 2, n
      x = r_mul(x, real(k, rk), v%err)
    end do
    if (v%err == 0) v = real_value(x)
  end function v_factorial

  ! V_COMPARE compares two values by a relation, 1 to 6: ==, <>, <, <=, >,
  ! >=. Equality is the whole value's, to within TOLERANCE; the order is
  ! the real parts'. The result is 1 for true and 0 for false.
  function v_compare(a, b, relation) result(v)
    type(value), intent(in) :: a, b
    integer, intent(in) :: relation
    type(value) :: v
    logical :: same, less
    real(rk) :: x, y

    if (pass_error(a, b, v)) return
    same = near(real(a%z, rk), real(b%z, rk)) .and. near(aimag(a%z), aimag(b%z))
    x = real(a%z, rk)
    y = real(b%z, rk)
    less = x < y .and. .not. near(x, y)
    select case (relation)
    case (1)
      v = logical_value(same)
    case (2)
      v = logical_value(.not. same)
    case (3)
      v = logical_value(less)
    case (4)
      v = logical_value(less .or. near(x, y))
    case (5)
      v = logical_value(.not. (less .or. near(x, y)))
    case default
      v = logical_value(.not. less)
    end select
  end function v_compare

  logical function near(x, y)
    real(rk), intent(in) :: x, y

    near = near_to(x, y, tolerance)
  end function near

  ! NEAR_TO: X and Y differ by no more than the part TOL of the larger.
  ! (Values within LIMIT can't overflow in X - Y; TOL is well above EPS, so
  ! TOL times the larger can't underflow but where both are near SMALL,
  ! when the test is made as a quotient.)
  logical function near_to(x, y, tol)
    real(rk), intent(in) :: x, y, tol
    real(rk) :: d, m

    d = abs(x - y)
    m = max(abs(x), abs(y))
    if (m >= 1) then
      near_to = d <= tol * m
    else
      near_to = d / tol <= m
    end if
  end function near_to

  ! PASS_ERROR: if A or B has an error, V is that value and the result is
  ! true.
  logical function pass_error(a, b, v)
    type(value), intent(in) :: a, b
    type(value), intent(out) :: v

    pass_error = .true.
    if (a%err /= 0) then
      v = a
    else if (b%err /= 0) then
      v = b
    else
      pass_error = .false.
      v%err = 0
      v%z = (0.0_rk, 0.0_rk)
    end if
  end function pass_error

  ! ------------------------------------------------------------------
  ! Showing values.
  !
  ! How a number is shown depends on three settings: DIGITS (SET DIGITS),
  ! NUMBER_FORMAT (SET FORMAT), and NUMBER_BASE (SET BASE). The formats are
  ! a scientific calculator's:
  !
  !   AUTO         DIGITS significant digits, trailing zeros dropped, in
  !                fixed notation from 1E-5 up to where the digits run out,
  !                scientific outside that: 2, 0.125, 6.02214076e+23
  !   FIXED        DIGITS places after the point, every one shown: 2.0000
  !   SCIENTIFIC   DIGITS significant digits, one before the point, every
  !                one shown: 2.00000000000e+0
  !   ENGINEERING  as SCIENTIFIC, but the exponent a multiple of 3, so one to
  !                three digits before the point: 12.3456789012e+3
  !
  ! A whole number (a real value, or both parts of a complex one) shown in
  ! another base (SET BASE) is written with a prefix, 0x, 0o, or 0b, as
  ! fcalc reads it back; any other value is shown in decimal.

  ! FORMAT_REAL shows X by the format. A negative number is shown with its
  ! minus sign, unless it shows as all zeros (FIXED's 0.0000).
  function format_real(x) result(text)
    real(rk), intent(in) :: x
    character(len=32) :: text

    select case (number_format)
    case (f_fixed)
      text = fixed_text(abs(x))
    case (f_scientific)
      text = scientific_text(abs(x), digits)
    case (f_engineering)
      text = engineering_text(abs(x))
    case default
      text = auto_text(abs(x))
    end select

    ! SCAN finds the first of a set of characters in a string (0 if none):
    ! here, any digit that isn't zero.
    if (x < 0 .and. scan(text, '123456789') > 0) text = '-' // trim(text)
  end function format_real

  ! DECIMAL_DIGITS writes A (positive) to N significant digits: D(1:N) the
  ! digits, without the point, and EX the power of ten of the first.
  !
  ! The number is written by Fortran's own conversion, to an internal file:
  ! a WRITE whose unit is a CHARACTER variable writes into the variable
  ! instead of to a device, and a READ from one reads it, so these are
  ! Fortran's way to turn numbers into text and back. The format is itself
  ! built at run time as text, (ESw.dE3): ES is scientific notation with one
  ! digit before the point, w the field's width, d the digits after the
  ! point, E3 a three-digit exponent. Fortran's conversion rounds the decimal
  ! digits correctly, so fcalc needn't; it only rearranges them. (Fortran 90
  ! has no I0, which would write a number in as few characters as it needs:
  ! I2 writes 1 as " 1", and a blank in a format is nothing.)
  subroutine decimal_digits(a, n, d, ex)
    real(rk), intent(in) :: a
    integer, intent(in) :: n
    character(len=max_digits), intent(out) :: d
    integer, intent(out) :: ex
    character(len=40) :: buf
    character(len=20) :: form
    integer :: at

    write (form, '(A, I2, A, I2, A)') '(ES', n + 10, '.', n - 1, 'E3)'
    write (buf, form) a
    buf = adjustl(buf)

    ! BUF is d.ddd...E+xxx (or d.E+xxx for one digit).
    d = buf(1:1) // buf(3:n + 1)
    at = index(buf, 'E')
    read (buf(at + 1:at + 4), '(I4)') ex
  end subroutine decimal_digits

  ! EXPONENT_TEXT is e+7 or e-12: as few digits as the exponent needs.
  function exponent_text(ex) result(text)
    integer, intent(in) :: ex
    character(len=6) :: text
    character(len=4) :: buf

    write (buf, '(I4)') abs(ex)
    if (ex < 0) then
      text = 'e-' // adjustl(buf)
    else
      text = 'e+' // adjustl(buf)
    end if
  end function exponent_text

  ! AUTO_TEXT shows A (not negative) to DIGITS significant digits, with no
  ! trailing zeros: in fixed notation from 1E-5 up to where the digits run
  ! out, in scientific notation outside that, as 1.5e-7 or 6.02214076e+23.
  function auto_text(a) result(text)
    real(rk), intent(in) :: a
    character(len=32) :: text
    character(len=max_digits) :: d
    integer :: k, ex

    if (a == 0) then
      text = '0'
      return
    end if

    call decimal_digits(a, digits, d, ex)

    ! K is the last digit that isn't a zero.
    k = digits
    do while (k > 1)
      if (d(k:k) /= '0') exit
      k = k - 1
    end do

    if (ex >= -5 .and. ex < digits) then
      if (ex < 0) then
        text = '0.' // repeat('0', -ex - 1) // d(1:k)
      else if (k <= ex + 1) then
        text = d(1:k) // repeat('0', ex + 1 - k)
      else
        text = d(1:ex + 1) // '.' // d(ex + 2:k)
      end if
    else
      text = d(1:1)
      if (k > 1) text = d(1:1) // '.' // d(2:k)
      text = trim(text) // exponent_text(ex)
    end if
  end function auto_text

  ! SCIENTIFIC_TEXT shows A (not negative) to N significant digits, every
  ! one shown, one before the point: 3.14159265359e+0. Zero is
  ! 0.00000000000e+0, the same width as the rest.
  function scientific_text(a, n) result(text)
    real(rk), intent(in) :: a
    integer, intent(in) :: n
    character(len=32) :: text
    character(len=max_digits) :: d
    integer :: ex

    if (a == 0) then
      d = repeat('0', n)
      ex = 0
    else
      call decimal_digits(a, n, d, ex)
    end if
    text = d(1:1)
    if (n > 1) text = d(1:1) // '.' // d(2:n)
    text = trim(text) // exponent_text(ex)
  end function scientific_text

  ! ENGINEERING_TEXT is SCIENTIFIC_TEXT with the exponent a multiple of 3
  ! (the powers that have names: kilo, mega, milli, micro), so the point
  ! moves right by MODULO(EX, 3), leaving one to three digits before it.
  ! MODULO, unlike MOD, takes the sign of the divisor: MODULO(-7, 3) is 2,
  ! so 1.5e-7 is 150e-9. With fewer digits than go before the point, zeros
  ! fill the places: 1.5e+5 to two digits is 150e+3.
  function engineering_text(a) result(text)
    real(rk), intent(in) :: a
    character(len=32) :: text
    character(len=max_digits) :: d
    integer :: ex, shift, lead

    if (a == 0) then
      d = repeat('0', digits)
      ex = 0
    else
      call decimal_digits(a, digits, d, ex)
    end if
    shift = modulo(ex, 3)
    lead = shift + 1

    if (digits < lead) then
      text = d(1:digits) // repeat('0', lead - digits)
    else if (digits == lead) then
      text = d(1:lead)
    else
      text = d(1:lead) // '.' // d(lead + 1:digits)
    end if
    text = trim(text) // exponent_text(ex - shift)
  end function engineering_text

  ! FIXED_TEXT shows A (not negative) with DIGITS places after the point,
  ! by Fortran's F editing: (F40.4) writes 3.14159... as 3.1416, rounded.
  ! A number too small to show any digit is all zeros, 0.0000. One so large
  ! that its digits before the point and the places after would pass the
  ! MAX_DIGITS the kind holds is shown in scientific notation instead, with
  ! as many digits after the point, so no digit is shown that the machines'
  ! last bits could make differ. Fortran may write a number below 1 with
  ! no zero before the point (.5000), and fcalc puts one there.
  function fixed_text(a) result(text)
    real(rk), intent(in) :: a
    character(len=32) :: text
    character(len=40) :: buf
    character(len=20) :: form

    if (a < 0.5_rk * 10.0_rk ** (-digits)) then
      text = '0.' // repeat('0', digits)
      return
    end if
    if (a >= 10.0_rk ** (max_digits - digits)) then
      text = scientific_text(a, min(digits + 1, max_digits))
      return
    end if

    write (form, '(A, I2, A)') '(F40.', digits, ')'
    write (buf, form) a
    buf = adjustl(buf)
    if (buf(1:1) == '.') buf = '0' // trim(buf)
    text = trim(buf)
  end function fixed_text

  ! IS_WHOLE_PART: X is a whole number fcalc may show in another base.
  logical function is_whole_part(x)
    real(rk), intent(in) :: x

    is_whole_part = abs(x) < whole_limit
    if (is_whole_part) is_whole_part = aint(x) == x
  end function is_whole_part

  ! BASED_TEXT shows the whole number X in NUMBER_BASE, with its prefix:
  ! 0xFF, 0o377, 0b11111111; zero is 0 in every base. The digits come
  ! last first, each the remainder of a division by the base. The bases
  ! are powers of two, so on a binary machine each division, AINT, and
  ! product here is exact: whole numbers below WHOLE_LIMIT need no integer
  ! type wider than the 32 bits every Fortran 90 compiler has.
  function based_text(x) result(text)
    real(rk), intent(in) :: x
    character(len=64) :: text
    character(len=*), parameter :: hex = '0123456789ABCDEF'
    character(len=56) :: shown
    real(rk) :: a, q, b
    integer :: k, r

    if (x == 0) then
      text = '0'
      return
    end if

    b = number_base
    a = abs(x)
    k = len(shown) + 1
    do
      q = aint(a / b)
      r = nint(a - q * b)
      k = k - 1
      shown(k:k) = hex(r + 1:r + 1)
      a = q
      if (a == 0) exit
    end do

    select case (number_base)
    case (16)
      text = '0x' // shown(k:)
    case (8)
      text = '0o' // shown(k:)
    case default
      text = '0b' // shown(k:)
    end select
    if (x < 0) text = '-' // trim(text)
  end function based_text

  ! FORMAT_VALUE shows a value: a real one as a number, a complex one as
  ! a + bi. A part smaller than the other by more than the digits shown
  ! is shown as zero: what the digits can't show. BESIDE, if it is given,
  ! is a magnitude the parts are shown beside, so a part smaller than it by
  ! more than the digits is shown as zero too: an element of a matrix is
  ! shown beside the matrix's largest (module FCARRAYS). Under FIXED, a part
  ! too small to show a digit is zero as well, so 1e-9 + 1e-6i is 0.0000.
  ! A value whose parts are whole numbers is shown in NUMBER_BASE, exactly:
  ! a whole number has no rounding to hide.
  function format_value(v, beside) result(text)
    type(value), intent(in) :: v
    real(rk), intent(in), optional :: beside
    character(len=120) :: text
    real(rk) :: x, y, part, big
    logical :: based

    x = real(v%z, rk)
    y = aimag(v%z)
    based = number_base /= 10 .and. is_whole_part(x) .and. is_whole_part(y)

    if (.not. based) then
      part = 10.0_rk ** (-digits)
      big = 0
      if (present(beside)) big = beside
      if (negligible(y, max(abs(x), big))) y = 0
      if (negligible(x, max(abs(y), big))) x = 0
      if (number_format == f_fixed) then
        if (abs(x) < 0.5_rk * part) x = 0
        if (abs(y) < 0.5_rk * part) y = 0
      end if
    end if

    if (y == 0) then
      text = part_text(x)
    else if (x == 0) then
      text = trim(part_text(y)) // 'i'
    else if (y < 0) then
      text = trim(part_text(x)) // ' - ' // trim(part_text(-y)) // 'i'
    else
      text = trim(part_text(x)) // ' + ' // trim(part_text(y)) // 'i'
    end if
  contains

    ! PART_TEXT shows one part, in the base or by the format.
    function part_text(p) result(t)
      real(rk), intent(in) :: p
      character(len=64) :: t

      if (based) then
        t = based_text(p)
      else
        t = format_real(p)
      end if
    end function part_text

    ! NEGLIGIBLE: |A| < |B| * PART, A too small beside B to show. The
    ! product is never formed where it could underflow (B near SMALL),
    ! which a VAX compiled /CHECK=UNDERFLOW would fault on: for |B| below 1
    ! the test is |A| / PART < |B|, whose quotient can't underflow, and an
    ! A of 1 or more can't be smaller than such a B.
    logical function negligible(a, b)
      real(rk), intent(in) :: a, b

      if (abs(b) >= 1) then
        negligible = abs(a) < abs(b) * part
      else if (abs(a) >= 1) then
        negligible = .false.
      else
        negligible = abs(a) / part < abs(b)
      end if
    end function negligible
  end function format_value
end module fcvalues
