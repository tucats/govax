package corevms

import (
	"fmt"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Mailboxes (docs/PHASE-26.md subtask 29): $CREMBX and $DELMBX.
//
// # What a mailbox is
//
// A mailbox is a software device for passing messages: one side writes
// a message with $QIO IO$_WRITEVBLK, the other reads it with
// IO$_READVBLK, and the mailbox holds messages in between, in order.
// VMS processes use mailboxes to talk to each other (and the system uses
// them to tell a process about events, such as a subprocess ending).
// Because it's a device, a mailbox is used through channels, like a
// terminal: $CREMBX creates one, named MBAn, and assigns a channel to it;
// other channels are assigned with $ASSIGN, by its name or by a logical
// name $CREMBX gave it.
//
// A mailbox is *temporary* (deleted when its last channel is deassigned)
// or *permanent* (kept until $DELMBX marks it for deletion, then deleted
// when its last channel goes).
//
// How reads and writes meet is the mailbox driver's business
// (mbxdriver.go); this file creates and deletes mailboxes.
//
// # In govax
//
// Each mailbox is a device record in the shared device table (class
// DC$_MAILBOX, so $GETDVI, SHOW DEVICE, and $ASSIGN see it), plus a
// Mailbox here holding its messages and waiting requests. govax has one
// process, so a mailbox connects the process with itself: a program's
// AST routines, or two parts of a program, can pass messages through it.
// Mailboxes are system state, like common event flag clusters: an
// Environment starts with none, and NewEnvironment removes any mailbox
// devices a previous Environment left in the device table.

// Status codes the mailbox services and driver return.
var (
	ssDevNotMbx = vmsdef.Symbols["SS$_DEVNOTMBX"]
	ssMbFull    = vmsdef.Symbols["SS$_MBFULL"]
	ssMbTooSml  = vmsdef.Symbols["SS$_MBTOOSML"]
)

// Mailbox defaults and limits. The defaults are the SYSGEN parameters
// DEFMBXMXMSG and DEFMBXBUFQUO at their usual values. The limit on the
// buffer quota is the manual's ("approximately 65355": 65535 less the
// size of a unit control block).
const (
	defaultMailboxMaxMsg = 256
	defaultMailboxBufQuo = 1056
	maxMailboxBufQuo     = 65355
	maxMailboxUnit       = 9999
	maxMailboxNameLength = 255

	// mailboxDevType is DT$_MBX, a mailbox's device type.
	mailboxDevType = 1
)

// mailboxDevChar is a mailbox's DEVCHAR: record oriented, available,
// shareable, a mailbox, capable of input and output.
var mailboxDevChar = vmsdef.Symbols["DEV$M_REC"] | vmsdef.Symbols["DEV$M_AVL"] |
	vmsdef.Symbols["DEV$M_SHR"] | vmsdef.Symbols["DEV$M_MBX"] |
	vmsdef.Symbols["DEV$M_IDV"] | vmsdef.Symbols["DEV$M_ODV"]

// Mailbox is one mailbox: its device record and its state.
type Mailbox struct {
	Device *iodev.Device
	Unit   uint32

	// Permanent is set for a permanent mailbox, and DeletePending once
	// $DELMBX has marked it for deletion.
	Permanent, DeletePending bool

	// MaxMsg is the largest message it takes, and BufQuo how many bytes
	// of messages it can hold. Protection is $CREMBX's promsk, recorded
	// but not enforced (govax has one process).
	MaxMsg, BufQuo, Protection uint32

	// The logical name $CREMBX gave it, if any: deleted with it.
	logicalName, logicalTable string
	logicalMode               lnm.Mode

	// messages are the messages written and not yet read, oldest first,
	// and readers the read requests waiting for one (mbxdriver.go).
	messages []*mailboxMessage
	readers  []*ioRequest

	// readAttention, writeAttention, and roomAttention are the attention
	// ASTs channels have enabled with IO$_SETMODE (mbxdriver.go): each is
	// delivered once, then forgotten.
	readAttention, writeAttention, roomAttention []attentionRequest
}

// attentionRequest is one attention AST a channel has enabled on a
// mailbox: the routine, its parameter, and the access mode it runs in.
type attentionRequest struct {
	channel          *channel
	ast, param, mode uint32
}

// mailboxMessage is one message in a mailbox.
type mailboxMessage struct {
	data string
	eof  bool   // an end-of-file message (IO$_WRITEOF)
	pid  uint32 // the writer's process ID, which the reader's IOSB gets

	// writer is the write request, if it waits for the message to be
	// read (a write without IO$M_NOW); nil if it has completed.
	writer *ioRequest
}

// Messages reports how many messages are waiting to be read.
func (m *Mailbox) Messages() int {
	m.prune()

	return len(m.messages)
}

// MailboxTable is the system's mailboxes.
type MailboxTable struct {
	byDevice map[*iodev.Device]*Mailbox
	lastUnit uint32
}

// NewMailboxTable returns an empty table.
func NewMailboxTable() *MailboxTable {
	return &MailboxTable{byDevice: map[*iodev.Device]*Mailbox{}}
}

// For returns the mailbox whose device is d, if d is a mailbox.
func (t *MailboxTable) For(d *iodev.Device) (*Mailbox, bool) {
	m, ok := t.byDevice[d]

	return m, ok
}

// All returns every mailbox, in no particular order.
func (t *MailboxTable) All() []*Mailbox {
	out := make([]*Mailbox, 0, len(t.byDevice))
	for _, m := range t.byDevice {
		out = append(out, m)
	}

	return out
}

// nextUnit returns the unit number for a new mailbox: the next after
// the last one used, from 1 to 9999 and then round again, skipping units
// still in use (VMS's rule). 0 if all are in use.
func (t *MailboxTable) nextUnit(devices *iodev.DeviceTable) uint32 {
	for range maxMailboxUnit {
		t.lastUnit = t.lastUnit%maxMailboxUnit + 1

		if _, used := devices.Find(fmt.Sprintf("MBA%d", t.lastUnit)); !used {
			return t.lastUnit
		}
	}

	return 0
}

// removeStaleMailboxes deletes the mailbox devices a previous System
// left in the shared device table: their messages were in its memory, so
// they don't survive INIT/VMINIT/ZERO. It runs once per System, not per
// process: a process created later must find the mailboxes other
// processes have made (docs/PHASE-43.md, bug 2).
func (sys *System) removeStaleMailboxes() {
	if sys.Devices == nil {
		return
	}

	for _, d := range sys.Devices.All() {
		if d.DevClass == iodev.DeviceClassMailbox {
			sys.Devices.Remove(d)
		}
	}
}

// serviceSysCrembx is SYS$CREMBX:
//
//	SYS$CREMBX [prmflg] ,chan ,[maxmsg] ,[bufquo] ,[promsk] ,[acmode] ,[lognam]
//
// It creates a mailbox, temporary (prmflg 0) or permanent (1), and
// assigns a channel to it from access mode acmode (maximized with the
// caller's), storing the channel's number in the word at chan. maxmsg and
// bufquo are the largest message and the space for messages, in bytes (0:
// the system defaults, 256 and 1056). The new mailbox is MBAn, n the next
// unit number.
//
// With lognam, a logical name for it is defined in LNM$TEMPORARY_MAILBOX
// or LNM$PERMANENT_MAILBOX (the job table and the system table),
// equated to "MBAn:" with the terminal attribute. If that name already
// names a mailbox, no new one is made: the channel is assigned to that
// one, so two parts of a program needn't agree which creates it.
//
// Creating a mailbox needs the TMPMBX privilege (a temporary one) or
// PRMMBX (a permanent one), and a permanent one's logical name, which goes
// in the system table, SYSNAM as well (privilege.go); finding an existing
// one by name needs neither.
//
// Status: SS$_IVSTSFLG for another prmflg; SS$_NOPRIV for a missing
// privilege; SS$_BADPARAM for a bufquo over 65355; SS$_IVLOGNAM for a lognam that's empty or over 255 characters;
// SS$_ACCVIO for a chan that can't be written or a lognam that can't be
// read; SS$_NOIOCHAN when all 9999 units are in use; an error of the
// logical-name define (SS$_TOOMANYLNAM, ...).
func serviceSysCrembx(env *Environment, argv []uint32) (uint32, error) {
	prmflg, chanAdr := byte(optArg(argv, 0)), optArg(argv, 1)
	maxmsg, bufquo, promsk := optArg(argv, 2), optArg(argv, 3), optArg(argv, 4)
	mode := max(optArg(argv, 5)&3, uint32(env.cpu.PSL().CurMod()))
	lognam := optArg(argv, 6)

	if prmflg > 1 {
		return ssIvStsFlg, nil
	}

	if maxmsg == 0 {
		maxmsg = defaultMailboxMaxMsg
	}

	if bufquo == 0 {
		bufquo = defaultMailboxBufQuo
	}

	if bufquo > maxMailboxBufQuo {
		return ssBadParam, nil
	}

	// The channel number goes here: check it can be written first, so a
	// failure leaves no mailbox behind.
	if chanAdr == 0 || env.mem.StoreWord(env.cpu, chanAdr, 0) != nil {
		return ssAccVio, nil
	}

	table := "LNM$TEMPORARY_MAILBOX"
	if prmflg == 1 {
		table = "LNM$PERMANENT_MAILBOX"
	}

	name := ""

	if lognam != 0 {
		s, ok, err := strGet(env, lognam, maxMailboxNameLength)
		if err != nil {
			return ssAccVio, nil
		}

		if !ok || s == "" {
			return ssIvLogNam, nil
		}

		name = s

		// An existing mailbox by that name gets the channel.
		if d, ok := env.namedMailbox(table, name); ok {
			env.storeNewChannel(chanAdr, name, d, mode)

			return ssNormal, nil
		}
	}

	priv := privTMPMBX
	if prmflg == 1 {
		priv = privPRMMBX

		if name != "" {
			priv |= privSYSNAM
		}
	}

	if !env.Process.hasPrivilege(priv) {
		return ssNoPriv, nil
	}

	unit := env.Mailboxes.nextUnit(env.Devices)
	if unit == 0 {
		return vmsdef.Symbols["SS$_NOIOCHAN"], nil
	}

	device := fmt.Sprintf("MBA%d", unit)
	d := env.Devices.Define(device, iodev.DeviceOptions{
		DevClass:   iodev.DeviceClassMailbox,
		DevType:    mailboxDevType,
		DevBufSize: maxmsg,
		DevChar:    mailboxDevChar,
		OwnUIC:     env.Process.UIC,
	})

	m := &Mailbox{Device: d, Unit: unit, Permanent: prmflg == 1, MaxMsg: maxmsg, BufQuo: bufquo, Protection: promsk & 0xFFFF}

	if name != "" {
		eqv := []lnm.Equivalence{{Value: device + ":", Attrs: lnm.AttrTerminal}}
		if _, err := env.Logicals.Define(table, name, lnm.Mode(mode), 0, eqv); err != nil {
			env.Devices.Remove(d)

			return lnmStatus(err)
		}

		m.logicalName, m.logicalTable, m.logicalMode = name, table, lnm.Mode(mode)
	}

	env.Mailboxes.byDevice[d] = m
	env.storeNewChannel(chanAdr, device, d, mode)

	return ssNormal, nil
}

// namedMailbox returns the mailbox device the logical name
// translates to in table, if it is one.
func (env *Environment) namedMailbox(table, name string) (*iodev.Device, bool) {
	e, err := env.Logicals.Translate(table, name, lnm.User, 0)
	if err != nil || len(e.Equivalences) == 0 {
		return nil, false
	}

	d, found := env.Devices.Find(e.Equivalences[0].Value)
	if !found {
		return nil, false
	}

	_, isMailbox := env.Mailboxes.For(d)

	return d, isMailbox
}

// storeNewChannel assigns a channel to d from mode and stores its number
// at chanAdr, which the caller has checked can be written.
func (env *Environment) storeNewChannel(chanAdr uint32, name string, d *iodev.Device, mode uint32) {
	c := env.newChannel(name, d, mode)
	_ = env.mem.StoreWord(env.cpu, chanAdr, c.Number)
}

// serviceSysDelmbx is SYS$DELMBX:
//
//	SYS$DELMBX chan
//
// It marks the mailbox that channel chan (low word) is assigned to for
// deletion. The mailbox goes when its last channel is deassigned; the
// caller's own channel isn't deassigned. (A temporary mailbox is deleted
// that way anyway.) SS$_IVCHAN for 0, SS$_NOPRIV if the channel isn't
// assigned or was assigned from a more privileged mode than the caller's
// or the caller lacks the PRMMBX privilege, SS$_DEVNOTMBX if its device
// isn't a mailbox.
func serviceSysDelmbx(env *Environment, argv []uint32) (uint32, error) {
	number := optArg(argv, 0) & 0xFFFF
	if number == 0 {
		return ssIvChan, nil
	}

	c, found := env.findChannel(number)
	if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
		return ssNoPriv, nil
	}

	m, ok := env.Mailboxes.For(c.Device)
	if !ok {
		return ssDevNotMbx, nil
	}

	if !env.Process.hasPrivilege(privPRMMBX) {
		return ssNoPriv, nil
	}

	m.DeletePending = true

	return ssNormal, nil
}

// releaseMailbox deletes the mailbox on d, if d is one whose last channel
// has just gone and that should go: a temporary mailbox, or a permanent
// one marked for deletion. Its device record and logical name go with
// it, and any messages in it are lost.
func (env *Environment) releaseMailbox(d *iodev.Device) {
	m, ok := env.Mailboxes.For(d)
	if !ok || d.RefCnt > 0 || (m.Permanent && !m.DeletePending) {
		return
	}

	delete(env.Mailboxes.byDevice, d)
	env.Devices.Remove(d)

	if m.logicalName != "" {
		_, _ = env.Logicals.Delete(m.logicalTable, m.logicalName, m.logicalMode)
	}
}

func registerMailboxServices(t *ServiceTable) {
	t.Register("SYS$CREMBX", serviceSysCrembx)
	t.Register("SYS$DELMBX", serviceSysDelmbx)
}
