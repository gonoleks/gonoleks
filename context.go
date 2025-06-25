package gonoleks

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/charmbracelet/log"
	"github.com/pelletier/go-toml/v2"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// handlerFunc is a request handler function
type handlerFunc func(c *Context)

// handlersChain is a slice of handlers
type handlersChain []handlerFunc

// Context represents the current HTTP request and response context
type Context struct {
	requestCtx  *fasthttp.RequestCtx
	paramValues map[string]string
	handlers    handlersChain
	index       int
	fullPath    string
}

// Negotiate contains the information for content negotiation
type Negotiate struct {
	Offered  []string
	HTMLName string
	HTMLData any
	JSONData any
	XMLData  any
	YAMLData any
	TOMLData any
	Data     any
}

// Context returns the underlying fasthttp RequestCtx object
func (c *Context) Context() *fasthttp.RequestCtx {
	return c.requestCtx
}

// Copy returns a copy of the current context that can be safely used outside the request's scope
// This has to be used when the context has to be passed to a goroutine
func (c *Context) Copy() *Context {
	contextCopy := &Context{
		requestCtx: c.requestCtx,
		index:      c.index,
		fullPath:   c.fullPath,
	}

	// Copy parameter map
	if c.paramValues != nil {
		contextCopy.paramValues = make(map[string]string, len(c.paramValues))
		for k, v := range c.paramValues {
			contextCopy.paramValues[k] = v
		}
	}

	// Copy handler chain
	if c.handlers != nil {
		contextCopy.handlers = make(handlersChain, len(c.handlers))
		copy(contextCopy.handlers, c.handlers)
	}

	return contextCopy
}

// FullPath returns a matched route full path
// For not found routes returns an empty string
//
//	app.GET("/user/:id", func(c *gonoleks.Context) {
//	    c.FullPath() == "/user/:id" // true
//	})
func (c *Context) FullPath() string {
	return c.fullPath
}

// Next calls the next handler in the chain
// Use this in middleware to continue processing
func (c *Context) Next() {
	c.index++
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

// IsAborted returns true if the current context was aborted
func (c *Context) IsAborted() bool {
	return c.index >= len(c.handlers)
}

// Abort prevents pending handlers from being called. Note that this will not stop the current handler
// Let's say you have an authorization middleware that validates that the current request is authorized
// If the authorization fails (ex: the password does not match), call Abort to ensure the remaining handlers
// for this request are not called.
func (c *Context) Abort() {
	c.index = len(c.handlers)
}

// AbortWithStatus calls `Abort()` and writes the headers with the specified status code
// For example, a failed attempt to authenticate a request could use: context.AbortWithStatus(401)
func (c *Context) AbortWithStatus(code int) {
	c.Abort()
	c.requestCtx.Response.SetStatusCode(code)
}

// AbortWithStatusJSON calls `Abort()` and then `JSON` internally
// This method stops the chain, writes the status code and return a JSON body
// It automatically sets the Content-Type header to "application/json"
func (c *Context) AbortWithStatusJSON(status int, jsonObj any) error {
	c.Abort()
	return c.JSON(status, jsonObj)
}

// AbortWithError calls `AbortWithStatus()` and logs the given error
func (c *Context) AbortWithError(code int, err error) error {
	c.AbortWithStatus(code)
	log.Error(ErrRequestAbortedWithError, "error", err, "code", code)
	return err
}

// Set is used to store a new key/value pair exclusively for this context
func (c *Context) Set(key string, value any) {
	if key == "" {
		return
	}

	if c.requestCtx.UserValue("keys") == nil {
		c.requestCtx.SetUserValue("keys", make(map[string]any))
	}

	keys, ok := c.requestCtx.UserValue("keys").(map[string]any)
	if !ok {
		// Reset if type assertion fails
		keys = make(map[string]any)
		c.requestCtx.SetUserValue("keys", keys)
	}

	keys[key] = value
}

// Get returns the value for the given key, ie: (value, true)
// If the value does not exist it returns (nil, false)
func (c *Context) Get(key string) (value any, exists bool) {
	if key == "" {
		return nil, false
	}

	if c.requestCtx.UserValue("keys") == nil {
		return nil, false
	}

	keys, ok := c.requestCtx.UserValue("keys").(map[string]any)
	if !ok {
		return nil, false
	}

	value, exists = keys[key]
	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics
func (c *Context) MustGet(key string) any {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic(fmt.Sprintf("Key %q does not exist", key))
}

// Param retrieves the value of a URL path parameter specified by the given key
func (c *Context) Param(key string) string {
	return c.paramValues[key]
}

// AddParam adds param to context and
// replaces path param key with given value for e2e testing purposes
// Example Route: "/user/:id"
// AddParam("id", 1)
// Result: "/user/1"
func (c *Context) AddParam(key, value string) {
	if c.paramValues == nil {
		c.paramValues = make(map[string]string)
	}
	c.paramValues[key] = value
}

// Query retrieves the value of a query string parameter from the request URL
func (c *Context) Query(key string) string {
	return getString(c.requestCtx.QueryArgs().Peek(key))
}

// DefaultQuery retrieves the value of a query string parameter from the request URL
// If the parameter does not exist or is empty, it returns the default value
func (c *Context) DefaultQuery(key, defaultValue string) string {
	v := c.requestCtx.QueryArgs().Peek(key)
	if len(v) == 0 {
		return defaultValue
	}
	return getString(v)
}

// GetQuery is like Query(), it returns the keyed url query value
// if it exists `(value, true)` (even when the value is an empty string),
// otherwise it returns `("", false)`
//
//	GET /?firstname=John&lastname=
//	("John", true) == c.GetQuery("firstname")
//	("", false) == c.GetQuery("id")
//	("", true) == c.GetQuery("lastname")
func (c *Context) GetQuery(key string) (string, bool) {
	v := c.requestCtx.QueryArgs().PeekBytes(getBytes(key))
	if v == nil {
		return "", false
	}
	return getString(v), true
}

// QueryArray returns a slice of strings for a given query key
// The length of the slice depends on the number of params with the given key
func (c *Context) QueryArray(key string) []string {
	values := []string{}
	c.requestCtx.QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) == key {
			values = append(values, getString(v))
		}
	})
	return values
}

