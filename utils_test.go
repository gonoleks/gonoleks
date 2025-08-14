package gonoleks

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAddress(t *testing.T) {
	// Test port resolution with colon
	assert.Equal(t, globalIpv4Addr+":5678", resolveAddress(":5678"))
	// Test empty port (default)
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(""))
	// Test invalid ports (fallback to default)
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(":abcd"))
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress(":70000"))
	assert.Equal(t, globalIpv4Addr+defaultPort, resolveAddress("5678"))

	// Test explicit addresses
	assert.Equal(t, "127.0.0.1:3000", resolveAddress("127.0.0.1:3000"))
	assert.Equal(t, globalIpv4Addr+":8080", resolveAddress("0.0.0.0:8080"))
	assert.Equal(t, "[::1]:3000", resolveAddress("[::1]:3000"))
	assert.Equal(t, globalIpv6Addr+":8080", resolveAddress("[::]:8080"))
}

func TestGetBytes(t *testing.T) {
	// Test ASCII and Unicode strings
	s := "ABC$€"
	b := getBytes(s)
	require.Equal(t, []byte(s), b)
	require.Equal(t, len(s), len(b))

	// Test non-Latin characters
	s = "سلام"
	b = getBytes(s)
	require.Equal(t, []byte(s), b)
	require.Equal(t, len(s), len(b))

	// Test empty string
	s = ""
	b = getBytes(s)
	require.Equal(t, []byte(nil), b)
	require.Equal(t, len(s), len(b))
}

func TestGetString(t *testing.T) {
	// Test byte array conversion
	b := []byte("ABC$€")
	s := getString(b)
	assert.Equal(t, "ABC$€", s)
	assert.Equal(t, len(b), len(s))

	// Test Unicode bytes
	b = []byte("سلام")
	s = getString(b)
	assert.Equal(t, "سلام", s)
	assert.Equal(t, len(b), len(s))

	// Test nil bytes
	b = nil
	s = getString(b)
	assert.Equal(t, "", s)
	assert.Equal(t, len(b), len(s))
}

func TestMarshalXML(t *testing.T) {
	// Test simple map
	h := H{
		"name":  "John",
		"email": "john@example.com",
	}
	data, err := xml.Marshal(h)
	require.NoError(t, err)
	actual := string(data)
	assert.Contains(t, actual, "<map>")
	assert.Contains(t, actual, "</map>")
	assert.Contains(t, actual, "<name>John</name>")
	assert.Contains(t, actual, "<email>john@example.com</email>")

	// Test mixed types
	h2 := H{
		"count":  42,
		"active": true,
		"name":   "Test",
	}
	data2, err := xml.Marshal(h2)
	require.NoError(t, err)
	actual2 := string(data2)
	assert.Contains(t, actual2, "<count>42</count>")
	assert.Contains(t, actual2, "<active>true</active>")
	assert.Contains(t, actual2, "<name>Test</name>")

	// Test empty map
	h3 := H{}
	data3, err := xml.Marshal(h3)
	require.NoError(t, err)
	assert.Equal(t, "<map></map>", string(data3))

	// Test nested structure
	h4 := H{
		"user": H{
			"id":   1,
			"name": "Arman",
		},
		"status": "active",
	}
	data4, err := xml.Marshal(h4)
	require.NoError(t, err)
	actual4 := string(data4)
	mapCount := strings.Count(actual4, "<map>")
	assert.Equal(t, 2, mapCount, "Expected 2 <map> tags for nested structure")
	assert.Contains(t, actual4, "<status>active</status>")
	assert.Contains(t, actual4, "<id>1</id>")
	assert.Contains(t, actual4, "<name>Arman</name>")
}
