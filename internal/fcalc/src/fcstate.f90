! fcalc: the state of a session, and files.
!
! SAVE writes the state (the settings, the variables, the functions) to a
! file of fcalc's own commands, which LOAD runs again, and a user can read and
! change in an editor:
!
!     ! fcalc state, saved 2026-11-21 10:42:17
!     set digits 12
!     set format auto
!     ...
!     r = 2.5;
!     v = [1, 2, 3];
!     f(x) = x^3 - 2*x - 5;
!
! Each line ends in ;, fcalc's quiet line, so running the file with @ shows
! nothing either. A variable's value is written with as many digits as it
! takes to read back the very same number (A.3 of the phase doc: no digit
! more), so LOAD restores it exactly, whatever SET DIGITS shows.
!
! LOG writes a transcript of the session to a file, appended to what is
! there; module FCMESSAGES' SAY writes every line to it.
!
! The files of commands that @ and LOAD run are read here, a line at a time,
! by the same nonadvancing READ as the terminal's lines. The main program
! runs each line, since only it knows the commands; it keeps a file open
! for each level of @ (a file may run another, up to MAX_DEPTH deep), each
! on a unit of its own.
!
! Fortran's I/O is by unit number: OPEN connects a unit to a file, READ and
! WRITE name the unit, and CLOSE ends the connection. The units here are
! chosen clear of the terminal's (5 and 6 by custom; * is the terminal).
! IOSTAT= on an OPEN, READ, or WRITE gives 0 on success and something else on
! failure instead of stopping the program, so fcalc reports the failure as one
! of its own messages and goes on.
module fcstate
  use fckinds
  use fcmessages
  use fcvalues, only: value, digits, number_format, number_base
  use fcarrays, only: quantity, is_array
  use fcsymbols, only: each_variable, each_function
  use fcfuncs, only: angle_mode, seed_given, seed_number
  implicit none
  private

  public :: format_names, angle_names, setting_names, setting_line
  public :: file_name, open_input, close_input, read_line, max_depth
  public :: save_state, open_log, close_log

  ! The words SET and SHOW know, each list in the order of the settings'
  ! codes (FCVALUES' F_AUTO to F_ENGINEERING, FCFUNCS' A_RADIANS to
  ! A_GRADS), so a code is a word's place. The elements of an array
  ! constructor must all have the same length, hence the blanks.
  character(len=11), parameter :: format_names(4) = (/ 'auto       ', &
    'fixed      ', 'scientific ', 'engineering' /)
  character(len=7), parameter :: angle_names(3) = (/ 'radians', 'degrees', 'grads  ' /)
  character(len=8), parameter :: setting_names(6) = (/ 'digits  ', 'format  ', &
    'angle   ', 'base    ', 'seed    ', 'echo    ' /)

  ! How deep files of commands may run one another.
  integer, parameter :: max_depth = 8

  ! The units: a file of commands at depth d is unit INPUT_UNIT + d, the log
  ! LOG_FILE, and a file being saved SAVE_FILE.
  integer, parameter :: input_unit = 20, log_file = 30, save_file = 31

  ! The longest line SAVE writes: well within the 199 characters a line
  ! may have, and easy to read in an editor.
  integer, parameter :: save_width = 100

