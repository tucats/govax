//
//  Copyright (C) 1997,1998,1999,2000,2001 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     service.c
//
//  Purpose:    Runtime support for VMS system services.
//
//  History:    11/08/99    Created.


#include "vax.h"
#include "shim.h"
#include "services.h"
#include "ss_def.h"
#include "dclrtl.h"

#define JPI__ACCOUNT    515	/* Always "USER" */
#define JPI__CLINAME    522	/* Always "EVAX" */

LONGWORD local_ef[ 4 ];
LONGWORD vms_exit_handler = 0L;
LONGWORD vms_ast_flag = 0L;

long set_region_size( long region, LONGWORD size );
long get_region_size( long region, LONGWORD * size );


SSDEF( sys_setast )
{
    vms_ast_flag = ( char ) argv[ 0 ];
    return SS_NORMAL;
}

SSDEF( sys_dclexh )
{

    vms_exit_handler = argv[ 0 ];
    return SS_NORMAL;

}

SSDEF( sys_expreg )
{
    LONGWORD retadr;
    LONGWORD size, start, end;
    LONGWORD mode, region;
    LONGWORD rc;

    mode = argv[ 2 ];
    region = argv[ 3 ];

    if( region < 0 || region > 2 )
        return SS_INVARG;

    /* Can't be more privileged than yourself */

    if( mode < vax.pslw.cur_mod )
        mode = vax.pslw.cur_mod;

    size = argv[ 0 ] * 512;   /* How many bytes? */

    /* Use the P0 region size to get more memory */

    get_region_size( region, &start );
    end = start + size - 1;
    set_region_size( region, end+1 );

    retadr = argv[ 1 ];
    if( retadr ) {

        rc = store_memory( retadr, ( void * ) &start, 4 );

        rc = store_memory( retadr + 4, ( void * ) &end, 4 );
    }
    vax.R1 = start; /* Expected side effect */
    
    return SS_NORMAL;
}


SSDEF( sys_clref )
{

    int slot, bit;

    argv[ 0 ] %= 0x00FF;
    slot = (int) argv[ 0 ] / 32;
    bit = (int) argv[ 0 ] & 0x1F;

    local_ef[ slot ] &= ~( 1<<bit);

    return SS_NORMAL;
}


SSDEF( sys_setef )
{
    int slot, bit;

    argv[ 0 ] %= 0x00FF;
    slot = (int) argv[ 0 ] / 32;
    bit = (int) argv[ 0 ] & 0x1F;

    local_ef[ slot ] |= ( 1<<bit);

    return SS_NORMAL;
}


SSDEF( sys_readef )
{

    int slot, bit, state;
    LONGWORD addr, rc;

    argv[ 0 ] &= 0x00FF;

    slot = (int) argv[ 0 ] / 32;
    bit = (int) argv[ 0 ] & 0x1F;

    if( argc == 2 ) {
        addr = argv[ 1 ];
        rc = store_memory( addr, ( void * ) &( local_ef[ slot ]), 4 );
        if( rc ) {
            return SS_ACCVIO;
        }
    }

    state = ( local_ef[ slot ] & ( 1 << bit )) ? SS_WASSET : SS_WASCLR;
    return state;
}

/*
 *	SYS$GETJPIW system service
 */

SSDEF( sys_getjpiw )
{
    char prcname[ 64 ];
    int size;
    int	efn;
    LONGWORD pid;
    short retsize;
    LONGWORD ptr, buffaddr, retaddr;
    short itemcode, bufflen;
    long rc, debug;

    debug = vax.debug & DBG_PROCESS;
    
    if( argc != 7 )
        return SS_INSFARG;
    
    efn = (int) argv[ 0 ];
    pid = (int) argv[ 1 ];
    if( pid ) {
        rc = load_memory( pid, ( void * ) &pid, 4 );
        if( rc )
            return SS_ACCVIO;
    }
    
    if( argv[ 2 ] ) {
        size = 63;
        str_get( argv[ 2 ], &size, prcname );
        prcname[ size ] = 0;
    }
    else
        size = 0;
        
    
/*  Scan the item list looking for things we know how to process. */

    ptr = argv[ 3 ];

    while( 1 ) {
    
        /* Get the next item list entry elements */
        
        rc = load_memory( ptr, ( void * ) &bufflen, 2 );
        if( rc )
            return SS_ACCVIO;
        rc = load_memory( ptr+2, ( void * ) &itemcode, 2 );
        if( rc )
            return SS_ACCVIO;
        
        /* If this is the end of the list then break out */
        if( bufflen == 0 && itemcode == 0 )
            break;
        
        rc = load_memory( ptr+4, ( void * ) &buffaddr, 4 );
        if( rc )
            return SS_ACCVIO;
        
        rc = load_memory( ptr+8, ( void * ) &retaddr, 4 );
        if( rc )
            return SS_ACCVIO;
        
        
        switch( itemcode ) {
       
         case JPI__ACCOUNT:
        
            if( debug ) 
                printf( "DEBUG: SYS$GETJPI returns ACCOUNT=\"USER\"\n" );
                
            rc = store_string( "USER    ", buffaddr, 8 );
            if( rc )
                return SS_ACCVIO;
            if( retaddr ) {
                retsize = 4;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
        
        case JPI__CLINAME:
        
            if( debug )
                printf( "DEBUG: SYS$GETJPI returns CLINAME=\"DCL\" in loc %08X\n", buffaddr );
            
            rc = store_string( "DCL\0", buffaddr, 4 );
            if( rc )
                return SS_ACCVIO;
            if( retaddr ) {
                retsize = 3;
                rc = store_memory( retaddr, ( void * ) &retsize, 2 );
                if( rc )
                    return SS_ACCVIO;
            }
            break;
            
        default:
            if( debug ) 
                printf( "DEBUG: SYS$GETJPIW found unrecognized item code %d\n",
                         itemcode );
            return SS_BADPARAM;
        }
    
        /* Advance to next item in itmlst */
        
        ptr = ptr + 12;
    }
    
    return SS_NORMAL;
    
}
