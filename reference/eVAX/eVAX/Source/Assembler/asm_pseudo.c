//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     asm_pseudo.c
//
//  Purpose:    Pseudo instruction handler for the assembler.  This handles all
//              the dot-format instructions.  These instructions may store data
//              in memory, or affect the state of the virtual VAX.  For example,
//              some are used to set processor register values.
//
//  History:    07/28/97    New header format standardization
//
//              07/24/97    Converted storage pseudo ops to use store_memory() so
//                          they will be ready for virtual memory
//
//              09/15/98    Added .F_FLOAT and .D_FLOAT statements for creating
//                          constants in-memory.
//
//              12/28/98    Mark symbols created by .ENTRY with the SYM_ENTRY
//                          flag, for disassembly purposes.
//
//              01/08/99    Fix goofup with .ASCID format
//
//              01/11/99    Clear or fix temporary symbols when we get to
//                          a new .entry, or change the .base register, or
//                          .end an assembly.
//
//              01/18/99    When a .END explicitly references a starting
//                          address, set a flag in the assembler control
//                          block that causes the console handler to call
//                          the __ENTRY location when the assembly finishes.
//
//                          Also, mark the .ENTRY name as having to be a
//                          unique symbol definition.
//
//              01/26/99    Add .CASE support.  Also, fix .SET so that it
//                          can take an expression.
//
//              02/03/99    Add .SCB pseudo opcode, which sets SCB vector
//                          entries.  Also added .REGION operator that
//                          switches between P0 and S0 deposit locations.
//
//              02/15/99    Fixed .SCB to require aligned data, also allow
//                          stack to be specified separately.  Also added
//                          .MODE to set the mode for assembly.  Added
//                          .VECTOR as shortcut for .SCB and handler.
//
//              05/20/99    Added .MODE, .VECTOR, .CONSOLE, .INCLUDE and .IF
//
//              05/25/99    Added qualifiers to .SET for /PERM, /ENTRY, /LABEL,
//                          to set symbol characteristics.  The /PERM is used
//                          in ssdef.asm, for example to make status codes
//                          permanently defined.
//
//              07/14/99    Added .SHIM for creating shared library shims when
//                          running an executable image.
//
//              09/20/99    Added tests to only allow SCB and similar directives
//                          to be used when the default microkernel is active.
//                          Added the .MICROKERNEL directive which indicates
//                          that the microkernel is being built.
//
//              11/09/99    Added .P1VECTOR which builds the P1 system service
//                          vector for VMS images.
//
//              05/17/00    Added .SCOPE to make it easier to create tables
//                          like the error message list in the microkernel.
//

#include "vax.h"
#include <ctype.h>
#include "asmproto.h"
#include "fpu.h"
#include "console_proto.h"
#include "pte.h"

//  Mapping of privileged register names.  There must be MAXPRIVREG of these or there
//  will trouble... :)

const char * pr_names[] = {
/*  0           1           2           3           4           5           6           7 */
    "KSP",      "ESP",      "SSP",      "USP",      "ISP",      "_PR05",    "_PR06",    "_PR07",
    "P0BR",     "P0LR",     "P1BR",     "P1LR",     "SBR",      "SLR",      "_PR14",    "_PR15",
    "PCBB",     "SCBB",     "IPL",      "ASTLVL",   "SIRR",     "SISR",     "_PR22",    "_PR23",
    "ICCS",     "NICR",     "ICR",      "TODR",     "_PR28",    "_PR29",    "_PR30",    "_PR31",
    "RXCS",     "RXDB",     "TXCS",     "TXDB",     "TBDR",     "_PR37",    "_PR38",    "_PR39",
    "_PR40",    "SAVISP",   "SAVPC",    "SAVPSL",   "WCSA",     "WCSB",     "_PR46",    "_PR47",
    "_PR48",    "_PR49",    "_PR50",    "_PR51",    "_PR52",    "_PR53",    "_PR54",    "_PR55",
    "MAPEN",    "TBIA",     "TBIS",     "_PR59",    "_PR60",    "PMR",      "SID",      "TBCHK",
    "_PR64",    "_PR65",    "_PR66",    "_PR67",    "_PR68",    "_PR69",    "_PR70",    "_PR71",
    "_PR72",    "_PR73",    "_PR74",    "_PR75",    "_PR76",    "_PR77",    "_PR78",    "_PR79",
    "_PR80",    "_PR81",    "_PR82",    "_PR83",    "_PR84",    "_PR85",    "_PR86",    "_PR87",
    "_PR88",    "_PR89",    "_PR90",    "_PR91",    "_PR92",    "_PR93",    "_PR94",    "_PR95",
    "_PR96",    "_PR97",    "_PR98",    "_PR99",    "_PR100",   "_PR101",   "_PR102",   "_PR103",
    "_PR104",   "_PR105",   "_PR106",   "_PR107",   "_PR108",   "_PR109",   "_PR110",   "_PR111",
    "_PR112",   "_PR113",   "_PR114",   "_PR115",   "_PR116",   "_PR117",   "_PR118",   "_PR119",
    "_PR120",   "_PR121",   "_PR122",   "_PR123",   "_PR124",   "_PR125",   "_PR126",   "_PR127",
    ""     };  /* Empty string needed by asm_expr3() */

