! WORDS: what CONCORD calls a word, and how a line is cut into them.
MODULE WORDS
  IMPLICIT NONE
  INTEGER, PARAMETER :: WORD_LENGTH = 12
CONTAINS
  ! The next word of LINE at or after POS, in capitals: a run of letters,
  ! anything else a separator, and POS moved past it; WORD all blanks when
  ! the line has no more.
  SUBROUTINE NEXT_WORD (LINE, POS, WORD)
    CHARACTER (LEN = *), INTENT (IN) :: LINE
    INTEGER, INTENT (INOUT) :: POS
    CHARACTER (LEN = WORD_LENGTH), INTENT (OUT) :: WORD
    INTEGER :: N
    WORD = ' '
    DO WHILE (POS <= LEN (LINE))
      IF (LETTER (LINE (POS:POS))) EXIT
      POS = POS + 1
    END DO
    N = 0
    DO WHILE (POS <= LEN (LINE))
      IF (.NOT. LETTER (LINE (POS:POS))) EXIT
      N = N + 1
      IF (N <= WORD_LENGTH) WORD (N:N) = CAPITAL (LINE (POS:POS))
      POS = POS + 1
    END DO
  END SUBROUTINE NEXT_WORD

  LOGICAL FUNCTION LETTER (C)
    CHARACTER, INTENT (IN) :: C
    LETTER = (C >= 'A' .AND. C <= 'Z') .OR. (C >= 'a' .AND. C <= 'z')
  END FUNCTION LETTER

  CHARACTER FUNCTION CAPITAL (C)
    CHARACTER, INTENT (IN) :: C
    CAPITAL = C
    IF (C >= 'a' .AND. C <= 'z') CAPITAL = CHAR (ICHAR (C) - ICHAR ('a') + ICHAR ('A'))
  END FUNCTION CAPITAL
END MODULE WORDS