// GetQueryArray returns a slice of strings for a given query key, plus
// a boolean value whether at least one value exists for the given key
func (c *Context) GetQueryArray(key string) ([]string, bool) {
	values := []string{}
	c.requestCtx.QueryArgs().VisitAll(func(k, v []byte) {
		if string(k) == key {
			values = append(values, getString(v))
		}
	})
	return values, len(values) > 0
}

// QueryMap returns a map for a given query key
func (c *Context) QueryMap(key string) map[string]string {
	result := make(map[string]string)
	c.requestCtx.QueryArgs().VisitAll(func(k, v []byte) {
		keyStr := string(k)
		// Check if the key has the format we're looking for (e.g., user[name], user[email])
		if strings.HasPrefix(keyStr, key+"[") && strings.HasSuffix(keyStr, "]") {
			// Extract the map key from between the brackets
			mapKey := keyStr[len(key)+1 : len(keyStr)-1]
			result[mapKey] = getString(v)
		}
	})
	return result
}

// GetQueryMap returns a map for a given query key, plus a boolean value
// whether at least one value exists for the given key
func (c *Context) GetQueryMap(key string) (map[string]string, bool) {
	result := c.QueryMap(key)
	return result, len(result) > 0
}

// PostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns an empty string ("")
func (c *Context) PostForm(key string) string {
	// First check if it's a urlencoded form
	if v := c.requestCtx.PostArgs().PeekBytes(getBytes(key)); len(v) > 0 {
		return getString(v)
	}

	// Then check if it's a multipart form
	form, err := c.requestCtx.MultipartForm()
	if err != nil {
		return ""
	}

	// Check if the key exists in the multipart form
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}

	return ""
}

// GetPostForm is like PostForm(key). It returns the specified key from a POST urlencoded
// form or multipart form when it exists `(value, true)` (even when the value is an empty string),
// otherwise it returns ("", false)
// For example, during a PATCH request to update the user's email:
//
//	email=mail@example.com --> ("mail@example.com", true) := GetPostForm("email") // Set email to "mail@example.com"
//	email=                 --> ("", true)                 := GetPostForm("email") // Set email to ""
//	                       --> ("", false)                := GetPostForm("email") // Do nothing with email
func (c *Context) GetPostForm(key string) (string, bool) {
	// First check if it's a urlencoded form
	if v := c.requestCtx.PostArgs().PeekBytes(getBytes(key)); v != nil {
		return getString(v), true
	}

	// Then check if it's a multipart form
	form, err := c.requestCtx.MultipartForm()
	if err != nil {
		return "", false
	}

	// Check if the key exists in the multipart form
	if values, exists := form.Value[key]; exists && len(values) > 0 {
		return values[0], true
	}

	return "", false
}

// DefaultPostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns the specified defaultValue string
// See: PostForm() and GetPostForm() for further information
func (c *Context) DefaultPostForm(key, defaultValue string) string {
	// First check if it's a urlencoded form
	if v := c.requestCtx.PostArgs().PeekBytes(getBytes(key)); len(v) > 0 {
		return getString(v)
	}

	// Then check if it's a multipart form
	form, err := c.requestCtx.MultipartForm()
	if err != nil {
		return defaultValue
	}

	// Check if the key exists in the multipart form
	if values := form.Value[key]; len(values) > 0 {
		return values[0]
	}

	return defaultValue
}

// PostFormArray returns a slice of strings for a given form key
// The length of the slice depends on the number of params with the given key
func (c *Context) PostFormArray(key string) []string {
	values := []string{}

	// First check if it's a urlencoded form
	c.requestCtx.PostArgs().VisitAll(func(k, v []byte) {
		if string(k) == key {
			values = append(values, getString(v))
		}
	})

	// Then check if it's a multipart form
	form, err := c.requestCtx.MultipartForm()
	if err == nil {
		if vals, exists := form.Value[key]; exists {
			values = append(values, vals...)
		}
	}

	return values
}

// GetPostFormArray returns a slice of strings for a given form key, plus
// a boolean value whether at least one value exists for the given key
func (c *Context) GetPostFormArray(key string) ([]string, bool) {
	values := c.PostFormArray(key)
	return values, len(values) > 0
}

// PostFormMap returns a map for a given form key
func (c *Context) PostFormMap(key string) map[string]string {
	result := make(map[string]string)

	// First check if it's a urlencoded form
	c.requestCtx.PostArgs().VisitAll(func(k, v []byte) {
		keyStr := string(k)
		if i := strings.IndexByte(keyStr, '['); i >= 0 && strings.HasPrefix(keyStr, key) {
			if j := strings.IndexByte(keyStr[i+1:], ']'); j >= 0 {
				mapKey := keyStr[i+1 : i+1+j]
				result[mapKey] = getString(v)
			}
		}
	})

	// Then check if it's a multipart form
	form, err := c.requestCtx.MultipartForm()
	if err == nil {
		for formKey, values := range form.Value {
			if i := strings.IndexByte(formKey, '['); i >= 0 && strings.HasPrefix(formKey, key) {
				if j := strings.IndexByte(formKey[i+1:], ']'); j >= 0 {
					mapKey := formKey[i+1 : i+1+j]
					if len(values) > 0 {
						result[mapKey] = values[0]
					}
				}
			}
		}
	}

	return result
}

// GetPostFormMap returns a map for a given form key, plus a boolean value
// whether at least one value exists for the given key
func (c *Context) GetPostFormMap(key string) (map[string]string, bool) {
	result := c.PostFormMap(key)
	return result, len(result) > 0
}

// Bind checks the Method and Content-Type to select a binding engine automatically,
// Depending on the "Content-Type" header different bindings are used, for example:
//
//	"application/json" --> JSON binding
//	"application/xml"  --> XML binding
//
// It parses the request's body as JSON if Content-Type == "application/json" using JSON or XML as a JSON input
// It decodes the json payload into the struct specified as a pointer
// It writes a 400 error and sets the Content-Type header to "text/plain; charset=utf-8"
func (c *Context) Bind(obj any) error {
	method := getString(c.requestCtx.Method())
	contentType := getString(c.requestCtx.Request.Header.ContentType())

	b := DefaultBind(method, contentType)

	err := b.Bind(c.requestCtx, obj)
	if err != nil {
		c.requestCtx.Response.SetStatusCode(StatusBadRequest)
		c.requestCtx.Response.Header.SetContentType(MIMETextPlainCharsetUTF8)
		c.requestCtx.Response.SetBodyString(err.Error())
		return err
	}

	return nil
}

// BindJSON is a shortcut for c.MustBindWith(obj, JSON)
func (c *Context) BindJSON(obj any) error {
	return c.MustBindWith(obj, JSON)
}