/*  0           1           2           3           4           5           6           7 */
    

    static struct PSEUDOS {
        char * name;
        short   code;
    } pseudos[] = {
        {   "BYTE",     1   },      //  Declare a byte
     // {   "DECB",     1   },
        {   "WORD",     2   },      //  Declare a word
     // {   "DECW",     2   },
        {   "LONG",     3   },      //  Declare a longword
     // {   "DECL",     3   },
        {   "BASE",     4   },      //  Set new base address (sets PC value)
        {   "SET",      5   },      //  Set a symbol
        {   "CLEAR",    6   },      //  Clear a symbol or all symbols
        {   "ASCII",    7   },      //  Ascii character string
        {   "ASCIZ",    8   },      //  Ascii character string followed by null byte
        {   "ASCIC",    9   },      //  Ascii character string preceded by 16-bit count
        {   "ASCID",    10  },      //  Ascii character string preceded by string descriptor
        {   "END",      11  },      //  End of input string
        {   "PSL",      12  },      //  Set PSL longword
        {   "PRINT",    13  },      //  Print a string to the console/listing
        {   "MASK"  ,   14  },      //  Create an entry point register mask
        {   "PTE",      15  },      //  Define a page table entry longword
        {   "F_FLOAT",  16  },      //  Store an F_FLOAT value
        {   "D_FLOAT",  17  },      //  Store a  D_FLOAT value
        
        //  Do not renumber the BLK* operators without modifying the code for
        //  them since they depend on the code to create a size scale value.
        
        {   "BLKB",     18  },      //  A block of bytes
        {   "BLKW",     19  },      //  A block of words
        {   "BLKL",     20  },      //  A block of longs
        {   "BLKF",     20  },      //  A block of F_FLOAT (same as longs)
        {   "BLKD",     21  },      //  A block of D_FLOATs
        
        //  Resume "free form" table assignments here.
        
        {   "ENTRY",    22  },      //  Define entry symbol AND register mask
        {   "SYM",      23  },      //  Define a system symbol

        //  Do not change the number for CASE without modifying the code
        //  below that empties the case_base field.

        {   "CASE",     24  },      //  CASEx table entry
        {   "SCB",      25  },      //  SCB vector slot
        {   "ALIGN",    26  },      //  Align deposit
        {   "REGION",   27  },      //  Choose region deposit
        {   "MODE",     28  },      //  Choose processor mode
        {   "VECTOR",   29  },      //  Declare SCB handler
        {   "CONSOLE",  30  },      //  Execute a console command
        {   "INCLUDE",  31  },      //  Include a nested assembly file
        {   "IF",       32  },      //  Conditional assembly
        {   "SHIM",     33  },      //  Create software RTL shim
        {   "MICROKERNEL",34},      //  Microkernel valid
        {   "SPACE",    35 },       //  A filled block of bytes
        {   "DATA",     36 },       //  Specify data section
        {   "TEXT",     37 },       //  Specify text (code) section
        
        //  Here are pseudo-ops mapped from aliased instructions
        
        {   "JEQL",     38 },       //  Long jump if equal
        {   "JEQLU",    38 },       //  --unsigned the same
        {   "JNEQ",     39 },       //  Long jump if not equal
        {   "JNEQU",    39 },       //  --unsigned the same

        //  And more misc operators.

        {   "P1VECTOR", 40 },       //  Generate P1 system service vector
        {   "SCOPE",    41 },       //  Set new local symbol scope

        //  End of table marker.  Must be last item.
        {   "",         -1  }};
        
    static struct RNAMES {
        char * name;
        short   number;
    } rname[] = {
        {   "R0",   0   },
        {   "R1",   1   },
        {   "R2",   2   },
        {   "R3",   3   },
        {   "R4",   4   },
        {   "R5",   5   },
        {   "R6",   6   },
        {   "R7",   7   },
        {   "R8",   8   },
        {   "R9",   9   },
        {   "R10",  10  },
        {   "R11",  11  },
        {   "R12",  12  },
        {   "R13",  13  },
        {   "R14",  14  },
        {   "R15",  15  },
        {   ".",    15  },
        {   "AP",   12  },
        {   "FP",   13  },
        {   "SP",   14  },
        {   "PC",   15  },
        {   0L, 0 }};
        

//  Assemble a pseudo operand

