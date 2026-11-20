! pidigits.f90: pi to as many decimal places as asked, by Machin's formula,
!
!     pi = 16 atan(1/5) - 4 atan(1/239),
!
! in fixed-point arithmetic on arrays of base-10000 words. The number of
! places is the text after the image's name, read by LIB$GET_FOREIGN, which
! only a foreign command or MCR gives a program:
!
!     $ PIDIGITS :== $dev:[dir]PIDIGITS
!     $ PIDIGITS 1000
!     $ MCR dev:[dir]PIDIGITS 1000
!
! With no text, as under RUN, it is 50; the limit is 100,000. The digits are printed in groups of ten, five groups
! to a line, the first line starting with "3." and the others lined up under
! it. The places printed are pi's own, cut off, not rounded.
!
! Each number is an allocatable array X(0:NW): X(0) the whole part, X(1) the
! first four places, and so on. Three words past the places asked for keep
! the error of the series' many divisions out of the places printed.
program pidigits
  implicit none
  integer, parameter :: base = 10000, guard = 3, maxplaces = 100000
  integer, external :: lib$get_foreign
  character(len=80) :: line
  integer :: status, length, places, nw, ios
  integer, allocatable :: pi(:), part(:), term(:), piece(:)

  ! The number of places: the foreign command's text, or 50.
  places = 50
  line = ' '
  length = 0
  status = lib$get_foreign(line, , length)
  if (mod(status, 2) == 1 .and. length > 0) then
    read (line(1:length), *, iostat=ios) places
    if (ios /= 0 .or. places < 1 .or. places > maxplaces) then
      write (*, '(1x, 4a, i7)') 'Not a number of places from 1 to the', &
        ' limit: "', line(1:length), '"; the limit is', maxplaces
      stop
    end if
  end if

  nw = (places + 3) / 4 + guard
  allocate (pi(0:nw), part(0:nw), term(0:nw), piece(0:nw))

  call arctan(5, 16, pi)
  call arctan(239, 4, part)
  call subtract(pi, part)

  call show(pi, places)
  deallocate (pi, part, term, piece)

contains

  ! X = factor * atan(1/m), by the series
  !     atan(1/m) = 1/m - 1/(3 m**3) + 1/(5 m**5) - ...,
  ! TERM holding factor / m**(2k+1), each term of the sum TERM / (2k+1)
  ! (made in PIECE; TERM and PIECE are the host's).
  subroutine arctan(m, factor, x)
    integer, intent(in) :: m, factor
    integer, intent(out) :: x(0:)
    integer :: k

    term = 0
    term(0) = factor
    call divide(term, m)
    x = term
    k = 0
    do
      call divide(term, m * m)
      if (all(term == 0)) exit
      k = k + 1
      piece = term
      call divide(piece, 2 * k + 1)
      if (mod(k, 2) == 1) then
        call subtract(x, piece)
      else
        call add(x, piece)
      end if
    end do
  end subroutine arctan

  ! X = X / D, long division from the whole part down. The remainder is
  ! less than D, so R * BASE + a word stays within INTEGER*4 while D is
  ! below 214,748: 239**2 is 57,121, and 2k+1 is about 1.43 times the
  ! number of places (each term is 25 times smaller), hence MAXPLACES.
  subroutine divide(x, d)
    integer, intent(inout) :: x(0:)
    integer, intent(in) :: d
    integer :: i, r, v

    r = 0
    do i = 0, ubound(x, 1)
      v = r * base + x(i)
      x(i) = v / d
      r = mod(v, d)
    end do
  end subroutine divide

  ! X = X + Y, carrying from the last word up.
  subroutine add(x, y)
    integer, intent(inout) :: x(0:)
    integer, intent(in) :: y(0:)
    integer :: i, carry, v

    carry = 0
    do i = ubound(x, 1), 0, -1
      v = x(i) + y(i) + carry
      carry = v / base
      x(i) = mod(v, base)
    end do
  end subroutine add

  ! X = X - Y (never negative here), borrowing from the last word up.
  subroutine subtract(x, y)
    integer, intent(inout) :: x(0:)
    integer, intent(in) :: y(0:)
    integer :: i, borrow, v

    borrow = 0
    do i = ubound(x, 1), 0, -1
      v = x(i) - y(i) - borrow
      if (v < 0) then
        v = v + base
        borrow = 1
      else
        borrow = 0
      end if
      x(i) = v
    end do
  end subroutine subtract

  ! Prints X's whole part and its first N places: groups of ten digits
  ! with a blank between, five groups to a line.
  subroutine show(x, n)
    integer, intent(in) :: x(0:), n
    character(len=56) :: out
    character(len=12) :: count
    integer :: i, col, word, digit

    write (count, '(i12)') n
    write (*, '(1x, 3a)') 'Pi to ', trim(adjustl(count)), ' decimal places:'
    write (*, '(1x)')
    write (out, '(i1, a)') x(0), '.'
    col = 2
    do i = 1, n
      word = x((i - 1) / 4 + 1)
      digit = mod(word / 10**(3 - mod(i - 1, 4)), 10)
      col = col + 1
      out(col:col) = char(ichar('0') + digit)
      if (mod(i, 50) == 0 .or. i == n) then
        write (*, '(1x, a)') out(1:col)
        out = ' '
        col = 2
      else if (mod(i, 10) == 0) then
        col = col + 1
      end if
    end do
  end subroutine show

end program pidigits
