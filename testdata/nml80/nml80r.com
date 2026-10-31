$ ! NML80R.COM - runs an image with the records the NAMELIST probe reads.
$ ! Written by testdata/nml80/gen.go. Each case's lines are under a line
$ ! holding its number, which the program reads up to.
$ RUN 'P1'
$ DECK
#0
 $GRP I=7 $END
 $GRP $END
 '
 $END
#1
 &GRP I=7 &END
 $GRP $END
 '
 $END
#2
 $grp i=7 $end
 $GRP $END
 '
 $END
#3
$GRP I=7 $END
 $GRP $END
 '
 $END
#4
X$GRP I=7 $END
 $GRP $END
 '
 $END
#5
  $GRP I=7 $END
 $GRP $END
 '
 $END
#6
     $GRP I=7 $END
 $GRP $END
 '
 $END
#7
	$GRP I=7 $END
 $GRP $END
 '
 $END
#8
 X $GRP I=7 $END
 $GRP $END
 '
 $END
#9
 I=7 $GRP I=8 $END
 $GRP $END
 '
 $END
#10
 IGNORED
 $GRP I=7 $END
 $GRP $END
 '
 $END
#11
 $OTHER I=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#12
 $GRPX I=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#13
 $GR I=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#14
 $ GRP I=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#15
 $OTHER $GRP I=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#16
 $GRP,I=7 $END
 $GRP $END
 '
 $END
#17
 $GRP=7 $END
 $GRP $END
 '
 $END
#18
 $GRPI=7 $END
 $GRP I=8 $END
 $GRP $END
 '
 $END
#19
 $GRP
 $GRP $END
 '
 $END
#20
 $GRP
 I=7
 $END
 $GRP $END
 '
 $END
#21
 $GRP

 I=7

 $END
 $GRP $END
 '
 $END
#22

 $GRP $END
 '
 $END
#23
 $
 $GRP $END
 '
 $END
#24
 &
 $GRP $END
 '
 $END
#25
 $GRP I=7 $
 $GRP $END
 '
 $END
#26
 $GRP I=7 &
 $GRP $END
 '
 $END
#27
 $GRP I=7 &END
 $GRP $END
 '
 $END
#28
 &GRP I=7 $END
 $GRP $END
 '
 $END
#29
 $GRP I=7$END
 $GRP $END
 '
 $END
#30
 $GRP I=7 $E
 $GRP $END
 '
 $END
#31
 $GRP I=7 $ENDX
 $GRP $END
 '
 $END
#32
 $GRP I=7 $OTHER
 $GRP $END
 '
 $END
#33
 $GRP I=7 $END R=1
 $GRP $END
 '
 $END
#34
 $GRP I=7 $END
 R=1
 $GRP $END
 '
 $END
#35
 $GRP $END
 $GRP $END
 '
 $END
#36
 $GRP $
 $GRP $END
 '
 $END
#37
 $GRP I=7 $ R=9
 $GRP $END
 '
 $END
#38
 $GRP I=7 /
 $GRP $END
 '
 $END
#39
 $GRP / I=7 $END
 $GRP $END
 '
 $END
#40
 $GRP I=7, $END
 $GRP $END
 '
 $END
#41
 $GRP T='A$B' $END
 $GRP $END
 '
 $END
#42
 $GRP I=$END
 $GRP $END
 '
 $END
#43
 $GRP I= $END
 $GRP $END
 '
 $END
#44
 $GRP R=2.5$END
 $GRP $END
 '
 $END
#45
 $GRP A=1,2$END
 $GRP $END
 '
 $END
#46
 $GRP L=T$END
 $GRP $END
 '
 $END
#47
 $GRP C=(1,2)$END
 $GRP $END
 '
 $END
#48
 $GRP T='X'$END
 $GRP $END
 '
 $END
#49
 $GRP I=7 $END
 $GRP R=1 $END
 $GRP $END
 '
 $END
#50
 $GRP I=7 $END $GRP R=1 $END
 $GRP J=9 $END
 $GRP $END
 '
 $END
#51
 $GRP I = 7 $END
 $GRP $END
 '
 $END
#52
 $GRP I =7 $END
 $GRP $END
 '
 $END
#53
 $GRP I= 7 $END
 $GRP $END
 '
 $END
#54
 $GRP	I	=	7	$END
 $GRP $END
 '
 $END
#55
 $GRP I=7 R=1 $END
 $GRP $END
 '
 $END
#56
 $GRP I=7,R=1 $END
 $GRP $END
 '
 $END
#57
 $GRP I=7 , R=1 $END
 $GRP $END
 '
 $END
#58
 $GRP I=7,
 R=1 $END
 $GRP $END
 '
 $END
#59
 $GRP I=7
 ,R=1 $END
 $GRP $END
 '
 $END
#60
 $GRP I=7,,R=1 $END
 $GRP $END
 '
 $END
#61
 $GRP ,I=7 $END
 $GRP $END
 '
 $END
#62
 $GRP I=7 ,, $END
 $GRP $END
 '
 $END
#63
 $GRP I=
 7 $END
 $GRP $END
 '
 $END
#64
 $GRP I
 =7 $END
 $GRP $END
 '
 $END
#65
 $GRP I=7 I=8 $END
 $GRP $END
 '
 $END
#66
 $GRP A(1)=7 A(1)=8 $END
 $GRP $END
 '
 $END
#67
 $GRP I=7,8 $END
 $GRP $END
 '
 $END
#68
 $GRP I=7 8 $END
 $GRP $END
 '
 $END
