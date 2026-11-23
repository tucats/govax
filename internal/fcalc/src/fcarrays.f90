! fcalc: quantities, the values the evaluator deals in: a scalar, or a
! vector or matrix of complex numbers.
!
! A QUANTITY is a VALUE (module FCVALUES) when it is a scalar, and an array
! of ROWS by COLS complex elements when it is not; a vector is an array with
! one row or one column. An array has at least two elements: an operation
! whose result would have one gives a scalar instead, so [5] is 5 and a
! row times a column is a number.
!
! The elements live in storage of their own, reached through a POINTER
! component, E. Fortran 90 has no allocatable components (they came in
! Fortran 2003), so a derived type that holds an array whose size is known
! only when the program runs holds a pointer to it, ALLOCATEd when it is
! made. And Fortran 90 has no destructors either: nothing frees that
! storage when a QUANTITY goes out of use, so fcalc keeps a rule of
! ownership, the way a C program does:
!
! - Every function that makes a quantity gives it storage of its own. The
!   caller owns it, and must either RELEASE it or pass it on to an owner.
! - Assignment (A = B) copies the pointer, not the elements: afterwards A
!   and B share storage, and only one of them may be released. fcalc uses
!   it only to move a quantity from one owner to another (as SET_VARIABLE
!   takes the quantity it is given).
! - DUPLICATE makes a copy with storage of its own, when two owners are
!   wanted (FIND_VARIABLE gives the caller a copy of the variable's value).
!
! A defined ASSIGNMENT(=) that copied the elements would seem safer, but it
! would leak: the result of a function, F(X) on A = F(X)'s right, is itself
! a quantity with storage, which a copying assignment would copy and then
! drop. Fortran 95 added nothing here; Fortran 2003's allocatable
! components and FINAL procedures are the cure.
!
! The module counts the arrays it has made and not yet released, so a leak
! shows: SHOW MEMORY reports the count, and the test scripts check it is 0
! once they have cleared their variables.
!
! A quantity's error is its scalar's: an array operation that fails gives a
! scalar quantity with the error set, as a scalar operation does.
module fcarrays
  use fckinds
  use fcmessages
  use fcvalues
  implicit none
  private

  public :: value, quantity
  public :: scalar_quantity, real_quantity, failure, new_array, array_of
  public :: release, duplicate, is_array, is_vector, element, element_at
  public :: numel, same_shape, quantity_error, quantity_truth, elements_of
  public :: arrays_in_use, largest_magnitude, show_quantity, list_quantity
  public :: max_elements

  type quantity
    integer :: rows
    integer :: cols
    type(value) :: s
    complex(rk), pointer :: e(:, :)
  end type quantity

  ! The most elements an array may have.
  integer, parameter :: max_elements = 10000

  ! The arrays made and not yet released.
  integer, save :: in_use = 0

  ! The longest line a matrix is shown in: a line's columns past it go in
  ! a group of their own.
  integer, parameter :: line_width = 132

contains

  ! ------------------------------------------------------------------
  ! Making and releasing quantities.

  ! SCALAR_QUANTITY is the scalar V as a quantity. Its pointer is
  ! nullified, so the quantity is defined in every part: a pointer that is
  ! neither associated nor nullified is undefined, and copying an undefined
  ! pointer is undefined too.
  function scalar_quantity(v) result(q)
    type(value), intent(in) :: v
    type(quantity) :: q

    q%rows = 1
    q%cols = 1
    q%s = v
    nullify (q%e)
  end function scalar_quantity

  function real_quantity(x) result(q)
    real(rk), intent(in) :: x
    type(quantity) :: q

    q = scalar_quantity(real_value(x))
  end function real_quantity

  ! FAILURE is a quantity with the error CODE.
  function failure(code) result(q)
    integer, intent(in) :: code
    type(quantity) :: q

    q = scalar_quantity(error_value(code))
  end function failure

  ! NEW_ARRAY is an M by N array, M * N at least 2, its elements not yet
  ! given values: the caller fills Q%E. TOOBIG if it would be too large.
  function new_array(m, n) result(q)
    integer, intent(in) :: m, n
    type(quantity) :: q

    if (m > max_elements / n) then
      q = failure(e_toobig)
      return
    end if
    allocate (q%e(m, n))
    in_use = in_use + 1
    q%rows = m
    q%cols = n
    q%s = real_value(0.0_rk)
  end function new_array

  ! ARRAY_OF is a quantity of the elements Z: a scalar if Z has one
  ! element, an array of Z's shape if it has more. Each element is made a
  ! value by CLEAN, so an element past LIMIT makes the whole an overflow,
  ! and one below SMALL is zero.
  !
  ! Z(:, :) is an assumed-shape array: SIZE(Z, 1) is its rows, SIZE(Z, 2)
  ! its columns. A caller may pass any rank-2 array expression, MATMUL(A, B)
  ! or RESHAPE(X, (/2, 3/)), and the compiler makes a temporary of it.
  function array_of(z) result(q)
    complex(rk), intent(in) :: z(:, :)
    type(quantity) :: q
    type(value) :: v
    integer :: i, j

    if (size(z) == 1) then
      q = scalar_quantity(clean(z(1, 1)))
      return
    end if
    q = new_array(size(z, 1), size(z, 2))
    if (quantity_error(q) /= 0) return
    do j = 1, q%cols
      do i = 1, q%rows
        v = clean(z(i, j))
        if (v%err /= 0) then
          call release(q)
          q = failure(v%err)
          return
        end if
        q%e(i, j) = v%z
      end do
    end do
  end function array_of

  ! RELEASE gives an array's storage back, leaving Q the scalar 0. A
  ! scalar has none to give.
  subroutine release(q)
    type(quantity), intent(inout) :: q

    if (is_array(q)) then
      deallocate (q%e)
      in_use = in_use - 1
    end if
    q = real_quantity(0.0_rk)
  end subroutine release

  ! DUPLICATE is a copy of Q with storage of its own.
  function duplicate(q) result(r)
    type(quantity), intent(in) :: q
    type(quantity) :: r

    if (.not. is_array(q)) then
      r = scalar_quantity(q%s)
      return
    end if
    r = new_array(q%rows, q%cols)
    if (quantity_error(r) == 0) r%e = q%e
  end function duplicate

  integer function arrays_in_use()
    arrays_in_use = in_use
  end function arrays_in_use

  ! ------------------------------------------------------------------
  ! What a quantity is.

  logical function is_array(q)
    type(quantity), intent(in) :: q

    is_array = q%rows * q%cols > 1
  end function is_array

  ! IS_VECTOR: Q is an array of one row or one column.
  logical function is_vector(q)
    type(quantity), intent(in) :: q

    is_vector = is_array(q) .and. (q%rows == 1 .or. q%cols == 1)
  end function is_vector

  integer function numel(q)
    type(quantity), intent(in) :: q

    numel = q%rows * q%cols
  end function numel

  logical function same_shape(a, b)
    type(quantity), intent(in) :: a, b

    same_shape = a%rows == b%rows .and. a%cols == b%cols
  end function same_shape

  integer function quantity_error(q)
    type(quantity), intent(in) :: q

    quantity_error = q%s%err
  end function quantity_error

  ! QUANTITY_TRUTH: a scalar is true if it isn't zero, and an array if no
  ! element is.
  logical function quantity_truth(q)
    type(quantity), intent(in) :: q

    if (is_array(q)) then
      quantity_truth = all(q%e /= (0.0_rk, 0.0_rk))
    else
      quantity_truth = truth(q%s)
    end if
  end function quantity_truth

  ! ELEMENT is the element (I, J) as a value; a scalar's every element is
  ! itself, so a scalar combines with each element of an array.
  function element(q, i, j) result(v)
    type(quantity), intent(in) :: q
    integer, intent(in) :: i, j
    type(value) :: v

    if (is_array(q)) then
      v%z = q%e(i, j)
      v%err = 0
    else
      v = q%s
    end if
  end function element

  ! ELEMENT_AT is the Kth element in Fortran's array element order, down
  ! each column in turn, as RESHAPE and a single index count them.
  function element_at(q, k) result(v)
    type(quantity), intent(in) :: q
    integer, intent(in) :: k

    type(value) :: v

    v = element(q, mod(k - 1, q%rows) + 1, (k - 1) / q%rows + 1)
  end function element_at

  ! ELEMENTS_OF is Q's elements as an array of its shape, a scalar's as a
  ! 1 by 1 array. The result's shape comes from the argument's components:
  ! a function's result may be an array whose bounds are expressions of its
  ! arguments, as an automatic array's may.
  function elements_of(q) result(z)
    type(quantity), intent(in) :: q
    complex(rk) :: z(q%rows, q%cols)

    if (is_array(q)) then
      z = q%e
    else
      z(1, 1) = q%s%z
    end if
  end function elements_of

  ! LARGEST_MAGNITUDE is the largest of Q's elements' magnitudes.
  real(rk) function largest_magnitude(q)
    type(quantity), intent(in) :: q
    integer :: i, j

    if (.not. is_array(q)) then
      largest_magnitude = c_abs(q%s%z)
      return
    end if
    largest_magnitude = 0
    do j = 1, q%cols
      do i = 1, q%rows
        largest_magnitude = max(largest_magnitude, c_abs(q%e(i, j)))
      end do
    end do
  end function largest_magnitude

  ! ------------------------------------------------------------------
  ! Showing quantities.

  ! SHOW_QUANTITY shows Q: a scalar as a value, after NAME = if NAME isn't
  ! blank; an array after a line NAME = (if NAME isn't blank), a line for
  ! each row, the columns lined up at the right. Each element is shown
  ! beside the largest, so a part smaller than it by more than the digits
  ! shown is shown as 0: the last bits a computation leaves (1E-17 beside
  ! 1) differ between machines, and the digits can't show them anyway.
  !
  ! A row longer than a line is shown in groups of columns, each group
  ! under a heading naming its columns.
  subroutine show_quantity(q, name)
    type(quantity), intent(in) :: q
    character(len=*), intent(in) :: name
    integer :: width(q%cols)
    character(len=line_width) :: line
    character(len=120) :: text
    real(rk) :: big
    integer :: i, j, first, last, used

    if (.not. is_array(q)) then
      if (len_trim(name) > 0) then
        call say(trim(name) // ' = ' // trim(format_value(q%s)))
      else
        call say(trim(format_value(q%s)))
      end if
      return
    end if

    if (len_trim(name) > 0) call say(trim(name) // ' =')
    big = largest_magnitude(q)

    ! Each column's width is its widest element's. (WIDTH is an automatic
    ! array: its size is the argument's Q%COLS.)
    do j = 1, q%cols
      width(j) = 0
      do i = 1, q%rows
        width(j) = max(width(j), len_trim(format_value(element(q, i, j), big)))
      end do
    end do

    first = 1
    do while (first <= q%cols)
      ! The columns that fit in a line, two blanks before each.
      last = first
      used = width(first) + 2
      do while (last < q%cols)
        if (used + width(last + 1) + 2 > line_width) exit
        last = last + 1
        used = used + width(last) + 2
      end do
      if (first > 1 .or. last < q%cols) call say(trim(columns_heading(first, last)))

      do i = 1, q%rows
        line = ' '
        used = 0
        do j = first, last
          text = format_value(element(q, i, j), big)
          used = used + 2 + width(j)
          line(used - len_trim(text) + 1:used) = trim(text)
        end do
        call say(line(:used))
      end do
      first = last + 1
    end do
  end subroutine show_quantity

  ! COLUMNS_HEADING is "columns 1 to 9:" (or "column 10:").
  function columns_heading(first, last) result(text)
    integer, intent(in) :: first, last
    character(len=40) :: text
    character(len=12) :: a, b

    write (a, '(I12)') first
    write (b, '(I12)') last
    if (first == last) then
      text = 'column ' // trim(adjustl(a)) // ':'
    else
      text = 'columns ' // trim(adjustl(a)) // ' to ' // trim(adjustl(b)) // ':'
    end if
  end function columns_heading

  ! LIST_QUANTITY shows NAME = Q on one line, as LIST does: an array as the
  ! literal that makes it, [1, 2; 3, 4], which may be longer than any line
  ! variable could hold, so it is written a piece at a time (SAY_PART).
  subroutine list_quantity(name, q)
    character(len=*), intent(in) :: name
    type(quantity), intent(in) :: q
    real(rk) :: big
    integer :: i, j

    if (.not. is_array(q)) then
      call say(trim(name) // ' = ' // trim(format_value(q%s)))
      return
    end if

    big = largest_magnitude(q)
    call say_part(trim(name) // ' = [')
    do i = 1, q%rows
      if (i > 1) call say_part('; ')
      do j = 1, q%cols
        if (j > 1) call say_part(', ')
        call say_part(trim(format_value(element(q, i, j), big)))
      end do
    end do
    call say_part(']')
    call say_end()
  end subroutine list_quantity
end module fcarrays
