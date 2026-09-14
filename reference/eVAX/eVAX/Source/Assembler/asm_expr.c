//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_expr.c
//
//  Purpose:    Parses numeric expression, using the routines in
//                      asm_value.c to read data.  Supports simple order
//                      precidence operations.
//
//  History:    01/05/99    Created.
//
//              05/11/99    Support creating a string pool;
//                          when an expression contains a
//                          quoted string, create a string and
//                          descriptor, and use the descriptor
//                          as the string address.
//                          See SHOW STRING command to display.
//
//              05/18/99    Added ability to create functions in expressions.  
//                          This will let me fill in some additional mechanisms
//                          more easily.  First function is the DEFINED()
//                          function which tells if a symbol is defined or not.
//
//              06/28/01    Numerous functions have been added and not documented.
//                          Oh well.  TODAY, I am adding the VERBOSE() function that
//                          tests to see if the console verbose messaging flag is
//                          set.  
//

#include "vax.h"
#include "vaxinstr.h"

#include <ctype.h>

#if defined( VMS ) || defined( macintosh )
#include <stat.h>
#else
#include <sys/stat.h>
#endif



//  Local prototypes

#include "asmproto.h"

LONGWORD asm_expr2( char ** P, LONGWORD * value );
LONGWORD asm_expr3( char ** P, LONGWORD * value );
LONGWORD asm_expr_math( char ** P, LONGWORD * value );


//      Parse an expression with math operators

LONGWORD asm_expr_math( char ** P, LONGWORD * value )
{
    LONGWORD v1, v2;
    LONGWORD rc;
    char * p;
    char op;
	int running;

    vax.assembler.flags &= ~ASM_EXPRESSION;

    rc = asm_expr2( P, &v1 );
    if( rc )
        return rc;

	running = 1;
    while( running ) {
        p = *P;
        flush_blanks( &p );

        if( isend( *p ))
            break;

        if( *p != '+' && *p != '-' ) {
            break;
        }

        op = *p;
        p++;
        rc = asm_expr2( &p, &v2 );
        if( rc )
            return rc;

        vax.assembler.flags |= ASM_EXPRESSION;
        if( op == '+' )
            v1 = v1 + v2;
        else
        if( op == '-' )
            v1 = v1 - v2;
        *P = p;
    }

    *value = v1;
    return VAX_OK;
}


//      Parse an expression with conditional operators

LONGWORD asm_expr( char ** P, LONGWORD * value )
{
    LONGWORD v1, v2;
    LONGWORD rc;
    char * p;
    char op;
	int running;

    vax.assembler.flags &= ~ASM_EXPRESSION;
 
    rc = asm_expr_math( P, &v1 );
    if( rc )
        return rc;

	running = 1;
    while( running ) {
        p = *P;
        flush_blanks( &p );

        if( isend( *p ))
            break;

        op = 0;
        if( *p == '=' )
            op = 1;
        else
        if( *p == '<' && *(p+1) == '>' ) {
            op = 2;
            p++;
        }
        else
        if( *p == '<' && *(p+1)  == '=' ) {
            op = 3;
            p++;
        }
        else
        if( *p == '<' )
            op = 4;
        else
        if( *p == '>' && *(p+1) == '=' ) {
            p++;
            op = 5;
        }
        else
        if( *p == '>' )
            op = 6;

        if( !op )
            break;

        p++;
        rc = asm_expr_math( &p, &v2 );
        if( rc )
            return rc;

        vax.assembler.flags |= ASM_EXPRESSION;

        /* Based on what kind of comparison, do the math... */

        switch( op ) {
        case 1: v1 = ( v1 == v2 ); break;
        case 2: v1 = ( v1 != v2 ); break;
        case 3: v1 = ( v1 <= v2 ); break;
        case 4: v1 = ( v1 <  v2 ); break;
        case 5: v1 = ( v1 >= v2 ); break;
        case 6: v1 = ( v1 >  v2 ); break;
        }

        *P = p;
    }

    *value = v1;
    return VAX_OK;
}



/*
 *  Parse an expression containing *, /
 */


LONGWORD asm_expr2( char ** P, LONGWORD * value )
{
    LONGWORD v1, v2;
    LONGWORD rc;
    char * p;
    char op;
	int running;

    rc = asm_expr3( P, ( LONGWORD * ) &v1 );
    if( rc )
        return rc;

	running = 1;
    while( running ) {
        p = *P;
        flush_blanks( &p );
        if( isend( *p ))
            break;

        if( *p != '*' && *p != '/' ) {
            break;
        }

        op = *p;
        p++;
        rc = asm_expr3( &p, ( LONGWORD * ) &v2 );
        if( rc )
            return rc;

        vax.assembler.flags |= ASM_EXPRESSION;
        if( op == '*' )
            v1 = v1 * v2;
        else
        if( op == '/' )
            v1 = v1 / v2;
        *P = p;
    }

    *value = v1;
    return VAX_OK;
}

