! fcalc: matrices, their products and solutions, and the matrix functions.
!
! The matrix product, the linear solve (A \ B), the inverse and the
! determinant, a matrix's powers, rank, norms, and eigenvalues, and the
! functions that make and reshape arrays (eye, zeros, linspace, reshape,
! diag, ...), and the polynomials' roots, poly, and polyval (module FCPOLY
! computes them), as a table like FCFUNCS': a row for each function, and a
! CASE in APPLY_MATRIX.
!
! Nothing here may trap, as nothing in FCVALUES may (module FCKINDS says
! why). Three ways of keeping to that are used, each where it fits:
!
! - Guarded arithmetic. The LU factorization and everything built on it
!   (solve, inverse, determinant, rank) does each operation on elements by
!   FCVALUES' C_ADD, C_MUL, and C_DIV, which check before they compute.
!   Slower than plain arithmetic, but fcalc's matrices are small.
!
! - A safe band. MATMUL and DOT_PRODUCT, Fortran 90's own matrix product
!   and dot product, can't be guarded inside; but if every element's parts
!   are zero or between 1E-100 and 1E100, no product of two is outside
!   1E-200 to 1E200, and no sum of fewer than 10,000 of them can overflow.
!   So the intrinsics are used when the operands are in that band
!   (IN_BAND), and a guarded loop otherwise.
!
! - Scaling. The eigenvalue iteration divides the matrix by its largest
!   element first, so every element is at most 1 and a unitary similarity
!   keeps them so; values that would only underflow are set to zero as
!   they appear (FLUSH_SPECKS), being far below anything the eigenvalues'
!   digits can show. The iteration then runs in plain complex arithmetic,
!   and the eigenvalues are scaled back.
!
! Fortran 90's array language is used throughout: whole-array expressions
! (W = Z, X = X - M), sections (H(K+1:N, K)), sections with a stride (a
! diagonal is every (M+1)th element of the array laid out flat), vector
! subscripts (a permutation), and the intrinsics that work on arrays:
! MATMUL, DOT_PRODUCT, TRANSPOSE, RESHAPE, CSHIFT, MAXLOC, SUM, CONJG.
module fcmatrix
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  use fcfuncs, only: builtin
  use fcpoly
  implicit none
  private

  public :: quantity
  public :: matrix_index, matrix_arity_ok, apply_matrix
  public :: matrix_product, solve, right_divide, matrix_power, transposed

  type(builtin), parameter :: functions(26) = (/ &
    builtin('size', 1, 2), builtin('rows', 1, 1), builtin('cols', 1, 1), &
    builtin('numel', 1, 1), builtin('transpose', 1, 1), builtin('trace', 1, 1), &
    builtin('det', 1, 1), builtin('inv', 1, 1), builtin('rank', 1, 1), &
    builtin('norm', 1, 2), builtin('dot', 2, 2), builtin('cross', 2, 2), &
    builtin('kron', 2, 2), builtin('reshape', 3, 3), builtin('diag', 1, 1), &
    builtin('eye', 1, 1), builtin('zeros', 1, 2), builtin('ones', 1, 2), &
    builtin('linspace', 3, 3), builtin('range', 2, 3), builtin('cumsum', 1, 1), &
    builtin('sort', 1, 1), builtin('eig', 1, 1), builtin('roots', 1, 1), &
    builtin('poly', 1, 1), builtin('polyval', 2, 2) /)

  ! The safe band for MATMUL and DOT_PRODUCT.
  real(rk), parameter :: band_low = 1.0e-100_rk, band_high = 1.0e100_rk

  ! In the eigenvalue iteration, on a matrix scaled to at most 1: a part
  ! below SPECK is set to zero, and a subdiagonal element below
  ! DEFLATE_LEVEL is taken as zero, splitting the matrix.
  real(rk), parameter :: speck = 1.0e-50_rk, deflate_level = 1.0e-30_rk

  ! How many QR steps an eigenvalue may take to converge.
  integer, parameter :: step_limit = 30

