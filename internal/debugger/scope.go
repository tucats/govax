package debugger

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/dbgsym"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
)

// SET SCOPE, CANCEL SCOPE, SHOW SCOPE, and the scope of names and
// registers (Phase 89, subtask 10; the VMS 5.5 Debugger Manual's SET
// SCOPE, and VMS 7.1's answers in testdata/dbg89).
//
// The scope is where the debugger looks a name up, and whose registers
// EXAMINE R2 means. It is a list of places, searched in order:
//
//   - a number n, the call level n out from the routine the program is
//     in (0 that routine, 1 its caller, ...): the levels SHOW CALLS lists;
//   - a path name, a routine (DBG89\REC) or, with /MODULE or when no
//     routine has the name, a module (DBG89B);
//   - "\", the global scope.
//
// With no SET SCOPE the list is the default one, every level from 0 out,
// and one of them is current: level 0, or the one SET SCOPE/CURRENT n
// chose, until the program next runs. SHOW SCOPE marks it with "*". A
// SET SCOPE list stays until CANCEL SCOPE or another SET SCOPE, across GO
// and STEP, its numbers counting the levels as they are then.
//
// The registers are the first place's: a level's are the ones it saw
// (machine.RegistersAt); a routine's are those of its innermost
// activation, even with an invocation number (SET SCOPE REC 1, as VMS
// 7.1's does); a module and a level past the last have none
// (%DEBUG-W-SYMNOTACT). A bare name is looked for in each place's module
// in turn, and then as it would be anyway. A name may also carry its
// own scope: 1\R2, REC 1\%R2 (the routine's second activation, 0 being
// the innermost), DBG89\REC 2\%R2, or 0\Y.

// scopeKind is what a place in the scope list is.
type scopeKind int

const (
	scopeLevel   scopeKind = iota // a call level, by number
	scopeRoutine                  // a routine, by path name
	scopeModule                   // a module
	scopeGlobal                   // "\"
)

// scopeEntry is one place in the scope list.
type scopeEntry struct {
	kind    scopeKind
	level   int    // scopeLevel's number
	module  string // scopeRoutine's and scopeModule's module
	routine string // scopeRoutine's routine

	// invocation is a routine's activation, 0 the innermost; exact says
	// it counts, as in REC 1\%R2 (in SET SCOPE REC 1 it doesn't).
	invocation int
	exact      bool

	// text is how SHOW SCOPE and the messages write the place.
	text string
}

// scopeState is the session's scope: list is SET SCOPE's, nil for the
// default list, whose current level is current.
type scopeState struct {
	list    []scopeEntry
	current int
}

// isDefault reports whether the default list is in effect.
func (s scopeState) isDefault() bool { return s.list == nil }

// callLevels are the program's call levels, innermost first, or none
// when it isn't stopped in code the debug symbols know (or has ended).
func (d *Debugger) callLevels() []machine.CallLevel {
	if d.imageExited {
		return nil
	}

	levels, ok, _ := d.Machine.CallLevels()
	if !ok {
		return nil
	}

	return levels
}

// levelNames are the levels' names as SHOW SCOPE and the register paths
// write them: a routine already seen at a level inside gets the number
// of its activations inside, "DBG89\REC 1".
func levelNames(levels []machine.CallLevel) []string {
	names := make([]string, len(levels))
	seen := map[string]int{}

	for i, l := range levels {
		path := l.Path()
		key := strings.ToUpper(path)

		names[i] = path
		if n := seen[key]; n > 0 {
			names[i] = fmt.Sprintf("%s %d", path, n)
		}

		seen[key]++
	}

	return names
}

// entryLevel is the level a place's registers come from; ok is false
// when it has none (a module, a routine not active, a level past the
// last).
func entryLevel(e scopeEntry, levels []machine.CallLevel) (int, bool) {
	switch e.kind {
	case scopeGlobal:
		return 0, true

	case scopeLevel:
		if e.level == 0 || e.level < len(levels) {
			return e.level, true
		}

	case scopeRoutine:
		want := 0
		if e.exact {
			want = e.invocation
		}

		for i, l := range levels {
			if strings.EqualFold(l.Module, e.module) && l.HasRoutine && strings.EqualFold(l.Routine, e.routine) {
				if want == 0 {
					return i, true
				}

				want--
			}
		}
	}

	return 0, false
}

