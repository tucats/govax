package corevms

import (
	"fmt"
	"strings"
)

// $GETQUI's items as text (docs/planning/PHASE-98 - batch queues.md,
// subtask 9): the names F$GETQUI takes for its item argument, and how it
// writes each, from the DCL Dictionary's Table DCLI-9 and VMS 7.1's answers
// (testdata/bat98's BAT98J and protection matrix). An item is one of
// $GETQUI's output items, written in its form; or a Boolean, one bit of a
// status or flags item, written TRUE or FALSE: QUEUE_STOPPED is
// QUEUE_STATUS's QUI$M_QUEUE_STOPPED, JOB_HOLDING JOB_STATUS's
// QUI$M_JOB_HOLDING. The Dictionary lists which bit belongs to which item;
// a name in both of its lists (JOB_RESTART) is the flags item's.

// QueueItem is one of F$GETQUI's items: the $GETQUI item code that gives
// its value, how the value is written, and, for a Boolean, the bit of the
// item's longword it tests (0 for none).
type QueueItem struct {
	Code uint16
	Form ItemForm
	Mask uint32
}

// queueItemForms are F$GETQUI's items that aren't Booleans, each with its
// form; FormDelta is a CPU time in 10-millisecond units, written as a
// delta time ("   0 00:05:00.00", BAT98J's CPU_LIMIT). VMS 7.1 wrote the
// UIC and the owner's by identifier ("[SYSTEM]", and "[0,0]" for an owner
// the caller may not see), the protection as a file's ("SYSTEM=E,
// OWNER=D, GROUP=R, WORLD=W"), and JOB_PID in hexadecimal.
var queueItemForms = map[string]ItemForm{
	"ACCOUNT_NAME": FormString, "AFTER_TIME": FormTime, "ASSIGNED_QUEUE_NAME": FormString,
	"AUTOSTART_ON": FormString, "BASE_PRIORITY": FormNumber, "CHARACTERISTICS": FormString,
	"CHARACTERISTIC_NAME": FormString, "CHARACTERISTIC_NUMBER": FormNumber,
	"CHECKPOINT_DATA": FormString, "CLI": FormString, "COMPLETED_BLOCKS": FormNumber,
	"CONDITION_VECTOR": FormNumber, "CPU_DEFAULT": FormDelta, "CPU_LIMIT": FormDelta,
	"DEFAULT_FORM_NAME": FormString, "DEFAULT_FORM_STOCK": FormString, "DEVICE_NAME": FormString,
	"ENTRY_NUMBER": FormNumber, "EXECUTING_JOB_COUNT": FormNumber, "FILE_COPIES": FormNumber,
	"FILE_COPIES_DONE": FormNumber, "FILE_COUNT": FormNumber, "FILE_FLAGS": FormNumber,
	"FILE_IDENTIFICATION": FormString, "FILE_SETUP_MODULES": FormString,
	"FILE_SPECIFICATION": FormString, "FILE_STATUS": FormNumber, "FIRST_PAGE": FormNumber,
	"FORM_DESCRIPTION": FormString, "FORM_FLAGS": FormNumber, "FORM_LENGTH": FormNumber,
	"FORM_MARGIN_BOTTOM": FormNumber, "FORM_MARGIN_LEFT": FormNumber,
	"FORM_MARGIN_RIGHT": FormNumber, "FORM_MARGIN_TOP": FormNumber, "FORM_NAME": FormString,
	"FORM_NUMBER": FormNumber, "FORM_SETUP_MODULES": FormString, "FORM_STOCK": FormString,
	"FORM_WIDTH": FormNumber, "GENERIC_TARGET": FormString, "HOLDING_JOB_COUNT": FormNumber,
	"INTERVENING_BLOCKS": FormNumber, "INTERVENING_JOBS": FormNumber,
	"JOB_COMPLETION_QUEUE": FormString, "JOB_COMPLETION_TIME": FormTime,
	"JOB_COPIES": FormNumber, "JOB_COPIES_DONE": FormNumber, "JOB_FLAGS": FormNumber,
	"JOB_LIMIT": FormNumber, "JOB_NAME": FormString, "JOB_PID": FormHex,
	"JOB_RESET_MODULES": FormString, "JOB_RETENTION_TIME": FormTime, "JOB_SIZE": FormNumber,
	"JOB_SIZE_MAXIMUM": FormNumber, "JOB_SIZE_MINIMUM": FormNumber, "JOB_STATUS": FormNumber,
	"LAST_PAGE": FormNumber, "LIBRARY_SPECIFICATION": FormString, "LOG_QUEUE": FormString,
	"LOG_SPECIFICATION": FormString, "MANAGER_NAME": FormString, "MANAGER_NODES": FormString,
	"MANAGER_STATUS": FormNumber, "NOTE": FormString, "OPERATOR_REQUEST": FormString,
	"OWNER_UIC": FormOwnerUIC, "PAGE_SETUP_MODULES": FormString,
	"PENDING_JOB_BLOCK_COUNT": FormNumber, "PENDING_JOB_COUNT": FormNumber,
	"PENDING_JOB_REASON": FormNumber, "PRIORITY": FormNumber, "PROCESSOR": FormString,
	"PROTECTION": FormProtection, "QUEUE_DESCRIPTION": FormString, "QUEUE_DIRECTORY": FormString,
	"QUEUE_FLAGS": FormNumber, "QUEUE_NAME": FormString, "QUEUE_STATUS": FormNumber,
	"REQUEUE_QUEUE_NAME": FormString, "RESTART_QUEUE_NAME": FormString,
	"RETAINED_JOB_COUNT": FormNumber, "SCSNODE_NAME": FormString,
	"SUBMISSION_TIME": FormTime, "TIMED_RELEASE_JOB_COUNT": FormNumber, "UIC": FormOwnerUIC,
	"USERNAME": FormString, "WSDEFAULT": FormNumber, "WSEXTENT": FormNumber,
	"WSQUOTA": FormNumber,
}

