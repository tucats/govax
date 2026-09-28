package rtl

import (
	"github.com/tucats/govax/internal/vm"
)

// The mailbox driver (docs/PHASE-26.md subtask 29): the $QIO functions
// of a mailbox (mailbox.go).
//
// # How reads and writes meet
//
// A mailbox holds messages: whole records, not a stream of bytes. Each
// write adds one message; each read takes the oldest one. The two sides
// needn't be ready at the same moment, and the driver pairs them up:
//
//   - A **write** when a read is already waiting hands its message
//     straight to that read, and both complete. Otherwise the message is
//     queued. A plain write then stays pending until the message is read
//     (the writer learns its message arrived); with IO$M_NOW it completes
//     at once.
//   - A **read** takes the oldest queued message, completing the read
//     and, if it waited, that message's write. With no message, a plain
//     read stays pending until a write arrives; with IO$M_NOW it
//     completes at once with SS$_ENDOFFILE.
//
// This is the first govax device whose requests can really wait, which
// is why $QIO has pending requests, $QIOW waits for them, and $CANCEL
// cancels them (qio.go).
//
// # The IOSB
//
//	read:   status, the message's length (or the buffer's, if the message
//	        was longer: SS$_BUFFEROVF, and the rest is lost), and the
//	        writer's process ID
//	write:  status, the message's length, and the reader's process ID
//	        (0 for an IO$M_NOW write)
//
// An end-of-file message (IO$_WRITEOF) is read as SS$_ENDOFFILE with no
// data. A write longer than the mailbox's largest message is rejected
// with SS$_MBTOOSML.
//
// # A full mailbox
//
// A write that doesn't fit in the mailbox's buffer space (its BUFQUO)
// depends on the process's *resource wait mode* ($SETRWM). VMS enables
// it for every process, and then the writer waits, in the RWMBX state,
// until reads have made room: govax's $QIO/$QIOW service returns ErrWait
// and runs again (as $HIBER does), so ASTs are delivered while it waits
// and one of them can read. With resource wait mode disabled, or the
// IO$M_NORSWAIT modifier on the write, it completes at once with
// SS$_MBFULL instead. Nothing has happened to the mailbox while the
// writer waits: the request is made afresh each time.
//
// # Attention ASTs
//
// A program can ask to be told, by an AST, when something happens to a
// mailbox, instead of waiting in a read or a write. IO$_SETMODE with one
// of these modifiers enables an attention AST for the channel:
//
//	IO$M_READATTN        a message arrives that no read is waiting for
//	                     (so the program should read it)
//	IO$M_WRTATTN         a read starts waiting on an empty mailbox (so
//	                     the program should write)
//	IO$M_MB_ROOM_NOTIFY  a message is read, making room (so a writer
//	                     that found the mailbox full can try again)
//
// with p1 the AST routine (0 to disable the channel's), p2 its
// parameter, and p3 the access mode it runs in (maximized with the
// caller's). Each is delivered once and then forgotten: the program
// enables it again to hear about the next event. One whose event has
// already happened when it's enabled (a message already waiting, a read
// already waiting) is delivered at once. $CANCEL and $DASSGN on the
// channel forget its attention ASTs.
//
// Pending requests are dropped lazily: a cancelled read, or a message
// whose write was cancelled, is skipped the next time the mailbox is
// used (prune).

var (
	ioModNow        = ioCode("IO$M_NOW")
	ioModNoRSWait   = ioCode("IO$M_NORSWAIT")
	ioModReadAttn   = ioCode("IO$M_READATTN")
	ioModWrtAttn    = ioCode("IO$M_WRTATTN")
	ioModRoomNotify = ioCode("IO$M_MB_ROOM_NOTIFY")
)

// mailboxFunctions is the mailbox driver's function table: the read and
// write variants (virtual, logical, and physical block are the same for
// a mailbox), end of file, and sense/set mode.
var mailboxFunctions = map[uint32]ioFunc{
	ioCode("IO$_READVBLK"):  mbxRead,
	ioCode("IO$_READLBLK"):  mbxRead,
	ioCode("IO$_READPBLK"):  mbxRead,
	ioCode("IO$_WRITEVBLK"): mbxWrite,
	ioCode("IO$_WRITELBLK"): mbxWrite,
	ioCode("IO$_WRITEPBLK"): mbxWrite,
	ioCode("IO$_WRITEOF"):   mbxWriteEOF,
	ioCode("IO$_SENSEMODE"): mbxSense,
	ioCode("IO$_SENSECHAR"): mbxSense,
	ioCode("IO$_SETMODE"):   mbxSetMode,
	ioCode("IO$_SETCHAR"):   mbxSetMode,
}

