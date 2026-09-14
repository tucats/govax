
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     DRIVER.C
//
//  Purpose:    This module contains the console interface for the VAX
//              emulator, and "sponsors" the SIOUX code that manages 
//              the pseudo-terminal emulator.  Note that SIOUX is a part
//              of the Metrowerks Standard Library
//
//  History:    07/28/97    New header format standardized on.
//
//              02/01/98    Added ASM_ADDRPROMPT flag support to control if
//                          the ASM> prompt includes the machine address that
//                          the intruction(s) will be written to.
//              12/07/98    Made slightly more portable so it could build
//                          on VMS as well.
//
//              12/14/98    Added fake TickCount for VMS.  Other ports will
//                          ultimately need to supply this, I guess.
//
//              01/03/99    Fix illegal abuse of string constant for init
//                          command.  Now we'll copy it to the command buffer.
//                          Also, let's get the initial command(s) from the
//                          file vax.init, which contains the first commands
//                          executed, such as setting up the memory model.
//
//              01/23/99    Created memory allocation shell.  I need this to be
//                          able to interject memory debugging tools on various
//                          platforms, that may interface with the O/S differently.
//
//              02/03/99    Change the name of the init file to be lower case.
//                          Most targets don't care, but Unix is case-sensitive
//                          and tends to default to lower case names.
//
//              02/25/99    Fix TickCount to be correct for more hosts.
//
//              06/29/01    Sigificant fixes to support command line arguments,
//                          and the "verbose" console flag for messaging.
//
//				08/14/01	Version 2.0
//

#include "vax.h"
#include "asmproto.h"
#include "console_proto.h"

#ifdef macintosh
#include "SIOUX.h"
#endif

#include <stdlib.h>
#include <stdio.h>

#ifdef VMS
#include <starlet.h>
#include <lib$routines.h>
#endif

/*
 *  If we're using the generic (non Mac, non VMS) version of TickCount
 *  then we need the time.h header file.
 */

#if !defined(VMS) && !defined(macintosh)
#include "time.h"
#endif

/*
 *  Definitions for command line parsing.
 */

#include "dclrtl.h"

static long bind_dcl( void );

/*
 *  Definitions for include file processing.
 */

#include "include.h"

struct INCLUDE * include = 0L;


/*
 *  Define the size of the initial default VAX.  This is large enough to be
 *  useful, and it doesn't cost much memory and is enough to bootstrap ourselves
 *  into console mode.  
 */

#define MINIMUM_VAX_MEMORY ( 2048L * 512L )     /* 1MB of memory */

/*
 *  Global statistics for memory management live here.
 */

LONGWORD total_allocated = 0L;
LONGWORD count_allocated = 0L;

LONGWORD total_freed = 0L;
LONGWORD count_freed = 0L;

/*
 *  This is the "master" VAX.  The runtimes are written to allow
 *  multiple VAXen at the same time.  However, the driver really
 *  has only this one.
 */

struct VAX * theVAX = 0L;

/*
 *  Holding area for the command
 */
 

/*
 *  Main entry point for the driver.
 */

