//go:build ignore

// gen writes Phase 93's probe of file protection (README.md): the command
// procedures prot93.com (SYSTEM's run: the accounts, the tree, and
// SYSTEM's parts), prot93u.com (each account's run), prot93b.com (the DCL
// utilities, run by SYSTEM and by each account), and prot93z.com (the
// listing at the end, and the accounts removed); the program's case
// files; hello.mar; the definition modules; and the govax console scripts
// exchange.cmd and copyout.cmd. Round 2 (README.md's "Round 2") is
// prot93r.com, prot93v.com, prot93y.com, prot93f.dat, prot93g.dat,
// exchange2.cmd, and copyout2.cmd. Run it from the repository root:
//
//	go run testdata/prot93/gen.go
//
// prot93.mar, the program the procedures run, is written by hand.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const dir = "testdata/prot93"

// disk is the logical name prot93.com defines, in the system table, for the
// exchange volume, whatever unit it is mounted on; the accounts' default
// device is it too.
const disk = "PROT93$DISK:"

// proc collects a procedure's lines.
type proc struct{ strings.Builder }

// line adds one line to the procedure.
func (p *proc) line(format string, args ...any) {
	fmt.Fprintf(p, format+"\n", args...)
}

// section starts a part of the log.
func (p *proc) section(title string) {
	p.line("$ !")
	p.line("$ WRITE SYS$OUTPUT \"######## %s\"", title)
}

// Protection codes the tree uses.
const (
	all     = "(S:RWED,O:RWED,G:RWED,W:RWED)"
	none    = "(S,O,G,W)"
	allDir  = "(S:RWE,O:RWE,G:RWE,W:RWE)"
	readDir = "(S:RE,O:RE,G:RE,W:RE)"
	stdProt = "(S:RWED,O:RWED,G:RE,W:RE)"
	noDel   = "(S:RWE,O:RWE,G:RWE,W:RWE)"
	ownerA  = "[200,1]"
	system  = "[1,4]"
)

// object is a file or directory the tree makes, with its owner,
// protection, and ACL (each ACE without its parentheses).
type object struct {
	name, owner, prot string
	acl               []string
}

// fFiles are [PROT93.F]'s files, each a different combination of
// categories, access, owner, and ACL, which every identity opens every
// way.
var fFiles = []object{
	{"FNONE", ownerA, none, nil},
	{"FALL", ownerA, all, nil},
	{"FR", ownerA, "(S:R,O:R,G:R,W:R)", nil},
	{"FW", ownerA, "(S:W,O:W,G:W,W:W)", nil},
	{"FRW", ownerA, "(S:RW,O:RW,G:RW,W:RW)", nil},
	{"FE", ownerA, "(S:E,O:E,G:E,W:E)", nil},
	{"FD", ownerA, "(S:D,O:D,G:D,W:D)", nil},
	{"FROT1", ownerA, "(S:R,O:W,G:E,W:D)", nil},
	{"FROT2", ownerA, "(S:W,O:E,G:D,W:R)", nil},
	{"FROT3", ownerA, "(S:E,O:D,G:R,W:W)", nil},
	{"FROT4", ownerA, "(S:D,O:R,G:W,W:E)", nil},
	{"FOWNER", ownerA, "(S,O:RWED,G,W)", nil},
	{"FGROUP", ownerA, "(S,O,G:RWED,W)", nil},
	{"FWORLD", ownerA, "(S,O,G,W:RWED)", nil},
	{"FSYS", ownerA, "(S:RWED,O,G,W)", nil},
	{"FUIC0", "[0,0]", none, nil},
	{"FUIC0ACL", "[0,0]", none, []string{"IDENTIFIER=PROT93B,ACCESS=READ"}},
	{"FACLDENY", ownerA, all, []string{"IDENTIFIER=PROT93B,ACCESS=NONE"}},
	{"FACLGRANT", ownerA, none, []string{"IDENTIFIER=PROT93C,ACCESS=READ+WRITE"}},
	{"FACLTEAM", ownerA, none, []string{"IDENTIFIER=PROT93_TEAM,ACCESS=READ"}},
	{"FACLOWN", ownerA, "(S:RWED,O:RWED,G,W)", []string{"IDENTIFIER=PROT93A,ACCESS=NONE"}},
	{"FACLCTL", ownerA, none, []string{"IDENTIFIER=PROT93C,ACCESS=CONTROL"}},
	{"FACLWILD", ownerA, none, []string{"IDENTIFIER=[200,*],ACCESS=READ"}},
	{"FACLSYS", ownerA, "(S:RWED,O,G,W)", []string{"IDENTIFIER=SYSTEM,ACCESS=NONE"}},
	{"FACLINT", ownerA, none, []string{"IDENTIFIER=INTERACTIVE,ACCESS=READ"}},
	{"FACLTWO", ownerA, none, []string{"IDENTIFIER=PROT93C,ACCESS=NONE", "IDENTIFIER=PROT93_TEAM,ACCESS=READ"}},
}

// accessDirs are directories that each give everyone one kind of access,
// each holding an A.DAT anyone may use.
var accessDirs = []object{
	{"DR", ownerA, "(S:R,O:R,G:R,W:R)", nil},
	{"DE", ownerA, "(S:E,O:E,G:E,W:E)", nil},
	{"DW", ownerA, "(S:WE,O:WE,G:WE,W:WE)", nil},
	{"DRW", ownerA, allDir, nil},
	{"DN", ownerA, none, nil},
	{"DWO", ownerA, "(S:W,O:W,G:W,W:W)", nil},
}

// paths are three-level paths with one level's access taken away, each
// ending in a PZ with X.DAT.
var paths = []struct {
	top        string
	px, py, pz string
}{
	{"PX1", none, allDir, allDir},
	{"PX2", allDir, none, allDir},
	{"PX3", allDir, allDir, none},
	{"PX4", "(S:E,O:E,G:E,W:E)", "(S:E,O:E,G:E,W:E)", "(S:E,O:E,G:E,W:E)"},
	{"PX5", allDir, allDir, allDir},
}

// otherDirs are the rest of the tree's directories.
var otherDirs = []object{
	{"NV", ownerA, allDir, nil},
	{"OWN", ownerA, allDir, nil},
	{"DP", ownerA, allDir, []string{
		"DEFAULT_PROTECTION,S:RWED,O:RWED,G:R,W:E",
		"IDENTIFIER=PROT93_TEAM,OPTIONS=DEFAULT,ACCESS=READ",
		"IDENTIFIER=PROT93B,OPTIONS=DEFAULT+NOPROPAGATE,ACCESS=READ+WRITE",
		"IDENTIFIER=PROT93A,ACCESS=READ",
	}},
	{"ER", ownerA, allDir, nil},
	{"ERX", ownerA, readDir, nil},
	{"RN", ownerA, allDir, nil},
	{"RNX", ownerA, readDir, nil},
	{"CTL", ownerA, allDir, nil},
	{"U", ownerA, allDir, nil},
	{"UDN", ownerA, "(S,O:RWE,G,W)", nil},
	{"UDE", ownerA, "(S:E,O:RWE,G:E,W:E)", nil},
	{"UDNW", ownerA, "(S:RE,O:RWE,G:RE,W:RE)", nil},
	{"IMG", ownerA, allDir, nil},
	{"PRF", ownerA, allDir, nil},
	{"ACE", system, allDir, nil},
	{"DEFP", system, allDir, nil},
	{"SEC", system, allDir, nil},
}

// tags are the identities that change files: SYSTEM's pass without its
// privileges (M0) and each account's first pass. Each has files of its
// own to delete, rename, and change.
var tags = []string{"M0", "A0", "B0", "C0"}

// uProts are the protections Part B's utilities meet: owner only, owner
// and group, and everyone; SYSTEM (in the system category, and the world)
// is denied the first two once its privileges are gone.
var uProts = []struct{ name, prot string }{
	{"NONE", "(S,O:RWED,G,W)"},
	{"GRP", "(S,O:RWED,G:RWED,W)"},
	{"WLD", all},
}

// imgProts are the images' and procedures' protections.
var imgProts = []struct{ name, prot string }{
	{"E", "(S:E,O:RWED,G:E,W:E)"},
	{"R", "(S:R,O:RWED,G:R,W:R)"},
	{"N", "(S,O:RWED,G,W)"},
}

// uKinds are Part B's per-tag files, one of each for each of uProts.
var uKinds = []string{"DEL", "PUR", "REN", "APP", "CPY", "OPW", "OPA", "SEC", "SFI", "SPR", "ACL", "OWN"}

// aceFiles are [PROT93.ACE]'s files, each with one ACL (some ACEs are
// for directories, and go on a directory of the name).
var aceFiles = []struct {
	name string
	dir  bool
	acl  []string
}{
	{"ACE01", false, []string{"IDENTIFIER=PROT93A,ACCESS=READ"}},
	{"ACE02", false, []string{"IDENTIFIER=PROT93A,ACCESS=READ+WRITE+EXECUTE+DELETE+CONTROL"}},
	{"ACE03", false, []string{"IDENTIFIER=[200,1],ACCESS=READ"}},
	{"ACE04", false, []string{"IDENTIFIER=[200,*],ACCESS=READ"}},
	{"ACE05", false, []string{"IDENTIFIER=[*,*],ACCESS=READ"}},
	{"ACE06", false, []string{"IDENTIFIER=PROT93_TEAM,ACCESS=NONE"}},
	{"ACE07", false, []string{"IDENTIFIER=PROT93A+PROT93_TEAM,ACCESS=READ"}},
	{"ACE08", false, []string{"IDENTIFIER=INTERACTIVE,ACCESS=READ"}},
	{"ACE09", false, []string{"IDENTIFIER=PROT93A,OPTIONS=PROTECTED,ACCESS=READ"}},
	{"ACE10", false, []string{"IDENTIFIER=PROT93A,OPTIONS=NOPROPAGATE,ACCESS=READ"}},
	{"ACE11", false, []string{"IDENTIFIER=PROT93A,OPTIONS=HIDDEN,ACCESS=READ"}},
	{"ACE12", false, []string{"IDENTIFIER=PROT93A,OPTIONS=DEFAULT,ACCESS=READ"}},
	{"ACE13", false, []string{"IDENTIFIER=PROT93A,OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ"}},
	{"ACE14", true, []string{"DEFAULT_PROTECTION,S:RWED,O:RWED,G:RE,W:"}},
	{"ACE15", true, []string{"DEFAULT_PROTECTION,OPTIONS=PROTECTED,S:RWED,O:RWED,G,W"}},
	{"ACE16", true, []string{"CREATOR,ACCESS=READ+WRITE"}},
	{"ACE17", false, []string{"ALARM=SECURITY,ACCESS=READ+WRITE+SUCCESS+FAILURE"}},
	{"ACE18", false, []string{"AUDIT=SECURITY,ACCESS=DELETE+FAILURE"}},
	{"ACE19", false, []string{"IDENTIFIER=PROT93G2,ACCESS=READ"}},
	{"ACE20", false, []string{"IDENTIFIER=PROT93A,ACCESS=READ", "IDENTIFIER=PROT93B,ACCESS=WRITE"}},
	{"ACE21", true, []string{"IDENTIFIER=PROT93A,OPTIONS=DEFAULT,ACCESS=READ"}},
}

