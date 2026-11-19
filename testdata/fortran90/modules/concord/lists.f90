! LISTS: a queue of line numbers, a linked list with pointers to both ends;
! LIST's components are private, so a user knows it only by its procedures.
MODULE LISTS
  IMPLICIT NONE
  PRIVATE
  PUBLIC :: LIST, START, APPEND, LAST, ITEMS, FREE, NODES
  TYPE ITEM
    INTEGER :: VALUE
    TYPE (ITEM), POINTER :: NEXT
  END TYPE ITEM
  TYPE LIST
    PRIVATE
    INTEGER :: COUNT
    TYPE (ITEM), POINTER :: HEAD, TAIL
  END TYPE LIST
  INTEGER :: NODES = 0
CONTAINS
  SUBROUTINE START (L)
    TYPE (LIST), INTENT (OUT) :: L
    L % COUNT = 0
    NULLIFY (L % HEAD, L % TAIL)
  END SUBROUTINE START

  SUBROUTINE APPEND (L, V)
    TYPE (LIST), INTENT (INOUT) :: L
    INTEGER, INTENT (IN) :: V
    TYPE (ITEM), POINTER :: NEW
    ALLOCATE (NEW)
    NODES = NODES + 1
    NEW % VALUE = V
    NULLIFY (NEW % NEXT)
    IF (ASSOCIATED (L % TAIL)) THEN
      L % TAIL % NEXT => NEW
    ELSE
      L % HEAD => NEW
    END IF
    L % TAIL => NEW
    L % COUNT = L % COUNT + 1
  END SUBROUTINE APPEND

  ! The last value appended, or 0 for an empty list.
  INTEGER FUNCTION LAST (L)
    TYPE (LIST), INTENT (IN) :: L
    LAST = 0
    IF (ASSOCIATED (L % TAIL)) LAST = L % TAIL % VALUE
  END FUNCTION LAST

  ! The list's values, as many as VALUES holds; N is how many there are.
  SUBROUTINE ITEMS (L, VALUES, N)
    TYPE (LIST), INTENT (IN) :: L
    INTEGER, INTENT (OUT) :: VALUES (:), N
    TYPE (ITEM), POINTER :: P
    N = 0
    P => L % HEAD
    DO WHILE (ASSOCIATED (P))
      N = N + 1
      IF (N <= SIZE (VALUES)) VALUES (N) = P % VALUE
      P => P % NEXT
    END DO
  END SUBROUTINE ITEMS

  SUBROUTINE FREE (L)
    TYPE (LIST), INTENT (INOUT) :: L
    TYPE (ITEM), POINTER :: P
    DO WHILE (ASSOCIATED (L % HEAD))
      P => L % HEAD
      L % HEAD => P % NEXT
      DEALLOCATE (P)
      NODES = NODES - 1
    END DO
    CALL START (L)
  END SUBROUTINE FREE
END MODULE LISTS
