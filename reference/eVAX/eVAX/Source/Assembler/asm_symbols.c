//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_symbols.c
//
//  Purpose:    Symbol table handler for the assembler.  Note that these symbols
//              are shared by the console interface for memory references and
//              the EXECUTE command.
//
//  History:    07/28/97    New header format standardization
//
//              01/30/98    Added support for forward referenced symbols.  The dump symbols
//                          command was updated to display forward reference information.  The
//                          get_symbol detects special cases of a valid forward reference,
//                          called from get_value() and generates forward reference lists.
//                          set_symbol() knows to dissolve forward reference lists when a
//                          concrete value is known.
//
//              02/01/98    Added ability to use check_unresolved_symbols() as tool for checking
//                          for one or more unresolved symbols.  We don't let the RUN command
//                          operate if there are unresolved symbols.
//
//              12/28/98    When dumping symbols, show the flag values, such as
//                          SYM_ENTRY or SYM_LABEL.
//
//              01/11/99    Added ability to support temp symbols.  These are
//                          scoped within an entry area, and are fixed by
//                          prefixing with the entry name when the assembly
//                          unit finishes.
//
//              01/18/99    Check for illegal duplicate definition of a
//                          symbol, controlled by the label parser and the
//                          .ENTRY handler
//
//              05/25/99    Add ability to clear non-permanent system symbols.  This is useful
//                          when you VMINIT or ZERO memory, so you can invalidate the symbols
//                          in the microkernel.
//
//              10/15/99    Added support for clearing non-permanent user symbols, 
//                          which is also supported by CLEAR TEMP SYMB console command.
//
//              04/27/01    Added ability to display SYM_STRING values


#include "vax.h"
#include "asmproto.h"

#ifdef WIN
#include <string.h>
#else
#include <strings.h>
#endif

#include <ctype.h>


//  Local routine that creates an empty symbol table node, shared by both
//  get symbol and set symbol.

static struct SYMBOL * make_symbol( char * sname );


//  This is a static sequence number that is used for unscoped temporary
//  variables.  The scope_name() routine uses this to generate scoped
//  names when name scoping is turned off.

static LONGWORD temp_seq = 0;
LONGWORD scope_name( char * in_name, char * out_name );


//  Check for unresolved symbols.  If the flag is set, then also report them to the
//  console.  If the flag is clear then just return a flag if there are any.

LONGWORD check_unresolved_symbols( int flag )
{

    struct SYMBOL * p;

//  If we are to report them, call down to dump_symbols.

    if( flag )
        return dump_symbols( flag );

//  Otherwise, search for unresolved symbols.

    for( p = vax.console.symbols; p; p = p-> next )
        if( p-> forward )
            return VAX_ASMUNDSYMS;

    for( p = system_symbols; p; p = p-> next )
        if( p-> forward )
            return VAX_ASMUNDSYMS;


    return VAX_OK;
}

//  Dump system symbol table.  These are symbols that can be used by
//  assembly code, etc. but are read-only and defined during startup.


LONGWORD dump_system_symbols(void)
{
    struct SYMBOL * p;
    char attr[ 256 ], *cp;

    printf( "\tSystem Symbols:\n" );

    for( p = system_symbols; p; p = p-> next ) {

        strcpy( attr, "sys " );

        if( p-> flags & SYM_PERMANENT )
            strcat( attr, "prm " );
        if( p-> flags & SYM_ENTRY )
            strcat( attr, "ent " );
        if( p-> flags & SYM_LABEL )
            strcat( attr, "lbl " );
        if( p-> flags & SYM_LOCAL )
            strcat( attr, "lcl " );

        if( p-> flags & SYM_STRING ) {
             cp = p-> svalue;
             strcat( attr, "str=\"" );
             strcat( attr, cp );
             strcat( attr, "\" " );
        }
        
        printf( "    %-26s  %08X (hex) %12d (dec) %s\n",
           p-> name, p-> value, p-> value, attr );
                           
        if( p-> forward )
            dump_forward_references( p );

    }

    return VAX_OK;

}



