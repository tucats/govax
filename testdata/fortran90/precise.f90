! precise.f90: a program portable to any Fortran 90 processor (docs/planning/
! PHASE-100 - Fortran 90 declarations and kinds.md, subtask 8). It asks for
! its working precision by SELECTED_REAL_KIND and SELECTED_INT_KIND rather
! than naming a size, so it prints the same on the VAX (D_floating) and under
! IEEE's arithmetic: every kind is a PARAMETER of a unit's own (there are no
! modules yet), every constant carries its kind, and every conversion says
! its KIND=. It finds a square root by Newton's method, pi by Simpson's rule
! over a function passed as an argument, a loan's payments, and a quadratic's
! complex roots.
program precise
  implicit none
  integer, parameter :: wp = selected_real_kind(p=12)
  integer, parameter :: ik = selected_int_kind(9), sk = selected_int_kind(2)
  integer, parameter :: steps = 64
  character(len=*), parameter :: rule = '----------------------------'
  real(kind=wp), parameter :: two = 2.0_wp, tolerance = 1.0e-12_wp
  real(wp) :: root, pi, balance, payment
  real(wp), external :: newton, simpson, curve
  integer(ik) :: tries, cents
  integer(kind=sk) :: month
  complex(wp) :: z1, z2
  logical :: enough

  enough = precision(root) >= 12 .and. range(root) >= 30
  write (*, '(1x, a, l2)') 'Precise enough:', enough
  write (*, '(1x, a)') rule

  root = newton(two, tolerance, tries)
  write (*, '(1x, a, f16.12, a, i2, a)') 'SQRT(2) =', root, ' after', tries, ' tries'
  write (*, '(1x, a, l2)') 'Squares back:', abs(root * root - two) < tolerance

  pi = 4.0_wp * simpson(curve, 0.0_wp, 1.0_wp, steps)
  write (*, '(1x, a, f13.9)') 'PI =', pi
  write (*, '(1x, a, i11)') 'PI * 10**8 =', nint(pi * 1.0e8_wp, kind=ik)
  write (*, '(1x, a)') rule

  balance = 1000.0_wp
  call payments(balance, 0.01_wp, 12_sk, payment)
  write (*, '(1x, a, f9.2)') 'Payment:', payment
  do month = 1_sk, 12_sk
     balance = balance * 1.01_wp - payment
     if (mod(month, 4_sk) == 0) then
        cents = nint(balance * 100, kind=ik)
        write (*, '(1x, a, i3, a, i7)') 'Month', month, ' owes cents:', cents
     end if
  end do
  write (*, '(1x, a, l2)') 'Paid off:', abs(balance) < 0.005_wp
  write (*, '(1x, a)') rule

  call roots(1.0_wp, 2.0_wp, 5.0_wp, z1, z2)
  write (*, '(1x, a, 2f8.3)') 'Root 1:', z1
  write (*, '(1x, a, 2f8.3)') 'Root 2:', z2
  write (*, '(1x, a, f8.3)') 'Their product:', real(z1 * z2, kind=wp)
  write (*, '(1x, a, i3)') 'Half the months:', int(real(month, kind=wp) / 2, kind=sk)
end program precise

! newton is the square root of a by Newton's method, stopping when two
! guesses are closer than tol; tries counts the steps.
function newton(a, tol, tries) result(x)
  implicit none
  integer, parameter :: wp = selected_real_kind(p=12), ik = selected_int_kind(9)
  real(wp), intent(in) :: a, tol
  integer(ik), intent(out) :: tries
  real(wp) :: x, last
  x = a
  last = 0
  tries = 0
  do while (abs(x - last) >= tol)
     last = x
     x = (x + a / x) / 2.0_wp
     tries = tries + 1
  end do
end function newton

! simpson integrates f from a to b over n intervals (n even).
function simpson(f, a, b, n)
  implicit real(selected_real_kind(p=12)) (a-h, o-z)
  integer, intent(in) :: n
  real(kind(a)), external :: f
  real(kind(a)) :: simpson
  h = (b - a) / n
  s = f(a) + f(b)
  do i = 1, n - 1
     if (mod(i, 2) == 1) then
        s = s + 4 * f(a + i * h)
     else
        s = s + 2 * f(a + i * h)
     end if
  end do
  simpson = s * h / 3
end function simpson

! curve is 1/(1+x**2), whose integral from 0 to 1 is pi/4.
real(selected_real_kind(p=12)) function curve(x)
  real(selected_real_kind(p=12)), intent(in) :: x
  curve = 1 / (1 + x * x)
end function curve

! payments is the monthly payment that pays off principal in months at
! the monthly rate.
subroutine payments(principal, rate, months, payment)
  implicit none
  integer, parameter :: wp = selected_real_kind(p=12), sk = selected_int_kind(2)
  real(wp), intent(in) :: principal, rate
  integer(sk), intent(in) :: months
  real(wp), intent(out) :: payment
  payment = principal * rate / (1 - (1 + rate) ** (-int(months)))
end subroutine payments

! roots gives the two roots of a*x**2 + b*x + c, complex or not.
subroutine roots(a, b, c, z1, z2)
  implicit none
  integer, parameter :: wp = selected_real_kind(p=12)
  real(wp), intent(in) :: a, b, c
  complex(wp), intent(out) :: z1, z2
  real(wp) :: d
  d = b * b - 4 * a * c
  if (d >= 0) then
     z1 = cmplx((-b + sqrt(d)) / (2 * a), 0, kind=wp)
     z2 = cmplx((-b - sqrt(d)) / (2 * a), 0, kind=wp)
  else
     z1 = cmplx(-b / (2 * a), sqrt(-d) / (2 * a), kind=wp)
     z2 = conjg(z1)
  end if
end subroutine roots