/*
 *  Parse and expression atom.
 */


LONGWORD asm_expr3( char ** P, LONGWORD * value )
{
    LONGWORD rc;
    char * p, *pp, esc;
    char rn[ 64 ];
    ULONGWORD regnum, n, size, linkage;
    extern char * pr_names[];
    char buff[ 256 ], *bp;
    int indirect, running;
    ULONGWORD stringpool, stringpool_size, stringpool_base;

    static struct REGNAMES {
       char * name; 
       LONGWORD   number;
    } regnames[] = 
    { { "R0",   0 },
      { "R1",   1 },
      { "R2",   2 },
      { "R3",   3 },
      { "R4",   4 },
      { "R5",   5 },
      { "R6",   6 },
      { "R7",   7 },
      { "R8",   8 },
      { "R9",   9 },
      { "R10", 10 },
      { "R11", 11 },
      { "R12", 12 },
      { "R13", 13 },
      { "R14", 14 },
      { "R15", 15 },
      { "AP",  12 },
      { "FP",  13 },
      { "SP",  14 },
      { "PC",  15 },
      { 0L,    0  }};

    flush_blanks( P );
    p = *P;
    rn[ 0 ] = 0;
    indirect = 0;
    
    if( *p == '@' ) {
        indirect = 1;
        p++;
        flush_blanks( &p );
    }
    
    if( isalpha( *p )) {

       rn[ 0 ] = *p;
       p++;
       n = 1;
	   running = 1;

       while( running ) {

           if( *p == '$' || *p == '_' || isalnum( *p )) {
             rn[ n++ ] = *p;
             p++;
             continue;
           }
           break;
       }
  
       regnum = 0xFFFFFFFF;
       rn[ n ] = 0;

       /* See if it's a function call */

       pp = p;
       flush_blanks( &pp );
       rc = asm_function( rn, &pp, value );
       if( rc != VAX_ASMNOTFUN ) {
           if( indirect ) 
                rc = VAX_ASMINVIND;
           *P = pp;
           return rc;
       }

       /* It might be an indirect register reference */
       
       if( indirect ) {
       
           /* See if it's the reserved name PSL */

           if( strcmp( rn, "PSL" ) == 0 ) {
               write_psl_bits();
               *value = vax.psl.reg;
               *P = p;
               return VAX_OK;
           }

           /* See if it's a register name */

           for( n = 0; regnames[ n ].name; n++ )
               if( strcmp( regnames[ n ].name, rn ) == 0 ) {
                  regnum = regnames[ n ].number;
                  break;
           }

           /* If searching regular register's didn't do it, check */
           /* out the special registers.                          */

           if( regnum == 0xFFFFFFFF  ) {
              for( n = 0; *pr_names[ n ]; n++ ) {
                    if( strcmp( pr_names[ n ], rn ) == 0 ) {
                        *value = vax.preg[ n ];
                        *P = p;
                        return VAX_OK;
                    }
              }
           }

           if( regnum != 0xFFFFFFFF ) {
               *value = vax.reg[ regnum ];
    //           printf( "Register parse finds register %ld\n", regnum );
               *P = p;
               return VAX_OK;
           }
       }
       
       p = *P;  /* Not a register, reset to where we started */

    }        

   if( indirect ) 
        return VAX_ASMINVIND;

//  See if it's the '.' which means "here"

    if( *p == '.' ) {
        p++;
        *P = p;
        *value = vax.console.deposit;
        return VAX_OK;
    }

//  See if it's a new expression in parenthesis
    
    if( *p == '(' ) {
        p++;
        rc = asm_expr( &p, value );
        if( rc == VAX_OK ) {
            flush_blanks( &p );
            if( *p == ')' )
               p++;
            *P = p;
        }
        return rc;
    }

//  Might be a quoted string.  If so we construct a descriptor on the
//  fly for it and hope it works out.

    if( *p == '"' ) {

        /* Get string pool area */
        strcpy( buff, "CONSOLE$STRINGPOOL" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool );
        if( rc )
            return rc;
        n = stringpool;

        strcpy( buff, "CONSOLE$STRINGPOOL_SIZE" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool_size );
        if( rc )
            return rc;

        strcpy( buff, "CONSOLE$STRINGPOOL_BASE" );
        bp = buff;
        rc = get_symbol( &bp, ( LONGWORD * ) &stringpool_base );
        if( rc )
            return rc;

        p++;
        size = 0;
        linkage = n; /* Make room for the linkage */
        rc = store_memory( n, ( void * ) &size, 4 );
        if( rc )
            return rc;
            
        n = n + 4;
        
        while (*p != '"' && !isend( *p )) {

            if( n > ( stringpool_base + stringpool_size + 16 ))
                return VAX_ASMSPOVF;

            if( *p == '\\' ) {
            
                p++;
                esc = *p;
                switch( esc ) {
                case 'n':   esc = '\n'; break;
                case 'r':   esc = '\r'; break;
                case 't':   esc = '\t'; break;
                }
                rc = store_memory( n, ( void * ) &esc, 1 );
            }
            else
                rc = store_memory( n, ( void * ) p, 1 );
                
            if( rc )
                return rc;
            size++;
            p++;
            n++;
        }

        /* Now construct a descriptor */
        if( n & 0x00000003 )
            n = ( n & 0xFFFFFFFC ) + 4;

        /* Write the descriptor length longword */
        
        rc = store_memory( n, ( void * ) &size, 4 );
        if( rc )
            return rc;

        /* Write the descriptor address (offset 4 for the linkage field) */
        
        stringpool = stringpool + 4;
        rc = store_memory( n+4, ( void * ) &stringpool, 4 );
        if( rc )
            return rc;

        /* Update the link to point to the descriptor address */
        
        rc = store_memory( linkage, ( void * ) &n, 4 );
        if( rc )
            return 0;
        
        /* Store the link list terminator */
        
        linkage = 0L;
        rc = store_memory( n+8, ( void * ) &linkage, 4 );
        if( rc )
            return 0;
            
        rc = set_symbol_direct( "CONSOLE$STRINGPOOL", n+8, 0 );
        if( rc )
            return rc;

        if( *p == '"' )
            p++;  /* Move past closing quote */

        *P = p;
        *value = n;

        return rc;        
    }

//  Nope, must just be a value or symbol.  Grab it.

    rc = asm_hex( P, ( ULONGWORD * )value );

    return rc;
}



