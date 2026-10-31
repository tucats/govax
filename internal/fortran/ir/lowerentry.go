package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// ENTRY statements (section 6.2.4 of the language manual). A subprogram with
// ENTRY statements has one body and several ways in, and each way in has its
// own argument list: X may be argument 1 when the routine is called by one
// name, argument 3 by another, and missing by a third. So the body can't reach
// X through the argument list, as a routine without ENTRY statements does
// (@4(AP)). Instead each dummy argument has a cell in static storage ($A1,
// $A2, ...) holding its address, each entry point begins by copying the
// addresses its list gives to those cells, and the body reaches X through its
// cell (@SUB.$A1). An entry point whose list doesn't name X leaves its cell
// alone, and section 6.2.4 says X is then undefined.
//
// FORTRAN can do it this way because its routines have static storage and
// aren't recursive (section 6.2): only one call of a routine is ever active,
// so one set of cells is enough.
//
// A function's names (its own, and its entry points') share one place for the
// value, so that defining one defines those of its type (section 6.2.4.1). But
// a return must leave the value as the name the caller used would: REAL in R0
// for one entry point, REAL*8 in R0 and R1 for another. If their machine types
// differ, each entry point leaves its number in a cell ($E), and a return is a
// Switch on it, to a block at the routine's end that returns the value in the
// entry point's type.

// returnBlock is a block that returns a function's value in one type, which
// the entry points of that type share.
type returnBlock struct {
	block *Block
	sym   *sema.Symbol
}

// prepareEntries makes the routine's argument cells, its entry points, and if
// they need it the cell and blocks that return a value by entry point. hidden
// is 1 for a function with a hidden first argument.
func (l *lowerer) prepareEntries(hidden int) {
	u, r := l.unit, l.r

	for i, p := range r.Params {
		p.Through = l.newCell("$A"+strconv.Itoa(i+1), ArgCell, Long, Long.Size())
		p.Through.Dummy = p.Name
	}

	first := &Entry{Name: u.Name, Number: 1, Args: l.entryArgs(u.Params, hidden)}
	r.Entries = []*Entry{first}
	l.entries = map[*sema.Entry]*Entry{}

	for i, e := range u.Entries {
		// A hidden argument's place is every name's: they have one value.
		if hidden != 0 {
			l.params[e.Sym] = l.params[u.Result]
		}

		entry := &Entry{Name: e.Sym.Name, Number: i + 2, Args: l.entryArgs(e.Params, hidden), Source: e.AST}
		r.Entries = append(r.Entries, entry)
		l.entries[e] = entry
	}

	if r.Result != None {
		l.prepareReturns()
	}
}

// entryArgs are an entry point's argument list's longwords: the hidden argument
// first if there is one, then the dummy arguments.
func (l *lowerer) entryArgs(params []*sema.Symbol, hidden int) []EntryArg {
	args := make([]EntryArg, 0, len(params)+hidden)

	if hidden != 0 {
		args = append(args, EntryArg{1, l.params[l.unit.Result]})
	}

	for i, s := range params {
		args = append(args, EntryArg{i + 1 + hidden, l.params[s]})
	}

	return args
}

// prepareReturns makes a function's selector cell and return blocks, if its
// names' values aren't all of one machine type.
func (l *lowerer) prepareReturns() {
	byType := map[Type]*returnBlock{}

	add := func(s *sema.Symbol) {
		if t := l.typeOf(s.Type); byType[t] == nil {
			byType[t] = &returnBlock{l.newBlock(), s}
			l.returns = append(l.returns, byType[t])
		}
	}

	add(l.unit.Result)

	for _, e := range l.unit.Entries {
		add(e.Sym)
	}

	if len(l.returns) > 1 {
		l.r.Selector = l.newCell("$E", SelectorCell, Long, Long.Size())

		return
	}

	// One type: every return returns it, as without ENTRY statements.
	l.made--
	l.returns = nil
}

// returnFromRoutine ends the current block by returning, by way of the
// entry point's own return block if the entry points' types differ.
func (l *lowerer) returnFromRoutine() {
	if l.r.Selector == nil {
		l.terminate(&Return{origin: l.origin(), Value: l.returnValue()})

		return
	}

	targets := make([]*Block, 0, len(l.r.Entries))
	targets = append(targets, l.returnBlockFor(l.unit.Result))

	for _, e := range l.unit.Entries {
		targets = append(targets, l.returnBlockFor(e.Sym))
	}

	selector := &Load{&Mem{Cell: l.r.Selector, T: Long}}
	l.terminate(&Switch{l.origin(), selector, targets, targets[0]})
}

// returnBlockFor is the block that returns the value of the name s.
func (l *lowerer) returnBlockFor(s *sema.Symbol) *Block {
	t := l.typeOf(s.Type)

	for _, b := range l.returns {
		if l.typeOf(b.sym.Type) == t {
			return b.block
		}
	}

	panic("ir: a function's name without a return block")
}

// layOutReturns lays out the return blocks, after the routine's last
// statement, which they are the last part of.
func (l *lowerer) layOutReturns() {
	for _, b := range l.returns {
		b.block.Note = "returns the value as " + l.typeOf(b.sym.Type).String()
		l.begin(b.block)
		l.terminate(&Return{origin: l.origin(), Value: &Load{l.symbolMem(b.sym)}})
	}
}

// entryPoint lowers an ENTRY statement: a block of its own for its entry point
// to go to, which control from the statements above falls into.
func (l *lowerer) entryPoint(s *sema.EntryPoint) {
	b := l.newBlock()
	b.Note = "ENTRY " + s.Entry.Sym.Name

	l.begin(b)
	l.entries[s.Entry].First = b
}
