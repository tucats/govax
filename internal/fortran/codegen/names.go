package codegen

import "strconv"

// maxSymbol is the longest symbol MACRO-32 takes: 31 characters.
const maxSymbol = 31

// namer makes the MACRO symbols of one module (Decision 10 of
// docs/PHASE-74 - FORTRAN plan.md). Everything a routine owns is named
// by the routine's name, a period, and its own name: SQUARES.I for the
// variable I, SQUARES.L10 for the statement labeled 10, SQUARES.$C1 for
// a DO loop's count. MACRO allows a period in a symbol and FORTRAN names
// can't hold one, so these never collide with a routine's entry point (a
// global symbol, its FORTRAN name) or with each other, and someone
// debugging the program can EXAMINE SQUARES.I.
//
// A name that would pass MACRO's 31 characters gets a numbered form,
// counted through the module so it is unique: as much of the routine's
// name as fits, then .$N and the number (A_VERY_LONG_ROUTINE_NAME.$N1).
// The storage map lists the numbered forms.
type namer struct {
	long int
}

// routineNames are one routine's symbols.
type routineNames struct {
	n      *namer
	prefix string

	// made are the symbols made so far, by name; long are the names
	// given numbered forms, in the order made, for the storage map.
	made map[string]string
	long []string
}

// routine starts naming the symbols of the routine named name.
func (n *namer) routine(name string) *routineNames {
	return &routineNames{n: n, prefix: name, made: map[string]string{}}
}

// symbol is the MACRO symbol for the routine's name: a variable, a
// dummy argument, a cell, a datum, or a label.
func (r *routineNames) symbol(name string) string {
	if s, ok := r.made[name]; ok {
		return s
	}

	s := r.prefix + "." + name
	if len(s) > maxSymbol {
		r.n.long++
		suffix := ".$N" + strconv.Itoa(r.n.long)
		s = r.prefix[:min(len(r.prefix), maxSymbol-len(suffix))] + suffix
		r.long = append(r.long, name)
	}

	r.made[name] = s

	return s
}
