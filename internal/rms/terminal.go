package rms

import (
	"bufio"
	"errors"
	"io"
)

// ctrlZ is the character a terminal user types to end input: a $GET that
// reads it returns RMS$_EOF.
const ctrlZ = 0x1A

// terminalRecord is SYS$GET's read of one record from the terminal
// (RMS Reference, $GET: "terminal input"). If the RAB asks for a prompt
// (RAB$V_PMT in RAB$L_ROP), the RAB$B_PSZ bytes at RAB$L_PBF are written
// first, as they are. A record then ends at a carriage return or a line
// feed (a host "\r\n" pair is one end), neither of which is part of it,
// or when capacity bytes have been read: the terminal driver ends a read
// when the buffer fills, and whatever was typed beyond it is the next
// record. A Ctrl/Z, or the end of the host's input with nothing read,
// is end of file: status is RMS$_EOF. status is 0 for a record.
func terminalRecord(ctx *Context, rabAddr uint32, capacity int) (record []byte, status uint32, err error) {
	rop, err := ctx.loadLongword(rabAddr + rabROP)
	if err != nil {
		return nil, 0, err
	}

	if rop&ropPMT != 0 {
		if err := writePrompt(ctx, rabAddr); err != nil {
			return nil, 0, err
		}
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

// writePrompt writes a RAB's prompt (RAB$L_PBF, RAB$B_PSZ) to the console.
func writePrompt(ctx *Context, rabAddr uint32) error {
	pbf, err := ctx.loadLongword(rabAddr + rabPBF)
	if err != nil {
		return err
	}

	psz, err := ctx.loadByte(rabAddr + rabPSZ)
	if err != nil {
		return err
	}

	if psz == 0 || ctx.Console == nil {
		return nil
	}

	prompt, err := ctx.loadFixedString(pbf, int(psz))
	if err != nil {
		return err
	}

	_, _ = io.WriteString(ctx.Console, prompt)

	return nil
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