// queueBooleans are F$GETQUI's Boolean items, by the item whose bits
// they are: each is the bit QUI$M_<name> of it. SECURITY_INACCESSIBLE is
// QUEUE_FLAGS's.
var queueBooleans = map[string][]string{
	"FILE_FLAGS": {
		"FILE_BURST", "FILE_DELETE", "FILE_DOUBLE_SPACE", "FILE_FLAG", "FILE_PAGE_HEADER",
		"FILE_PAGINATE", "FILE_PASSALL", "FILE_TRAILER",
	},
	"FILE_STATUS": {"FILE_CHECKPOINTED", "FILE_EXECUTING"},
	"FORM_FLAGS":  {"FORM_SHEET_FEED", "FORM_TRUNCATE", "FORM_WRAP"},
	"JOB_FLAGS": {
		"JOB_CPU_LIMIT", "JOB_ERROR_RETENTION", "JOB_FILE_BURST", "JOB_FILE_BURST_ONE",
		"JOB_FILE_FLAG", "JOB_FILE_FLAG_ONE", "JOB_FILE_PAGINATE", "JOB_FILE_TRAILER",
		"JOB_FILE_TRAILER_ONE", "JOB_LOG_DELETE", "JOB_LOG_NULL", "JOB_LOG_SPOOL",
		"JOB_LOWERCASE", "JOB_NOTIFY", "JOB_RESTART", "JOB_RETENTION", "JOB_WSDEFAULT",
		"JOB_WSEXTENT", "JOB_WSQUOTA",
	},
	"JOB_STATUS": {
		"JOB_ABORTING", "JOB_EXECUTING", "JOB_HOLDING", "JOB_INACCESSIBLE", "JOB_REFUSED",
		"JOB_REQUEUE", "JOB_RETAINED", "JOB_STARTING", "JOB_TIMED_RELEASE", "JOB_SUSPENDED",
		"JOB_PENDING", "JOB_STALLED",
	},
	"MANAGER_STATUS": {
		"MANAGER_FAILOVER", "MANAGER_RUNNING", "MANAGER_START_PENDING", "MANAGER_STARTING",
		"MANAGER_STOPPED", "MANAGER_STOPPING",
	},
	"PENDING_JOB_REASON": {
		"PEND_CHAR_MISMATCH", "PEND_JOB_SIZE_MAX", "PEND_JOB_SIZE_MIN",
		"PEND_LOWERCASE_MISMATCH", "PEND_NO_ACCESS", "PEND_QUEUE_BUSY", "PEND_QUEUE_STATE",
		"PEND_STOCK_MISMATCH",
	},
	"QUEUE_FLAGS": {
		"QUEUE_ACL_SPECIFIED", "QUEUE_AUTOSTART", "QUEUE_BATCH", "QUEUE_CPU_DEFAULT",
		"QUEUE_CPU_LIMIT", "QUEUE_FILE_BURST", "QUEUE_FILE_BURST_ONE", "QUEUE_FILE_FLAG",
		"QUEUE_FILE_FLAG_ONE", "QUEUE_FILE_PAGINATE", "QUEUE_FILE_TRAILER",
		"QUEUE_FILE_TRAILER_ONE", "QUEUE_GENERIC", "QUEUE_GENERIC_SELECTION",
		"QUEUE_JOB_BURST", "QUEUE_JOB_FLAG", "QUEUE_JOB_SIZE_SCHED", "QUEUE_JOB_TRAILER",
		"QUEUE_NO_INITIAL_FF", "QUEUE_PRINTER", "QUEUE_RECORD_BLOCKING", "QUEUE_RETAIN_ALL",
		"QUEUE_RETAIN_ERROR", "QUEUE_SWAP", "QUEUE_TERMINAL", "QUEUE_WSDEFAULT",
		"QUEUE_WSEXTENT", "QUEUE_WSQUOTA", "SECURITY_INACCESSIBLE",
	},
	"QUEUE_STATUS": {
		"QUEUE_ALIGNING", "QUEUE_AUTOSTART_INACTIVE", "QUEUE_AVAILABLE", "QUEUE_BUSY",
		"QUEUE_CLOSED", "QUEUE_IDLE", "QUEUE_LOWERCASE", "QUEUE_PAUSED", "QUEUE_PAUSING",
		"QUEUE_REMOTE", "QUEUE_RESETTING", "QUEUE_RESUMING", "QUEUE_SERVER",
		"QUEUE_STALLED", "QUEUE_STARTING", "QUEUE_STOP_PENDING", "QUEUE_STOPPED",
		"QUEUE_STOPPING", "QUEUE_UNAVAILABLE",
	},
}

