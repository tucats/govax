! Fortran 90's edit descriptors (the standard's 10.5): B, EN, ES, a repeated
! slash, and G of an INTEGER, LOGICAL, or CHARACTER item, in FORMAT
! statements, in formats written as character constants, and in formats held
! in variables, which FTNRTL reads when the program runs.
program edits
  implicit none
  integer :: i, k
  integer(2) :: h
  logical :: yes
  character(len=8) :: lines(4), text
  character(len=48) :: fmt
  real :: x, values(6)
  data values /12345.678, 0.001234, 999.9996, 9.9996, 0.0, -0.5/

  ! B: binary, with m digits at least; m 0 and a zero value is blanks.
  h = -1
  write (*, 10) 5, 5, 255
  write (*, 11) h, 0, 0
10 format (1x, '|', b8, '|', b8.6, '|', b12, '|')
11 format (1x, '|', b16, '|', b4.0, '|', b4, '|')

  ! EN and ES: an exponent a multiple of 3, and one digit before the point.
  do i = 1, 6
    write (*, 20) values(i), values(i)
  end do
20 format (1x, '|', en12.3, '|', es12.3, '|')

  ! ES and EN ignore the scale factor, which F still takes.
  write (*, '(1x, 2p, es12.3, en12.3, f8.2)') 1.0, 1.0, 1.0
  write (*, '(1x, es10.2e3)') 1.5e-20

  ! G of every type: Iw, Lw, Aw for the first three, d not used.
  k = 42
  yes = .true.
  text = 'ABC'
  x = 3.14159
  write (*, 30) k, k, yes, text, x
30 format (1x, '|', g6.1, '|', g6.3, '|', g3.1, '|', g8.1, '|', g12.4, '|')

  ! And on input; ES as F.
  k = 0
  yes = .false.
  text = '1011'
  read (text, '(b4)') i
  text = '1.5E3'
  read (text, '(es8.1)') x
  text = '  42'
  read (text, '(g4.1)') k
  text = 'T'
  read (text, '(g1.1)') yes
  write (*, '(1x, i4, f8.1, i4, l2)') i, x, k, yes

  ! An internal file's blanks in numbers are nothing, BLANK='NULL' (the
  ! standard's 9.2.2.2), where VAX FORTRAN's are zeros: the trailing blanks
  ! after 1.5E3 above, and the blank inside 12 3, add no digits.
  text = '12 3'
  read (text, '(i5)') k
  write (*, '(1x, i6)') k
  read (text, '(bz, i5)') k
  write (*, '(1x, i6)') k

  ! r/: r records end, r-1 of them empty.
  lines = '--------'
  write (lines, '(a, 2/, a)') 'X', 'Y'
  write (*, '(1x, 4("|", a, "|"))') lines

  ! A format held in a variable: quotation marks, B, EN, and r/ read when
  ! the program runs.
  fmt = '(1X, "B=", B6, " EN=", EN10.2, " say ""hi""")'
  write (*, fmt) 6, 1234.5
  fmt = '(1X, A, 2/, 1X, A)'
  write (*, fmt) 'P', 'Q'
end program edits
