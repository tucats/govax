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
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#0'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) 284
      PRINT *, '=0', IOS
      PRINT *, '#1'
      WRITE (6, '(1X,I4)', IOSTAT=IOS) -284
      PRINT *, '=1', IOS
      PRINT *, '#2'
      WRITE (6, '(1X,I4)', IOSTAT=IOS) 0
      PRINT *, '=2', IOS
      PRINT *, '#3'
      WRITE (6, '(1X,I5)', IOSTAT=IOS) 174
      PRINT *, '=3', IOS
      PRINT *, '#4'
      WRITE (6, '(1X,I2)', IOSTAT=IOS) 3244
      PRINT *, '=4', IOS
      PRINT *, '#5'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) -473
      PRINT *, '=5', IOS
      PRINT *, '#6'
      WRITE (6, '(1X,I4.0)', IOSTAT=IOS) 0
      PRINT *, '=6', IOS
      PRINT *, '#7'
      WRITE (6, '(1X,I4.2)', IOSTAT=IOS) 1
      PRINT *, '=7', IOS
      PRINT *, '#8'
      WRITE (6, '(1X,I4.4)', IOSTAT=IOS) 1
      PRINT *, '=8', IOS
      PRINT *, '#9'
      WRITE (6, '(1X,I3.4)', IOSTAT=IOS) 1
      PRINT *, '=9', IOS
      PRINT *, '#10'
      WRITE (6, '(1X,I1)', IOSTAT=IOS) 0
      PRINT *, '=10', IOS
      PRINT *, '#11'
      VW(1) = -7
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VW(1)
      PRINT *, '=11', IOS
      PRINT *, '#12'
      VB(1) = -7
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VB(1)
      PRINT *, '=12', IOS
      PRINT *, '#13'
      VWUI(1) = 1
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VWU(1)
      PRINT *, '=13', IOS
      PRINT *, '#14'
      VLUI(1) = 1
      WRITE (6, '(1X,I6)', IOSTAT=IOS) VLU(1)
      PRINT *, '=14', IOS
      PRINT *, '#15'
      WRITE (6, '(1X,O6)', IOSTAT=IOS) 32767
      PRINT *, '=15', IOS
      PRINT *, '#16'
      VW(1) = -32767
      WRITE (6, '(1X,O6)', IOSTAT=IOS) VW(1)
      PRINT *, '=16', IOS
      PRINT *, '#17'
      WRITE (6, '(1X,O2)', IOSTAT=IOS) 14261
      PRINT *, '=17', IOS
      PRINT *, '#18'
      WRITE (6, '(1X,O4)', IOSTAT=IOS) 27
      PRINT *, '=18', IOS
      PRINT *, '#19'
      WRITE (6, '(1X,O4.2)', IOSTAT=IOS) 7
      PRINT *, '=19', IOS
      PRINT *, '#20'
      WRITE (6, '(1X,O4.4)', IOSTAT=IOS) 7
      PRINT *, '=20', IOS
      PRINT *, '#21'
      VF(1) = 10.5
      WRITE (6, '(1X,O8)', IOSTAT=IOS) VF(1)
      PRINT *, '=21', IOS
      PRINT *, '#22'
      WRITE (6, '(1X,O12)', IOSTAT=IOS) -1
      PRINT *, '=22', IOS
      PRINT *, '#23'
      WRITE (6, '(1X,O6)', IOSTAT=IOS) -1
      PRINT *, '=23', IOS
      PRINT *, '#24'
      WRITE (6, '(1X,O4.0)', IOSTAT=IOS) 0
      PRINT *, '=24', IOS
      PRINT *, '#25'
      VB(1) = -1
      WRITE (6, '(1X,O4)', IOSTAT=IOS) VB(1)
      PRINT *, '=25', IOS
      PRINT *, '#26'
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) 32767
      PRINT *, '=26', IOS
      PRINT *, '#27'
      VW(1) = -32767
      WRITE (6, '(1X,Z5)', IOSTAT=IOS) VW(1)
      PRINT *, '=27', IOS
      PRINT *, '#28'
      WRITE (6, '(1X,Z2)', IOSTAT=IOS) 16
      PRINT *, '=28', IOS
      PRINT *, '#29'
      VF(1) = -10.5
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) VF(1)
      PRINT *, '=29', IOS
      PRINT *, '#30'
      WRITE (6, '(1X,Z3.3)', IOSTAT=IOS) 2708
      PRINT *, '=30', IOS
      PRINT *, '#31'
      WRITE (6, '(1X,Z6.4)', IOSTAT=IOS) 2708
      PRINT *, '=31', IOS
      PRINT *, '#32'
      WRITE (6, '(1X,Z9)', IOSTAT=IOS) -1
      PRINT *, '=32', IOS
      PRINT *, '#33'
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) -1
      PRINT *, '=33', IOS
      PRINT *, '#34'
      VF(1) = 1.0
      WRITE (6, '(1X,Z8)', IOSTAT=IOS) VF(1)
      PRINT *, '=34', IOS
      PRINT *, '#35'
      VF(1) = 1.0
      WRITE (6, '(1X,O12)', IOSTAT=IOS) VF(1)
      PRINT *, '=35', IOS
      PRINT *, '#36'
      VD(1) = 1.0D0
      WRITE (6, '(1X,Z16)', IOSTAT=IOS) VD(1)
      PRINT *, '=36', IOS
      PRINT *, '#37'
      VB(1) = -1
      WRITE (6, '(1X,Z4)', IOSTAT=IOS) VB(1)
      PRINT *, '=37', IOS
      PRINT *, '#38'
      VF(1) = 3.25
      WRITE (6, '(1X,I5)', IOSTAT=IOS) VF(1)
      PRINT *, '=38', IOS
      PRINT *, '#39'
      WRITE (6, '(1X,I5)', IOSTAT=IOS) 'HI'
      PRINT *, '=39', IOS
      END

      SUBROUTINE S2
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#40'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I5)', IOSTAT=IOS) VD(1)
      PRINT *, '=40', IOS
      PRINT *, '#41'
      WRITE (6, '(1X,O5)', IOSTAT=IOS) 'HI'
      PRINT *, '=41', IOS
      PRINT *, '#42'
      WRITE (6, '(1X,Z5)', IOSTAT=IOS) 'HI'
      PRINT *, '=42', IOS
      PRINT *, '#43'
      VF(1) = 2.3547188
      WRITE (6, '(1X,F8.5)', IOSTAT=IOS) VF(1)
      PRINT *, '=43', IOS
      PRINT *, '#44'
      VF(1) = 8789.7361
      WRITE (6, '(1X,F9.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=44', IOS
      PRINT *, '#45'
      VF(1) = 51.44
      WRITE (6, '(1X,F2.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=45', IOS
      PRINT *, '#46'
      VF(1) = -23.24352
      WRITE (6, '(1X,F10.4)', IOSTAT=IOS) VF(1)
      PRINT *, '=46', IOS
      PRINT *, '#47'
      VF(1) = 325.013
      WRITE (6, '(1X,F5.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=47', IOS
      PRINT *, '#48'
      VF(1) = -0.2
      WRITE (6, '(1X,F5.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=48', IOS
      PRINT *, '#49'
      VF(1) = -0.2
      WRITE (6, '(1X,F4.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=49', IOS
      PRINT *, '#50'
      VF(1) = 0.5
      WRITE (6, '(1X,F3.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=50', IOS
      PRINT *, '#51'
      VF(1) = 0.5
      WRITE (6, '(1X,F2.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=51', IOS
      PRINT *, '#52'
      VF(1) = 0.0
      WRITE (6, '(1X,F1.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=52', IOS
      PRINT *, '#53'
      VF(1) = 0.0
      WRITE (6, '(1X,F2.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=53', IOS
      PRINT *, '#54'
      VF(1) = 0.0
      WRITE (6, '(1X,F8.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=54', IOS
      PRINT *, '#55'
      VF(1) = 0.4
      WRITE (6, '(1X,F5.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=55', IOS
      PRINT *, '#56'
      VF(1) = 0.6
      WRITE (6, '(1X,F5.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=56', IOS
      PRINT *, '#57'
      VF(1) = -0.01
      WRITE (6, '(1X,F6.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=57', IOS
      PRINT *, '#58'
      VF(1) = -0.06
      WRITE (6, '(1X,F6.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=58', IOS
      PRINT *, '#59'
      VF(1) = 2.25
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=59', IOS
      PRINT *, '#60'
      VF(1) = 2.35
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=60', IOS
      PRINT *, '#61'
      VF(1) = 2.45
      WRITE (6, '(1X,F5.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=61', IOS
      PRINT *, '#62'
      VF(1) = 99.995
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=62', IOS
      PRINT *, '#63'
      VF(1) = 0.5
      WRITE (6, '(1X,F2.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=63', IOS
      PRINT *, '#64'
      VF(1) = 0.5
      WRITE (6, '(1X,F3.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=64', IOS
      PRINT *, '#65'
      VF(1) = 1.0E20
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=65', IOS
      PRINT *, '#66'
      VD(1) = 0.1D0
      WRITE (6, '(1X,F20.15)', IOSTAT=IOS) VD(1)
      PRINT *, '=66', IOS
      PRINT *, '#68'
      VH(1) = 0.1Q0
      WRITE (6, '(1X,F45.40)', IOSTAT=IOS) VH(1)
      PRINT *, '=68', IOS
      PRINT *, '#69'
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) 3
      PRINT *, '=69', IOS
      PRINT *, '#70'
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) 'HI'
      PRINT *, '=70', IOS
      PRINT *, '#71'
      VW(1) = 3
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VW(1)
      PRINT *, '=71', IOS
      PRINT *, '#72'
      VF(1) = 475867.222
      WRITE (6, '(1X,E9.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=72', IOS
      PRINT *, '#73'
      VF(1) = 475867.222
      WRITE (6, '(1X,E12.5)', IOSTAT=IOS) VF(1)
      PRINT *, '=73', IOS
      PRINT *, '#74'
      VF(1) = 0.00069
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=74', IOS
      PRINT *, '#75'
      VF(1) = -0.5555
      WRITE (6, '(1X,E10.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=75', IOS
      PRINT *, '#76'
      VF(1) = -0.5555
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=76', IOS
      PRINT *, '#77'
      VF(1) = 56.12
      WRITE (6, '(1X,E5.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=77', IOS
      PRINT *, '#78'
      VF(1) = -1.001
      WRITE (6, '(1X,E14.5E4)', IOSTAT=IOS) VF(1)
      PRINT *, '=78', IOS
      PRINT *, '#79'
      VF(1) = 0.000123
      WRITE (6, '(1X,E14.3E6)', IOSTAT=IOS) VF(1)
      PRINT *, '=79', IOS
      PRINT *, '#80'
      VF(1) = 0.0
      WRITE (6, '(1X,E10.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=80', IOS
      END

      SUBROUTINE S3
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#81'
      VF(1) = 0.5555
      WRITE (6, '(1X,E9.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=81', IOS
      PRINT *, '#82'
      VF(1) = 0.5555
      WRITE (6, '(1X,E8.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=82', IOS
      PRINT *, '#83'
      VF(1) = 0.5555
      WRITE (6, '(1X,E7.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=83', IOS
      PRINT *, '#84'
      VF(1) = 1.0E5
      WRITE (6, '(1X,E12.4E1)', IOSTAT=IOS) VF(1)
      PRINT *, '=84', IOS
      PRINT *, '#85'
      VF(1) = 1.0E12
      WRITE (6, '(1X,E12.4E1)', IOSTAT=IOS) VF(1)
      PRINT *, '=85', IOS
      PRINT *, '#86'
      VF(1) = 1.0E5
      WRITE (6, '(1X,E12.4E4)', IOSTAT=IOS) VF(1)
      PRINT *, '=86', IOS
      PRINT *, '#87'
      VF(1) = 1.0
      WRITE (6, '(1X,E5.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=87', IOS
      PRINT *, '#88'
      VF(1) = 1.0
      WRITE (6, '(1X,E6.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=88', IOS
      PRINT *, '#89'
      VF(1) = 1.0
      WRITE (6, '(1X,E7.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=89', IOS
      PRINT *, '#90'
      VF(1) = 1.0
      WRITE (6, '(1X,E8.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=90', IOS
      PRINT *, '#91'
      VF(1) = 0.0363
      WRITE (6, '(1X,D14.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=91', IOS
      PRINT *, '#92'
      VF(1) = 1.2
      WRITE (6, '(1X,D9.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=92', IOS
      PRINT *, '#93'
      VF(1) = 1.0E-38
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=93', IOS
      PRINT *, '#94'
      VF(1) = 1.0E38
      WRITE (6, '(1X,E12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=94', IOS
      PRINT *, '#95'
      VF(1) = 1.0E9
      WRITE (6, '(1X,E9.2E1)', IOSTAT=IOS) VF(1)
      PRINT *, '=95', IOS
      PRINT *, '#96'
      VF(1) = 1.0E9
      WRITE (6, '(1X,E9.2E2)', IOSTAT=IOS) VF(1)
      PRINT *, '=96', IOS
      PRINT *, '#97'
      VD(1) = 5413.87625793D0
      WRITE (6, '(1X,D23.12)', IOSTAT=IOS) VD(1)
      PRINT *, '=97', IOS
      PRINT *, '#98'
      VD(1) = 0.0363D0
      WRITE (6, '(1X,D14.3)', IOSTAT=IOS) VD(1)
      PRINT *, '=98', IOS
      PRINT *, '#99'
      VD(1) = 1.0D30
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=99', IOS
      PRINT *, '#100'
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) 1
      PRINT *, '=100', IOS
      PRINT *, '#103'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=103', IOS
      PRINT *, '#104'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=104', IOS
      PRINT *, '#107'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=107', IOS
      PRINT *, '#108'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=108', IOS
      PRINT *, '#111'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VH(1)
      PRINT *, '=111', IOS
      PRINT *, '#112'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VH(1)
      PRINT *, '=112', IOS
      PRINT *, '#115'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VH(1)
      PRINT *, '=115', IOS
      PRINT *, '#116'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VH(1)
      PRINT *, '=116', IOS
      PRINT *, '#119'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=119', IOS
      PRINT *, '#120'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VH(1)
      PRINT *, '=120', IOS
      PRINT *, '#123'
      VH(1) = 1.0Q1000
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VH(1)
      PRINT *, '=123', IOS
      PRINT *, '#124'
      VH(1) = 1.0Q-1000
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VH(1)
      PRINT *, '=124', IOS
      PRINT *, '#125'
      VD(1) = 1.0D30
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=125', IOS
      PRINT *, '#127'
      VF(1) = -270.139
      WRITE (6, '(1X,1PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=127', IOS
      PRINT *, '#128'
      VF(1) = -270.139
      WRITE (6, '(1X,1PE12.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=128', IOS
      PRINT *, '#129'
      VF(1) = -270.139
      WRITE (6, '(1X,-1PE12.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=129', IOS
      PRINT *, '#130'
      VF(1) = 270.139
      WRITE (6, '(1X,2PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=130', IOS
      PRINT *, '#131'
      VF(1) = 270.139
      WRITE (6, '(1X,4PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=131', IOS
      PRINT *, '#132'
      VF(1) = 270.139
      WRITE (6, '(1X,5PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=132', IOS
      PRINT *, '#133'
      VF(1) = 270.139
      WRITE (6, '(1X,-2PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=133', IOS
      END

      SUBROUTINE S4
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#134'
      VF(1) = 270.139
      WRITE (6, '(1X,-3PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=134', IOS
      PRINT *, '#135'
      VF(1) = 270.139
      WRITE (6, '(1X,-3PE12.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=135', IOS
      PRINT *, '#136'
      VF(1) = 270.139
      WRITE (6, '(1X,-4PE12.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=136', IOS
      PRINT *, '#137'
      VF(1) = 270.139
      WRITE (6, '(1X,0PE12.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=137', IOS
      PRINT *, '#138'
      VF(1) = 25.0
      WRITE (6, '(1X,2PF8.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=138', IOS
      PRINT *, '#139'
      VF(1) = 25.0
      WRITE (6, '(1X,-2PF8.2)', IOSTAT=IOS) VF(1)
      PRINT *, '=139', IOS
      PRINT *, '#140'
      VF(1) = 5.0
      WRITE (6, '(1X,2PG13.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=140', IOS
      PRINT *, '#141'
      VF(1) = 5.0E8
      WRITE (6, '(1X,2PG13.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=141', IOS
      PRINT *, '#142'
      VF(1) = 0.0363
      WRITE (6, '(1X,1PD14.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=142', IOS
      PRINT *, '#143'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      WRITE (6, '(1X,F8.2,2PF8.2,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
      PRINT *, '=143', IOS
      PRINT *, '#144'
      VF(1) = 25.0
      VF(2) = 25.0
      WRITE (6, '(1X,2PF8.2,0PF8.2)', IOSTAT=IOS) VF(1), VF(2)
      PRINT *, '=144', IOS
      PRINT *, '#145'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      VF(4) = 25.0
      VF(5) = 25.0
      VF(6) = 25.0
      WRITE (6, '(1X,F8.2,2PF8.2,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
     &, VF(4), VF(5), VF(6)
      PRINT *, '=145', IOS
      PRINT *, '#146'
      VF(1) = 25.0
      VF(2) = 25.0
      VF(3) = 25.0
      WRITE (6, '(1X,F8.2,2P,F8.2)', IOSTAT=IOS) VF(1), VF(2), VF(3)
      PRINT *, '=146', IOS
      PRINT *, '#147'
      VF(2) = 25.0
      WRITE (6, '(1X,2P,I4,F8.2)', IOSTAT=IOS) 5, VF(2)
      PRINT *, '=147', IOS
      PRINT *, '#148'
      VF(1) = 0.01234567
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=148', IOS
      PRINT *, '#149'
      VF(1) = -0.12345678
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=149', IOS
      PRINT *, '#150'
      VF(1) = 1.23456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=150', IOS
      PRINT *, '#151'
      VF(1) = 12.3456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=151', IOS
      PRINT *, '#152'
      VF(1) = 123.456789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=152', IOS
      PRINT *, '#153'
      VF(1) = -1234.56789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=153', IOS
      PRINT *, '#154'
      VF(1) = 12345.6789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=154', IOS
      PRINT *, '#155'
      VF(1) = 123456.789
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=155', IOS
      PRINT *, '#156'
      VF(1) = -1234567.89
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=156', IOS
      PRINT *, '#157'
      VF(1) = 0.0
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=157', IOS
      PRINT *, '#158'
      VF(1) = 0.0
      WRITE (6, '(1X,G15.7E2)', IOSTAT=IOS) VF(1)
      PRINT *, '=158', IOS
      PRINT *, '#159'
      VF(1) = 0.0
      WRITE (6, '(1X,1PG15.7E2)', IOSTAT=IOS) VF(1)
      PRINT *, '=159', IOS
      PRINT *, '#160'
      VF(1) = 0.09999999
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) VF(1)
      PRINT *, '=160', IOS
      PRINT *, '#161'
      VF(1) = 999.6
      WRITE (6, '(1X,G13.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=161', IOS
      PRINT *, '#162'
      VF(1) = 0.09996
      WRITE (6, '(1X,G13.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=162', IOS
      PRINT *, '#163'
      VF(1) = 5.0
      WRITE (6, '(1X,G8.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=163', IOS
      PRINT *, '#164'
      VF(1) = 5.0
      WRITE (6, '(1X,G7.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=164', IOS
      PRINT *, '#165'
      VF(1) = 5.0
      WRITE (6, '(1X,G6.3)', IOSTAT=IOS) VF(1)
      PRINT *, '=165', IOS
      PRINT *, '#166'
      VF(1) = 5.0
      WRITE (6, '(1X,G13.0)', IOSTAT=IOS) VF(1)
      PRINT *, '=166', IOS
      PRINT *, '#167'
      VF(1) = 5.0
      WRITE (6, '(1X,G13.3E4)', IOSTAT=IOS) VF(1)
      PRINT *, '=167', IOS
      PRINT *, '#168'
      VF(1) = 5.0E8
      WRITE (6, '(1X,G13.3E4)', IOSTAT=IOS) VF(1)
      PRINT *, '=168', IOS
      PRINT *, '#169'
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) 5
      PRINT *, '=169', IOS
      PRINT *, '#170'
      WRITE (6, '(1X,G13.6)', IOSTAT=IOS) 'HI'
      PRINT *, '=170', IOS
      PRINT *, '#171'
      VD(1) = 5.0D0
      WRITE (6, '(1X,G25.15)', IOSTAT=IOS) VD(1)
      PRINT *, '=171', IOS
      PRINT *, '#172'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,SP,F8.2,SS,F8.2,S,F8.2)', IOSTAT=IOS) VF(1), VF(2),
     & VF(3)
      PRINT *, '=172', IOS
      PRINT *, '#173'
      WRITE (6, '(1X,SP,I5,SS,I5,S,I5)', IOSTAT=IOS) 1, 1, 1
      PRINT *, '=173', IOS
      END

      SUBROUTINE S5
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#174'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,SP,E12.3,G13.3,D12.3)', IOSTAT=IOS) VF(1), VF(2), V
     &F(3)
      PRINT *, '=174', IOS
      PRINT *, '#175'
      WRITE (6, '(1X,SP,O5,Z5)', IOSTAT=IOS) 1, 1
      PRINT *, '=175', IOS
      PRINT *, '#176'
      WRITE (6, '(1X,SP,I1)', IOSTAT=IOS) 1
      PRINT *, '=176', IOS
      PRINT *, '#177'
      VF(1) = 1.0
      WRITE (6, '(1X,SP,F3.1)', IOSTAT=IOS) VF(1)
      PRINT *, '=177', IOS
      PRINT *, '#178'
      WRITE (6, '(1X,SP,I5)', IOSTAT=IOS) 0
      PRINT *, '=178', IOS
      PRINT *, '#179'
      VLUI(1) = 1
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VLU(1)
      PRINT *, '=179', IOS
      PRINT *, '#180'
      VLUI(1) = 0
      WRITE (6, '(1X,L1)', IOSTAT=IOS) VLU(1)
      PRINT *, '=180', IOS
      PRINT *, '#181'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 1
      PRINT *, '=181', IOS
      PRINT *, '#182'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 2
      PRINT *, '=182', IOS
      PRINT *, '#183'
      VWUI(1) = 1
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VWU(1)
      PRINT *, '=183', IOS
      PRINT *, '#184'
      VF(1) = 3.25
      WRITE (6, '(1X,L5)', IOSTAT=IOS) VF(1)
      PRINT *, '=184', IOS
      PRINT *, '#185'
      WRITE (6, '(1X,L5)', IOSTAT=IOS) 'HI'
      PRINT *, '=185', IOS
      PRINT *, '#186'
      VLUI(1) = 1
      WRITE (6, '(1X,L0)', IOSTAT=IOS) VLU(1)
      PRINT *, '=186', IOS
      PRINT *, '#187'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'OHMS'
      PRINT *, '=187', IOS
      PRINT *, '#188'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'VOLTS'
      PRINT *, '=188', IOS
      PRINT *, '#189'
      WRITE (6, '(1X,A5)', IOSTAT=IOS) 'AMPERES'
      PRINT *, '=189', IOS
      PRINT *, '#190'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=190', IOS
      PRINT *, '#191'
      WRITE (6, '(1X,A3)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=191', IOS
      PRINT *, '#192'
      WRITE (6, '(1X,A8)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=192', IOS
      PRINT *, '#193'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 1094861636
      PRINT *, '=193', IOS
      PRINT *, '#194'
      WRITE (6, '(1X,A2)', IOSTAT=IOS) 1094861636
      PRINT *, '=194', IOS
      PRINT *, '#195'
      WRITE (6, '(1X,A6)', IOSTAT=IOS) 1094861636
      PRINT *, '=195', IOS
      PRINT *, '#196'
      VW(1) = 16706
      WRITE (6, '(1X,A)', IOSTAT=IOS) VW(1)
      PRINT *, '=196', IOS
      PRINT *, '#197'
      VB(1) = 65
      WRITE (6, '(1X,A)', IOSTAT=IOS) VB(1)
      PRINT *, '=197', IOS
      PRINT *, '#198'
      VF(1) = 1.0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VF(1)
      PRINT *, '=198', IOS
      PRINT *, '#199'
      VD(1) = 1.0D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      PRINT *, '=199', IOS
      PRINT *, '#200'
      VLUI(1) = 1094861636
      WRITE (6, '(1X,A)', IOSTAT=IOS) VLU(1)
      PRINT *, '=200', IOS
      PRINT *, '#201'
      WRITE (6, '(1X,I)', IOSTAT=IOS) 42
      PRINT *, '=201', IOS
      PRINT *, '#202'
      VW(1) = 42
      WRITE (6, '(1X,I)', IOSTAT=IOS) VW(1)
      PRINT *, '=202', IOS
      PRINT *, '#203'
      VB(1) = 42
      WRITE (6, '(1X,I)', IOSTAT=IOS) VB(1)
      PRINT *, '=203', IOS
      PRINT *, '#204'
      VLUI(1) = 1
      WRITE (6, '(1X,I)', IOSTAT=IOS) VLU(1)
      PRINT *, '=204', IOS
      PRINT *, '#205'
      VWUI(1) = 1
      WRITE (6, '(1X,I)', IOSTAT=IOS) VWU(1)
      PRINT *, '=205', IOS
      PRINT *, '#206'
      VF(1) = 3.25
      WRITE (6, '(1X,I)', IOSTAT=IOS) VF(1)
      PRINT *, '=206', IOS
      PRINT *, '#207'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VD(1)
      PRINT *, '=207', IOS
      PRINT *, '#209'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VH(1)
      PRINT *, '=209', IOS
      PRINT *, '#210'
      WRITE (6, '(1X,I)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=210', IOS
      PRINT *, '#211'
      WRITE (6, '(1X,O)', IOSTAT=IOS) 42
      PRINT *, '=211', IOS
      PRINT *, '#212'
      VW(1) = 42
      WRITE (6, '(1X,O)', IOSTAT=IOS) VW(1)
      PRINT *, '=212', IOS
      PRINT *, '#213'
      VB(1) = 42
      WRITE (6, '(1X,O)', IOSTAT=IOS) VB(1)
      PRINT *, '=213', IOS
      PRINT *, '#214'
      VLUI(1) = 1
      WRITE (6, '(1X,O)', IOSTAT=IOS) VLU(1)
      PRINT *, '=214', IOS
      END

      SUBROUTINE S6
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#215'
      VWUI(1) = 1
      WRITE (6, '(1X,O)', IOSTAT=IOS) VWU(1)
      PRINT *, '=215', IOS
      PRINT *, '#216'
      VF(1) = 3.25
      WRITE (6, '(1X,O)', IOSTAT=IOS) VF(1)
      PRINT *, '=216', IOS
      PRINT *, '#217'
      VD(1) = 3.25D0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VD(1)
      PRINT *, '=217', IOS
      PRINT *, '#219'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VH(1)
      PRINT *, '=219', IOS
      PRINT *, '#220'
      WRITE (6, '(1X,O)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=220', IOS
      PRINT *, '#221'
      WRITE (6, '(1X,Z)', IOSTAT=IOS) 42
      PRINT *, '=221', IOS
      PRINT *, '#222'
      VW(1) = 42
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VW(1)
      PRINT *, '=222', IOS
      PRINT *, '#223'
      VB(1) = 42
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VB(1)
      PRINT *, '=223', IOS
      PRINT *, '#224'
      VLUI(1) = 1
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VLU(1)
      PRINT *, '=224', IOS
      PRINT *, '#225'
      VWUI(1) = 1
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VWU(1)
      PRINT *, '=225', IOS
      PRINT *, '#226'
      VF(1) = 3.25
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VF(1)
      PRINT *, '=226', IOS
      PRINT *, '#227'
      VD(1) = 3.25D0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VD(1)
      PRINT *, '=227', IOS
      PRINT *, '#229'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VH(1)
      PRINT *, '=229', IOS
      PRINT *, '#230'
      WRITE (6, '(1X,Z)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=230', IOS
      PRINT *, '#231'
      WRITE (6, '(1X,L)', IOSTAT=IOS) 42
      PRINT *, '=231', IOS
      PRINT *, '#232'
      VW(1) = 42
      WRITE (6, '(1X,L)', IOSTAT=IOS) VW(1)
      PRINT *, '=232', IOS
      PRINT *, '#233'
      VB(1) = 42
      WRITE (6, '(1X,L)', IOSTAT=IOS) VB(1)
      PRINT *, '=233', IOS
      PRINT *, '#234'
      VLUI(1) = 1
      WRITE (6, '(1X,L)', IOSTAT=IOS) VLU(1)
      PRINT *, '=234', IOS
      PRINT *, '#235'
      VWUI(1) = 1
      WRITE (6, '(1X,L)', IOSTAT=IOS) VWU(1)
      PRINT *, '=235', IOS
      PRINT *, '#236'
      VF(1) = 3.25
      WRITE (6, '(1X,L)', IOSTAT=IOS) VF(1)
      PRINT *, '=236', IOS
      PRINT *, '#237'
      VD(1) = 3.25D0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VD(1)
      PRINT *, '=237', IOS
      PRINT *, '#239'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VH(1)
      PRINT *, '=239', IOS
      PRINT *, '#240'
      WRITE (6, '(1X,L)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=240', IOS
      PRINT *, '#241'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 42
      PRINT *, '=241', IOS
      PRINT *, '#242'
      VW(1) = 42
      WRITE (6, '(1X,A)', IOSTAT=IOS) VW(1)
      PRINT *, '=242', IOS
      PRINT *, '#243'
      VB(1) = 42
      WRITE (6, '(1X,A)', IOSTAT=IOS) VB(1)
      PRINT *, '=243', IOS
      PRINT *, '#244'
      VLUI(1) = 1
      WRITE (6, '(1X,A)', IOSTAT=IOS) VLU(1)
      PRINT *, '=244', IOS
      PRINT *, '#245'
      VWUI(1) = 1
      WRITE (6, '(1X,A)', IOSTAT=IOS) VWU(1)
      PRINT *, '=245', IOS
      PRINT *, '#246'
      VF(1) = 3.25
      WRITE (6, '(1X,A)', IOSTAT=IOS) VF(1)
      PRINT *, '=246', IOS
      PRINT *, '#247'
      VD(1) = 3.25D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      PRINT *, '=247', IOS
      PRINT *, '#249'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VH(1)
      PRINT *, '=249', IOS
      PRINT *, '#250'
      WRITE (6, '(1X,A)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=250', IOS
      PRINT *, '#251'
      WRITE (6, '(1X,F)', IOSTAT=IOS) 42
      PRINT *, '=251', IOS
      PRINT *, '#252'
      VW(1) = 42
      WRITE (6, '(1X,F)', IOSTAT=IOS) VW(1)
      PRINT *, '=252', IOS
      PRINT *, '#253'
      VB(1) = 42
      WRITE (6, '(1X,F)', IOSTAT=IOS) VB(1)
      PRINT *, '=253', IOS
      PRINT *, '#254'
      VLUI(1) = 1
      WRITE (6, '(1X,F)', IOSTAT=IOS) VLU(1)
      PRINT *, '=254', IOS
      PRINT *, '#255'
      VWUI(1) = 1
      WRITE (6, '(1X,F)', IOSTAT=IOS) VWU(1)
      PRINT *, '=255', IOS
      PRINT *, '#256'
      VF(1) = 3.25
      WRITE (6, '(1X,F)', IOSTAT=IOS) VF(1)
      PRINT *, '=256', IOS
      PRINT *, '#257'
      VD(1) = 3.25D0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VD(1)
      PRINT *, '=257', IOS
      PRINT *, '#259'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VH(1)
      PRINT *, '=259', IOS
      END

      SUBROUTINE S7
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#260'
      WRITE (6, '(1X,F)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=260', IOS
      PRINT *, '#261'
      WRITE (6, '(1X,E)', IOSTAT=IOS) 42
      PRINT *, '=261', IOS
      PRINT *, '#262'
      VW(1) = 42
      WRITE (6, '(1X,E)', IOSTAT=IOS) VW(1)
      PRINT *, '=262', IOS
      PRINT *, '#263'
      VB(1) = 42
      WRITE (6, '(1X,E)', IOSTAT=IOS) VB(1)
      PRINT *, '=263', IOS
      PRINT *, '#264'
      VLUI(1) = 1
      WRITE (6, '(1X,E)', IOSTAT=IOS) VLU(1)
      PRINT *, '=264', IOS
      PRINT *, '#265'
      VWUI(1) = 1
      WRITE (6, '(1X,E)', IOSTAT=IOS) VWU(1)
      PRINT *, '=265', IOS
      PRINT *, '#266'
      VF(1) = 3.25
      WRITE (6, '(1X,E)', IOSTAT=IOS) VF(1)
      PRINT *, '=266', IOS
      PRINT *, '#267'
      VD(1) = 3.25D0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VD(1)
      PRINT *, '=267', IOS
      PRINT *, '#269'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VH(1)
      PRINT *, '=269', IOS
      PRINT *, '#270'
      WRITE (6, '(1X,E)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=270', IOS
      PRINT *, '#271'
      WRITE (6, '(1X,G)', IOSTAT=IOS) 42
      PRINT *, '=271', IOS
      PRINT *, '#272'
      VW(1) = 42
      WRITE (6, '(1X,G)', IOSTAT=IOS) VW(1)
      PRINT *, '=272', IOS
      PRINT *, '#273'
      VB(1) = 42
      WRITE (6, '(1X,G)', IOSTAT=IOS) VB(1)
      PRINT *, '=273', IOS
      PRINT *, '#274'
      VLUI(1) = 1
      WRITE (6, '(1X,G)', IOSTAT=IOS) VLU(1)
      PRINT *, '=274', IOS
      PRINT *, '#275'
      VWUI(1) = 1
      WRITE (6, '(1X,G)', IOSTAT=IOS) VWU(1)
      PRINT *, '=275', IOS
      PRINT *, '#276'
      VF(1) = 3.25
      WRITE (6, '(1X,G)', IOSTAT=IOS) VF(1)
      PRINT *, '=276', IOS
      PRINT *, '#277'
      VD(1) = 3.25D0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VD(1)
      PRINT *, '=277', IOS
      PRINT *, '#279'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VH(1)
      PRINT *, '=279', IOS
      PRINT *, '#280'
      WRITE (6, '(1X,G)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=280', IOS
      PRINT *, '#281'
      WRITE (6, '(1X,D)', IOSTAT=IOS) 42
      PRINT *, '=281', IOS
      PRINT *, '#282'
      VW(1) = 42
      WRITE (6, '(1X,D)', IOSTAT=IOS) VW(1)
      PRINT *, '=282', IOS
      PRINT *, '#283'
      VB(1) = 42
      WRITE (6, '(1X,D)', IOSTAT=IOS) VB(1)
      PRINT *, '=283', IOS
      PRINT *, '#284'
      VLUI(1) = 1
      WRITE (6, '(1X,D)', IOSTAT=IOS) VLU(1)
      PRINT *, '=284', IOS
      PRINT *, '#285'
      VWUI(1) = 1
      WRITE (6, '(1X,D)', IOSTAT=IOS) VWU(1)
      PRINT *, '=285', IOS
      PRINT *, '#286'
      VF(1) = 3.25
      WRITE (6, '(1X,D)', IOSTAT=IOS) VF(1)
      PRINT *, '=286', IOS
      PRINT *, '#287'
      VD(1) = 3.25D0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VD(1)
      PRINT *, '=287', IOS
      PRINT *, '#289'
      VH(1) = 3.25Q0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VH(1)
      PRINT *, '=289', IOS
      PRINT *, '#290'
      WRITE (6, '(1X,D)', IOSTAT=IOS) 'HELLO'
      PRINT *, '=290', IOS
      PRINT *, '#291'
      VFC(1) = (2.3547188,3.456732)
      WRITE (6, '(1X,2F8.5)', IOSTAT=IOS) VFC(1)
      PRINT *, '=291', IOS
      PRINT *, '#292'
      VFC(1) = (47587.222,56.123)
      WRITE (6, '(1X,E9.2,'', '',E5.3)', IOSTAT=IOS) VFC(1)
      PRINT *, '=292', IOS
      PRINT *, '#293'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,F8.2)', IOSTAT=IOS) VFC(1)
      PRINT *, '=293', IOS
      PRINT *, '#294'
      VFC(1) = (1.0,2.0)
      VFC(2) = (3.0,4.0)
      WRITE (6, '(1X,F8.2,''|'',F8.2,''|'',F8.2,''|'',F8.2)', IOSTAT=IOS
     &) VFC(1), VFC(2)
      PRINT *, '=294', IOS
      PRINT *, '#295'
      VDC(1) = (1.5D0,-2.5D0)
      WRITE (6, '(1X,2F12.5)', IOSTAT=IOS) VDC(1)
      PRINT *, '=295', IOS
      PRINT *, '#297'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,I5,I5)', IOSTAT=IOS) VFC(1)
      PRINT *, '=297', IOS
      PRINT *, '#298'
      VFC(1) = (1.0,2.0)
      WRITE (6, '(1X,F)', IOSTAT=IOS) VFC(1)
      PRINT *, '=298', IOS
      PRINT *, '#299'
      VDC(1) = (1.0D0,2.0D0)
      WRITE (6, '(1X,D)', IOSTAT=IOS) VDC(1)
      PRINT *, '=299', IOS
      PRINT *, '#300'
      WRITE (6, '(''A'',T5,''B'',TL2,''C'',TR2,''D'',3X,''E'',5X)', IOST
     &AT=IOS)
      PRINT *, '=300', IOS
      PRINT *, '#301'
      WRITE (6, '(T10,''X'',T1,'' Y'')', IOSTAT=IOS)
      PRINT *, '=301', IOS
      PRINT *, '#302'
      WRITE (6, '(1X,''K='',I6,5X)', IOSTAT=IOS) 5
      PRINT *, '=302', IOS
      PRINT *, '#303'
      WRITE (6, '(3HABC,2H12)', IOSTAT=IOS)
      PRINT *, '=303', IOS
      END

      SUBROUTINE S8
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#304'
      WRITE (6, '('' A''''B'')', IOSTAT=IOS)
      PRINT *, '=304', IOS
      PRINT *, '#305'
      WRITE (6, '(3H''B'')', IOSTAT=IOS)
      PRINT *, '=305', IOS
      PRINT *, '#306'
      WRITE (6, '(TL5,'' A'')', IOSTAT=IOS)
      PRINT *, '=306', IOS
      PRINT *, '#307'
      WRITE (6, '('' ABC'',TL100,''D'')', IOSTAT=IOS)
      PRINT *, '=307', IOS
      PRINT *, '#308'
      WRITE (6, '(T1,''A'',T1,''B'')', IOSTAT=IOS)
      PRINT *, '=308', IOS
      PRINT *, '#309'
      WRITE (6, '(T200,''X'')', IOSTAT=IOS)
      PRINT *, '=309', IOS
      PRINT *, '#310'
      WRITE (6, '(1X,''ABC'',T2,''D'')', IOSTAT=IOS)
      PRINT *, '=310', IOS
      PRINT *, '#311'
      WRITE (6, '(1X,''A'',0X,''B'')', IOSTAT=IOS)
      PRINT *, '=311', IOS
      PRINT *, '#312'
      WRITE (6, '('' '',TR0,''A'')', IOSTAT=IOS)
      PRINT *, '=312', IOS
      PRINT *, '#313'
      WRITE (6, '('' '',T0,''A'')', IOSTAT=IOS)
      PRINT *, '=313', IOS
      PRINT *, '#314'
      WRITE (6, '(1X''ABC'')', IOSTAT=IOS)
      PRINT *, '=314', IOS
      PRINT *, '#315'
      WRITE (6, '(''A''1X)', IOSTAT=IOS)
      PRINT *, '=315', IOS
      PRINT *, '#316'
      WRITE (6, '(''A''1X''B'')', IOSTAT=IOS)
      PRINT *, '=316', IOS
      PRINT *, '#317'
      WRITE (6, '('' A''/'' B'')', IOSTAT=IOS)
      PRINT *, '=317', IOS
      PRINT *, '#318'
      WRITE (6, '(1X,I3''A'')', IOSTAT=IOS) 1
      PRINT *, '=318', IOS
      PRINT *, '#319'
      WRITE (6, '(1X''A''I3)', IOSTAT=IOS) 1
      PRINT *, '=319', IOS
      PRINT *, '#320'
      WRITE (6, '(1X,2''A'')', IOSTAT=IOS)
      PRINT *, '=320', IOS
      PRINT *, '#321'
      WRITE (6, '(1X,3HAB)', IOSTAT=IOS)
      PRINT *, '=321', IOS
      PRINT *, '#322'
      WRITE (6, '(1X,'''')', IOSTAT=IOS)
      PRINT *, '=322', IOS
      PRINT *, '#323'
      WRITE (6, '(1X,I3,,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=323', IOS
      PRINT *, '#324'
      WRITE (6, '(1X I3 I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=324', IOS
      PRINT *, '#325'
      WRITE (6, '(1XI3I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=325', IOS
      PRINT *, '#326'
      WRITE (6, '( 1 X , I 3 )', IOSTAT=IOS) 1, 2
      PRINT *, '=326', IOS
      PRINT *, '#327'
      WRITE (6, '(1x,i3,i3)', IOSTAT=IOS) 1, 2
      PRINT *, '=327', IOS
      PRINT *, '#328'
      WRITE (6, '(1X,0I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=328', IOS
      PRINT *, '#329'
      WRITE (6, '(1X,I0)', IOSTAT=IOS) 1, 2
      PRINT *, '=329', IOS
      PRINT *, '#330'
      WRITE (6, '(1X,0X,''A'')', IOSTAT=IOS) 1, 2
      PRINT *, '=330', IOS
      PRINT *, '#331'
      WRITE (6, '(1X,J3)', IOSTAT=IOS) 1, 2
      PRINT *, '=331', IOS
      PRINT *, '#332'
      WRITE (6, '(1X,I3', IOSTAT=IOS) 1, 2
      PRINT *, '=332', IOS
      PRINT *, '#333'
      WRITE (6, '1X,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=333', IOS
      PRINT *, '#334'
      WRITE (6, '(1X,I3))', IOSTAT=IOS) 1, 2
      PRINT *, '=334', IOS
      PRINT *, '#335'
      WRITE (6, '(1X,I3)XYZ', IOSTAT=IOS) 1, 2
      PRINT *, '=335', IOS
      PRINT *, '#336'
      WRITE (6, '  (1X,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=336', IOS
      PRINT *, '#337'
      WRITE (6, '(1X,2)', IOSTAT=IOS) 1, 2
      PRINT *, '=337', IOS
      PRINT *, '#338'
      WRITE (6, '(1X,2X3)', IOSTAT=IOS) 1, 2
      PRINT *, '=338', IOS
      PRINT *, '#339'
      WRITE (6, '(1X,+3I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=339', IOS
      PRINT *, '#340'
      WRITE (6, '(1X,-1I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=340', IOS
      PRINT *, '#341'
      WRITE (6, '(1X,I-3)', IOSTAT=IOS) 1, 2
      PRINT *, '=341', IOS
      PRINT *, '#342'
      WRITE (6, '(1X,I3.-1)', IOSTAT=IOS) 1, 2
      PRINT *, '=342', IOS
      PRINT *, '#343'
      WRITE (6, '(1X,F8)', IOSTAT=IOS) 1, 2
      PRINT *, '=343', IOS
      END

      SUBROUTINE S9
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#344'
      WRITE (6, '(1X,F.2)', IOSTAT=IOS) 1, 2
      PRINT *, '=344', IOS
      PRINT *, '#345'
      WRITE (6, '(1X,F8.)', IOSTAT=IOS) 1, 2
      PRINT *, '=345', IOS
      PRINT *, '#346'
      WRITE (6, '(1X,E8.2E)', IOSTAT=IOS) 1, 2
      PRINT *, '=346', IOS
      PRINT *, '#347'
      WRITE (6, '(1X,G8.2E0)', IOSTAT=IOS) 1, 2
      PRINT *, '=347', IOS
      PRINT *, '#348'
      WRITE (6, '(1X,I3.2.1)', IOSTAT=IOS) 1, 2
      PRINT *, '=348', IOS
      PRINT *, '#349'
      WRITE (6, '(1X,1P)', IOSTAT=IOS) 1, 2
      PRINT *, '=349', IOS
      PRINT *, '#350'
      WRITE (6, '(1X,P)', IOSTAT=IOS) 1, 2
      PRINT *, '=350', IOS
      PRINT *, '#351'
      WRITE (6, '(1X,P,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=351', IOS
      PRINT *, '#352'
      WRITE (6, '(1X,2P3I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=352', IOS
      PRINT *, '#353'
      WRITE (6, '(1X,2PI3)', IOSTAT=IOS) 1, 2
      PRINT *, '=353', IOS
      PRINT *, '#354'
      WRITE (6, '(1X,+2PF8.2)', IOSTAT=IOS) 1, 2
      PRINT *, '=354', IOS
      PRINT *, '#355'
      WRITE (6, '(1X,I32768)', IOSTAT=IOS) 1, 2
      PRINT *, '=355', IOS
      PRINT *, '#356'
      WRITE (6, '(1X,32768I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=356', IOS
      PRINT *, '#357'
      WRITE (6, '(1X,I32767)', IOSTAT=IOS) 1, 2
      PRINT *, '=357', IOS
      PRINT *, '#358'
      WRITE (6, '(1X,F8.256)', IOSTAT=IOS) 1, 2
      PRINT *, '=358', IOS
      PRINT *, '#359'
      WRITE (6, '(1X,F8.255)', IOSTAT=IOS) 1, 2
      PRINT *, '=359', IOS
      PRINT *, '#360'
      WRITE (6, '(1X,E12.4E256)', IOSTAT=IOS) 1, 2
      PRINT *, '=360', IOS
      PRINT *, '#361'
      WRITE (6, '(1X,128PF8.2)', IOSTAT=IOS) 1, 2
      PRINT *, '=361', IOS
      PRINT *, '#362'
      WRITE (6, '(1X,-129PF8.2)', IOSTAT=IOS) 1, 2
      PRINT *, '=362', IOS
      PRINT *, '#363'
      WRITE (6, '(1X,<3>I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=363', IOS
      PRINT *, '#364'
      WRITE (6, '(1X,I<3>)', IOSTAT=IOS) 1, 2
      PRINT *, '=364', IOS
      PRINT *, '#365'
      WRITE (6, '(1X,''A)', IOSTAT=IOS) 1, 2
      PRINT *, '=365', IOS
      PRINT *, '#366'
      WRITE (6, '(1X,3HAB', IOSTAT=IOS) 1, 2
      PRINT *, '=366', IOS
      PRINT *, '#367'
      WRITE (6, '(1X,0HA)', IOSTAT=IOS) 1, 2
      PRINT *, '=367', IOS
      PRINT *, '#368'
      WRITE (6, '(1X,H)', IOSTAT=IOS) 1, 2
      PRINT *, '=368', IOS
      PRINT *, '#369'
      WRITE (6, '(1X,T)', IOSTAT=IOS) 1, 2
      PRINT *, '=369', IOS
      PRINT *, '#370'
      WRITE (6, '(1X,TL)', IOSTAT=IOS) 1, 2
      PRINT *, '=370', IOS
      PRINT *, '#371'
      WRITE (6, '(1X,TR)', IOSTAT=IOS) 1, 2
      PRINT *, '=371', IOS
      PRINT *, '#372'
      WRITE (6, '(1X,X)', IOSTAT=IOS) 1, 2
      PRINT *, '=372', IOS
      PRINT *, '#373'
      WRITE (6, '(1X,3$)', IOSTAT=IOS) 1, 2
      PRINT *, '=373', IOS
      PRINT *, '#374'
      WRITE (6, '(1X,3:)', IOSTAT=IOS) 1, 2
      PRINT *, '=374', IOS
      PRINT *, '#375'
      WRITE (6, '(1X,3/)', IOSTAT=IOS) 1, 2
      PRINT *, '=375', IOS
      PRINT *, '#376'
      WRITE (6, '(1X,BN3)', IOSTAT=IOS) 1, 2
      PRINT *, '=376', IOS
      PRINT *, '#377'
      WRITE (6, '(1X,3BN)', IOSTAT=IOS) 1, 2
      PRINT *, '=377', IOS
      PRINT *, '#378'
      WRITE (6, '(1X,Q3)', IOSTAT=IOS) 1, 2
      PRINT *, '=378', IOS
      PRINT *, '#379'
      WRITE (6, '(1X,3Q)', IOSTAT=IOS) 1, 2
      PRINT *, '=379', IOS
      PRINT *, '#380'
      WRITE (6, '(1X,SP3)', IOSTAT=IOS) 1, 2
      PRINT *, '=380', IOS
      PRINT *, '#381'
      WRITE (6, '(1X,3SP)', IOSTAT=IOS) 1, 2
      PRINT *, '=381', IOS
      PRINT *, '#382'
      WRITE (6, '(1X,3T5)', IOSTAT=IOS) 1, 2
      PRINT *, '=382', IOS
      PRINT *, '#383'
      WRITE (6, '(1X,I3:I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=383', IOS
      END

      SUBROUTINE S10
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#384'
      WRITE (6, '(1X,I3/I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=384', IOS
      PRINT *, '#385'
      WRITE (6, '(1X,I3$)', IOSTAT=IOS) 1, 2
      PRINT *, '=385', IOS
      PRINT *, '#386'
      WRITE (6, '(1X,/I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=386', IOS
      PRINT *, '#387'
      WRITE (6, '(1X,I3/,/I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=387', IOS
      PRINT *, '#388'
      WRITE (6, '(1X,:I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=388', IOS
      PRINT *, '#389'
      WRITE (6, '(1X,$,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=389', IOS
      PRINT *, '#390'
      WRITE (6, '(1X,/,I3)', IOSTAT=IOS) 1, 2
      PRINT *, '=390', IOS
      PRINT *, '#392'
      WRITE (6, ' ', IOSTAT=IOS)
      PRINT *, '=392', IOS
      PRINT *, '#393'
      WRITE (6, '()', IOSTAT=IOS)
      PRINT *, '=393', IOS
      PRINT *, '#394'
      WRITE (6, '( )', IOSTAT=IOS)
      PRINT *, '=394', IOS
      PRINT *, '#395'
      WRITE (6, '(,)', IOSTAT=IOS)
      PRINT *, '=395', IOS
      PRINT *, '#396'
      WRITE (6, '(1X,)', IOSTAT=IOS)
      PRINT *, '=396', IOS
      PRINT *, '#397'
      WRITE (6, '(,1X)', IOSTAT=IOS)
      PRINT *, '=397', IOS
      PRINT *, '#398'
      WRITE (6, '(1X,2(I2,1X))', IOSTAT=IOS) 1, 2
      PRINT *, '=398', IOS
      PRINT *, '#399'
      WRITE (6, '(1X,2(I2,1X),I3)', IOSTAT=IOS) 1, 2, 3
      PRINT *, '=399', IOS
      PRINT *, '#400'
      WRITE (6, '(1X,2(2(I2,1X)))', IOSTAT=IOS) 1, 2, 3, 4
      PRINT *, '=400', IOS
      PRINT *, '#401'
      WRITE (6, '(1X,(I2))', IOSTAT=IOS) 1
      PRINT *, '=401', IOS
      PRINT *, '#402'
      WRITE (6, '(1X,0(I2))', IOSTAT=IOS) 1
      PRINT *, '=402', IOS
      PRINT *, '#403'
      WRITE (6, '(1X,())', IOSTAT=IOS) 1
      PRINT *, '=403', IOS
      PRINT *, '#404'
      WRITE (6, '(1X,(((I3))))', IOSTAT=IOS) 3
      PRINT *, '=404', IOS
      PRINT *, '#405'
      WRITE (6, '(1X,((((((((I3)))))))))', IOSTAT=IOS) 8
      PRINT *, '=405', IOS
      PRINT *, '#406'
      WRITE (6, '(1X,((((((((((((((((I3)))))))))))))))))', IOSTAT=IOS) 1
     &6
      PRINT *, '=406', IOS
      PRINT *, '#407'
      WRITE (6, '(1X,((((((((((((((((((((I3)))))))))))))))))))))', IOSTA
     &T=IOS) 20
      PRINT *, '=407', IOS
      PRINT *, '#408'
      WRITE (6, '(1X,(((((((((((((((((((((I3))))))))))))))))))))))', IOS
     &TAT=IOS) 21
      PRINT *, '=408', IOS
      PRINT *, '#409'
      WRITE (6, '(1X,((((((((((((((((((((((((((((((((I3)))))))))))))))))
     &))))))))))))))))', IOSTAT=IOS) 32
      PRINT *, '=409', IOS
      PRINT *, '#410'
      WRITE (6, '(1X,(((((((((((((((((((((((((((((((((((((((((((((((((((
     &(((((((((((((I3)))))))))))))))))))))))))))))))))))))))))))))))))))
     &))))))))))))))', IOSTAT=IOS) 64
      PRINT *, '=410', IOS
      PRINT *, '#411'
      WRITE (6, '(1X,I3)', IOSTAT=IOS) 1, 2, 3
      PRINT *, '=411', IOS
      PRINT *, '#412'
      WRITE (6, '(1X,I3,/,1X,I3)', IOSTAT=IOS) 1, 2, 3
      PRINT *, '=412', IOS
      PRINT *, '#413'
      WRITE (6, '(1X,I3,2(1X,I2,1X,I4))', IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 
     &7, 8
      PRINT *, '=413', IOS
      PRINT *, '#414'
      WRITE (6, '(1X,I3,(1X,I2),I4)', IOSTAT=IOS) 1, 2, 3, 4, 5, 6, 7, 8
      PRINT *, '=414', IOS
      PRINT *, '#415'
      WRITE (6, '(1X,I3,(1X,I2),(1X,I4))', IOSTAT=IOS) 1, 2, 3, 4, 5, 6
      PRINT *, '=415', IOS
      PRINT *, '#416'
      VF(1) = 1.0
      VF(2) = 1.0
      VF(3) = 1.0
      WRITE (6, '(1X,2PF8.2,(F8.2))', IOSTAT=IOS) VF(1), VF(2), VF(3)
      PRINT *, '=416', IOS
      PRINT *, '#417'
      WRITE (6, '(1X,SP,I3,(I3))', IOSTAT=IOS) 1, 2, 3
      PRINT *, '=417', IOS
      PRINT *, '#418'
      WRITE (6, '(//,'' A''//)', IOSTAT=IOS)
      PRINT *, '=418', IOS
      PRINT *, '#419'
      WRITE (6, '('' X'',/)', IOSTAT=IOS)
      PRINT *, '=419', IOS
      PRINT *, '#420'
      WRITE (6, '(/)', IOSTAT=IOS)
      PRINT *, '=420', IOS
      PRINT *, '#421'
      WRITE (6, '(//)', IOSTAT=IOS)
      PRINT *, '=421', IOS
      PRINT *, '#422'
      WRITE (6, '()', IOSTAT=IOS)
      PRINT *, '=422', IOS
      PRINT *, '#423'
      WRITE (6, '('' I='',I2,'' J='',I2)', IOSTAT=IOS) 3
      PRINT *, '=423', IOS
      PRINT *, '#424'
      WRITE (6, '('' K='',I2,:,'' L='',I2)', IOSTAT=IOS) 4
      PRINT *, '=424', IOS
      END

      SUBROUTINE S11
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#425'
      WRITE (6, '('' K='',I2,:)', IOSTAT=IOS) 4, 5
      PRINT *, '=425', IOS
      PRINT *, '#426'
      WRITE (6, '(:,'' A'')', IOSTAT=IOS)
      PRINT *, '=426', IOS
      PRINT *, '#427'
      WRITE (6, '('' A'',:,'' B'')', IOSTAT=IOS)
      PRINT *, '=427', IOS
      PRINT *, '#428'
      WRITE (6, '(1X,''A'')', IOSTAT=IOS) 1
      PRINT *, '=428', IOS
      PRINT *, '#429'
      WRITE (6, '(1X)', IOSTAT=IOS) 1
      PRINT *, '=429', IOS
      PRINT *, '#430'
      WRITE (6, '('''')', IOSTAT=IOS) 1
      PRINT *, '=430', IOS
      PRINT *, '#431'
      WRITE (6, '(1X,Q,I3)', IOSTAT=IOS) 9, 1
      PRINT *, '=431', IOS
      PRINT *, '#432'
      WRITE (6, '(1X,Q)', IOSTAT=IOS) 1
      PRINT *, '=432', IOS
      PRINT *, '#433'
      WRITE (6, '(1X,BN,I3,BZ)', IOSTAT=IOS) 1
      PRINT *, '=433', IOS
      PRINT *, '#434'
      WRITE (6, '('' A'',$)', IOSTAT=IOS)
      PRINT *, '=434', IOS
      PRINT *, '#435'
      WRITE (6, '('' B'')', IOSTAT=IOS)
      PRINT *, '=435', IOS
      PRINT *, '#436'
      WRITE (6, '(''+C'',$)', IOSTAT=IOS)
      PRINT *, '=436', IOS
      PRINT *, '#437'
      WRITE (6, '(''+D'')', IOSTAT=IOS)
      PRINT *, '=437', IOS
      PRINT *, '#438'
      WRITE (6, '(''$E'',$)', IOSTAT=IOS)
      PRINT *, '=438', IOS
      PRINT *, '#439'
      WRITE (6, '(''$F'')', IOSTAT=IOS)
      PRINT *, '=439', IOS
      PRINT *, '#440'
      WRITE (6, '(''0G'',$)', IOSTAT=IOS)
      PRINT *, '=440', IOS
      PRINT *, '#441'
      WRITE (6, '(''0H'')', IOSTAT=IOS)
      PRINT *, '=441', IOS
      PRINT *, '#442'
      WRITE (6, '(''1I'',$)', IOSTAT=IOS)
      PRINT *, '=442', IOS
      PRINT *, '#443'
      WRITE (6, '(''1J'')', IOSTAT=IOS)
      PRINT *, '=443', IOS
      PRINT *, '#444'
      WRITE (6, '(''XK'',$)', IOSTAT=IOS)
      PRINT *, '=444', IOS
      PRINT *, '#445'
      WRITE (6, '(''XL'')', IOSTAT=IOS)
      PRINT *, '=445', IOS
      PRINT *, '#446'
      WRITE (6, '(1X,$)', IOSTAT=IOS)
      PRINT *, '=446', IOS
      PRINT *, '#447'
      WRITE (6, '($)', IOSTAT=IOS)
      PRINT *, '=447', IOS
      PRINT *, '#448'
      WRITE (6, '($,'' M'')', IOSTAT=IOS)
      PRINT *, '=448', IOS
      PRINT *, '#449'
      WRITE (6, '('' N'',$,'' O'')', IOSTAT=IOS)
      PRINT *, '=449', IOS
      PRINT *, '#450'
      WRITE (6, '('' P'',$/'' Q'')', IOSTAT=IOS)
      PRINT *, '=450', IOS
      PRINT *, '#451'
      WRITE (6, '('' R''/$)', IOSTAT=IOS)
      PRINT *, '=451', IOS
      PRINT *, '#452'
      WRITE (6, '(1X,I3,$)', IOSTAT=IOS) 1, 2
      PRINT *, '=452', IOS
      PRINT *, '#453'
      WRITE (6, '(1X,T79,''X'')', IOSTAT=IOS)
      PRINT *, '=453', IOS
      PRINT *, '#454'
      WRITE (6, '(1X,T80,''X'')', IOSTAT=IOS)
      PRINT *, '=454', IOS
      PRINT *, '#455'
      WRITE (6, '(1X,T81,''X'')', IOSTAT=IOS)
      PRINT *, '=455', IOS
      PRINT *, '#456'
      WRITE (6, '(1X,T132,''X'')', IOSTAT=IOS)
      PRINT *, '=456', IOS
      PRINT *, '#457'
      WRITE (6, '(1X,T133,''X'')', IOSTAT=IOS)
      PRINT *, '=457', IOS
      PRINT *, '#458'
      WRITE (6, '(1X,T134,''X'')', IOSTAT=IOS)
      PRINT *, '=458', IOS
      PRINT *, '#459'
      WRITE (6, '(1X,T256,''X'')', IOSTAT=IOS)
      PRINT *, '=459', IOS
      PRINT *, '#460'
      WRITE (6, '(1X,T1000,''X'')', IOSTAT=IOS)
      PRINT *, '=460', IOS
      PRINT *, '#461'
      WRITE (6, '(1X,T32767,''X'')', IOSTAT=IOS)
      PRINT *, '=461', IOS
      PRINT *, '#462'
      WRITE (6, '(1X,T32768,''X'')', IOSTAT=IOS)
      PRINT *, '=462', IOS
      PRINT *, '#463'
      WRITE (6, '(1X,131A1)', IOSTAT=IOS) 'A'
      PRINT *, '=463', IOS
      PRINT *, '#464'
      WRITE (6, '(1X,132A1)', IOSTAT=IOS) 'A'
      PRINT *, '=464', IOS
      END

      SUBROUTINE S12
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#465'
      WRITE (6, '(1X,133A1)', IOSTAT=IOS) 'A'
      PRINT *, '=465', IOS
      PRINT *, '#466'
      WRITE (6, '(1X,134A1)', IOSTAT=IOS) 'A'
      PRINT *, '=466', IOS
      PRINT *, '#467'
      WRITE (6, '(1X,I5000)', IOSTAT=IOS) 1
      PRINT *, '=467', IOS
      END

      OPTIONS /G_FLOATING
      SUBROUTINE S13
      INTEGER IOS
      INTEGER*2 VW(4)
      BYTE VB(4)
      LOGICAL*4 VLU(4)
      INTEGER*4 VLUI(4)
      LOGICAL*2 VWU(4)
      INTEGER*2 VWUI(4)
      EQUIVALENCE (VLU, VLUI), (VWU, VWUI)
      REAL VF(4)
      DOUBLE PRECISION VD(4)
      REAL*16 VH(4)
      COMPLEX VFC(4)
      COMPLEX*16 VDC(4)
      PRINT *, '#67'
      VD(1) = 0.1D0
      WRITE (6, '(1X,F30.25)', IOSTAT=IOS) VD(1)
      PRINT *, '=67', IOS
      PRINT *, '#101'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=101', IOS
      PRINT *, '#102'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=102', IOS
      PRINT *, '#105'
      VD(1) = 1.0D100
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=105', IOS
      PRINT *, '#106'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=106', IOS
      PRINT *, '#109'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VD(1)
      PRINT *, '=109', IOS
      PRINT *, '#110'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E3)', IOSTAT=IOS) VD(1)
      PRINT *, '=110', IOS
      PRINT *, '#113'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VD(1)
      PRINT *, '=113', IOS
      PRINT *, '#114'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E2)', IOSTAT=IOS) VD(1)
      PRINT *, '=114', IOS
      PRINT *, '#117'
      VD(1) = 1.0D100
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=117', IOS
      PRINT *, '#118'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,G14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=118', IOS
      PRINT *, '#121'
      VD(1) = 1.0D100
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VD(1)
      PRINT *, '=121', IOS
      PRINT *, '#122'
      VD(1) = 1.0D-100
      WRITE (6, '(1X,E14.4E4)', IOSTAT=IOS) VD(1)
      PRINT *, '=122', IOS
      PRINT *, '#126'
      VD(1) = 1.0D30
      WRITE (6, '(1X,D14.4)', IOSTAT=IOS) VD(1)
      PRINT *, '=126', IOS
      PRINT *, '#208'
      VD(1) = 3.25D0
      WRITE (6, '(1X,I)', IOSTAT=IOS) VD(1)
      PRINT *, '=208', IOS
      PRINT *, '#218'
      VD(1) = 3.25D0
      WRITE (6, '(1X,O)', IOSTAT=IOS) VD(1)
      PRINT *, '=218', IOS
      PRINT *, '#228'
      VD(1) = 3.25D0
      WRITE (6, '(1X,Z)', IOSTAT=IOS) VD(1)
      PRINT *, '=228', IOS
      PRINT *, '#238'
      VD(1) = 3.25D0
      WRITE (6, '(1X,L)', IOSTAT=IOS) VD(1)
      PRINT *, '=238', IOS
      PRINT *, '#248'
      VD(1) = 3.25D0
      WRITE (6, '(1X,A)', IOSTAT=IOS) VD(1)
      PRINT *, '=248', IOS
      PRINT *, '#258'
      VD(1) = 3.25D0
      WRITE (6, '(1X,F)', IOSTAT=IOS) VD(1)
      PRINT *, '=258', IOS
      PRINT *, '#268'
      VD(1) = 3.25D0
      WRITE (6, '(1X,E)', IOSTAT=IOS) VD(1)
      PRINT *, '=268', IOS
      PRINT *, '#278'
      VD(1) = 3.25D0
      WRITE (6, '(1X,G)', IOSTAT=IOS) VD(1)
      PRINT *, '=278', IOS
      PRINT *, '#288'
      VD(1) = 3.25D0
      WRITE (6, '(1X,D)', IOSTAT=IOS) VD(1)
      PRINT *, '=288', IOS
      PRINT *, '#296'
      VDC(1) = (1.5D0,-2.5D0)
      WRITE (6, '(1X,2F12.5)', IOSTAT=IOS) VDC(1)
      PRINT *, '=296', IOS
      END
