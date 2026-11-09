package dclshell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// SPAWN/HOST, govax's own: the command is a program on the host, run with
// exec, not a command for a VAX subprocess. No VAX process is made.
//
// DCL waits for the program unless /NOWAIT. Waiting, the machine stands
// still, no other process running, and the program has the terminal.
// With /NOWAIT the program runs by itself, the machine going on. It has no
// input but /INPUT, and DCL shows what it writes (without /OUTPUT) and
// finishes up after it between commands and while the prompt waits, on
// DCL's own goroutine (pollHostJobs), because neither DCL's output nor RMS
// may be used from another. Like a subprocess, it ends when govax does
// (EndHostJobs).
//
// The command is split into a program and its arguments as EDIT's editor
// command is (dclcmd.SplitCommand). No shell is involved, so a host shell's
// pipes and wildcards need one named: SPAWN/HOST sh -c "ls *.txt | wc -l".
//
// Host programs and their arguments have lowercase letters, so the command
// is the text as typed, not the grammar's uppercased copy (hostCommandText).
//
// /INPUT and /OUTPUT become the program's standard input and output. Each
// is a file name by the rules every command's are (rms.Session.Locate): a
// host file is handed to the program as it is, and a volume file's records
// are its input, or its output lines become a new version's records.
//
// /SYMBOLS and /LOGICAL_NAMES, which a subprocess gets by default, must be
// asked for here. They add to the program's environment, on top of govax's
// own: GOVAX_SYM_name for each DCL symbol and GOVAX_LOG_name for each
// process logical name a subprocess would get, a "$" in a name becoming
// "_" (see hostVariable).

// Prefixes of the environment variables /SYMBOLS and /LOGICAL_NAMES make.
const (
	hostSymbolPrefix  = "GOVAX_SYM_"
	hostLogicalPrefix = "GOVAX_LOG_"
)

// spawnHost is SPAWN/HOST (see above). A program that can't be run, or that
// ends with a failure, is a VAX_HOSTCMD error, which sets $STATUS as any
// failing command's does.
func (c *Interpreter) spawnHost(o SpawnOptions) error {
	// /PROCESS and /PROMPT describe a VAX subprocess.
	if o.Process != "" || o.Prompt != "" {
		return vmserrors.New(vmserrors.CLI_CONFLICT)
	}

	if strings.TrimSpace(o.Command) == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	o.Command = hostCommandText(c.line, o.Command)

	words, err := dclcmd.SplitCommand(o.Command)
	if err != nil {
		return vmserrors.New(vmserrors.VAX_HOSTCMD, o.Command, err.Error())
	}

	cmd := exec.Command(words[0], words[1:]...) //nolint:gosec // running it is the point
	cmd.Env = append(os.Environ(), c.hostEnvironment(o)...)

	// finish does whatever is left of the redirections once the program
	// has ended: it closes host files and writes a volume file. A deferred
	// call would lose the error from writing the file.
	finish, err := c.redirectHost(cmd, o)
	if err != nil {
		return err
	}

	if o.NoWait {
		return c.startHost(cmd, o.Command, finish)
	}

	ran := c.runHost(cmd)
	saved := finish()

	if ran != nil {
		return vmserrors.New(vmserrors.VAX_HOSTCMD, o.Command, ran.Error())
	}

	return saved
}

// hostCommandText is command, the end of the command line that the grammar
// has uppercased outside quotes, in line's own case.
func hostCommandText(line, command string) string {
	line = strings.TrimRight(line, " \t")
	if tail := len(line) - len(command); tail >= 0 && strings.EqualFold(line[tail:], command) {
		return line[tail:]
	}

	return command
}

// runHost runs cmd to its end. A program whose output isn't redirected
// writes on the terminal directly when it can, so that it sees a terminal
// there, but DCL's output when that has gone elsewhere: a procedure's
// /OUTPUT file, or a subprocess's SYS$OUTPUT.
func (c *Interpreter) runHost(cmd *exec.Cmd) error {
	if c.LendTerminal != nil && !c.outputElsewhere() {
		return c.LendTerminal(cmd)
	}

	if cmd.Stdout == nil {
		cmd.Stdout = c.Out
	}

	if cmd.Stderr == nil {
		cmd.Stderr = c.Out
	}

	if c.LendTerminal != nil {
		return c.LendTerminal(cmd)
	}

	return cmd.Run()
}

// outputElsewhere reports whether DCL's output isn't the terminal's.
func (c *Interpreter) outputElsewhere() bool {
	level := c.currentLevel()

	return c.subCLI != nil || (level != nil && level.redirected)
}