int main( int argc, char * argv[] )
{

    LONGWORD rc, commandloop;
    char * cmd, *p, *pp, *pq, vname[ 32 ];
    char * prompt, prompt_buff[ 32 ];
    char * build;
    int n, argn;

#ifdef DEBUG
    build = "Debug";
#else
    build = "Optimized";
#endif

#ifdef macintosh
    SIOUXSettings.asktosaveonclose = 0;
    SIOUXSettings.autocloseonquit = 1;      /* 0 = wait for QUIT, 1 = auto close */
    SIOUXSettings.tabspaces = 4;            /* To match the CW Editor tab stops */
#endif

    cmd = getmem( 512 );
    
//  Prime stdout.  This is needed on some systems (Notably Mac OS) to cause the
//  console window to be drawn.

    printf( "%s", "" );
    
#ifdef macintosh
    SIOUXSetTitle( "\peVAX Console" );
#endif

    vax_init = 0L;

/*  Initialize the DCL parser */

    rc = DCLread( "evax.dcl" );    /* later, build in-memory */
    if( DCLERROR( rc )) {
        printf( "Unable to initialize command language grammar.\n" );
        getc(stdin);
        return 0;
    };

    rc = bind_dcl();
    if( rc ) {
        printf( "Unabled to bind verbs to internal handlers.\n" );
        getc( stdin );
        return 0;
    }
    
/*  The initial command creates a 2MB machine and initializes virtual memory for it. */

    strcpy( cmd, "INCLUDE \"vax.init\"" );
    p = cmd;
    commandloop = 1;
    
    alloc_vax( MINIMUM_VAX_MEMORY );
    if( !vax_init ) {
        printf( "Catastrophic failure initializing VAX.\n" );
        return( 0 );
    }
    
    vax.console.running = 1;
    argn = 0;
    
    pq = getmem( 1024 );
    pq[0] = 0;
    
    for( n = 1; n < argc; n++ ) {
        strcat( pq, " " );
        strcat( pq, argv[ n ]);
    }
    uppercase(pq);
    set_symbol_string_direct( "CONSOLE$ARG_CMD", pq, ( SYM_STRING | SYM_PERMANENT ));
    
    /* At this point we know if there was anything on the command line.  If not, print */
    /* out the full header.  Otherwise, we stay quiet for now. */
    
    if( *pq == 0 )
        printf( "eVAX 2.0\n%s build %s %s\n(C) 1997-2001 Forest Edge Software\n\n" ,
            build, __DATE__, __TIME__ );

    for( n = 1; n < argc; n++ ) {
        pp = argv[ n ];
        if( *pp == '-' ) {
            argn++;
            sprintf( vname, "CONSOLE$ARG_%d", argn );
            pq = getmem( strlen( pp ) );
            strcpy( pq, pp+1 );
            set_symbol_string_direct( vname, pq, ( SYM_STRING | SYM_PERMANENT ));
        }
        else {
            pq = getmem( strlen( pp ) + 1 );
            strcpy( pq, pp );
            set_symbol_string_direct( "CONSOLE$ARG_FILE", pq, ( SYM_STRING | SYM_PERMANENT ));
        }
    }    
    set_symbol_direct( "CONSOLE$ARG_COUNT", argn, SYM_PERMANENT );
        
    while( commandloop ) {

        //  If an allocation failure occured, or this is the first time
        //  we loop, create a minimal VAX so there's storage associated
        //  with the rest of the loop.  If that fails, we're hosed so quit.
        
 
        //  Make the global VAX be a copy of the active vax
        
        theVAX = &vax;

        //  If something caused the console to stop, exit
            
        if( !vax.console.running ) {
            commandloop = 0;
            continue;
        }
        
        //  See if there is an include file we should read.
        
        if( include ) {
                            
                p = fgets( cmd, 511, include-> fp );
                
                //  If its EOF, then close this file and process a dummy comment
                
                if( p == 0L ) {
                    pop_include();
                    cmd[ 0 ] = ';';
                    cmd[ 1 ] = 0;
                    p = cmd;
                }
                else {
                
                    if( strcmp( p, ";//EOF*\n" ) == 0 ) {
                        pop_include();
                        p = ";";
                        printf( "Not a valid include file!\n" );
                        
                    }
                    else
                    if( include-> verify )
                        printf( "[%s]  %s", include-> fname, p );
                }
        }
        else
        if( p == 0L ) {
        
            if( vax.console.assembler_mode ) {
            
                if( vax.assembler.flags & ASM_ADDRPROMPT ) {
                    sprintf( prompt_buff, "[%08X]  ASM", vax.console.deposit );
                    prompt = prompt_buff;
                }
                else
                    prompt = "ASM";
            }
            else
                prompt = "VAX";
                
            printf( "%s> ",prompt );
            p = fgets( cmd, 127, stdin );
        }
        
        if( p == 0L ) {
            commandloop = 0;
            continue;
        }
                    
        uppercase( p );


        //  If there's no command, forget about it.
        
        flush_blanks( &p );
        if( isend( *p )) {
            p = 0L;
            continue;
        }
            
        rc = console( p );
                
        if( rc == VAX_UNHANDLED ) {
            printf( "%%CPU-I-EXCEPTION, last exception code was %s (%04X)\n",
                     vaxexcept(( short ) vax.fault.code) , vax.fault.code );
        }

        //  If we got an error then flush the input file(s) if any.
        
        if(( rc != VAX_OK ) && ( rc != VAX_HALT )) {
            while( include )
                pop_include();
        }
        
        
        p = 0L;
        
    }
    
    //  If at the end of command processing, there's no then it means
    //  there was catastrophic memory failure.  If possible, put out a 
    //  message and we're done.  In this case, we force the user to choose
    //  QUIT from the menu to get out; otherwise he won't see this message.
    
    if( !vax_init ) {
        printf( "%%EMUL-F-NOMEMORY, cannot allocate enough memory to continue\n" );

#ifdef macintosh
        printf( "                  -- choose \"Quit\" from the \"File\" menu to exit.\n" );
        SIOUXSettings.autocloseonquit = 1;
#endif

    }
    
    return 0;
}

#define DEFAULT_SEARCH_DIR "/usr/local/bin/"

