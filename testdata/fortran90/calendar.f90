! Dates and a month's calendar (docs/planning/PHASE-108 - Fortran 90 intrinsic
! procedures.md, subtask 8). A date's day number counts from 1 January 1970 by
! the civil calendar's 400-year eras, divided by MODULO; its weekday is MODULO
! of that, right for the negative day numbers before 1970 as MOD wouldn't be; its week
! of the year a CEILING, its whole months since another date a FLOOR. Each date
! is packed into 16 bits by MVBITS (the year less 1900, MODULO 128, in bits 9 to
! 15, so 1600 comes back as 1984; the month in 5 to 8, the day in 0 to 4),
! turned by ISHFTC, and unpacked by IBITS; a
! month's name is folded into one INTEGER by TRANSFER and IEOR, and the months'
! and weekdays' initials are substrings of constants. A month is laid
! out under a centered title. Last, the length of a day in REAL's spacing at each
! power of two up to its day numbers: EXPONENT, FRACTION, SPACING, and NEAREST.
program calendar
  implicit none
  character(len=9), parameter :: names(0:6) = (/ 'Sunday   ', 'Monday   ', &
    'Tuesday  ', 'Wednesday', 'Thursday ', 'Friday   ', 'Saturday ' /)
  character(len=9), parameter :: months(12) = (/ 'January  ', 'February ', &
    'March    ', 'April    ', 'May      ', 'June     ', 'July     ', &
    'August   ', 'September', 'October  ', 'November ', 'December ' /)
  integer, parameter :: dates(3, 6) = reshape((/ 1970, 1, 1, 1977, 10, 25, &
    2000, 2, 29, 1900, 3, 1, 1600, 1, 1, 2026, 11, 19 /), (/ 3, 6 /))
  character(len=7), parameter :: initials = 'SMTWTFS'
  integer :: i, y, m, d, n, packed, turned, w, ix(2)
  real :: x

  print '(1x, a)', '      date     day  weekday   week  packed  turned    unpacked'
  do i = 1, size(dates, 2)
    y = dates(1, i)
    m = dates(2, i)
    d = dates(3, i)
    n = daynum(y, m, d)
    w = ceiling((n - daynum(y, 1, 1) + 1) / 7.0)
    packed = 0
    call mvbits(modulo(y - 1900, 128), 0, 7, packed, 9)
    call mvbits(m, 0, 4, packed, 5)
    call mvbits(d, 0, 5, packed, 0)
    turned = ishftc(packed, 4, 16)
    print '(1x, i4, 2("-", i2.2), i8, 2x, a9, i5, 4x, z4.4, 4x, z4.4, 2x, i4, 2("-", i2.2))', &
      y, m, d, n, names(modulo(n + 4, 7)), w, packed, turned, &
      1900 + ibits(packed, 9, 7), ibits(packed, 5, 4), ibits(packed, 0, 5)
  end do

  print '(1x)'
  print '(1x, a, i6)', 'whole months, 1977-10-25 to 2026-11-19:', &
    floor((daynum(2026, 11, 19) - daynum(1977, 10, 25)) / 30.436875)
  print '(1x, a, i6)', 'whole months, 2026-11-19 to 1977-10-25:', &
    floor((daynum(1977, 10, 25) - daynum(2026, 11, 19)) / 30.436875)
  ix = transfer(months(11)(1:8), ix)
  print '(1x, a, 2(1x, z8.8), a, z8.8)', 'November as INTEGERs:', ix, ', folded ', ieor(ix(1), ix(2))
  print '(1x, 12(a3, :, 1x))', (months(i)(1:3), i = 1, 12)
  print '(1x, 7(a1, :, 1x))', (initials(i:i), i = 1, 7)

  print '(1x)'
  call month(2026, 11)
  print '(1x)'
  call month(2000, 2)

  print '(1x)'
  print '(1x, a)', '  days  exponent  fraction     spacing  next'
  x = 1.0
  do i = 0, 14, 2
    print '(1x, f6.0, i10, f10.6, e12.4, l6)', x, exponent(x), fraction(x), &
      spacing(x), nearest(x, 1.0) - x == spacing(x)
    x = x * 4.0
  end do
  x = real(daynum(2026, 11, 19))
  print '(1x, f6.0, i10, f10.6, e12.4)', x, exponent(x), fraction(x), spacing(x)

contains

  ! The day number of y-m-d, 1970-01-01 being 0: the year counted from March,
  ! so a leap day is a year's last, in eras of 400 years (146,097 days); the
  ! era is the year's MODULO taken from it, over 400, so a year before 0 is in
  ! a negative era.
  integer function daynum(y, m, d)
    integer, intent(in) :: y, m, d
    integer :: yy, era, yoe, doy

    yy = y
    if (m <= 2) yy = y - 1
    yoe = modulo(yy, 400)
    era = (yy - yoe) / 400
    doy = (153 * modulo(m - 3, 12) + 2) / 5 + d - 1
    daynum = era * 146097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719468
  end function daynum

  ! A month laid out by weeks, under its name and year centered.
  subroutine month(y, m)
    integer, intent(in) :: y, m
    character(len=20) :: title, row
    integer :: first, last, day, col

    title = trim(months(m)) // ' ' // char(ichar('0') + y / 1000) // &
      char(ichar('0') + modulo(y / 100, 10)) // char(ichar('0') + modulo(y / 10, 10)) // &
      char(ichar('0') + modulo(y, 10))
    print '(1x, a)', repeat(' ', (20 - len_trim(title)) / 2) // trim(title)
    print '(1x, a)', 'Su Mo Tu We Th Fr Sa'
    first = daynum(y, m, 1)
    if (m == 12) then
      last = daynum(y + 1, 1, 1) - 1
    else
      last = daynum(y, m + 1, 1) - 1
    end if
    row = ''
    col = modulo(first + 4, 7)
    do day = first, last
      write (row(3 * col + 1:3 * col + 2), '(i2)') day - first + 1
      col = col + 1
      if (col == 7 .or. day == last) then
        print '(1x, a)', trim(row)
        row = ''
        col = 0
      end if
    end do
  end subroutine month

end program calendar
