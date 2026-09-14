package rtl

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
