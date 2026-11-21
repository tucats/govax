! fcalc: polynomials, as vectors of their coefficients.
!
! A polynomial is a vector of its coefficients, the highest power's first:
! [1, -3, 2] is x^2 - 3x + 2. (MATLAB and Octave write them so, and poly
! and roots undo each other.) Three functions, in FCMATRIX's table:
!
!     roots(p)      the roots of p, a column (FCMATRIX sorts them)
!     poly(r)       the polynomial whose roots are the elements of r, with
!                   leading coefficient 1, a row
!     polyval(p, x) p's value at x, at each element of x
!
! The roots are found by the Durand-Kerner method (also called Weierstrass's
! method), which improves every root at once. For a polynomial with leading
! coefficient 1 and roots r1, ..., rn,
!
!     p(z) = (z - r1)(z - r2) ... (z - rn),
!
! so if z1, ..., zn are guesses, rk is zk less p(zk) divided by the product
! of (zk - zj) over the other guesses j, exactly, when the other guesses
! are right. The method makes that step for each guess in turn, using the
! guesses already improved (as the Gauss-Seidel method for linear systems
! does), until no guess moves. Each simple root is then found to the
! machine's precision, quadratically; a root of multiplicity m only to
! about 1/m of the digits, which is all any method can promise, since a
! change of one in the last bit of a coefficient moves such a root that
! far. A step of Newton's method on each root (z less p(z)/p'(z)) then
! polishes it, if it makes p(z) smaller.
!
! The guesses start on a circle that holds every root, by Fujiwara's bound
! (every root of z^n + a1 z^(n-1) + ... + an lies within twice the largest
! of |a1|, |a2|^(1/2), ..., |an/2|^(1/n)), spread around it with an offset
! so no guess starts on a line of symmetry of the roots (where the
! iteration could stay, unable to leave the real axis for a complex pair).
!
! Zero coefficients at the end are roots at zero, taken out first, so the
! roots of x^3 - x^2 are 0, 0, and 1 exactly. Every operation is guarded
! (FCVALUES' C_ADD, C_SUB, C_MUL, C_DIV), so nothing traps.
module fcpoly
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  implicit none
  private

  public :: quantity
  public :: polynomial_roots, polynomial_of, polynomial_value

  ! How many sweeps the iteration may make, and how many in a row may
  ! fail to improve on its best before it stops.
  integer, parameter :: sweep_limit = 500, stale_limit = 20

contains

  ! COEFFICIENTS: Q, a vector or a scalar, has its elements in C(1:N);
  ! NOTVECTOR for a matrix. C is a pointer array, ALLOCATEd here: the
  ! caller DEALLOCATEs it.
  logical function coefficients(q, c, n, bad)
    type(quantity), intent(in) :: q
    complex(rk), pointer :: c(:)
    integer, intent(out) :: n
    type(quantity), intent(out) :: bad
    type(value) :: v
    integer :: k

    coefficients = .false.
    nullify (c)
    bad = real_quantity(0.0_rk)
    if (is_array(q) .and. .not. is_vector(q)) then
      bad = failure(e_notvector)
      return
    end if
    n = numel(q)
    allocate (c(n))
    do k = 1, n
      ! (A component of a function's result can't be selected directly,
      ! ELEMENT_AT(Q, K)%Z: the value goes in a variable first.)
      v = element_at(q, k)
      c(k) = v%z
    end do
    coefficients = .true.
  end function coefficients

  ! ------------------------------------------------------------------
  ! Roots.

  ! POLYNOMIAL_ROOTS is roots(p): a column of P's roots, or a scalar for
  ! one. DOMAIN if P has no roots (it is a constant, or all zero).
  function polynomial_roots(p) result(q)
    type(quantity), intent(in) :: p
    type(quantity) :: q
    complex(rk), pointer :: c(:)
    integer :: n, first, last, degree, err

    if (.not. coefficients(p, c, n, q)) return

    ! FIRST is the first coefficient not 0, LAST the last: the zeros
    ! before FIRST don't count, and those after LAST are roots at 0.
    first = 1
    do while (first <= n)
      if (c(first) /= (0.0_rk, 0.0_rk)) exit
      first = first + 1
    end do
    last = n
    do while (last > first)
      if (c(last) /= (0.0_rk, 0.0_rk)) exit
      last = last - 1
    end do
    if (first >= n) then
      ! A constant, or every coefficient 0.
      deallocate (c)
      q = failure(e_domain)
      return
    end if

    degree = n - first
    err = 0
    q = roots_column(c(first:last), degree, err)
    deallocate (c)
  end function polynomial_roots

  ! ROOTS_COLUMN: the DEGREE roots, as a column, of a polynomial whose
  ! coefficients are A followed by zeros (taken off A already, so A(1) isn't
  ! 0, nor is A's last): each such zero is a root at 0.
  function roots_column(a, degree, err) result(q)
    complex(rk), intent(in) :: a(:)
    integer, intent(in) :: degree
    integer, intent(inout) :: err
    type(quantity) :: q
    complex(rk) :: m(size(a)), z(degree, 1)
    integer :: k, n

    ! M is A divided by its leading coefficient: monic, M(1) = 1. N, its
    ! degree, is how many roots aren't 0.
    n = size(a) - 1
    m(1) = (1.0_rk, 0.0_rk)
    do k = 2, n + 1
      m(k) = c_div(a(k), a(1), err)
    end do
    if (err /= 0) then
      q = failure(err)
      return
    end if

    z = (0.0_rk, 0.0_rk)
    if (n == 1) then
      ! z + m(2) = 0: the root, exactly.
      z(1, 1) = -m(2)
    else if (n > 1) then
      if (.not. durand_kerner(m, z(1:n, 1), err)) then
        if (err == 0) err = e_noroot
        q = failure(err)
        return
      end if
    end if
    ! Z(N+1:DEGREE) are the roots at zero, set above.
    q = array_of(z)
  end function roots_column

  ! DURAND_KERNER finds Z, the roots of the monic polynomial M of degree
  ! SIZE(Z) (at least 2), none of them 0: false if the iteration doesn't
  ! settle, or ERR if an operation fails.
  logical function durand_kerner(m, z, err) result(ok)
    complex(rk), intent(in) :: m(:)
    complex(rk), intent(out) :: z(:)
    integer, intent(inout) :: err
    complex(rk) :: num, den, delta, w
    real(rk) :: radius, angle, moved, best
    integer :: i, j, n, sweep, stale

    ok = .false.
    n = size(z)

    ! The circle the guesses start on: Fujiwara's bound, the largest of
    ! |m(k+1)|^(1/k), with the last term halved, doubled. A power 1/k of
    ! a magnitude is EXP(LOG(magnitude) / k), which can't overflow.
    radius = 0
    terms: do j = 1, n
      w = m(j + 1)
      if (j == n) w = w / 2
      if (w == (0.0_rk, 0.0_rk)) cycle terms
      radius = max(radius, exp(log(c_abs(w)) / j))
    end do terms
    radius = 2 * radius

    ! The guesses: N points around the circle, the first at an angle of
    ! 0.4 radians. (Complex arithmetic on parts no larger than RADIUS,
    ! guarded, since a part of COS and SIN may be tiny.)
    do i = 1, n
      angle = 2 * pi * (i - 1) / n + 0.4_rk
      z(i) = c_mul(cmplx(radius, 0.0_rk, rk), cmplx(cos(angle), sin(angle), rk), err)
    end do

    best = huge(1.0_rk)
    stale = 0
    do sweep = 1, sweep_limit
      ! MOVED is the largest step a guess took, as a part of its size.
      moved = 0
      do i = 1, n
        num = horner(m, z(i), err)
        den = (1.0_rk, 0.0_rk)
        do j = 1, n
          if (j /= i) den = c_mul(den, c_sub(z(i), z(j), err), err)
        end do
        if (err /= 0) return
        ! Two guesses the same can't be told apart: leave this one.
        if (den == (0.0_rk, 0.0_rk)) cycle
        delta = c_div(num, den, err)
        z(i) = c_sub(z(i), delta, err)
        if (err /= 0) return
        if (z(i) /= (0.0_rk, 0.0_rk)) then
          moved = max(moved, r_div(c_abs(delta), c_abs(z(i)), err))
        else
          moved = max(moved, c_abs(delta))
        end if
      end do
      if (err /= 0) return

      if (moved <= 4 * eps) then
        ok = .true.
        exit
      end if

      ! Near a multiple root the steps stop shrinking long before they
      ! reach 4 EPS: rounding moves such a root back and forth. When the
      ! best step hasn't improved for STALE_LIMIT sweeps, the roots are as
      ! good as they will get, if the steps are small at all.
      if (moved < best) then
        best = moved
        stale = 0
      else
        stale = stale + 1
        if (stale >= stale_limit) then
          ok = best <= 1.0e-3_rk
          exit
        end if
      end if
    end do
    if (.not. ok) return

    do i = 1, n
      z(i) = polished(m, z(i))
    end do
  end function durand_kerner

  ! POLISHED is Z after up to two of Newton's steps on the polynomial M,
  ! each kept only if it makes |M(Z)| smaller. A step that fails (one too
  ! large to take) leaves Z as it is: the root is already as good as the
  ! iteration made it.
  complex(rk) function polished(m, z)
    complex(rk), intent(in) :: m(:), z
    complex(rk) :: p, dp, w, step
    integer :: round, k, err

    polished = z
    err = 0
    do round = 1, 2
      ! P and DP, M(Z) and its derivative, by Horner's rule together: the
      ! derivative's sum gathers each partial sum of the value's.
      p = m(1)
      dp = (0.0_rk, 0.0_rk)
      do k = 2, size(m)
        dp = c_add(c_mul(dp, polished, err), p, err)
        p = c_add(c_mul(p, polished, err), m(k), err)
      end do
      if (err /= 0 .or. dp == (0.0_rk, 0.0_rk) .or. p == (0.0_rk, 0.0_rk)) return
      step = c_div(p, dp, err)
      w = c_sub(polished, step, err)
      if (err /= 0) return
      if (c_abs(horner(m, w, err)) >= c_abs(p) .or. err /= 0) return
      polished = w
    end do
  end function polished

  ! HORNER is the polynomial C's value at Z, by Horner's rule: for c1 z^2 +
  ! c2 z + c3, ((c1) z + c2) z + c3, a multiplication and an addition for
  ! each coefficient, and no power computed.
  complex(rk) function horner(c, z, err)
    complex(rk), intent(in) :: c(:), z
    integer, intent(inout) :: err
    integer :: k

    horner = c(1)
    do k = 2, size(c)
      horner = c_add(c_mul(horner, z, err), c(k), err)
    end do
  end function horner

  ! ------------------------------------------------------------------
  ! From roots, and values.

  ! POLYNOMIAL_OF is poly(r): the coefficients of (x - r1)(x - r2) ...,
  ! a row, multiplied out one factor at a time. Multiplying c(1) x^k + ...
  ! by (x - r) shifts each coefficient one place along and takes r times
  ! its neighbor from it, which the loop does from the right, so each
  ! neighbor is still the old one when it is used.
  function polynomial_of(r) result(q)
    type(quantity), intent(in) :: r
    type(quantity) :: q
    complex(rk), pointer :: roots(:)
    complex(rk), allocatable :: c(:, :)
    integer :: n, j, k, err

    if (.not. coefficients(r, roots, n, q)) return
    allocate (c(1, n + 1))
    c = (0.0_rk, 0.0_rk)
    c(1, 1) = (1.0_rk, 0.0_rk)
    err = 0
    do k = 1, n
      do j = k + 1, 2, -1
        c(1, j) = c_sub(c(1, j), c_mul(roots(k), c(1, j - 1), err), err)
      end do
    end do
    deallocate (roots)
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(c)
    end if
    deallocate (c)
  end function polynomial_of

  ! POLYNOMIAL_VALUE is polyval(p, x): P's value at X, or at each element
  ! of X, an array of X's shape.
  function polynomial_value(p, x) result(q)
    type(quantity), intent(in) :: p, x
    type(quantity) :: q
    complex(rk), pointer :: c(:)
    complex(rk) :: z(x%rows, x%cols)
    integer :: n, i, j, err

    if (.not. coefficients(p, c, n, q)) return
    z = elements_of(x)
    err = 0
    do j = 1, x%cols
      do i = 1, x%rows
        z(i, j) = horner(c, z(i, j), err)
      end do
    end do
    deallocate (c)
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(z)
    end if
  end function polynomial_value
end module fcpoly