//  Dump symbol table.  The flag tells us if we are doing a SHOW ALL SYMBOL command or
//  if we are showing unresolved symbols.

LONGWORD dump_symbols( int flag )
{

    struct SYMBOL * p;
    LONGWORD found;
    char flagbuff[ 256 ], *cp;

    found = 0;
    
    if( vax.console.symbols ) {
        
        if( flag == 0 )
            printf( "    Symbols:\n" );
        
        for( p = vax.console.symbols; p; p = p-> next ) {
        
        /* Test for unresolved references to this symbol */
        
            if( p-> forward == 0L ) {
            
            /* No unresolved... if we are printing everything, do it... */
                    
                if( flag == 0 ) {
                    flagbuff[ 0 ] = 0;
                    if( p-> flags & SYM_ENTRY )
                        strcat( flagbuff, " ent" );
                    if( p-> flags & SYM_LABEL )
                        strcat( flagbuff, " lbl" );
                    if( p-> flags & SYM_LOCAL )
                        strcat( flagbuff, " lcl" );
                    if( p-> flags & SYM_PERMANENT )
                        strcat( flagbuff, " prm" );
                    if( p-> flags & SYM_STRING ) {
                        cp = p-> svalue;
                        strcat( flagbuff, "str=\"" );
                        strcat( flagbuff, cp );
                        strcat( flagbuff, "\" " );
                    }
            
                    printf( "    %-26s  %08X (hex) %12d (dec) %s\n", 
                      p-> name, p-> value, p-> value, flagbuff );
                }
            }
            
            /* Unresolved, so we always print those... */
                
            else {
            
                if( flag != 0 && found == 0 ) {
                    found = 1;
                    printf( "Unresolved symbols:\n" );
                }
                
                dump_forward_references( p );
                                
            }
        }
    }

    /* If no symbols at all, then print that if we were dumping symbols */
    
    else
        if( flag == 0 )
            printf( "\tNo symbols defined\n" );

    /* If this was for unresolved symbols, do the same for the system symbols */
    
    if( flag && system_symbols ) {
        
        for( p = system_symbols; p; p = p-> next ) {
        
        /* Test for unresolved references to this symbol */
        
            if( p-> forward ) {
                        
                if( flag != 0 && found == 0 ) {
                    found = 1;
                    printf( "Unresolved symbols:\n" );
                }
                
                dump_forward_references( p );
                                
            }
        }
    }

    /* If we were checking for unresolved, and there were some return error */
    
    if( flag != 0 && found )
        return VAX_ASMUNDSYMS;
    else
        return VAX_OK;
}




//  Dump forward references for a symbol.  Returns number of forward references found.
//  If the return is zero, no output has occurred. 

LONGWORD dump_forward_references( struct SYMBOL * p )
{

    struct FSYMBOL * fp;
    LONGWORD count;
    int dispm;
    char dname[ 12 ];
    static char * sname = "0bw3l567q9ABCDEF";

    if( !vax_init )
        return 0L;

    if( p-> forward == 0L )
        return 0L;
            
    for( count = 0, fp = p-> forward; fp; fp = fp-> next )
        count++;
        
    printf( "\t%-32s    %d unresolved forward reference%s\n", 
        p-> name, count, ( count == 1 ) ? "" : "s" );
        
    for( fp = p-> forward; fp; fp = fp-> next ) {
    
        dispm = (int) fp-> displacement;
        
        switch( dispm & K_ADDR_MODE ) {

        case K_CASE_BASE:   strcpy( dname, "case." );   break;        
        case K_ADDR_BASE:   strcpy( dname, " abs." );   break;
        case K_DISP_BASE:   strcpy( dname, "disp." );   break;
        case K_BRANCH_BASE: strcpy( dname, "  br." );   break;
        
        default: sprintf( dname, "[%2d].", ( int ) dispm & K_ADDR_MODE ); break;
        
        }
        
    /* This code depends on the length(s) of the mode strings above */
    /* all being of equal and constant length!                      */
    
        dname[ 5 ] = sname[ dispm & K_ADDR_SIZE ];
        dname[ 6 ] = '\0';              
        
        
        printf( "\t\t%-8s at %08X\n", dname, fp-> location );
    }
    printf( "\n" ); /* After forward reference list */

    return count;
    
}

