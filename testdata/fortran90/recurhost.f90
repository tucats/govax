! recurhost.f90: internal procedures of RECURSIVE hosts (docs/planning/PHASE-103 -
! Fortran 90 procedures and interfaces.md, subtask 4). Each activation of WALK
! has its own MINE and TAG in its stack frame, and its internal procedures,
! called after the deeper activations have returned, must reach that
! activation's: the host passes its FP as a hidden last argument, and REPORT
! passes it on to its sibling SHOW. TRI's internal procedure changes its host's
! RESULT variable and reads its dummy argument, both in each activation's frame.
program recurhost
  integer total, tri
  total = 0
  call walk(3, total)
  write (*, '(1x, a, i5)') 'total:', total
  write (*, '(1x, a, i8)') 'tri(5):', tri(5)
end program recurhost

recursive subroutine walk(depth, total)
  integer depth, total
  integer mine
  character(len=6) :: tag
  mine = depth * 10
  write (tag, '(a, i1)') 'level', depth
  if (depth > 1) call walk(depth - 1, total)
  call add
  call report
contains
  subroutine add
    total = total + mine
  end subroutine add

  subroutine report
    call show(mine)
  end subroutine report

  subroutine show(v)
    integer v
    write (*, '(1x, 2a, i4, i6)') tag, ':', v, total
  end subroutine show
end subroutine walk

recursive integer function tri(n) result(t)
  integer n
  t = 0
  if (n > 0) t = tri(n - 1)
  call bump
contains
  subroutine bump
    t = t + n
  end subroutine bump
end function tri
