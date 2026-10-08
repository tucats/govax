package lck

import (
	"errors"
	"slices"
)

// ID is a lock identification: what $ENQ returns in the lock status block
// and what $ENQ (for a conversion) and $DEQ are given back. 0 is never a
// lock's.
type ID uint32

// Owner is the process that owns a lock: govax passes the process ID.
type Owner uint32

// ValueBlock is a resource's 16-byte lock value block.
type ValueBlock [16]byte

// MaxNameLength is the longest resource name $ENQ accepts.
const MaxNameLength = 31

// State is where a lock is on its resource's queues.
type State uint8

// The three lock states. (The numbering is govax's own; $GETLKI's
// LKI$C_ codes, if it's ever implemented, are a separate mapping.)
const (
	Granted    State = iota // on the granted queue
	Converting              // on the conversion queue: granted at Mode, asking for Requested
	Waiting                 // on the waiting queue: not granted at all yet
)

// String names the state as SHOW commands do.
func (s State) String() string {
	return [...]string{"Granted", "Converting", "Waiting"}[s]
}

// The errors the Manager's operations return. Each corresponds to a VMS
// condition, named in its comment, that the services report.
var (
	ErrBadMode          = errors.New("lck: invalid lock mode")                      // SS$_BADPARAM
	ErrBadName          = errors.New("lck: resource name length is 0 or over 31")   // SS$_IVBUFLEN
	ErrInvalidLock      = errors.New("lck: no such lock, or not the caller's")      // SS$_IVLOCKID
	ErrParentNotGranted = errors.New("lck: the parent lock isn't granted")          // SS$_PARNOTGRANT
	ErrNotQueued        = errors.New("lck: not granted at once, and NOQUEUE given") // SS$_NOTQUEUED
	ErrNotGranted       = errors.New("lck: the lock to convert isn't granted")      // SS$_CVTUNGRANT
	ErrSublocks         = errors.New("lck: the lock has sublocks")                  // SS$_SUBLOCKS
	ErrCancelGranted    = errors.New("lck: the conversion was already granted")     // SS$_CANCELGRANT
)

// Lock is one lock: a process's claim, granted or not, on a resource.
type Lock struct {
	ID    ID
	Owner Owner

	// AccessMode is the access mode the lock was taken from (0 kernel to
	// 3 user): $DEQ with LCK$M_DEQALL dequeues the locks at an access
	// mode and less privileged ones.
	AccessMode uint8

	// Mode is the mode the lock is granted at (meaningless while
	// Waiting), and Requested the mode asked for: the same as Mode once
	// granted, the new mode while Converting.
	Mode      Mode
	Requested Mode
	State     State

	// Parent is the lock this one is a sublock of, or nil; Resource is
	// the resource it locks.
	Parent   *Lock
	Resource *Resource

	// Blocking is whether the owner asked to be told (a blocking AST)
	// when this lock blocks a request.
	Blocking bool

	// ValueBlock is whether the latest request ($ENQ, or the conversion)
	// asked for the value block. Value is the lock's copy of it: the
	// resource's, copied when the lock was granted or converted up, and
	// ValueNotValid reports that the resource's was marked invalid then
	// (SS$_VALNOTVALID).
	ValueBlock    bool
	Value         ValueBlock
	ValueNotValid bool

	// Data is the caller's: what it needs to complete the request in the
	// owner's process (its lock status block, event flag, and ASTs).
	Data any

	// notified is whether Blocking's notice has been given since the
	// lock was last granted; sublocks counts the locks whose Parent this
	// is.
	notified bool
	sublocks int
}

// Resource is a named resource, existing while any lock names it.
type Resource struct {
	// Name is the resource name; Group the UIC group it's named in (0 for
	// a system resource); AccessMode the access mode it's named in; and
	// Parent the resource of the parent lock, for a sub-resource.
	Name       string
	Group      uint16
	AccessMode uint8
	Parent     *Resource

	// Value is the resource's value block, and valueInvalid whether it's
	// been marked invalid since it was last written.
	Value        ValueBlock
	valueInvalid bool

	// The three queues, in order (the granted queue's order doesn't
	// matter); and children, the sub-resources naming this one Parent.
	granted, converting, waiting []*Lock
	children                     int
}