contains

  ! ------------------------------------------------------------------
  ! The table.

  integer function matrix_index(name)
    character(len=*), intent(in) :: name
    integer :: k

    matrix_index = 0
    do k = 1, size(functions)
      if (functions(k)%name == name) then
        matrix_index = k
        return
      end if
    end do
  end function matrix_index

  logical function matrix_arity_ok(id, n)
    integer, intent(in) :: id, n

    matrix_arity_ok = n >= functions(id)%least .and. n <= functions(id)%most
  end function matrix_arity_ok

  ! APPLY_MATRIX is the function ID of ARGS, none with an error. ARGS stay
  ! the caller's; the result is new.
  function apply_matrix(id, args) result(q)
    integer, intent(in) :: id
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    type(quantity) :: roots
    integer :: k

    select case (functions(id)%name)
    case ('size')
      if (size(args) == 1) then
        ! RESHAPE turns the list (/ rows, cols /) into a 1 by 2 array, a row.
        q = array_of(reshape((/ cmplx(real(args(1)%rows, rk), 0.0_rk, rk), &
          cmplx(real(args(1)%cols, rk), 0.0_rk, rk) /), (/ 1, 2 /)))
      else if (count_argument(args(2), k, q)) then
        select case (k)
        case (1)
          q = real_quantity(real(args(1)%rows, rk))
        case (2)
          q = real_quantity(real(args(1)%cols, rk))
        case default
          q = failure(e_domain)
        end select
      end if
    case ('rows')
      q = real_quantity(real(args(1)%rows, rk))
    case ('cols')
      q = real_quantity(real(args(1)%cols, rk))
    case ('numel')
      q = real_quantity(real(numel(args(1)), rk))
    case ('transpose')
      q = transposed(args(1), .false.)
    case ('trace')
      q = trace_of(args(1))
    case ('det')
      q = determinant(args(1))
    case ('inv')
      q = inverse(args(1))
    case ('rank')
      q = rank_of(args(1))
    case ('norm')
      q = norm_of(args)
    case ('dot')
      q = dot_of(args(1), args(2))
    case ('cross')
      q = cross_of(args(1), args(2))
    case ('kron')
      q = kron_of(args(1), args(2))
    case ('reshape')
      q = reshaped(args)
    case ('diag')
      q = diagonal(args(1))
    case ('eye', 'zeros', 'ones')
      q = filled(functions(id)%name, args)
    case ('linspace')
      q = spaced(args)
    case ('range')
      q = range_of(args)
    case ('cumsum')
      q = cumulative(args(1))
    case ('sort')
      q = sorted(args(1))
    case ('roots')
      ! The roots in BEFORE's order, as eig's, so the order in which the
      ! iteration found them doesn't show.
      q = polynomial_roots(args(1))
      if (is_array(q)) then
        roots = sorted(q)
        call release(q)
        q = roots
      end if
    case ('poly')
      q = polynomial_of(args(1))
    case ('polyval')
      q = polynomial_value(args(1), args(2))
    case default
      q = eigenvalues(args(1))
    end select
  end function apply_matrix

  ! ------------------------------------------------------------------
  ! Arguments.

  ! COUNT_ARGUMENT: Q is a whole number from 1 to MAX_ELEMENTS, N; if not,
  ! BAD says why.
  logical function count_argument(q, n, bad)
    type(quantity), intent(in) :: q
    integer, intent(out) :: n
    type(quantity), intent(out) :: bad
    real(rk) :: x

    count_argument = .false.
    n = 0
    bad = real_quantity(0.0_rk)
    if (.not. real_scalar(q, x, bad)) return
    if (aint(x) /= x) then
      bad = failure(e_notwhole)
    else if (x < 1) then
      bad = failure(e_domain)
    else if (x > max_elements) then
      bad = failure(e_toobig)
    else
      n = nint(x)
      count_argument = .true.
    end if
  end function count_argument

  ! REAL_SCALAR: Q is a real scalar, X; if not, BAD says why.
  logical function real_scalar(q, x, bad)
    type(quantity), intent(in) :: q
    real(rk), intent(out) :: x
    type(quantity), intent(inout) :: bad

    real_scalar = .false.
    x = 0
    if (is_array(q)) then
      bad = failure(e_notscalar)
    else if (.not. is_real(q%s)) then
      bad = failure(e_notreal)
    else
      x = real(q%s%z, rk)
      real_scalar = .true.
    end if
  end function real_scalar

  logical function square(q)
    type(quantity), intent(in) :: q

    square = q%rows == q%cols
  end function square

  ! IN_BAND: every part of Z is zero or between BAND_LOW and BAND_HIGH. X
  ! and Y are automatic arrays of Z's shape; ABS, REAL, and AIMAG of an
  ! array are arrays, element by element, and ALL is true if every element
  ! of its logical array argument is.
  logical function in_band(z)
    complex(rk), intent(in) :: z(:, :)
    real(rk) :: x(size(z, 1), size(z, 2)), y(size(z, 1), size(z, 2))

    x = abs(real(z, rk))
    y = abs(aimag(z))
    in_band = all((x == 0 .or. (x >= band_low .and. x <= band_high)) .and. &
      (y == 0 .or. (y >= band_low .and. y <= band_high)))
  end function in_band

  ! IDENTITY is the N by N identity matrix.
  function identity(n) result(z)
    integer, intent(in) :: n
    complex(rk) :: z(n, n)
    integer :: k

    z = (0.0_rk, 0.0_rk)
    do k = 1, n
      z(k, k) = (1.0_rk, 0.0_rk)
    end do
  end function identity

  ! ------------------------------------------------------------------
  ! Products.

  ! TRANSPOSED is Q's transpose, or its conjugate transpose (A', which a
  ! complex matrix usually wants: it is the adjoint).
  function transposed(a, conjugate) result(q)
    type(quantity), intent(in) :: a
    logical, intent(in) :: conjugate
    type(quantity) :: q

    if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
      if (conjugate) q = scalar_quantity(complex_value(real(a%s%z, rk), -aimag(a%s%z)))
    else if (conjugate) then
      q = array_of(conjg(transpose(a%e)))
    else
      q = array_of(transpose(a%e))
    end if
  end function transposed

  ! MATRIX_PRODUCT is A * B, A's columns as many as B's rows.
  function matrix_product(a, b) result(q)
    type(quantity), intent(in) :: a, b
    type(quantity) :: q

    if (a%cols /= b%rows) then
      q = failure(e_notconform)
    else if (in_band(elements_of(a)) .and. in_band(elements_of(b))) then
      q = array_of(matmul(elements_of(a), elements_of(b)))
    else
      q = guarded_product(elements_of(a), elements_of(b))
    end if
  end function matrix_product

  ! GUARDED_PRODUCT is X * Y, each element a sum of guarded products.
  function guarded_product(x, y) result(q)
    complex(rk), intent(in) :: x(:, :), y(:, :)
    type(quantity) :: q
    complex(rk) :: z(size(x, 1), size(y, 2))
    integer :: i, j, k, err

    err = 0
    do j = 1, size(y, 2)
      do i = 1, size(x, 1)
        z(i, j) = (0.0_rk, 0.0_rk)
        do k = 1, size(x, 2)
          z(i, j) = c_add(z(i, j), c_mul(x(i, k), y(k, j), err), err)
        end do
      end do
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(z)
    end if
  end function guarded_product

  ! DOT_OF is dot(u, v), the sum of conj(u(k)) * v(k), of any two arrays of
  ! as many elements: DOT_PRODUCT conjugates its first argument, as the
  ! inner product of complex vectors must, so dot(v, v) is |v|**2.
  function dot_of(u, v) result(q)
    type(quantity), intent(in) :: u, v
    type(quantity) :: q
    complex(rk) :: x(u%rows * u%cols), y(v%rows * v%cols), s
    integer :: k, err

    if (numel(u) /= numel(v)) then
      q = failure(e_notconform)
      return
    end if
    x = reshape(elements_of(u), (/ numel(u) /))
    y = reshape(elements_of(v), (/ numel(v) /))
    if (in_band(elements_of(u)) .and. in_band(elements_of(v))) then
      q = scalar_quantity(clean(dot_product(x, y)))
      return
    end if
    err = 0
    s = (0.0_rk, 0.0_rk)
    do k = 1, size(x)
      s = c_add(s, c_mul(conjg(x(k)), y(k), err), err)
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = scalar_quantity(clean(s))
    end if
  end function dot_of

  ! CROSS_OF is the cross product of two vectors of three elements,
  !     u x v = (u2 v3 - u3 v2, u3 v1 - u1 v3, u1 v2 - u2 v1)
  ! shaped as U. CSHIFT(x, k) shifts an array's elements k places to the
  ! left, the first ones coming round to the end: CSHIFT(u, 1) is
  ! (u2, u3, u1), so the product is CSHIFT(u, 1) CSHIFT(v, 2) - CSHIFT(u, 2)
  ! CSHIFT(v, 1), element by element.
  function cross_of(u, v) result(q)
    type(quantity), intent(in) :: u, v
    type(quantity) :: q
    complex(rk) :: x(3), y(3), a(3), b(3), c(3), d(3), r(3)
    integer :: k, err

    if (numel(u) /= 3 .or. numel(v) /= 3 .or. .not. (is_vector(u) .and. is_vector(v))) then
      q = failure(e_notvector)
      return
    end if
    x = reshape(u%e, (/ 3 /))
    y = reshape(v%e, (/ 3 /))
    a = cshift(x, 1)
    b = cshift(y, 2)
    c = cshift(x, 2)
    d = cshift(y, 1)
    err = 0
    do k = 1, 3
      r(k) = c_sub(c_mul(a(k), b(k), err), c_mul(c(k), d(k), err), err)
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(reshape(r, (/ u%rows, u%cols /)))
    end if
  end function cross_of

  ! KRON_OF is the Kronecker product: A's every element times the whole of
  ! B, laid out as A's elements are.
  function kron_of(a, b) result(q)
    type(quantity), intent(in) :: a, b
    type(quantity) :: q

    if (numel(a) > max_elements / numel(b)) then
      q = failure(e_toobig)
    else
      q = kron_elements(elements_of(a), elements_of(b))
    end if
  end function kron_of

  function kron_elements(x, y) result(q)
    complex(rk), intent(in) :: x(:, :), y(:, :)
    type(quantity) :: q
    complex(rk) :: z(size(x, 1) * size(y, 1), size(x, 2) * size(y, 2))
    integer :: i, j, k, l, my, ny, err

    my = size(y, 1)
    ny = size(y, 2)
    err = 0
    do j = 1, size(x, 2)
      do i = 1, size(x, 1)
        do l = 1, ny
          do k = 1, my
            z((i - 1) * my + k, (j - 1) * ny + l) = c_mul(x(i, j), y(k, l), err)
          end do
        end do
      end do
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(z)
    end if
  end function kron_elements

  ! TRACE_OF is the sum of a square matrix's diagonal.
  function trace_of(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q
    complex(rk) :: s
    integer :: k, err

    if (.not. square(a)) then
      q = failure(e_notsquare)
      return
    end if
    if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
      return
    end if
    err = 0
    s = (0.0_rk, 0.0_rk)
    do k = 1, a%rows
      s = c_add(s, a%e(k, k), err)
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = scalar_quantity(clean(s))
    end if
  end function trace_of

  ! ------------------------------------------------------------------
  ! The LU factorization, and what is built on it.

  ! LU_FACTOR factors the N by N matrix W in place as P W = L U, by
  ! Gaussian elimination with partial pivoting: L is unit lower triangular
  ! (its multipliers are kept below W's diagonal), U upper triangular (on
  ! and above it), and P the row interchanges, PIV(k) the row swapped with
  ! row k at step k. SIGN is the determinant of P, +1 or -1.
  !
  ! At each step the pivot is the column's largest element below the
  ! diagonal (MAXLOC finds where it is), so no multiplier is larger than 1
  ! and the elimination is stable in practice. A pivot no larger than N EPS
  ! times the matrix's largest element is taken as zero: the matrix is
  ! SINGULAR, to within what its digits can tell, and the factorization
  ! stops there. [1, 2, 3; 4, 5, 6; 7, 8, 9] has a pivot of about 1E-16 at
  ! its last step, rounding's noise, and is singular.
  !
  ! W is an explicit-shape array, W(N, N), the FORTRAN 77 way: the caller
  ! says how large it is. The other arrays here are assumed-shape, the
  ! Fortran 90 way.
  subroutine lu_factor(w, n, piv, sign, singular, err)
    integer, intent(in) :: n
    complex(rk), intent(inout) :: w(n, n)
    integer, intent(out) :: piv(n)
    real(rk), intent(out) :: sign
    logical, intent(out) :: singular
    integer, intent(inout) :: err
    real(rk) :: mag(n), big, tol
    complex(rk) :: row(n), l
    integer :: i, j, k, p, at(1)

    sign = 1
    singular = .false.
    piv = (/ (k, k = 1, n) /)

    big = 0
    do j = 1, n
      do i = 1, n
        big = max(big, c_abs(w(i, j)))
      end do
    end do
    tol = r_mul(big, n * eps, err)

    do k = 1, n
      do i = k, n
        mag(i) = c_abs(w(i, k))
      end do
      ! MAXLOC returns an array, a subscript for each dimension of its
      ! argument (here one), counted from the section's start.
      at = maxloc(mag(k:n))
      p = at(1) + k - 1
      if (mag(p) <= tol) then
        singular = .true.
        return
      end if
      piv(k) = p
      if (p /= k) then
        row = w(k, :)
        w(k, :) = w(p, :)
        w(p, :) = row
        sign = -sign
      end if
      do i = k + 1, n
        l = c_div(w(i, k), w(k, k), err)
        w(i, k) = l
        if (l == (0.0_rk, 0.0_rk)) cycle
        do j = k + 1, n
          w(i, j) = c_sub(w(i, j), c_mul(l, w(k, j), err), err)
        end do
      end do
    end do
  end subroutine lu_factor

  ! DETERMINANT is det(A): U's diagonal multiplied, times P's sign; 0 for a
  ! singular matrix.
  function determinant(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. square(a)) then
      q = failure(e_notsquare)
    else if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
    else
      q = lu_determinant(a%e, a%rows)
    end if
  end function determinant

  function lu_determinant(z, n) result(q)
    integer, intent(in) :: n
    complex(rk), intent(in) :: z(n, n)
    type(quantity) :: q
    complex(rk) :: w(n, n), d
    integer :: piv(n), k, err
    real(rk) :: sign
    logical :: singular

    w = z
    err = 0
    call lu_factor(w, n, piv, sign, singular, err)
    if (singular) then
      q = real_quantity(0.0_rk)
      return
    end if
    d = cmplx(sign, 0.0_rk, rk)
    do k = 1, n
      d = c_mul(d, w(k, k), err)
    end do
    if (err /= 0) then
      q = failure(err)
    else
      q = scalar_quantity(clean(d))
    end if
  end function lu_determinant

  ! SOLVE is A \ B, the X with A X = B, for a square A and a B of as many
  ! rows (each column of B a right-hand side).
  function solve(a, b) result(q)
    type(quantity), intent(in) :: a, b
    type(quantity) :: q

    if (.not. square(a)) then
      q = failure(e_notsquare)
    else if (a%rows /= b%rows) then
      q = failure(e_notconform)
    else
      q = lu_solve(elements_of(a), elements_of(b))
    end if
  end function solve

  ! LU_SOLVE solves Z X = Y: Z is factored, Y's rows are interchanged as
  ! Z's were, then L C = P Y is solved for C going down (forward
  ! substitution), and U X = C for X going up (back substitution).
  function lu_solve(z, y) result(q)
    complex(rk), intent(in) :: z(:, :), y(:, :)
    type(quantity) :: q
    complex(rk) :: w(size(z, 1), size(z, 1)), x(size(y, 1), size(y, 2))
    complex(rk) :: row(size(y, 2))
    integer :: piv(size(z, 1)), n, m, i, j, k, err
    real(rk) :: sign
    logical :: singular

    n = size(z, 1)
    m = size(y, 2)
    w = z
    x = y
    err = 0
    call lu_factor(w, n, piv, sign, singular, err)
    if (singular) then
      q = failure(e_singular)
      return
    end if

    do k = 1, n
      if (piv(k) /= k) then
        row = x(k, :)
        x(k, :) = x(piv(k), :)
        x(piv(k), :) = row
      end if
    end do

    do k = 1, n
      do i = k + 1, n
        if (w(i, k) == (0.0_rk, 0.0_rk)) cycle
        do j = 1, m
          x(i, j) = c_sub(x(i, j), c_mul(w(i, k), x(k, j), err), err)
        end do
      end do
    end do

    do k = n, 1, -1
      do j = 1, m
        x(k, j) = c_div(x(k, j), w(k, k), err)
      end do
      do i = 1, k - 1
        if (w(i, k) == (0.0_rk, 0.0_rk)) cycle
        do j = 1, m
          x(i, j) = c_sub(x(i, j), c_mul(w(i, k), x(k, j), err), err)
        end do
      end do
    end do

    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(x)
    end if
  end function lu_solve

  ! INVERSE is inv(A): A \ I.
  function inverse(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. square(a)) then
      q = failure(e_notsquare)
    else if (.not. is_array(a)) then
      q = scalar_quantity(real_value(1.0_rk) / a%s)
    else
      q = lu_solve(a%e, identity(a%rows))
    end if
  end function inverse

  ! RIGHT_DIVIDE is A / B, the X with X B = A: transposed, B.' X.' = A.',
  ! a solve.
  function right_divide(a, b) result(q)
    type(quantity), intent(in) :: a, b
    type(quantity) :: q
    type(quantity) :: t

    if (.not. square(b)) then
      q = failure(e_notsquare)
    else if (a%cols /= b%cols) then
      q = failure(e_notconform)
    else
      t = lu_solve(transpose(elements_of(b)), transpose(elements_of(a)))
      q = transposed(t, .false.)
      call release(t)
    end if
  end function right_divide

  ! MATRIX_POWER is A ^ B, for a square A and a whole B: by squaring, as
  ! FCVALUES' V_POWER computes a number's power, and through the inverse
  ! for a negative B. A ^ 0 is the identity.
  function matrix_power(a, b) result(q)
    type(quantity), intent(in) :: a, b
    type(quantity) :: q
    type(quantity) :: base, t
    real(rk) :: x
    integer :: k
    logical :: started

    if (.not. square(a)) then
      q = failure(e_notsquare)
      return
    end if
    if (.not. real_scalar(b, x, q)) return
    if (aint(x) /= x) then
      q = failure(e_notwhole)
      return
    end if
    if (abs(x) > 1.0e9_rk) then
      q = failure(e_overflow)
      return
    end if
    k = nint(x)
    if (k == 0) then
      q = array_of(identity(a%rows))
      return
    end if
    if (k < 0) then
      base = inverse(a)
      if (quantity_error(base) /= 0) then
        q = base
        return
      end if
      k = -k
    else
      base = duplicate(a)
    end if

    started = .false.
    do
      if (mod(k, 2) == 1) then
        if (started) then
          t = matrix_product(q, base)
          call release(q)
          q = t
        else
          q = duplicate(base)
          started = .true.
        end if
        if (quantity_error(q) /= 0) exit
      end if
      k = k / 2
      if (k == 0) exit
      t = matrix_product(base, base)
      call release(base)
      base = t
      if (quantity_error(base) /= 0) then
        if (started) call release(q)
        q = base
        return
      end if
    end do
    call release(base)
  end function matrix_power

  ! ------------------------------------------------------------------
  ! Rank and norms.

  ! RANK_OF is the number of A's rows (or columns) that are independent, to
  ! within its digits: Gaussian elimination with complete pivoting, each
  ! step's pivot the largest element left anywhere, counting the pivots
  ! larger than max(m, n) EPS times A's largest element.
  function rank_of(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. is_array(a)) then
      q = logical_quantity(truth(a%s))
    else
      q = real_quantity(real(rank_count(a%e), rk))
    end if
  end function rank_of

  function logical_quantity(b) result(q)
    logical, intent(in) :: b
    type(quantity) :: q

    q = scalar_quantity(logical_value(b))
  end function logical_quantity

  integer function rank_count(z)
    complex(rk), intent(in) :: z(:, :)
    complex(rk) :: w(size(z, 1), size(z, 2)), row(size(z, 2)), col(size(z, 1)), l
    real(rk) :: mag(size(z, 1), size(z, 2)), tol
    integer :: m, n, i, j, k, p, c, at(2), err

    m = size(z, 1)
    n = size(z, 2)
    w = z
    err = 0
    do j = 1, n
      do i = 1, m
        mag(i, j) = c_abs(w(i, j))
      end do
    end do
    tol = r_mul(maxval(mag), max(m, n) * eps, err)

    rank_count = 0
    do k = 1, min(m, n)
      do j = k, n
        do i = k, m
          mag(i, j) = c_abs(w(i, j))
        end do
      end do
      at = maxloc(mag(k:m, k:n))
      p = at(1) + k - 1
      c = at(2) + k - 1
      if (mag(p, c) <= tol) exit
      rank_count = rank_count + 1
      row = w(k, :)
      w(k, :) = w(p, :)
      w(p, :) = row
      col = w(:, k)
      w(:, k) = w(:, c)
      w(:, c) = col
      do i = k + 1, m
        l = c_div(w(i, k), w(k, k), err)
        do j = k + 1, n
          w(i, j) = c_sub(w(i, j), c_mul(l, w(k, j), err), err)
        end do
      end do
    end do
  end function rank_count

  ! NORM_OF is norm(v [, p]): a vector's p-norm, (sum |v(k)|**p)**(1/p), 2
  ! if P isn't given, its largest magnitude if P is HUGE; a matrix's norm
  ! is its Frobenius norm (the 2-norm of its elements) without P, its
  ! largest column sum of magnitudes with P = 1, and its largest row sum
  ! with P = HUGE.
  function norm_of(args) result(q)
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    real(rk) :: p

    p = 2
    if (size(args) == 2) then
      if (.not. real_scalar(args(2), p, q)) return
      if (p < 1) then
        q = failure(e_domain)
        return
      end if
    end if

    if (is_array(args(1)) .and. .not. is_vector(args(1)) .and. size(args) == 2) then
      if (p == 1) then
        q = line_sums(args(1), .true.)
      else if (p >= limit) then
        q = line_sums(args(1), .false.)
      else
        q = failure(e_domain)
      end if
    else
      q = p_norm(reshape(elements_of(args(1)), (/ numel(args(1)) /)), p)
    end if
  end function norm_of

  ! P_NORM is Z's p-norm, every magnitude first divided by the largest, S,
  ! so no power of one can overflow: S (sum (|z(k)| / S)**p)**(1/p). The
  ! powers are FCVALUES' guarded ones, which make a power too small to
  ! matter zero.
  function p_norm(z, p) result(q)
    complex(rk), intent(in) :: z(:)
    real(rk), intent(in) :: p
    type(quantity) :: q
    type(value) :: total, term, ratio
    real(rk) :: s
    integer :: k, err

    s = 0
    do k = 1, size(z)
      s = max(s, c_abs(z(k)))
    end do
    if (s == 0 .or. p >= limit) then
      q = real_quantity(s)
      return
    end if

    total = real_value(0.0_rk)
    do k = 1, size(z)
      err = 0
      ratio = real_value(r_div(c_abs(z(k)), s, err))
      if (p == 1) then
        term = ratio
      else if (p == 2) then
        term = ratio * ratio
      else
        term = ratio ** real_value(p)
      end if
      total = total + term
    end do
    if (p == 1) then
      q = scalar_quantity(total * real_value(s))
    else if (p == 2) then
      q = scalar_quantity(real_value(sqrt(real(total%z, rk))) * real_value(s))
    else
      q = scalar_quantity(total ** real_value(1 / p) * real_value(s))
    end if
  end function p_norm

  ! LINE_SUMS is the largest of A's column sums of magnitudes (COLUMNS
  ! true), or of its row sums.
  function line_sums(a, columns) result(q)
    type(quantity), intent(in) :: a
    logical, intent(in) :: columns
    type(quantity) :: q
    real(rk) :: s, best
    integer :: i, j, err

    err = 0
    best = 0
    if (columns) then
      do j = 1, a%cols
        s = 0
        do i = 1, a%rows
          s = r_add(s, c_abs(a%e(i, j)), err)
        end do
        best = max(best, s)
      end do
    else
      do i = 1, a%rows
        s = 0
        do j = 1, a%cols
          s = r_add(s, c_abs(a%e(i, j)), err)
        end do
        best = max(best, s)
      end do
    end if
    if (err /= 0) then
      q = failure(err)
    else
      q = real_quantity(best)
    end if
  end function line_sums

  ! ------------------------------------------------------------------
  ! Making and rearranging arrays.

  ! RESHAPED is reshape(a, m, n): A's elements, in array element order,
  ! as M rows and N columns.
  function reshaped(args) result(q)
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    integer :: m, n

    if (.not. count_argument(args(2), m, q)) return
    if (.not. count_argument(args(3), n, q)) return
    if (m > max_elements / n) then
      q = failure(e_toobig)
    else if (m * n /= numel(args(1))) then
      q = failure(e_notconform)
    else
      q = array_of(reshape(elements_of(args(1)), (/ m, n /)))
    end if
  end function reshaped

  ! DIAGONAL is diag(a): a vector's elements on a square matrix's
  ! diagonal, or a matrix's diagonal as a column.
  function diagonal(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q
    integer :: n

    if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
    else if (is_vector(a)) then
      n = numel(a)
      if (n > max_elements / n) then
        q = failure(e_toobig)
      else
        q = diagonal_matrix(reshape(a%e, (/ n /)))
      end if
    else
      q = diagonal_of(a%e)
    end if
  end function diagonal

  function diagonal_matrix(v) result(q)
    complex(rk), intent(in) :: v(:)
    type(quantity) :: q
    complex(rk) :: z(size(v), size(v))
    integer :: k

    z = (0.0_rk, 0.0_rk)
    do k = 1, size(v)
      z(k, k) = v(k)
    end do
    q = array_of(z)
  end function diagonal_matrix

  ! DIAGONAL_OF is Z's diagonal. Laid out flat, in array element order, an
  ! M-row matrix's element (k, k) is element 1 + (k - 1)(M + 1): the
  ! diagonal is every (M + 1)th element from the first, a section with a
  ! stride, FLAT(1 : last : M + 1).
  function diagonal_of(z) result(q)
    complex(rk), intent(in) :: z(:, :)
    type(quantity) :: q
    complex(rk) :: flat(size(z)), d(min(size(z, 1), size(z, 2)))
    integer :: m, k

    m = size(z, 1)
    k = size(d)
    flat = reshape(z, (/ size(z) /))
    d = flat(1:1 + (k - 1) * (m + 1):m + 1)
    q = array_of(reshape(d, (/ k, 1 /)))
  end function diagonal_of

  ! FILLED is eye(n), zeros(m [, n]), or ones(m [, n]).
  function filled(name, args) result(q)
    character(len=*), intent(in) :: name
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    integer :: m, n

    if (.not. count_argument(args(1), m, q)) return
    n = m
    if (size(args) == 2) then
      if (.not. count_argument(args(2), n, q)) return
    end if
    if (m > max_elements / n) then
      q = failure(e_toobig)
      return
    end if
    select case (name)
    case ('eye')
      q = array_of(identity(m))
    case ('zeros')
      q = constant_array(m, n, (0.0_rk, 0.0_rk))
    case default
      q = constant_array(m, n, (1.0_rk, 0.0_rk))
    end select
  end function filled

  function constant_array(m, n, c) result(q)
    integer, intent(in) :: m, n
    complex(rk), intent(in) :: c
    type(quantity) :: q
    complex(rk) :: z(m, n)

    z = c
    q = array_of(z)
  end function constant_array

  ! SPACED is linspace(a, b, n): N values from A to B, evenly spaced, in a
  ! row; the last is B itself, not A plus (N - 1) steps, which rounding
  ! could leave a bit away from it.
  function spaced(args) result(q)
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    integer :: n

    if (is_array(args(1)) .or. is_array(args(2))) then
      q = failure(e_notscalar)
    else if (count_argument(args(3), n, q)) then
      q = spaced_values(args(1)%s, args(2)%s, n)
    end if
  end function spaced

  function spaced_values(a, b, n) result(q)
    type(value), intent(in) :: a, b
    integer, intent(in) :: n
    type(quantity) :: q
    complex(rk) :: z(1, n)
    type(value) :: step, v
    integer :: k

    if (n == 1) then
      q = scalar_quantity(b)
      return
    end if
    step = (b - a) / real_value(real(n - 1, rk))
    do k = 1, n - 1
      v = a + step * real_value(real(k - 1, rk))
      if (v%err /= 0) then
        q = failure(v%err)
        return
      end if
      z(1, k) = v%z
    end do
    z(1, n) = b%z
    q = array_of(z)
  end function spaced_values

  ! RANGE_OF is range(a, b [, s]): A, A + S, A + 2S, ... as far as B, in a
  ! row; S is 1 if it isn't given. The count is (B - A) / S + 1, less any
  ! fraction, with a little allowance for rounding, so range(0, 1, 0.1)
  ! ends at 1 although 1 / 0.1 might come out a bit under 10.
  function range_of(args) result(q)
    type(quantity), intent(in) :: args(:)
    type(quantity) :: q
    real(rk) :: a, b, s, t
    integer :: n, err

    if (.not. real_scalar(args(1), a, q)) return
    if (.not. real_scalar(args(2), b, q)) return
    s = 1
    if (size(args) == 3) then
      if (.not. real_scalar(args(3), s, q)) return
    end if
    err = 0
    if (s == 0) then
      q = failure(e_domain)
      return
    end if
    t = r_div(r_add(b, -a, err), s, err)
    if (err /= 0) then
      q = failure(err)
      return
    end if
    if (t < -1.0e-9_rk) then
      q = failure(e_domain)
      return
    end if
    t = t + abs(t) * 1.0e-9_rk + 1.0e-9_rk
    if (t >= max_elements) then
      q = failure(e_toobig)
      return
    end if
    n = int(t) + 1
    q = range_values(a, s, n)
  end function range_of

  function range_values(a, s, n) result(q)
    real(rk), intent(in) :: a, s
    integer, intent(in) :: n
    type(quantity) :: q
    complex(rk) :: z(1, n)
    type(value) :: v
    integer :: k

    do k = 1, n
      v = real_value(a) + real_value(s) * real_value(real(k - 1, rk))
      if (v%err /= 0) then
        q = failure(v%err)
        return
      end if
      z(1, k) = v%z
    end do
    q = array_of(z)
  end function range_values

  ! CUMULATIVE is cumsum(a): a vector's running sums, along it; a
  ! matrix's down each column.
  function cumulative(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
    else
      q = running_sums(a%e)
    end if
  end function cumulative

  function running_sums(z) result(q)
    complex(rk), intent(in) :: z(:, :)
    type(quantity) :: q
    complex(rk) :: w(size(z, 1), size(z, 2))
    integer :: i, j, err

    w = z
    err = 0
    if (size(z, 1) == 1) then
      do j = 2, size(z, 2)
        w(1, j) = c_add(w(1, j - 1), w(1, j), err)
      end do
    else
      do j = 1, size(z, 2)
        do i = 2, size(z, 1)
          w(i, j) = c_add(w(i - 1, j), w(i, j), err)
        end do
      end do
    end if
    if (err /= 0) then
      q = failure(err)
    else
      q = array_of(w)
    end if
  end function running_sums

  ! SORTED is sort(a): a vector's elements in order, along it; a matrix's
  ! down each column. The order is by real part, then imaginary part
  ! (BEFORE).
  function sorted(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
    else
      q = sorted_lines(a%e, sameness(largest_magnitude(a)))
    end if
  end function sorted

  function sorted_lines(z, scale) result(q)
    complex(rk), intent(in) :: z(:, :)
    real(rk), intent(in) :: scale
    type(quantity) :: q
    complex(rk) :: w(size(z, 1), size(z, 2))
    integer :: j

    w = z
    if (size(z, 1) == 1) then
      call merge_sort(w(1, :), scale)
    else
      do j = 1, size(z, 2)
        call merge_sort(w(:, j), scale)
      end do
    end if
    q = array_of(w)
  end function sorted_lines

  ! SAMENESS is how far apart two parts may be and count as the same in
  ! sorting: as far as the digits shown can't tell, beside BIG, the
  ! largest magnitude among the values sorted. Computed by R_MUL, so a
  ! tiny BIG gives 0 and never an underflow.
  real(rk) function sameness(big)
    real(rk), intent(in) :: big
    integer :: err

    err = 0
    sameness = r_mul(big, 10.0_rk ** (-digits), err)
  end function sameness

  ! BEFORE: X comes before Y: its real part is less, by more than SCALE,
  ! or the real parts are the same and its imaginary part is less. Real
  ! parts that differ only by rounding count as the same, so a conjugate
  ! pair's member with the negative imaginary part comes first however
  ! their computed real parts differ, on any machine.
  logical function before(x, y, scale)
    complex(rk), intent(in) :: x, y
    real(rk), intent(in) :: scale

    if (abs(real(x, rk) - real(y, rk)) > scale) then
      before = real(x, rk) < real(y, rk)
    else
      before = aimag(y) - aimag(x) > scale
    end if
  end function before

  ! MERGE_SORT sorts X by BEFORE: each half sorted (recursively), then the
  ! two merged, taking the left half's element unless the right's comes
  ! before it, so elements the same keep their order (the sort is stable).
  ! LEFT and RIGHT are automatic arrays, a new pair at each level of the
  ! recursion. An array section such as W(:, J) may be passed to the
  ! assumed-shape X, and the sort changes the section in place.
  recursive subroutine merge_sort(x, scale)
    complex(rk), intent(inout) :: x(:)
    real(rk), intent(in) :: scale
    complex(rk) :: left(size(x) / 2), right(size(x) - size(x) / 2)
    integer :: i, j, k

    if (size(x) < 2) return
    left = x(1:size(left))
    right = x(size(left) + 1:)
    call merge_sort(left, scale)
    call merge_sort(right, scale)
    i = 1
    j = 1
    do k = 1, size(x)
      if (j > size(right)) then
        x(k) = left(i)
        i = i + 1
      else if (i > size(left)) then
        x(k) = right(j)
        j = j + 1
      else if (before(right(j), left(i), scale)) then
        x(k) = right(j)
        j = j + 1
      else
        x(k) = left(i)
        i = i + 1
      end if
    end do
  end subroutine merge_sort

  ! ------------------------------------------------------------------
  ! Eigenvalues.

  ! EIGENVALUES is eig(a): a square matrix's eigenvalues, a column in
  ! BEFORE's order (so the order in which they converged doesn't show).
  !
  ! The method is the one most libraries use: reduce A to upper Hessenberg
  ! form (zero below the first subdiagonal) by a similarity, which keeps
  ! the eigenvalues, then run the shifted QR iteration on it, which makes
  ! the subdiagonal elements tend to zero, the last one fastest; when one is
  ! negligible the matrix splits there, and a 1 by 1 block at the bottom is
  ! an eigenvalue. In complex arithmetic a single shift serves every
  ! matrix, real or complex, its eigenvalues real or not.
  function eigenvalues(a) result(q)
    type(quantity), intent(in) :: a
    type(quantity) :: q

    if (.not. square(a)) then
      q = failure(e_notsquare)
    else if (.not. is_array(a)) then
      q = scalar_quantity(a%s)
    else
      q = eigen_column(a%e, a%rows)
    end if
  end function eigenvalues

  function eigen_column(z, n) result(q)
    integer, intent(in) :: n
    complex(rk), intent(in) :: z(n, n)
    type(quantity) :: q
    complex(rk) :: h(n, n), ev(n, 1)
    real(rk) :: big, cut, x, y
    integer :: i, j, err
    logical :: ok

    big = 0
    do j = 1, n
      do i = 1, n
        big = max(big, c_abs(z(i, j)))
      end do
    end do
    if (big == 0) then
      q = constant_array(n, 1, (0.0_rk, 0.0_rk))
      return
    end if

    ! H is Z divided by BIG, a part smaller than BIG * SPECK being zero:
    ! dividing it would only underflow. (CUT is 0 if BIG * SPECK would be
    ! too small to compute; then no part divided by BIG is that small.)
    err = 0
    cut = r_mul(big, speck, err)
    do j = 1, n
      do i = 1, n
        x = real(z(i, j), rk)
        y = aimag(z(i, j))
        if (abs(x) < cut) then
          x = 0
        else
          x = x / big
        end if
        if (abs(y) < cut) then
          y = 0
        else
          y = y / big
        end if
        h(i, j) = cmplx(x, y, rk)
      end do
    end do

    call hessenberg(h, n)
    call qr_iterate(h, n, ev(:, 1), ok)
    if (.not. ok) then
      q = failure(e_noconverge)
      return
    end if

    do i = 1, n
      ev(i, 1) = c_mul(ev(i, 1), cmplx(big, 0.0_rk, rk), err)
    end do
    if (err /= 0) then
      q = failure(err)
      return
    end if
    call merge_sort(ev(:, 1), sameness(maxval(abs(ev))))
    q = array_of(ev)
  end function eigen_column

  ! FLUSH_SPECKS sets to zero every part of H smaller than SPECK. WHERE
  ! assigns only to the elements where its mask is true.
  subroutine flush_specks(h)
    complex(rk), intent(inout) :: h(:, :)

    where (abs(real(h, rk)) < speck) h = cmplx(0.0_rk, aimag(h), rk)
    where (abs(aimag(h)) < speck) h = cmplx(real(h, rk), 0.0_rk, rk)
  end subroutine flush_specks

  ! HESSENBERG reduces H to upper Hessenberg form by Householder
  ! reflections. At step k, the part of column k below the subdiagonal,
  ! x = H(k+1:n, k), is reflected onto its first element: with
  !     alpha = -(x1 / |x1|) |x|,   v = x - alpha e1,
  ! the reflector P = I - 2 v v* / (v* v) takes x to alpha e1. (Taking alpha
  ! against x1's direction makes v1 = x1 - alpha a sum of two numbers of
  ! the same direction, so nothing cancels.) H becomes P H P, a similarity
  ! (P is its own inverse), which zeroes column k below the subdiagonal.
  !
  ! DOT_PRODUCT(v, c) is v* c, conjugating v; SUM(r * v) is r v without.
  subroutine hessenberg(h, n)
    integer, intent(in) :: n
    complex(rk), intent(inout) :: h(n, n)
    complex(rk) :: v(n), alpha, w
    real(rk) :: xnorm, f
    integer :: i, j, k, m

    call flush_specks(h)
    do k = 1, n - 2
      m = n - k
      xnorm = sqrt(sum(abs(h(k + 1:n, k)) ** 2))
      if (xnorm == 0) cycle
      if (h(k + 1, k) == (0.0_rk, 0.0_rk)) then
        alpha = -xnorm
      else
        alpha = -(h(k + 1, k) / abs(h(k + 1, k))) * xnorm
      end if
      v(1:m) = h(k + 1:n, k)
      v(1) = v(1) - alpha
      f = 2 / sum(abs(v(1:m)) ** 2)

      do j = k, n
        w = f * dot_product(v(1:m), h(k + 1:n, j))
        h(k + 1:n, j) = h(k + 1:n, j) - w * v(1:m)
      end do
      do i = 1, n
        w = f * sum(h(i, k + 1:n) * v(1:m))
        h(i, k + 1:n) = h(i, k + 1:n) - w * conjg(v(1:m))
      end do
      h(k + 1, k) = alpha
      h(k + 2:n, k) = (0.0_rk, 0.0_rk)
      call flush_specks(h)
    end do
  end subroutine hessenberg

  ! QR_ITERATE finds the eigenvalues of the Hessenberg matrix H, EV(1:N),
  ! working from the bottom: HI is the last row not yet done, and the
  ! active block is rows and columns L to HI, L just below the last
  ! negligible subdiagonal element. One QR step on the block, with a shift
  ! MU: H - MU I = Q R, then R Q + MU I, a similarity again. Q is made of
  ! plane rotations, each zeroing one subdiagonal element of H - MU I; R Q
  ! applies them again from the right. The shift is Wilkinson's, the
  ! eigenvalue of the block's last 2 by 2 nearer its last element, with
  ! which the last subdiagonal element shrinks quadratically; at the 10th
  ! and 20th steps without an eigenvalue a different shift breaks any
  ! cycle. OK is false if an eigenvalue takes more than STEP_LIMIT steps.
  !
  ! Only the block is changed: the eigenvalues of a block upper triangular
  ! matrix are its diagonal blocks', and the rest of H doesn't affect them.
  subroutine qr_iterate(h, n, ev, ok)
    integer, intent(in) :: n
    complex(rk), intent(inout) :: h(n, n)
    complex(rk), intent(out) :: ev(n)
    logical, intent(out) :: ok
    complex(rk) :: s(n), mu, t1, t2
    real(rk) :: c(n)
    integer :: hi, l, i, j, k, steps

    ok = .true.
    hi = n
    steps = 0
    do while (hi >= 1)
      l = hi
      do while (l > 1)
        if (abs(h(l, l - 1)) <= eps * (abs(h(l - 1, l - 1)) + abs(h(l, l))) .or. &
            abs(h(l, l - 1)) < deflate_level) then
          h(l, l - 1) = (0.0_rk, 0.0_rk)
          exit
        end if
        l = l - 1
      end do
      if (l == hi) then
        ev(hi) = h(hi, hi)
        hi = hi - 1
        steps = 0
        cycle
      end if

      steps = steps + 1
      if (steps > step_limit) then
        ok = .false.
        return
      end if
      if (steps == 10 .or. steps == 20) then
        mu = h(hi, hi) + 0.75_rk * abs(h(hi, hi - 1))
      else
        mu = wilkinson(h(hi - 1, hi - 1), h(hi - 1, hi), h(hi, hi - 1), h(hi, hi))
      end if

      do k = l, hi
        h(k, k) = h(k, k) - mu
      end do
      do k = l, hi - 1
        call rotation(h(k, k), h(k + 1, k), c(k), s(k))
        do j = k, hi
          t1 = h(k, j)
          t2 = h(k + 1, j)
          h(k, j) = c(k) * t1 + s(k) * t2
          h(k + 1, j) = -conjg(s(k)) * t1 + c(k) * t2
        end do
        call flush_specks(h(k:k + 1, k:hi))
      end do
      do k = l, hi - 1
        do i = l, k + 1
          t1 = h(i, k)
          t2 = h(i, k + 1)
          h(i, k) = c(k) * t1 + conjg(s(k)) * t2
          h(i, k + 1) = -s(k) * t1 + c(k) * t2
        end do
      end do
      do k = l, hi
        h(k, k) = h(k, k) + mu
      end do
      call flush_specks(h(l:hi, l:hi))
    end do
  end subroutine qr_iterate

  ! ROTATION is the plane rotation G = [c, s; -conj(s), c], c real, with
  ! G [a; b] = [r; 0]: c = |a| / r, s = (a / |a|) conj(b) / r, where
  ! r = sqrt(|a|**2 + |b|**2).
  subroutine rotation(a, b, c, s)
    complex(rk), intent(in) :: a, b
    real(rk), intent(out) :: c
    complex(rk), intent(out) :: s
    real(rk) :: aa, r

    if (b == (0.0_rk, 0.0_rk)) then
      c = 1
      s = (0.0_rk, 0.0_rk)
    else if (a == (0.0_rk, 0.0_rk)) then
      c = 0
      s = (1.0_rk, 0.0_rk)
    else
      aa = abs(a)
      r = sqrt(aa ** 2 + abs(b) ** 2)
      c = aa / r
      s = (a / aa) * conjg(b) / r
    end if
  end subroutine rotation

  ! WILKINSON is the eigenvalue of [a, b; c, d] nearer d: with
  ! m = (a + d) / 2 and r = sqrt(((a - d) / 2)**2 + b c), m + r or m - r.
  complex(rk) function wilkinson(a, b, c, d)
    complex(rk), intent(in) :: a, b, c, d
    complex(rk) :: m, r

    m = (a + d) / 2
    r = sqrt(((a - d) / 2) ** 2 + b * c)
    if (abs(m + r - d) <= abs(m - r - d)) then
      wilkinson = m + r
    else
      wilkinson = m - r
    end if
  end function wilkinson
end module fcmatrix
