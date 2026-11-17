! cases.f90: every form of INTEGER selection (docs/planning/PHASE-101 -
! Fortran 90 control constructs.md, subtask 5): caseForm's thresholds on both
! sides (a table for four tests over 16 values, a chain over 17; a chain of
! seven sparse tests, a search of eight); a table of seven values filling it;
! open ranges on both sides of a table; an INTEGER(1) selector whose ranges
! reach its limits; a function's value as the selector; a CASE construct
! inside another, both named, with their names on the CASE statements; and a
! GO TO to a labeled END SELECT from inside a CASE block. Each line of digits
! is one function's answer for each value of a list, worked out by hand.
program cases
  implicit none
  integer :: i, j
  integer :: near(20), far(11), wide(14)
  integer(1) :: bytes(7)
  character(len=20) :: line
  integer, external :: table16, chain17, chain7, search8, open2, small, isq, pairs, skips
  data near / -1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18 /
  data far / 0, 100, 150, 200, 300, 400, 500, 600, 700, 800, 900 /
  data wide / -100, -10, -9, -3, -2, -1, 0, 3, 4, 5, 6, 49, 50, 1000 /
  data bytes / -128, -1, 0, 1, 99, 100, 127 /

  do i = 1, 20
     line(i:i) = char(48 + table16(near(i)))
  end do
  write (*, '(1x, a, a)') 'table16: ', line(1:20)

  do i = 1, 20
     line(i:i) = char(48 + chain17(near(i)))
  end do
  write (*, '(1x, a, a)') 'chain17: ', line(1:20)

  do i = 1, 11
     line(i:i) = char(48 + chain7(far(i)))
  end do
  write (*, '(1x, a, a)') 'chain7:  ', line(1:11)

  do i = 1, 11
     line(i:i) = char(48 + search8(far(i)))
  end do
  write (*, '(1x, a, a)') 'search8: ', line(1:11)

  ! A table of seven values, one for each day of the week.
  do i = 0, 8
     select case (i)
     case (1, 7)
        line(i+1:i+1) = 'S'
     case (2)
        line(i+1:i+1) = 'M'
     case (3, 5)
        line(i+1:i+1) = 'T'
     case (4)
        line(i+1:i+1) = 'W'
     case (6)
        line(i+1:i+1) = 'F'
     case default
        line(i+1:i+1) = '-'
     end select
  end do
  write (*, '(1x, a, a)') 'days:    ', line(1:9)

  do i = 1, 14
     line(i:i) = char(48 + open2(wide(i)))
  end do
  write (*, '(1x, a, a)') 'open2:   ', line(1:14)

  do i = 1, 7
     line(i:i) = char(48 + small(bytes(i)))
  end do
  write (*, '(1x, a, a)') 'small:   ', line(1:7)

  ! A function's value as the selector, evaluated once.
  do i = 1, 20
     select case (isq(near(i)))
     case (0)
        line(i:i) = '1'
     case (1:9)
        line(i:i) = '2'
     case (10:99)
        line(i:i) = '3'
     case (100:)
        line(i:i) = '4'
     end select
  end do
  write (*, '(1x, a, a)') 'squares: ', line(1:20)

  do i = 0, 3
     do j = 0, 3
        line(4*i+j+1:4*i+j+1) = char(48 + pairs(i, j))
     end do
  end do
  write (*, '(1x, a, a)') 'pairs:   ', line(1:16)

  do i = 0, 6
     line(i+1:i+1) = char(48 + skips(i))
  end do
  write (*, '(1x, a, a)') 'skips:   ', line(1:7)
end program cases

! Four tests over the values 1 to 16: a table.
integer function table16(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (1)
     table16 = 1
  case (4:5)
     table16 = 2
  case (16)
     table16 = 3
  case default
     table16 = 0
  end select
end function table16

! The same four tests over 1 to 17, one value too sparse for a table: a chain.
integer function chain17(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (1)
     chain17 = 1
  case (4:5)
     chain17 = 2
  case (17)
     chain17 = 3
  case default
     chain17 = 0
  end select
end function chain17

! Seven sparse values: a chain.
integer function chain7(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (100); chain7 = 1
  case (200); chain7 = 2
  case (300); chain7 = 3
  case (400); chain7 = 4
  case (500); chain7 = 5
  case (600); chain7 = 6
  case (700); chain7 = 7
  case default; chain7 = 0
  end select
end function chain7

! Eight sparse values: a binary search.
integer function search8(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (100); search8 = 1
  case (200); search8 = 2
  case (300); search8 = 3
  case (400); search8 = 4
  case (500); search8 = 5
  case (600); search8 = 6
  case (700); search8 = 7
  case (800); search8 = 8
  case default; search8 = 0
  end select
end function search8

! A table from -2 to 5, and open ranges below and above it.
integer function open2(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (:-10)
     open2 = 1
  case (-2)
     open2 = 2
  case (0:3)
     open2 = 3
  case (5)
     open2 = 4
  case (50:)
     open2 = 5
  case default
     open2 = 0
  end select
end function open2

! An INTEGER(1) selector, its ranges reaching the kind's limits.
integer function small(b)
  implicit none
  integer(1), intent(in) :: b
  select case (b)
  case (-128:-1)
     small = 1
  case (0)
     small = 2
  case (100:127)
     small = 3
  case default
     small = 0
  end select
end function small

integer function isq(i)
  implicit none
  integer, intent(in) :: i
  isq = i * i
end function isq

! A CASE construct inside another, each named, the names on its CASEs.
integer function pairs(i, j)
  implicit none
  integer, intent(in) :: i, j
  rows: select case (i)
  case (:0) rows
     pairs = 0
  case (1, 2) rows
     cols: select case (j - i)
     case (0) cols
        pairs = 1
     case (1:) cols
        pairs = 2
     case default cols
        pairs = 3
     end select cols
  case default rows
     pairs = 4
  end select rows
end function pairs

! A GO TO from inside a CASE block to its construct's END SELECT.
integer function skips(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (1:5)
     skips = 1
     if (i > 3) go to 10
     skips = 2
  case default
     skips = 9
10 end select
end function skips
