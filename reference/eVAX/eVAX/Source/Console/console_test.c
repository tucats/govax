//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_test.c
//
//  Purpose:    This module implements the TEST command which implements a variety
//              of low-level console hooks.  Some of these are undocumented to the
//              end user!
//
//  History:    02/18/98        Initial implementation
//
//              11/17/99        Make DCL compatible.
//
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
#include "vaxinstr.h"
#include "fpu.h"
#include "dclrtl.h"

LONGWORD console_test_dcl( long id )
{
    LONGWORD rc, value;
    char * p;
    FILE * fp;
    LONGWORD i, j, k, m;
    LONGWORD scale[ 6 ];
    
    LONGWORD long0, long1;
    double dbl;
    
    
    static char * instruction_table = "instruction_table.h";
    
    static char * mode_name[] = {
        "OP_NL", "OP_RD", "OP_WR", "OP_MD", "OP_AD", "OP_VA", "OP_BR"};
        
    /* vax = *Vax;  -- now use global vax */
    p = DCLgetstring( DCL_CALLBACK_PARAMETER, 161 );
    i = id; /* Dummy statement */

//  If no we have no work to do.
    
    if( !vax_init ) {
        rc = VAX_NOVAX;
        return rc;
    }

    
    
//  Find out what test operation we are doing.

    rc = asm_dec( &p, &value );
    if( rc != VAX_OK )
        return rc;
    
//  Based on the test code, do work...

    switch( value ) {
    

    case 11004: /* Toggle VM tracing on-off */
    
        vax.console.vmtrace = vax.console.vmtrace ? 0 : 1;
        printf( "VM translation tracing %s\n",
                vax.console.vmtrace ? "enabled" : "disabled" );
        break;
                        
    case 11003: /* Toggle TB CACHE on-off */
    
        vax.console.tbcache = vax.console.tbcache ? 0 : 1;
        printf( "VM translation buffer caching %s\n",
                vax.console.vmtrace ? "enabled" : "disabled" );
        break;
    
    case 11002: /* Convert .2 to D_FLOAT and store it */
    
        dbl = 0.2;
        rc = fpu_store( dbl, &long0, &long1 );
        rc = store_memory( 0x200, ( void * ) &long0, 4 );
        rc = store_memory( 0x204, ( void * ) &long1, 4 );
        
        printf( "TEST 11002: EXAM/BYTE 0200 for results\n" );
        
        break;
        
    case 11001:
        
        fp = xfopen( instruction_table, "w" );
        if( fp == 0L ) {
            printf( "Cannot open file \"%s\"\n", instruction_table );
            return VAX_OK;
        }
        
        fprintf( fp, "/*\n" );
        fprintf( fp, " *\t%s\n *\n *\n", instruction_table );
        fprintf( fp, " *\tThis header file contains the table of VAX instructions and what the\n" );
        fprintf( fp, " *\tEmulator must know about each of them.  This table was originally \n" );
        fprintf( fp, " *\tcreated by hand, but is now generated _by_ the emulator itself with\n" );
        fprintf( fp, " *\ta TEST 11001 console command.  This allows us to modify the structure\n" );
        fprintf( fp, " *\tof the table easily without hand-editing the entire file.\n *\n" );
        fprintf( fp, " *\tTHEREFORE, DO NOT CHANGE THIS FILE BY HAND, BUT INSTEAD MODIFY THE\n" );
        fprintf( fp, " *\tBEHAVIOR OF THE console_test.c MODULE TO GENERATE THE NEW DATA!\n */\n\n" );
        
        fprintf( fp, "struct INSTRUCTION instruction[] = {\n" );
        
        for( i = 0; i < 500; i++ ) {
        
            j = 0; k = 0;
            
            if( instruction[ i ].name[ 0 ] == 0 )
                break;
            
            for( m = instruction[ i ].operand_count; m < 6; m++ )
                scale[ m ] = 0;
                
            fprintf( fp, "\n{   /* %-8s */\n", instruction[ i ].name );
            fprintf( fp, "\t\t{ %2d, %2d, %2d, %2d, %2d, %2d },   /*  operand scales */\n", 
                            scale[ 0 ],
                            scale[ 1 ],
                            scale[ 2 ],
                            scale[ 3 ],
                            scale[ 4 ],
                            scale[ 5 ]
                            );
            fprintf( fp, "\t\t%-14s  /* data type for short literals  */\n",
                            instruction[ i ].type ? "OP_TYPE_FLOAT," : "OP_TYPE_INT,  " );
            
            fprintf( fp, "\t\t0L,             /* Entry point for emulator      */\n" );
            fprintf( fp, "\t\t0x%02X, 0x%02X,     /* Opcode (extended+normal)      */\n",
                            instruction[ i ].extended,
                            instruction[ i ].opcode );
            fprintf( fp, "\t\t%1d,              /* Number of operands            */\n", 
                            instruction[ i ].operand_count );
            
            fprintf( fp, "\t\tACCESS( " );
            
            for( j = 0; j < 6; j++ ) {
            
                k = GETMODE( instruction[ i ].access, j );
                
                fprintf( fp, "%s", mode_name[ k ] );
                if( j < 5 )
                    fprintf( fp, ", " );
            }
            
            fprintf( fp, " ),\n" );
            fprintf( fp, "\t\t\"%s\"\n", instruction[ i ].name );
            fprintf( fp, "},\n" );
            
            
        }
        fprintf( fp, "\n/* End of array denoted by null name */\n" );
        fprintf( fp, "{ { 0, 0, 0, 0, 0, 0 }, 0, 0L, 0X00, 0X00, -1,    -1, \"\"    }};\n" );
            
        fclose( fp );
        
        printf( "Wrote %d instruction descriptors to file \"%s\"\n", 
                    i, instruction_table );
        
        break;
        
        
        
    default:
        return VAX_UNKTEST;
    }
    
    return VAX_OK;
}
