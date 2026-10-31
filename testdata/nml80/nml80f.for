C     NML80F.FOR - written by testdata/nml80/gen.go: the NAMELIST probe's
C     cases as FORTRAN statements, for FTNRTL. TestNML80Oracle holds its
C     output to VMS's FORRTL's, vax/nml80-forrtl.txt.
      PROGRAM NML80F
      WRITE (6,'(A)') ' ##### Finding the group'
      WRITE (6,'(A)') ' #0 < $GRP I=7 $END>'
      CALL RM(0,1,'#0')
      WRITE (6,'(A)') ' #1 < &GRP I=7 &END>'
      CALL RM(1,1,'#1')
      WRITE (6,'(A)') ' #2 < $grp i=7 $end>'
      CALL RM(2,1,'#2')
      WRITE (6,'(A)') ' #3 <$GRP I=7 $END>'
      CALL RM(3,1,'#3')
      WRITE (6,'(A)') ' #4 <X$GRP I=7 $END>'
      CALL RM(4,1,'#4')
      WRITE (6,'(A)') ' #5 <  $GRP I=7 $END>'
      CALL RM(5,1,'#5')
      WRITE (6,'(A)') ' #6 <     $GRP I=7 $END>'
      CALL RM(6,1,'#6')
      WRITE (6,'(A)') ' #7 <^I$GRP I=7 $END>'
      CALL RM(7,1,'#7')
      WRITE (6,'(A)') ' #8 < X $GRP I=7 $END>'
      CALL RM(8,1,'#8')
      WRITE (6,'(A)') ' #9 < I=7 $GRP I=8 $END>'
      CALL RM(9,1,'#9')
      WRITE (6,'(A)') ' #10 < IGNORED/ $GRP I=7 $END>'
      CALL RM(10,1,'#10')
      WRITE (6,'(A)') ' #11 < $OTHER I=7 $END/ $GRP I=8 $END>'
      CALL RM(11,1,'#11')
      WRITE (6,'(A)') ' #12 < $GRPX I=7 $END/ $GRP I=8 $END>'
      CALL RM(12,1,'#12')
      WRITE (6,'(A)') ' #13 < $GR I=7 $END/ $GRP I=8 $END>'
      CALL RM(13,1,'#13')
      WRITE (6,'(A)') ' #14 < $ GRP I=7 $END/ $GRP I=8 $END>'
      CALL RM(14,1,'#14')
      WRITE (6,'(A)') ' #15 < $OTHER $GRP I=7 $END/ $GRP I=8 $END>'
      CALL RM(15,1,'#15')
      WRITE (6,'(A)') ' #16 < $GRP,I=7 $END>'
      CALL RM(16,1,'#16')
      WRITE (6,'(A)') ' #17 < $GRP=7 $END>'
      CALL RM(17,1,'#17')
      WRITE (6,'(A)') ' #18 < $GRPI=7 $END/ $GRP I=8 $END>'
      CALL RM(18,1,'#18')
      WRITE (6,'(A)') ' #19 < $GRP>'
      CALL RM(19,1,'#19')
      WRITE (6,'(A)') ' #20 < $GRP/ I=7/ $END>'
      CALL RM(20,1,'#20')
      WRITE (6,'(A)') ' #21 < $GRP// I=7// $END>'
      CALL RM(21,1,'#21')
      WRITE (6,'(A)') ' #22 <>'
      CALL RM(22,1,'#22')
      WRITE (6,'(A)') ' #23 < $>'
      CALL RM(23,1,'#23')
      WRITE (6,'(A)') ' #24 < &>'
      CALL RM(24,1,'#24')
      WRITE (6,'(A)') ' ##### Ending the group'
      WRITE (6,'(A)') ' #25 < $GRP I=7 $>'
      CALL RM(25,1,'#25')
      WRITE (6,'(A)') ' #26 < $GRP I=7 &>'
      CALL RM(26,1,'#26')
      WRITE (6,'(A)') ' #27 < $GRP I=7 &END>'
      CALL RM(27,1,'#27')
      WRITE (6,'(A)') ' #28 < &GRP I=7 $END>'
      CALL RM(28,1,'#28')
      WRITE (6,'(A)') ' #29 < $GRP I=7$END>'
      CALL RM(29,1,'#29')
      WRITE (6,'(A)') ' #30 < $GRP I=7 $E>'
      CALL RM(30,1,'#30')
      WRITE (6,'(A)') ' #31 < $GRP I=7 $ENDX>'
      CALL RM(31,1,'#31')
      WRITE (6,'(A)') ' #32 < $GRP I=7 $OTHER>'
      CALL RM(32,1,'#32')
      WRITE (6,'(A)') ' #33 < $GRP I=7 $END R=1>'
      CALL RM(33,1,'#33')
      WRITE (6,'(A)') ' #34 < $GRP I=7 $END/ R=1>'
      CALL RM(34,1,'#34')
      WRITE (6,'(A)') ' #35 < $GRP $END>'
      CALL RM(35,1,'#35')
      WRITE (6,'(A)') ' #36 < $GRP $>'
      CALL RM(36,1,'#36')
      WRITE (6,'(A)') ' #37 < $GRP I=7 $ R=9>'
      CALL RM(37,1,'#37')
      WRITE (6,'(A)') ' #38 < $GRP I=7 />'
      CALL RM(38,1,'#38')
      WRITE (6,'(A)') ' #39 < $GRP / I=7 $END>'
      CALL RM(39,1,'#39')
      WRITE (6,'(A)') ' #40 < $GRP I=7, $END>'
      CALL RM(40,1,'#40')
      WRITE (6,'(A)') ' #41 < $GRP T=''A$B'' $END>'
      CALL RM(41,1,'#41')
      WRITE (6,'(A)') ' #42 < $GRP I=$END>'
      CALL RM(42,1,'#42')
      WRITE (6,'(A)') ' #43 < $GRP I= $END>'
      CALL RM(43,1,'#43')
      WRITE (6,'(A)') ' #44 < $GRP R=2.5$END>'
      CALL RM(44,1,'#44')
      WRITE (6,'(A)') ' #45 < $GRP A=1,2$END>'
      CALL RM(45,1,'#45')
      WRITE (6,'(A)') ' #46 < $GRP L=T$END>'
      CALL RM(46,1,'#46')
      WRITE (6,'(A)') ' #47 < $GRP C=(1,2)$END>'
      CALL RM(47,1,'#47')
      WRITE (6,'(A)') ' #48 < $GRP T=''X''$END>'
      CALL RM(48,1,'#48')
      WRITE (6,'(A)') ' #49 < $GRP I=7 $END/ $GRP R=1 $END>'
      CALL RM(49,2,'#49')
      WRITE (6,'(A)') ' #50 < $GRP I=7 $END $GRP R=1 $END/ $GRP J=9 $END
     &>'
      CALL RM(50,2,'#50')
      WRITE (6,'(A)') ' ##### Separators between assignments'
      WRITE (6,'(A)') ' #51 < $GRP I = 7 $END>'
      CALL RM(51,1,'#51')
      WRITE (6,'(A)') ' #52 < $GRP I =7 $END>'
      CALL RM(52,1,'#52')
      WRITE (6,'(A)') ' #53 < $GRP I= 7 $END>'
      CALL RM(53,1,'#53')
      WRITE (6,'(A)') ' #54 < $GRP^II^I=^I7^I$END>'
      CALL RM(54,1,'#54')
      WRITE (6,'(A)') ' #55 < $GRP I=7 R=1 $END>'
      CALL RM(55,1,'#55')
      WRITE (6,'(A)') ' #56 < $GRP I=7,R=1 $END>'
      CALL RM(56,1,'#56')
      WRITE (6,'(A)') ' #57 < $GRP I=7 , R=1 $END>'
      CALL RM(57,1,'#57')
      WRITE (6,'(A)') ' #58 < $GRP I=7,/ R=1 $END>'
      CALL RM(58,1,'#58')
      WRITE (6,'(A)') ' #59 < $GRP I=7/ ,R=1 $END>'
      CALL RM(59,1,'#59')
      WRITE (6,'(A)') ' #60 < $GRP I=7,,R=1 $END>'
      CALL RM(60,1,'#60')
      WRITE (6,'(A)') ' #61 < $GRP ,I=7 $END>'
      CALL RM(61,1,'#61')
      WRITE (6,'(A)') ' #62 < $GRP I=7 ,, $END>'
      CALL RM(62,1,'#62')
      WRITE (6,'(A)') ' #63 < $GRP I=/ 7 $END>'
      CALL RM(63,1,'#63')
      WRITE (6,'(A)') ' #64 < $GRP I/ =7 $END>'
      CALL RM(64,1,'#64')
      WRITE (6,'(A)') ' #65 < $GRP I=7 I=8 $END>'
      CALL RM(65,1,'#65')
      WRITE (6,'(A)') ' #66 < $GRP A(1)=7 A(1)=8 $END>'
      CALL RM(66,1,'#66')
      WRITE (6,'(A)') ' ##### Scalars and how many values'
      WRITE (6,'(A)') ' #67 < $GRP I=7,8 $END>'
      CALL RM(67,1,'#67')
      WRITE (6,'(A)') ' #68 < $GRP I=7 8 $END>'
      CALL RM(68,1,'#68')
      WRITE (6,'(A)') ' #69 < $GRP I=2*7 $END>'
      CALL RM(69,1,'#69')
      WRITE (6,'(A)') ' #70 < $GRP I=1*7 $END>'
      CALL RM(70,1,'#70')
      WRITE (6,'(A)') ' #71 < $GRP I=0*7 $END>'
      CALL RM(71,1,'#71')
      WRITE (6,'(A)') ' #72 < $GRP I=1* $END>'
      CALL RM(72,1,'#72')
      WRITE (6,'(A)') ' #73 < $GRP I=2* $END>'
      CALL RM(73,1,'#73')
      WRITE (6,'(A)') ' #74 < $GRP I=, $END>'
      CALL RM(74,1,'#74')
      WRITE (6,'(A)') ' #75 < $GRP I=,, $END>'
      CALL RM(75,1,'#75')
      WRITE (6,'(A)') ' #76 < $GRP I=,8 $END>'
      CALL RM(76,1,'#76')
      WRITE (6,'(A)') ' #77 < $GRP I=,,R=1 $END>'
      CALL RM(77,1,'#77')
      WRITE (6,'(A)') ' #78 < $GRP I=, R=1 $END>'
      CALL RM(78,1,'#78')
      WRITE (6,'(A)') ' #79 < $GRP I=7, 8, R=1 $END>'
      CALL RM(79,1,'#79')
      WRITE (6,'(A)') ' #80 < $GRP R=1,2 $END>'
      CALL RM(80,1,'#80')
      WRITE (6,'(A)') ' #81 < $GRP T=''A'',''B'' $END>'
      CALL RM(81,1,'#81')
      WRITE (6,'(A)') ' #82 < $GRP L=T,F $END>'
      CALL RM(82,1,'#82')
      WRITE (6,'(A)') ' #83 < $GRP L=T F $END>'
      CALL RM(83,1,'#83')
      WRITE (6,'(A)') ' #84 < $GRP C=(1,2),(3,4) $END>'
      CALL RM(84,1,'#84')
      WRITE (6,'(A)') ' ##### Names'
      WRITE (6,'(A)') ' #85 < $GRP i=7 $END>'
      CALL RM(85,1,'#85')
      WRITE (6,'(A)') ' #86 < $GRP NOPE=1 $END>'
      CALL RM(86,1,'#86')
      WRITE (6,'(A)') ' #87 < $GRP NOPE $END>'
      CALL RM(87,1,'#87')
      WRITE (6,'(A)') ' #88 < $GRP LONGNAME1=7 $END>'
      CALL RM(88,1,'#88')
      WRITE (6,'(A)') ' #89 < $GRP LONGNAME=7 $END>'
      CALL RM(89,1,'#89')
      WRITE (6,'(A)') ' #90 < $GRP LONGNAME12=7 $END>'
      CALL RM(90,1,'#90')
      WRITE (6,'(A)') ' #91 < $GRP LONG NAME1=7 $END>'
      CALL RM(91,1,'#91')
      WRITE (6,'(A)') ' #92 < $GRP GRP=7 $END>'
      CALL RM(92,1,'#92')
      WRITE (6,'(A)') ' #93 < $GRP I 7 $END>'
      CALL RM(93,1,'#93')
      WRITE (6,'(A)') ' #94 < $GRP I $END>'
      CALL RM(94,1,'#94')
      WRITE (6,'(A)') ' #95 < $GRP =7 $END>'
      CALL RM(95,1,'#95')
      WRITE (6,'(A)') ' #96 < $GRP 7 $END>'
      CALL RM(96,1,'#96')
      WRITE (6,'(A)') ' #97 < $GRP 7=1 $END>'
      CALL RM(97,1,'#97')
      WRITE (6,'(A)') ' #98 < $GRP ''I''=7 $END>'
      CALL RM(98,1,'#98')
      WRITE (6,'(A)') ' #99 < $GRP I==7 $END>'
      CALL RM(99,1,'#99')
      WRITE (6,'(A)') ' #100 < $GRP I=7 8=9 $END>'
      CALL RM(100,1,'#100')
      WRITE (6,'(A)') ' #101 < $GRP I=7 R $END>'
      CALL RM(101,1,'#101')
      WRITE (6,'(A)') ' #102 < $GRP I=7 NOPE=1 $END>'
      CALL RM(102,1,'#102')
      WRITE (6,'(A)') ' #103 < $GRP I=7 NOPE $END>'
      CALL RM(103,1,'#103')
      WRITE (6,'(A)') ' #104 < $GRP A=1 2 I=3 $END>'
      CALL RM(104,1,'#104')
      WRITE (6,'(A)') ' #105 < $GRP A=1 2 NOPE=3 $END>'
      CALL RM(105,1,'#105')
      WRITE (6,'(A)') ' #106 < $GRP A=1 2 NOPE $END>'
      CALL RM(106,1,'#106')
      WRITE (6,'(A)') ' #107 < $GRP LA=T F I=3 $END>'
      CALL RM(107,1,'#107')
      WRITE (6,'(A)') ' #108 < $GRP LA=T I=3 $END>'
      CALL RM(108,1,'#108')
      WRITE (6,'(A)') ' #109 < $GRP LA=T L=F $END>'
      CALL RM(109,1,'#109')
      WRITE (6,'(A)') ' #110 < $GRP LA=T,L=F $END>'
      CALL RM(110,1,'#110')
      WRITE (6,'(A)') ' #111 < $GRP LA=F T=1 $END>'
      CALL RM(111,1,'#111')
      WRITE (6,'(A)') ' #112 < $GRP LA=T(1)=''X'' $END>'
      CALL RM(112,1,'#112')
      WRITE (6,'(A)') ' ##### Arrays: values'
      WRITE (6,'(A)') ' #113 < $GRP A=5,6,7 $END>'
      CALL RM(113,1,'#113')
      WRITE (6,'(A)') ' #114 < $GRP A=5 6 7 $END>'
      CALL RM(114,1,'#114')
      WRITE (6,'(A)') ' #115 < $GRP A=5 , 6 , 7 $END>'
      CALL RM(115,1,'#115')
      WRITE (6,'(A)') ' #116 < $GRP A=5,/ 6,/ 7 $END>'
      CALL RM(116,1,'#116')
      WRITE (6,'(A)') ' #117 < $GRP A=5,6 $END>'
      CALL RM(117,1,'#117')
      WRITE (6,'(A)') ' #118 < $GRP A=5,6,7,8 $END>'
      CALL RM(118,1,'#118')
      WRITE (6,'(A)') ' #119 < $GRP A=5,6,7,8,9 $END>'
      CALL RM(119,1,'#119')
      WRITE (6,'(A)') ' #120 < $GRP A=5,,7 $END>'
      CALL RM(120,1,'#120')
      WRITE (6,'(A)') ' #121 < $GRP A=,6 $END>'
      CALL RM(121,1,'#121')
      WRITE (6,'(A)') ' #122 < $GRP A=,,7 $END>'
      CALL RM(122,1,'#122')
      WRITE (6,'(A)') ' #123 < $GRP A=5,6, $END>'
      CALL RM(123,1,'#123')
      WRITE (6,'(A)') ' #124 < $GRP A=5,6,7, $END>'
      CALL RM(124,1,'#124')
      WRITE (6,'(A)') ' #125 < $GRP A=5,6,7,, $END>'
      CALL RM(125,1,'#125')
      WRITE (6,'(A)') ' #126 < $GRP A=,,,8 $END>'
      CALL RM(126,1,'#126')
      WRITE (6,'(A)') ' #127 < $GRP A=3*5 $END>'
      CALL RM(127,1,'#127')
      WRITE (6,'(A)') ' #128 < $GRP A=2*5 $END>'
      CALL RM(128,1,'#128')
      WRITE (6,'(A)') ' #129 < $GRP A=4*5 $END>'
      CALL RM(129,1,'#129')
      WRITE (6,'(A)') ' #130 < $GRP A=2*5,6 $END>'
      CALL RM(130,1,'#130')
      WRITE (6,'(A)') ' #131 < $GRP A=2*5,6,7 $END>'
      CALL RM(131,1,'#131')
      WRITE (6,'(A)') ' #132 < $GRP A=1*5,6 $END>'
      CALL RM(132,1,'#132')
      WRITE (6,'(A)') ' #133 < $GRP A=0*5,6 $END>'
      CALL RM(133,1,'#133')
      WRITE (6,'(A)') ' #134 < $GRP A=2*,6 $END>'
      CALL RM(134,1,'#134')
      WRITE (6,'(A)') ' #135 < $GRP A=2* 6 $END>'
      CALL RM(135,1,'#135')
      WRITE (6,'(A)') ' #136 < $GRP A=3* $END>'
      CALL RM(136,1,'#136')
      WRITE (6,'(A)') ' #137 < $GRP A=4* $END>'
      CALL RM(137,1,'#137')
      WRITE (6,'(A)') ' #138 < $GRP A=2 * 5 $END>'
      CALL RM(138,1,'#138')
      WRITE (6,'(A)') ' #139 < $GRP A=2* 5,6 $END>'
      CALL RM(139,1,'#139')
      WRITE (6,'(A)') ' #140 < $GRP A=5,6,7 R=1 $END>'
      CALL RM(140,1,'#140')
      WRITE (6,'(A)') ' #141 < $GRP A=5,6,7,R=1 $END>'
      CALL RM(141,1,'#141')
      WRITE (6,'(A)') ' #142 < $GRP A=5,6,R=1 $END>'
      CALL RM(142,1,'#142')
      WRITE (6,'(A)') ' #143 < $GRP A=5,6,,R=1 $END>'
      CALL RM(143,1,'#143')
      WRITE (6,'(A)') ' #144 < $GRP RA=1.5,2.5 $END>'
      CALL RM(144,1,'#144')
      WRITE (6,'(A)') ' #145 < $GRP RA=2*1.5 $END>'
      CALL RM(145,1,'#145')
      WRITE (6,'(A)') ' #146 < $GRP LA=T,T $END>'
      CALL RM(146,1,'#146')
      WRITE (6,'(A)') ' #147 < $GRP LA=2*F $END>'
      CALL RM(147,1,'#147')
      WRITE (6,'(A)') ' #148 < $GRP TA=''X'',''Y'',''Z'' $END>'
      CALL RM(148,1,'#148')
      WRITE (6,'(A)') ' #149 < $GRP TA=2*''QQ'' $END>'
      CALL RM(149,1,'#149')
      WRITE (6,'(A)') ' #150 < $GRP TA=''A'',''B'',''C'',''D'' $END>'
      CALL RM(150,1,'#150')
      WRITE (6,'(A)') ' #151 < $GRP CA=(5,6),(7,8) $END>'
      CALL RM(151,1,'#151')
      WRITE (6,'(A)') ' #152 < $GRP CA=2*(5,6) $END>'
      CALL RM(152,1,'#152')
      WRITE (6,'(A)') ' #153 < $GRP CA=(5,6) (7,8) $END>'
      CALL RM(153,1,'#153')
      WRITE (6,'(A)') ' #154 < $GRP CA=,(7,8) $END>'
      CALL RM(154,1,'#154')
      WRITE (6,'(A)') ' #155 < $GRP CA=1,2 $END>'
      CALL RM(155,1,'#155')
      WRITE (6,'(A)') ' #156 < $GRP B=5,6,7,8 $END>'
      CALL RM(156,1,'#156')
      WRITE (6,'(A)') ' #157 < $GRP B=5,6 $END>'
      CALL RM(157,1,'#157')
      WRITE (6,'(A)') ' #158 < $GRP B=4*5 $END>'
      CALL RM(158,1,'#158')
      WRITE (6,'(A)') ' #159 < $GRP B=5*5 $END>'
      CALL RM(159,1,'#159')
      WRITE (6,'(A)') ' #160 < $GRP AL=5,6,7 $END>'
      CALL RM(160,1,'#160')
      WRITE (6,'(A)') ' ##### Arrays: subscripts'
      WRITE (6,'(A)') ' #161 < $GRP A(2)=5 $END>'
      CALL RM(161,1,'#161')
      WRITE (6,'(A)') ' #162 < $GRP A(1)=5 $END>'
      CALL RM(162,1,'#162')
      WRITE (6,'(A)') ' #163 < $GRP A(3)=5 $END>'
      CALL RM(163,1,'#163')
      WRITE (6,'(A)') ' #164 < $GRP A(0)=5 $END>'
      CALL RM(164,1,'#164')
      WRITE (6,'(A)') ' #165 < $GRP A(4)=5 $END>'
      CALL RM(165,1,'#165')
      WRITE (6,'(A)') ' #166 < $GRP A(-1)=5 $END>'
      CALL RM(166,1,'#166')
      WRITE (6,'(A)') ' #167 < $GRP A(+2)=5 $END>'
      CALL RM(167,1,'#167')
      WRITE (6,'(A)') ' #168 < $GRP A( 2 )=5 $END>'
      CALL RM(168,1,'#168')
      WRITE (6,'(A)') ' #169 < $GRP A (2)=5 $END>'
      CALL RM(169,1,'#169')
      WRITE (6,'(A)') ' #170 < $GRP A(2) =5 $END>'
      CALL RM(170,1,'#170')
      WRITE (6,'(A)') ' #171 < $GRP A(2)= 5 $END>'
      CALL RM(171,1,'#171')
      WRITE (6,'(A)') ' #172 < $GRP A(2)=5,6 $END>'
      CALL RM(172,1,'#172')
      WRITE (6,'(A)') ' #173 < $GRP A(2)=5,6,7 $END>'
      CALL RM(173,1,'#173')
      WRITE (6,'(A)') ' #174 < $GRP A(2)=2*5 $END>'
      CALL RM(174,1,'#174')
      WRITE (6,'(A)') ' #175 < $GRP A(2)=3*5 $END>'
      CALL RM(175,1,'#175')
      WRITE (6,'(A)') ' #176 < $GRP A(2)=,6 $END>'
      CALL RM(176,1,'#176')
      WRITE (6,'(A)') ' #177 < $GRP A(2)=,,7 $END>'
      CALL RM(177,1,'#177')
      WRITE (6,'(A)') ' #178 < $GRP A(3)=5,6 $END>'
      CALL RM(178,1,'#178')
      WRITE (6,'(A)') ' #179 < $GRP A(1,1)=5 $END>'
      CALL RM(179,1,'#179')
      WRITE (6,'(A)') ' #180 < $GRP A()=5 $END>'
      CALL RM(180,1,'#180')
      WRITE (6,'(A)') ' #181 < $GRP A(2=5 $END>'
      CALL RM(181,1,'#181')
      WRITE (6,'(A)') ' #182 < $GRP A(2 $END>'
      CALL RM(182,1,'#182')
      WRITE (6,'(A)') ' #183 < $GRP A(I)=5 $END>'
      CALL RM(183,1,'#183')
      WRITE (6,'(A)') ' #184 < $GRP A(1.5)=5 $END>'
      CALL RM(184,1,'#184')
      WRITE (6,'(A)') ' #185 < $GRP A(1+1)=5 $END>'
      CALL RM(185,1,'#185')
      WRITE (6,'(A)') ' #186 < $GRP A(99999999999)=5 $END>'
      CALL RM(186,1,'#186')
      WRITE (6,'(A)') ' #187 < $GRP A(2)(1:1)=5 $END>'
      CALL RM(187,1,'#187')
      WRITE (6,'(A)') ' #188 < $GRP A(1:2)=5 $END>'
      CALL RM(188,1,'#188')
      WRITE (6,'(A)') ' #189 < $GRP A(2/ )=5 $END>'
      CALL RM(189,1,'#189')
      WRITE (6,'(A)') ' #190 < $GRP B(1,2)=5 $END>'
      CALL RM(190,1,'#190')
      WRITE (6,'(A)') ' #191 < $GRP B(2,1)=5 $END>'
      CALL RM(191,1,'#191')
      WRITE (6,'(A)') ' #192 < $GRP B(2,2)=5 $END>'
      CALL RM(192,1,'#192')
      WRITE (6,'(A)') ' #193 < $GRP B( 1 , 2 )=5 $END>'
      CALL RM(193,1,'#193')
      WRITE (6,'(A)') ' #194 < $GRP B(2)=5 $END>'
      CALL RM(194,1,'#194')
      WRITE (6,'(A)') ' #195 < $GRP B(3,1)=5 $END>'
      CALL RM(195,1,'#195')
      WRITE (6,'(A)') ' #196 < $GRP B(1,3)=5 $END>'
      CALL RM(196,1,'#196')
      WRITE (6,'(A)') ' #197 < $GRP B(0,1)=5 $END>'
      CALL RM(197,1,'#197')
      WRITE (6,'(A)') ' #198 < $GRP B(1,2,1)=5 $END>'
      CALL RM(198,1,'#198')
      WRITE (6,'(A)') ' #199 < $GRP B(2,1)=5,6,7 $END>'
      CALL RM(199,1,'#199')
      WRITE (6,'(A)') ' #200 < $GRP B(2,1)=5,6,7,8 $END>'
      CALL RM(200,1,'#200')
      WRITE (6,'(A)') ' #201 < $GRP B(1,)=5 $END>'
      CALL RM(201,1,'#201')
      WRITE (6,'(A)') ' #202 < $GRP B(,1)=5 $END>'
      CALL RM(202,1,'#202')
      WRITE (6,'(A)') ' #203 < $GRP AL(-1)=5 $END>'
      CALL RM(203,1,'#203')
      WRITE (6,'(A)') ' #204 < $GRP AL(0)=5 $END>'
      CALL RM(204,1,'#204')
      WRITE (6,'(A)') ' #205 < $GRP AL(0)=5,6 $END>'
      CALL RM(205,1,'#205')
      WRITE (6,'(A)') ' #206 < $GRP AL(0)=5,6,7 $END>'
      CALL RM(206,1,'#206')
      WRITE (6,'(A)') ' #207 < $GRP AL(1)=5 $END>'
      CALL RM(207,1,'#207')
      WRITE (6,'(A)') ' #208 < $GRP AL(-2)=5 $END>'
      CALL RM(208,1,'#208')
      WRITE (6,'(A)') ' #209 < $GRP AL(2)=5 $END>'
      CALL RM(209,1,'#209')
      WRITE (6,'(A)') ' #210 < $GRP I(1)=5 $END>'
      CALL RM(210,1,'#210')
      WRITE (6,'(A)') ' #211 < $GRP R(1)=5 $END>'
      CALL RM(211,1,'#211')
      WRITE (6,'(A)') ' #212 < $GRP CA(2)=(5,6) $END>'
      CALL RM(212,1,'#212')
      WRITE (6,'(A)') ' #213 < $GRP LA(2)=T $END>'
      CALL RM(213,1,'#213')
      WRITE (6,'(A)') ' #214 < $GRP RA(2)=1.5,2.5 $END>'
      CALL RM(214,1,'#214')
      WRITE (6,'(A)') ' ##### Character values and substrings'
      WRITE (6,'(A)') ' #215 < $GRP T=''ABC'' $END>'
      CALL RM(215,1,'#215')
      WRITE (6,'(A)') ' #216 < $GRP T='''' $END>'
      CALL RM(216,1,'#216')
      WRITE (6,'(A)') ' #217 < $GRP T='' '' $END>'
      CALL RM(217,1,'#217')
      WRITE (6,'(A)') ' #218 < $GRP T=''ABCDE'' $END>'
      CALL RM(218,1,'#218')
      WRITE (6,'(A)') ' #219 < $GRP T=''ABCDEFGH'' $END>'
      CALL RM(219,1,'#219')
      WRITE (6,'(A)') ' #220 < $GRP T=''A''''B'' $END>'
      CALL RM(220,1,'#220')
      WRITE (6,'(A)') ' #221 < $GRP T='''''''' $END>'
      CALL RM(221,1,'#221')
      WRITE (6,'(A)') ' #222 < $GRP T=''AB'' ''CD'' $END>'
      CALL RM(222,1,'#222')
      WRITE (6,'(A)') ' #223 < $GRP T=''AB''X $END>'
      CALL RM(223,1,'#223')
      WRITE (6,'(A)') ' #224 < $GRP T=''AB''R=1 $END>'
      CALL RM(224,1,'#224')
      WRITE (6,'(A)') ' #225 < $GRP T=''AB'',R=1 $END>'
      CALL RM(225,1,'#225')
      WRITE (6,'(A)') ' #226 < $GRP T=ABC $END>'
      CALL RM(226,1,'#226')
      WRITE (6,'(A)') ' #227 < $GRP T=7 $END>'
      CALL RM(227,1,'#227')
      WRITE (6,'(A)') ' #228 < $GRP T="ABC" $END>'
      CALL RM(228,1,'#228')
      WRITE (6,'(A)') ' #229 < $GRP T=.TRUE. $END>'
      CALL RM(229,1,'#229')
      WRITE (6,'(A)') ' #230 < $GRP T=(1,2) $END>'
      CALL RM(230,1,'#230')
      WRITE (6,'(A)') ' #231 < $GRP T=''AB/ CD'' $END>'
      CALL RM(231,1,'#231')
      WRITE (6,'(A)') ' #232 < $GRP T=''AB/CD'' $END>'
      CALL RM(232,1,'#232')
      WRITE (6,'(A)') ' #233 < $GRP T=''AB/ / CD'' $END>'
      CALL RM(233,1,'#233')
      WRITE (6,'(A)') ' #234 < $GRP T=''AB>'
      CALL RM(234,1,'#234')
      WRITE (6,'(A)') ' #235 < $GRP T(2:3)=''ZZ'' $END>'
      CALL RM(235,1,'#235')
      WRITE (6,'(A)') ' #236 < $GRP T(2:)=''ZZ'' $END>'
      CALL RM(236,1,'#236')
      WRITE (6,'(A)') ' #237 < $GRP T(:2)=''ZZ'' $END>'
      CALL RM(237,1,'#237')
      WRITE (6,'(A)') ' #238 < $GRP T(:)=''ZZ'' $END>'
      CALL RM(238,1,'#238')
      WRITE (6,'(A)') ' #239 < $GRP T(0:2)=''ZZ'' $END>'
      CALL RM(239,1,'#239')
      WRITE (6,'(A)') ' #240 < $GRP T(2:9)=''ZZ'' $END>'
      CALL RM(240,1,'#240')
      WRITE (6,'(A)') ' #241 < $GRP T(3:2)=''ZZ'' $END>'
      CALL RM(241,1,'#241')
      WRITE (6,'(A)') ' #242 < $GRP T(2:2)=''ZZ'' $END>'
      CALL RM(242,1,'#242')
      WRITE (6,'(A)') ' #243 < $GRP T( 2 : 3 )=''ZZ'' $END>'
      CALL RM(243,1,'#243')
      WRITE (6,'(A)') ' #244 < $GRP T (2:3)=''ZZ'' $END>'
      CALL RM(244,1,'#244')
      WRITE (6,'(A)') ' #245 < $GRP T(2:3) =''ZZ'' $END>'
      CALL RM(245,1,'#245')
      WRITE (6,'(A)') ' #246 < $GRP T(2)=''ZZ'' $END>'
      CALL RM(246,1,'#246')
      WRITE (6,'(A)') ' #247 < $GRP T(2:3)=''A'',''B'' $END>'
      CALL RM(247,1,'#247')
      WRITE (6,'(A)') ' #248 < $GRP T(2:3)=''VERYLONG'' $END>'
      CALL RM(248,1,'#248')
      WRITE (6,'(A)') ' #249 < $GRP TA(2)=''X'' $END>'
      CALL RM(249,1,'#249')
      WRITE (6,'(A)') ' #250 < $GRP TA(2)=''X'',''Y'' $END>'
      CALL RM(250,1,'#250')
      WRITE (6,'(A)') ' #251 < $GRP TA(2)=''X'',''Y'',''Z'' $END>'
      CALL RM(251,1,'#251')
      WRITE (6,'(A)') ' #252 < $GRP TA(2)(2:)=''Z'' $END>'
      CALL RM(252,1,'#252')
      WRITE (6,'(A)') ' #253 < $GRP TA(2)(2:3)=''Y'',''Z'' $END>'
      CALL RM(253,1,'#253')
      WRITE (6,'(A)') ' #254 < $GRP TA(2)(1:1)=''Y'',''Z'' $END>'
      CALL RM(254,1,'#254')
      WRITE (6,'(A)') ' #255 < $GRP TA(2) (2:)=''Z'' $END>'
      CALL RM(255,1,'#255')
      WRITE (6,'(A)') ' #256 < $GRP TA(4)=''X'' $END>'
      CALL RM(256,1,'#256')
      WRITE (6,'(A)') ' #257 < $GRP TA(2:3)=''X'' $END>'
      CALL RM(257,1,'#257')
      WRITE (6,'(A)') ' #258 < $GRP TA(2)(5:6)=''X'' $END>'
      CALL RM(258,1,'#258')
      WRITE (6,'(A)') ' #259 < $GRP I(1:2)=5 $END>'
      CALL RM(259,1,'#259')
      WRITE (6,'(A)') ' #260 < $GRP R(1:2)=5 $END>'
      CALL RM(260,1,'#260')
      WRITE (6,'(A)') ' ##### Numbers to numeric items'
      WRITE (6,'(A)') ' #261 < $GRP I=-3 $END>'
      CALL RM(261,1,'#261')
      WRITE (6,'(A)') ' #262 < $GRP I=+3 $END>'
      CALL RM(262,1,'#262')
      WRITE (6,'(A)') ' #263 < $GRP I=1.5 $END>'
      CALL RM(263,1,'#263')
      WRITE (6,'(A)') ' #264 < $GRP I=14.6 $END>'
      CALL RM(264,1,'#264')
      WRITE (6,'(A)') ' #265 < $GRP I=1E2 $END>'
      CALL RM(265,1,'#265')
      WRITE (6,'(A)') ' #266 < $GRP I=1.5E3 $END>'
      CALL RM(266,1,'#266')
      WRITE (6,'(A)') ' #267 < $GRP I=1D2 $END>'
      CALL RM(267,1,'#267')
      WRITE (6,'(A)') ' #268 < $GRP I=1Q2 $END>'
      CALL RM(268,1,'#268')
      WRITE (6,'(A)') ' #269 < $GRP I=.5 $END>'
      CALL RM(269,1,'#269')
      WRITE (6,'(A)') ' #270 < $GRP I=5. $END>'
      CALL RM(270,1,'#270')
      WRITE (6,'(A)') ' #271 < $GRP I=-.5 $END>'
      CALL RM(271,1,'#271')
      WRITE (6,'(A)') ' #272 < $GRP I=1E $END>'
      CALL RM(272,1,'#272')
      WRITE (6,'(A)') ' #273 < $GRP I=E2 $END>'
      CALL RM(273,1,'#273')
      WRITE (6,'(A)') ' #274 < $GRP I=1.2.3 $END>'
      CALL RM(274,1,'#274')
      WRITE (6,'(A)') ' #275 < $GRP I=2147483647 $END>'
      CALL RM(275,1,'#275')
      WRITE (6,'(A)') ' #276 < $GRP I=2147483648 $END>'
      CALL RM(276,1,'#276')
      WRITE (6,'(A)') ' #277 < $GRP I=-2147483648 $END>'
      CALL RM(277,1,'#277')
      WRITE (6,'(A)') ' #278 < $GRP I=T $END>'
      CALL RM(278,1,'#278')
      WRITE (6,'(A)') ' #279 < $GRP I=F $END>'
      CALL RM(279,1,'#279')
      WRITE (6,'(A)') ' #280 < $GRP I=.TRUE. $END>'
      CALL RM(280,1,'#280')
      WRITE (6,'(A)') ' #281 < $GRP I=.FALSE. $END>'
      CALL RM(281,1,'#281')
      WRITE (6,'(A)') ' #282 < $GRP I=''A'' $END>'
      CALL RM(282,1,'#282')
      WRITE (6,'(A)') ' #283 < $GRP I=(1,2) $END>'
      CALL RM(283,1,'#283')
      WRITE (6,'(A)') ' #284 < $GRP I=^X10 $END>'
      CALL RM(284,1,'#284')
      WRITE (6,'(A)') ' #285 < $GRP I="17 $END>'
      CALL RM(285,1,'#285')
      WRITE (6,'(A)') ' #286 < $GRP I=''10''X $END>'
      CALL RM(286,1,'#286')
      WRITE (6,'(A)') ' #287 < $GRP I=3HABC $END>'
      CALL RM(287,1,'#287')
      WRITE (6,'(A)') ' #288 < $GRP I=7ABC $END>'
      CALL RM(288,1,'#288')
      WRITE (6,'(A)') ' #289 < $GRP I=+ $END>'
      CALL RM(289,1,'#289')
      WRITE (6,'(A)') ' #290 < $GRP I=- $END>'
      CALL RM(290,1,'#290')
      WRITE (6,'(A)') ' #291 < $GRP I=. $END>'
      CALL RM(291,1,'#291')
      WRITE (6,'(A)') ' #292 < $GRP J=70000 $END>'
      CALL RM(292,1,'#292')
      WRITE (6,'(A)') ' #293 < $GRP J=-32768 $END>'
      CALL RM(293,1,'#293')
      WRITE (6,'(A)') ' #294 < $GRP J=32768 $END>'
      CALL RM(294,1,'#294')
      WRITE (6,'(A)') ' #295 < $GRP J=1.5 $END>'
      CALL RM(295,1,'#295')
      WRITE (6,'(A)') ' #296 < $GRP R=-3 $END>'
      CALL RM(296,1,'#296')
      WRITE (6,'(A)') ' #297 < $GRP R=1 $END>'
      CALL RM(297,1,'#297')
      WRITE (6,'(A)') ' #298 < $GRP R=1.5 $END>'
      CALL RM(298,1,'#298')
      WRITE (6,'(A)') ' #299 < $GRP R=1.5E2 $END>'
      CALL RM(299,1,'#299')
      WRITE (6,'(A)') ' #300 < $GRP R=1.5D2 $END>'
      CALL RM(300,1,'#300')
      WRITE (6,'(A)') ' #301 < $GRP R=1.5Q2 $END>'
      CALL RM(301,1,'#301')
      WRITE (6,'(A)') ' #302 < $GRP R=.5 $END>'
      CALL RM(302,1,'#302')
      WRITE (6,'(A)') ' #303 < $GRP R=1E40 $END>'
      CALL RM(303,1,'#303')
      WRITE (6,'(A)') ' #304 < $GRP R=1E-40 $END>'
      CALL RM(304,1,'#304')
      WRITE (6,'(A)') ' #305 < $GRP R=T $END>'
      CALL RM(305,1,'#305')
      WRITE (6,'(A)') ' #306 < $GRP R=.TRUE. $END>'
      CALL RM(306,1,'#306')
      WRITE (6,'(A)') ' #307 < $GRP R=''A'' $END>'
      CALL RM(307,1,'#307')
      WRITE (6,'(A)') ' #308 < $GRP R=(1,2) $END>'
      CALL RM(308,1,'#308')
      WRITE (6,'(A)') ' #309 < $GRP R=ABC $END>'
      CALL RM(309,1,'#309')
      WRITE (6,'(A)') ' #310 < $GRP R=1.2.3 $END>'
      CALL RM(310,1,'#310')
      WRITE (6,'(A)') ' #311 < $GRP D=1 $END>'
      CALL RM(311,1,'#311')
      WRITE (6,'(A)') ' #312 < $GRP D=0.1 $END>'
      CALL RM(312,1,'#312')
      WRITE (6,'(A)') ' #313 < $GRP D=1.5D0 $END>'
      CALL RM(313,1,'#313')
      WRITE (6,'(A)') ' #314 < $GRP D=0.1D0 $END>'
      CALL RM(314,1,'#314')
      WRITE (6,'(A)') ' #315 < $GRP D=1D40 $END>'
      CALL RM(315,1,'#315')
      WRITE (6,'(A)') ' #316 < $GRP D=1E40 $END>'
      CALL RM(316,1,'#316')
      WRITE (6,'(A)') ' #317 < $GRP D=1D-40 $END>'
      CALL RM(317,1,'#317')
      WRITE (6,'(A)') ' #318 < $GRP D=T $END>'
      CALL RM(318,1,'#318')
      WRITE (6,'(A)') ' #319 < $GRP Q=1 $END>'
      CALL RM(319,1,'#319')
      WRITE (6,'(A)') ' #320 < $GRP Q=0.1 $END>'
      CALL RM(320,1,'#320')
      WRITE (6,'(A)') ' #321 < $GRP Q=1.5Q0 $END>'
      CALL RM(321,1,'#321')
      WRITE (6,'(A)') ' #322 < $GRP Q=0.1Q0 $END>'
      CALL RM(322,1,'#322')
      WRITE (6,'(A)') ' #323 < $GRP Q=1Q4000 $END>'
      CALL RM(323,1,'#323')
      WRITE (6,'(A)') ' #324 < $GRP Q=1Q5000 $END>'
      CALL RM(324,1,'#324')
      WRITE (6,'(A)') ' ##### Logical values'
      WRITE (6,'(A)') ' #325 < $GRP L=T $END>'
      CALL RM(325,1,'#325')
      WRITE (6,'(A)') ' #326 < $GRP L=F $END>'
      CALL RM(326,1,'#326')
      WRITE (6,'(A)') ' #327 < $GRP L=.T $END>'
      CALL RM(327,1,'#327')
      WRITE (6,'(A)') ' #328 < $GRP L=.F $END>'
      CALL RM(328,1,'#328')
      WRITE (6,'(A)') ' #329 < $GRP L=.T. $END>'
      CALL RM(329,1,'#329')
      WRITE (6,'(A)') ' #330 < $GRP L=.F. $END>'
      CALL RM(330,1,'#330')
      WRITE (6,'(A)') ' #331 < $GRP L=.TRUE. $END>'
      CALL RM(331,1,'#331')
      WRITE (6,'(A)') ' #332 < $GRP L=.FALSE. $END>'
      CALL RM(332,1,'#332')
      WRITE (6,'(A)') ' #333 < $GRP L=TRUE $END>'
      CALL RM(333,1,'#333')
      WRITE (6,'(A)') ' #334 < $GRP L=FALSE $END>'
      CALL RM(334,1,'#334')
      WRITE (6,'(A)') ' #335 < $GRP L=t $END>'
      CALL RM(335,1,'#335')
      WRITE (6,'(A)') ' #336 < $GRP L=f $END>'
      CALL RM(336,1,'#336')
      WRITE (6,'(A)') ' #337 < $GRP L=.true. $END>'
      CALL RM(337,1,'#337')
      WRITE (6,'(A)') ' #338 < $GRP L=TX $END>'
      CALL RM(338,1,'#338')
      WRITE (6,'(A)') ' #339 < $GRP L=FX $END>'
      CALL RM(339,1,'#339')
      WRITE (6,'(A)') ' #340 < $GRP L=.X $END>'
      CALL RM(340,1,'#340')
      WRITE (6,'(A)') ' #341 < $GRP L=. $END>'
      CALL RM(341,1,'#341')
      WRITE (6,'(A)') ' #342 < $GRP L=1 $END>'
      CALL RM(342,1,'#342')
      WRITE (6,'(A)') ' #343 < $GRP L=0 $END>'
      CALL RM(343,1,'#343')
      WRITE (6,'(A)') ' #344 < $GRP L=-1 $END>'
      CALL RM(344,1,'#344')
      WRITE (6,'(A)') ' #345 < $GRP L=2 $END>'
      CALL RM(345,1,'#345')
      WRITE (6,'(A)') ' #346 < $GRP L=1.5 $END>'
      CALL RM(346,1,'#346')
      WRITE (6,'(A)') ' #347 < $GRP L=''T'' $END>'
      CALL RM(347,1,'#347')
      WRITE (6,'(A)') ' #348 < $GRP L=(1,2) $END>'
      CALL RM(348,1,'#348')
      WRITE (6,'(A)') ' #349 < $GRP L=X $END>'
      CALL RM(349,1,'#349')
      WRITE (6,'(A)') ' #350 < $GRP L=ABC $END>'
      CALL RM(350,1,'#350')
      WRITE (6,'(A)') ' #351 < $GRP L=TT $END>'
      CALL RM(351,1,'#351')
      WRITE (6,'(A)') ' #352 < $GRP M=T $END>'
      CALL RM(352,1,'#352')
      WRITE (6,'(A)') ' #353 < $GRP N=T $END>'
      CALL RM(353,1,'#353')
      WRITE (6,'(A)') ' #354 < $GRP M=F $END>'
      CALL RM(354,1,'#354')
      WRITE (6,'(A)') ' #355 < $GRP N=F $END>'
      CALL RM(355,1,'#355')
      WRITE (6,'(A)') ' #356 < $GRP M=1 $END>'
      CALL RM(356,1,'#356')
      WRITE (6,'(A)') ' #357 < $GRP N=1 $END>'
      CALL RM(357,1,'#357')
      WRITE (6,'(A)') ' #358 < $GRP M=300 $END>'
      CALL RM(358,1,'#358')
      WRITE (6,'(A)') ' #359 < $GRP N=300 $END>'
      CALL RM(359,1,'#359')
      WRITE (6,'(A)') ' ##### Complex values'
      WRITE (6,'(A)') ' #360 < $GRP C=(5,6) $END>'
      CALL RM(360,1,'#360')
      WRITE (6,'(A)') ' #361 < $GRP C=( 5 , 6 ) $END>'
      CALL RM(361,1,'#361')
      WRITE (6,'(A)') ' #362 < $GRP C=(5.5,-6.5) $END>'
      CALL RM(362,1,'#362')
      WRITE (6,'(A)') ' #363 < $GRP C=(1E2,1D2) $END>'
      CALL RM(363,1,'#363')
      WRITE (6,'(A)') ' #364 < $GRP C=5 $END>'
      CALL RM(364,1,'#364')
      WRITE (6,'(A)') ' #365 < $GRP C=5.5 $END>'
      CALL RM(365,1,'#365')
      WRITE (6,'(A)') ' #366 < $GRP C=(5) $END>'
      CALL RM(366,1,'#366')
      WRITE (6,'(A)') ' #367 < $GRP C=(5,) $END>'
      CALL RM(367,1,'#367')
      WRITE (6,'(A)') ' #368 < $GRP C=(,6) $END>'
      CALL RM(368,1,'#368')
      WRITE (6,'(A)') ' #369 < $GRP C=(,) $END>'
      CALL RM(369,1,'#369')
      WRITE (6,'(A)') ' #370 < $GRP C=() $END>'
      CALL RM(370,1,'#370')
      WRITE (6,'(A)') ' #371 < $GRP C=(5,6,7) $END>'
      CALL RM(371,1,'#371')
      WRITE (6,'(A)') ' #372 < $GRP C=(5 6) $END>'
      CALL RM(372,1,'#372')
      WRITE (6,'(A)') ' #373 < $GRP C=(5,6 $END>'
      CALL RM(373,1,'#373')
      WRITE (6,'(A)') ' #374 < $GRP C=(A,B) $END>'
      CALL RM(374,1,'#374')
      WRITE (6,'(A)') ' #375 < $GRP C=T $END>'
      CALL RM(375,1,'#375')
      WRITE (6,'(A)') ' #376 < $GRP C=''A'' $END>'
      CALL RM(376,1,'#376')
      WRITE (6,'(A)') ' #377 < $GRP C=, $END>'
      CALL RM(377,1,'#377')
      WRITE (6,'(A)') ' #378 < $GRP C=2*(5,6) $END>'
      CALL RM(378,1,'#378')
      WRITE (6,'(A)') ' #379 < $GRP C=1*(5,6) $END>'
      CALL RM(379,1,'#379')
      WRITE (6,'(A)') ' #380 < $GRP C=(5,6)X $END>'
      CALL RM(380,1,'#380')
      WRITE (6,'(A)') ' #381 < $GRP C=(5,/ 6) $END>'
      CALL RM(381,1,'#381')
      WRITE (6,'(A)') ' #382 < $GRP C=(/ 5,6) $END>'
      CALL RM(382,1,'#382')
      WRITE (6,'(A)') ' #383 < $GRP C=(5/ ,6) $END>'
      CALL RM(383,1,'#383')
      WRITE (6,'(A)') ' #384 < $GRP C=(5,6/ ) $END>'
      CALL RM(384,1,'#384')
      WRITE (6,'(A)') ' #385 < $GRP E=(5.5D0,-6.5D0) $END>'
      CALL RM(385,1,'#385')
      WRITE (6,'(A)') ' #386 < $GRP E=(0.1,0.2) $END>'
      CALL RM(386,1,'#386')
      WRITE (6,'(A)') ' #387 < $GRP E=0.1 $END>'
      CALL RM(387,1,'#387')
      WRITE (6,'(A)') ' ##### Prompting, at a file'
      WRITE (6,'(A)') ' #388 < ?>'
      CALL RM(388,1,'#388')
      WRITE (6,'(A)') ' #389 < =?>'
      CALL RM(389,1,'#389')
      WRITE (6,'(A)') ' #390 <?>'
      CALL RM(390,1,'#390')
      WRITE (6,'(A)') ' #391 <=?>'
      CALL RM(391,1,'#391')
      WRITE (6,'(A)') ' #392 < ? $GRP I=7 $END>'
      CALL RM(392,1,'#392')
      WRITE (6,'(A)') ' #393 < ?/ $GRP I=7 $END>'
      CALL RM(393,1,'#393')
      WRITE (6,'(A)') ' #394 < =?/ $GRP I=7 $END>'
      CALL RM(394,1,'#394')
      WRITE (6,'(A)') ' #395 < $GRP ? $END>'
      CALL RM(395,1,'#395')
      WRITE (6,'(A)') ' #396 < $GRP I=7 ? $END>'
      CALL RM(396,1,'#396')
      WRITE (6,'(A)') ' #397 < $GRP I=? $END>'
      CALL RM(397,1,'#397')
      WRITE (6,'(A)') ' #398 < $GRP =? $END>'
      CALL RM(398,1,'#398')
      WRITE (6,'(A)') ' ##### G_floating'
      WRITE (6,'(A)') ' #399 < $GRP G=1.5 $END>'
      CALL RG(399,1,'#399')
      WRITE (6,'(A)') ' #400 < $GRP G=0.1 $END>'
      CALL RG(400,1,'#400')
      WRITE (6,'(A)') ' #401 < $GRP G=0.1D0 $END>'
      CALL RG(401,1,'#401')
      WRITE (6,'(A)') ' #402 < $GRP G=1D300 $END>'
      CALL RG(402,1,'#402')
      WRITE (6,'(A)') ' #403 < $GRP H=(0.1,0.2) $END>'
      CALL RG(403,1,'#403')
      WRITE (6,'(A)') ' #404 < $GRP H=0.1 $END>'
      CALL RG(404,1,'#404')
      WRITE (6,'(A)') ' #405 < $GRP GA=2*1.5 $END>'
      CALL RG(405,1,'#405')
      WRITE (6,'(A)') ' ##### Round 2: what ends a value'
      WRITE (6,'(A)') ' #406 < $GRP A=1 8=9 $END>'
      CALL RM(406,1,'#406')
      WRITE (6,'(A)') ' #407 < $GRP A=8=9 $END>'
      CALL RM(407,1,'#407')
      WRITE (6,'(A)') ' #408 < $GRP I=8=9 $END>'
      CALL RM(408,1,'#408')
      WRITE (6,'(A)') ' #409 < $GRP A=1 ''X''=9 $END>'
      CALL RM(409,1,'#409')
      WRITE (6,'(A)') ' #410 < $GRP A=(1,2)=9 $END>'
      CALL RM(410,1,'#410')
      WRITE (6,'(A)') ' #411 < $GRP I=1.5=2 $END>'
      CALL RM(411,1,'#411')
      WRITE (6,'(A)') ' #412 < $GRP A=1(2) $END>'
      CALL RM(412,1,'#412')
      WRITE (6,'(A)') ' #413 < $GRP I=7) $END>'
      CALL RM(413,1,'#413')
      WRITE (6,'(A)') ' #414 < $GRP I=7''A'' $END>'
      CALL RM(414,1,'#414')
      WRITE (6,'(A)') ' #415 < $GRP I=7/ $END>'
      CALL RM(415,1,'#415')
      WRITE (6,'(A)') ' #416 < $GRP I=7&END $END>'
      CALL RM(416,1,'#416')
      WRITE (6,'(A)') ' #417 < $GRP I=7* $END>'
      CALL RM(417,1,'#417')
      WRITE (6,'(A)') ' #418 < $GRP A=2*7* $END>'
      CALL RM(418,1,'#418')
      WRITE (6,'(A)') ' #419 < $GRP I=1E+ $END>'
      CALL RM(419,1,'#419')
      WRITE (6,'(A)') ' #420 < $GRP I=1E+2 $END>'
      CALL RM(420,1,'#420')
      WRITE (6,'(A)') ' #421 < $GRP I=1.E2 $END>'
      CALL RM(421,1,'#421')
      WRITE (6,'(A)') ' #422 < $GRP I=1E2.5 $END>'
      CALL RM(422,1,'#422')
      WRITE (6,'(A)') ' #423 < $GRP I=1EE2 $END>'
      CALL RM(423,1,'#423')
      WRITE (6,'(A)') ' #424 < $GRP I=1D $END>'
      CALL RM(424,1,'#424')
      WRITE (6,'(A)') ' #425 < $GRP I=1Q $END>'
      CALL RM(425,1,'#425')
      WRITE (6,'(A)') ' #426 < $GRP I=+.5E1 $END>'
      CALL RM(426,1,'#426')
      WRITE (6,'(A)') ' #427 < $GRP I=1 E2 $END>'
      CALL RM(427,1,'#427')
      WRITE (6,'(A)') ' #428 < $GRP I=1.5F $END>'
      CALL RM(428,1,'#428')
      WRITE (6,'(A)') ' #429 < $GRP I=1E-1 $END>'
      CALL RM(429,1,'#429')
      WRITE (6,'(A)') ' #430 < $GRP R=1.5$END $END>'
      CALL RM(430,1,'#430')
      WRITE (6,'(A)') ' #431 < $GRP M=T$END $END>'
      CALL RM(431,1,'#431')
      WRITE (6,'(A)') ' #432 < $GRP M=.TRUE.$END $END>'
      CALL RM(432,1,'#432')
      WRITE (6,'(A)') ' #433 < $GRP M=TRUE$END $END>'
      CALL RM(433,1,'#433')
      WRITE (6,'(A)') ' #434 < $GRP M=T&END $END>'
      CALL RM(434,1,'#434')
      WRITE (6,'(A)') ' #435 < $GRP A=T,F,T $END>'
      CALL RM(435,1,'#435')
      WRITE (6,'(A)') ' #436 < $GRP A=TX FX $END>'
      CALL RM(436,1,'#436')
      WRITE (6,'(A)') ' #437 < $GRP A=2*T $END>'
      CALL RM(437,1,'#437')
      WRITE (6,'(A)') ' #438 < $GRP A=2*TX $END>'
      CALL RM(438,1,'#438')
      WRITE (6,'(A)') ' #439 < $GRP T $END>'
      CALL RM(439,1,'#439')
      WRITE (6,'(A)') ' #440 < $GRP F=1 $END>'
      CALL RM(440,1,'#440')
      WRITE (6,'(A)') ' #441 < $GRP TX=1 $END>'
      CALL RM(441,1,'#441')
      WRITE (6,'(A)') ' #442 < $GRP A=1 TEST=3 $END>'
      CALL RM(442,1,'#442')
      WRITE (6,'(A)') ' #443 < $GRP A=1 TEST $END>'
      CALL RM(443,1,'#443')
      WRITE (6,'(A)') ' #444 < $GRP A=1 FOO $END>'
      CALL RM(444,1,'#444')
      WRITE (6,'(A)') ' #445 < $GRP A=1 BAR $END>'
      CALL RM(445,1,'#445')
      WRITE (6,'(A)') ' #446 < $GRP A=T(1) $END>'
      CALL RM(446,1,'#446')
      WRITE (6,'(A)') ' #447 < $GRP L=F(1)=1 $END>'
      CALL RM(447,1,'#447')
      WRITE (6,'(A)') ' #448 < $GRP L=T,R=1 $END>'
      CALL RM(448,1,'#448')
      WRITE (6,'(A)') ' #449 < $GRP L=.TRUE.R=1 $END>'
      CALL RM(449,1,'#449')
      WRITE (6,'(A)') ' #450 < $GRP M=T1 $END>'
      CALL RM(450,1,'#450')
      WRITE (6,'(A)') ' #451 < $GRP M=T_1 $END>'
      CALL RM(451,1,'#451')
      WRITE (6,'(A)') ' #452 < $GRP M=T$ $END>'
      CALL RM(452,1,'#452')
      WRITE (6,'(A)') ' #453 < $GRP M=T$X $END>'
      CALL RM(453,1,'#453')
      WRITE (6,'(A)') ' #454 < $GRP M=T( $END>'
      CALL RM(454,1,'#454')
      WRITE (6,'(A)') ' #455 < $GRP A=.T,.F $END>'
      CALL RM(455,1,'#455')
      WRITE (6,'(A)') ' #456 < $GRP A=.X $END>'
      CALL RM(456,1,'#456')
      WRITE (6,'(A)') ' #457 < $GRP A=.1.2 $END>'
      CALL RM(457,1,'#457')
      WRITE (6,'(A)') ' #458 < $GRP NOPE$=1 $END>'
      CALL RM(458,1,'#458')
      WRITE (6,'(A)') ' #459 < $GRP NOPE_1=1 $END>'
      CALL RM(459,1,'#459')
      WRITE (6,'(A)') ' #460 < $GRP NOPE$ $END>'
      CALL RM(460,1,'#460')
      WRITE (6,'(A)') ' #461 < $GRP M=.T.X $END>'
      CALL RM(461,1,'#461')
      WRITE (6,'(A)') ' #462 < $GRP M=.FX $END>'
      CALL RM(462,1,'#462')
      WRITE (6,'(A)') ' #463 < $GRP M=T$END I=5 $END>'
      CALL RM(463,1,'#463')
      WRITE (6,'(A)') ' #464 < $GRP M=TRUE$X I=5 $END>'
      CALL RM(464,1,'#464')
      WRITE (6,'(A)') ' #465 < $GRP LONGNAME1$=7 $END>'
      CALL RM(465,1,'#465')
      WRITE (6,'(A)') ' #466 < $GRP I=5$X=6 $END>'
      CALL RM(466,1,'#466')
      WRITE (6,'(A)') ' #467 < $GRP I=5 $X=6 $END>'
      CALL RM(467,1,'#467')
      WRITE (6,'(A)') ' #468 < $GRP M=T$=1 $END>'
      CALL RM(468,1,'#468')
      WRITE (6,'(A)') ' ##### Round 2: complex parts and references'
      WRITE (6,'(A)') ' #469 < $GRP C=(T,F) $END>'
      CALL RM(469,1,'#469')
      WRITE (6,'(A)') ' #470 < $GRP C=(.TRUE.,1) $END>'
      CALL RM(470,1,'#470')
      WRITE (6,'(A)') ' #471 < $GRP C=(1,T) $END>'
      CALL RM(471,1,'#471')
      WRITE (6,'(A)') ' #472 < $GRP C=(1E,2) $END>'
      CALL RM(472,1,'#472')
      WRITE (6,'(A)') ' #473 < $GRP C=(+1,+2) $END>'
      CALL RM(473,1,'#473')
      WRITE (6,'(A)') ' #474 < $GRP C=(1.2.3,2) $END>'
      CALL RM(474,1,'#474')
      WRITE (6,'(A)') ' #475 < $GRP T(2:3)(1:1)=''Z'' $END>'
      CALL RM(475,1,'#475')
      WRITE (6,'(A)') ' #476 < $GRP TA(2)(1:2)(1:1)=''Z'' $END>'
      CALL RM(476,1,'#476')
      WRITE (6,'(A)') ' #477 < $GRP T(A:B)=''Z'' $END>'
      CALL RM(477,1,'#477')
      WRITE (6,'(A)') ' #478 < $GRP T(1.5:2)=''Z'' $END>'
      CALL RM(478,1,'#478')
      WRITE (6,'(A)') ' #479 < $GRP T(1:2:3)=''Z'' $END>'
      CALL RM(479,1,'#479')
      WRITE (6,'(A)') ' #480 < $GRP T(1,2)=''Z'' $END>'
      CALL RM(480,1,'#480')
      WRITE (6,'(A)') ' #481 < $GRP A(1,I)=5 $END>'
      CALL RM(481,1,'#481')
      WRITE (6,'(A)') ' #482 < $GRP A(2I)=5 $END>'
      CALL RM(482,1,'#482')
      WRITE (6,'(A)') ' #483 < $GRP A($)=5 $END>'
      CALL RM(483,1,'#483')
      WRITE (6,'(A)') ' #484 < $GRP A(''1'')=5 $END>'
      CALL RM(484,1,'#484')
      WRITE (6,'(A)') ' #485 < $GRP A(1)5 $END>'
      CALL RM(485,1,'#485')
      WRITE (6,'(A)') ' #486 < $GRP T(2:3)5 $END>'
      CALL RM(486,1,'#486')
      WRITE (6,'(A)') ' #487 < $GRP A(2)(1)=5 $END>'
      CALL RM(487,1,'#487')
      WRITE (6,'(A)') ' #488 < $GRP TA(2)(2)=''Z'' $END>'
      CALL RM(488,1,'#488')
      WRITE (6,'(A)') ' #489 < $GRP TA(2)(2:3) $END>'
      CALL RM(489,1,'#489')
      WRITE (6,'(A)') ' #490 < $GRP T(+2:+3)=''Z'' $END>'
      CALL RM(490,1,'#490')
      WRITE (6,'(A)') ' #491 < $GRP T(-1:3)=''Z'' $END>'
      CALL RM(491,1,'#491')
      WRITE (6,'(A)') ' #492 < $GRP T(99999999999:3)=''Z'' $END>'
      CALL RM(492,1,'#492')
      WRITE (6,'(A)') ' #493 < $GRP T(1:99999999999)=''Z'' $END>'
      CALL RM(493,1,'#493')
      WRITE (6,'(A)') ' #494 < $GRP A(0002)=5 $END>'
      CALL RM(494,1,'#494')
      WRITE (6,'(A)') ' #495 < $GRP A(+)=5 $END>'
      CALL RM(495,1,'#495')
      WRITE (6,'(A)') ' #496 < $GRP A(-)=5 $END>'
      CALL RM(496,1,'#496')
      WRITE (6,'(A)') ' #497 < $GRP T(:)(:)=''Z'' $END>'
      CALL RM(497,1,'#497')
      WRITE (6,'(A)') ' #498 < $GRP A(2,)=5 $END>'
      CALL RM(498,1,'#498')
      WRITE (6,'(A)') ' ##### Round 2: repeats'
      WRITE (6,'(A)') ' #499 < $GRP A=1,0*5 $END>'
      CALL RM(499,1,'#499')
      WRITE (6,'(A)') ' #500 < $GRP A=02*5 $END>'
      CALL RM(500,1,'#500')
      WRITE (6,'(A)') ' #501 < $GRP A=2*-5 $END>'
      CALL RM(501,1,'#501')
      WRITE (6,'(A)') ' #502 < $GRP A=2*+5 $END>'
      CALL RM(502,1,'#502')
      WRITE (6,'(A)') ' #503 < $GRP A=2*5*6 $END>'
      CALL RM(503,1,'#503')
      WRITE (6,'(A)') ' #504 < $GRP A=2*''A'' $END>'
      CALL RM(504,1,'#504')
      WRITE (6,'(A)') ' #505 < $GRP A=99999999999*5 $END>'
      CALL RM(505,1,'#505')
      WRITE (6,'(A)') ' #506 < $GRP A=2*(1,2) $END>'
      CALL RM(506,1,'#506')
      WRITE (6,'(A)') ' #507 < $GRP A=2* $END>'
      CALL RM(507,1,'#507')
      WRITE (6,'(A)') ' #508 < $GRP A=2*$END $END>'
      CALL RM(508,1,'#508')
      WRITE (6,'(A)') ' #509 < $GRP A=2*, $END>'
      CALL RM(509,1,'#509')
      WRITE (6,'(A)') ' #510 < $GRP A=2*,,7 $END>'
      CALL RM(510,1,'#510')
      WRITE (6,'(A)') ' #511 < $GRP A=1* $END>'
      CALL RM(511,1,'#511')
      WRITE (6,'(A)') ' #512 < $GRP A=1*,5 $END>'
      CALL RM(512,1,'#512')
      WRITE (6,'(A)') ' #513 < $GRP T=2*''X'' $END>'
      CALL RM(513,1,'#513')
      WRITE (6,'(A)') ' #514 < $GRP T=2* $END>'
      CALL RM(514,1,'#514')
      WRITE (6,'(A)') ' #515 < $GRP T=1*''XY'' $END>'
      CALL RM(515,1,'#515')
      WRITE (6,'(A)') ' #516 < $GRP CA=2*5 $END>'
      CALL RM(516,1,'#516')
      WRITE (6,'(A)') ' #517 < $GRP A=*5 $END>'
      CALL RM(517,1,'#517')
      WRITE (6,'(A)') ' #518 < $GRP A=+2*5 $END>'
      CALL RM(518,1,'#518')
      WRITE (6,'(A)') ' #519 < $GRP A=2.*5 $END>'
      CALL RM(519,1,'#519')
      WRITE (6,'(A)') ' #520 < $GRP A=2.5*5 $END>'
      CALL RM(520,1,'#520')
      WRITE (6,'(A)') ' #521 < $GRP TA=1*''X'',1*''Y'' $END>'
      CALL RM(521,1,'#521')
      WRITE (6,'(A)') ' ##### Round 2: question marks at a file'
      WRITE (6,'(A)') ' #522 < $GRP ? I=7 $END>'
      CALL RM(522,1,'#522')
      WRITE (6,'(A)') ' #523 < $GRP ?/ I=7 $END>'
      CALL RM(523,1,'#523')
      WRITE (6,'(A)') ' #524 < $GRP I=7 ?/ R=1 $END>'
      CALL RM(524,1,'#524')
      WRITE (6,'(A)') ' #525 < $GRP I=? R=1 $END>'
      CALL RM(525,1,'#525')
      WRITE (6,'(A)') ' #526 < $GRP =?/ I=7 $END>'
      CALL RM(526,1,'#526')
      WRITE (6,'(A)') ' #527 < $GRP ?? I=7 $END>'
      CALL RM(527,1,'#527')
      WRITE (6,'(A)') ' #528 < $GRP I=7 ?R=1 $END>'
      CALL RM(528,1,'#528')
      WRITE (6,'(A)') ' #529 < $GRP A=1 ? 2 $END>'
      CALL RM(529,1,'#529')
      WRITE (6,'(A)') ' #530 < ?X>'
      CALL RM(530,1,'#530')
      WRITE (6,'(A)') ' #531 < ?X $GRP I=1 $END>'
      CALL RM(531,1,'#531')
      WRITE (6,'(A)') ' #532 < $GRP A=1,?,3 $END>'
      CALL RM(532,1,'#532')
      WRITE (6,'(A)') ' #533 < $GRP T=''?'' $END>'
      CALL RM(533,1,'#533')
      WRITE (6,'(A)') ' ##### Writes: where a record breaks around repea
     &ts and separators'
      WRITE (6,'(A)') ' #534 two runs of texts of 15'
      CALL W534
      WRITE (6,'(A)') ' #535 two runs of texts of 16'
      CALL W535
      WRITE (6,'(A)') ' #536 two runs of texts of 17'
      CALL W536
      WRITE (6,'(A)') ' #537 two runs of texts of 18'
      CALL W537
      WRITE (6,'(A)') ' #538 two runs of texts of 19'
      CALL W538
      WRITE (6,'(A)') ' #539 two runs of texts of 20'
      CALL W539
      WRITE (6,'(A)') ' #540 a text of 56, a repeat'
      CALL W540
      WRITE (6,'(A)') ' #541 a text of 57, a repeat'
      CALL W541
      WRITE (6,'(A)') ' #542 a text of 58, a repeat'
      CALL W542
      WRITE (6,'(A)') ' #543 a text of 59, a repeat'
      CALL W543
      WRITE (6,'(A)') ' #544 a text of 60, a repeat'
      CALL W544
      WRITE (6,'(A)') ' #545 a text of 61, a repeat'
      CALL W545
      WRITE (6,'(A)') ' #546 a text of 62, a repeat'
      CALL W546
      WRITE (6,'(A)') ' #547 a text of 63, a repeat'
      CALL W547
      WRITE (6,'(A)') ' #548 a text of 64, a repeat'
      CALL W548
      WRITE (6,'(A)') ' #549 a text of 65, a repeat'
      CALL W549
      WRITE (6,'(A)') ' #550 a text of 66, a repeat'
      CALL W550
      WRITE (6,'(A)') ' #551 a text of 67, a repeat'
      CALL W551
      WRITE (6,'(A)') ' #552 a text of 68, a repeat'
      CALL W552
      WRITE (6,'(A)') ' #553 a text of 69, a repeat'
      CALL W553
      WRITE (6,'(A)') ' #554 a text of 70, a repeat'
      CALL W554
      WRITE (6,'(A)') ' #555 a repeat of a text of 60'
      CALL W555
      WRITE (6,'(A)') ' #556 a repeat of a text of 61'
      CALL W556
      WRITE (6,'(A)') ' #557 a repeat of a text of 62'
      CALL W557
      WRITE (6,'(A)') ' #558 a repeat of a text of 63'
      CALL W558
      WRITE (6,'(A)') ' #559 a repeat of a text of 64'
      CALL W559
      WRITE (6,'(A)') ' #560 a repeat of a text of 65'
      CALL W560
      WRITE (6,'(A)') ' #561 a repeat of a text of 66'
      CALL W561
      WRITE (6,'(A)') ' #562 a repeat of a text of 67'
      CALL W562
      WRITE (6,'(A)') ' #563 a repeat of a text of 68'
      CALL W563
      WRITE (6,'(A)') ' #564 a repeat of a text of 69'
      CALL W564
      WRITE (6,'(A)') ' #565 a repeat of a text of 70'
      CALL W565
      WRITE (6,'(A)') ' #566 logicals differing but both true'
      CALL W566
      WRITE (6,'(A)') ' #567 a repeat after 0 values, a name of 1'
      CALL W567
      WRITE (6,'(A)') ' #568 a repeat after 1 values, a name of 1'
      CALL W568
      WRITE (6,'(A)') ' #569 a repeat after 2 values, a name of 1'
      CALL W569
      WRITE (6,'(A)') ' #570 a repeat after 3 values, a name of 1'
      CALL W570
      WRITE (6,'(A)') ' #571 a repeat after 4 values, a name of 1'
      CALL W571
      WRITE (6,'(A)') ' #572 a repeat after 5 values, a name of 1'
      CALL W572
      WRITE (6,'(A)') ' #573 a repeat after 0 values, a name of 9'
      CALL W573
      WRITE (6,'(A)') ' #574 a repeat after 1 values, a name of 9'
      CALL W574
      WRITE (6,'(A)') ' #575 a repeat after 2 values, a name of 9'
      CALL W575
      WRITE (6,'(A)') ' #576 a repeat after 3 values, a name of 9'
      CALL W576
      WRITE (6,'(A)') ' #577 a repeat after 4 values, a name of 9'
      CALL W577
      WRITE (6,'(A)') ' #578 a repeat after 5 values, a name of 9'
      CALL W578
      WRITE (6,'(A)') ' #579 a repeat after 0 values, a name of 17'
      CALL W579
      WRITE (6,'(A)') ' #580 a repeat after 1 values, a name of 17'
      CALL W580
      WRITE (6,'(A)') ' #581 a repeat after 2 values, a name of 17'
      CALL W581
      WRITE (6,'(A)') ' #582 a repeat after 3 values, a name of 17'
      CALL W582
      WRITE (6,'(A)') ' #583 a repeat after 4 values, a name of 17'
      CALL W583
      WRITE (6,'(A)') ' #584 a repeat after 5 values, a name of 17'
      CALL W584
      WRITE (6,'(A)') ' #585 a repeat after 0 values, a name of 25'
      CALL W585
      WRITE (6,'(A)') ' #586 a repeat after 1 values, a name of 25'
      CALL W586
      WRITE (6,'(A)') ' #587 a repeat after 2 values, a name of 25'
      CALL W587
      WRITE (6,'(A)') ' #588 a repeat after 3 values, a name of 25'
      CALL W588
      WRITE (6,'(A)') ' #589 a repeat after 4 values, a name of 25'
      CALL W589
      WRITE (6,'(A)') ' #590 a repeat after 5 values, a name of 25'
      CALL W590
      WRITE (6,'(A)') ' #591 a logical repeat after 21 values'
      CALL W591
      WRITE (6,'(A)') ' #592 a logical repeat after 22 values'
      CALL W592
      WRITE (6,'(A)') ' #593 a logical repeat after 23 values'
      CALL W593
      WRITE (6,'(A)') ' #594 a logical repeat after 24 values'
      CALL W594
      WRITE (6,'(A)') ' ##### Writes: every type'
      WRITE (6,'(A)') ' #595 one integer'
      CALL W595
      WRITE (6,'(A)') ' #596 the extreme integers'
      CALL W596
      WRITE (6,'(A)') ' #597 a word'
      CALL W597
      WRITE (6,'(A)') ' #598 logicals'
      CALL W598
      WRITE (6,'(A)') ' #599 a byte logical'
      CALL W599
      WRITE (6,'(A)') ' #600 reals'
      CALL W600
      WRITE (6,'(A)') ' #601 doubles'
      CALL W601
      WRITE (6,'(A)') ' #602 quads'
      CALL W602
      WRITE (6,'(A)') ' #603 complex'
      CALL W603
      WRITE (6,'(A)') ' #604 double complex'
      CALL W604
      WRITE (6,'(A)') ' #605 characters'
      CALL W605
      WRITE (6,'(A)') ' #606 apostrophes'
      CALL W606
      WRITE (6,'(A)') ' #607 a long group name'
      CALL W607
      WRITE (6,'(A)') ' #608 a short group name'
      CALL W608
      WRITE (6,'(A)') ' ##### Writes: names'
      WRITE (6,'(A)') ' #609 a name of 1'
      CALL W609
      WRITE (6,'(A)') ' #610 a name of 2'
      CALL W610
      WRITE (6,'(A)') ' #611 a name of 3'
      CALL W611
      WRITE (6,'(A)') ' #612 a name of 4'
      CALL W612
      WRITE (6,'(A)') ' #613 a name of 5'
      CALL W613
      WRITE (6,'(A)') ' #614 a name of 6'
      CALL W614
      WRITE (6,'(A)') ' #615 a name of 7'
      CALL W615
      WRITE (6,'(A)') ' #616 a name of 8'
      CALL W616
      WRITE (6,'(A)') ' #617 a name of 9'
      CALL W617
      WRITE (6,'(A)') ' #618 a name of 10'
      CALL W618
      WRITE (6,'(A)') ' #619 a name of 11'
      CALL W619
      WRITE (6,'(A)') ' #620 a name of 12'
      CALL W620
      WRITE (6,'(A)') ' #621 a name of 13'
      CALL W621
      WRITE (6,'(A)') ' #622 a name of 14'
      CALL W622
      WRITE (6,'(A)') ' #623 a name of 15'
      CALL W623
      WRITE (6,'(A)') ' #624 a name of 16'
      CALL W624
      WRITE (6,'(A)') ' #625 a name of 17'
      CALL W625
      WRITE (6,'(A)') ' #626 a name of 24'
      CALL W626
      WRITE (6,'(A)') ' #627 a name of 25'
      CALL W627
      WRITE (6,'(A)') ' #628 a name of 31'
      CALL W628
      WRITE (6,'(A)') ' #629 a name of 31, an array'
      CALL W629
      WRITE (6,'(A)') ' #630 a name of 31, a long text'
      CALL W630
      WRITE (6,'(A)') ' ##### Writes: arrays and repeats'
      WRITE (6,'(A)') ' #631 one element'
      CALL W631
      WRITE (6,'(A)') ' #632 all the same'
      CALL W632
      WRITE (6,'(A)') ' #633 repeats first'
      CALL W633
      WRITE (6,'(A)') ' #634 repeats last'
      CALL W634
      WRITE (6,'(A)') ' #635 repeats between'
      CALL W635
      WRITE (6,'(A)') ' #636 two runs'
      CALL W636
      WRITE (6,'(A)') ' #637 no repeats'
      CALL W637
      WRITE (6,'(A)') ' #638 an array, then a scalar'
      CALL W638
      WRITE (6,'(A)') ' #639 a scalar, then an array'
      CALL W639
      WRITE (6,'(A)') ' #640 two dimensions'
      CALL W640
      WRITE (6,'(A)') ' #641 lower bounds'
      CALL W641
      WRITE (6,'(A)') ' #642 words'
      CALL W642
      WRITE (6,'(A)') ' #643 words, the same'
      CALL W643
      WRITE (6,'(A)') ' #644 byte logicals'
      CALL W644
      WRITE (6,'(A)') ' #645 logicals'
      CALL W645
      WRITE (6,'(A)') ' #646 logicals, the same'
      CALL W646
      WRITE (6,'(A)') ' #647 reals'
      CALL W647
      WRITE (6,'(A)') ' #648 reals, repeats'
      CALL W648
      WRITE (6,'(A)') ' #649 doubles'
      CALL W649
      WRITE (6,'(A)') ' #650 doubles, repeats'
      CALL W650
      WRITE (6,'(A)') ' #651 quads'
      CALL W651
      WRITE (6,'(A)') ' #652 complex'
      CALL W652
      WRITE (6,'(A)') ' #653 complex, repeats'
      CALL W653
      WRITE (6,'(A)') ' #654 double complex'
      CALL W654
      WRITE (6,'(A)') ' #655 characters'
      CALL W655
      WRITE (6,'(A)') ' #656 characters, repeats'
      CALL W656
      WRITE (6,'(A)') ' #657 integers across records'
      CALL W657
      WRITE (6,'(A)') ' #658 integers, repeats at the record''s end'
      CALL W658
      WRITE (6,'(A)') ' ##### Writes: where records break'
      WRITE (6,'(A)') ' #659 a text of 56, last'
      CALL W659
      WRITE (6,'(A)') ' #660 a text of 56, then another'
      CALL W660
      WRITE (6,'(A)') ' #661 a text of 57, last'
      CALL W661
      WRITE (6,'(A)') ' #662 a text of 57, then another'
      CALL W662
      WRITE (6,'(A)') ' #663 a text of 58, last'
      CALL W663
      WRITE (6,'(A)') ' #664 a text of 58, then another'
      CALL W664
      WRITE (6,'(A)') ' #665 a text of 59, last'
      CALL W665
      WRITE (6,'(A)') ' #666 a text of 59, then another'
      CALL W666
      WRITE (6,'(A)') ' #667 a text of 60, last'
      CALL W667
      WRITE (6,'(A)') ' #668 a text of 60, then another'
      CALL W668
      WRITE (6,'(A)') ' #669 a text of 61, last'
      CALL W669
      WRITE (6,'(A)') ' #670 a text of 61, then another'
      CALL W670
      WRITE (6,'(A)') ' #671 a text of 62, last'
      CALL W671
      WRITE (6,'(A)') ' #672 a text of 62, then another'
      CALL W672
      WRITE (6,'(A)') ' #673 a text of 63, last'
      CALL W673
      WRITE (6,'(A)') ' #674 a text of 63, then another'
      CALL W674
      WRITE (6,'(A)') ' #675 a text of 64, last'
      CALL W675
      WRITE (6,'(A)') ' #676 a text of 64, then another'
      CALL W676
      WRITE (6,'(A)') ' #677 a text of 65, last'
      CALL W677
      WRITE (6,'(A)') ' #678 a text of 65, then another'
      CALL W678
      WRITE (6,'(A)') ' #679 a text of 66, last'
      CALL W679
      WRITE (6,'(A)') ' #680 a text of 66, then another'
      CALL W680
      WRITE (6,'(A)') ' #681 a text of 67, last'
      CALL W681
      WRITE (6,'(A)') ' #682 a text of 67, then another'
      CALL W682
      WRITE (6,'(A)') ' #683 a text of 68, last'
      CALL W683
      WRITE (6,'(A)') ' #684 a text of 68, then another'
      CALL W684
      WRITE (6,'(A)') ' #685 a text of 69, last'
      CALL W685
      WRITE (6,'(A)') ' #686 a text of 69, then another'
      CALL W686
      WRITE (6,'(A)') ' #687 a text of 70, last'
      CALL W687
      WRITE (6,'(A)') ' #688 a text of 70, then another'
      CALL W688
      WRITE (6,'(A)') ' #689 a text of 71, last'
      CALL W689
      WRITE (6,'(A)') ' #690 a text of 71, then another'
      CALL W690
      WRITE (6,'(A)') ' #691 a text of 72, last'
      CALL W691
      WRITE (6,'(A)') ' #692 a text of 72, then another'
      CALL W692
      WRITE (6,'(A)') ' #693 a text of 73, last'
      CALL W693
      WRITE (6,'(A)') ' #694 a text of 73, then another'
      CALL W694
      WRITE (6,'(A)') ' #695 a text of 74, last'
      CALL W695
      WRITE (6,'(A)') ' #696 a text of 74, then another'
      CALL W696
      WRITE (6,'(A)') ' #697 a text of 75, last'
      CALL W697
      WRITE (6,'(A)') ' #698 a text of 75, then another'
      CALL W698
      WRITE (6,'(A)') ' #699 a text of 76, last'
      CALL W699
      WRITE (6,'(A)') ' #700 a text of 76, then another'
      CALL W700
      WRITE (6,'(A)') ' #701 a text of 77, last'
      CALL W701
      WRITE (6,'(A)') ' #702 a text of 77, then another'
      CALL W702
      WRITE (6,'(A)') ' #703 a text of 78, last'
      CALL W703
      WRITE (6,'(A)') ' #704 a text of 78, then another'
      CALL W704
      WRITE (6,'(A)') ' #705 a text of 79, last'
      CALL W705
      WRITE (6,'(A)') ' #706 a text of 79, then another'
      CALL W706
      WRITE (6,'(A)') ' #707 a text of 80, last'
      CALL W707
      WRITE (6,'(A)') ' #708 a text of 80, then another'
      CALL W708
      WRITE (6,'(A)') ' #709 a text of 81, last'
      CALL W709
      WRITE (6,'(A)') ' #710 a text of 81, then another'
      CALL W710
      WRITE (6,'(A)') ' #711 a text of 82, last'
      CALL W711
      WRITE (6,'(A)') ' #712 a text of 82, then another'
      CALL W712
      WRITE (6,'(A)') ' #713 a text of 100'
      CALL W713
      WRITE (6,'(A)') ' #714 a text of 158'
      CALL W714
      WRITE (6,'(A)') ' #715 a text of 159'
      CALL W715
      WRITE (6,'(A)') ' #716 a text of 160'
      CALL W716
      WRITE (6,'(A)') ' #717 a text of 161'
      CALL W717
      WRITE (6,'(A)') ' #718 a text of 162'
      CALL W718
      WRITE (6,'(A)') ' #719 a text of 200'
      CALL W719
      WRITE (6,'(A)') ' #720 a text of 255'
      CALL W720
      WRITE (6,'(A)') ' #721 apostrophes across a record'
      CALL W721
      WRITE (6,'(A)') ' #722 an apostrophe at the break'
      CALL W722
      WRITE (6,'(A)') ' #723 texts of 10'
      CALL W723
      WRITE (6,'(A)') ' #724 texts of 11'
      CALL W724
      WRITE (6,'(A)') ' #725 texts of 12'
      CALL W725
      WRITE (6,'(A)') ' #726 texts of 13'
      CALL W726
      WRITE (6,'(A)') ' #727 texts of 14'
      CALL W727
      WRITE (6,'(A)') ' #728 texts of 15'
      CALL W728
      WRITE (6,'(A)') ' #729 texts of 16'
      CALL W729
      WRITE (6,'(A)') ' #730 texts of 17'
      CALL W730
      WRITE (6,'(A)') ' #731 texts of 18'
      CALL W731
      WRITE (6,'(A)') ' #732 texts of 19'
      CALL W732
      WRITE (6,'(A)') ' #733 texts of 20'
      CALL W733
      WRITE (6,'(A)') ' #734 texts of 21'
      CALL W734
      WRITE (6,'(A)') ' #735 texts of 22'
      CALL W735
      WRITE (6,'(A)') ' #736 texts of 23'
      CALL W736
      WRITE (6,'(A)') ' #737 texts of 24'
      CALL W737
      WRITE (6,'(A)') ' #738 texts of 25'
      CALL W738
      WRITE (6,'(A)') ' #739 texts of 26'
      CALL W739
      WRITE (6,'(A)') ' #740 texts of 27'
      CALL W740
      WRITE (6,'(A)') ' #741 texts of 28'
      CALL W741
      WRITE (6,'(A)') ' #742 texts of 29'
      CALL W742
      WRITE (6,'(A)') ' #743 texts of 30'
      CALL W743
      WRITE (6,'(A)') ' #744 texts of 31'
      CALL W744
      WRITE (6,'(A)') ' #745 texts of 32'
      CALL W745
      WRITE (6,'(A)') ' #746 texts of 33'
      CALL W746
      WRITE (6,'(A)') ' #747 texts of 34'
      CALL W747
      WRITE (6,'(A)') ' #748 texts of 35'
      CALL W748
      WRITE (6,'(A)') ' #749 texts of 36'
      CALL W749
      WRITE (6,'(A)') ' #750 texts of 37'
      CALL W750
      WRITE (6,'(A)') ' #751 texts of 38'
      CALL W751
      WRITE (6,'(A)') ' #752 texts of 39'
      CALL W752
      WRITE (6,'(A)') ' #753 texts of 40'
      CALL W753
      WRITE (6,'(A)') ' #754 repeated texts of 30'
      CALL W754
      WRITE (6,'(A)') ' #755 repeated texts of 31'
      CALL W755
      WRITE (6,'(A)') ' #756 repeated texts of 32'
      CALL W756
      WRITE (6,'(A)') ' #757 repeated texts of 33'
      CALL W757
      WRITE (6,'(A)') ' #758 repeated texts of 34'
      CALL W758
      WRITE (6,'(A)') ' #759 repeated texts of 35'
      CALL W759
      WRITE (6,'(A)') ' #760 repeated texts of 36'
      CALL W760
      WRITE (6,'(A)') ' #761 a repeat of a long text'
      CALL W761
      WRITE (6,'(A)') ' #762 integers after a name of 9'
      CALL W762
      WRITE (6,'(A)') ' #763 reals after a name of 17'
      CALL W763
      WRITE (6,'(A)') ' #764 integers after a name of 10'
      CALL W764
      WRITE (6,'(A)') ' #765 reals after a name of 18'
      CALL W765
      WRITE (6,'(A)') ' #766 integers after a name of 11'
      CALL W766
      WRITE (6,'(A)') ' #767 reals after a name of 19'
      CALL W767
      WRITE (6,'(A)') ' #768 integers after a name of 12'
      CALL W768
      WRITE (6,'(A)') ' #769 reals after a name of 20'
      CALL W769
      WRITE (6,'(A)') ' #770 integers after a name of 13'
      CALL W770
      WRITE (6,'(A)') ' #771 reals after a name of 21'
      CALL W771
      WRITE (6,'(A)') ' #772 integers after a name of 14'
      CALL W772
      WRITE (6,'(A)') ' #773 reals after a name of 22'
      CALL W773
      WRITE (6,'(A)') ' #774 integers after a name of 15'
      CALL W774
      WRITE (6,'(A)') ' #775 reals after a name of 23'
      CALL W775
      WRITE (6,'(A)') ' #776 integers after a name of 16'
      CALL W776
      WRITE (6,'(A)') ' #777 reals after a name of 24'
      CALL W777
      WRITE (6,'(A)') ' #778 repeats at a record''s end, reals'
      CALL W778
      WRITE (6,'(A)') ' #779 complex across a record'
      CALL W779
      WRITE (6,'(A)') ' #780 double complex across a record'
      CALL W780
      WRITE (6,'(A)') ' #781 a complex that ends a record'
      CALL W781
      WRITE (6,'(A)') ' ##### Writes: G_floating'
      WRITE (6,'(A)') ' #782 G'
      CALL W782
      WRITE (6,'(A)') ' #783 G arrays'
      CALL W783
      WRITE (6,'(A)') ' ##### The end of SYS$INPUT'
      WRITE (6,'(A)') ' #784'
      CALL RM(784,1,' ')
      WRITE (6,'(A)') ' #785'
      CALL RM(785,1,' ')
      END
      SUBROUTINE RM(NCASE9,NTIME9,MARK9)
      INTEGER NCASE9,NTIME9,IOS9
      CHARACTER*(*) MARK9
      CHARACTER*200 LINE9
      INTEGER*4 I
      INTEGER*2 J
      LOGICAL*4 L
      LOGICAL*2 M
      LOGICAL*1 N
      REAL*4 R
      REAL*8 D
      REAL*16 Q
      COMPLEX*8 C
      COMPLEX*16 E
      CHARACTER*5 T
      INTEGER*4 A(1:3)
      INTEGER*4 B(1:2,1:2)
      INTEGER*4 AL(-1:1)
      REAL*4 RA(1:3)
      CHARACTER*3 TA(1:3)
      COMPLEX*8 CA(1:2)
      LOGICAL*4 LA(1:2)
      INTEGER*4 LONGNAME1
      NAMELIST /GRP/ I, J, L, M, N, R, D, Q, C, E, T, A, B, AL, RA, TA, 
     &CA, LA, LONGNAME1
      I=42
      J=43
      L=.TRUE.
      M=.FALSE.
      N=.TRUE.
      R=2.5
      D=3.5
      Q=4.5
      C=(1.5,-2.5)
      E=(3.5,-4.5)
      T='HELLO'
      A(1)=1
      A(2)=1
      A(3)=1
      B(1,1)=1
      B(2,1)=2
      B(1,2)=3
      B(2,2)=4
      AL(-1)=7
      AL(0)=8
      AL(1)=9
      RA(1)=1.0
      RA(2)=1.0
      RA(3)=1.0
      TA(1)='AAA'
      TA(2)='BBB'
      TA(3)='CCC'
      CA(1)=(1.0,1.0)
      CA(2)=(2.0,2.0)
      LA(1)=.TRUE.
      LA(2)=.FALSE.
      LONGNAME1=-5
      IF (MARK9 .EQ. ' ') GO TO 20
   10 READ (5,'(A)') LINE9
      IF (LINE9 .NE. MARK9) GO TO 10
   20 DO 30 K9=1,NTIME9
      IOS9=0
      READ (5,GRP,IOSTAT=IOS9)
   30 CONTINUE
      IF (IOS9 .LT. 0) IOS9=24
      WRITE (6,'(1X,''='',I5,'' e='',I5)') NCASE9,IOS9
      WRITE (6,GRP)
      END
      OPTIONS /G_FLOATING
      SUBROUTINE RG(NCASE9,NTIME9,MARK9)
      INTEGER NCASE9,NTIME9,IOS9
      CHARACTER*(*) MARK9
      CHARACTER*200 LINE9
      REAL*8 G
      COMPLEX*16 H
      REAL*8 GA(1:2)
      NAMELIST /GRP/ G, H, GA
      G=2.5
      H=(1.5,-2.5)
      GA(1)=1.0
      GA(2)=1.0
      IF (MARK9 .EQ. ' ') GO TO 20
   10 READ (5,'(A)') LINE9
      IF (LINE9 .NE. MARK9) GO TO 10
   20 DO 30 K9=1,NTIME9
      IOS9=0
      READ (5,GRP,IOSTAT=IOS9)
   30 CONTINUE
      IF (IOS9 .LT. 0) IOS9=24
      WRITE (6,'(1X,''='',I5,'' e='',I5)') NCASE9,IOS9
      WRITE (6,GRP)
      END
      SUBROUTINE W534
      CHARACTER*15 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W535
      CHARACTER*16 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W536
      CHARACTER*17 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W537
      CHARACTER*18 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W538
      CHARACTER*19 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W539
      CHARACTER*20 A(1:7)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCC'
      A(4)='CCCCCCCCCCCCCCCCCCCC'
      A(5)='DDDDDDDDDDDDDDDDDDDD'
      A(6)='EEEEEEEEEEEEEEEEEEEE'
      A(7)='EEEEEEEEEEEEEEEEEEEE'
      WRITE (6,GRP)
      END
      SUBROUTINE W540
      CHARACTER*56 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W541
      CHARACTER*57 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W542
      CHARACTER*58 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W543
      CHARACTER*59 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W544
      CHARACTER*60 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &'
      WRITE (6,GRP)
      END
      SUBROUTINE W545
      CHARACTER*61 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &A'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &B'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &B'
      WRITE (6,GRP)
      END
      SUBROUTINE W546
      CHARACTER*62 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BB'
      WRITE (6,GRP)
      END
      SUBROUTINE W547
      CHARACTER*63 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W548
      CHARACTER*64 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W549
      CHARACTER*65 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W550
      CHARACTER*66 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W551
      CHARACTER*67 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W552
      CHARACTER*68 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W553
      CHARACTER*69 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W554
      CHARACTER*70 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBB'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
     &BBBBBBBBBB'
      WRITE (6,GRP)
      END
      SUBROUTINE W555
      CHARACTER*60 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &'
      WRITE (6,GRP)
      END
      SUBROUTINE W556
      CHARACTER*61 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &A'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &A'
      WRITE (6,GRP)
      END
      SUBROUTINE W557
      CHARACTER*62 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AA'
      WRITE (6,GRP)
      END
      SUBROUTINE W558
      CHARACTER*63 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W559
      CHARACTER*64 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W560
      CHARACTER*65 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W561
      CHARACTER*66 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W562
      CHARACTER*67 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W563
      CHARACTER*68 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W564
      CHARACTER*69 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W565
      CHARACTER*70 A(1:2)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
     &AAAAAAAAAA'
      WRITE (6,GRP)
      END
      SUBROUTINE W566
      LOGICAL*4 A(1:2)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W567
      INTEGER*4 N(1:4)
      NAMELIST /GRP/ N
      N(1)=6
      N(2)=6
      N(3)=6
      N(4)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W568
      INTEGER*4 N(1:5)
      NAMELIST /GRP/ N
      N(1)=1
      N(2)=6
      N(3)=6
      N(4)=6
      N(5)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W569
      INTEGER*4 N(1:6)
      NAMELIST /GRP/ N
      N(1)=1
      N(2)=2
      N(3)=6
      N(4)=6
      N(5)=6
      N(6)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W570
      INTEGER*4 N(1:7)
      NAMELIST /GRP/ N
      N(1)=1
      N(2)=2
      N(3)=3
      N(4)=6
      N(5)=6
      N(6)=6
      N(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W571
      INTEGER*4 N(1:8)
      NAMELIST /GRP/ N
      N(1)=1
      N(2)=2
      N(3)=3
      N(4)=4
      N(5)=6
      N(6)=6
      N(7)=6
      N(8)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W572
      INTEGER*4 N(1:9)
      NAMELIST /GRP/ N
      N(1)=1
      N(2)=2
      N(3)=3
      N(4)=4
      N(5)=5
      N(6)=6
      N(7)=6
      N(8)=6
      N(9)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W573
      INTEGER*4 NNNNNNNNN(1:4)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=6
      NNNNNNNNN(2)=6
      NNNNNNNNN(3)=6
      NNNNNNNNN(4)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W574
      INTEGER*4 NNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=6
      NNNNNNNNN(3)=6
      NNNNNNNNN(4)=6
      NNNNNNNNN(5)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W575
      INTEGER*4 NNNNNNNNN(1:6)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=2
      NNNNNNNNN(3)=6
      NNNNNNNNN(4)=6
      NNNNNNNNN(5)=6
      NNNNNNNNN(6)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W576
      INTEGER*4 NNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=2
      NNNNNNNNN(3)=3
      NNNNNNNNN(4)=6
      NNNNNNNNN(5)=6
      NNNNNNNNN(6)=6
      NNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W577
      INTEGER*4 NNNNNNNNN(1:8)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=2
      NNNNNNNNN(3)=3
      NNNNNNNNN(4)=4
      NNNNNNNNN(5)=6
      NNNNNNNNN(6)=6
      NNNNNNNNN(7)=6
      NNNNNNNNN(8)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W578
      INTEGER*4 NNNNNNNNN(1:9)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=2
      NNNNNNNNN(3)=3
      NNNNNNNNN(4)=4
      NNNNNNNNN(5)=5
      NNNNNNNNN(6)=6
      NNNNNNNNN(7)=6
      NNNNNNNNN(8)=6
      NNNNNNNNN(9)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W579
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:4)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=6
      NNNNNNNNNNNNNNNNN(2)=6
      NNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNN(4)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W580
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNN(2)=6
      NNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNN(5)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W581
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:6)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNN(6)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W582
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W583
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:8)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNN(7)=6
      NNNNNNNNNNNNNNNNN(8)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W584
      INTEGER*4 NNNNNNNNNNNNNNNNN(1:9)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNN(7)=6
      NNNNNNNNNNNNNNNNN(8)=6
      NNNNNNNNNNNNNNNNN(9)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W585
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:4)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W586
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(5)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W587
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:6)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(6)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W588
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W589
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:8)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNNNNNNNNNNN(5)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(7)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(8)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W590
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN(1:9)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(7)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(8)=6
      NNNNNNNNNNNNNNNNNNNNNNNNN(9)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W591
      LOGICAL*4 A(1:25)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      A(21)=.TRUE.
      A(22)=.FALSE.
      A(23)=.FALSE.
      A(24)=.FALSE.
      A(25)=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W592
      LOGICAL*4 A(1:26)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      A(21)=.TRUE.
      A(22)=.FALSE.
      A(23)=.TRUE.
      A(24)=.TRUE.
      A(25)=.TRUE.
      A(26)=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W593
      LOGICAL*4 A(1:27)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      A(21)=.TRUE.
      A(22)=.FALSE.
      A(23)=.TRUE.
      A(24)=.FALSE.
      A(25)=.FALSE.
      A(26)=.FALSE.
      A(27)=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W594
      LOGICAL*4 A(1:28)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      A(21)=.TRUE.
      A(22)=.FALSE.
      A(23)=.TRUE.
      A(24)=.FALSE.
      A(25)=.TRUE.
      A(26)=.TRUE.
      A(27)=.TRUE.
      A(28)=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W595
      INTEGER*4 I
      NAMELIST /GRP/ I
      I=42
      WRITE (6,GRP)
      END
      SUBROUTINE W596
      INTEGER*4 I
      INTEGER*4 J
      NAMELIST /GRP/ I, J
      I=2147483647
      J=(-2147483647-1)
      WRITE (6,GRP)
      END
      SUBROUTINE W597
      INTEGER*2 J
      NAMELIST /GRP/ J
      J=-5
      WRITE (6,GRP)
      END
      SUBROUTINE W598
      LOGICAL*4 L
      LOGICAL*2 M
      LOGICAL*1 N
      NAMELIST /GRP/ L, M, N
      L=.TRUE.
      M=.FALSE.
      N=.TRUE.
      WRITE (6,GRP)
      END
      SUBROUTINE W599
      LOGICAL*1 N
      NAMELIST /GRP/ N
      N=.FALSE.
      WRITE (6,GRP)
      END
      SUBROUTINE W600
      REAL*4 R
      REAL*4 S
      REAL*4 U
      REAL*4 V
      NAMELIST /GRP/ R, S, U, V
      R=2.5
      S=-1.0E10
      U=0.0
      V=1.0E-5
      WRITE (6,GRP)
      END
      SUBROUTINE W601
      REAL*8 D
      REAL*8 F
      REAL*8 G
      NAMELIST /GRP/ D, F, G
      D=2.5D0
      F=-1.0D30
      G=0.0D0
      WRITE (6,GRP)
      END
      SUBROUTINE W602
      REAL*16 Q
      REAL*16 P
      NAMELIST /GRP/ Q, P
      Q=2.5Q0
      P=-1.0Q300
      WRITE (6,GRP)
      END
      SUBROUTINE W603
      COMPLEX*8 C
      COMPLEX*8 D
      NAMELIST /GRP/ C, D
      C=(1.5,-2.5)
      D=(0.0,1.0E10)
      WRITE (6,GRP)
      END
      SUBROUTINE W604
      COMPLEX*16 E
      NAMELIST /GRP/ E
      E=(1.5D0,-2.5D0)
      WRITE (6,GRP)
      END
      SUBROUTINE W605
      CHARACTER*5 T
      CHARACTER*1 U
      CHARACTER*3 V
      NAMELIST /GRP/ T, U, V
      T='HELLO'
      U='X'
      V='   '
      WRITE (6,GRP)
      END
      SUBROUTINE W606
      CHARACTER*5 T
      CHARACTER*1 U
      CHARACTER*3 V
      NAMELIST /GRP/ T, U, V
      T='A''B  '
      U=''''
      V=''''''''
      WRITE (6,GRP)
      END
      SUBROUTINE W607
      INTEGER*4 I
      NAMELIST /GROUPNAMEOFTHIRTYONECHARACTERS1/ I
      I=1
      WRITE (6,GROUPNAMEOFTHIRTYONECHARACTERS1)
      END
      SUBROUTINE W608
      INTEGER*4 I
      NAMELIST /G/ I
      I=1
      WRITE (6,G)
      END
      SUBROUTINE W609
      INTEGER*4 N
      NAMELIST /GRP/ N
      N=1
      WRITE (6,GRP)
      END
      SUBROUTINE W610
      INTEGER*4 NN
      NAMELIST /GRP/ NN
      NN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W611
      INTEGER*4 NNN
      NAMELIST /GRP/ NNN
      NNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W612
      INTEGER*4 NNNN
      NAMELIST /GRP/ NNNN
      NNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W613
      INTEGER*4 NNNNN
      NAMELIST /GRP/ NNNNN
      NNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W614
      INTEGER*4 NNNNNN
      NAMELIST /GRP/ NNNNNN
      NNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W615
      INTEGER*4 NNNNNNN
      NAMELIST /GRP/ NNNNNNN
      NNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W616
      INTEGER*4 NNNNNNNN
      NAMELIST /GRP/ NNNNNNNN
      NNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W617
      INTEGER*4 NNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W618
      INTEGER*4 NNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNN
      NNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W619
      INTEGER*4 NNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNN
      NNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W620
      INTEGER*4 NNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNN
      NNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W621
      INTEGER*4 NNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNN
      NNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W622
      INTEGER*4 NNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNN
      NNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W623
      INTEGER*4 NNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W624
      INTEGER*4 NNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W625
      INTEGER*4 NNNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W626
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W627
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W628
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN=1
      WRITE (6,GRP)
      END
      SUBROUTINE W629
      INTEGER*4 NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(1:8)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(7)=7
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN(8)=8
      WRITE (6,GRP)
      END
      SUBROUTINE W630
      CHARACTER*60 NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W631
      INTEGER*4 A(1:1)
      NAMELIST /GRP/ A
      A(1)=5
      WRITE (6,GRP)
      END
      SUBROUTINE W632
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=5
      A(2)=5
      A(3)=5
      A(4)=5
      WRITE (6,GRP)
      END
      SUBROUTINE W633
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=5
      A(2)=5
      A(3)=5
      A(4)=6
      WRITE (6,GRP)
      END
      SUBROUTINE W634
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=6
      A(2)=5
      A(3)=5
      A(4)=5
      WRITE (6,GRP)
      END
      SUBROUTINE W635
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=6
      A(2)=5
      A(3)=5
      A(4)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W636
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=5
      A(2)=5
      A(3)=6
      A(4)=6
      WRITE (6,GRP)
      END
      SUBROUTINE W637
      INTEGER*4 A(1:4)
      NAMELIST /GRP/ A
      A(1)=1
      A(2)=2
      A(3)=3
      A(4)=4
      WRITE (6,GRP)
      END
      SUBROUTINE W638
      INTEGER*4 A(1:2)
      INTEGER*4 I
      NAMELIST /GRP/ A, I
      A(1)=1
      A(2)=2
      I=3
      WRITE (6,GRP)
      END
      SUBROUTINE W639
      INTEGER*4 I
      INTEGER*4 A(1:2)
      NAMELIST /GRP/ I, A
      I=3
      A(1)=1
      A(2)=2
      WRITE (6,GRP)
      END
      SUBROUTINE W640
      INTEGER*4 B(1:2,1:2)
      NAMELIST /GRP/ B
      B(1,1)=1
      B(2,1)=2
      B(1,2)=3
      B(2,2)=4
      WRITE (6,GRP)
      END
      SUBROUTINE W641
      INTEGER*4 B(-1:1)
      NAMELIST /GRP/ B
      B(-1)=1
      B(0)=1
      B(1)=4
      WRITE (6,GRP)
      END
      SUBROUTINE W642
      INTEGER*2 A(1:16)
      NAMELIST /GRP/ A
      A(1)=1
      A(2)=2
      A(3)=3
      A(4)=4
      A(5)=5
      A(6)=6
      A(7)=7
      A(8)=8
      A(9)=9
      A(10)=10
      A(11)=11
      A(12)=12
      A(13)=13
      A(14)=14
      A(15)=15
      A(16)=16
      WRITE (6,GRP)
      END
      SUBROUTINE W643
      INTEGER*2 A(1:16)
      NAMELIST /GRP/ A
      A(1)=-7
      A(2)=-7
      A(3)=-7
      A(4)=-7
      A(5)=-7
      A(6)=-7
      A(7)=-7
      A(8)=-7
      A(9)=-7
      A(10)=-7
      A(11)=-7
      A(12)=-7
      A(13)=-7
      A(14)=-7
      A(15)=-7
      A(16)=-7
      WRITE (6,GRP)
      END
      SUBROUTINE W644
      LOGICAL*1 A(1:20)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      WRITE (6,GRP)
      END
      SUBROUTINE W645
      LOGICAL*4 A(1:30)
      NAMELIST /GRP/ A
      A(1)=.TRUE.
      A(2)=.FALSE.
      A(3)=.TRUE.
      A(4)=.FALSE.
      A(5)=.TRUE.
      A(6)=.FALSE.
      A(7)=.TRUE.
      A(8)=.FALSE.
      A(9)=.TRUE.
      A(10)=.FALSE.
      A(11)=.TRUE.
      A(12)=.FALSE.
      A(13)=.TRUE.
      A(14)=.FALSE.
      A(15)=.TRUE.
      A(16)=.FALSE.
      A(17)=.TRUE.
      A(18)=.FALSE.
      A(19)=.TRUE.
      A(20)=.FALSE.
      A(21)=.TRUE.
      A(22)=.FALSE.
      A(23)=.TRUE.
      A(24)=.FALSE.
      A(25)=.TRUE.
      A(26)=.FALSE.
      A(27)=.TRUE.
      A(28)=.FALSE.
      A(29)=.TRUE.
      A(30)=.FALSE.
      WRITE (6,GRP)
      END
      SUBROUTINE W646
      LOGICAL*2 A(1:3)
      LOGICAL*4 B(1:2)
      NAMELIST /GRP/ A, B
      A(1)=.TRUE.
      A(2)=.TRUE.
      A(3)=.TRUE.
      B(1)=.FALSE.
      B(2)=.FALSE.
      WRITE (6,GRP)
      END
      SUBROUTINE W647
      REAL*4 A(1:7)
      NAMELIST /GRP/ A
      A(1)=0.5
      A(2)=1.5
      A(3)=2.5
      A(4)=3.5
      A(5)=4.5
      A(6)=5.5
      A(7)=6.5
      WRITE (6,GRP)
      END
      SUBROUTINE W648
      REAL*4 A(1:6)
      NAMELIST /GRP/ A
      A(1)=1.0
      A(2)=1.0
      A(3)=2.0
      A(4)=2.0
      A(5)=2.0
      A(6)=3.0
      WRITE (6,GRP)
      END
      SUBROUTINE W649
      REAL*8 A(1:5)
      NAMELIST /GRP/ A
      A(1)=0.5D0
      A(2)=1.5D0
      A(3)=2.5D0
      A(4)=3.5D0
      A(5)=4.5D0
      WRITE (6,GRP)
      END
      SUBROUTINE W650
      REAL*8 A(1:5)
      NAMELIST /GRP/ A
      A(1)=1.0D0
      A(2)=1.0D0
      A(3)=2.0D0
      A(4)=2.0D0
      A(5)=2.0D0
      WRITE (6,GRP)
      END
      SUBROUTINE W651
      REAL*16 A(1:3)
      NAMELIST /GRP/ A
      A(1)=1.5Q0
      A(2)=2.5Q0
      A(3)=3.5Q0
      WRITE (6,GRP)
      END
      SUBROUTINE W652
      COMPLEX*8 A(1:5)
      NAMELIST /GRP/ A
      A(1)=(0.5,-0.5)
      A(2)=(1.5,-1.5)
      A(3)=(2.5,-2.5)
      A(4)=(3.5,-3.5)
      A(5)=(4.5,-4.5)
      WRITE (6,GRP)
      END
      SUBROUTINE W653
      COMPLEX*8 A(1:3)
      NAMELIST /GRP/ A
      A(1)=(1.0,2.0)
      A(2)=(1.0,2.0)
      A(3)=(3.0,4.0)
      WRITE (6,GRP)
      END
      SUBROUTINE W654
      COMPLEX*16 A(1:3)
      NAMELIST /GRP/ A
      A(1)=(1.5D0,2.5D0)
      A(2)=(3.5D0,4.5D0)
      A(3)=(5.5D0,6.5D0)
      WRITE (6,GRP)
      END
      SUBROUTINE W655
      CHARACTER*3 A(1:3)
      NAMELIST /GRP/ A
      A(1)='AAA'
      A(2)='BBB'
      A(3)='CCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W656
      CHARACTER*3 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAA'
      A(2)='AAA'
      A(3)='AAA'
      A(4)='CCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W657
      INTEGER*4 A(1:20)
      NAMELIST /GRP/ A
      A(1)=1
      A(2)=2
      A(3)=3
      A(4)=4
      A(5)=5
      A(6)=6
      A(7)=7
      A(8)=8
      A(9)=9
      A(10)=10
      A(11)=11
      A(12)=12
      A(13)=13
      A(14)=14
      A(15)=15
      A(16)=16
      A(17)=17
      A(18)=18
      A(19)=19
      A(20)=20
      WRITE (6,GRP)
      END
      SUBROUTINE W658
      INTEGER*4 A(1:9)
      NAMELIST /GRP/ A
      A(1)=1
      A(2)=2
      A(3)=3
      A(4)=4
      A(5)=5
      A(6)=6
      A(7)=6
      A(8)=6
      A(9)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W659
      CHARACTER*56 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W660
      CHARACTER*56 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W661
      CHARACTER*57 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W662
      CHARACTER*57 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W663
      CHARACTER*58 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W664
      CHARACTER*58 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W665
      CHARACTER*59 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W666
      CHARACTER*59 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W667
      CHARACTER*60 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W668
      CHARACTER*60 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W669
      CHARACTER*61 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W670
      CHARACTER*61 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W671
      CHARACTER*62 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W672
      CHARACTER*62 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W673
      CHARACTER*63 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &'
      WRITE (6,GRP)
      END
      SUBROUTINE W674
      CHARACTER*63 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W675
      CHARACTER*64 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &X'
      WRITE (6,GRP)
      END
      SUBROUTINE W676
      CHARACTER*64 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &X'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W677
      CHARACTER*65 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XX'
      WRITE (6,GRP)
      END
      SUBROUTINE W678
      CHARACTER*65 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W679
      CHARACTER*66 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W680
      CHARACTER*66 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W681
      CHARACTER*67 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W682
      CHARACTER*67 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W683
      CHARACTER*68 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W684
      CHARACTER*68 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W685
      CHARACTER*69 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W686
      CHARACTER*69 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W687
      CHARACTER*70 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W688
      CHARACTER*70 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W689
      CHARACTER*71 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W690
      CHARACTER*71 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W691
      CHARACTER*72 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W692
      CHARACTER*72 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W693
      CHARACTER*73 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W694
      CHARACTER*73 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W695
      CHARACTER*74 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W696
      CHARACTER*74 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W697
      CHARACTER*75 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W698
      CHARACTER*75 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W699
      CHARACTER*76 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W700
      CHARACTER*76 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W701
      CHARACTER*77 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W702
      CHARACTER*77 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W703
      CHARACTER*78 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W704
      CHARACTER*78 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W705
      CHARACTER*79 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W706
      CHARACTER*79 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W707
      CHARACTER*80 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W708
      CHARACTER*80 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W709
      CHARACTER*81 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W710
      CHARACTER*81 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W711
      CHARACTER*82 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W712
      CHARACTER*82 T
      CHARACTER*1 U
      NAMELIST /GRP/ T, U
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXX'
      U='U'
      WRITE (6,GRP)
      END
      SUBROUTINE W713
      CHARACTER*100 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W714
      CHARACTER*158 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W715
      CHARACTER*159 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W716
      CHARACTER*160 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W717
      CHARACTER*161 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W718
      CHARACTER*162 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W719
      CHARACTER*200 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W720
      CHARACTER*255 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W721
      CHARACTER*100 T
      NAMELIST /GRP/ T
      T=''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''
     &''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''
     &''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''''
     &''''''
      WRITE (6,GRP)
      END
      SUBROUTINE W722
      CHARACTER*80 T
      NAMELIST /GRP/ T
      T='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXX''YYYYYYYYYYYY'
      WRITE (6,GRP)
      END
      SUBROUTINE W723
      CHARACTER*10 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAA'
      A(2)='BBBBBBBBBB'
      A(3)='CCCCCCCCCC'
      A(4)='DDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W724
      CHARACTER*11 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAA'
      A(2)='BBBBBBBBBBB'
      A(3)='CCCCCCCCCCC'
      A(4)='DDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W725
      CHARACTER*12 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W726
      CHARACTER*13 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W727
      CHARACTER*14 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W728
      CHARACTER*15 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W729
      CHARACTER*16 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W730
      CHARACTER*17 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W731
      CHARACTER*18 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W732
      CHARACTER*19 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W733
      CHARACTER*20 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W734
      CHARACTER*21 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W735
      CHARACTER*22 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W736
      CHARACTER*23 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W737
      CHARACTER*24 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W738
      CHARACTER*25 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W739
      CHARACTER*26 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W740
      CHARACTER*27 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W741
      CHARACTER*28 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W742
      CHARACTER*29 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W743
      CHARACTER*30 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W744
      CHARACTER*31 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W745
      CHARACTER*32 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W746
      CHARACTER*33 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W747
      CHARACTER*34 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W748
      CHARACTER*35 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W749
      CHARACTER*36 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W750
      CHARACTER*37 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W751
      CHARACTER*38 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W752
      CHARACTER*39 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W753
      CHARACTER*40 A(1:4)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(3)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      A(4)='DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD'
      WRITE (6,GRP)
      END
      SUBROUTINE W754
      CHARACTER*30 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W755
      CHARACTER*31 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W756
      CHARACTER*32 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W757
      CHARACTER*33 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W758
      CHARACTER*34 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W759
      CHARACTER*35 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W760
      CHARACTER*36 A(1:5)
      NAMELIST /GRP/ A
      A(1)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(2)='AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
      A(3)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(4)='BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'
      A(5)='CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC'
      WRITE (6,GRP)
      END
      SUBROUTINE W761
      CHARACTER*100 A(1:2)
      NAMELIST /GRP/ A
      A(1)='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      A(2)='XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
     &XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX'
      WRITE (6,GRP)
      END
      SUBROUTINE W762
      INTEGER*4 NNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNN
      NNNNNNNNN(1)=1
      NNNNNNNNN(2)=2
      NNNNNNNNN(3)=3
      NNNNNNNNN(4)=4
      NNNNNNNNN(5)=5
      NNNNNNNNN(6)=6
      NNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W763
      REAL*4 NNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W764
      INTEGER*4 NNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNN
      NNNNNNNNNN(1)=1
      NNNNNNNNNN(2)=2
      NNNNNNNNNN(3)=3
      NNNNNNNNNN(4)=4
      NNNNNNNNNN(5)=5
      NNNNNNNNNN(6)=6
      NNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W765
      REAL*4 NNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W766
      INTEGER*4 NNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNN
      NNNNNNNNNNN(1)=1
      NNNNNNNNNNN(2)=2
      NNNNNNNNNNN(3)=3
      NNNNNNNNNNN(4)=4
      NNNNNNNNNNN(5)=5
      NNNNNNNNNNN(6)=6
      NNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W767
      REAL*4 NNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W768
      INTEGER*4 NNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNN
      NNNNNNNNNNNN(1)=1
      NNNNNNNNNNNN(2)=2
      NNNNNNNNNNNN(3)=3
      NNNNNNNNNNNN(4)=4
      NNNNNNNNNNNN(5)=5
      NNNNNNNNNNNN(6)=6
      NNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W769
      REAL*4 NNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W770
      INTEGER*4 NNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNN
      NNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W771
      REAL*4 NNNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W772
      INTEGER*4 NNNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNNN
      NNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W773
      REAL*4 NNNNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W774
      INTEGER*4 NNNNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W775
      REAL*4 NNNNNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W776
      INTEGER*4 NNNNNNNNNNNNNNNN(1:7)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNN(1)=1
      NNNNNNNNNNNNNNNN(2)=2
      NNNNNNNNNNNNNNNN(3)=3
      NNNNNNNNNNNNNNNN(4)=4
      NNNNNNNNNNNNNNNN(5)=5
      NNNNNNNNNNNNNNNN(6)=6
      NNNNNNNNNNNNNNNN(7)=7
      WRITE (6,GRP)
      END
      SUBROUTINE W777
      REAL*4 NNNNNNNNNNNNNNNNNNNNNNNN(1:5)
      NAMELIST /GRP/ NNNNNNNNNNNNNNNNNNNNNNNN
      NNNNNNNNNNNNNNNNNNNNNNNN(1)=0.5
      NNNNNNNNNNNNNNNNNNNNNNNN(2)=1.5
      NNNNNNNNNNNNNNNNNNNNNNNN(3)=2.5
      NNNNNNNNNNNNNNNNNNNNNNNN(4)=3.5
      NNNNNNNNNNNNNNNNNNNNNNNN(5)=4.5
      WRITE (6,GRP)
      END
      SUBROUTINE W778
      REAL*4 A(1:9)
      NAMELIST /GRP/ A
      A(1)=1.0
      A(2)=2.0
      A(3)=3.0
      A(4)=4.0
      A(5)=5.0
      A(6)=5.0
      A(7)=6.0
      A(8)=6.0
      A(9)=7.0
      WRITE (6,GRP)
      END
      SUBROUTINE W779
      COMPLEX*8 A(1:8)
      NAMELIST /GRP/ A
      A(1)=(0.5,-0.5)
      A(2)=(1.5,-1.5)
      A(3)=(2.5,-2.5)
      A(4)=(3.5,-3.5)
      A(5)=(4.5,-4.5)
      A(6)=(5.5,-5.5)
      A(7)=(6.5,-6.5)
      A(8)=(7.5,-7.5)
      WRITE (6,GRP)
      END
      SUBROUTINE W780
      COMPLEX*16 A(1:8)
      NAMELIST /GRP/ A
      A(1)=(0.5D0,-0.5D0)
      A(2)=(1.5D0,-1.5D0)
      A(3)=(2.5D0,-2.5D0)
      A(4)=(3.5D0,-3.5D0)
      A(5)=(4.5D0,-4.5D0)
      A(6)=(5.5D0,-5.5D0)
      A(7)=(6.5D0,-6.5D0)
      A(8)=(7.5D0,-7.5D0)
      WRITE (6,GRP)
      END
      SUBROUTINE W781
      COMPLEX*16 A(1:2)
      NAMELIST /GRP/ A
      A(1)=(1.0D30,-1.0D30)
      A(2)=(1.0D30,-1.0D30)
      WRITE (6,GRP)
      END
      OPTIONS /G_FLOATING
      SUBROUTINE W782
      REAL*8 G
      COMPLEX*16 H
      NAMELIST /GRP/ G, H
      G=2.5D0
      H=(1.5D0,-2.5D0)
      WRITE (6,GRP)
      END
      OPTIONS /G_FLOATING
      SUBROUTINE W783
      REAL*8 A(1:5)
      NAMELIST /GRP/ A
      A(1)=1.5D0
      A(2)=1.5D0
      A(3)=2.5D0
      A(4)=3.5D0
      A(5)=4.5D0
      WRITE (6,GRP)
      END