// defModules are the system definition macros whose values round 2's
// services need: each is assembled alone, GLOBAL, so its object's global
// symbols are the values (decode.go).
var defModules = []string{"CHP", "OSS", "ACE", "ACL", "ARM", "KGB", "PRV"}

// spec is a file in the tree: "[PROT93.sub]name".
func spec(sub, name string) string {
	return disk + "[PROT93." + sub + "]" + name
}

// dirFile is the directory file of a directory below [PROT93]:
// "A.B" is [PROT93.A]B.DIR.
func dirFile(path string) string {
	i := strings.LastIndex(path, ".")
	if i < 0 {
		return disk + "[PROT93]" + path + ".DIR"
	}

	return disk + "[PROT93." + path[:i] + "]" + path[i+1:] + ".DIR"
}

// security sets an object's owner, protection, and ACL.
func (p *proc) security(target, owner, prot string, acl []string) {
	q := ""
	if owner != "" {
		q += "/OWNER=" + owner
	}

	if prot != "" {
		q += "/PROTECTION=" + prot
	}

	if q != "" {
		p.line("$ SET SECURITY%s %s", q, target)
	}

	if len(acl) > 0 {
		p.line("$ SET SECURITY/ACL=(%s) %s", joinACEs(acl), target)
	}
}

// joinACEs writes ACEs as /ACL takes them: one in parentheses, or several
// each in its own.
func joinACEs(acl []string) string {
	if len(acl) == 1 {
		return acl[0]
	}

	parts := make([]string, len(acl))
	for i, a := range acl {
		parts[i] = "(" + a + ")"
	}

	return strings.Join(parts, ",")
}

// file makes a file from the template and sets its security.
func (p *proc) file(target, owner, prot string, acl []string) {
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, target)
	p.security(target, owner, prot, acl)
}

// logged is a procedure's opening: run without "LOGGED" as P1, it runs
// itself again with its output in [PROT93LOG]'s log, and says where.
func (p *proc) logged(logName, title string) {
	p.line("$ IF P1 .EQS. \"LOGGED\" THEN GOTO BEGIN")
	p.line("$ HERE = F$ENVIRONMENT(\"PROCEDURE\")")
	p.line("$ DEV = F$PARSE(HERE,,,\"DEVICE\")")
	p.line("$ LOGFILE = DEV + \"[PROT93LOG]\" + %s", logName)
	p.line("$ @'HERE'/OUTPUT='LOGFILE' LOGGED")
	p.line("$ WRITE SYS$OUTPUT \"%s finished; the log is \", LOGFILE", title)
	p.line("$ EXIT")
	p.line("$ BEGIN:")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")
}

// run runs the program on a case file, under the tag given.
func (p *proc) run(tag, cases string) {
	p.line("$ DEFINE PROT93_TAG %s", tag)
	p.line("$ DEFINE/USER_MODE SYS$INPUT %s", cases)
	p.line("$ RUN %s[000000]PROT93", disk)
	p.line("$ SHOW SYMBOL $STATUS")
}

// The case files.
const (
	coreCases   = disk + "[000000]PROT93C.DAT"
	changeCases = disk + "[000000]PROT93D.DAT"
	aceCases    = disk + "[000000]PROT93E.DAT"
	fidCases    = disk + "[PROT93LOG]FIDS.DAT"
	dropPrivs   = "$ SET PROCESS/PRIVILEGES=(NOREADALL,NOGRPPRV,NOSYSPRV,NOBYPASS)"

	// showPrivs shows the four privileges that matter to protection.
	// (F$GETJPI's CURPRIV can be longer than a DCL token when a process
	// holds every privilege.)
	showPrivs = `$ WRITE SYS$OUTPUT "READALL=", F$PRIVILEGE("READALL"), " GRPPRV=", F$PRIVILEGE("GRPPRV"), ` +
		`" SYSPRV=", F$PRIVILEGE("SYSPRV"), " BYPASS=", F$PRIVILEGE("BYPASS")`
	restorePrivs = "$ SET PROCESS/PRIVILEGES=(READALL,GRPPRV,SYSPRV,BYPASS)"
)

