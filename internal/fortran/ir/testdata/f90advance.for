      OPTIONS /F90
C     Fortran 90's nonadvancing I/O (Phase 109, subtask 5): ADVANCE='NO'
C     a control block flag; SIZE= a longword FTN$IO_END fills, then stored
C     in the variable; EOR= a branch on -2 before the Sign that END= and
C     ERR= take; IOSTAT= alone with END= -2 going on; and ADVANCE= by a
C     value, FTN$ADVANCE choosing one of three begin calls.
      PROGRAM F90ADV
      CHARACTER*8 S, A
      INTEGER*2 N
      WRITE (6, '(A)', ADVANCE='NO') S
      READ (5, '(A)', ADVANCE='NO', SIZE=N, EOR=10, END=20) S
   10 READ (5, '(A)', ADVANCE='NO', IOSTAT=I, END=20) S
   20 READ (5, '(A)', ADVANCE=A) S
      END
