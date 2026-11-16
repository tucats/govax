!  Copyright (c) 1997-2026 Forest Edge Software
!                See LICENSE for applicable MIT license information.
!
!   DCLRTL grammar definition for AUTHORIZE, govax's account utility
!   (internal/authorize; docs/planning/PHASE-95 - authentication.md,
!   Decision 10). It is parsed apart from console.dcl, as debug.dcl is:
!   these are the commands at AUTHORIZE's "UAF> " prompt, and the one
!   command "AUTHORIZE command" and "MCR AUTHORIZE command" run.
!
!   The account qualifiers are the fields of a record govax keeps
!   (the phase doc's A.3); the ones it doesn't (/ACCESS, /PRIMEDAYS,
!   /PWDLIFETIME, /PWDMINIMUM, /PWDEXPIRED, /EXPIRATION, /GENERATE_PASSWORD,
!   and the access time qualifiers) are left out, so they are IVQUAL.
!   Every value is $any: internal/authorize reads each as AUTHORIZE does,
!   with its own messages (UICERR, BADVALUE, VALTOOBIG), and checks the
!   flag and privilege names, an unknown one CLI's IVKEYW as on VMS 7.1.
!   The four verbs that take them repeat the list, since a grammar's
!   qualifiers belong to one verb.
!
!   The identifier commands are the verbs' /IDENTIFIER (and SHOW's and
!   LIST's /RIGHTS) switching to a syntax of their own, as CREATE/DIRECTORY
!   does in console.dcl, so a qualifier one form lacks is IVQUAL there
!   (SHOW/RIGHTS/FULL, the probe's case 96).
!

grammar authorize

    ! ADD/IDENTIFIER name [/VALUE=IDENTIFIER=n | /VALUE=UIC=[g,m]]
    ! [/ATTRIBUTES=(...)]: a new identifier.
    syntax add_identifier
        parameter   id_name/id=600               -
                    /type=$any                   -
                    /prompt="Identifier"
        qualifier   value/id=601/nonegatable     -
                    /type=$any
        qualifier   attributes/id=602/nonegatable -
                    /type=$any/list

    ! REMOVE/IDENTIFIER name: the identifier and its holder records.
    syntax remove_identifier
        parameter   id_name/id=610               -
                    /type=$any                   -
                    /prompt="Identifier"

    ! RENAME/IDENTIFIER old new.
    syntax rename_identifier
        parameter   old/id=620                   -
                    /type=$any                   -
                    /prompt="Identifier"
        parameter   new/id=621                   -
                    /type=$any                   -
                    /prompt="New identifier"

    ! MODIFY/IDENTIFIER name [/VALUE=...] [/ATTRIBUTES=(...)] [/NAME=new].
    syntax modify_identifier
        parameter   id_name/id=630               -
                    /type=$any                   -
                    /prompt="Identifier"
        qualifier   value/id=631/nonegatable     -
                    /type=$any
        qualifier   attributes/id=632/nonegatable -
                    /type=$any/list
        qualifier   name/id=633/nonegatable      -
                    /type=$any

    ! GRANT/IDENTIFIER name holder [/ATTRIBUTES=(...)], REVOKE/IDENTIFIER
    ! name holder: a holder record of the holder's UIC, which the holder,
    ! a UIC identifier's name, gives.
    syntax grant_identifier
        parameter   id_name/id=640               -
                    /type=$any                   -
                    /prompt="Identifier"
        parameter   holder/id=641                -
                    /type=$any                   -
                    /prompt="Holder"
        qualifier   attributes/id=642/nonegatable -
                    /type=$any/list
    syntax revoke_identifier
        parameter   id_name/id=650               -
                    /type=$any                   -
                    /prompt="Identifier"
        parameter   holder/id=651                -
                    /type=$any                   -
                    /prompt="Holder"

    ! SHOW/IDENTIFIER [name] [/VALUE=...] [/FULL], LIST/IDENTIFIER the same
    ! into RIGHTSLIST.LIS; SHOW/RIGHTS name and LIST/RIGHTS name, what a UIC
    ! identifier's UIC holds.
    syntax show_identifier
        parameter   id_name/id=660               -
                    /type=$any
        qualifier   full/id=661
        qualifier   value/id=662/nonegatable     -
                    /type=$any
    syntax show_rights
        parameter   id_name/id=670               -
                    /type=$any
    syntax list_identifier
        parameter   id_name/id=680               -
                    /type=$any
        qualifier   full/id=681
        qualifier   value/id=682/nonegatable     -
                    /type=$any
    syntax list_rights
        parameter   id_name/id=690               -
                    /type=$any

    ! SHOW username [/BRIEF | /FULL]: the accounts the username matches (%
    ! and * wildcards), in username order.
    verb show/id=15
        parameter   username/id=16               -
                    /type=$any                   -
                    /prompt="Username"
        qualifier   brief/id=17
        qualifier   full/id=18
        qualifier   identifier/syntax=show_identifier
        qualifier   rights/syntax=show_rights

    ! LIST [username] [/BRIEF | /FULL]: SHOW's report, of every account by
    ! default, into SYSUAF.LIS in the default directory.
    verb list/id=19
        parameter   username/id=20               -
                    /type=$any
        qualifier   brief/id=21
        qualifier   full/id=22
        qualifier   identifier/syntax=list_identifier
        qualifier   rights/syntax=list_rights

    ! GRANT and REVOKE have only their /IDENTIFIER forms.
    verb grant/id=23
        qualifier   identifier/syntax=grant_identifier
    verb revoke/id=24
        qualifier   identifier/syntax=revoke_identifier

    ! CREATE/RIGHTS makes the rights database, which govax's file always
    ! has: it is accepted and does nothing.
    verb create/id=25
        qualifier   rights/id=26

    ! EXIT ends a session at the UAF> prompt.
    verb exit/id=1

    ! ADD username [/qualifiers]: a new account, DEFAULT's record with the
    ! qualifiers' fields, and its UIC identifier.
    verb add/id=2
        parameter   username/id=3                -
                    /type=$any                   -
                    /prompt="Username"
        qualifier   account/id=100/nonegatable   -
                    /type=$any
        qualifier   astlm/id=101/nonegatable     -
                    /type=$any
        qualifier   biolm/id=102/nonegatable     -
                    /type=$any
        qualifier   bytlm/id=103/nonegatable     -
                    /type=$any
        qualifier   cli/id=104/nonegatable       -
                    /type=$any
        qualifier   clitables/id=105/nonegatable -
                    /type=$any
        qualifier   cputime/id=106/nonegatable   -
                    /type=$any
        qualifier   defprivileges/id=107/nonegatable -
                    /type=$any/list
        qualifier   device/id=108/nonegatable    -
                    /type=$any
        qualifier   diolm/id=109/nonegatable     -
                    /type=$any
        qualifier   directory/id=110/nonegatable -
                    /type=$any
        qualifier   enqlm/id=111/nonegatable     -
                    /type=$any
        qualifier   fillm/id=112/nonegatable     -
                    /type=$any
        qualifier   flags/id=113/nonegatable     -
                    /type=$any/list
        qualifier   jtquota/id=114/nonegatable   -
                    /type=$any
        qualifier   lgicmd/id=115/nonegatable    -
                    /type=$any
        qualifier   maxacctjobs/id=116/nonegatable -
                    /type=$any
        qualifier   maxdetach/id=117/nonegatable -
                    /type=$any
        qualifier   maxjobs/id=118/nonegatable   -
                    /type=$any
        qualifier   owner/id=119/nonegatable     -
                    /type=$any
        qualifier   password/id=120              -
                    /type=$any
        qualifier   pbytlm/id=121/nonegatable    -
                    /type=$any
        qualifier   pgflquota/id=122/nonegatable -
                    /type=$any
        qualifier   prclm/id=123/nonegatable     -
                    /type=$any
        qualifier   priority/id=124/nonegatable  -
                    /type=$any
        qualifier   privileges/id=125/nonegatable -
                    /type=$any/list
        qualifier   queprio/id=126/nonegatable   -
                    /type=$any
        qualifier   shrfillm/id=127/nonegatable  -
                    /type=$any
        qualifier   tqelm/id=128/nonegatable     -
                    /type=$any
        qualifier   uic/id=129/nonegatable       -
                    /type=$any
        qualifier   wsdefault/id=130/nonegatable -
                    /type=$any
        qualifier   wsextent/id=131/nonegatable  -
                    /type=$any
        qualifier   wsquota/id=132/nonegatable   -
                    /type=$any
        qualifier   identifier/syntax=add_identifier

    ! COPY old new [/qualifiers]: a new account, a copy of old's record.
    verb copy/id=4
        parameter   old/id=5                     -
                    /type=$any                   -
                    /prompt="Username"
        parameter   new/id=6                     -
                    /type=$any                   -
                    /prompt="New username"
        qualifier   account/id=200/nonegatable   -
                    /type=$any
        qualifier   astlm/id=201/nonegatable     -
                    /type=$any
        qualifier   biolm/id=202/nonegatable     -
                    /type=$any
        qualifier   bytlm/id=203/nonegatable     -
                    /type=$any
        qualifier   cli/id=204/nonegatable       -
                    /type=$any
        qualifier   clitables/id=205/nonegatable -
                    /type=$any
        qualifier   cputime/id=206/nonegatable   -
                    /type=$any
        qualifier   defprivileges/id=207/nonegatable -
                    /type=$any/list
        qualifier   device/id=208/nonegatable    -
                    /type=$any
        qualifier   diolm/id=209/nonegatable     -
                    /type=$any
        qualifier   directory/id=210/nonegatable -
                    /type=$any
        qualifier   enqlm/id=211/nonegatable     -
                    /type=$any
        qualifier   fillm/id=212/nonegatable     -
                    /type=$any
        qualifier   flags/id=213/nonegatable     -
                    /type=$any/list
        qualifier   jtquota/id=214/nonegatable   -
                    /type=$any
        qualifier   lgicmd/id=215/nonegatable    -
                    /type=$any
        qualifier   maxacctjobs/id=216/nonegatable -
                    /type=$any
        qualifier   maxdetach/id=217/nonegatable -
                    /type=$any
        qualifier   maxjobs/id=218/nonegatable   -
                    /type=$any
        qualifier   owner/id=219/nonegatable     -
                    /type=$any
        qualifier   password/id=220              -
                    /type=$any
        qualifier   pbytlm/id=221/nonegatable    -
                    /type=$any
        qualifier   pgflquota/id=222/nonegatable -
                    /type=$any
        qualifier   prclm/id=223/nonegatable     -
                    /type=$any
        qualifier   priority/id=224/nonegatable  -
                    /type=$any
        qualifier   privileges/id=225/nonegatable -
                    /type=$any/list
        qualifier   queprio/id=226/nonegatable   -
                    /type=$any
        qualifier   shrfillm/id=227/nonegatable  -
                    /type=$any
        qualifier   tqelm/id=228/nonegatable     -
                    /type=$any
        qualifier   uic/id=229/nonegatable       -
                    /type=$any
        qualifier   wsdefault/id=230/nonegatable -
                    /type=$any
        qualifier   wsextent/id=231/nonegatable  -
                    /type=$any
        qualifier   wsquota/id=232/nonegatable   -
                    /type=$any

    ! MODIFY username [/qualifiers]: the accounts the username matches
    ! (% and * wildcards) changed.
    verb modify/id=7
        parameter   username/id=8                -
                    /type=$any                   -
                    /prompt="Username"
        qualifier   account/id=300/nonegatable   -
                    /type=$any
        qualifier   astlm/id=301/nonegatable     -
                    /type=$any
        qualifier   biolm/id=302/nonegatable     -
                    /type=$any
        qualifier   bytlm/id=303/nonegatable     -
                    /type=$any
        qualifier   cli/id=304/nonegatable       -
                    /type=$any
        qualifier   clitables/id=305/nonegatable -
                    /type=$any
        qualifier   cputime/id=306/nonegatable   -
                    /type=$any
        qualifier   defprivileges/id=307/nonegatable -
                    /type=$any/list
        qualifier   device/id=308/nonegatable    -
                    /type=$any
        qualifier   diolm/id=309/nonegatable     -
                    /type=$any
        qualifier   directory/id=310/nonegatable -
                    /type=$any
        qualifier   enqlm/id=311/nonegatable     -
                    /type=$any
        qualifier   fillm/id=312/nonegatable     -
                    /type=$any
        qualifier   flags/id=313/nonegatable     -
                    /type=$any/list
        qualifier   jtquota/id=314/nonegatable   -
                    /type=$any
        qualifier   lgicmd/id=315/nonegatable    -
                    /type=$any
        qualifier   maxacctjobs/id=316/nonegatable -
                    /type=$any
        qualifier   maxdetach/id=317/nonegatable -
                    /type=$any
        qualifier   maxjobs/id=318/nonegatable   -
                    /type=$any
        qualifier   owner/id=319/nonegatable     -
                    /type=$any
        qualifier   password/id=320              -
                    /type=$any
        qualifier   pbytlm/id=321/nonegatable    -
                    /type=$any
        qualifier   pgflquota/id=322/nonegatable -
                    /type=$any
        qualifier   prclm/id=323/nonegatable     -
                    /type=$any
        qualifier   priority/id=324/nonegatable  -
                    /type=$any
        qualifier   privileges/id=325/nonegatable -
                    /type=$any/list
        qualifier   queprio/id=326/nonegatable   -
                    /type=$any
        qualifier   shrfillm/id=327/nonegatable  -
                    /type=$any
        qualifier   tqelm/id=328/nonegatable     -
                    /type=$any
        qualifier   uic/id=329/nonegatable       -
                    /type=$any
        qualifier   wsdefault/id=330/nonegatable -
                    /type=$any
        qualifier   wsextent/id=331/nonegatable  -
                    /type=$any
        qualifier   wsquota/id=332/nonegatable   -
                    /type=$any
        qualifier   identifier/syntax=modify_identifier

    ! DEFAULT [/qualifiers] is MODIFY DEFAULT: the record ADD copies.
    verb default/id=9
        qualifier   account/id=400/nonegatable   -
                    /type=$any
        qualifier   astlm/id=401/nonegatable     -
                    /type=$any
        qualifier   biolm/id=402/nonegatable     -
                    /type=$any
        qualifier   bytlm/id=403/nonegatable     -
                    /type=$any
        qualifier   cli/id=404/nonegatable       -
                    /type=$any
        qualifier   clitables/id=405/nonegatable -
                    /type=$any
        qualifier   cputime/id=406/nonegatable   -
                    /type=$any
        qualifier   defprivileges/id=407/nonegatable -
                    /type=$any/list
        qualifier   device/id=408/nonegatable    -
                    /type=$any
        qualifier   diolm/id=409/nonegatable     -
                    /type=$any
        qualifier   directory/id=410/nonegatable -
                    /type=$any
        qualifier   enqlm/id=411/nonegatable     -
                    /type=$any
        qualifier   fillm/id=412/nonegatable     -
                    /type=$any
        qualifier   flags/id=413/nonegatable     -
                    /type=$any/list
        qualifier   jtquota/id=414/nonegatable   -
                    /type=$any
        qualifier   lgicmd/id=415/nonegatable    -
                    /type=$any
        qualifier   maxacctjobs/id=416/nonegatable -
                    /type=$any
        qualifier   maxdetach/id=417/nonegatable -
                    /type=$any
        qualifier   maxjobs/id=418/nonegatable   -
                    /type=$any
        qualifier   owner/id=419/nonegatable     -
                    /type=$any
        qualifier   password/id=420              -
                    /type=$any
        qualifier   pbytlm/id=421/nonegatable    -
                    /type=$any
        qualifier   pgflquota/id=422/nonegatable -
                    /type=$any
        qualifier   prclm/id=423/nonegatable     -
                    /type=$any
        qualifier   priority/id=424/nonegatable  -
                    /type=$any
        qualifier   privileges/id=425/nonegatable -
                    /type=$any/list
        qualifier   queprio/id=426/nonegatable   -
                    /type=$any
        qualifier   shrfillm/id=427/nonegatable  -
                    /type=$any
        qualifier   tqelm/id=428/nonegatable     -
                    /type=$any
        qualifier   uic/id=429/nonegatable       -
                    /type=$any
        qualifier   wsdefault/id=430/nonegatable -
                    /type=$any
        qualifier   wsextent/id=431/nonegatable  -
                    /type=$any
        qualifier   wsquota/id=432/nonegatable   -
                    /type=$any

    ! REMOVE username: the account, and its UIC identifier.
    verb remove/id=10
        parameter   username/id=11               -
                    /type=$any                   -
                    /prompt="Username"
        qualifier   identifier/syntax=remove_identifier

    ! RENAME old new: the account's username, and its UIC identifier's.
    verb rename/id=12
        parameter   old/id=13                    -
                    /type=$any                   -
                    /prompt="Username"
        parameter   new/id=14                    -
                    /type=$any                   -
                    /prompt="New username"
        qualifier   identifier/syntax=rename_identifier