#69
 $GRP I=2*7 $END
 $GRP $END
 '
 $END
#70
 $GRP I=1*7 $END
 $GRP $END
 '
 $END
#71
 $GRP I=0*7 $END
 $GRP $END
 '
 $END
#72
 $GRP I=1* $END
 $GRP $END
 '
 $END
#73
 $GRP I=2* $END
 $GRP $END
 '
 $END
#74
 $GRP I=, $END
 $GRP $END
 '
 $END
#75
 $GRP I=,, $END
 $GRP $END
 '
 $END
#76
 $GRP I=,8 $END
 $GRP $END
 '
 $END
#77
 $GRP I=,,R=1 $END
 $GRP $END
 '
 $END
#78
 $GRP I=, R=1 $END
 $GRP $END
 '
 $END
#79
 $GRP I=7, 8, R=1 $END
 $GRP $END
 '
 $END
#80
 $GRP R=1,2 $END
 $GRP $END
 '
 $END
#81
 $GRP T='A','B' $END
 $GRP $END
 '
 $END
#82
 $GRP L=T,F $END
 $GRP $END
 '
 $END
#83
 $GRP L=T F $END
 $GRP $END
 '
 $END
#84
 $GRP C=(1,2),(3,4) $END
 $GRP $END
 '
 $END
#85
 $GRP i=7 $END
 $GRP $END
 '
 $END
#86
 $GRP NOPE=1 $END
 $GRP $END
 '
 $END
#87
 $GRP NOPE $END
 $GRP $END
 '
 $END
#88
 $GRP LONGNAME1=7 $END
 $GRP $END
 '
 $END
#89
 $GRP LONGNAME=7 $END
 $GRP $END
 '
 $END
#90
 $GRP LONGNAME12=7 $END
 $GRP $END
 '
 $END
#91
 $GRP LONG NAME1=7 $END
 $GRP $END
 '
 $END
#92
 $GRP GRP=7 $END
 $GRP $END
 '
 $END
#93
 $GRP I 7 $END
 $GRP $END
 '
 $END
#94
 $GRP I $END
 $GRP $END
 '
 $END
#95
 $GRP =7 $END
 $GRP $END
 '
 $END
#96
 $GRP 7 $END
 $GRP $END
 '
 $END
#97
 $GRP 7=1 $END
 $GRP $END
 '
 $END
#98
 $GRP 'I'=7 $END
 $GRP $END
 '
 $END
#99
 $GRP I==7 $END
 $GRP $END
 '
 $END
#100
 $GRP I=7 8=9 $END
 $GRP $END
 '
 $END
#101
 $GRP I=7 R $END
 $GRP $END
 '
 $END
#102
 $GRP I=7 NOPE=1 $END
 $GRP $END
 '
 $END
#103
 $GRP I=7 NOPE $END
 $GRP $END
 '
 $END
#104
 $GRP A=1 2 I=3 $END
 $GRP $END
 '
 $END
#105
 $GRP A=1 2 NOPE=3 $END
 $GRP $END
 '
 $END
#106
 $GRP A=1 2 NOPE $END
 $GRP $END
 '
 $END
#107
 $GRP LA=T F I=3 $END
 $GRP $END
 '
 $END
#108
 $GRP LA=T I=3 $END
 $GRP $END
 '
 $END
#109
 $GRP LA=T L=F $END
 $GRP $END
 '
 $END
#110
 $GRP LA=T,L=F $END
 $GRP $END
 '
 $END
#111
 $GRP LA=F T=1 $END
 $GRP $END
 '
 $END
#112
 $GRP LA=T(1)='X' $END
 $GRP $END
 '
 $END
#113
 $GRP A=5,6,7 $END
 $GRP $END
 '
 $END
#114
 $GRP A=5 6 7 $END
 $GRP $END
 '
 $END
#115
 $GRP A=5 , 6 , 7 $END
 $GRP $END
 '
 $END
#116
 $GRP A=5,
 6,
 7 $END
 $GRP $END
 '
 $END
#117
 $GRP A=5,6 $END
 $GRP $END
 '
 $END
#118
 $GRP A=5,6,7,8 $END
 $GRP $END
 '
 $END
#119
 $GRP A=5,6,7,8,9 $END
 $GRP $END
 '
 $END
#120
 $GRP A=5,,7 $END
 $GRP $END
 '
 $END
#121
 $GRP A=,6 $END
 $GRP $END
 '
 $END
#122
 $GRP A=,,7 $END
 $GRP $END
 '
 $END
#123
 $GRP A=5,6, $END
 $GRP $END
 '
 $END
#124
 $GRP A=5,6,7, $END
 $GRP $END
 '
 $END
#125
 $GRP A=5,6,7,, $END
 $GRP $END
 '
 $END
#126
 $GRP A=,,,8 $END
 $GRP $END
 '
 $END
#127
 $GRP A=3*5 $END
 $GRP $END
 '
 $END
#128
 $GRP A=2*5 $END
 $GRP $END
 '
 $END
#129
 $GRP A=4*5 $END
 $GRP $END
 '
 $END
#130
 $GRP A=2*5,6 $END
 $GRP $END
 '
 $END
#131
 $GRP A=2*5,6,7 $END
 $GRP $END
 '
 $END
#132
 $GRP A=1*5,6 $END
 $GRP $END
 '
 $END
#133
 $GRP A=0*5,6 $END
 $GRP $END
 '
 $END
#134
 $GRP A=2*,6 $END
 $GRP $END
 '
 $END
#135
 $GRP A=2* 6 $END
 $GRP $END
 '
 $END