// BindXML is a shortcut for c.MustBindWith(obj, XML)
func (c *Context) BindXML(obj any) error {
	return c.MustBindWith(obj, XML)
}

// BindQuery is a shortcut for c.MustBindWith(obj, Query)
func (c *Context) BindQuery(obj any) error {
	return c.MustBindWith(obj, Query)
}

// BindYAML is a shortcut for c.MustBindWith(obj, YAML)
func (c *Context) BindYAML(obj any) error {
	return c.MustBindWith(obj, YAML)
}

// BindTOML is a shortcut for c.MustBindWith(obj, TOML)
func (c *Context) BindTOML(obj any) error {
	return c.MustBindWith(obj, TOML)
}

// BindPlain is a shortcut for c.MustBindWith(obj, Plain)
func (c *Context) BindPlain(obj any) error {
	return c.MustBindWith(obj, Plain)
}

// BindHeader is a shortcut for c.MustBindWith(obj, Header)
func (c *Context) BindHeader(obj any) error {
	return c.MustBindWith(obj, Header)
}

// BindUri binds the passed struct pointer using URI
// It will abort the request with HTTP 400 if any error occurs
func (c *Context) BindUri(obj any) error {
	if err := Uri.BindUri(c.paramValues, obj); err != nil {
		c.requestCtx.Response.SetStatusCode(StatusBadRequest)
		c.requestCtx.Response.Header.SetContentType(MIMETextPlainCharsetUTF8)
		c.requestCtx.Response.SetBodyString(err.Error())
		return err
	}
	return nil
}

// BindForm binds form data to a struct
// It uses the "schema" tag to map form fields to struct fields
func (c *Context) BindForm(obj any) error {
	// Parse form data from the request
	args := c.requestCtx.PostArgs()
	if args == nil || args.Len() == 0 {
		return nil
	}

	// Convert fasthttp args to url.Values
	values := make(url.Values)
	args.VisitAll(func(key, value []byte) {
		values.Add(string(key), string(value))
	})

	// Use gorilla/schema to decode form values into the struct
	err := formDecoder.Decode(obj, values)
	if err != nil {
		log.Error(ErrFormBindingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrFormBind, err)
	}

	return nil
}

// BindMultipartForm binds multipart form data to a struct
// It handles both form fields and file uploads
// maxMemory limits the maximum size of in-memory uploaded files
func (c *Context) BindMultipartForm(obj any, maxMemory int64) error {
	// Parse multipart form
	form, err := c.requestCtx.MultipartForm()
	if err != nil {
		log.Error(ErrMultipartFormParsingFailed, "error", err)
		return err
	}

	// Convert form values to url.Values
	values := make(url.Values)
	for key, vals := range form.Value {
		for _, val := range vals {
			values.Add(key, val)
		}
	}

	// Use gorilla/schema to decode form values into the struct
	err = formDecoder.Decode(obj, values)
	if err != nil {
		log.Error(ErrFormBindingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrFormBind, err)
	}

	return nil
}

// MustBindWith binds the passed struct pointer using the specified binding engine
// It will abort the request with HTTP 400 if any error occurs
func (c *Context) MustBindWith(obj any, b Binding) error {
	if err := b.Bind(c.requestCtx, obj); err != nil {
		c.requestCtx.Response.SetStatusCode(StatusBadRequest)
		c.requestCtx.Response.Header.SetContentType(MIMETextPlainCharsetUTF8)
		c.requestCtx.Response.SetBodyString(err.Error())
		return err
	}
	return nil
}

// ShouldBind checks the Method and Content-Type to select a binding engine automatically,
// Depending on the "Content-Type" header different bindings are used, for example:
//
//	"application/json" --> JSON binding
//	"application/xml"  --> XML binding
//
// It parses the request's body as JSON if Content-Type == "application/json" using JSON or XML as a JSON input
// It decodes the json payload into the struct specified as a pointer
// Like c.Bind() but this method does not set the response status code to 400 or abort if input is not valid
func (c *Context) ShouldBind(obj any) error {
	method := getString(c.requestCtx.Method())
	contentType := getString(c.requestCtx.Request.Header.ContentType())

	b := DefaultBind(method, contentType)

	return b.Bind(c.requestCtx, obj)
}

// ShouldBindJSON is a shortcut for c.ShouldBindWith(obj, JSON)
func (c *Context) ShouldBindJSON(obj any) error {
	return c.ShouldBindWith(obj, JSON)
}

