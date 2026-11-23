! The FORALL statement (Phase 112, subtask 8): one index and several, a stride
! up and down, a mask that uses the indexes, an index set of no tuples, bounds
! known only when the program runs, a target that is an array at each tuple,
! PURE functions in the value and the mask, CHARACTER and derived-type
! targets, and the as-if-all-at-once rule where the value reads elements other
! tuples write (a shift each way, a reversal, a transpose in place). The index
! names are the statement's own: the program's I and J are left as they were.
module shapes
  implicit none

  type point
    integer :: x, y
  end type point

contains

  pure integer function sq(k)
    integer, intent(in) :: k
    sq = k * k
  end function sq

  pure logical function odd(k)
    integer, intent(in) :: k
    odd = mod(k, 2) == 1
  end function odd

end module shapes

program foralls
  use shapes
  implicit none
  integer, parameter :: n = 4
  real :: a(n,n), b(n,n), h(n,n)
  integer :: v(8), w(8), m(3,3), i, j, k, lo, hi
  character(len=3) :: names(5)
  type(point) :: pts(4)

  i = -1
  j = -2

  a = 0.0
  forall (i = 1:n) a(i,i) = real(i)
  b = 0.0
  forall (i = 1:n, j = 1:n, a(i,j) /= 0.0) b(i,j) = 1.0 / a(i,j)
  print '(4F8.4)', b
  forall (i = 1:n, j = 1:n) h(i,j) = 1.0 / real(i + j - 1)
  print '(4F8.4)', h
  print '(2I4)', i, j

  ! The value reads elements other tuples write: every right side first.
  v = (/ (k, k = 1, 8) /)
  forall (i = 2:8) v(i) = v(i-1)
  print '(8I3)', v
  v = (/ (k, k = 1, 8) /)
  forall (i = 1:7) v(i) = v(i+1)
  print '(8I3)', v
  v = (/ (k, k = 1, 8) /)
  forall (i = 1:8) v(i) = v(9-i)
  print '(8I3)', v
  ! A mask that reads elements other tuples write: computed first too.
  v = (/ (k, k = 1, 8) /)
  forall (i = 2:8, v(i-1) > 3) v(i) = v(i-1) * 10
  print '(8I3)', v
  m = reshape((/ (k, k = 1, 9) /), (/ 3, 3 /))
  forall (i = 1:3, j = 1:3) m(i,j) = m(j,i)
  print '(3I3)', m

  ! The same element read and written: no temporary needed.
  v = (/ (k, k = 1, 8) /)
  forall (i = 1:8, odd(i)) v(i) = sq(v(i))
  print '(8I3)', v

  ! Strides, an empty index set, and bounds known only when running.
  w = 0
  forall (i = 1:8:3) w(i) = i
  forall (i = 8:1:-3) w(i) = -i
  forall (i = 5:4) w(i) = 99
  print '(8I3)', w
  lo = 3
  hi = 6
  v = (/ (k, k = 1, 8) /)
  forall (i = lo:hi) v(i) = v(i+1) + v(i-1)
  print '(8I3)', v
  hi = lo - 1
  forall (i = lo:hi) v(i) = 0
  print '(8I3)', v

  ! A target that is an array at each tuple.
  forall (i = 1:n) a(i,:) = h(:,i) * 2.0
  print '(4F8.4)', a

  ! A mask of a pure function and an index; CHARACTER and records.
  names = (/ 'one', 'two', 'six', 'ten', 'end' /)
  forall (i = 1:5, i /= 3) names(i)(2:2) = '*'
  print '(5(1X,A))', names
  forall (i = 1:4) names(i) = names(i+1)
  print '(5(1X,A))', names
  forall (i = 1:5) names(i) = names(6-i)(3:3) // names(i)(1:1) // 'x'
  print '(5(1X,A))', names
  forall (i = 1:4) pts(i) = point(i, sq(i))
  forall (i = 1:3) pts(i) = pts(i+1)
  print '(4(1X,I2,I3))', pts
  forall (i = 1:4) pts(i)%y = pts(5-i)%x
  print '(4(1X,I2,I3))', pts
end program foralls