// Locks returns the resource's locks: the granted queue, then the
// conversion queue, then the waiting queue.
func (r *Resource) Locks() []*Lock {
	return slices.Concat(r.granted, r.converting, r.waiting)
}

// resourceKey is what makes two resources the same (section 13.2.1's
// list: parent resource, group, access mode, and name).
type resourceKey struct {
	parent *Resource
	group  uint16
	mode   uint8
	name   string
}

// Manager is a lock database: every resource and lock in the system.
type Manager struct {
	locks     map[ID]*Lock
	resources map[resourceKey]*Resource
	lastID    ID
}

// NewManager returns an empty lock database.
func NewManager() *Manager {
	return &Manager{locks: map[ID]*Lock{}, resources: map[resourceKey]*Resource{}}
}

// EventKind is what happened to a lock, for its owner to be told.
type EventKind uint8

// The kinds of Event.
const (
	// EventGranted: a waiting lock was granted, or a queued conversion
	// was. Its $ENQ completes: SS$_NORMAL (or SS$_VALNOTVALID) in the
	// lock status block, the event flag, the completion AST.
	EventGranted EventKind = iota

	// EventBlocking: the lock blocks a request; its blocking AST is due.
	EventBlocking

	// EventAborted: a waiting lock, or a converting one, was dequeued
	// before it was granted: its $ENQ completes with SS$_ABORT.
	EventAborted

	// EventCanceled: a queued conversion was canceled ($DEQ with
	// LCK$M_CANCEL); the lock stays granted at its old mode, and the
	// conversion's $ENQ completes with SS$_CANCEL.
	EventCanceled
)

// Event is one thing an operation did to a lock: possibly one owned by a
// process other than the caller.
type Event struct {
	Kind EventKind
	Lock *Lock
}

// Request is a new lock request: $ENQ's arguments, decoded.
type Request struct {
	Owner Owner
	Mode  Mode

	// Name is the resource name; Group the UIC group it's named in (0
	// with LCK$M_SYSTEM); AccessMode the access mode (the least
	// privileged of the caller's and $ENQ's acmode). With a Parent, the
	// parent lock's access mode is used instead.
	Name       string
	Group      uint16
	AccessMode uint8
	Parent     ID

	// NoQueue, ValueBlock, and Blocking are LCK$M_NOQUEUE,
	// LCK$M_VALBLK, and whether a blocking AST was given.
	NoQueue    bool
	ValueBlock bool
	Blocking   bool

	// Data is kept in the lock for the caller (see Lock.Data).
	Data any
}

// Enqueue requests a new lock. It returns the lock, granted or waiting,
// and the events the request caused for other locks (blocking notices).
// A lock granted at once has no event of its own: the caller completes
// the request itself.
func (m *Manager) Enqueue(req Request) (*Lock, []Event, error) {
	if !req.Mode.Valid() {
		return nil, nil, ErrBadMode
	}

	if len(req.Name) == 0 || len(req.Name) > MaxNameLength {
		return nil, nil, ErrBadName
	}

	var parent *Lock

	if req.Parent != 0 {
		p, ok := m.locks[req.Parent]
		if !ok || p.Owner != req.Owner {
			return nil, nil, ErrInvalidLock
		}

		if p.State == Waiting {
			return nil, nil, ErrParentNotGranted
		}

		parent = p
		req.AccessMode = p.AccessMode
	}

	var parentRes *Resource
	if parent != nil {
		parentRes = parent.Resource
	}

	key := resourceKey{parent: parentRes, group: req.Group, mode: req.AccessMode, name: req.Name}
	res := m.resources[key]

	// A resource that doesn't exist yet can't be locked by anyone: the
	// request is grantable, so the NOQUEUE test below needs no resource.
	fresh := res == nil
	if fresh {
		res = &Resource{Name: req.Name, Group: req.Group, AccessMode: req.AccessMode, Parent: parentRes}
	}

	l := &Lock{
		Owner: req.Owner, AccessMode: req.AccessMode,
		Requested: req.Mode, State: Waiting,
		Parent: parent, Resource: res,
		Blocking: req.Blocking, ValueBlock: req.ValueBlock, Data: req.Data,
	}

	grantable := len(res.converting) == 0 && len(res.waiting) == 0 && compatibleWithAll(res, l, req.Mode)
	if !grantable && req.NoQueue {
		return nil, nil, ErrNotQueued
	}

	if fresh {
		m.resources[key] = res

		if parentRes != nil {
			parentRes.children++
		}
	}

	if parent != nil {
		parent.sublocks++
	}

	l.ID = m.newID()
	m.locks[l.ID] = l

	if grantable {
		m.grant(l, true)
		res.granted = append(res.granted, l)
	} else {
		res.waiting = append(res.waiting, l)
	}

	return l, m.blockingNotices(res), nil
}