#136
 $GRP A=3* $END
 $GRP $END
 '
 $END
#137
 $GRP A=4* $END
 $GRP $END
 '
 $END
#138
 $GRP A=2 * 5 $END
 $GRP $END
 '
 $END
#139
 $GRP A=2* 5,6 $END
 $GRP $END
 '
 $END
#140
 $GRP A=5,6,7 R=1 $END
 $GRP $END
 '
 $END
#141
 $GRP A=5,6,7,R=1 $END
 $GRP $END
 '
 $END
#142
 $GRP A=5,6,R=1 $END
 $GRP $END
 '
 $END
#143
 $GRP A=5,6,,R=1 $END
 $GRP $END
 '
 $END
#144
 $GRP RA=1.5,2.5 $END
 $GRP $END
 '
 $END
#145
 $GRP RA=2*1.5 $END
 $GRP $END
 '
 $END
#146
 $GRP LA=T,T $END
 $GRP $END
 '
 $END
#147
 $GRP LA=2*F $END
 $GRP $END
 '
 $END
#148
 $GRP TA='X','Y','Z' $END
 $GRP $END
 '
 $END
#149
 $GRP TA=2*'QQ' $END
 $GRP $END
 '
 $END
#150
 $GRP TA='A','B','C','D' $END
 $GRP $END
 '
 $END
#151
 $GRP CA=(5,6),(7,8) $END
 $GRP $END
 '
 $END
#152
 $GRP CA=2*(5,6) $END
 $GRP $END
 '
 $END
#153
 $GRP CA=(5,6) (7,8) $END
 $GRP $END
 '
 $END
#154
 $GRP CA=,(7,8) $END
 $GRP $END
 '
 $END
#155
 $GRP CA=1,2 $END
 $GRP $END
 '
 $END
#156
 $GRP B=5,6,7,8 $END
 $GRP $END
 '
 $END
#157
 $GRP B=5,6 $END
 $GRP $END
 '
 $END
#158
 $GRP B=4*5 $END
 $GRP $END
 '
 $END
#159
 $GRP B=5*5 $END
 $GRP $END
 '
 $END
#160
 $GRP AL=5,6,7 $END
 $GRP $END
 '
 $END
#161
 $GRP A(2)=5 $END
 $GRP $END
 '
 $END
#162
 $GRP A(1)=5 $END
 $GRP $END
 '
 $END
#163
 $GRP A(3)=5 $END
 $GRP $END
 '
 $END
#164
 $GRP A(0)=5 $END
 $GRP $END
 '
 $END
#165
 $GRP A(4)=5 $END
 $GRP $END
 '
 $END
#166
 $GRP A(-1)=5 $END
 $GRP $END
 '
 $END
#167
 $GRP A(+2)=5 $END
 $GRP $END
 '
 $END
#168
 $GRP A( 2 )=5 $END
 $GRP $END
 '
 $END
#169
 $GRP A (2)=5 $END
 $GRP $END
 '
 $END
#170
 $GRP A(2) =5 $END
 $GRP $END
 '
 $END
#171
 $GRP A(2)= 5 $END
 $GRP $END
 '
 $END
#172
 $GRP A(2)=5,6 $END
 $GRP $END
 '
 $END
#173
 $GRP A(2)=5,6,7 $END
 $GRP $END
 '
 $END
#174
 $GRP A(2)=2*5 $END
 $GRP $END
 '
 $END
#175
 $GRP A(2)=3*5 $END
 $GRP $END
 '
 $END
#176
 $GRP A(2)=,6 $END
 $GRP $END
 '
 $END
#177
 $GRP A(2)=,,7 $END
 $GRP $END
 '
 $END
#178
 $GRP A(3)=5,6 $END
 $GRP $END
 '
 $END
#179
 $GRP A(1,1)=5 $END
 $GRP $END
 '
 $END
#180
 $GRP A()=5 $END
 $GRP $END
 '
 $END