// ShouldBindXML is a shortcut for c.ShouldBindWith(obj, XML)
func (c *Context) ShouldBindXML(obj any) error {
	return c.ShouldBindWith(obj, XML)
}

// ShouldBindQuery is a shortcut for c.ShouldBindWith(obj, Query)
func (c *Context) ShouldBindQuery(obj any) error {
	return c.ShouldBindWith(obj, Query)
}

// ShouldBindYAML is a shortcut for c.ShouldBindWith(obj, YAML)
func (c *Context) ShouldBindYAML(obj any) error {
	return c.ShouldBindWith(obj, YAML)
}

// ShouldBindTOML is a shortcut for c.ShouldBindWith(obj, TOML)
func (c *Context) ShouldBindTOML(obj any) error {
	return c.ShouldBindWith(obj, TOML)
}

// ShouldBindPlain is a shortcut for c.ShouldBindWith(obj, Plain)
func (c *Context) ShouldBindPlain(obj any) error {
	return c.ShouldBindWith(obj, Plain)
}

// ShouldBindHeader is a shortcut for c.ShouldBindWith(obj, Header)
func (c *Context) ShouldBindHeader(obj any) error {
	return c.ShouldBindWith(obj, Header)
}

// ShouldBindUri binds the passed struct pointer using the specified binding engine
func (c *Context) ShouldBindUri(obj any) error {
	return Uri.BindUri(c.paramValues, obj)
}

// ShouldBindForm binds form data to the provided struct
func (c *Context) ShouldBindForm(obj any) error {
	return c.BindForm(obj)
}

// ShouldBindWith binds the passed struct pointer using the specified binding engine
func (c *Context) ShouldBindWith(obj any, b Binding) error {
	return b.Bind(c.requestCtx, obj)
}

// ShouldBindBodyWith is similar with ShouldBindWith, but it stores the request
// body into the context, and reuse when it is called again
// NOTE: This method reads the body before binding. So you should use
// ShouldBindWith for better performance if you need to call only once
func (c *Context) ShouldBindBodyWith(obj any, bb BindingBody) error {
	body := c.requestCtx.Request.Body()
	return bb.BindBody(body, obj)
}

// ShouldBindBodyWithJSON is a shortcut for c.ShouldBindBodyWith(obj, JSON)
func (c *Context) ShouldBindBodyWithJSON(obj any) error {
	return c.ShouldBindBodyWith(obj, JSON)
}

// ShouldBindBodyWithXML is a shortcut for c.ShouldBindBodyWith(obj, XML).
func (c *Context) ShouldBindBodyWithXML(obj any) error {
	return c.ShouldBindBodyWith(obj, XML)
}

// ShouldBindBodyWithYAML is a shortcut for c.ShouldBindBodyWith(obj, YAML)
func (c *Context) ShouldBindBodyWithYAML(obj any) error {
	return c.ShouldBindBodyWith(obj, YAML)
}

// ShouldBindBodyWithTOML is a shortcut for c.ShouldBindBodyWith(obj, TOML)
func (c *Context) ShouldBindBodyWithTOML(obj any) error {
	return c.ShouldBindBodyWith(obj, TOML)
}

// ShouldBindBodyWithPlain is a shortcut for c.ShouldBindBodyWith(obj, Plain)
func (c *Context) ShouldBindBodyWithPlain(obj any) error {
	return c.ShouldBindBodyWith(obj, Plain)
}

