package fdl

import "fmt"

// Kind is the kind of value an attribute takes (the manual's 4.1: switch,
// keyword, string, number), and the few with forms of their own.
type Kind int

// The kinds of value.
const (
	// Switch is YES or NO, TRUE or FALSE, or an abbreviation of one (T,
	// Y, F, N).
	Switch Kind = iota

	// Number is a decimal number, with an optional sign.
	Number

	// String is text in single or double quotation marks, or the rest of
	// the statement without them.
	String

	// Keyword is one of the attribute's keywords, or an abbreviation of one.
	Keyword

	// Protection is "(system:RWED, owner:RWED, group:RE, world:)".
	Protection

	// Owner is a UIC, "[200,201]", or an identifier, "[SYSTEM]".
	Owner

	// Date is a quoted absolute time, "1-JAN-2030 00:00:00.00".
	Date

	// Character is NULL_VALUE's: a character in quotation marks, or its
	// code in decimal.
	Character

	// FreeText is any text, kept as written: SYSTEM's SOURCE (VMS 7.1 took
	// "VAX/VMS", which isn't one of the manual's keywords) and AREA's
	// POSITION.
	FreeText
)

// attribute is one secondary attribute of a primary.
type attribute struct {
	name     string
	kind     Kind
	keywords []string // a Keyword attribute's values
}

// primary is one primary attribute: its name, whether it takes a number
// (KEY 0, AREA 1) or text (TITLE, IDENT), and its secondaries.
type primary struct {
	name        string
	numbered    bool
	text        bool
	secondaries []attribute
}

// Attribute constructors, so the table below reads as the manual's.
func sw(name string) attribute  { return attribute{name: name, kind: Switch} }
func num(name string) attribute { return attribute{name: name, kind: Number} }
func str(name string) attribute { return attribute{name: name, kind: String} }
func txt(name string) attribute { return attribute{name: name, kind: FreeText} }
func kw(name string, keywords ...string) attribute {
	return attribute{name: name, kind: Keyword, keywords: keywords}
}

// Keyword values.
var (
	organizations   = []string{"SEQUENTIAL", "RELATIVE", "INDEXED"}
	formats         = []string{"FIXED", "STREAM", "STREAM_CR", "STREAM_LF", "UNDEFINED", "VARIABLE", "VFC"}
	carriageControl = []string{"CARRIAGE_RETURN", "FORTRAN", "NONE", "PRINT"}
	keyTypes        = []string{
		"STRING", "DSTRING", "BIN2", "BIN4", "BIN8", "INT2", "INT4", "INT8", "DECIMAL",
		"DBIN2", "DBIN4", "DBIN8", "DINT2", "DINT4", "DINT8", "DDECIMAL", "COLLATED", "DCOLLATED",
	}
)

// segments are a KEY's SEGn_LENGTH and SEGn_POSITION, n from 0 to 7.
func segments() []attribute {
	var a []attribute

	for n := range MaxSegments {
		a = append(a, num(fmt.Sprintf("SEG%d_LENGTH", n)), num(fmt.Sprintf("SEG%d_POSITION", n)))
	}

	return a
}

// MaxSegments is how many segments a key may have.
const MaxSegments = 8

