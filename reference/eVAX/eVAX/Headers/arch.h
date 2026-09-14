//
//  Copyright © 1997,1998 Forest Edge Software, see License.txt for Rights
//
//  Program:    VAX Emulator, a "Virtual VAX"
//
//  Author:     Tom Cole
//
//  Module:     arch.h
//
//  Purpose:    This header file definitions for architectural features
//      of the host environment.
//
//  History:    05/18/99    Initial creation, pulled from vax.pch
//
//              09/27/99    Added initial support for TRU64 based on input and
//                          testing from Hartmut Becker <becker@rto.dec.com>
//                          and Sergey Tikhonov <tsv@excom.spb.su>
//
//              10/03/99    Okay, it seems that there's no benefit in being a
//                          truly 64-bit system, emulating a VAX.  So I'm converting
//                          all long's to an explicit definition of a 32-bit integer.
//
//                          It looks like the safe route is to use a local typedef of
//                          LONGWORD that matches what we are using.  Also, use QUADWORD to
//                          reflect a 64-bit integer.
//
//                          I WILL PROBABLY GET SOME OF THIS WRONG, SO PORTS MAY BE ROUGH
//                          FOR THE NEXT CYCLE!
//
//              04/15/02    Added HAS_HARDWARE_CLOCK for Unix, VMS
//
//              09/14/26    Fixed LONGWORD/ULONGWORD/QUADWORD to be true fixed-width
//                          types (stdint.h int32_t/uint32_t/int64_t) on every
//                          platform, retiring the HAS64BITLONGS escape hatch that
//                          was never defined for this build.  See AUDIT.md 5.1.

/*
 *  This header file is typically included after the system headers
 *  from vax.pch.  This should set up the macro definitions, etc.
 *  for the box we are running on.  In the compilation mechanism
 *  for the port, specify the platform as a define on the command.
 *
 *  The "supported" platforms the system has been built for are:
 *
 *   MAC         MacOS
 *   MACHTEN     MachTen (BSD under MacOS)
 *   LINUXPPC    PowerPC-based Linux systems
 *   HPUX        HP-UX 10.20
 *   WIN         Windows 98 or Windows NT
 *   LINUX86     Intel-based Linux systems
 *   FREEBSD     Intel-based FreeBSD systems
 *   TRU64       Compaq's Unix for Alpha (64-bits!)
 *   VMS         Alpha (!) VMS
 */


#define UNKNOWN_ARCH 1

#if defined( MAC ) || defined( macintosh )
#ifndef MAC    /* CodeWarrior gives us "macintosh" */
#define MAC 1
#endif
#undef UNKNOWN_ARCH
#define BIGENDIAN   1
#endif


#if defined( IPHONE ) 
#undef UNKNOWN_ARCH
#define BIGENDIAN   0
#define HAS_HARDWARE_CLOCK 0
#endif


#ifdef VMS
#define BIGENDIAN   0
#define HAS_HARDWARE_CLOCK 1
#undef UNKNOWN_ARCH
#endif

#ifdef  TRU64
#define BIGENDIAN   0
#define HAS_HARDWARE_CLOCK 1
#undef UNKNOWN_ARCH
#endif



#ifdef HPUX     
#define BIGENDIAN   1
#define HAS_HARDWARE_CLOCK 1
#undef UNKNOWN_ARCH
#endif



#ifdef WIN
#define BIGENDIAN   0
#define HAS_HARDWARE_CLOCK 0
#define INVERTEDLONGCHAR    0  /* VSS seems to want 0 here */
#undef UNKNOWN_ARCH
#endif



#ifdef FREEBSD
#define BIGENDIAN   0
#define HAS_HARDWARE_CLOCK 1
#undef UNKNOWN_ARCH
#endif


#ifdef LINUX86
#define BIGENDIAN 0
#define HAS_HARDWARE_CLOCK 1
#undef UNKNOWN_ARCH
#endif


#if defined( MACHTEN ) || defined( LINUXPPC )
#define BIGENDIAN 1
#undef UNKNOWN_ARCH
#define HAS_HARDWARE_CLOCK 1
#endif

/*
 *  LONGWORD/ULONGWORD/QUADWORD are the fixed-width stand-ins for a VAX
 *  32-bit longword / 64-bit quadword used throughout this emulator (see
 *  the History note above -- this is exactly the "explicit definition of
 *  a 32-bit integer" the original author intended).  They must NOT track
 *  the host's native `long` width, which is 64 bits on essentially every
 *  modern platform this builds on (including LINUX86, the arch this
 *  project's Xcode target actually builds with).  Previously this was
 *  gated behind a hand-maintained HAS64BITLONGS #ifdef that was only ever
 *  defined for TRU64, so every other 64-bit host silently got 8-byte
 *  "longwords" -- see AUDIT.md sec. 0 and task 5.1 for the fallout.
 *  <stdint.h> exact-width types make this unconditional instead.
 */
#include <stdint.h>

typedef int32_t  LONGWORD;
typedef uint32_t ULONGWORD;
typedef int64_t  QUADWORD;

#ifdef UNKNOWN_ARCH

#error Architecture not defined!
 
You must define an architecture in your build process to specify what build
environment and/or chip architecture you are using to build eVAX.  See the
file arch.h for more information.

#endif


/*
 *  Build additional definitions from the above definitions
 */

#if BIGENDIAN && !defined( INVERTEDLONGCHAR )

#define CHAR4( ch1, ch2, ch3, ch4 )   \
  ((( unsigned long ) ch1 << 24 ) |   \
   (( unsigned long ) ch2 << 16 ) |   \
   (( unsigned long ) ch3 << 8  ) |   \
   (( unsigned long ) ch4       ))

#else

#define CHAR4( ch1, ch2, ch3, ch4 )   \
  ((( unsigned long ) ch4 << 24 ) |   \
   (( unsigned long ) ch3 << 16 ) |   \
   (( unsigned long ) ch2 << 8  ) |   \
   (( unsigned long ) ch1       ))

#endif


