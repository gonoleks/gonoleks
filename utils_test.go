package gonoleks

import (
	"encoding/xml"
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

func TestMarshalXML(t *testing.T) {
	// Test with simple string values
	h := H{
		"name":  "John",
		"email": "john@example.com",
	}

	data, err := xml.Marshal(h)
	require.NoError(t, err)

	expected := `<map><name>John</name><email>john@example.com</email></map>`
	// Since map iteration order is not guaranteed, we need to check both possible orders
	expected2 := `<map><email>john@example.com</email><name>John</name></map>`
	actual := string(data)
	assert.True(t, actual == expected || actual == expected2, "Expected one of %s or %s, got %s", expected, expected2, actual)

	// Test with mixed types
	h2 := H{
		"count":  42,
		"active": true,
		"name":   "Test",
	}

	data2, err := xml.Marshal(h2)
	require.NoError(t, err)
	actual2 := string(data2)

	assert.Contains(t, actual2, "<map>")
	assert.Contains(t, actual2, "</map>")
	assert.Contains(t, actual2, "<count>42</count>")
	assert.Contains(t, actual2, "<active>true</active>")
	assert.Contains(t, actual2, "<name>Test</name>")

	// Test with empty map
	h3 := H{}
	data3, err := xml.Marshal(h3)
	require.NoError(t, err)
	assert.Equal(t, "<map></map>", string(data3))

	// Test with nested structure
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

	// Note: The key "user" is lost and nested H becomes another <map> element
	assert.Contains(t, actual4, "<map>")
	assert.Contains(t, actual4, "</map>")
	assert.Contains(t, actual4, "<status>active</status>")
	// The nested structure appears as <map><map>...</map>... due to how MarshalXML works
	assert.Contains(t, actual4, "<map><map>")
	assert.Contains(t, actual4, "<id>1</id>")
	assert.Contains(t, actual4, "<name>Arman</name>")
}