FILE * xfopen( char * name, char * mode ) {

    char tmpfn[512];
    char * fnp = name;
    
    FILE * in;
    
    in = fopen(fnp, mode);
    if( in == 0L ) {
        strcpy( tmpfn, DEFAULT_SEARCH_DIR );
        strcat( tmpfn, name );
        fnp = tmpfn;
        in = fopen(fnp, mode);
    }
    return in;
}


/*
 *  POP_INCLUDE()
 *
 *  Close out and free the top-most include file entry on the include stack.
 */

LONGWORD pop_include(void)
{

    struct INCLUDE * next;
    
    if( include == 0L )
        return VAX_OK;
        
    next = include-> next;
    vax.console.assembler_mode = include-> assembler_mode;
    
    fclose( include-> fp );
    freemem( include );

    include = next;
    return VAX_OK;
}


/*
 *  PUSH_INCLUDE()
 *
 *  Push a new include file on the stack, making it the current input
 *  area.
 */

LONGWORD push_include( char * name_string, char * extension )
{

    FILE * f;
    LONGWORD size;
    struct INCLUDE * next;
    char * p;
    LONGWORD n;
    char fname[ 256 ];
    
    
/*
 *  First, find the filename.  If it is enclosed in quoted, toss 'em out.
 */

    p = name_string;
    flush_blanks( &p );
    if( *p == '"' ) {
        p++;
        for( n = 0; p[ n ]; n++ )
            if( p[ n ] == '"' ) {
                p[ n ] = '\0';
                break;
            }
    }

/*
 *  Move the name to a local buffer.  We need to do this since we must be able
 *  to modify the name to handle default extensions.
 */

    strcpy( fname, p );
    
/*
 *  See if we can open the file.  If it fails, try again with an appropriate
 *  extension to see if that works better.
 */
 
    f = xfopen( p, "r" );
    if( f == 0L ) {

        /* Add a default extension and try again */
        
        strcat( fname, extension );       
        f = xfopen( fname, "r" );
        
        /* If it still fails, signal an error */
        
        if( f == 0L ) {
            printf( "%%VAX-E-FNAME, Cannot open file \"%s\"\n", p );
            return VAX_FNF;
        }
    }


/*
 *  Okay, manufacture an include structure for it.
 */
 
    size = sizeof( struct INCLUDE ) + strlen( p ) + 1;
    next = ( struct INCLUDE * ) getmem( size );
    next-> next = include;
    include = next;
    include-> fp = f;
    strcpy( include-> fname, fname );
    include-> assembler_mode = vax.console.assembler_mode;


    return VAX_OK;
}


#ifdef VMS

/*
 *  Here's the VMS-specific implementation of TickCount.  The intent
 *  of TickCount is to return 60'th of a second counts.
 */

LONGWORD TickCount(void)
{
    ULONGWORD ticks, oldticks;
    ULONGWORD tvalue[ 2 ];
    static ULONGWORD base_time = 0L;

    sys$gettim( &tvalue );
    ticks = tvalue[ 0 ] / 1000000;
    ticks = ticks * 6;
//    ticks = ticks / 10;

    if( base_time == 0L ) {
        base_time = ticks;
        return 1;
    }

    ticks = ticks - base_time;

    return ticks;
}
    
#endif

#if !defined(VMS) && !defined(macintosh)

/*
 *  Here's an even more generalized TickCount.  This depends on the MSL
 *  or Unix clock() function.  Hopefully this will work for all Unixes.
 */

LONGWORD TickCount(void)
{

    ULONGWORD count;
    static ULONGWORD baseline = 0L;

    count = clock() * 60;
    count = count / CLOCKS_PER_SEC;

    if( baseline == 0L )
        baseline = count - 1;

    return count - baseline;
}

#endif


/*
 *  Utility memory allocator.  Shell for malloc().
 */

char * getmem( LONGWORD size )
{
    LONGWORD tsize;
    char * p;
#ifdef DEBUG
    LONGWORD n;
#endif
#if defined(VMS)
    LONGWORD sts;
#endif

    
    tsize = size + sizeof( LONGWORD );


/* While not a fatal error, it is a HUGE performance hit to not allocate */
/* memory on longword boundaries on VMS.  So use a runtime that will make */
/* sure it works.                                                         */

#if defined( VMS )
    sts = lib$get_vm( &tsize, &p );
    if( !( sts & 0x00000001 ))
        p = 0L;

#else
    p = malloc( tsize );
#endif

    if( p == 0L ) {
        printf( "FATAL: OUT OF MEMORY\n" );
        return 0L;
    }
    
    total_allocated += tsize;
    count_allocated++;
    
    /* Store the longword at the beginnning of the memory we pulled up. */
    /* Then move the pointer after the storage.  */
    
    
    *( LONGWORD * ) p = tsize;
    p = p + sizeof( LONGWORD );

#ifdef DEBUG
    /*  Stomp on the memory in a way we can recognize */
    
    for( n = 0; n < size; n++ ) 
        p[ n ] = 0xFA;
#endif

    return p;
}