LONGWORD asm_pseudo( char * Buffer )
{
    char q, esc;
    LONGWORD count_pc, string_pc;
    short count;
    LONGWORD oldrgn, newrgn;    
    LONGWORD n, match, rc, code, perm;
    LONGWORD long1, long2, disp, scale, verb, verb2, vector, stack;
    int running;

    union VALUE {
        unsigned char   byte[ 4 ];
        unsigned short  word[ 2 ];
        ULONGWORD   longword;
    } value;
    char * p = Buffer;
    char b[ 32 ], *bp, ch;
    LONGWORD shim_addr;
    char rtl[ 40 ];
    
    union PTE pte;
    
    // Skip the optional first dot.
    running = 1;
    flush_blanks( &p );
    if( *p == '.' )
        p++;
        
    //  Copy the opcode to a local buffer.
    
    n = 0;
    while( !isend( *p ) && !is_blank( *p ) && ( *p != '/' )) {
    
        ch = *(p++);
        if( ch >= 'a' && ch <= 'z' )
            ch = ( char ) ( ch - 32 );
        b[ n++ ] = ch;
        
    }
    b[ n ] = 0;

    //  Search the list of privileged registers to see if it's a match
    
    match = 0;
    for( n = 0; n < MAXPRIVREG; n++ ) {
    
        bp = ( char * ) pr_names[ n ];
        if( *bp == 0 )
            break;
            
        if( strcmp( b, bp ) == 0 ) {
            match = 1;
            break;
        }
    }

    //  If a match was found, it's a pseudo operand like .KSP and we set the
    //  associated privilege register now and we're done.
        
    if( match ) {

        vax.assembler.case_base = 0L;
    
        //  Parse the actual numeric value to be placed in the register
        
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;

        if( vax.pslw.cur_mod > 0 ) {
            set_fault( EXC_PRIV, 0 );
            return handle_fault();
        }
                
        vax.preg[ n ] = value.longword;
        return VAX_OK;
    }

    //  It wasn't a privileged register name, so search the list of known pseudo
    //  opcodes to find a match.
    
    match = 0;
    for( n = 0; pseudos[ n ].name[ 0 ]; n++ ) {
        if( strcmp( pseudos[ n ].name, b ) == 0 ) {
            match = 1;
            break;
        }
    }
    
    if( !match )
        return VAX_ASMNOTPSEUDO;    // Non zero code means no pseudo operand handled.
        
    //  Else handle the actual function of the routine.  If it's other
    //  than the .CASE operator, empty out the case base value so it
    //  resets the next time we do a .CASE block.
    
    code = pseudos[ n ].code;
    rc = VAX_OK;

    if( code != 24 ) /****MUST MATCH THE .CASE VALUE ****/
        vax.assembler.case_base = 0L;
    
    switch( code ) {
    
    case 1:     //  .BYTE
        
        while( running ) {
        
            flush_blanks( &p );
            if( isend( *p ))
                break;
            if( *p == ',' ) {
                p++;
            }
            
            rc = asm_value( &p, &value.longword, K_ADDR_B );
            if( rc )
                return rc;
            
            if( value.longword > 0x00FF )
                return VAX_ASMINVBYTE;

#if BIGENDIAN            
            rc = store_memory( vax.console.deposit, 
                          ( void * ) &value.byte[ 3 ], 1 );
#else
            rc = store_memory( vax.console.deposit, 
                          ( void * ) &value.byte[ 0 ], 1 );
#endif

            if( rc )
                return rc;

            vax.console.deposit += 1;
        }
        
        break;
    
    case 2:     //  .WORD
        
        while( running ) {
        
            flush_blanks( &p );
            if( isend( *p ))
                break;
            if( *p == ',' ) {
                p++;
            }
            rc = asm_value( &p, &value.longword, K_ADDR_W );
            if( rc )
                return rc;
            
            if( value.longword > 0x0000FFFFL )
                return VAX_ASMINVWORD;

#if BIGENDIAN            
            rc = store_memory( vax.console.deposit, 
                        ( void * ) &value.word[ 1 ], 2 );
#else
            rc = store_memory( vax.console.deposit, 
                        ( void * ) &value.word[ 0 ], 2 );
#endif
            if( rc )
                return rc;

            vax.console.deposit += 2;
        }
        break;
        
    case 3:     //  .LONG

        while( running ) {        
            flush_blanks( &p );
            if( isend( *p ))
                break;
            if( *p == ',' ) {
                p++;
            }
            rc = asm_value( &p, &value.longword, K_ADDR_L );
            if( rc )
                return rc;
            
            rc = store_memory( vax.console.deposit, ( void * ) &value.longword, 4 );
            if( rc )
                return rc;

            vax.console.deposit += 4;
            
        }
        break;
            
    case 4:     //  .BASE   value           ! Set location to store into.
                //                          ! .SET PC value     would be identical

        //  Because we're starting new, flush or fix temp symbols

        rc = scope_symbols();
        if( rc )
            return rc;

        //  Get the new base value
        
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;
        
        vax.console.deposit = value.longword;
        
        break;

    case 5:     //  .SET SYM VALUE
        flush_blanks( &p );

        perm = 0;

        //  Look for qualifiers that affect the symbol.

        /*  /PERMANENT      the symbol cannot be deleted    */

        bp = p;
        rc = read_verb( &p, &verb2 );
        if( rc )
            return rc;
        if( verb2 == CHAR4( '/','P','E','R' ) ||
            verb2 == CHAR4( '/','P','R','M' )) {
            perm = perm | SYM_PERMANENT;
        }
        else {
            p = bp;
        }
        flush_blanks( &p );


        /*  /ENTRY          the symbol is an entry          */

        bp = p;
        rc = read_verb( &p, &verb2 );
        if( rc )
            return rc;
        if( verb2 == CHAR4( '/','E','N','T' )) {
            perm = perm | SYM_ENTRY;
        }
        else {
            p = bp;
        }
        flush_blanks( &p );


        /*  /LABEL          the symbol is a label           */

        bp = p;
        rc = read_verb( &p, &verb2 );
        if( rc )
            return rc;
        if( verb2 == CHAR4( '/','L','A','B' ) ||
            verb2 == CHAR4( '/','L','B','L' )) {
            perm = perm | SYM_LABEL;
        }
        else {
            p = bp;
        }
        flush_blanks( &p );


        //  Copy the name to a local buffer.
        

        n = 0;
        while( !is_blank( *p ) && !( *p == ',' ) && !(*p == '=' ) && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;

        flush_blanks( &p );
        if( *p == '=' || *p == ',' ) {
            p++;
            flush_blanks( &p );
        }
        
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;
    
        //  Check for special cases of VAX processor registers.
        
        for( n = 0; rname[ n ].name; n++ ) {
            if( strcmp( b, rname[ n ].name ) == 0 ) {
                vax.reg[ rname[ n ].number ] = value.longword;
                return VAX_OK;
            }
        }
        
        //  Otherwise, store the symbol away.  If there are extended
        //  attributes, set them as well.
        
        bp = b;
        rc = set_symbol( &bp, value.longword, (int) perm );

        return rc;
    
    case 6:     //  .CLEAR SYM    or   .CLEAR ! all

        flush_blanks( &p );

        //  See if there is a name.  If not, then we clear all symbols
        
        if( isend( *p ))
            return clear_all_symbols();
        
        //  Copy the name to a local buffer.
        
        n = 0;
        while( !is_blank( *p ) && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;

                
        rc = clear_symbol( b );
        return rc;
    
    case 7:     //  .ASCII  "string" [,"string"...]         ! Store characters as-is    
    case 8:     //  .ASCIZ  "string" [,"string"...]         ! Store chars followed by zero
    case 9:     //  .ASCIC  "string" [,"string"...]         ! Store counted chars
    case 10:    //  .ASCID  "string" [,"string"...]         ! Descriptor and string storage
    
    
        count = 0;
        count_pc = vax.console.deposit;
    
        if( code == 9 ) {   /* .ascic counted string, leading word is size */
            rc = store_memory( count_pc, (void * ) &count, 2 );
            if( rc )
                return rc;
            vax.console.deposit += 2;
        }
        
        if( code == 10 ) {  /* .ascid descriptor followed by string */
        
            /* Write out length of zero.  count_pc is used later to */
            /* update the string length after parsing it.           */

            rc = store_memory( count_pc, (void * ) &count, 2 );
            if( rc )
                return rc;

            /* Descriptor type, 0 = string */

            rc = store_memory( count_pc + 2, (void * ) &count, 2 );
            if( rc )
                return rc;
            
            /* Advance to second longword which contains the address */
            vax.console.deposit += 4;
            string_pc = vax.console.deposit + 4;
            rc = store_memory( vax.console.deposit, ( void * ) &string_pc, 4 );
            if( rc )
                return rc;
            
            /* Advance past the address longword to be ready to write string */
            vax.console.deposit += 4;
    
        }
        //  While there is more in the string...
        
        while( running ) {
            
            //  Find the start of the string to store in memory.  Give up if
            //  we're at the end of the string.
            
            flush_blanks( &p );
            if( isend( *p ))
                break;
            
            //  See if this is a quoted string "foo", 'foo', or /foo/ and if so
            //  remember the quote and strip it.  If it isn't quoted, then just
            //  note it with a null q character.
                
            q = *p;
            if( q == '/' || q == '\'' || q == '"' )
                p++;
            else
                q = 0;
            
            //  Pick up characters from the operand buffer
            
            while( *p ) {
            
                //  If this matches the quote character we're done
                
                if( *p == q )
                    break;
                
                //  If there wasn't a quote character and this is a blank, we're done
                
                if( q == 0 && is_blank( *p ))
                    break;

                //  See if it's an escape character, if so then process it.
                
                if( *p == '\\' ) {
                
                    p++;
                    esc = *p;
                    switch( esc ) {
                    
                    case 'n':   esc = '\n'; break;
                    case 'r':   esc = '\r'; break;
                    case 't':   esc = '\t'; break;
                    }
                    rc = store_memory( vax.console.deposit++, ( void * ) &esc, 1 );
                }
                
                //  Otherwise just store the character in memory and advance
                else    
                    rc = store_memory( vax.console.deposit++, ( unsigned char * ) p, 1 );
                    
                if( rc )
                    return rc;
                count++;
                p++;
            }
            
            //  If the current character matches the quote (and isn't zero) then skip
            //  past the closing quote character.
            
            if (*p == q && q )
                p++;
            
            flush_blanks( &p );
            if( *p != ',' )
                break;
                
        }
        
        if( code == 8 ) {   /*  .ASCIZ */
            count = 0;
            rc = store_memory( vax.console.deposit++, ( unsigned char * ) &count, 1 );
            if( rc )
                return rc;
        }
        else
        if( code == 9 || code == 10 ) { /*  .ASCIC  or .ASCID */
            rc = store_memory( count_pc, ( unsigned char * ) &count, 2 );
            if( rc )
                return rc;
        }
        
        break;  /* All done with ASCII variations... */


    case 11:    //  .END
    
        //  Fix the temp symbols, if any

        rc = scope_symbols();
        if( rc )
            return rc;

        //  If there is a default entry name, set __ENTRY to that value.

        flush_blanks( &p );
        if( !isend( *p )) {
        
            rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
            if( rc )
                return rc;

            rc = set_symbol_direct( "__ENTRY", value.longword, 0 );

            vax.assembler.flags |= ASM_ENTRY;
        }

                
        //  Get out of assembler mode.

        vax.console.assembler_mode = 0;
        break;

    case 12:    //  .PSL
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;
        vax.psl.reg = value.longword;
        
        /* Since the PSL is stored two ways, let's read the bit pattern */
        /* and update the "Wide" version too */
        
        read_psl_bits();
    
        break;

    case 13:    //  .PRINT

        rc = console_print( &p );
        break;
        
    case 14:    //  .MASK
        flush_blanks( &p );
        rc = asm_mask( &p, ( LONGWORD * ) &value.longword );
        
        count = ( short ) value.longword;
        count_pc = vax.console.deposit;
        
        rc = store_memory( count_pc, (void * ) &count, 2 );
        if( rc )
            return rc;

        vax.console.deposit += 2;
        break;
    
    case 15:    //  .PTE    b_valid, w_protmask, w_modifymask, w_owner, l_pfn
    
        if( !MKVALID ) {
            rc = VAX_NOMK;
            break;
        }
        
        flush_blanks( &p );
        if( *p != '<' ) {
            rc = VAX_ASMINVPTE;
            break;
        }
        pte.longword = 0L;
        
        //  Parse the valid bit value
        
        p++;
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;
        
        pte.bit.v = (unsigned int) value.longword;

        flush_blanks( &p );
        if( *p != ',' ) {
            rc = VAX_ASMINVPTE;
            break;
        }

        //  Parse the protection code PTE$K_xxx
            
        p++;
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;
        
        pte.bit.prot = (unsigned int) value.longword;

        flush_blanks( &p );
        if( *p != ',' ) {
            rc = VAX_ASMINVPTE;
            break;
        }

        //  Parse the modify bit
        
        p++;
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;
        
        pte.bit.m = (unsigned int) value.longword;

        flush_blanks( &p );
        if( *p != ',' ) {
            rc = VAX_ASMINVPTE;
            break;
        }

        //  Parse the PFN number.  This is a physical address, which we then
        //  shift right 9 bits to make it be a page number.
        
                
        p++;
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;
        
        pte.bit.pfn = (int)( value.longword >> 9 );

        flush_blanks( &p );
        if( *p != '>' ) {
            rc = VAX_ASMINVPTE;
            break;
        }
        p++;

        count_pc = vax.console.deposit;
        rc = store_memory( count_pc, (void * ) &pte.longword, 4 );
        vax.console.deposit += 4;
        if( rc )
            return rc;
        break;

    case 16:    // .F_FLOAT
    case 17:    // .D_FLOAT
    
        while( running ) {        
            flush_blanks( &p );
            if( isend( *p ))
                break;
            if( *p == ',' ) {
                p++;
            }
            rc = asm_float( &p );
            if( rc  )
                return rc;
            
            rc = fpu_store( vax.last_float, &long1, 
                    ( code == 17 ? /* D_FLOAT */ &long2 : 0L /* F_FLOAT */ ) );
            if( rc )
                return rc;
                
            rc = store_memory( vax.console.deposit, ( void * ) &long1, 4 );
			if( rc )
                return rc;
            vax.console.deposit += 4;

            /* If D_FLOAT then store LSB mantissa as well */
            
            if( code == 17 ) {
                rc = store_memory( vax.console.deposit, ( void * ) &long2, 4 );
				if( rc )
                    return rc;
                vax.console.deposit += 4;
            }
            
        }
        break;

    case 18:    //  .BLKB
    case 19:    //  .BLKW
    case 20:    //  .BLKL, .BLKF
    case 21:    //  .BLKD
    
        /*  Get the count of items */
    
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;

        scale = ( code - 18 );          //  Don't change code table without fixing this!!
        scale = 1 << scale;             //  Create a multiplier 2**scale
        
        value.longword *= scale;        //  How many bytes are we talking about?
        
        /*  Zero out the storage and advance the deposit pointer */
        
        long2 = 0;
        for( n = 0; ( ULONGWORD ) n < value.longword; n++ ) {
            rc = store_memory( vax.console.deposit, ( void * ) &long2, 1 );
			if( rc )
                break;
            vax.console.deposit += 1;     //  Adjust the next deposit accordingly.
        }

        break;


    case 22:    //  .ENTRY


        //  Since we're starting a new .entry, fix the temp names
        //  of the last entry, if any.

       rc =  scope_symbols();
       if( rc )
           return rc;

        //  Copy the name to a local buffer.
        
        n = 0;
        flush_blanks( &p );
        while( !is_blank( *p ) && 
               ( *p == '_' || *p == '$' || isalnum( *p ))
                 && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;
        
        //  Define the symbol as pointing to the current address.  We must
        //  use a pointer to the buffer since the set_symbol code wants to
        //  (re-)parse the symbol name again for us, so we give them a ptr
        //  to play with.  We know we've already advanced the p ptr ourselves.
        
        bp = b;
        
        vax.assembler.flags |= ASM_UNIQUE;
        rc = set_symbol( &bp, vax.console.deposit, SYM_ENTRY );

        if( rc )
            return rc;

        if( vax.assembler.flags & ASM_SCOPENAMES )
        strcpy( vax.assembler.cur_entry, b );
        else
            vax.assembler.cur_entry[ 0 ] = 0;

        //  Now, search for the comma character.
        
        flush_blanks( &p );
        if( *p == ',' ) {
            p++;
            flush_blanks( &p );
        }

        if( !isend( *p )) {
        
            //  Parse out the mask that must follow it.
         
            rc = asm_mask( &p, ( LONGWORD * ) &value.longword );
            if( rc )
                return rc;

        }
        else {
            value.longword = 0;
            rc = VAX_OK;
        }

        count = ( short ) value.longword;
        count_pc = vax.console.deposit;
        
        rc = store_memory( count_pc, (void * ) &count, 2 );

        vax.console.deposit += 2;
        break;


    case 24:  // .CASE base, item, item, item

        /* If this is the first .CASE, then establish the base */

        if( vax.assembler.case_base == 0L )
        vax.assembler.case_base = vax.console.deposit;

        /* Now, scan words.  They are special CASE words */

        while( running ) {
        
            flush_blanks( &p );
            if( isend( *p ))
                break;
            if( *p == ',' ) {
                p++;
            }
            rc = asm_value( &p, &value.longword, K_CASE_W );
            if( rc )
                return rc;
            
            if( value.longword > 0x0000FFFFL )
                return VAX_ASMINVWORD;

#if BIGENDIAN            
            rc = store_memory( vax.console.deposit, 
                        ( void * ) &value.word[ 1 ], 2 );
#else
            rc = store_memory( vax.console.deposit, 
                        ( void * ) &value.word[ 0 ], 2 );
#endif
            if( rc )
                return rc;

            vax.console.deposit += 2;
        }
        break;

    case 25:   /* .SCB code, addr, stack */

        if( !MKVALID ) {
            rc = VAX_NOMK;
            break;
        }

        // Get the vector code.  Usually uses the predefined symbols
        // like ESC$ARITH are used.

        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;

        if( value.longword > 0x000000FF )
            return VAX_ASMINVSCB;

        if(( value.longword & 0x00000003 ) != 0 )
            return VAX_ASMINVSCB;

        // Get the address of the value to write.  We save the deposit addr
        // and then set it up to be the SCB address so we can make forward
        // references work.  If we get an error reading the address, we've
        // got to restore the deposit field.

        long1 = vax.console.deposit;
        vax.console.deposit = 0x80000000 + vax.SCBB + value.longword;

        flush_blanks( &p );
        if( *p == ',' )
           p++;

        rc = asm_value( &p, ( ULONGWORD * ) &value.longword, K_ADDR_L );
        if( rc ) {
            vax.console.deposit = long1;
            return rc;
        }

        if( value.longword != 0xFFFFFFFF ) {
        
        // The address MUST be aligned on a quadword boundary if it's a
        //  real address.  If it's -1 then it's a special code that means
        //  the native console will handle it for us.

            vector = ( ULONGWORD ) value.longword & 0x00000003UL;

            if( vector ) {
                vax.console.deposit = long1;
                return VAX_ASMINVSCB;
            }
        }
        
        // Skip the optional comma and get the stack, if any.

        flush_blanks( &p );
        if( *p == ',' )
           p++;

        if( !isend( *p )) {
           rc = read_verb( &p, &verb2 );
           if( rc ) {
                vax.console.deposit = long1;
                break;
           }
           if( verb2 == CHAR4('I','S','P',' ') )
               value.longword += 1;
           else
           if( verb2 == CHAR4('K','S','P',' ') || 
               verb2 == CHAR4('U','S','P',' ') || 
           verb2 == CHAR4('S','S','P',' ') )
               value.longword += 0;
           else {
               vax.console.deposit = long1;
               return VAX_ASMINVSCB;
           }
        }

        // Write the value to the memory location in the SCB, and restore
        // the deposit location to what it was.

        rc = store_memory( vax.console.deposit, ( void * ) &value.longword, 4 );
        vax.console.deposit = long1;
        break;
        

    case 26:   /* .ALIGN size */

        //  Get the size value
        
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;

        //   Divide by the alignment and multiply again.  This sets
        //   n to be the aligned value LESS than the current base.
        //   If it is less (i.e. not already aligned) then add the
        //   alignment to get aligned value GREATER than base.

        n = vax.console.deposit / value.longword;
        n = n * value.longword;
        if( ( ULONGWORD ) n < vax.console.deposit )
            vax.console.deposit = n + value.longword;
        
        break;

    case 27:   /* .REGION S0 | SYSTEM | P0 | PROCESS */

        if( !MKVALID ) {
            rc = VAX_NOMK;
            break;
        }

        rc = read_verb( &p, &long1 );

        switch( long1 ) {

        case CHAR4('2',' ',' ',' '):
        case CHAR4('S','0',' ',' '):
        case CHAR4('S','Y','S',' '):
        case CHAR4('S','Y','S','T'):
            newrgn = 1;
            break;

        case CHAR4('0',' ',' ',' '):
        case CHAR4('P','0',' ',' '):
        case CHAR4('P','R','O','C'):
            newrgn = 0;
            break;

        default:
            return VAX_ASMINVRGN;
        }

        //  See where we are currently depositing.  If it's the same
        //  as the region we're switching to we have no work to do.

        oldrgn = ( LONGWORD ) vax.console.deposit < 0;
        if( oldrgn == newrgn )
            return VAX_OK;

        //  If we were depositing to P0 then save the current deposit
        //  location there, and swith to the saved S0 location.

        if( oldrgn == 0 ) {
            vax.console.p0_deposit = vax.console.deposit;
            vax.console.deposit = vax.console.s0_deposit;
        }

        //  Otherwise we were depositing to S0 so save the current one
        //  there and switch to the saved P0 location.

        else {
            vax.console.s0_deposit = vax.console.deposit;
            vax.console.deposit = vax.console.p0_deposit;
        }

        rc = VAX_OK;        

        break;

    case 28:   /* .MODE USER | KERNEL | SUPERVISOR | EXEC */

        rc = read_verb( &p, &long1 );
        if( rc )
            break;

        switch( long1 ) {

        case CHAR4('U',' ',' ',' '):
        case CHAR4('U','S','E','R'):
            long1 = AM_USER;
            break;

        case CHAR4('S',' ',' ',' '):
        case CHAR4('S','U','P','E'):
            long1 = AM_SUPERVISOR;
            break;

        case CHAR4('E',' ',' ',' '):
        case CHAR4('E','X','E','C'):
            long1 = AM_EXECUTIVE;
            break;

        case CHAR4('K',' ',' ',' '):
        case CHAR4('K','E','R','N'):
            long1 = AM_KERNEL;
            break;

        }
        set_mode_stack( long1 );
        invalidate_tb_prot();
        break;

    case 29:   /* .VECTOR code, stack */

        if( !MKVALID ) {
            rc = VAX_NOMK;
            break;
        }

        // Get the vector code.  Usually uses the predefined symbols
        // like ESC$ARITH are used.

        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            return rc;

        if( value.longword > 0x000000FF )
            return VAX_ASMINVSCB;

        if(( value.longword & 0x00000003)  != 0 )
            return VAX_ASMINVSCB;

        vector = value.longword;
        long1 = vax.console.deposit;
        vax.console.deposit = 0x80000000 + vax.SCBB + vector;

        flush_blanks( &p );
        if( *p == ',' )
           p++;

        // Skip the optional comma and get the stack, if any.

        flush_blanks( &p );
        if( *p == ',' )
           p++;
        stack = 0;
        if( !isend( *p )) {
           rc = read_verb( &p, &verb2 );
           if( rc ) {
                vax.console.deposit = long1;
                break;
           }
           if( verb2 == CHAR4('I','S','P',' ') )
               stack = 1;
           else
           if( verb2 == CHAR4('E','S','P',' ') || verb2 == CHAR4('K','S','P',' ') || 
               verb2 == CHAR4('U','S','P',' ') || verb2 == CHAR4('S','S','P',' ') )
                   stack = 0;
           else {
               vax.console.deposit = long1;
               return VAX_ASMINVSCB;
           }
        }

        // Get the address of the value to write.  This is the current
        // deposit location aligned on an 8 byte boundary.

        n = long1 / 8;
        n = n * 8;
        if( n < long1 )
            long1 = n + 8;

        // Write the value to the memory location in the SCB, and restore
        // the deposit location to where we start writing the handler.

        value.longword = long1 + stack;
        
        long2 = vax.pslw.cur_mod;
        vax.pslw.cur_mod = 0;

        rc = store_memory( vax.console.deposit, 
               ( void * ) &value.longword, 4 );

        vax.pslw.cur_mod = long2;
        vax.console.deposit = long1;
        break;
        
    case 30:   // .CONSOLE <cmd>

        flush_blanks( &p );
        vax.console.assembler_mode = 0;
        rc = console_dispatch( p );
        vax.console.assembler_mode = 1;
        break;

    case 31:   //  .INCLUDE "file.asm"

        flush_blanks( &p );
        rc = console_asm(  &p );
        break;

    case 32:   //   .IF  expression <stmt>

        rc = asm_expr( &p, &long1 );
        if( rc )
            return rc;

        flush_blanks( &p );
        bp = p;

        read_verb( &p, &verb );
        if( verb != CHAR4('T','H','E','N') )
            p = bp;

        if( long1 == 0L ) {

            while( !isend( *p ))
                p++;
        }

        return assemble( &p );

    case 33:    //  .SHIM libname, offset, routinename, id

        if( !MKVALID ) {
            rc = VAX_NOMK;
            break;
        }

        //  Since we're starting a new .entry, fix the temp names
        //  of the last entry, if any.

       rc =  scope_symbols();
       if( rc )
           return rc;

        //  Copy the name to a local buffer.
        
        n = 0;
        flush_blanks( &p );
        while( !is_blank( *p ) && 
               ( *p == '_' || *p == '$' || isalnum( *p ))
                 && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;
        
        //  Parse the code value.
        
        flush_blanks( &p );
        if( *p == ',' )
            p++;
        flush_blanks( &p );
        if( *p == ',' ) {
            long1 = 0;
            rc = 0;
        }
        else
            rc = asm_expr( &p, &long1 );
        if( rc )
            return rc;

        //  If the code value is non-zero, then this is an XFC shim, and we must
        //  generate the shim stub.
        
        if( long1 ) {
        
            //  Define the symbol as pointing to the current address.  We must
            //  use a pointer to the buffer since the set_symbol code wants to
            //  (re-)parse the symbol name again for us, so we give them a ptr
            //  to play with.  We know we've already advanced the p ptr ourselves.
            
            bp = b;
            
            vax.assembler.flags |= ASM_UNIQUE;
            rc = set_symbol( &bp, vax.console.deposit, SYM_ENTRY );
            shim_addr = vax.console.deposit;

            //  Store the empty entry mask
            
            value.word[ 0 ] = 0;
            rc = store_memory( vax.console.deposit, 
                            ( void * ) &value.word[ 0 ], 2 );
            if( rc )
                return rc;
            vax.console.deposit += 2;


            //  Write the MOVL I^code, R0 statement

            value.byte[ 0 ] = 0xD0;     /* MOVL */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            vax.console.deposit += 1;

            value.byte[ 0 ] = 0x8F;     /* i^# */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            vax.console.deposit += 1;

            /* The code */
            rc = store_memory( vax.console.deposit, ( void * ) &long1, 4 );
			if( rc )
                return rc;
            vax.console.deposit += 4;


            value.byte[ 0 ] = 0x50;     /* R0 */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            vax.console.deposit += 1;


            //  Write the XFC #XFC$SHIM statement
            
            value.byte[ 0 ] = 0xFC;     /* XFC */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            vax.console.deposit += 1;
            
            value.byte[ 0 ] = 0x7D;     /* #XFC$SHIM */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            
            vax.console.deposit += 1;
            
            //  Write the RET statement
            
            value.byte[ 0 ] = 0x04;     /* RET */
            rc = store_memory( vax.console.deposit, ( void * ) &value.byte[ 0 ], 1 );
            if( rc )
                return rc;
            vax.console.deposit += 1;

        }
        
        //  Otherwise it's not a shim, and we need to use the symbol name to get the
        //  actual existing value.
        
        else {
            bp = b;
            rc = get_symbol( &bp, &shim_addr );
            if( rc )
                return rc;
        }
        
        
        //  Parse the runtime library name 
        
        flush_blanks( &p );
        if( *p == ',' )
            p++;

        n = 0;
        flush_blanks( &p );
        while( !is_blank( *p ) && 
               ( *p == '_' || *p == '$' || isalnum( *p ))
                 && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;

        //  Parse the offset in the RTL
        
        flush_blanks( &p );
        if( *p== ',')
            p++;
        rc = asm_expr( &p, &long1 );
        if( rc )
            return rc;

        //  Construct the shim symbol name
            
        sprintf( rtl, "SHIM$%s_%08X", b, long1 );
        
        //  Set the shim symbol to the shim address
        
        rc = set_symbol_direct( rtl, shim_addr, 0 );

        return rc;
    
    case 34:    //  .MICROKERNEL
    
        if( !VMVALID )
            return VAX_NOVMINIT;
            
        vax.console.microkernel_valid = 1;
        rc = VAX_OK;
        break;
    
    
    case 35:    //  .SPACE bytes [, fill ]
                //  (for compatability with GAS sources)
    
        /*  Get the count of items */
    
        flush_blanks( &p );
        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;

        long1 = value.longword;
        flush_blanks( &p );
        if( *p == ',' ) {
            p++;
            rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
            if( rc )
                break;
            ch = ( char ) value.longword;
        }
        else
            ch = 0;
        
        for( n = 0; n < long1; n++ ) {
        
            rc = store_memory( vax.console.deposit, 
                          ( void * ) &ch, 1 );
            if( rc )
                break;
            vax.console.deposit++;
        }
        rc = VAX_OK;
        break;

    case 36:    //  .data <section-number>

        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;

        printf( "Relocation to .data section %u ignored\n", value.longword );
        break;  

    case 37:    //  .text <section-number>

        rc = asm_expr( &p, ( LONGWORD * ) &value.longword );
        if( rc )
            break;

        printf( "Relocation to .text section %u ignored\n", value.longword );
        break;  

    //  Pseudo instructions for conditional JMP instructions.  Generate the inverted
    //  case of the branch around a long JMP to the real destination.
    
    case 38:    //  .JEQL   dest        ; This really turns into BNEQ around a JMP
    case 39:    //  .JNEQ   dest        ; This really turns into BEQL around a JMP
        
        if( code == 38 )
            ch = 0x12;  /* BNEQ */
        else
        if( code == 39 )
            ch = 0x13;  /* BEQL */
            
        rc = store_memory( vax.console.deposit++, ( unsigned char * ) &ch, 1 );
        if( rc )
            return rc;
        
        ch = 0x06;  /* displacement around the branch */
        rc = store_memory( vax.console.deposit++, ( unsigned char * ) &ch, 1 );
        if( rc )
            return rc;
        
        ch = 0x17;  /* JMP */
        rc = store_memory( vax.console.deposit++, ( unsigned char * ) &ch, 1 );
        if( rc )
            return rc;
        
        ch = ( char ) 0x9FU; /* @# addressing mode */
        rc = store_memory( vax.console.deposit++, ( unsigned char * ) &ch, 1 );
        if( rc )
            return rc;
        
        /* Parse the destination.  We do it here so fixups work if it's */
        /*  a forward reference (often the case). */
        
        disp = K_ADDR_BASE + 4;     /* Absolute address fixup (longword size) */
        rc = asm_value( &p, &value.longword, disp ); 
        if( rc )
            return rc;

        rc = store_memory( vax.console.deposit, ( unsigned char * ) &value.longword, 4 );
        if( rc )
            return rc;

        vax.console.deposit += 4;
        break;

    case 40:  //  .P1VECTOR   ; Generate P1 vector for system services

        return p1_init();

    case 41:  //  .SCOPE


        //  Since we're starting a new scope, fix the temp names
        //  of the last entry, if any.

       rc =  scope_symbols();
       if( rc )
           return rc;

        //  Copy the name to a local buffer.
        
        n = 0;
        flush_blanks( &p );
        while( !is_blank( *p ) && 
               ( *p == '_' || *p == '$' || isalnum( *p ))
                 && !isend( *p )) {
        
            ch = *(p++);
            if( ch >= 'a' && ch <= 'z' )
                ch = ( char ) ( ch - 32 );
            b[ n++ ] = ch;
            
        }
        b[ n ] = 0;
        
        //  Define the symbol as pointing to the current address.  We must
        //  use a pointer to the buffer since the set_symbol code wants to
        //  (re-)parse the symbol name again for us, so we give them a ptr
        //  to play with.  We know we've already advanced the p ptr ourselves.
        
        bp = b;
        
        vax.assembler.flags |= ASM_UNIQUE;
        rc = set_symbol( &bp, vax.console.deposit, SYM_LABEL );

        if( rc )
            return rc;

        if( vax.assembler.flags & ASM_SCOPENAMES )
        strcpy( vax.assembler.cur_entry, b );
        else
            vax.assembler.cur_entry[ 0 ] = 0;

        break;


    }

    
    return rc;
    
}
