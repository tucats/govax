package rtl

import "fmt"

// itemListEntry is one VMS $GETxxx-style item-list entry: a 12-byte record
// of (buffer length, item code, buffer address, return-length address),
// matching the identical loop structure repeated in sys_getjpiw, sys_getdviw
// and sys_trnlnm.
type itemListEntry struct {
	BuffLen  uint16
	ItemCode uint16
	BuffAddr uint32
	RetAddr  uint32
}

// walkItemList reads consecutive 12-byte item-list entries starting at ptr,
// calling visit for each one until a zero (BuffLen == 0 && ItemCode == 0)
// terminator entry is reached — matching every $GETxxx service's own
// "while(1) { load entry; if (bufflen==0 && itemcode==0) break; ...; ptr +=
// 12; }" loop.
//
// Every service built on this shares the same "a bad pointer anywhere in
// this walk is SS_ACCVIO, an item code this service doesn't recognize is
// SS_BADPARAM, and stops the whole item list rather than skipping just that
// entry" shape, so status carries that VMS status code directly rather than
// a Go error: 0 means "ran every entry to completion" (the handler should
// go on to return SS_NORMAL itself), nonzero means "stop now and return this
// status from the service." visit follows the same convention for one entry.
func (env *Environment) walkItemList(ptr uint32, visit func(itemListEntry) uint32) uint32 {
	for {
		buffLen, err := env.mem.LoadWord(env.cpu, ptr)
		if err != nil {
			return ssAccVio
		}

		itemCode, err := env.mem.LoadWord(env.cpu, ptr+2)
		if err != nil {
			return ssAccVio
		}

		if buffLen == 0 && itemCode == 0 {
			return 0
		}

		buffAddr, err := env.mem.LoadLongword(env.cpu, ptr+4)
		if err != nil {
			return ssAccVio
		}

		retAddr, err := env.mem.LoadLongword(env.cpu, ptr+8)
		if err != nil {
			return ssAccVio
		}

		if status := visit(itemListEntry{BuffLen: buffLen, ItemCode: itemCode, BuffAddr: buffAddr, RetAddr: retAddr}); status != 0 {
			return status
		}

		ptr += 12
	}
}

// setRetLen stores size at e.RetAddr, matching every item-list handler's own
// "if (retaddr) { retsize = size; store_memory(retaddr, &retsize, 2); }"
// tail — a no-op when the caller didn't ask for the returned-length count.
// Returns ssAccVio on a bad RetAddr, 0 otherwise, following walkItemList's
// own status-code convention.
func (env *Environment) setRetLen(e itemListEntry, size uint16) uint32 {
	if e.RetAddr == 0 {
		return 0
	}

	if err := env.mem.StoreWord(env.cpu, e.RetAddr, size); err != nil {
		return ssAccVio
	}

	return 0
}

// maxItemListChain bounds how many item lists walkItemListChain follows,
// so a chain that loops back on itself is SS$_BADPARAM instead of a hang.
const maxItemListChain = 64

// chainFollowed is walkItemListChain's private "stop this list, follow
// the chain" signal from its visit wrapper. No VMS status is all ones.
const chainFollowed = ^uint32(0)

// walkItemListChain is walkItemList for the logical-name services, whose
// item lists may end in an LNM$_CHAIN entry (item code chain) whose
// buffer address is another item list to process next. A zero address
// ends the walk.
func (env *Environment) walkItemListChain(ptr uint32, chain uint16, visit func(itemListEntry) uint32) uint32 {
	for lists := 0; lists < maxItemListChain; lists++ {
		if ptr == 0 {
			return 0
		}

		next := uint32(0)

		status := env.walkItemList(ptr, func(e itemListEntry) uint32 {
			if e.ItemCode == chain {
				next = e.BuffAddr

				return chainFollowed
			}

			return visit(e)
		})

		if status != chainFollowed {
			return status
		}

		ptr = next
	}

	return ssBadParam
}

// itemValue is the data one information item returns ($GETJPI's
// JPI$_PID, $GETSYI's SYI$_VERSION, ...), as the bytes to store in the
// item's buffer. Numbers are stored low byte first, as the VAX stores
// them. The constructors below make the common kinds.
type itemValue struct {
	data string
}

// itemString is a character-string item, returned as it is.
func itemString(s string) itemValue { return itemValue{data: s} }

// itemPadded is a fixed-width string item, blank-padded to width (VMS
// pads names like JPI$_USERNAME and SYI$_VERSION with blanks).
func itemPadded(s string, width int) itemValue { return itemString(padded(s, width)) }

// itemByte, itemWord, itemLong, and itemQuad are 1-, 2-, 4-, and 8-byte
// numeric items.
func itemByte(v uint8) itemValue  { return itemValue{data: string([]byte{v})} }
func itemWord(v uint16) itemValue { return itemValue{data: string([]byte{byte(v), byte(v >> 8)})} }
func itemLong(v uint32) itemValue { return itemValue{data: littleEndian(uint64(v), 4)} }
func itemQuad(v uint64) itemValue { return itemValue{data: littleEndian(v, 8)} }

// padded returns s blank-padded on the right to width characters.
func padded(s string, width int) string { return fmt.Sprintf("%-*s", width, s) }

// littleEndian returns the low n bytes of v, low byte first.
func littleEndian(v uint64, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(v >> (8 * i))
	}

	return string(b)
}

// storeItem writes v into e's buffer, truncated to the buffer's length
// (so a longword into a 2-byte buffer keeps its low word), and the length
// stored to e's return-length address. Returns walkItemList's status
// convention: 0, or SS$_ACCVIO for a buffer or return length that can't
// be written.
func (env *Environment) storeItem(e itemListEntry, v itemValue) uint32 {
	n, _, err := storeBuffer(env, e.BuffAddr, e.BuffLen, v.data)
	if err != nil {
		return ssAccVio
	}

	return env.setRetLen(e, n)
}
