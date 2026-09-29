// Package obj reads, writes, dumps, and checks VAX/VMS object modules: the
// .OBJ files MACRO-32 and the other VAX language processors produce and the
// VMS linker reads. See docs/PHASE-27.md.
//
// The format is the VAX object language, specified in chapter 7 of the VMS
// 5.0 Linker Utility Manual. An object module is a sequence of
// variable-length records, each starting with a record type byte:
//
//   - header records (HDR): the main module header (MHD) first, then the
//     language processor name (LNM), then optional text headers (source
//     files, title, copyright, and so on);
//   - global symbol directory records (GSD), each holding one or more
//     subrecords that define program sections (psects) and define or
//     refer to global symbols;
//   - text information and relocation records (TIR), whose commands drive
//     the linker's stack machine to store the module's code and data,
//     relocating addresses as they go; debugger (DBG) and traceback (TBT)
//     records use the same commands to build the debug symbol table;
//   - link option records (LNK), naming more files for the linker to
//     search;
//   - exactly one end of module record (EOM or EOMW), last.
//
// Module keeps every record, in order, with each record's own subrecords
// or commands, so Encode(Decode(records)) reproduces the original records
// byte for byte. Builder packs a module's content into records for
// callers that don't care how it's split.
//
// The numeric codes and record layouts come from the VAX modules of VMS
// 7.3's objfmt.sdl, generated into vmsdef.OBJConstants.
package obj
