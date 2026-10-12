package machine

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vm"
)

// ImageProcess is one process's image state (Phase 43): the images
// activated in its P0, the symbols their activation defined, and where
// its IMAGE$INIT driver goes. Image activation (image.go, run.go) reads
// and writes the process's memory through its own address space, so an
// image can be activated in a process that isn't the current one.
//
// Process 1, the console's, has one for the console's whole life
// (Console.images), so what RUN loaded stays listed after an INIT or
// VMINIT, as it always has. Every other process gets one when an image is
// first activated in it (Console.ImageState).
type ImageProcess struct {
	m *Machine

	// env is the process; nil for process 1, which is whatever
	// Environment the console's RTL is (VMINIT builds a new one).
	env *corevms.Environment

	// ICBList is the process's loaded-image list, in load order (main
	// image first, each dependency appended as it's loaded). RUN empties
	// it (ResetICBList) before loading the next image.
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
func (m *Machine) images() *ImageProcess {
	if m.proc1Images == nil {
		m.proc1Images = &ImageProcess{m: m, symbols: map[string]uint32{}}
	}

	return m.proc1Images
}

// ImageState returns env's image state: process 1's for the console's own
// process, otherwise env's own, made empty the first time.
func (m *Machine) ImageState(env *corevms.Environment) *ImageProcess {
	if env == nil || env == m.processOne {
		return m.images()
	}

	if m.otherImages == nil {
		m.otherImages = map[*corevms.Environment]*ImageProcess{}
	}

	p := m.otherImages[env]
	if p == nil {
		p = &ImageProcess{m: m, env: env, symbols: map[string]uint32{}}
		m.otherImages[env] = p
	}

	return p
}

// process returns the Environment p belongs to.
func (p *ImageProcess) process() *corevms.Environment {
	if p.env == nil {
		return p.m.processOne
	}

	return p.env
}

// space returns the address space p's process's memory is reached
// through. Before VMINIT there's none: memory mapping is off, and every
// address is physical (TranslateIn returns it as it is).
func (p *ImageProcess) space() vm.AddressSpace {
	if env := p.process(); env != nil && env.Space != nil {
		return env.Space.AddressSpace
	}

	return vm.CurrentAddressSpace(p.m.CPU)
}

// setSymbol records an activation symbol (see symbols).
func (p *ImageProcess) setSymbol(name string, v uint32) {
	p.symbols[name] = v

	if p.env == nil {
		p.m.Symbols.Set(name, v, SymbolSystem)
	}
}

// The loader's memory access: through the process's address space, with
// kernel-mode protection (this is the console activating an image, not
// the program running).

func (p *ImageProcess) storeBytes(addr uint32, data []byte) error {
	return p.m.Mem.StoreIn(p.m.CPU, p.space(), addr, data)
}

func (p *ImageProcess) loadBytes(addr uint32, n int) ([]byte, error) {
	buf := make([]byte, n)

	return buf, p.m.Mem.LoadIn(p.m.CPU, p.space(), addr, buf)
}

func (p *ImageProcess) loadByte(addr uint32) (byte, error) {
	b, err := p.loadBytes(addr, 1)
	if err != nil {
		return 0, err
	}

	return b[0], nil
}

func (p *ImageProcess) loadWord(addr uint32) (uint16, error) {
	b, err := p.loadBytes(addr, 2)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint16(b), nil
}

func (p *ImageProcess) loadLong(addr uint32) (uint32, error) {
	return p.m.Mem.LoadLongwordIn(p.m.CPU, p.space(), addr)
}

func (p *ImageProcess) storeLong(addr, v uint32) error {
	return p.m.Mem.StoreLongwordIn(p.m.CPU, p.space(), addr, v)
}

// Symbol is the value of name among the symbols p's images defined as
// they were activated, and true; false if none did.
func (p *ImageProcess) Symbol(name string) (uint32, bool) {
	v, ok := p.symbols[name]

	return v, ok
}
