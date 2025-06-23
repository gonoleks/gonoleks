package gonoleks

import (
	"encoding/xml"
	"strconv"
	"strings"
	"unsafe"

	"github.com/charmbracelet/log"
)

const (
	globalIpv4Addr = "0.0.0.0" // Default binding address for all network interfaces
	defaultPort    = ":8080"   // Default port for the server
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
// Returns a properly formatted address string with the global IPv4 address
func resolveAddress(portStr string) string {
	if portStr == "" {
		log.Warnf(ErrEmptyPortFormat, defaultPort)
		return globalIpv4Addr + defaultPort
	}

	if strings.HasPrefix(portStr, ":") {
		portNum, err := strconv.Atoi(portStr[1:])
		if err != nil || portNum < 1 || portNum > 65535 {
			log.With("port", portStr).Warnf(ErrInvalidPortFormat, defaultPort)
			return globalIpv4Addr + defaultPort
		}
		return globalIpv4Addr + portStr
	}

	// Invalid format
	log.With("port", portStr).Warnf(ErrInvalidPortFormat, defaultPort)
	return globalIpv4Addr + defaultPort
}

// getBytes converts string to []byte without copying
// Don't modify the returned slice
// #nosec G103 - Safe unsafe usage
func getBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// getString converts []byte to string without copying
// Don't modify the input slice after calling this
// #nosec G103 - Safe unsafe usage
func getString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
