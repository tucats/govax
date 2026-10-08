$ ! P5ARGS.COM - probe 5's step 8, the services the first run didn't
$ ! reach (README.md). Run it, from SYSTEM, with the exchange volume as
$ ! the default directory:
$ !
$ !     @P5ARGS/OUTPUT=P5ARGS.LOG
$ !
$ SET NOON
$ MACRO P5ARGS
$ LINK P5ARGS
$ RUN P5ARGS
$ EXIT
