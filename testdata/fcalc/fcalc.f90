! fcalc, a scientific calculator: the main program.
!
! fcalc reads lines at the prompt until QUIT, EXIT, or the end of the
! input. A line that starts with a command word is a command; NAME = EXPR
! assigns a variable; F(X, Y) = EXPR defines a function; V(I, J) = EXPR
! assigns elements of an array; anything else is an expression, whose value
! is shown and kept as ANS. A line starting with ! is a comment.
!
! The modules each do one part of the work, and this program joins them:
! a line is read (READ_LINE), cut into tokens (FCLEXER, through SCAN_LINE),
! classified by its first tokens (EXECUTE), parsed into a tree (FCTREE),
! and evaluated (FCEVAL); an error anywhere is recorded (FCMESSAGES) and
! reported once the line is done.
!
! The procedures after CONTAINS are internal procedures of the program:
! they see the program's variables (LINE, ECHO, DONE, ...) by host
! association, as if declared in each of them, which is how they share
! the state of the session without passing it around.
program fcalc
  use fckinds
  use fcmessages
  use fcvalues
  use fcarrays
  use fclexer
  use fctree
  use fceval
  use fcsymbols
  use fcfuncs
  use fcnumeric
  implicit none

  ! The words SET and SHOW know, each list in the order of the settings'
  ! codes (FCVALUES' F_AUTO to F_ENGINEERING, FCFUNCS' A_RADIANS to
  ! A_GRADS), so a code is a word's place. The elements of an array
  ! constructor must all have the same length, hence the blanks.
  character(len=11), parameter :: format_names(4) = (/ 'auto       ', &
    'fixed      ', 'scientific ', 'engineering' /)
  character(len=7), parameter :: angle_names(3) = (/ 'radians', 'degrees', 'grads  ' /)
  character(len=8), parameter :: settings(6) = (/ 'digits  ', 'format  ', &
    'angle   ', 'base    ', 'seed    ', 'echo    ' /)

  character(len=line_length + 1) :: line
  integer :: length, status
  logical :: echo, done, echoed, quiet

  echo = .false.
  done = .false.
  call start_symbols()
  call set_variable('ans', real_quantity(0.0_rk))

  do while (.not. done)
    call say_more('fcalc> ')
    call read_line(status)
    if (status < 0) then
      ! The end of the input: end the prompt's line.
      call say('')
      exit
    end if

    ! A line too long isn't echoed: VMS's FORTRAN gives none of it.
    echoed = echo .and. status == 0
    if (echoed) call say(line(:length))

    if (status > 0) then
      call fail(e_linelong, 0)
    else
      call execute(line(:length))
    end if

    ! SET ECHO echoes the line that set it.
    if (echo .and. .not. echoed .and. status == 0) call say(line(:length))

    call report()
  end do

contains

  ! READ_LINE reads the next line into LINE(:LENGTH), by nonadvancing
  ! input: STATUS is 0, or 1 for a line too long (whose rest is passed
  ! over), or -1 at the end of the input.
  !
  ! A nonadvancing READ (ADVANCE='NO') reads part of a record and leaves
  ! the rest for the next READ. SIZE= gives how many characters it read,
  ! so a line's length is known without counting trailing blanks; EOR=
  ! names a statement label to go to when the record (the line) ends before
  ! the variable is full, which is the usual case, and END= one for the end
  ! of the input. These labels, 10, 20, and 30, are the old-fashioned GO TO
  ! targets Fortran's I/O statements still use. A line too long for the system
  ! to read (on VMS, past 199 characters) is an error the READ's IOSTAT=
  ! catches, after which the next READ reads the next line.
  subroutine read_line(status)
    integer, intent(out) :: status
    character(len=80) :: rest
    integer :: ios

    status = 0
    length = 0
    read (*, '(A)', advance='NO', size=length, eor=10, end=30, iostat=ios) line
    status = 1
    if (ios /= 0) return

    ! The line filled LINE without ending: pass over the rest.
    do
      read (*, '(A)', advance='NO', eor=20, end=20, iostat=ios) rest
      if (ios /= 0) return
    end do