LONGWORD get_symbol_direct( char * Symbol, LONGWORD * value )
{

    char sn[ 65 ], * sp;
    strncpy( sn, Symbol, 63 );
    sn[ 64 ] =0;
    sp = sn;
    
    return get_symbol( &sp, value );
}


//  Get a symbol value, where the smbol value is pointed to by the Symbol pointer/pointer.
//  This may the input stream, or a symbolic value acquired elsewhere.  As such, leading
//  blanks, an optional "quote" mark, etc. are stripped, and the value case-normalized.
//  The 'fref' flag tells if we allow forward references or not.  If so, we always get a
//  value back (0) but a forward reference is stored for it.

LONGWORD get_symbol( char ** Symbol, LONGWORD * value )
{

    struct SYMBOL * p;
    char * symbol, *cp;
    char sname[ 255 ];
    LONGWORD qflag, n;
    struct FSYMBOL * fp;
    LONGWORD fref, found, was_scoped;
    
//     if( !vax_init )
//         return VAX_NOVAX;


/*
 *  Get the forward reference flag value.  Note that if the displacement flag
 *  is set to K_NOFORWARD we disable support for forward references.
 */
 
    fref = vax.assembler.location;
    if( vax.assembler.displacement == K_NOFORWARD )
        fref = 0L;
        
    symbol = *Symbol;   
    cp = symbol;

/*  By default, we assume no forward reference resulted. */

/*  vax.assembler.was_forward = 0; */

    flush_blanks( &cp );

//  Common mistake is to create a symbol d^ instead of ^d for
//  decimal, etc.  Check for these cases, since there is (currently)
//  no valid syntax for D^, etc.

    if( cp[ 1 ] == '^' && 
       ( cp[ 0 ] == 'D' ||
         cp[ 0 ] == 'X' ||
         cp[ 0 ] == 'M' ))
            return VAX_ASMINVCONST;

//  Check to see if this is quoted (archaic syntax).  If so, then strip off
//  the quote character and remember to look for the closing instance.

    if( *cp == '\'' ) {
        qflag = 1;
        cp++;
    }
    else
        qflag = 0;

    for( n = 0; n < 32; n++ ) {
    
        sname[ n ] = *cp;
        if( *cp == '\'' ) {
            cp++;
            break;
        }
            
        if(*cp != '_' && *cp != '$' &&  !isalnum( *cp ))
            break;
            
        if( is_blank( *cp ) || isend( *cp ))
            break;
        cp++;
    }
    
    sname[ n ] = '\0';
    *Symbol = cp;

#ifdef DEBUG

    if( vax.debug & DBG_SYMBOLS ) {
        printf( "DEBUG(SYMBOLS): Lookup symbol %s\n", sname );
    }
#endif

    was_scoped = scope_name( sname, sname );
    
    vax.console.last_symbol = 0L;
    found = 0;
    
    for( p = vax.console.symbols; p; p = p-> next ) 
        if( strcmp( sname, p-> name ) == 0 ) {
            vax.console.last_symbol = p;
            *value = p-> value;
            found = 1;
            break;
        }

//  There's also a table of "predefined" symbols we should check as well.

    if( !found )
        for( p = system_symbols; p; p = p-> next ) {
            if( strcmp( sname, p-> name ) == 0 ) {
                vax.console.last_symbol = p;
                *value = p-> value;
                found = 2;
                break;
            }
        }

//  If it was found and scoped, make sure the local flag is set

    if( found && was_scoped )
        p-> flags |= SYM_LOCAL;

//  If it was found and we dont' worry about forward references we're done.

    if( found && !fref )
        return VAX_OK;
        
//  If not found and can't be forward referenced, then we fail.

    if( !found && !fref  )
        return VAX_ASMUNDSYM;

//  If the symbol is resolved (no pending forward references), then we're done.

    if( found && p-> forward == 0L )
        return VAX_OK;

    
//  If it doesn't exist at all, make a symbol.

    if( p == 0L ) {
        p = make_symbol( sname );
        *value = 0L;
        if( p == 0L )
            return VAX_MEM;
    }
    
//  Create a forward reference that points to the memory space in "fref"

    fp = ( struct FSYMBOL * ) getmem( sizeof( struct FSYMBOL ));
    if( fp == 0L )
        return VAX_MEM;
    
    /* Forward references *MUST* be put on the head of the chain */
    /* for the assembler DISP(R3) syntax to work (it assumes it  */
    /* can modify the forward reference as the first item on the */
    /* chain after assembling the displacement.                  */
    
    fp-> next = p-> forward;
    p-> forward = fp;
    
    fp-> location = fref;
    fp-> displacement = vax.assembler.displacement;

//  If the mode is a case block, then store away the case base for 
//  this forward reference. 

    if( fp-> displacement == K_CASE_W )
        p-> value = vax.assembler.case_base;

//  Indicate that we had a forward reference

    vax.assembler.was_forward += 1;
    
    return VAX_OK;
}



