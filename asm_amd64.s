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
    MOVQ    a+0(FP), R0            // first uint64
    MOVQ    b+8(FP), R1            // second uint64
    
    // Simple combination: a * prime + b
    MOVQ    $1099511628211, R2     // fnvPrime
    MULQ    R2                     // a *= prime (result in AX)
    ADDQ    R1, AX                 // a += b
    
    MOVQ    AX, ret+16(FP)         // return combined hash
    RET

// func asmCombinedHash(method, path string) uint32
TEXT ·asmCombinedHash(SB), NOSPLIT, $0-36
    MOVQ    method_base+0(FP), SI   // method data pointer
    MOVQ    method_len+8(FP), CX    // method length
    MOVQ    path_base+16(FP), DI    // path data pointer
    MOVQ    path_len+24(FP), DX     // path length
    
    // FNV-1a constants
    MOVQ    $14695981039346656037, AX  // fnvOffsetBasis
    MOVQ    $1099511628211, R8         // fnvPrime
    
    // Hash method
    TESTQ   CX, CX
    JZ      hash_path
    
method_loop:
    MOVBQZX (SI), BX               // load method byte
    MULQ    R8                     // hash *= fnvPrime
    XORQ    BX, AX                 // hash ^= byte
    INCQ    SI                     // advance pointer
    DECQ    CX                     // decrement counter
    JNZ     method_loop
    
hash_path:
    // Hash path
    TESTQ   DX, DX
    JZ      done_combined
    
path_loop:
    MOVBQZX (DI), BX               // load path byte
    MULQ    R8                     // hash *= fnvPrime
    XORQ    BX, AX                 // hash ^= byte
    INCQ    DI                     // advance pointer
    DECQ    DX                     // decrement counter
    JNZ     path_loop
    
done_combined:
    MOVL    AX, ret+32(FP)         // return uint32(hash)
    RET
