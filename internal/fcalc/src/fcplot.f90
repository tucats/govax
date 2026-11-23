! fcalc: TABLE and PLOT, an expression's values in columns and as a graph.
!
!     TABLE expr[, expr...] FOR x FROM a TO b [STEP s]
!     TABLE v[, w...]
!     PLOT expr[, expr...] FOR x FROM a TO b [SIZE w, h] [Y lo, hi]
!     PLOT v[, w...] [VERSUS u] [SIZE w, h] [Y lo, hi]
!
! TABLE with FOR evaluates each expression at A, A + S, A + 2S, ..., up to
! B (S is (B - A)/10 if no STEP is given), as FCNUMERIC's commands do,
! through FCEVAL's EVALUATE_WITH: the name after FOR stands for the point,
! and a variable of that name is left alone. Each row shows the point and
! the expressions' values there. Without FOR, each expression is a vector,
! all of one length, and the table is the vectors side by side.
!
! PLOT with FOR evaluates each expression at as many points as the graph
! is wide, from A to B, and marks each value in its column. Without FOR,
! each expression is a vector, drawn against its index (1, 2, ...), or,
! after VERSUS, against the elements of the vector U. The graph is W
! characters wide and H high (61 by 19 unless SIZE says otherwise); its
! vertical range is the values' smallest to largest, unless Y gives it.
! Each curve has its mark, *, o, #, x, @, %, &, and $ in order, named in
! the legend under the graph; a later curve's mark covers an earlier one's.
! The axes are drawn where zero is in the range: a row of - for y = 0 and
! a column of | for x = 0, + where they cross; at the graph's left edge the
! frame is the y axis.
!
! A point where an expression can't be evaluated (1/X at 0) is the error's
! name in a table, DIVZERO, and no mark in a graph; so is a point whose
! value isn't real, in a graph, which has no place for an imaginary part.
! The errors that are a point's are the arithmetic's (POINT_ERROR); any
! other, a name not defined or a vector where a number was wanted, stops
! the command, at every point alike.
!
! A graph is exact in one way only: a value's row is the nearest to it, in
! H rows from the range's top to its bottom, and a point's column the
! nearest in W. So a mark is where the value rounds to, by Fortran's NINT,
! and a value halfway between two rows goes away from zero, wherever the
! arithmetic leaves it; a test chooses values that aren't halfway, so
! machines with different last bits draw the same marks.
!
! This module exercises arrays whose size is known only when the program
! runs: an automatic CHARACTER array of the graph's rows, whose length and
! number are its arguments' (GRID in DRAW); allocatable arrays of the
! samples and the table's cells; an array-valued function (SAMPLES); WHERE;
! MINVAL and MAXVAL with MASK=; and LEN_TRIM of an array, which, like
! every elemental intrinsic, is the array of each element's.
module fcplot
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  use fctree
  use fceval
  ! USE ... ONLY: the command readers FCNUMERIC's commands share, and
  ! nothing else of FCNUMERIC's.
  use fcnumeric, only: at_word, read_word, read_name, read_bound, at_end_of_line
  implicit none
  private

  public :: table_command, plot_command

  ! The most expressions a command takes (one mark each in a graph), and
  ! the most rows a table has.
  integer, parameter :: max_columns = 8, max_rows = 1000

  ! A graph's size unless SIZE gives it, and the limits SIZE may give:
  ! the widest leaves room on a 132-character line for the labels.
  integer, parameter :: default_width = 61, default_height = 19
  integer, parameter :: min_width = 10, max_width = 100, min_height = 3, max_height = 60

  ! The curves' marks, in order.
  character(len=max_columns), parameter :: marks = '*o#x@%&$'

  ! The longest text a table's cell holds: a value as FORMAT_VALUE shows it.
  integer, parameter :: cell_length = 120

  ! An expression of the command: its tree, the column it starts at, and
  ! its text as the user typed it, for a heading or the legend.
  !
  ! Fortran 90 has no array of pointers: an array's elements can't each
  ! be a pointer. An array of a derived type with a pointer component is
  ! the way to one, LIST(K)%P.
  type expression
    type(node), pointer :: p
    integer :: column
    character(len=line_length) :: text
  end type expression

  ! How PLOT places its points across: at points it chooses, FOR; at a
  ! vector's elements, VERSUS; or at the index.
  integer, parameter :: by_points = 1, by_versus = 2, by_index = 3