//  Create a symbol node

struct SYMBOL * make_symbol( char * sname )
{

    struct SYMBOL * p, *last, *sym;
    LONGWORD size, n, len, system;
    char localname[ 256 ];
    LONGWORD was_scoped;

/*
 *  First, see if this is a "system symbol" which contains a $ character.
 */

    
    system = 0;
    len = strlen( sname );
    for( n = 0; n < len; n++ )
        if( sname[ n ] == '$' )
            system = 1;

/*
 *  See if it's a scoped name.  If so, then let's make it explicit.
 */

    was_scoped = scope_name( sname, localname );
    len = strlen( localname );

#ifdef DEBUG

    if( vax.debug & DBG_SYMBOLS  ) {
        printf( "DEBUG(SYMBOLS): creating %s symbol %s\n",
           system ? "system" : "user", sname );
    }
#endif

    size = len + sizeof( struct SYMBOL );
    p = ( struct SYMBOL * ) getmem( size );
    if( p == 0L ) 
        return p;
        
    if( system )  {
        last = 0L;
        for( sym = system_symbols; sym; sym = sym-> next ) {
            if( strcmp( sym-> name, sname ) < 0 ) {
                last = sym;
                continue;
            }
            break;
        }

        if( last == 0L ) {
            p-> next = system_symbols;
            system_symbols = p;
        }
        else {
            p-> next = last-> next;
            last-> next = p;
        }
    }
    else {
        last = 0L;
        for( sym = vax.console.symbols; sym; sym = sym-> next ) {
            if( strcmp( sym-> name, sname ) < 0 ) {
                last = sym;
                continue;
            }
            break;
        }

        if( last == 0L ) {
            p-> next = vax.console.symbols;
            vax.console.symbols = p;
        }
        else {
            p-> next = last-> next;
            last-> next = p;
        }
    }
    
    p-> forward = 0L;
    vax.console.last_symbol = p;
    strcpy( p-> name, sname );
    p-> value = 0L;
    p-> svalue = 0L;
    p-> flags = was_scoped ? SYM_LOCAL : SYM_NONE;
    if( system )
        p-> flags |= SYM_SYSTEM;


    return p;
    
}


LONGWORD set_symbol_direct( char * symbol, LONGWORD value, int flags )
{

    char buff[ 256 ], *p;

    strcpy( buff, symbol );
    p = buff;

    return set_symbol( &p, value, flags );
}

