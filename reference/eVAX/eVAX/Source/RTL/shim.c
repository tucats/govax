//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     shim.c
//
//  Purpose:    This module contains the code that manages runtime library shims.
//              When a RUN command is used to invoke a VMS image file, it wil usually
//              contain references to runtime libraries.  These can be resolved by
//              "software shims" which represent the real runtime routines, but
//              transfer control here instead.
//
//              To create a shim, define the entry point for the routine as a standard
//              VAX routine.  Many of these are defined in the kernel.asm file, for
//              example.
//
//              Then, create a shim label, of the form SHIM$<library>_<vector> where the
//              name is a string with the library name (LIBRTL, DECC_SHR, etc.) and
//              the vector entry expressed as a hexadecimal number.  For example,
//              the LIB$PUT_OUTPUT routine is at vector offset 00000478 in the LIBRTL
//              sharable image.  So the shim would define SHIM$LIBRTL_00000478 as the
//              symbolic name of the shim, and equate it to the LIB$PUT_OUTPUT routine
//              defined in the microkernel.
//
//              Note that to ensure that SHIM names are created as system symbols, they
//              have the SHIM$ prefix added.
//
//              When a shim runs, it may want to implement the code in native as
//              in the LIB$PUT_OUTPUT entry, or may want to vector it to a native
//              runtime in the emulator.  For this purpose, a special XFC entry is
//              used, XFC XFC$SHIM
//
//              When the XFC handler sees this, it transfers control to this dispatch
//              routine.  The dispatcher uses the contents of R0 to define which run
//              time entry is to be invoked.  This is called, and can use any of the
//              information in the virtual machine state to do it's work.
//
//              When the shim handler returns, control resumes on the instruction that
//              follows the XFC call, assuming that the runtime has set any required
//              resulting state information in the machine (return codes, etc.).
//              
//
//  History:    07/14/99    Initially created.
//
//              11/01/99    Adding more CRTL functions so I can write test programs.
//


#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "memmap.h"         // to instantiate memory map list headers (uses GLOBAL)
#include "shim.h"



struct RTL_ENTRY rtl_entry_list[] = {
{   0L, "<null>"        },
{   0L, "LIB$ADAWI"     },      /*      1 */
{   0L, "STR$UPCASE"    },
{   0L, "EXE$INPUT"     },
{   0L, "EXE$OPEN"      },      /*      4 */
{   0L, "EXE$CLOSE"     },      /*      5 */
{   0L, "EXE$READ"      },      /*      6 */
{   0L, "EXE$WRITE"     },      /*      7 */
{   0L, "DECC$PRINTF"   },      /*      8 */
{   0L, "DECC$SPRINTF"  },      /*      9 */
{   0L,	"DECC$STRCMP"	},      /*     10 */
{   0L, "DECC$STRNCMP"  },      /*     11 */
{   0L, "DECC$STRNCPY"  },      /*     12 */
{   0L, "DECC$ATOI"     },      /*     13 */
{   0L, "DECC$GETS"     },      /*     14 */
{   0L, "DECC$MALLOC"   },      /*     15 */
{   0L, "DECC$FREE"     },      /*     16 */

{   0L, "DECC$ISALNUM"  },      /*     17 */
{   0L, "DECC$ISALPHA"  },      /*     18 */
{   0L, "DECC$ISCNTRL"  },      /*     19 */
{   0L, "DECC$ISDIGIT"  },      /*     20 */
{   0L, "DECC$ISGRAPH"  },      /*     21 */
{   0L, "DECC$ISLOWER"  },      /*     22 */
{   0L, "DECC$ISPRINT"  },      /*     23 */
{   0L, "DECC$ISPUNCT"  },      /*     24 */
{   0L, "DECC$ISSPACE"  },      /*     25 */
{   0L, "DECC$ISUPPER"  },      /*     26 */
{   0L, "DECC$ISXDIGIT" },      /*     27 */
{   0L, "DECC$ISASCII"  },      /*     28 */

{   0L, "LIB$GET_VM"    },      /*     29 */
{   0L, "LIB$FREE_VM"   },      /*     30 */
{   0L, "LIB$DELETE_VM_ZONE" }, /*     31 */

{   0L, "DECC$TIME",    },      /*     32 */

{   0L, "<end-of-list>" }};

#define MAXSHIM (( sizeof( rtl_entry_list ) / sizeof( struct RTL_ENTRY )) - 1 )




