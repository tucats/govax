//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_label.c
//
//  Purpose:    Pseudoassembler for the console interface to the VAX.
//
//
//  History:    02/01/98    Created by removing from asm.c
//
//                      12/28/98        Mark symbols we create as LABEL
//

#include "vax.h"
#include "vaxinstr.h"

//  Local prototypes

#include "asmproto.h"



//  See if there's a label on the line.  If so, then handle it and step past it.

LONGWORD asm_label( char ** P )
{

    char * p = *P;
    LONGWORD n, rc;
    int symflags;
    
    char b[ 32 ], *bp, ch;
    
    flush_blanks( &p );
    
    for( n = 0; p[ n ] != 0; n++ ) {
    
        if( is_blank( p[ n ]) || isend( p[ n ]))
            return VAX_OK;
        
        if( p[ n ] == ':' ) {
            break;
        }
        
        ch = p[ n ];
        b[ n ] = ch;
    }
    
    if( p[ n ] == 0 )
        return VAX_OK;
        
    b[ n ] = 0;
    bp = b;
    

    symflags = SYM_LABEL;
    
    /* If it's a "::" label it must be a global symbol, make it permanent */
    
    if( p[ n ] == ':' && p[ n+1 ] == ':' ) {
        n++;
        symflags |= SYM_PERMANENT;
    }
    
    vax.assembler.flags |= ASM_UNIQUE;    
    rc = set_symbol( &bp, vax.console.deposit, symflags );
 
    
    *P = &( p[ n + 1 ]);
    return rc;
}
