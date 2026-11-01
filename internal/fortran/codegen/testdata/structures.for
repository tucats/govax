C     Structure declarations: each one's layout is in the storage map.
C     A structure makes no storage, so the program's variables are as
C     they would be without them. (Records are Phase 82's subtask 3.)
      PROGRAM SHAPES
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
      DATE = 1.5
      END
