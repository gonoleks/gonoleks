//go:build amd64 || arm64

package gonoleks

// Assembly function declarations
func asmStringHash(s string) uint64
func asmCombinedHash(a, b uint64) uint64

// ultraFastStringHash provides assembly-optimized string hash
func ultraFastStringHash(s string) uint64 {
	return asmStringHash(s)
}

// ultraFastCombinedHash provides assembly-optimized combined hash
func ultraFastCombinedHash(a, b uint64) uint64 {
	return asmCombinedHash(a, b)
}
