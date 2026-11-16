$!  ADDUSER.COM - add a user account to govax: SYS$MANAGER:ADDUSER.COM.
$!
$!  Run it as SYSTEM (or with SYSPRV):
$!
$!     $ @SYS$MANAGER:ADDUSER
$!
$!  It asks for the new account's username, owner, UIC, password, home
$!  directory, and privileges, each with a default in brackets that Return
$!  takes; adds the account with AUTHORIZE; makes the home directory, owned
$!  by the account and closed to everyone else, unless it is already there,
$!  when it gives it to the account; and writes a starter LOGIN.COM there.
$!  Ctrl/Z at any question stops it, nothing added.
$!
$!  The account can then be used with govax's --user option:
$!
$!     % govax --user SMITH
$!
$!  govax supplies this file and replaces it when govax is updated; to
$!  change it, copy it under another name first.
$!
$ verifying = 'f$verify(0)'
$ on control_y then goto stopped
$ status = 1
$!
$!  Only a process that may change the authorization file may add to it:
$!  one with SYSPRV, or one in a system UIC group (MAXSYSGROUP, 8 unless
$!  vax.sysgen.maxsysgroup says otherwise).
$!
$ maxsysgroup = f$config("vax.sysgen.maxsysgroup")
$ if maxsysgroup .eqs. "" then maxsysgroup = 8
$ if .not. f$privilege("SYSPRV") .and. f$getjpi("","GRP") .gt. f$integer(maxsysgroup)
$ then
$    write sys$output "%ADDUSER-E-NOPRIV, adding an account needs SYSPRV or a system UIC"
$    status = %X10000024
$    goto done
$ endif
$!
$!  The username: 1 to 12 letters, digits, dollar signs, and underscores,
$!  not one the rights database already has.
$!
$ ask_user:
$ read/prompt="Username: "/end_of_file=stopped sys$command user
$ user = f$edit(user, "trim,upcase")
$ if user .eqs. "" then goto ask_user
$ if f$length(user) .gt. 12
$ then
$    write sys$output "%ADDUSER-W-LONG, a username is at most 12 characters"
$    goto ask_user
$ endif
$ if f$identifier(user, "name_to_number") .ne. 0
$ then
$    write sys$output "%ADDUSER-W-EXISTS, ", user, " is already a username or an identifier"
$    goto ask_user
$ endif
$!
$ read/prompt="Owner (the user's full name): "/end_of_file=stopped sys$command owner
$ owner = f$edit(owner, "trim,compress")
$!
$!  The UIC: a group, in octal, then a member, the first one of the group
$!  no account uses offered.
$!
$ ask_group:
$ read/prompt="UIC group number, octal [200]: "/end_of_file=stopped sys$command group
$ group = f$edit(group, "trim")
$ if group .eqs. "" then group = "200"
$ if f$type(group) .nes. "INTEGER" .or. f$locate("8", group) .lt. f$length(group) -
     .or. f$locate("9", group) .lt. f$length(group)
$ then
$    write sys$output "%ADDUSER-W-OCTAL, a group number is octal, 1 to 37776"
$    goto ask_group
$ endif
$ group_value = f$integer("%O" + group)
$ if group_value .lt. 1 .or. group_value .gt. %O37776 then goto ask_group
$ member_value = 1
$ next_member:
$ if f$identifier(group_value * %X10000 + member_value, "number_to_name") .nes. ""
$ then
$    member_value = member_value + 1
$    goto next_member
$ endif
$ free = f$fao("!%U", group_value * %X10000 + member_value)
$ free = f$element(1, ",", free) - "]"
$ ask_member:
$ read/prompt="UIC member number, octal [''free']: "/end_of_file=stopped sys$command member
$ member = f$edit(member, "trim")
$ if member .eqs. "" then member = free
$ if f$type(member) .nes. "INTEGER" .or. f$locate("8", member) .lt. f$length(member) -
     .or. f$locate("9", member) .lt. f$length(member)
$ then
$    write sys$output "%ADDUSER-W-OCTAL, a member number is octal, 1 to 177776"
$    goto ask_member
$ endif
$ member_value = f$integer("%O" + member)
$ if member_value .lt. 1 .or. member_value .gt. %O177776 then goto ask_member
$ uic = f$fao("!%U", group_value * %X10000 + member_value)
$ if f$extract(0, 1, f$fao("!%I", group_value * %X10000 + member_value)) .nes. "["
$ then
$    write sys$output "%ADDUSER-W-UICUSED, ", uic, " is already ", -
        f$fao("!%I", group_value * %X10000 + member_value), "'s UIC"
