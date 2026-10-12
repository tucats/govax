package console

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
)

// cmdContext is what the console's utility commands (DIRECTORY, TYPE,
// COPY, MOUNT, DEFINE, MACRO, LINK, ...) work on: the DCL running the
// command (its output, default directory, logical names, and process),
// the machine's devices and mounted volumes, and where govax finds its
// files. Console.cmd makes one for each command.
type cmdContext struct {
	// Out is the DCL's output.
	Out io.Writer

	// ContainerSession is the DCL's default directory and file lookup
	// (Shell.ContainerSession), Logicals its process's logical names,
	// and RTL its process (nil before INIT).
	ContainerSession *rms.Session
	Logicals         *lnm.Database
	RTL              *corevms.Environment

	// Mounts and Devices are the machine's mounted volumes and devices.
	Mounts  *rms.MountTable
	Devices *iodev.DeviceTable

	// Paths is govax's search path for its own files, and HostLibrary
	// the host directory LINK and MACRO look in for VMS's libraries
	// (syslib.go).
	Paths       *respath.Resolver
	HostLibrary string

	// ScreenSize, when set, gives the screen's height and width for
	// paged output (page.go), instead of asking the terminal.
	ScreenSize func() (height, width int)

	// Verbose is SET VERBOSE's: whether MOUNT reports what it did.
	Verbose bool

	// Running says the DCL hasn't been told to quit, for a command that
	// reads its input until the end (CREATE).
	Running func() bool

	// SetStatus makes status the command's $STATUS (DEASSIGN, SHOW
	// LOGICAL).
	SetStatus func(status uint32)

	// ReadReply reads a line typed at the terminal (paging's "press
	// RETURN"); ok is false at its end.
	ReadReply func() (line string, ok bool)

	// TypeDCL types spec if it names something only DCL has (SYS$INPUT
	// in a procedure, a file DCL's OPEN opened), reporting whether it
	// did.
	TypeDCL func(spec string) (handled bool, err error)
}

// Printf writes to the DCL's output.
func (c *cmdContext) Printf(format string, args ...any) { fmt.Fprintf(c.Out, format, args...) }

// cmd returns the context for a utility command run by the DCL running
// now.
func (c *Console) cmd() *cmdContext {
	return &cmdContext{
		Out:              c.Out,
		ContainerSession: c.ContainerSession,
		Logicals:         c.Logicals,
		RTL:              c.RTL,
		Mounts:           c.Mounts,
		Devices:          c.Devices,
		Paths:            c.Paths,
		HostLibrary:      c.HostLibrary,
		ScreenSize:       c.ScreenSize,
		Verbose:          c.Verbose,
		Running:          c.Running,
		SetStatus:        func(status uint32) { c.setCommandStatus(status, nil) },
		ReadReply:        c.readReply,
		TypeDCL: func(spec string) (bool, error) {
			if c.isInputStream(spec) {
				return true, c.typeInput()
			}

			return c.typeOpenFile(spec), nil
		},
	}
}

// Commands returns the context the utility commands work on, for the DCL
// running now: tests call the commands through it.
func (c *Console) Commands() *cmdContext { return c.cmd() }
