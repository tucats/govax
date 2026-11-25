! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/ftn86/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/ftn86/vax/round7/mar
!     govax console < testdata/ftn86/copyout.cmd
!
! A run goes to vax/round7, beside the one the tests read in vax/, until it's checked;
! its mar/ is the sources it built, for TestFTN86Objects.
MOUNT/NOWRITE DUA5: "testdata/disks/ftn86-exchange.dsk"
COPY/QUIET DUA5:[000000]FTN86.LOG "testdata/ftn86/vax/round7/ftn86.log"/HOST
COPY/QUIET DUA5:[FTN86]FTNRTL.MAP "testdata/ftn86/vax/round7/ftnrtl.map"/HOST
COPY/QUIET/BINARY DUA5:[FTN86]FTNRTL.EXE "testdata/ftn86/vax/round7/ftnrtl.exe"/HOST
COPY/QUIET DUA5:[FTN86]FTNRTL.OPT "testdata/ftn86/vax/round7/mar/ftnrtl.opt"/HOST
COPY/QUIET DUA5:[FTN86]FTN_VECTOR.OBJ "testdata/ftn86/vax/round7/ftn_vector.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_VECTOR.MAR "testdata/ftn86/vax/round7/mar/ftn_vector.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_CONTROL.OBJ "testdata/ftn86/vax/round7/ftn_control.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_CONTROL.MAR "testdata/ftn86/vax/round7/mar/ftn_control.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_UNITS.OBJ "testdata/ftn86/vax/round7/ftn_units.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_UNITS.MAR "testdata/ftn86/vax/round7/mar/ftn_units.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_STATEMENT.OBJ "testdata/ftn86/vax/round7/ftn_statement.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_STATEMENT.MAR "testdata/ftn86/vax/round7/mar/ftn_statement.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_FORMAT.OBJ "testdata/ftn86/vax/round7/ftn_format.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_FORMAT.MAR "testdata/ftn86/vax/round7/mar/ftn_format.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_EDIT.OBJ "testdata/ftn86/vax/round7/ftn_edit.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_EDIT.MAR "testdata/ftn86/vax/round7/mar/ftn_edit.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_FLOAT.OBJ "testdata/ftn86/vax/round7/ftn_float.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_FLOAT.MAR "testdata/ftn86/vax/round7/mar/ftn_float.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_LISTOUT.OBJ "testdata/ftn86/vax/round7/ftn_listout.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_LISTOUT.MAR "testdata/ftn86/vax/round7/mar/ftn_listout.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_LISTIN.OBJ "testdata/ftn86/vax/round7/ftn_listin.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_LISTIN.MAR "testdata/ftn86/vax/round7/mar/ftn_listin.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_NAMELIST.OBJ "testdata/ftn86/vax/round7/ftn_namelist.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_NAMELIST.MAR "testdata/ftn86/vax/round7/mar/ftn_namelist.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INTERNAL.OBJ "testdata/ftn86/vax/round7/ftn_internal.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INTERNAL.MAR "testdata/ftn86/vax/round7/mar/ftn_internal.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_OPEN.OBJ "testdata/ftn86/vax/round7/ftn_open.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_OPEN.MAR "testdata/ftn86/vax/round7/mar/ftn_open.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INQUIRE.OBJ "testdata/ftn86/vax/round7/ftn_inquire.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INQUIRE.MAR "testdata/ftn86/vax/round7/mar/ftn_inquire.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_POSITION.OBJ "testdata/ftn86/vax/round7/ftn_position.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_POSITION.MAR "testdata/ftn86/vax/round7/mar/ftn_position.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_UNFORMATTED.OBJ "testdata/ftn86/vax/round7/ftn_unformatted.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_UNFORMATTED.MAR "testdata/ftn86/vax/round7/mar/ftn_unformatted.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_DIRECT.OBJ "testdata/ftn86/vax/round7/ftn_direct.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_DIRECT.MAR "testdata/ftn86/vax/round7/mar/ftn_direct.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_RECORD.OBJ "testdata/ftn86/vax/round7/ftn_record.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_RECORD.MAR "testdata/ftn86/vax/round7/mar/ftn_record.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_KEYED.OBJ "testdata/ftn86/vax/round7/ftn_keyed.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_KEYED.MAR "testdata/ftn86/vax/round7/mar/ftn_keyed.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INTRINSICS.OBJ "testdata/ftn86/vax/round7/ftn_intrinsics.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_INTRINSICS.MAR "testdata/ftn86/vax/round7/mar/ftn_intrinsics.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_SYSTEM.OBJ "testdata/ftn86/vax/round7/ftn_system.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_SYSTEM.MAR "testdata/ftn86/vax/round7/mar/ftn_system.mar"/HOST
COPY/QUIET DUA5:[FTN86]FTN_ALLOC.OBJ "testdata/ftn86/vax/round7/ftn_alloc.obj"/HOST
COPY/QUIET DUA5:[FTN86]FTN_ALLOC.MAR "testdata/ftn86/vax/round7/mar/ftn_alloc.mar"/HOST
DISMOUNT DUA5:
EXIT
