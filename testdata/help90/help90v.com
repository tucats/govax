$ ! HELP90V.COM - part G of round 1 of the Phase 90 probe: HELP on VMS's
$ ! own EXCHANGE, SUBMIT, and ANALYZE modules. Written by
$ ! testdata/help90/gen.go. Its log holds VMS's own help text, so it stays
$ ! out of the repository (the phase doc's Decision 9). Run it after
$ ! HELP90.COM, from the same directory:
$ !
$ !     @HELP90V/OUTPUT=HELP90V.LOG
$ !
$ SET NOON
$ SET VERIFY
$ L = F$ENVIRONMENT("DEFAULT")
$ DEFINE H90DIR 'L'
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
