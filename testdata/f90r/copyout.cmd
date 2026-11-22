! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/f90r/gen.go. Run from the repository root, after the VAX has dismounted it:
!
!     mkdir -p testdata/f90r/vax/mar testdata/f90r/vax/fcalc
!     govax console < testdata/f90r/copyout.cmd
!
MOUNT/NOWRITE DUA5: "testdata/disks/f90r-exchange.dsk"
COPY/QUIET DUA5:[000000]F90R.LOG "testdata/f90r/vax/f90r.log"/HOST
COPY/QUIET DUA5:[FTNRTL]FTNRTL.MAP "testdata/f90r/vax/ftnrtl.map"/HOST
COPY/QUIET/BINARY DUA5:[FTNRTL]FTNRTL.EXE "testdata/f90r/vax/ftnrtl.exe"/HOST
COPY/QUIET DUA5:[FTNRTL]FTNRTL.OPT "testdata/f90r/vax/mar/ftnrtl.opt"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_VECTOR.OBJ "testdata/f90r/vax/ftn_vector.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_VECTOR.MAR "testdata/f90r/vax/mar/ftn_vector.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_CONTROL.OBJ "testdata/f90r/vax/ftn_control.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_CONTROL.MAR "testdata/f90r/vax/mar/ftn_control.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_UNITS.OBJ "testdata/f90r/vax/ftn_units.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_UNITS.MAR "testdata/f90r/vax/mar/ftn_units.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_STATEMENT.OBJ "testdata/f90r/vax/ftn_statement.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_STATEMENT.MAR "testdata/f90r/vax/mar/ftn_statement.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_FORMAT.OBJ "testdata/f90r/vax/ftn_format.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_FORMAT.MAR "testdata/f90r/vax/mar/ftn_format.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_EDIT.OBJ "testdata/f90r/vax/ftn_edit.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_EDIT.MAR "testdata/f90r/vax/mar/ftn_edit.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_FLOAT.OBJ "testdata/f90r/vax/ftn_float.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_FLOAT.MAR "testdata/f90r/vax/mar/ftn_float.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_LISTOUT.OBJ "testdata/f90r/vax/ftn_listout.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_LISTOUT.MAR "testdata/f90r/vax/mar/ftn_listout.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_LISTIN.OBJ "testdata/f90r/vax/ftn_listin.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_LISTIN.MAR "testdata/f90r/vax/mar/ftn_listin.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_NAMELIST.OBJ "testdata/f90r/vax/ftn_namelist.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_NAMELIST.MAR "testdata/f90r/vax/mar/ftn_namelist.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INTERNAL.OBJ "testdata/f90r/vax/ftn_internal.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INTERNAL.MAR "testdata/f90r/vax/mar/ftn_internal.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_OPEN.OBJ "testdata/f90r/vax/ftn_open.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_OPEN.MAR "testdata/f90r/vax/mar/ftn_open.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INQUIRE.OBJ "testdata/f90r/vax/ftn_inquire.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INQUIRE.MAR "testdata/f90r/vax/mar/ftn_inquire.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_POSITION.OBJ "testdata/f90r/vax/ftn_position.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_POSITION.MAR "testdata/f90r/vax/mar/ftn_position.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_UNFORMATTED.OBJ "testdata/f90r/vax/ftn_unformatted.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_UNFORMATTED.MAR "testdata/f90r/vax/mar/ftn_unformatted.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_DIRECT.OBJ "testdata/f90r/vax/ftn_direct.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_DIRECT.MAR "testdata/f90r/vax/mar/ftn_direct.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_RECORD.OBJ "testdata/f90r/vax/ftn_record.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_RECORD.MAR "testdata/f90r/vax/mar/ftn_record.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_KEYED.OBJ "testdata/f90r/vax/ftn_keyed.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_KEYED.MAR "testdata/f90r/vax/mar/ftn_keyed.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INTRINSICS.OBJ "testdata/f90r/vax/ftn_intrinsics.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_INTRINSICS.MAR "testdata/f90r/vax/mar/ftn_intrinsics.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_SYSTEM.OBJ "testdata/f90r/vax/ftn_system.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_SYSTEM.MAR "testdata/f90r/vax/mar/ftn_system.mar"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_ALLOC.OBJ "testdata/f90r/vax/ftn_alloc.obj"/HOST
COPY/QUIET DUA5:[FTNRTL]FTN_ALLOC.MAR "testdata/f90r/vax/mar/ftn_alloc.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCPLATFORM.OBJ "testdata/f90r/vax/fcalc/fcplatform.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCPLATFORM.MAR "testdata/f90r/vax/fcalc/fcplatform.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCKINDS.OBJ "testdata/f90r/vax/fcalc/fckinds.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCKINDS.MAR "testdata/f90r/vax/fcalc/fckinds.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCMESSAGES.OBJ "testdata/f90r/vax/fcalc/fcmessages.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCMESSAGES.MAR "testdata/f90r/vax/fcalc/fcmessages.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCVALUES.OBJ "testdata/f90r/vax/fcalc/fcvalues.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCVALUES.MAR "testdata/f90r/vax/fcalc/fcvalues.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCARRAYS.OBJ "testdata/f90r/vax/fcalc/fcarrays.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCARRAYS.MAR "testdata/f90r/vax/fcalc/fcarrays.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCLEXER.OBJ "testdata/f90r/vax/fcalc/fclexer.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCLEXER.MAR "testdata/f90r/vax/fcalc/fclexer.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCTREE.OBJ "testdata/f90r/vax/fcalc/fctree.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCTREE.MAR "testdata/f90r/vax/fcalc/fctree.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCSYMBOLS.OBJ "testdata/f90r/vax/fcalc/fcsymbols.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCSYMBOLS.MAR "testdata/f90r/vax/fcalc/fcsymbols.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCFUNCS.OBJ "testdata/f90r/vax/fcalc/fcfuncs.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCFUNCS.MAR "testdata/f90r/vax/fcalc/fcfuncs.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCPOLY.OBJ "testdata/f90r/vax/fcalc/fcpoly.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCPOLY.MAR "testdata/f90r/vax/fcalc/fcpoly.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCMATRIX.OBJ "testdata/f90r/vax/fcalc/fcmatrix.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCMATRIX.MAR "testdata/f90r/vax/fcalc/fcmatrix.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCEVAL.OBJ "testdata/f90r/vax/fcalc/fceval.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCEVAL.MAR "testdata/f90r/vax/fcalc/fceval.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCNUMERIC.OBJ "testdata/f90r/vax/fcalc/fcnumeric.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCNUMERIC.MAR "testdata/f90r/vax/fcalc/fcnumeric.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCPLOT.OBJ "testdata/f90r/vax/fcalc/fcplot.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCPLOT.MAR "testdata/f90r/vax/fcalc/fcplot.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCSTATE.OBJ "testdata/f90r/vax/fcalc/fcstate.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCSTATE.MAR "testdata/f90r/vax/fcalc/fcstate.mar"/HOST
COPY/QUIET DUA5:[FCALC]FCALC.OBJ "testdata/f90r/vax/fcalc/fcalc.obj"/HOST
COPY/QUIET DUA5:[FCALC]FCALC.MAR "testdata/f90r/vax/fcalc/fcalc.mar"/HOST
COPY/QUIET/BINARY DUA5:[FCALC]FCALC.EXE "testdata/f90r/vax/fcalc/fcalc.exe"/HOST
DISMOUNT DUA5:
EXIT
