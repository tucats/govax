//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_time.c
//
//  Purpose:    This module implements the TIME command, executes the text that
//              follows it and displays the time it took to execute.  Granularity
//              is currently in 60ths of a second.
//
//  History:    02/11/98        Initial implementation
//
//              09/24/98        Added MIPS rating to output
//              12/14/98        Make profiling conditional (not on VMS)
//

#include "vax.h"
#include "console_proto.h"

#if defined( macintosh ) && defined( PROFILING )
#include <Profiler.h>
#endif



LONGWORD console_time( char ** P )
{
    LONGWORD rc;
    char * p;
    LONGWORD start, end, duration;
    double seconds, mips;
    LONGWORD profiling;
        
    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

/*
 *  See if we are doing profiling.
 */

    read_verb( &p, &profiling );
    if( profiling != CHAR4('/','P','R','O') ) {
        profiling = 0;
        p = *P;
    }
    else
        profiling = 1;
        
/*
 *  If we are profiling, set it up.
 */

    if( profiling ) {
    
#if !defined( macintosh ) || !defined( PROFILING )
        printf( "NOTE: Profiling not supported in this version.\n" );
        profiling = 0;
#else   
        ProfilerInit( collectDetailed, bestTimeBase, 
                    /* Number of routines to track */ 200, 
                    /* Stack depth to scan on entry */  8 );
                    
        ProfilerClear();
#endif
    }

    flush_blanks( &p );
    if( isend( *p )) {
    printf( "TickCount() function returns %d\n", TickCount() );
    return VAX_OK;
    }

    
    vax.console.instruction_count = 0L;
    start = TickCount();
    rc = console_dispatch( p );
    end = TickCount();


#if defined( macintosh ) && defined( PROFILING )
    if( profiling ) {
        ProfilerDump( "\pProfile Data" );
        ProfilerTerm();
    }
#endif


    duration = end - start;
    if( duration < 0L )
        duration = -duration;

    seconds = ( double ) duration / 60.00;
    if( seconds )
        mips = (( double ) vax.console.instruction_count / 1000000.0) / seconds;
    else
        mips = 0.0;
        
    printf( "Elapsed time: %f seconds", seconds );
    if( vax.console.instruction_count )
        printf( ", executed %u instructions (%f MIPS)", 
            vax.console.instruction_count, mips );
    printf( "\n" );
    
    *P = p + strlen( p );
    
    return rc;

}