contains

  ! ------------------------------------------------------------------
  ! Reading the commands.

  ! READ_LIST reads the expressions separated by commas into LIST(1:COUNT),
  ! each with its column and text, cut from the line LINE.
  logical function read_list(line, list, count)
    character(len=*), intent(in) :: line
    type(expression), intent(inout) :: list(:)
    integer, intent(out) :: count

    read_list = .false.
    count = 0
    do
      if (count == size(list)) then
        call fail(e_toomany, current_column(), 'expressions')
        return
      end if
      count = count + 1
      list(count)%column = current_column()
      list(count)%p => parse_expression()
      if (failed()) return
      ! The expression's text is the line from its first column to the
      ! next token's, without the blanks before that one. ADJUSTL moves a
      ! string's leading blanks to its end, where TRIM drops them.
      list(count)%text = line(list(count)%column:current_column() - 1)
      if (.not. is_operator(',')) exit
      call advance(1)
    end do
    read_list = .true.
  end function read_list

  ! FREE_LIST frees the trees of LIST(1:COUNT).
  subroutine free_list(list, count)
    type(expression), intent(inout) :: list(:)
    integer, intent(in) :: count
    integer :: k

    do k = 1, count
      call free_tree(list(k)%p)
    end do
  end subroutine free_list

  ! READ_COMMA: the current token is a comma, and is passed; if not,
  ! KEYWORD.
  logical function read_comma()
    read_comma = is_operator(',')
    if (read_comma) then
      call advance(1)
    else
      call fail(e_keyword, current_column(), "','")
    end if
  end function read_comma

  ! READ_WHOLE reads an expression whose value is a whole number from LOW
  ! to HIGH, N: NOTWHOLE if it isn't whole, DOMAIN if it is out of range.
  logical function read_whole(n, low, high)
    integer, intent(out) :: n
    integer, intent(in) :: low, high
    type(value) :: x
    integer :: column

    read_whole = .false.
    n = low
    if (.not. read_bound(x, .true., column)) return
    if (.not. is_whole(x)) then
      call fail(e_notwhole, column)
    else if (real(x%z, rk) < low .or. real(x%z, rk) > high) then
      call fail(e_domain, column)
    else
      n = nint(real(x%z, rk))
      read_whole = .true.
    end if
  end function read_whole

  ! ------------------------------------------------------------------
  ! Evaluating at a point.

  ! POINT_ERROR: the error CODE is one an expression's arithmetic makes at
  ! a point, not one it makes everywhere.
  logical function point_error(code)
    integer, intent(in) :: code

    select case (code)
    case (e_divzero, e_overflow, e_domain, e_notreal, e_notwhole, e_singular, e_noconverge)
      point_error = .true.
    case default
      point_error = .false.
    end select
  end function point_error

  ! POINT_AT evaluates the tree P, which starts at COLUMN, with NAME
  ! standing for X. True with its value, V, and CODE 0; or true with a
  ! point's error in CODE, which is cleared, not reported; or false after
  ! an error that stops the command, left to be reported.
  logical function point_at(p, column, name, x, v, code)
    type(node), pointer :: p
    integer, intent(in) :: column
    character(len=*), intent(in) :: name
    type(value), intent(in) :: x
    type(value), intent(out) :: v
    integer, intent(out) :: code
    type(quantity) :: q

    point_at = .false.
    code = 0
    v = real_value(0.0_rk)
    q = evaluate_with(p, name, x)
    if (failed() .or. quantity_error(q) /= 0) then
      code = error_code()
      if (code == 0) code = quantity_error(q)
      call release(q)
      if (point_error(code)) then
        call clear_error()
        point_at = .true.
      else if (.not. failed()) then
        call fail(code, column)
      end if
      return
    end if
    if (is_array(q)) then
      call release(q)
      call fail(e_notscalar, column)
      return
    end if
    v = q%s
    point_at = .true.
  end function point_at

  ! WITHOUT_NOISE is X, or 0 if X is no larger than LEVEL: a point
  ! computed as A + K*S, which should be 0, is a rounding error instead
  ! (-0.3 + 3*0.1 is 5.6E-17), and would be shown as one.
  real(rk) function without_noise(x, level)
    real(rk), intent(in) :: x, level

    without_noise = x
    if (abs(x) <= level) without_noise = 0
  end function without_noise

  ! NOISE_LEVEL is the rounding error's size in points from A to B: 64
  ! epsilons of the larger end, computed so it can't underflow.
  real(rk) function noise_level(a, b)
    real(rk), intent(in) :: a, b
    integer :: err

    err = 0
    noise_level = r_mul(64 * eps, max(abs(a), abs(b)), err)
  end function noise_level

  ! SAMPLES is N points from A to B, evenly spaced, the first A and the
  ! last B: an array-valued function, whose result's size is its argument
  ! N's, known only when it is called. (A caller needs the function's
  ! explicit interface to use such a result, which a module procedure
  ! always has.)
  function samples(a, b, n) result(x)
    real(rk), intent(in) :: a, b
    integer, intent(in) :: n
    real(rk) :: x(n)
    real(rk) :: step, level
    integer :: j, err

    err = 0
    step = r_div(r_add(b, -a, err), real(n - 1, rk), err)
    level = noise_level(a, b)
    do j = 1, n
      x(j) = without_noise(r_add(a, r_mul(real(j - 1, rk), step, err), err), level)
    end do
    x(n) = b
  end function samples

  ! ------------------------------------------------------------------
  ! TABLE.

  subroutine table_command(line)
    character(len=*), intent(in) :: line
    type(expression) :: list(max_columns)
    character(len=name_length) :: name
    character(len=line_length) :: name_text
    type(value) :: a, b, s
    integer :: count, k, column, name_column, step_column
    logical :: stepped

    do k = 1, max_columns
      nullify (list(k)%p)
    end do
    count = 0
    stepped = .false.
    call advance(1)

    read: do
      if (.not. read_list(line, list, count)) exit read
      if (at_word('for')) then
        call advance(1)
        name_column = current_column()
        if (.not. read_name(name)) exit read
        name_text = line(name_column:current_column() - 1)
        if (.not. read_word('from', 'FROM')) exit read
        if (.not. read_bound(a, .true., column)) exit read
        if (.not. read_word('to', 'TO')) exit read
        if (.not. read_bound(b, .true., column)) exit read
        if (at_word('step')) then
          call advance(1)
          stepped = .true.
          step_column = current_column()
          if (.not. read_bound(s, .true., column)) exit read
        end if
        if (.not. at_end_of_line()) exit read
        call table_of_points()
      else
        if (.not. at_end_of_line()) exit read
        call table_of_vectors(list, count)
      end if
      exit read
    end do read

    call free_list(list, count)

  contains

    ! TABLE_OF_POINTS is the table with FOR. An internal procedure of
    ! TABLE_COMMAND: it sees the command's variables, LIST, NAME, A, B, S,
    ! by host association.
    subroutine table_of_points()
      character(len=cell_length), allocatable :: cells(:, :)
      character(len=line_length) :: heads(max_columns + 1)
      real(rk) :: x0, x1, step, steps, level
      type(value) :: v, w
      integer :: n, j, k, code, err

      x0 = real(a%z, rk)
      x1 = real(b%z, rk)
      err = 0

      ! How many rows: the steps from A to B, and one more for A. A count
      ! a rounding error short of a whole number (0.3 / 0.1 is
      ! 2.9999999999999996) is that number, so B's row isn't lost.
      if (x0 == x1) then
        n = 1
        step = 0
      else
        if (stepped) then
          step = real(s%z, rk)
        else
          step = r_div(r_add(x1, -x0, err), 10.0_rk, err)
        end if
        if (step == 0) then
          call fail(e_badstep, step_column)
          return
        end if
        steps = r_div(r_add(x1, -x0, err), step, err)
        if (steps < 0) then
          call fail(e_badstep, step_column)
          return
        end if
        if (err /= 0 .or. steps > max_rows) then
          call fail(e_toomany, step_column, 'rows')
          return
        end if
        n = int(steps + 1.0e-9_rk) + 1
        if (n > max_rows) then
          call fail(e_toomany, step_column, 'rows')
          return
        end if
      end if

      allocate (cells(n, count + 1))
      heads(1) = name_text
      heads(2:count + 1) = list(1:count)%text
      level = noise_level(x0, x1)

      do j = 1, n
        v = real_value(without_noise(r_add(x0, r_mul(real(j - 1, rk), step, err), err), level))
        cells(j, 1) = format_value(v)
        do k = 1, count
          if (.not. point_at(list(k)%p, list(k)%column, name, v, w, code)) then
            deallocate (cells)
            return
          end if
          if (code == 0) then
            cells(j, k + 1) = format_value(w)
          else
            cells(j, k + 1) = error_ident(code)
          end if
        end do
      end do

      call show_table(heads(1:count + 1), cells)
      deallocate (cells)
    end subroutine table_of_points
  end subroutine table_command

  ! TABLE_OF_VECTORS is the table without FOR: the vectors LIST(1:COUNT)
  ! side by side, each a column.
  subroutine table_of_vectors(list, count)
    type(expression), intent(inout) :: list(:)
    integer, intent(in) :: count
    character(len=cell_length), allocatable :: cells(:, :)
    type(quantity) :: q
    integer :: n, j, k

    n = 0
    do k = 1, count
      q = evaluate(list(k)%p)
      if (failed()) then
        call release(q)
        if (allocated(cells)) deallocate (cells)
        return
      end if
      if (.not. is_vector(q)) then
        call fail(e_notvector, list(k)%column)
      else if (k > 1 .and. numel(q) /= n) then
        call fail(e_notconform, list(k)%column)
      else if (numel(q) > max_rows) then
        call fail(e_toomany, list(k)%column, 'rows')
      end if
      if (failed()) then
        call release(q)
        if (allocated(cells)) deallocate (cells)
        return
      end if
      if (k == 1) then
        n = numel(q)
        allocate (cells(n, count))
      end if
      do j = 1, n
        cells(j, k) = format_value(element_at(q, j))
      end do
      call release(q)
    end do

    call show_table(list(1:count)%text, cells)
    deallocate (cells)
  end subroutine table_of_vectors

  ! SHOW_TABLE shows the table: a heading over each column, a rule under
  ! it, and the cells, each column as wide as its widest entry, right-
  ! aligned, two blanks before it, as a matrix's columns are shown.
  subroutine show_table(heads, cells)
    character(len=*), intent(in) :: heads(:), cells(:, :)
    integer :: widths(size(heads))
    integer :: j, k

    ! LEN_TRIM of an array is an array of each element's length: LEN_TRIM,
    ! like most intrinsic functions, is elemental. MAXVAL of a column's
    ! lengths is its widest cell's. (A heading is trimmed after ADJUSTL:
    ! its text may start with a blank.)
    do k = 1, size(heads)
      widths(k) = max(len_trim(adjustl(heads(k))), maxval(len_trim(cells(:, k))))
    end do

    do k = 1, size(heads)
      call say_part('  ' // right(adjustl(heads(k)), widths(k)))
    end do
    call say_end()
    do k = 1, size(heads)
      call say_part('  ' // repeat('-', widths(k)))
    end do
    call say_end()
    do j = 1, size(cells, 1)
      do k = 1, size(heads)
        call say_part('  ' // right(cells(j, k), widths(k)))
      end do
      call say_end()
    end do

  contains

    ! RIGHT is TEXT, trimmed, right-aligned in WIDTH characters: a function
    ! whose result's length is an expression of its argument, a run-time
    ! CHARACTER length.
    function right(text, width) result(r)
      character(len=*), intent(in) :: text
      integer, intent(in) :: width
      character(len=width) :: r

      r = repeat(' ', width - len_trim(text)) // trim(text)
    end function right
  end subroutine show_table

  ! ------------------------------------------------------------------
  ! PLOT.

  subroutine plot_command(line)
    character(len=*), intent(in) :: line
    type(expression) :: list(max_columns)
    type(node), pointer :: across
    character(len=name_length) :: name
    type(value) :: a, b, bound
    integer :: count, k, column, across_column, form, width, height
    real(rk) :: ylo, yhi, xlo, xhi
    logical :: y_given
    ! The points: each one's place across, XS(J), and each curve's value
    ! there, YS(J, K), if OK(J, K) says it has one.
    real(rk), allocatable :: xs(:), ys(:, :)
    logical, allocatable :: ok(:, :)

    do k = 1, max_columns
      nullify (list(k)%p)
    end do
    nullify (across)
    count = 0
    form = by_index
    width = default_width
    height = default_height
    y_given = .false.
    ylo = 0
    yhi = 0
    call advance(1)

    read: do
      if (.not. read_list(line, list, count)) exit read
      if (at_word('for')) then
        call advance(1)
        form = by_points
        if (.not. read_name(name)) exit read
        if (.not. read_word('from', 'FROM')) exit read
        if (.not. read_bound(a, .true., column)) exit read
        if (.not. read_word('to', 'TO')) exit read
        if (.not. read_bound(b, .true., column)) exit read
        if (real(b%z, rk) <= real(a%z, rk)) then
          call fail(e_badlimits, column)
          exit read
        end if
      else if (at_word('versus')) then
        call advance(1)
        form = by_versus
        across_column = current_column()
        across => parse_expression()
        if (failed()) exit read
      end if

      ! The options, in any order. EXIT READ leaves the outer construct,
      ! READ, from inside this unnamed one: a construct name lets EXIT
      ! leave more than the innermost loop.
      do
        if (at_word('size')) then
          call advance(1)
          if (.not. read_whole(width, min_width, max_width)) exit read
          if (.not. read_comma()) exit read
          if (.not. read_whole(height, min_height, max_height)) exit read
        else if (at_word('y')) then
          call advance(1)
          if (.not. read_bound(bound, .true., column)) exit read
          ylo = real(bound%z, rk)
          if (.not. read_comma()) exit read
          if (.not. read_bound(bound, .true., column)) exit read
          yhi = real(bound%z, rk)
          if (yhi <= ylo) then
            call fail(e_badlimits, column)
            exit read
          end if
          y_given = .true.
        else
          exit
        end if
      end do
      if (.not. at_end_of_line()) exit read

      select case (form)
      case (by_points)
        call points_by_for()
      case (by_versus)
        call points_by_versus()
      case default
        call points_by_index()
      end select
      if (failed()) exit read

      ! The range down: the values', or Y's. (MINVAL and MAXVAL with MASK=
      ! look only at the elements whose MASK is true.)
      if (.not. any(ok)) then
        call fail(e_nopoints, list(1)%column)
        exit read
      end if
      if (.not. y_given) then
        ylo = minval(ys, mask=ok)
        yhi = maxval(ys, mask=ok)
        call widen(ylo, yhi)
      else
        ! WHERE assigns only the elements its mask selects: a value outside
        ! the range Y gives has no mark.
        where (ok) ok = ys >= ylo .and. ys <= yhi
      end if

      call draw(xs, ys, ok, xlo, xhi, ylo, yhi, width, height, list(1:count))
      exit read
    end do read

    call free_list(list, count)
    call free_tree(across)
    if (allocated(xs)) deallocate (xs)
    if (allocated(ys)) deallocate (ys)
    if (allocated(ok)) deallocate (ok)

  contains

    ! POINTS_BY_FOR: each expression at WIDTH points from A to B, one a
    ! column.
    subroutine points_by_for()
      type(value) :: v
      integer :: j, k, code

      xlo = real(a%z, rk)
      xhi = real(b%z, rk)
      allocate (xs(width), ys(width, count), ok(width, count))
      xs = samples(xlo, xhi, width)
      ys = 0
      ok = .false.
      do j = 1, width
        do k = 1, count
          if (.not. point_at(list(k)%p, list(k)%column, name, real_value(xs(j)), v, code)) return
          if (code == 0 .and. is_real(v)) then
            ys(j, k) = real(v%z, rk)
            ok(j, k) = .true.
          end if
        end do
      end do
    end subroutine points_by_for

    ! POINTS_BY_VERSUS: each expression a vector, against the vector after
    ! VERSUS, whose elements must be real, and as many as each one's.
    subroutine points_by_versus()
      type(quantity) :: q
      type(value) :: v
      integer :: n, j, k

      q = evaluate(across)
      if (failed()) then
        call release(q)
        return
      end if
      if (.not. is_vector(q)) then
        call fail(e_notvector, across_column)
        call release(q)
        return
      end if
      n = numel(q)
      allocate (xs(n), ys(n, count), ok(n, count))
      do j = 1, n
        v = element_at(q, j)
        if (.not. is_real(v)) then
          call fail(e_notreal, across_column)
          call release(q)
          return
        end if
        xs(j) = real(v%z, rk)
      end do
      call release(q)

      do k = 1, count
        if (.not. vector_of(k, q)) return
        if (numel(q) /= n) then
          call fail(e_notconform, list(k)%column)
          call release(q)
          return
        end if
        call take_column(q, k)
        call release(q)
      end do
      xlo = minval(xs)
      xhi = maxval(xs)
      call widen(xlo, xhi)
    end subroutine points_by_versus

    ! POINTS_BY_INDEX: each expression a vector, against its index; the
    ! vectors may have different lengths, and the longest sets the range.
    subroutine points_by_index()
      type(quantity) :: q(max_columns)
      integer :: n, j, k

      n = 0
      do k = 1, count
        if (.not. vector_of(k, q(k))) then
          do j = 1, k - 1
            call release(q(j))
          end do
          return
        end if
        n = max(n, numel(q(k)))
      end do

      ! An array constructor with an implied DO: (J, J = 1, N) is the
      ! values of J from 1 to N, here as reals.
      allocate (xs(n), ys(n, count), ok(n, count))
      xs = (/ (real(j, rk), j = 1, n) /)
      ys = 0
      ok = .false.
      do k = 1, count
        call take_column(q(k), k)
        call release(q(k))
      end do
      xlo = 1
      xhi = n
    end subroutine points_by_index

    ! VECTOR_OF: the Kth expression's value is a vector, Q.
    logical function vector_of(k, q)
      integer, intent(in) :: k
      type(quantity), intent(out) :: q

      vector_of = .false.
      q = evaluate(list(k)%p)
      if (failed()) then
        call release(q)
        return
      end if
      if (.not. is_vector(q)) then
        call fail(e_notvector, list(k)%column)
        call release(q)
        return
      end if
      vector_of = .true.
    end function vector_of

    ! TAKE_COLUMN puts the vector Q's real elements in column K of YS.
    subroutine take_column(q, k)
      type(quantity), intent(in) :: q
      integer, intent(in) :: k
      type(value) :: v
      integer :: j

      do j = 1, numel(q)
        v = element_at(q, j)
        ok(j, k) = is_real(v)
        ys(j, k) = real(v%z, rk)
      end do
    end subroutine take_column
  end subroutine plot_command

  ! WIDEN makes a range of one value, LO = HI, into one around it: V less
  ! and more max(1, |V|), so a constant's line is in the middle.
  subroutine widen(lo, hi)
    real(rk), intent(inout) :: lo, hi
    real(rk) :: d

    if (lo /= hi) return
    d = max(1.0_rk, abs(lo))
    lo = lo - d
    hi = hi + d
  end subroutine widen

  ! DRAW draws the graph: the points' marks in a grid WIDTH by HEIGHT, the
  ! axes, a frame with the range's labels, and the legend.
  !
  ! GRID is an automatic array: a local array whose size (here, and its
  ! elements' length too) is given by the procedure's arguments, made when
  ! the procedure is called and gone when it returns.
  subroutine draw(xs, ys, ok, xlo, xhi, ylo, yhi, width, height, list)
    real(rk), intent(in) :: xs(:), ys(:, :), xlo, xhi, ylo, yhi
    logical, intent(in) :: ok(:, :)
    integer, intent(in) :: width, height
    type(expression), intent(in) :: list(:)
    character(len=width) :: grid(height)
    character(len=120) :: top, bottom, left, rightmost
    character(len=2 * max_width + 120) :: across
    integer :: j, k, r, c, r0, c0, labels, at

    ! The axes first, so a mark covers them.
    grid = ' '
    r0 = 0
    c0 = 0
    if (ylo <= 0 .and. yhi >= 0) then
      r0 = row_of(0.0_rk)
      grid(r0) = repeat('-', width)
    end if
    if (xlo <= 0 .and. xhi >= 0) c0 = column_of(0.0_rk)
    ! At the first column the frame stands for the y axis.
    if (c0 > 1) then
      do r = 1, height
        grid(r)(c0:c0) = '|'
      end do
      if (r0 > 0) grid(r0)(c0:c0) = '+'
    end if

    do k = 1, size(list)
      do j = 1, size(xs)
        if (.not. ok(j, k)) cycle
        r = row_of(ys(j, k))
        c = column_of(xs(j))
        grid(r)(c:c) = marks(k:k)
      end do
    end do

    ! The labels: the range's top and bottom beside the first and last
    ! rows, and 0 beside the x axis between them; under the grid, the
    ! range's left end, its right end, and 0 at the y axis, if there's room.
    top = label(yhi)
    bottom = label(ylo)
    labels = max(len_trim(top), len_trim(bottom))
    do r = 1, height
      if (r == 1) then
        call say(right_of(top, labels) // ' |' // trim(grid(r)))
      else if (r == height) then
        call say(right_of(bottom, labels) // ' |' // trim(grid(r)))
      else if (r == r0) then
        call say(right_of('0', labels) // ' |' // trim(grid(r)))
      else
        call say(repeat(' ', labels) // ' |' // trim(grid(r)))
      end if
    end do
    call say(repeat(' ', labels) // ' +' // repeat('-', width))

    left = label(xlo)
    rightmost = label(xhi)
    across = left
    at = max(width - len_trim(rightmost) + 1, len_trim(left) + 2)
    across(at:) = rightmost
    if (c0 > len_trim(left) + 1 .and. c0 < at - 1) across(c0:c0) = '0'
    call say(repeat(' ', labels + 2) // trim(across))

    ! The legend, a curve's mark and its expression's text each.
    call say_part(repeat(' ', labels + 2))
    do k = 1, size(list)
      if (k > 1) call say_part('   ')
      call say_part(marks(k:k) // ' ' // trim(adjustl(list(k)%text)))
    end do
    call say_end()

  contains

    ! ROW_OF is the row a value Y is nearest, 1 the top (YHI) and HEIGHT
    ! the bottom (YLO): Y's fraction of the way down the range, times the
    ! rows between, rounded. Each subtraction and the division are guarded
    ! (R_ADD, R_DIV), so values near the VAX's smallest can't underflow.
    integer function row_of(y)
      real(rk), intent(in) :: y
      integer :: err

      err = 0
      row_of = 1 + nint(r_div(r_add(yhi, -y, err), r_add(yhi, -ylo, err), err) * (height - 1))
    end function row_of

    integer function column_of(x)
      real(rk), intent(in) :: x
      integer :: err

      err = 0
      column_of = 1 + nint(r_div(r_add(x, -xlo, err), r_add(xhi, -xlo, err), err) * (width - 1))
    end function column_of

    ! LABEL is X as a label shows it: by the format and base, to at most
    ! six significant digits.
    function label(x) result(text)
      real(rk), intent(in) :: x
      character(len=120) :: text
      integer :: kept

      kept = digits
      digits = min(digits, 6)
      text = adjustl(format_value(real_value(x)))
      digits = kept
    end function label

    function right_of(text, width) result(r)
      character(len=*), intent(in) :: text
      integer, intent(in) :: width
      character(len=width) :: r

      r = repeat(' ', width - len_trim(text)) // trim(text)
    end function right_of
  end subroutine draw
end module fcplot
