! examples.f90: the standard's own examples of the IF, CASE, and DO constructs
! (ISO/IEC 1539:1991, 8.1.2.3, 8.1.3.3, and 8.1.4.5) made into a program
! (docs/planning/PHASE-101 - Fortran 90 control constructs.md, subtask 5).
! Each fragment is as the standard prints it, in its capitals, but for what
! makes it run: unit IUN a scratch file written first, SUBA counting and
! summing what it is given (SUBB to SUBZ left out), SUBNEG counting, CALCULATE
! giving back its argument, the standard's STOP an EXIT, its ARRAY(1:M) an
! implied DO, and its "..." gone. The standard gives the results of examples
! 5, 6, 9, and 10; the others are worked out by hand.
program examples
  implicit none
  integer, parameter :: iun = 10
  character(len=8) :: cvar, texts(3)
  character(len=80) :: line
  logical :: prop, nextprop
  real :: a, b, c, d, x, sum, sum_max, result, array(10)
  real :: as(3), cs(3), values(5)
  integer :: i, j, k, l, m, n, ios, level, iqual, iqual_min, silly, calls, negs
  real :: total
  character(len=16) :: said
  integer :: nsaid
  integer, external :: signum
  common /counts/ calls, negs, total
  common /saidc/ said
  common /saidn/ nsaid
  data as / 2.0, -1.0, -2.0 /, cs / 4.0, 3.0, -5.0 /
  data texts / '(a(b)cd)', '(()abcde', 'ab)(cdef' /
  data values / 1.5, -2.0, 3.25, -0.5, 4.0 /

  ! 8.1.2.3: IF constructs.
  CVAR = 'RESET'
  I = 1; J = 2; K = 3
  IF (CVAR .EQ. 'RESET') THEN
  I = 0; J = 0; K = 0
  END IF
  write (*, '(1x, a, 3i2)') 'reset:', i, j, k

  prop = .false.
  nextprop = .true.
  proofs: do n = 1, 5
     PROOF_DONE: IF (PROP) THEN
     WRITE (*, '(" QED")')
     exit proofs
     ELSE
     PROP = NEXTPROP
     END IF PROOF_DONE
  end do proofs
  write (*, '(1x, a, i2)') 'proved at', n

  do n = 1, 3
     a = as(n)
     c = cs(n)
     IF (A .GT. 0) THEN
     B = C/A
     IF (B .GT. 0) THEN
     D = 1.0
     END IF
     ELSE IF (C .GT. 0) THEN
     B = A/C
     D = -1.0
     ELSE
     B = ABS (MAX (A, C))
     D = 0
     END IF
     write (*, '(1x, a, 2f8.4)') 'abcd:', b, d
  end do

  ! 8.1.3.3: CASE constructs.
  write (*, '(1x, a, 3i3)') 'signum:', signum(-5), signum(0), signum(7)

  do m = 1, 3
     line = texts(m)
     write (*, '(1x, 2a)') 'parens: ', line(1:8)
     LEVEL=0
     DO I = 1, 80
     CHECK_PARENS: SELECT CASE (LINE (I:I))
     CASE ('(')
     LEVEL = LEVEL + 1
     CASE (')')
     LEVEL = LEVEL - 1
     IF (LEVEL .LT. 0) THEN
     PRINT *, 'UNEXPECTED RIGHT PARENTHESIS'
     EXIT
     END IF
     CASE DEFAULT
     ! Ignore all other characters
     END SELECT CHECK_PARENS
     END DO
     IF (LEVEL .GT. 0) THEN
     PRINT *, 'MISSING RIGHT PARENTHESIS'
     END IF
  end do

  ! The three equivalent fragments, THIS saying I and THAT A.
  do silly = 1, 2
     nsaid = 0
     IF (SILLY .EQ. 1) THEN
     CALL THIS
     ELSE
     CALL THAT
     END IF
     SELECT CASE (SILLY .EQ. 1)
     CASE (.TRUE.)
     CALL THIS
     CASE (.FALSE.)
     CALL THAT
     END SELECT
     SELECT CASE (SILLY)
     CASE DEFAULT
     CALL THAT
     CASE (1)
     CALL THIS
     END SELECT
     write (*, '(1x, a, i2, 2a)') 'silly', silly, ': ', said(1:nsaid)
  end do

  ! Several selections of one block, SUB saying 1 and OTHER 0.
  nsaid = 0
  do n = 0, 9
     SELECT CASE (N)
     CASE (1, 3:5, 8) ! Selects 1, 3, 4, 5, 8
     CALL SUB
     CASE DEFAULT
     CALL OTHER
     END SELECT
  end do
  write (*, '(1x, 2a)') 'selects: ', said(1:nsaid)

  ! 8.1.4.5: DO constructs. Unit IUN holds five numbers, two negative.
  open (iun, status='scratch')
  do i = 1, 5
     write (iun, '(1x, g14.7)') values(i)
  end do

  ! Example 2.
  rewind (iun)
  calls = 0; negs = 0; total = 0.0
  READ (IUN, '(1X, G14.7)', IOSTAT = IOS) X
  DO WHILE (IOS .EQ. 0)
  IF (X .GE. 0.) THEN
  CALL SUBA (X)
  ENDIF
  READ (IUN, '(1X, G14.7)', IOSTAT = IOS) X
  END DO
  write (*, '(1x, a, 2i3, f7.2)') 'example 2:', calls, negs, total

  ! Example 3.
  rewind (iun)
  calls = 0; negs = 0; total = 0.0
  DO ! A "DO WHILE + 1/2" loop
  READ (IUN, '(1X, G14.7)', IOSTAT = IOS) X
  IF (IOS .NE. 0) EXIT
  IF (X < 0.) CYCLE
  CALL SUBA (X)
  END DO
  write (*, '(1x, a, 2i3, f7.2)') 'example 3:', calls, negs, total

  ! Example 7.
  rewind (iun)
  calls = 0; negs = 0; total = 0.0
  DO 70
  READ (IUN, '(1X, G14.7)', IOSTAT = IOS) X
  IF (IOS .NE. 0) EXIT
  IF (X < 0.) GOTO 70
  CALL SUBA (X)
  CYCLE
