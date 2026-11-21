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
  use fcplatform, only: cc
  use fckinds, only: name_length
  implicit none

  ! PRIVATE alone makes everything in the module private by default: a unit
  ! that USEs it sees only what a PUBLIC statement names. This keeps the
  ! module's own variables (the error waiting, below) safe from change by
  ! anything but its own procedures.
  private

  public :: say, say_more, fail, failed, clear_error, report, error_code
  public :: indent

  ! The message codes, an index into MESSAGES.
  integer, parameter, public :: e_badchar = 1, e_badnumber = 2, e_operand = 3, &
    e_rparen = 4, e_extra = 5, e_undefined = 6, e_nofunc = 7, e_constant = 8, &
    e_divzero = 9, e_overflow = 10, e_domain = 11, e_notreal = 12, &
    e_toodeep = 13, e_linelong = 14, e_namelong = 15, e_notwhole = 16, &
    e_badset = 17, w_notdef = 18, e_badcmd = 19, e_namereq = 20, &
    e_reserved = 21, e_argcount = 22, e_builtin = 23, e_recursion = 24, &
    e_params = 25, e_dupparam = 26

  ! Each message: its severity, its identifier, and its text, in which !AS
  ! stands for the text the error names (VMS's notation for "a string
  ! argument here"). TYPE ... END TYPE defines a derived type, a record of
  ! named components; a component is reached with %, as messages(k)%text.
  type message
    character(len=1) :: severity
    character(len=9) :: ident
    character(len=40) :: text
  end type message

  ! The table itself: a constant array of MESSAGEs, made by an array
  ! constructor, (/ ... /), whose elements are structure constructors,
  ! message('E', 'BADCHAR', ...), each giving the components in order. A
  ! shorter text is padded with blanks to the component's length. & at a
  ! line's end continues the statement on the next line.
  type(message), parameter :: messages(26) = (/ &
    message('E', 'BADCHAR', 'invalid character'), &
    message('E', 'BADNUMBER', 'invalid number'), &
    message('E', 'OPERAND', 'operand expected'), &
    message('E', 'RPAREN', 'missing right parenthesis'), &
    message('E', 'EXTRA', 'unexpected text'), &
    message('E', 'UNDEFINED', '!AS is not defined'), &
    message('E', 'NOFUNC', 'no function named !AS'), &
    message('E', 'CONSTANT', '!AS is a constant'), &
    message('E', 'DIVZERO', 'division by zero'), &
    message('E', 'OVERFLOW', 'result is too large'), &
    message('E', 'DOMAIN', 'argument is out of range'), &
    message('E', 'NOTREAL', 'operand must be real'), &
    message('E', 'TOODEEP', 'expression is nested too deeply'), &
    message('E', 'LINELONG', 'line is too long'), &
    message('E', 'NAMELONG', 'name is too long'), &
    message('E', 'NOTWHOLE', 'operand must be a whole number'), &
    message('E', 'BADSET', 'unknown setting'), &
    message('W', 'NOTDEF', '!AS was not defined'), &
    message('E', 'BADCMD', 'invalid command'), &
    message('E', 'NAMEREQ', 'name expected'), &
    message('E', 'RESERVED', '!AS is a reserved word'), &
    message('E', 'ARGCOUNT', 'wrong number of arguments for !AS'), &
    message('E', 'BUILTIN', '!AS is a built-in function'), &
    message('E', 'RECURSION', 'functions call each other too deeply'), &
    message('E', 'PARAMS', 'too many parameters'), &
    message('E', 'DUPPARAM', '!AS is already a parameter') /)

  ! The error not yet reported: its code (0 for none), column (0 for none),
  ! and text. SAVE keeps a module variable's value from one call to the
  ! next (and = 0 gives its value when the program starts); without SAVE, a
  ! module variable may lose its value when no active unit uses the module.
  integer, save :: code = 0, column = 0
  character(len=name_length), save :: argument = ' '

  ! How far right of the line's start the typed text begins: the prompt's
  ! width, so the pointer falls under the column.
  integer, save :: indent = 7

  ! The longest record SAY writes.
  integer, parameter :: width = 132

  ! CONTAINS separates the module's declarations from its procedures, the
  ! module procedures, which see everything declared above it.
contains

  ! SAY writes one line, in records of at most WIDTH characters: VMS's
  ! terminal records end at 133 bytes, the carriage control among them, and
  ! a longer one is an error. So a longer line goes on in the next record,
  ! on every system alike.
  subroutine say(text)
    character(len=*), intent(in) :: text
    integer :: at

    ! WRITE (*, format) writes to the terminal; the format '(2A)' says two
    ! character items, each written as it is. TEXT(i:j) is a substring:
    ! the characters from i to j. A DO with no control loops until an EXIT.
    at = 1
    do
      write (*, '(2A)') cc, text(at:min(len(text), at + width - 1))
      at = at + width
      if (at > len(text)) exit
    end do
  end subroutine say

  ! SAY_MORE writes text and leaves the line open, for a prompt: ADVANCE='NO'
  ! is Fortran 90's nonadvancing output, which writes a record without
  ! ending it, so the next output or input goes on the same line.
  subroutine say_more(text)
    character(len=*), intent(in) :: text

    write (*, '(2A)', advance='NO') cc, text
  end subroutine say_more

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

  logical function failed()
    failed = code /= 0
  end function failed

  integer function error_code()
    error_code = code
  end function error_code

  subroutine clear_error()
    code = 0
    column = 0
    argument = ' '
  end subroutine clear_error

  ! REPORT writes the waiting error, with a pointer to its column, and
  ! clears it.
  subroutine report()
    character(len=80) :: text
    integer :: at

    if (code == 0) return

    ! REPEAT(' ', n) is n blanks, and // joins strings: the ^ goes under
    ! the column, past the prompt.
    if (column > 0) call say(repeat(' ', indent + column - 1) // '^')

    ! INDEX finds where one string starts in another (0 if nowhere); the
    ! !AS is replaced by the text the error names. TRIM drops trailing
    ! blanks, which a fixed-length CHARACTER variable is padded with.
    text = messages(code)%text
    at = index(text, '!AS')
    if (at > 0) text = text(:at - 1) // trim(argument) // text(at + 3:)

    call say('%FCALC-' // messages(code)%severity // '-' // &
      trim(messages(code)%ident) // ', ' // trim(text))
    call clear_error()
  end subroutine report
end module fcmessages
