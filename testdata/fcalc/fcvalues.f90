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
  public :: digits, r_add, r_mul, r_div
  public :: exp_real, exp_value, log_value, clean, angle_of

  ! A VALUE: Z, the number, and ERR, 0 or the code of the error that made
  ! it. A function whose result is a VALUE returns both at once.
  type value
    complex(rk) :: z
    integer :: err
  end type value

  ! The significant digits a value is shown with (SET DIGITS).
  integer, save :: digits = 12

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
    is_whole = is_real(v) .and. abs(x) < 4.0e15_rk
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

    m = max(abs(real(z, rk)), abs(aimag(z)))
    n = min(abs(real(z, rk)), abs(aimag(z)))
    if (n <= m * 1.0e-17_rk) then
      c_abs = m
    else
      q = n / m
      c_abs = m * sqrt(1 + q * q)
    end if
  end function c_abs

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
    v%z = cmplx(r_add(real(a%z, rk), real(b%z, rk), err), &
      r_add(aimag(a%z), aimag(b%z), err), rk)
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
    real(rk) :: p, q, r, s
    integer :: err

    if (pass_error(a, b, v)) return
    err = 0
    p = real(a%z, rk)
    q = aimag(a%z)
    r = real(b%z, rk)
    s = aimag(b%z)
    v%z = cmplx(r_add(r_mul(p, r, err), -r_mul(q, s, err), err), &
      r_add(r_mul(p, s, err), r_mul(q, r, err), err), rk)
    v%err = err
  end function v_mul

  ! V_DIV divides by Smith's method, which scales by the divisor's larger
  ! part so that no product is larger than it must be. The textbook
  !     (p + qi) / (r + si) = ((pr + qs) + (qr - ps)i) / (r**2 + s**2)
  ! squares the divisor's parts, which overflows for parts near 1E154 and
  ! underflows near 1E-154, although the quotient may be ordinary. Smith
  ! divides through by the larger of r and s first: with |r| >= |s| and
  ! t = s/r (at most 1 in size), the denominator is r + s t, never larger
  ! than 2|r|, and the numerators are p + q t and q - p t.
  function v_div(a, b) result(v)
    type(value), intent(in) :: a, b
    type(value) :: v
    real(rk) :: p, q, r, s, ratio, den
    integer :: err

    if (pass_error(a, b, v)) return
    err = 0
    p = real(a%z, rk)
    q = aimag(a%z)
    r = real(b%z, rk)
    s = aimag(b%z)

    if (s == 0) then
      v%z = cmplx(r_div(p, r, err), r_div(q, r, err), rk)
    else if (r == 0) then
      v%z = cmplx(r_div(q, s, err), -r_div(p, s, err), rk)
    else if (abs(r) >= abs(s)) then
      ratio = s / r
      den = r_add(r, r_mul(s, ratio, err), err)
      v%z = cmplx(r_div(r_add(p, r_mul(q, ratio, err), err), den, err), &
        r_div(r_add(q, -r_mul(p, ratio, err), err), den, err), rk)
    else
      ratio = r / s
      den = r_add(s, r_mul(r, ratio, err), err)
      v%z = cmplx(r_div(r_add(r_mul(p, ratio, err), q, err), den, err), &
        r_div(r_add(r_mul(q, ratio, err), -p, err), den, err), rk)
    end if

    ! A part negated above may be an IEEE machine's -0, which a VAX hasn't:
    ! CLEAN makes it 0, so later functions (ATAN2's branch) agree on both.
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
    real(rk) :: d

    d = abs(x - y)
    near = d <= tolerance * max(abs(x), abs(y))
  end function near

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

  ! FORMAT_REAL shows X to DIGITS significant digits, with no trailing
  ! zeros: in fixed notation from 1E-5 up to where the digits run out, in
  ! scientific notation outside that, as 1.5e-7 or 6.02214076e+23.
  !
  ! The number is first written by Fortran's own conversion, to an
  ! internal file: a WRITE whose unit is a CHARACTER variable writes into
  ! the variable instead of to a device, and a READ from one reads it, so
  ! these are Fortran's way to turn numbers into text and back. The format
  ! is itself built at run time as text, (ESw.dE3): ES is scientific
  ! notation with one digit before the point, w the field's width, d the
  ! digits after the point, E3 a three-digit exponent. Fortran's conversion
  ! rounds the decimal digits correctly, so FORMAT_REAL needn't; it only
  ! rearranges them.
  function format_real(x) result(text)
    real(rk), intent(in) :: x
    character(len=32) :: text
    character(len=40) :: buf
    character(len=20) :: form
    character(len=max_digits) :: d
    character(len=1) :: sign
    integer :: k, ex, at

    if (x == 0) then
      text = '0'
      return
    end if

    ! (Fortran 90 has no I0: a blank in a format is nothing.)
    write (form, '(A, I2, A, I2, A)') '(ES', digits + 10, '.', digits - 1, 'E3)'
    write (buf, form) abs(x)
    buf = adjustl(buf)

    ! BUF is d.ddd...E+xxx: the digits, without the point, and the
    ! exponent.
    d = buf(1:1) // buf(3:digits + 1)
    at = index(buf, 'E')
    read (buf(at + 1:at + 4), '(I4)') ex

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
      sign = '+'
      if (ex < 0) sign = '-'
      write (buf, '(I3)') abs(ex)
      text = trim(text) // 'e' // sign // trim(adjustl(buf))
    end if

    if (x < 0) text = '-' // trim(text)
  end function format_real

  ! FORMAT_VALUE shows a value: a real one as a number, a complex one as
  ! a + bi. A part smaller than the other by more than the digits shown
  ! is shown as zero: what the digits can't show.
  function format_value(v) result(text)
    type(value), intent(in) :: v
    character(len=72) :: text
    real(rk) :: x, y, part

    x = real(v%z, rk)
    y = aimag(v%z)
    part = 10.0_rk ** (-digits)
    if (negligible(y, x)) y = 0
    if (negligible(x, y)) x = 0

    if (y == 0) then
      text = format_real(x)
    else if (x == 0) then
      text = trim(format_real(y)) // 'i'
    else if (y < 0) then
      text = trim(format_real(x)) // ' - ' // trim(format_real(-y)) // 'i'
    else
      text = trim(format_real(x)) // ' + ' // trim(format_real(y)) // 'i'
    end if
  contains

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
