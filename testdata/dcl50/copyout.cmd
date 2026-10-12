! COPYOUT.CMD - copies Phase 50's probe logs off dcl50.dsk into the
! repository (README.md). Run from the repository root after the round's
! @PROBE50A/OUTPUT=PROBE50A.LOG and its session at the terminal (round
! 6; round 5's were @PROBE50T/OUTPUT=PROBE50T.LOG and the PROBE50Y
! session, round 4's @PROBE50F/OUTPUT=PROBE50F.LOG and @PROBE50Q; round 1's was
! @PROBE50/OUTPUT=PROBE50.LOG, round 2's @PROBE50V/OUTPUT=PROBE50V.LOG,
! and round 3's @PROBE50I/OUTPUT=PROBE50I.LOG, copied to vax/probe50.log,
! vax/probe50v.log, and vax/probe50i.log the same way):
!
!     govax console < testdata/dcl50/copyout.cmd
!
! Round 5's PROBE50N.LOG was the log PROBE50T's T1 made with /OUTPUT.
! A session at the terminal is kept by hand: round 6's as
! vax/probe50a-term.log, round 5's PROBE50Y session as vax/probe50y.log,
! round 4's PROBE50Q session as vax/probe50q.log. (Round 6's R3 logs
! and R4's file need no copy: PROBE50A.LOG's DIRECTORY and DUMP show them.)
!
MOUNT DUA1 "testdata/disks/dcl50.dsk"
COPY DUA1:[000000]PROBE50A.LOG "testdata/dcl50/vax/probe50a.log"/HOST/QUIET
DISMOUNT DUA1
