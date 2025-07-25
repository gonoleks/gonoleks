package gonoleks

import (
	"encoding/xml"
	"strconv"
	"strings"
	"unsafe"

	"github.com/charmbracelet/log"
)

const (
	globalIpv4Addr = "0.0.0.0" // Wildcard IPv4 address (binds to all interfaces)
	globalIpv6Addr = "[::]"    // Wildcard IPv6 address (binds to all interfaces)
	defaultPort    = ":8080"   // Fallback port
)

// H is a shortcut for map[string]any
type H map[string]any

// MarshalXML allows type H to be used with xml.Marshal
func (h H) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = "map"
	start.Name.Space = ""

	if err := e.EncodeToken(start); err != nil {
		return err
	}

	elem := &xml.StartElement{
		Name: xml.Name{Space: ""},
		Attr: nil,
	}

	for key, value := range h {
		elem.Name.Local = key
		if err := e.EncodeElement(value, *elem); err != nil {
			return err
		}
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// resolveAddress validates and resolves the provided port string into a complete address
// It handles empty ports, ports with colon prefix, and invalid port formats
// Returns a properly formatted address string with IPv4 as default
func resolveAddress(portStr string) string {
	if portStr == "" {
		log.Warnf("Empty port format, using default port %s", defaultPort)
		return globalIpv4Addr + defaultPort
	}

	if strings.HasPrefix(portStr, ":") {
		portNum, err := strconv.Atoi(portStr[1:])
		if err != nil || portNum < 1 || portNum > 65535 {
			log.With("port", portStr).Warnf("Invalid port format, using default port %s", defaultPort)
			return globalIpv4Addr + defaultPort
		}
		// Default to IPv4 when only port is specified
		return globalIpv4Addr + portStr
	}

	// If it doesn't start with colon but contains colon, it's a complete address
	if strings.Contains(portStr, ":") {
		return portStr
	}

	// If there's no colon at all, it's invalid (port number without colon)
	// Fall back to default port
	log.With("port", portStr).Warnf("Invalid port format, using default port %s", defaultPort)
	return globalIpv4Addr + defaultPort
}

// detectNetworkProtocol determines the network protocol based on the address format
func detectNetworkProtocol(address string) string {
	// IPv6 addresses are enclosed in brackets or contain multiple colons
	if strings.Contains(address, "[") || strings.Count(address, ":") > 1 {
		return NetworkTCP6
	}
	// IPv4 addresses contain dots
	if strings.Contains(address, ".") {
		return NetworkTCP4
	}
	// Default to IPv4 for ambiguous cases
	return NetworkTCP4
}

// getBytes converts string to []byte without copying
// Don't modify the returned slice
// #nosec G103 - Safe unsafe usage
func getBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// getString converts []byte to string without copying
// Don't modify the input slice after calling this
// #nosec G103 - Safe unsafe usage
func getString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}
