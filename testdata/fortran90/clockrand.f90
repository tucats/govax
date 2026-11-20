! clockrand.f90: Fortran 90's intrinsic subroutines of the clock and random
! numbers (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md,
! subtask 7), printing only what any processor must: RANDOM_SEED's size, a
! sequence repeated after PUT and after GET then PUT, numbers from 0 up to but
! not including 1 in two kinds, by element and whole, SYSTEM_CLOCK's rate and
! maximum, a count in its range, and DATE_AND_TIME's texts agreeing with its
! VALUES and with one another. Whatever the seed, the time, or the zone, it
! prints the same.
program clockrand
  implicit none
  integer :: n, i, c1, c2, rate, cmax, y, m, d, hh, mm, ss, ms, zh, zm
  integer, allocatable :: seed(:), back(:)
  real :: a(5), b(5), x, x2, grid(3, 4)
  double precision :: p(4), p2(4)
  character(len=8) :: date
  character(len=10) :: time
  character(len=5) :: zone
  integer :: v(8), w(8)

  ! The seed: its size, then a sequence that PUT starts again.
  call random_seed(size=n)
  write (*, '(1x, a, l2)') 'Seed size positive:', n > 0
  allocate (seed(n), back(n))
  seed = (/ (1000 + 37 * i, i = 1, n) /)
  call random_seed(put=seed)
  call random_number(a)
  call random_seed(put=seed)
  call random_number(b)
  write (*, '(1x, a, l2)') 'Repeated after PUT:', all(a == b)
  write (*, '(1x, a, l2)') 'From 0 to 1:', all(a >= 0.0 .and. a < 1.0)

  ! GET where a sequence has got to, and PUT it back: the next is repeated.
  call random_seed(get=back)
  call random_number(x)
  call random_seed(put=back)
  call random_number(harvest=x2)
  write (*, '(1x, a, l2)') 'Repeated after GET:', x == x2

  ! An array is filled in array element order: a matrix's numbers are the
  ! same as a vector's.
  call random_seed(put=seed)
  call random_number(grid)
  call random_seed(put=seed)
  call random_number(a)
  write (*, '(1x, a, l2)') 'Matrix as vector:', all(grid(1:3, 1) == a(1:3)) .and. grid(1, 2) == a(4) &
    .and. grid(2, 2) == a(5)

  ! Double precision, by element and whole.
  call random_number(p)
  write (*, '(1x, a, l2)') 'Double from 0 to 1:', all(p >= 0.0d0 .and. p < 1.0d0)
  call random_seed(put=back)
  do i = 1, 4
    call random_number(p2(i))
  end do
  call random_seed(put=back)
  call random_number(p)
  write (*, '(1x, a, l2)') 'Double by element:', all(p == p2)

  ! The clock.
  call system_clock(c1, rate, cmax)
  write (*, '(1x, a, 2l2)') 'Rate and maximum positive:', rate > 0, cmax > 0
  call system_clock(count=c2)
  write (*, '(1x, a, l2)') 'Counts in range:', c1 >= 0 .and. c1 <= cmax .and. c2 >= 0 .and. c2 <= cmax
  write (*, '(1x, a, l2)') 'Not backward:', c2 >= c1 .or. c1 - c2 > cmax / 2

  ! The date and time: the texts' fields are VALUES' numbers.
  call date_and_time(date, time, zone, v)
  write (*, '(1x, a, l2)') 'Date digits:', verify(date, '0123456789') == 0
  read (date, '(i4, 2i2)') y, m, d
  write (*, '(1x, a, l2)') 'Date agrees:', y == v(1) .and. m == v(2) .and. d == v(3)
  write (*, '(1x, a, l2)') 'Month and day:', v(2) >= 1 .and. v(2) <= 12 .and. v(3) >= 1 .and. v(3) <= 31
  write (*, '(1x, a, l2)') 'Time form:', time(7:7) == '.' .and. verify(time(1:6) // time(8:10), '0123456789') == 0
  read (time, '(3i2, 1x, i3)') hh, mm, ss, ms
  write (*, '(1x, a, l2)') 'Time agrees:', hh == v(5) .and. mm == v(6) .and. ss == v(7) .and. ms == v(8)
  write (*, '(1x, a, l2)') 'Time in range:', v(5) <= 23 .and. v(6) <= 59 .and. v(7) <= 60 .and. v(8) <= 999 &
    .and. all(v(5:8) >= 0)
  if (v(4) == -huge(0)) then
    write (*, '(1x, a, l2)') 'Zone agrees:', zone == ' '
  else
    read (zone, '(1x, 2i2)') zh, zm
    if (zone(1:1) == '-') then
      zh = -zh
      zm = -zm
    end if
    write (*, '(1x, a, l2)') 'Zone agrees:', (zone(1:1) == '+' .or. zone(1:1) == '-') .and. zh * 60 + zm == v(4)
  end if

  ! Keywords in any order, and the arguments left out.
  call date_and_time(values=w, date=date)
  write (*, '(1x, a, l2)') 'By keyword:', w(1) == v(1) .and. date(1:4) /= '    '
  call date_and_time()
  call system_clock()
  write (*, '(1x, a)') 'Done'
end program clockrand
