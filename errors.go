package gonoleks

import "errors"

// Logging errors
const (
	ErrEmptyPortFormat            = "Empty port format, using default port %s"
	ErrInvalidPortFormat          = "Invalid port format, using default port %s"
	ErrCacheCreationFailed        = "Cache creation failed"
	ErrRecoveredFromError         = "Recovered from error"
	ErrFormParsingFailed          = "Form parsing failed"
	ErrMultipartFormParsingFailed = "Multipart form parsing failed"
	ErrFormBindingFailed          = "Form binding failed"
	ErrRequestAbortedWithError    = "Request aborted with error"
	ErrUnsupportedContentType     = "Unsupported Content-Type"
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
	ErrJSONMarshal                 = errors.New("failed to marshal JSON")
	ErrIndentedJSONMarshal         = errors.New("failed to marshal JSON for IndentedJSON")
	ErrAsciiJSONMarshal            = errors.New("failed to marshal JSON for AsciiJSON")
	ErrPureJSONMarshal             = errors.New("failed to marshal JSON for PureJSON")
	ErrSecureJSONMarshal           = errors.New("failed to marshal JSON for SecureJSON")
	ErrXMLMarshal                  = errors.New("failed to marshal XML")
	ErrYAMLMarshal                 = errors.New("failed to marshal YAML")
	ErrTOMLMarshal                 = errors.New("failed to marshal TOML")
	ErrProtoBufMarshal             = errors.New("failed to marshal ProtoBuf")
	ErrHTMLTemplateRender          = errors.New("failed to render HTML template")
	ErrFormBind                    = errors.New("failed to bind form data")
	ErrContentType                 = errors.New("content type is not supported")
	ErrProtoMessageInterface       = errors.New("data does not implement proto.Message interface")
	ErrInvalidRequestEmptyBody     = errors.New("invalid request: empty body")
	ErrInvalidRequestEmptyForm     = errors.New("invalid request: empty form")
	ErrInvalidRequestEmptyQuery    = errors.New("invalid request: empty query")
	ErrInvalidUriParams            = errors.New("invalid uri params")
	ErrPlainBindPointer            = errors.New("plain binding requires a string pointer")
	ErrUnsupportedSliceElementType = errors.New("unsupported slice element type")
	ErrCannotReadNilBody           = errors.New("cannot read nil body")
	ErrNamedCookieNotPresent       = errors.New("http: named cookie not present")
	ErrOfferedFormatsNotProvided   = errors.New("negotiate: offered formats not provided")
	ErrMatchingFormatNotFound      = errors.New("negotiate: matching format not found")
	ErrTemplateEngineNotSet        = errors.New("template engine not set")
	ErrTemplateNotFound            = errors.New("template not found")
	ErrDataMustBeMapStringAny      = errors.New("data must be a map[string]any")
	ErrFileNotFound                = errors.New("file Not Found")
)