// newID returns an unused lock ID.
func (m *Manager) newID() ID {
	for {
		m.lastID++
		if _, used := m.locks[m.lastID]; m.lastID != 0 && !used {
			return m.lastID
		}
	}
}

// Lock returns the lock with ID id, if there is one.
func (m *Manager) Lock(id ID) (*Lock, bool) {
	l, ok := m.locks[id]

	return l, ok
}

// Locks returns the locks owner holds or waits for, by ID.
func (m *Manager) Locks(owner Owner) []*Lock {
	var out []*Lock

	for _, l := range m.locks {
		if l.Owner == owner {
			out = append(out, l)
		}
	}

	slices.SortFunc(out, func(a, b *Lock) int { return int(a.ID) - int(b.ID) })

	return out
}

// ConvertOptions are a conversion's options: LCK$M_NOQUEUE and
// LCK$M_VALBLK, with the caller's value block (stored if the lock is
// converted down from PW or EX); whether the conversion asks for a
// blocking AST; and the caller's new Data for the lock.
type ConvertOptions struct {
	NoQueue    bool
	ValueBlock bool
	Value      ValueBlock
	Blocking   bool
	Data       any
}

// Convert asks for lock id's mode to change to mode. It returns the
// lock, granted at the new mode or converting, and the events caused for
// other locks. With NoQueue, a conversion that can't be granted at once
// leaves the lock as it was (ErrNotQueued).
func (m *Manager) Convert(owner Owner, id ID, mode Mode, opts ConvertOptions) (*Lock, []Event, error) {
	if !mode.Valid() {
		return nil, nil, ErrBadMode
	}

	l, ok := m.locks[id]
	if !ok || l.Owner != owner {
		return nil, nil, ErrInvalidLock
	}

	if l.State != Granted {
		return nil, nil, ErrNotGranted
	}

	res := l.Resource

	grantable := mode.noMoreRestrictive(l.Mode) ||
		len(res.converting) == 0 && compatibleWithAll(res, l, mode)
	if !grantable && opts.NoQueue {
		return nil, nil, ErrNotQueued
	}

	l.ValueBlock, l.Blocking, l.Data = opts.ValueBlock, opts.Blocking, opts.Data
	l.Requested = mode

	if !grantable {
		l.State = Converting
		res.granted = remove(res.granted, l)
		res.converting = append(res.converting, l)

		return l, m.blockingNotices(res), nil
	}

	// A PW or EX holder converting down stores its value block.
	if opts.ValueBlock && l.Mode.writes() && mode < l.Mode {
		res.Value = opts.Value
		res.valueInvalid = false
	}

	m.grant(l, mode > l.Mode)

	// A lock that converted down may let queued requests through.
	events := m.regrant(res)

	return l, append(events, m.blockingNotices(res)...), nil
}

// DequeueOptions are $DEQ's options for one lock: LCK$M_CANCEL (cancel a
// queued conversion or a waiting request, nothing else), the caller's
// value block (stored if the lock is PW or EX), and LCK$M_INVVALBLK
// (mark the resource's value block invalid instead, for a PW or EX lock).
type DequeueOptions struct {
	Cancel     bool
	Value      *ValueBlock
	Invalidate bool
}