//  Set a SYM_STRING symbol's value.  This is a native host string pointer,
//  not a VAX-facing numeric value, so it's kept in struct SYMBOL's `svalue`
//  field rather than truncated through the 32-bit `value` LONGWORD (see
//  AUDIT.md N1).  Reuses set_symbol_direct() for name lookup/creation, then
//  fills in svalue on the resulting symbol.

LONGWORD set_symbol_string_direct( char * symbol, char * strvalue, int flags )
{

    LONGWORD rc;

    rc = set_symbol_direct( symbol, 0L, flags );
    if( rc == VAX_OK )
        vax.console.last_symbol-> svalue = strvalue;

    return rc;
}

//  Set a symbol value


LONGWORD set_symbol( char ** Symbol, LONGWORD value, int flags )
{
    
    LONGWORD v, rc;
    
    LONGWORD disp, disp_mode, ivalue, was_scoped;
    short dispw;
    char dispb;
    
    char * p, *cp, *symbol;
    struct FSYMBOL * fp, *fnext;
    
    char sname[ 255 ];
    LONGWORD qflag, n;

    symbol = *Symbol;   
    cp = symbol;

    flush_blanks( &cp );
    
    if( *cp == '\'' ) {
        qflag = 1;
        cp++;
    }
    else
        qflag = 0;

    for( n = 0; n < 32; n++ ) {
    
        sname[ n ] = *cp;
        if( *cp == '\'' ) {
            cp++;
            break;
        }
        
        if( *cp != '_' && *cp != '$' && !isalnum( *cp ))
            break;
            
        if( is_blank( *cp ) || isend( *cp ))
            break;
        cp++;
    }
    
    sname[ n ] = '\0';
    *Symbol = cp;

    was_scoped = scope_name( sname, sname );    
    p = sname;
    rc = get_symbol( &p, &v );
    if( rc == VAX_OK && was_scoped ) 
        vax.console.last_symbol -> flags |= SYM_LOCAL;

//  If the symbol was found, but we must create a unique one (usually when
//  a label or entry is defined) then fail.  We don't fail if the symbol
//  already existed but has forward references, though, since that means
//  we are defining it now.

    if( vax.assembler.flags & ASM_UNIQUE ) {
        vax.assembler.flags &= ~ASM_UNIQUE;

        if( rc == VAX_OK && ( vax.console.last_symbol-> forward == 0L ))
            return VAX_ASMDUPSYM;
    }

//  If the symbol wasn't found, then try to create it

    if( rc == VAX_ASMUNDSYM ) {
        if( make_symbol( sname ) == 0L )   // Make a symbol -- if it fails...
            return VAX_MEM;                 // ... we're out of memory.
    }

//  Save the old value in case we forward reference something that needs
//  it (like a K_CASE_W symbol).  Then set the value.  Also, add in any
//  flags passed into this call.

    ivalue = vax.console.last_symbol-> value;
    vax.console.last_symbol-> value = value;
    vax.console.last_symbol-> flags = vax.console.last_symbol->flags | flags;
    
//  If this was a forward referenced symbol that we now have a value for, then
//  let's allow that...

    for( fp = vax.console.last_symbol-> forward; fp; fp = fnext ) {
    
        fnext = fp-> next;
        disp = value - fp-> location;
        disp_mode = fp-> displacement;
        
        /* If it's branch mode, we subtract the size of the value from the displacement */
        /* since branch displacements are from the byte _following_ the displacement data */
        
        if(( disp_mode & K_ADDR_MODE ) == K_BRANCH_BASE )
            disp -= ( disp_mode & K_ADDR_SIZE );
            
        /* Note that in all these cases, the ADDR switch comes first and falls */
        /* through to the DISP case... be careful in ordering these cases!!    */
        
        switch( fp-> displacement ) {
        
        case K_CASE_W:

    /*  In this case, the displacement we'll write is the offset from */
    /*  the value we have less the base of the case block, rather than */
    /*  the location of the forward reference itself.  */

            disp = value - ivalue;
            if( disp < -32768 || disp > 32767 ) 
                return VAX_ASMFDISPWORD;
            
            dispw = ( short ) disp;
            store_memory( fp-> location, ( void * ) &dispw, 2 );
            break;
            
        case K_ADDR_B:
        
            disp = ( char ) value;
            
            /* Fall through to displacement case! */
            
        case K_DISP_B:
        case K_BRANCH_B:
        
            if( disp < -128 || disp > 127 ) 
                return VAX_ASMFDISPBYTE;
            
            dispb = ( char ) disp;
            store_memory( fp-> location, ( void * ) &dispb, 1 );
            break;

        case K_ADDR_W:
        
            disp = value;
            
        case K_DISP_W:
        case K_BRANCH_W:
        
            if( disp < -32768 || disp > 32767 ) 
                return VAX_ASMFDISPWORD;
            
            dispw = ( short ) disp;
            store_memory( fp-> location, ( void * ) &dispw, 2 );
            break;

        case K_ADDR_L:
        
            disp = value;
            
        case K_DISP_L:
        case K_BRANCH_L:
        
            store_memory( fp-> location, ( void * ) &disp, 4 );
            break;
        }
        
        freemem( fp ); // All done with forward reference now!
    }
    
    vax.console.last_symbol-> forward = 0L;
    
    return VAX_OK;
}

