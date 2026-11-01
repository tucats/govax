! COPYOUT.CMD - copies VMS's results off the exchange volume. Written by
! testdata/link82/gen.go. Run from the repository root, after the VAX has
! dismounted the volume:
!
!     govax console < testdata/link82/copyout.cmd
!
MOUNT/NOWRITE DUA2 "testdata/disks/link82-exchange.dsk"
COPY DUA2:[000000]LINK82.LOG "testdata/link82/vax/link82.log"/HOST
COPY DUA2:[000000]LINK82.MLG "testdata/link82/vax/link82.mlg"/HOST
COPY DUA2:[000000]C01.MAP "testdata/link82/vax/c01.map"/HOST
COPY DUA2:[000000]C02.MAP "testdata/link82/vax/c02.map"/HOST
COPY DUA2:[000000]C03.MAP "testdata/link82/vax/c03.map"/HOST
COPY DUA2:[000000]C04.MAP "testdata/link82/vax/c04.map"/HOST
COPY DUA2:[000000]C05.MAP "testdata/link82/vax/c05.map"/HOST
COPY DUA2:[000000]C06.MAP "testdata/link82/vax/c06.map"/HOST
COPY DUA2:[000000]C07.MAP "testdata/link82/vax/c07.map"/HOST
COPY DUA2:[000000]C08.MAP "testdata/link82/vax/c08.map"/HOST
COPY DUA2:[000000]C09.MAP "testdata/link82/vax/c09.map"/HOST
COPY DUA2:[000000]C10.MAP "testdata/link82/vax/c10.map"/HOST
COPY DUA2:[000000]C11.MAP "testdata/link82/vax/c11.map"/HOST
COPY DUA2:[000000]C12.MAP "testdata/link82/vax/c12.map"/HOST
COPY DUA2:[000000]C13.MAP "testdata/link82/vax/c13.map"/HOST
COPY DUA2:[000000]C14.MAP "testdata/link82/vax/c14.map"/HOST
COPY DUA2:[000000]C15.MAP "testdata/link82/vax/c15.map"/HOST
COPY DUA2:[000000]C16.MAP "testdata/link82/vax/c16.map"/HOST
COPY DUA2:[000000]C17.MAP "testdata/link82/vax/c17.map"/HOST
COPY DUA2:[000000]C18.MAP "testdata/link82/vax/c18.map"/HOST
COPY DUA2:[000000]C19.MAP "testdata/link82/vax/c19.map"/HOST
COPY DUA2:[000000]C20.MAP "testdata/link82/vax/c20.map"/HOST
COPY DUA2:[000000]C21.MAP "testdata/link82/vax/c21.map"/HOST
COPY DUA2:[000000]C22.MAP "testdata/link82/vax/c22.map"/HOST
COPY DUA2:[000000]C23.MAP "testdata/link82/vax/c23.map"/HOST
COPY DUA2:[000000]C24.MAP "testdata/link82/vax/c24.map"/HOST
COPY DUA2:[000000]C25.MAP "testdata/link82/vax/c25.map"/HOST
COPY DUA2:[000000]C26.MAP "testdata/link82/vax/c26.map"/HOST
COPY DUA2:[000000]C27.MAP "testdata/link82/vax/c27.map"/HOST
COPY DUA2:[000000]C28.MAP "testdata/link82/vax/c28.map"/HOST
COPY DUA2:[000000]C29.MAP "testdata/link82/vax/c29.map"/HOST
COPY DUA2:[000000]C30.MAP "testdata/link82/vax/c30.map"/HOST
COPY DUA2:[000000]C31.MAP "testdata/link82/vax/c31.map"/HOST
COPY DUA2:[000000]C32.MAP "testdata/link82/vax/c32.map"/HOST
COPY DUA2:[000000]C33.MAP "testdata/link82/vax/c33.map"/HOST
COPY DUA2:[000000]C34.MAP "testdata/link82/vax/c34.map"/HOST
COPY DUA2:[000000]C35.MAP "testdata/link82/vax/c35.map"/HOST
COPY DUA2:[000000]C36.MAP "testdata/link82/vax/c36.map"/HOST
COPY DUA2:[000000]C37.MAP "testdata/link82/vax/c37.map"/HOST
COPY DUA2:[000000]C38.MAP "testdata/link82/vax/c38.map"/HOST
COPY DUA2:[000000]C39.MAP "testdata/link82/vax/c39.map"/HOST
COPY DUA2:[000000]C40.MAP "testdata/link82/vax/c40.map"/HOST
COPY DUA2:[000000]C41.MAP "testdata/link82/vax/c41.map"/HOST
COPY DUA2:[000000]C42.MAP "testdata/link82/vax/c42.map"/HOST
COPY DUA2:[000000]C43.MAP "testdata/link82/vax/c43.map"/HOST
COPY DUA2:[000000]C44.MAP "testdata/link82/vax/c44.map"/HOST
COPY DUA2:[000000]C45.MAP "testdata/link82/vax/c45.map"/HOST
COPY DUA2:[000000]C46.MAP "testdata/link82/vax/c46.map"/HOST
COPY DUA2:[000000]C47.MAP "testdata/link82/vax/c47.map"/HOST
COPY DUA2:[000000]C48.MAP "testdata/link82/vax/c48.map"/HOST
COPY DUA2:[000000]C49.MAP "testdata/link82/vax/c49.map"/HOST
COPY DUA2:[000000]C50.MAP "testdata/link82/vax/c50.map"/HOST
COPY DUA2:[000000]C51.MAP "testdata/link82/vax/c51.map"/HOST
COPY DUA2:[000000]C52.MAP "testdata/link82/vax/c52.map"/HOST
COPY DUA2:[000000]C53.MAP "testdata/link82/vax/c53.map"/HOST
COPY DUA2:[000000]C54.MAP "testdata/link82/vax/c54.map"/HOST
COPY DUA2:[000000]C55.MAP "testdata/link82/vax/c55.map"/HOST
COPY DUA2:[000000]C56.MAP "testdata/link82/vax/c56.map"/HOST
COPY DUA2:[000000]C57.MAP "testdata/link82/vax/c57.map"/HOST
DISMOUNT DUA2