// Dequeue removes lock id, or with Cancel cancels its queued request. It
// returns the events it caused: for the lock itself (EventAborted or
// EventCanceled, for a request not yet granted) and for other locks
// granted because it went.
func (m *Manager) Dequeue(owner Owner, id ID, opts DequeueOptions) ([]Event, error) {
	l, ok := m.locks[id]
	if !ok || l.Owner != owner {
		return nil, ErrInvalidLock
	}

	res := l.Resource

	if opts.Cancel {
		switch l.State {
		case Granted:
			return nil, ErrCancelGranted
		case Converting:
			res.converting = remove(res.converting, l)
			res.granted = append(res.granted, l)
			l.State, l.Requested = Granted, l.Mode
			events := append([]Event{{EventCanceled, l}}, m.regrant(res)...)

			return append(events, m.blockingNotices(res)...), nil
		}
		// A waiting request is aborted, as without Cancel.
	}

	if l.sublocks > 0 {
		return nil, ErrSublocks
	}

	if l.State != Waiting && l.Mode.writes() {
		switch {
		case opts.Invalidate:
			res.valueInvalid = true
		case opts.Value != nil:
			res.Value = *opts.Value
			res.valueInvalid = false
		}
	}

	return m.remove(l), nil
}

// DequeueAll is $DEQ with LCK$M_DEQALL: with parent 0, it dequeues every
// lock owner has at access mode accessMode or a less privileged one;
// with a parent lock, every sublock of it (and theirs), but not the
// parent. Sublocks go before their parents.
func (m *Manager) DequeueAll(owner Owner, accessMode uint8, parent ID) ([]Event, error) {
	var p *Lock

	if parent != 0 {
		var ok bool
		if p, ok = m.locks[parent]; !ok || p.Owner != owner {
			return nil, ErrInvalidLock
		}
	}

	return m.removeWhere(func(l *Lock) bool {
		if p != nil {
			return l != p && descends(l, p)
		}

		return l.Owner == owner && l.AccessMode >= accessMode
	}), nil
}

// Release dequeues every lock owner has: its rundown. If abnormal, each
// PW or EX lock's resource has its value block marked invalid (the
// manual's "terminated abnormally").
func (m *Manager) Release(owner Owner, abnormal bool) []Event {
	if abnormal {
		for _, l := range m.locks {
			if l.Owner == owner && l.State != Waiting && l.Mode.writes() {
				l.Resource.valueInvalid = true
			}
		}
	}

	return m.removeWhere(func(l *Lock) bool { return l.Owner == owner })
}

// removeWhere removes every lock match accepts, deepest sublocks first,
// returning the events.
func (m *Manager) removeWhere(match func(*Lock) bool) []Event {
	var doomed []*Lock

	for _, l := range m.locks {
		if match(l) {
			doomed = append(doomed, l)
		}
	}

	// Queued requests (waiting and converting locks) first, so that
	// removing a granted lock doesn't grant one about to go; then
	// deepest first; then by ID, so the order (and the events) don't
	// depend on Go's map order. (A converting lock taken before its
	// sublocks is harmless: remove doesn't look at them.)
	slices.SortFunc(doomed, func(a, b *Lock) int {
		if wa, wb := a.State != Granted, b.State != Granted; wa != wb {
			if wa {
				return -1
			}

			return 1
		}

		if da, db := depth(a), depth(b); da != db {
			return db - da
		}

		return int(a.ID) - int(b.ID)
	})

	var events []Event

	for _, l := range doomed {
		events = append(events, m.remove(l)...)
	}

	return events
}

// depth is how many parents l has.
func depth(l *Lock) int {
	n := 0
	for p := l.Parent; p != nil; p = p.Parent {
		n++
	}

	return n
}

// descends reports whether ancestor is one of l's parents.
func descends(l, ancestor *Lock) bool {
	for p := l.Parent; p != nil; p = p.Parent {
		if p == ancestor {
			return true
		}
	}

	return false
}

// remove takes l off its resource and out of the database, deletes the
// resource if nothing names it now, and grants what that lets through.
func (m *Manager) remove(l *Lock) []Event {
	res := l.Resource

	var events []Event

	switch l.State {
	case Granted:
		res.granted = remove(res.granted, l)
	case Converting:
		res.converting = remove(res.converting, l)
		events = append(events, Event{EventAborted, l})
	case Waiting:
		res.waiting = remove(res.waiting, l)
		events = append(events, Event{EventAborted, l})
	}

	delete(m.locks, l.ID)

	if l.Parent != nil {
		l.Parent.sublocks--
	}

	if m.deleteIfUnused(res) {
		return events
	}

	events = append(events, m.regrant(res)...)

	return append(events, m.blockingNotices(res)...)
}