// ClientIP returns the client IP address
// It tries to determine the real IP address by checking various headers
// in the following order:
// 1. X-Forwarded-For
// 2. X-Real-IP
// 3. RemoteIP (direct connection)
func (c *Context) ClientIP() string {
	// Check X-Forwarded-For header first
	if xff := c.GetHeader(HeaderXForwardedFor); xff != "" {
		// X-Forwarded-For can contain multiple IPs (client, proxy1, proxy2, ...)
		// The client IP is the first one in the list
		if commaIndex := strings.IndexByte(xff, ','); commaIndex >= 0 {
			return strings.TrimSpace(xff[:commaIndex])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xrip := c.GetHeader(HeaderXRealIP); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	// Fall back to direct connection IP
	return c.RemoteIP()
}

// RemoteIP parses the IP from Request.RemoteAddr, normalizes and returns the IP (without the port)
func (c *Context) RemoteIP() string {
	return c.requestCtx.RemoteIP().String()
}

// ContentType returns the Content-Type header of the request
func (c *Context) ContentType() string {
	return getString(c.requestCtx.Request.Header.ContentType())
}

// IsWebsocket returns true if the request headers indicate that a websocket
// handshake is being initiated by the client
func (c *Context) IsWebsocket() bool {
	if strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade") &&
		strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		return true
	}
	return false
}

// Status sets the HTTP response code without sending any content
func (c *Context) Status(code int) *Context {
	c.requestCtx.Response.SetStatusCode(code)
	return c
}

// Header sets a response header
func (c *Context) Header(key, value string) *Context {
	c.requestCtx.Response.Header.Set(key, value)
	return c
}

// GetHeader returns value from request headers
func (c *Context) GetHeader(key string) string {
	return getString(c.requestCtx.Request.Header.PeekBytes(getBytes(key)))
}

// Body returns the complete raw request body as a string
// This provides access to the payload submitted in the HTTP request
func (c *Context) Body() string {
	return string(c.requestCtx.Request.Body())
}

// GetRawData returns the raw request body data as a byte slice
// It returns an error if the request body is nil
// This is equivalent to Gin's GetRawData method
func (c *Context) GetRawData() ([]byte, error) {
	body := c.requestCtx.Request.Body()
	if body == nil {
		return nil, ErrCannotReadNilBody
	}
	return body, nil
}

// SetCookie adds a Set-Cookie header to the ResponseWriter's headers
// The provided cookie must have a valid Name
// Invalid cookies may be silently dropped
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}

	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)

	cookie.SetKey(name)
	cookie.SetValue(url.QueryEscape(value))
	cookie.SetPath(path)
	cookie.SetDomain(domain)
	cookie.SetMaxAge(maxAge)
	cookie.SetSecure(secure)
	cookie.SetHTTPOnly(httpOnly)

	if maxAge > 0 {
		cookie.SetExpire(time.Now().Add(time.Duration(maxAge) * time.Second))
	} else if maxAge < 0 {
		cookie.SetExpire(time.Unix(1, 0))
	}

	c.requestCtx.Response.Header.SetCookie(cookie)
}

// Cookie returns the named cookie provided in the request or ErrNoCookie if not found
// And return the named cookie is unescaped
// If multiple cookies match the given name, only one cookie will be returned
func (c *Context) Cookie(name string) (string, error) {
	cookie := c.requestCtx.Request.Header.Cookie(name)
	if len(cookie) == 0 {
		return "", ErrNamedCookieNotPresent
	}
	val, err := url.QueryUnescape(string(cookie))
	if err != nil {
		return "", err
	}
	return val, nil
}

// HTML renders an HTML template with data
func (c *Context) HTML(code int, name string, obj any) {
	k, ok := c.requestCtx.UserValue("gonoleksApp").(*gonoleks)
	if !ok || k.htmlRender == nil {
		_ = c.AbortWithError(StatusInternalServerError, ErrTemplateEngineNotSet)
		return
	}

	// Get the template renderer instance
	render := k.htmlRender.Instance(name, obj)

	// Set status code and content type
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.Set("Content-Type", MIMETextHTMLCharsetUTF8)

	// Render the template
	if err := render.Render(c.requestCtx); err != nil {
		_ = c.AbortWithError(StatusInternalServerError, fmt.Errorf("%v: %w", ErrHTMLTemplateRender, err))
	}
}

// JSON serializes the given struct as JSON into the response body
// It also sets the Content-Type as "application/json; charset=utf-8"
func (c *Context) JSON(code int, obj any) error {
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationJSONCharsetUTF8)
	c.requestCtx.Response.SetStatusCode(code)

	// Use pre-allocated buffer from fasthttp for better performance
	jsonBytes, err := sonic.ConfigFastest.Marshal(obj)
	if err != nil {
		log.Error(ErrJSONMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrJSONMarshal, err)
	}

	// Write directly to response body
	c.requestCtx.Response.SetBody(jsonBytes)
	return nil
}

