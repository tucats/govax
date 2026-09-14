
//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, All Rights Reserved
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     save_binary.c
//
//  Purpose:    This module implements the SAVE command, writes the
//              current machine state to a binary file.  This is 
//              different than console_save.c which implements the
//              SAVE TEXT command.
//
//  History:    02/09/98        First implementation.
//
//              09/22/99        Added SAVE/ROM to save the ROM memory
//                              area as a sectioned binary save file.
//
//              09/26/99        Added SAVE/NVRAM as well.
//

#include "vax.h"
#include "console_proto.h"
#include "asmproto.h"

#include <stdio.h>
#include <time.h>


/*
 *  Define the version number for the current file format we write out.
 */
 
#define VERSION_MAJOR   2
#define VERSION_MINOR   1


int swap_bytes( void * addr, int size );

/*
 *  Save the entire VAX in a binary file image.
 */

LONGWORD save_binary( char ** P )
{
    LONGWORD pages, page, j, k, found, base;
    char * p, *fn;
    FILE * fp;
    LONGWORD n, quoted;
    char buffer[ 256 ];
    
    
/*
 *  Fetch the file name.  If it's missing, then we accept that and
 *  create a default name.
 */
 
    p = *P;
    flush_blanks( &p );
    if( isend( *p )) 
        fn = "Saved.VAX";
    else
        fn = p; 

/*
 *  Clean up the name, we get all kinds of crap.
 */
 
    if( *fn == '\"' ) {
        quoted = 1;
        fn++;
    }
    else
        quoted = 0;


    for( n = 0; fn[ n ]; n++ ) {
    
        if( fn[ n ] == '\n' || fn[ n ] == '\"' )
            fn[ n ] = '\0';
    }

//  If the file name was not quoted, there may be trailing blanks to
//  be removed.

    if( !quoted ) 
        for( n = n - 1; p[ n ] == ' '; n-- )
            p[ n ] = '\0';


//  Since we're going to replace the file, just remove it if it already
//  exists.  We don't care about return codes here.

    remove( fn );

/*
 *  Try to open the file.  If we can't then bail out.
 */
 
    fp = fopen( fn, "w" );
    if( fp == 0L ) {
        printf( "Can't open file \"%s\"\n.", p );
        return VAX_OK;
    }

/*
 *  Write out a header record that marks the file as unreadable by
 *  an include file.  This is to prevent a user from using SAVE
 *  to create an include file when he meant SAVE AS.
 *
 *  Note that this header is exactly 8 bytes LONGWORD!
 */
 
    fputs( ";//EOF*\n", fp );

/*
 *  Write out a version number for the file format.  Later we'll use
 *  this to make sure we handle files correctly!
 */

    buffer[ 0 ] = VERSION_MAJOR;
    buffer[ 1 ] = VERSION_MINOR;

    /* Write out -2- instances of size -1- bytes */

    fwrite( buffer, 2, 1, fp );

/*
 *  Write out sizeof(struct VAX) as a pinned 4-byte field (not
 *  sizeof(LONGWORD), which no longer tracks the struct layout).  A loader
 *  built against a different `struct VAX` layout can then detect the
 *  mismatch and refuse the file instead of silently misreading it
 *  (see AUDIT.md V1's cross-build hazard note).
 */

    {
        int32_t structsize = (int32_t) sizeof( struct VAX );
        fwrite( &structsize, 1, 4, fp );
    }

/*
 *  Write out the VAX structure itself now (one instance of given size).
 */

    fwrite( &vax, sizeof( struct VAX ), 1, fp );

/*
 *  For each page of physical memory that is non-zero, write the page out.
 */

    pages = vax.memsize / 512;
    
    for( page = 0; page < pages; page++ ) {
    
    /* See if this one is all zeroes (most are) */
        
        base = page * 512;
        found = 0;
        k = 512;
        
        for( j = 0; j < k; j++ ) {
            if( vax.memory[ base + j ] != 0 )
                found = 1;
        }
        
        if( found ) {
        
            /* If non zero, write out 512 -1- byte values */
            /* prefaced by address and page count longwords */
            
            fwrite( &base, 1, 4, fp );
            k = 1;
            fwrite( &k, 1, 4, fp );

            fwrite( &( vax.memory[ base ]), 512, 1, fp );
        }

    }

/*
 *  At the end of the page writes, write out a zero length ISD which means we are done.
 */

    base = 0L;
    k = 0L;

    fwrite( &base, 1, 4, fp );
    fwrite( &k,    1, 4, fp );

/*
 *  Our work is done; close the file, clean up the parse pointer, and skip out.
 */

    fclose( fp );
    while( !isend( *p )) {
        p++;
    }
    *P = p;
    return VAX_OK;

}



/*
 *  Save the entire ROM image in a binary file.
 */

