! Phase 103's forms (docs/planning/PHASE-103 - Fortran 90 procedures and
! interfaces.md, subtask 2): internal procedures, interface blocks of every
! kind, keyword arguments in CALL, defined operators and their precedence,
! and A.B.C read by whether A is a record.
program procs
  structure /pair/
    real a, b
    record /pair/ inner
  end structure
  record /pair/ p, q(3)
  interface
    subroutine solve(a, tol)
      real a(*)
      real, optional :: tol
    end subroutine solve
    function area(r)
      real area, r
    end function
  end interface
  interface swap
    subroutine swapr(x, y)
      real x, y
    end subroutine
    module procedure swapi, swapc
  end interface
  interface operator(.cross.)
    function cross(u, v)
      real cross(3), u(3), v(3)
    end function cross
  end interface
  interface operator (+)
  end interface
  interface operator(==)
  end interface
  interface operator(/)
  end interface
  interface operator(//)
  end interface
  interface assignment(=)
  end interface
  call solve(x, tol=1.0e-6)
  call solve(tol=t, a=x)
  y = .inverse. a ** 2
  y = a + b .cross. c .eqv. d
  y = x.dot.z
  y = (x).dot.z + 1 .dot. 2
  y = p.a + q(2).inner.b
  y = p.a.dot.z
  y = x.a
contains
  subroutine inner(n)
    y = p.a.dot.z
    y = n.dot.z
  contains
  end subroutine inner
  real function twice(x)
    entry again(x)
    twice = 2*x
  end function twice
end program procs

subroutine host
  interface
    subroutine s(f)
      interface
        function f(x)
        end function
      end interface
      y = 1
      data z /1.0/
    end subroutine
    x = 1
  end interface
  end interface
  call s
contains
  subroutine a
  end
  x = 2
end subroutine host

subroutine nocontains
contains
end

subroutine open
  interface operator(=)
  end interface
  interface
    subroutine t
    end subroutine t
end

subroutine unended
  y = a .b1. c
contains
  subroutine u
  end subroutine u
program next
end program next
