! Items by descriptor (Phase 109, subtask 7): a section of a variable, an
! assumed-shape array, an array pointer's target, and a ranked component are
! each one item, which FTNRTL walks in array element order: written and read
! formatted, list-directed, unformatted, and nonadvancing, with strides going
! up and down in two dimensions, an empty section, and an end of file part way
! through one. And sections as internal files, written and read, one whose
! elements are one after another and one whose elements aren't.
program ioitems
  implicit none
  type point
    real :: x, y
  end type point
  real, target :: m(4, 5)
  real, pointer :: pm(:, :)
  real :: v(10), buf(6)
  integer :: i, j, ios, k(8)
  character(len=6) :: lines(6)
  character(len=8) :: rec(4)
  character(len=5) :: names(4)
  type(point) :: pts(4)

  m = reshape((/ (real(i), i = 1, 20) /), (/ 4, 5 /))
  print '(1x, a, 6f5.1)', 'strided: ', m(1:4:2, 1:5:2)
  pm => m(4:1:-1, 2:5:3)
  print '(1x, a, 8f5.1)', 'pointer: ', pm
  call show(m(2:4:2, 4:5), 'dummy:   ')

  k = (/ (10 * i, i = 1, 8) /)
  print *, k(8:2:-3)

  v = 0.0
  call fill(v(1:10:3))
  print '(1x, a, 10f4.1)', 'filled:  ', v
  read '(4f4.0)', m(1:3:2, 2:3)
  print '(1x, a, 8f5.1)', 'read2d:  ', m(:, 2:3)
  j = 0
  print '(1x, a, 3f5.1)', 'empty:', v(1:j)

  open (unit = 1, status = 'scratch', form = 'unformatted')
  write (1) m(4:1:-3, 1:5:4), k(1:8:7)
  rewind (1)
  buf = 0.0
  read (1) buf(6:3:-1), k(7:6:-1)
  close (1)
  print '(1x, a, 4f5.1, 2i4)', 'unform:  ', buf(3:6), k(6:7)

  names = (/ 'alpha', 'bravo', 'charl', 'delta' /)
  print '(1x, a, 2a4)', 'chars:   ', names(4:1:-3)(2:4)

  pts%x = (/ 1.0, 2.0, 3.0, 4.0 /)
  pts%y = 10.0 * pts%x
  print '(1x, a, 2f5.1)', 'ys:      ', pts(4:1:-2)%y
  read *, pts(1:3:2)%x
  print '(1x, a, 4f5.1)', 'xs:      ', pts%x

  lines = '------'
  write (lines(1:6:2), '(i6)') 11, 22, 33
  print '(1x, a, 6a)', 'file:    ', lines
  read (lines(5:1:-2), '(i6)') k(1:3)
  print '(1x, a, 3i4)', 'back:    ', k(1:3)
  rec = '........'
  write (rec(2:3), '(a8)') 'abcdefgh', 'ijklmnop'
  print '(1x, a, 4a)', 'recs:    ', rec

  open (unit = 2, status = 'scratch', form = 'formatted')
  write (2, '(2f4.1)') 9.5, 8.5
  rewind (2)
  v = 0.0
  read (2, '(f4.1)', iostat = ios) v(1:10:3)
  close (2)
  print '(1x, a, i3, 4f5.1)', 'eof:     ', ios, v(1:10:3)

  write (*, '(a, 3i3)', advance = 'no') ' na:', k(3:1:-1)
  write (*, '(i3)') 99

contains

  subroutine show(a, label)
    real, intent(in) :: a(:, :)
    character(len=*), intent(in) :: label
    print '(1x, a, 6f5.1)', label, a
  end subroutine show

  subroutine fill(a)
    real, intent(out) :: a(:)
    read *, a
  end subroutine fill

end program ioitems
