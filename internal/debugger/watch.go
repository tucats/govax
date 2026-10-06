package debugger

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET WATCH, SHOW WATCH, and CANCEL WATCH as the VMS debugger has them
// (docs/PHASE-42.md, subtask 13; the formats are from
// testdata/dbgcmd/vax/watch.dlg).
//
// A *watchpoint* stops the program when what is stored at a location
// changes. The VMS debugger does it by looking at the location after each
// instruction (it has no hardware to ask), and so does this one: the
// location is read after every instruction the program runs, and compared
// with what it held before. The report names the instruction that changed
// it and shows the old and new values:
//
//	watch of DBGCMD\WATCHL at DBGCMD\START\%LINE 40
//	    40:         ADDL2   #2, WATCHL
//	   old value: 00000000
//	   new value: 00000002
//	break at DBGCMD\START\ODD
//	    41: ODD:    SOBGTR  R3, LOOP
//
// A watchpoint on an array (SET WATCH BUFFER) watches each element and
// reports each one that changed; an instruction that changes several (the
// VAX's MOVC3 copies a whole block) gives one report each, last element
// first, as the VMS debugger does.

// Watchpoint is one watched location.
type Watchpoint struct {
	// Addr is where the watched data starts. Elem is the size in bytes of
	// one item (a longword is 4, a byte is 1), and Count how many items
	// there are: 1 for a single datum, more for an array.
	Addr  uint32
	Elem  uint32
	Count uint32

	// Array is true for a watch of a whole array, whose name is shown with
	// its bounds (BUFFER[0:15]) and whose reports name the element
	// (BUFFER[3]); Lower is the index of its first element.
	Array bool
	Lower int32

	// Name is the data's path name (DBGCMD\WATCHL), without any bounds.
	Name string

	// Temporary removes the watchpoint once it has reported. After is
	// /AFTER:n, which skips the first n-1 changes. When and Do are the
	// WHEN and DO clauses, as for a breakpoint.
	Temporary bool
	After     int
	hits      int
	When, Do  string

	// old is what the data held when the program last looked: the
	// baseline the next instruction's result is compared with. nil when
	// the memory couldn't be read.
	old []byte
}

// size is the number of bytes the watchpoint covers.
func (w *Watchpoint) size() uint32 { return w.Elem * w.Count }

// bindWatch binds the SET WATCH, SHOW WATCH, and CANCEL WATCH commands.
func (d *Dispatcher) bindWatch() {
	d.Grammar.Bind("SET_WATCH", func(id int64, r *dcl.Result) error { return d.Debugger.setWatch(r) })
	d.Grammar.Bind("SHOW_WATCH", func(id int64, r *dcl.Result) error { return d.Debugger.showWatch() })
	d.Grammar.Bind("CANCEL_WATCH", func(id int64, r *dcl.Result) error { return d.Debugger.cancelWatch(r) })
}

// setWatch implements SET WATCH location[,location...] [WHEN (condition)]
// [DO (commands)]. A location that is a data symbol is watched by the
// symbol's type and extent: BUFFER, a 16-byte array, is watched as 16 bytes.
// Anything else (an address, a register-relative expression) is watched as a
// longword, as EXAMINE shows it.
func (d *Debugger) setWatch(r *dcl.Result) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	target, when, do, err := splitClauses(r.String("TARGET"))
	if err != nil {
		return err
	}

	if target == "" {
		return vmserrors.New(vmserrors.DBG_SYNTAX, "")
	}

	proto := Watchpoint{Temporary: r.Present("TEMPORARY"), When: when, Do: do}

	if r.Present("AFTER") {
		if proto.After = int(r.Int("AFTER")); proto.After < 1 {
			return vmserrors.New(vmserrors.DBG_SYNTAX, fmt.Sprint(r.Int("AFTER")))
		}
	}

	// Work out every location before watching any, so a bad one in the
	// list makes none.
	splits := splitTop(target, ',')
	made := make([]*Watchpoint, 0, len(splits))

	for _, text := range splits {
		addr, err := d.evalWhole(text)
		if err != nil {
			return err
		}

		wp := proto
		d.sizeWatch(&wp, addr)

		made = append(made, &wp)
	}

	for _, wp := range made {
		// Watching what is already watched replaces it, so the new
		// options take its place.
		d.Watchpoints = slices.DeleteFunc(d.Watchpoints, func(old *Watchpoint) bool { return old.Addr == wp.Addr })

		wp.snapshot(d)

		d.Watchpoints = append(d.Watchpoints, wp)
	}

	return nil
}

