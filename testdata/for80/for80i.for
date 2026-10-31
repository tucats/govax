C     FOR80I.FOR - written by testdata/for80/gen.go: round 1's
C     formatted and list-directed input cases, from SYS$INPUT, for govax.
C     TestFor80InputOracle holds what they store to vax/for80-vax.txt.
      PROGRAM FOR80I
      CALL R1
      CALL R2
      CALL R3
      CALL R4
      CALL R5
      CALL R6
      CALL R7
      CALL R8
      END

      SUBROUTINE R1
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#538')
      CALL PRE
      READ (5, '(I4)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=538', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#539')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=539', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#540')
      CALL PRE
      READ (5, '(I9)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=540', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#541')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=541', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#542')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=542', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#543')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=543', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#544')
      CALL PRE
      READ (5, '(BZ,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=544', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#545')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=545', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#546')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=546', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#547')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=547', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#548')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=548', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#549')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=549', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#550')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=550', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#551')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=551', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#552')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=552', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#553')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=553', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#554')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=554', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#555')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=555', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#556')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=556', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#557')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=557', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#558')
      CALL PRE
      READ (5, '(I5)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=558', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#559')
      CALL PRE
      READ (5, '(I5)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=559', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#560')
      CALL PRE
      READ (5, '(I6)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=560', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#561')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=561', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#562')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=562', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#563')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=563', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#564')
      CALL PRE
      READ (5, '(I4.2)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=564', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#565')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=565', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#566')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=566', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#567')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=567', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#568')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=568', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#569')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=569', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#570')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=570', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#571')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=571', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#572')
      CALL PRE
      READ (5, '(O5)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=572', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#573')
      CALL PRE
      READ (5, '(O4)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=573', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#574')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=574', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#575')
      CALL PRE
      READ (5, '(O)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=575', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#576')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=576', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#577')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=577', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R2
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#578')
      CALL PRE
      READ (5, '(O12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=578', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#579')
      CALL PRE
      READ (5, '(O12)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=579', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#580')
      CALL PRE
      READ (5, '(O6)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=580', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#581')
      CALL PRE
      READ (5, '(O6)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=581', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#582')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=582', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#583')
      CALL PRE
      READ (5, '(O12)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=583', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#584')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=584', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#585')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=585', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#586')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=586', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#587')
      CALL PRE
      READ (5, '(Z)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=587', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#588')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=588', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#589')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=589', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#590')
      CALL PRE
      READ (5, '(Z8)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=590', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#591')
      CALL PRE
      READ (5, '(Z9)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=591', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#592')
      CALL PRE
      READ (5, '(Z4)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=592', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#593')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=593', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#594')
      CALL PRE
      READ (5, '(Z8)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=594', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#595')
      CALL PRE
      READ (5, '(Z16)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=595', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#596')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=596', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#597')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=597', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#598')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=598', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#599')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=599', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#600')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=600', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#601')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=601', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#602')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=602', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#603')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=603', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#604')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=604', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#605')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=605', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#606')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=606', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#607')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=607', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#608')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=608', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#609')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=609', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#610')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=610', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#611')
      CALL PRE
      READ (5, '(L)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=611', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#612')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VWU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=612', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#613')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=613', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#614')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=614', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#615')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=615', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#616')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=616', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#617')
      CALL PRE
      READ (5, '(F9.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=617', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R3
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#618')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=618', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#619')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=619', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#620')
      CALL PRE
      READ (5, '(F8.0)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=620', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#621')
      CALL PRE
      READ (5, '(F8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=621', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#622')
      CALL PRE
      READ (5, '(F8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=622', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#623')
      CALL PRE
      READ (5, '(BN,F8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=623', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#624')
      CALL PRE
      READ (5, '(BZ,F8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=624', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#625')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=625', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#626')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=626', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#627')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=627', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#628')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=628', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#629')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=629', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#630')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=630', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#631')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=631', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#632')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=632', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#633')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=633', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#634')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=634', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#635')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=635', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#636')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=636', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#637')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=637', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#638')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=638', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#639')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=639', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#640')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=640', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#641')
      CALL PRE
      READ (5, '(E9.3)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=641', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#642')
      CALL PRE
      READ (5, '(E12.4)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=642', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#643')
      CALL PRE
      READ (5, '(E15.3)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=643', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#644')
      CALL PRE
      READ (5, '(E12.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=644', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#645')
      CALL PRE
      READ (5, '(BZ,D10.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=645', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#646')
      CALL PRE
      READ (5, '(D10.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=646', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#647')
      CALL PRE
      READ (5, '(D15.3)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=647', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#648')
      CALL PRE
      READ (5, '(G13.6)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=648', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#649')
      CALL PRE
      READ (5, '(G13.6)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=649', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#650')
      CALL PRE
      READ (5, '(F)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=650', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#651')
      CALL PRE
      READ (5, '(E)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=651', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#652')
      CALL PRE
      READ (5, '(G)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=652', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#653')
      CALL PRE
      READ (5, '(D)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=653', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#654')
      CALL PRE
      READ (5, '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=654', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#655')
      CALL PRE
      READ (5, '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=655', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#656')
      CALL PRE
      READ (5, '(-3PE10.5)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=656', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#657')
      CALL PRE
      READ (5, '(2PF8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=657', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R4
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#658')
      CALL PRE
      READ (5, '(2PF8.2)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=658', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#659')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=659', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#661')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=661', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 4)
      CALL SYNC('#662')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=662', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#663')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=663', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#664')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=664', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#665')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) TX(1)(1:4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=665', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 4)
      CALL SYNC('#666')
      CALL PRE
      READ (5, '(E9.3)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=666', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#668')
      CALL PRE
      READ (5, '(E12.3)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=668', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 4)
      CALL SYNC('#669')
      CALL PRE
      READ (5, '(2F8.5)', IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=669', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#670')
      CALL PRE
      READ (5, '(E9.1,F9.3)', IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=670', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#671')
      CALL PRE
      READ (5, '(2F8.5)', IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=671', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 4)
      CALL SYNC('#672')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=672', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#673')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=673', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#674')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=674', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#675')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=675', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 6)
      CALL SYNC('#676')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:8)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=676', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 8)
      CALL SYNC('#677')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=677', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#678')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=678', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#679')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=679', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#680')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=680', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#681')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=681', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#682')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=682', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 4)
      CALL SYNC('#683')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=683', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#684')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=684', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 5)
      CALL SYNC('#685')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:8)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=685', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 8)
      CALL SYNC('#686')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=686', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#687')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=687', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#688')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=688', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#689')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=689', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#690')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=690', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#691')
      CALL PRE
      READ (5, '(T7,A3,T1,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=691', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL SYNC('#692')
      CALL PRE
      READ (5, '(TR2,A1,TL2,A1)', IOSTAT=IOS) TX(1)(1:1), TX(2)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=692', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SHOWT(1, 1)
      CALL SYNC('#693')
      CALL PRE
      READ (5, '(1X,I2,3X,I2)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=693', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#694')
      CALL PRE
      READ (5, '(T50,A1)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=694', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#695')
      CALL PRE
      READ (5, '(TL50,A1)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=695', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#696')
      CALL PRE
      READ (5, '(Q,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=696', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL SYNC('#697')
      CALL PRE
      READ (5, '(A2,Q)', IOSTAT=IOS) TX(1)(1:2), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=697', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 2)
      CALL SHOWV(1, 1)
      CALL SYNC('#698')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=698', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#699')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=699', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R5
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#700')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=700', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#701')
      CALL PRE
      READ (5, '(5HXXXXX,A1)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=701', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#702')
      CALL PRE
      READ (5, '(''XXXXX'',A1)', IOSTAT=IOS) TX(1)(1:1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=702', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 1)
      CALL SYNC('#703')
      CALL PRE
      READ (5, '(2I6,2F10.2)', IOSTAT=IOS) S(1,1), S(1,2), VF(1,3), VF(1
     &,4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=703', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SYNC('#704')
      CALL PRE
      READ (5, '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=704', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#705')
      CALL PRE
      READ (5, '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=705', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#706')
      CALL PRE
      READ (5, '(I6,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=706', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL SYNC('#707')
      CALL PRE
      READ (5, '(A3,I3)', IOSTAT=IOS) TX(1)(1:3), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=707', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SHOWV(1, 1)
      CALL SYNC('#708')
      CALL PRE
      READ (5, '(L6,I3)', IOSTAT=IOS) VLU(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=708', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#709')
      CALL PRE
      READ (5, '(O6,Z6,I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=709', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#710')
      CALL PRE
      READ (5, '(I3,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=710', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#711')
      CALL PRE
      READ (5, '(3I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=711', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#712')
      CALL PRE
      READ (5, '(A3,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=712', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL SYNC('#713')
      CALL PRE
      READ (5, '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=713', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#714')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=714', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#715')
      CALL PRE
      READ (5, '(I3,:,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=715', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#716')
      CALL PRE
      READ (5, '(I3,$)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=716', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#717')
      CALL PRE
      READ (5, '(SP,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=717', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#718')
      CALL PRE
      READ (5, '(BN,I3,BZ,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=718', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#719')
      CALL PRE
      READ (5, '(BZ,I3,BN,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=719', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#732')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=732', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#733')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=733', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#734')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=734', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#735')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=735', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#736')
      CALL PRE
      READ (5, '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=736', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#737')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=737', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#738')
      CALL PRE
      READ (5, '(/,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=738', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#739')
      CALL PRE
      READ (5, '(//,I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=739', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#740')
      CALL PRE
      READ (5, '(I3,/)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=740', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#741')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=741', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 5)
      CALL SYNC('#742')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=742', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 5)
      CALL SYNC('#743')
      CALL PRE
      READ (5, '(5HXXXXX)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=743', IOS, 
     &IE, IS, IV, IU, IC
      CALL SYNC('#744')
      CALL PRE
      READ (5, '(Q,A)', IOSTAT=IOS) S(1,1), TX(2)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=744', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 5)
      CALL SYNC('#745')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=745', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 4)
      CALL SYNC('#746')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=746', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#747')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=747', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#748')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=748', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#749')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=749', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#750')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=750', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#751')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=751', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R6
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#752')
      CALL PRE
      READ (5, '(I2,I2,I2)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=752', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#753')
      CALL PRE
      READ (5, '()', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=753', IOS, 
     &IE, IS, IV, IU, IC
      CALL SYNC('#754')
      CALL PRE
      READ (5, '(1X)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=754', IOS, 
     &IE, IS, IV, IU, IC
      CALL SYNC('#755')
      CALL PRE
      READ (5, '(1X,''A'')', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=755', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#756')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=756', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#757')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=757', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#758')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=758', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#759')
      CALL PRE
      READ (5, '(T1,I1,T1,I1)', IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=759', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#760')
      CALL PRE
      READ (5, '(I1,$)', IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=760', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#761')
      CALL PRE
      READ (5, '(I1,/,F4.1)', IOSTAT=IOS) S(1,1), VF(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=761', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#762')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VF(1,1), VF(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=762', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#763')
      CALL PRE
      READ (5, '(I1,F4.1,(I1))', IOSTAT=IOS) S(1,1), VF(1,2), S(1,3), S(
     &1,4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=763', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SYNC('#764')
      CALL PRE
      READ (5, '(2I1,/,I1)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3), S(1,4),
     & S(1,5), S(1,6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=764', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SHOWV(4, 1)
      CALL SHOWV(5, 1)
      CALL SYNC('#765')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), VF(1,2), VFC(1,3), VFC(1,4), VLU(1
     &,5), VLU(1,6), S(1,7), S(1,8), VF(1,9), VD(1,10), TX(11)(1:14), VF
     &(1,12), VF(1,13)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=765', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 2)
      CALL SHOWV(3, 2)
      CALL SHOWV(4, 1)
      CALL SHOWV(5, 1)
      CALL SHOWV(6, 1)
      CALL SHOWV(7, 1)
      CALL SHOWV(8, 1)
      CALL SHOWV(9, 2)
      CALL SHOWT(10, 14)
      CALL SHOWV(11, 1)
      CALL SHOWV(12, 1)
      CALL SYNC('#766')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=766', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#767')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=767', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#768')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=768', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#769')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=769', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#770')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=770', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#771')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=771', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#772')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=772', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#773')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=773', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#774')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=774', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#775')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=775', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#776')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=776', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#777')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=777', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#778')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1), S(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=778', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#779')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=779', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#780')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=780', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#781')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=781', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#782')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VF(1,1), VF(1,2), VF(1,3), VF(1,4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=782', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SYNC('#783')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=783', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#784')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VF(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=784', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#785')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=785', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#786')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=786', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#787')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=787', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#788')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=788', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#789')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=789', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#790')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=790', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#791')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=791', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      END

      SUBROUTINE R7
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#792')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=792', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 4)
      CALL SYNC('#793')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=793', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#794')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VFC(1,1), VFC(1,2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=794', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SHOWV(1, 2)
      CALL SYNC('#795')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VLU(1,1), VLU(1,2), VLU(1,3), VLU(1,4), VL
     &U(1,5), VLU(1,6), VLU(1,7), VLU(1,8)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=795', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SHOWV(4, 1)
      CALL SHOWV(5, 1)
      CALL SHOWV(6, 1)
      CALL SHOWV(7, 1)
      CALL SYNC('#796')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VLU(1,1), VLU(1,2), VLU(1,3), VLU(1,4), VL
     &U(1,5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=796', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SHOWV(4, 1)
      CALL SYNC('#797')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=797', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#798')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VLU(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=798', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#799')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=799', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 5)
      CALL SYNC('#800')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=800', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 5)
      CALL SYNC('#801')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=801', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#802')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=802', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 4)
      CALL SYNC('#803')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=803', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 6)
      CALL SYNC('#804')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=804', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#805')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=805', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#806')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=806', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 3)
      CALL SYNC('#807')
      CALL PRE
      READ (5, *, IOSTAT=IOS) TX(1)(1:2), TX(2)(1:2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=807', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWT(0, 2)
      CALL SHOWT(1, 2)
      CALL SYNC('#808')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=808', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#809')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=809', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#810')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=810', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#811')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=811', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#812')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=812', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#813')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=813', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#814')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=814', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#815')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=815', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#816')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VW(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=816', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#817')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=817', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#818')
      CALL PRE
      READ (5, *, IOSTAT=IOS) VB(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=818', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      CALL SYNC('#819')
      CALL PRE
      READ (5, *, IOSTAT=IOS) S(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=819', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 1)
      END

      OPTIONS /G_FLOATING
      SUBROUTINE R8
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CALL SYNC('#660')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=660', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      CALL SYNC('#667')
      CALL PRE
      READ (5, '(E9.3)', IOSTAT=IOS) VD(1,1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=667', IOS, 
     &IE, IS, IV, IU, IC
      CALL SHOWV(0, 2)
      END

      SUBROUTINE SYNC(M)
      CHARACTER*(*) M
      CHARACTER*80 LINE
   10 READ (5, '(A)', END=20, ERR=10) LINE
      IF (LINE .NE. M) GO TO 10
   20 RETURN
      END

      SUBROUTINE PRE
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      DO 10 J = 1, 16
      DO 5 K = 1, 4
    5 S(K,J) = 1431655765
   10 TX(J) = 'UUUUUUUUUUUUUUUU' // 'UUUUUUUUUUUUUUUU' //
     &        'UUUUUUUUUUUUUUUU' // 'UUUUUUUUUUUUUUUU'
      END

      SUBROUTINE SHOWV(J, N)
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      PRINT *, 'v', J, (S(K,J+1), K = 1, N)
      END

      SUBROUTINE SHOWT(J, N)
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      PRINT *, 'v', J, '|' // TX(J+1)(1:N) // '|'
      END
