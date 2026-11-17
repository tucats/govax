! selects.f90: Fortran 90's CASE construct (docs/planning/PHASE-101 - Fortran
! 90 control constructs.md, subtask 4), each form the compiler gives it: an
! INTEGER construct by a table, a binary search, and a chain; INTEGER(2);
! LOGICAL; CHARACTER by a chain and by a search, a substring found at run
! time, and a concatenation of a passed-length dummy; a constant selector;
! CYCLE and EXIT from inside a CASE block. Each line of digits is one
! function's answer for each value of a list, worked out by hand.
program selects
  implicit none
  integer, parameter :: mode = 2
  integer :: i, j, n, tab, srch, short, pick, vowels, others, blanks, consonants
  integer :: tabbed(23), searched(24)
  integer(2) :: shorts(6)
  logical :: truth
  character(len=24) :: line
  character(len=8) :: words(8)
  character(len=15) :: text
  data tabbed / -3, -2, -1, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, &
       14, 15, 16, 99, 100, 1000 /
  data searched / -5, 0, 1, 9, 10, 11, 19, 20, 25, 29, 30, 34, 35, 36, 39, &
       40, 41, 49, 50, 51, 59, 60, 61, 1000 /
  data shorts / -32768, -1, 0, 29999, 30000, 32767 /
  data words / 'apple', 'm', 'mango', 'zz', 'zebra', 'nut', 'Apple', ' ' /

  ! A table: CASEL from -1 to 14, and 100: tested outside it.
  do i = 1, 23
     line(i:i) = char(48 + tab(tabbed(i)))
  end do
  write (*, '(1x, a, a)') 'table:  ', line(1:23)

  ! A binary search over eight tests.
  do i = 1, 24
     line(i:i) = char(48 + srch(searched(i)))
  end do
  write (*, '(1x, a, a)') 'search: ', line(1:24)

  ! A chain on an INTEGER(2).
  do i = 1, 6
     line(i:i) = char(48 + short(shorts(i)))
  end do
  write (*, '(1x, a, a)') 'short:  ', line(1:6)

  ! LOGICAL, with DEFAULT for .FALSE.
  line = ' '
  do i = 1, 2
     truth = i == 1
     select case (truth)
     case (.true.)
        line(i:i) = 'T'
     case default
        line(i:i) = 'F'
     end select
  end do
  write (*, '(1x, a, a)') 'logic:  ', line(1:2)

  ! CHARACTER: ranges of text, the shorter padded with blanks.
  do i = 1, 8
     select case (words(i))
     case ('a':'m', 'zz')
        line(i:i) = '1'
     case ('n':'y')
        line(i:i) = '2'
     case default
        line(i:i) = '0'
     end select
  end do
  write (*, '(1x, a, a)') 'words:  ', line(1:8)

  ! CHARACTER by a search, a character at a time.
  text = 'hello, world 42'
  vowels = 0
  consonants = 0
  blanks = 0
  others = 0
  do j = 1, len(text)
     select case (text(j:j))
     case ('a', 'e', 'i', 'o', 'u')
        vowels = vowels + 1
     case ('b':'d', 'f':'h', 'j':'n', 'p':'t', 'v':'z')
        consonants = consonants + 1
     case (' ')
        blanks = blanks + 1
     case default
        others = others + 1
     end select
  end do
  write (*, '(1x, a, 4i3)') 'letters:', vowels, consonants, blanks, others

  ! A concatenation of a passed-length dummy.
  write (*, '(1x, a, 4i2)') 'pick:   ', pick('a'), pick('b'), pick('c'), pick('ab')

  ! A constant selector.
  select case (mode)
  case (1)
     write (*, '(1x, a)') 'mode one'
  case (2:)
     write (*, '(1x, a)') 'mode two or more'
  end select

  ! CYCLE and EXIT from inside CASE blocks.
  n = 0
  outer: do i = 1, 10
     select case (i)
     case (3)
        cycle outer
     case (6)
        exit outer
     end select
     n = n + i
  end do outer
  write (*, '(1x, a, i3, a, i3)') 'sum:', n, ', i =', i
end program selects

integer function tab(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (-1)
     tab = 1
  case (3, 14)
     tab = 2
  case (100:)
     tab = 3
  case default
     tab = 0
  end select
end function tab

integer function srch(i)
  implicit none
  integer, intent(in) :: i
  select case (i)
  case (10, 20:29)
     srch = 1
  case (40, 50, 60:)
     srch = 2
  case (:0)
     srch = 3
  case (35)
     srch = 4
  case default
     srch = 5
  end select
end function srch

integer function short(k)
  implicit none
  integer(2), intent(in) :: k
  select case (k)
  case (30000:)
     short = 1
  case (-32768:-1)
     short = 3
  case default
     short = 0
  end select
end function short

integer function pick(s)
  implicit none
  character(len=*), intent(in) :: s
  select case (s // 'x')
  case ('ax')
     pick = 1
  case ('bx', 'cx')
     pick = 2
  case default
     pick = 0
  end select
end function pick
