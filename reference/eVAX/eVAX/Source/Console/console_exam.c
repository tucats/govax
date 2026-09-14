//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     module.c
//
//  Purpose:    This console command displays memory storage in the virtual
//              VAX processor.
//
//  History:    07/28/97    New header format standardization
//
//              02/01/98    Modify the EXAM command to _only_ display the
//                          contents of memory.  Also, extend supported
//                          data types.  Finally, the final parameter is
//                          now an ending address, not a count.
//
//              01/06/99    Allow address(es) to be specified by expressions.
//                          Also fixed incorrect BIGENDIAN usages.
//
//              09/21/99    Make EXAM/INSTR an alias for DISASM for compatability
//                          with the VMS debugger.  Add /AZ, /AD shortcuts.
//
//              11/29/99    Fixed snarky bugs regarding the count of data displayed,
//                          especially for non-scalar types like /AZ.
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "fpu.h"
#include "pte.h"

static struct REGNAMES {
    LONGWORD    name;
    LONGWORD    reg;
    char *      str;
} regnames[] = {
{   CHAR4('R','0',' ',' '),  0, "R0"    },
{   CHAR4('R','1',' ',' '),  1, "R1"    },
{   CHAR4('R','2',' ',' '),  2, "R2"    },
{   CHAR4('R','3',' ',' '),  3, "R3"    },
{   CHAR4('R','4',' ',' '),  4, "R4"    },
{   CHAR4('R','5',' ',' '),  5, "R5"    },
{   CHAR4('R','6',' ',' '),  6, "R6"    },
{   CHAR4('R','7',' ',' '),  7, "R7"    },
{   CHAR4('R','8',' ',' '),  8, "R8"    },
{   CHAR4('R','9',' ',' '),  9, "R9"    },
{   CHAR4('R','1','0',' '), 10, "R10"   },
{   CHAR4('R','1','1',' '), 11, "R11"   },
{   CHAR4('R','1','2',' '), 12, "AP"    },
{   CHAR4('R','1','3',' '), 13, "FP"    },
{   CHAR4('R','1','4',' '), 14, "SP"    },
{   CHAR4('R','1','5',' '), 15, "PC"    },
{   CHAR4('P','C',' ',' '), 15, "PC"    },
{   CHAR4('S','P',' ',' '), 14, "SP"    },
{   CHAR4('F','P',' ',' '), 13, "FP"    },
{   CHAR4('A','P',' ',' '), 12, "AP"    }};



