//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS and other computers
//
//  Author:     Tom Cole
//
//  Module:     librtl_file.c
//
//  Purpose:    Runtime support for software "shims" that simulate runtime library
//              calls in VMS shared libraries.
//
//              This module implements file primitives for open, close, etc.
//
//  History:    11/05/99    New header format standardization, built from librtl.c
//
//              12/15/99    Add support for FILE* drive I/O functions
//

#include "vax.h"
#include "vaxinstr.h"
#include "asmproto.h"       // for get_symbol()
#include "memmap.h"         // to instantiate memory map list headers (uses GLOBAL)
#include "shim.h"

#include <stdio.h>
#include <errno.h>
#include <fcntl.h>

#if !defined(WIN)
#include <unistd.h>         // This doesn't appear to exist on Windows...
#else
#include <io.h>             // But this does...
#define open _open
#define write _write
#define close _close
#define read _read
#endif

#if defined( VMS )
#include <unixio.h>         //  VMS puts these separate from unistd.h
#endif


struct FILE_LIST {
    struct FILE_LIST * next;
    FILE *             f;
    LONGWORD           vax_f;
    LONGWORD           vax_buff;
};

struct FILE_LIST * file_list = 0L;


/*----------------------------------------------------------------------*
 *                                                                      *
 *    fid = exe$open( char * name, int mode );                          *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD exe_open( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD nameaddr;
    unsigned char name[ 256 ];
    long mode, n, rc;
    LONGWORD fid, mask;

/*
 *  Must be exactly two arguments
 */
 
    if( argc < 2 || argc >> 3 )
        return 0;
        
/*
 *  Get the name from memory
 */
 
    nameaddr = argv[ 0 ];
    for( n = 0; n < 255; n++ ) {
        rc = load_byte( nameaddr + n, &( name[ n ]));
        if( rc )
            return 0;
        if( name[ n ] == 0 )
            break;
    }
    
/*
 *  Get the mode.
 */
  
    mode = argv[ 1 ];
 

/*
 *  Open the file and return the id.
 */
 
    if( argc == 3 ) {
        mask = argv[ 2 ];
        fid = open( ( char * ) name, (int) mode, mask );
    }
    else
         fid = open( ( char * ) name, (int) mode );

    set_errno( errno );
    return fid;
}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    fid = exe$close( int fid );                                       *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD exe_close( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD fid;

/*
 *  Must be exactly one argument
 */
 
    if( argc != 1 )
        return 0;
        
    fid = argv[ 0 ];
    close( (int)fid );
    
    set_errno( errno );
    return fid;
}




/*----------------------------------------------------------------------*
 *                                                                      *
 *    fid = exe$read( char * name, int mode );                          *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD exe_read( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD addr, len;
    long n, count, rc;
    LONGWORD fid;
    char * io_buff, * bp;
    

/*
 *  Must be exactly three arguments
 */
 
    if( argc != 3 )
        return 0;
        
/*
 *  Get the arguments.
 */
 
    fid = argv[ 0 ];        /* file id */
    addr = argv[ 1 ];       /* memory buffer to read into */
    len = argv[ 2 ];        /* size of buffer */

/*
 *  Try to allocate a buffer.
 */
 
    io_buff = getmem( len );
    if( io_buff == 0L )
        return 0;

/*
 *  Do the read to the local buffer.
 */

    if( fid == 0 ) {
        bp = fgets( io_buff, (int) len, stdin );
        count = strlen( bp );
        if( count > 0 && bp[ count-1 ] == '\r' )
            count--;
    }
    else
        count = read( (int) fid, io_buff, len );
    
    for( n = 0; n < count; n++ ) {
    
        rc = store_memory( addr + n, ( unsigned char * ) &( io_buff[ n ]), 1 );
        if( rc )
            return 0;
            
    }
    
    set_errno( errno );
    return count;
}


/*----------------------------------------------------------------------*
 *                                                                      *
 *    fid = exe$write( char * name, int mode );                          *
 *                                                                      *
 *----------------------------------------------------------------------*/

LONGWORD exe_write( LONGWORD argc, LONGWORD * argv )
{

    LONGWORD addr, len;
    long n, count, rc;
    LONGWORD fid;
    char * io_buff;
    

/*
 *  Must be exactly three arguments
 */
 
    if( argc != 3 )
        return 0;
        
/*
 *  Get the arguments.
 */
 
    fid = argv[ 0 ];        /* file id */
    addr = argv[ 1 ];       /* memory buffer to write from */
    len = argv[ 2 ];        /* size of buffer */

/*
 *  Try to allocate a buffer.
 */
 
    io_buff = getmem( len );
    if( io_buff == 0L )
        return 0;

/*
 *  copy the data to the local buffer.
 */
 
    for( n = 0; n < len; n++ ) {
        rc = load_byte( addr + n, ( unsigned char * ) &( io_buff[ n ]));
        if( rc )
            return 0;
    }
    
/*
 *  Do the write from the local buffer.  Note that if it's file '1' then
 *  we cheat for formatting purposes to the console.
 */

    if( fid == 1 ) {
        io_buff[ len ] = 0;
        printf( "%s", io_buff );
        count = len;
    }
    else
        count = write( (int) fid, io_buff, len );
        
    set_errno( errno );
    return count;
}