// registerPlace is whose registers EXAMINE and DEPOSIT see now: a level,
// or, when ok is false, a place that has none, named by missing.
func (d *Debugger) registerPlace(levels []machine.CallLevel) (level int, missing string, ok bool) {
	if d.scope.isDefault() {
		if d.scope.current < len(levels) {
			return d.scope.current, "", true
		}

		return 0, "", true
	}

	first := d.scope.list[0]

	if level, ok := entryLevel(first, levels); ok {
		return level, "", true
	}

	return 0, first.text, false
}

// regView is the registers of one call level, as EXAMINE and DEPOSIT see
// them: scope is the path their names are written under ("" for none),
// and missing names a place that has no registers.
type regView struct {
	level   int
	regs    machine.LevelRegisters
	scope   string
	missing string
}

// registerView is the view of the registers the scope gives now.
func (d *Debugger) registerView() (regView, error) {
	levels := d.callLevels()

	level, missing, ok := d.registerPlace(levels)
	if !ok {
		return regView{missing: missing}, nil
	}

	regs, err := d.Machine.RegistersAt(levels, level)
	if err != nil {
		return regView{}, err
	}

	v := regView{level: level, regs: regs, scope: d.registerScope()}
	if level > 0 {
		v.scope = levelNames(levels)[level]
	}

	return v, nil
}

// label is how a register of the view is named: DBG89\REC 1\%R2.
func (v regView) label(name string) string {
	if v.missing != "" {
		return v.missing + `\%` + name
	}

	if v.scope != "" {
		return v.scope + `\%` + name
	}

	return "%" + name
}

// get is a general register's contents, or %DEBUG-W-SYMNOTACT for one the
// view hasn't.
func (v regView) get(reg registers.Reg, name string) (uint32, error) {
	if v.missing != "" || !v.regs.Active[reg] {
		return 0, vmserrors.New(vmserrors.DBG_SYMNOTACT, v.label(name))
	}

	return v.regs.Value[reg], nil
}

// psl is the view's PSL.
func (v regView) psl() (uint32, error) {
	if v.missing != "" {
		return 0, vmserrors.New(vmserrors.DBG_SYMNOTACT, v.label("PSL"))
	}

	return v.regs.PSL, nil
}

// put stores a register of the view: a live one in the CPU, one a frame
// saved in the frame, so the RET gives it back to the routine. SP and the
// PSL of an outer level are what a RET computes and no frame holds, so
// they can't be changed (govax's choice: the probe's DEPOSITs were
// written with ^X, which VMS's debugger doesn't read).
func (d *Debugger) put(v regView, reg registers.Reg, name string, value uint32) error {
	if _, err := v.get(reg, name); err != nil {
		return err
	}

	if v.level == 0 {
		d.Machine.CPU.SetGPR(reg, value)

		return nil
	}

	if slot := v.regs.Slot[reg]; slot != 0 {
		return d.Machine.WriteBytes(slot, []byte{byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)})
	}

	if reg == registers.SP {
		return vmserrors.New(vmserrors.DBG_OPTNOTALLOW, "DEPOSIT")
	}

	d.Machine.CPU.SetGPR(reg, value)

	return nil
}

// scopeRegister is the evaluator's Register hook: a register of the
// scope's level, when that isn't the program's own level 0.
func (d *Debugger) scopeRegister(name string) (uint32, bool, error) {
	if d.scope.isDefault() && d.scope.current == 0 {
		return 0, false, nil
	}

	v, err := d.registerView()
	if err != nil {
		return 0, true, err
	}

	if name == "PSL" {
		value, err := v.psl()

		return value, true, err
	}

	reg, ok := registerLocations[name]
	if !ok {
		return 0, false, nil
	}

	value, err := v.get(reg, name)

	return value, true, err
}

// scopeModules are the modules, in the scope list's order, that a bare
// name is looked for in before the usual search: with the default list,
// only when SET SCOPE/CURRENT chose an outer level.
func (d *Debugger) scopeModules() []string {
	levels := d.callLevels()

	if d.scope.isDefault() {
		if d.scope.current == 0 || d.scope.current >= len(levels) {
			return nil
		}

		return []string{levels[d.scope.current].Module}
	}

	var modules []string

	for _, e := range d.scope.list {
		switch e.kind {
		case scopeRoutine, scopeModule:
			modules = append(modules, e.module)

		case scopeLevel:
			if e.level < len(levels) {
				modules = append(modules, levels[e.level].Module)
			}
		}
	}

	return modules
}

