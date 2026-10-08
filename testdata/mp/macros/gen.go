//go:build ignore

// gen writes the Phase 45 system service macro probes (docs/PHASE-45.md,
// "the process-service macros"): VAX MACRO programs that call each
// process-management and related system service macro in several forms,
// for real MACRO to assemble on VMS, plus the command procedure that does
// it and the govax console scripts that build the exchange volume and copy
// the results back. Run it from the repository root:
//
//	go run testdata/mp/macros/gen.go
//
// Everything is written from the argument lists in the System Services
// Reference Manual. The programs are assembled with /NOLIST, so no listing
// of a macro expansion is made; govax's own macros are then written to
// make the same object code (internal/asm's TestServiceMacroObjects).
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const dir = "testdata/mp/macros"

// An argument's kind says how its macro passes it: v, a longword by
// value; w, a word by value; a, an address (of a longword, a quadword,
// a descriptor, a list...).
type arg struct {
	name string
	kind byte
}

// A service is a macro: its keywords in order, and which are required (the
// manual leaves the rest in brackets). Round 1 (vax/round1) guessed that
// the first few were; real MACRO reported an error for a call without
// SCHDWK's DAYTIM, SETPRI's PRI, GETJPI's ITMLST, and so on, so round 2
// names them.
type service struct {
	name     string
	args     []arg
	required []string
}

func (s service) isRequired(name string) bool {
	for _, r := range s.required {
		if r == name {
			return true
		}
	}

	return false
}

func v(n string) arg { return arg{n, 'v'} }
func w(n string) arg { return arg{n, 'w'} }
func a(n string) arg { return arg{n, 'a'} }

