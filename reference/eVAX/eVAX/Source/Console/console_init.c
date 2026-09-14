
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     console_init.c
//
//  Purpose:    This module implements the INIT and INIT/ROM commands
//
//
//  History:    09/23/99    New header standardization.  Also added INIT/ROM command.
//
//              09/26/99    Set console symbols CONSOLE$ROM_BASE, etc. when ROM or
//                          NVRAM is loaded.


#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"
LONGWORD init_rom( char ** P, int flag );



LONGWORD console_init( char ** P )
{
    LONGWORD saved_radix, saved_disasm, saved_verify, rc;
    ULONGWORD value;
    char * p;
    LONGWORD verb;
    
    p = *P;
    flush_blanks( &p );
    rc = read_verb( &p, &verb );
    if( rc == VAX_OK && verb == CHAR4('/','R','O','M')) {
        *P = p;
        return init_rom( P, 0 /* ROM */ );
    }

    if( rc == VAX_OK && verb == CHAR4('/','N','V','R')) {
        *P = p;
        return init_rom( P, 1 /* NVRAM */ );
    }

    p = *P;
        
    rc = asm_hex( &p, &value );
    if( rc ) {
        *P = p;
        return rc;
    }
        
    value = value * 512;
    
    saved_radix = vax.console.radix;
    saved_disasm = vax.console.disasm;
    saved_verify = vax.console.verify;
    
    free_vax( &vax );
    rc = alloc_vax( value );
    if( rc ) {
        return rc;
    }
        
    vax.console.radix = saved_radix;
    vax.console.disasm = saved_disasm;
    vax.console.verify = saved_verify;
    
    /* This zeros memory AND will also clear all non-permanent symbols */

    rc = console_zero( &p );
    *P = p;
    return rc;
}

/*
 *  INIT/ROM /BASE=<address> /SIZE=<kilobytes>
 *  INIT/NVRAM /BASE=<address> /SIZE=<kilobytes>
 *
 *  Flag    0 - ROM
 *          1 - NVRAM
 *
 */

LONGWORD init_rom( char ** P, int flag )
{
    extern char * nvram;
    extern char * rom;
    extern char * rom_name, * nvram_name;
    char * p;
    int parsing, base_set, size_set;
    ULONGWORD base, size, data, paddr;
    LONGWORD rc, verb;
    char * kind;
    char ** ptr;    /* Handle to whatever we are initializing */
    char * buff;
    
    p = *P;
    if( !vax_init )
        return VAX_NOVAX;
    if( vax.pslw.cur_mod > 0 )
        return VAX_NOTKERNEL;

    flush_blanks( &p );
    parsing = ( *p == '/' );
    
/*
 *  Look for qualifiers. 
 *
 *      /BASE=base-address
 *      /SIZE=size (in Kilobytes)   
 */
 
    base_set = size_set = 0;
    
    if( flag == 0 ) {
        base = 0x20040000;                  /* Default base address */
        size = 256;                         /* Default is 256K ROM size */
        kind = "ROM";
        ptr = &rom;
    }
    else {
        base = 0x20140400;                  /* Default base of NVRAM */
        size = 1;                           /* Default is 1K NVRAM */
        kind = "NVRAM";
        ptr = &nvram;
    }
    
    while( parsing ) {
    
        flush_blanks( &p );
        if( isend( *p )) {
            parsing = 0;
            continue;
        };
        
        if( *p != '/' ) {
            parsing = 0;
            continue;
        }
        
        rc = read_verb( &p, &verb );
        if( rc )
            return rc;
        
        flush_blanks( &p );
        if( *p != '=' ) {
            return VAX_SYNTAX;
        }
        p++;
        
    
        rc = asm_hex( &p, ( ULONGWORD * ) &data );
        if( rc )
            return rc;
        
        if( verb == CHAR4( '/','B','A','S' )) {
            base = data;
            base_set = 1;
        }
        else
        if( verb == CHAR4( '/','S','I','Z' )) {
            size = data;
            size_set = 1;
        }
        else {
            return VAX_SYNTAX;
        }
    }
    
    
        
    
    if( base < vax.memsize ) {
        printf( "%s address space overlaps physical memory (last physical address is %08X\n",
                    kind, vax.memsize );
        return VAX_BADROM;
    }
    if( size > 2048 ) {
        printf( "%s size too large (max 2MB)\n", kind );
        return VAX_BADROM;
    }
    
    /* Destroy the old storage if any */
    
    if( *ptr )
        freemem( *ptr );
            
    /* Set up for the new storage */
    
    size = size * 1024;  /* Convert kilobytes to bytes */
    
    *ptr = getmem( size );
    if( *ptr  == 0L ) {
        printf( "Cannot allocate memory for %s\n", kind );
        return VAX_MEM;
    }
    
    
    *P = p;
    
    /* Zero the memory */
    
    buff = *ptr;
    for( paddr = 0; paddr < size; paddr++ )
        buff[ paddr ] = 0;
    
    if( flag == 0 ) {
        vax.rom_base = base;
        vax.rom_end = base + size - 1;
        strcpy( rom_name, "<empty>" );
        set_symbol_direct( "CONSOLE$ROM_BASE", vax.rom_base, 0 );
        set_symbol_direct( "CONSOLE$ROM_END", vax.rom_end, 0 );
        set_symbol_direct( "CONSOLE$ROM_SIZE", size, 0 );
    }
    else {
        vax.nvram_base = base;
        vax.nvram_end = base + size - 1;
        strcpy( nvram_name, "<empty>" );
        set_symbol_direct( "CONSOLE$NVRAM_BASE", base, 0 );
        set_symbol_direct( "CONSOLE$NVRAM_END", vax.nvram_end, 0 );
        set_symbol_direct( "CONSOLE$NVRAM_SIZE", size, 0 );
    }
    
    if( vax.console.flags & CONSOLE_VERBOSE )
        printf( "Empty %s created from %08X to %08X\n", kind, base, base + size - 1 );
    

    return VAX_OK;
}