// authorize is the AUTHORIZE session that makes the accounts and their
// identifiers.
var authorize = []string{
	"ADD/IDENTIFIER PROT93G2/VALUE=UIC=[200,177777]",
	"ADD/IDENTIFIER PROT93G3/VALUE=UIC=[300,177777]",
	account("PROT93A", "[200,1]", "TMPMBX,NETMBX"),
	account("PROT93B", "[200,2]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
	account("PROT93C", "[300,1]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
	"ADD/IDENTIFIER PROT93_TEAM",
	"GRANT/IDENTIFIER PROT93_TEAM PROT93C",
	"SHOW PROT93A/BRIEF",
	"SHOW PROT93B/BRIEF",
	"SHOW PROT93C/BRIEF",
	"SHOW/IDENTIFIER PROT93*",
	"EXIT",
}

// account is AUTHORIZE's ADD of one account: the password never expires,
// the default privileges are only TMPMBX and NETMBX, and privs are the ones
// it may turn on.
func account(name, uic, privs string) string {
	return fmt.Sprintf("ADD %s/UIC=%s/PASSWORD=PROT93PW/NOPWDEXPIRED/PWDLIFETIME=NONE/FLAGS=NODISUSER"+
		"/DEVICE=%s/DIRECTORY=[PROT93LOG]/PRIVILEGES=(%s)/DEFPRIVILEGES=(TMPMBX,NETMBX)/ACCOUNT=PROT93"+
		"/OWNER=\"PHASE 93 PROBE\"", name, uic, disk, privs)
}

// buildSystem is prot93.com, SYSTEM's run.
func buildSystem() string {
	var p proc

	p.line("$ ! PROT93.COM - Phase 93's probe of file protection: SYSTEM's run.")
	p.line("$ ! Written by testdata/prot93/gen.go. Run it as SYSTEM, from the exchange")
	p.line("$ ! volume, mounted /SYSTEM (README.md):")
	p.line("$ !")
	p.line("$ !     @DUA1:[000000]PROT93")
	p.line("$ !")
	p.line("$ ! It defines PROT93$DISK in the system table for the volume, makes the")
	p.line("$ ! accounts PROT93A, PROT93B, and PROT93C, builds the tree, and runs")
	p.line("$ ! SYSTEM's parts. Its log is PROT93$DISK:[PROT93LOG]PROT93.LOG.")
	p.line("$ !")
	p.line("$ IF P1 .EQS. \"LOGGED\" THEN GOTO BEGIN")
	p.line("$ HERE = F$ENVIRONMENT(\"PROCEDURE\")")
	p.line("$ DEV = F$PARSE(HERE,,,\"DEVICE\")")
	p.line("$ DEFINE/SYSTEM/EXECUTIVE_MODE PROT93$DISK 'DEV'")
	p.line("$ LOGFILE = DEV + \"[PROT93LOG]PROT93.LOG\"")
	p.line("$ @'HERE'/OUTPUT='LOGFILE' LOGGED")
	p.line("$ WRITE SYS$OUTPUT \"PROT93 finished; the log is \", LOGFILE")
	p.line("$ EXIT")
	p.line("$ BEGIN:")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")

	environment(&p)
	volume(&p)
	accounts(&p)
	build(&p)
	tree(&p)
	profiles(&p)
	defaultProtection(&p)
	setSecurity(&p)
	malformed(&p)
	acls(&p)
	display(&p)
	systemProgram(&p)

	p.section("PART B AS SYSTEM WITHOUT PRIVILEGES")
	p.line(dropPrivs)
	p.line(showPrivs)
	p.line("$ @%s[000000]PROT93B M0", disk)
	p.line(restorePrivs)

	p.section("END")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line("$ WRITE SYS$OUTPUT \"PROT93 FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// environment shows the system, SYSTEM's identity, and its defaults.
func environment(p *proc) {
	p.section("ENVIRONMENT")
	p.line("$ WRITE SYS$OUTPUT F$GETSYI(\"VERSION\"), \" \", F$GETSYI(\"HW_NAME\"), \" \", F$GETSYI(\"NODENAME\")")
	p.line("$ SHOW LOGICAL PROT93$DISK")
	p.line("$ WRITE SYS$OUTPUT F$GETJPI(\"\",\"USERNAME\"), \" \", F$GETJPI(\"\",\"UIC\")")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line("$ SHOW PROTECTION")
	p.line("$ WRITE SYS$OUTPUT F$ENVIRONMENT(\"PROTECTION\")")
	p.line("$ SAVED_PROTECTION = F$ENVIRONMENT(\"PROTECTION\")")
	p.line("$ SHOW SECURITY/CLASS=VOLUME %s", disk)
	p.line("$ DIRECTORY/SECURITY %s[000000]", disk)
}

// volume opens the volume to every user: its own protection, the MFD's,
// the log directory's, and the procedures' and the program's files.
func volume(p *proc) {
	p.section("VOLUME")
	p.line("$ SET VOLUME/PROTECTION=(S:RWCD,O:RWCD,G:RWCD,W:RWCD) %s", disk)
	p.line("$ SET SECURITY/PROTECTION=%s %s[000000]000000.DIR", allDir, disk)
	p.line("$ SET SECURITY/PROTECTION=%s %s[000000]PROT93LOG.DIR", allDir, disk)
	p.line("$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) %s[000000]*.*;*/EXCLUDE=(*.DIR)", disk)
	p.line("$ SHOW SECURITY/CLASS=VOLUME %s", disk)
	p.line("$ DIRECTORY/SECURITY %s[000000]", disk)
}

// accounts makes the probe's accounts and identifiers.
func accounts(p *proc) {
	p.section("ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range authorize {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT F$IDENTIFIER(\"PROT93A\",\"NAME_TO_NUMBER\")")
	p.line("$ WRITE SYS$OUTPUT F$IDENTIFIER(\"PROT93_TEAM\",\"NAME_TO_NUMBER\")")
}

// build assembles and links the program and the image Part B runs, and
// assembles the definition modules.
func build(p *proc) {
	p.section("BUILD")
	p.line("$ SET DEFAULT %s[000000]", disk)
	p.line("$ DEFINE/USER_MODE SYS$OUTPUT PROT93.MLG")
	p.line("$ MACRO/NOLIST PROT93")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ DEFINE/USER_MODE SYS$OUTPUT PROT93.LLG")
	p.line("$ LINK/NOMAP PROT93")
	p.line("$ SHOW SYMBOL $STATUS")
	p.line("$ IF F$SEARCH(\"PROT93.EXE\") .EQS. \"\" THEN EXIT")
	p.line("$ MACRO/NOLIST HELLO")
	p.line("$ LINK/NOMAP HELLO")
	p.line("$ SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE) PROT93.EXE,HELLO.EXE")

	for _, d := range defModules {
		p.line("$ MACRO/NOLIST DEF_%s", d)
	}

	p.line("$ DIRECTORY/SIZE DEF_*.OBJ")
}

// tree makes the files and directories every part uses.
func tree(p *proc) {
	p.section("TREE")
	p.line("$ CREATE %s[PROT93LOG]T.DAT", disk)
	p.line("RECORD ONE OF THE TEMPLATE")
	p.line("$ SET SECURITY/PROTECTION=%s %s[PROT93LOG]T.DAT", stdProt, disk)
	p.line("$ CREATE/DIRECTORY %s[PROT93]", disk)

	var dirs []object

	mk := func(path, owner, prot string, acl []string) {
		p.line("$ CREATE/DIRECTORY %s[PROT93.%s]", disk, path)
		dirs = append(dirs, object{path, owner, prot, acl})
	}

	mk("F", ownerA, allDir, nil)

	for _, d := range accessDirs {
		mk(d.name, d.owner, d.prot, d.acl)
	}

	for _, x := range paths {
		mk(x.top, ownerA, x.px, nil)
		mk(x.top+".PY", ownerA, x.py, nil)
		mk(x.top+".PY.PZ", ownerA, x.pz, nil)
	}

	for _, d := range otherDirs {
		mk(d.name, d.owner, d.prot, d.acl)
	}

	p.line("$ !")
	p.line("$ ! [PROT93.F]: the access matrix.")

	for _, f := range fFiles {
		p.file(spec("F", f.name+".DAT"), f.owner, f.prot, f.acl)
	}

	p.line("$ !")
	p.line("$ ! Directories of one access each, and the paths.")

	for _, d := range accessDirs {
		p.file(spec(d.name, "A.DAT"), ownerA, all, nil)
	}

	for _, x := range paths {
		p.file(spec(x.top+".PY.PZ", "X.DAT"), ownerA, "(S:R,O:R,G:R,W:R)", nil)
	}

	p.line("$ !")
	p.line("$ ! Each identity's own files to change.")

	for _, t := range tags {
		perTag(p, t)
	}

	p.line("$ !")
	p.line("$ ! Part B's files: the utilities' inputs, the images, the procedures.")

	for _, u := range uProts {
		p.file(spec("U", "U"+u.name+".DAT"), ownerA, u.prot, nil)
		p.line("$ COPY %s[000000]HELLO.MAR %s", disk, spec("U", "UMAC"+u.name+".MAR"))
		p.security(spec("U", "UMAC"+u.name+".MAR"), ownerA, u.prot, nil)
		p.line("$ COPY %s[000000]HELLO.OBJ %s", disk, spec("U", "ULNK"+u.name+".OBJ"))
		p.security(spec("U", "ULNK"+u.name+".OBJ"), ownerA, u.prot, nil)
		p.line("$ LIBRARY/CREATE/OBJECT %s %s[000000]HELLO.OBJ", spec("U", "ULIB"+u.name+".OLB"), disk)
		p.security(spec("U", "ULIB"+u.name+".OLB"), ownerA, u.prot, nil)
	}

	p.file(spec("UDN", "A.DAT"), ownerA, all, nil)
	p.file(spec("UDE", "A.DAT"), ownerA, all, nil)

	for _, im := range imgProts {
		p.line("$ COPY %s[000000]HELLO.EXE %s", disk, spec("IMG", "HELLO"+im.name+".EXE"))
		p.security(spec("IMG", "HELLO"+im.name+".EXE"), ownerA, im.prot, nil)
		p.line("$ CREATE %s", spec("IMG", "PROC"+im.name+".COM"))
		p.line("$ WRITE SYS$OUTPUT \"PROC%s RAN\"", im.name)
		p.security(spec("IMG", "PROC"+im.name+".COM"), ownerA, im.prot, nil)
	}

	p.line("$ !")
	p.line("$ ! The directories' own security, last.")

	for _, d := range dirs {
		p.security(dirFile(d.name), d.owner, d.prot, d.acl)
	}

	p.line("$ !")
	p.line("$ ! File IDs for the opens by file ID, which pass over directories.")
	p.line("$ OPEN/WRITE FIDF %s", fidCases)

	for _, f := range []struct{ file, fac string }{
		{spec("DN", "A.DAT"), "G"},
		{spec("F", "FNONE.DAT"), "G"},
		{spec("F", "FR.DAT"), "G"},
		{spec("F", "FALL.DAT"), "GP"},
		{spec("PX1.PY.PZ", "X.DAT"), "G"},
	} {
		p.line("$ FID = F$FILE_ATTRIBUTES(\"%s\",\"FID\") - \"(\" - \")\"", f.file)
		p.line("$ FA = F$ELEMENT(0,\",\",FID)")
		p.line("$ FB = F$ELEMENT(1,\",\",FID)")
		p.line("$ FC = F$ELEMENT(2,\",\",FID)")
		p.line("$ WRITE FIDF \"I- %s[000000]|''FA'|''FB'|''FC'|%s\"", disk, f.fac)
	}

	p.line("$ CLOSE FIDF")
	p.line("$ SET SECURITY/PROTECTION=%s %s", stdProt, fidCases)
	p.line("$ TYPE %s", fidCases)
	p.line("$ DIRECTORY/SECURITY %s[PROT93...]", disk)
}

// perTag makes one identity's files: what the program's changing cases,
// and Part B's, delete, rename, supersede, and change.
func perTag(p *proc, t string) {
	nv := func(n string) string { return spec("NV", n+"_"+t+".DAT") }

	p.file(nv("V"), ownerA, stdProt, nil)
	p.file(nv("W"), ownerA, all, nil)
	p.file(nv("L")+";1", ownerA, noDel, nil)
	p.line("$ SET FILE/VERSION_LIMIT=2 %s;1", nv("L"))
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, nv("L"))
	p.security(nv("L")+";2", ownerA, all, nil)
	p.file(nv("SUP"), ownerA, noDel, nil)

	p.file(spec("ER", "EA_"+t+".DAT"), ownerA, all, nil)
	p.file(spec("ER", "EB_"+t+".DAT"), ownerA, noDel, nil)
	p.file(spec("ER", "QD_"+t+".DAT"), ownerA, all, nil)
	p.file(spec("ERX", "EC_"+t+".DAT"), ownerA, all, nil)

	p.file(spec("RN", "RA_"+t+".DAT"), ownerA, all, nil)
	p.file(spec("RN", "RB_"+t+".DAT"), ownerA, noDel, nil)
	p.file(spec("RN", "RC_"+t+".DAT"), ownerA, all, nil)
	p.file(spec("RN", "RD_"+t+".DAT"), ownerA, "(S:R,O:R,G:R,W:R)", nil)
	p.file(spec("RNX", "RE_"+t+".DAT"), ownerA, all, nil)

	control := []string{"IDENTIFIER=PROT93C,ACCESS=CONTROL"}
	p.file(spec("CTL", "CT_"+t+".DAT"), ownerA, stdProt, nil)
	p.file(spec("CTL", "CC_"+t+".DAT"), ownerA, stdProt, control)
	p.file(spec("CTL", "CO_"+t+".DAT"), ownerA, stdProt, nil)
	p.file(spec("CTL", "DCT_"+t+".DAT"), ownerA, stdProt, nil)
	p.file(spec("CTL", "DCC_"+t+".DAT"), ownerA, stdProt, control)
	p.file(spec("CTL", "DCO_"+t+".DAT"), ownerA, stdProt, nil)

	p.file(spec("OWN", "NVO_"+t+".DAT"), ownerA, all, nil)
	p.file(spec("UDNW", "DLW_"+t+".DAT"), ownerA, all, nil)

	for _, u := range uProts {
		for _, k := range uKinds {
			f := spec("U", k+"_"+u.name+"_"+t+".DAT")
			p.file(f, ownerA, u.prot, nil)

			if k == "PUR" {
				p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, f)
				p.security(f, ownerA, u.prot, nil)
			}
		}
	}
}

// profiles is what DCL's commands give new files, as SYSTEM with its
// privileges: an owner's directory, previous versions, explicit values, new
// directories.
func profiles(p *proc) {
	p.section("PROFILES OF NEW FILES")
	prf := disk + "[PROT93.PRF]"
	t := disk + "[PROT93LOG]T.DAT"

	p.line("$ SET DEFAULT %s", prf)
	p.line("$ CREATE P1.DAT")
	p.line("RECORD")
	p.line("$ COPY %s P2.DAT", spec("F", "FROT1.DAT"))
	p.line("$ COPY/PROTECTION=(W:R) %s P3.DAT", t)
	p.line("$ CREATE/PROTECTION=(S:RWED,O:RWED,G,W) P4.DAT")
	p.line("RECORD")
	p.line("$ CREATE/OWNER_UIC=[300,1] P5.DAT")
	p.line("RECORD")
	p.line("$ APPEND/NEW_VERSION %s P6.DAT", t)
	p.line("$ DIRECTORY/SECURITY %s", prf)
	p.line("$ SET SECURITY/PROTECTION=(W:RWED)/OWNER=[300,1] P1.DAT")
	p.line("$ COPY %s P1.DAT", t)
	p.line("$ CREATE P1.DAT")
	p.line("RECORD")
	p.line("$ DIRECTORY/SECURITY P1.DAT;*")
	p.line("$ RENAME P4.DAT P4R.DAT")
	p.line("$ DIRECTORY/SECURITY P4R.DAT")
	p.line("$ CREATE/DIRECTORY [.SUB1]")
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=PARENT [.SUB2]")
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=[300,1] [.SUB3]")
	p.line("$ CREATE/DIRECTORY/PROTECTION=(W:RE) [.SUB4]")
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=[300,1]/PROTECTION=(G:RWE) [.SUB5]")
	p.line("$ DIRECTORY/SECURITY %s*.DIR", prf)
	p.line("$ CREATE/DIRECTORY %s[PROT93TOP]", disk)
	p.line("$ CREATE/DIRECTORY %s[300001]", disk)
	p.line("$ CREATE/DIRECTORY %s[200002.SUB]", disk)
	p.line("$ DIRECTORY/SECURITY %s[000000]*.DIR", disk)
	p.line("$ DIRECTORY/SECURITY %s[200002]*.DIR", disk)
	p.line("$ !")
	p.line("$ ! The directory with a default protection ACE and DEFAULT ACEs.")
	p.line("$ SHOW SECURITY %s", dirFile("DP"))
	p.line("$ CREATE %s", spec("DP", "S1.DAT"))
	p.line("RECORD")
	p.line("$ COPY %s %s", t, spec("DP", "S2.DAT"))
	p.line("$ CREATE/PROTECTION=(W:RE) %s", spec("DP", "S3.DAT"))
	p.line("RECORD")
	p.line("$ CREATE/DIRECTORY %s[PROT93.DP.SUB]", disk)
	p.line("$ CREATE %s[PROT93.DP.SUB]S4.DAT", disk)
	p.line("RECORD")
	p.line("$ SHOW SECURITY %s,%s,%s", spec("DP", "S1.DAT"), spec("DP", "S2.DAT"), spec("DP", "S3.DAT"))
	p.line("$ SHOW SECURITY %s", dirFile("DP.SUB"))
	p.line("$ SHOW SECURITY %s[PROT93.DP.SUB]S4.DAT", disk)
	p.line("$ !")
	p.line("$ ! A file of another owner's, given a new version by SYSTEM.")
	p.line("$ COPY %s %s", t, spec("OWN", "SYS1.DAT"))
	p.line("$ SET SECURITY/OWNER=[300,1]/PROTECTION=(S:RWED,O:RWED,G:R,W:R) %s", spec("OWN", "SYS1.DAT"))
	p.line("$ COPY %s %s", t, spec("OWN", "SYS1.DAT"))
	p.line("$ DIRECTORY/SECURITY %s", spec("OWN", "SYS1.DAT;*"))
	p.line("$ SET DEFAULT %s[000000]", disk)
}

// defaultProtection is SET PROTECTION/DEFAULT, SHOW PROTECTION, and
// F$ENVIRONMENT, with a file made after each change.
func defaultProtection(p *proc) {
	p.section("DEFAULT PROTECTION")

	for i, code := range []string{"(G:RWED,W:R)", "(S:R)", "(W)", "", "(SYSTEM:RWED,OWNER:RWED,GROUP:RE,WORLD)", "(s:rwe,o:rwe)", "(S=RWED,O=RWED,G=RE,W=RE)"} {
		if code == "" {
			p.line("$ SET PROTECTION/DEFAULT")
		} else {
			p.line("$ SET PROTECTION=%s/DEFAULT", code)
		}

		p.line("$ SHOW PROTECTION")
		p.line("$ WRITE SYS$OUTPUT F$ENVIRONMENT(\"PROTECTION\")")
		p.line("$ CREATE %s", spec("DEFP", fmt.Sprintf("D%d.DAT", i+1)))
		p.line("RECORD")
		p.line("$ DIRECTORY/SECURITY %s", spec("DEFP", fmt.Sprintf("D%d.DAT", i+1)))
	}

	p.line("$ SET PROTECTION=('SAVED_PROTECTION')/DEFAULT")
	p.line("$ SHOW PROTECTION")
	p.line("$ CREATE/DIRECTORY %s[PROT93.DEFP.SUB]", disk)
	p.line("$ SHOW SECURITY %s", dirFile("DEFP.SUB"))
}

// setSecurity is SET SECURITY and its older forms on SYSTEM's own files.
func setSecurity(p *proc) {
	p.section("SET SECURITY")
	sec := func(n string) string { return spec("SEC", n) }

	for i := 1; i <= 14; i++ {
		p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, sec(fmt.Sprintf("S%d.DAT", i)))
	}

	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, sec("S9.DAT"))
	p.line("$ DIRECTORY/SECURITY %s", sec("*.*;*"))

	for _, c := range []string{
		"SET SECURITY/PROTECTION=(G:RWED) " + sec("S1.DAT"),
		"SET SECURITY/PROTECTION=(S:RWED,O:RWED,G:RE,W:RE)/LOG " + sec("S2.DAT"),
		"SET SECURITY/OWNER=[200,1] " + sec("S3.DAT"),
		"SET SECURITY/OWNER=PROT93A " + sec("S4.DAT"),
		"SET SECURITY/OWNER=[PROT93G2,PROT93B] " + sec("S5.DAT"),
		"SET SECURITY/OWNER=PROT93_TEAM " + sec("S5.DAT"),
		"SET SECURITY/PROTECTION=(W:RWED)/OWNER=[300,1] " + sec("S6.DAT"),
		"SET SECURITY/DEFAULT/LOG " + sec("S6.DAT"),
		"SET SECURITY/LIKE=NAME=" + sec("S2.DAT") + " " + sec("S7.DAT"),
		"SET SECURITY/LIKE=NAME=" + sec("S3.DAT") + "/COPY_ATTRIBUTE=OWNER " + sec("S8.DAT"),
		"SET SECURITY/PROTECTION=(W:RE) " + sec("S9.DAT;*"),
		"SET FILE/PROTECTION=(W:RW) " + sec("S10.DAT"),
		"SET FILE/OWNER_UIC=[300,1] " + sec("S10.DAT"),
		"SET FILE/OWNER_UIC " + sec("S11.DAT"),
		"SET PROTECTION=(W:RWE) " + sec("S12.DAT"),
		"SET PROTECTION " + sec("S13.DAT"),
		"SET ACL/ACL=(IDENTIFIER=PROT93A,ACCESS=READ) " + sec("S14.DAT"),
		"SHOW ACL " + sec("S14.DAT"),
		"SET SECURITY/PROTECTION=(W:R)/EXCLUDE=S1.DAT/LOG " + sec("S1*.DAT"),
		"SET SECURITY/PROTECTION=(W:E)/BY_OWNER=[200,1]/LOG " + sec("S*.DAT"),
		"SET SECURITY/CLASS=FILE/PROTECTION=(G:R) " + sec("S1.DAT"),
		"SET SECURITY/PROTECTION=(W:R) " + sec("NOSUCH.DAT"),
		"SET SECURITY/PROTECTION=(W:R) " + disk + "[PROT93.NOSUCH]X.DAT",
		"SET SECURITY/DEFAULT " + dirFile("SEC"),
	} {
		p.line("$ %s", c)
	}

	p.line("$ SHOW SECURITY %s", sec("S*.DAT;*"))
	p.line("$ SHOW SECURITY %s", dirFile("SEC"))
	p.line("$ !")
	p.line("$ ! The lexical functions' items.")

	for _, item := range []string{"PRO", "UIC", "GRP", "MBM", "FID", "DID"} {
		p.line("$ WRITE SYS$OUTPUT \"%s=\", F$FILE_ATTRIBUTES(\"%s\",\"%s\")", item, sec("S3.DAT"), item)
	}

	p.line("$ WRITE SYS$OUTPUT \"PRO=\", F$FILE_ATTRIBUTES(\"%s\",\"PRO\")", spec("F", "FNONE.DAT"))
	p.line("$ WRITE SYS$OUTPUT \"PRO=\", F$FILE_ATTRIBUTES(\"%s\",\"PRO\")", spec("F", "FROT1.DAT"))
	p.line("$ WRITE SYS$OUTPUT \"UIC=\", F$FILE_ATTRIBUTES(\"%s\",\"UIC\")", spec("F", "FUIC0.DAT"))
	p.line("$ WRITE SYS$OUTPUT F$PRIVILEGE(\"BYPASS,SYSPRV\")")
}

