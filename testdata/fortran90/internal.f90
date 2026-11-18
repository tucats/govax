! internal.f90: internal procedures of a main program (docs/planning/PHASE-103 -
! Fortran 90 procedures and interfaces.md, subtask 3). Each sees its host's
! names by host association: its variables, arrays, COMMON, and implicit rules.
! A name an internal procedure declares is its own, hiding the host's; one it
! only uses is the host's if the host has it, wherever the host uses it (STEPS
! and I, which the host first uses in its executable statements), and else its
! own (K). An internal function's type is its own declaration's (TOTAL is REAL
! though the host's letters make T INTEGER), or the host's implicit rules'
! (TWICE). A sibling calls a sibling, a recursive one calls itself, and the
! host's TWICE hides the file's external TWICE, which OTHER calls.
program internal
  implicit integer (a-z)
  real :: x(3) = (/ 1.5, 2.5, 3.0 /)
  common /tally/ calls
  character(len=5) :: tag = 'host '
  calls = 0
  steps = 0
  write (*, '(1x, a, f6.2)') 'sum:', total()
  call show('first')
  do i = 1, 3
    call bump
  end do
  write (*, '(1x, a, 2i4)') 'steps, i:', steps, i
  write (*, '(1x, a, i4)') 'twice(21):', twice(21)
  write (*, '(1x, a, i8)') 'fact(10):', fact(10)
  write (*, '(1x, 3a)') 'tag: [', tag, ']'
  call retag
  write (*, '(1x, 3a)') 'tag: [', tag, ']'
  write (*, '(1x, 3a)') '[', initials('Ada Lovelace'), ']'
  write (*, '(1x, a, i3)') 'calls:', calls
  call other
contains
  real function total()
    total = x(1) + x(2) + x(3)
    calls = calls + 1
  end function total

  subroutine show(what)
    character*(*) what
    integer steps
    steps = len(what)
    write (*, '(1x, 2a, i3)') what, ':', steps
    calls = calls + 1
  end subroutine show

  subroutine bump
    steps = steps + i
    calls = calls + 1
  end subroutine bump

  function twice(n)
    twice = n + n
  end function twice

  recursive function fact(n) result(f)
    if (n <= 1) then
      f = 1
    else
      f = n * fact(n - 1)
    end if
  end function fact

  subroutine retag
    tag(1:1) = 'H'
    call show(tag)
  end subroutine retag

  character*2 function initials(name)
    character*(*) name
    k = index(name, ' ')
    initials = name(1:1) // name(k+1:k+1)
  end function initials
end program internal

subroutine other
  integer twice
  write (*, '(1x, a, i4)') 'external twice(7):', twice(7)
end subroutine other

integer function twice(n)
  integer n
  twice = 3 * n
end function twice
