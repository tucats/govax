! Fortran 95's WHERE (Phase 112, subtask 10): a WHERE nested in a WHERE,
! ELSEWHERE with a mask of its own, and named constructs. Each statement is
! done under its control mask (the masks of the blocks around it, and the
! masks before its block false), and each mask is computed when its statement
! is reached, only where its control mask holds.
program wheres
  implicit none
  integer :: a(8), b(8), i, k(6), v(7)
  real :: x(5), y(5), s(3,4)
  integer :: c(3,4), g(3,3)

  ! The survey's probe: odd and even among the positive, and two ELSEWHEREs.
  a = (/ (i, i = -3, 4) /)
  b = 0
  outer: where (a > 0)
    where (mod(a, 2) == 0)
      b = 2
    elsewhere
      b = 1
    end where
  elsewhere (a < -1) outer
    b = -2
  elsewhere outer
    b = -1
  end where outer
  print '(8I3)', b

  ! A nested mask is computed only where the one around it holds: 1/X is
  ! never computed for a zero X, which would end the program on a VAX.
  x = (/ 0.0, 2.0, -4.0, 1.0, 0.0 /)
  where (x /= 0.0)
    where (1.0 / x > 0.4)
      y = 1.0 / x
    elsewhere
      y = -1.0
    end where
  elsewhere
    y = 0.0
  end where
  print '(5F6.2)', y

  ! A masked ELSEWHERE's mask is computed when it is reached, after the
  ! block before it: V(5) is 100 by then, so the fourth element's test of
  ! V(2:7) holds.
  k = (/ (i, i = 1, 6) /)
  v = (/ (i, i = 1, 7) /)
  where (k > 4)
    v(1:6) = 100
  elsewhere (v(2:7) > 50)
    v(1:6) = -1
  elsewhere
    v(1:6) = 0
  end where
  print '(7I4)', v

  ! The statements of a block in turn: the WHERE statement after the
  ! assignment reads K's new values.
  k = (/ 1, 2, 3, 4, 5, 6 /)
  where (k > 2)
    k = k * 10
    where (k > 40) k = -k
  elsewhere (k == 1)
    k = 7
  end where
  print '(6I4)', k

  ! Two dimensions, sizes known only when the subroutine runs.
  s = reshape((/ 12.0, 6.0, 1.0, -3.0, -20.0, 0.0, &
                 7.0, 0.5, -11.0, 0.0, 15.0, -1.0 /), (/ 3, 4 /))
  call classify(s, c)
  do i = 1, 3
    print '(4I3)', c(i,:)
  end do

  ! In a FORALL, a row at a time, with names on the nested construct.
  g = reshape((/ 1, -2, 0, 4, 0, -6, -7, 8, 9 /), (/ 3, 3 /))
  forall (i = 1:3)
    rows: where (g(i,:) > 0)
      g(i,:) = g(i,:) * 10
    elsewhere (g(i,:) < -5) rows
      g(i,:) = 0
    elsewhere rows
      g(i,:) = 1
    end where rows
  end forall
  do i = 1, 3
    print '(3I4)', g(i,:)
  end do

contains

  ! C is 3 above 10, 2 above 5, 1 above 0, -2 below -10, -1 below 0, else
  ! 0; a nested WHERE in each kind of block.
  subroutine classify(x, c)
    real, intent(in) :: x(:,:)
    integer, intent(out) :: c(:,:)

    c = 0
    bysign: where (x > 0.0)
      bysize: where (x > 10.0)
        c = 3
      elsewhere (x > 5.0) bysize
        c = 2
      elsewhere bysize
        c = 1
      end where bysize
    elsewhere (x < 0.0) bysign
      c = -1
      where (x < -10.0) c = -2
    end where bysign
  end subroutine classify

end program wheres
