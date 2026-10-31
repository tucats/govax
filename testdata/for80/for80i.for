C     FOR80I.FOR - written by testdata/for80/gen.go: round 1's
C     formatted input cases, from SYS$INPUT, for govax.
C     TestFor80InputOracle holds what they store to vax/for80-vax.txt.
      PROGRAM FOR80I
      CALL R1
      CALL R2
      CALL R3
      CALL R4
      CALL R5
      CALL R6
      END

      SUBROUTINE R1
      INTEGER IOS
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
      PRINT *, '=538', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#539')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=539', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#540')
      CALL PRE
      READ (5, '(I9)', IOSTAT=IOS) S(1,1)
      PRINT *, '=540', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#541')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=541', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#542')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=542', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#543')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=543', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#544')
      CALL PRE
      READ (5, '(BZ,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=544', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#545')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=545', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#546')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=546', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#547')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=547', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#548')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=548', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#549')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=549', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#550')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=550', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#551')
      CALL PRE
      READ (5, '(I12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=551', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#552')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=552', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#553')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=553', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#554')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=554', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#555')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=555', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#556')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=556', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#557')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=557', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#558')
      CALL PRE
      READ (5, '(I5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=558', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#559')
      CALL PRE
      READ (5, '(I5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=559', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#560')
      CALL PRE
      READ (5, '(I6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=560', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#561')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=561', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#562')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=562', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#563')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=563', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#564')
      CALL PRE
      READ (5, '(I4.2)', IOSTAT=IOS) S(1,1)
      PRINT *, '=564', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#565')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) S(1,1)
      PRINT *, '=565', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#566')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) S(1,1)
      PRINT *, '=566', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#567')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=567', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#568')
      CALL PRE
      READ (5, '(I)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=568', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#569')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=569', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#570')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=570', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#571')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=571', IOS
      CALL SHOWT(0, 3)
      CALL SYNC('#572')
      CALL PRE
      READ (5, '(O5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=572', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#573')
      CALL PRE
      READ (5, '(O4)', IOSTAT=IOS) S(1,1)
      PRINT *, '=573', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#574')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=574', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#575')
      CALL PRE
      READ (5, '(O)', IOSTAT=IOS) S(1,1)
      PRINT *, '=575', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#576')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=576', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#577')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=577', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R2
      INTEGER IOS
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
      PRINT *, '=578', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#579')
      CALL PRE
      READ (5, '(O12)', IOSTAT=IOS) S(1,1)
      PRINT *, '=579', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#580')
      CALL PRE
      READ (5, '(O6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=580', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#581')
      CALL PRE
      READ (5, '(O6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=581', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#582')
      CALL PRE
      READ (5, '(O3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=582', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#583')
      CALL PRE
      READ (5, '(O12)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=583', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#584')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=584', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#585')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=585', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#586')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) S(1,1)
      PRINT *, '=586', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#587')
      CALL PRE
      READ (5, '(Z)', IOSTAT=IOS) S(1,1)
      PRINT *, '=587', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#588')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=588', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#589')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=589', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#590')
      CALL PRE
      READ (5, '(Z8)', IOSTAT=IOS) S(1,1)
      PRINT *, '=590', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#591')
      CALL PRE
      READ (5, '(Z9)', IOSTAT=IOS) S(1,1)
      PRINT *, '=591', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#592')
      CALL PRE
      READ (5, '(Z4)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=592', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#593')
      CALL PRE
      READ (5, '(Z5)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=593', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#594')
      CALL PRE
      READ (5, '(Z8)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=594', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#595')
      CALL PRE
      READ (5, '(Z16)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=595', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#596')
      CALL PRE
      READ (5, '(Z3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=596', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#597')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=597', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#598')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=598', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#599')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=599', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#600')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=600', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#601')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=601', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#602')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=602', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#603')
      CALL PRE
      READ (5, '(L2)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=603', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#604')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=604', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#605')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=605', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#606')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=606', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#607')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=607', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#608')
      CALL PRE
      READ (5, '(L5)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=608', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#609')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=609', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#610')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=610', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#611')
      CALL PRE
      READ (5, '(L)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=611', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#612')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VWU(1,1)
      PRINT *, '=612', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#613')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) S(1,1)
      PRINT *, '=613', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#614')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=614', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#615')
      CALL PRE
      READ (5, '(L1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=615', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#616')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=616', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#617')
      CALL PRE
      READ (5, '(F9.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=617', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R3
      INTEGER IOS
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
      PRINT *, '=618', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#619')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=619', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#620')
      CALL PRE
      READ (5, '(F8.0)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=620', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#621')
      CALL PRE
      READ (5, '(F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=621', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#622')
      CALL PRE
      READ (5, '(F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=622', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#623')
      CALL PRE
      READ (5, '(BN,F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=623', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#624')
      CALL PRE
      READ (5, '(BZ,F8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=624', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#625')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=625', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#626')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=626', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#627')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=627', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#628')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=628', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#629')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=629', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#630')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=630', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#631')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=631', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#632')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=632', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#633')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=633', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#634')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=634', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#635')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=635', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#636')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=636', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#637')
      CALL PRE
      READ (5, '(F5.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=637', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#638')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=638', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#639')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=639', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#640')
      CALL PRE
      READ (5, '(F9.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=640', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#641')
      CALL PRE
      READ (5, '(E9.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=641', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#642')
      CALL PRE
      READ (5, '(E12.4)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=642', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#643')
      CALL PRE
      READ (5, '(E15.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=643', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#644')
      CALL PRE
      READ (5, '(E12.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=644', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#645')
      CALL PRE
      READ (5, '(BZ,D10.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=645', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#646')
      CALL PRE
      READ (5, '(D10.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=646', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#647')
      CALL PRE
      READ (5, '(D15.3)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=647', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#648')
      CALL PRE
      READ (5, '(G13.6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=648', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#649')
      CALL PRE
      READ (5, '(G13.6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=649', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#650')
      CALL PRE
      READ (5, '(F)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=650', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#651')
      CALL PRE
      READ (5, '(E)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=651', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#652')
      CALL PRE
      READ (5, '(G)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=652', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#653')
      CALL PRE
      READ (5, '(D)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=653', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#654')
      CALL PRE
      READ (5, '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=654', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#655')
      CALL PRE
      READ (5, '(3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=655', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#656')
      CALL PRE
      READ (5, '(-3PE10.5)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=656', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#657')
      CALL PRE
      READ (5, '(2PF8.2)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=657', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R4
      INTEGER IOS
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
      PRINT *, '=658', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#659')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=659', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#661')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VH(1)
      PRINT *, '=661', IOS
      CALL SHOWV(0, 4)
      CALL SYNC('#662')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) S(1,1)
      PRINT *, '=662', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#663')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=663', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#664')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=664', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#665')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) TX(1)(1:4)
      PRINT *, '=665', IOS
      CALL SHOWT(0, 4)
      CALL SYNC('#666')
      CALL PRE
      READ (5, '(E9.3)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=666', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#668')
      CALL PRE
      READ (5, '(E12.3)', IOSTAT=IOS) VH(1)
      PRINT *, '=668', IOS
      CALL SHOWV(0, 4)
      CALL SYNC('#669')
      CALL PRE
      READ (5, '(2F8.5)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=669', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#670')
      CALL PRE
      READ (5, '(E9.1,F9.3)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=670', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#671')
      CALL PRE
      READ (5, '(2F8.5)', IOSTAT=IOS) VDC(1)
      PRINT *, '=671', IOS
      CALL SHOWV(0, 4)
      CALL SYNC('#672')
      CALL PRE
      READ (5, '(F8.5)', IOSTAT=IOS) VFC(1,1)
      PRINT *, '=672', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#673')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=673', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#674')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=674', IOS
      CALL SHOWT(0, 3)
      CALL SYNC('#675')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:6)
      PRINT *, '=675', IOS
      CALL SHOWT(0, 6)
      CALL SYNC('#676')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) TX(1)(1:8)
      PRINT *, '=676', IOS
      CALL SHOWT(0, 8)
      CALL SYNC('#677')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VB(1,1)
      PRINT *, '=677', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#678')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=678', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#679')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) S(1,1)
      PRINT *, '=679', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#680')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=680', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#681')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=681', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#682')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VH(1)
      PRINT *, '=682', IOS
      CALL SHOWV(0, 4)
      CALL SYNC('#683')
      CALL PRE
      READ (5, '(A6)', IOSTAT=IOS) VLU(1,1)
      PRINT *, '=683', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#684')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=684', IOS
      CALL SHOWT(0, 5)
      CALL SYNC('#685')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:8)
      PRINT *, '=685', IOS
      CALL SHOWT(0, 8)
      CALL SYNC('#686')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:3)
      PRINT *, '=686', IOS
      CALL SHOWT(0, 3)
      CALL SYNC('#687')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) S(1,1)
      PRINT *, '=687', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#688')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) VD(1,1)
      PRINT *, '=688', IOS
      CALL SHOWV(0, 2)
      CALL SYNC('#689')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=689', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#690')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) S(1,1)
      PRINT *, '=690', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#691')
      CALL PRE
      READ (5, '(T7,A3,T1,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      PRINT *, '=691', IOS
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL SYNC('#692')
      CALL PRE
      READ (5, '(TR2,A1,TL2,A1)', IOSTAT=IOS) TX(1)(1:1), TX(2)(1:1)
      PRINT *, '=692', IOS
      CALL SHOWT(0, 1)
      CALL SHOWT(1, 1)
      CALL SYNC('#693')
      CALL PRE
      READ (5, '(1X,I2,3X,I2)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=693', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#694')
      CALL PRE
      READ (5, '(T50,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=694', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#695')
      CALL PRE
      READ (5, '(TL50,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=695', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#696')
      CALL PRE
      READ (5, '(Q,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      PRINT *, '=696', IOS
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL SYNC('#697')
      CALL PRE
      READ (5, '(A2,Q)', IOSTAT=IOS) TX(1)(1:2), S(1,2)
      PRINT *, '=697', IOS
      CALL SHOWT(0, 2)
      CALL SHOWV(1, 1)
      CALL SYNC('#698')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) VW(1,1)
      PRINT *, '=698', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#699')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) VF(1,1)
      PRINT *, '=699', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R5
      INTEGER IOS
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
      PRINT *, '=700', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#701')
      CALL PRE
      READ (5, '(5HXXXXX,A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=701', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#702')
      CALL PRE
      READ (5, '(''XXXXX'',A1)', IOSTAT=IOS) TX(1)(1:1)
      PRINT *, '=702', IOS
      CALL SHOWT(0, 1)
      CALL SYNC('#703')
      CALL PRE
      READ (5, '(2I6,2F10.2)', IOSTAT=IOS) S(1,1), S(1,2), VF(1,3), VF(1
     &,4)
      PRINT *, '=703', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SYNC('#704')
      CALL PRE
      READ (5, '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=704', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#705')
      CALL PRE
      READ (5, '(2I6)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=705', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#706')
      CALL PRE
      READ (5, '(I6,A3)', IOSTAT=IOS) S(1,1), TX(2)(1:3)
      PRINT *, '=706', IOS
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 3)
      CALL SYNC('#707')
      CALL PRE
      READ (5, '(A3,I3)', IOSTAT=IOS) TX(1)(1:3), S(1,2)
      PRINT *, '=707', IOS
      CALL SHOWT(0, 3)
      CALL SHOWV(1, 1)
      CALL SYNC('#708')
      CALL PRE
      READ (5, '(L6,I3)', IOSTAT=IOS) VLU(1,1), S(1,2)
      PRINT *, '=708', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#709')
      CALL PRE
      READ (5, '(O6,Z6,I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=709', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#710')
      CALL PRE
      READ (5, '(I3,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=710', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#711')
      CALL PRE
      READ (5, '(3I3)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3)
      PRINT *, '=711', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#712')
      CALL PRE
      READ (5, '(A3,A3)', IOSTAT=IOS) TX(1)(1:3), TX(2)(1:3)
      PRINT *, '=712', IOS
      CALL SHOWT(0, 3)
      CALL SHOWT(1, 3)
      CALL SYNC('#713')
      CALL PRE
      READ (5, '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=713', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#714')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=714', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#715')
      CALL PRE
      READ (5, '(I3,:,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=715', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#716')
      CALL PRE
      READ (5, '(I3,$)', IOSTAT=IOS) S(1,1)
      PRINT *, '=716', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#717')
      CALL PRE
      READ (5, '(SP,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=717', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#718')
      CALL PRE
      READ (5, '(BN,I3,BZ,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=718', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#719')
      CALL PRE
      READ (5, '(BZ,I3,BN,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=719', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#732')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=732', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#733')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=733', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#734')
      CALL PRE
      READ (5, '(BN,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=734', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#735')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=735', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#736')
      CALL PRE
      READ (5, '(I3,/,I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=736', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#737')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=737', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#738')
      CALL PRE
      READ (5, '(/,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=738', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#739')
      CALL PRE
      READ (5, '(//,I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=739', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#740')
      CALL PRE
      READ (5, '(I3,/)', IOSTAT=IOS) S(1,1)
      PRINT *, '=740', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#741')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=741', IOS
      CALL SHOWT(0, 5)
      CALL SYNC('#742')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:5)
      PRINT *, '=742', IOS
      CALL SHOWT(0, 5)
      CALL SYNC('#743')
      CALL PRE
      READ (5, '(5HXXXXX)', IOSTAT=IOS)
      PRINT *, '=743', IOS
      CALL SYNC('#744')
      CALL PRE
      READ (5, '(Q,A)', IOSTAT=IOS) S(1,1), TX(2)(1:5)
      PRINT *, '=744', IOS
      CALL SHOWV(0, 1)
      CALL SHOWT(1, 5)
      CALL SYNC('#745')
      CALL PRE
      READ (5, '(A)', IOSTAT=IOS) TX(1)(1:4)
      PRINT *, '=745', IOS
      CALL SHOWT(0, 4)
      CALL SYNC('#746')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=746', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#747')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=747', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#748')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=748', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#749')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=749', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#750')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=750', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#751')
      CALL PRE
      READ (5, '(Q)', IOSTAT=IOS) S(1,1)
      PRINT *, '=751', IOS
      CALL SHOWV(0, 1)
      END

      SUBROUTINE R6
      INTEGER IOS
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
      PRINT *, '=752', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SYNC('#753')
      CALL PRE
      READ (5, '()', IOSTAT=IOS)
      PRINT *, '=753', IOS
      CALL SYNC('#754')
      CALL PRE
      READ (5, '(1X)', IOSTAT=IOS)
      PRINT *, '=754', IOS
      CALL SYNC('#755')
      CALL PRE
      READ (5, '(1X,''A'')', IOSTAT=IOS) S(1,1)
      PRINT *, '=755', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#756')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=756', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#757')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1)
      PRINT *, '=757', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#758')
      CALL PRE
      READ (5, '(I3)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=758', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#759')
      CALL PRE
      READ (5, '(T1,I1,T1,I1)', IOSTAT=IOS) S(1,1), S(1,2)
      PRINT *, '=759', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#760')
      CALL PRE
      READ (5, '(I1,$)', IOSTAT=IOS) S(1,1)
      PRINT *, '=760', IOS
      CALL SHOWV(0, 1)
      CALL SYNC('#761')
      CALL PRE
      READ (5, '(I1,/,F4.1)', IOSTAT=IOS) S(1,1), VF(1,2)
      PRINT *, '=761', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#762')
      CALL PRE
      READ (5, '(F4.1)', IOSTAT=IOS) VF(1,1), VF(1,2)
      PRINT *, '=762', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SYNC('#763')
      CALL PRE
      READ (5, '(I1,F4.1,(I1))', IOSTAT=IOS) S(1,1), VF(1,2), S(1,3), S(
     &1,4)
      PRINT *, '=763', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SYNC('#764')
      CALL PRE
      READ (5, '(2I1,/,I1)', IOSTAT=IOS) S(1,1), S(1,2), S(1,3), S(1,4),
     & S(1,5), S(1,6)
      PRINT *, '=764', IOS
      CALL SHOWV(0, 1)
      CALL SHOWV(1, 1)
      CALL SHOWV(2, 1)
      CALL SHOWV(3, 1)
      CALL SHOWV(4, 1)
      CALL SHOWV(5, 1)
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
