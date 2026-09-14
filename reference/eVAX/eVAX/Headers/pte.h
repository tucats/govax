//
//  Copyright (C) 1997,1998,1999 Forest Edge Software, see License.txt for Rights
//
//  Program:    eVAX, a "Virtual VAX" for Mac OS Computers
//
//  Author:     Tom Cole
//
//  Module:     pte.h
//
//  Purpose:    This defines the PTE union that describes a page table
//              entry.  This is used by the VM routines and console and
//              assembler code that deals with PTEs.
//
//  History:    02/14/99    Extracted vm.c
//
//              05/29/99    Fixed problem where PTE was not stored in correct
//                          bit field order on some hosts.
//
//

    
#ifdef VMS
#pragma nomember_alignment
#endif

union PTE {

#if BIGENDIAN
    struct PTEBITS {
        unsigned int     v:1;        /* Valid bit */
        unsigned int     prot:4;     /* protection array */
        unsigned int     m:1;        /* modify bit */
        unsigned int     z:1;        /* mbz */
        unsigned int     own:2;      /* owner bits */
        unsigned int     s:2;        /* Digital software bits */
        unsigned int     pfn:21;     /* page frame number */
    } bit;
#else
    struct PTEBITS {
        unsigned int     pfn:21;     /* page frame number */
        unsigned int     s:2;        /* Digital software bits */
        unsigned int     own:2;      /* owner bits */
        unsigned int     z:1;        /* mbz */
        unsigned int     m:1;        /* modify bit */
        unsigned int     prot:4;     /* protection array */
        unsigned int     v:1;        /* Valid bit */
    } bit;

#endif

    ULONGWORD longword;
};

#ifdef VMS
#pragma member_alignment
#endif
        
//
//  Defines for page table access modes, passed to vm() and tracevm()
//

#define VM_READ     0   // Calling vm() for read
#define VM_WRITE    1   // Calling vm() for write
#define VM_NOSIGNAL   0x80      // Bit flag added when no signalling is
                                // to be done, i.e. probe()

//
//  Define the page table access modes, from SRM Chapter 5.
//

#define PTE_K_NA         0
#define PTE_K_KW         2
#define PTE_K_KR     3
#define PTE_K_UW     4
#define PTE_K_EW     5
#define PTE_K_ERKW   6
#define PTE_K_ER     7
#define PTE_K_SW     8
#define PTE_K_SREW   9
#define PTE_K_SRKW  10
#define PTE_K_SR    11
#define PTE_K_URSW  12
#define PTE_K_UREW  13
#define PTE_K_URKW  14
#define PTE_K_UR    15