//  Clear non-permanent symbols from the user symbol list.

LONGWORD clear_temp_symbols( void )
{
	int running;

    struct SYMBOL * p;
    
	running = 1;
    while( running ) {
    
        for( p = vax.console.symbols; p; p = p-> next  ) {
            if( !( p-> flags & SYM_PERMANENT ))
                break;
        }
        
        if( p == 0 )
            break;
        
        clear_symbol( p-> name );
    }
    
    return VAX_OK;
}


//  Clear a single symbol from the symbol table

LONGWORD clear_symbol( char * symbol )
{

    struct SYMBOL * p, * last;
    struct FSYMBOL *fp, *fnext;

    LONGWORD found;
    
    last = 0L;
    found = 0;
    
    for( p = vax.console.symbols; p; p = p-> next ) {
        if( strcmp( symbol, p-> name ) == 0 ) {
            found = 1;
            break;
        }
        last = p;
    }
    
    if( !found )
        return VAX_ASMUNDSYM;
    
    if( last == 0L )
        vax.console.symbols = p-> next;
    else
        last-> next = p-> next;
    
    // Blow off any dangling forward references
    
    for( fp = p-> forward; fp != 0L; fp = fnext ) {
        fnext = fp-> next;
        freemem( fp );
    }
    
    freemem( p );
    return VAX_OK;
}

//  Clear the user temporary symbols from the table.  The symbols
//  must be resolved or an error occurs.  This is called when a 
//  .entry is found in the assembler.  It frees up the name space
//  for symbols.


LONGWORD scope_symbols( void )
{
    LONGWORD rc;
    struct SYMBOL * p;


    rc = VAX_OK;

/*  Scan to see if there are unresolved forward refs.  We may or */
/*  may not care about this, of course.                          */


    if( vax.assembler.flags & ASM_RESTMP ) {
        for( p = vax.console.symbols; p; p = p-> next ) {
            if(( p-> name[ 0 ] == '_' ) && ( p-> name[ 1 ] != '_' )) {
                if( p-> forward ) {

                    printf( "%%ASM-E-UNRESTMP, unresolved symbol %s\n",
                          p-> name );
                    rc = VAX_ASMUNRESTMP;
                }
            }
        }
    }
    if( rc )
        return rc;

    vax.assembler.cur_entry[ 0 ] = 0;

#ifdef DEBUG

    if( vax.debug & DBG_SYMBOLS  ) {
        if( vax.assembler.cur_entry[ 0 ] )
            printf( "DEBUG(SYMBOLS): fixing scope name %s\n", 
            vax.assembler.cur_entry );
    }
#endif

/*  Let's mark the last location before the scope change for future use */

    if( vax.debug & DBG_SYMBOLS  ) {

        if( vax.assembler.cur_entry[ 0 ] != '$' &&
            vax.assembler.cur_entry[ 0 ] != 0 ) {
            strcat( vax.assembler.cur_entry, "$$END" );
            rc = set_symbol_direct("__TEMP", vax.console.deposit-1, SYM_NONE );
            strcpy( vax.console.last_symbol-> name, vax.assembler.cur_entry );
        }
    }
    
    return VAX_OK;
}
    


    
//  Clear all the symbols from the table

