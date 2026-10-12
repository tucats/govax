package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/vmserrors"
)

// DEPOSIT/INSTRUCTION (docs/PHASE-42.md, subtask 19): assemble one machine
// instruction and store it at a location, as the VMS debugger's
//
//	DEPOSIT/INSTRUCTION SUB2+2 = 'MOVL #20A,R0'
//
// does. The instruction is assembled by govax's assembler (internal/asm),
// at the location's address, so that a branch's or a PC-relative operand's
// displacement is right for where the instruction will be.
//
// The debugger and the assembler read an operand differently, so the text
// is rewritten before the assembler sees it (instructionText):
//
//   - A number without a radix prefix is in the debugger's input radix
//     (hexadecimal unless SET RADIX says otherwise), where the assembler
//     would read it in decimal, as MACRO does. Each one is rewritten in
//     hexadecimal with MACRO's ^X prefix: #20A becomes #^X20A.
//   - A name is the program's (a label, a routine, a path name such as
//     DBGCMD\LOOP, %LINE n), which a fresh assembler doesn't know. Each one
//     is replaced with its address, as the debugger evaluates it.
//
// Register names, the opcode, and the addressing-mode letters before a
// "^" (B^, W^, L^, S^, I^, G^) are left as they are, and so is a floating
// literal (#1.5), which the assembler reads in decimal as the debugger
// does.

// depositInstruction assembles the instruction text (in quotes or
// apostrophes) for the address addr and returns its bytes.
func (d *Debugger) depositInstruction(addr uint32, value string) ([]byte, error) {
	if !quoted(value) {
		return nil, vmserrors.New(vmserrors.DBG_NEEDQUOTE, strings.ToUpper(strings.TrimSpace(value)))
	}

	text, err := d.instructionText(unquote(value))
	if err != nil {
		return nil, err
	}

	a := asm.New(false)
	a.SetOrigin(addr)
	a.BeginInteractive()

	if _, err := a.AssembleLine(text); err != nil {
		return nil, vmserrors.Wrap(vmserrors.DBG_ASMERROR, err, strings.ToUpper(unquote(value)))
	}

	code := a.BytesRange(addr, a.Deposit())
	if len(code) == 0 {
		return nil, vmserrors.New(vmserrors.DBG_ASMERROR, strings.ToUpper(unquote(value)))
	}

	return code, nil
}

// instructionRegisters are the register names an operand may use, which
// stay as they are.
var instructionRegisters = map[string]bool{
	"R0": true, "R1": true, "R2": true, "R3": true, "R4": true, "R5": true,
	"R6": true, "R7": true, "R8": true, "R9": true, "R10": true, "R11": true,
	"R12": true, "R13": true, "R14": true, "R15": true,
	"AP": true, "FP": true, "SP": true, "PC": true,
}

// instructionText rewrites an instruction's operands for the assembler:
// each number in the debugger's input radix, and each name the program
// defines, becomes ^X and its value in hexadecimal. The opcode (the first
// word) is left alone.
func (d *Debugger) instructionText(text string) (string, error) {
	text = strings.TrimSpace(text)

	opcode, operands := text, ""
	if i := strings.IndexAny(text, " \t"); i >= 0 {
		opcode, operands = text[:i], text[i+1:]
	}

	var out strings.Builder

	out.WriteString(opcode)
	out.WriteByte(' ')

	s := operands

	for i := 0; i < len(s); {
		ch := s[i]

		switch {
		// A radix prefix and the number after it stay together.
		case ch == '^' && i+1 < len(s) && strings.ContainsRune("XxDdOoBb", rune(s[i+1])):
			j := i + 2
			for j < len(s) && isAlnum(s[j]) {
				j++
			}

			out.WriteString(s[i:j])
			i = j

		// %LINE n, a name the debugger knows.
		case ch == '%' || isNameStart(ch):
			j := i + 1
			for j < len(s) && (isAlnum(s[j]) || s[j] == '\\' || s[j] == '%' ||
				(s[j] == ' ' && strings.HasSuffix(strings.ToUpper(s[i:j]), "%LINE"))) {
				j++
			}

			word := s[i:j]
			upper := strings.ToUpper(word)

			switch {
			// An addressing-mode letter before "^" (B^4(R1)).
			case j < len(s) && s[j] == '^' && len(word) == 1:
				out.WriteString(word)
			case instructionRegisters[upper]:
				out.WriteString(word)
			case strings.HasPrefix(upper, "%") && instructionRegisters[upper[1:]]:
				out.WriteString(word[1:])
			default:
				v, err := d.evalWhole(word)
				if err != nil {
					return "", err
				}

				fmt.Fprintf(&out, "^X%X", v)
			}

			i = j

		// A number: in the input radix, unless it is a floating literal.
		case isDigit(ch):
			j := i
			for j < len(s) && (isAlnum(s[j]) || s[j] == '.') {
				j++
			}

			word := s[i:j]

			if strings.Contains(word, ".") {
				out.WriteString(word)
			} else {
				v, err := d.evalWhole(word)
				if err != nil {
					return "", err
				}

				fmt.Fprintf(&out, "^X%X", v)
			}

			i = j

		default:
			out.WriteByte(ch)
			i++
		}
	}

	return out.String(), nil
}

// isDigit, isNameStart, and isAlnum classify an operand's characters.
func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isNameStart(ch byte) bool {
	return ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '_' || ch == '$'
}

func isAlnum(ch byte) bool { return isNameStart(ch) || isDigit(ch) }
