! fcalc, a scientific calculator: the kinds and limits every module shares.
!
! fcalc keeps every number as a COMPLEX of kind RK, the kind with at least
! 15 decimal digits and an exponent range of 300: G_floating on a VAX, where
! fcalc is compiled /G_FLOATING, and IEEE double elsewhere. (Under the VAX's
! default D_floating no kind has that range, and SELECTED_REAL_KIND's -2
! stops the compile: fcalc must be built /G_FLOATING.)
!
! The two machines' arithmetic differs at its edges: a VAX's largest G value
! is about 8.98E307 and IEEE's 1.80E308, a VAX has no denormal numbers, and
! a VAX traps on overflow and division by zero where IEEE makes an infinity.
! So fcalc never goes near either edge: no value it keeps is larger than
! LIMIT or, but for zero, smaller than SMALL, and every operation checks its
! operands before it is done (module FCVALUES).
module fckinds
  implicit none
  public

  integer, parameter :: rk = selected_real_kind(15, 300)

  ! The largest and smallest magnitudes fcalc keeps, well inside both
  ! machines' ranges (RANGE(1.0_RK) is 307 on both).
  real(rk), parameter :: limit = 1.0e300_rk
  real(rk), parameter :: small = 1.0e-300_rk

  ! LOG(LIMIT) and LOG(SMALL): EXP of anything outside them is out of range.
  real(rk), parameter :: log_limit = 690.775527898213705205_rk
  real(rk), parameter :: log_small = -690.775527898213705205_rk

  real(rk), parameter :: pi = 3.14159265358979323846264338328_rk
  real(rk), parameter :: e_value = 2.71828182845904523536028747135_rk

  ! The machine's epsilon, the same 2**-52 on both machines.
  real(rk), parameter :: eps = epsilon(1.0_rk)

  ! The longest line fcalc reads (VMS's FORTRAN reads a terminal's records
  ! of up to 199 bytes), and the longest name.
  integer, parameter :: line_length = 199
  integer, parameter :: name_length = 31

  ! The most significant digits a value is shown with.
  integer, parameter :: max_digits = 15
end module fckinds