// mailboxFor returns the request's mailbox. A channel to a device of the
// mailbox class always has one: only $CREMBX makes them.
func (env *Environment) mailboxFor(req *ioRequest) *Mailbox {
	m, _ := env.Mailboxes.For(req.channel.Device)

	return m
}

// prune drops finished requests: reads that were cancelled, and messages
// whose waiting write was cancelled.
func (m *Mailbox) prune() {
	readers := m.readers[:0]

	for _, r := range m.readers {
		if !r.done {
			readers = append(readers, r)
		}
	}

	m.readers = readers

	messages := m.messages[:0]

	for _, msg := range m.messages {
		if msg.writer == nil || !msg.writer.done {
			messages = append(messages, msg)
		}
	}

	m.messages = messages
}

// queuedBytes is how much buffer space the queued messages use.
func (m *Mailbox) queuedBytes() uint32 {
	n := uint32(0)
	for _, msg := range m.messages {
		n += uint32(len(msg.data))
	}

	return n
}

// mbxRead is IO$_READVBLK (and IO$_READLBLK, IO$_READPBLK):
//
//	p1  buffer address          p2  buffer size
//
// With IO$M_NOW, an empty mailbox completes the read with SS$_ENDOFFILE
// instead of leaving it pending. See this file's opening comment.
func mbxRead(env *Environment, req *ioRequest) (ioStatus, uint32) {
	if !env.accessible(req.p[0], req.p[1]&0xFFFF, vm.AccessWrite) {
		return ioStatus{}, ssAccVio
	}

	m := env.mailboxFor(req)
	m.prune()

	if len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]
		status := env.receive(req, msg)

		env.deliverAttention(&m.roomAttention)

		return status, 0
	}

	if req.modified(ioModNow) {
		return ioStatus{status: ssEndOfFile}, 0
	}

	m.readers = append(m.readers, req)

	env.deliverAttention(&m.writeAttention)

	return ioStatus{}, ioPending
}

// receive gives message msg to the read reader: the data goes into its
// buffer, and msg's waiting write (if any) completes. It returns the
// read's completion status.
func (env *Environment) receive(reader *ioRequest, msg *mailboxMessage) ioStatus {
	if msg.writer != nil {
		env.completeIO(msg.writer, ioStatus{status: ssNormal, count: uint16(len(msg.data)), info: env.Process.PID})
	}

	if msg.eof {
		return ioStatus{status: ssEndOfFile, info: msg.pid}
	}

	size := int(reader.p[1] & 0xFFFF)
	n := min(size, len(msg.data))

	// The buffer was checked when the read was queued.
	_ = env.mem.Store(env.cpu, reader.p[0], []byte(msg.data[:n]))

	status := uint32(ssNormal)
	if n < len(msg.data) {
		status = ssBufferOvf
	}

	return ioStatus{status: status, count: uint16(n), info: msg.pid}
}

// mbxWrite is IO$_WRITEVBLK (and IO$_WRITELBLK, IO$_WRITEPBLK):
//
//	p1  buffer address          p2  message size
//
// See this file's opening comment.
func mbxWrite(env *Environment, req *ioRequest) (ioStatus, uint32) {
	m := env.mailboxFor(req)
	size := req.p[1] & 0xFFFF

	if size > m.MaxMsg {
		return ioStatus{}, ssMbTooSml
	}

	if !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	data, err := loadBytes(env, req.p[0], int(size))
	if err != nil {
		return ioStatus{}, ssAccVio
	}

	return env.send(m, req, &mailboxMessage{data: data, pid: env.Process.PID})
}

// mbxWriteEOF is IO$_WRITEOF: an end-of-file message, which its reader
// gets as SS$_ENDOFFILE. It is otherwise a write of no data.
func mbxWriteEOF(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.send(env.mailboxFor(req), req, &mailboxMessage{eof: true, pid: env.Process.PID})
}

