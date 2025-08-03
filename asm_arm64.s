//go:build arm64 && !purego

#include "textflag.h"

// func asmStringHash(s string) uint64
TEXT ·asmStringHash(SB), NOSPLIT, $0-24
    MOVD    s_base+0(FP), R0       // string data pointer
    MOVD    s_len+8(FP), R1        // string length
    
    // FNV-1a constants
    MOVD    $14695981039346656037, R2  // fnvOffsetBasis
    MOVD    $1099511628211, R3         // fnvPrime
    
    CBZ     R1, done_str
    
loop_str:
    MOVBU   (R0), R4               // load byte
    MUL     R3, R2                 // hash *= fnvPrime
    EOR     R4, R2                 // hash ^= byte
    ADD     $1, R0                 // advance pointer
    SUB     $1, R1                 // decrement counter
    CBNZ    R1, loop_str
    
done_str:
    MOVD    R2, ret+16(FP)         // return uint64(hash)
    RET

// func asmCombinedHash(a, b uint64) uint64
TEXT ·asmCombinedHash(SB), NOSPLIT, $0-24
    MOVD    a+0(FP), R0            // first hash
    MOVD    b+8(FP), R1            // second hash
    
    // Simple combination: a * prime + b
    MOVD    $1099511628211, R2     // fnvPrime
    MUL     R2, R0                 // a *= prime
    ADD     R1, R0                 // a += b
    
    MOVD    R0, ret+16(FP)         // return combined hash
    RET