LONGWORD console_exam( char ** P )
{
    LONGWORD rc;
    char * p, *saved_p;
    char * sname;
    
    LONGWORD regverb, verb, size, count, maxbcount, bcount, len, maxlen;
    ULONGWORD value, n;
    LONGWORD reg, is_reg = 0;
    
    LONGWORD cp, d_long, d_long2;
    double dbl;
    
    short d_word;
    unsigned char d_byte;
    char ch, cbuff[ 64 ];
    
    union PTE pte;

/*
 *  Get local addressability to virtual VAX and command buffer
 */
 
    /* vax = *Vax;  -- now use global vax */
    p = *P;
	ch = ' ';
    rc = VAX_OK;
    
/*
 *  If there is no active virtual we are done!
 */
 
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

/*
 *  Set the default length of the output.  If the type causes
 *  this to become zero, we don't allow or parse an end-point.
 */

    count = 0x7FFFFFFF;
    bcount = 0;
    maxbcount = 15;
    sname = 0L;
    size = 0;    
	value = 0;

/*
 *  Look for size/format options that tell how to interpret the
 *  memory.
 */
    
    saved_p = p;
    rc = read_verb( &p, &verb );
    
    if( rc == VAX_OK ) {
    
        switch( verb ) {
        
        case CHAR4('/','I','N','S'):
            
            *P = p;
            return console_disasm( P );
            break;
            
        case CHAR4('/','F','_','F'):
        
            size = 4;
            break;
        
        case CHAR4('/','D','_','F'):
        
            size = 8;
            break;
            
        case CHAR4('/','P','T','E'):
        
            size = 4;
            break;
            
        case CHAR4('/','D','S','C'):
        case CHAR4('/','D','E','S'):
        case CHAR4('/','A','D',' '):
        
            size = 1;
            count = -1;
            verb = CHAR4('/','D','E','S');
            break;
        
        case CHAR4('/','C','O','U'):
        
            size = 1;
            count = -1;
            break;
        
        case CHAR4('/','Z',' ',' '):
        case CHAR4('/','Z','E','R'):
        case CHAR4('/','A','Z',' '):
            size = 1;
            count = -1;
            verb = CHAR4('/','Z','E','R');
            break;
            
        case CHAR4('/','A','S','C'):
        case CHAR4('/','A',' ',' '):
            size = 1;
            count = -1;
            maxbcount = 48; /* Max bytes per line; greater for plain ascii... */
            
            /* Get the ascii type.  If not a valid keyword, assume standard length */
            saved_p = p;
            read_verb( &p, &verb );
            
            if( verb == CHAR4('/','Z','E','R') || verb == CHAR4('/','N','U','L') ||
                verb == CHAR4('/','D','E','S') || verb == CHAR4('/','C','O','U') ) {
                size = 1;
                count = -1;
            }
            else {
                verb = CHAR4('/','A','S','C');
                p = saved_p;
                count = 16;
            }
                
            break;
            
        case CHAR4('/','B','Y','T'):
        case CHAR4('/','B',' ',' '):
            size = 1;
            verb = CHAR4('/','B','Y','T');
            break;
        
        case CHAR4('/','W','O','R'):
        case CHAR4('/','W',' ',' '):
            size = 2;
            verb = CHAR4('/','W','O','R');
            break;
        
        case CHAR4('/','L','O','N'):
        case CHAR4('/','L',' ',' '):
            size = 4;
            verb = CHAR4('/','L','O','N');
            break;

        /* In the default case, assume dumping longs, and back up the token pointer */
        
        default:
            verb = CHAR4('/','L','O','N');
            size = 4;
            p = saved_p;
        }
    }
    
    /*  See if it's a register.  If so, note it */
    
    saved_p = p;
    reg = 0;
    is_reg = 0;
    
    rc = read_verb( &p, &regverb );
    
    //  Search the list of register names to see if it's a match
    
    for( n = 0; n < 20; n++ ) {
        if( regverb == regnames[ n ].name ) {
            is_reg = 1;
            count = 1;
            reg = regnames[ n ].reg;
            break;
        }
    }
    
    //  If it was a match, see if this is the only item on the command
    
    if( is_reg ) {
    
        flush_blanks( &p );
        
        //  If not, then back up and forget it.
        
        if( !isend( *p )) {
            p = saved_p;
            is_reg = 0;
        }
    }
    
    //  If it was not a register name, back up and forget about it
    
    else {
        p = saved_p;
        is_reg = 0;
    }
    
    /*  If complex type (like a counted string, etc.) then registers aren't legal */
    
    if( is_reg ) {
        if( count < 0 ) {
            return VAX_INVREGUSE;
        }
        else {
            value = 0;
            count = 1;
        }
    }
    
    /*  If we don't know it's a register, then get starting address */
 
    if( !is_reg ) {   
        rc = asm_expr( &p, ( LONGWORD * ) &value );
        if( rc )
            goto exit;

        
        /*  Get ending location, if the type hasn't disallowed it */
            
        if( count > 0 ) {
            flush_blanks( &p );
            if( !isend( *p )) {
                rc = asm_expr( &p, ( LONGWORD *) &count );
                if( rc )
                    goto exit;
                count = count - value + 1;
                if( count < 1 ) {
                    rc = VAX_INVENDADDR;
                    goto exit;
                }
            }
        }
    }
/*
 *  If the ending count was never given it's still MAXINT.  If so, then let's
 *  assume we go for one iteration only.
 */

    if( count == 0x7FFFFFFF )
        count = 1;
        
/*
 *  Handle the special cases now.  These are cases where we format
 *  memory as datastructures.  The verb contains the type of the
 *  data structure.  If it's an unknown one, then bail out...
 */
 
    if( count < 0 ) {

        switch( verb ) {
        
        /*  Determine the size as a counted string */
        
        case CHAR4('/','C','O','U'):
        
            rc = load_memory( value, ( void * ) &d_word, 2 );
            if( rc )
                return rc;
            value = value + 2;
            count = d_word;
            verb = CHAR4('/','A','S','C');
            maxbcount = 48;
            sname = "counted ASCII";
            break;
            
            
    
        /*  Determine the size and length from a descriptor */
        
        case CHAR4('/','D','E','S'):
        case CHAR4('/','D','S','C'):
        
            rc = load_memory( value, ( void * ) &d_word, 2 );
            if( rc )
                return rc;
            rc = load_memory( value + 4, ( void * ) &d_long, 4 );
            if( rc )
                return rc;
                
            verb = CHAR4('/','A','S','C');
            maxbcount = 48;
            sname = "descriptor";
            
            value = d_long;
            count = d_word;
            break;
        
        case CHAR4('/','Z','E','R'):
        case CHAR4('/','Z',' ',' '):
        
            for( count = 0; count < 1024; count++ ) {
                rc = load_memory( value + count, ( void * ) &ch, 1 );
                if( rc )
                    return rc;
                    
                if( ch == 0 ) {
                    break;
                }
            }
            verb = CHAR4('/','A','S','C');
            maxbcount = 48;
            sname = "null-terminated ASCII";
            
            break;
            
        default:
            rc = VAX_UNKSTRUCT;
            goto exit;
        }
            
    }
    
    
    len = 0;
    cp = 0;
    cbuff[ 0 ] = 0;
    maxlen = 0;
    bcount = 0;

    if( sname != 0L )
        printf( "Length of %s string is %d (%04X) bytes\n", sname, count, count );
        
    for( n = value; n < /* was <= */ value + count; n += size ) {

        if( len == 0 ) {
        
            if( is_reg )
                printf( "%-4s: ", regnames[ reg ].str );
            else
                printf( "%08X: ", n );
            cp = 0;
        }
            
        switch( verb ) {

        case CHAR4('/','F','_','F'):
        
            if( is_reg )
                d_long = vax.reg[ reg ];
            else {
                rc = load_memory( n, ( void * ) &d_long, 4 );
                if( rc )
                    return rc;
            }
            
            bcount += 4;
            
            rc = fpu_load( d_long, 0L, &dbl );
            if( rc )
                return rc;
            
            printf( "%9.9f\n", dbl );
            break;
        
        case CHAR4('/','D','_','F'):
        
            if( is_reg ) {
                d_long = vax.reg[ reg ];
                d_long2 = vax.reg[ reg+1 ];
            }
            else {
                rc = load_memory( n, ( void * ) &d_long, 4 );
                if( rc )
                    return rc;
                
                rc = load_memory( n + 4, ( void * ) &d_long2, 4 );
                if( rc )
                    return rc;
                }
                
            bcount += 8;
            
            rc = fpu_load( d_long, d_long2, &dbl );
            if( rc )
                return rc;
            
            printf( "%12.9f\n", dbl );
            break;
            
            
        case CHAR4('/','P','T','E'):
        
            if( is_reg )
                pte.longword = vax.reg[ reg ];
            else {
                rc = load_memory( n, ( void * ) &pte.longword, 4 );
                if( rc )
                    return rc;
            }
            bcount += 4;
            
            printf( "%08X   V:%d  PROT: %03d   M: %d  OWN: %02d  PFN: %08X  S:%02d\n",
                    pte.longword,
                    pte.bit.v ? 1 : 0,
                    ( unsigned int ) pte.bit.prot,
                    pte.bit.m ? 1 : 0,
                    ( unsigned int ) pte.bit.own,
                    ( LONGWORD ) pte.bit.pfn << 9,  /* Recalc as address */
                    ( unsigned int ) pte.bit.s );
                    
            break;
            
        case CHAR4('/','A','S','C'):
        
            if( is_reg )
                ch = ( char ) ( vax.reg[ reg ] & 0x0FF );
            else {
                rc = load_memory( n, ( void * ) &ch, 1 );
                if( rc )
                    return rc;
            }
                
            if( ch < ' ' || ( unsigned char ) ch > 127 )
                ch = '.';
                
            printf( "%c",ch );
            len += 1;
            bcount += 1;
            break;
            
        case CHAR4('/','B','Y','T'):
            
            if( is_reg )
                d_byte = ( unsigned char ) ( vax.reg[ reg ] & 0x0FF );
            else {
                rc = load_memory( n, ( void * ) &d_byte, 1 );
                if( rc )
                    return rc;
            }
            
            printf( "%02X ", (LONGWORD ) d_byte );
            cbuff[ cp++ ] = d_byte;
            len += 3;
            bcount += 1;
            break;
                                
        case CHAR4('/','W','O','R'):
        
            if( is_reg ) 
                d_word = ( short ) ( vax.reg[ reg ] & 0x0FFFF );
            else {
                rc = load_memory( n, ( void * ) &d_word, 2 );
                if( rc )
                    return rc;
            }
            
#if 1 /* !BIGENDIAN */

            printf( "%02X%02X ", 
                ( ULONGWORD ) (( d_word & 0xFF00 ) >> 8 ),
                ( ULONGWORD ) (( d_word & 0x00FF ) >> 0 ));
            
            /* Note reverse order from hex display */
            cbuff[ cp++ ] = ( char ) (( d_word & 0x00FF ) >> 0 );
            cbuff[ cp++ ] = ( char ) (( d_word & 0xFF00 ) >> 8 );

#else
            printf( "%02lX%02lX ", 
                ( ULONGWORD ) (( d_word & 0x00FF ) >> 0 ),
                ( ULONGWORD ) (( d_word & 0xFF00 ) >> 8 ));
            
            /* Note reverse order from hex display */
            cbuff[ cp++ ] = (( d_word & 0xFF00 ) >> 8 );
            cbuff[ cp++ ] = (( d_word & 0x00FF ) >> 0 );

#endif

            bcount += 2;
            len += 5;
            break;
            
        case CHAR4('/','L','O','N'):
        
            if( is_reg )
                d_long = vax.reg[ reg ];
            else {
                rc = load_memory( n, ( void * ) &d_long, 4 );
                if( rc )
                    return rc;
            }

#if 1 /*  !BIGENDIAN */

            printf( "%02X%02X%02X%02X ", 
                ( ULONGWORD ) (( d_long & 0xFF000000L ) >> 24 ),
                ( ULONGWORD ) (( d_long & 0x00FF0000L ) >> 16 ),
                ( ULONGWORD ) (( d_long & 0x0000FF00L ) >>  8 ),
                ( ULONGWORD ) (( d_long & 0x000000FFL ) >>  0 ));

            /* Note reverse order from hex display */                           
                cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x000000FFL ) >>  0 );
                cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x0000FF00L ) >>  8 );
                cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x00FF0000L ) >> 16 );
                cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0xFF000000L ) >> 24 );
            