LONGWORD shim_init( void )
{

    shim_declare( 1, lib_adawi );
    shim_declare( 2, str_upcase );
    shim_declare( 3, exe_input );
    shim_declare( 4, exe_open );
    shim_declare( 5, exe_close );
    shim_declare( 6, exe_read );
    shim_declare( 7, exe_write );
    shim_declare( 8, decc_printf );
    shim_declare( 9, decc_sprintf );
    shim_declare( 10, decc_strcmp );
    shim_declare( 11, decc_strncmp );
    shim_declare( 12, decc_strncpy );
    shim_declare( 13, decc_atoi );
    shim_declare( 14, decc_gets );

    shim_declare( 15, decc_malloc );
    shim_declare( 16, decc_free );

    shim_declare( 17, decc_isalnum );
    shim_declare( 18, decc_isalpha );
    shim_declare( 19, decc_iscntrl );
    shim_declare( 20, decc_isdigit );
    shim_declare( 21, decc_isgraph );
    shim_declare( 22, decc_islower );
    shim_declare( 23, decc_isprint );
    shim_declare( 24, decc_ispunct );
    shim_declare( 25, decc_isspace );
    shim_declare( 26, decc_isupper );
    shim_declare( 27, decc_isxdigit );
    shim_declare( 28, decc_isascii );

    shim_declare( 29, lib_get_vm );
    shim_declare( 30, lib_free_vm );
    shim_declare( 31, lib_delete_vm_zone );

    shim_declare( 32, decc_time );

    return VAX_OK;
}




LONGWORD shim_declare(  LONGWORD  code, CALLV entry )
{

    if( code < 0 || code > MAXSHIM ) {
        printf( "Internal error initializing SHIM table; unimplemented SHIM code %04X\n",
            code );
        return set_fault( EXC_RESOP, 0 );
    }
    
    rtl_entry_list[ code ].addr = entry;
    
    return VAX_OK;
}


LONGWORD shim_dump(void)
{

    struct SYMBOL * s, *sp;
    LONGWORD first = 1;
    struct SYMBOL * find_label( LONGWORD dest, LONGWORD flags );

    for( s = system_symbols; s; s = s-> next ) {

        if( strncmp( s-> name, "SHIM$", 5 ) != 0L )
            continue;
        
        if( first ) {
            first = 0;
            printf( "RTL SHIMS:\n" );
        }
        
        sp = find_label( s-> value, SYM_ENTRY );
        if( !sp )
            sp = find_label( s-> value, SYM_LABEL );
            
        if( sp )
            printf( "    %-32s = %08X  %s\n", s-> name, s-> value, sp-> name );
        else
            printf( "    %-32s = %08X\n", s-> name, s-> value );
        
    }   
    
    if( first )
        printf( "No RTL shims defined.\n" );
        
    return VAX_OK;
}

LONGWORD shim(void)
{
    LONGWORD    code;
    LONGWORD    argc;
    LONGWORD    *argv;
    LONGWORD    size;
    LONGWORD    rc, n;
    
    LONGWORD    (*callv)( LONGWORD, LONGWORD * );

    code = vax.R0;

/*
 *  Make sure it's implemented
 */

    if( code < 0 || code > MAXSHIM ) {
        printf( "Invalid SHIM invocation (%04X)\n", code );
        return set_fault( EXC_RESOP, 0 );
    }
    
    if( rtl_entry_list[ code ].addr == 0L ) {
        printf( "Unimplemented SHIM invocation (%04X)\n", code );
        return set_fault( EXC_RESOP, 0 );
    }
    
/*
 *  Find out how many arguments there are
 */

    rc = load_memory( vax.AP, ( void * ) &argc, 4 );
    if( rc )
        return rc;

/*
 *  Allocate an argument array natively for them.
 */

    size = sizeof( LONGWORD ) * argc;
    argv = ( LONGWORD * ) getmem( size );
    if( argv == 0L )
        return VAX_MEM;

/*
 *  Get the arguments locally in native format
 */

    for( n = 0; n < argc; n++ ) {
    
        rc = load_memory( ( vax.AP ) + (( n + 1 ) * 4 ), ( void * ) &( argv[ n ]), 4 );
        if( rc )
            return rc;
        
    }

    
    callv =  rtl_entry_list[ code ].addr;
    rc = (*callv)( argc, argv );
    
    vax.R0 = rc;
    freemem( argv );


    return VAX_OK;
}


/*
 *  Initialize the runtime library environment.   This is the equivalent of
 *  calling the RTL initialization entry points.
 */

LONGWORD lib_initialize(void)
{

    decc_init_memory();                 /* Initialize the memory manager */

    return VAX_OK;
}