// malformed is the messages for protection codes, UICs, and identifiers
// that aren't right, and the forms that are.
func malformed(p *proc) {
	p.section("MALFORMED AND ODD VALUES")
	s1 := spec("SEC", "S1.DAT")

	for _, code := range []string{
		"(X:R)", "(S:RQ)", "(S:R,S:W)", "(SYSTEM=RW)", "(SYS:RWED,OWN:RWED,GRO:RE,WOR)",
		"(s:rwed)", "(S: RWED)", "(S:RWED,)", "S:R", "(S:RRR)", "(W:RE,G:RE)", "()",
		"(S:RWEDC)", "(SY:R)", "(SYSTEMS:R)", "(S:RW,O:RW,G:RW,W:RW,X:R)",
	} {
		p.line("$ SET SECURITY/PROTECTION=%s %s", code, s1)
		p.line("$ WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"%s\",\"PRO\")", s1)
	}

	for _, uic := range []string{"[1,99999]", "[400000,1]", "NOSUCH", "[NOSUCH]", "[1]", "[1,4,5]", "1,4", "[PROT93G2,*]", "[200,1"} {
		p.line("$ SET SECURITY/OWNER=%s %s", uic, s1)
	}

	p.line("$ SET SECURITY/OWNER=[1,4]/PROTECTION=(S:RWED,O:RWED,G:RE,W) %s", s1)
	p.line("$ CREATE/PROTECTION=(Q:R) %s", spec("SEC", "M1.DAT"))
	p.line("RECORD")
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=[NOSUCH] %s[PROT93.SEC.M2]", disk)
	p.line("$ CREATE/DIRECTORY/PROTECTION=(S:Z) %s[PROT93.SEC.M3]", disk)
	p.line("$ SET PROTECTION=(S:Z)/DEFAULT")
	p.line("$ SHOW PROTECTION")
	p.line("$ COPY/PROTECTION=(S:,) %s[PROT93LOG]T.DAT %s", disk, spec("SEC", "M4.DAT"))
	p.line("$ SET FILE/PROTECTION=(Q) %s", s1)
	p.line("$ SET PROTECTION=(Q:R) %s", s1)
	p.line("$ SHOW SECURITY %s", s1)
}

// acls is SET SECURITY/ACL: every ACE form on a file of its own (each
// file's bytes dumped by the program), the positional qualifiers, and the
// rules for protected and nopropagate ACEs.
func acls(p *proc) {
	p.section("ACLS")

	for _, a := range aceFiles {
		target := spec("ACE", a.name+".DAT")

		if a.dir {
			p.line("$ CREATE/DIRECTORY %s[PROT93.ACE.%s]", disk, a.name)
			target = dirFile("ACE." + a.name)
		} else {
			p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, target)
		}

		p.line("$ SET SECURITY/ACL=(%s) %s", joinACEs(a.acl), target)
		p.line("$ SHOW SECURITY %s", target)
	}

	m := spec("ACE", "M.DAT")
	p.line("$ !")
	p.line("$ ! Changing one ACL step by step.")
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, m)

	for _, c := range []string{
		"/ACL=(IDENTIFIER=PROT93A,ACCESS=READ)",
		"/ACL=(IDENTIFIER=PROT93B,ACCESS=READ)",
		"/ACL=(IDENTIFIER=PROT93C,ACCESS=READ)/AFTER=(IDENTIFIER=PROT93B,ACCESS=READ)",
		"/ACL=(IDENTIFIER=PROT93C,ACCESS=READ)/REPLACE=(IDENTIFIER=PROT93C,ACCESS=WRITE)",
		"/ACL=(IDENTIFIER=PROT93A,OPTIONS=PROTECTED,ACCESS=EXECUTE)",
		"/ACL=(IDENTIFIER=PROT93A,OPTIONS=NOPROPAGATE,ACCESS=DELETE)",
		"/ACL=(IDENTIFIER=PROT93B,ACCESS=READ)/DELETE",
		"/ACL=(IDENTIFIER=PROT93A,ACCESS=READ)",
		"/ACL=(IDENTIFIER=PROT93B,ACCESS=WRITE)/DELETE",
		"/ACL/DELETE",
		"/ACL=(IDENTIFIER=PROT93C,ACCESS=READ)",
		"/ACL/DELETE=ALL",
		"/ACL=(IDENTIFIER=PROT93A,ACCESS=READ)/DELETE",
		"/ACL=((IDENTIFIER=PROT93A,ACCESS=READ),(IDENTIFIER=PROT93B,ACCESS=WRITE),(IDENTIFIER=PROT93C,ACCESS=EXECUTE))",
		"/ACL=(IDENTIFIER=PROT93C,ACCESS=READ)/DELETE=ALL",
	} {
		p.line("$ SET SECURITY%s %s", c, m)
		p.line("$ SHOW SECURITY %s", m)
	}

	p.line("$ !")
	p.line("$ ! /LIKE and /DEFAULT with protected and nopropagate ACEs.")
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("ACE", "L1.DAT"))
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("ACE", "L2.DAT"))
	p.line("$ SET SECURITY/ACL=((IDENTIFIER=PROT93A,OPTIONS=PROTECTED,ACCESS=READ),(IDENTIFIER=PROT93B,OPTIONS=NOPROPAGATE,ACCESS=READ),(IDENTIFIER=PROT93C,ACCESS=READ)) %s", spec("ACE", "L1.DAT"))
	p.line("$ SET SECURITY/ACL=(IDENTIFIER=PROT93_TEAM,OPTIONS=PROTECTED,ACCESS=WRITE) %s", spec("ACE", "L2.DAT"))
	p.line("$ SET SECURITY/LIKE=NAME=%s %s", spec("ACE", "L1.DAT"), spec("ACE", "L2.DAT"))
	p.line("$ SHOW SECURITY %s,%s", spec("ACE", "L1.DAT"), spec("ACE", "L2.DAT"))
	p.line("$ SET SECURITY/DEFAULT %s", spec("ACE", "L1.DAT"))
	p.line("$ SHOW SECURITY %s", spec("ACE", "L1.DAT"))
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("ACE", "L1.DAT"))
	p.line("$ SHOW SECURITY %s", spec("ACE", "L1.DAT"))
	p.line("$ COPY %s %s", spec("ACE", "L2.DAT"), spec("ACE", "L3.DAT"))
	p.line("$ SHOW SECURITY %s", spec("ACE", "L3.DAT"))
	p.line("$ !")
	p.line("$ ! Malformed ACEs.")

	for _, a := range []string{
		"(IDENTIFIER=NOSUCH,ACCESS=READ)", "(IDENTIFIER=PROT93A,ACCESS=FLY)", "(FOO=BAR)",
		"(IDENTIFIER=PROT93A,ACCESS=READ", "(IDENTIFIER=PROT93A)", "(DEFAULT_PROTECTION,S:Q)",
		"(IDENTIFIER=PROT93A,OPTIONS=WHATEVER,ACCESS=READ)",
	} {
		p.line("$ SET SECURITY/ACL=%s %s", a, m)
	}

	p.line("$ SHOW SECURITY %s", m)
	p.line("$ DIRECTORY/ACL %s", spec("ACE", "*.*"))
	p.line("$ DIRECTORY/SECURITY %s", spec("ACE", "*.*"))
	p.line("$ !")
	p.line("$ ! The ACLs' bytes, as $DISPLAY's XABPRO returns them.")
	p.run("M4", aceCases)
}

