$ ! MILESTONE.COM - the multiprocessing milestone on VMS
$ ! (testdata/mp/README.md, "The milestone"): MSPARENT and MSCHILD, built
$ ! and run both ways, each run's three files typed and then deleted.
$ ! Run it with the exchange volume as the default directory:
$ !
$ !     @MILESTONE/OUTPUT=MILESTONE.LOG
$ !
$ SET NOON
$ MACRO MSCHILD, MSPARENT
$ LINK MSCHILD
$ LINK MSPARENT
$ HERE = F$ENVIRONMENT("DEFAULT")
$ MSPARENT :== $'HERE'MSPARENT.EXE
$ MSPARENT CREPRC 'HERE'MSCHILD.EXE
$ TYPE SHARED.DAT, PARENT.DAT, CHILD.DAT
$ DELETE SHARED.DAT;*, PARENT.DAT;*, CHILD.DAT;*
$ MSPARENT SPAWN 'HERE'MSCHILD.EXE
$ TYPE SHARED.DAT, PARENT.DAT, CHILD.DAT
$ EXIT
