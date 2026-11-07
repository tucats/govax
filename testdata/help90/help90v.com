$ ! HELP90V.COM - parts F and G of round 1 of the Phase 90 probe: the
$ ! debugger's HELP, and HELP on VMS's own EXCHANGE, SUBMIT, and ANALYZE
$ ! modules. Written by testdata/help90/gen.go. Its log may hold VMS's own
$ ! help text, so it stays out of the repository (the phase doc's
$ ! Decision 9). Run it after HELP90.COM, from the same directory:
$ !
$ !     @HELP90V/OUTPUT=HELP90V.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
$ !
$ ! Part F: the debugger's HELP, with DBG$HELP naming the probe's library,
$ ! then with nothing defined.
$ !
$ MACRO/DEBUG DBGT
$ LINK/DEBUG DBGT
$ DEFINE/USER_MODE DBG$INPUT HELP90D.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ DEFINE/USER_MODE DBG$HELP H90DIR:H90D
$ RUN DBGT
$ TYPE DBGH.DLG
$ DEFINE/USER_MODE DBG$INPUT HELP90V.DBG
$ DEFINE/USER_MODE DBG$DECW$DISPLAY " "
$ RUN DBGT
$ !
$ ! Part G: VMS's own modules, from the main library.
$ !
$ HV := HELP/NOPROMPT/NOUSERLIBRARY
$ HV
$ HV HELP
$ HV EXCHANGE
$ HV EXCHANGE COPY
$ HV EXCHANGE COPY Qualifiers
$ HV EXCHANGE COPY /ALLOCATION
$ HV EXCHANGE /NETWORK
$ HV EXCHANGE...
$ HV SUBMIT
$ HV SUBMIT Parameter
$ HV SUBMIT Parameter filespec
$ HV SUBMIT /AFTER
$ HV SUBMIT/AFTER
$ HV SUBMIT Qualifiers /AFTER
$ HV SUBMIT /AF
$ HV SUBMIT *
$ HV SUBMIT...
$ HV ANALYZE
$ HV ANALYZE /IMAGE
$ HV ANALYZE/IMAGE
$ HV ANALYZE /IMAGE Qualifiers
$ HV ANALYZE /IMAGE /GST
$ HV ANALYZE /AUDIT /SELECT
$ HV ANALYZE /AUDIT /SELECT ACCESS
$ HV ANALYZE /MEDIA Command_Qualifiers
$ HV ANALYZE /MEDIA /LOG
$ HV ANALYZE /OBJ
$ HV NOSUCH_H90
$ ! /LIBRARY without a device and directory, and /NOLIBRARY alone.
$ HELP/NOPROMPT/NOUSERLIBRARY/LIBRARY=H90A ALPS
$ HELP/NOPROMPT/NOLIBRARY/NOUSERLIBRARY ALPS
$ EXIT