// display is DIRECTORY's and SHOW SECURITY's layouts of the access matrix.
func display(p *proc) {
	p.section("DISPLAY")
	f := spec("F", "")

	for _, q := range []string{"/OWNER", "/PROTECTION", "/OWNER/PROTECTION", "/SECURITY", "/ACL", "/FULL"} {
		p.line("$ DIRECTORY%s %s", q, f)
	}

	p.line("$ SHOW SECURITY %s*.*", f)
	p.line("$ SHOW SECURITY %s", dirFile("F"))
	p.line("$ SHOW SECURITY %s", dirFile("DN"))
	p.line("$ DIRECTORY/OWNER/PROTECTION %s[PROT93]", disk)
}

// systemProgram runs the program as SYSTEM: without the four privileges
// (M0, with the changing cases), with READALL alone (M1), and with all four
// (M4).
func systemProgram(p *proc) {
	p.section("PROGRAM AS SYSTEM")
	p.line(dropPrivs)
	p.line(showPrivs)
	p.run("M0", coreCases)
	p.run("M0", changeCases)
	p.run("M0", fidCases)
	p.line("$ SET PROCESS/PRIVILEGES=READALL")
	p.line(showPrivs)
	p.run("M1", coreCases)
	p.run("M1", fidCases)
	p.line(restorePrivs)
	p.line(showPrivs)
	p.run("M4", coreCases)
	p.run("M4", fidCases)
}

