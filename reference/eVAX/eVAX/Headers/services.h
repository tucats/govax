//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     services.h
//
//  Purpose:    This module contains prototypes for native system 
//              services that support running VMS images.
//
//  History:    11/08/99     Created.
//

#define SSDEF( name ) LONGWORD name( LONGWORD argc, LONGWORD * argv )

SSDEF( sys_assign );
SSDEF( sys_clref );
SSDEF( sys_setef );
SSDEF( sys_readef );
SSDEF( sys_expreg );
SSDEF( sys_trnlnm );
SSDEF( sys_dclexh );
SSDEF( sys_getdviw );
SSDEF( sys_getjpiw );
SSDEF( sys_setast );
SSDEF( sys_cli );

SSDEF( rms_create );
SSDEF( rms_connect );
SSDEF( rms_put );
