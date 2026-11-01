C USEROPEN - OPEN's USEROPEN= (section 9.1.28), CONVERT= (the 2005
C manual's; NATIVE only, as yet), SHARED, and READONLY: a routine of the
C program's own opening a unit's file through RMS, given the FAB, the RAB,
C and the unit; records written and read through the unit it opened; a
C routine's failure; and each failure's IOSTAT the number VMS's FORTRAN
C run-time library gave in testdata/for81's runs.
      PROGRAM USEROP
      EXTERNAL MAKEIT, FAILIT
      INTEGER IOS, N, CALLS, UNIT
      CHARACTER*16 TEXT, CONV
      COMMON /SEEN/ CALLS, UNIT
C
C The routine makes the file; FTNRTL then reads and writes it as any other.
C
      OPEN (1, FILE='UO.DAT', STATUS='NEW', USEROPEN=MAKEIT)
      WRITE (1, '(A)') 'FIRST', 'SECOND'
      REWIND 1
      READ (1, '(A)') TEXT
      WRITE (6, '(1X,A,I2,I3,1X,A)') 'CALLED', CALLS, UNIT, TEXT
      CLOSE (1, DISPOSE='DELETE')
C
C A routine that fails fails the OPEN: FOR$_OPEFAI.
C
      OPEN (2, FILE='UO2.DAT', STATUS='NEW', USEROPEN=FAILIT,
     1 IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'FAILED', IOS
C
C CONVERT='NATIVE' is what an unformatted unit has anyway; any other
C conversion isn't here yet (FOR$_NOTIMP), and a name that isn't one is
C FOR$_KEYVALERR.
C
      OPEN (3, FILE='CV.DAT', STATUS='NEW', FORM='UNFORMATTED',
     1 CONVERT='NATIVE')
      INQUIRE (3, CONVERT=CONV)
      WRITE (6, '(1X,A,1X,A)') 'CONVERT', CONV
      CLOSE (3, DISPOSE='DELETE')
      OPEN (3, FILE='CV.DAT', STATUS='NEW', FORM='UNFORMATTED',
     1 CONVERT='VAXG', IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'VAXG', IOS
      CONV = 'OTHER'
      OPEN (3, FILE='CV.DAT', STATUS='NEW', FORM='UNFORMATTED',
     1 CONVERT=CONV, IOSTAT=IOS)
      WRITE (6, '(1X,A,I3)') 'OTHER', IOS
C
C SHARED: two units on one file, each with its own place in it.
C READONLY: no writes (FOR$_WRIREAFIL).
C
      OPEN (4, FILE='SH.DAT', STATUS='NEW', SHARED)
      WRITE (4, '(A)') 'ONE', 'TWO'
      OPEN (5, FILE='SH.DAT', STATUS='OLD', SHARED, READONLY)
      READ (5, '(A)') TEXT
      WRITE (6, '(1X,A,1X,A)') 'SHARED', TEXT
      WRITE (5, '(A)', IOSTAT=IOS) 'THREE'
      WRITE (6, '(1X,A,I3)') 'READONLY', IOS
      CLOSE (5)
      CLOSE (4, DISPOSE='DELETE')
      END
C
C MAKEIT is a USEROPEN routine: a function, given the FAB, the RAB, and the
C unit, each by reference, that creates and connects the file, returning
C RMS's status; it counts its calls and keeps the unit.
C
      INTEGER FUNCTION MAKEIT(FAB, RAB, LUN)
      INTEGER FAB(20), RAB(17), LUN
      INTEGER SYS$CREATE, SYS$CONNECT, CALLS, UNIT
      COMMON /SEEN/ CALLS, UNIT
      CALLS = CALLS + 1
      UNIT = LUN
      MAKEIT = SYS$CREATE(FAB)
      IF (MOD(MAKEIT, 2) .EQ. 1) MAKEIT = SYS$CONNECT(RAB)
      END
C
C FAILIT fails as RMS would for a file that isn't there: RMS$_FNF, 18292 (hex).
C
      INTEGER FUNCTION FAILIT(FAB, RAB, LUN)
      INTEGER FAB(20), RAB(17), LUN
      FAILIT = 98962
      END