/*
 *  Utility memory free.  Shell for free().
 */

void freemem( void * pt )
{

#ifdef DEBUG
    LONGWORD n;
#endif

    LONGWORD size;
    char * p;
    
    if( pt == 0L )
        return;
        
/*  Back up the pointer so we can recover the length */

    p = pt;
    p = p - sizeof( LONGWORD );
    size = *( LONGWORD * ) p;
    
    total_freed += size;
    count_freed ++;

/*  Stomp on the memory in a way we can recognize later */

#ifdef DEBUG
    for( n = 0; n < size; n++ ) 
        p[ n ] = 0xFD;
#endif


#if defined(VMS)
    lib$free_vm( &size, &p );
#else
    free( p );
#endif

    return;
}

/*
 *  Utility to dump memory issues.
 */

void printmem(void)
{
    
//#ifdef DEBUG
    LONGWORD delta;
    LONGWORD memsize;
//#endif


//#ifdef DEBUG

    if( theVAX == 0L ) 
        memsize = 0L;
    else
        memsize = theVAX->memsize;

    printf( "\nTotal bytes allocated: %-8d  (%d allocations)\n", 
         total_allocated, count_allocated );
    printf(   "Total bytes freed:     %-8d  (%d frees)\n", 
         total_freed, count_freed );

    printf( "\nVAX physical memory:   %-8d\n", memsize );
    
    delta = total_allocated - total_freed;
    delta = delta - memsize;

    printf( "Other memory in use:   %-8d\n", delta );

//#else
//    printf( "SHOW MEMORY STAT command not supported in optimized build.\n" );
//#endif

    return;
}


static long bind_dcl(void)
{

    static int done = 0;
    
    if( done )
        return 0;
    
    done = 1;
    
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_breakpoint",     (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_sym_temp",       (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_mem_stat",       (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_error",          (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_break_all",      (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_sym_all",        (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_interrupt_all",  (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_break_fault_all",(void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_break_fault",    (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_tb",             (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_symbols",        (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_strings",        (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_breakpoint",     (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_interrupt",      (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_memory",         (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "clear_profiles",       (void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX, 	"evax", "clear_break_instr",	(void*) console_clear_dcl   );
    DCLbind( DCL_BIND_SYNTAX, 	"evax", "clear_break_instr_all",(void*) console_clear_dcl   );

    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_string",          (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_nvram",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_error",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_mode",            (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_shim",            (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_page",            (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_call_frames",     (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_quantum",         (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_debug",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_assembler_flags", (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_instructions",    (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_break",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_reg",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_step",            (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_psl",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_cpu",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_base",            (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_memory",          (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_stack",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_isp",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_ksp",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_esp",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_ssp",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_usp",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_stack",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_fault",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_radix",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_trace",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_scb",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_sym",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_rom",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_tb",              (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_map",             (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_sym_all",         (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_sym_sys",         (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_sym_tmp",         (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_sym_unres",       (void*) console_show_dcl    );
    DCLbind( DCL_BIND_VERB,     "evax", "show",                 (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_images",          (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_share",           (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_regions",         (void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,	"evax", "show_expand",		(void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,	"evax",	"show_command_args",	(void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,	"evax",	"show_break_instr",	(void*) console_show_dcl    );
    DCLbind( DCL_BIND_SYNTAX,	"evax", "show_clock",		(void*) console_show_dcl    );
    DCLbind( DCL_BIND_VERB,     "evax", "exit",                 (void*) console_exit_dcl    );
    
    DCLbind( DCL_BIND_VERB,     "evax", "test",                 (void*) console_test_dcl    );

    DCLbind( DCL_BIND_SYNTAX,	"evax",	"define_logical",	(void*) define_logical	    );
    DCLbind( DCL_BIND_SYNTAX,	"evax",	"show_logical",		(void*) show_logical	    );
    DCLbind( DCL_BIND_SYNTAX,	"evax", "define_device",	(void*) define_device	    );
    DCLbind( DCL_BIND_SYNTAX,   "evax", "show_device",          (void*) show_device         );
    DCLbind( DCL_BIND_SYNTAX, 	"evax",	"show_watchpoints",	(void*) console_show_dcl    );
    
    DCLbind( DCL_BIND_VERB,     "evax", "vminit", (void*) console_vminit_dcl );
    
    return VAX_OK;
}
