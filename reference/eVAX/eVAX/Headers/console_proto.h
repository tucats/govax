//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     console_proto.h
//
//  Purpose:    This header defines the prototypes for the command handlers
//              for the virtual VAX console.  They are kept separate here so
//              when a new command handler is added, only the console modules
//              need to be rebuilt.
//
//  History:    08/06/97    New header format standardization
//
//              09/24/98    Added CALL command
//


/*  Routine to execute a single command */

LONGWORD console_dispatch( char * cmd );
LONGWORD console_dispatch_init( void );


/*  Routines for each specific command  */

LONGWORD console_rom( char ** P );
LONGWORD console_init( char ** P );
LONGWORD console_zero( char ** P );
LONGWORD console_show( char ** P );
LONGWORD console_set( char ** P );
LONGWORD console_disasm( char ** P );
LONGWORD console_asm( char ** P );
LONGWORD console_exam( char ** P );
LONGWORD console_exec( char ** P );
LONGWORD console_quit( char ** P );
LONGWORD console_save( char ** p );
LONGWORD console_clear( char ** p );
LONGWORD console_step( char ** p );
LONGWORD console_load( char ** p );
LONGWORD console_time( char ** p );
LONGWORD console_include( char ** p );
LONGWORD console_asm( char ** P );
LONGWORD console_print( char ** P );
LONGWORD console_test( char ** P );
LONGWORD console_call( char ** P );
LONGWORD console_do( char ** P );
LONGWORD console_if( char ** P );
LONGWORD console_boot( char ** P );
LONGWORD console_run( char ** P );

/* 
 *  Console DCL handler routines.
 */

LONGWORD console_show_dcl( long id );
LONGWORD console_clear_dcl( long id );
LONGWORD console_exit_dcl( long id );
LONGWORD console_test_dcl( long id );
LONGWORD console_vminit_dcl( long id );

/*
 *  Console support routines
 */

LONGWORD save_binary( char ** p );
LONGWORD show_calls( LONGWORD count );
LONGWORD save_rom( char ** p );
LONGWORD save_nvram( char ** P );

/*
 *  Prototypes for image loading
 */
void init_ihd_maps( void );
char * find_image( char * fp );
void dump_icb_list( int full );
long get_region_size( int region, LONGWORD * size );
long set_region_size( int region, LONGWORD size );
LONGWORD decc_dump_memory( int flag );
/*
 *  This data structure handles command dispatch
 */

typedef LONGWORD (*console_handler)( char ** p ) ;

struct CONSOLE_DISPATCH_TABLE {
    LONGWORD    verb[ 3 ];
    console_handler handler;
};

LONGWORD push_command( char * cmd );
