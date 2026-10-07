package console

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vm"
)

// imageProcess is one process's image state (Phase 43): the images
// activated in its P0, the symbols their activation defined, and where
// its IMAGE$INIT driver goes. Image activation (image.go, run.go) reads
// and writes the process's memory through its own address space, so an
// image can be activated in a process that isn't the current one.
//
// Process 1, the console's, has one for the console's whole life
// (Console.images), so what RUN loaded stays listed after an INIT or
// VMINIT, as it always has. Every other process gets one when an image is
// first activated in it (Console.imagesOf).
type imageProcess struct {
	c *Console

	// env is the process; nil for process 1, which is whatever
	// Environment the console's RTL is (VMINIT builds a new one).
	env *corevms.Environment

	// ICBList is the process's loaded-image list, in load order (main
	// image first, each dependency appended as it's loaded). RUN empties
	// it (resetICBList) before loading the next image.
	ICBList []*ICB

	// symbols are the transfer addresses activation found: MAIN, and
	// SHARE$name_INITIALIZE and SHARE$name_TRANSFER_n for each shareable
	// image. Process 1's are also set in the console's symbol table, so
	// EXAMINE MAIN and the like keep working; another process's are only
	// here, so activating an image in it can't change what the console's
	// names mean.
	symbols map[string]uint32

	// driver is the address of the page holding the process's IMAGE$INIT
	// driver: a pool page of its own, allocated the first time it's
	// needed. Zero for process 1, whose driver is at CONSOLE$SCRATCH+8.
	// Each process needs its own because the driver stays on the
	// process's call stack while its image runs: the image returns into
	// the driver's $EXIT call.
	driver uint32
}

// images returns process 1's image state.
func (c *Console) images() *imageProcess {
	if c.proc1Images == nil {
		c.proc1Images = &imageProcess{c: c, symbols: map[string]uint32{}}
	}

	return c.proc1Images
}

// imagesOf returns env's image state: process 1's for the console's own
// process, otherwise env's own, made empty the first time.
func (c *Console) imagesOf(env *corevms.Environment) *imageProcess {
	if env == nil || env == c.RTL {
		return c.images()
	}

	if c.otherImages == nil {
		c.otherImages = map[*corevms.Environment]*imageProcess{}
	}

	p := c.otherImages[env]
	if p == nil {
		p = &imageProcess{c: c, env: env, symbols: map[string]uint32{}}
		c.otherImages[env] = p
	}

	return p
}

// process returns the Environment p belongs to.
func (p *imageProcess) process() *corevms.Environment {
	if p.env == nil {
		return p.c.RTL
	}

	return p.env
}

// space returns the address space p's process's memory is reached
// through. Before VMINIT there's none: memory mapping is off, and every
// address is physical (TranslateIn returns it as it is).
func (p *imageProcess) space() vm.AddressSpace {
	if env := p.process(); env != nil && env.Space != nil {
		return env.Space.AddressSpace
	}

	return vm.CurrentAddressSpace(p.c.CPU)
}

// setSymbol records an activation symbol (see symbols).
func (p *imageProcess) setSymbol(name string, v uint32) {
	p.symbols[name] = v

	if p.env == nil {
		p.c.Symbols.Set(name, v, SymbolSystem)
	}
}

// The loader's memory access: through the process's address space, with
// kernel-mode protection (this is the console activating an image, not
// the program running).

func (p *imageProcess) storeBytes(addr uint32, data []byte) error {
	return p.c.Mem.StoreIn(p.c.CPU, p.space(), addr, data)
}

func (p *imageProcess) loadBytes(addr uint32, n int) ([]byte, error) {
	buf := make([]byte, n)

	return buf, p.c.Mem.LoadIn(p.c.CPU, p.space(), addr, buf)
}

func (p *imageProcess) loadByte(addr uint32) (byte, error) {
	b, err := p.loadBytes(addr, 1)
	if err != nil {
		return 0, err
	}

	return b[0], nil
}

func (p *imageProcess) loadWord(addr uint32) (uint16, error) {
	b, err := p.loadBytes(addr, 2)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint16(b), nil
}

func (p *imageProcess) loadLong(addr uint32) (uint32, error) {
	return p.c.Mem.LoadLongwordIn(p.c.CPU, p.space(), addr)
}

func (p *imageProcess) storeLong(addr, v uint32) error {
	return p.c.Mem.StoreLongwordIn(p.c.CPU, p.space(), addr, v)
}
