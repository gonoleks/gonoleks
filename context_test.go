package gonoleks

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/gonoleks/gonoleks/testdata/protoexample"
)

//go:embed testdata/test_file.txt
var testEmbedFS embed.FS

type TestUser struct {
	Name  string `json:"name" xml:"name" yaml:"name"`
	Email string `json:"email" xml:"email" yaml:"email"`
}

func createTestContext() (*Context, *fasthttp.RequestCtx) {
	requestCtx := &fasthttp.RequestCtx{}

	// Create a test Gonoleks app instance
	app := &Gonoleks{
		secureJsonPrefix: "while(1);",
	}

	// Set the gonoleks app in the request context for methods that need it
	requestCtx.SetUserValue("gonoleksApp", app)

	ctx := &Context{
		requestCtx:  requestCtx,
		paramValues: make(map[string]string),
		handlers:    make(handlersChain, 0),
		index:       -1,
		fullPath:    "/test",
	}
	return ctx, requestCtx
}

func TestContextBasics(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test Context() method
	assert.Equal(t, requestCtx, ctx.Context())

	// Test FullPath
	ctx.fullPath = "/user/:id"
	assert.Equal(t, "/user/:id", ctx.FullPath())

	// Test Copy
	ctx.paramValues["id"] = "123"
	ctx.handlers = append(ctx.handlers, func(c *Context) {})
	ctx.index = 0
	copy := ctx.Copy()
	assert.Equal(t, ctx.fullPath, copy.fullPath)
	assert.Equal(t, ctx.index, copy.index)
	assert.Equal(t, "123", copy.paramValues["id"])
	assert.Equal(t, len(ctx.handlers), len(copy.handlers))

	// Test Set/Get/MustGet
	ctx.Set("key", "value")
	val, exists := ctx.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value", val)
	val, exists = ctx.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, val)
	assert.Equal(t, "value", ctx.MustGet("key"))
	assert.Panics(t, func() { ctx.MustGet("nonexistent") })
}

func TestContextHandlerFlow(t *testing.T) {
	ctx, _ := createTestContext()

	// Test Next and handler execution
	handlerCalled := false
	ctx.handlers = append(ctx.handlers, func(c *Context) {
		handlerCalled = true
	})
	ctx.Next()
	assert.True(t, handlerCalled)
	assert.Equal(t, 0, ctx.index)

	// Test IsAborted and Abort
	ctx.handlers = append(ctx.handlers, func(c *Context) {})
	assert.False(t, ctx.IsAborted())
	ctx.Abort()
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, len(ctx.handlers), ctx.index)

	// Test AbortWithStatus
	ctx, requestCtx := createTestContext()
	ctx.AbortWithStatus(StatusUnauthorized)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusUnauthorized, requestCtx.Response.StatusCode())

	// Test AbortWithStatusJSON
	ctx, requestCtx = createTestContext()
	err := ctx.AbortWithStatusJSON(StatusUnauthorized, map[string]string{"error": "unauthorized"})
	assert.Nil(t, err)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusUnauthorized, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "unauthorized")

	// Test AbortWithError
	ctx, requestCtx = createTestContext()
	testErr := errors.New("test error")
	err = ctx.AbortWithError(StatusInternalServerError, testErr)
	assert.Equal(t, testErr, err)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusInternalServerError, requestCtx.Response.StatusCode())
}

func TestContextParameters(t *testing.T) {
	ctx, _ := createTestContext()

	// Test AddParam and Param
	ctx.AddParam("id", "123")
	assert.Equal(t, "123", ctx.paramValues["id"])
	assert.Equal(t, "123", ctx.Param("id"))
}

func TestContextQueryParams(t *testing.T) {
	ctx, requestCtx := createTestContext()
	requestCtx.QueryArgs().Add("name", "john")
	requestCtx.QueryArgs().Add("empty", "")
	requestCtx.QueryArgs().Add("ids", "1")
	requestCtx.QueryArgs().Add("ids", "2")
	requestCtx.QueryArgs().Add("user[name]", "alice")
	requestCtx.QueryArgs().Add("user[email]", "alice@example.com")

	// Test Query methods
	assert.Equal(t, "john", ctx.Query("name"))
	assert.Equal(t, "", ctx.Query("nonexistent"))
	assert.Equal(t, "john", ctx.DefaultQuery("name", "default"))
	assert.Equal(t, "default", ctx.DefaultQuery("nonexistent", "default"))

	// Test GetQuery
	val, exists := ctx.GetQuery("name")
	assert.True(t, exists)
	assert.Equal(t, "john", val)
	val, exists = ctx.GetQuery("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", val)

	// Test QueryArray
	ids := ctx.QueryArray("ids")
	assert.Equal(t, 2, len(ids))
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "2")

	// Test QueryMap
	userMap := ctx.QueryMap("user")
	assert.Equal(t, 2, len(userMap))
	assert.Equal(t, "alice", userMap["name"])
	assert.Equal(t, "alice@example.com", userMap["email"])
}

