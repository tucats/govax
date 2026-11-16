package dclshell

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmserrors"
)

// DCL in a batch job (docs/planning/PHASE-98 - batch queues.md, subtask 8;
// Decision 6). The job controller (corevms's batch.go) creates the job's
// process logged in to the job's user, its SYS$OUTPUT the log file, and
// starts its CLI with corevms.CLIStartup's Batch set; the CLI then does
// what VMS 7.1's did in the probe's runs (testdata/bat98/vax, the job
// logs and THE JOB'S VIEW):
//
//   - SYS$LOGIN, SYS$LOGIN_DEVICE, and SYS$SCRATCH are the account's home
//     in the job table and SYS$DISK in the process table, executive mode,
//     and the default directory is there, as at an interactive login
//     (dcllogin.go);
//   - the global symbol $RESTART is "FALSE", or "TRUE" in a restarted run,
//     when BATCH$RESTART is the job's last SET RESTART_VALUE; Ctrl/T and
//     Ctrl/Y are disabled, and the prompt is "$ ";
//   - the login procedures run, SYLOGIN and the account's (LOGIN.COM at
//     depth 1), with verification off (dclloginproc.go);
//   - each of the job's files runs as the job's own command stream at
//     command level 0 (F$ENVIRONMENT("DEPTH") 0 in it, 1 in a procedure
//     it calls), with the job's parameters as P1 to P8. govax runs each at
//     a command level of its own, so that its labels, GOTO, and DCL's error
//     handling are a procedure's, and counts that level as level 0
//     (Shell.batchBase);
//   - the first file is verified as the login procedures leave verification
//     (off), and each later one starts verified;
//   - a file that ends with an error or a severe error ends the job (its
//     EXIT's status, or a command's that DCL's ON action ended it on, with
//     no message for it in the log); one ending with a warning goes on to
//     the next; the job's status is its last file's;
//   - at the end, LOGOUT's report ends the log (subcli.go's cliCommand).

// runBatch is a batch job's CLI: its whole run, on the interpreter's
// goroutine, with the process's Shell current (see above).
func (cli *subprocessCLI) runBatch() {
	c := cli.c
	b := cli.batch
	sh := cli.shell

	if b.Device != "" || b.Directory != "" {
		if err := sh.setLoginDirectory(b.Device, b.Directory); err != nil {
			c.Printf("%%%s\n", err.Error())
		}
	}

	globals := sh.dclSymbols.globals()

	restart := "FALSE"
	if b.Restarted {
		restart = "TRUE"
		globals["BATCH$RESTART"] = dclSymbol{name: "BATCH$RESTART", minLength: len("BATCH$RESTART"), value: b.RestartValue, global: true}
	}

	globals["$RESTART"] = dclSymbol{name: "$RESTART", minLength: len("$RESTART"), value: restart, global: true}

	// No key reaches a batch job: Ctrl/T and Ctrl/Y are both disabled
	// (F$ENVIRONMENT's CONTROL "" and NOCONTROL "T,Y", as VMS 7.1's
	// job showed).
	sh.control = controlKeys{noY: true}

	dispatch := c.DispatchConsole

	if err := c.RunLoginProcedures(b.LGICMD, dispatch); err != nil {
		c.Printf("%%%s\n", err.Error())
	}

	status := uint32(ssNormal)

	for i, file := range b.Files {
		if !c.Running() || c.stopping {
			break
		}

		if i > 0 {
			c.Verify = true
		}

		c.RTL.SetBatchFile(i)

		sh.batchBase = 1
		exit, err := c.runProcedure(procedureCommand{file: batchFileName(file), params: b.Parameters}, dispatch)
		sh.batchBase = 0

		if err != nil {
			if !vmserrors.MessageInhibited(err) {
				c.Printf("%%%s\n", err.Error())
			}

			status = errorStatus(err)

			break
		}

		status = exit.status

		if severity := status & 7; severity == 2 || severity == 4 {
			break
		}
	}

	c.setStatus(status, nil)
	cli.status = status
}

// batchFileName is a job's file as DCL opens it: its name without the
// node a job's file is named with ("_GOVAX$DUA1:[000000]BAT98OK.COM;1"
// is "DUA1:[000000]BAT98OK.COM;1").
func batchFileName(spec string) string {
	if i := strings.Index(spec, "$"); i > 0 && strings.HasPrefix(spec, "_") && !strings.ContainsAny(spec[:i], ":[<") {
		return spec[i+1:]
	}

	return spec
}

// errorStatus is the condition value an error stands for, SS$_ABORT for
// one that has none.
func errorStatus(err error) uint32 {
	if st := corevms.StartupStatus(err); st != 0 {
		return st
	}

	return ssAbort
}