// redirectHost sets cmd's standard input and output from o's /INPUT and
// /OUTPUT, and returns what to do when the program has ended. It reads an
// input file before touching the output, so a bad /INPUT leaves no output.
func (c *Interpreter) redirectHost(cmd *exec.Cmd, o SpawnOptions) (finish func() error, err error) {
	closeInput := func() {}

	if o.Input != "" {
		var input io.Reader

		if input, closeInput, err = c.hostInput(o.Input); err != nil {
			return nil, err
		}

		cmd.Stdin = input
	}

	if o.Output == "" {
		return func() error { closeInput(); return nil }, nil
	}

	saveOutput, err := c.hostOutput(cmd, o.Output)
	if err != nil {
		closeInput()

		return nil, err
	}

	return func() error { closeInput(); return saveOutput() }, nil
}

// hostInput opens the file name as a program's standard input, returning
// it and what closes it. A host file is the file itself, so the program
// can seek in it; a volume file is its records, one line each.
func (c *Interpreter) hostInput(name string) (io.Reader, func(), error) {
	loc, err := c.ContainerSession.Locate(name, false)
	if err != nil {
		return nil, nil, dclcmd.FileFailure(err, name)
	}

	if loc.Host {
		f, err := os.Open(loc.Name)
		if err != nil {
			return nil, nil, vmserrors.New(vmserrors.VAX_HOSTCMD, name, hostErrorText(err))
		}

		return f, func() { _ = f.Close() }, nil
	}

	records, _, err := c.ContainerSession.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return nil, nil, dclcmd.FileFailure(err, name)
	}

	var text bytes.Buffer

	for _, record := range records {
		text.Write(record)
		text.WriteByte('\n')
	}

	return &text, func() {}, nil
}

// hostOutput makes the file name cmd's standard output, and returns what
// to do when the program has ended. A host file is the file itself. A
// volume file is made afterward from what the program wrote, a line a
// record, because the program can't write to a volume.
func (c *Interpreter) hostOutput(cmd *exec.Cmd, name string) (func() error, error) {
	// The session now, not when the program ends: by then the DCL running
	// may be another process's.
	session := c.ContainerSession

	loc, err := session.Locate(name, false)
	if err != nil {
		return nil, dclcmd.FileFailure(err, name)
	}

	if loc.Host {
		f, err := os.Create(loc.Name)
		if err != nil {
			return nil, vmserrors.New(vmserrors.VAX_HOSTCMD, name, hostErrorText(err))
		}

		cmd.Stdout = f

		return f.Close, nil
	}

	var text bytes.Buffer

	cmd.Stdout = &text

	return func() error {
		var records [][]byte

		if text.Len() > 0 {
			records = bytes.Split(bytes.TrimSuffix(text.Bytes(), []byte("\n")), []byte("\n"))
		}

		if _, err := session.CreateRecordFile(loc, rms.TextRecords, records); err != nil {
			return dclcmd.FileFailure(err, name)
		}

		return nil
	}, nil
}

// hostErrorText is what the host said was wrong with a file, without the
// operation and path, which the message has already.
func hostErrorText(err error) string {
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return unwrapped.Error()
	}

	return err.Error()
}

// hostEnvironment is the NAME=value strings o's /SYMBOLS and /LOGICAL_NAMES
// add to a host program's environment, sorted by name.
func (c *Interpreter) hostEnvironment(o SpawnOptions) []string {
	variables := map[string]string{}

	if o.HostLogicals && c.Logicals != nil {
		for _, e := range corevms.InheritedNames(c.Logicals) {
			values := make([]string, len(e.Equivalences))
			for i, equivalence := range e.Equivalences {
				values[i] = equivalence.Value
			}

			// InheritedNames lists a name's outermost access mode first,
			// the one a translation finds, so an inner mode's is left
			// out. A search list's values are separated by commas.
			addHostVariable(variables, hostLogicalPrefix, e.Name, strings.Join(values, ","))
		}
	}

	if o.HostSymbols {
		// The tables are in search order, so the first definition of a
		// name is the one DCL would use.
		for _, table := range c.dclSymbols.searchOrder() {
			for name, symbol := range table {
				addHostVariable(variables, hostSymbolPrefix, name, symbol.value)
			}
		}
	}

	env := make([]string, 0, len(variables))
	for name, value := range variables {
		env = append(env, name+"="+value)
	}

	sort.Strings(env)

	return env
}