10  if (length > line_length) status = 1
20  return
30  status = -1
  end subroutine read_line

  ! EXECUTE runs one line. A line ending in ; is quiet: an expression's
  ! value, an assignment, or a definition is made but not shown, as in
  ! MATLAB and Octave (a ; in brackets separates a matrix's rows, but none
  ! ends a line).
  subroutine execute(line_text)
    character(len=*), intent(in) :: line_text
    integer :: k, last

    if (len_trim(line_text) == 0) return
    k = verify(line_text, ' ' // achar(9))
    if (line_text(k:k) == '!') return

    last = len_trim(line_text)
    quiet = line_text(last:last) == ';'
    if (quiet) last = last - 1
    call run(line_text(:last))
  end subroutine execute

  ! RUN runs the line TEXT.
  subroutine run(text)
    character(len=*), intent(in) :: text
    type(token) :: first, second

    call scan_line(text)
    if (failed()) return

    first = token_at(1)
    second = token_at(2)

    if (first%kind == t_name .and. .not. (second%kind == t_operator .and. &
        (second%text == '=' .or. second%text == '('))) then
      select case (first%text)
      case ('quit', 'exit')
        done = .true.
        return
      case ('list')
        call list_command()
        return
      case ('clear')
        call clear_command()
        return
      case ('set')
        call set_command()
        return
      case ('show')
        call show_command()
        return
      case ('roots')
        call roots_command()
        return
      end select
    end if

    ! SOLVE, INTEGRATE, and DIFF are reserved words, so they may be followed
    ! by a parenthesis: DIFF (X + 1)^2 FOR X AT 1.
    if (first%kind == t_name .and. .not. (second%kind == t_operator .and. &
        second%text == '=')) then
      select case (first%text)
      case ('solve')
        call solve_command()
        return
      case ('integrate')
        call integrate_command()
        return
      case ('diff')
        call diff_command()
        return
      end select
    end if

    if (first%kind == t_name .and. second%kind == t_operator .and. second%text == '=') then
      call assignment(first)
    else if (is_definition() .and. .not. indexes_array(first%text)) then
      call definition(text)
    else
      call statement()
    end if
  end subroutine run

  ! INDEXES_ARRAY: NAME(...) = would assign elements of NAME, a variable
  ! holding an array, rather than define a function: NAME isn't also a
  ! function. So v(k) = 0 assigns an element of the array v, while f(x) =
  ! x^2 defines f.
  logical function indexes_array(name)
    character(len=*), intent(in) :: name

    indexes_array = holds_array(name)
    if (indexes_array) indexes_array = .not. associated(find_function(name))
  end function indexes_array

  ! STATEMENT is an expression, whose value is shown and kept as ANS, or an
  ! assignment to elements, V(...) = EXPR: which, the parse of the line's
  ! start says, a call of V followed by =.
  subroutine statement()
    type(node), pointer :: tree
    type(quantity) :: v

    tree => parse_expression()
    if (failed()) return
    if (is_operator('=') .and. kind_at(1) == t_name .and. tree%kind == n_call) then
      call element_assignment(tree)
      return
    end if
    if (.not. at_end()) then
      call fail(e_extra, current_column())
      call free_tree(tree)
      return
    end if

    v = evaluate(tree)
    call free_tree(tree)
    if (failed()) then
      call release(v)
      return
    end if
    if (.not. quiet) call show_quantity(v, '')
    call set_variable('ans', v)
  end subroutine statement

  ! ASSIGNMENT is NAME = EXPR. The variable takes the value: it is shown
  ! first, then given to SET_VARIABLE, which owns it from then on.
  subroutine assignment(name)
    type(token), intent(in) :: name
    type(quantity) :: v

    if (.not. usable_name(name)) return

    call advance(2)
    if (.not. whole_expression(v)) return
    if (.not. quiet) call show_quantity(v, name%text)
    call set_variable(name%text, v)
  end subroutine assignment

  ! ELEMENT_ASSIGNMENT is V(...) = EXPR, TARGET the tree of V(...): the
  ! elements of the variable V that the indexes name are given EXPR's
  ! value. The variable's value is copied, changed, and set again.
  subroutine element_assignment(target)
    type(node), pointer :: target
    type(token) :: name
    type(quantity) :: x, y

    name = token_at(1)
    if (.not. usable_name(name)) then
      call free_tree(target)
      return
    end if
    if (.not. find_variable(name%text, x)) then
      call fail(e_undefined, name%column, name%text)
      call free_tree(target)
      return
    end if

    call advance(1)
    if (whole_expression(y)) then
      if (assign_into(x, target, y)) then
        if (.not. quiet) call show_quantity(x, name%text)
        call set_variable(name%text, x)
        x = real_quantity(0.0_rk)
      end if
    end if
    call release(y)
    call release(x)
    call free_tree(target)
  end subroutine element_assignment

  ! WHOLE_EXPRESSION parses and evaluates the rest of the line as an
  ! expression: false after an error, when V is released.
  logical function whole_expression(v)
    type(quantity), intent(out) :: v
    type(node), pointer :: tree

    whole_expression = .false.
    v = real_quantity(0.0_rk)
    tree => parse_expression()
    if (failed()) return
    if (.not. at_end()) then
      call fail(e_extra, current_column())
      call free_tree(tree)
      return
    end if

    v = evaluate(tree)
    call free_tree(tree)
    whole_expression = .not. failed()
    if (.not. whole_expression) call release(v)
  end function whole_expression

  ! IS_DEFINITION: the line starts f(x, y) =, a function's definition. A
  ! call, f(2) or f(x + 1), has something other than names between its
  ! parentheses, or no = after them.
  logical function is_definition()
    type(token) :: t
    integer :: k

    is_definition = .false.
    if (kind_at(1) /= t_name) return
    t = token_at(2)
    if (t%kind /= t_operator .or. t%text /= '(') return

    k = 3
    t = token_at(k)
    if (.not. (t%kind == t_operator .and. t%text == ')')) then
      do
        if (kind_at(k) /= t_name) return
        t = token_at(k + 1)
        if (t%kind /= t_operator) return
        k = k + 2
        if (t%text == ')') exit
        if (t%text /= ',') return
      end do
      k = k - 1
    end if

    ! K is the ) token's place.
    t = token_at(k + 1)
    is_definition = t%kind == t_operator .and. t%text == '='
  end function is_definition

  ! DEFINITION is f(x, y) = expr: the function's name and parameters are
  ! checked, the expression parsed, and the tree and the text kept.
  subroutine definition(text)
    character(len=*), intent(in) :: text
    type(token) :: name, t
    character(len=name_length) :: params(max_params)
    type(node), pointer :: tree
    integer :: count, k

    name = token_at(1)
    if (.not. usable_name(name)) return
    if (known_function(name%text)) then
      call fail(e_builtin, name%column, name%text)
      return
    end if

    ! The parameters: the names from the third token, each followed by a
    ! comma or the ).
    count = 0
    k = 3
    do while (kind_at(k) == t_name)
      t = token_at(k)
      if (.not. usable_name(t)) return
      if (count == max_params) then
        call fail(e_params, t%column)
        return
      end if
      if (any(params(1:count) == t%text)) then
        call fail(e_dupparam, t%column, t%text)
        return
      end if
      count = count + 1
      params(count) = t%text
      k = k + 2
    end do
    if (count == 0) k = k + 1

    ! K is the = token's place; the expression starts after it.
    call advance(k)
    tree => parse_expression()
    if (failed()) return
    if (.not. at_end()) then
      call fail(e_extra, current_column())
      call free_tree(tree)
      return
    end if

    t = token_at(k + 1)
    call define_function(name%text, count, params, tree, trim(text(t%column:)))
    if (.not. quiet) call list_one(name%text)
  end subroutine definition

  ! LIST_ONE shows the function NAME as LIST does.
  subroutine list_one(name)
    character(len=*), intent(in) :: name
    type(function_entry), pointer :: fn

    fn => find_function(name)
    call say(trim(heading(fn)) // ' = ' // trim(fn%text))
  end subroutine list_one

  ! USABLE_NAME: the token T may name a variable, a function, or a
  ! parameter: it isn't a reserved word or a constant.
  logical function usable_name(t)
    type(token), intent(in) :: t

    usable_name = .false.
    if (is_reserved(t%text)) then
      call fail(e_reserved, t%column, t%text)
    else if (is_constant(t%text)) then
      call fail(e_constant, t%column, t%text)
    else
      usable_name = .true.
    end if
  end function usable_name

  logical function is_reserved(name)
    character(len=*), intent(in) :: name

    is_reserved = name == 'or' .or. name == 'and' .or. name == 'not' .or. name == 'mod' .or. &
      name == 'solve' .or. name == 'integrate' .or. name == 'diff'
  end function is_reserved

  ! LIST [VARIABLES | FUNCTIONS | ALL]: with no word, ALL, the variables
  ! and then the functions.
  subroutine list_command()
    type(token) :: what
    character(len=name_length) :: which

    which = 'all'
    what = token_at(2)
    if (what%kind == t_name) then
      which = what%text
      if (which /= 'variables' .and. which /= 'functions' .and. which /= 'all') then
        call fail(e_badcmd, what%column)
        return
      end if
      call advance(1)
    end if
    call advance(1)
    if (.not. at_end()) then
      call fail(e_extra, current_column())
      return
    end if
    if (which /= 'functions') call list_variables()
    if (which /= 'variables') call list_functions()
  end subroutine list_command

  ! CLEAR name... | CLEAR ALL
  subroutine clear_command()
    type(token) :: what
    integer :: k
    logical :: gone

    what = token_at(2)
    if (what%kind == t_name .and. what%text == 'all' .and. kind_at(3) == t_end) then
      call clear_variables()
      call clear_functions()
      call set_variable('ans', real_quantity(0.0_rk))
      return
    end if

    if (what%kind /= t_name) then
      call fail(e_namereq, what%column)
      return
    end if
    do k = 2, token_count() - 1
      what = token_at(k)
      if (what%kind /= t_name) then
        call fail(e_namereq, what%column)
        return
      end if
    end do
    ! A name may be both a variable and a function: CLEAR forgets both.
    do k = 2, token_count() - 1
      what = token_at(k)
      gone = remove_variable(what%text)
      if (remove_function(what%text)) gone = .true.
      if (.not. gone) then
        call fail(w_notdef, what%column, what%text)
        call report()
      end if
    end do
  end subroutine clear_command

  ! SHOW MEMORY: how many arrays and tree nodes are in use, so a leak shows.
  ! SHOW SETTINGS: every setting, each as the SET command that makes it, so
  ! the lines may be typed back, or kept in a file and run with @, to set
  ! them again. SHOW DIGITS, FORMAT, ANGLE, BASE, SEED, or ECHO shows one.
  subroutine show_command()
    type(token) :: what
    character(len=12) :: count
    integer :: k

    what = token_at(2)
    if (what%kind /= t_name) then
      call fail(e_badset, what%column)
      return
    end if
    if (kind_at(3) /= t_end) then
      call fail(e_extra, token_column(3))
      return
    end if

    select case (what%text)
    case ('memory')
      write (count, '(I12)') arrays_in_use()
      call say('arrays in use: ' // trim(adjustl(count)))
      write (count, '(I12)') nodes_in_use()
      call say('tree nodes in use: ' // trim(adjustl(count)))
    case ('settings')
      do k = 1, size(settings)
        if (settings(k) == 'seed' .and. .not. seed_given) cycle
        call say(trim(setting_line(settings(k))))
      end do
    case default
      ! ANY of a comparison of a scalar with an array: is the word one of
      ! the settings? The comparison is made element by element, giving an
      ! array of LOGICALs, which ANY reduces to one.
      if (.not. any(settings == what%text)) then
        call fail(e_badset, what%column)
        return
      end if
      call say(trim(setting_line(what%text)))
    end select
  end subroutine show_command

  ! SETTING_LINE is the SET command that makes the setting WHAT as it is
  ! now: set digits 12. A seed never given is a comment, which fcalc
  ! passes over if it is typed back.
  function setting_line(what) result(text)
    character(len=*), intent(in) :: what
    character(len=40) :: text
    character(len=12) :: number

    select case (what)
    case ('digits')
      write (number, '(I12)') digits
      text = 'set digits ' // adjustl(number)
    case ('format')
      text = 'set format ' // format_names(number_format)
    case ('angle')
      text = 'set angle ' // angle_names(angle_mode)
    case ('base')
      select case (number_base)
      case (16)
        text = 'set base hex'
      case (8)
        text = 'set base octal'
      case (2)
        text = 'set base binary'
      case default
        text = 'set base decimal'
      end select
    case ('seed')
      if (seed_given) then
        write (number, '(I12)') seed_number
        text = 'set seed ' // adjustl(number)
      else
        text = '! no seed has been set'
      end if
    case default
      if (echo) then
        text = 'set echo'
      else
        text = 'set noecho'
      end if
    end select
  end function setting_line

  ! SET [NO]ECHO, SET DIGITS n, SET FORMAT AUTO | FIXED | SCIENTIFIC |
  ! ENGINEERING, SET ANGLE RADIANS | DEGREES | GRADS, SET BASE DECIMAL |
  ! HEX | OCTAL | BINARY, SET SEED n
  subroutine set_command()
    type(token) :: what, how
    integer :: k, n

    what = token_at(2)
    if (what%kind /= t_name) then
      call fail(e_badset, what%column)
      return
    end if

    select case (what%text)
    case ('echo', 'noecho')
      if (kind_at(3) /= t_end) then
        call fail(e_extra, token_column(3))
        return
      end if
      echo = what%text == 'echo'

    case ('angle')
      if (.not. word_argument(how)) return
      k = word_index(how, angle_names)
      if (k > 0) angle_mode = k

    case ('format')
      if (.not. word_argument(how)) return
      k = word_index(how, format_names)
      if (k > 0) number_format = k

    case ('base')
      if (.not. word_argument(how)) return
      select case (how%text)
      case ('decimal')
        number_base = 10
      case ('hex', 'hexadecimal')
        number_base = 16
      case ('octal')
        number_base = 8
      case ('binary')
        number_base = 2
      case default
        call fail(e_badset, how%column)
      end select

    case ('digits')
      ! A whole number of significant digits (or, under FIXED, of places
      ! after the point), 1 to MAX_DIGITS.
      if (whole_argument(1, max_digits, n)) digits = n

    case ('seed')
      if (whole_argument(0, 999999999, n)) call seed_random(n)

    case default
      call fail(e_badset, what%column)
    end select
  end subroutine set_command

  ! WORD_ARGUMENT: SET's third token is a word, HOW, and the line's last.
  logical function word_argument(how)
    type(token), intent(out) :: how

    word_argument = .false.
    how = token_at(3)
    if (how%kind /= t_name) then
      call fail(e_badset, how%column)
    else if (kind_at(4) /= t_end) then
      call fail(e_extra, token_column(4))
    else
      word_argument = .true.
    end if
  end function word_argument

  ! WORD_INDEX is HOW's place among NAMES, or 0 after an error if it is
  ! none of them.
  integer function word_index(how, names)
    type(token), intent(in) :: how
    character(len=*), intent(in) :: names(:)
    integer :: k

    word_index = 0
    do k = 1, size(names)
      if (names(k) == how%text) word_index = k
    end do
    if (word_index == 0) call fail(e_badset, how%column)
  end function word_index

  ! WHOLE_ARGUMENT: SET's third token is a whole number from LOW to HIGH,
  ! N, and the line's last. (A minus sign is a token of its own, so a
  ! negative number is out of range at the sign.)
  logical function whole_argument(low, high, n)
    integer, intent(in) :: low, high
    integer, intent(out) :: n
    type(token) :: how

    whole_argument = .false.
    n = 0
    how = token_at(3)
    if (how%kind == t_operator .and. how%text == '-' .and. kind_at(4) == t_number) then
      call fail(e_domain, how%column)
    else if (how%kind /= t_number .or. how%imaginary) then
      call fail(e_badset, how%column)
    else if (kind_at(4) /= t_end) then
      call fail(e_extra, token_column(4))
    else if (aint(how%number) /= how%number .or. how%number < low .or. &
        how%number > high) then
      call fail(e_domain, how%column)
    else
      n = nint(how%number)
      whole_argument = .true.
    end if
  end function whole_argument

  ! TOKEN_COLUMN is the column of the Kth token.
  integer function token_column(k)
    integer, intent(in) :: k
    type(token) :: t

    t = token_at(k)
    token_column = t%column
  end function token_column
end program fcalc
