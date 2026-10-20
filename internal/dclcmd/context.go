package dclcmd

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
)

// Context is what the console's utility commands (DIRECTORY, TYPE,
// COPY, MOUNT, DEFINE, MACRO, LINK, ...) work on: the DCL running the
// command (its output, default directory, logical names, and process),
// the machine's devices and mounted volumes, and where govax finds its
// files. The console's Console.cmd makes one for each command.
type Context struct {
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

	// SharePrefix is SET SHARE_PREFIX's prefix, where a shareable image
	// is looked for on the host first (rms.ImageLocator).
	SharePrefix string

	// ShimLinks are the routines govax's shims stand in for, which LINK
	// takes as symbols (GovaxSymbols).
	ShimLinks []corevms.ShimLink

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

	// InputStream reports whether spec names SYS$INPUT, itself or
	// through logical names (LINK's SYS$INPUT/OPTIONS).
	InputStream func(spec string) bool

	// VerifyImage reports whether image verification is on, when a
	// utility shows the lines it reads from a file (LINK's options
	// files).
	VerifyImage func() bool
}

// Printf writes to the DCL's output.
func (c *Context) Printf(format string, args ...any) { fmt.Fprintf(c.Out, format, args...) }