$    goto ask_member
$ endif
$!
$!  The password, typed twice and not shown. None at all is allowed: the
$!  account then logs in asking for nothing.
$!
$ ask_password:
$ set terminal/noecho
$ read/prompt="Password (Return for none): "/end_of_file=stopped sys$command password
$ read/prompt="Password again: "/end_of_file=stopped sys$command again
$ set terminal/echo
$ if password .nes. again
$ then
$    write sys$output "%ADDUSER-W-MISMATCH, the two passwords differ; type them again"
$    goto ask_password
$ endif
$!
$!  The home directory: on the default volume, if one is mounted, and
$!  named for the user.
$!
$ device = f$config("vax.default.volume.device")
$ if device .eqs. "" then device = "DUA1:"
$ if f$config("vax.default.volume.file") .eqs. "" .or. .not. f$getdvi(device, "exists")
$ then
$    device = "SYS$SYSDEVICE:"
$ else
$    if .not. f$getdvi(device, "mnt") then device = "SYS$SYSDEVICE:"
$ endif
$ if f$extract(f$length(device) - 1, 1, device) .nes. ":" then device = device + ":"
$ read/prompt="Home device [''device']: "/end_of_file=stopped sys$command answer
$ answer = f$edit(answer, "trim,upcase")
$ if answer .nes. "" then device = answer
$ if f$extract(f$length(device) - 1, 1, device) .nes. ":" then device = device + ":"
$ directory = "[" + user + "]"
$ read/prompt="Home directory [''directory']: "/end_of_file=stopped sys$command answer
$ answer = f$edit(answer, "trim,upcase")
$ if answer .nes. "" then directory = answer
$!
$!  The privileges, authorized and default alike.
$!
$ write sys$output "Privileges:"
$ write sys$output "  1  a plain user: TMPMBX and NETMBX"
$ write sys$output "  2  a list of your own"
$ ask_privileges:
$ read/prompt="Which [1]: "/end_of_file=stopped sys$command answer
$ answer = f$edit(answer, "trim")
$ privileges = "TMPMBX,NETMBX"
$ if answer .eqs. "" .or. answer .eqs. "1" then goto add
$ if answer .nes. "2" then goto ask_privileges
$ read/prompt="Privileges, separated by commas: "/end_of_file=stopped sys$command privileges
$ privileges = f$edit(privileges, "collapse,upcase")
$ if privileges .eqs. "" then privileges = "TMPMBX,NETMBX"
$!
$!  Add the account. ADD starts from the DEFAULT account, which is
$!  disabled, so the new one is enabled.
$!
$ add:
$ command = "ADD " + user + "/UIC=" + uic + "/DEVICE=" + device + "/DIRECTORY=" + directory
$ command = command + "/PRIVILEGES=(NOALL," + privileges + ")/DEFPRIVILEGES=(NOALL," + privileges + ")"
$ command = command + "/FLAGS=NODISUSER"
$ if owner .nes. "" then command = command + "/OWNER=""" + owner + """"
$ if password .nes. "" then command = command + "/PASSWORD=""" + password + """"
$ authorize 'command'
$ status = $status
$ if .not. status then goto done
$ if f$identifier(user, "name_to_number") .eq. 0
$ then
$    write sys$output "%ADDUSER-E-NOTADDED, AUTHORIZE didn't add ", user
$    status = %X10000002
$    goto done
$ endif
$ password = ""
$ again = ""
$!
$!  The home directory, owned by the account: made, or given to it. Its
$!  own file is [A]B.DIR for [A.B], and [000000]A.DIR for [A].
$!
$ home = device + directory
$ path = directory - "[" - "]" - "<" - ">"
$ parent = ""
$ leaf = path
$ i = 0
$ next_part:
$ part = f$element(i, ".", path)
$ if part .eqs. "." then goto have_parts
$ if i .eq. 1 then parent = leaf
$ if i .gt. 1 then parent = parent + "." + leaf
$ leaf = part
$ i = i + 1
$ goto next_part
$ have_parts:
$ if parent .eqs. "" then parent = "000000"
$ if f$parse(home) .eqs. ""
$ then
$    create/directory 'home' /owner_uic='uic' /protection=(S:RWE,O:RWE,G,W)
$    status = $status
$    if .not. status then goto done
$ else
$    set security/owner='uic' 'device'['parent']'leaf'.DIR
$    status = $status
$    if .not. status then goto done
$    write sys$output "%ADDUSER-I-EXISTING, ", home, " was already there; it is now ", user, "'s"
$ endif
$!
$!  A starter LOGIN.COM, the account's own, unless one is there.
$!
$ login = home + "LOGIN.COM"
$ if f$search(login) .eqs. ""
$ then
$    open/write login_file 'login'
$    write login_file "$! LOGIN.COM - ''user''s login procedure, run at each login after"
$    write login_file "$! SYS$MANAGER:SYLOGIN.COM. Put the commands you want each session"
$    write login_file "$! to start with here: symbols, logical names, SET commands."
$    write login_file "$!"
$    write login_file "$! For example:"
$    write login_file "$!"
$    write login_file "$!    $ home :== set default sys$login"
$    write login_file "$!"
$    write login_file "$ exit"
$    close login_file
$    set security/owner='uic'/protection=(S:RWED,O:RWED,G,W) 'login'
$ endif
$!
$ write sys$output ""
$ write sys$output "Added ", user, ", ", uic, ", at home in ", home, "."
$ write sys$output "To log in as ", user, ", start govax with:  govax --user ", f$edit(user, "lowercase")
$ status = 1
$ goto done
$!
$ stopped:
$ set terminal/echo
$ write sys$output ""
$ write sys$output "%ADDUSER-I-STOPPED, no account added"
$ status = %X10000001
$!
$ done:
$ set terminal/echo
$ verifying = f$verify(verifying)
$ exit status
