package gonoleks

import "errors"

// Logging errors
const (
	ErrEmptyPortFormat         = "Empty port format, using default port %s"
	ErrInvalidPortFormat       = "Invalid port format, using default port %s"
	ErrRecoveredFromError      = "Recovered from error"
	ErrRequestAbortedWithError = "Request aborted with error"
)

// Marshaling errors
var (
	ErrJSONMarshalingFailed         = errors.New("JSON marshaling failed")
	ErrIndentedJSONMarshalingFailed = errors.New("IndentedJSON marshaling failed")
	ErrAsciiJSONMarshalingFailed    = errors.New("AsciiJSON marshaling failed")
	ErrPureJSONMarshalingFailed     = errors.New("PureJSON marshaling failed")
	ErrSecureJSONMarshalingFailed   = errors.New("SecureJSON marshaling failed")
	ErrJSONParsingFailed            = errors.New("JSON body parsing failed")
	ErrXMLMarshalingFailed          = errors.New("XML marshaling failed")
	ErrYAMLMarshalingFailed         = errors.New("YAML marshaling failed")
	ErrTOMLMarshalingFailed         = errors.New("TOML marshaling failed")
	ErrProtoBufMarshalingFailed     = errors.New("ProtoBuf marshaling failed")
)

// Rendering errors
var (
	ErrJSONMarshal               = errors.New("failed to marshal JSON")
	ErrIndentedJSONMarshal       = errors.New("failed to marshal JSON for IndentedJSON")
	ErrAsciiJSONMarshal          = errors.New("failed to marshal JSON for AsciiJSON")
	ErrPureJSONMarshal           = errors.New("failed to marshal JSON for PureJSON")
	ErrSecureJSONMarshal         = errors.New("failed to marshal JSON for SecureJSON")
	ErrXMLMarshal                = errors.New("failed to marshal XML")
	ErrYAMLMarshal               = errors.New("failed to marshal YAML")
	ErrTOMLMarshal               = errors.New("failed to marshal TOML")
	ErrProtoBufMarshal           = errors.New("failed to marshal ProtoBuf")
	ErrHTMLTemplateRender        = errors.New("failed to render HTML template")
	ErrProtoMessageInterface     = errors.New("data does not implement proto.Message interface")
	ErrCannotReadNilBody         = errors.New("cannot read nil body")
	ErrNamedCookieNotPresent     = errors.New("http: named cookie not present")
	ErrOfferedFormatsNotProvided = errors.New("negotiate: offered formats not provided")
	ErrMatchingFormatNotFound    = errors.New("negotiate: matching format not found")
	ErrTemplateEngineNotSet      = errors.New("template engine not set")
	ErrTemplateNotFound          = errors.New("template not found")
	ErrFileNotFound              = errors.New("file Not Found")
)
