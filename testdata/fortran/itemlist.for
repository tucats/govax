C     An item list, as $GETJPIW takes one, built from a structure (Phase 82,
C     subtask 9): each item a word of length, a word of code, and two addresses,
C     and a longword of 0 at the end. Phase 83 hands it to the service; here its
C     bytes are read back through the union's other maps. Under /ALIGN and
C     /WARN=ALIGNMENT nothing moves and nothing is said, since an item list is
C     naturally aligned as it stands.
      PROGRAM ITEMLIST
CDEC$ OPTIONS /ALIGN /WARN=ALIGNMENT
      STRUCTURE /ITEM/
        INTEGER*2 BUFLEN
        INTEGER*2 CODE
        INTEGER*4 BUFADR
        INTEGER*4 RETADR
      END STRUCTURE
      STRUCTURE /ITMLST/
        UNION
          MAP
            RECORD /ITEM/ ITEMS(2)
            INTEGER*4 LAST /0/
          END MAP
          MAP
            BYTE RAW(28)
          END MAP
          MAP
            INTEGER*4 LONGS(7)
          END MAP
        END UNION
      END STRUCTURE
CDEC$ END OPTIONS
      RECORD /ITMLST/ L
      INTEGER*4 PID, PIDLEN
      INTEGER*2 USERLEN
      CHARACTER*12 USER
      L.ITEMS(1).BUFLEN = 4
      L.ITEMS(1).CODE = '319'X
      L.ITEMS(1).BUFADR = %LOC(PID)
      L.ITEMS(1).RETADR = %LOC(PIDLEN)
      L.ITEMS(2).BUFLEN = LEN(USER)
      L.ITEMS(2).CODE = '202'X
      L.ITEMS(2).BUFADR = %LOC(USER)
      L.ITEMS(2).RETADR = %LOC(USERLEN)
      PRINT *, 'SIZE', SIZEOF(L)
C     The words, low byte first.
      PRINT *, 'FIRST', (L.RAW(I), I = 1, 4)
      PRINT *, 'SECOND', (L.RAW(I), I = 13, 16)
      PRINT *, 'LONGS', L.LONGS(1), L.LONGS(4), L.LONGS(7)
C     The addresses, each at its offset in its item.
      IF (L.LONGS(2) .EQ. %LOC(PID)) PRINT *, 'BUFADR 1'
      IF (L.LONGS(3) .EQ. %LOC(PIDLEN)) PRINT *, 'RETADR 1'
      IF (L.LONGS(5) .EQ. %LOC(USER)) PRINT *, 'BUFADR 2'
      IF (L.LONGS(6) .EQ. %LOC(USERLEN)) PRINT *, 'RETADR 2'
      END