#181
 $GRP A(2=5 $END
 $GRP $END
 '
 $END
#182
 $GRP A(2 $END
 $GRP $END
 '
 $END
#183
 $GRP A(I)=5 $END
 $GRP $END
 '
 $END
#184
 $GRP A(1.5)=5 $END
 $GRP $END
 '
 $END
#185
 $GRP A(1+1)=5 $END
 $GRP $END
 '
 $END
#186
 $GRP A(99999999999)=5 $END
 $GRP $END
 '
 $END
#187
 $GRP A(2)(1:1)=5 $END
 $GRP $END
 '
 $END
#188
 $GRP A(1:2)=5 $END
 $GRP $END
 '
 $END
#189
 $GRP A(2
 )=5 $END
 $GRP $END
 '
 $END
#190
 $GRP B(1,2)=5 $END
 $GRP $END
 '
 $END
#191
 $GRP B(2,1)=5 $END
 $GRP $END
 '
 $END
#192
 $GRP B(2,2)=5 $END
 $GRP $END
 '
 $END
#193
 $GRP B( 1 , 2 )=5 $END
 $GRP $END
 '
 $END
#194
 $GRP B(2)=5 $END
 $GRP $END
 '
 $END
#195
 $GRP B(3,1)=5 $END
 $GRP $END
 '
 $END
#196
 $GRP B(1,3)=5 $END
 $GRP $END
 '
 $END
#197
 $GRP B(0,1)=5 $END
 $GRP $END
 '
 $END
#198
 $GRP B(1,2,1)=5 $END
 $GRP $END
 '
 $END
#199
 $GRP B(2,1)=5,6,7 $END
 $GRP $END
 '
 $END
#200
 $GRP B(2,1)=5,6,7,8 $END
 $GRP $END
 '
 $END
#201
 $GRP B(1,)=5 $END
 $GRP $END
 '
 $END
#202
 $GRP B(,1)=5 $END
 $GRP $END
 '
 $END
#203
 $GRP AL(-1)=5 $END
 $GRP $END
 '
 $END
#204
 $GRP AL(0)=5 $END
 $GRP $END
 '
 $END
#205
 $GRP AL(0)=5,6 $END
 $GRP $END
 '
 $END
#206
 $GRP AL(0)=5,6,7 $END
 $GRP $END
 '
 $END
#207
 $GRP AL(1)=5 $END
 $GRP $END
 '
 $END
#208
 $GRP AL(-2)=5 $END
 $GRP $END
 '
 $END
#209
 $GRP AL(2)=5 $END
 $GRP $END
 '
 $END
#210
 $GRP I(1)=5 $END
 $GRP $END
 '
 $END
#211
 $GRP R(1)=5 $END
 $GRP $END
 '
 $END
#212
 $GRP CA(2)=(5,6) $END
 $GRP $END
 '
 $END
#213
 $GRP LA(2)=T $END
 $GRP $END
 '
 $END
#214
 $GRP RA(2)=1.5,2.5 $END
 $GRP $END
 '
 $END
#215
 $GRP T='ABC' $END
 $GRP $END
 '
 $END
#216
 $GRP T='' $END
 $GRP $END
 '
 $END
#217
 $GRP T=' ' $END
 $GRP $END
 '
 $END
#218
 $GRP T='ABCDE' $END
 $GRP $END
 '
 $END
#219
 $GRP T='ABCDEFGH' $END
 $GRP $END
 '
 $END
#220
 $GRP T='A''B' $END
 $GRP $END
 '
 $END
#221
 $GRP T='''' $END
 $GRP $END
 '
 $END
#222
 $GRP T='AB' 'CD' $END
 $GRP $END
 '
 $END
#223
 $GRP T='AB'X $END
 $GRP $END
 '
 $END
#224
 $GRP T='AB'R=1 $END
 $GRP $END
 '
 $END
#225
 $GRP T='AB',R=1 $END
 $GRP $END
 '
 $END
#226
 $GRP T=ABC $END
 $GRP $END
 '
 $END
#227
 $GRP T=7 $END
 $GRP $END
 '
 $END
#228
 $GRP T="ABC" $END
 $GRP $END
 '
 $END
#229
 $GRP T=.TRUE. $END
 $GRP $END
 '
 $END
#230
 $GRP T=(1,2) $END
 $GRP $END
 '
 $END
#231
 $GRP T='AB
 CD' $END
 $GRP $END
 '
 $END
#232
 $GRP T='AB
CD' $END
 $GRP $END
 '
 $END
#233
 $GRP T='AB
 
 CD' $END
 $GRP $END
 '
 $END
#234
 $GRP T='AB
 $GRP $END
 '
 $END
#235
 $GRP T(2:3)='ZZ' $END
 $GRP $END
 '
 $END
#236
 $GRP T(2:)='ZZ' $END
 $GRP $END
 '
 $END
#237
 $GRP T(:2)='ZZ' $END
 $GRP $END
 '
 $END
#238
 $GRP T(:)='ZZ' $END
 $GRP $END
 '
 $END
#239
 $GRP T(0:2)='ZZ' $END
 $GRP $END
 '
 $END
#240
 $GRP T(2:9)='ZZ' $END
 $GRP $END
 '
 $END
#241
 $GRP T(3:2)='ZZ' $END
 $GRP $END
 '
 $END
#242
 $GRP T(2:2)='ZZ' $END
 $GRP $END
 '
 $END
#243
 $GRP T( 2 : 3 )='ZZ' $END
 $GRP $END
 '
 $END
#244
 $GRP T (2:3)='ZZ' $END
 $GRP $END
 '
 $END
#245
 $GRP T(2:3) ='ZZ' $END
 $GRP $END
 '
 $END
#246
 $GRP T(2)='ZZ' $END
 $GRP $END
 '
 $END
#247
 $GRP T(2:3)='A','B' $END
 $GRP $END
 '
 $END
#248
 $GRP T(2:3)='VERYLONG' $END
 $GRP $END
 '
 $END
#249
 $GRP TA(2)='X' $END
 $GRP $END
 '
 $END
#250
 $GRP TA(2)='X','Y' $END
 $GRP $END
 '
 $END
#251
 $GRP TA(2)='X','Y','Z' $END
 $GRP $END
 '
 $END
#252
 $GRP TA(2)(2:)='Z' $END
 $GRP $END
 '
 $END
#253
 $GRP TA(2)(2:3)='Y','Z' $END
 $GRP $END
 '
 $END
#254
 $GRP TA(2)(1:1)='Y','Z' $END
 $GRP $END
 '
 $END
#255
 $GRP TA(2) (2:)='Z' $END
 $GRP $END
 '
 $END
#256
 $GRP TA(4)='X' $END
 $GRP $END
 '
 $END
#257
 $GRP TA(2:3)='X' $END
 $GRP $END
 '
 $END
#258
 $GRP TA(2)(5:6)='X' $END
 $GRP $END
 '
 $END
#259
 $GRP I(1:2)=5 $END
 $GRP $END
 '
 $END
#260
 $GRP R(1:2)=5 $END
 $GRP $END
 '
 $END
#261
 $GRP I=-3 $END
 $GRP $END
 '
 $END
#262
 $GRP I=+3 $END
 $GRP $END
 '
 $END
#263
 $GRP I=1.5 $END
 $GRP $END
 '
 $END
#264
 $GRP I=14.6 $END
 $GRP $END
 '
 $END
#265
 $GRP I=1E2 $END
 $GRP $END
 '
 $END
#266
 $GRP I=1.5E3 $END
 $GRP $END
 '
 $END
#267
 $GRP I=1D2 $END
 $GRP $END
 '
 $END
#268
 $GRP I=1Q2 $END
 $GRP $END
 '
 $END
#269
 $GRP I=.5 $END
 $GRP $END
 '
 $END
#270
 $GRP I=5. $END
 $GRP $END
 '
 $END
#271
 $GRP I=-.5 $END
 $GRP $END
 '
 $END
#272
 $GRP I=1E $END
 $GRP $END
 '
 $END
#273
 $GRP I=E2 $END
 $GRP $END
 '
 $END
#274
 $GRP I=1.2.3 $END
 $GRP $END
 '
 $END
#275
 $GRP I=2147483647 $END
 $GRP $END
 '
 $END
#276
 $GRP I=2147483648 $END
 $GRP $END
 '
 $END
#277
 $GRP I=-2147483648 $END
 $GRP $END
 '
 $END
#278
 $GRP I=T $END
 $GRP $END
 '
 $END
#279
 $GRP I=F $END
 $GRP $END
 '
 $END
#280
 $GRP I=.TRUE. $END
 $GRP $END
 '
 $END
#281
 $GRP I=.FALSE. $END
 $GRP $END
 '
 $END
#282
 $GRP I='A' $END
 $GRP $END
 '
 $END
#283
 $GRP I=(1,2) $END
 $GRP $END
 '
 $END
#284
 $GRP I=^X10 $END
 $GRP $END
 '
 $END
#285
 $GRP I="17 $END
 $GRP $END
 '
 $END
#286
 $GRP I='10'X $END
 $GRP $END
 '
 $END
#287
 $GRP I=3HABC $END
 $GRP $END
 '
 $END
#288
 $GRP I=7ABC $END
 $GRP $END
 '
 $END
#289
 $GRP I=+ $END
 $GRP $END
 '
 $END
#290
 $GRP I=- $END
 $GRP $END
 '
 $END
#291
 $GRP I=. $END
 $GRP $END
 '
 $END
#292
 $GRP J=70000 $END
 $GRP $END
 '
 $END
#293
 $GRP J=-32768 $END
 $GRP $END
 '
 $END
#294
 $GRP J=32768 $END
 $GRP $END
 '
 $END
#295
 $GRP J=1.5 $END
 $GRP $END
 '
 $END
#296
 $GRP R=-3 $END
 $GRP $END
 '
 $END
#297
 $GRP R=1 $END
 $GRP $END
 '
 $END
#298
 $GRP R=1.5 $END
 $GRP $END
 '
 $END
#299
 $GRP R=1.5E2 $END
 $GRP $END
 '
 $END
#300
 $GRP R=1.5D2 $END
 $GRP $END
 '
 $END
#301
 $GRP R=1.5Q2 $END
 $GRP $END
 '
 $END
#302
 $GRP R=.5 $END
 $GRP $END
 '
 $END
#303
 $GRP R=1E40 $END
 $GRP $END
 '
 $END
#304
 $GRP R=1E-40 $END
 $GRP $END
 '
 $END
#305
 $GRP R=T $END
 $GRP $END
 '
 $END
#306
 $GRP R=.TRUE. $END
 $GRP $END
 '
 $END
#307
 $GRP R='A' $END
 $GRP $END
 '
 $END
#308
 $GRP R=(1,2) $END
 $GRP $END
 '
 $END
#309
 $GRP R=ABC $END
 $GRP $END
 '
 $END
#310
 $GRP R=1.2.3 $END
 $GRP $END
 '
 $END
#311
 $GRP D=1 $END
 $GRP $END
 '
 $END
#312
 $GRP D=0.1 $END
 $GRP $END
 '
 $END
#313
 $GRP D=1.5D0 $END
 $GRP $END
 '
 $END
#314
 $GRP D=0.1D0 $END
 $GRP $END
 '
 $END
#315
 $GRP D=1D40 $END
 $GRP $END
 '
 $END
#316
 $GRP D=1E40 $END
 $GRP $END
 '
 $END
#317
 $GRP D=1D-40 $END
 $GRP $END
 '
 $END
#318
 $GRP D=T $END
 $GRP $END
 '
 $END
#319
 $GRP Q=1 $END
 $GRP $END
 '
 $END
#320
 $GRP Q=0.1 $END
 $GRP $END
 '
 $END
#321
 $GRP Q=1.5Q0 $END
 $GRP $END
 '
 $END
#322
 $GRP Q=0.1Q0 $END
 $GRP $END
 '
 $END
#323
 $GRP Q=1Q4000 $END
 $GRP $END
 '
 $END
#324
 $GRP Q=1Q5000 $END
 $GRP $END
 '
 $END
#325
 $GRP L=T $END
 $GRP $END
 '
 $END
#326
 $GRP L=F $END
 $GRP $END
 '
 $END
#327
 $GRP L=.T $END
 $GRP $END
 '
 $END
#328
 $GRP L=.F $END
 $GRP $END
 '
 $END
#329
 $GRP L=.T. $END
 $GRP $END
 '
 $END
#330
 $GRP L=.F. $END
 $GRP $END
 '
 $END
#331
 $GRP L=.TRUE. $END
 $GRP $END
 '
 $END
#332
 $GRP L=.FALSE. $END
 $GRP $END
 '
 $END
#333
 $GRP L=TRUE $END
 $GRP $END
 '
 $END
#334
 $GRP L=FALSE $END
 $GRP $END
 '
 $END
#335
 $GRP L=t $END
 $GRP $END
 '
 $END
#336
 $GRP L=f $END
 $GRP $END
 '
 $END
#337
 $GRP L=.true. $END
 $GRP $END
 '
 $END
#338
 $GRP L=TX $END
 $GRP $END
 '
 $END
#339
 $GRP L=FX $END
 $GRP $END
 '
 $END
#340
 $GRP L=.X $END
 $GRP $END
 '
 $END
#341
 $GRP L=. $END
 $GRP $END
 '
 $END
#342
 $GRP L=1 $END
 $GRP $END
 '
 $END
#343
 $GRP L=0 $END
 $GRP $END
 '
 $END
#344
 $GRP L=-1 $END
 $GRP $END
 '
 $END
#345
 $GRP L=2 $END
 $GRP $END
 '
 $END
#346
 $GRP L=1.5 $END
 $GRP $END
 '
 $END
#347
 $GRP L='T' $END
 $GRP $END
 '
 $END
#348
 $GRP L=(1,2) $END
 $GRP $END
 '
 $END
#349
 $GRP L=X $END
 $GRP $END
 '
 $END
#350
 $GRP L=ABC $END
 $GRP $END
 '
 $END
#351
 $GRP L=TT $END
 $GRP $END
 '
 $END
#352
 $GRP M=T $END
 $GRP $END
 '
 $END
#353
 $GRP N=T $END
 $GRP $END
 '
 $END
#354
 $GRP M=F $END
 $GRP $END
 '
 $END
#355
 $GRP N=F $END
 $GRP $END
 '
 $END
#356
 $GRP M=1 $END
 $GRP $END
 '
 $END
#357
 $GRP N=1 $END
 $GRP $END
 '
 $END
#358
 $GRP M=300 $END
 $GRP $END
 '
 $END
#359
 $GRP N=300 $END
 $GRP $END
 '
 $END
#360
 $GRP C=(5,6) $END
 $GRP $END
 '
 $END
#361
 $GRP C=( 5 , 6 ) $END
 $GRP $END
 '
 $END
#362
 $GRP C=(5.5,-6.5) $END
 $GRP $END
 '
 $END
#363
 $GRP C=(1E2,1D2) $END
 $GRP $END
 '
 $END
#364
 $GRP C=5 $END
 $GRP $END
 '
 $END
#365
 $GRP C=5.5 $END
 $GRP $END
 '
 $END
#366
 $GRP C=(5) $END
 $GRP $END
 '
 $END
#367
 $GRP C=(5,) $END
 $GRP $END
 '
 $END
#368
 $GRP C=(,6) $END
 $GRP $END
 '
 $END
#369
 $GRP C=(,) $END
 $GRP $END
 '
 $END
#370
 $GRP C=() $END
 $GRP $END
 '
 $END
#371
 $GRP C=(5,6,7) $END
 $GRP $END
 '
 $END
#372
 $GRP C=(5 6) $END
 $GRP $END
 '
 $END
#373
 $GRP C=(5,6 $END
 $GRP $END
 '
 $END
#374
 $GRP C=(A,B) $END
 $GRP $END
 '
 $END
#375
 $GRP C=T $END
 $GRP $END
 '
 $END
#376
 $GRP C='A' $END
 $GRP $END
 '
 $END
#377
 $GRP C=, $END
 $GRP $END
 '
 $END
#378
 $GRP C=2*(5,6) $END
 $GRP $END
 '
 $END
#379
 $GRP C=1*(5,6) $END
 $GRP $END
 '
 $END
#380
 $GRP C=(5,6)X $END
 $GRP $END
 '
 $END
#381
 $GRP C=(5,
 6) $END
 $GRP $END
 '
 $END
#382
 $GRP C=(
 5,6) $END
 $GRP $END
 '
 $END
#383
 $GRP C=(5
 ,6) $END
 $GRP $END
 '
 $END
#384
 $GRP C=(5,6
 ) $END
 $GRP $END
 '
 $END
#385
 $GRP E=(5.5D0,-6.5D0) $END
 $GRP $END
 '
 $END
#386
 $GRP E=(0.1,0.2) $END
 $GRP $END
 '
 $END
#387
 $GRP E=0.1 $END
 $GRP $END
 '
 $END
#388
 ?
 $GRP $END
 '
 $END
#389
 =?
 $GRP $END
 '
 $END
#390
?
 $GRP $END
 '
 $END
#391
=?
 $GRP $END
 '
 $END
#392
 ? $GRP I=7 $END
 $GRP $END
 '
 $END
#393
 ?
 $GRP I=7 $END
 $GRP $END
 '
 $END
#394
 =?
 $GRP I=7 $END
 $GRP $END
 '
 $END
#395
 $GRP ? $END
 $GRP $END
 '
 $END
#396
 $GRP I=7 ? $END
 $GRP $END
 '
 $END
#397
 $GRP I=? $END
 $GRP $END
 '
 $END
#398
 $GRP =? $END
 $GRP $END
 '
 $END
#399
 $GRP G=1.5 $END
 $GRP $END
 '
 $END
#400
 $GRP G=0.1 $END
 $GRP $END
 '
 $END
#401
 $GRP G=0.1D0 $END
 $GRP $END
 '
 $END
#402
 $GRP G=1D300 $END
 $GRP $END
 '
 $END
#403
 $GRP H=(0.1,0.2) $END
 $GRP $END
 '
 $END
#404
 $GRP H=0.1 $END
 $GRP $END
 '
 $END
#405
 $GRP GA=2*1.5 $END
 $GRP $END
 '
 $END
#406
 $GRP A=1 8=9 $END
 $GRP $END
 '
 $END
#407
 $GRP A=8=9 $END
 $GRP $END
 '
 $END
#408
 $GRP I=8=9 $END
 $GRP $END
 '
 $END
#409
 $GRP A=1 'X'=9 $END
 $GRP $END
 '
 $END
#410
 $GRP A=(1,2)=9 $END
 $GRP $END
 '
 $END
#411
 $GRP I=1.5=2 $END
 $GRP $END
 '
 $END
#412
 $GRP A=1(2) $END
 $GRP $END
 '
 $END
#413
 $GRP I=7) $END
 $GRP $END
 '
 $END
#414
 $GRP I=7'A' $END
 $GRP $END
 '
 $END
#415
 $GRP I=7/ $END
 $GRP $END
 '
 $END
#416
 $GRP I=7&END $END
 $GRP $END
 '
 $END
#417
 $GRP I=7* $END
 $GRP $END
 '
 $END
#418
 $GRP A=2*7* $END
 $GRP $END
 '
 $END
#419
 $GRP I=1E+ $END
 $GRP $END
 '
 $END
#420
 $GRP I=1E+2 $END
 $GRP $END
 '
 $END
#421
 $GRP I=1.E2 $END
 $GRP $END
 '
 $END
#422
 $GRP I=1E2.5 $END
 $GRP $END
 '
 $END
#423
 $GRP I=1EE2 $END
 $GRP $END
 '
 $END
#424
 $GRP I=1D $END
 $GRP $END
 '
 $END
#425
 $GRP I=1Q $END
 $GRP $END
 '
 $END
#426
 $GRP I=+.5E1 $END
 $GRP $END
 '
 $END
#427
 $GRP I=1 E2 $END
 $GRP $END
 '
 $END
#428
 $GRP I=1.5F $END
 $GRP $END
 '
 $END
#429
 $GRP I=1E-1 $END
 $GRP $END
 '
 $END
#430
 $GRP R=1.5$END $END
 $GRP $END
 '
 $END
#431
 $GRP M=T$END $END
 $GRP $END
 '
 $END
#432
 $GRP M=.TRUE.$END $END
 $GRP $END
 '
 $END
#433
 $GRP M=TRUE$END $END
 $GRP $END
 '
 $END
#434
 $GRP M=T&END $END
 $GRP $END
 '
 $END
#435
 $GRP A=T,F,T $END
 $GRP $END
 '
 $END
#436
 $GRP A=TX FX $END
 $GRP $END
 '
 $END
#437
 $GRP A=2*T $END
 $GRP $END
 '
 $END
#438
 $GRP A=2*TX $END
 $GRP $END
 '
 $END
#439
 $GRP T $END
 $GRP $END
 '
 $END
#440
 $GRP F=1 $END
 $GRP $END
 '
 $END
#441
 $GRP TX=1 $END
 $GRP $END
 '
 $END
#442
 $GRP A=1 TEST=3 $END
 $GRP $END
 '
 $END
#443
 $GRP A=1 TEST $END
 $GRP $END
 '
 $END
#444
 $GRP A=1 FOO $END
 $GRP $END
 '
 $END
#445
 $GRP A=1 BAR $END
 $GRP $END
 '
 $END
#446
 $GRP A=T(1) $END
 $GRP $END
 '
 $END
#447
 $GRP L=F(1)=1 $END
 $GRP $END
 '
 $END
#448
 $GRP L=T,R=1 $END
 $GRP $END
 '
 $END
#449
 $GRP L=.TRUE.R=1 $END
 $GRP $END
 '
 $END
#450
 $GRP M=T1 $END
 $GRP $END
 '
 $END
#451
 $GRP M=T_1 $END
 $GRP $END
 '
 $END
#452
 $GRP M=T$ $END
 $GRP $END
 '
 $END
#453
 $GRP M=T$X $END
 $GRP $END
 '
 $END
#454
 $GRP M=T( $END
 $GRP $END
 '
 $END
#455
 $GRP A=.T,.F $END
 $GRP $END
 '
 $END
#456
 $GRP A=.X $END
 $GRP $END
 '
 $END
#457
 $GRP A=.1.2 $END
 $GRP $END
 '
 $END
#458
 $GRP NOPE$=1 $END
 $GRP $END
 '
 $END
#459
 $GRP NOPE_1=1 $END
 $GRP $END
 '
 $END
#460
 $GRP NOPE$ $END
 $GRP $END
 '
 $END
#461
 $GRP M=.T.X $END
 $GRP $END
 '
 $END
#462
 $GRP M=.FX $END
 $GRP $END
 '
 $END
#463
 $GRP M=T$END I=5 $END
 $GRP $END
 '
 $END
#464
 $GRP M=TRUE$X I=5 $END
 $GRP $END
 '
 $END
#465
 $GRP LONGNAME1$=7 $END
 $GRP $END
 '
 $END
#466
 $GRP I=5$X=6 $END
 $GRP $END
 '
 $END
#467
 $GRP I=5 $X=6 $END
 $GRP $END
 '
 $END
#468
 $GRP M=T$=1 $END
 $GRP $END
 '
 $END
#469
 $GRP C=(T,F) $END
 $GRP $END
 '
 $END
#470
 $GRP C=(.TRUE.,1) $END
 $GRP $END
 '
 $END
#471
 $GRP C=(1,T) $END
 $GRP $END
 '
 $END
#472
 $GRP C=(1E,2) $END
 $GRP $END
 '
 $END
#473
 $GRP C=(+1,+2) $END
 $GRP $END
 '
 $END
#474
 $GRP C=(1.2.3,2) $END
 $GRP $END
 '
 $END
#475
 $GRP T(2:3)(1:1)='Z' $END
 $GRP $END
 '
 $END
#476
 $GRP TA(2)(1:2)(1:1)='Z' $END
 $GRP $END
 '
 $END
#477
 $GRP T(A:B)='Z' $END
 $GRP $END
 '
 $END
#478
 $GRP T(1.5:2)='Z' $END
 $GRP $END
 '
 $END
#479
 $GRP T(1:2:3)='Z' $END
 $GRP $END
 '
 $END
#480
 $GRP T(1,2)='Z' $END
 $GRP $END
 '
 $END
#481
 $GRP A(1,I)=5 $END
 $GRP $END
 '
 $END
#482
 $GRP A(2I)=5 $END
 $GRP $END
 '
 $END
#483
 $GRP A($)=5 $END
 $GRP $END
 '
 $END
#484
 $GRP A('1')=5 $END
 $GRP $END
 '
 $END
#485
 $GRP A(1)5 $END
 $GRP $END
 '
 $END
#486
 $GRP T(2:3)5 $END
 $GRP $END
 '
 $END
#487
 $GRP A(2)(1)=5 $END
 $GRP $END
 '
 $END
#488
 $GRP TA(2)(2)='Z' $END
 $GRP $END
 '
 $END
#489
 $GRP TA(2)(2:3) $END
 $GRP $END
 '
 $END
#490
 $GRP T(+2:+3)='Z' $END
 $GRP $END
 '
 $END
#491
 $GRP T(-1:3)='Z' $END
 $GRP $END
 '
 $END
#492
 $GRP T(99999999999:3)='Z' $END
 $GRP $END
 '
 $END
#493
 $GRP T(1:99999999999)='Z' $END
 $GRP $END
 '
 $END
#494
 $GRP A(0002)=5 $END
 $GRP $END
 '
 $END
#495
 $GRP A(+)=5 $END
 $GRP $END
 '
 $END
#496
 $GRP A(-)=5 $END
 $GRP $END
 '
 $END
#497
 $GRP T(:)(:)='Z' $END
 $GRP $END
 '
 $END
#498
 $GRP A(2,)=5 $END
 $GRP $END
 '
 $END
#499
 $GRP A=1,0*5 $END
 $GRP $END
 '
 $END
#500
 $GRP A=02*5 $END
 $GRP $END
 '
 $END
#501
 $GRP A=2*-5 $END
 $GRP $END
 '
 $END
#502
 $GRP A=2*+5 $END
 $GRP $END
 '
 $END
#503
 $GRP A=2*5*6 $END
 $GRP $END
 '
 $END
#504
 $GRP A=2*'A' $END
 $GRP $END
 '
 $END
#505
 $GRP A=99999999999*5 $END
 $GRP $END
 '
 $END
#506
 $GRP A=2*(1,2) $END
 $GRP $END
 '
 $END
#507
 $GRP A=2* $END
 $GRP $END
 '
 $END
#508
 $GRP A=2*$END $END
 $GRP $END
 '
 $END
#509
 $GRP A=2*, $END
 $GRP $END
 '
 $END
#510
 $GRP A=2*,,7 $END
 $GRP $END
 '
 $END
#511
 $GRP A=1* $END
 $GRP $END
 '
 $END
#512
 $GRP A=1*,5 $END
 $GRP $END
 '
 $END
#513
 $GRP T=2*'X' $END
 $GRP $END
 '
 $END
#514
 $GRP T=2* $END
 $GRP $END
 '
 $END
#515
 $GRP T=1*'XY' $END
 $GRP $END
 '
 $END
#516
 $GRP CA=2*5 $END
 $GRP $END
 '
 $END
#517
 $GRP A=*5 $END
 $GRP $END
 '
 $END
#518
 $GRP A=+2*5 $END
 $GRP $END
 '
 $END
#519
 $GRP A=2.*5 $END
 $GRP $END
 '
 $END
#520
 $GRP A=2.5*5 $END
 $GRP $END
 '
 $END
#521
 $GRP TA=1*'X',1*'Y' $END
 $GRP $END
 '
 $END
#522
 $GRP ? I=7 $END
 $GRP $END
 '
 $END
#523
 $GRP ?
 I=7 $END
 $GRP $END
 '
 $END
#524
 $GRP I=7 ?
 R=1 $END
 $GRP $END
 '
 $END
#525
 $GRP I=? R=1 $END
 $GRP $END
 '
 $END
#526
 $GRP =?
 I=7 $END
 $GRP $END
 '
 $END
#527
 $GRP ?? I=7 $END
 $GRP $END
 '
 $END
#528
 $GRP I=7 ?R=1 $END
 $GRP $END
 '
 $END
#529
 $GRP A=1 ? 2 $END
 $GRP $END
 '
 $END
#530
 ?X
 $GRP $END
 '
 $END
#531
 ?X $GRP I=1 $END
 $GRP $END
 '
 $END
#532
 $GRP A=1,?,3 $END
 $GRP $END
 '
 $END
#533
 $GRP T='?' $END
 $GRP $END
 '
 $END
$ EOD
$ EXIT
