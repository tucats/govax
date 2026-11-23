! fcalc, a scientific calculator: the kinds and limits every module shares.
!
! A note for readers new to Fortran 90 (the rest of fcalc's files build on
! it). A program is units: a main program (fcalc.f90's PROGRAM FCALC) and
! any number of MODULEs. A module holds declarations (constants, types,
! variables) and, after CONTAINS, procedures; another unit gets them by
! naming the module in a USE statement, as `use fckinds`. A module must be
! compiled before any unit that USEs it, which is why compiles.txt lists
! fcalc's files in an order: each file uses only those before it.
!
! Every module starts IMPLICIT NONE: without it, Fortran gives an
! undeclared name a type by its first letter (I to N INTEGER, the rest
! REAL), so a misspelled name is silently a new variable. With it, every
! name must be declared, and a misspelling is a compile error.
!
! fcalc keeps every number as a COMPLEX of kind RK. In Fortran 90 a type
! may come in several KINDs, each a different precision or range: REAL(4)
! and REAL(8) are two kinds of REAL. Kind numbers differ from compiler to
! compiler, so a portable program never writes one; it asks for the
! precision it needs, with SELECTED_REAL_KIND(p, r): the kind with at least
! p significant decimal digits and a decimal exponent range of at least r.
! Here that is the kind with 15 digits and a range of 300, which is
! G_floating on a VAX, where fcalc is compiled /G_FLOATING, and IEEE double
! precision elsewhere. (Under the VAX's default D_floating REAL(8) reaches
! only 1E38, so the kind is REAL(16), H_floating, larger and slower; hence
! /G_FLOATING.) A constant of the kind is written with the kind after
! an underscore: 1.0_rk. Without it, 1.0 would be a default REAL, single
! precision, and 0.1 would be correct to only 7 digits.
!
! Why "every number is complex"? A calculator meets complex numbers as soon
! as it takes the square root of a negative number; keeping every value
! complex means one kind of value everywhere, with a real number just one
! whose imaginary part is zero.
!
! The two machines' arithmetic differs at its edges: a VAX's largest G value
! is about 8.98E307 and IEEE's 1.80E308, a VAX has no denormal numbers (the
! tiny values IEEE keeps below its smallest normal number), and a VAX traps
! (stops the program with a fault) on overflow and division by zero where
! IEEE quietly makes an infinity. fcalc must print the same lines on both,
! so it never goes near either edge: no value it keeps is larger than
! LIMIT or, but for zero, smaller than SMALL, and every operation checks its
! operands before it is done (module FCVALUES). This is a common discipline
! in numerical programming: decide the range your values live in, and make
! every operation either stay in it or report that it can't.
module fckinds
  implicit none

  ! PUBLIC: everything in this module may be used by a unit that USEs it.
  ! (The other modules start PRIVATE instead, and list what they share.)
  public

  ! A named constant: PARAMETER makes RK a constant fixed when compiling,
  ! so it may be used as a kind, in REAL(RK) and 1.0_RK.
  integer, parameter :: rk = selected_real_kind(15, 300)

  ! The largest and smallest magnitudes fcalc keeps, well inside both
  ! machines' ranges (RANGE(1.0_RK) is 307 on both).
  real(rk), parameter :: limit = 1.0e300_rk
  real(rk), parameter :: small = 1.0e-300_rk

  ! LOG(LIMIT) and LOG(SMALL): EXP of anything outside them is out of
  ! range. (Written out: a constant's value in Fortran 90 may call only the
  ! intrinsic functions of INTEGER and CHARACTER values, and inquiries such
  ! as EPSILON below, not LOG of a REAL.)
  real(rk), parameter :: log_limit = 690.775527898213705205_rk
  real(rk), parameter :: log_small = -690.775527898213705205_rk

  ! More digits than the kind holds: the compiler rounds them to the
  ! nearest value the kind has, the best it can do.
  real(rk), parameter :: pi = 3.14159265358979323846264338328_rk
  real(rk), parameter :: e_value = 2.71828182845904523536028747135_rk

  ! The machine's epsilon, the difference between 1 and the next larger
  ! number of the kind: 2**-52 on both machines. EPSILON is an inquiry
  ! function: it looks only at its argument's kind, never its value.
  real(rk), parameter :: eps = epsilon(1.0_rk)

  ! The longest line fcalc reads (VMS's FORTRAN reads a terminal's records
  ! of up to 199 bytes), and the longest name.
  integer, parameter :: line_length = 199
  integer, parameter :: name_length = 31

  ! The most significant digits a value is shown with.
  integer, parameter :: max_digits = 15

  ! The largest whole number fcalc counts as one: below 2**52, where every
  ! whole number of the kind is exact on both machines and the next one up
  ! differs by 1. A value this large or larger is never shown in another
  ! base or used as a count.
  real(rk), parameter :: whole_limit = 4.0e15_rk
end module fckinds
