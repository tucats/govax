! fcalc, a scientific calculator: the main program.
!
! fcalc reads lines at the prompt until QUIT, EXIT, or the end of the
! input. A line that starts with a command word is a command; NAME = EXPR
! assigns a variable; F(X, Y) = EXPR defines a function; V(I, J) = EXPR
! assigns elements of an array; anything else is an expression, whose value
! is shown and kept as ANS. A line starting with ! is a comment.
!
! Run as a foreign command with text after its name ($ FCALC 2^10 + 1), it
! runs that one line and ends.
!
! HELP shows the FCALC topic of VMS's help library, SYS$HELP:HELPLIB.HLB,
! the text DCL's HELP FCALC shows, through FCPLATFORM's SHOW_HELP.
!
! The modules each do one part of the work, and this program joins them:
! a line is read (READ_LINE), cut into tokens (FCLEXER, through SCAN_LINE),
! classified by its first tokens (EXECUTE), parsed into a tree (FCTREE),
! and evaluated (FCEVAL); an error anywhere is recorded (FCMESSAGES) and
! reported once the line is done. @ (or RUN) and LOAD run a file's lines
! the same way, a line at a time (RUN_FILE); FCSTATE reads the files, and
! saves the state and keeps the log.
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
  use fcplot
  use fcstate
  use fcplatform, only: foreign_text, show_help
  implicit none

  character(len=line_length + 1) :: line
  integer :: length, status
  logical :: echo, done, echoed, quiet

  ! DEPTH is how many files of commands are running, one inside another;
  ! LOADING is true while LOAD runs one, whose lines are all quiet;
  ! ABANDON is true after an error in a file, while every file running
  ! ends, each saying where it was.
  integer :: depth
  logical :: loading, abandon

  echo = .false.
  done = .false.
  depth = 0
  loading = .false.
  abandon = .false.
  call start_symbols()
  call set_variable('ans', real_quantity(0.0_rk))

  ! The foreign command's text, if there is any: that line alone, shown
  ! (if it fails) with the pointer under it at the line's start, and then
  ! fcalc is done. (Not STOP, which on VMS says FORTRAN STOP.)
  call foreign_text(line, length)
  if (length > 0) then
    indent = 0
    if (length > line_length) then
      call fail(e_linelong, 0)
      call report()
    else
      call execute(line(:length))
      call report(line(:length))
    end if
    done = .true.
  end if

  do while (.not. done)
    call say_more('fcalc> ')
    call read_line(line, length, status)
    if (status < 0) then
      ! The end of the input: end the prompt's line.
      call say('')
      exit
    end if

    ! A line too long isn't echoed: VMS's FORTRAN gives none of it. A line
    ! the terminal shows itself (it echoes what is typed) goes to the log.
    echoed = echo .and. status == 0
    if (echoed) then
      call say(line(:length))
    else if (status == 0) then
      call log_line(line(:length))
    else
      call log_line('')
    end if

    if (status > 0) then
      call fail(e_linelong, 0)
    else
      call execute(line(:length))
    end if

    ! SET ECHO echoes the line that set it.
    if (echo .and. .not. echoed .and. status == 0) call say(line(:length))

    call report()
    abandon = .false.
  end do

