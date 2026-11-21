! fcalc: the lexer, which turns a line into tokens.
!
! A token is a number (its value, and whether it was written with an i, so
! imaginary), a name (in lowercase: fcalc's names are case-insensitive, as
! DCL's are), an operator, or the end of the line. Each knows the column it
! started in, for an error's pointer.
!
! A lexer (or scanner) is the first stage of reading a language: it groups
! characters into tokens, so the parser (module FCTREE) deals in "the
! number 2.5" and "the operator <=" rather than in characters. Fortran 90's
! character intrinsics do most of the work here: INDEX(set, c) > 0 asks
! whether the character C is in the string SET, VERIFY(text, set) finds the
! first character of TEXT not in SET (0 if all are), and IACHAR and ACHAR
! convert between a character and its ASCII code.
module fclexer
  use fckinds
  use fcmessages
  implicit none
  private

  public :: token, tokenize, t_end, t_number, t_name, t_operator

  integer, parameter :: t_end = 0, t_number = 1, t_name = 2, t_operator = 3

  type token
    integer :: kind
    integer :: column
    character(len=name_length) :: text
    real(rk) :: number
    logical :: imaginary
  end type token

  ! The characters, by kind.
  character(len=*), parameter :: letters = &
    'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
  character(len=*), parameter :: digit_set = '0123456789'
  character(len=*), parameter :: name_set = letters // digit_set // '_$'
  character(len=*), parameter :: blanks = ' ' // achar(9)

  ! The operators of two characters, then those of one. The element by
  ! element operators start with a period (.* ./ .^), and .' is the
  ! transpose; ' alone is the conjugate transpose, and \ the solve.
  ! "'" is a character constant of one apostrophe (Fortran 90 lets a
  ! character constant be written in either kind of quotes), and ACHAR(92)
  ! is a backslash, written so because some compilers take a backslash in
  ! a character constant as the start of an escape, as C does.
  character(len=2), parameter :: pairs(9) = (/ '==', '<>', '<=', '>=', '**', &
    '.*', './', '.^', ".'" /)
  character(len=*), parameter :: singles = '+-*/^!(),=<>[];:' // "'" // achar(92)

contains

  ! TOKENIZE turns LINE into TOKENS(1:COUNT), the last of kind T_END. On an
  ! error it calls FAIL and stops.
  !
  ! TOKENS(:) is an assumed-shape array: the dummy takes its size (and
  ! shape) from the array the caller passes, which SIZE(TOKENS) would give.
  ! Like an OPTIONAL argument it needs an explicit interface at the call,
  ! which a module procedure has.
  subroutine tokenize(line, tokens, count)
    character(len=*), intent(in) :: line
    type(token), intent(out) :: tokens(:)
    integer, intent(out) :: count
    integer :: at, last, k

    count = 0
    at = 1
    last = len_trim(line)

    do
      ! Pass over blanks and tabs.
      if (at <= last) then
        k = verify(line(at:last), blanks)
        if (k == 0) then
          at = last + 1
        else
          at = at + k - 1
        end if
      end if

      count = count + 1
      tokens(count)%column = at
      tokens(count)%text = ' '
      tokens(count)%number = 0
      tokens(count)%imaginary = .false.

      if (at > last) then
        tokens(count)%kind = t_end
        return
      end if

      if (index(digit_set, line(at:at)) > 0 .or. &
          (line(at:at) == '.' .and. next_is_digit(line, at + 1))) then
        call number_token(line, at, tokens(count))
      else if (index(letters, line(at:at)) > 0) then
        call name_token(line, at, tokens(count))
      else
        call operator_token(line, at, tokens(count))
      end if
      if (failed()) return
    end do
  end subroutine tokenize

  logical function next_is_digit(line, at)
    character(len=*), intent(in) :: line
    integer, intent(in) :: at

    next_is_digit = .false.
    if (at <= len(line)) next_is_digit = index(digit_set, line(at:at)) > 0
  end function next_is_digit

  ! NAME_TOKEN takes a name: a letter, then letters, digits, _, and $. It is
  ! lowercased a character at a time by its ASCII code: in ASCII the capital
  ! letters are 65 to 90 and the small ones 97 to 122, the same distance
  ! apart for every letter.
  subroutine name_token(line, at, t)
    character(len=*), intent(in) :: line
    integer, intent(inout) :: at
    type(token), intent(inout) :: t
    integer :: k, length, c

    k = verify(line(at:), name_set)
    if (k == 0) then
      length = len(line) - at + 1
    else
      length = k - 1
    end if
    if (length > name_length) then
      call fail(e_namelong, at)
      return
    end if

    t%kind = t_name
    t%text = line(at:at + length - 1)
    do k = 1, length
      c = iachar(t%text(k:k))
      if (c >= iachar('A') .and. c <= iachar('Z')) &
        t%text(k:k) = achar(c - iachar('A') + iachar('a'))
    end do
    at = at + length
  end subroutine name_token

  ! NUMBER_TOKEN takes a number: digits with a point, or a point and
  ! digits; an exponent, E and digits with a sign or none; and an i after
  ! it all for an imaginary one. Its value is read by an internal READ,
  ! after its size is checked, so a number no machine could hold is an
  ! error and not a trap.
  !
  ! The size check counts LEAD, the power of ten of the number's first
  ! digit that isn't zero (2 for 123.4, -3 for 0.00123), and adds the
  ! exponent: the number is about 10**(LEAD + POWER), known before the
  ! READ converts it. READ (text, *) is list-directed: it reads a value in
  ! any form Fortran writes constants, and IOSTAT= gives a nonzero status
  ! instead of stopping the program if it can't.
  subroutine number_token(line, at, t)
    character(len=*), intent(in) :: line
    integer, intent(inout) :: at
    type(token), intent(inout) :: t
    integer :: first, k, lead, power, ios, c
    logical :: point, seen

    if (based_number(line, at, t)) return

    first = at
    point = .false.
    seen = .false.

    ! LEAD is the power of ten of the first digit that isn't zero, counted
    ! as the digits are read.
    lead = 0
    do while (at <= len(line))
      c = index(digit_set, line(at:at)) - 1
      if (c >= 0) then
        if (c > 0 .and. .not. seen) then
          seen = .true.
          if (point) lead = lead - 1
        else if (seen .and. .not. point) then
          lead = lead + 1
        else if (.not. seen .and. point) then
          lead = lead - 1
        end if
      else if (line(at:at) == '.' .and. .not. point) then
        point = .true.
      else
        exit
      end if
      at = at + 1
    end do

    ! An exponent, if the E is followed by digits.
    power = 0
    if (at <= len(line)) then
      if (line(at:at) == 'e' .or. line(at:at) == 'E') then
        k = at + 1
        if (k <= len(line)) then
          if (line(k:k) == '+' .or. line(k:k) == '-') k = k + 1
        end if
        if (next_is_digit(line, k)) then
          do while (next_is_digit(line, k))
            if (power < 100000) power = power * 10 + index(digit_set, line(k:k)) - 1
            k = k + 1
          end do
          if (line(at + 1:at + 1) == '-') power = -power
          at = k
        end if
      end if
    end if

    t%kind = t_number
    t%text = ' '
    if (seen) then
      if (lead + power > 300) then
        call fail(e_overflow, first)
        return
      end if
      if (lead + power >= -301) then
        read (line(first:at - 1), *, iostat=ios) t%number
        if (ios /= 0) then
          call fail(e_badnumber, first)
          return
        end if
        if (abs(t%number) > limit) then
          call fail(e_overflow, first)
          return
        end if
        if (abs(t%number) < small) t%number = 0
      end if
    end if

    ! An i makes the number imaginary, unless it starts a name.
    if (at <= len(line)) then
      if (line(at:at) == 'i' .or. line(at:at) == 'I') then
        k = at + 1
        if (k > len(line)) then
          t%imaginary = .true.
        else if (index(name_set, line(k:k)) == 0) then
          t%imaginary = .true.
        end if
        if (t%imaginary) at = at + 1
      end if
    end if
  end subroutine number_token

  ! BASED_NUMBER takes a whole number in another base, if LINE(AT:) starts
  ! with one: 0x and hexadecimal digits, 0o and octal ones, or 0b and binary
  ! ones (the prefix's letter in either case, as the digits A to F are), as
  ! fcalc shows whole numbers under SET BASE. An i after it makes it
  ! imaginary, as after any number. The value is added up a digit at a time
  ! in REAL(RK), exact for whole numbers below WHOLE_LIMIT; a larger one is
  ! an overflow, and a letter or digit run on after the digits (0b102) makes
  ! the number invalid. The result is false, and nothing taken, if LINE(AT:)
  ! isn't such a number: 0 followed by anything else is an ordinary number.
  logical function based_number(line, at, t)
    character(len=*), intent(in) :: line
    integer, intent(inout) :: at
    type(token), intent(inout) :: t
    character(len=*), parameter :: hex = '0123456789abcdef'
    integer :: rbase, k, first, c
    real(rk) :: x

    based_number = .false.
    if (at + 2 > len(line)) return
    if (line(at:at) /= '0') return
    select case (line(at + 1:at + 1))
    case ('x', 'X')
      rbase = 16
    case ('o', 'O')
      rbase = 8
    case ('b', 'B')
      rbase = 2
    case default
      return
    end select
    if (digit_of(line(at + 2:at + 2)) < 0) return

    based_number = .true.
    first = at
    t%kind = t_number
    x = 0
    k = at + 2
    do while (k <= len(line))
      c = digit_of(line(k:k))
      if (c < 0) exit
      ! The test before the product: X * RBASE + C stays below WHOLE_LIMIT.
      if (x >= (whole_limit - c) / rbase) then
        call fail(e_overflow, first)
        return
      end if
      x = x * rbase + c
      k = k + 1
    end do
    at = k
    t%number = x

    ! An i makes it imaginary, unless more of a name follows; anything else
    ! of a name's characters run on is an error.
    if (at <= len(line)) then
      if (line(at:at) == 'i' .or. line(at:at) == 'I') then
        if (at == len(line)) then
          t%imaginary = .true.
        else if (index(name_set, line(at + 1:at + 1)) == 0) then
          t%imaginary = .true.
        end if
        if (t%imaginary) at = at + 1
      end if
    end if
    if (at <= len(line)) then
      if (index(name_set, line(at:at)) > 0) call fail(e_badnumber, first)
    end if

  contains

    ! DIGIT_OF is the digit CH stands for in RBASE, the base, or -1 if it isn't one.
    ! A capital letter is lowered first, by its ASCII code, as NAME_TOKEN
    ! lowers a name.
    integer function digit_of(ch)
      character(len=1), intent(in) :: ch
      character(len=1) :: low
      integer :: code

      low = ch
      code = iachar(ch)
      if (code >= iachar('A') .and. code <= iachar('Z')) &
        low = achar(code - iachar('A') + iachar('a'))
      digit_of = index(hex, low) - 1
      if (digit_of >= rbase) digit_of = -1
    end function digit_of
  end function based_number

  ! OPERATOR_TOKEN takes an operator, ** standing for ^.
  subroutine operator_token(line, at, t)
    character(len=*), intent(in) :: line
    integer, intent(inout) :: at
    type(token), intent(inout) :: t
    integer :: k

    t%kind = t_operator
    if (at < len(line)) then
      do k = 1, size(pairs)
        if (line(at:at + 1) == pairs(k)) then
          t%text = pairs(k)
          if (t%text == '**') t%text = '^'
          at = at + 2
          return
        end if
      end do
    end if

    if (index(singles, line(at:at)) == 0) then
      call fail(e_badchar, at)
      return
    end if
    t%text = line(at:at)
    at = at + 1
  end subroutine operator_token
end module fclexer