LONGWORD clear_all_symbols( void )
{

    struct SYMBOL * p, * next;
    struct FSYMBOL * fp, *fnext;
    
    for( p = vax.console.symbols; p; p = next ) {
        next = p-> next;
    
    // Blow off any dangling forward references
    
        for( fp = p-> forward; fp != 0L; fp = fnext ) {
            fnext = fp-> next;
            freemem( fp );
        }
        
        freemem( p );
    }
    
    vax.console.symbols = 0L;
    return VAX_OK;
}


//  Clear all system symbols that are labels or entries.

LONGWORD clear_system_symbols( void )
{

    struct SYMBOL * p, * last, *next;
    struct FSYMBOL *fp, *fnext;

    LONGWORD found;
    
    last = 0L;
    found = 0;
    
    if( vax.debug & DBG_SYMBOLS )
        printf( "DEBUG: Deleting system symbols\n" );

    for( p = system_symbols; p; p = next ) {

        next = p-> next;

        if( vax.debug & DBG_SYMBOLS  )
            printf( "DEBUG: symbol %s ", p-> name );

        if( p-> flags & SYM_PERMANENT ) {
            last = p;
            if( vax.debug & DBG_SYMBOLS  )
                printf( "is permanent\n" );
            continue;
        }
        if( vax.debug & DBG_SYMBOLS  )
            printf( "; deleting symbol " );

        if( p == system_symbols ) {
            if( vax.debug & DBG_SYMBOLS  )
                printf( "from front of list\n" );
            system_symbols = next;
            last = 0L;
        }
        else {
            if( vax.debug & DBG_SYMBOLS  )
                printf( "from after node %p\n", ( void * ) last );
            last-> next = next;
        }
 

 
    // Blow off any dangling forward references
    
        for( fp = p-> forward; fp != 0L; fp = fnext ) {
            fnext = fp-> next;
            freemem( fp );
        }
    
        freemem( p );

    }

    return VAX_OK;
}


//  Generate a scoped name.  Returns 0 if the name is unchanged, else
//  returns 1 if it was modified.  A name that needs scope is one that
//  begins with "_" but not with "__".

LONGWORD scope_name( char * in_name, char * out_name )
{
    char * p;
    char localname[ 256 ];
    LONGWORD same_buffer = 0;

    if( in_name == out_name )
        same_buffer = 1;

    if( in_name[ 0 ] != '_' ) {
        if( !same_buffer )
            strcpy( out_name, in_name );
        return 0;
    }

    if( in_name[ 1 ] == '_' ) {
        if( !same_buffer )
            strcpy( out_name, in_name );
        return 0;
    }

    if( vax.assembler.cur_entry[ 0 ] == 0 ) {
        temp_seq++;
        sprintf( vax.assembler.cur_entry, "__%d", (LONGWORD) temp_seq );
    }

    p = ( same_buffer ? localname : out_name );

    strcpy( p, vax.assembler.cur_entry );
    strcat( p, "_" );
    strcat( p, &( in_name[ 1 ]));

#ifdef DEBUG

    if( vax.debug & DBG_SYMBOLS  ) {
            printf( "DEBUG(SYMBOLS): scope name to %s\n", p );
    }
#endif

    if( same_buffer )
        strcpy( out_name, localname );

    return 1;
}

