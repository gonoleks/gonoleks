//go:build amd64 && !purego

#include "textflag.h"

// func asmStringHash(s string) uint64
TEXT ·asmStringHash(SB), NOSPLIT, $0-24
    MOVQ    s_base+0(FP), SI       // string data pointer
    MOVQ    s_len+8(FP), CX        // string length
    
    // FNV-1a constants
    MOVQ    $14695981039346656037, AX  // fnvOffsetBasis
    MOVQ    $1099511628211, DX         // fnvPrime
    
    TESTQ   CX, CX
    JZ      done_str
    
loop_str:
    MOVBQZX (SI), BX               // load byte
    MULQ    DX                     // hash *= fnvPrime
    XORQ    BX, AX                 // hash ^= byte
    INCQ    SI                     // advance pointer
    DECQ    CX                     // decrement counter
    JNZ     loop_str
    
done_str:
    MOVQ    AX, ret+16(FP)         // return uint64(hash) - Changed from MOVL to MOVQ
    RET

// func asmCombinedHash(a, b uint64) uint64
TEXT ·asmCombinedHash(SB), NOSPLIT, $0-24
    MOVQ    a+0(FP), AX            // first uint64
    MOVQ    b+8(FP), BX            // second uint64
    
    // Simple combination: a * prime + b
    MOVQ    $1099511628211, CX     // fnvPrime
    MULQ    CX                     // a *= prime (result in AX)
    ADDQ    BX, AX                 // a += b
    
    MOVQ    AX, ret+16(FP)         // return combined hash
    RET