// IndentedJSON serializes the provided data to formatted JSON with indentation and line breaks
// This format is more human-readable but less efficient for production use
// It automatically sets the Content-Type header to "application/json"
func (c *Context) IndentedJSON(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationJSON)

	raw, err := sonic.ConfigFastest.MarshalIndent(obj, "", "    ")
	if err != nil {
		log.Error(ErrIndentedJSONMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrIndentedJSONMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// SecureJSON serializes the provided data to JSON with a security prefix
// The prefix helps prevent JSON hijacking attacks by making the response non-executable
// It automatically sets the Content-Type header to "application/json"
func (c *Context) SecureJSON(code int, obj any) error {
	const securePrefix = "while(1);"

	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationJSON)

	raw, err := sonic.ConfigFastest.Marshal(obj)
	if err != nil {
		log.Error(ErrSecureJSONMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrSecureJSONMarshal, err)
	}

	// Prefix the JSON with the secure string
	c.requestCtx.Response.SetBodyRaw(getBytes(securePrefix + string(raw)))
	return nil
}

// AsciiJSON serializes the provided data to JSON with all non-ASCII characters escaped
// This format ensures compatibility with systems that cannot handle Unicode characters
// It automatically sets the Content-Type header to "application/json"
func (c *Context) AsciiJSON(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationJSON)

	ret, err := sonic.ConfigFastest.Marshal(obj)
	if err != nil {
		log.Error(ErrAsciiJSONMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrAsciiJSONMarshal, err)
	}

	// Escape all non-ASCII and special characters as \uXXXX
	var builder strings.Builder
	for _, r := range string(ret) {
		if r < 0x20 || r > 0x7e || r == '<' || r == '>' || r == '&' {
			builder.WriteString("\\u")
			hex := strconv.FormatInt(int64(r), 16)
			for len(hex) < 4 {
				hex = "0" + hex
			}
			builder.WriteString(hex)
		} else {
			builder.WriteRune(r)
		}
	}
	asciiJSON := builder.String()

	c.requestCtx.Response.SetBodyRaw(getBytes(asciiJSON))
	return nil
}