contains

  ! EXECUTE runs one line. A line ending in ; is quiet: an expression's
  ! value, an assignment, or a definition is made but not shown, as in
  ! MATLAB and Octave (a ; in brackets separates a matrix's rows, but none
  ! ends a line).
  !
  ! EXECUTE, RUN, and RUN_FILE are RECURSIVE: a line may be @file, whose
  ! lines EXECUTE runs while the first call is still active. Without
  ! RECURSIVE, a compiler may keep a procedure's local variables in one
  ! fixed place (the VAX's FORTRAN does), which a second, inner call would
  ! overwrite.
  recursive subroutine execute(line_text)
    character(len=*), intent(in) :: line_text
    integer :: k, last

    if (len_trim(line_text) == 0) return
    k = verify(line_text, ' ' // achar(9))
    if (line_text(k:k) == '!') return

    last = len_trim(line_text)
    quiet = line_text(last:last) == ';'
    if (quiet) last = last - 1
    if (loading) quiet = .true.
    call run(line_text(:last))
  end subroutine execute

  ! RUN runs the line TEXT.
  recursive subroutine run(text)
    character(len=*), intent(in) :: text
    type(token) :: first, second

    ! The commands that name a file come first: a file's name isn't made
    ! of fcalc's tokens (it may have [ ] and ; on VMS, / elsewhere).
    if (file_command(text)) return

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

    ! SOLVE, INTEGRATE, DIFF, TABLE, and PLOT are reserved words, so they
    ! may be followed by a parenthesis: DIFF (X + 1)^2 FOR X AT 1.
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
      case ('table')
        call table_command(text)
        return
      case ('plot')
        call plot_command(text)
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

  ! FILE_COMMAND runs TEXT and is true, if it is a command that names a
  ! file: @file, RUN file, LOAD file, SAVE file, LOG file, or LOG OFF. As
  ! with the other commands, a word followed by = or ( isn't one: RUN = 3
  ! assigns a variable, and LOG(8, 2) is the logarithm.
  recursive logical function file_command(text)
    character(len=*), intent(in) :: text
    character(len=8) :: word
    integer :: at, next, k

    file_command = .false.
    at = verify(text, ' ' // achar(9))
    if (at == 0) return

    if (text(at:at) == '@') then
      word = '@'
      next = at + 1
    else
      ! The word: the letters from AT, lowercased.
      next = at
      do while (next <= len(text))
        if (index('abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ', text(next:next)) == 0) exit
        next = next + 1
      end do
      if (next - at > len(word)) return
      word = lowercase(text(at:next - 1))
      if (word /= 'run' .and. word /= 'load' .and. word /= 'save' .and. word /= 'log' .and. &
        word /= 'help') return
      ! The next character that isn't blank: = or ( makes the line no
      ! command, and so does a word's going on (RUNNING = 1).
      k = verify(text(next:), ' ' // achar(9))
      if (k > 0) then
        k = next + k - 1
        if (text(k:k) == '=' .or. text(k:k) == '(') return
        if (k == next .and. index('0123456789_$', text(k:k)) > 0) return
      end if
    end if

    file_command = .true.
    if (word == 'help') then
      call help_command(text(next:))
    else
      call file_argument(text, next, word)
    end if
  end function file_command

  ! HELP_COMMAND is HELP [topic ...]: the help library's FCALC topic, or a
  ! topic under it (HELP COMMANDS SOLVE), shown as DCL's HELP FCALC shows it
  ! (module FCPLATFORM's SHOW_HELP). HELP is with the commands that name a
  ! file because its topics needn't be fcalc's tokens (HELP @).
  subroutine help_command(topics)
    character(len=*), intent(in) :: topics

    if (.not. show_help('FCALC ' // trim(adjustl(topics)), help_line)) call fail(e_nohelp, 0)
  end subroutine help_command

  ! FILE_ARGUMENT runs the command WORD, its file named in TEXT from NEXT:
  ! one word, with nothing after it.
  recursive subroutine file_argument(text, next, word)
    character(len=*), intent(in) :: text, word
    integer, intent(in) :: next
    integer :: first, last, k

    k = verify(text(next:), ' ' // achar(9))
    if (k == 0) then
      call fail(e_filereq, len(text) + 1)
      return
    end if
    first = next + k - 1
    last = scan(text(first:), ' ' // achar(9))
    if (last == 0) then
      last = len(text)
    else
      last = first + last - 2
      k = verify(text(last + 1:), ' ' // achar(9))
      if (k > 0) then
        call fail(e_extra, last + k)
        return
      end if
    end if

    select case (word)
    case ('@', 'run')
      call run_file(trim(file_name(text(first:last))), first, .false.)
    case ('load')
      call run_file(trim(file_name(text(first:last))), first, .true.)
    case ('save')
      call save_state(trim(file_name(text(first:last))), first)
    case default
      if (lowercase(text(first:last)) == 'off') then
        call close_log(first)
      else
        call open_log(trim(file_name(text(first:last))), first)
      end if
    end select
  end subroutine file_argument

  ! RUN_FILE runs the lines of the file NAME, named at COLUMN, as if each
  ! were typed: for LOAD, the variables and functions are cleared first, and
  ! every line is quiet. A line ending in an error ends the file, and every
  ! file that ran it (ABANDON), each saying the line it was at: in the
  ! innermost, the error is reported with the line itself, which the
  ! terminal hasn't shown. QUIT or EXIT ends the file alone. Under SET ECHO
  ! each line is shown after the prompt, as a typed line is (but not while
  ! LOAD runs).
  recursive subroutine run_file(name, column, load)
    character(len=*), intent(in) :: name
    integer, intent(in) :: column
    logical, intent(in) :: load
    character(len=line_length + 1) :: text
    integer :: count, used, status
    logical :: was_loading, shown

    if (depth == max_depth) then
      call fail(e_nesting, column)
      return
    end if
    if (.not. open_input(name, depth + 1, column)) return
    depth = depth + 1

    was_loading = loading
    if (load) then
      call clear_variables()
      call clear_functions()
      call set_variable('ans', real_quantity(0.0_rk))
      loading = .true.
    end if

    count = 0
    do
      call read_line(text, used, status, depth)
      if (status < 0) exit
      count = count + 1

      shown = echo .and. .not. loading .and. status == 0
      if (shown) call say('fcalc> ' // text(:used))
      if (status > 0) then
        call fail(e_linelong, 0)
      else
        call execute(text(:used))
      end if

      if (failed()) then
        if (error_is_warning()) then
          if (shown) then
            call report()
          else
            call report(text(:used))
          end if
          cycle
        end if
        if (shown .or. status > 0) then
          call report()
        else
          call report(text(:used))
        end if
        abandon = .true.
      end if
      if (abandon) then
        call also(i_infile, trim(whole_number(count)) // ' of ' // name)
        exit
      end if
      if (done) then
        done = .false.
        exit
      end if
    end do

    call close_input(depth)
    depth = depth - 1
    loading = was_loading
  end subroutine run_file

  ! WHOLE_NUMBER is N in as few characters as it takes.
  function whole_number(n) result(text)
    integer, intent(in) :: n
    character(len=12) :: text

    write (text, '(I12)') n
    text = adjustl(text)
  end function whole_number

  ! LOWERCASE is TEXT with its capital letters made small ones, by their
  ! places in two strings (which works in any character set).
  function lowercase(text) result(small)
    character(len=*), intent(in) :: text
    character(len=len(text)) :: small
    integer :: k, at

    small = text
    do k = 1, len(text)
      at = index('ABCDEFGHIJKLMNOPQRSTUVWXYZ', text(k:k))
      if (at > 0) small(k:k) = 'abcdefghijklmnopqrstuvwxyz'(at:at)
    end do
  end function lowercase

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
      name == 'solve' .or. name == 'integrate' .or. name == 'diff' .or. &
      name == 'table' .or. name == 'plot'
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
      do k = 1, size(setting_names)
        if (setting_names(k) == 'seed' .and. .not. seed_given) cycle
        call say(trim(setting_line(setting_names(k), echo)))
      end do
    case default
      ! ANY of a comparison of a scalar with an array: is the word one of
      ! the settings? The comparison is made element by element, giving an
      ! array of LOGICALs, which ANY reduces to one.
      if (.not. any(setting_names == what%text)) then
        call fail(e_badset, what%column)
        return
      end if
      call say(trim(setting_line(what%text, echo)))
    end select
  end subroutine show_command

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
