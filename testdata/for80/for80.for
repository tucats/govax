C     FOR80.FOR - written by testdata/for80/gen.go: round 1's output
C     cases to unit 6, for govax. TestFor80Oracle holds its records to
C     vax/for80-vax.txt.
      PROGRAM FOR80
      CALL S1
      CALL S2
      CALL S3
      CALL S4
      CALL S5
      CALL S6
      CALL S7
      CALL S8
      CALL S9
      CALL S10
      CALL S11
      CALL S12
      CALL S13
      END

      SUBROUTINE S1
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#0'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) 284
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=0', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#1'
      WRITE (6, '(1X,I4)', IOSTAT=IOS) -284
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=1', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#2'
      WRITE (6, '(1X,I4)', IOSTAT=IOS) 0
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=2', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#3'
      WRITE (6, '(1X,I5)', IOSTAT=IOS) 174
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=3', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#4'
      WRITE (6, '(1X,I2)', IOSTAT=IOS) 3244
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=4', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#5'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) -473
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=5', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#6'
      WRITE (6, '(1X,I4.0)', IOSTAT=IOS) 0
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=6', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#7'
      WRITE (6, '(1X,I4.2)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=7', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#8'
      WRITE (6, '(1X,I4.4)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=8', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#9'
      WRITE (6, '(1X,I3.4)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=9', IOS, IE
     &, IS, IV, IU, IC
      WRITE (6,*) '#10'
      WRITE (6, '(1X,I1)', IOSTAT=IOS) 0
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=10', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#11'
      VW(1) = -7
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=11', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#12'
      VB(1) = -7
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=12', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#13'
      VWUI(1) = 1
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=13', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#14'
      VLUI(1) = 1
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=14', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#15'
      WRITE (6, '(1X,O6)', IOSTAT=IOS) 32767
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=15', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#16'
      VW(1) = -32767
      WRITE (6, '(1X,O6)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=16', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#17'
      WRITE (6, '(1X,O2)', IOSTAT=IOS) 14261
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=17', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#18'
      WRITE (6, '(1X,O4)', IOSTAT=IOS) 27
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=18', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#19'
      WRITE (6, '(1X,O4.2)', IOSTAT=IOS) 7
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=19', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#20'
      WRITE (6, '(1X,O4.4)', IOSTAT=IOS) 7
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=20', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#21'
      VF(1) = 10.5
      WRITE (6, '(1X,O8)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=21', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#22'
      WRITE (6, '(1X,O12)', IOSTAT=IOS) -1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=22', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#23'
      WRITE (6, '(1X,O6)', IOSTAT=IOS) -1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=23', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#24'
      WRITE (6, '(1X,O4.0)', IOSTAT=IOS) 0
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=24', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#25'
      VB(1) = -1
      WRITE (6, '(1X,O4)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=25', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#26'
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) 32767
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=26', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#27'
      VW(1) = -32767
      WRITE (6, '(1X,Z5)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=27', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#28'
      WRITE (6, '(1X,Z2)', IOSTAT=IOS) 16
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=28', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#29'
      VF(1) = -10.5
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=29', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#30'
      WRITE (6, '(1X,Z3.3)', IOSTAT=IOS) 2708
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=30', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#31'
      WRITE (6, '(1X,Z6.4)', IOSTAT=IOS) 2708
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=31', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#32'
      WRITE (6, '(1X,Z9)', IOSTAT=IOS) -1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=32', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#33'
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) -1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=33', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#34'
      VF(1) = 1.0
      WRITE (6, '(1X,Z8)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=34', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#35'
      VF(1) = 1.0
      WRITE (6, '(1X,O12)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=35', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#36'
      VD(1) = 1.0D0
      WRITE (6, '(1X,Z16)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=36', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#37'
      VB(1) = -1
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=37', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#38'
      VF(1) = 3.25
      WRITE (6, '(1X,I5)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=38', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#39'
      WRITE (6, '(1X,I5)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=39', IOS, I
     &E, IS, IV, IU, IC
      END

      SUBROUTINE S2
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#40'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I5)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=40', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#41'
      WRITE (6, '(1X,O5)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=41', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#42'
      WRITE (6, '(1X,Z5)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=42', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#43'
      VF(1) = 2.3547188
      WRITE (6, '(1X,F8.5)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=43', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#44'
      VF(1) = 8789.7361
      WRITE (6, '(1X,F9.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=44', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#45'
      VF(1) = 51.44
      WRITE (6, '(1X,F2.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=45', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#46'
      VF(1) = -23.24352
      WRITE (6, '(1X,F10.4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=46', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#47'
      VF(1) = 325.013
      WRITE (6, '(1X,F5.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=47', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#48'
      VF(1) = -0.2
      WRITE (6, '(1X,F5.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=48', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#49'
      VF(1) = -0.2
      WRITE (6, '(1X,F4.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=49', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#50'
      VF(1) = 0.5
      WRITE (6, '(1X,F3.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=50', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#51'
      VF(1) = 0.5
      WRITE (6, '(1X,F2.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=51', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#52'
      VF(1) = 0.0
      WRITE (6, '(1X,F1.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=52', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#53'
      VF(1) = 0.0
      WRITE (6, '(1X,F2.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=53', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#54'
      VF(1) = 0.0
      WRITE (6, '(1X,F8.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=54', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#55'
      VF(1) = 0.4
      WRITE (6, '(1X,F5.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=55', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#56'
      VF(1) = 0.6
      WRITE (6, '(1X,F5.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=56', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#57'
      VF(1) = -0.01
      WRITE (6, '(1X,F6.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=57', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#58'
      VF(1) = -0.06
      WRITE (6, '(1X,F6.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=58', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#59'
      VF(1) = 2.25
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=59', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#60'
      VF(1) = 2.35
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=60', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#61'
      VF(1) = 2.45
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=61', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#62'
      VF(1) = 99.995
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=62', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#63'
      VF(1) = 0.5
      WRITE (6, '(1X,F2.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=63', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#64'
      VF(1) = 0.5
      WRITE (6, '(1X,F3.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=64', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#65'
      VF(1) = 1.0E20
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=65', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#66'
      VD(1) = 0.1D0
      WRITE (6, '(1X,F20.15)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=66', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#68'
      VH(1) = 0.1Q0
      WRITE (6, '(1X,F45.40)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=68', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#69'
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=69', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#70'
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=70', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#71'
      VW(1) = 3
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=71', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#72'
      VF(1) = 475867.222
      WRITE (6, '(1X,E9.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=72', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#73'
      VF(1) = 475867.222
      WRITE (6, '(1X,E12.5)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=73', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#74'
      VF(1) = 0.00069
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=74', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#75'
      VF(1) = -0.5555
      WRITE (6, '(1X,E10.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=75', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#76'
      VF(1) = -0.5555
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=76', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#77'
      VF(1) = 56.12
      WRITE (6, '(1X,E5.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=77', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#78'
      VF(1) = -1.001
      WRITE (6, '(1X,E14.5E4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=78', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#79'
      VF(1) = 0.000123
      WRITE (6, '(1X,E14.3E6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=79', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#80'
      VF(1) = 0.0
      WRITE (6, '(1X,E10.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=80', IOS, I
     &E, IS, IV, IU, IC
      END

      SUBROUTINE S3
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#81'
      VF(1) = 0.5555
      WRITE (6, '(1X,E9.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=81', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#82'
      VF(1) = 0.5555
      WRITE (6, '(1X,E8.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=82', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#83'
      VF(1) = 0.5555
      WRITE (6, '(1X,E7.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=83', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#84'
      VF(1) = 1.0E5
      WRITE (6, '(1X,E12.4E1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=84', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#85'
      VF(1) = 1.0E12
      WRITE (6, '(1X,E12.4E1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=85', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#86'
      VF(1) = 1.0E5
      WRITE (6, '(1X,E12.4E4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=86', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#87'
      VF(1) = 1.0
      WRITE (6, '(1X,E5.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=87', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#88'
      VF(1) = 1.0
      WRITE (6, '(1X,E6.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=88', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#89'
      VF(1) = 1.0
      WRITE (6, '(1X,E7.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=89', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#90'
      VF(1) = 1.0
      WRITE (6, '(1X,E8.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=90', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#91'
      VF(1) = 0.0363
      WRITE (6, '(1X,D14.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=91', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#92'
      VF(1) = 1.2
      WRITE (6, '(1X,D9.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=92', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#93'
      VF(1) = 1.0E-38
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=93', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#94'
      VF(1) = 1.0E38
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=94', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#95'
      VF(1) = 1.0E9
      WRITE (6, '(1X,E9.2E1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=95', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#96'
      VF(1) = 1.0E9
      WRITE (6, '(1X,E9.2E2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=96', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#97'
      VD(1) = 5413.87625793D0
      WRITE (6, '(1X,D23.12)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=97', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#98'
      VD(1) = 0.0363D0
      WRITE (6, '(1X,D14.3)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=98', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#99'
      VD(1) = 1.0D30
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=99', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#100'
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=100', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#103'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=103', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#104'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=104', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#107'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=107', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#108'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=108', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#111'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=111', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#112'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=112', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#115'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=115', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#116'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=116', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#119'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=119', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#120'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=120', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#123'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=123', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#124'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=124', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#125'
      VD(1) = 1.0D30
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=125', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#127'
      VF(1) = -270.139
      WRITE (6, '(1X,1PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=127', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#128'
      VF(1) = -270.139
      WRITE (6, '(1X,1PE12.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=128', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#129'
      VF(1) = -270.139
      WRITE (6, '(1X,-1PE12.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=129', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#130'
      VF(1) = 270.139
      WRITE (6, '(1X,2PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=130', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#131'
      VF(1) = 270.139
      WRITE (6, '(1X,4PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=131', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#132'
      VF(1) = 270.139
      WRITE (6, '(1X,5PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=132', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#133'
      VF(1) = 270.139
      WRITE (6, '(1X,-2PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=133', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S4
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#134'
      VF(1) = 270.139
      WRITE (6, '(1X,-3PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=134', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#135'
      VF(1) = 270.139
      WRITE (6, '(1X,-3PE12.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=135', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#136'
      VF(1) = 270.139
      WRITE (6, '(1X,-4PE12.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=136', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#137'
      VF(1) = 270.139
      WRITE (6, '(1X,0PE12.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=137', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#138'
      VF(1) = 25.0
      WRITE (6, '(1X,2PF8.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=138', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#139'
      VF(1) = 25.0
      WRITE (6, '(1X,-2PF8.2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=139', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#140'
      VF(1) = 5.0
      WRITE (6, '(1X,2PG13.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=140', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#141'
      VF(1) = 5.0E8
      WRITE (6, '(1X,2PG13.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=141', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#142'
      VF(1) = 0.0363
      WRITE (6, '(1X,1PD14.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=142', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#143'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      WRITE (6, '(1X,F8.2,2PF8.2,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=143', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#144'
      VF(1) = 25.0
      VF(2) = 25.0
      WRITE (6, '(1X,2PF8.2,0PF8.2)', IOSTAT=IOS) VF(1), VF(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=144', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#145'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      VF(4) = 25.0
      VF(5) = 25.0
      VF(6) = 25.0
      WRITE (6, '(1X,F8.2,2PF8.2,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
     &, VF(4), VF(5), VF(6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=145', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#146'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      WRITE (6, '(1X,F8.2,2P,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=146', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#147'
      VF(2) = 25.0
      WRITE (6, '(1X,2P,I4,F8.2)', IOSTAT=IOS) 5, VF(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=147', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#148'
      VF(1) = 0.01234567
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=148', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#149'
      VF(1) = -0.12345678
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=149', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#150'
      VF(1) = 1.23456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=150', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#151'
      VF(1) = 12.3456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=151', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#152'
      VF(1) = 123.456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=152', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#153'
      VF(1) = -1234.56789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=153', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#154'
      VF(1) = 12345.6789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=154', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#155'
      VF(1) = 123456.789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=155', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#156'
      VF(1) = -1234567.89
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=156', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#157'
      VF(1) = 0.0
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=157', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#158'
      VF(1) = 0.0
      WRITE (6, '(1X,G15.7E2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=158', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#159'
      VF(1) = 0.0
      WRITE (6, '(1X,1PG15.7E2)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=159', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#160'
      VF(1) = 0.09999999
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=160', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#161'
      VF(1) = 999.6
      WRITE (6, '(1X,G13.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=161', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#162'
      VF(1) = 0.09996
      WRITE (6, '(1X,G13.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=162', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#163'
      VF(1) = 5.0
      WRITE (6, '(1X,G8.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=163', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#164'
      VF(1) = 5.0
      WRITE (6, '(1X,G7.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=164', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#165'
      VF(1) = 5.0
      WRITE (6, '(1X,G6.3)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=165', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#166'
      VF(1) = 5.0
      WRITE (6, '(1X,G13.0)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=166', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#167'
      VF(1) = 5.0
      WRITE (6, '(1X,G13.3E4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=167', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#168'
      VF(1) = 5.0E8
      WRITE (6, '(1X,G13.3E4)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=168', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#169'
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) 5
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=169', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#170'
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=170', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#171'
      VD(1) = 5.0D0
      WRITE (6, '(1X,G25.15)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=171', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#172'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,SP,F8.2,SS,F8.2,S,F8.2)', IOSTAT=IOS) VF(1), VF(2),
     & VF(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=172', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#173'
      WRITE (6, '(1X,SP,I5,SS,I5,S,I5)', IOSTAT=IOS) 1, 1, 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=173', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S5
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#174'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,SP,E12.3,G13.3,D12.3)', IOSTAT=IOS) VF(1), VF(2), V
     &F(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=174', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#175'
      WRITE (6, '(1X,SP,O5,Z5)', IOSTAT=IOS) 1, 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=175', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#176'
      WRITE (6, '(1X,SP,I1)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=176', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#177'
      VF(1) = 1.0
      WRITE (6, '(1X,SP,F3.1)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=177', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#178'
      WRITE (6, '(1X,SP,I5)', IOSTAT=IOS) 0
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=178', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#179'
      VLUI(1) = 1
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=179', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#180'
      VLUI(1) = 0
      WRITE (6, '(1X,L1)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=180', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#181'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=181', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#182'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=182', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#183'
      VWUI(1) = 1
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=183', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#184'
      VF(1) = 3.25
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=184', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#185'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 'HI'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=185', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#186'
      VLUI(1) = 1
      WRITE (6, '(1X,L0)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=186', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#187'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'OHMS'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=187', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#188'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'VOLTS'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=188', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#189'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'AMPERES'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=189', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#190'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=190', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#191'
      WRITE (6, '(1X,A3)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=191', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#192'
      WRITE (6, '(1X,A8)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=192', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#193'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 1094861636
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=193', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#194'
      WRITE (6, '(1X,A2)', IOSTAT=IOS) 1094861636
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=194', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#195'
      WRITE (6, '(1X,A6)', IOSTAT=IOS) 1094861636
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=195', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#196'
      VW(1) = 16706
      WRITE (6, '(1X,A)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=196', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#197'
      VB(1) = 65
      WRITE (6, '(1X,A)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=197', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#198'
      VF(1) = 1.0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=198', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#199'
      VD(1) = 1.0D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=199', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#200'
      VLUI(1) = 1094861636
      WRITE (6, '(1X,A)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=200', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#201'
      WRITE (6, '(1X,I)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=201', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#202'
      VW(1) = 42
      WRITE (6, '(1X,I)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=202', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#203'
      VB(1) = 42
      WRITE (6, '(1X,I)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=203', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#204'
      VLUI(1) = 1
      WRITE (6, '(1X,I)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=204', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#205'
      VWUI(1) = 1
      WRITE (6, '(1X,I)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=205', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#206'
      VF(1) = 3.25
      WRITE (6, '(1X,I)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=206', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#207'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=207', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#209'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=209', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#210'
      WRITE (6, '(1X,I)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=210', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#211'
      WRITE (6, '(1X,O)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=211', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#212'
      VW(1) = 42
      WRITE (6, '(1X,O)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=212', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#213'
      VB(1) = 42
      WRITE (6, '(1X,O)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=213', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#214'
      VLUI(1) = 1
      WRITE (6, '(1X,O)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=214', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S6
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#215'
      VWUI(1) = 1
      WRITE (6, '(1X,O)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=215', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#216'
      VF(1) = 3.25
      WRITE (6, '(1X,O)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=216', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#217'
      VD(1) = 3.25D0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=217', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#219'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=219', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#220'
      WRITE (6, '(1X,O)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=220', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#221'
      WRITE (6, '(1X,Z)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=221', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#222'
      VW(1) = 42
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=222', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#223'
      VB(1) = 42
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=223', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#224'
      VLUI(1) = 1
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=224', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#225'
      VWUI(1) = 1
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=225', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#226'
      VF(1) = 3.25
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=226', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#227'
      VD(1) = 3.25D0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=227', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#229'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=229', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#230'
      WRITE (6, '(1X,Z)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=230', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#231'
      WRITE (6, '(1X,L)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=231', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#232'
      VW(1) = 42
      WRITE (6, '(1X,L)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=232', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#233'
      VB(1) = 42
      WRITE (6, '(1X,L)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=233', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#234'
      VLUI(1) = 1
      WRITE (6, '(1X,L)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=234', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#235'
      VWUI(1) = 1
      WRITE (6, '(1X,L)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=235', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#236'
      VF(1) = 3.25
      WRITE (6, '(1X,L)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=236', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#237'
      VD(1) = 3.25D0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=237', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#239'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=239', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#240'
      WRITE (6, '(1X,L)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=240', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#241'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=241', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#242'
      VW(1) = 42
      WRITE (6, '(1X,A)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=242', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#243'
      VB(1) = 42
      WRITE (6, '(1X,A)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=243', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#244'
      VLUI(1) = 1
      WRITE (6, '(1X,A)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=244', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#245'
      VWUI(1) = 1
      WRITE (6, '(1X,A)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=245', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#246'
      VF(1) = 3.25
      WRITE (6, '(1X,A)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=246', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#247'
      VD(1) = 3.25D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=247', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#249'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=249', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#250'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=250', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#251'
      WRITE (6, '(1X,F)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=251', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#252'
      VW(1) = 42
      WRITE (6, '(1X,F)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=252', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#253'
      VB(1) = 42
      WRITE (6, '(1X,F)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=253', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#254'
      VLUI(1) = 1
      WRITE (6, '(1X,F)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=254', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#255'
      VWUI(1) = 1
      WRITE (6, '(1X,F)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=255', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#256'
      VF(1) = 3.25
      WRITE (6, '(1X,F)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=256', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#257'
      VD(1) = 3.25D0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=257', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#259'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=259', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S7
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#260'
      WRITE (6, '(1X,F)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=260', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#261'
      WRITE (6, '(1X,E)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=261', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#262'
      VW(1) = 42
      WRITE (6, '(1X,E)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=262', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#263'
      VB(1) = 42
      WRITE (6, '(1X,E)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=263', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#264'
      VLUI(1) = 1
      WRITE (6, '(1X,E)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=264', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#265'
      VWUI(1) = 1
      WRITE (6, '(1X,E)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=265', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#266'
      VF(1) = 3.25
      WRITE (6, '(1X,E)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=266', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#267'
      VD(1) = 3.25D0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=267', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#269'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=269', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#270'
      WRITE (6, '(1X,E)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=270', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#271'
      WRITE (6, '(1X,G)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=271', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#272'
      VW(1) = 42
      WRITE (6, '(1X,G)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=272', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#273'
      VB(1) = 42
      WRITE (6, '(1X,G)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=273', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#274'
      VLUI(1) = 1
      WRITE (6, '(1X,G)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=274', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#275'
      VWUI(1) = 1
      WRITE (6, '(1X,G)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=275', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#276'
      VF(1) = 3.25
      WRITE (6, '(1X,G)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=276', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#277'
      VD(1) = 3.25D0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=277', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#279'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=279', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#280'
      WRITE (6, '(1X,G)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=280', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#281'
      WRITE (6, '(1X,D)', IOSTAT=IOS) 42
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=281', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#282'
      VW(1) = 42
      WRITE (6, '(1X,D)', IOSTAT=IOS) VW(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=282', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#283'
      VB(1) = 42
      WRITE (6, '(1X,D)', IOSTAT=IOS) VB(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=283', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#284'
      VLUI(1) = 1
      WRITE (6, '(1X,D)', IOSTAT=IOS) VLU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=284', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#285'
      VWUI(1) = 1
      WRITE (6, '(1X,D)', IOSTAT=IOS) VWU(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=285', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#286'
      VF(1) = 3.25
      WRITE (6, '(1X,D)', IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=286', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#287'
      VD(1) = 3.25D0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=287', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#289'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VH(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=289', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#290'
      WRITE (6, '(1X,D)', IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=290', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#291'
      VFC(1) = (2.3547188,3.456732)
      WRITE (6, '(1X,2F8.5)', IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=291', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#292'
      VFC(1) = (47587.222,56.123)
      WRITE (6, '(1X,E9.2,'', '',E5.3)', IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=292', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#293'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=293', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#294'
      VFC(1) = (1.0,2.0)
      VFC(2) = (3.0,4.0)
      WRITE (6, '(1X,F8.2,''|'',F8.2,''|'',F8.2,''|'',F8.2)', IOSTAT=IOS
     &) VFC(1), VFC(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=294', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#295'
      VDC(1) = (1.5D0,-2.5D0)
      WRITE (6, '(1X,2F12.5)', IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=295', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#297'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,I5,I5)', IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=297', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#298'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,F)', IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=298', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#299'
      VDC(1) = (1.0D0,2.0D0)
      WRITE (6, '(1X,D)', IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=299', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#300'
      WRITE (6, '(''A'',T5,''B'',TL2,''C'',TR2,''D'',3X,''E'',5X)', IOST
     &AT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=300', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#301'
      WRITE (6, '(T10,''X'',T1,'' Y'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=301', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#302'
      WRITE (6, '(1X,''K='',I6,5X)', IOSTAT=IOS) 5
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=302', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#303'
      WRITE (6, '(3HABC,2H12)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=303', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S8
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#304'
      WRITE (6, '('' A''''B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=304', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#305'
      WRITE (6, '(3H''B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=305', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#306'
      WRITE (6, '(TL5,'' A'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=306', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#307'
      WRITE (6, '('' ABC'',TL100,''D'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=307', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#308'
      WRITE (6, '(T1,''A'',T1,''B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=308', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#309'
      WRITE (6, '(T200,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=309', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#310'
      WRITE (6, '(1X,''ABC'',T2,''D'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=310', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#311'
      WRITE (6, '(1X,''A'',0X,''B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=311', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#312'
      WRITE (6, '('' '',TR0,''A'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=312', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#313'
      WRITE (6, '('' '',T0,''A'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=313', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#314'
      WRITE (6, '(1X''ABC'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=314', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#315'
      WRITE (6, '(''A''1X)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=315', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#316'
      WRITE (6, '(''A''1X''B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=316', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#317'
      WRITE (6, '('' A''/'' B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=317', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#318'
      WRITE (6, '(1X,I3''A'')', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=318', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#319'
      WRITE (6, '(1X''A''I3)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=319', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#320'
      WRITE (6, '(1X,2''A'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=320', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#321'
      WRITE (6, '(1X,3HAB)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=321', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#322'
      WRITE (6, '(1X,'''')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=322', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#323'
      WRITE (6, '(1X,I3,,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=323', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#324'
      WRITE (6, '(1X I3 I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=324', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#325'
      WRITE (6, '(1XI3I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=325', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#326'
      WRITE (6, '( 1 X , I 3 )', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=326', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#327'
      WRITE (6, '(1x,i3,i3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=327', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#328'
      WRITE (6, '(1X,0I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=328', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#329'
      WRITE (6, '(1X,I0)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=329', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#330'
      WRITE (6, '(1X,0X,''A'')', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=330', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#331'
      WRITE (6, '(1X,J3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=331', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#332'
      WRITE (6, '(1X,I3', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=332', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#333'
      WRITE (6, '1X,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=333', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#334'
      WRITE (6, '(1X,I3))', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=334', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#335'
      WRITE (6, '(1X,I3)XYZ', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=335', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#336'
      WRITE (6, '  (1X,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=336', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#337'
      WRITE (6, '(1X,2)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=337', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#338'
      WRITE (6, '(1X,2X3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=338', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#339'
      WRITE (6, '(1X,+3I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=339', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#340'
      WRITE (6, '(1X,-1I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=340', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#341'
      WRITE (6, '(1X,I-3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=341', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#342'
      WRITE (6, '(1X,I3.-1)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=342', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#343'
      WRITE (6, '(1X,F8)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=343', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S9
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#344'
      WRITE (6, '(1X,F.2)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=344', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#345'
      WRITE (6, '(1X,F8.)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=345', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#346'
      WRITE (6, '(1X,E8.2E)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=346', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#347'
      WRITE (6, '(1X,G8.2E0)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=347', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#348'
      WRITE (6, '(1X,I3.2.1)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=348', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#349'
      WRITE (6, '(1X,1P)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=349', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#350'
      WRITE (6, '(1X,P)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=350', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#351'
      WRITE (6, '(1X,P,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=351', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#352'
      WRITE (6, '(1X,2P3I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=352', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#353'
      WRITE (6, '(1X,2PI3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=353', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#354'
      WRITE (6, '(1X,+2PF8.2)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=354', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#355'
      WRITE (6, '(1X,I32768)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=355', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#356'
      WRITE (6, '(1X,32768I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=356', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#357'
      WRITE (6, '(1X,I32767)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=357', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#358'
      WRITE (6, '(1X,F8.256)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=358', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#359'
      WRITE (6, '(1X,F8.255)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=359', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#360'
      WRITE (6, '(1X,E12.4E256)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=360', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#361'
      WRITE (6, '(1X,128PF8.2)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=361', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#362'
      WRITE (6, '(1X,-129PF8.2)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=362', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#363'
      WRITE (6, '(1X,<3>I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=363', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#364'
      WRITE (6, '(1X,I<3>)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=364', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#365'
      WRITE (6, '(1X,''A)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=365', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#366'
      WRITE (6, '(1X,3HAB', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=366', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#367'
      WRITE (6, '(1X,0HA)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=367', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#368'
      WRITE (6, '(1X,H)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=368', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#369'
      WRITE (6, '(1X,T)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=369', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#370'
      WRITE (6, '(1X,TL)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=370', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#371'
      WRITE (6, '(1X,TR)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=371', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#372'
      WRITE (6, '(1X,X)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=372', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#373'
      WRITE (6, '(1X,3$)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=373', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#374'
      WRITE (6, '(1X,3:)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=374', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#375'
      WRITE (6, '(1X,3/)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=375', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#376'
      WRITE (6, '(1X,BN3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=376', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#377'
      WRITE (6, '(1X,3BN)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=377', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#378'
      WRITE (6, '(1X,Q3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=378', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#379'
      WRITE (6, '(1X,3Q)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=379', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#380'
      WRITE (6, '(1X,SP3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=380', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#381'
      WRITE (6, '(1X,3SP)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=381', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#382'
      WRITE (6, '(1X,3T5)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=382', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#383'
      WRITE (6, '(1X,I3:I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=383', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S10
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#384'
      WRITE (6, '(1X,I3/I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=384', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#385'
      WRITE (6, '(1X,I3$)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=385', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#386'
      WRITE (6, '(1X,/I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=386', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#387'
      WRITE (6, '(1X,I3/,/I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=387', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#388'
      WRITE (6, '(1X,:I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=388', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#389'
      WRITE (6, '(1X,$,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=389', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#390'
      WRITE (6, '(1X,/,I3)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=390', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#392'
      WRITE (6, ' ', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=392', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#393'
      WRITE (6, '()', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=393', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#394'
      WRITE (6, '( )', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=394', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#395'
      WRITE (6, '(,)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=395', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#396'
      WRITE (6, '(1X,)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=396', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#397'
      WRITE (6, '(,1X)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=397', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#398'
      WRITE (6, '(1X,2(I2,1X))', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=398', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#399'
      WRITE (6, '(1X,2(I2,1X),I3)', IOSTAT=IOS) 1, 2, 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=399', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#400'
      WRITE (6, '(1X,2(2(I2,1X)))', IOSTAT=IOS) 1, 2, 3, 4
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=400', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#401'
      WRITE (6, '(1X,(I2))', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=401', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#402'
      WRITE (6, '(1X,0(I2))', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=402', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#403'
      WRITE (6, '(1X,())', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=403', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#404'
      WRITE (6, '(1X,(((I3))))', IOSTAT=IOS) 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=404', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#405'
      WRITE (6, '(1X,((((((((I3)))))))))', IOSTAT=IOS) 8
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=405', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#406'
      WRITE (6, '(1X,((((((((((((((((I3)))))))))))))))))', IOSTAT=IOS) 1
     &6
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=406', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#407'
      WRITE (6, '(1X,((((((((((((((((((((I3)))))))))))))))))))))', IOSTA
     &T=IOS) 20
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=407', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#408'
      WRITE (6, '(1X,(((((((((((((((((((((I3))))))))))))))))))))))', IOS
     &TAT=IOS) 21
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=408', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#409'
      WRITE (6, '(1X,((((((((((((((((((((((((((((((((I3)))))))))))))))))
     &))))))))))))))))', IOSTAT=IOS) 32
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=409', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#410'
      WRITE (6, '(1X,(((((((((((((((((((((((((((((((((((((((((((((((((((
     &(((((((((((((I3)))))))))))))))))))))))))))))))))))))))))))))))))))
     &))))))))))))))', IOSTAT=IOS) 64
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=410', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#411'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) 1, 2, 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=411', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#412'
      WRITE (6, '(1X,I3,/,1X,I3)', IOSTAT=IOS) 1, 2, 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=412', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#413'
      WRITE (6, '(1X,I3,2(1X,I2,1X,I4))', IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 
     &7, 8
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=413', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#414'
      WRITE (6, '(1X,I3,(1X,I2),I4)', IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7, 8
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=414', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#415'
      WRITE (6, '(1X,I3,(1X,I2),(1X,I4))', IOSTAT=IOS) 1, 2, 3, 4, 5, 6
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=415', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#416'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,2PF8.2,(F8.2))', IOSTAT=IOS) VF(1), VF(2), VF(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=416', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#417'
      WRITE (6, '(1X,SP,I3,(I3))', IOSTAT=IOS) 1, 2, 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=417', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#418'
      WRITE (6, '(//,'' A''//)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=418', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#419'
      WRITE (6, '('' X'',/)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=419', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#420'
      WRITE (6, '(/)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=420', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#421'
      WRITE (6, '(//)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=421', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#422'
      WRITE (6, '()', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=422', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#423'
      WRITE (6, '('' I='',I2,'' J='',I2)', IOSTAT=IOS) 3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=423', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#424'
      WRITE (6, '('' K='',I2,:,'' L='',I2)', IOSTAT=IOS) 4
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=424', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S11
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#425'
      WRITE (6, '('' K='',I2,:)', IOSTAT=IOS) 4, 5
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=425', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#426'
      WRITE (6, '(:,'' A'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=426', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#427'
      WRITE (6, '('' A'',:,'' B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=427', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#428'
      WRITE (6, '(1X,''A'')', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=428', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#429'
      WRITE (6, '(1X)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=429', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#430'
      WRITE (6, '('''')', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=430', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#431'
      WRITE (6, '(1X,Q,I3)', IOSTAT=IOS) 9, 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=431', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#432'
      WRITE (6, '(1X,Q)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=432', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#433'
      WRITE (6, '(1X,BN,I3,BZ)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=433', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#434'
      WRITE (6, '('' A'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=434', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#435'
      WRITE (6, '('' B'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=435', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#436'
      WRITE (6, '(''+C'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=436', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#437'
      WRITE (6, '(''+D'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=437', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#438'
      WRITE (6, '(''$E'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=438', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#439'
      WRITE (6, '(''$F'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=439', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#440'
      WRITE (6, '(''0G'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=440', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#441'
      WRITE (6, '(''0H'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=441', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#442'
      WRITE (6, '(''1I'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=442', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#443'
      WRITE (6, '(''1J'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=443', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#444'
      WRITE (6, '(''XK'',$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=444', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#445'
      WRITE (6, '(''XL'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=445', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#446'
      WRITE (6, '(1X,$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=446', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#447'
      WRITE (6, '($)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=447', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#448'
      WRITE (6, '($,'' M'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=448', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#449'
      WRITE (6, '('' N'',$,'' O'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=449', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#450'
      WRITE (6, '('' P'',$/'' Q'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=450', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#451'
      WRITE (6, '('' R''/$)', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=451', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#452'
      WRITE (6, '(1X,I3,$)', IOSTAT=IOS) 1, 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=452', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#453'
      WRITE (6, '(1X,T79,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=453', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#454'
      WRITE (6, '(1X,T80,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=454', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#455'
      WRITE (6, '(1X,T81,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=455', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#456'
      WRITE (6, '(1X,T132,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=456', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#457'
      WRITE (6, '(1X,T133,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=457', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#458'
      WRITE (6, '(1X,T134,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=458', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#459'
      WRITE (6, '(1X,T256,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=459', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#460'
      WRITE (6, '(1X,T1000,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=460', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#461'
      WRITE (6, '(1X,T32767,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=461', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#462'
      WRITE (6, '(1X,T32768,''X'')', IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=462', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#463'
      WRITE (6, '(1X,131A1)', IOSTAT=IOS) 'A'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=463', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#464'
      WRITE (6, '(1X,132A1)', IOSTAT=IOS) 'A'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=464', IOS, 
     &IE, IS, IV, IU, IC
      END

      SUBROUTINE S12
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#465'
      WRITE (6, '(1X,133A1)', IOSTAT=IOS) 'A'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=465', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#466'
      WRITE (6, '(1X,134A1)', IOSTAT=IOS) 'A'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=466', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#467'
      WRITE (6, '(1X,I5000)', IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=467', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#482'
      WRITE (6, *, IOSTAT=IOS) 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=482', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#483'
      WRITE (6, *, IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=483', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#484'
      WRITE (6, *, IOSTAT=IOS) -2147483647, 2147483647
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=484', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#485'
      VW(1) = 7
      VB(2) = 7
      VLUI(3) = 1
      VWUI(4) = 1
      VLUI(5) = 0
      WRITE (6, *, IOSTAT=IOS) VW(1), VB(2), VLU(3), VWU(4), VLU(5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=485', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#486'
      VF(1) = 3.4
      WRITE (6, *, IOSTAT=IOS) VF(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=486', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#487'
      VF(1) = 3.4
      VF(2) = 3.4
      VF(3) = 3.4
      VF(4) = 3.4
      VF(5) = 3.4
      WRITE (6, *, IOSTAT=IOS) VF(1), VF(2), VF(3), VF(4), VF(5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=487', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#488'
      VF(1) = 3.4
      VF(2) = 3.4
      VF(3) = 3.4
      VF(4) = 3.4
      VF(5) = 3.4
      VF(6) = 3.4
      WRITE (6, *, IOSTAT=IOS) VF(1), VF(2), VF(3), VF(4), VF(5), VF(6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=488', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#489'
      VF(1) = 0.0
      VF(2) = 1.0E20
      VF(3) = -1.0E-20
      VF(4) = 0.1
      VF(5) = 1.0E6
      VF(6) = 1.0E7
      WRITE (6, *, IOSTAT=IOS) VF(1), VF(2), VF(3), VF(4), VF(5), VF(6)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=489', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#490'
      VD(1) = 3.4D0
      WRITE (6, *, IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=490', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#491'
      VD(1) = 3.4D0
      VD(2) = 3.4D0
      VD(3) = 3.4D0
      VD(4) = 3.4D0
      WRITE (6, *, IOSTAT=IOS) VD(1), VD(2), VD(3), VD(4)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=491', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#492'
      VD(1) = 0.0D0
      VD(2) = 1.0D20
      VD(3) = -1.0D-20
      WRITE (6, *, IOSTAT=IOS) VD(1), VD(2), VD(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=492', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#494'
      VH(1) = 3.4Q0
      VH(2) = 1.0Q2000
      WRITE (6, *, IOSTAT=IOS) VH(1), VH(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=494', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#495'
      VFC(1) = (1.0,2.0)
      WRITE (6, *, IOSTAT=IOS) VFC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=495', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#496'
      VFC(1) = (1.0,2.0)
      VFC(2) = (3.0,4.0)
      WRITE (6, *, IOSTAT=IOS) VFC(1), VFC(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=496', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#497'
      VFC(1) = (1.0,2.0)
      VFC(2) = (3.0,4.0)
      VFC(3) = (5.0,6.0)
      VFC(4) = (7.0,8.0)
      VFC(5) = (9.0,1.0)
      WRITE (6, *, IOSTAT=IOS) VFC(1), VFC(2), VFC(3), VFC(4), VFC(5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=497', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#498'
      VDC(1) = (1.0D0,-2.5D0)
      WRITE (6, *, IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=498', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#499'
      VDC(1) = (1.0D0,-2.5D0)
      VDC(2) = (1.0D0,-2.5D0)
      VDC(3) = (1.0D0,-2.5D0)
      WRITE (6, *, IOSTAT=IOS) VDC(1), VDC(2), VDC(3)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=499', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#501'
      WRITE (6, *, IOSTAT=IOS) 'HELLO'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=501', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#502'
      WRITE (6, *, IOSTAT=IOS) 'HELLO', 'WORLD'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=502', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#503'
      WRITE (6, *, IOSTAT=IOS) 1, 'HELLO', 2
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=503', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#504'
      WRITE (6, *, IOSTAT=IOS) 'HELLO', 1
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=504', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#505'
      VF(2) = 1.0
      VLUI(4) = 1
      VD(5) = 1.0D0
      WRITE (6, *, IOSTAT=IOS) 1, VF(2), 'A', VLU(4), VD(5)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=505', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#507'
      WRITE (6, *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=507', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#508'
      WRITE (6, *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=508', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#509'
      WRITE (6, *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=509', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#510'
      WRITE (6, *, IOSTAT=IOS) 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=510', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#511'
      VLUI(3) = 1
      VLUI(4) = 1
      VLUI(5) = 1
      WRITE (6, *, IOSTAT=IOS) 'AB', 'CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC
     &CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC
     &', VLU(3), VLU(4), VLU(5), -3
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=511', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#512'
      WRITE (6, *, IOSTAT=IOS)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=512', IOS, 
     &IE, IS, IV, IU, IC
      END

      OPTIONS /G_FLOATING
      SUBROUTINE S13
      INTEGER IOS, IE, IS, IV, IU, IC
      INTEGER*2 VW(16)
      BYTE VB(16)
      LOGICAL*4 VLU(16)
      INTEGER*4 VLUI(16)
      LOGICAL*2 VWU(16)
      INTEGER*2 VWUI(16)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(16)
      DOUBLE PRECISION VD(16)
      REAL*16 VH(16)
      COMPLEX VFC(16)
      COMPLEX*16 VDC(16)
      WRITE (6,*) '#67'
      VD(1) = 0.1D0
      WRITE (6, '(1X,F30.25)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=67', IOS, I
     &E, IS, IV, IU, IC
      WRITE (6,*) '#101'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=101', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#102'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=102', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#105'
      VD(1) = 1.0D100
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=105', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#106'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=106', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#109'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=109', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#110'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=110', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#113'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=113', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#114'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=114', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#117'
      VD(1) = 1.0D100
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=117', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#118'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=118', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#121'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=121', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#122'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=122', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#126'
      VD(1) = 1.0D30
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=126', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#208'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=208', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#218'
      VD(1) = 3.25D0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=218', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#228'
      VD(1) = 3.25D0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=228', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#238'
      VD(1) = 3.25D0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=238', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#248'
      VD(1) = 3.25D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=248', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#258'
      VD(1) = 3.25D0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=258', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#268'
      VD(1) = 3.25D0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=268', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#278'
      VD(1) = 3.25D0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=278', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#288'
      VD(1) = 3.25D0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VD(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=288', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#296'
      VDC(1) = (1.5D0,-2.5D0)
      WRITE (6, '(1X,2F12.5)', IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=296', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#493'
      VD(1) = 3.4D0
      VD(2) = 1.0D200
      WRITE (6, *, IOSTAT=IOS) VD(1), VD(2)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=493', IOS, 
     &IE, IS, IV, IU, IC
      WRITE (6,*) '#500'
      VDC(1) = (1.0D0,-2.5D0)
      WRITE (6, *, IOSTAT=IOS) VDC(1)
      CALL ERRSNS(IE, IS, IV, IU, IC)
      WRITE (6, '(1X,A,I6,I6,1X,Z8.8,1X,Z8.8,I6,1X,Z8.8)') '=500', IOS, 
     &IE, IS, IV, IU, IC
      END
