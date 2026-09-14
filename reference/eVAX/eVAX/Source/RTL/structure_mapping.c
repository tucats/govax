//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     structure_mapping.c
//
//  Purpose:    Functions for mapping VAX memory-based structures into native
//		data structures, for support of runtime functions.
//
//  History:    12/05/2001	Created from storage.c modules.
//
//		12/05/2001	Added field mapping, highwater marks.


#include "vax.h"
#include "memmap.h"
#include "ss_def.h"


LONGWORD map_dump( void )
{

    struct MAP * mp;
    struct MAPLIST * ml;
    
    if( maps )
        printf( "STRUCTURE MAPS:\n\n" );
    else {
        printf( "No structures mapped.\n" );
        return VAX_OK;
    }
    
    for( mp = maps; mp; mp = mp-> next ) {
    
        printf( "    %-32s\n", mp-> name );
        if( !mp-> list )
            printf( "        <no members>\n" );
        else
            printf( "        %-32s  %4s %4s   Size Type\n",
                "Name", "Nat.", "VAX" );
                
        for( ml = mp-> list; ml; ml = ml-> next ) {
        
            printf( "        %-32s  %04X  %04X  %3d    %d\n",
                ml-> name, ml-> p_offset, ml-> v_offset, ml-> size, ml-> kind );
                
        }
        printf( "\n" );
    }

    return VAX_OK;
}


LONGWORD map_init( char * name )
{

    struct MAP * mp;

    if( find_map( name ))
        return VAX_MAPEXISTS;
    
    mp = ( struct MAP * ) getmem( sizeof( struct MAP ));

    mp-> next = maps;
    maps = mp;
    
    strcpy( mp-> name, name );
    mp-> list = 0L;
    mp-> highwater = 0L;
    
    return VAX_OK;
}


struct MAP * find_map( char * name )
{

    struct MAP * mp;
    
    for( mp = maps; mp; mp = mp-> next )
        if( strcmp( mp-> name, name ) == 0L )
            return mp;
    
    return 0L;
}

LONGWORD map_int( char * name, char * member, LONGWORD struct_offset, short size )
{
    return map_add( name, member, -1, struct_offset, size, 0 );
}

LONGWORD map_chr( char * name, char * member, LONGWORD struct_offset, short size )
{
    return map_add( name, member, -1, struct_offset, size, 1 );
}



LONGWORD map_add( char * name, char * member, 
              LONGWORD vax_offset, LONGWORD struct_offset, short size, short kind )
{

    struct MAP * mp;
    struct MAPLIST * ml;
    
    mp = find_map( name );
    if( !mp )
        return VAX_MAPNF;
    
    for( ml = mp-> list; ml; ml = ml-> next ) {
    
        if(strcmp( ml-> name, member ) == 0L )
            return VAX_MAPEXISTS;
    }
    
    ml = ( struct MAPLIST * ) getmem( sizeof( struct MAPLIST ));
    ml-> next = mp-> list;
    mp-> list = ml;
    strcpy( ml-> name, member );
    ml-> p_offset = struct_offset;
    
    if( vax_offset == MAPNEXT ) {
        vax_offset = mp-> highwater;
        mp-> highwater += size;
    }
    ml-> v_offset = vax_offset;
    ml-> size = size;
    ml-> kind = kind;
    
    return VAX_OK;

}

LONGWORD store_field( LONGWORD base, char * str_name, char * field_name, void * addr, short len )
{

    return store_memory( base + map_offset( str_name, field_name ), addr, len );
}


LONGWORD map_offset( char * name, char * field )
{
    struct MAPLIST * ml;
    struct MAP * mp;
    
    mp = find_map(name );
    if( !mp )
        return -1;
    
    for( ml = mp-> list; ml; ml = ml-> next ) {
        if( strcmp( ml-> name, field ) == 0 ) {
            return ml-> v_offset;
        }
    }
    printf( "ERROR: Bogus field map for %s.%s\n", name, field );
    return -1;
}

LONGWORD map( char * name, void * dest, ULONGWORD addr )
{

    struct MAP * mp;
    struct MAPLIST * ml;
    char * p;
    LONGWORD rc, n;
    
    mp = find_map( name );
    if( !mp ) 
        return VAX_MAPNF;
    
    for( ml = mp-> list; ml; ml = ml-> next ) {
    
        switch( ml-> kind ) {
        
        case 0:     /* Integer */
                p = ( char * ) dest + ml-> p_offset;    
                rc = load_memory( addr + ml-> v_offset, ( void * ) p, ml-> size );
                if( rc )
                    return rc;
                break;
        
        case 1:     /* Character */
        
                for( n = 0; n < ml-> size; n++ ) {
                    p = ( char * ) dest + ml-> p_offset + n;
                    rc = load_memory( addr + ml->v_offset + n, ( void * ) p, 1 );
                    if( rc )
                        return rc;
                }
                break;
        
        default:
                printf( "MMAP Error, field of wrong type!\n" );
                break;
        }
    }
    
    return VAX_OK;
}