// sizeWatch fills in what the debug symbols say about the data at addr: its
// name, the size of an item, and, for an array, how many there are.
func (d *Debugger) sizeWatch(wp *Watchpoint, addr uint32) {
	wp.Addr, wp.Elem, wp.Count = addr, defaultSize, 1
	wp.Name = d.locationName(addr)

	prog := d.Console.DebugProgramAt(addr)
	if prog == nil {
		return
	}

	datum, _, _ := prog.DatumAt(addr)
	if datum == nil {
		// Inside an array: watch that element alone, by its type.
		if elem, ok := prog.ElementDatumAt(addr); ok {
			datum = elem
		}
	} else if datum.IsArray() && len(datum.Descriptor.Bounds) == 1 {
		bounds := datum.Descriptor.Bounds[0]
		wp.Array, wp.Lower = true, int32(bounds.Lower)
		wp.Count = uint32(bounds.Upper-bounds.Lower) + 1
		wp.Name = strings.SplitN(wp.Name, "[", 2)[0]
	}

	if datum == nil {
		return
	}

	// An item the debugger can't show as a number (text, an octaword) is
	// watched as longwords, which is how EXAMINE shows it, and a whole
	// array of such items isn't watched as an array.
	if size := datum.ElementSize(); size > 0 && size <= 8 {
		wp.Elem = size
	} else if wp.Array {
		wp.Array, wp.Count = false, 1
	}
}

// snapshot records what the watched memory holds now as the baseline.
func (w *Watchpoint) snapshot(d *Debugger) {
	if data, err := d.Console.ReadBytes(w.Addr, w.size()); err == nil {
		w.old = data
	} else {
		w.old = nil
	}
}

// snapshotWatches takes every watchpoint's baseline. It is done when a run
// starts, so that what the user changed while the program was stopped (a
// DEPOSIT) isn't reported as the program's doing.
func (d *Debugger) snapshotWatches() {
	for _, wp := range d.Watchpoints {
		wp.snapshot(d)
	}
}

// watchHit is called after the instruction that began at pc has run. It
// reports each watched item that instruction changed and, if there was
// one, reports whether the program should stop (it does: a watchpoint
// stops it after the instruction, "break at" the next).
func (d *Debugger) watchHit(pc uint32) bool {
	if len(d.Watchpoints) == 0 {
		return false
	}

	c := d.Console
	stopped := false

	// Work from a copy: a temporary watchpoint removes itself.
	for _, wp := range slices.Clone(d.Watchpoints) {
		now, err := c.ReadBytes(wp.Addr, wp.size())
		if err != nil {
			continue
		}

		before := wp.old
		wp.old = now

		if before == nil {
			continue
		}

		changed := wp.changedItems(before, now)
		if len(changed) == 0 {
			continue
		}

		if wp.After > 0 {
			if wp.hits++; wp.hits < wp.After {
				continue
			}
		}

		if wp.When != "" && !d.conditionHolds(wp.When) {
			continue
		}

		// Each item that changed is a report, and a stop of its own.
		// The source line of the instruction is shown again for each.
		for _, item := range changed {
			d.shown.ok = false

			d.reportWatch(wp, item, pc, before, now)
		}

		if wp.Temporary {
			d.Watchpoints = slices.DeleteFunc(d.Watchpoints, func(w *Watchpoint) bool { return w == wp })
		}

		d.pendingDo = wp.Do
		stopped = true
	}

	return stopped
}

