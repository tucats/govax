C     FOR80X.FOR - written by testdata/for80/gen.go: the internal file,
C     ENCODE, and DECODE cases of rounds 1 and 3, for govax.
C     TestFor80InternalOracle holds them to vax/for80-vax.txt and
C     vax/for80c-forrtl.txt.
      PROGRAM FOR80X
      CALL R1
      CALL R2
      CALL R3
      CALL R4
      CALL R5
      CALL R6
      CALL R7
      CALL R8
      CALL R9
      CALL R10
      END

      SUBROUTINE R1
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      WRITE (BUF(1:20), '('' I='',I5)', IOSTAT=IOS) 42
      PRINT *, '=513', IOS
      CALL SHOWB(1, 20)
      CALL PRE
      VF(1,2) = 2.5
      WRITE (BUF(1:20), '('' I='',I5,'' F='',F7.3)', IOSTAT=IOS) 42, VF(
     &1,2)
      PRINT *, '=514', IOS
      CALL SHOWB(1, 20)
      CALL PRE
      VF(1,2) = 2.5
      WRITE (BUF(1:10), '('' I='',I5,'' F='',F7.3)', IOSTAT=IOS) 42, VF(
     &1,2)
      PRINT *, '=515', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '('' A'',/,'' B'')', IOSTAT=IOS)
      PRINT *, '=516', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(/)', IOSTAT=IOS)
      PRINT *, '=517', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '()', IOSTAT=IOS)
      PRINT *, '=518', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(''ABC'',$)', IOSTAT=IOS)
      PRINT *, '=519', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(''+ABC'')', IOSTAT=IOS)
      PRINT *, '=520', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:5), '(A)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=521', IOS
      CALL SHOWB(1, 5)
      CALL PRE
      WRITE (BUF(1:4), '(A)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=522', IOS
      CALL SHOWB(1, 4)
      CALL PRE
      WRITE (BUF(1:6), '(A)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=523', IOS
      CALL SHOWB(1, 6)
      CALL PRE
      VF(1,2) = 2.5
      WRITE (BUF(1:40), *, IOSTAT=IOS) 42, VF(1,2)
      PRINT *, '=524', IOS
      CALL SHOWB(1, 40)
      CALL PRE
      VF(1,2) = 2.5
      WRITE (BUF(1:10), *, IOSTAT=IOS) 42, VF(1,2)
      PRINT *, '=525', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:40), *, IOSTAT=IOS) 'HELLO'
      PRINT *, '=526', IOS
      CALL SHOWB(1, 40)
      CALL PRE
      WRITE (W5, '(A)', IOSTAT=IOS) 'ONE'
      PRINT *, '=527', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(A,/,A)', IOSTAT=IOS) 'ONE', 'TWO'
      PRINT *, '=528', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(A,/,A,/,A)', IOSTAT=IOS) 'ONE', 'TWO', 'THREE'
      PRINT *, '=529', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(A,/,A,/,A,/,A)', IOSTAT=IOS) 'ONE', 'TWO', 'THREE', '
     &FOUR'
      PRINT *, '=530', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      ENCODE (12, '(3I4)', BUF, IOSTAT=IOS) 9012, 5678, 1234
      PRINT *, '=531', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (20, '(3I4)', BUF, IOSTAT=IOS) 9012, 5678, 1234
      PRINT *, '=532', IOS
      CALL SHOWB(1, 20)
      CALL PRE
      ENCODE (8, '(3I4)', BUF, IOSTAT=IOS) 9012, 5678, 1234
      PRINT *, '=533', IOS
      CALL SHOWB(1, 8)
      CALL PRE
      ENCODE (12, '(I4,/,I4)', BUF, IOSTAT=IOS) 1, 2
      PRINT *, '=534', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (12, '(I4)', BUF, IOSTAT=IOS) 1, 2
      PRINT *, '=535', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (0, '(I4)', BUF, IOSTAT=IOS) 1
      PRINT *, '=536', IOS
      CALL SHOWB(1, 4)
      CALL PRE
      ENCODE (-1, '(I4)', BUF, IOSTAT=IOS) 1
      PRINT *, '=537', IOS
      CALL SHOWB(1, 4)
      CALL PRE
      IN(1:4) = '2788'
      N = 4
      READ (IN(1:N), '(I4)', IOSTAT=IOS) S(1,1)
      PRINT *, '=538', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '-26'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=539', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = '      312'
      N = 9
      READ (IN(1:N), '(I9)', IOSTAT=IOS) S(1,1)
      PRINT *, '=540', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '   '
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=541', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=542', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=543', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(BZ,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=544', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '12 '
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=545', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '12 '
      N = 3
      READ (IN(1:N), '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=546', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = ' 12'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=547', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '2147483647'
      N = 10
      READ (IN(1:N), '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=548', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '2147483648'
      N = 10
      READ (IN(1:N), '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=549', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:11) = '-2147483648'
      N = 11
      READ (IN(1:N), '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=550', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:11) = '-2147483649'
      N = 11
      READ (IN(1:N), '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=551', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = 'ABC'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=552', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R2
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=553', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '+12'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=554', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '- 1'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=555', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1-2'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=556', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1,2'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=557', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '32767'
      N = 5
      READ (IN(1:N), '(I5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=558', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '32768'
      N = 5
      READ (IN(1:N), '(I5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=559', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '-32768'
      N = 6
      READ (IN(1:N), '(I6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=560', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '127'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=561', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '128'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=562', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '12'
      N = 2
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=563', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '12'
      N = 2
      READ (IN(1:N), '(I4.2)', IOSTAT=IOS) S(1,1)
      PRINT *, '=564', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '42'
      N = 2
      READ (IN(1:N), '(I)', IOSTAT=IOS) S(1,1)
      PRINT *, '=565', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '  42'
      N = 4
      READ (IN(1:N), '(I)', IOSTAT=IOS) S(1,1)
      PRINT *, '=566', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '42'
      N = 2
      READ (IN(1:N), '(I)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=567', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '42'
      N = 2
      READ (IN(1:N), '(I)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=568', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '1'
      N = 1
      READ (IN(1:N), '(I3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=569', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '1'
      N = 1
      READ (IN(1:N), '(I3)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=570', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = 'ABC'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=571', IOS
      CALL SHOWT(0, 3)
      CALL PRE
      IN(1:5) = '32767'
      N = 5
      READ (IN(1:N), '(O5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=572', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '16234'
      N = 5
      READ (IN(1:N), '(O4)', IOSTAT=IOS) S(1,1)
      PRINT *, '=573', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '970'
      N = 3
      READ (IN(1:N), '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=574', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '17'
      N = 2
      READ (IN(1:N), '(O)', IOSTAT=IOS) S(1,1)
      PRINT *, '=575', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '-7'
      N = 2
      READ (IN(1:N), '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=576', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=577', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:11) = '37777777777'
      N = 11
      READ (IN(1:N), '(O12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=578', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:11) = '77777777777'
      N = 11
      READ (IN(1:N), '(O12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=579', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '177777'
      N = 6
      READ (IN(1:N), '(O6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=580', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '777777'
      N = 6
      READ (IN(1:N), '(O6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=581', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '7'
      N = 1
      READ (IN(1:N), '(O3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=582', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '7'
      N = 1
      READ (IN(1:N), '(O12)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=583', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:3) = 'A94'
      N = 3
      READ (IN(1:N), '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=584', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = 'A23DEF'
      N = 6
      READ (IN(1:N), '(Z5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=585', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '95.AF2'
      N = 6
      READ (IN(1:N), '(Z5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=586', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = 'FF'
      N = 2
      READ (IN(1:N), '(Z)', IOSTAT=IOS) S(1,1)
      PRINT *, '=587', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = 'a9f'
      N = 3
      READ (IN(1:N), '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=588', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '-A9'
      N = 3
      READ (IN(1:N), '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=589', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = 'FFFFFFFF'
      N = 8
      READ (IN(1:N), '(Z8)', IOSTAT=IOS) S(1,1)
      PRINT *, '=590', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = '1FFFFFFFF'
      N = 9
      READ (IN(1:N), '(Z9)', IOSTAT=IOS) S(1,1)
      PRINT *, '=591', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = 'FFFF'
      N = 4
      READ (IN(1:N), '(Z4)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=592', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R3
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:5) = 'FFFFF'
      N = 5
      READ (IN(1:N), '(Z5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=593', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '40800000'
      N = 8
      READ (IN(1:N), '(Z8)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=594', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '40800000'
      N = 8
      READ (IN(1:N), '(Z16)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=595', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:1) = 'G'
      N = 1
      READ (IN(1:N), '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=596', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=597', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 't'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=598', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '.T'
      N = 2
      READ (IN(1:N), '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=599', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '.t'
      N = 2
      READ (IN(1:N), '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=600', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'F'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=601', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'f'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=602', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '.F'
      N = 2
      READ (IN(1:N), '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=603', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = ' '
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=604', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '    T'
      N = 5
      READ (IN(1:N), '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=605', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = 'TRUE '
      N = 5
      READ (IN(1:N), '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=606', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '.TRUE'
      N = 5
      READ (IN(1:N), '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=607', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = 'FALSE'
      N = 5
      READ (IN(1:N), '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=608', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'X'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=609', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '1'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=610', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=611', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VWU(1,1)
      PRINT *, '=612', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) S(1,1)
      PRINT *, '=613', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=614', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = 'T'
      N = 1
      READ (IN(1:N), '(L1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=615', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:9) = '123456789'
      N = 9
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=616', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = '-1234.567'
      N = 9
      READ (IN(1:N), '(F9.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=617', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '24.77E+2'
      N = 8
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=618', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '1234567.89'
      N = 10
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=619', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '    1234'
      N = 8
      READ (IN(1:N), '(F8.0)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=620', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '    1234'
      N = 8
      READ (IN(1:N), '(F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=621', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '  1 2  3'
      N = 8
      READ (IN(1:N), '(F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=622', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '  1 2  3'
      N = 8
      READ (IN(1:N), '(BN,F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=623', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '  1 2  3'
      N = 8
      READ (IN(1:N), '(BZ,F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=624', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '     '
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=625', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5E3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=626', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5D3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=627', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5Q3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=628', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5+3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=629', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5-3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=630', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '1E'
      N = 2
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=631', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = 'ABC'
      N = 3
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=632', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R4
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:1) = '.'
      N = 1
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=633', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '+'
      N = 1
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=634', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '-.5'
      N = 3
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=635', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.2.3'
      N = 5
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=636', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1,5'
      N = 3
      READ (IN(1:N), '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=637', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '1.0E39'
      N = 6
      READ (IN(1:N), '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=638', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:7) = '1.0E-39'
      N = 7
      READ (IN(1:N), '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=639', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:7) = '1.0E+39'
      N = 7
      READ (IN(1:N), '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=640', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = '734.432E3'
      N = 9
      READ (IN(1:N), '(E9.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=641', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:12) = '  1022.43E-6'
      N = 12
      READ (IN(1:N), '(E12.4)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=642', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:15) = '52.3759663     '
      N = 15
      READ (IN(1:N), '(E15.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=643', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:12) = '210.5271D+10'
      N = 12
      READ (IN(1:N), '(E12.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=644', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '12345     '
      N = 10
      READ (IN(1:N), '(BZ,D10.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=645', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '   123.45 '
      N = 10
      READ (IN(1:N), '(D10.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=646', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:15) = '367.4981763D+04'
      N = 15
      READ (IN(1:N), '(D15.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=647', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(G13.6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=648', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = '1.5E3'
      N = 5
      READ (IN(1:N), '(G13.6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=649', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(F)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=650', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(E)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=651', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(G)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=652', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1.5'
      N = 3
      READ (IN(1:N), '(D)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=653', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '   37.614 '
      N = 10
      READ (IN(1:N), '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=654', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '  37.614E2'
      N = 10
      READ (IN(1:N), '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=655', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:10) = '    37.614'
      N = 10
      READ (IN(1:N), '(-3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=656', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '    1234'
      N = 8
      READ (IN(1:N), '(2PF8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=657', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = '   12.34'
      N = 8
      READ (IN(1:N), '(2PF8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=658', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = '123456789'
      N = 9
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=659', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:9) = '123456789'
      N = 9
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VH(1)
      PRINT *, '=661', IOS
      CALL SHOWV(0, 4)
      CALL PRE
      IN(1:4) = '12.5'
      N = 4
      READ (IN(1:N), '(F4.1)', IOSTAT=IOS) S(1,1)
      PRINT *, '=662', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '12.5'
      N = 4
      READ (IN(1:N), '(F4.1)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=663', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '12.5'
      N = 4
      READ (IN(1:N), '(F4.1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=664', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '12.5'
      N = 4
      READ (IN(1:N), '(F4.1)', IOSTAT=IOS) TX(1)(1:4)
      PRINT *, '=665', IOS
      CALL SHOWT(0, 4)
      CALL PRE
      IN(1:6) = '1.0E99'
      N = 6
      READ (IN(1:N), '(E9.3)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=666', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:8) = '1.0E1000'
      N = 8
      READ (IN(1:N), '(E12.3)', IOSTAT=IOS) VH(1)
      PRINT *, '=668', IOS
      CALL SHOWV(0, 4)
      CALL PRE
      IN(1:16) = '1234567812345.67'
      N = 16
      READ (IN(1:N), '(2F8.5)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=669', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:16) = '   1.5E3     2.5'
      N = 16
      READ (IN(1:N), '(E9.1,F9.3)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=670', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:16) = '1234567812345.67'
      N = 16
      READ (IN(1:N), '(2F8.5)', IOSTAT=IOS) VDC(1)
      PRINT *, '=671', IOS
      CALL SHOWV(0, 4)
      CALL PRE
      IN(1:8) = '12345678'
      N = 8
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=672', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=673', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=674', IOS
      CALL SHOWT(0, 3)
      END

      SUBROUTINE R5
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) TX(1)(1:6)
      PRINT *, '=675', IOS
      CALL SHOWT(0, 6)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) TX(1)(1:8)
      PRINT *, '=676', IOS
      CALL SHOWT(0, 8)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=677', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=678', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) S(1,1)
      PRINT *, '=679', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=680', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=681', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VH(1)
      PRINT *, '=682', IOS
      CALL SHOWV(0, 4)
      CALL PRE
      IN(1:6) = 'PAGE #'
      N = 6
      READ (IN(1:N), '(A6)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=683', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = 'HELLO'
      N = 5
      READ (IN(1:N), '(A)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=684', IOS
      CALL SHOWT(0, 5)
      CALL PRE
      IN(1:5) = 'HELLO'
      N = 5
      READ (IN(1:N), '(A)', IOSTAT=IOS) TX(1)(1:8)
      PRINT *, '=685', IOS
      CALL SHOWT(0, 8)
      CALL PRE
      IN(1:5) = 'HELLO'
      N = 5
      READ (IN(1:N), '(A)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=686', IOS
      CALL SHOWT(0, 3)
      CALL PRE
      IN(1:4) = 'ABCD'
      N = 4
      READ (IN(1:N), '(A)', IOSTAT=IOS) S(1,1)
      PRINT *, '=687', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:8) = 'ABCDEFGH'
      N = 8
      READ (IN(1:N), '(A)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=688', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:2) = 'AB'
      N = 2
      READ (IN(1:N), '(A)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=689', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = 'AB'
      N = 2
      READ (IN(1:N), '(A)', IOSTAT=IOS) S(1,1)
      PRINT *, '=690', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:9) = 'ABC   XYZ'
      N = 9
      READ (IN(1:N), '(T7,A3,T1,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      PRINT *, '=691', IOS
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(TR2,A1,TL2,A1)', IOSTAT=IOS) TX(1)(1:1), TX(2)(1:
     &1)
      PRINT *, '=692', IOS
      CALL SHOWT(0, 1)
      CALL SHOWT(1, 1)
      CALL PRE
      IN(1:8) = 'A12BBB34'
      N = 8
      READ (IN(1:N), '(1X,I2,3X,I2)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=693', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:3) = 'ABC'
      N = 3
      READ (IN(1:N), '(T50,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=694', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:3) = 'ABC'
      N = 3
      READ (IN(1:N), '(TL50,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=695', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(Q,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      PRINT *, '=696', IOS
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(A2,Q)', IOSTAT=IOS) TX(1)(1:2), S(1,2)
      PRINT *, '=697', IOS
      CALL SHOWT(0, 2)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(Q)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=698', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(Q)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=699', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:5) = 'ABCDE'
      N = 5
      READ (IN(1:N), '(Q)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=700', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:6) = 'ABCDEF'
      N = 6
      READ (IN(1:N), '(5HXXXXX,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=701', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:6) = 'ABCDEF'
      N = 6
      READ (IN(1:N), '(''XXXXX'',A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=702', IOS
      CALL SHOWT(0, 1)
      CALL PRE
      IN(1:11) = '1,-2,1.0,35'
      N = 11
      READ (IN(1:N), '(2I6,2F10.2)', IOSTAT=IOS) S(1,1), S(1,2), VF(1,3)
     &, VF(1,4)
      PRINT *, '=703', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL PRE
      IN(1:2) = ',,'
      N = 2
      READ (IN(1:N), '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=704', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:2) = '1,'
      N = 2
      READ (IN(1:N), '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=705', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:5) = '1,ABC'
      N = 5
      READ (IN(1:N), '(I6,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      PRINT *, '=706', IOS
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL PRE
      IN(1:6) = 'A,B123'
      N = 6
      READ (IN(1:N), '(A3,I3)', IOSTAT=IOS) TX(1)(1:3), S(1,2)
      PRINT *, '=707', IOS
      CALL SHOWT(0, 3)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:5) = 'T,123'
      N = 5
      READ (IN(1:N), '(L6,I3)', IOSTAT=IOS) VLU(1,1), S(1,2)
      PRINT *, '=708', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:5) = '7,F,1'
      N = 5
      READ (IN(1:N), '(O6,Z6,I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=709', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:7) = '123,456'
      N = 7
      READ (IN(1:N), '(I3,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=710', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:1) = '1'
      N = 1
      READ (IN(1:N), '(3I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=711', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:2) = 'AB'
      N = 2
      READ (IN(1:N), '(A3,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      PRINT *, '=712', IOS
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=713', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=714', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      END

      SUBROUTINE R6
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3,:,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=715', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3,$)', IOSTAT=IOS) S(1,1)
      PRINT *, '=716', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(SP,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=717', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '1 21 2'
      N = 6
      READ (IN(1:N), '(BN,I3,BZ,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=718', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:6) = '1 21 2'
      N = 6
      READ (IN(1:N), '(BZ,I3,BN,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=719', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:5) = '1 2 3'
      N = 5
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=720', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:4) = '1,2/'
      N = 4
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=721', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:9) = '''ABC'' 1.5'
      N = 9
      READ (IN(1:N), *, IOSTAT=IOS) TX(1)(1:3), VF(1,2)
      PRINT *, '=722', IOS
      CALL SHOWT(0, 3)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=723', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=724', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3,/,I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=725', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=726', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (12, '(3I4)', IN, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=727', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (8, '(3I4)', IN, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=728', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (12, '(I4,/,I4)', IN, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=729', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (12, '(I4)', IN, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=730', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:2) = '12'
      N = 2
      DECODE (0, '(I4)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=731', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      WRITE (BUF(1:10), '('' ABC'',$)', IOSTAT=IOS)
      PRINT *, '=2000', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(''+ABC'',$)', IOSTAT=IOS)
      PRINT *, '=2001', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(T5,''A'')', IOSTAT=IOS)
      PRINT *, '=2002', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(T10,''A'')', IOSTAT=IOS)
      PRINT *, '=2003', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(T11,''A'')', IOSTAT=IOS)
      PRINT *, '=2004', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(11X,''A'')', IOSTAT=IOS)
      PRINT *, '=2005', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(20X)', IOSTAT=IOS)
      PRINT *, '=2006', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(5X)', IOSTAT=IOS)
      PRINT *, '=2007', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(''ABC'',TL2,''X'')', IOSTAT=IOS)
      PRINT *, '=2008', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(''ABCDEFGHIJKL'')', IOSTAT=IOS)
      PRINT *, '=2009', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(5HABCDE,5HFGHIJ,1HK)', IOSTAT=IOS)
      PRINT *, '=2010', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:3), '(I5)', IOSTAT=IOS) 42
      PRINT *, '=2011', IOS
      CALL SHOWB(1, 3)
      CALL PRE
      VF(1,1) = 2.5
      WRITE (BUF(1:3), '(F20.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=2012', IOS
      CALL SHOWB(1, 3)
      CALL PRE
      WRITE (BUF(1:4), '(I2,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=2013', IOS
      CALL SHOWB(1, 4)
      CALL PRE
      WRITE (BUF(1:10), '(I0)', IOSTAT=IOS) 1
      PRINT *, '=2014', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '(1X,I1)', IOSTAT=IOS) 42
      PRINT *, '=2015', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), '('' A'',/)', IOSTAT=IOS)
      PRINT *, '=2016', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (W5, '(A,/)', IOSTAT=IOS) 'ONE'
      PRINT *, '=2017', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(/)', IOSTAT=IOS)
      PRINT *, '=2018', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(//)', IOSTAT=IOS)
      PRINT *, '=2019', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(///)', IOSTAT=IOS)
      PRINT *, '=2020', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(A)', IOSTAT=IOS) 'ONE', 'TWO'
      PRINT *, '=2021', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '(A)', IOSTAT=IOS) 'ONE', 'TWO', 'THREE'
      PRINT *, '=2022', IOS
      CALL SHOWB(1, 15)
      END

      SUBROUTINE R7
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      WRITE (W5, '(A)', IOSTAT=IOS) 'ONE', 'TWO', 'THREE', 'FOUR'
      PRINT *, '=2023', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      WRITE (W5, '('' ABC'',$,/,'' DEF'')', IOSTAT=IOS)
      PRINT *, '=2024', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      ENCODE (12, '(I4,/)', BUF, IOSTAT=IOS) 1
      PRINT *, '=2025', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (12, '(/)', BUF, IOSTAT=IOS)
      PRINT *, '=2026', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (12, '(I4,$)', BUF, IOSTAT=IOS) 1
      PRINT *, '=2027', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (12, '(I4,:)', BUF, IOSTAT=IOS) 1
      PRINT *, '=2028', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      ENCODE (5, '()', BUF, IOSTAT=IOS)
      PRINT *, '=2029', IOS
      CALL SHOWB(1, 5)
      CALL PRE
      ENCODE (12, '('' ABC'',$)', BUF, IOSTAT=IOS)
      PRINT *, '=2030', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      WRITE (BUF(1:300), '(T290,''A'')', IOSTAT=IOS)
      PRINT *, '=2031', IOS
      CALL SHOWB(201, 300)
      CALL PRE
      WRITE (BUF(1:300), '(T300,''A'')', IOSTAT=IOS)
      PRINT *, '=2032', IOS
      CALL SHOWB(201, 300)
      CALL PRE
      WRITE (BUF(1:300), '(T301,''A'')', IOSTAT=IOS)
      PRINT *, '=2033', IOS
      CALL SHOWB(201, 300)
      CALL PRE
      WRITE (BUF(1:150), '(150A)', IOSTAT=IOS) 'A', 'B'
      PRINT *, '=2034', IOS
      CALL SHOWB(51, 150)
      CALL PRE
      WRITE (BUF(1:60), '(I50)', IOSTAT=IOS) 1
      PRINT *, '=2035', IOS
      CALL SHOWB(1, 60)
      CALL PRE
      WRITE (BUF(1:300), '(I250)', IOSTAT=IOS) 1
      PRINT *, '=2036', IOS
      CALL SHOWB(201, 300)
      CALL PRE
      IN(1:300) = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBBBBBBBCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC
     &CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      N = 300
      READ (R300X1, '(T290,A5,Q)', IOSTAT=IOS) TX(1)(1:5), S(1,2)
      PRINT *, '=2037', IOS
      CALL SHOWT(0, 5)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:300) = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBBBBBBBCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC
     &CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      N = 300
      READ (R300X1, '(T250,A5)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=2038', IOS
      CALL SHOWT(0, 5)
      CALL PRE
      IN(1:300) = '                                                     
     &                                                                  
     &                                                                  
     &                                                                  
     &                                             1234'
      N = 300
      READ (R300X1, *, IOSTAT=IOS) S(1,1)
      PRINT *, '=2039', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      WRITE (BUF(1:12), *, IOSTAT=IOS) 1
      PRINT *, '=2040', IOS
      CALL SHOWB(1, 12)
      CALL PRE
      WRITE (BUF(1:11), *, IOSTAT=IOS) 1
      PRINT *, '=2041', IOS
      CALL SHOWB(1, 11)
      CALL PRE
      WRITE (BUF(1:13), *, IOSTAT=IOS) 1, 2
      PRINT *, '=2042', IOS
      CALL SHOWB(1, 13)
      CALL PRE
      WRITE (BUF(1:23), *, IOSTAT=IOS) 1, 2
      PRINT *, '=2043', IOS
      CALL SHOWB(1, 23)
      CALL PRE
      WRITE (BUF(1:24), *, IOSTAT=IOS) 1, 2
      PRINT *, '=2044', IOS
      CALL SHOWB(1, 24)
      CALL PRE
      WRITE (BUF(1:25), *, IOSTAT=IOS) 1, 2
      PRINT *, '=2045', IOS
      CALL SHOWB(1, 25)
      CALL PRE
      WRITE (BUF(1:10), *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2046', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:4), *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2047', IOS
      CALL SHOWB(1, 4)
      CALL PRE
      WRITE (BUF(1:3), *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2048', IOS
      CALL SHOWB(1, 3)
      CALL PRE
      WRITE (BUF(1:2), *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2049', IOS
      CALL SHOWB(1, 2)
      CALL PRE
      WRITE (BUF(1:1), *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2050', IOS
      CALL SHOWB(1, 1)
      CALL PRE
      WRITE (BUF(1:10), *, IOSTAT=IOS) 'ABCDEFGHIJKLMNOPQRSTUVWXY'
      PRINT *, '=2051', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), *, IOSTAT=IOS) 'ABC', 'DEFGHIJ'
      PRINT *, '=2052', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), *, IOSTAT=IOS) 'ABC', 'DEFGHIJKL'
      PRINT *, '=2053', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:10), *, IOSTAT=IOS)
      PRINT *, '=2054', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:1), *, IOSTAT=IOS)
      PRINT *, '=2055', IOS
      CALL SHOWB(1, 1)
      CALL PRE
      VF(1,1) = 2.5
      WRITE (BUF(1:20), *, IOSTAT=IOS) VF(1,1)
      PRINT *, '=2056', IOS
      CALL SHOWB(1, 20)
      CALL PRE
      VF(1,1) = 2.5
      WRITE (BUF(1:15), *, IOSTAT=IOS) VF(1,1)
      PRINT *, '=2057', IOS
      CALL SHOWB(1, 15)
      CALL PRE
      VF(1,1) = 2.5
      WRITE (BUF(1:14), *, IOSTAT=IOS) VF(1,1)
      PRINT *, '=2058', IOS
      CALL SHOWB(1, 14)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      WRITE (BUF(1:40), *, IOSTAT=IOS) VFC(1,1)
      PRINT *, '=2059', IOS
      CALL SHOWB(1, 40)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      WRITE (BUF(1:20), *, IOSTAT=IOS) VFC(1,1)
      PRINT *, '=2060', IOS
      CALL SHOWB(1, 20)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      WRITE (BUF(1:10), *, IOSTAT=IOS) VFC(1,1)
      PRINT *, '=2061', IOS
      CALL SHOWB(1, 10)
      CALL PRE
      WRITE (BUF(1:99), *, IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7, 8
      PRINT *, '=2062', IOS
      CALL SHOWB(1, 99)
      END

      SUBROUTINE R8
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      WRITE (BUF(1:99), *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      PRINT *, '=2063', IOS
      CALL SHOWB(1, 99)
      CALL PRE
      WRITE (BUF(1:90), *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA', 'B'
      PRINT *, '=2064', IOS
      CALL SHOWB(1, 90)
      CALL PRE
      WRITE (W13, *, IOSTAT=IOS) 1, 2
      PRINT *, '=2065', IOS
      CALL SHOWB(1, 39)
      CALL PRE
      WRITE (W13, *, IOSTAT=IOS) 1, 2, 3
      PRINT *, '=2066', IOS
      CALL SHOWB(1, 39)
      CALL PRE
      WRITE (W13, *, IOSTAT=IOS) 1, 2, 3, 4
      PRINT *, '=2067', IOS
      CALL SHOWB(1, 39)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) 1
      PRINT *, '=2068', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) 'ABC', 'DEF'
      PRINT *, '=2069', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) 'ABCDEFGHIJKLMNOPQRSTUVWXY'
      PRINT *, '=2070', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) 'ABCDEFGHIJKLMNOPQRSTUVWXYZ01234567'
      PRINT *, '=2071', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W30, *, IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7
      PRINT *, '=2072', IOS
      CALL SHOWB(1, 90)
      CALL PRE
      WRITE (W33, *, IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
      PRINT *, '=2073', IOS
      CALL SHOWB(1, 99)
      CALL PRE
      WRITE (W2, *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2074', IOS
      CALL SHOWB(1, 6)
      CALL PRE
      WRITE (W1, *, IOSTAT=IOS) 'ABC'
      PRINT *, '=2075', IOS
      CALL SHOWB(1, 3)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) IN(1:NZERO), 'ABCDEFGHIJKL'
      PRINT *, '=2076', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W10, *, IOSTAT=IOS) 'A', 1
      PRINT *, '=2077', IOS
      CALL SHOWB(1, 30)
      CALL PRE
      WRITE (W13, *, IOSTAT=IOS) 'A', 1, 2
      PRINT *, '=2078', IOS
      CALL SHOWB(1, 39)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      WRITE (W13, *, IOSTAT=IOS) VFC(1,1)
      PRINT *, '=2079', IOS
      CALL SHOWB(1, 39)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      VFC(1,2) = (3.5,4.5)
      WRITE (W20, *, IOSTAT=IOS) VFC(1,1), VFC(1,2)
      PRINT *, '=2080', IOS
      CALL SHOWB(1, 60)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      VFC(1,2) = (3.5,4.5)
      WRITE (W25, *, IOSTAT=IOS) VFC(1,1), VFC(1,2)
      PRINT *, '=2081', IOS
      CALL SHOWB(1, 75)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      VFC(1,2) = (3.5,4.5)
      WRITE (W30, *, IOSTAT=IOS) VFC(1,1), VFC(1,2)
      PRINT *, '=2082', IOS
      CALL SHOWB(1, 90)
      CALL PRE
      VFC(1,2) = (1.5,-2.5)
      WRITE (W15, *, IOSTAT=IOS) 1, VFC(1,2)
      PRINT *, '=2083', IOS
      CALL SHOWB(1, 45)
      CALL PRE
      VFC(1,2) = (1.5,-2.5)
      WRITE (W25, *, IOSTAT=IOS) 1, VFC(1,2)
      PRINT *, '=2084', IOS
      CALL SHOWB(1, 75)
      CALL PRE
      VFC(1,2) = (1.5,-2.5)
      WRITE (W35, *, IOSTAT=IOS) 1, VFC(1,2)
      PRINT *, '=2085', IOS
      CALL SHOWB(6, 105)
      CALL PRE
      VFC(1,1) = (1.5,-2.5)
      WRITE (W40, *, IOSTAT=IOS) VFC(1,1), 1, 2
      PRINT *, '=2086', IOS
      CALL SHOWB(21, 120)
      CALL PRE
      N = 0
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1)
      PRINT *, '=2091', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '   '
      N = 3
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1)
      PRINT *, '=2092', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '1'
      N = 1
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2093', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:2) = '1 '
      N = 2
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2094', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:2) = '1,'
      N = 2
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2095', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:7) = '1 2 3 4'
      N = 7
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1)
      PRINT *, '=2096', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:1) = '/'
      N = 1
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1)
      PRINT *, '=2097', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '''ABC'
      N = 4
      READ (IN(1:N), *, IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=2098', IOS
      CALL SHOWT(0, 3)
      CALL PRE
      IN(1:3) = '2*5'
      N = 3
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=2099', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:2) = '2*'
      N = 2
      READ (IN(1:N), *, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2100', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:10) = '1 2 3 4 5 '
      N = 10
      READ (R3X3, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=2101', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL PRE
      IN(1:6) = '1 2 3 '
      N = 6
      READ (R3X2, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3), S(1,4)
      PRINT *, '=2102', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL PRE
      IN(1:8) = '1,,2,,3,'
      N = 8
      READ (R3X2, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3), S(1,4)
      PRINT *, '=2103', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL PRE
      IN(1:9) = '''ABCDEF'' '
      N = 9
      READ (R3X3, *, IOSTAT=IOS) TX(1)(1:6)
      PRINT *, '=2104', IOS
      CALL SHOWT(0, 6)
      CALL PRE
      IN(1:7) = '''ABCDEF'
      N = 7
      READ (R3X2, *, IOSTAT=IOS) TX(1)(1:6)
      PRINT *, '=2105', IOS
      CALL SHOWT(0, 6)
      CALL PRE
      IN(1:7) = '1/ 2 3 '
      N = 7
      READ (R3X2, *, IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=2106', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      END

      SUBROUTINE R9
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2107', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2108', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '1 2'
      N = 3
      READ (IN(1:N), '(BZ,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2109', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '12'
      N = 2
      READ (IN(1:N), '(I5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2110', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:2) = '12'
      N = 2
      READ (IN(1:N), '(A5)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=2111', IOS
      CALL SHOWT(0, 5)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2112', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(I3,/)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2113', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(/)', IOSTAT=IOS)
      PRINT *, '=2114', IOS
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(/,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2115', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(Q,I2)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2116', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      N = 0
      READ (IN(1:N), '(I3)', IOSTAT=IOS)
      PRINT *, '=2117', IOS
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (IN(1:N), '(T10,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2118', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(/,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2119', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(//,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2120', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3,/)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2121', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:6) = '123456'
      N = 6
      READ (R3X2, '(I3,//)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2122', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:3) = '123'
      N = 3
      READ (R3X1, '(I3,/)', IOSTAT=IOS) S(1,1)
      PRINT *, '=2123', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (12, '(I4,/)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=2124', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:12) = '123456789012'
      N = 12
      DECODE (12, '(/)', IN, IOSTAT=IOS)
      PRINT *, '=2125', IOS
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (-1, '(I4)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=2126', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (-1, '(A4)', IN, IOSTAT=IOS) TX(1)(1:4)
      PRINT *, '=2127', IOS
      CALL SHOWT(0, 4)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (40000, '(I4)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=2128', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (70000, '(I4)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=2129', IOS
      CALL SHOWV(0, 1)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (4, '(I2,Q)', IN, IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=2130', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (4, '(A)', IN, IOSTAT=IOS) TX(1)(1:8)
      PRINT *, '=2131', IOS
      CALL SHOWT(0, 8)
      CALL PRE
      IN(1:4) = '1234'
      N = 4
      DECODE (4, '(T3,I2)', IN, IOSTAT=IOS) S(1,1)
      PRINT *, '=2132', IOS
      CALL SHOWV(0, 1)
      END

      OPTIONS /G_FLOATING
      SUBROUTINE R10
      INTEGER IOS, N, NZERO
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      INTEGER*2 VW(8,16)
      BYTE VB(16,16)
      LOGICAL*4 VLU(4,16)
      LOGICAL*2 VWU(8,16)
      INTEGER*4 VLUI(16), VWUI2(16)
      REAL VF(4,16)
      DOUBLE PRECISION VD(2,16)
      REAL*16 VH(16)
      COMPLEX VFC(2,16)
      COMPLEX*16 VDC(16)
      EQUIVALENCE (S, VW, VB, VLU, VWU, VF, VD, VH, VFC, VDC)
      CHARACTER*5 W5(3)
      CHARACTER*3 R3X2(2)
      CHARACTER*300 R300X1(1)
      CHARACTER*13 W13(3)
      CHARACTER*10 W10(3)
      CHARACTER*30 W30(3)
      CHARACTER*33 W33(3)
      CHARACTER*2 W2(3)
      CHARACTER*1 W1(3)
      CHARACTER*20 W20(3)
      CHARACTER*25 W25(3)
      CHARACTER*15 W15(3)
      CHARACTER*35 W35(3)
      CHARACTER*40 W40(3)
      CHARACTER*3 R3X3(3)
      CHARACTER*3 R3X1(1)
      EQUIVALENCE (BUF, W5)
      EQUIVALENCE (IN, R3X2)
      EQUIVALENCE (IN, R300X1)
      EQUIVALENCE (BUF, W13)
      EQUIVALENCE (BUF, W10)
      EQUIVALENCE (BUF, W30)
      EQUIVALENCE (BUF, W33)
      EQUIVALENCE (BUF, W2)
      EQUIVALENCE (BUF, W1)
      EQUIVALENCE (BUF, W20)
      EQUIVALENCE (BUF, W25)
      EQUIVALENCE (BUF, W15)
      EQUIVALENCE (BUF, W35)
      EQUIVALENCE (BUF, W40)
      EQUIVALENCE (IN, R3X3)
      EQUIVALENCE (IN, R3X1)
      DATA NZERO /0/
      CALL PRE
      IN(1:9) = '123456789'
      N = 9
      READ (IN(1:N), '(F8.5)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=660', IOS
      CALL SHOWV(0, 2)
      CALL PRE
      IN(1:7) = '1.0E100'
      N = 7
      READ (IN(1:N), '(E9.3)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=667', IOS
      CALL SHOWV(0, 2)
      END

      SUBROUTINE PRE
      INTEGER*4 S(4,16)
      CHARACTER*64 TX(16)
      COMMON /SLOTS/ S, TX
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      DO 10 J = 1, 16
      DO 5 K = 1, 4
    5 S(K,J) = 1431655765
   10 TX(J) = 'UUUUUUUUUUUUUUUU' // 'UUUUUUUUUUUUUUUU' //
     &        'UUUUUUUUUUUUUUUU' // 'UUUUUUUUUUUUUUUU'
      DO 20 J = 1, 2048
      BUF(J:J) = 'U'
   20 IN(J:J) = 'U'
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

      SUBROUTINE SHOWB(M, N)
      CHARACTER*2048 BUF, IN
      COMMON /FILES/ BUF, IN
      WRITE (6, '(1X,A,1X,I3,A)') 'v', 99, '|' // BUF(M:N) // '|'
      END
