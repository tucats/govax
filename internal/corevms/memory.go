package corevms

// LIBVM_* flags.
const (
	libvmMalloc = 1
	libvmLibrtl = 2
	libvmVirgin = 4
	libvmCarved = 8
	libvmZeroed = 16
)

// memBlock is one allocated-or-free memory block, the Go equivalent of
// struct MEMBLK.
type memBlock struct {
	addr, size, reqSize, flags, zone uint32
}

// zeroRange stores size zero bytes starting at addr, matching decc_malloc's
// own zero-fill loop for a LIBVM_ZEROED allocation.
func (env *Environment) zeroRange(addr, size uint32) error {
	for n := uint32(0); n < size; n++ {
		if err := env.mem.StoreByte(env.cpu, addr+n, 0); err != nil {
			return err
		}
	}

	return nil
}

// shimDeccMalloc is DECC$MALLOC/LIB$GET_VM's underlying allocator.
// Two-argument and three-argument forms come from LIB$GET_VM (flag=
// LIBVM_LIBRTL) and plain malloc() (flag=LIBVM_MALLOC, no zone) — see
// decc_malloc's own "kludge" comment.
func shimDeccMalloc(env *Environment, argv []uint32) (uint32, error) {
	size := argv[0]
	roundedSize := (size + 16) &^ 0xF

	flag := uint32(libvmMalloc)
	if len(argv) > 1 {
		flag = argv[1]
	}

	var zone uint32

	if len(argv) > 2 {
		zone = argv[2]
	}

	// Search the free list for a block that will do.
	for i, p := range env.memFreed {
		if p.size < size {
			continue
		}

		env.memFreed = append(env.memFreed[:i], env.memFreed[i+1:]...)

		if p.size > roundedSize {
			leftover := &memBlock{
				size:  p.size - roundedSize,
				addr:  p.addr + roundedSize,
				flags: libvmCarved,
			}
			env.memFreed = append(env.memFreed, leftover)
		}

		p.size = roundedSize
		p.reqSize = size
		p.flags = flag
		p.zone = zone
		env.memAllocated = append(env.memAllocated, p)

		if flag&libvmZeroed != 0 {
			if err := env.zeroRange(p.addr, size); err != nil {
				return 0xFFFFFFFF, nil
			}
		}

		return p.addr, nil
	}

	// Nothing on the free list fits; carve new space out of the P0 region.
	region := env.RegionSize[0]
	if region == 0 {
		region = 0x0200
	}

	q := &memBlock{size: roundedSize, reqSize: size, addr: region, zone: zone, flags: flag}
	env.memAllocated = append(env.memAllocated, q)

	if flag&libvmZeroed != 0 {
		if err := env.zeroRange(q.addr, size); err != nil {
			return 0xFFFFFFFF, nil
		}
	}

	pageRounded := (size + 512) &^ 0x1FF
	if q.size < pageRounded {
		env.memFreed = append(env.memFreed, &memBlock{
			size:  pageRounded - q.size,
			addr:  q.addr + q.size,
			flags: libvmVirgin,
		})
	}

	env.RegionSize[0] = region + pageRounded

	return q.addr, nil
}

// freeBlock is decc_free's core, shared by shimDeccFree and
// shimLibDeleteVMZone (which frees several blocks by zone). Returns false
// if addr doesn't fall inside any currently allocated block.
func (env *Environment) freeBlock(addr uint32) bool {
	idx := -1

	for i, p := range env.memAllocated {
		if addr >= p.addr && addr < p.addr+p.size {
			idx = i

			break
		}
	}

	if idx == -1 {
		return false
	}

	p := env.memAllocated[idx]
	env.memAllocated = append(env.memAllocated[:idx], env.memAllocated[idx+1:]...)
	p.flags = 0

	// See if this joins an existing free block, either immediately before
	// or immediately after it.
	for _, q := range env.memFreed {
		switch {
		case q.addr+q.size == p.addr: // q immediately precedes p
			q.size += p.size

			return true

		case p.addr+p.size == q.addr: // q immediately follows p
			q.addr = p.addr
			q.size += p.size

			return true
		}
	}

	env.memFreed = append(env.memFreed, p)

	return true
}

// shimDeccFree is DECC$FREE.
func shimDeccFree(env *Environment, argv []uint32) (uint32, error) {
	if !env.freeBlock(argv[0]) {
		return 0xFFFFFFFF, nil // -1
	}

	return 0, nil
}

func registerMemoryShims(t *ShimTable) {
	t.Register(15, "DECC$MALLOC", shimDeccMalloc)
	t.Register(16, "DECC$FREE", shimDeccFree)
}
