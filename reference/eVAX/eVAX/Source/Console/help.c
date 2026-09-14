//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     help.c
//
//  Purpose:    This handles the HELP command in the console. It parses tokens
//              as needed and constructs a "key" that is used to locate the help
//              text in the VAX.HELP file.  The format for this file is documented
//              at the top of the file itself (before any keys).
//
//  History:    07/28/97    New header format standardization
//
//              02/03/99    Intel byteswaps multibyte character constants,
//                          so the values from read_verb() must be inverted
//                          (again) to match the text keys in the help file.
//
//                          Also, change the name of the help file to be lower
//                          case.  VMS, PC, and Mac don't care, but Unix
//                          does, and tends to use lower case names by default.
//
//              06/15/99    INVERTEDLONGCHAR was set wrong for Windows, at least
//                          as developed under VSS.  


#include "vax.h"


static void normalize_verb( char * p );


/*
 *  Parse command tokens to form a help key and display it.
 */

LONGWORD help( char ** P )
{

    char * p = *P;
    LONGWORD rc, verb, text_displayed;
    char b[ 32 ];
    char d[ 128 ];
    LONGWORD n;
    FILE * f;
    
    n = 1;
    rc = VAX_OK;

    b[ 0 ] = '$';
    b[ 1 ] = 0;
    /* vax = *Vax;  -- now use global vax */
    if( !vax_init )
        return VAX_NOVAX;

    while( !isend( *p )) {
    
        flush_blanks( &p );    

        if( isend( *p ))
            break;
        
        read_verb( &p, &verb );
        normalize_verb(( char * ) &verb );    

        if( b[ 1 ] != 0 )
            b[ n++ ] = ',';

/*
 *  If we dont' care about alignment we can generate this directly.
 */

#if defined( macintosh ) || defined( VMS )
        *( LONGWORD * ) &( b[ n ]) = verb;

#else
/*
 *  Generate a more costly move.
 */

        memcpy( &( b[ n ]), &verb, 4 );
#endif

        b[ n + 4 ] = 0;
        n = n + 4;
    }
    
    if( b[ 1 ] == 0 )
        strcpy( b, "$HELP" );
    
    strcat( b, "\n" );
    
    *P = p;
    
    f = xfopen( "vax.help", "r" );
    if( f == 0L )
        return VAX_NOHELPFILE;
    
    text_displayed = 0;
    printf( "\n" );
    p = ( char * ) 1;

    while( p != 0L ) {
    
        p = fgets(  d, 255, f );
        if( p == 0L ) {
            rc = VAX_NOHELP;
            break;
        }
        
        if( *p == '$' ) {
    
            if( strcmp( b, p ) == 0 ) {

                            //  Found a match, read the help text
                                
                while( p ) {
                    p = fgets(  d, 255, f );
                    if( p == 0L )
                        break;
                    if( *p == '$' ) {
                        if( text_displayed )
                            break;
                        else
                            continue;
                    }
                    
                    text_displayed = 1;
                    printf( "%s", p );
                    
                }
                rc = VAX_OK;
                break;      //  All done, close 'er up
            }
            
        }
        
    }
    
    fclose( f );
    
    return rc;
}

/*
 *  Routine used to normalize a verb, so it matches the way that we
 *  have to parse on INTEL platforms, where a character constant is
 *  stored in reversed order.
 */

void normalize_verb( char * p )
{

	char ch;

	ch = *p;

#if INVERTEDLONGCHAR

    char ch;
    ch = p[ 0 ]; p[ 0 ] = p[ 3 ]; p[ 3 ] = ch;
    ch = p[ 1 ]; p[ 1 ] = p[ 2 ]; p[ 2 ] = ch;

#endif
    return;
}