70 CALL SUBNEG (X) ! SUBNEG called only when X < 0.
  write (*, '(1x, a, 2i3, f7.2)') 'example 7:', calls, negs, total
  close (iun)

  ! Unit IUN, unformatted, holds three records of IQUAL, M, and M values.
  open (iun, status='scratch', form='unformatted')
  write (iun) 3
  write (iun) 5, 3, 1.0, -2.0, 3.0
  write (iun) 1, 2, 100.0, 100.0
  write (iun) 4, 4, 4.0, 5.0, 6.0, 7.0
  iqual_min = 2
  sum_max = 12.0

  ! Example 4.
  rewind (iun)
  SUM = 0.0
  READ (IUN) N
  OUTER: DO L = 1, N ! A DO with a construct name
  READ (IUN) IQUAL, M, (ARRAY (I), I = 1, M)
  IF (IQUAL < IQUAL_MIN) CYCLE OUTER ! Skip inner loop
  INNER: DO 40 I = 1, M ! A DO with a label and a name
  CALL CALCULATE (ARRAY (I), RESULT)
  IF (RESULT < 0.0) CYCLE
  SUM = SUM + RESULT
  IF (SUM > SUM_MAX) EXIT OUTER
40 END DO INNER
  END DO OUTER
  write (*, '(1x, a, f7.2, 2i3)') 'example 4:', sum, l, i

  ! Example 8.
  rewind (iun)
  SUM = 0.0
  READ (IUN) N
  DO 80, L = 1, N
  READ (IUN) IQUAL, M, (ARRAY (I), I = 1, M)
  IF (IQUAL < IQUAL_MIN) M = 0 ! Skip inner loop
  DO 80 I = 1, M
  CALL CALCULATE (ARRAY (I), RESULT)
  IF (RESULT < 0.) CYCLE
  SUM = SUM + RESULT
  IF (SUM > SUM_MAX) GOTO 81
80 CONTINUE ! This CONTINUE is shared by both loops
81 CONTINUE
  write (*, '(1x, a, f7.2, 2i3)') 'example 8:', sum, l, i
  close (iun)

  ! Example 5.
  N = 0
  DO 50, I = 1, 10
  J = I
  DO K = 1, 5
  L = K
  N = N + 1 ! This statement executes 50 times
  END DO ! Nonlabeled DO inside a labeled DO
50 CONTINUE
  write (*, '(1x, a, 5i4)') 'example 5:', i, j, k, l, n

  ! Example 6.
  N = 0
  DO I = 1, 10
  J = I
  DO 60, K = 5, 1 ! This inner loop is never executed
  L = K
  N = N + 1
60 CONTINUE ! Labeled DO inside a nonlabeled DO
  END DO
  write (*, '(1x, a, 4i4)') 'example 6:', i, j, k, n

  ! Example 9.
  N = 0
  DO 100 I = 1, 10
  J = I
  DO 100 K = 1, 5
  L = K
100 N = N + 1 ! This statement executes 50 times
  write (*, '(1x, a, 5i4)') 'example 9:', i, j, k, l, n

  ! Example 10.
  N = 0
  DO 200 I = 1, 10
  J = I
  DO 200 K = 5, 1 ! This inner loop is never executed
  L = K
200 N = N + 1
  write (*, '(1x, a, 4i4)') 'example 10:', i, j, k, n
end program examples

! 8.1.3.3's integer signum function.
INTEGER FUNCTION SIGNUM (N)
SELECT CASE (N)
CASE (:-1)
SIGNUM = -1
CASE (0)
SIGNUM = 0
CASE (1:)
SIGNUM = 1
END SELECT
END

subroutine note(c)
  implicit none
  character(len=1), intent(in) :: c
  character(len=16) :: said
  integer :: nsaid
  common /saidc/ said
  common /saidn/ nsaid
  nsaid = nsaid + 1
  said(nsaid:nsaid) = c
end subroutine note

subroutine this
  call note('I')
end subroutine this

subroutine that
  call note('A')
end subroutine that

subroutine sub
  call note('1')
end subroutine sub

subroutine other
  call note('0')
end subroutine other

subroutine suba(x)
  implicit none
  real, intent(in) :: x
  integer :: calls, negs
  real :: total
  common /counts/ calls, negs, total
  calls = calls + 1
  total = total + x
end subroutine suba

subroutine subneg(x)
  implicit none
  real, intent(in) :: x
  integer :: calls, negs
  real :: total
  common /counts/ calls, negs, total
  negs = negs + 1
end subroutine subneg

subroutine calculate(x, result)
  implicit none
  real, intent(in) :: x
  real, intent(out) :: result
  result = x
end subroutine calculate