// queueItems are every F$GETQUI item, by name.
var queueItems = func() map[string]QueueItem {
	m := map[string]QueueItem{}

	for name, form := range queueItemForms {
		m[name] = QueueItem{Code: quiCode(name), Form: form}
	}

	for n := 1; n <= 8; n++ {
		name := "PARAMETER_" + string(rune('0'+n))
		m[name] = QueueItem{Code: quiCode(name), Form: FormString}
	}

	for item, names := range queueBooleans {
		for _, name := range names {
			m[name] = QueueItem{Code: quiCode(item), Form: FormBoolean, Mask: quiMask(name)}
		}
	}

	return m
}()

// QueueItemNamed is the F$GETQUI item called name (in capitals), if there
// is one.
func QueueItemNamed(name string) (QueueItem, bool) {
	it, ok := queueItems[strings.ToUpper(name)]

	return it, ok
}

// QueueFunctionCode is the $GETQUI function F$GETQUI's function argument
// names ("" is CANCEL_OPERATION, the Dictionary's other form), if there is
// one.
func QueueFunctionCode(name string) (uint16, bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "" {
		name = "CANCEL_OPERATION"
	}

	switch name {
	case "CANCEL_OPERATION", "DISPLAY_CHARACTERISTIC", "DISPLAY_ENTRY", "DISPLAY_FILE",
		"DISPLAY_FORM", "DISPLAY_JOB", "DISPLAY_MANAGER", "DISPLAY_QUEUE", "TRANSLATE_QUEUE":
		return quiCode(name), true
	}

	return 0, false
}

// QueueSearchFlag is the QUI$M_SEARCH_ bit of F$GETQUI's flag keyword
// name (Table DCLI-8), if there is one.
func QueueSearchFlag(name string) (uint32, bool) {
	switch name = strings.ToUpper(strings.TrimSpace(name)); name {
	case "ALL_JOBS", "BATCH", "EXECUTING_JOBS", "FREEZE_CONTEXT", "GENERIC", "HOLDING_JOBS",
		"PENDING_JOBS", "PRINTER", "RETAINED_JOBS", "SERVER", "SYMBIONT", "TERMINAL",
		"THIS_JOB", "TIMED_RELEASE_JOBS", "WILDCARD":
		return quiMask("SEARCH_" + name), true
	}

	return 0, false
}

// deltaTicks writes a CPU time in 10-millisecond units as a delta time,
// "dddd hh:mm:ss.cc".
func deltaTicks(ticks uint32) string {
	cs := uint64(ticks)

	return fmt.Sprintf("%4d %02d:%02d:%02d.%02d", cs/8640000, cs/360000%24, cs/6000%60, cs/100%60, cs%100)
}