// deleteIfUnused deletes res if no lock or sub-resource names it (and
// then its parent, if that's now unused), reporting whether it did.
func (m *Manager) deleteIfUnused(res *Resource) bool {
	if len(res.granted)+len(res.converting)+len(res.waiting) > 0 || res.children > 0 {
		return false
	}

	delete(m.resources, resourceKey{parent: res.Parent, group: res.Group, mode: res.AccessMode, name: res.Name})

	if res.Parent != nil {
		res.Parent.children--
		m.deleteIfUnused(res.Parent)
	}

	return true
}

// regrant grants queued requests that can be now: the conversion queue
// from its head while its head is compatible with every other lock; and
// then, if it's empty, the waiting queue the same way.
func (m *Manager) regrant(res *Resource) []Event {
	var events []Event

	for len(res.converting) > 0 {
		l := res.converting[0]
		if !compatibleWithAll(res, l, l.Requested) {
			return events
		}

		res.converting = res.converting[1:]
		res.granted = append(res.granted, l)
		m.grant(l, l.Requested > l.Mode)
		events = append(events, Event{EventGranted, l})
	}

	for len(res.waiting) > 0 {
		l := res.waiting[0]
		if !compatibleWithAll(res, l, l.Requested) {
			return events
		}

		res.waiting = res.waiting[1:]
		res.granted = append(res.granted, l)
		m.grant(l, true)
		events = append(events, Event{EventGranted, l})
	}

	return events
}

// grant sets l granted at its requested mode (its queue is the caller's
// to change); up is whether this is a new lock or a conversion to a
// higher mode, which copies the resource's value block if asked for.
func (m *Manager) grant(l *Lock, up bool) {
	l.Mode, l.State = l.Requested, Granted
	l.notified = false
	l.ValueNotValid = false

	if l.ValueBlock && up {
		l.Value = l.Resource.Value
		l.ValueNotValid = l.Resource.valueInvalid
	}
}

// blockingNotices returns a blocking event for each granted lock on res
// that wants one, hasn't had one since it was granted, and is
// incompatible with a queued request.
func (m *Manager) blockingNotices(res *Resource) []Event {
	queued := slices.Concat(res.converting, res.waiting)
	if len(queued) == 0 {
		return nil
	}

	var events []Event

	for _, h := range slices.Concat(res.granted, res.converting) {
		if !h.Blocking || h.notified {
			continue
		}

		for _, q := range queued {
			if q != h && !q.Requested.Compatible(h.Mode) {
				h.notified = true
				events = append(events, Event{EventBlocking, h})

				break
			}
		}
	}

	return events
}

// compatibleWithAll reports whether mode is compatible with the granted
// mode of every lock on res but l: the granted locks and the converting
// ones, which keep their granted modes until converted.
func compatibleWithAll(res *Resource, l *Lock, mode Mode) bool {
	for _, q := range [][]*Lock{res.granted, res.converting} {
		for _, o := range q {
			if o != l && !mode.Compatible(o.Mode) {
				return false
			}
		}
	}

	return true
}

// remove returns q without l.
func remove(q []*Lock, l *Lock) []*Lock {
	return slices.DeleteFunc(q, func(o *Lock) bool { return o == l })
}

// Notifier is implemented by a lock's Data when its owner is to be told
// of the lock's events: govax's $ENQ completes its request or delivers a
// blocking AST, and RMS's own locks wake a waiting stream.
type Notifier interface {
	Notify(Event)
}

// Deliver tells each event's lock owner of it, if the lock's Data is a
// Notifier. Every caller of an operation passes its events here, in the
// order returned.
func Deliver(events []Event) {
	for _, e := range events {
		if n, ok := e.Lock.Data.(Notifier); ok {
			n.Notify(e)
		}
	}
}
