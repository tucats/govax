
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_save.c
//
//  Purpose:    This module implements the SAVE command, which writes the current
//              state of the virtual VAX (processor and memory) to a text file in
//              human readable format.  This file can be re-read with the "@"
//              console driver command.
//
//  History:    08/06/97    New header format standardization
//
//              09/23/99    Added SAVE/ROM which saves the stored ROM program in
//                          a binary file format (default name "default.rom").  This
//                          can be used to load a ROM file later.
//
//              09/26/99    Added SAVE/NVRAM which is pretty much like SAVE/ROM.
//
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

#include <time.h>

LONGWORD console_save( char ** P )
{
    LONGWORD rc;
    char * p, * saved_p;
    LONGWORD n, base, verb, nonzero_memory, first;
    FILE * f;
    short zero;
    unsigned char byte;
    struct SYMBOL * sym;
    char buff[ 4 ], cmd[ 80 ];
    extern char * pr_names[];
    LONGWORD saved_mapen, quoted;
    
    time_t lclTime;
    struct tm *now;
    
    lclTime = time( NULL );
    now = localtime( &lclTime );
    strftime( cmd, 80, "%a,  %b %d, %Y  %H:%M %Z", now );

    /* vax = *Vax;  -- now use global vax */
    p = *P;
    
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }
    saved_mapen = vax.MAPEN;


//  If there are unresolved forward references, disallow the
//  save operation.  The saved image would be incomplete.

    for( sym = vax.console.symbols; sym; sym = sym-> next ) {
        if( sym-> forward != 0L ) 
            return VAX_ASMFORWARD;
    }
    
    
//  Do the work.  Start by fetching the filename.
    
    flush_blanks( &p );
    
    saved_p = p;
    read_verb( &p, &verb );
    flush_blanks( &p );

//  Special case; if you use /ROM then you mean to save a ROM image

    if( verb == CHAR4( '/','R','O','M' )) {
        *P = p;
        return save_rom( P );
    }

//  Special case; if you use /NVRAM then you mean to save an NVRAM image

    if( verb == CHAR4( '/','N','V','R' )) {
        *P = p;
        return save_nvram( P );
    }
    
//  Special case; if you don't say SAVE TEXT then you mean binary, and we
//  handle that elsewhere.

    if( verb != CHAR4( '/','T','E','X') && verb != CHAR4('T','E','X','T' )) {
        *P = saved_p;
        return save_binary( P );
    }
    else {
        flush_blanks( &p );
    }

//  If there was never a name on the command, then it's an error.
    
    if( isend( *p ))
        return VAX_INCOMPLETE;

//  If the name is quoted, skip the quote and remember that it was there.       
    if( *p == '\"' ) {
        quoted = 1;
        p++;
    }
    else
        quoted = 0;

//  Find the end of the name, which is either a null, end-of-line, or quote.
    
    for( n = 0; p[ n ]; n++ ) {
    
        if( p[ n ] == '\n' || p[ n ] == 0 || p[ n ] == '\"' )
            p[ n ] = '\0';
    }

//  If the file name was not quoted, there may be trailing blanks to
//  be removed.

    if( !quoted ) 
        for( n = n - 1; p[ n ] == ' '; n-- )
            p[ n ] = '\0';

//  If the file already exists, delete it.  We don't care if this fails
//  or not.

    remove( p );

//  Try to open the file for output.  If it doesn't work, then puke out.
        
    f = xfopen( p, "w" );
    if( f == 0L ) {
        printf( "Can't open file \"%s\"\n", p );
        return VAX_OK;
    }

//  The open succeeded, let the user know we're working on it and then
//  write a nice header to the file.

    printf( "Saving CPU state to file \"%s\"\n", p );
    
    fprintf( f, ";TXT\n;\n" );
    fprintf( f, ";\tSaved VAX state as of %s\n", cmd );
    fprintf( f, ";\n\nINIT ^D%u\n", vax.memsize >> 9 );
    