LONGWORD save_rom( char ** P )
{
    LONGWORD pages, page, j, k, found, base;
    char * p, *fn;
    FILE * fp;
    LONGWORD n, quoted, tmp;
    extern char * rom;
    
    
/*
 *  Fetch the file name.  If it's missing, then we accept that and
 *  create a default name.
 */
 
    p = *P;
    flush_blanks( &p );
    if( isend( *p )) 
        fn = "default.rom";
    else
        fn = p; 

/*
 *  Clean up the name, we get all kinds of crap.
 */
 
    if( *fn == '\"' ) {
        quoted = 1;
        fn++;
    }
    else
        quoted = 0;


    for( n = 0; fn[ n ]; n++ ) {
    
        if( fn[ n ] == '\n' || fn[ n ] == '\"' )
            fn[ n ] = '\0';
    }

//  If the file name was not quoted, there may be trailing blanks to
//  be removed.

    if( !quoted ) 
        for( n = n - 1; p[ n ] == ' '; n-- )
            p[ n ] = '\0';


//  Since we're going to replace the file, just remove it if it already
//  exists.  We don't care about return codes here.

    remove( fn );

/*
 *  Try to open the file.  If we can't then bail out.
 */
 
    fp = fopen( fn, "w" );
    if( fp == 0L ) {
        printf( "Can't open file \"%s\"\n.", p );
        while( !isend( *p )) {
            p++;
        }
        *P = p;
        return VAX_OK;
    }

/*
 *  Write out a header record that marks the file as binary.  This
 *  lets the ROM command figure out how to read the file.
 */
 
    fputs( ";ROMIMG\n", fp );

/*
 *  Write out the base address and size
 */

    tmp = vax.rom_base;
#if !BIGENDIAN
    swap_bytes( &tmp, sizeof(tmp));
#endif
    fwrite( &(tmp), 1, 4, fp );

    tmp = vax.rom_end;
#if !BIGENDIAN
    swap_bytes( &tmp, sizeof(tmp));
#endif

    fwrite( &(tmp), 1, 4, fp );
    
    
/*
 *  For each page of rom memory that is non-zero, write the page out.
 */

    pages = (( vax.rom_end + 1 ) - vax.rom_base ) / 512;
    
    for( page = 0; page < pages; page++ ) {
    
    /* See if this one is all zeroes (most are) */
        
        base = page * 512;
        found = 0;
        k = 512;
        
        for( j = 0; j < k; j++ ) {
            if( rom[ base + j ] != 0 )
                found = 1;
        }
        
        if( found ) {
        
            /* If non zero, write out 512 -1- byte values */
            /* prefaced by address and page count longwords */
            
            tmp = base;
#if !BIGENDIAN
            swap_bytes( &tmp, sizeof(tmp));
#endif
            fwrite( &tmp, 1, 4, fp );

            tmp = 1;
#if !BIGENDIAN
            swap_bytes( &tmp, sizeof(tmp));
#endif
            fwrite( &tmp, 1, 4, fp );

            fwrite( &( rom[ base ]), 512, 1, fp );
        }

    }

/*
 *  At the end of the page writes, write out a zero length ISD which means we are done.
 */

    base = 0L;
    k = 0L;

    fwrite( &base, 1, 4, fp );
    fwrite( &k,    1, 4, fp );
    
/*
 *  Our work is done; close the file, clean up the parse pointer, and skip out.
 */
    
    fclose( fp );
    while( !isend( *p )) {
        p++;
    }
    *P = p;

    return VAX_OK;

}

/*
 *  Save the NVRAM image in a binary file.
 */

LONGWORD save_nvram( char ** P )
{
    extern char * nvram;
    LONGWORD n;
    char * p, *fn;
    FILE * fp;
    LONGWORD quoted;
    LONGWORD size,  tmp;
    
    
/*
 *  Fetch the file name.  If it's missing, then we accept that and
 *  create a default name.
 */
 
    p = *P;
    flush_blanks( &p );
    if( isend( *p )) 
        fn = "default.nvram";
    else
        fn = p; 

/*
 *  Clean up the name, we get all kinds of crap.
 */
 
    if( *fn == '\"' ) {
        quoted = 1;
        fn++;
    }
    else
        quoted = 0;


    for( n = 0; fn[ n ]; n++ ) {
    
        if( fn[ n ] == '\n' || fn[ n ] == '\"' )
            fn[ n ] = '\0';
    }

//  If the file name was not quoted, there may be trailing blanks to
//  be removed.

    if( !quoted ) 
        for( n = n - 1; p[ n ] == ' '; n-- )
            p[ n ] = '\0';


//  Since we're going to replace the file, just remove it if it already
//  exists.  We don't care about return codes here.

    remove( fn );

/*
 *  Try to open the file.  If we can't then bail out.
 */
 
    fp = fopen( fn, "w" );
    if( fp == 0L ) {
        printf( "Can't open file \"%s\"\n.", p );
        while( !isend( *p )) {
            p++;
        }
        *P = p;
        return VAX_OK;
    }



/*
 *  Write out the base address and size
 */

    tmp = vax.nvram_base;
#if !BIGENDIAN
    swap_bytes( &tmp, sizeof(tmp));
#endif

    fwrite( &(tmp), 1, 4, fp );

    size = ( vax.nvram_end + 1 ) - vax.nvram_base;
    tmp = size;

#if !BIGENDIAN
    swap_bytes( &tmp, sizeof(tmp));
#endif

    fwrite( &tmp, 1, 4, fp );

/*
 *  The NVRAM isn't very large.  Just write the whole darn thing out.
 */

    fwrite( nvram, size, 1, fp );
    
    
/*
 *  Our work is done; close the file, clean up the parse pointer, and skip out.
 */
    
    fclose( fp );
    while( !isend( *p )) {
        p++;
    }
    *P = p;

    return VAX_OK;

}

