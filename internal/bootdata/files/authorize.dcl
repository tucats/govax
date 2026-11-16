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

grammar authorize

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

    ! RENAME old new: the account's username, and its UIC identifier's.
    verb rename/id=12
        parameter   old/id=13                    -
                    /type=$any                   -
                    /prompt="Username"
        parameter   new/id=14                    -
                    /type=$any                   -
                    /prompt="New username"
