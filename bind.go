package gonoleks

import (
	"encoding/xml"
	"net/url"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/pelletier/go-toml/v2"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v3"
)

// Binding interface defines methods for binding HTTP request data to structs
type Binding interface {
	Name() string
	Bind(*fasthttp.RequestCtx, any) error
}

// BindingBody adds BindBody method to Binding interface
type BindingBody interface {
	Binding
	BindBody([]byte, any) error
}

// BindingUri adds BindUri method to Binding interface
type BindingUri interface {
	Name() string
	BindUri(map[string]string, any) error
}

// Request binding implementations
type (
	jsonBinding   struct{}
	formBinding   struct{}
	queryBinding  struct{}
	xmlBinding    struct{}
	yamlBinding   struct{}
	tomlBinding   struct{}
	headerBinding struct{}
	uriBinding    struct{}
	plainBinding  struct{}
)

// Binding instances
var (
	JSON   = jsonBinding{}
	XML    = xmlBinding{}
	Form   = formBinding{}
	Query  = queryBinding{}
	YAML   = yamlBinding{}
	TOML   = tomlBinding{}
	Header = headerBinding{}
	Uri    = uriBinding{}
	Plain  = plainBinding{}
)

// EnableDecoderUseNumber makes JSON decoder treat numbers as Number type
// instead of float64
var EnableDecoderUseNumber = false

// EnableDecoderDisallowUnknownFields makes JSON decoder reject unknown fields
var EnableDecoderDisallowUnknownFields = false

// Name returns the name of JSON binding
func (jsonBinding) Name() string {
	return "json"
}

// Bind binds JSON request data to the provided struct
func (jsonBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	body := ctx.Request.Body()
	if len(body) == 0 {
		return ErrInvalidRequestEmptyBody
	}
	return JSON.BindBody(body, obj)
}

// BindBody binds JSON body bytes to the provided struct
func (jsonBinding) BindBody(body []byte, obj any) error {
	return sonic.ConfigFastest.Unmarshal(body, obj)
}

// Name returns the name of XML binding
func (xmlBinding) Name() string {
	return "xml"
}

// Bind binds XML request data to the provided struct
func (xmlBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	body := ctx.Request.Body()
	if len(body) == 0 {
		return ErrInvalidRequestEmptyBody
	}
	return XML.BindBody(body, obj)
}

// BindBody binds XML body bytes to the provided struct
func (xmlBinding) BindBody(body []byte, obj any) error {
	return xml.Unmarshal(body, obj)
}

// Name returns the name of Form binding
func (formBinding) Name() string {
	return "form"
}

// Bind binds form data to the provided struct
func (formBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	// Get content type
	contentType := getString(ctx.Request.Header.ContentType())

	if strings.HasPrefix(contentType, MIMEMultipartForm) {
		// Handle multipart form
		form, err := ctx.MultipartForm()
		if err != nil {
			return err
		}

		// Convert form values to url.Values
		values := make(url.Values)
		for k, v := range form.Value {
			for _, vv := range v {
				values.Add(k, vv)
			}
		}

		return formDecoder.Decode(obj, values)
	}

	// Handle regular form
	args := ctx.PostArgs()
	if args == nil || args.Len() == 0 {
		return ErrInvalidRequestEmptyForm
	}

	// Convert fasthttp args to url.Values
	values := make(url.Values)
	args.VisitAll(func(key, value []byte) {
		values.Add(string(key), string(value))
	})

	return formDecoder.Decode(obj, values)
}

// Name returns the name of Query binding
func (queryBinding) Name() string {
	return "query"
}

// Bind binds query parameters to the provided struct
func (queryBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	args := ctx.QueryArgs()
	if args == nil || args.Len() == 0 {
		return ErrInvalidRequestEmptyQuery
	}

	// Convert fasthttp args to url.Values
	values := make(url.Values)
	args.VisitAll(func(key, value []byte) {
		values.Add(string(key), string(value))
	})

	return formDecoder.Decode(obj, values)
}

// Name returns the name of YAML binding
func (yamlBinding) Name() string {
	return "yaml"
}

// Bind binds YAML request data to the provided struct
func (yamlBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	body := ctx.Request.Body()
	if len(body) == 0 {
		return ErrInvalidRequestEmptyBody
	}
	return YAML.BindBody(body, obj)
}

// BindBody binds YAML body bytes to the provided struct
func (yamlBinding) BindBody(body []byte, obj any) error {
	return yaml.Unmarshal(body, obj)
}

// Name returns the name of TOML binding
func (tomlBinding) Name() string {
	return "toml"
}

// Bind binds TOML request data to the provided struct
func (tomlBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	body := ctx.Request.Body()
	if len(body) == 0 {
		return ErrInvalidRequestEmptyBody
	}
	return TOML.BindBody(body, obj)
}

// BindBody binds TOML body bytes to the provided struct
func (tomlBinding) BindBody(body []byte, obj any) error {
	return toml.Unmarshal(body, obj)
}

// Name returns the name of Header binding
func (headerBinding) Name() string {
	return "header"
}

// Bind binds header data to the provided struct
func (headerBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	// Convert fasthttp headers to url.Values
	values := make(url.Values)
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		// Convert header keys to lowercase for case-insensitive matching
		values.Add(strings.ToLower(string(key)), string(value))
	})

	return formDecoder.Decode(obj, values)
}

// Name returns the name of Uri binding
func (uriBinding) Name() string {
	return "uri"
}

// BindUri binds URI parameters to the provided struct
func (uriBinding) BindUri(params map[string]string, obj any) error {
	if len(params) == 0 {
		return ErrInvalidUriParams
	}

	// Convert map to url.Values
	values := make(url.Values)
	for k, v := range params {
		values.Add(k, v)
	}

	return formDecoder.Decode(obj, values)
}

// Name returns the name of Plain binding
func (plainBinding) Name() string {
	return "plain"
}

// Bind binds plain text request data to the provided struct
func (plainBinding) Bind(ctx *fasthttp.RequestCtx, obj any) error {
	body := ctx.Request.Body()
	if len(body) == 0 {
		return ErrInvalidRequestEmptyBody
	}
	return Plain.BindBody(body, obj)
}

// BindBody binds plain text body bytes to the provided struct
func (plainBinding) BindBody(body []byte, obj any) error {
	// For plain text, we just set the string value to the object if it's a string pointer
	if ptr, ok := obj.(*string); ok {
		*ptr = getString(body)
		return nil
	}
	return ErrPlainBindPointer
}

// DefaultBind returns the appropriate binding based on the HTTP method and Content-Type header
func DefaultBind(method string, contentType string) Binding {
	if method == MethodGet {
		return Query
	}

	switch {
	case strings.HasPrefix(contentType, MIMEApplicationJSON):
		return JSON
	case strings.HasPrefix(contentType, MIMEApplicationXML), strings.HasPrefix(contentType, MIMETextXML):
		return XML
	case strings.HasPrefix(contentType, MIMEApplicationYAML):
		return YAML
	case strings.HasPrefix(contentType, MIMEApplicationTOML):
		return TOML
	case strings.HasPrefix(contentType, MIMEApplicationForm):
		return Form
	case strings.HasPrefix(contentType, MIMEMultipartForm):
		return Form
	case strings.HasPrefix(contentType, MIMETextPlain):
		return Plain
	default:
		return JSON
	}
}
