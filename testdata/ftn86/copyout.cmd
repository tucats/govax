! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/ftn86/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     govax console < testdata/ftn86/copyout.cmd
!
MOUNT/NOWRITE DUA2: "testdata/disks/ftn86-exchange.dsk"
COPY/QUIET DUA2:[000000]FTN86.LOG "testdata/ftn86/vax/ftn86.log"/HOST
COPY/QUIET DUA2:[FTN86]FTNRTL.MAP "testdata/ftn86/vax/ftnrtl.map"/HOST
COPY/QUIET/BINARY DUA2:[FTN86]FTNRTL.EXE "testdata/ftn86/vax/ftnrtl.exe"/HOST
COPY/QUIET DUA2:[FTN86]FTN_VECTOR.OBJ "testdata/ftn86/vax/ftn_vector.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_CONTROL.OBJ "testdata/ftn86/vax/ftn_control.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_UNITS.OBJ "testdata/ftn86/vax/ftn_units.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_STATEMENT.OBJ "testdata/ftn86/vax/ftn_statement.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_FORMAT.OBJ "testdata/ftn86/vax/ftn_format.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_EDIT.OBJ "testdata/ftn86/vax/ftn_edit.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_FLOAT.OBJ "testdata/ftn86/vax/ftn_float.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_LISTOUT.OBJ "testdata/ftn86/vax/ftn_listout.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_LISTIN.OBJ "testdata/ftn86/vax/ftn_listin.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_NAMELIST.OBJ "testdata/ftn86/vax/ftn_namelist.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_INTERNAL.OBJ "testdata/ftn86/vax/ftn_internal.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_OPEN.OBJ "testdata/ftn86/vax/ftn_open.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_INQUIRE.OBJ "testdata/ftn86/vax/ftn_inquire.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_POSITION.OBJ "testdata/ftn86/vax/ftn_position.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_UNFORMATTED.OBJ "testdata/ftn86/vax/ftn_unformatted.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_DIRECT.OBJ "testdata/ftn86/vax/ftn_direct.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_RECORD.OBJ "testdata/ftn86/vax/ftn_record.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_KEYED.OBJ "testdata/ftn86/vax/ftn_keyed.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_INTRINSICS.OBJ "testdata/ftn86/vax/ftn_intrinsics.obj"/HOST
COPY/QUIET DUA2:[FTN86]FTN_SYSTEM.OBJ "testdata/ftn86/vax/ftn_system.obj"/HOST
DISMOUNT DUA2:
QUIT
