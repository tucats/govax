C     Variable format expressions (section 8.3.2): the manual's example,
C     each kind of number, when they are computed, what they may reach,
C     and values their uses can't take.
      PROGRAM VFE
      DIMENSION A(5)
      INTEGER W
      COMMON /C/ NC
      DATA A /1., 2., 3., 4., 5./
C     The manual's example.
      DO 10 I = 1, 10
      WRITE (6, 100) I
  100 FORMAT (I<MAX(I,5)>)
   10 CONTINUE
      DO 20 I = 1, 5
      WRITE (6, 101) (A(I), J = 1, I)
  101 FORMAT (<I>F10.<I-1>)
   20 CONTINUE
C     Each number a descriptor has.
      W = 4
      M = 3
      K = 2
      WRITE (6, 200) 7
  200 FORMAT (1X, I<W>.<M>)
      WRITE (6, 201) 1, 2
  201 FORMAT (1X, I1, <K>X, I1)
      WRITE (6, 202)
  202 FORMAT (1X, 'ABCDEF', T<K+2>, 'X', TR<K>, 'Y', TL<K-1>, 'Z')
      WRITE (6, 203) 1.5
  203 FORMAT (1X, <K>PF8.2)
      WRITE (6, 204) 150.0
  204 FORMAT (1X, -<K>PF8.2)
      WRITE (6, 205) 1.5
  205 FORMAT (1X, E12.4E<K+1>)
      WRITE (6, 206) 1, 2, 3, 4
  206 FORMAT (1X, <K>(I<K>, ','))
      X = 2.7
      WRITE (6, 207) 5
  207 FORMAT (1X, I<X*2.>)
      ASSIGN 200 TO IFMT
      WRITE (6, IFMT) 8
C     A width for each item, a repeat count when it is reached, and neither
C     once the list has run out.
      NC = 1
      WRITE (6, 208) 1, 2, 3
  208 FORMAT (1X, 3I<NEXT()>)
      NC = 1
      WRITE (6, 209) 1, 2, 3
  209 FORMAT (1X, <NEXT()>I3)
      WRITE (6, 210) 9
  210 FORMAT (1X, I1, I<NEXT()>)
      WRITE (6, 211) NC
  211 FORMAT (1X, 'NC', I3)
C     An expression with calls, in a statement with temporaries of its own.
      NC = 0
      WRITE (6, 212) (J+NEXT(), J = 1, 2)
  212 FORMAT (1X, 2I<NEXT()+NEXT()>)
C     A function that does I/O of its own.
      PRINT 213, 5
  213 FORMAT (1X, I<NOISY()>)
C     Dummy arguments, in every way a routine reaches them.
      CALL SHOW(5, 'AB', A, 3)
      CALL EV(2)
      CALL EV2(0, 3)
C     Values their uses can't take.
      K = -1
      WRITE (6, 500, IOSTAT=IOS) 1
  500 FORMAT (1X, I<K>)
      WRITE (6, 900) IOS
      K = 0
      WRITE (6, 501, IOSTAT=IOS) 1
  501 FORMAT (1X, <K>I3)
      WRITE (6, 900) IOS
      WRITE (6, 502, IOSTAT=IOS) 1.0
  502 FORMAT (1X, E12.4E<K>)
      WRITE (6, 900) IOS
      WRITE (6, 503, IOSTAT=IOS) 1
  503 FORMAT (1X, <K>(I3))
      WRITE (6, 900) IOS
      K = -1
      WRITE (6, 504, IOSTAT=IOS) 1
  504 FORMAT (<K>X, I3)
      WRITE (6, 900) IOS
      K = 200
      WRITE (6, 505, IOSTAT=IOS) 1.0
  505 FORMAT (1X, <K>PF8.2)
      WRITE (6, 900) IOS
      K = 256
      WRITE (6, 506, ERR=30) 1
  506 FORMAT (1X, I5.<K>)
      PRINT *, 'NOT HERE'
   30 PRINT *, 'ERR='
C     A width of 0 is what I0 is: no room for the digits, and a record.
      K = 0
      WRITE (6, 507, IOSTAT=IOS) 1
  507 FORMAT (1X, I<K>)
      WRITE (6, 900) IOS
C     A run-time format can't have one.
      WRITE (6, '(1X,I<3>)', IOSTAT=IOS) 1
      WRITE (6, 900) IOS
  900 FORMAT (1X, 'IOS', I3)
      END
      INTEGER FUNCTION NEXT()
      COMMON /C/ NC
      NC = NC + 1
      NEXT = NC
      END
      INTEGER FUNCTION NOISY()
      WRITE (6, '(1X,A)') 'INSIDE'
      NOISY = 4
      END
      SUBROUTINE SHOW(N, S, B, L)
      CHARACTER*(*) S
      DIMENSION B(L)
      ISF(I) = I + 1
      WRITE (6, 300) N
  300 FORMAT (1X, I<N>)
      WRITE (6, 301) N
  301 FORMAT (1X, I<LEN(S//'XYZ')>)
      WRITE (6, 302) N
  302 FORMAT (1X, I<B(L)>)
      WRITE (6, 303) N
  303 FORMAT (1X, I<ISF(L)>)
      END
      SUBROUTINE EV(N)
      ENTRY EV2(M, N)
      WRITE (6, 400) N
  400 FORMAT (1X, I<N>)
      END
