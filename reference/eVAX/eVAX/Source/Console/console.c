
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console.c
//
//  Purpose:    This executes console commands for the virtual VAX.  It accepts a
//              null terminated string, and executes the command(s) in the string.
//              The string can contain multiple commands, separated by semicolons.
//
//              This is the dispatch point for all console commands except the "@"
//              operator, which is really a driver function.
//
//
//  History:    07/28/97    New header format standardization
//
//              01/18/99    If we finish up a command and the ASM_ENTRY
//                          flag has been set, it means we just finished
//                          assembling something with a .END directive.
//                          In that case, initiate a CALL __ENTRY to be
//                          invoked automatically.
//
//                          Also, broke out the operation to push a new
//                          command on the stack as push_command(), since
//                          it's used several places now.
//
//              02/01/99    Handle SET DEBUG dropping us into a native debugger.
//
//		07/13/01    Corrected problem with attention not working correctly
//			    because it was updating the wrong instance of the VAX
//			    control structures.
//
//              03/29/02    VMS didn't handle the TODR signal the same as other POSIX
//                          implementations; added VMS-specific code to reinstate the
//                          signal() each time the todr timer is popped.
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

#include <stdlib.h>
#include <stdio.h>
#include <signal.h>
#include <unistd.h>

#if defined(VMS)
#include <lib$routines>      // Needed for SET DEBUG support
#include <ssdef>
#endif

LONGWORD invoke_debugger( void );

/*
 *  This data structure defines the command stack.  This is used by commands that
 *  need to be broken up (containing ";" separators) and by commands that need to
 *  generate execution of other commands (which are pushed on the stack and then
 *  executed.
 */

struct CMD {
    struct CMD *next;
    char cmd[1];
};

static struct CMD * command_stack = 0L;
LONGWORD console_dispatch( char * cmd );
void attention( int code );
void todr_timer( int code );


/*
 *  Main entry point for a single console command.
 */

LONGWORD console( char * cmd )
{
    LONGWORD n, len, eos, rc;
    char * p;
    struct CMD * cp;
    char comment, separator, qf;

 

    eos = 0;
    p = cmd;
    len = strlen( cmd );
    rc = VAX_OK;
    /* vax = *Vax;  -- now use global vax */

    signal( SIGINT, attention );
    signal( SIGALRM, todr_timer );
    ualarm( 100, 100 );   /* Timer pop every 10ms? */
    
    
    separator = vax.console.separator;
    comment = vax.console.comment;
    
        
//  Scan the string forwards to find the end.  The end can be either
//  a null terminator or a comment.

    for( n = 0; n < len; n++ ) {
        if( cmd[ n ] == 0 )
            break;
        if( /* cmd[ n ] == comment || */ cmd[ n ] == '\r' || cmd[ n ] == '\n' ) {
            cmd[ n ] = '\0';
            n--;
            break;
        }
    }
    

//  Scan the string backwards, picking off commands in reverse order.

    qf = 0;
    for( n = len; n >= 0; n-- ) {

        //  If this is a command boundary...
        
        if( cmd[ n ] == '"' )
            qf = ( char ) ( 1 - qf );
            
        if( n == 0 || ( !qf && cmd[ n ] == separator )) {

            //  Make sure preceding string is null terminated, and mark this start.
            
            if( n == 0 )
                p = cmd;
            else {
                cmd[ n ] = 0;
                p  = &( cmd[ n + 1 ]);
            }

            rc = push_command( p );
                        if( rc )
                            break;
            
        }
    }
    

//  Now we run the stack, in reverse order to how we created it, until there
//  are no more commands on the stack.  As LONGWORD as no errors occur, we 
//  execute each command we find.
    
    while( command_stack ) {
        
        //  If no error yet, execute the next command
            
        if( rc == VAX_OK || rc == VAX_HALT )
            rc = console_dispatch( command_stack-> cmd );

        //  If we got an unexpected return code, print it out.
                
        if( rc != VAX_OK && rc != VAX_NORMAL ) {
            printf( "%s\n", vaxmsg( rc ));
            set_symbol_direct("$STATUS", rc, 0 );
        }
            

        //  If at this point we're supposed to hit the native debugger, do it.

        if( vax.debug & DBG_DEBUG ) {
            vax.debug &= ~DBG_DEBUG;   /* Turn the flag off */
            invoke_debugger();
        }


        //  Either way, pop the stack
        
        cp = command_stack-> next;
        freemem( command_stack );
        command_stack = cp;

        //  If this triggered a call to __ENTRY, set it up.

        if( vax.assembler.flags & ASM_ENTRY ) {
            vax.assembler.flags &= ~ASM_ENTRY;
                        push_command( "CALL __ENTRY" );
        }
    }
    

//  All done, return the last return code we got.

    return rc;
}


/*
 *  Push a command on the stack.
 */

LONGWORD push_command( char * p )
{

    LONGWORD size;
    struct CMD * cp;
            
//  Allocate a command stack entry of the right size
            
    size = strlen( p ) + 1;
    cp = ( struct CMD * ) getmem( sizeof( struct CMD ) + size );

    if( cp == 0L )
        return VAX_MEM;
            
//  Link this at head of the stack
            
    cp-> next = command_stack;
    command_stack = cp;
    flush_blanks( &p );
    strcpy( cp-> cmd, p );
    return VAX_OK;
}



/*
 *  Invoke the native debugger.
 */

LONGWORD invoke_debugger( void )
{

    vax.in_debugger = 1;
#if defined(VMS) 
    lib$signal( SS$_DEBUG );
#else
#if defined(macintosh)
    Debugger();
#else
    printf( "SET DEBUG not supported in this port\n" );
#endif
#endif

    vax.in_debugger = 0;

    return VAX_OK;
}

void todr_timer( int code )
{
    
    if( vax.clock_running && vax.halted == 0 )	/* ICCS<0> and running */
        vax.clock++;
        
    if( vax.quantum.current )
        vax.quantum.current--;

/* VMS requires that the signal be explicity re-instated */

#ifdef VMS
    signal( SIGALRM, todr_timer );
#endif


}


void attention( int code )
{

	int dummy = code;
	int y;

	y = dummy;

    vax.halted = VAX_ATTENTION;
    return;
}
