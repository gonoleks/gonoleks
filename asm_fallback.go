//go:build !amd64 && !arm64

package gonoleks

import (
	"hash/fnv"
	"unsafe"
)

// Fallback implementations for non-optimized architectures

func asmStringHash(s string) uint64 {
	h := fnv.New64a()
	h.Write(unsafe.Slice(unsafe.StringData(s), len(s)))
	return h.Sum64()
}

func asmCombinedHash(a, b uint64) uint64 {
	h := fnv.New64a()
	h.Write((*[8]byte)(unsafe.Pointer(&a))[:])
	h.Write((*[8]byte)(unsafe.Pointer(&b))[:])
	return h.Sum64()
}

func ultraFastStringHash(s string) uint64 {
	return asmStringHash(s)
}

func ultraFastCombinedHash(a, b uint64) uint64 {
	return asmCombinedHash(a, b)
}
