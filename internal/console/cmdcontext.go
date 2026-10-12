package console

// cmd returns the context for a utility command run by the DCL running
// now.
func (c *Console) cmd() *Context {
	return &Context{
		Out:              c.Out,
		ContainerSession: c.ContainerSession,
		Logicals:         c.Logicals,
		RTL:              c.RTL,
		Mounts:           c.Mounts,
		Devices:          c.Devices,
		Paths:            c.Paths,
		HostLibrary:      c.HostLibrary,
		ShimLinks:        shimTable,
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
func (c *Console) Commands() *Context { return c.cmd() }
