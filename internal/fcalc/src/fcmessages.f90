! fcalc: output and messages.
!
! Every line fcalc writes goes through SAY, so the carriage control the
! system wants is in one place (module FCPLATFORM). An error is a code, the
! column of the line it is about, and a text it names (a variable's name),
! kept here until it is reported: the parser and the evaluator stop at the
! first, and the main program reports it in VMS's form,
!
!     fcalc> 1/0
!             ^
!     %FCALC-E-DIVZERO, division by zero
!
! the pointer under the column of the line the user typed after the prompt.
!
! Why keep an error instead of printing it at once? The parser and the
! evaluator are deep in recursive calls when they find one; recording it
! and returning lets every level unwind normally, freeing what it built,
! and the main program prints the message once everything is tidy. (Fortran
! 90 has no exceptions: a program carries its errors back itself.)
module fcmessages
  ! USE ... ONLY takes just the names listed, so a reader sees at once
  ! what this module needs from each other one.
  use fcplatform, only: cc, record_room, prompt_ends
  use fckinds, only: line_length
  implicit none

  ! PRIVATE alone makes everything in the module private by default: a unit
  ! that USEs it sees only what a PUBLIC statement names. This keeps the
  ! module's own variables (the error waiting, below) safe from change by
  ! anything but its own procedures.
  private

  public :: say, say_more, fail, failed, clear_error, report, error_code
  public :: indent, say_part, say_end, error_ident, error_is_warning
  public :: also, log_unit, log_line, help_line

  ! The message codes, an index into MESSAGES.
  integer, parameter, public :: e_badchar = 1, e_badnumber = 2, e_operand = 3, &
    e_rparen = 4, e_extra = 5, e_undefined = 6, e_nofunc = 7, e_constant = 8, &
    e_divzero = 9, e_overflow = 10, e_domain = 11, e_notreal = 12, &
    e_toodeep = 13, e_linelong = 14, e_namelong = 15, e_notwhole = 16, &
    e_badset = 17, w_notdef = 18, e_badcmd = 19, e_namereq = 20, &
    e_reserved = 21, e_argcount = 22, e_builtin = 23, e_recursion = 24, &
    e_params = 25, e_dupparam = 26, e_notconform = 27, e_notsquare = 28, &
    e_singular = 29, e_index = 30, e_notscalar = 31, e_noconverge = 32, &
    e_badrange = 33, e_toobig = 34, e_rbracket = 35, e_notvector = 36, &
    e_keyword = 37, e_noroot = 38, e_samesign = 39, w_inaccurate = 40, &
    e_badstep = 41, e_toomany = 42, e_badlimits = 43, e_nopoints = 44, &
    e_filereq = 45, e_nofile = 46, e_fileerr = 47, e_nesting = 48, &
    e_fileloop = 49, w_nolog = 50, i_infile = 51, e_nohelp = 52

  ! Each message: its severity, its identifier, and its text, in which !AS
  ! stands for the text the error names (VMS's notation for "a string
  ! argument here"). TYPE ... END TYPE defines a derived type, a record of
  ! named components; a component is reached with %, as messages(k)%text.
  type message
    character(len=1) :: severity
    character(len=10) :: ident
    character(len=40) :: text
  end type message

  ! The table itself: a constant array of MESSAGEs, made by an array
  ! constructor, (/ ... /), whose elements are structure constructors,
  ! message('E', 'BADCHAR', ...), each giving the components in order. A
  ! shorter text is padded with blanks to the component's length. & at a
  ! line's end continues the statement on the next line; a statement may
  ! have at most 39 continuation lines, so some lines hold two.
  type(message), parameter :: messages(52) = (/ &
    message('E', 'BADCHAR', 'invalid character'), message('E', 'BADNUMBER', 'invalid number'), &
    message('E', 'OPERAND', 'operand expected'), message('E', 'RPAREN', 'missing right parenthesis'), &
    message('E', 'EXTRA', 'unexpected text'), message('E', 'UNDEFINED', '!AS is not defined'), &
    message('E', 'NOFUNC', 'no function named !AS'), &
    message('E', 'CONSTANT', '!AS is a constant'), &
    message('E', 'DIVZERO', 'division by zero'), message('E', 'OVERFLOW', 'result is too large'), &
    message('E', 'DOMAIN', 'argument is out of range'), message('E', 'NOTREAL', 'operand must be real'), &
    message('E', 'TOODEEP', 'expression is nested too deeply'), &
    message('E', 'LINELONG', 'line is too long'), message('E', 'NAMELONG', 'name is too long'), &
    message('E', 'NOTWHOLE', 'operand must be a whole number'), &
    message('E', 'BADSET', 'unknown setting'), message('W', 'NOTDEF', '!AS was not defined'), &
    message('E', 'BADCMD', 'invalid command'), &
    message('E', 'NAMEREQ', 'name expected'), message('E', 'RESERVED', '!AS is a reserved word'), &
    message('E', 'ARGCOUNT', 'wrong number of arguments for !AS'), &
    message('E', 'BUILTIN', '!AS is a built-in function'), &
    message('E', 'RECURSION', 'functions call each other too deeply'), &
    message('E', 'PARAMS', 'too many parameters'), &
    message('E', 'DUPPARAM', '!AS is already a parameter'), &
    message('E', 'NOTCONFORM', 'shapes of the operands do not agree'), &
    message('E', 'NOTSQUARE', 'matrix must be square'), &
    message('E', 'SINGULAR', 'matrix is singular'), &
    message('E', 'INDEX', 'index is out of range'), &
    message('E', 'NOTSCALAR', 'operand must be a scalar'), &
    message('E', 'NOCONVERGE', 'eigenvalues did not converge'), &
    message('E', 'BADRANGE', 'a range is allowed only in an index'), &
    message('E', 'TOOBIG', 'array is too large'), &
    message('E', 'RBRACKET', 'missing right bracket'), &
    message('E', 'NOTVECTOR', 'operand must be a vector'), &
    message('E', 'KEYWORD', '!AS expected'), message('E', 'NOROOT', 'no root was found'), &
    message('E', 'SAMESIGN', 'no change of sign between the bounds'), &
    message('W', 'INACCURATE', 'error may be as large as !AS'), &
    message('E', 'BADSTEP', 'step does not lead from FROM to TO'), message('E', 'TOOMANY', 'too many !AS'), &
    message('E', 'BADLIMITS', 'lower limit must be below upper limit'), &
    message('E', 'NOPOINTS', 'no point can be plotted'), &
    message('E', 'FILEREQ', 'file name expected'), message('E', 'NOFILE', 'file !AS was not found'), &
    message('E', 'FILEERR', 'file !AS could not be used'), &
    message('E', 'NESTING', 'files are nested too deeply'), message('E', 'FILELOOP', 'file !AS is already running'), &
    message('W', 'NOLOG', 'no log file is open'), message('I', 'INFILE', 'in line !AS'), &
    message('E', 'NOHELP', 'no help is available') /)

  ! The module's variables are declared INTEGERs first, CHARACTERs last. A
  ! compiler lays a module's variables out in the order declared, and a
  ! CHARACTER variable's length is any number of bytes, so an INTEGER after
  ! one may fall off its natural (longword) boundary, which a VAX reaches
  ! more slowly: /WARNINGS=ALIGNMENT reports it as %FORT-I-ALIGNCOMMON.

  ! The error not yet reported: its code (0 for none), column (0 for none),
  ! and text, ARGUMENT, below (a name, or a file's: as long as a line). SAVE
  ! keeps a module variable's value from one call to the next (and = 0 gives
  ! its value when the program starts); without SAVE, a module variable may
  ! lose its value when no active unit uses the module.
  integer, save :: code = 0, column = 0

  ! How far right of the line's start the typed text begins: the prompt's
  ! width, so the pointer falls under the column.
  integer, save :: indent = 7

  ! The longest record SAY writes.
  integer, parameter :: width = 132

  ! The unit of the log file LOG opened (module FCSTATE), or 0 for none:
  ! everything SAY and SAY_MORE write to the terminal is written there too,
  ! without the carriage control, which a file's records don't have.
  integer, save :: log_unit = 0

  ! A line SAY_PART is building, PENDING, and how much of it is filled.
  integer, save :: filled = 0

  ! The length of the prompt SAY_MORE left open in its record, or 0 when
  ! there is none.
  integer, save :: open_prompt = 0

  character(len=line_length), save :: argument = ' '
  character(len=width), save :: pending = ' '

  ! CONTAINS separates the module's declarations from its procedures, the
  ! module procedures, which see everything declared above it.
contains

  ! SAY writes one line, in records of at most WIDTH characters: VMS's
  ! terminal records end at 133 bytes, the carriage control among them, and
  ! a longer one is an error. So a longer line goes on in the next record,
  ! on every system alike.
  !
  ! After a prompt, the line may go on in the prompt's record (PROMPT_ENDS
  ! says when: reading a file, not a terminal). Then it has no carriage
  ! control of its own, since its record already has one, and its first
  ! piece is only as long as the record has room for after the prompt. The
  ! log's records are always pieces of WIDTH, so the log is the same however
  ! fcalc read its lines.
  subroutine say(text)
    character(len=*), intent(in) :: text
    integer :: at, last, room
    logical :: going_on

    ! WRITE (*, format) writes to the terminal; the format '(2A)' says two
    ! character items, each written as it is. TEXT(i:j) is a substring:
    ! the characters from i to j. A DO with no control loops until an EXIT.
    going_on = open_prompt > 0
    if (going_on) going_on = .not. prompt_ends()
    room = width
    if (going_on) room = min(width, record_room - open_prompt)
    open_prompt = 0

    at = 1
    do
      last = min(len(text), at + room - 1)
      if (going_on) then
        write (*, '(A)') text(at:last)
      else
        write (*, '(2A)') cc, text(at:last)
      end if
      going_on = .false.
      room = width
      at = last + 1
      if (at > len(text)) exit
    end do

    at = 1
    do
      call log_line(text(at:min(len(text), at + width - 1)))
      at = at + width
      if (at > len(text)) exit
    end do
  end subroutine say

  ! LOG_LINE writes TEXT to the log, if one is open, ending its record:
  ! a line the terminal showed and the log must have too (one typed after
  ! the prompt, which the terminal echoes itself).
  subroutine log_line(text)
    character(len=*), intent(in) :: text

    if (log_unit /= 0) write (log_unit, '(A)') text
  end subroutine log_line

  ! SAY_MORE writes text and leaves the line open, for a prompt: ADVANCE='NO'
  ! is Fortran 90's nonadvancing output, which writes a record without
  ! ending it, so the next output or input goes on the same line.
  subroutine say_more(text)
    character(len=*), intent(in) :: text

    write (*, '(2A)', advance='NO') cc, text
    if (log_unit /= 0) write (log_unit, '(A)', advance='NO') text
    open_prompt = len(text)
  end subroutine say_more

  ! SAY_PART adds text to a line being built a piece at a time, and SAY_END
  ! writes what is left of it: together they write the same records SAY
  ! would of the whole line, for a line too long to hold in one variable
  ! (LIST's of a large matrix). Fortran 90 has no strings that grow, so a
  ! long line is written as it is made: each time PENDING is full and more
  ! comes, PENDING is written as a record.
  subroutine say_part(text)
    character(len=*), intent(in) :: text
    integer :: at, k

    at = 1
    do while (at <= len(text))
      if (filled == width) then
        call say(pending)
        filled = 0
      end if
      k = min(len(text) - at + 1, width - filled)
      pending(filled + 1:filled + k) = text(at:at + k - 1)
      filled = filled + k
      at = at + k
    end do
  end subroutine say_part

  subroutine say_end()
    call say(pending(:filled))
    filled = 0
  end subroutine say_end

  ! FAIL records an error, unless one is already waiting: the first is the
  ! one reported.
  !
  ! INTENT(IN) promises the routine won't change an argument (the compiler
  ! holds it to that); INTENT(OUT) that it will give it a value, and
  ! INTENT(INOUT) both. CHARACTER(LEN=*) takes its length from whatever
  ! string the caller passes. OPTIONAL means a caller may leave TEXT out;
  ! PRESENT(TEXT) tells whether it did, and an absent argument must not be
  ! touched. A procedure with an OPTIONAL argument needs an explicit
  ! interface where it is called, which a module procedure always has.
  subroutine fail(what, where, text)
    integer, intent(in) :: what, where
    character(len=*), intent(in), optional :: text

    if (code /= 0) return
    code = what
    column = where
    argument = ' '
    if (present(text)) argument = text
  end subroutine fail

  ! ERROR_IDENT is the identifier of the message CODE, DIVZERO: TABLE shows
  ! it in place of a value it couldn't compute.
  function error_ident(what) result(ident)
    integer, intent(in) :: what
    character(len=10) :: ident

    ident = messages(what)%ident
  end function error_ident

  logical function failed()
    failed = code /= 0
  end function failed

  ! ERROR_IS_WARNING: the error waiting is only a warning (or a note), after
  ! which a file of commands goes on.
  logical function error_is_warning()
    error_is_warning = .false.
    if (code /= 0) error_is_warning = messages(code)%severity /= 'E'
  end function error_is_warning

  integer function error_code()
    error_code = code
  end function error_code

  subroutine clear_error()
    code = 0
    column = 0
    argument = ' '
  end subroutine clear_error

  ! REPORT writes the waiting error, with a pointer to its column, and
  ! clears it. SHOWN, if given, is the line the error is about, which the
  ! terminal doesn't show (a line of a file, or the foreign command's): it
  ! is written first, indented as a typed line is, for the pointer.
  subroutine report(shown)
    character(len=*), intent(in), optional :: shown

    if (code == 0) return

    ! REPEAT(' ', n) is n blanks, and // joins strings: the ^ goes under
    ! the column, past the prompt.
    if (column > 0) then
      if (present(shown)) call say(repeat(' ', indent) // shown)
      call say(repeat(' ', indent + column - 1) // '^')
    end if

    call say('%' // trim(message_text(code, argument)))
    call clear_error()
  end subroutine report

  ! ALSO writes a message that goes on from the one before, with - in place
  ! of %, as VMS writes the second and later messages of one condition:
  !     -FCALC-I-INFILE, in line 3 of setup.fcalc
  subroutine also(what, text)
    integer, intent(in) :: what
    character(len=*), intent(in) :: text

    call say('-' // trim(message_text(what, text)))
  end subroutine also

  ! HELP_LINE writes one line of help text (module FCPLATFORM's SHOW_HELP
  ! passes it to the librarian, which calls it for each line), and gives a
  ! VMS status, 1 for success: the librarian stops at a failure. A function
  ! passed as an argument must be a module procedure or an external one,
  ! never an internal procedure, which Fortran 90 doesn't allow there.
  integer function help_line(text)
    character(len=*), intent(in) :: text

    call say(text)
    help_line = 1
  end function help_line

  ! MESSAGE_TEXT is the message WHAT naming TEXT, without its first
  ! character: FCALC-E-NOFILE, file x.fcalc was not found.
  function message_text(what, text) result(line)
    integer, intent(in) :: what
    character(len=*), intent(in) :: text
    character(len=line_length + 60) :: line
    integer :: at

    ! INDEX finds where one string starts in another (0 if nowhere); the
    ! !AS is replaced by the text the error names. TRIM drops trailing
    ! blanks, which a fixed-length CHARACTER variable is padded with.
    line = messages(what)%text
    at = index(line, '!AS')
    if (at > 0) line = line(:at - 1) // trim(text) // line(at + 3:)
    line = 'FCALC-' // messages(what)%severity // '-' // &
      trim(messages(what)%ident) // ', ' // trim(line)
  end function message_text
end module fcmessages
