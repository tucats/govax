! Phase 113, subtask 2: SPREAD, CSHIFT, and EOSHIFT with a DIM= known only
! when the program runs, and RESHAPE with an ORDER= known only then; each
! compared with the same call by constants.
program rundims
  implicit none
  integer :: v(3), m(2, 3), d, k, o(2), r(3, 2)
  integer :: s(2, 3), t(3, 2)
  character(len=2) :: c(2, 2)

  v = (/ 1, 2, 3 /)
  m = reshape((/ 1, 2, 3, 4, 5, 6 /), (/ 2, 3 /))
  c = reshape((/ 'ab', 'cd', 'ef', 'gh' /), (/ 2, 2 /))

  do d = 1, 2
    write (*, '(1x, a, i1, a, 6i3)') 'spread dim ', d, ':', spread(v(1:2), d, 3)
    write (*, '(1x, a, i1, a, 6i3)') 'cshift dim ', d, ':', cshift(m, 1, d)
    write (*, '(1x, a, i1, a, 6i3)') 'eoshift dim', d, ':', eoshift(m, -1, 9, d)
    write (*, '(1x, a, i1, a, 4(1x, a))') 'eoshc dim  ', d, ':', eoshift(c, 1, dim=d)
  end do

  ! A SHIFT array, whose extent follows DIM.
  d = 2
  write (*, '(1x, a, 6i3)') 'cshift by row:', cshift(m, (/ 1, 2 /), d)
  d = 1
  write (*, '(1x, a, 6i3)') 'cshift by col:', cshift(m, (/ 1, 0, 1 /), d)

  ! SPREAD's result in an assignment, its shape known only now.
  d = 1
  s = spread(v(1:3), d, 2)
  write (*, '(1x, a, 6i3)') 'spread into s:', s
  d = 2
  t = spread(v(1:3), d, 2)
  write (*, '(1x, a, 6i3)') 'spread into t:', t
  k = size(spread(v, d, 4), 1)
  write (*, '(1x, a, 2i3)') 'sizes:', k, size(spread(v, d, 4), 2)

  ! A scalar SOURCE: DIM can only be 1.
  d = 1
  write (*, '(1x, a, 4i3)') 'scalar:', spread(7, d, 4)

  ! RESHAPE's ORDER.
  o = (/ 2, 1 /)
  r = reshape((/ 1, 2, 3, 4, 5, 6 /), (/ 3, 2 /), order=o)
  write (*, '(1x, a, 6i3)') 'order 2 1:', r
  write (*, '(1x, a, 6i3)') 'constant: ', reshape((/ 1, 2, 3, 4, 5, 6 /), (/ 3, 2 /), order=(/ 2, 1 /))
  o = (/ 1, 2 /)
  r = reshape((/ 1, 2, 3, 4, 5, 6 /), (/ 3, 2 /), order=o)
  write (*, '(1x, a, 6i3)') 'order 1 2:', r
  o = (/ 2, 1 /)
  write (*, '(1x, a, 8i3)') 'padded:   ', reshape((/ 1, 2, 3 /), (/ 2, 4 /), pad=(/ 0 /), order=o)
end program rundims