LONGWORD asm_function( char * theName, char ** P, LONGWORD * value )
{

    char * p, *fp;
    LONGWORD found, n, rc, fnum;

    char * str_arg[ 4 ];
    LONGWORD  long_arg[ 4 ];
    char argstr[ 256 ];
    LONGWORD cp, size;
    struct SYMBOL * sp;
    LONGWORD d_long;
    short d_word;
    char d_byte;
    int running;
    struct stat f_info;
    
#define F_NONE  0
#define F_LONG  1
#define F_STR   2


    static struct FNAME {
        LONGWORD    fid;
        LONGWORD    argc;
        char    argtype[ 4 ];
        char    * fname;
    } fnames[] = {

        /*  ID     Argc     Arg types for args 0-3            Name       */

        {   100,    1,  {   F_STR,  F_NONE, F_NONE, F_NONE },   "DEFINED"   },
        {   101,    0,  {   F_NONE, F_NONE, F_NONE, F_NONE },   "PMEMSIZE"  },
        {   102,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "LONG"      },
        {   103,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "WORD"      },
        {   104,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "BYTE"      },
        {   105,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "ULONG"     },
        {   106,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "UWORD"     },
        {   107,    1,  {   F_LONG, F_NONE, F_NONE, F_NONE },   "UBYTE"     },
        {   108,    1,  {   F_STR,  F_NONE, F_NONE, F_NONE },   "FEXISTS"   },
        {   108,    1,  {   F_STR,  F_NONE, F_NONE, F_NONE },   "FILEEXISTS"},
        {   108,    1,  {   F_STR,  F_NONE, F_NONE, F_NONE },   "FILE_EXISTS"},
        {   109,    0,  {   F_NONE, F_NONE, F_NONE, F_NONE },   "MKVALID"   },
        {   110,    0,  {   F_NONE, F_NONE, F_NONE, F_NONE },   "VMVALID"   },
        {   111,    0,  {   F_NONE, F_NONE, F_NONE, F_NONE },   "VERBOSE"   },
        {   0,      0,  {   F_NONE, F_NONE, F_NONE, F_NONE },   ""          }};


    p = *P;
    found = 0;
    flush_blanks( &p );

    for( fnum = 0; fnum < 50; fnum++ ) {
        fp = fnames[ fnum ].fname;
        if( *fp == 0 )
            break;
        if( strcmp( theName, fp ) == 0 ) {
            found = fnames[ fnum ].fid;
            break;
        }
    }

    if( !found )
        return VAX_ASMNOTFUN;

/*
 *  Any arguments?
 */

    for( n = 0; n < 4 ; n++ )
        str_arg[ n ] = 0L;

    n = 0;
    flush_blanks( &p );
    if( *p == '(' ) {

        p++;
		running = 1;
        while( running ) {

            flush_blanks( &p );
            if( *p == ')' ) {
                p++;
                break;
            }
            if( isend( *p ))
                break;
            if( n >= 4 )
                return VAX_ASMNUMFARG;

            if( *p == '"' ) {
                p++;
                cp = 0;
                while( !isend( *p ) && *p != '"' ) {
                    argstr[ cp ] = *p;
                    cp++;
                    p++;
                }
                if( *p == '"' )
                    p++;

                argstr[ cp ] = 0;
                size = cp + 1;
                str_arg[ n ] = getmem( size );
                strcpy( str_arg[ n ], argstr );

                if( fnames[ fnum ].argtype[ n ] != F_STR ) {
                    return VAX_ASMMMFARG;
                }

            }
            else {
                rc = asm_expr( &p, &( long_arg[ n ]));
                if( rc )
                    return rc;
                if( fnames[ fnum ].argtype[ n ] != F_LONG ) {
                    return VAX_ASMMMFARG;
                }
            }

            flush_blanks( &p );
            if( *p == ',' )
                p++;

            n++;
        }

    }

    if( n != fnames[ fnum ].argc )
        return VAX_ASMNUMFARG;

    *P = p;
    rc = VAX_OK;


/*-----------------------------------------------------------------*
 *                                                                 *
 *  Function dispatcher.  There is a CASE block for each function  *
 *  supported.  The value 'n' tells how many arguments (up to 4)   *
 *  there are for the function.  str_arg[n] will contain a ptr to  *
 *  the string if it's a string argument, else long_arg[ n ] will  *
 *  be the argument.  The result is always numeric and written     *
 *  to *value when done.  Set RC to a non-VAX_OK value to signal   *
 *  an error if you get one.                                       *
 *-----------------------------------------------------------------*/

    switch( found ) {

    case 100:   /*  DEFINED( "symbol" ) */

                /*  Returns 0   if the symbol is undefined.         */
                /*          1   if the symbol is a user symbol.     */
                /*          2   if the symbol is a system symbol.   */


        /* Make sure we have one string argument */

        if( str_arg[ 0 ] == 0L || n != 1 ) {
            rc = VAX_ASMINVFARG;
            break;
        }

        uppercase( str_arg[ 0 ]);

        //  Search the regular symbol table.

        found = 0;
        for( sp = vax.console.symbols; sp; sp = sp-> next ) 
            if( strcmp( str_arg[ 0 ], sp-> name ) == 0 ) {
                found = 1;
                break;
        }

        //  There's also a table of "predefined" symbols we should check as well.

        if( !found )
            for( sp = system_symbols; sp; sp = sp-> next ) 
                if( strcmp( str_arg[ 0 ], sp-> name ) == 0 ) {
                    found = 2;
                    break;
                }

        if( found && sp-> forward )
            found = found + 16;

        *value = found;
        break;


    case 101:   /* PMEMSIZE() */

                /*  Returns the size of the virtual VAX physical memory partition */

        *value = vax.memsize;
        break;


    case 102:   /* LONG( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_long, 4 );
        if( rc )
            return rc;
        
        *value = d_long;
        break;
        
    case 103:   /* WORD( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_word, 2 );
        if( rc )
            return rc;
        
        *value = ( LONGWORD ) d_word;
        break;
        
    case 104:   /* BYTE( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_byte, 1 );
        if( rc )
            return rc;
        
        *value = ( LONGWORD ) d_byte;
        break;
        
    case 105:   /* LONG( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_long, 4 );
        if( rc )
            return rc;
        
        *value = ( ULONGWORD ) d_long;
        break;
        
    case 106:   /* WORD( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_word, 2 );
        if( rc )
            return rc;
        
        *value = ( unsigned short ) d_word;
        break;
        
    case 107:   /* BYTE( addr ) */

        rc = load_memory( long_arg[ 0 ], ( void * ) &d_byte, 1 );
        if( rc )
            return rc;
        
        *value = ( unsigned char ) d_byte;
        break;
    
    case 108:   /*  FILEEXISTS( "symbol" ) */

                /*  Returns 0   if the file is not found.       */
                /*          1   if the file is found.           */

        /* Make sure we have one string argument */

        if( str_arg[ 0 ] == 0L || n != 1 ) {
            rc = VAX_ASMINVFARG;
            break;
        }

        *value = stat( str_arg[ 0 ], &f_info ) ? 0 : 1;
        
        break;
    
    case 109:   /*  MKVALID() */
    
        *value = MKVALID;
        break;
    
    case 110:   /*  VMVALID() */
    
        *value = VMVALID;
        break;
    
    case 111:	/*  VERBOSE() */
        *value = ( vax.console.flags & CONSOLE_VERBOSE ) ? 1 : 0;
        break;


    default:
        rc = VAX_ASMINVFARG;

    }

    /*  We're done, free up any string arguments we allocated */

    for( n = 0; n < 4; n++ ) 
        if( str_arg[ n ] )
            freemem( str_arg[ n ] );

    return rc;
}


