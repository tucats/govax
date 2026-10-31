      PROGRAM DIALOG
C     An interactive program at the terminal (Phase 80, subtask 14):
C     prompts left open by $ and their replies, a command loop ended by
C     the input's end (END=), ACCEPT of a reply with A and of numbers
C     list-directed (a repeat count, a null value keeping what an item
C     held, a slash ending the list early), a bad number taken by
C     IOSTAT=, and a NAMELIST group read from the terminal and written
C     back.
      CHARACTER*20 WHO
      CHARACTER*1 CMD
      REAL A(5), S
      INTEGER K, I, IOS
      LOGICAL VERBOSE
      NAMELIST /SETUP/ K, VERBOSE, A
      DATA A /5*9.0/, K /0/, VERBOSE /.FALSE./
   10 FORMAT (' Command: ', $)
   20 FORMAT (1X, 'Sum of the first ', I1, ': ', F8.3)
      TYPE '('' Your name? '', $)'
      ACCEPT '(A)', WHO
      TYPE *, 'Hello, ', WHO(:INDEX(WHO, ' ') - 1), '.'
  100 TYPE 10
      READ (5, '(A)', END=900) CMD
      IF (CMD .EQ. 'A') THEN
         TYPE '('' Five numbers: '', $)'
         ACCEPT *, A
      ELSE IF (CMD .EQ. 'S') THEN
         TYPE '('' How many? '', $)'
         READ (5, *, IOSTAT=IOS) K
         IF (IOS .NE. 0) THEN
            TYPE *, 'Not a number:', IOS
         ELSE
            S = 0.0
            DO I = 1, K
               S = S + A(I)
            END DO
            PRINT 20, K, S
         END IF
      ELSE IF (CMD .EQ. 'N') THEN
         READ (5, SETUP)
      ELSE IF (CMD .EQ. 'L') THEN
         WRITE (6, SETUP)
      ELSE
         TYPE *, 'Unknown command: ', CMD
      END IF
      GO TO 100
  900 TYPE *, 'Goodbye.'
      END
