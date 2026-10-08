package corevms

import (
	"slices"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// RMS's record devices (docs/PHASE-46.md, subtask 6): the mailboxes and
// the null device internal/rms opens as record streams when a file
// specification names one (internal/rms/recdevice.go). This is the
// device side: the Environment is RMS's rms.DeviceOpener.
//
// Opening assigns a channel to the device, in executive mode (RMS's), so
// a temporary mailbox lasts while the file is open; closing deassigns
// it. The access the FAB asks for is checked against the mailbox's
// protection when it's opened (read for FAB$V_GET, write for FAB$V_PUT).
//
// A $PUT is a mailbox write that doesn't wait for its message to be
// read: a waiting read gets it at once, or it's queued and the $PUT
// completes. If the mailbox is full, the writer waits for room, as a
// $QIO write does in resource wait mode (RWMBX), or, with resource wait
// mode off, gets SS$_MBFULL. A $GET takes the oldest message, or, with
// none, waits (LEF) for one. *Unconfirmed:* that VMS's RMS doesn't wait
// for a $PUT's message to be read; the subtask's design says so, and an
// output mailbox a parent reads only when it chooses needs it.

// fabFACGet and fabFACPut are FAB$B_FAC's read and write bits.
var (
	fabFACGet = byte(vmsdef.Symbols["FAB$M_GET"])
	fabFACPut = byte(vmsdef.Symbols["FAB$M_PUT"])
)

// recordDevice is one RMS stream on a mailbox or NL:.
type recordDevice struct {
	env *Environment
	ch  *channel
	mbx *Mailbox // nil for NL:
}

// OpenRecordDevice is rms.DeviceOpener's one method: device (a physical
// name, or NL) is opened if it's a mailbox or the null device.
func (env *Environment) OpenRecordDevice(device string, fac byte) (rms.RecordDevice, bool, uint32) {
	if device == "NL" {
		device = nullDeviceName
	}

	d, ok := env.Devices.Find(device)
	if !ok || d.DevClass != iodev.DeviceClassMailbox {
		return nil, false, 0
	}

	mbx, isMailbox := env.Mailboxes.For(d)

	if isMailbox {
		want := uint32(0)
		if fac&fabFACGet != 0 {
			want |= accessRead
		}

		if fac&fabFACPut != 0 {
			want |= accessWrite
		}

		if !env.mailboxAccess(d, want) {
			return nil, true, ssNoPriv
		}
	}

	c := env.newChannel(device, d, uint32(vax.Executive))

	return &recordDevice{env: env, ch: c, mbx: mbx}, true, ssNormal
}

// Characteristics is the device's DEVCHAR.
func (r *recordDevice) Characteristics() uint32 { return r.env.devChar(r.ch.Device) }

// MaxRecord is a mailbox's largest message; NL: has no limit.
func (r *recordDevice) MaxRecord() uint32 {
	if r.mbx == nil {
		return 0
	}

	return r.mbx.MaxMsg
}

// Put writes record as one message (see this file's opening comment).
// It returns ErrWait while the mailbox is full.
func (r *recordDevice) Put(record []byte) (uint32, error) {
	m, env := r.mbx, r.env
	if m == nil {
		return ssNormal, nil
	}

	if uint32(len(record)) > m.MaxMsg {
		return ssMbTooSml, nil
	}

	req := &ioRequest{channel: r.ch, function: ioCode("IO$_WRITEVBLK"), modifiers: ioModNow, owner: env}

	st, reject := env.send(m, req, &mailboxMessage{data: string(record), pid: env.Process.PID})
	if reject == ioResourceWait {
		return 0, ErrWait
	}

	return st.status, nil
}

// Get reads the oldest message (see this file's opening comment). It
// returns ErrWait while the mailbox is empty.
func (r *recordDevice) Get() ([]byte, uint32, error) {
	m, env := r.mbx, r.env
	if m == nil {
		return nil, ssEndOfFile, nil
	}

	m.prune()

	if len(m.messages) == 0 {
		// A read is waiting for a message: a write attention AST is due,
		// and a writer waiting for room may hand its message over.
		env.deliverAttention(&m.writeAttention)

		if !slices.Contains(m.recordReaders, env) {
			m.recordReaders = append(m.recordReaders, env)
		}

		return nil, 0, env.waitOn(sched.StateLEF, sched.ResourceNone, eventFlagBoost, func() bool {
			m.prune()

			return len(m.messages) > 0
		})
	}

	m.recordReaders = slices.DeleteFunc(m.recordReaders, func(e *Environment) bool { return e == env })

	msg := m.messages[0]
	m.messages = m.messages[1:]

	if msg.writer != nil {
		env.completeIO(msg.writer, ioStatus{status: ssNormal, count: uint16(len(msg.data)), info: env.Process.PID})
	}

	env.deliverAttention(&m.roomAttention)
	env.mailboxResourceAvailable()

	if msg.eof {
		return nil, ssEndOfFile, nil
	}

	return []byte(msg.data), ssNormal, nil
}

// reportToRecordReaders tells the processes waiting in an RMS $GET on
// m that a message has come (reportEvent, with an I/O completion's
// boost): each whose wait is over runs as soon as the scheduler allows,
// rather than at its next look at the waiters. Every write that queues a
// message calls it (send), a $QIO write's as well as an RMS $PUT's.
func (m *Mailbox) reportToRecordReaders() {
	for _, e := range m.recordReaders {
		if e.waiting != nil {
			e.reportEvent(sched.ClassIOCompletion)
		}
	}
}

// Close gives back the stream's channel: a temporary mailbox with no
// other channel goes with it.
func (r *recordDevice) Close() {
	if r.mbx != nil {
		r.mbx.recordReaders = slices.DeleteFunc(r.mbx.recordReaders, func(e *Environment) bool { return e == r.env })
	}

	if slices.Contains(r.env.channels, r.ch) {
		r.env.releaseChannel(r.ch)
	}
}

// PutOutput writes record to SYS$OUTPUT as one record, which is what
// LIB$PUT_OUTPUT does. If SYS$OUTPUT names a mailbox or NL:, the record
// is a message written there, through a stream the process keeps open in
// executive mode (as VMS keeps SYS$OUTPUT open as a process-permanent
// file), which a change of SYS$OUTPUT's translation replaces; it returns
// the write's status, or ErrWait while the mailbox is full. Otherwise the
// record is a line on the terminal, and the status is SS$_NORMAL.
func (env *Environment) PutOutput(record string) (uint32, error) {
	device, st := env.deviceName("SYS$OUTPUT")
	if st != 0 {
		env.writeConsole(record + "\n")

		return ssNormal, nil
	}

	out := env.outputStream
	if out == nil || out.ch.Name != device || !slices.Contains(env.channels, out.ch) {
		if out != nil {
			out.Close()
		}

		env.outputStream = nil

		dev, found, st := env.OpenRecordDevice(device, fabFACPut)
		if !found {
			env.writeConsole(record + "\n")

			return ssNormal, nil
		}

		if st != ssNormal {
			return st, nil
		}

		out = dev.(*recordDevice)
		env.outputStream = out
	}

	return out.Put([]byte(record))
}
