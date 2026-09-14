#ifndef ____COMMON_HEADER_LOADED
#define ____COMMON_HEADER_LOADED
/****************************************************************************
**
**  This header file is included by all header files.  The purpose of this
**  header file is to include those things which are either done by all
**  header files or introduce no name space violations by doing so.
**
*****************************************************************************
**  Header not defined by any specification or standard
*****************************************************************************
**
**  Copyright Digital Equipment Corporation 1997.  All rights reserved.
**
**  Restricted Rights: Use, duplication, or disclosure by the U.S.
**  Government is subject to restrictions as set forth in subparagraph
**  (c) (1) (ii) of DFARS 252.227-7013, or in FAR 52.227-19, or in FAR
**  52.227-14 Alt. III, as applicable.
**
**  This software is proprietary to and embodies the confidential
**  technology of Digital Equipment Corporation. Possession, use, or
**  copying of this software and media is authorized only pursuant to a
**  valid written license from Digital or an authorized sublicensor.
**
******************************************************************************
*/

#pragma __nostandard


/*
** Set up feature test macros
*/
# if defined _XOPEN_SOURCE_EXTENDED && !defined _XOPEN_SOURCE
#   define _XOPEN_SOURCE
# endif

# if defined _XOPEN_SOURCE 
#   if !defined _POSIX_C_SOURCE
#      define _POSIX_C_SOURCE 2
#   endif
#   if _POSIX_C_SOURCE < 2
#      undef  _POSIX_C_SOURCE
#      define _POSIX_C_SOURCE 2
#   endif
# endif

# if defined _POSIX_SOURCE 
#   if !defined _POSIX_C_SOURCE
#      define _POSIX_C_SOURCE 1
#   endif
#   if _POSIX_C_SOURCE < 1
#      undef  _POSIX_C_SOURCE     /*  Use _POSIX_C_SOURCE with a value of 1 */
#      define _POSIX_C_SOURCE 1   /*  instead of _POSIX_SOURCE (obsolete)   */
#   endif
# endif

# if defined _POSIX_C_SOURCE && !defined _ANSI_C_SOURCE
#   define _ANSI_C_SOURCE
# endif 

# if defined __HIDE_FORBIDDEN_NAMES && !defined _ANSI_C_SOURCE
#   define _ANSI_C_SOURCE
# endif


/*
**  If the compiler was not already given a definition for __CRTL_VER, then
**  define it now.
*/
#ifndef __CRTL_VER
#   define __CRTL_VER __VMS_VER
#endif


/*
**  The DEC C RTL relies on the use of extern_prefix support in the compilers.
**  The scenario of a customer getting these new headers via DEC C++, while at
**  the same time using a compiler before DEC C V5.2 is still supported.
*/
#if defined(__DECC_VER)
#   if (__DECC_VER > 50230003)
#      define __CAN_USE_EXTERN_PREFIX 1
#   endif
#else
#   if defined(__DECCXX)
#      define __CAN_USE_EXTERN_PREFIX 1
#   endif
#endif


/*
**  Define typedefs which are used throughout the header files.  Pointer size
**  is only allowed on OpenVMS Alpha after V7.0.
*/
#ifdef __INITIAL_POINTER_SIZE
#   if __INITIAL_POINTER_SIZE
#      if (__CRTL_VER < 70000000) || !defined __ALPHA
#         error " Pointer size usage not permitted before OpenVMS Alpha V7.0"
#      endif
#      pragma __pointer_size __save
#      pragma __pointer_size 32
#   endif
#endif

    typedef unsigned int __id_t;
    typedef unsigned int __size_t;
    typedef unsigned int __u_int;
    typedef unsigned int __uid_t;
    typedef unsigned int __useconds_t;
    typedef unsigned long int __time_t;
    typedef unsigned short __gid_t;
    typedef unsigned short __ino_t;
    typedef unsigned short __mode_t;
    typedef int __nlink_t;
    typedef int __off_t;
    typedef int __pid_t;
    typedef int __ssize_t;
    typedef char *__caddr_t;
    typedef char *__dev_t;
    typedef char *__va_list;
    typedef unsigned int   __in_addr_t;
    typedef unsigned short __in_port_t;
    typedef unsigned char  __sa_family_t;
    typedef unsigned int __wchar_t;
    typedef __wchar_t * __wchar_ptr32;
    typedef const __wchar_t * __const_wchar_ptr32;
    typedef int __wint_t;
    typedef int __wctrans_t;
    typedef int __wctype_t;

    struct _iobuf;
    typedef struct _iobuf *__FILE;
    typedef __FILE * __FILE_ptr32;

    typedef char * __char_ptr32;
    typedef const char * __const_char_ptr32;

    typedef void * __void_ptr32;
    typedef const void * __const_void_ptr32;

    typedef unsigned short * __unsigned_short_ptr32;
    typedef const unsigned short * __const_unsigned_short_ptr32;

    typedef char ** __char_ptr_ptr32;
    typedef char * const *  __char_ptr_const_ptr32;

#ifdef __ALPHA
    typedef __int64 * __int64_ptr32;
    typedef const __int64 * __const_int64_ptr32;
#endif

#ifdef __INITIAL_POINTER_SIZE
#   if __INITIAL_POINTER_SIZE
#      pragma __pointer_size 64
#   endif
#endif

    typedef char * __char_ptr64;
    typedef const char * __const_char_ptr64;

    typedef void * __void_ptr64;
    typedef const void * __const_void_ptr64;

    typedef __wchar_t * __wchar_ptr64;
    typedef const __wchar_t * __const_wchar_ptr64;

#ifdef __INITIAL_POINTER_SIZE
#   if __INITIAL_POINTER_SIZE
#      pragma __pointer_size __restore
#   endif
#endif

#pragma __standard
#endif /* ____COMMON_HEADER_LOADED */
