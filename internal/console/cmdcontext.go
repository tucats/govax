package console

import (
	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
)

// cmd returns the context for a utility command run by the DCL running
// now.
func (c *Console) cmd() *dclcmd.Context {
	return &dclcmd.Context{
		Out:              c.Out,
		ContainerSession: c.ContainerSession,
		Logicals:         c.Logicals,
		RTL:              c.RTL,
		Mounts:           c.Mounts,
		Devices:          c.Devices,
		Paths:            c.Paths,
		HostLibrary:      c.HostLibrary,
		ShimLinks:        machine.ShimTable,
		ScreenSize:       c.ScreenSize,
		Verbose:          c.Verbose,
		Running:          c.Running,
		SetStatus:        func(status uint32) { c.SetCommandStatus(status, nil) },
		ReadReply:        c.ReadReply,
		TypeDCL:          c.TypeDCL,
		InputStream:      c.IsInputStream,
		VerifyImage:      c.VerifyImage,
	}
}

// Commands returns the context the utility commands work on, for the DCL
// running now: tests call the commands through it.
func (c *Console) Commands() *dclcmd.Context { return c.cmd() }
