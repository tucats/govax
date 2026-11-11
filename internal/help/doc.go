// Package help is VMS's HELP: it reads help modules, finds the topics a
// request names, and lays them out as VMS 7.1's HELP does (docs/planning/PHASE-90 -
// VMS help.md). It is plain Go, knowing nothing of RMS, DCL, or the
// console; DCL's HELP, the debugger's HELP, and LBR$OUTPUT_HELP are to be
// its callers.
//
// # Help modules
//
// A help library (a .HLB file, which internal/lbr reads) holds modules,
// each one level-1 topic. A module's records are lines of text, some of
// them key lines: a level number, a blank or a tab, and the key.
//
//	1 COPY
//	   Text of COPY.
//	2 Parameters
//	   Text of COPY's parameters.
//	2 Qualifiers
//	/LOG
//	   Text of COPY's /LOG qualifier.
//
// A key line starts a topic one level below the key line before it, so
// the keys make a tree (ParseModule builds it, a Topic per key). A line
// starting with "/" in column 1 is a qualifier line: it starts a small
// topic of its own inside the topic whose text it is in, which HELP finds
// both there ("HELP COPY Qualifiers /LOG") and one level up ("HELP COPY
// /LOG"), and lists among its grandparent's subtopics.
//
// # Requests and displays
//
// Split turns a request into words, Lookup finds the topics the words
// name in a Library, and Render lays the answer out at a width. Lookup's
// answer is a list of Displays, one per topic shown (several when a word
// is ambiguous or a wildcard), or the "Sorry, no documentation" display
// when nothing matched.
//
// # Prompting
//
// A Session is the interactive HELP: it answers the first request, then
// prompts ("Topic?", "ALPHA Subtopic?") and answers each response, going
// down into a topic with subtopics, up a level at an empty response, and
// into another library at "@library". It reads and writes through
// functions its caller gives it, so the console, the debugger (whose
// requests are looked up under its library's DEBUG module), and
// LBR$OUTPUT_HELP can each supply their own terminal, file, or program.
//
// Every rule here was settled by testdata/help90's probe, which asked
// VMS 7.1's HELP about modules written for it; the phase doc's "What the
// probe settled" records the answers, and TestHelp90Oracle holds this
// package to VMS's logs. Where the probe didn't settle a rule, the code
// says so ("unconfirmed").
package help
