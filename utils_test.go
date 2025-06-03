package gonoleks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAddress(t *testing.T) {
	// Valid port with colon
	assert.Equal(t, globalIpv4Addr+":5678", resolveAddress(":5678"))
	// Empty port string
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(""))
	// Invalid port (non-numeric)
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(":abcd"))
	// Invalid port (out of range)
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(":70000"))
	// Port without colon
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress("5678"))
}

func TestGetBytes(t *testing.T) {
	s := "ABC$€"
	b := getBytes(s)
	require.Equal(t, []byte(s), b)
	require.Equal(t, len(s), len(b))

	s = "سلام"
	b = getBytes(s)
	require.Equal(t, []byte(s), b)
	require.Equal(t, len(s), len(b))

	s = ""
	b = getBytes(s)
	require.Equal(t, []byte(nil), b)
	require.Equal(t, len(s), len(b))
}

func TestGetString(t *testing.T) {
	b := []byte("ABC$€")
	s := getString(b)
	assert.Equal(t, "ABC$€", s)
	assert.Equal(t, len(b), len(s))

	b = []byte("سلام")
	s = getString(b)
	assert.Equal(t, "سلام", s)
	assert.Equal(t, len(b), len(s))

	b = nil
	s = getString(b)
	assert.Equal(t, "", s)
	assert.Equal(t, len(b), len(s))
}
