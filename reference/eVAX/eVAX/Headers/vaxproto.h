//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     vaxproto.h
//
//  Purpose:    This module prototype definitions for shared routines
//              in the emulator _other_ than the assembler or the
//              instruction emulators.  Included from vax.pch.
//
//  History:    02/23/98    Unified header
//
//



//  Public prototypes

LONGWORD execute_vax( short disasm );
LONGWORD execute_symbol( char * symbol, short disasm );
LONGWORD decode_instruction( struct OPCODE * opcode, short disasm );
LONGWORD dump_registers( void );
LONGWORD dump_psl( void );
LONGWORD exam_reg( LONGWORD verb );
LONGWORD exam_reg_string( char * p );
LONGWORD set_reg( char ** P );
LONGWORD format_instruction( char * prefix, ULONGWORD addr );
void pad( char * buff, int len );
LONGWORD break_over_instruction( int opcode );
LONGWORD show_regions( void );

struct BREAKSTR * set_break( LONGWORD pc, int after, int kind );
long clear_break( struct BREAKSTR * bptr );


LONGWORD assemble( char ** buffer );
LONGWORD probe( ULONGWORD addr, short size, short mode );
LONGWORD uppercase( char * p );
LONGWORD read_verb( char ** P, LONGWORD * code );

LONGWORD help( char ** P );
LONGWORD console( char * cmd );
LONGWORD get_return( LONGWORD * Addr );
long format_exception( void );

LONGWORD decode_operand(          /* The virtual machine state    */
                    struct OPCODE * opcode,     /* The opcode structure         */
                    short opcount,              /* Which operand are we doing?  */
                    LONGWORD * the_pc,              /* Virtual PC address           */
                    short *Treg,                /* Index of temp regs           */
                    short scale,                /* Scale of operand             */
                    short idx );                 /* Are we in indexed mode       */

LONGWORD disasm_operand(          /* The virtual machine state    */
                    struct OPCODE * opcode,     /* The opcode structure         */
                    short opcount,              /* Which operand are we doing?  */
                    LONGWORD * the_pc,              /* Virtual PC address           */
                    short *Treg,                /* Index of temp regs           */
                    short scale,                /* Scale of operand             */
                    short idx,                  /* Are we in indexed mode       */
                    char * disasm );            /* Disasm buffer if any         */
                    
LONGWORD alloc_vax( LONGWORD physmem );
LONGWORD free_vax( struct VAX * theVax );
LONGWORD init_emulators( void );

LONGWORD read_psl_bits( void );
LONGWORD write_psl_bits( void );

LONGWORD handle_fault( void );
LONGWORD set_fault( LONGWORD code, LONGWORD count, ... );
LONGWORD interrupt( LONGWORD code, LONGWORD ipl, LONGWORD quantum );
int poll_keyboard( void );

LONGWORD load_memory( LONGWORD addr, unsigned char * dest, short count);
LONGWORD load_byte( LONGWORD address, unsigned char * dest );
LONGWORD load_register( short regn, ULONGWORD address, short count);

int add_watchpoint( ULONGWORD addr, short size, char * name );
int show_watchpoints( void );
int delete_watchpoint( ULONGWORD addr );

LONGWORD store_io( LONGWORD address, unsigned char * src, short count );
LONGWORD load_io( LONGWORD address, unsigned char * src, short count );
LONGWORD pmvalid( ULONGWORD address );
unsigned char * get_address( ULONGWORD address );

LONGWORD push_data( void * data, short len );
LONGWORD store_memory( LONGWORD addr, unsigned char * src, short count);
LONGWORD sext( LONGWORD data, short size );
LONGWORD vm( ULONGWORD * Addr, short * size, short mode );
void invalidate_tb( void );
void invalidate_tb_prot( void );
void invalidate_page( ULONGWORD addr );
LONGWORD set_priv_reg( short reg, LONGWORD value );
void set_mode_stack( LONGWORD newmode );
char * get_operand( struct OPCODE * opcode, short n, short mode );
LONGWORD put_operand( struct OPCODE * opcode,
                  short n,
                  short mode,
                  char * data );

LONGWORD get_memory_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD * result_p );
LONGWORD get_register_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD * result_p );

LONGWORD set_memory_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD data );
LONGWORD set_register_field( LONGWORD position, LONGWORD size, LONGWORD base, LONGWORD data );
ULONGWORD bit_sext( ULONGWORD data, LONGWORD size );

char * vaxmsg( LONGWORD rc );
char * vaxexcept( short code );

char * format_mask( unsigned short mask );

LONGWORD flush_blanks( char ** P );
LONGWORD uppercase( char * p );

FILE * xfopen( char * name, char * mode);

int   is_blank( char ch );

void freemem( void * p );
char * getmem( LONGWORD size );
void printmem( void );

long p1_init( void );

short   isend( char ch );
long define_logical( long id );
long show_logical( long id );
LONGWORD set_logical( char * table, char * name, char * value, long attr );
LONGWORD init_logicals( void );
long define_device( long id );
long show_device( long id );

/*
 *  This is a Mac function, lets simulate it for other hosts.  It
 *  should be defined in DRIVER.C
 */

#ifndef macintosh
LONGWORD TickCount( void );
#endif


#include "emulator_entries.h"


