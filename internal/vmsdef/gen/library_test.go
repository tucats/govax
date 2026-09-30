package main

import (
	"reflect"
	"testing"

	"github.com/tucats/govax/internal/obj"
)

// definitionModule returns a module like STARLET's SYS$SSDEF: an empty
// absolute psect, absolute symbol definitions, and a text record that only
// sets the relocation base. change alters it before it's returned.
func definitionModule(change func(*obj.Module)) *obj.Module {
	m := &obj.Module{Records: []obj.Record{
		&obj.MainHeader{Name: "SYS$SSDEF"},
		&obj.GSD{Subrecords: []obj.Subrecord{
			&obj.Psect{Name: ". ABS ."},
			&obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF, Name: "SS$_NORMAL", Value: 1},
			&obj.Symbol{Type: obj.GSDSymbol, Flags: obj.SymDEF, Name: "SS$_ACCVIO", Value: 12},
		}},
		&obj.TIR{Type: obj.RecTIR, Commands: []obj.Command{{Op: obj.OpStackPsectLong}, {Op: obj.OpSetRelocBase}}},
		&obj.EOM{},
	}}

	if change != nil {
		change(m)
	}

	return m
}

func TestDefinitions(t *testing.T) {
	defs, ok := definitions(definitionModule(nil))
	if want := map[string]uint32{"SS$_NORMAL": 1, "SS$_ACCVIO": 12}; !ok || !reflect.DeepEqual(defs, want) {
		t.Errorf("definitions = %v, %v; want %v", defs, ok, want)
	}

	gsd := func(m *obj.Module) *obj.GSD { return m.Records[1].(*obj.GSD) }

	for name, change := range map[string]func(*obj.Module){
		"a psect with contents": func(m *obj.Module) { gsd(m).Subrecords[0].(*obj.Psect).Alloc = 4 },
		"a relocatable symbol": func(m *obj.Module) {
			gsd(m).Subrecords[1].(*obj.Symbol).Flags |= obj.SymREL
		},
		"a reference": func(m *obj.Module) {
			gsd(m).Subrecords = append(gsd(m).Subrecords, &obj.Symbol{Type: obj.GSDSymbol, Name: "SYS$EXIT"})
		},
		"a text record that stores": func(m *obj.Module) {
			m.Records[2].(*obj.TIR).Commands = append(m.Records[2].(*obj.TIR).Commands, obj.Command{Op: obj.OpStoreLong})
		},
		"a traceback record": func(m *obj.Module) { m.Records[2].(*obj.TIR).Type = obj.RecTBT },
	} {
		if defs, ok := definitions(definitionModule(change)); ok {
			t.Errorf("a module with %s is a definition module: %v", name, defs)
		}
	}
}
