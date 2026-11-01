C     Structure declarations and their layouts: the manual's examples
C     (sections 2.2.5.1, 4.15.1, 4.15.3), whose sizes it gives: STRA 5,
C     STRB 18, STR 11, DATE 4, APPOINTMENT 89, WORDS_LONG 6.
      PROGRAM LAYOUT
      STRUCTURE /STRA/
        CHARACTER*1 CHR
        INTEGER*4 INT
      END STRUCTURE
      STRUCTURE /STRB/
        REAL*8 FLT
        RECORD /STRA/ STR(2)
      END STRUCTURE
      STRUCTURE /STR/
        INTEGER*4 TAG
        UNION
          MAP
            REAL*4 FLT
            CHARACTER*2 CHR
          END MAP
          MAP
            INTEGER*2 INT
          END MAP
        END UNION
        LOGICAL*1 LOG
      END STRUCTURE
      STRUCTURE /DATE/
        LOGICAL*1 DAY, MONTH
        INTEGER*2 YEAR
      END STRUCTURE
      STRUCTURE /APPOINTMENT/
        RECORD /DATE/ APP_DATE
        STRUCTURE /TIME/ APP_TIME(2)
          LOGICAL*1 HOUR, MINUTE
        END STRUCTURE
        CHARACTER*20 APP_MEMO(4)
        LOGICAL*1 APP_FLAG
      END STRUCTURE
      STRUCTURE /WORDS_LONG/
        UNION
          MAP
            INTEGER*2 WORD_0, WORD_1, WORD_2
          END MAP
          MAP
            INTEGER*4 LONG
          END MAP
        END UNION
      END STRUCTURE
C     Initial values, %FILL, a PARAMETER inside, a nested structure with
C     no name, a union in a map, and the initial values of two maps of
C     one union, the later winning (section 4.15.3).
      STRUCTURE /ITEM/
        INTEGER*2 LEN /4/, CODE /'0319'X/
        PARAMETER (NB = 3)
        INTEGER*4 %FILL
        CHARACTER*4 NAME /'ABCD'/, PAD*2 /'X'/
        INTEGER*2 ARR(0:NB-1) /1, 2*7/
        STRUCTURE PAIR(2)
          BYTE LO, HI
        END STRUCTURE
        UNION
          MAP
            INTEGER*4 WHOLE /-1/
          END MAP
          MAP
            INTEGER*2 HALF(2) /2*5/
            UNION
              MAP
                LOGICAL*1 B1
              END MAP
            END UNION
          END MAP
        END UNION
      END STRUCTURE
C     A structure, a variable, and a field may share a name.
      DATE = 1.0
      END
