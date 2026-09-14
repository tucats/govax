package vax

// PSL is the VAX Processor Status Longword. Bit layout matches the VAX
// architecture manual's PSL diagram (and the non-BIGENDIAN branch of the C
// source's struct PSL_BITS, which is LSB-first declaration order and agrees
// with it on every actually-supported modern little-endian build target):
//
//	31   30   29 28   27  26  25 24   23 22   21   20   16 15   8 7  6  5 4 3 2 1 0
//	CM | TP | MBZ | FPD | IS | CUR_MOD | PRV_MOD | MBZ | IPL | MBZ | DV FU IV T N Z V C
//
// The C source additionally keeps a "wide" struct PSL_W shadow copy of these
// bits, synced via read_psl_bits/write_psl_bits, purely as a 1999-era speed
// hack for C bitfield access. Go bit-masking has no such penalty to work
// around, so PSL_W is not ported — this single uint32 with accessor methods
// stands in for both union PSL and struct PSL_W.
type PSL uint32

const (
	pslC      = 1 << 0
	pslV      = 1 << 1
	pslZ      = 1 << 2
	pslN      = 1 << 3
	pslT      = 1 << 4
	pslIV     = 1 << 5
	pslFU     = 1 << 6
	pslDV     = 1 << 7
	pslIPLLo  = 16
	pslIPLLen = 5
	pslIPL    = 0x1F << pslIPLLo
	pslPrvLo  = 22
	pslPrv    = 0x3 << pslPrvLo
	pslCurLo  = 24
	pslCur    = 0x3 << pslCurLo
	pslIS     = 1 << 26
	pslFPD    = 1 << 27
	pslTP     = 1 << 30
	pslCM     = 1 << 31
)

// AccessMode is one of the four VAX processor access modes, used by CUR_MOD
// and PRV_MOD.
type AccessMode uint32

// Access mode values, matching arch.h's AM_* constants: kernel is the most
// privileged, user the least.
const (
	Kernel AccessMode = iota
	Executive
	Supervisor
	User
)

func flag(v PSL, mask uint32) bool { return v&PSL(mask) != 0 }

func setFlag(v PSL, mask uint32, on bool) PSL {
	if on {
		return v | PSL(mask)
	}
	return v &^ PSL(mask)
}

// Condition codes, set by most instructions after computing a result.

func (p PSL) N() bool { return flag(p, pslN) }
func (p PSL) Z() bool { return flag(p, pslZ) }
func (p PSL) V() bool { return flag(p, pslV) }
func (p PSL) C() bool { return flag(p, pslC) }

func (p *PSL) SetN(v bool) { *p = setFlag(*p, pslN, v) }
func (p *PSL) SetZ(v bool) { *p = setFlag(*p, pslZ, v) }
func (p *PSL) SetV(v bool) { *p = setFlag(*p, pslV, v) }
func (p *PSL) SetC(v bool) { *p = setFlag(*p, pslC, v) }

// SetNZVC sets all four condition codes at once, as most instructions do
// after computing a result.
func (p *PSL) SetNZVC(n, z, v, c bool) {
	p.SetN(n)
	p.SetZ(z)
	p.SetV(v)
	p.SetC(c)
}

// Trap/trace enables and trace pending.

func (p PSL) T() bool  { return flag(p, pslT) }
func (p PSL) IV() bool { return flag(p, pslIV) }
func (p PSL) FU() bool { return flag(p, pslFU) }
func (p PSL) DV() bool { return flag(p, pslDV) }
func (p PSL) TP() bool { return flag(p, pslTP) }

func (p *PSL) SetT(v bool)  { *p = setFlag(*p, pslT, v) }
func (p *PSL) SetIV(v bool) { *p = setFlag(*p, pslIV, v) }
func (p *PSL) SetFU(v bool) { *p = setFlag(*p, pslFU, v) }
func (p *PSL) SetDV(v bool) { *p = setFlag(*p, pslDV, v) }
func (p *PSL) SetTP(v bool) { *p = setFlag(*p, pslTP, v) }

// Privileged fields: interrupt priority level, access modes, interrupt
// stack, first-part-done, compatibility mode.

func (p PSL) IPL() uint32 { return uint32(p>>pslIPLLo) & (1<<pslIPLLen - 1) }

func (p *PSL) SetIPL(v uint32) {
	*p = (*p &^ pslIPL) | PSL(v&(1<<pslIPLLen-1))<<pslIPLLo
}

func (p PSL) CurMod() AccessMode { return AccessMode(p>>pslCurLo) & 0x3 }

func (p *PSL) SetCurMod(m AccessMode) {
	*p = (*p &^ pslCur) | PSL(m&0x3)<<pslCurLo
}

func (p PSL) PrvMod() AccessMode { return AccessMode(p>>pslPrvLo) & 0x3 }

func (p *PSL) SetPrvMod(m AccessMode) {
	*p = (*p &^ pslPrv) | PSL(m&0x3)<<pslPrvLo
}

func (p PSL) IS() bool  { return flag(p, pslIS) }
func (p PSL) FPD() bool { return flag(p, pslFPD) }
func (p PSL) CM() bool  { return flag(p, pslCM) }

func (p *PSL) SetIS(v bool)  { *p = setFlag(*p, pslIS, v) }
func (p *PSL) SetFPD(v bool) { *p = setFlag(*p, pslFPD, v) }
func (p *PSL) SetCM(v bool)  { *p = setFlag(*p, pslCM, v) }

// PSL returns the processor status longword.
func (c *CPU) PSL() PSL { return c.psl }

// SetPSL replaces the processor status longword wholesale.
func (c *CPU) SetPSL(p PSL) { c.psl = p }