// send is the body of both writes: give msg to a waiting read, or queue
// it (see this file's opening comment).
func (env *Environment) send(m *Mailbox, req *ioRequest, msg *mailboxMessage) (ioStatus, uint32) {
	m.prune()

	count := uint16(len(msg.data))

	if len(m.readers) > 0 {
		reader := m.readers[0]
		m.readers = m.readers[1:]

		env.completeIO(reader, env.receive(reader, msg))

		return ioStatus{status: ssNormal, count: count, info: env.Process.PID}, 0
	}

	if m.queuedBytes()+uint32(len(msg.data)) > m.BufQuo {
		if env.Process.ResourceWaitDisabled || req.modified(ioModNoRSWait) {
			return ioStatus{status: ssMbFull}, 0
		}

		return ioStatus{}, ioResourceWait
	}

	m.messages = append(m.messages, msg)

	env.deliverAttention(&m.readAttention)

	if req.modified(ioModNow) {
		return ioStatus{status: ssNormal, count: count}, 0
	}

	msg.writer = req

	return ioStatus{}, ioPending
}

// mbxSense is IO$_SENSEMODE (and IO$_SENSECHAR): the IOSB's count word is
// the number of messages waiting, and its second longword the number of
// bytes they hold.
func mbxSense(env *Environment, req *ioRequest) (ioStatus, uint32) {
	m := env.mailboxFor(req)
	m.prune()

	return ioStatus{status: ssNormal, count: uint16(len(m.messages)), info: m.queuedBytes()}, 0
}

// mbxSetMode is IO$_SETMODE (and IO$_SETCHAR):
//
//	p1  AST routine (0: disable)   p2  AST parameter   p3  access mode
//
// With IO$M_READATTN, IO$M_WRTATTN, or IO$M_MB_ROOM_NOTIFY (any of them
// together), it enables or disables the channel's attention AST of that
// kind (see this file's opening comment), replacing one the channel
// already had. Without them (VMS uses it to change the mailbox's
// protection, which govax doesn't enforce), it does nothing.
func mbxSetMode(env *Environment, req *ioRequest) (ioStatus, uint32) {
	m := env.mailboxFor(req)
	m.prune()

	mode := max(req.p[2]&3, req.mode)

	kinds := []struct {
		modifier uint32
		list     *[]attentionRequest
		due      bool // its event has already happened
	}{
		{ioModReadAttn, &m.readAttention, len(m.messages) > 0},
		{ioModWrtAttn, &m.writeAttention, len(m.readers) > 0},
		{ioModRoomNotify, &m.roomAttention, false},
	}

	for _, k := range kinds {
		if !req.modified(k.modifier) {
			continue
		}

		*k.list = withoutChannel(*k.list, req.channel)

		if req.p[0] == 0 {
			continue
		}

		*k.list = append(*k.list, attentionRequest{channel: req.channel, ast: req.p[0], param: req.p[1], mode: mode})

		if k.due {
			env.deliverAttention(k.list)
		}
	}

	return ioStatus{status: ssNormal}, 0
}

// withoutChannel returns list without c's entries.
func withoutChannel(list []attentionRequest, c *channel) []attentionRequest {
	out := list[:0]

	for _, a := range list {
		if a.channel != c {
			out = append(out, a)
		}
	}

	return out
}

// deliverAttention queues every attention AST in list, which is then
// empty: each is delivered once.
func (env *Environment) deliverAttention(list *[]attentionRequest) {
	for _, a := range *list {
		env.queueAST(a.ast, a.param, a.mode)
	}

	*list = nil
}

// cancelAttention forgets channel c's attention ASTs, on whatever
// mailbox it's assigned to ($CANCEL, $DASSGN).
func (env *Environment) cancelAttention(c *channel) {
	if env.Mailboxes == nil {
		return
	}

	m, ok := env.Mailboxes.For(c.Device)
	if !ok {
		return
	}

	m.readAttention = withoutChannel(m.readAttention, c)
	m.writeAttention = withoutChannel(m.writeAttention, c)
	m.roomAttention = withoutChannel(m.roomAttention, c)
}