// scopedName is the evaluator's Scoped hook: the path a bare name is
// looked up as, its scope's module's own name if one has it.
func (d *Debugger) scopedName(name string) (string, bool) {
	for _, module := range d.scopeModules() {
		m := d.moduleNamed(module)
		if m == nil {
			continue
		}

		if s, ok := m.Symbols.Get(name); ok && strings.EqualFold(s.Scope, m.Name) {
			return m.Name + `\` + s.Name, true
		}
	}

	return "", false
}

// withScope runs fn with the scope list just e, as a name's own scope
// (1\R2, REC 1\%R2) has it.
func (d *Debugger) withScope(e scopeEntry, fn func() error) error {
	saved := d.scope
	d.scope = scopeState{list: []scopeEntry{e}}

	defer func() { d.scope = saved }()

	return fn()
}

// splitScoped splits a name with a scope of its own, "1\R2", "REC 1\%R2",
// "DBG89\REC 2\%R2", into the place and the name after it. ok is false
// for any other text, a path name such as DBG89\Y among them: only a
// number, an invocation number, or a register after a path makes one.
func (d *Debugger) splitScoped(item string) (e scopeEntry, name string, ok bool, err error) {
	item = strings.TrimSpace(item)

	i := strings.LastIndexByte(item, '\\')
	if i <= 0 || i == len(item)-1 {
		return scopeEntry{}, "", false, nil
	}

	head, name := strings.TrimSpace(item[:i]), item[i+1:]
	_, _, _, isRegister := registerOf(name)

	if n, err := strconv.Atoi(head); err == nil && n >= 0 {
		return scopeEntry{kind: scopeLevel, level: n, text: head}, name, true, nil
	}

	path, invocation, numbered := cutInvocation(head)
	if !numbered && !isRegister {
		return scopeEntry{}, "", false, nil
	}

	if numbered && !isRegister {
		return scopeEntry{}, "", false, vmserrors.New(vmserrors.DBG_MISINVNUM, item)
	}

	e, found := d.pathScope(path, false)
	if !found {
		return scopeEntry{}, "", false, vmserrors.New(vmserrors.DBG_NOSYMBOL, item)
	}

	if e.kind == scopeRoutine {
		e.invocation, e.exact = invocation, true
		if invocation > 0 {
			e.text += fmt.Sprintf(" %d", invocation)
		}
	}

	return e, name, true, nil
}

// cutInvocation splits "REC 1" into the path and its invocation number.
func cutInvocation(text string) (path string, n int, ok bool) {
	i := strings.LastIndexAny(text, " \t")
	if i < 0 {
		return text, 0, false
	}

	n, err := strconv.Atoi(text[i+1:])
	if err != nil || n < 0 {
		return text, 0, false
	}

	return strings.TrimSpace(text[:i]), n, true
}

// pathScope finds the routine or module a path names: MODULE\ROUTINE, a
// routine of any module, or (asModule, or when no routine has the name) a
// module.
func (d *Debugger) pathScope(path string, asModule bool) (scopeEntry, bool) {
	parts := strings.Split(path, `\`)

	if asModule {
		if m := d.moduleNamed(path); m != nil && len(parts) == 1 {
			return scopeEntry{kind: scopeModule, module: m.Name, text: m.Name}, true
		}

		return scopeEntry{}, false
	}

	routineEntry := func(m *dbgsym.Module, name string) (scopeEntry, bool) {
		for _, r := range m.Routines {
			if strings.EqualFold(r.Name, name) {
				l := machine.CallLevel{Module: m.Name, Routine: r.Name, HasRoutine: true}

				return scopeEntry{kind: scopeRoutine, module: m.Name, routine: r.Name, text: l.Path()}, true
			}
		}

		return scopeEntry{}, false
	}

	switch len(parts) {
	case 1:
		for _, m := range d.allModules() {
			if e, ok := routineEntry(m, parts[0]); ok {
				return e, true
			}
		}

		if m := d.moduleNamed(parts[0]); m != nil {
			return scopeEntry{kind: scopeModule, module: m.Name, text: m.Name}, true
		}

	case 2:
		if m := d.moduleNamed(parts[0]); m != nil {
			return routineEntry(m, parts[1])
		}
	}

	return scopeEntry{}, false
}

// setScope runs SET SCOPE[/MODULE] location[,location...] and SET
// SCOPE/CURRENT n.
func (d *Debugger) setScope(text string, asModule, current bool) error {
	if err := d.Machine.RequireInit(); err != nil {
		return err
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return vmserrors.New(vmserrors.DBG_NEEDMORE)
	}

	items, err := splitList(text)
	if err != nil {
		return err
	}

	if current {
		return d.setCurrentScope(items)
	}

	list := make([]scopeEntry, 0, len(items))

	for _, item := range items {
		e, err := d.scopeItem(strings.TrimSpace(item), asModule)
		if err != nil {
			return err
		}

		list = append(list, e)
	}

	// A routine or module SET SCOPE names has its symbols set, as SET
	// MODULE would (VMS's SHOW MODULE says so after SET SCOPE DBG89B).
	for _, e := range list {
		if m := d.moduleNamed(e.module); m != nil && e.module != "" {
			if d.symbolsSet == nil {
				d.seedModules()
			}

			d.symbolsSet[m] = true
		}
	}

	d.scope = scopeState{list: list}

	return nil
}

// scopeItem reads one place of SET SCOPE's list.
func (d *Debugger) scopeItem(item string, asModule bool) (scopeEntry, error) {
	if item == `\` {
		return scopeEntry{kind: scopeGlobal, text: `\`}, nil
	}

	if !asModule && item != "" && (item[0] >= '0' && item[0] <= '9' || item[0] == '%' || item[0] == '(') {
		n, err := d.Machine.EvalWholeIn(10, item)
		if err != nil {
			return scopeEntry{}, err
		}

		return scopeEntry{kind: scopeLevel, level: int(n), text: strconv.Itoa(int(n))}, nil
	}

	path, invocation, numbered := cutInvocation(item)

	e, ok := d.pathScope(path, asModule)
	if !ok {
		return scopeEntry{}, vmserrors.New(vmserrors.DBG_BADSCOPE, item)
	}

	if numbered && e.kind == scopeRoutine {
		e.invocation = invocation
		e.text += fmt.Sprintf(" %d", invocation)
	}

	return e, nil
}

// setCurrentScope is SET SCOPE/CURRENT n: level n becomes the default
// list's current one, which only a number in the list's range can be.
func (d *Debugger) setCurrentScope(items []string) error {
	if !d.scope.isDefault() {
		return vmserrors.New(vmserrors.DBG_NODEFSCPE, "SET SCOPE/CURRENT")
	}

	if len(items) > 1 {
		return vmserrors.New(vmserrors.DBG_NOSCOPELIST)
	}

	item := strings.TrimSpace(items[0])
	if item == "" || !(item[0] >= '0' && item[0] <= '9' || item[0] == '%' || item[0] == '(') {
		return vmserrors.New(vmserrors.DBG_NOTNUMSCOPE)
	}

	n, err := d.Machine.EvalWholeIn(10, item)
	if err != nil {
		return err
	}

	if int(n) >= max(len(d.callLevels()), 1) {
		return vmserrors.New(vmserrors.DBG_NOTINSCOPE)
	}

	d.scope.current = int(n)

	return nil
}

// cancelScope runs CANCEL SCOPE: the default list again, level 0 current.
func (d *Debugger) cancelScope() error {
	d.scope = scopeState{}

	return nil
}

// resumeScope is what the program's going on does to the scope: SET
// SCOPE/CURRENT's level is forgotten, a SET SCOPE list kept.
func (d *Debugger) resumeScope() { d.scope.current = 0 }

// showScope runs SHOW SCOPE. The default list is every level, innermost
// first, the current one marked "*", each with what is there:
//
//	scope:
//	 *  0 [ = DBG89\LEAF ],
//	    1 [ = DBG89\REC ],
//	    2 [ = DBG89\REC 1 ],
//
// (a routine already seen at a level inside written with the number of
// its activations inside). With no levels, after the program's end, it
// is " *  0" alone. A SET SCOPE list is its places, a level that is there
// with what is there ("    1 [ = DBG89\REC ]"), a path name, a level past
// the last, or "\" alone, none marked.
func (d *Debugger) showScope() error {
	c := d.Machine

	if err := c.RequireInit(); err != nil {
		return err
	}

	levels := d.callLevels()
	names := levelNames(levels)

	var lines []string

	if d.scope.isDefault() {
		if len(levels) == 0 {
			lines = append(lines, " *  0")
		}

		for level, name := range names {
			mark := ' '
			if level == d.scope.current {
				mark = '*'
			}

			lines = append(lines, fmt.Sprintf(" %c %2d [ = %s ]", mark, level, name))
		}
	} else {
		for _, e := range d.scope.list {
			switch {
			case e.kind == scopeLevel && e.level < len(levels):
				lines = append(lines, fmt.Sprintf("   %2d [ = %s ]", e.level, names[e.level]))
			case e.kind == scopeLevel:
				lines = append(lines, fmt.Sprintf("   %2d", e.level))
			default:
				lines = append(lines, "    "+e.text)
			}
		}
	}

	c.Printf("scope: \n")

	// Every place but the last ends with a comma and a blank.
	for i, l := range lines {
		if i < len(lines)-1 {
			l += ", "
		}

		c.Printf("%s\n", l)
	}

	return nil
}

// scopeBuiltin is the value of a scope built-in symbol, a level of the
// default list: %CURRENT_SCOPE_ENTRY the current one, %NEXT_SCOPE_ENTRY
// the one out from it, %PREVIOUS_SCOPE_ENTRY the one in, each going no
// further than the list's ends (that %NEXT_SCOPE_ENTRY stops at the last
// level is unconfirmed). With a SET SCOPE list there is no default list
// (%DEBUG-E-NODEFSCPE).
func (d *Debugger) scopeBuiltin(name string) (uint32, bool, error) {
	switch name {
	case "%CURRENT_SCOPE_ENTRY", "%NEXT_SCOPE_ENTRY", "%PREVIOUS_SCOPE_ENTRY":
	default:
		return 0, false, nil
	}

	if !d.scope.isDefault() {
		return 0, true, vmserrors.New(vmserrors.DBG_NODEFSCPE, name)
	}

	cur := d.scope.current
	last := max(len(d.callLevels())-1, 0)

	switch name {
	case "%NEXT_SCOPE_ENTRY":
		cur = min(cur+1, last)
	case "%PREVIOUS_SCOPE_ENTRY":
		cur = max(cur-1, 0)
	}

	return uint32(cur), true, nil
}

// examineRegisterInstruction is EXAMINE/INSTRUCTION of a register, in the
// scope or with one of its own (REC 1\%PC): VMS 7.1's shows the register's
// contents, and the registers after it, decoded as an instruction
// (testdata/dbg89: the bytes of 0000046D are "CVTWD S^#04,S^#0.5000000").
// done is false for an item that isn't a register.
func (d *Debugger) examineRegisterInstruction(item string, opts machine.DisassembleOptions) (done bool, err error) {
	reg, name, isPSL, ok := registerOf(item)

	if !ok {
		e, rest, scoped, err := d.splitScoped(item)
		if err != nil {
			return true, err
		}

		if !scoped {
			return false, nil
		}

		if reg, name, isPSL, ok = registerOf(rest); !ok {
			return false, nil
		}

		return true, d.withScope(e, func() error { return d.registerInstruction(reg, name, isPSL, opts) })
	}

	return true, d.registerInstruction(reg, name, isPSL, opts)
}

// registerInstruction shows the instruction a register's bytes hold.
func (d *Debugger) registerInstruction(reg registers.Reg, name string, isPSL bool, opts machine.DisassembleOptions) error {
	view, err := d.registerView()
	if err != nil {
		return err
	}

	var data []byte

	add := func(v uint32) { data = append(data, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }

	if isPSL {
		v, err := view.psl()
		if err != nil {
			return err
		}

		add(v)
	} else {
		if _, err := view.get(reg, name); err != nil {
			return err
		}

		// The registers after it supply the bytes of a longer instruction,
		// as far as the PC (unconfirmed: the probe's needed three bytes).
		for at := reg; at <= registers.PC; at++ {
			v, err := view.get(at, registerName(at))
			if err != nil {
				break
			}

			add(v)
		}
	}

	text, err := d.Machine.InstructionInBytes(data, opts)
	if err != nil {
		return err
	}

	d.Machine.Printf("%s%s\n", tabPad(view.label(name)+":"), text)

	return nil
}