func TestContextPostForm(t *testing.T) {
	ctx, requestCtx := createTestContext()
	requestCtx.Request.Header.SetContentType(MIMEApplicationForm)
	requestCtx.PostArgs().Add("name", "john")
	requestCtx.PostArgs().Add("empty", "")
	requestCtx.PostArgs().Add("ids", "1")
	requestCtx.PostArgs().Add("ids", "2")
	requestCtx.PostArgs().Add("user[name]", "bob")
	requestCtx.PostArgs().Add("user[email]", "bob@example.com")

	// Test PostForm methods
	assert.Equal(t, "john", ctx.PostForm("name"))
	assert.Equal(t, "", ctx.PostForm("nonexistent"))
	assert.Equal(t, "john", ctx.DefaultPostForm("name", "default"))
	assert.Equal(t, "default", ctx.DefaultPostForm("nonexistent", "default"))

	// Test GetPostForm
	val, exists := ctx.GetPostForm("name")
	assert.True(t, exists)
	assert.Equal(t, "john", val)
	val, exists = ctx.GetPostForm("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", val)

	// Test PostFormArray
	ids := ctx.PostFormArray("ids")
	assert.Equal(t, 2, len(ids))
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "2")

	// Test PostFormMap
	userMap := ctx.PostFormMap("user")
	assert.Equal(t, 2, len(userMap))
	assert.Equal(t, "bob", userMap["name"])
	assert.Equal(t, "bob@example.com", userMap["email"])
}

func TestContextClientIP(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test with X-Forwarded-For header
	requestCtx.Request.Header.Set(HeaderXForwardedFor, "192.168.1.100, 10.0.0.1")
	ip := ctx.ClientIP()
	assert.Equal(t, "192.168.1.100", ip)

	// Test with X-Real-IP header
	ctx, requestCtx = createTestContext()
	requestCtx.Request.Header.Set(HeaderXRealIP, "192.168.1.200")
	ip = ctx.ClientIP()
	assert.Equal(t, "192.168.1.200", ip)

	// Test fallback to RemoteIP
	ctx, _ = createTestContext()
	ip = ctx.ClientIP()
	assert.Equal(t, "0.0.0.0", ip)
}

func TestContextHeaders(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test Status and Header
	ctx.Status(StatusCreated)
	assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())
	ctx.Header(HeaderXTest, "value")
	assert.Equal(t, "value", string(requestCtx.Response.Header.Peek(HeaderXTest)))

	// Test GetHeader
	requestCtx.Request.Header.Set(HeaderXTest, "request-value")
	assert.Equal(t, "request-value", ctx.GetHeader(HeaderXTest))
	assert.Equal(t, "", ctx.GetHeader("Nonexistent"))

	// Test GetRawData
	testData := []byte("test data")
	requestCtx.Request.SetBody(testData)
	data, err := ctx.GetRawData()
	assert.Nil(t, err)
	assert.Equal(t, testData, data)

	// Test Cookie and SetCookie
	ctx.SetCookie("test", "value", 3600, "/", "example.com", true, true)
	requestCtx.Request.Header.SetCookie("test", "value")
	val, err := ctx.Cookie("test")
	assert.Nil(t, err)
	assert.Equal(t, "value", val)
	_, err = ctx.Cookie("nonexistent")
	assert.NotNil(t, err)

	// Test SetAccepted
	ctx.SetAccepted(MIMEApplicationJSON, MIMEApplicationXML)
	acceptHeader := string(requestCtx.Response.Header.Peek(HeaderAccept))
	assert.Equal(t, "application/json, application/xml", acceptHeader)
}

