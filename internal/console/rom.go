package console

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// romMagic is the exact 8-byte header a binary ROM image file starts with —
// ";ROMIMG\r" (note the trailing CR, not LF: a holdover from the original
// 1997-99 Mac development this project's own reference material notes
// elsewhere, confirmed against testdata/rom/xdefault.rom's actual bytes).
// console_rom's own reader only fgets 7 bytes of this ("read up to n-1"),
// leaving the CR for load_rom's first read to consume — this port just
// treats the whole 8 bytes as one fixed magic value.
var romMagic = [8]byte{';', 'R', 'O', 'M', 'I', 'M', 'G', '\r'}

// SaveROM writes the console's ROM image to path in the binary format
// save_binary.c's save_rom produces: the 8-byte magic, big-endian
// base/end addresses, then a sequence of (big-endian ROM-relative offset,
// big-endian page count [always 1], 512 bytes of page data) entries for
// every non-all-zero 512-byte page, terminated by a zero-count entry —
// matching reference/AUDIT.md's V1 finding that pins these fields to a
// literal 4 bytes each (already reflected in the C source read for this
// port, not a live bug to route around).
func (c *Console) SaveROM(path string) error {
	if len(c.ROM) == 0 {
		return fmt.Errorf("console: no ROM image loaded")
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(romMagic[:]); err != nil {
		return err
	}
	if err := writeBE32(f, c.ROMBase); err != nil {
		return err
	}
	if err := writeBE32(f, c.ROMEnd); err != nil {
		return err
	}

	for base := uint32(0); base+512 <= uint32(len(c.ROM)); base += 512 {
		page := c.ROM[base : base+512]
		if allZero(page) {
			continue
		}
		if err := writeBE32(f, base); err != nil {
			return err
		}
		if err := writeBE32(f, 1); err != nil {
			return err
		}
		if _, err := f.Write(page); err != nil {
			return err
		}
	}

	if err := writeBE32(f, 0); err != nil {
		return err
	}
	return writeBE32(f, 0)
}

// LoadROM reads a binary ROM image file, matching load_rom. path is
// resolved through c.Paths (docs/PHASE-15.md).
func (c *Console) LoadROM(path string) error {
	f, err := c.Paths.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var magic [8]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return fmt.Errorf("console: reading ROM magic: %w", err)
	}
	if magic != romMagic {
		return fmt.Errorf("console: %s is not a binary ROM image", path)
	}

	base, err := readBE32(f)
	if err != nil {
		return fmt.Errorf("console: reading ROM base address: %w", err)
	}
	end, err := readBE32(f)
	if err != nil {
		return fmt.Errorf("console: reading ROM end address: %w", err)
	}

	size := end + 1 - base
	rom := make([]byte, size)

	for {
		addr, err := readBE32(f)
		if err != nil {
			break // matches load_rom's own "short read ends the loop" behavior
		}
		count, err := readBE32(f)
		if err != nil {
			break
		}
		if count == 0 {
			break
		}
		if uint64(addr)+uint64(count)*512 > uint64(size) {
			return fmt.Errorf("console: ROM image too large; error loading page at %08X", addr)
		}
		if _, err := io.ReadFull(f, rom[addr:addr+count*512]); err != nil {
			return fmt.Errorf("console: reading ROM page at %08X: %w", addr, err)
		}
	}

	c.ROM = rom
	c.ROMBase = base
	c.ROMEnd = end
	return nil
}

// SaveNVRAM writes the console's NVRAM image to path, matching
// save_binary.c's save_nvram: big-endian base address and size (no magic
// header, and no per-page structure — the whole buffer is written at once).
func (c *Console) SaveNVRAM(path string) error {
	if len(c.NVRAM) == 0 {
		return fmt.Errorf("console: no NVRAM image loaded")
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := writeBE32(f, c.NVRAMBase); err != nil {
		return err
	}
	if err := writeBE32(f, uint32(len(c.NVRAM))); err != nil {
		return err
	}
	_, err = f.Write(c.NVRAM)
	return err
}

// LoadNVRAM reads an NVRAM image file, matching load_nvram. path is
// resolved through c.Paths (docs/PHASE-15.md).
func (c *Console) LoadNVRAM(path string) error {
	f, err := c.Paths.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	base, err := readBE32(f)
	if err != nil {
		return fmt.Errorf("console: reading NVRAM base address: %w", err)
	}
	size, err := readBE32(f)
	if err != nil {
		return fmt.Errorf("console: reading NVRAM size: %w", err)
	}

	nvram := make([]byte, size)
	if _, err := io.ReadFull(f, nvram); err != nil {
		return fmt.Errorf("console: reading NVRAM data: %w", err)
	}

	c.NVRAM = nvram
	c.NVRAMBase = base
	c.NVRAMEnd = base + size - 1
	return nil
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func writeBE32(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

func readBE32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}
