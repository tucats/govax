package rms

import (
	"bufio"
	"errors"
	"io"
)

// ctrlZ is the character a terminal user types to end input: a $GET that
// reads it first returns RMS$_EOF. Typed after some text, it ends that
// text's record instead, and the end of file is the next read's (govax's
// front end delivers a second CTRL/Z for it; cmd/govax/attention.go).
const ctrlZ = 0x1A

// terminalRecord is SYS$GET's read of one record from the terminal
// (RMS Reference, $GET: "terminal input"). If the RAB asks for a prompt
// (RAB$V_PMT in RAB$L_ROP), the RAB$B_PSZ bytes at RAB$L_PBF are written
// first, as they are. A record then ends at a carriage return or a line
// feed (a host "\r\n" pair is one end), neither of which is part of it,
// or when capacity bytes have been read: the terminal driver ends a read
// when the buffer fills, and whatever was typed beyond it is the next
// record. A Ctrl/Z ends a record too, but with nothing read before it
// is end of file: status is RMS$_EOF, as it is at the end of the host's
// input with nothing read. status is 0 for a record.
func terminalRecord(ctx *Context, rabAddr uint32, capacity int) (record []byte, status uint32, err error) {
	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return nil, 0, err
	}

	prompt := ""

	if rop&ropPMT != 0 {
		if prompt, err = readPrompt(ctx, rabAddr); err != nil {
			return nil, 0, err
		}
	}

	if ctx.AwaitTerminal != nil {
		if err := ctx.AwaitTerminal(capacity, prompt); err != nil {
			return nil, 0, err
		}

		if ctx.TerminalDone != nil {
			defer ctx.TerminalDone()
		}
	} else if prompt != "" && ctx.Console != nil {
		_, _ = io.WriteString(ctx.Console, prompt)
	}

	r := ctx.ConsoleIn
	if r == nil {
		return nil, rmsEOF, nil
	}

	for len(record) < capacity {
		b, readErr := r.ReadByte()
		if readErr != nil {
			if errors.Is(readErr, io.EOF) && len(record) > 0 {
				break
			}

			return nil, rmsEOF, nil
		}

		switch b {
		case ctrlZ:
			if len(record) > 0 {
				return record, 0, nil
			}

			return nil, rmsEOF, nil

		case '\r':
			swallowLineFeed(r)

			return record, 0, nil

		case '\n':
			return record, 0, nil
		}

		record = append(record, b)
	}

	return record, 0, nil
}

// readPrompt reads a RAB's prompt (RAB$L_PBF, RAB$B_PSZ).
func readPrompt(ctx *Context, rabAddr uint32) (string, error) {
	pbf, err := ctx.loadLongword(rabAddr + rabPBF)
	if err != nil {
		return "", err
	}

	psz, err := ctx.loadByte(rabAddr + rabPSZ)
	if err != nil {
		return "", err
	}

	if psz == 0 {
		return "", nil
	}

	return ctx.loadFixedString(pbf, int(psz))
}

// swallowLineFeed reads the "\n" of a "\r\n" pair, but only if it has
// already arrived: waiting for it would hold up a record that's complete.
func swallowLineFeed(r *bufio.Reader) {
	if r.Buffered() == 0 {
		return
	}

	if next, err := r.Peek(1); err == nil && next[0] == '\n' {
		_, _ = r.ReadByte()
	}
}