// addHostVariable adds the variable for a logical name or symbol to
// variables, unless one of that name is there already. A name or value the
// host can't carry is left out: a process-permanent file's logical name, for
// one, is an escape and binary bytes.
func addHostVariable(variables map[string]string, prefix, name, value string) {
	if strings.ContainsRune(name, '=') || strings.ContainsAny(value, "\x00\x1b") {
		return
	}

	variable := hostVariable(prefix, name)
	if _, found := variables[variable]; !found {
		variables[variable] = value
	}
}

// hostVariable is the environment variable for a logical name or symbol:
// PARTS$DATA with the logical names' prefix is GOVAX_LOG_PARTS_DATA.
func hostVariable(prefix, name string) string {
	return prefix + strings.ReplaceAll(name, "$", "_")
}

// hostJob is a SPAWN/HOST/NOWAIT program still to be finished up.
type hostJob struct {
	cmd     *exec.Cmd
	command string

	// output holds what the program has written and DCL hasn't shown.
	output *hostOutputQueue

	// finish completes its /INPUT and /OUTPUT (redirectHost).
	finish func() error

	// done gets the program's result when it has ended.
	done chan error
}

// hostOutputQueue is what a /NOWAIT program writes, on its way from the
// goroutines that package exec copies the program's output with to DCL's.
type hostOutputQueue struct {
	mu   sync.Mutex
	text bytes.Buffer
}

// Write adds p to the queue; it is what makes the queue an io.Writer.
func (q *hostOutputQueue) Write(p []byte) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.text.Write(p)
}

// take removes and returns the queue's whole lines, or with all set,
// everything in it, a last line with no newline being given one.
func (q *hostOutputQueue) take(all bool) []byte {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := bytes.LastIndexByte(q.text.Bytes(), '\n') + 1
	if all {
		n = q.text.Len()
	}

	text := bytes.Clone(q.text.Next(n))
	if all && n > 0 && text[n-1] != '\n' {
		text = append(text, '\n')
	}

	return text
}

// hostWaitDelay is how long DCL lets a program's own children keep its
// output open after the program has ended, before it stops listening.
const hostWaitDelay = 2 * time.Second

// startHost is SPAWN/HOST/NOWAIT: it starts cmd and returns.
func (c *Interpreter) startHost(cmd *exec.Cmd, command string, finish func() error) error {
	job := &hostJob{cmd: cmd, command: command, output: &hostOutputQueue{}, finish: finish, done: make(chan error, 1)}

	if cmd.Stdout == nil {
		cmd.Stdout = job.output
	}

	if cmd.Stderr == nil {
		cmd.Stderr = job.output
	}

	cmd.WaitDelay = hostWaitDelay

	if err := cmd.Start(); err != nil {
		_ = finish()

		return vmserrors.New(vmserrors.VAX_HOSTCMD, command, err.Error())
	}

	go func() { job.done <- cmd.Wait() }()

	c.hostJobs = append(c.hostJobs, job)

	return nil
}

// pollHostJobs shows what the /NOWAIT programs have written, in whole
// lines, and finishes up after those that have ended, saying so only if
// something went wrong. Only DCL's goroutine may call it.
func (c *Interpreter) pollHostJobs() {
	running := c.hostJobs[:0]

	for _, job := range c.hostJobs {
		select {
		case err := <-job.done:
			c.endHostJob(job, err)

		default:
			c.showHostOutput(job.output.take(false))

			running = append(running, job)
		}
	}

	c.hostJobs = running
}

// endHostJob finishes up after a /NOWAIT program that has ended with err.
func (c *Interpreter) endHostJob(job *hostJob, err error) {
	c.showHostOutput(job.output.take(true))

	if err != nil {
		c.Printf("%s\n", vmserrors.New(vmserrors.VAX_HOSTCMD, job.command, err.Error()).Error())
	}

	if err := job.finish(); err != nil {
		c.Printf("%s\n", err.Error())
	}
}

// showHostOutput writes a program's text as DCL's output.
func (c *Interpreter) showHostOutput(text []byte) {
	if len(text) > 0 && c.Out != nil {
		_, _ = c.Out.Write(text)
	}
}

// EndHostJobs ends the /NOWAIT programs still running as govax ends, as a
// process's subprocesses end with it, and saves what they wrote to their
// /OUTPUT files while the volumes are still mounted.
func (c *Interpreter) EndHostJobs() {
	for _, job := range c.hostJobs {
		select {
		case err := <-job.done:
			c.endHostJob(job, err)

		default:
			if job.cmd != nil && job.cmd.Process != nil {
				_ = job.cmd.Process.Kill()
			}

			<-job.done
			c.endHostJob(job, nil) // ended by us, so no failure to report
		}
	}

	c.hostJobs = nil
}
