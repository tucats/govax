! COPYOUT.CMD - copies Phase 98's probe results off bat98-exchange.dsk
! into testdata/bat98/vax (README.md). Written by testdata/bat98/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/bat98/copyout.cmd
!
! A file the run didn't make is a message, and the rest go on.
MOUNT DUA5 "testdata/disks/bat98-exchange.dsk"
DIRECTORY DUA5:[BAT98LOG],[BAT98SCR],[000000]*.OBJ
COPY DUA5:[BAT98LOG]BAT98.LOG "testdata/bat98/vax/bat98.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R1.LOG "testdata/bat98/vax/bat98r1.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R2.LOG "testdata/bat98/vax/bat98r2.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R3.LOG "testdata/bat98/vax/bat98r3.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R4.LOG "testdata/bat98/vax/bat98r4.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R5.LOG "testdata/bat98/vax/bat98r5.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R6.LOG "testdata/bat98/vax/bat98r6.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R7.LOG "testdata/bat98/vax/bat98r7.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R8.LOG "testdata/bat98/vax/bat98r8.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R10.LOG "testdata/bat98/vax/bat98r10.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R11.LOG "testdata/bat98/vax/bat98r11.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R12.LOG "testdata/bat98/vax/bat98r12.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R13.LOG "testdata/bat98/vax/bat98r13.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R14.LOG "testdata/bat98/vax/bat98r14.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R15.LOG "testdata/bat98/vax/bat98r15.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R16.LOG "testdata/bat98/vax/bat98r16.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98L1.LOG "testdata/bat98/vax/bat98l1.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98L2.LOG "testdata/bat98/vax/bat98l2.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98L3.LOG "testdata/bat98/vax/bat98l3.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98L4.LOG "testdata/bat98/vax/bat98l4.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R17.LOG "testdata/bat98/vax/bat98r17.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R18.LOG "testdata/bat98/vax/bat98r18.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R19.LOG "testdata/bat98/vax/bat98r19.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R21.LOG "testdata/bat98/vax/bat98r21.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98R22.LOG "testdata/bat98/vax/bat98r22.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98X1.LOG "testdata/bat98/vax/bat98x1.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98X2.LOG "testdata/bat98/vax/bat98x2.log"/HOST/QUIET
COPY DUA5:[BAT98LOG]BAT98X3.LOG "testdata/bat98/vax/bat98x3.log"/HOST/QUIET
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
DISMOUNT DUA5