func TestContextJSONRendering(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := TestUser{Name: "john", Email: "john@example.com"}

	// Test basic JSON
	err := ctx.JSON(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, "application/json; charset=utf-8", string(requestCtx.Response.Header.ContentType()))
	body := string(requestCtx.Response.Body())
	assert.Contains(t, body, "john")
	assert.Contains(t, body, "john@example.com")

	// Test IndentedJSON
	ctx, requestCtx = createTestContext()
	err = ctx.IndentedJSON(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "john")

	// Test SecureJSON
	ctx, requestCtx = createTestContext()
	err = ctx.SecureJSON(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	body = string(requestCtx.Response.Body())
	assert.Contains(t, body, "john")
	assert.True(t, body[0:5] == "while" || body[0:1] == "{")

	// Test AsciiJSON
	ctx, requestCtx = createTestContext()
	err = ctx.AsciiJSON(StatusOK, map[string]string{"message": "hello 世界"})
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	body = string(requestCtx.Response.Body())
	assert.Contains(t, body, "\\u4e16\\u754c")

	// Test PureJSON
	ctx, requestCtx = createTestContext()
	err = ctx.PureJSON(StatusOK, map[string]string{"html": "<b>Hello</b>"})
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	body = string(requestCtx.Response.Body())
	assert.Contains(t, body, "<b>Hello</b>")
}

func TestContextXMLRendering(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := TestUser{Name: "john", Email: "john@example.com"}

	err := ctx.XML(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationXML, string(requestCtx.Response.Header.ContentType()))
	body := string(requestCtx.Response.Body())
	assert.Contains(t, body, "<TestUser>")
	assert.Contains(t, body, "<name>john</name>")
	assert.Contains(t, body, "<email>john@example.com</email>")
	assert.Contains(t, body, "</TestUser>")
}

func TestContextYAMLRendering(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := TestUser{Name: "john", Email: "john@example.com"}

	err := ctx.YAML(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationYAML, string(requestCtx.Response.Header.ContentType()))
	body := string(requestCtx.Response.Body())
	assert.Contains(t, body, "name: john")
	assert.Contains(t, body, "email: john@example.com")
}

func TestContextProtoBuf(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := &protoexample.TestMessage{
		Name:  "test",
		Id:    123,
		Email: "test@example.com",
		Tags:  []string{"tag1", "tag2"},
	}

	err := ctx.ProtoBuf(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationProtoBuf, string(requestCtx.Response.Header.ContentType()))
	assert.NotEmpty(t, requestCtx.Response.Body())
}

func TestContextStringRendering(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test basic string
	result := ctx.String(StatusOK, "Hello %s", "World")
	assert.NotNil(t, result)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMETextPlainCharsetUTF8, string(requestCtx.Response.Header.ContentType()))
	assert.Equal(t, "Hello World", string(requestCtx.Response.Body()))

	// Test with UTF-8
	ctx, requestCtx = createTestContext()
	result = ctx.String(StatusOK, "Hello 世界")
	assert.NotNil(t, result)
	assert.Equal(t, "Hello 世界", string(requestCtx.Response.Body()))
}

func TestContextDataAndRedirect(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test Data
	testData := []byte("test data")
	ctx.Data(StatusOK, MIMETextPlain, testData)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
	assert.Equal(t, testData, requestCtx.Response.Body())

	// Test Redirect
	ctx, requestCtx = createTestContext()
	ctx.Redirect(StatusFound, "https://example.com")
	assert.Equal(t, StatusFound, requestCtx.Response.StatusCode())
	assert.Equal(t, "https://example.com", string(requestCtx.Response.Header.Peek(HeaderLocation)))
}

func TestContextFileHandling(t *testing.T) {
	// Test file existence check
	testFilePath := "testdata/test_file.txt"
	_, err := os.Stat(testFilePath)
	assert.NoError(t, err, "Test file should exist")

	// Test FileFromFS with MapFS
	testFS := fstest.MapFS{
		"test.txt": &fstest.MapFile{
			Data: []byte("Hello from test filesystem!"),
		},
	}
	data, err := fs.ReadFile(testFS, "test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "Hello from test filesystem!", string(data))

	// Test that FileFromFS method can be called
	assert.NotPanics(t, func() {
		ctx, _ := createTestContext()
		ctx.FileFromFS("test.txt", testFS)
	})

	// Test with embed.FS
	var embedFS fs.FS = testEmbedFS
	data, err = fs.ReadFile(embedFS, "testdata/test_file.txt")
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	assert.NotPanics(t, func() {
		ctx, _ := createTestContext()
		ctx.FileFromFS("testdata/test_file.txt", embedFS)
	})
}
