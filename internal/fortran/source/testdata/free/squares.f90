! Squares: the same program in free form (here) and fixed form
! (squares.for), which must make the same statements.
program squares
  integer :: i, total
  total = 0
  do 10 i = 1, 5; total = total + i*i
10 continue
  print *, 'the sum of the squares ', &
           'of 1 to 5 is', total   ! a comment
  print 20, &
     & total
20 format (' TOTAL = ', &
          i5)
end program squares
