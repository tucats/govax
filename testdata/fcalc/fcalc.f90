! fcalc, a scientific calculator: the main program.
!
! fcalc reads lines at the prompt until QUIT, EXIT, or the end of the
! input. A line that starts with a command word is a command; NAME = EXPR
! assigns a variable; anything else is an expression, whose value is shown
! and kept as ANS. A line starting with ! is a comment.
program fcalc
  use fckinds
  use fcmessages
  use fcvalues
  use fclexer
  use fctree
  use fceval
  use fcsymbols
  implicit none

  character(len=line_length + 1) :: line
  integer :: length, status
  logical :: echo, done, echoed

  echo = .false.
  done = .false.
  call start_symbols()
  call set_variable('ans', real_value(0.0_rk))

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
  ! over), or -1 at the end of the input. A line too long for the system
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

  ! EXECUTE runs one line.
  subroutine execute(text)
    character(len=*), intent(in) :: text
    type(token) :: first, second
    integer :: k

    if (len_trim(text) == 0) return
    k = verify(text, ' ' // achar(9))
    if (text(k:k) == '!') return

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
      end select
    end if

    if (first%kind == t_name .and. second%kind == t_operator .and. second%text == '=') then
      call assignment(first)
    else
      call expression()
    end if
  end subroutine execute

  ! EXPRESSION shows an expression's value and keeps it as ANS.
  subroutine expression()
    type(value) :: v

    if (.not. whole_expression(v)) return
    call set_variable('ans', v)
    call say(trim(format_value(v)))
  end subroutine expression

  ! ASSIGNMENT is NAME = EXPR.
  subroutine assignment(name)
    type(token), intent(in) :: name
    type(value) :: v

    if (is_reserved(name%text)) then
      call fail(e_reserved, name%column, name%text)
      return
    end if
    if (is_constant(name%text)) then
      call fail(e_constant, name%column, name%text)
      return
    end if

    call advance(2)
    if (.not. whole_expression(v)) return
    call set_variable(name%text, v)
    call say(trim(name%text) // ' = ' // trim(format_value(v)))
  end subroutine assignment

  ! WHOLE_EXPRESSION parses and evaluates the rest of the line as an
  ! expression: false after an error.
  logical function whole_expression(v)
    type(value), intent(out) :: v
    type(node), pointer :: tree

    whole_expression = .false.
    v = real_value(0.0_rk)
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
  end function whole_expression

  logical function is_reserved(name)
    character(len=*), intent(in) :: name

    is_reserved = name == 'or' .or. name == 'and' .or. name == 'not' .or. name == 'mod'
  end function is_reserved

  ! LIST [VARIABLES | ALL]
  subroutine list_command()
    type(token) :: what

    what = token_at(2)
    if (what%kind == t_name) then
      if (what%text /= 'variables' .and. what%text /= 'all') then
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
    call list_variables()
  end subroutine list_command

  ! CLEAR name... | CLEAR ALL
  subroutine clear_command()
    type(token) :: what
    integer :: k

    what = token_at(2)
    if (what%kind == t_name .and. what%text == 'all' .and. kind_at(3) == t_end) then
      call clear_variables()
      call set_variable('ans', real_value(0.0_rk))
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
    do k = 2, token_count() - 1
      what = token_at(k)
      if (.not. remove_variable(what%text)) then
        call fail(w_notdef, what%column, what%text)
        call report()
      end if
    end do
  end subroutine clear_command

  ! SET [NO]ECHO
  subroutine set_command()
    type(token) :: what

    what = token_at(2)
    if (what%kind /= t_name .or. kind_at(3) /= t_end) then
      call fail(e_badset, what%column)
      return
    end if
    select case (what%text)
    case ('echo')
      echo = .true.
    case ('noecho')
      echo = .false.
    case default
      call fail(e_badset, what%column)
    end select
  end subroutine set_command
end program fcalc