contains

  ! SETTING_LINE is the SET command that makes the setting WHAT as it is
  ! now: set digits 12. A seed never given is a comment, which fcalc
  ! passes over if it is typed back. ECHO is the main program's setting.
  function setting_line(what, echo) result(text)
    character(len=*), intent(in) :: what
    logical, intent(in) :: echo
    character(len=40) :: text

    select case (what)
    case ('digits')
      text = 'set digits ' // whole_text(digits)
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
        text = 'set seed ' // whole_text(seed_number)
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

  ! WHOLE_TEXT is N in as few characters as it takes: 12, -3. (Fortran 95's
  ! I0 edit descriptor does this; Fortran 90 has none, so the number is
  ! written in a field wide enough for any, and ADJUSTL moves it to the
  ! left, leaving the blanks for TRIM.)
  function whole_text(n) result(text)
    integer, intent(in) :: n
    character(len=12) :: text

    write (text, '(I12)') n
    text = adjustl(text)
  end function whole_text

  ! FILE_NAME is the file TEXT names, with .fcalc after it if it has no type
  ! of its own: state is state.fcalc, notes.txt itself. The type is looked
  ! for after the directory, which ends in ] or > on VMS and / on Unix.
  ! The result's length depends on the argument's, LEN(TEXT) + 6: a
  ! function's result may have a length known only when it is called.
  function file_name(text) result(name)
    character(len=*), intent(in) :: text
    character(len=len(text) + 6) :: name
    integer :: k

    ! SCAN with BACK=.TRUE. finds the last of the characters in the set.
    k = scan(text, ']>:/', back=.true.)
    name = text
    if (index(text(k + 1:), '.') == 0) name = text // '.fcalc'
  end function file_name

  ! OPEN_INPUT opens the file NAME to read, at DEPTH (1 for the first): false
  ! after an error at COLUMN, NOFILE if there is no such file, FILELOOP if
  ! it is one already running (which would run itself forever: fcalc has no
  ! way to stop it). INQUIRE asks about a file without opening it: EXIST=
  ! says whether it is there, and OPENED= whether a unit is connected to it,
  ! which the system answers however the name is written (on VMS, x and
  ! X.FCALC;1 are one file). STATUS='OLD' opens one that must be there, and
  ! ACTION='READ' only to read it.
  logical function open_input(name, depth, column)
    character(len=*), intent(in) :: name
    integer, intent(in) :: depth, column
    logical :: there, running
    integer :: ios

    open_input = .false.
    inquire (file=name, exist=there, opened=running, iostat=ios)
    if (ios /= 0 .or. .not. there) then
      call fail(e_nofile, column, name)
      return
    end if
    if (running) then
      call fail(e_fileloop, column, name)
      return
    end if
    open (unit=input_unit + depth, file=name, status='OLD', action='READ', iostat=ios)
    if (ios /= 0) then
      call fail(e_fileerr, column, name)
      return
    end if
    open_input = .true.
  end function open_input

  ! CLOSE_INPUT closes the file of commands at DEPTH.
  subroutine close_input(depth)
    integer, intent(in) :: depth

    close (input_unit + depth)
  end subroutine close_input

  ! READ_LINE reads the next line into TEXT(:LENGTH), from the terminal, or
  ! from the file of commands at DEPTH if it is given: STATUS is 0, or 1 for
  ! a line too long (whose rest is passed over), or -1 at the end of the
  ! input. TEXT's last character is room to tell a line too long from one
  ! that just fits.
  !
  ! A nonadvancing READ (ADVANCE='NO') reads part of a record and leaves
  ! the rest for the next READ. SIZE= gives how many characters it read,
  ! so a line's length is known without counting trailing blanks; EOR=
  ! names a statement label to go to when the record (the line) ends before
  ! the variable is full, which is the usual case, and END= one for the end
  ! of the input. These labels, 10, 20, and 30, are the old-fashioned GO TO
  ! targets Fortran's I/O statements still use. A line too long for the
  ! system to read (on VMS, a terminal's line past 199 characters) is an
  ! error the READ's IOSTAT= catches, after which the next READ reads the
  ! next line.
  !
  ! The terminal is unit *, which isn't a number, so each READ is written
  ! twice, once for each.
  subroutine read_line(text, length, status, depth)
    character(len=*), intent(out) :: text
    integer, intent(out) :: length, status
    integer, intent(in), optional :: depth
    character(len=80) :: rest
    integer :: ios

    status = 0
    length = 0
    if (present(depth)) then
      read (input_unit + depth, '(A)', advance='NO', size=length, eor=10, end=30, &
        iostat=ios) text
    else
      read (*, '(A)', advance='NO', size=length, eor=10, end=30, iostat=ios) text
    end if
    status = 1
    if (ios /= 0) return

    ! The line filled TEXT without ending: pass over the rest, a piece at a
    ! time. Each piece is read into REST only to be dropped (so /WARNINGS
    ! reports REST as UNREAD): a nonadvancing READ with no item reads nothing,
    ! so it would never reach the end of the record.
    do
      if (present(depth)) then
        read (input_unit + depth, '(A)', advance='NO', eor=20, end=20, iostat=ios) rest
      else
        read (*, '(A)', advance='NO', eor=20, end=20, iostat=ios) rest
      end if
      if (ios /= 0) return
    end do
10  if (length > len(text) - 1) status = 1
20  return
30  status = -1
  end subroutine read_line

  ! NOW is the date and time, 2026-11-21 10:42:17, from the intrinsic
  ! subroutine DATE_AND_TIME, whose DATE is CCYYMMDD and TIME hhmmss.sss.
  function now() result(text)
    character(len=19) :: text
    character(len=8) :: date
    character(len=10) :: time

    call date_and_time(date, time)
    text = date(1:4) // '-' // date(5:6) // '-' // date(7:8) // ' ' // &
      time(1:2) // ':' // time(3:4) // ':' // time(5:6)
  end function now

  ! ------------------------------------------------------------------
  ! SAVE.

  ! SAVE_STATE writes the state to the file NAME, replacing it: false after
  ! an error at COLUMN. STATUS='REPLACE' makes a new file whether or not
  ! there was one (on VMS, a new version of it), and ACTION='WRITE' opens it
  ! only to write. Echo isn't saved: it is how a session is shown, not part
  ! of what it computes.
  subroutine save_state(name, column)
    character(len=*), intent(in) :: name
    integer, intent(in) :: column
    integer :: k, ios

    open (unit=save_file, file=name, status='REPLACE', action='WRITE', iostat=ios)
    if (ios /= 0) then
      call fail(e_fileerr, column, name)
      return
    end if

    write (save_file, '(2A)') '! fcalc state, saved ', now()
    do k = 1, size(setting_names) - 1
      write (save_file, '(A)') trim(setting_line(setting_names(k), .false.))
    end do

    ! SAVE_VARIABLE and SAVE_FUNCTION are passed as arguments: FCSYMBOLS
    ! calls them for each name in its trees.
    call each_variable(save_variable)
    call each_function(save_function)
    close (save_file)
  end subroutine save_state

  ! SAVE_FUNCTION writes a function's definition, f(x) = x^2;
  subroutine save_function(head, text)
    character(len=*), intent(in) :: head, text

    write (save_file, '(A)') head // ' = ' // text // ';'
  end subroutine save_function

  ! SAVE_VARIABLE writes NAME = value;. An array is a literal, [1, 2; 3, 4],
  ! if it fits on a line; a larger one is made by zeros(rows, cols) and
  ! given its elements a piece of a row at a time, m(2, 1:6) = [...], each
  ! line at most SAVE_WIDTH characters.
  subroutine save_variable(name, q)
    character(len=*), intent(in) :: name
    type(quantity), intent(in) :: q
    character(len=save_width) :: items
    character(len=60) :: item
    character(len=12) :: row, first, last
    integer :: i, j, k, room, filled

    if (.not. is_array(q)) then
      write (save_file, '(A)') name // ' = ' // trim(complex_text(q%s%z)) // ';'
      return
    end if

    ! The whole literal, if it fits: ROOM is what's left for the elements
    ! after NAME = [ and ];.
    if (literal_fits()) return

    write (save_file, '(A)') name // ' = zeros(' // trim(whole_text(q%rows)) // ', ' // &
      trim(whole_text(q%cols)) // ');'
    do i = 1, q%rows
      row = whole_text(i)
      j = 1
      do while (j <= q%cols)
        ! The piece from column J: as many elements as fit after
        ! NAME(I, J:K) = [ and before ];, with K as wide as the columns' count.
        first = whole_text(j)
        room = save_width - len(name) - len_trim(row) - len_trim(first) - &
          len_trim(whole_text(q%cols)) - 12
        items = ' '
        filled = 0
        k = j
        do while (k <= q%cols)
          item = complex_text(q%e(i, k))
          if (k > j) item = ', ' // trim(item)
          if (k > j .and. filled + len_trim(item) > room) exit
          items(filled + 1:) = item
          filled = filled + len_trim(item)
          k = k + 1
        end do
        last = whole_text(k - 1)
        write (save_file, '(A)') name // '(' // trim(row) // ', ' // trim(first) // ':' // &
          trim(last) // ') = [' // items(:filled) // '];'
        j = k
      end do
    end do

  contains

    ! LITERAL_FITS writes the array as one literal and is true, if it fits
    ! on a line. An internal procedure of a module procedure sees its
    ! host's arguments and variables, Q and ITEMS among them.
    logical function literal_fits()
      integer :: r, c, used

      literal_fits = .false.
      room = save_width - len(name) - 6
      items = ' '
      used = 0
      do r = 1, q%rows
        do c = 1, q%cols
          item = complex_text(q%e(r, c))
          if (c > 1) then
            item = ', ' // trim(item)
          else if (r > 1) then
            item = '; ' // trim(item)
          end if
          if (used + len_trim(item) > room) return
          items(used + 1:) = item
          used = used + len_trim(item)
        end do
      end do
      write (save_file, '(A)') name // ' = [' // items(:used) // '];'
      literal_fits = .true.
    end function literal_fits
  end subroutine save_variable

  ! COMPLEX_TEXT is Z as fcalc reads it: 2.5, -3i, 1 + 0.5i, 2 - 1e-20i.
  function complex_text(z) result(text)
    complex(rk), intent(in) :: z
    character(len=60) :: text
    real(rk) :: a, b

    a = real(z, rk)
    b = aimag(z)
    if (b == 0) then
      text = exact_text(a)
    else if (a == 0) then
      text = trim(exact_text(b)) // 'i'
    else if (b > 0) then
      text = trim(exact_text(a)) // ' + ' // trim(exact_text(b)) // 'i'
    else
      text = trim(exact_text(a)) // ' - ' // trim(exact_text(-b)) // 'i'
    end if
  end function complex_text

  ! EXACT_TEXT is X in the fewest significant digits that read back as X
  ! itself: 2.5, 0.1, 0.3333333333333333, 1e-20, 6.02214076e+23, the
  ! exponent written as fcalc shows one.
  !
  ! A REAL(RK) has 53 bits, about 15.95 decimal digits: 15 digits don't
  ! always tell two neighboring values apart, and 17 always do. So X is
  ! written in scientific notation with 15 digits, then 16, then 17,
  ! until reading the digits back (by the same list-directed READ that
  ! FCLEXER uses for numbers typed) gives X again. Both machines convert
  ! decimal to binary and back with correct rounding, so both write the
  ! same digits for the same value.
  !
  ! ESw.dE3 writes a number with one digit before the point, d after it, and
  ! an exponent of three digits: 3.333E-001. The format itself is made at
  ! run time by an internal WRITE, since d changes: a format may be any
  ! character expression, not only a constant.
  function exact_text(x) result(text)
    real(rk), intent(in) :: x
    character(len=30) :: text
    character(len=30) :: written
    character(len=17) :: d
    character(len=16) :: form
    real(rk) :: back
    integer :: n, ex, ios, k

    text = '0'
    if (x == 0) return

    do n = 15, 17
      write (form, '(A,I2,A,I2,A)') '(ES', n + 10, '.', n - 1, 'E3)'
      write (written, form) abs(x)
      read (written, *, iostat=ios) back
      if (ios == 0 .and. back == abs(x)) exit
    end do

    ! The digits without the point, D, and the exponent, EX: the value is
    ! D(1).D(2:) times 10**EX. Trailing zeros are dropped.
    written = adjustl(written)
    k = index(written, 'E')
    d = written(1:1) // written(3:k - 1)
    read (written(k + 1:), *) ex
    n = len_trim(d)
    do while (n > 1 .and. d(n:n) == '0')
      n = n - 1
    end do

    if (ex >= 0 .and. ex < 15) then
      ! Fixed notation, EX + 1 digits before the point: 2.5, 400.
      if (n <= ex + 1) then
        text = d(1:n) // repeat('0', ex + 1 - n)
      else
        text = d(1:ex + 1) // '.' // d(ex + 2:n)
      end if
    else if (ex < 0 .and. ex >= -5) then
      ! Fixed notation, zeros after the point: 0.001.
      text = '0.' // repeat('0', -ex - 1) // d(1:n)
    else if (n == 1) then
      text = d(1:1) // 'e' // exponent_text(ex)
    else
      text = d(1:1) // '.' // d(2:n) // 'e' // exponent_text(ex)
    end if
    if (x < 0) text = '-' // trim(text)
  end function exact_text

  ! EXPONENT_TEXT is EX with its sign: +23, -7.
  function exponent_text(ex) result(text)
    integer, intent(in) :: ex
    character(len=13) :: text

    if (ex < 0) then
      text = whole_text(ex)
    else
      text = '+' // whole_text(ex)
    end if
  end function exponent_text

  ! ------------------------------------------------------------------
  ! LOG.

  ! OPEN_LOG starts a transcript in the file NAME, closing any other first:
  ! everything SAY writes is added to it (module FCMESSAGES' LOG_UNIT),
  ! after a heading with the time. POSITION='APPEND' opens a file at its end,
  ! so a log is added to; STATUS='UNKNOWN' makes the file if there is none.
  subroutine open_log(name, column)
    character(len=*), intent(in) :: name
    integer, intent(in) :: column
    integer :: ios

    if (log_unit /= 0) call close_log(column)
    open (unit=log_file, file=name, status='UNKNOWN', action='WRITE', &
      position='APPEND', iostat=ios)
    if (ios /= 0) then
      call fail(e_fileerr, column, name)
      return
    end if
    log_unit = log_file
    write (log_unit, '(2A)') '! fcalc log, opened ', now()
  end subroutine open_log

  ! CLOSE_LOG ends the transcript: LOG OFF, a warning at COLUMN if there was
  ! none.
  subroutine close_log(column)
    integer, intent(in) :: column

    if (log_unit == 0) then
      call fail(w_nolog, column)
      return
    end if
    close (log_unit)
    log_unit = 0
  end subroutine close_log
end module fcstate