// primaries is every primary attribute and its secondaries, the manual's
// 4.1.1 to 4.1.13, in the order the manual says a file has them. A
// secondary's abbreviation is matched against its primary's list only.
var primaries = []primary{
	{name: "TITLE", text: true},
	{name: "IDENT", text: true},
	{name: "SYSTEM", secondaries: []attribute{str("DEVICE"), txt("SOURCE"), txt("TARGET")}},
	{name: "FILE", secondaries: []attribute{
		num("ALLOCATION"), sw("ASYNCHRONOUS"), sw("BEST_TRY_CONTIGUOUS"), num("BUCKET_SIZE"),
		num("CLUSTER_SIZE"), num("CONTEXT"), sw("CONTIGUOUS"), sw("CREATE_IF"), str("DEFAULT_NAME"),
		sw("DEFERRED_WRITE"), sw("DELETE_ON_CLOSE"), sw("DIRECTORY_ENTRY"), num("EXTENSION"),
		sw("FILE_MONITORING"), num("GLOBAL_BUFFER_COUNT"), num("MAX_RECORD_NUMBER"),
		sw("MAXIMIZE_VERSION"), num("MT_BLOCK_SIZE"), sw("MT_CLOSE_REWIND"), sw("MT_CURRENT_POSITION"),
		sw("MT_NOT_EOF"), sw("MT_OPEN_REWIND"), txt("MT_PROTECTION"), str("NAME"),
		sw("NON_FILE_STRUCTURED"), kw("ORGANIZATION", organizations...), sw("OUTPUT_FILE_PARSE"),
		{name: "OWNER", kind: Owner}, sw("PRINT_ON_CLOSE"), {name: "PROTECTION", kind: Protection},
		sw("READ_CHECK"), num("REVISION"), sw("SEQUENTIAL_ONLY"), txt("STORED_SEMANTICS"),
		sw("SUBMIT_ON_CLOSE"), sw("SUPERSEDE"), sw("TEMPORARY"), sw("TRUNCATE_ON_CLOSE"),
		sw("USER_FILE_OPEN"), num("WINDOW_SIZE"), sw("WRITE_CHECK"),
	}},
	{name: "DATE", secondaries: []attribute{
		{name: "BACKUP", kind: Date}, {name: "CREATION", kind: Date},
		{name: "EXPIRATION", kind: Date}, {name: "REVISION", kind: Date},
	}},
	{name: "RECORD", secondaries: []attribute{
		sw("BLOCK_SPAN"), kw("CARRIAGE_CONTROL", carriageControl...), num("CONTROL_FIELD_SIZE"),
		kw("FORMAT", formats...), sw("MSB_RECORD_LENGTH"), num("SIZE"),
	}},
	{name: "ACCESS", secondaries: []attribute{
		sw("BLOCK_IO"), sw("DELETE"), sw("GET"), sw("PUT"), sw("RECORD_IO"), sw("TRUNCATE"), sw("UPDATE"),
	}},
	{name: "NETWORK", secondaries: []attribute{
		num("BLOCK_COUNT"), sw("LINK_CACHE_ENABLE"), num("LINK_TIMEOUT"), sw("NETWORK_DATA_CHECKING"),
	}},
	{name: "SHARING", secondaries: []attribute{
		sw("DELETE"), sw("GET"), sw("MULTISTREAM"), sw("PROHIBIT"), sw("PUT"), sw("UPDATE"), sw("USER_INTERLOCK"),
	}},
	{name: "CONNECT", secondaries: []attribute{
		sw("ASYNCHRONOUS"), sw("BLOCK_IO"), sw("BUCKET_IO"), num("CONTEXT"), sw("END_OF_FILE"),
		sw("FAST_DELETE"), sw("FILL_BUCKETS"), sw("KEY_GREATER_EQUAL"), sw("KEY_GREATER_THAN"),
		sw("KEY_LIMIT"), num("KEY_OF_REFERENCE"), sw("LOCATE_MODE"), sw("LOCK_ON_READ"),
		sw("LOCK_ON_WRITE"), sw("MANUAL_UNLOCKING"), num("MULTIBLOCK_COUNT"), num("MULTIBUFFER_COUNT"),
		sw("NOLOCK"), sw("NONEXISTENT_RECORD"), sw("READ_AHEAD"), sw("READ_REGARDLESS"),
		sw("TIMEOUT_ENABLE"), num("TIMEOUT_PERIOD"), sw("TRUNCATE_ON_PUT"), sw("TT_CANCEL_CONTROL_O"),
		sw("TT_PROMPT"), sw("TT_PURGE_TYPE_AHEAD"), sw("TT_READ_NOECHO"), sw("TT_READ_NOFILTER"),
		sw("TT_UPCASE_INPUT"), sw("UPDATE_IF"), sw("WAIT_FOR_RECORD"), sw("WRITE_BEHIND"),
	}},
	{name: "AREA", numbered: true, secondaries: []attribute{
		num("ALLOCATION"), sw("ASYNCHRONOUS"), sw("BEST_TRY_CONTIGUOUS"), num("BUCKET_SIZE"),
		sw("CONTIGUOUS"), sw("EXACT_POSITIONING"), num("EXTENSION"), txt("POSITION"), num("VOLUME"),
	}},
	{name: "KEY", numbered: true, secondaries: append([]attribute{
		sw("CHANGES"), str("COLLATING_SEQUENCE"), num("DATA_AREA"), num("DATA_FILL"),
		sw("DATA_KEY_COMPRESSION"), sw("DATA_RECORD_COMPRESSION"), sw("DUPLICATES"), num("INDEX_AREA"),
		sw("INDEX_COMPRESSION"), num("INDEX_FILL"), num("LENGTH"), num("LEVEL1_INDEX_AREA"), str("NAME"),
		sw("NULL_KEY"), {name: "NULL_VALUE", kind: Character}, num("POSITION"), num("PROLOG"),
		kw("TYPE", keyTypes...),
	}, segments()...)},
	{name: "ANALYSIS_OF_AREA", numbered: true, secondaries: []attribute{num("RECLAIMED_SPACE")}},
	{name: "ANALYSIS_OF_KEY", numbered: true, secondaries: []attribute{
		num("DATA_FILL"), num("DATA_KEY_COMPRESSION"), num("DATA_RECORD_COMPRESSION"),
		num("DATA_RECORD_COUNT"), num("DATA_SPACE_OCCUPIED"), num("DEPTH"), num("DUPLICATES_PER_SIDR"),
		num("INDEX_COMPRESSION"), num("INDEX_FILL"), num("INDEX_SPACE_OCCUPIED"),
		num("LEVEL1_RECORD_COUNT"), num("MEAN_DATA_LENGTH"), num("MEAN_INDEX_LENGTH"),
	}},
}

// match finds word, already in capitals, among names: an exact match, or
// else the one name it's an abbreviation of. found is -1 when none is;
// ambiguous says more than one is.
func match(word string, names []string) (found int, ambiguous bool) {
	found = -1

	for i, n := range names {
		if n == word {
			return i, false
		}
	}

	if word == "" {
		return -1, false
	}

	for i, n := range names {
		if len(word) < len(n) && n[:len(word)] == word {
			if found >= 0 {
				return -1, true
			}

			found = i
		}
	}

	return found, false
}

// primaryNames and secondaryNames are the tables' names, for match.
func primaryNames() []string {
	names := make([]string, len(primaries))
	for i, p := range primaries {
		names[i] = p.name
	}

	return names
}

func (p *primary) secondaryNames() []string {
	names := make([]string, len(p.secondaries))
	for i, a := range p.secondaries {
		names[i] = a.name
	}

	return names
}