// changedItems lists the indexes (0 for the first item) of the items whose
// bytes differ between before and now, highest first.
func (w *Watchpoint) changedItems(before, now []byte) []uint32 {
	var items []uint32

	for i := w.Count; i > 0; i-- {
		lo, hi := (i-1)*w.Elem, i*w.Elem

		if !slices.Equal(before[lo:hi], now[lo:hi]) {
			items = append(items, i-1)
		}
	}

	return items
}

// reportWatch shows the report for item of wp, changed by the instruction
// at pc (the program is now at the next one):
//
//	watch of DBGCMD\BUFFER[15] at DBGCMD\START\%LINE 43
//	   43:         MOVC3   #16, SOURCE, BUFFER
//	   old value: 00
//	   new value: 46
//	break at DBGCMD\START\%LINE 44
//	   44:         JSB     BUMP
func (d *Debugger) reportWatch(wp *Watchpoint, item, pc uint32, before, now []byte) {
	c := d.Console

	name := wp.Name
	if wp.Array {
		name = fmt.Sprintf("%s[%d]", wp.Name, wp.Lower+int32(item))
	}

	c.Printf("watch of %s at %s\n", name, c.LocationText(pc))
	d.showSource(pc)

	c.Printf("   old value: %s\n", d.itemValue(before, item, wp.Elem))
	c.Printf("   new value: %s\n", d.itemValue(now, item, wp.Elem))

	next := c.CPU.GPR(vax.PC)
	c.Printf("break at %s\n", c.LocationText(next))
	d.showSource(next)
}

// itemValue is item of data (items of size bytes each) as a number in the
// output radix, as wide as the item (a byte is two hexadecimal digits).
func (d *Debugger) itemValue(data []byte, item, size uint32) string {
	var v uint64

	for i := size; i > 0; i-- {
		v = v<<8 | uint64(data[item*size+i-1])
	}

	return formatRadix(v, size, d.outputRadix)
}

// showWatch implements SHOW WATCH: each watchpoint, or the message that
// none are set.
func (d *Debugger) showWatch() error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	if len(d.Watchpoints) == 0 {
		d.Console.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOWATCHES))

		return nil
	}

	for _, wp := range d.Watchpoints {
		text := "watchpoint of " + wp.Name

		if wp.Array {
			text += fmt.Sprintf("[%d:%d]", wp.Lower, wp.Lower+int32(wp.Count)-1)
		}

		if wp.Temporary {
			text += " [temporary]"
		}

		d.Console.Printf("%s\n", text)

		if wp.After > 0 {
			d.Console.Printf("   /after: %d\n", wp.After)
		}

		if wp.When != "" {
			d.Console.Printf("   when %s\n", wp.When)
		}

		if wp.Do != "" {
			d.Console.Printf("   do %s\n", wp.Do)
		}
	}

	return nil
}

// cancelWatch implements CANCEL WATCH location[,location...], or /ALL.
// Cancelling nothing that was set is %DEBUG-I-NOWATCHES (unconfirmed: the
// probe cancelled only what was set).
func (d *Debugger) cancelWatch(r *dcl.Result) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	before := len(d.Watchpoints)

	if r.Present("ALL") {
		d.Watchpoints = nil
	} else {
		target := strings.TrimSpace(r.String("TARGET"))
		if target == "" {
			return vmserrors.New(vmserrors.DBG_SYNTAX, "")
		}

		for _, text := range splitTop(target, ',') {
			addr, err := d.evalWhole(text)
			if err != nil {
				return err
			}

			d.Watchpoints = slices.DeleteFunc(d.Watchpoints, func(w *Watchpoint) bool { return w.Addr == addr })
		}
	}

	if len(d.Watchpoints) == before {
		d.Console.Printf("%%%s\n", vmserrors.New(vmserrors.DBG_NOWATCHES))
	}

	return nil
}