#else
            printf( "%02lX%02lX%02lX%02lX ", 
                ( ULONGWORD ) (( d_long & 0x000000FFL ) >>  0 ),
                ( ULONGWORD ) (( d_long & 0x0000FF00L ) >>  8 ),
                ( ULONGWORD ) (( d_long & 0x00FF0000L ) >> 16 ),
                ( ULONGWORD ) (( d_long & 0xFF000000L ) >> 24 ));

            /* Note reverse order from hex display */                           
            cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0xFF000000L ) >> 24 ),
            cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x00FF0000L ) >> 16 ),
            cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x0000FF00L ) >>  8 ),
            cbuff[ cp++ ] = ( unsigned char ) (( d_long & 0x000000FFL ) >>  0 );
            
#endif

            bcount += 4;
            len += 9;
            break;
        }
        
    /*
     *  See if we've hit the end-of line and need to dump the character data.
     */
     
        if( bcount > maxbcount ) {
        
            maxlen = len;
            for( len = 0; len < cp; len++ ) {
                ch = cbuff[ len ];
                if( ch < ' ' || ( unsigned char ) ch > 127 )
                    cbuff[ len ] = '.';
            }
            cbuff[ cp ] = 0;
            cp = 0;
            len = 0;
            bcount = 0;
            printf( "   %s\n", cbuff );
        }
    }

    if( len > 0 ) {
    
        for( ; len < maxlen; len++ )
            printf( " " );
            
        for( len = 0; len < cp; len++ ) {
            ch = cbuff[ len ];
            if( ch < ' ' || ( unsigned char ) ch > 127 )
                cbuff[ len ] = '.';
        }
        cbuff[ cp ] = 0;
        cp = 0;
        len = 0;
        printf( "   %s\n", cbuff );
    }
    
    printf( "\n" );
            
    
/*
 *  Store parameters back to caller.
 */

exit:

    /* *Vax = vax; -- use global vax instead -- */
    *P = p;
    return rc;
}
