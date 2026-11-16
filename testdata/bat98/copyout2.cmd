! COPYOUT2.CMD - copies Phase 98's probe results off bat98-exchange.dsk
! into testdata/bat98/vax (README.md). Written by testdata/bat98/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/copyout2.cmd
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/bat98-exchange.dsk"
DIRECTORY DUA5:[BAT98LOG],[BAT98SCR],[000000]*.OBJ
COPY DUA5:[BAT98LOG]BAT98B.LOG "testdata/bat98/vax/bat98b.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98DROP.LOG "testdata/bat98/vax/bat98drop.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]HAND1.LOG "testdata/bat98/vax/hand1.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BOOTR.LOG "testdata/bat98/vax/bootr.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BOOTN.LOG "testdata/bat98/vax/bootn.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98X3B.LOG "testdata/bat98/vax/bat98x3b.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98V_SYSTEM.LOG "testdata/bat98/vax/bat98v_system.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98V_BAT98.LOG "testdata/bat98/vax/bat98v_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98V_BAT98O.LOG "testdata/bat98/vax/bat98v_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98V_BAT98W.LOG "testdata/bat98/vax/bat98v_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_A_BAT98.LOG "testdata/bat98/vax/bat98p_a_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_A_BAT98O.LOG "testdata/bat98/vax/bat98p_a_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_A_BAT98W.LOG "testdata/bat98/vax/bat98p_a_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_B_BAT98.LOG "testdata/bat98/vax/bat98p_b_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_B_BAT98O.LOG "testdata/bat98/vax/bat98p_b_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_B_BAT98W.LOG "testdata/bat98/vax/bat98p_b_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_C_BAT98.LOG "testdata/bat98/vax/bat98p_c_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_C_BAT98O.LOG "testdata/bat98/vax/bat98p_c_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_C_BAT98W.LOG "testdata/bat98/vax/bat98p_c_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_D_BAT98.LOG "testdata/bat98/vax/bat98p_d_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_D_BAT98O.LOG "testdata/bat98/vax/bat98p_d_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_D_BAT98W.LOG "testdata/bat98/vax/bat98p_d_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_E_BAT98.LOG "testdata/bat98/vax/bat98p_e_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_E_BAT98O.LOG "testdata/bat98/vax/bat98p_e_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_E_BAT98W.LOG "testdata/bat98/vax/bat98p_e_bat98w.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_F_BAT98.LOG "testdata/bat98/vax/bat98p_f_bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_F_BAT98O.LOG "testdata/bat98/vax/bat98p_f_bat98o.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98P_F_BAT98W.LOG "testdata/bat98/vax/bat98p_f_bat98w.log"/HOST/QUIET
COPY DUA5:[000000]DEF_SJC.OBJ "testdata/bat98/vax/def_sjc.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_QUI.OBJ "testdata/bat98/vax/def_qui.obj"/HOST/BINARY/QUIET
COPY DUA5:[000000]DEF_JBC.OBJ "testdata/bat98/vax/def_jbc.obj"/HOST/BINARY/QUIET
DISMOUNT DUA5
