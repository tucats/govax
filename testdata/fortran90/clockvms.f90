! clockvms.f90: the intrinsic subroutines of the clock and random numbers as
! govax's FTNRTL gives them (docs/planning/PHASE-108 - Fortran 90 intrinsic
! procedures.md, findings 9 to 11): MTH$RANDOM's generator, SEED = 69069 *
! SEED + 1 modulo 2**32, each REAL the seed's high 24 bits over 2**24, a wider
! format several draws joined, the first seed 1234567, a seed of one longword
! (two words for INTEGER*2); SYSTEM_CLOCK's rate 100 and maximum HUGE of its
! kind; DATE_AND_TIME's zone from SYS$TIMEZONE_DIFFERENTIAL, defined here by
! LIB$SET_LOGICAL in the process table (and deleted at the end), so the run
! doesn't depend on the host's zone. Each number is printed as integers, a
! draw's 24 bits, so it can be worked by hand from the formula.
program clockvms
  implicit none
  integer :: n, s(1), g(1), i, rate, cmax, v(8), parts(5)
  integer(2) :: n2, w(2), rate2, max2, v2(8)
  real :: a(4), x
  double precision :: d
  real(16) :: h
  character(len=5) :: zone
  character(len=12) :: wide
  character(len=3) :: narrow
  integer, external :: lib$set_logical, lib$delete_logical

  call random_seed(size=n)
  call random_seed(size=n2)
  write (*, '(1x, a, 2i3)') 'SIZE:', n, n2
  call random_seed(get=g)
  write (*, '(1x, a, i12)') 'First seed:', g(1)
  call random_number(a)
  write (*, '(1x, a, 4i10)') 'Draws:', int(a * 2.0**24)
  call random_seed(get=g)
  write (*, '(1x, a, i12)') 'Seed now:', g(1)
  call random_seed()
  call random_seed(get=g)
  write (*, '(1x, a, i12)') 'Restored:', g(1)

  ! A seed of 1 is followed by 69070, whose high 24 bits are 269.
  s(1) = 1
  call random_seed(put=s)
  call random_number(x)
  write (*, '(1x, a, i10)') 'After 1:', int(x * 2.0**24)
  call random_seed(get=w)
  write (*, '(1x, a, 2i8)') 'As words:', w
  w = (/ 1_2, 0_2 /)
  call random_seed(put=w)
  call random_number(x)
  write (*, '(1x, a, i10)') 'Words put:', int(x * 2.0**24)

  ! REAL(8): three draws, the third cut to 8 bits.
  call random_seed(put=s)
  call random_number(d)
  parts(1) = int(d * 2.0d0**24)
  parts(2) = int(mod(d * 2.0d0**24, 1.0d0) * 2.0d0**24)
  parts(3) = int(mod(d * 2.0d0**48, 1.0d0) * 2.0d0**8)
  write (*, '(1x, a, 3i10, l2)') 'REAL(8):', parts(1:3), mod(d * 2.0d0**56, 1.0d0) == 0.0d0

  ! REAL(16): five draws, the fifth cut to 17 bits.
  call random_seed(put=s)
  call random_number(h)
  do i = 1, 4
    parts(i) = int(mod(h * 2.0_16**(24 * i), 2.0_16**24))
  end do
  parts(5) = int(mod(h * 2.0_16**113, 2.0_16**17))
  write (*, '(1x, a, 5i10, l2)') 'REAL(16):', parts, mod(h * 2.0_16**113, 1.0_16) == 0.0_16

  ! The clock's rate and maximum, each kind.
  call system_clock(count_rate=rate, count_max=cmax)
  call system_clock(count_rate=rate2, count_max=max2)
  write (*, '(1x, a, 2i12, 2i8)') 'Clock:', rate, cmax, rate2, max2

  ! The zone, as VMS writes it and otherwise.
  call zone_of('-18000')
  call zone_of('19800')
  call zone_of('+3600')
  call zone_of('0')
  call zone_of('-1800')
  call zone_of('ABC')
  call zone_of('-')

  i = lib$set_logical('SYS$TIMEZONE_DIFFERENTIAL', '19800')
  call date_and_time(zone=wide)
  call date_and_time(zone=narrow)
  write (*, '(1x, a, 3a)') 'Lengths: "', wide, '" "' // narrow // '"'
  call date_and_time(values=v2)
  write (*, '(1x, a, i8)') 'INTEGER(2) zone:', v2(4)
  i = lib$set_logical('SYS$TIMEZONE_DIFFERENTIAL', 'NONE')
  call date_and_time(values=v2)
  write (*, '(1x, a, i8)') 'INTEGER(2) none:', v2(4)
  i = lib$delete_logical('SYS$TIMEZONE_DIFFERENTIAL')

contains

  ! zone_of shows DATE_AND_TIME's ZONE and VALUES(4) with the zone text t.
  subroutine zone_of(t)
    character(len=*), intent(in) :: t
    integer :: k

    k = lib$set_logical('SYS$TIMEZONE_DIFFERENTIAL', t)
    zone = 'xxxxx'
    call date_and_time(zone=zone, values=v)
    write (*, '(1x, a, a8, 3a, i12)') 'Zone', t, ': "', zone, '"', v(4)
  end subroutine zone_of
end program clockvms
