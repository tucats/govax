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
	var (
		b     strings.Builder
		calls []string
	)

	fmt.Fprintf(&b, "\t.TITLE\tSVC_%s\tevery form of $%s\n\t.IDENT\t/V1.0/\n;\n", s.name, s.name)
	b.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	b.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\n")

	for i := range 14 {
		fmt.Fprintf(&b, "ADR%d:\t.LONG\t0,0\n", i+1)
	}

	b.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n")
	fmt.Fprintf(&b, "\t.ENTRY\tSVC_%s,^M<R6,R7>\n", s.name)

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
				m[g.name] = g.given(i)
			}
		}

		return m
	}

	for _, form := range []string{"$" + s.name + "_S", "$" + s.name} {
		fmt.Fprintf(&b, "; %s\n", form)

		// Nothing, only the required, everything.
		emit(call(form, s, nil))
		emit(call(form, s, base()))

		all := map[string]string{}
		for i, g := range s.args {
			all[g.name] = g.given(i)
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
					some[h.name] = h.given(j)
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
			one[g.name] = g.given(i)

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

			for _, f := range forms {
				one := base()
				one[g.name] = f

				emit(call(form, s, one))
			}

			_ = i
		}
	}

	b.WriteString("\tRET\n\t.END\tSVC_" + s.name + "\n")

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

	// The CALLG forms and the post-7.1 arguments may not exist: their
	// errors go to a log of their own, for the author to audit.
	errs := []string{
		"\t$CREPRC_G\tARGLST=ADR1\n\t$QIOW_G\tARGLST=ADR1\n\t$GETJPI_G\tARGLST=ADR1\n\t$WAKE_G\tARGLST=ADR1\n",
		"\t$CREPRC_S\tIMAGE=ADR1, ITEMLST=ADR2\n\t$CREPRC_S\tIMAGE=ADR1, NODE=ADR2\n",
		"\t$GETDVI_S\tITMLST=ADR1, NULLARG=ADR2\n",
		"\t$CREMBX_S\tCHAN=ADR1, FLAGS=#1\n",
		"\t$CREPRC_S\tNOSUCHKEYWORD=1\n",
	}

	var eb strings.Builder

	eb.WriteString("\t.TITLE\tERR_SVC\tforms that may not exist\n\t.IDENT\t/V1.0/\n;\n")
	eb.WriteString("; Written by testdata/mp/macros/gen.go (docs/PHASE-45.md).\n;\n")
	eb.WriteString("\t.PSECT\tDATA,WRT,NOEXE,LONG\nADR1:\t.LONG\t0\nADR2:\t.LONG\t0\n")
	eb.WriteString("\t.PSECT\tCODE,EXE,NOWRT,LONG\n\t.ENTRY\tERR_SVC,^M<>\n")

	for _, e := range errs {
		eb.WriteString(e)
	}

	eb.WriteString("\tRET\n\t.END\tERR_SVC\n")
	write("err_svc.mar", eb.String())

	// The command procedure.
	var c strings.Builder

	c.WriteString("$ ! MACROS.COM - Phase 45's system service macro probes\n")
	c.WriteString("$ ! (testdata/mp/macros/README.md). Written by gen.go. Run it with the\n")
	c.WriteString("$ ! exchange volume as the default directory:\n$ !\n")
	c.WriteString("$ !     @MACROS/OUTPUT=MACROS.LOG\n$ !\n")
	c.WriteString("$ ! Each program is assembled with /NOLIST, so no listing of a macro\n")
	c.WriteString("$ ! expansion is made, and its object is analyzed.\n$ !\n")
	c.WriteString("$ SET NOON\n$ SET VERIFY\n")

	for _, n := range names {
		fmt.Fprintf(&c, "$ MACRO/NOLIST %s\n$ ANALYZE/OBJECT/OUTPUT=%s.ANL %s.OBJ\n", strings.ToUpper(n), strings.ToUpper(n), strings.ToUpper(n))
	}

	c.WriteString("$ SET NOVERIFY\n$ EXIT\n")
	write("macros.com", c.String())

	// The error probe is assembled apart, with its messages in a log.
	write("macroserr.com", "$ ! MACROSERR.COM - the forms that may not exist (README.md). Run it after\n"+
		"$ ! MACROS.COM, with the same default directory:\n$ !\n"+
		"$ !     @MACROSERR/OUTPUT=ERRORS.LOG\n$ !\n"+
		"$ SET NOON\n$ SET VERIFY\n$ MACRO/NOLIST ERR_SVC\n$ SET NOVERIFY\n$ EXIT\n")

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

	files := append(append([]string{}, names...), "err_svc")

	for _, n := range files {
		up := strings.ToUpper(n)
		fmt.Fprintf(&x, "COPY \"%s/%s.mar\"/HOST DUA1:[000000]%s.MAR\n", dir, n, up)
		fmt.Fprintf(&o, "COPY DUA1:[000000]%s.OBJ \"%s/vax/%s.obj\"/HOST/BINARY/QUIET\n", up, dir, n)

		if n != "err_svc" {
			fmt.Fprintf(&o, "COPY DUA1:[000000]%s.ANL \"%s/vax/%s.anl\"/HOST/QUIET\n", up, dir, n)
		}
	}

	for _, n := range []string{"macros.com", "macroserr.com"} {
		fmt.Fprintf(&x, "COPY \"%s/%s\"/HOST DUA1:[000000]%s\n", dir, n, strings.ToUpper(n))
	}

	x.WriteString("DIRECTORY DUA1:[000000]\nDISMOUNT DUA1\n")
	fmt.Fprintf(&o, "COPY DUA1:[000000]MACROS.LOG \"%s/vax/macros.log\"/HOST/QUIET\n", dir)
	fmt.Fprintf(&o, "COPY DUA1:[000000]ERRORS.LOG \"%s/vax/errors.log\"/HOST/QUIET\n", dir)
	o.WriteString("DISMOUNT DUA1\n")

	write("exchange.cmd", x.String())
	write("copyout.cmd", o.String())
}

func write(name, text string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		log.Fatal(err)
	}
}