var services = []service{
	{"CREPRC", []arg{a("PIDADR"), a("IMAGE"), a("INPUT"), a("OUTPUT"), a("ERROR"), a("PRVADR"), a("QUOTA"), a("PRCNAM"),
		v("BASPRI"), v("UIC"), w("MBXUNT"), v("STSFLG")}, nil},
	{"DELPRC", []arg{a("PIDADR"), a("PRCNAM")}, nil},
	{"WAKE", []arg{a("PIDADR"), a("PRCNAM")}, nil},
	{"HIBER", nil, nil},
	{"SCHDWK", []arg{a("PIDADR"), a("PRCNAM"), a("DAYTIM"), a("REPTIM")}, []string{"DAYTIM"}},
	{"CANWAK", []arg{a("PIDADR"), a("PRCNAM")}, nil},
	{"FORCEX", []arg{a("PIDADR"), a("PRCNAM"), v("CODE")}, nil},
	{"SUSPND", []arg{a("PIDADR"), a("PRCNAM"), v("FLAGS")}, nil},
	{"RESUME", []arg{a("PIDADR"), a("PRCNAM")}, nil},
	{"SETPRI", []arg{a("PIDADR"), a("PRCNAM"), v("PRI"), a("PRVPRI")}, []string{"PRI"}},
	{"SETPRN", []arg{a("PRCNAM")}, nil},
	{"GETJPI", []arg{v("EFN"), a("PIDADR"), a("PRCNAM"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"GETJPIW", []arg{v("EFN"), a("PIDADR"), a("PRCNAM"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"GETDVI", []arg{v("EFN"), w("CHAN"), a("DEVNAM"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"GETDVIW", []arg{v("EFN"), w("CHAN"), a("DEVNAM"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"CREMBX", []arg{v("PRMFLG"), a("CHAN"), v("MAXMSG"), v("BUFQUO"), v("PROMSK"), v("ACMODE"), a("LOGNAM")}, []string{"CHAN"}},
	{"DELMBX", []arg{w("CHAN")}, []string{"CHAN"}},
	{"SETIMR", []arg{v("EFN"), a("DAYTIM"), a("ASTADR"), v("REQIDT"), v("FLAGS")}, []string{"DAYTIM"}},
	{"CANTIM", []arg{v("REQIDT"), v("ACMODE")}, nil},
	{"WAITFR", []arg{v("EFN")}, []string{"EFN"}},
	{"SETEF", []arg{v("EFN")}, []string{"EFN"}},
	{"CLREF", []arg{v("EFN")}, []string{"EFN"}},
	{"READEF", []arg{v("EFN"), a("STATE")}, []string{"EFN", "STATE"}},
}

// extServices are the third round's: the other system services govax
// implements, with the manual's argument lists as best they are known
// (a keyword that is wrong shows as an error in the VMS run, and the next
// round tries another). required lists what the manual doesn't bracket;
// the probe finds out from the errors which are really required.
var extServices = []service{
	{"ADJSTK", []arg{v("ACMODE"), v("ADJUST"), a("NEWADR")}, nil},
	{"ADJWSL", []arg{v("PAGCNT"), a("WSETLM")}, []string{"PAGCNT"}},
	{"ALLOC", []arg{a("DEVNAM"), a("PHYLEN"), a("PHYBUF"), v("ACMODE")}, []string{"DEVNAM"}},
	{"DALLOC", []arg{a("DEVNAM"), v("ACMODE")}, nil},
	{"ASCEFC", []arg{v("EFN"), a("NAME"), v("PROT"), v("PERM")}, []string{"EFN", "NAME"}},
	{"DACEFC", []arg{v("EFN")}, []string{"EFN"}},
	{"DLCEFC", []arg{a("NAME")}, []string{"NAME"}},
	{"WFLAND", []arg{v("EFN"), v("MASK")}, []string{"EFN", "MASK"}},
	{"WFLOR", []arg{v("EFN"), v("MASK")}, []string{"EFN", "MASK"}},
	{"SYNCH", []arg{v("EFN"), a("IOSB")}, []string{"EFN", "IOSB"}},
	{"CANCEL", []arg{w("CHAN")}, []string{"CHAN"}},
	{"SETAST", []arg{v("ENBFLG")}, []string{"ENBFLG"}},
	{"DCLAST", []arg{a("ASTADR"), v("ASTPRM"), v("ACMODE")}, []string{"ASTADR"}},
	{"DCLEXH", []arg{a("DESBLK")}, []string{"DESBLK"}},
	{"CANEXH", []arg{a("DESBLK")}, nil},
	{"SETEXV", []arg{v("VECTOR"), a("ADDRES"), v("ACMODE"), a("PRVHND")}, []string{"VECTOR", "ADDRES"}},
	{"SETPRV", []arg{v("ENBFLG"), a("PRVADR"), v("PRMFLG"), a("PRVPRV")}, []string{"ENBFLG"}},
	{"CMKRNL", []arg{a("ROUTIN"), a("ARGLST")}, []string{"ROUTIN"}},
	{"CMEXEC", []arg{a("ROUTIN"), a("ARGLST")}, []string{"ROUTIN"}},
	{"ASCTIM", []arg{a("TIMLEN"), a("TIMBUF"), a("TIMADR"), v("CVTFLG")}, []string{"TIMBUF"}},
	{"BINTIM", []arg{a("TIMBUF"), a("TIMADR")}, []string{"TIMBUF", "TIMADR"}},
	{"GETTIM", []arg{a("TIMADR")}, []string{"TIMADR"}},
	{"NUMTIM", []arg{a("TIMBUF"), a("TIMADR")}, []string{"TIMBUF"}},
	{"FAOL", []arg{a("CTRSTR"), a("OUTLEN"), a("OUTBUF"), a("PRMLST")}, []string{"CTRSTR", "OUTBUF", "PRMLST"}},
	{"FAO", []arg{a("CTRSTR"), a("OUTLEN"), a("OUTBUF"), v("P1"), v("P2"), v("P3"), v("P4")}, []string{"CTRSTR", "OUTBUF"}},
	{"PUTMSG", []arg{a("MSGVEC"), a("ACTRTN"), a("FACNAM"), v("ACTPRM")}, []string{"MSGVEC"}},
	{"GETMSG", []arg{v("MSGID"), a("MSGLEN"), a("BUFADR"), v("FLAGS"), a("OUTADR")}, []string{"MSGID", "BUFADR"}},
	{"CRELNM", []arg{a("ATTR"), a("TABNAM"), a("LOGNAM"), v("ACMODE"), a("ITMLST")}, []string{"TABNAM", "LOGNAM", "ITMLST"}},
	{"DELLNM", []arg{a("TABNAM"), a("LOGNAM"), v("ACMODE")}, []string{"TABNAM"}},
	{"TRNLNM", []arg{a("ATTR"), a("TABNAM"), a("LOGNAM"), a("ACMODE"), a("ITMLST")}, []string{"TABNAM", "LOGNAM", "ITMLST"}},
	{"CRELNT", []arg{a("ATTR"), a("RESNAM"), a("RESLEN"), v("QUOTA"), v("PROMSK"), a("TABNAM"), a("PARTAB"), v("ACMODE")}, []string{"TABNAM"}},
	{"CRELOG", []arg{v("TBL"), a("LOGNAM"), a("EQLNAM"), v("ACMODE")}, []string{"LOGNAM", "EQLNAM"}},
	{"TRNLOG", []arg{a("LOGNAM"), a("RLENGTH"), a("RESBUF"), a("TABLE"), a("ACMODE"), v("DSBMSK")}, []string{"LOGNAM", "RESBUF"}},
	{"DELLOG", []arg{v("TBL"), a("LOGNAM"), v("ACMODE")}, []string{"LOGNAM"}},
	{"SNDOPR", []arg{a("MSGBUF"), w("CHAN")}, []string{"MSGBUF"}},
	{"EXPREG", []arg{v("PAGCNT"), a("RETADR"), v("ACMODE"), v("REGION")}, []string{"PAGCNT"}},
	{"CNTREG", []arg{v("PAGCNT"), a("RETADR"), v("ACMODE"), v("REGION")}, []string{"PAGCNT"}},
	{"CRETVA", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"DELTVA", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"LCKPAG", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"ULKPAG", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"LKWSET", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"ULWSET", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, []string{"INADR"}},
	{"SETPRT", []arg{a("INADR"), a("RETADR"), v("ACMODE"), v("PROT"), a("PRVPRT")}, []string{"INADR", "PROT"}},
	{"SETRWM", []arg{v("WATFLG")}, nil},
	{"GETSYI", []arg{v("EFN"), a("CSIDADR"), a("NODENAME"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"GETSYIW", []arg{v("EFN"), a("CSIDADR"), a("NODENAME"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, []string{"ITMLST"}},
	{"IDTOASC", []arg{v("ID"), a("NAMLEN"), a("RESNAM"), a("VALID"), a("ATTRIB"), a("CONTXT")}, []string{"ID", "RESNAM"}},
	{"ASCTOID", []arg{a("NAME"), a("ID"), a("ATTRIB")}, []string{"NAME"}},
	{"UNWIND", []arg{a("DEPADR"), a("NEWPC")}, nil},
}

// number is a distinctive value for argument i of a call: its position
// in the hex digits, so that a transposed push shows in the object.
func number(i int) string { return fmt.Sprintf("#^X%X%X", i+1, i+1) }

// value is an argument given as the probe's i-th symbol or number.
func (g arg) given(i int) string {
	if g.kind == 'a' {
		return fmt.Sprintf("ADR%d", i+1)
	}

	return number(i)
}

// call renders one macro call, with args given by the map from keyword to
// its text, in keyword order.
func call(macro string, s service, given map[string]string) string {
	var parts []string

	for _, g := range s.args {
		if t, ok := given[g.name]; ok {
			parts = append(parts, g.name+"="+t)
		}
	}

	if len(parts) == 0 {
		return "\t" + macro
	}

	return "\t" + macro + "\t" + strings.Join(parts, ", ")
}

// probe writes one service's program: the macro in its short (_S) and
// long (no suffix) forms, with the required arguments only, every
// argument, every optional argument left out in turn, every optional
// argument alone beside the required ones, and each argument in the
// addressing forms the macros must tell apart. After every call is a
// marker, ".LONG ^X7A7Axxxx" (xxxx the call's number from 1), which
// dumpcode.go finds in the object to tell where each call's code ends,
// whatever the call did.
func probe(s service) (string, []string) {
	return probeForms(s, []string{"$" + s.name + "_S", "$" + s.name}, false, "")
}

// probeForms is probe for the given macro names. In list style, the forms
// ("$NAME") take their values without "#", as an argument list's .LONGs
// do. prefix names the program (SVC_ for the first rounds, LST_ and EXT_
// for the third).
func probeForms(s service, formNames []string, list bool, prefix string) (string, []string) {
	var (
		b     strings.Builder
		calls []string
	)

	if prefix == "" {
		prefix = "SVC_"
	}

	fmt.Fprintf(&b, "\t.TITLE\t%s%s\tevery form of $%s\n\t.IDENT\t/V1.0/\n;\n", prefix, s.name, s.name)
	b.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	b.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\n")

	for i := range 14 {
		fmt.Fprintf(&b, "ADR%d:\t.LONG\t0,0\n", i+1)
	}

	b.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n")
	fmt.Fprintf(&b, "\t.ENTRY\t%s%s,^M<R6,R7>\n", prefix, s.name)

	// In the argument-list form (no suffix) a value is written without its
	// "#"; bare says the form being written is one.
	bare := false

	given := func(g arg, i int) string {
		t := g.given(i)
		if bare && g.kind != 'a' {
			// A bare ^X would be read as a delimited string by MACRO's
			// argument scanner, so the value is written in decimal.
			n, _ := strconv.ParseInt(strings.TrimPrefix(t, "#^X"), 16, 64)
			t = strconv.FormatInt(n, 10)
		}

		return t
	}

	emit := func(text string) {
		calls = append(calls, strings.TrimSpace(text))
		b.WriteString(text + "\n")
		fmt.Fprintf(&b, "\t.LONG\t^X7A7A%04X\n", len(calls))
	}

	// base is a call giving the required arguments only.
	base := func() map[string]string {
		m := map[string]string{}

		for i, g := range s.args {
			if s.isRequired(g.name) {
				m[g.name] = given(g, i)
			}
		}

		return m
	}

	for _, form := range formNames {
		fmt.Fprintf(&b, "; %s\n", form)

		// In the argument-list form (no suffix) a value is written without
		// its "#"; the _G form takes the list's address.
		if strings.HasSuffix(form, "_G") {
			for _, f := range []string{"", "ADR1", "ARGLST=ADR1", "(R6)", "4(R6)", "@ADR1", "ADR1[R7]", "@#ADR1", "-(R6)", "R6"} {
				if f == "" {
					emit("\t" + form)
				} else {
					emit("\t" + form + "\t" + f)
				}
			}

			continue
		}

		bare = list && !strings.HasSuffix(form, "_S")

		// Nothing, only the required, everything.
		emit(call(form, s, nil))
		emit(call(form, s, base()))

		all := map[string]string{}
		for i, g := range s.args {
			all[g.name] = given(g, i)
		}

		emit(call(form, s, all))

		// Each optional argument left out, the rest given.
		for i, g := range s.args {
			if s.isRequired(g.name) {
				continue
			}

			some := map[string]string{}
			for j, h := range s.args {
				if j != i {
					some[h.name] = given(h, j)
				}
			}

			emit(call(form, s, some))
		}

		// Each optional argument alone with the required ones.
		for i, g := range s.args {
			if s.isRequired(g.name) {
				continue
			}

			one := base()
			one[g.name] = given(g, i)

			emit(call(form, s, one))
		}

		// Each argument in the forms the macros must tell apart, the
		// others at their required values.
		for i, g := range s.args {
			var forms []string

			switch g.kind {
			case 'a':
				forms = []string{"(R6)", "-(R6)", "ADR1[R7]", "#5", "0", "@#ADR1"}
			case 'w':
				forms = []string{"R6", "#5", "#0", "0", "ADR1", "#^X1234"}
			default:
				forms = []string{"R6", "#5", "#0", "0", "ADR1", "#^X12345678", "ADR1[R7]"}
			}

			if bare && g.kind != 'a' {
				forms = []string{"R6", "5", "0", "ADR1", "305419896", "ADR1[R7]", "(R6)", "#5"}
			}

			for _, f := range forms {
				one := base()
				one[g.name] = f

				emit(call(form, s, one))
			}

			_ = i
		}
	}

	b.WriteString("\tRET\n\t.END\t" + prefix + s.name + "\n")

	return b.String(), calls
}

func main() {
	var names []string

	for _, s := range services {
		name := "svc_" + strings.ToLower(s.name)
		text, calls := probe(s)
		write(name+".mar", text)
		write(name+".calls", strings.Join(calls, "\n")+"\n")

		names = append(names, name)
	}

	// The command procedures and console scripts below are for the latest
	// round only (round 4): the results of rounds 2 and 3 (svc_*, lst_*,
	// ext_*) are in vax/ already. names is reset here, then the round's
	// files added.
	names = nil

	// Round 3 (written, but not in the scripts): the argument-list and _G
	// forms of the services above, the other system services in all their
	// forms, and the extra keywords.
	for _, s := range services {
		name := "lst_" + strings.ToLower(s.name)
		text, calls := probeForms(s, []string{"$" + s.name, "$" + s.name + "_G"}, true, "LST_")
		write(name+".mar", text)
		write(name+".calls", strings.Join(calls, "\n")+"\n")
	}

	for _, s := range extServices {
		name := "ext_" + strings.ToLower(s.name)
		text, calls := probeForms(s, []string{"$" + s.name + "_S", "$" + s.name, "$" + s.name + "_G"}, true, "EXT_")
		write(name+".mar", text)
		write(name+".calls", strings.Join(calls, "\n")+"\n")
	}

	{
		text, calls := extraProbe()
		write("ext_extra.mar", text)
		write("ext_extra.calls", strings.Join(calls, "\n")+"\n")
	}

	// Round 4: every other system service, by keyword and by position.
	for _, s := range r4Services {
		name := "r4_" + strings.ToLower(s.name)
		text, calls := r4Probe(s, "R4")
		write(name+".mar", text)
		write(name+".calls", strings.Join(calls, "\n")+"\n")
	}

	// Round 5: what round 4 left open.
	var round5, round6 []string

	{
		text, calls := r5Probe()
		write("r5_misc.mar", text)
		write("r5_misc.calls", strings.Join(calls, "\n")+"\n")

		round5 = append(round5, "r5_misc")
	}

	// Round 6 (docs/PHASE-46 - interprocess comm.md): the global section
	// services, which have no macros yet, and the lock services Phase 47
	// needs, by round 4's method.
	for _, s := range r6Services {
		name := "r6_" + strings.ToLower(s.name)
		text, calls := r4Probe(s, "R6")
		write(name+".mar", text)
		write(name+".calls", strings.Join(calls, "\n")+"\n")

		round6 = append(round6, name)
	}

	names = append(append(names, round5...), round6...)

	// The command procedures: one per round, each with its own log, and
	// MACROS.COM, which runs both.
	roundProcedure := func(n int, programs []string) {
		var c strings.Builder

		fmt.Fprintf(&c, "$ ! MACROS%d.COM - round %d of the system service macro probes\n", n, n)
		c.WriteString("$ ! (testdata/mp/macros/README.md). Written by gen.go. MACROS.COM runs it.\n$ !\n")
		c.WriteString("$ ! Each program is assembled with /NOLIST, so no listing of a macro\n")
		c.WriteString("$ ! expansion is made, and its object is analyzed.\n$ !\n")
		c.WriteString("$ SET NOON\n$ SET VERIFY\n")

		for _, p := range programs {
			up := strings.ToUpper(p)
			fmt.Fprintf(&c, "$ MACRO/NOLIST %s\n$ ANALYZE/OBJECT/OUTPUT=%s.ANL %s.OBJ\n", up, up, up)
		}

		c.WriteString("$ SET NOVERIFY\n$ EXIT\n")
		write(fmt.Sprintf("macros%d.com", n), c.String())
	}

	roundProcedure(5, round5)
	roundProcedure(6, round6)

	write("macros.com", `$ ! MACROS.COM - the system service macro probes, rounds 5 and 6
$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the
$ ! exchange volume as the default directory:
$ !
$ !     @MACROS
$ !
$ ! Each round writes its own log, MACROS5.LOG and MACROS6.LOG.
$ !
$ @MACROS5/OUTPUT=MACROS5.LOG
$ @MACROS6/OUTPUT=MACROS6.LOG
$ EXIT
`)

	// The govax console scripts.
	var x, o strings.Builder

	x.WriteString("! EXCHANGE.CMD - builds the system service macro probes' exchange volume\n")
	x.WriteString("! with govax (testdata/mp/macros/README.md). Written by gen.go. Run from\n! the repository root:\n!\n")
	x.WriteString("!     govax console < testdata/mp/macros/exchange.cmd\n!\n")
	x.WriteString("INITIALIZE/CONTAINER \"testdata/disks/mp-macros.dsk\" /DEVICE=RD53 MPMACROS\n")
	x.WriteString("MOUNT/WRITE DUA1 \"testdata/disks/mp-macros.dsk\"\n")

	o.WriteString("! COPYOUT.CMD - copies the results of the system service macro probes off\n")
	o.WriteString("! mp-macros.dsk into testdata/mp/macros/vax/ (README.md). Written by gen.go.\n! Run from the repository root:\n!\n")
	o.WriteString("!     govax console < testdata/mp/macros/copyout.cmd\n!\n")
	o.WriteString("MOUNT DUA1 \"testdata/disks/mp-macros.dsk\"\n")

	files := append([]string{}, names...)

	for _, n := range files {
		up := strings.ToUpper(n)
		fmt.Fprintf(&x, "COPY \"%s/%s.mar\"/HOST DUA1:[000000]%s.MAR\n", dir, n, up)
		fmt.Fprintf(&o, "COPY DUA1:[000000]%s.OBJ \"%s/vax/%s.obj\"/HOST/BINARY/QUIET\n", up, dir, n)

		fmt.Fprintf(&o, "COPY DUA1:[000000]%s.ANL \"%s/vax/%s.anl\"/HOST/QUIET\n", up, dir, n)
	}

	for _, n := range []string{"macros.com", "macros5.com", "macros6.com"} {
		fmt.Fprintf(&x, "COPY \"%s/%s\"/HOST DUA1:[000000]%s\n", dir, n, strings.ToUpper(n))
	}

	x.WriteString("DIRECTORY DUA1:[000000]\nDISMOUNT DUA1\n")
	fmt.Fprintf(&o, "COPY DUA1:[000000]MACROS5.LOG \"%s/vax/macros5.log\"/HOST/QUIET\n", dir)
	fmt.Fprintf(&o, "COPY DUA1:[000000]MACROS6.LOG \"%s/vax/macros6.log\"/HOST/QUIET\n", dir)
	o.WriteString("DISMOUNT DUA1\n")

	write("exchange.cmd", x.String())
	write("copyout.cmd", o.String())
}

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}

// extraProbe is the keywords the earlier rounds could only guess at, each
// call with a distinctive value so that an accepted keyword shows in the
// pushes and a refused one as an error: $GETDVI's NULLARG and $CREMBX's
// FLAGS (alone, and with the arguments they pair with), and candidate
// names for $CREPRC's two extra arguments.
func extraProbe() (string, []string) {
	var (
		b     strings.Builder
		calls []string
	)

	b.WriteString("\t.TITLE\tEXT_EXTRA\tkeywords the first rounds could only guess at\n\t.IDENT\t/V1.0/\n;\n")
	b.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	b.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\n")

	for i := range 14 {
		fmt.Fprintf(&b, "ADR%d:\t.LONG\t0,0\n", i+1)
	}

	b.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n\t.ENTRY\tEXT_EXTRA,^M<R6,R7>\n")

	for _, c := range []string{
		"$GETDVI_S\tITMLST=ADR4, NULLARG=#^X99",
		"$GETDVI_S\tITMLST=ADR4, NULLARG=#0",
		"$GETDVI_S\tITMLST=ADR4, NULLARG=#0, ASTPRM=#^X77",
		"$GETDVI_S\tITMLST=ADR4, NULLARG=#^X99, ASTPRM=#0",
		"$GETDVI_S\tITMLST=ADR4, NULLARG=R6",
		"$GETDVI_S\tITMLST=ADR4, NULLARG=ADR1",
		"$GETDVIW_S\tITMLST=ADR4, NULLARG=#^X99",
		"$CREMBX_S\tCHAN=ADR2, FLAGS=#^X99",
		"$CREMBX_S\tCHAN=ADR2, FLAGS=#0, LOGNAM=ADR7",
		"$CREMBX_S\tCHAN=ADR2, FLAGS=#^X99, LOGNAM=0, ACMODE=#0",
		"$CREMBX_S\tCHAN=ADR2, FLAGS=R6",
		"$CREMBX_S\tCHAN=ADR2, FLAGS=ADR1",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NODE=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NODENAME=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, ITEM_LIST=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, PRCITM=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, ARGLST=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NULLARG=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NULLARG1=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NULLARG2=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, ARG13=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, ARG14=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, ITEMLIST=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, DUMMY=ADR9",
		"$CREPRC_S\tIMAGE=ADR2, NODE_NAME=ADR9",
		"$CREPRC_S\tNOSUCHKEYWORD=ADR9",
		"$CREPRC_S\tNOSUCHKEYWORD=#^X99",
		"$WAKE_S\tNOSUCHKEYWORD=ADR9",
	} {
		calls = append(calls, c)
		b.WriteString("\t" + c + "\n")
		fmt.Fprintf(&b, "\t.LONG\t^X7A7A%04X\n", len(calls))
	}

	b.WriteString("\tRET\n\t.END\tEXT_EXTRA\n")

	return b.String(), calls
}

// r4Service is one of round 4's: a service's keywords as the manual gives
// them (no guess at which are required; the probe leaves each out in turn),
// other names that might be the right ones for a keyword the earlier rounds
// found unknown, and calls of its own.
type r4Service struct {
	name  string
	args  []arg
	alts  []string
	extra []string
}

// fao is $FAO's argument list: the three it always has, and P1 to P17.
func fao() []arg {
	list := []arg{a("CTRSTR"), a("OUTLEN"), a("OUTBUF")}

	for i := 1; i <= 17; i++ {
		list = append(list, v(fmt.Sprintf("P%d", i)))
	}

	return list
}

// r4Services are the services round 3 left open (docs/PHASE-45.md): every
// other service govax implements, their keywords the manual's. Round 3
// showed that a keyword a macro doesn't know is taken, silently, for the
// text of a positional argument (the object then refers to a symbol of
// that name), so an unknown keyword shows in the object as a reference to
// it, and a call with the wrong arguments for a required one is an error
// MACRO reports.
var r4Services = []r4Service{
	{"ADJSTK", []arg{v("ACMODE"), w("ADJUST"), a("NEWADR")}, nil, nil},
	{"ADJWSL", []arg{v("PAGCNT"), a("WSETLM")}, nil, nil},
	{"ALLOC", []arg{a("DEVNAM"), a("PHYLEN"), a("PHYBUF"), v("ACMODE"), v("FLAGS")}, []string{"FLAG", "ALLFLG", "OPTIONS"}, nil},
	{"DALLOC", []arg{a("DEVNAM"), v("ACMODE")}, nil, nil},
	{"ASCEFC", []arg{v("EFN"), a("NAME"), v("PROT"), v("PERM")}, nil, nil},
	{"DACEFC", []arg{v("EFN")}, nil, nil},
	{"DLCEFC", []arg{a("NAME")}, nil, nil},
	{"WFLAND", []arg{v("EFN"), v("MASK")}, nil, nil},
	{"WFLOR", []arg{v("EFN"), v("MASK")}, nil, nil},
	{"SYNCH", []arg{v("EFN"), a("IOSB")}, nil, nil},
	{"CANCEL", []arg{w("CHAN")}, nil, nil},
	{"SETAST", []arg{v("ENBFLG")}, nil, nil},
	{"DCLAST", []arg{a("ASTADR"), v("ASTPRM"), v("ACMODE")}, nil, nil},
	{"DCLEXH", []arg{a("DESBLK")}, nil, nil},
	{"CANEXH", []arg{a("DESBLK")}, nil, nil},
	{"SETEXV", []arg{v("VECTOR"), a("ADDRES"), v("ACMODE"), a("PRVHND")}, []string{"ADDRESS"}, nil},
	{"SETPRV", []arg{v("ENBFLG"), a("PRVADR"), v("PRMFLG"), a("PRVPRV")}, nil, nil},
	{"CMKRNL", []arg{a("ROUTIN"), a("ARGLST")}, nil, nil},
	{"CMEXEC", []arg{a("ROUTIN"), a("ARGLST")}, nil, nil},
	{"ASCTIM", []arg{a("TIMLEN"), a("TIMBUF"), a("TIMADR"), v("CVTFLG")}, nil, nil},
	{"BINTIM", []arg{a("TIMBUF"), a("TIMADR")}, nil, nil},
	{"GETTIM", []arg{a("TIMADR")}, nil, nil},
	{"NUMTIM", []arg{a("TIMBUF"), a("TIMADR")}, nil, nil},
	{"FAOL", []arg{a("CTRSTR"), a("OUTLEN"), a("OUTBUF"), a("PRMLST")}, nil, nil},
	{"FAO", fao(), nil, nil},
	{"PUTMSG", []arg{a("MSGVEC"), a("ACTRTN"), a("FACNAM"), v("ACTPRM")}, nil, nil},
	{"GETMSG", []arg{v("MSGID"), a("MSGLEN"), a("BUFADR"), v("FLAGS"), a("OUTADR")}, nil, nil},
	{"CRELNM", []arg{a("ATTR"), a("TABNAM"), a("LOGNAM"), a("ACMODE"), a("ITMLST")}, nil, nil},
	{"DELLNM", []arg{a("TABNAM"), a("LOGNAM"), a("ACMODE")}, nil, nil},
	{"TRNLNM", []arg{a("ATTR"), a("TABNAM"), a("LOGNAM"), a("ACMODE"), a("ITMLST")}, nil, nil},
	{"CRELNT", []arg{a("ATTR"), a("RESNAM"), a("RESLEN"), a("QUOTA"), a("PROMSK"), a("TABNAM"), a("PARTAB"), a("ACMODE")}, nil, nil},
	{"CRELOG", []arg{v("TBLFLG"), a("LOGNAM"), a("EQLNAM"), v("ACMODE")}, []string{"TBL", "TABLE", "TABFLG"}, nil},
	{"DELLOG", []arg{v("TBLFLG"), a("LOGNAM"), v("ACMODE")}, []string{"TBL", "TABLE", "TABFLG"}, nil},
	{"TRNLOG", []arg{a("LOGNAM"), a("RSLLEN"), a("RSLBUF"), a("TABLE"), a("ACMODE"), v("DSBMSK")},
		[]string{"RLENGTH", "RESLEN", "RESBUF", "RESULT", "DSBMASK"}, nil},
	{"SNDOPR", []arg{a("MSGBUF"), w("CHAN")}, nil, nil},
	{"EXPREG", []arg{v("PAGCNT"), a("RETADR"), v("ACMODE"), v("REGION")}, nil, nil},
	{"CNTREG", []arg{v("PAGCNT"), a("RETADR"), v("ACMODE"), v("REGION")}, nil, nil},
	{"CRETVA", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"DELTVA", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"LCKPAG", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"ULKPAG", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"LKWSET", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"ULWSET", []arg{a("INADR"), a("RETADR"), v("ACMODE")}, nil, nil},
	{"SETPRT", []arg{a("INADR"), a("RETADR"), v("ACMODE"), v("PROT"), a("PRVPRT")}, nil, nil},
	{"SETRWM", []arg{v("WATFLG")}, nil, nil},
	{"GETSYI", []arg{v("EFN"), a("CSIDADR"), a("NODENAME"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, nil, nil},
	{"GETSYIW", []arg{v("EFN"), a("CSIDADR"), a("NODENAME"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM")}, nil, nil},
	{"IDTOASC", []arg{v("ID"), a("NAMLEN"), a("RESNAM"), a("RESID"), a("ATTRIB"), a("CONTXT")},
		[]string{"IDENT", "IDENTIFIER", "IDVAL", "VALUE", "RESNAME", "RESULT", "RESIDENT", "VALID", "CONTEXT"}, nil},
	{"ASCTOID", []arg{a("NAME"), a("ID"), a("ATTRIB")}, nil, nil},
	{"UNWIND", []arg{a("DEPADR"), a("NEWPC")}, nil, nil},
	{"BRKTHRU", []arg{v("EFN"), a("MSGBUF"), v("SENDTO"), v("SNDTYP"), a("IOSB"), v("CARCON"), v("FLAGS"), v("REQID"), v("TIMOUT"), a("ASTADR"), v("ASTPRM")}, nil, nil},
	{"BRKTHRUW", []arg{v("EFN"), a("MSGBUF"), v("SENDTO"), v("SNDTYP"), a("IOSB"), v("CARCON"), v("FLAGS"), v("REQID"), v("TIMOUT"), a("ASTADR"), v("ASTPRM")}, nil, nil},
	// $CREPRC's thirteenth and fourteenth arguments: the keywords are
	// known (round 3), their sizes are not.
	{"CREPRC", []arg{a("ITMLST"), a("NODE")}, nil, []string{
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=-(R6)",
		"$CREPRC_S\tIMAGE=ADR2, NODE=-(R6)",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=(R6)+",
		"$CREPRC_S\tIMAGE=ADR2, NODE=(R6)+",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=ADR9[R7]",
		"$CREPRC_S\tIMAGE=ADR2, NODE=ADR9[R7]",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=#5",
		"$CREPRC_S\tIMAGE=ADR2, NODE=#5",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=0",
		"$CREPRC_S\tIMAGE=ADR2, NODE=0",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=0, NODE=0",
		"$CREPRC_S\tIMAGE=ADR2, ITMLST=ADR9, NODE=ADR10",
	}},
}

// r6Services are round 6's (docs/PHASE-46 - interprocess comm.md): the
// global section services, which govax implements (Phase 46) but has no
// macros for, and the lock management services Phase 47 implements. The
// keywords are the System Services Reference Manual's. $ENQ's RSDM_ID
// (resource domains) may be later than VMS 7.3; if it is, it shows as a
// reference to a symbol of that name, and the positional calls show how
// many arguments the macro has.
var r6Services = []r4Service{
	{"CRMPSC", []arg{a("INADR"), a("RETADR"), v("ACMODE"), v("FLAGS"), a("GSDNAM"), a("IDENT"), v("RELPAG"),
		w("CHAN"), v("PAGCNT"), v("VBN"), v("PROT"), v("PFC")}, nil, nil},
	{"MGBLSC", []arg{a("INADR"), a("RETADR"), v("ACMODE"), v("FLAGS"), a("GSDNAM"), a("IDENT"), v("RELPAG")}, nil, nil},
	{"DGBLSC", []arg{v("FLAGS"), a("GSDNAM"), a("IDENT")}, nil, nil},
	{"ENQ", []arg{v("EFN"), v("LKMODE"), a("LKSB"), v("FLAGS"), a("RESNAM"), v("PARID"), a("ASTADR"), v("ASTPRM"),
		a("BLKAST"), v("ACMODE"), v("RSDM_ID"), v("NULLARG")}, []string{"RSDMID", "NULL"}, nil},
	{"ENQW", []arg{v("EFN"), v("LKMODE"), a("LKSB"), v("FLAGS"), a("RESNAM"), v("PARID"), a("ASTADR"), v("ASTPRM"),
		a("BLKAST"), v("ACMODE"), v("RSDM_ID"), v("NULLARG")}, []string{"RSDMID", "NULL"}, nil},
	{"DEQ", []arg{v("LKID"), a("VALBLK"), v("ACMODE"), v("FLAGS")}, nil, nil},
	{"GETLKI", []arg{v("EFN"), a("LKIDADR"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM"), v("NULLARG")}, nil, nil},
	{"GETLKIW", []arg{v("EFN"), a("LKIDADR"), a("ITMLST"), a("IOSB"), a("ASTADR"), v("ASTPRM"), v("NULLARG")}, nil, nil},
}

// r4Probe writes one service's round 4 program (and round 6's: round is
// the prefix of its title and entry point). Every call is the short
// form (the argument-list and CALLG forms follow one rule, round 3), and
// ends with a marker as the earlier rounds' do. For a service of n
// arguments, in this order:
//
//   - no arguments, and every argument by keyword;
//   - each argument left out of the full call, so that an error shows the
//     ones that are required;
//   - each argument alone, by keyword, with an address (ADRk) and in the
//     form that shows its size, -(R6): a call with a keyword the macro
//     doesn't have shows as a reference to a symbol of that name;
//   - each alternative name for a keyword, alone;
//   - the first k arguments by position, k from 1 to n, each written ADRk:
//     the object shows each one's order and how it is passed (PUSHL for a
//     value, PUSHAB for an address, MOVZWL or CVTWL for a word), whatever
//     the macro calls it;
//   - each adjacent pair left out of the full call, which shows the pairs
//     the macro joins into one CLRQ;
//   - every argument by keyword as zero (0 for an address, #0 for a value).
func r4Probe(s r4Service, round string) (string, []string) {
	var (
		b     strings.Builder
		calls []string
	)

	n := len(s.args)

	fmt.Fprintf(&b, "\t.TITLE\t%s_%s\tevery argument of $%s\n\t.IDENT\t/V1.0/\n;\n", round, s.name, s.name)
	b.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	b.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\n")

	for i := range max(n, 14) + 2 {
		fmt.Fprintf(&b, "ADR%d:\t.LONG\t0,0\n", i+1)
	}

	b.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n")
	fmt.Fprintf(&b, "\t.ENTRY\t%s_%s,^M<R6,R7>\n", round, s.name)

	macro := "$" + s.name + "_S"

	emit := func(text string) {
		calls = append(calls, strings.TrimSpace(text))
		b.WriteString(text + "\n")
		fmt.Fprintf(&b, "\t.LONG\t^X7A7A%04X\n", len(calls))
	}

	// keyword renders a call from the arguments whose index is in use,
	// with text(i) the argument's text.
	keyword := func(use func(i int) bool, text func(i int) string) string {
		var parts []string

		for i, g := range s.args {
			if use(i) {
				parts = append(parts, g.name+"="+text(i))
			}
		}

		if len(parts) == 0 {
			return "\t" + macro
		}

		return "\t" + macro + "\t" + strings.Join(parts, ", ")
	}

	// A value is a distinctive number, an address ADRk.
	given := func(i int) string { return s.args[i].given(i) }
	zero := func(i int) string {
		if s.args[i].kind == 'a' {
			return "0"
		}

		return "#0"
	}

	none := func(int) bool { return false }
	all := func(int) bool { return true }

	emit(keyword(none, given))
	emit(keyword(all, given))

	// Each argument left out of the full call.
	for k := range s.args {
		emit(keyword(func(i int) bool { return i != k }, given))
	}

	// Each argument alone: as the probe's i-th address or number, and as
	// -(R6), which shows how big a thing an address argument points to.
	for k := range s.args {
		emit(keyword(func(i int) bool { return i == k }, given))
		emit(keyword(func(i int) bool { return i == k }, func(int) string { return "-(R6)" }))
	}

	for _, alt := range s.alts {
		emit("\t" + macro + "\t" + alt + "=ADR9")
	}

	// The first k arguments by position.
	for k := 1; k <= n; k++ {
		parts := make([]string, k)
		for i := range k {
			parts[i] = fmt.Sprintf("ADR%d", i+1)
		}

		emit("\t" + macro + "\t" + strings.Join(parts, ", "))
	}

	// Each adjacent pair left out.
	for k := 0; k+1 < n; k++ {
		emit(keyword(func(i int) bool { return i != k && i != k+1 }, given))
	}

	emit(keyword(all, zero))

	for _, c := range s.extra {
		emit("\t" + c)
	}

	b.WriteString("\tRET\n\t.END\t" + round + "_" + s.name + "\n")

	return b.String(), calls
}

// r5Probe is round 5: the few things round 4 left open (docs/PHASE-45.md).
// $TRNLOG's LOGNAM and RSLLEN and $CRELNT's TABNAM never had a call in
// which -(R6) could show their size, because another required argument was
// missing; $IDTOASC's third argument has a keyword that is not RESNAM or
// any of round 4's candidates, and an unknown size.
func r5Probe() (string, []string) {
	var (
		b     strings.Builder
		calls []string
	)

	b.WriteString("\t.TITLE\tR5_MISC\twhat round 4 left open\n\t.IDENT\t/V1.0/\n;\n")
	b.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	b.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\n")

	for i := range 14 {
		fmt.Fprintf(&b, "ADR%d:\t.LONG\t0,0\n", i+1)
	}

	b.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n\t.ENTRY\tR5_MISC,^M<R6,R7>\n")

	list := []string{
		"$TRNLOG_S\tLOGNAM=-(R6), RSLBUF=ADR3",
		"$TRNLOG_S\tLOGNAM=ADR1, RSLBUF=ADR3, RSLLEN=-(R6)",
		"$TRNLOG_S\tLOGNAM=ADR1, RSLBUF=-(R6)",
		"$TRNLOG_S\tLOGNAM=ADR1, RSLBUF=ADR3, TABLE=-(R6)",
		"$TRNLOG_S\tLOGNAM=ADR1, RSLBUF=ADR3, ACMODE=-(R6)",
		"$CRELNT_S\tPARTAB=ADR7, TABNAM=-(R6)",
		"$CRELNT_S\tPARTAB=ADR7, TABNAM=(R6)+",
		"$CRELNT_S\tPARTAB=ADR7, TABNAM=ADR6[R7]",
		"$CRELNT_S\tPARTAB=-(R6), TABNAM=ADR6",
		"$IDTOASC_S\t#1, ADR2, -(R6)",
		"$IDTOASC_S\t#1, ADR2, (R6)+",
		"$IDTOASC_S\t#1, ADR2, ADR3[R7]",
		"$IDTOASC_S\t#1, ADR2, ADR3, -(R6)",
		"$IDTOASC_S\t#1, -(R6)",
	}

	// Candidates for $IDTOASC's third argument's keyword.
	for _, k := range []string{"RSLNAM", "RSLBUF", "RSLDSC", "NAMBUF", "NAMDSC", "NAMDESC", "NAMEDSC", "NAME", "NAMADR",
		"RESNM", "RESDSC", "RESDESC", "RESBUF", "RESLT", "RESNAMDSC", "IDNAM", "IDNAME", "BUF", "BUFFER", "BUFADR", "DESCR", "DSC",
		"RESNAMBUF", "RESNMBUF", "RESNMDSC", "IDNMBUF", "IDNMDSC", "TEXT", "STRING", "STR", "STRBUF", "STRDSC", "OUTBUF", "OUTADR"} {
		list = append(list, fmt.Sprintf("$IDTOASC_S\t%s=ADR9", k))
	}

	for _, c := range list {
		calls = append(calls, c)
		b.WriteString("\t" + c + "\n")
		fmt.Fprintf(&b, "\t.LONG\t^X7A7A%04X\n", len(calls))
	}

	b.WriteString("\tRET\n\t.END\tR5_MISC\n")

	return b.String(), calls
}
