C The same in fixed form.
      SUBROUTINE CONTINUED(ARG
     1UMENT)
      CHARACTER*40 TEXT
      TEXT = 'a character constant over three lines, with a ! in it'
      X = 1 +
     1    2
      Y = 3
      Z = 4
   30 FORMAT (6H!;&ab!, I3) ! the Hollerith text is not a comment
      CALL S (
     11, 2 )
      END