// PureJSON serializes the provided data to JSON without escaping HTML characters
// This format is useful when the JSON payload contains HTML that should be preserved
// It automatically sets the Content-Type header to "application/json"
func (c *Context) PureJSON(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationJSON)

	raw, err := sonic.ConfigFastest.Marshal(obj)
	if err != nil {
		log.Error(ErrPureJSONMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrPureJSONMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// XML serializes the provided data to XML format and sets it as the response body
// It automatically sets the Content-Type header to "application/xml"
func (c *Context) XML(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationXML)

	raw, err := xml.Marshal(obj)
	if err != nil {
		log.Error(ErrXMLMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrXMLMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// YAML serializes the provided data to YAML format and sets it as the response body
// It automatically sets the Content-Type header to "application/x-yaml"
func (c *Context) YAML(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationYAML)

	raw, err := yaml.Marshal(obj)
	if err != nil {
		log.Error(ErrYAMLMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrXMLMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// TOML serializes the provided data to TOML format and sets it as the response body
// It automatically sets the Content-Type header to "application/toml"
func (c *Context) TOML(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationTOML)

	raw, err := toml.Marshal(obj)
	if err != nil {
		log.Error(ErrTOMLMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrTOMLMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// ProtoBuf serializes the provided data to Protocol Buffer format and sets it as the response body
// It automatically sets the Content-Type header to "application/x-protobuf"
// The data parameter must implement the proto.Message interface
func (c *Context) ProtoBuf(code int, obj any) error {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(MIMEApplicationProtoBuf)

	// Check if data implements proto.Message interface
	msg, ok := obj.(proto.Message)
	if !ok {
		err := ErrProtoMessageInterface
		log.Error(ErrProtoBufMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrProtoBufMarshal, err)
	}

	raw, err := proto.Marshal(msg)
	if err != nil {
		log.Error(ErrProtoBufMarshalingFailed, "error", err)
		return fmt.Errorf("%v: %w", ErrProtoBufMarshal, err)
	}

	c.requestCtx.Response.SetBodyRaw(raw)
	return nil
}

// String sets body of response for string type
func (c *Context) String(code int, format string, values ...any) *Context {
	c.requestCtx.Response.SetStatusCode(code)
	formatted := fmt.Sprintf(format, values...)
	c.requestCtx.Response.SetBodyRaw(getBytes(formatted))
	return c
}

// ParseBody deserializes the request body into the provided data structure
// Supports JSON, form data, and multipart form content types
func (c *Context) ParseBody(obj any) error {
	contentType := string(c.requestCtx.Request.Header.ContentType())

	// Handle JSON content type
	if strings.HasPrefix(contentType, MIMEApplicationJSON) {
		err := sonic.ConfigFastest.Unmarshal(c.requestCtx.Request.Body(), obj)
		if err != nil {
			log.Error(ErrJSONParsingFailed, "error", err)
			return err
		}
		return nil
	}

	// Handle form data content type
	if strings.HasPrefix(contentType, MIMEApplicationForm) {
		return c.BindForm(obj)
	}

	// Handle multipart form content type
	if strings.HasPrefix(contentType, MIMEMultipartForm) {
		// Default to 32MB max memory
		return c.BindMultipartForm(obj, 32<<20)
	}

	log.Error(ErrUnsupportedContentType, "contentType", contentType)
	return fmt.Errorf("%v: %s", ErrContentType, contentType)
}

// Redirect performs an HTTP redirect to the specified location
// It sets the appropriate status code (usually 301, 302, 307, or 308) and the Location header
// Returns the context instance for method chaining
func (c *Context) Redirect(code int, location string) *Context {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.Set("Location", location)
	return c
}

// Data writes the given data to the response body and sets the Content-Type
func (c *Context) Data(code int, contentType string, data []byte) *Context {
	c.requestCtx.Response.SetStatusCode(code)
	c.requestCtx.Response.Header.SetContentType(contentType)
	c.requestCtx.Response.SetBodyRaw(data)
	return c
}

// File writes the specified file into the body stream in an efficient way
func (c *Context) File(filePath string) {
	c.requestCtx.SendFile(filePath)
}

// FileAttachment writes the specified file into the body stream in an efficient way
// On the client side, the file will typically be downloaded with the given filename
func (c *Context) FileAttachment(filePath, fileName string) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_ = c.AbortWithError(StatusNotFound, ErrFileNotFound)
		return
	}

	// Set Content-Disposition header for attachment
	c.requestCtx.Response.Header.Set(HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", fileName))

	// Use SendFile for efficient file serving
	c.requestCtx.SendFile(filePath)
}

// Negotiate performs content negotiation based on the Accept header
// It selects the best matching format from the offered formats and responds with the appropriate data
func (c *Context) Negotiate(code int, config Negotiate) error {
	if len(config.Offered) == 0 {
		return ErrOfferedFormatsNotProvided
	}

	// Get the best matching format based on the Accept header
	format := c.NegotiateFormat(config.Offered...)

	// Respond with the appropriate format
	switch format {
	case MIMEApplicationJSON:
		return c.JSON(code, config.JSONData)
	case MIMEApplicationXML:
		return c.XML(code, config.XMLData)
	case MIMEApplicationYAML:
		return c.YAML(code, config.YAMLData)
	case MIMEApplicationTOML:
		return c.TOML(code, config.TOMLData)
	case MIMETextHTML:
		c.HTML(code, config.HTMLName, config.HTMLData)
		return nil
	default:
		// If no specific format matches, use the generic Data field
		if config.Data != nil {
			// Convert data to bytes
			var data []byte
			var contentType string

			switch d := config.Data.(type) {
			case string:
				data = []byte(d)
				contentType = MIMETextPlain
			case []byte:
				data = d
				contentType = MIMEOctetStream
			default:
				// Default to JSON for other types
				var err error
				data, err = sonic.Marshal(config.Data)
				if err != nil {
					return err
				}
				contentType = MIMEApplicationJSON
			}

			c.Data(code, contentType, data)
			return nil
		}

		// If no format matches and no Data field, return an error
		return ErrMatchingFormatNotFound
	}
}

// NegotiateFormat returns the most preferred content type from the client's Accept header
// If none of the offered formats match, returns an empty string
func (c *Context) NegotiateFormat(offered ...string) string {
	if len(offered) == 0 {
		return ""
	}

	acceptHeader := c.GetHeader("Accept")
	if acceptHeader == "" {
		return offered[0]
	}

	// Parse Accept header
	parts := strings.Split(acceptHeader, ",")

	// Simple implementation: just check if any of the offered formats
	// are in the Accept header, in the order they appear
	for _, part := range parts {
		mimeType := strings.TrimSpace(strings.Split(part, ";")[0])
		for _, offer := range offered {
			if mimeType == offer {
				return offer
			}
		}
	}

	return ""
}

// SetAccepted sets the formats that are accepted by the client
func (c *Context) SetAccepted(formats ...string) {
	c.Header("Accept", strings.Join(formats, ", "))
}