// buildUser is prot93u.com, each account's run.
func buildUser() string {
	var p proc

	p.line("$ ! PROT93U.COM - Phase 93's probe of file protection: an account's run.")
	p.line("$ ! Written by testdata/prot93/gen.go. Run it logged in as PROT93A, then")
	p.line("$ ! PROT93B, then PROT93C, after SYSTEM's PROT93.COM (README.md):")
	p.line("$ !")
	p.line("$ !     @PROT93$DISK:[000000]PROT93U")
	p.line("$ !")
	p.line("$ ! Its log is PROT93$DISK:[PROT93LOG]PROT93U_username.LOG.")
	p.line("$ !")
	p.logged("\"PROT93U_\" + F$EDIT(F$GETJPI(\"\",\"USERNAME\"),\"TRIM\") + \".LOG\"", "PROT93U")

	p.section("ENVIRONMENT")
	p.line("$ USER = F$EDIT(F$GETJPI(\"\",\"USERNAME\"),\"TRIM\")")
	p.line("$ LETTER = F$EXTRACT(F$LENGTH(USER)-1,1,USER)")
	p.line("$ T = LETTER + \"0\"")
	p.line("$ WRITE SYS$OUTPUT USER, \" \", F$GETJPI(\"\",\"UIC\"), \" \", T")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line("$ SHOW PROTECTION")
	p.line("$ WRITE SYS$OUTPUT F$ENVIRONMENT(\"PROTECTION\")")
	p.line("$ SHOW DEFAULT")

	p.section("PROGRAM, NO PRIVILEGES")
	p.line(dropPrivs)
	p.run("'T'", coreCases)
	p.run("'T'", changeCases)
	p.run("'T'", fidCases)

	p.section("PROGRAM, EACH PRIVILEGE")
	p.line("$ ! The passes only for an account that may have the privileges.")
	p.line("$ SET PROCESS/PRIVILEGES=BYPASS")
	p.line("$ IF .NOT. F$PRIVILEGE(\"BYPASS\") THEN GOTO NO_PASSES")
	p.line("$ N = 1")
	p.line("$ PASS:")
	p.line("$ PRIV = F$ELEMENT(N,\",\",\"NONE,READALL,GRPPRV,SYSPRV,BYPASS\")")
	p.line("$ IF PRIV .EQS. \",\" THEN GOTO NO_PASSES")
	p.line(dropPrivs)
	p.line("$ SET PROCESS/PRIVILEGES='PRIV'")
	p.line(showPrivs)
	p.line("$ TN = LETTER + F$STRING(N)")
	p.run("'TN'", coreCases)
	p.run("'TN'", fidCases)
	p.line("$ N = N + 1")
	p.line("$ GOTO PASS")
	p.line("$ NO_PASSES:")
	p.line(dropPrivs)

	p.section("PART B")
	p.line("$ @%s[000000]PROT93B 'T'", disk)

	p.section("ACL-GRANTED AND -DENIED FILES")

	for _, f := range fFiles {
		if len(f.acl) > 0 || f.owner == "[0,0]" {
			p.line("$ TYPE %s", spec("F", f.name+".DAT"))
		}
	}

	p.section("CONTROL")
	ctl := func(n string) string { return spec("CTL", n+"_'T'.DAT") }
	p.line("$ SET SECURITY/PROTECTION=(W:RWED) %s", ctl("DCT"))
	p.line("$ SET SECURITY/PROTECTION=(W:RWED) %s", ctl("DCC"))
	p.line("$ SET SECURITY/ACL=(IDENTIFIER=PROT93A,ACCESS=READ) %s", ctl("DCC"))
	p.line("$ ME = F$GETJPI(\"\",\"UIC\")")
	p.line("$ SET SECURITY/OWNER='ME' %s", ctl("DCO"))
	p.line("$ SHOW SECURITY %s", spec("CTL", "D*_'T'.DAT"))
	p.line("$ SET SECURITY/PROTECTION=(W:RE) %s", spec("F", "FACLCTL.DAT"))
	p.line("$ SHOW SECURITY %s", spec("F", "FACLCTL.DAT"))

	p.section("OWN FILES")
	own := disk + "[PROT93LOG]OWN_'T'.DAT"
	p.line("$ CREATE %s", own)
	p.line("RECORD")
	p.line("$ SHOW SECURITY %s", own)
	p.line("$ SET SECURITY/PROTECTION=(W:RWED) %s", own)
	p.line("$ SET SECURITY/OWNER=[300,1] %s", own)
	p.line("$ SET SECURITY/OWNER=[200,2] %s", own)
	p.line("$ SHOW SECURITY %s", own)
	p.line("$ SET PROTECTION=(G:RWED,W:R)/DEFAULT")
	p.line("$ SHOW PROTECTION")
	p.line("$ CREATE %s[PROT93LOG]DEF_'T'.DAT", disk)
	p.line("RECORD")
	p.line("$ SHOW SECURITY %s[PROT93LOG]DEF_'T'.DAT", disk)
	p.line("$ SET PROTECTION=(S:RWED,O:RWED,G:RE,W)/DEFAULT")

	p.section("NEW FILES AND DIRECTORIES IN OTHERS' DIRECTORIES")
	p.line("$ CREATE/DIRECTORY %s[PROT93.OWN.D'T']", disk)
	p.line("$ CREATE/DIRECTORY %s[PROT93.DP.D'T']", disk)
	p.line("$ CREATE/DIRECTORY %s[PROT93.DR.D'T']", disk)
	p.line("$ CREATE %s", spec("DP", "N_'T'.DAT"))
	p.line("RECORD")
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("OWN", "CP_'T'.DAT"))
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("OWN", "NVO_'T'.DAT"))
	p.line("$ SHOW SECURITY %s", dirFile("OWN.D'T'"))
	p.line("$ SHOW SECURITY %s", dirFile("DP.D'T'"))
	p.line("$ SHOW SECURITY %s,%s", spec("DP", "N_'T'.DAT"), spec("OWN", "CP_'T'.DAT"))
	p.line("$ SHOW SECURITY %s", spec("OWN", "NVO_'T'.DAT;*"))

	p.section("END")
	p.line("$ WRITE SYS$OUTPUT \"PROT93U FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// buildUtilities is prot93b.com, Part B: DCL's utilities on files an
// identity may and may not use, and in directories it may and may not use.
// P1 is the tag naming the identity's own files.
func buildUtilities() string {
	var p proc

	p.line("$ ! PROT93B.COM - Phase 93's probe: DCL's utilities and file protection.")
	p.line("$ ! Written by testdata/prot93/gen.go; run by PROT93.COM and PROT93U.COM,")
	p.line("$ ! with P1 the tag of the identity's own files (M0, A0, B0, C0).")
	p.line("$ SET NOON")
	p.line("$ T = P1")
	p.line("$ U = \"%s[PROT93.U]\"", disk)
	p.line("$ OT = \"%s[PROT93LOG]\" + T + \"_\"", disk)
	p.line("$ IMG = \"%s[PROT93.IMG]\"", disk)
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")

	for _, u := range uProts {
		x := u.name
		f := "'U'U" + x + ".DAT"
		p.section("UTILITIES ON U" + x + ".DAT")

		for _, c := range []string{
			"TYPE " + f,
			"COPY " + f + " 'OT'U" + x + ".TMP",
			"DUMP/RECORD " + f,
			"DUMP/HEADER/BLOCKS=COUNT=0 " + f,
			"SEARCH " + f + " RECORD",
			"DIFFERENCES " + f + " 'U'UWLD.DAT",
			"OPEN/READ FU " + f,
			"READ FU REC",
			"SHOW SYMBOL REC",
			"CLOSE FU",
			"WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"''U'U" + x + ".DAT\",\"EOF\")",
			"WRITE SYS$OUTPUT F$FILE_ATTRIBUTES(\"''U'U" + x + ".DAT\",\"PRO\")",
			"WRITE SYS$OUTPUT F$SEARCH(\"''U'U" + x + ".DAT\")",
			"DIRECTORY " + f,
			"DIRECTORY/FULL " + f,
			"DIRECTORY/SECURITY " + f,
			"SHOW SECURITY " + f,
			"ANALYZE/RMS_FILE/OUTPUT=NL: " + f,
			"CONVERT " + f + " 'OT'U" + x + ".CNV",
			"BACKUP " + f + " 'OT'U" + x + ".BCK/SAVE_SET",
			"MACRO/NOLIST/OBJECT='OT'M" + x + ".OBJ 'U'UMAC" + x + ".MAR",
			"LINK/NOMAP/EXECUTABLE='OT'L" + x + ".EXE 'U'ULNK" + x + ".OBJ",
			"LIBRARY/LIST 'U'ULIB" + x + ".OLB",
		} {
			p.line("$ %s", c)
		}

		p.section("CHANGES TO " + x + " FILES")
		k := func(kind string) string { return "'U'" + kind + "_" + x + "_'T'.DAT" }

		for _, c := range []string{
			"DELETE " + k("DEL") + ";*",
			"PURGE " + k("PUR"),
			"RENAME " + k("REN") + " 'U'REN2_" + x + "_'T'.DAT",
			"APPEND 'U'UWLD.DAT " + k("APP"),
			"COPY 'U'UWLD.DAT " + k("CPY"),
			"OPEN/WRITE FU " + k("OPW"),
			"WRITE FU \"WRITTEN\"",
			"CLOSE FU",
			"OPEN/APPEND FU " + k("OPA"),
			"WRITE FU \"APPENDED\"",
			"CLOSE FU",
			"SET SECURITY/PROTECTION=(W:R) " + k("SEC"),
			"SET FILE/PROTECTION=(W:R) " + k("SFI"),
			"SET PROTECTION=(W:R) " + k("SPR"),
			"SET SECURITY/ACL=(IDENTIFIER=PROT93C,ACCESS=READ) " + k("ACL"),
			"SET SECURITY/OWNER=[200,2] " + k("OWN"),
			"DIRECTORY/SECURITY 'U'*_" + x + "_'T'.*;*",
		} {
			p.line("$ %s", c)
		}
	}

	p.section("DIRECTORIES")

	for _, c := range []string{
		"DIRECTORY " + disk + "[PROT93.UDN]",
		"DIRECTORY " + disk + "[PROT93.UDN]A.DAT",
		"TYPE " + disk + "[PROT93.UDN]A.DAT",
		"COPY 'U'UWLD.DAT " + disk + "[PROT93.UDN]N_'T'.DAT",
		"CREATE/DIRECTORY " + disk + "[PROT93.UDN.S'T']",
		"DIRECTORY " + disk + "[PROT93.UDE]",
		"DIRECTORY " + disk + "[PROT93.UDE]*.DAT",
		"DIRECTORY " + disk + "[PROT93.UDE]A.DAT",
		"DIRECTORY/FULL " + disk + "[PROT93.UDE]A.DAT",
		"TYPE " + disk + "[PROT93.UDE]A.DAT",
		"WRITE SYS$OUTPUT F$SEARCH(\"" + disk + "[PROT93.UDE]*.DAT\")",
		"WRITE SYS$OUTPUT F$SEARCH(\"" + disk + "[PROT93.UDE]A.DAT\")",
		"WRITE SYS$OUTPUT F$PARSE(\"" + disk + "[PROT93.UDN]A.DAT\")",
		"DELETE " + disk + "[PROT93.UDNW]DLW_'T'.DAT;*",
		"CREATE " + disk + "[PROT93.UDNW]CR_'T'.DAT",
		"SET DEFAULT " + disk + "[PROT93.UDN]",
		"SHOW DEFAULT",
		"DIRECTORY",
		"SET DEFAULT 'HOME'",
	} {
		p.line("$ %s", c)
	}

	p.section("IMAGES AND PROCEDURES")

	for _, im := range imgProts {
		p.line("$ RUN 'IMG'HELLO%s.EXE", im.name)
		p.line("$ SHOW SYMBOL $STATUS")
		p.line("$ @'IMG'PROC%s.COM", im.name)
		p.line("$ SHOW SYMBOL $STATUS")
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ EXIT")

	return p.String()
}

// buildFinal is prot93z.com: the whole tree as it was left, then the
// accounts removed.
func buildFinal() string {
	var p proc

	p.line("$ ! PROT93Z.COM - Phase 93's probe: the listing at the end, and the")
	p.line("$ ! accounts removed. Written by testdata/prot93/gen.go. Run it as SYSTEM")
	p.line("$ ! after the accounts' runs (README.md):")
	p.line("$ !")
	p.line("$ !     @PROT93$DISK:[000000]PROT93Z")
	p.line("$ !")
	p.logged("\"PROT93Z.LOG\"", "PROT93Z")
	p.section("THE TREE")
	p.line("$ DIRECTORY/SECURITY %s[PROT93...]*.*;*", disk)
	p.line("$ DIRECTORY/SECURITY %s[PROT93LOG]*.*;*", disk)
	p.line("$ DIRECTORY/SECURITY %s[000000]*.*;*", disk)
	p.line("$ DIRECTORY/SECURITY %s[PROT93TOP...]*.*;*,[300001...]*.*;*,[200002...]*.*;*", disk)
	p.section("REMOVING THE ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range []string{
		"REMOVE PROT93A", "REMOVE PROT93B", "REMOVE PROT93C",
		"REMOVE/IDENTIFIER PROT93_TEAM", "REMOVE/IDENTIFIER PROT93G2", "REMOVE/IDENTIFIER PROT93G3",
		"EXIT",
	} {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT \"PROT93Z FINISHED\"")
	p.line("$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PROT93$DISK")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// coreFile is the cases every identity runs in every privilege pass:
// opens, profiles, lookups, searches, the ACP's reads, and new files.
func coreFile() string {
	var b strings.Builder

	add := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	for _, f := range fFiles {
		n := spec("F", f.name+".DAT")
		for _, fac := range []string{"G", "P", "GP", "B"} {
			add("O- %s|%s", n, fac)
		}

		add("A- %s", n)
		add("QA %s", n)
		add("QR %s", n)
		add("QW %s", n)
	}

	for _, d := range accessDirs {
		add("S- %s", spec(d.name, "*.*;*"))
		add("S- %s", spec(d.name, "A.DAT"))
		add("P- %s", spec(d.name, "A.DAT"))
		add("O- %s|G", spec(d.name, "A.DAT"))
		add("QA %s", spec(d.name, "A.DAT"))
		add("C- %s", spec(d.name, "C_@.DAT"))
	}

	for _, x := range paths {
		add("O- %s|G", spec(x.top+".PY.PZ", "X.DAT"))
		add("S- %s", spec(x.top+".PY.PZ", "*.*;*"))
		add("QA %s", spec(x.top+".PY.PZ", "X.DAT"))
	}

	add("C- %s", spec("OWN", "O_@.DAT"))
	add("CX %s|F000|00C80001", spec("OWN", "XA_@.DAT"))
	add("CX %s|F000|00C80002", spec("OWN", "XB_@.DAT"))
	add("CX %s|F000|012C0001", spec("OWN", "XC_@.DAT"))
	add("CX %s|F000|00010004", spec("OWN", "XS_@.DAT"))
	add("CX %s|F000|", spec("OWN", "XZ_@.DAT"))
	add("C- %s", spec("DP", "DP_@.DAT"))

	return b.String()
}

// changeFile is the cases that change an identity's own files, run once
// per identity: new versions, supersedes, erases, renames, and the ACP's
// changes.
func changeFile() string {
	var b strings.Builder

	add := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	add("CI %s", spec("NV", "V_@.DAT"))
	add("C- %s", spec("NV", "V_@.DAT"))
	add("C- %s", spec("NV", "W_@.DAT"))
	add("C- %s", spec("NV", "L_@.DAT"))
	add("CS %s", spec("NV", "SUP_@.DAT;1"))
	add("E- %s", spec("ER", "EA_@.DAT"))
	add("E- %s", spec("ER", "EB_@.DAT"))
	add("E- %s", spec("ERX", "EC_@.DAT"))
	add("R- %s|%s", spec("RN", "RA_@.DAT"), spec("RN", "RA2_@.DAT"))
	add("R- %s|%s", spec("RN", "RB_@.DAT"), spec("RN", "RB2_@.DAT"))
	add("R- %s|%s", spec("RN", "RC_@.DAT"), spec("RNX", "RC2_@.DAT"))
	add("R- %s|%s", spec("RN", "RD_@.DAT"), spec("RN", "RD2_@.DAT"))
	add("R- %s|%s", spec("RNX", "RE_@.DAT"), spec("RN", "RE2_@.DAT"))
	add("QM %s|EE00", spec("CTL", "CT_@.DAT"))
	add("QM %s|EE00", spec("CTL", "CC_@.DAT"))
	add("QM %s|FA00|00C80002", spec("CTL", "CO_@.DAT"))
	add("QD %s", spec("ER", "QD_@.DAT"))
	add("QC %s", spec("DW", "QC_@.DAT"))
	add("QC %s", spec("DR", "QC_@.DAT"))
	add("QC %s", spec("DRW", "QC_@.DAT"))

	for _, n := range []string{"V", "W", "L", "SUP"} {
		add("S- %s", spec("NV", n+"_@.DAT;*"))
	}

	add("A- %s", spec("NV", "V_@.DAT"))
	add("A- %s", spec("NV", "W_@.DAT"))
	add("S- %s", spec("RN", "*_@.DAT;*"))
	add("S- %s", spec("ER", "*_@.DAT;*"))

	return b.String()
}

// aceFile is SYSTEM's dump of each ACE file's ACL.
func aceFile() string {
	var b strings.Builder

	for _, a := range aceFiles {
		if a.dir {
			fmt.Fprintf(&b, "A- %s\n", dirFile("ACE."+a.name))
		} else {
			fmt.Fprintf(&b, "A- %s\n", spec("ACE", a.name+".DAT"))
		}
	}

	fmt.Fprintf(&b, "A- %s\n", dirFile("DP"))
	fmt.Fprintf(&b, "A- %s\n", spec("ACE", "M.DAT"))

	return b.String()
}

const hello = `	.TITLE	HELLO	Phase 93's image to run
; Written by testdata/prot93/gen.go: PROT93B.COM runs copies of it with
; different protections.
	.PSECT	DATA,WRT,NOEXE
MSG:	.ASCID	/HELLO RAN/
	.PSECT	CODE,NOWRT,EXE
	.ENTRY	START,^M<>
	PUSHAQ	MSG
	CALLS	#1,G^LIB$PUT_OUTPUT
	RET
	.END	START
`

// defModule is one definition macro called GLOBAL, for its values.
func defModule(name string) string {
	return fmt.Sprintf("\t.TITLE\tDEF_%s\t$%sDEF's values, in the object's global symbols\n"+
		"; Written by testdata/prot93/gen.go.\n"+
		"\t$%sDEF\tGLOBAL\n"+
		"\t.END\n", name, name, name)
}

// exchangeFiles are the files exchange.cmd copies to the volume's
// [000000].
var exchangeFiles = []string{
	"prot93.mar", "hello.mar", "prot93.com", "prot93u.com", "prot93b.com", "prot93z.com",
	"prot93c.dat", "prot93d.dat", "prot93e.dat",
}

func exchange() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE.CMD - builds Phase 93's probe exchange volume with govax.
! Written by testdata/prot93/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/prot93-exchange.dsk" /DEVICE=RD53 PROT93
MOUNT/WRITE DUA5 "testdata/disks/prot93-exchange.dsk"
CREATE/DIRECTORY DUA5:[PROT93LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
`)

	files := append([]string{}, exchangeFiles...)
	for _, d := range defModules {
		files = append(files, "def_"+strings.ToLower(d)+".mar")
	}

	for _, f := range files {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}

	b.WriteString("DIRECTORY/OWNER/PROTECTION DUA5:[000000]\nDISMOUNT DUA5\n")

	return b.String()
}

func copyout() string {
	var b strings.Builder

	b.WriteString(`! COPYOUT.CMD - copies Phase 93's probe results off prot93-exchange.dsk
! into testdata/prot93/vax (README.md). Written by testdata/prot93/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout.cmd
!
! The program and the definition objects come back byte for byte
! (/BINARY): a later oracle runs VMS's own image, and decode.go reads the
! objects' values.
MOUNT DUA5 "testdata/disks/prot93-exchange.dsk"
`)

	for _, l := range []string{"PROT93.LOG", "PROT93U_PROT93A.LOG", "PROT93U_PROT93B.LOG", "PROT93U_PROT93C.LOG", "PROT93Z.LOG"} {
		fmt.Fprintf(&b, "COPY DUA5:[PROT93LOG]%s \"%s/vax/%s\"/HOST/QUIET\n", l, dir, strings.ToLower(l))
	}

	fmt.Fprintf(&b, "COPY/BINARY DUA5:[000000]PROT93.EXE \"%s/vax/prot93.exe\"/HOST/QUIET\n", dir)

	for _, d := range defModules {
		fmt.Fprintf(&b, "COPY/BINARY DUA5:[000000]DEF_%s.OBJ \"%s/vax/def_%s.obj\"/HOST/QUIET\n", d, dir, strings.ToLower(d))
	}

	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

// Round 2 (README.md's "Round 2"). Round 1's SET SECURITY of PROT93.EXE
// and HELLO.EXE named both files in one command, which VMS 7.1's SET
// SECURITY refuses (NOLIST), so the image kept SYSTEM's default protection
// and no account could run it without a privilege: the accounts' passes
// without privileges (A0, B0, C0) and with GRPPRV (B2, C2) were lost. Round
// 2 runs them on the same volume, whose per-tag files those passes never
// touched, with a few cases round 1 left open. Round 1's procedures are
// left as they ran.

// The round 2 case files.
const (
	core2Cases   = disk + "[000000]PROT93F.DAT"
	change2Cases = disk + "[000000]PROT93G.DAT"
)

// r2Dirs are round 2's new directories: one a lookup without execute can't
// pass, for creating, erasing, and renaming in it and in [.DW] (write and
// execute, no read).
var r2Dirs = []object{
	{"DRWO", ownerA, "(S:RW,O:RW,G:RW,W:RW)", nil},
}

// r2Users are the accounts by tag: each one's UIC and username, for the
// files round 2 makes for it alone.
var r2Users = []struct{ tag, user, uic string }{
	{"M0", "SYSTEM", "[1,4]"},
	{"A0", "PROT93A", "[200,1]"},
	{"B0", "PROT93B", "[200,2]"},
	{"C0", "PROT93C", "[300,1]"},
}

// r2Procs are round 2's command procedures to run, each with a line to
// show that its lines ran (round 1's CREATE took the procedure's $ line as
// the end of its data, so its procedures were empty).
var r2Procs = imgProts

// buildRound2System is prot93r.com: the accounts again, the image's
// protection, round 2's files, SYSTEM's round 2 cases.
func buildRound2System() string {
	var p proc

	p.line("$ ! PROT93R.COM - Phase 93's probe, round 2: SYSTEM's run.")
	p.line("$ ! Written by testdata/prot93/gen.go. Run it as SYSTEM, with round 1's")
	p.line("$ ! exchange volume mounted /SYSTEM (README.md's \"Round 2\"):")
	p.line("$ !")
	p.line("$ !     @DUA1:[000000]PROT93R")
	p.line("$ !")
	p.line("$ ! Its log is PROT93$DISK:[PROT93LOG]PROT93R.LOG.")
	p.line("$ !")
	p.line("$ IF P1 .EQS. \"LOGGED\" THEN GOTO BEGIN")
	p.line("$ HERE = F$ENVIRONMENT(\"PROCEDURE\")")
	p.line("$ DEV = F$PARSE(HERE,,,\"DEVICE\")")
	p.line("$ DEFINE/SYSTEM/EXECUTIVE_MODE PROT93$DISK 'DEV'")
	p.line("$ LOGFILE = DEV + \"[PROT93LOG]PROT93R.LOG\"")
	p.line("$ @'HERE'/OUTPUT='LOGFILE' LOGGED")
	p.line("$ WRITE SYS$OUTPUT \"PROT93R finished; the log is \", LOGFILE")
	p.line("$ EXIT")
	p.line("$ BEGIN:")
	p.line("$ SET NOON")
	p.line("$ SET VERIFY")

	environment(&p)

	p.section("ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range []string{
		account("PROT93A", "[200,1]", "TMPMBX,NETMBX"),
		account("PROT93B", "[200,2]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
		account("PROT93C", "[300,1]", "TMPMBX,NETMBX,READALL,GRPPRV,SYSPRV,BYPASS"),
		// Round 1's value, which the tree's ACEs hold.
		"ADD/IDENTIFIER PROT93_TEAM/VALUE=IDENTIFIER=65537",
		"GRANT/IDENTIFIER PROT93_TEAM PROT93C",
		"SHOW/IDENTIFIER/FULL PROT93",
		"SHOW/IDENTIFIER/FULL PROT93_TEAM",
		"EXIT",
	} {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT F$IDENTIFIER(\"PROT93_TEAM\",\"NAME_TO_NUMBER\")")

	p.section("IMAGES")
	p.line("$ ! Round 1 named both images in one SET SECURITY, which 7.1 refuses.")

	for _, f := range []string{"PROT93.EXE", "HELLO.EXE"} {
		p.line("$ SET SECURITY/PROTECTION=%s %s[000000]%s", stdProt, disk, f)
		p.line("$ SHOW SECURITY %s[000000]%s", disk, f)
	}

	p.section("ROUND 2 TREE")
	p.line("$ ! FACLCTL as round 1 made it: its owner and PROT93C changed it in Part B.")
	p.security(spec("F", "FACLCTL.DAT"), "", none, nil)

	for _, d := range r2Dirs {
		p.line("$ CREATE/DIRECTORY %s[PROT93.%s]", disk, d.name)
		p.file(spec(d.name, "A.DAT"), ownerA, all, nil)
		p.security(dirFile(d.name), d.owner, d.prot, d.acl)
	}

	for _, u := range r2Users {
		t := u.tag
		p.file(spec("DW", "E2_"+t+".DAT"), ownerA, all, nil)
		p.file(spec("DW", "N2_"+t+".DAT"), ownerA, all, nil)
		p.file(spec("DRWO", "E3_"+t+".DAT"), ownerA, all, nil)
		p.file(spec("DRWO", "N3_"+t+".DAT"), ownerA, all, nil)
		p.file(spec("NV", "NP_"+t+".DAT"), ownerA, all, []string{
			"IDENTIFIER=PROT93A,OPTIONS=NOPROPAGATE,ACCESS=READ",
			"IDENTIFIER=PROT93_TEAM,ACCESS=READ",
			"IDENTIFIER=PROT93B,OPTIONS=PROTECTED+NOPROPAGATE,ACCESS=READ",
		})
		p.file(spec("NV", "SUPD_"+t+".DAT"), ownerA, "(S:RD,O:RD,G:RD,W:RD)", nil)
		p.file(spec("NV", "SUPW_"+t+".DAT"), ownerA, all, nil)
		p.file(spec("NV", "NVR_"+t+".DAT"), ownerA, "(S:R,O:R,G:R,W:R)", nil)
		p.file(spec("CTL", "OS_"+t+".DAT"), u.uic, stdProt, nil)
		p.file(spec("CTL", "OT_"+t+".DAT"), ownerA, stdProt,
			[]string{"IDENTIFIER=" + u.user + ",ACCESS=CONTROL"})
	}

	for _, im := range r2Procs {
		f := spec("IMG", "PRO2"+im.name+".COM")
		p.line("$ OPEN/WRITE PF %s", f)
		p.line("$ WRITE PF \"$ WRITE SYS$OUTPUT \"\"PRO2%s RAN\"\"\"", im.name)
		p.line("$ CLOSE PF")
		p.security(f, ownerA, im.prot, nil)
	}

	p.line("$ DIRECTORY/SECURITY %s[PROT93...]*2*.*;*,*_M0.*;*", disk)

	p.section("OBSOLETE FORMS WITH NO VALUE")
	sp := func(n string) string { return spec("SEC", n) }
	p.file(sp("SP1.DAT"), ownerA, all, nil)
	p.line("$ SET PROTECTION %s", sp("SP1.DAT"))
	p.line("$ SHOW SECURITY %s", sp("SP1.DAT"))
	p.file(sp("SP2.DAT"), ownerA, all, nil)
	p.line("$ SET FILE/OWNER_UIC %s", sp("SP2.DAT"))
	p.line("$ SHOW SECURITY %s", sp("SP2.DAT"))
	p.file(sp("SP3.DAT"), ownerA, all, nil)
	p.line("$ SET FILE/PROTECTION %s", sp("SP3.DAT"))
	p.line("$ SHOW SECURITY %s", sp("SP3.DAT"))
	p.line("$ CREATE/DIRECTORY %s[PROT93.SEC.SD1]", disk)
	p.security(dirFile("SEC.SD1"), ownerA, "(S:RWE,O:RWE,G:E,W)", nil)
	p.line("$ SET SECURITY/DEFAULT %s", dirFile("SEC.SD1"))
	p.line("$ SHOW SECURITY %s", dirFile("SEC.SD1"))
	p.line("$ SHOW SECURITY %s", spec("ACE", "L2.DAT"))

	p.section("PROGRAM AS SYSTEM")
	p.line(dropPrivs)
	p.line(showPrivs)
	p.run("M0", core2Cases)
	p.run("M0", change2Cases)
	p.line(restorePrivs)
	p.line(showPrivs)
	p.run("M4", core2Cases)

	p.section("END")
	p.line("$ WRITE SYS$OUTPUT \"PROT93R FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// buildRound2User is prot93v.com: an account's lost passes, without
// privileges first (so RMS's directory cache is empty for them), then with
// GRPPRV, then round 2's DCL cases.
func buildRound2User() string {
	var p proc

	p.line("$ ! PROT93V.COM - Phase 93's probe, round 2: an account's run.")
	p.line("$ ! Written by testdata/prot93/gen.go. Run it logged in as PROT93A, then")
	p.line("$ ! PROT93B, then PROT93C, after SYSTEM's PROT93R.COM (README.md):")
	p.line("$ !")
	p.line("$ !     @PROT93$DISK:[000000]PROT93V")
	p.line("$ !")
	p.line("$ ! Its log is PROT93$DISK:[PROT93LOG]PROT93V_username.LOG.")
	p.line("$ !")
	p.logged("\"PROT93V_\" + F$EDIT(F$GETJPI(\"\",\"USERNAME\"),\"TRIM\") + \".LOG\"", "PROT93V")

	p.section("ENVIRONMENT")
	p.line("$ USER = F$EDIT(F$GETJPI(\"\",\"USERNAME\"),\"TRIM\")")
	p.line("$ LETTER = F$EXTRACT(F$LENGTH(USER)-1,1,USER)")
	p.line("$ T = LETTER + \"0\"")
	p.line("$ ME = F$GETJPI(\"\",\"UIC\")")
	p.line("$ WRITE SYS$OUTPUT USER, \" \", ME, \" \", T")
	p.line("$ SHOW PROCESS/PRIVILEGES")
	p.line("$ SHOW SECURITY %s[000000]PROT93.EXE", disk)

	p.section("PROGRAM, NO PRIVILEGES")
	p.line(dropPrivs)
	p.line(showPrivs)
	p.run("'T'", coreCases)
	p.run("'T'", core2Cases)
	p.run("'T'", changeCases)
	p.run("'T'", change2Cases)
	p.run("'T'", fidCases)

	p.section("PROGRAM, GRPPRV")
	p.line("$ SET PROCESS/PRIVILEGES=GRPPRV")
	p.line(showPrivs)
	p.line("$ IF .NOT. F$PRIVILEGE(\"GRPPRV\") THEN GOTO NO_GRPPRV")
	p.line("$ TN = LETTER + \"2\"")
	p.run("'TN'", coreCases)
	p.run("'TN'", core2Cases)
	p.run("'TN'", fidCases)
	p.line("$ NO_GRPPRV:")
	p.line(dropPrivs)

	p.section("OWNERS")
	ctl := func(n string) string { return spec("CTL", n+"_'T'.DAT") }
	p.line("$ ! OS: owned by this account; OT: owned by PROT93A, whose ACL gives this")
	p.line("$ ! account CONTROL.")
	p.line("$ SET SECURITY/OWNER='ME' %s", ctl("OS"))
	p.line("$ SET SECURITY/OWNER=[200,1] %s", ctl("OS"))
	p.line("$ SET SECURITY/OWNER='ME' %s", ctl("OT"))
	p.line("$ SET SECURITY/PROTECTION=(W:RWED) %s", ctl("OT"))
	p.line("$ SHOW SECURITY %s", ctl("OS"))
	p.line("$ SHOW SECURITY %s", ctl("OT"))
	p.line("$ CREATE/OWNER_UIC='ME' %s", spec("OWN", "CM_'T'.DAT"))
	p.line("RECORD")
	p.line("$ CREATE/OWNER_UIC=[200,1] %s", spec("OWN", "CA_'T'.DAT"))
	p.line("RECORD")
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=[200,1] %s[PROT93.OWN.DO'T']", disk)
	p.line("$ CREATE/DIRECTORY/OWNER_UIC=PARENT %s[PROT93.OWN.DP'T']", disk)
	p.line("$ DIRECTORY/SECURITY %s", spec("OWN", "C%_'T'.DAT;*"))
	p.line("$ DIRECTORY/SECURITY %s", spec("OWN", "D%'T'.DIR;*"))

	p.section("NEW FILES' ACLS")
	p.line("$ COPY %s[PROT93LOG]T.DAT %s", disk, spec("DP", "R2_'T'.DAT"))
	p.line("$ SHOW SECURITY %s", spec("DP", "R2_'T'.DAT"))
	p.line("$ SHOW SECURITY %s", spec("NV", "NP_'T'.DAT;*"))

	p.section("PROCEDURES")

	for _, im := range r2Procs {
		p.line("$ @%s", spec("IMG", "PRO2"+im.name+".COM"))
		p.line("$ SHOW SYMBOL $STATUS")
	}

	p.section("END")
	p.line("$ WRITE SYS$OUTPUT \"PROT93V FINISHED\"")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// core2File is round 2's cases for every pass: the directory without
// execute, as [PROT93.DR] and the rest are in coreFile.
func core2File() string {
	var b strings.Builder

	add := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	for _, d := range r2Dirs {
		add("S- %s", spec(d.name, "*.*;*"))
		add("S- %s", spec(d.name, "A.DAT"))
		add("P- %s", spec(d.name, "A.DAT"))
		add("O- %s|G", spec(d.name, "A.DAT"))
		add("QA %s", spec(d.name, "A.DAT"))
		add("C- %s", spec(d.name, "C_@.DAT"))
		add("QC %s", spec(d.name, "QC_@.DAT"))
	}

	return b.String()
}

// change2File is round 2's changing cases, once per identity: erases and
// renames in directories without read or without execute, a new version
// of a file with NOPROPAGATE ACEs, supersedes and new versions of files
// without write.
func change2File() string {
	var b strings.Builder

	add := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	add("E- %s", spec("DW", "E2_@.DAT"))
	add("E- %s", spec("DRWO", "E3_@.DAT"))
	add("R- %s|%s", spec("DW", "N2_@.DAT"), spec("DW", "N2X_@.DAT"))
	add("R- %s|%s", spec("DRWO", "N3_@.DAT"), spec("DRWO", "N3X_@.DAT"))
	add("C- %s", spec("NV", "NP_@.DAT"))
	add("A- %s", spec("NV", "NP_@.DAT"))
	add("CS %s", spec("NV", "SUPD_@.DAT;1"))
	add("CS %s", spec("NV", "SUPW_@.DAT;1"))
	add("C- %s", spec("NV", "NVR_@.DAT"))

	for _, n := range []string{"NP", "SUPD", "SUPW", "NVR"} {
		add("S- %s", spec("NV", n+"_@.DAT;*"))
	}

	add("S- %s", spec("DW", "*_@.DAT;*"))
	add("S- %s", spec("DRWO", "*_@.DAT;*"))

	return b.String()
}

// buildRound2Final is prot93y.com: round 2's files listed, then the
// accounts removed, with the group identifier AUTHORIZE made for PROT93A
// (round 1's PROT93Z.COM left it).
func buildRound2Final() string {
	var p proc

	p.line("$ ! PROT93Y.COM - Phase 93's probe, round 2: the listing at the end, and")
	p.line("$ ! the accounts removed. Written by testdata/prot93/gen.go. Run it as")
	p.line("$ ! SYSTEM after the accounts' runs (README.md's \"Round 2\"):")
	p.line("$ !")
	p.line("$ !     @PROT93$DISK:[000000]PROT93Y")
	p.line("$ !")
	p.logged("\"PROT93Y.LOG\"", "PROT93Y")
	p.section("THE TREE")
	p.line("$ DIRECTORY/SECURITY %s[PROT93...]*.*;*", disk)
	p.section("REMOVING THE ACCOUNTS")
	p.line("$ HOME = F$ENVIRONMENT(\"DEFAULT\")")
	p.line("$ SET DEFAULT SYS$SYSTEM")
	p.line("$ RUN SYS$SYSTEM:AUTHORIZE")

	for _, l := range []string{
		"REMOVE PROT93A", "REMOVE PROT93B", "REMOVE PROT93C",
		"REMOVE/IDENTIFIER PROT93_TEAM", "REMOVE/IDENTIFIER PROT93",
		"EXIT",
	} {
		p.line("%s", l)
	}

	p.line("$ SET DEFAULT 'HOME'")
	p.line("$ WRITE SYS$OUTPUT \"PROT93Y FINISHED\"")
	p.line("$ DEASSIGN/SYSTEM/EXECUTIVE_MODE PROT93$DISK")
	p.line("$ SET NOVERIFY")
	p.line("$ EXIT")

	return p.String()
}

// round2Files are the files exchange2.cmd copies to round 1's volume.
var round2Files = []string{"prot93r.com", "prot93v.com", "prot93y.com", "prot93f.dat", "prot93g.dat"}

func exchange2() string {
	var b strings.Builder

	b.WriteString(`! EXCHANGE2.CMD - adds round 2 of Phase 93's probe to round 1's volume.
! Written by testdata/prot93/gen.go. Run from the repository root, after
! keeping a copy of round 1's volume (README.md's "Round 2"):
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/prot93-exchange.dsk"
`)

	for _, f := range round2Files {
		fmt.Fprintf(&b, "COPY \"%s/%s\"/HOST DUA5:[000000]%s\n", dir, f, strings.ToUpper(f))
	}

	b.WriteString("DIRECTORY DUA5:[000000]PROT93*.*\nDISMOUNT DUA5\n")

	return b.String()
}

func copyout2() string {
	var b strings.Builder

	b.WriteString(`! COPYOUT2.CMD - copies round 2's logs off prot93-exchange.dsk into
! testdata/prot93/vax/round2 (README.md's "Round 2"). Written by
! testdata/prot93/gen.go. Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout2.cmd
!
MOUNT DUA5 "testdata/disks/prot93-exchange.dsk"
`)

	for _, l := range []string{"PROT93R.LOG", "PROT93V_PROT93A.LOG", "PROT93V_PROT93B.LOG", "PROT93V_PROT93C.LOG", "PROT93Y.LOG"} {
		fmt.Fprintf(&b, "COPY DUA5:[PROT93LOG]%s \"%s/vax/round2/%s\"/HOST/QUIET\n", l, dir, strings.ToLower(l))
	}

	b.WriteString("DISMOUNT DUA5\n")

	return b.String()
}

func main() {
	files := map[string]string{
		"prot93.com":    buildSystem(),
		"prot93u.com":   buildUser(),
		"prot93b.com":   buildUtilities(),
		"prot93z.com":   buildFinal(),
		"prot93c.dat":   coreFile(),
		"prot93d.dat":   changeFile(),
		"prot93e.dat":   aceFile(),
		"hello.mar":     hello,
		"exchange.cmd":  exchange(),
		"copyout.cmd":   copyout(),
		"prot93r.com":   buildRound2System(),
		"prot93v.com":   buildRound2User(),
		"prot93y.com":   buildRound2Final(),
		"prot93f.dat":   core2File(),
		"prot93g.dat":   change2File(),
		"exchange2.cmd": exchange2(),
		"copyout2.cmd":  copyout2(),
	}

	for _, d := range defModules {
		files["def_"+strings.ToLower(d)+".mar"] = defModule(d)
	}

	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}