//  fprintf( f, "SET %sVERIFY\n", vax.console.verify ? "" : "NO" );

    fprintf( f, "SET %sDISASM\n", vax.console.disasm ? "" : "NO" );
    fprintf( f, "SET RADIX HEX\n" );

    
    fprintf( f, "\nASM\n\n" );

//  Save internal registers

    fprintf( f, ";\tNon-zero internal registers\n\n" );
    
    for( n = 0; n < MAXPRIVREG; n++ ) {
    
        if( vax.preg[ n ] )
            fprintf( f, "\t.%-6s\t%08X\n", pr_names[ n ], vax.preg[ n ] );
    }

    write_psl_bits();  /* Make sure bit pattern matches cached copy */
    
    if( vax.psl.reg != 0L )
        fprintf( f, "\t.PSL   \t%08X\n", vax.psl.reg );

//  Save the processor registers

    fprintf( f, "\n;\tNon-zero processor registers\n\n" );
    
    for( n = 0; n < 16; n++ ) {
        if( vax.reg[ n ] == 0L )
            continue;
        fprintf( f, "\t.SET  R%d %08X\n", n, vax.reg[ n ] );
    }

//  Save the nonzero memory locations.  If there aren't any we don't even
//  put out the header comment.

    base = 0; zero = 0; cmd[ 0 ] = 0; nonzero_memory = 0; first = 1;
        
    for( n = 0; (ULONGWORD) n < vax.memsize; n++ ) {
    
        byte = vax.memory[ n ];
        
        if( zero && byte == 0 )
            continue;
        
        if( !zero && byte == 0 ) {
            zero = 1;
            continue;
        }
        
        if( zero && byte != 0 ) {

            if( first ) {
                first = 0;
                nonzero_memory = 1;
                fprintf( f, "\n;\n;\tNon-zero storage\n;\n\n" );
                saved_mapen = vax.MAPEN;
                vax.MAPEN = 0;
                fprintf( f, "\t.MAPEN 0\t\t;\tTemporarily disable virtual memory\n\n" );
            }

            zero = 0;
            if( cmd[ 0 ] ) {
                fprintf( f, "%s\n", cmd );
                cmd[ 0 ] = 0;
            }
            fprintf( f, "\t.BASE  %08X\n", n );
        }
        
        sprintf( buff, "%03X", byte );
        if( cmd[ 0 ] == 0 )
            strcpy( cmd, "\t.BYTE  " );
        else
            strcat( cmd, "," );
        strcat( cmd, buff );
        
        if( strlen( cmd ) > 60 ) {
            fprintf( f, "%s\n", cmd );
            cmd[ 0 ] = 0;
        }
        
    }
    
    if( cmd[ 0 ] ) {
        fprintf( f, "%s\n", cmd );
        cmd[ 0 ] = 0;
    }

    if( nonzero_memory && saved_mapen )
        fprintf( f, "\n\t.MAPEN 1\t\t;\tRe-enable virtual memory\n\n" );
            
    fprintf( f, "\n;\n;\tSet default deposit address\n;\n" );
    fprintf( f, "\n\t.BASE  %08X\n\t.END\n\n", vax.console.deposit );

    if( vax.console.symbols ) {
        fprintf( f, ";\n;\tDefined symbols\n;\n\n" );
        for( sym = vax.console.symbols; sym; sym = sym-> next ) {
            fprintf( f, "SET %s=%08X\n", sym-> name, sym-> value );
        }
        fprintf( f, "\n" );
    }
    
    if( vax.console.radix != 16 ) {
        fprintf( f, "\nSET RADIX %s\n\n", vax.console.radix == 2 ? "BIN" : "DEC" );
    }
    
//  All done, close the file

    fclose( f );
    
    /* *Vax = vax; -- use global vax instead -- */
    
    while(  !isend( **P))
        (*P)++;
        
    return VAX_OK;
}
