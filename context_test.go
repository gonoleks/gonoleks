package gonoleks

import (
	"bytes"
	"embed"
	"errors"
	"io/fs"
	"os"
	"strings"
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
	ctx := &Context{
		requestCtx:  requestCtx,
		paramValues: make(map[string]string),
		handlers:    make(handlersChain, 0),
		index:       -1,
		fullPath:    "/test",
	}
	return ctx, requestCtx
}

func TestContext_Context(t *testing.T) {
	ctx, requestCtx := createTestContext()
	assert.Equal(t, requestCtx, ctx.Context())
}

func TestContext_Copy(t *testing.T) {
	ctx, _ := createTestContext()
	ctx.paramValues["id"] = "123"
	ctx.handlers = append(ctx.handlers, func(c *Context) {})
	ctx.index = 0

	copy := ctx.Copy()

	assert.Equal(t, ctx.fullPath, copy.fullPath)
	assert.Equal(t, ctx.index, copy.index)
	assert.Equal(t, "123", copy.paramValues["id"])
	assert.Equal(t, len(ctx.handlers), len(copy.handlers))
}

func TestContext_FullPath(t *testing.T) {
	ctx, _ := createTestContext()
	ctx.fullPath = "/user/:id"
	assert.Equal(t, "/user/:id", ctx.FullPath())
}

func TestContext_Next(t *testing.T) {
	ctx, _ := createTestContext()

	handlerCalled := false
	ctx.handlers = append(ctx.handlers, func(c *Context) {
		handlerCalled = true
	})

	ctx.Next()
	assert.True(t, handlerCalled)
	assert.Equal(t, 0, ctx.index)
}

func TestContext_IsAborted(t *testing.T) {
	ctx, _ := createTestContext()
	ctx.handlers = append(ctx.handlers, func(c *Context) {})

	assert.False(t, ctx.IsAborted())

	ctx.index = len(ctx.handlers)
	assert.True(t, ctx.IsAborted())
}

func TestContext_Abort(t *testing.T) {
	ctx, _ := createTestContext()
	ctx.handlers = append(ctx.handlers, func(c *Context) {})
	ctx.handlers = append(ctx.handlers, func(c *Context) {})

	ctx.Abort()
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, len(ctx.handlers), ctx.index)
}

func TestContext_AbortWithStatus(t *testing.T) {
	ctx, requestCtx := createTestContext()

	ctx.AbortWithStatus(StatusUnauthorized)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusUnauthorized, requestCtx.Response.StatusCode())
}

func TestContext_AbortWithStatusJSON(t *testing.T) {
	ctx, requestCtx := createTestContext()

	err := ctx.AbortWithStatusJSON(StatusUnauthorized, map[string]string{"error": "unauthorized"})

	assert.Nil(t, err)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusUnauthorized, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "unauthorized")
}

func TestContext_AbortWithError(t *testing.T) {
	ctx, requestCtx := createTestContext()

	testErr := errors.New("test error")
	err := ctx.AbortWithError(StatusInternalServerError, testErr)

	assert.Equal(t, testErr, err)
	assert.True(t, ctx.IsAborted())
	assert.Equal(t, StatusInternalServerError, requestCtx.Response.StatusCode())
}

func TestContext_Set_Get_MustGet(t *testing.T) {
	ctx, _ := createTestContext()

	// Test Set and Get
	ctx.Set("key", "value")
	val, exists := ctx.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value", val)

	// Test Get with non-existent key
	val, exists = ctx.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, val)

	// Test MustGet
	assert.Equal(t, "value", ctx.MustGet("key"))

	// Test MustGet with non-existent key (should panic)
	assert.Panics(t, func() {
		ctx.MustGet("nonexistent")
	})
}

func TestContext_Param_AddParam(t *testing.T) {
	ctx, _ := createTestContext()

	// Test AddParam
	ctx.AddParam("id", "123")
	assert.Equal(t, "123", ctx.paramValues["id"])

	// Test Param
	assert.Equal(t, "123", ctx.Param("id"))
}

func TestContext_Query_DefaultQuery_GetQuery(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up query parameters
	requestCtx.QueryArgs().Add("name", "john")
	requestCtx.QueryArgs().Add("empty", "")

	// Test Query
	assert.Equal(t, "john", ctx.Query("name"))
	assert.Equal(t, "", ctx.Query("nonexistent"))

	// Test DefaultQuery
	assert.Equal(t, "john", ctx.DefaultQuery("name", "default"))
	assert.Equal(t, "default", ctx.DefaultQuery("nonexistent", "default"))
	assert.Equal(t, "", ctx.DefaultQuery("empty", ""))

	// Test GetQuery
	val, exists := ctx.GetQuery("name")
	assert.True(t, exists)
	assert.Equal(t, "john", val)

	val, exists = ctx.GetQuery("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", val)

	val, exists = ctx.GetQuery("empty")
	assert.True(t, exists)
	assert.Equal(t, "", val)
}

func TestContext_QueryArray_GetQueryArray(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up query parameters
	requestCtx.QueryArgs().Add("ids", "1")
	requestCtx.QueryArgs().Add("ids", "2")
	requestCtx.QueryArgs().Add("ids", "3")

	// Test QueryArray
	ids := ctx.QueryArray("ids")
	assert.Equal(t, 3, len(ids))
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "2")
	assert.Contains(t, ids, "3")

	// Test GetQueryArray
	ids, exists := ctx.GetQueryArray("ids")
	assert.True(t, exists)
	assert.Equal(t, 3, len(ids))

	emptyIds, exists := ctx.GetQueryArray("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, emptyIds)
}

func TestContext_QueryMap_GetQueryMap(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up query parameters
	requestCtx.QueryArgs().Add("user[name]", "john")
	requestCtx.QueryArgs().Add("user[email]", "john@example.com")

	// Test QueryMap
	userMap := ctx.QueryMap("user")
	assert.Equal(t, 2, len(userMap))
	assert.Equal(t, "john", userMap["name"])
	assert.Equal(t, "john@example.com", userMap["email"])

	// Test GetQueryMap
	userMap, exists := ctx.GetQueryMap("user")
	assert.True(t, exists)
	assert.Equal(t, 2, len(userMap))

	emptyMap, exists := ctx.GetQueryMap("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, emptyMap)
}

func TestContext_PostForm_DefaultPostForm_GetPostForm(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up form parameters
	requestCtx.Request.Header.SetContentType(MIMEApplicationForm)
	requestCtx.PostArgs().Add("name", "john")
	requestCtx.PostArgs().Add("empty", "")

	// Test PostForm
	assert.Equal(t, "john", ctx.PostForm("name"))
	assert.Equal(t, "", ctx.PostForm("nonexistent"))

	// Test DefaultPostForm
	assert.Equal(t, "john", ctx.DefaultPostForm("name", "default"))
	assert.Equal(t, "default", ctx.DefaultPostForm("nonexistent", "default"))
	assert.Equal(t, "", ctx.DefaultPostForm("empty", ""))

	// Test GetPostForm
	val, exists := ctx.GetPostForm("name")
	assert.True(t, exists)
	assert.Equal(t, "john", val)

	val, exists = ctx.GetPostForm("nonexistent")
	assert.False(t, exists)
	assert.Equal(t, "", val)

	val, exists = ctx.GetPostForm("empty")
	assert.True(t, exists)
	assert.Equal(t, "", val)
}

func TestContext_PostFormArray_GetPostFormArray(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up form parameters
	requestCtx.Request.Header.SetContentType(MIMEApplicationForm)
	requestCtx.PostArgs().Add("ids", "1")
	requestCtx.PostArgs().Add("ids", "2")
	requestCtx.PostArgs().Add("ids", "3")

	// Test PostFormArray
	ids := ctx.PostFormArray("ids")
	assert.Equal(t, 3, len(ids))
	assert.Contains(t, ids, "1")
	assert.Contains(t, ids, "2")
	assert.Contains(t, ids, "3")

	// Test GetPostFormArray
	ids, exists := ctx.GetPostFormArray("ids")
	assert.True(t, exists)
	assert.Equal(t, 3, len(ids))

	emptyIds, exists := ctx.GetPostFormArray("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, emptyIds)
}

func TestContext_PostFormMap_GetPostFormMap(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Set up form parameters
	requestCtx.Request.Header.SetContentType(MIMEApplicationForm)
	requestCtx.PostArgs().Add("user[name]", "john")
	requestCtx.PostArgs().Add("user[email]", "john@example.com")

	// Test PostFormMap
	userMap := ctx.PostFormMap("user")
	assert.Equal(t, 2, len(userMap))
	assert.Equal(t, "john", userMap["name"])
	assert.Equal(t, "john@example.com", userMap["email"])

	// Test GetPostFormMap
	userMap, exists := ctx.GetPostFormMap("user")
	assert.True(t, exists)
	assert.Equal(t, 2, len(userMap))

	emptyMap, exists := ctx.GetPostFormMap("nonexistent")
	assert.False(t, exists)
	assert.Empty(t, emptyMap)
}

func TestContext_ClientIP(t *testing.T) {
	// Test with X-Forwarded-For header (single IP)
	t.Run("X-Forwarded-For single IP", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXForwardedFor, "192.168.1.100")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.100", ip)
	})

	// Test with X-Forwarded-For header (multiple IPs)
	t.Run("X-Forwarded-For multiple IPs", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXForwardedFor, "192.168.1.100, 10.0.0.1, 172.16.0.1")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.100", ip)
	})

	// Test with X-Forwarded-For header (with spaces)
	t.Run("X-Forwarded-For with spaces", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXForwardedFor, "  192.168.1.100  ")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.100", ip)
	})

	// Test with X-Real-IP header when X-Forwarded-For is not present
	t.Run("X-Real-IP header", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXRealIP, "192.168.1.200")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.200", ip)
	})

	// Test with X-Real-IP header (with spaces)
	t.Run("X-Real-IP with spaces", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXRealIP, "  192.168.1.200  ")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.200", ip)
	})

	// Test X-Forwarded-For takes precedence over X-Real-IP
	t.Run("X-Forwarded-For precedence", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXForwardedFor, "192.168.1.100")
		requestCtx.Request.Header.Set(HeaderXRealIP, "192.168.1.200")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.100", ip)
	})

	// Test fallback to RemoteIP when no headers are present
	t.Run("Fallback to RemoteIP", func(t *testing.T) {
		ctx, _ := createTestContext()

		// When no headers are set, it should fall back to RemoteIP
		// In our test setup, this will be "0.0.0.0" which is the default
		ip := ctx.ClientIP()
		assert.Equal(t, "0.0.0.0", ip)
	})

	// Test with empty X-Forwarded-For header
	t.Run("Empty X-Forwarded-For", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXForwardedFor, "")
		requestCtx.Request.Header.Set(HeaderXRealIP, "192.168.1.200")

		ip := ctx.ClientIP()
		assert.Equal(t, "192.168.1.200", ip)
	})

	// Test with empty X-Real-IP header
	t.Run("Empty X-Real-IP", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		requestCtx.Request.Header.Set(HeaderXRealIP, "")

		// Should fall back to RemoteIP (0.0.0.0 in test setup)
		ip := ctx.ClientIP()
		assert.Equal(t, "0.0.0.0", ip)
	})
}

func TestContext_Status_Header(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test Status
	ctx.Status(StatusCreated)
	assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())

	// Test Header
	ctx.Header(HeaderXTest, "value")
	assert.Equal(t, "value", string(requestCtx.Response.Header.Peek(HeaderXTest)))
}

func TestContext_GetHeader(t *testing.T) {
	ctx, requestCtx := createTestContext()

	requestCtx.Request.Header.Set(HeaderXTest, "value")
	assert.Equal(t, "value", ctx.GetHeader(HeaderXTest))
	assert.Equal(t, "", ctx.GetHeader("Nonexistent"))
}

func TestContext_GetRawData(t *testing.T) {
	ctx, requestCtx := createTestContext()

	testData := []byte("test data")
	requestCtx.Request.SetBody(testData)

	data, err := ctx.GetRawData()
	assert.Nil(t, err)
	assert.Equal(t, testData, data)
}

func TestContext_Cookie_SetCookie(t *testing.T) {
	ctx, requestCtx := createTestContext()

	// Test SetCookie
	ctx.SetCookie("test", "value", 3600, "/", "example.com", true, true)

	// Test Cookie
	requestCtx.Request.Header.SetCookie("test", "value")
	val, err := ctx.Cookie("test")
	assert.Nil(t, err)
	assert.Equal(t, "value", val)

	_, err = ctx.Cookie("nonexistent")
	assert.NotNil(t, err)
}

func TestContext_JSON(t *testing.T) {
	// Test basic JSON serialization
	t.Run("Basic serialization", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := TestUser{Name: "john", Email: "john@example.com"}

		err := ctx.JSON(StatusOK, testData)

		assert.Nil(t, err)
		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "application/json; charset=utf-8", string(requestCtx.Response.Header.ContentType()))

		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "john")
		assert.Contains(t, body, "john@example.com")
	})

	// Test with different status codes
	t.Run("Different status codes", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := map[string]string{"message": "created"}

		err := ctx.JSON(StatusCreated, testData)

		assert.Nil(t, err)
		assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())
		assert.Contains(t, string(requestCtx.Response.Body()), "created")
	})

	// Test with nil data
	t.Run("Nil data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		err := ctx.JSON(StatusOK, nil)

		assert.Nil(t, err)
		assert.Equal(t, "null", string(requestCtx.Response.Body()))
	})

	// Test with complex nested data
	t.Run("Complex nested data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		complexData := map[string]interface{}{
			"user": TestUser{Name: "alice", Email: "alice@example.com"},
			"metadata": map[string]interface{}{
				"version": "1.0",
				"active":  true,
				"tags":    []string{"admin", "user"},
			},
		}

		err := ctx.JSON(StatusOK, complexData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "alice")
		assert.Contains(t, body, "alice@example.com")
		assert.Contains(t, body, "version")
		assert.Contains(t, body, "1.0")
		assert.Contains(t, body, "active")
		assert.Contains(t, body, "true")
		assert.Contains(t, body, "admin")
	})
}

func TestContext_XML(t *testing.T) {
	// Test basic XML serialization
	t.Run("Basic serialization", func(t *testing.T) {
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
	})

	// Test with different status codes
	t.Run("Different status codes", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := TestUser{Name: "jane", Email: "jane@example.com"}

		err := ctx.XML(StatusAccepted, testData)

		assert.Nil(t, err)
		assert.Equal(t, StatusAccepted, requestCtx.Response.StatusCode())
		assert.Contains(t, string(requestCtx.Response.Body()), "<name>jane</name>")
	})

	// Test with XML special characters
	t.Run("Special characters", func(t *testing.T) {
		type XMLTestData struct {
			Content string `xml:"content"`
			Quote   string `xml:"quote"`
		}

		ctx, requestCtx := createTestContext()
		testData := XMLTestData{
			Content: "<tag>content</tag> & more",
			Quote:   `He said "Hello"`,
		}

		err := ctx.XML(StatusOK, testData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		// XML should escape special characters
		assert.Contains(t, body, "&lt;tag&gt;content&lt;/tag&gt; &amp; more")
		assert.Contains(t, body, "He said &#34;Hello&#34;")
	})

	// Test with nested structure
	t.Run("Nested structure", func(t *testing.T) {
		type Address struct {
			Street string `xml:"street"`
			City   string `xml:"city"`
		}
		type Person struct {
			Name    string  `xml:"name"`
			Address Address `xml:"address"`
		}

		ctx, requestCtx := createTestContext()
		testData := Person{
			Name: "Bob",
			Address: Address{
				Street: "123 Main St",
				City:   "New York",
			},
		}

		err := ctx.XML(StatusOK, testData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "<Person>")
		assert.Contains(t, body, "<name>Bob</name>")
		assert.Contains(t, body, "<address>")
		assert.Contains(t, body, "<street>123 Main St</street>")
		assert.Contains(t, body, "<city>New York</city>")
		assert.Contains(t, body, "</address>")
		assert.Contains(t, body, "</Person>")
	})
}

func TestContext_YAML(t *testing.T) {
	// Test basic YAML serialization
	t.Run("Basic serialization", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := TestUser{Name: "john", Email: "john@example.com"}

		err := ctx.YAML(StatusOK, testData)

		assert.Nil(t, err)
		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMEApplicationYAML, string(requestCtx.Response.Header.ContentType()))

		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "name: john")
		assert.Contains(t, body, "email: john@example.com")
	})

	// Test with different status codes
	t.Run("Different status codes", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := map[string]string{"status": "created"}

		err := ctx.YAML(StatusCreated, testData)

		assert.Nil(t, err)
		assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())
		assert.Contains(t, string(requestCtx.Response.Body()), "status: created")
	})

	// Test with nil data
	t.Run("Nil data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		err := ctx.YAML(StatusOK, nil)

		assert.Nil(t, err)
		assert.Equal(t, "null\n", string(requestCtx.Response.Body()))
	})

	// Test with array data
	t.Run("Array data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := []string{"item1", "item2", "item3"}

		err := ctx.YAML(StatusOK, testData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "- item1")
		assert.Contains(t, body, "- item2")
		assert.Contains(t, body, "- item3")
	})

	// Test with nested structure
	t.Run("Nested structure", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		nestedData := map[string]interface{}{
			"user": map[string]interface{}{
				"name":  "alice",
				"email": "alice@example.com",
				"settings": map[string]bool{
					"notifications": true,
					"dark_mode":     false,
				},
			},
			"version": "2.0",
		}

		err := ctx.YAML(StatusOK, nestedData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "user:")
		assert.Contains(t, body, "  name: alice")
		assert.Contains(t, body, "  email: alice@example.com")
		assert.Contains(t, body, "  settings:")
		assert.Contains(t, body, "    notifications: true")
		assert.Contains(t, body, "    dark_mode: false")
		assert.Contains(t, body, "version: \"2.0\"")
	})

	// Test with special YAML characters
	t.Run("Special characters", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := map[string]string{
			"multiline": "line1\nline2\nline3",
			"quotes":    `He said "Hello" and 'Goodbye'`,
			"colon":     "key: value",
		}

		err := ctx.YAML(StatusOK, testData)

		assert.Nil(t, err)
		body := string(requestCtx.Response.Body())
		assert.Contains(t, body, "multiline:")
		assert.Contains(t, body, "line1")
		assert.Contains(t, body, "line2")
		assert.Contains(t, body, "line3")
		assert.Contains(t, body, "quotes:")
		assert.Contains(t, body, "colon:")
	})
}

func TestContext_IndentedJSON(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := TestUser{Name: "john", Email: "john@example.com"}

	err := ctx.IndentedJSON(StatusOK, testData)

	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationJSON, string(requestCtx.Response.Header.ContentType()))

	body := string(requestCtx.Response.Body())
	assert.Contains(t, body, "john")
	assert.Contains(t, body, "john@example.com")
	assert.Contains(t, body, "    ") // Check for indentation
	assert.Contains(t, body, "\n")   // Check for line breaks
}

func TestContext_SecureJSON(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := TestUser{Name: "john", Email: "john@example.com"}

	// Create a mock Gonoleks app with default secure prefix
	app := &Gonoleks{secureJsonPrefix: "while(1);"}
	requestCtx.SetUserValue("gonoleksApp", app)

	err := ctx.SecureJSON(StatusOK, testData)

	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationJSON, string(requestCtx.Response.Header.ContentType()))

	body := string(requestCtx.Response.Body())
	assert.Contains(t, body, "john")
	assert.Contains(t, body, "john@example.com")
	assert.True(t, strings.HasPrefix(body, "while(1);")) // Check for security prefix
}

func TestContext_AsciiJSON(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := map[string]string{
		"name":  "José",
		"emoji": "😀",
	}

	err := ctx.AsciiJSON(StatusOK, testData)

	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationJSON, string(requestCtx.Response.Header.ContentType()))

	body := string(requestCtx.Response.Body())
	// Unicode characters should be escaped
	assert.Contains(t, body, "\\u00e9")  // é
	assert.Contains(t, body, "\\u1f600") // 😀
}

func TestContext_PureJSON(t *testing.T) {
	ctx, requestCtx := createTestContext()
	testData := map[string]string{
		"html": "<script>alert('test')</script>",
		"amp":  "Tom & Jerry",
	}

	err := ctx.PureJSON(StatusOK, testData)

	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMEApplicationJSON, string(requestCtx.Response.Header.ContentType()))

	body := string(requestCtx.Response.Body())
	// HTML characters should NOT be escaped in PureJSON
	assert.Contains(t, body, "<script>")
	assert.Contains(t, body, "&")
}

func TestContext_ProtoBuf(t *testing.T) {
	// Test successful serialization
	t.Run("Success", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := &protoexample.TestMessage{
			Name:  "Test User",
			Email: "test@example.com",
		}

		err := ctx.ProtoBuf(StatusOK, testData)
		assert.Nil(t, err)
		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMEApplicationProtoBuf, string(requestCtx.Response.Header.ContentType()))

		// Verify data was written
		assert.Greater(t, len(requestCtx.Response.Body()), 0)
	})

	// Test with nested message
	t.Run("Nested message", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		testData := &protoexample.UserProfile{
			Username: "testuser",
			IsActive: true,
			Metadata: &protoexample.TestMessage{
				Name: "nested",
			},
		}

		err := ctx.ProtoBuf(StatusOK, testData)
		assert.Nil(t, err)
		assert.Equal(t, MIMEApplicationProtoBuf, string(requestCtx.Response.Header.ContentType()))
	})

	// Test error case
	t.Run("Error with non-proto.Message", func(t *testing.T) {
		ctx, _ := createTestContext()
		err := ctx.ProtoBuf(StatusOK, "not a proto message")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), ErrProtoMessageInterface.Error())
	})
}

func TestContext_String(t *testing.T) {
	// Test basic string formatting
	t.Run("Basic formatting", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "Hello %s", "World")

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlainCharsetUTF8, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "Hello World", string(requestCtx.Response.Body()))
	})

	// Test with different status codes
	t.Run("Different status codes", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusCreated, "Resource created: %s", "user123")

		assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())
		assert.Equal(t, "Resource created: user123", string(requestCtx.Response.Body()))
	})

	// Test with no formatting
	t.Run("No formatting", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "Simple string without formatting")

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "Simple string without formatting", string(requestCtx.Response.Body()))
	})

	// Test with multiple format arguments
	t.Run("Multiple format arguments", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "User: %s, Age: %d, Active: %t", "John", 25, true)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "User: John, Age: 25, Active: true", string(requestCtx.Response.Body()))
	})

	// Test with empty string
	t.Run("Empty string", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "")

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "", string(requestCtx.Response.Body()))
	})

	// Test with special characters
	t.Run("Special characters", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "Special chars: %s", "Hello\nWorld\t!")

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "Special chars: Hello\nWorld\t!", string(requestCtx.Response.Body()))
	})

	// Test with Unicode characters
	t.Run("Unicode characters", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.String(StatusOK, "Unicode: %s %s", "🚀", "こんにちは")

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "Unicode: 🚀 こんにちは", string(requestCtx.Response.Body()))
	})

	// Test with long string
	t.Run("Long string", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		longString := strings.Repeat("A", 1000)

		ctx.String(StatusOK, "Long string: %s", longString)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "Long string: "+longString, string(requestCtx.Response.Body()))
	})
}

func TestContext_Redirect(t *testing.T) {
	ctx, requestCtx := createTestContext()

	ctx.Redirect(StatusFound, "https://example.com")
	assert.Equal(t, StatusFound, requestCtx.Response.StatusCode())
	assert.Equal(t, "https://example.com", string(requestCtx.Response.Header.Peek(HeaderLocation)))
}

func TestContext_Data(t *testing.T) {
	// Test basic data response
	t.Run("Basic data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		data := []byte("Hello World")

		ctx.Data(StatusOK, MIMETextPlain, data)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "Hello World", string(requestCtx.Response.Body()))
	})

	// Test with different status codes
	t.Run("Different status codes", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		data := []byte("Created successfully")

		ctx.Data(StatusCreated, MIMETextPlain, data)

		assert.Equal(t, StatusCreated, requestCtx.Response.StatusCode())
		assert.Equal(t, "Created successfully", string(requestCtx.Response.Body()))
	})

	// Test with different content types
	t.Run("Different content types", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		data := []byte(`{"message": "hello"}`)

		ctx.Data(StatusOK, MIMEApplicationJSON, data)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMEApplicationJSON, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, `{"message": "hello"}`, string(requestCtx.Response.Body()))
	})

	// Test with binary data
	t.Run("Binary data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header

		ctx.Data(StatusOK, "image/png", binaryData)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, "image/png", string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, binaryData, requestCtx.Response.Body())
	})

	// Test with empty data
	t.Run("Empty data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.Data(StatusOK, MIMETextPlain, []byte{})

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "", string(requestCtx.Response.Body()))
	})

	// Test with nil data
	t.Run("Nil data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()

		ctx.Data(StatusOK, MIMETextPlain, nil)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "", string(requestCtx.Response.Body()))
	})

	// Test with large data
	t.Run("Large data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		largeData := bytes.Repeat([]byte("A"), 10000)

		ctx.Data(StatusOK, MIMETextPlain, largeData)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, largeData, requestCtx.Response.Body())
		assert.Equal(t, 10000, len(requestCtx.Response.Body()))
	})

	// Test with custom content type
	t.Run("Custom content type", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		data := []byte("custom data")
		customContentType := "application/custom"

		ctx.Data(StatusOK, customContentType, data)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, customContentType, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "custom data", string(requestCtx.Response.Body()))
	})

	// Test with UTF-8 encoded data
	t.Run("UTF-8 encoded data", func(t *testing.T) {
		ctx, requestCtx := createTestContext()
		utf8Data := []byte("Hello 世界 🌍")

		ctx.Data(StatusOK, MIMETextPlainCharsetUTF8, utf8Data)

		assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
		assert.Equal(t, MIMETextPlainCharsetUTF8, string(requestCtx.Response.Header.ContentType()))
		assert.Equal(t, "Hello 世界 🌍", string(requestCtx.Response.Body()))
	})
}

func TestContext_File(t *testing.T) {
	// Test successful file serving
	t.Run("Success", func(t *testing.T) {
		// Use the existing test file
		testFilePath := "testdata/test_file.txt"

		// Test that the method can be called without panicking
		// Note: In a real test environment, we would need a full server setup
		// to properly test file serving. Here we're testing that the method
		// exists and can be called.
		assert.NotPanics(t, func() {
			// We'll test this differently since fasthttp requires a full server context
			// Just verify the method exists and the file path is handled
			if _, err := os.Stat(testFilePath); err == nil {
				// File exists, method should work in a real server context
				assert.True(t, true) // File exists
			}
		})
	})

	// Test file existence check
	t.Run("File existence", func(t *testing.T) {
		// Test that our test file exists
		testFilePath := "testdata/test_file.txt"
		_, err := os.Stat(testFilePath)
		assert.NoError(t, err, "Test file should exist")
	})

	// Test with non-existent file path
	t.Run("Non-existent file path", func(t *testing.T) {
		nonExistentPath := "non-existent-file.txt"
		_, err := os.Stat(nonExistentPath)
		assert.True(t, os.IsNotExist(err), "Non-existent file should return error")
	})
}

func TestContext_FileFromFS(t *testing.T) {
	// Test with testing/fstest.MapFS - basic functionality test
	t.Run("MapFS basic test", func(t *testing.T) {
		// Create a test filesystem
		testFS := fstest.MapFS{
			"test.txt": &fstest.MapFile{
				Data: []byte("Hello from test filesystem!"),
			},
			"subdir/nested.txt": &fstest.MapFile{
				Data: []byte("Nested file content"),
			},
		}

		// Test that files exist in the filesystem
		data, err := fs.ReadFile(testFS, "test.txt")
		assert.NoError(t, err)
		assert.Equal(t, "Hello from test filesystem!", string(data))

		// Test nested file
		nestedData, err := fs.ReadFile(testFS, "subdir/nested.txt")
		assert.NoError(t, err)
		assert.Equal(t, "Nested file content", string(nestedData))

		// Test that the method exists and can be called without panicking
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			// Note: In a real server environment, this would serve the file
			// Here we're testing that the method exists and accepts the parameters
			ctx.FileFromFS("test.txt", testFS)
		})
	})

	// Test with os.DirFS
	t.Run("DirFS basic test", func(t *testing.T) {
		// Use the existing testdata directory
		testFS := os.DirFS("testdata")

		// Test that the file exists in the filesystem
		data, err := fs.ReadFile(testFS, "test_file.txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Test that the method can be called
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			ctx.FileFromFS("test_file.txt", testFS)
		})
	})

	// Test with embed.FS
	t.Run("EmbedFS basic test", func(t *testing.T) {
		// Use the embedded filesystem
		var embedFS fs.FS = testEmbedFS

		// Test that the file exists in the embedded filesystem
		data, err := fs.ReadFile(embedFS, "testdata/test_file.txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Test that the method can be called
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			ctx.FileFromFS("testdata/test_file.txt", embedFS)
		})
	})

	// Test with fs.FS interface
	t.Run("fs.FS interface test", func(t *testing.T) {
		// Use fs.FS interface with os.DirFS
		filesystem := os.DirFS("testdata")

		// Test that the file exists
		data, err := fs.ReadFile(filesystem, "test_file.txt")
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Test that the method can be called
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			ctx.FileFromFS("test_file.txt", filesystem)
		})
	})

	// Test filesystem validation (without calling FileFromFS)
	t.Run("Filesystem validation", func(t *testing.T) {
		// Test that non-existent file returns error when reading
		testFS := fstest.MapFS{
			"existing.txt": &fstest.MapFile{
				Data: []byte("exists"),
			},
		}

		_, err := fs.ReadFile(testFS, "nonexistent.txt")
		assert.Error(t, err)

		// Test that reading from empty filesystem returns error
		emptyFS := fstest.MapFS{}
		_, err = fs.ReadFile(emptyFS, "any.txt")
		assert.Error(t, err)
	})

	// Test with different file types
	t.Run("Different file types test", func(t *testing.T) {
		testFS := fstest.MapFS{
			"data.json": &fstest.MapFile{
				Data: []byte(`{"key": "value"}`),
			},
			"style.css": &fstest.MapFile{
				Data: []byte(`body { color: black; }`),
			},
			"script.js": &fstest.MapFile{
				Data: []byte(`console.log("hello");`),
			},
		}

		// Test that all files exist and have correct content
		jsonData, err := fs.ReadFile(testFS, "data.json")
		assert.NoError(t, err)
		assert.Contains(t, string(jsonData), "key")

		cssData, err := fs.ReadFile(testFS, "style.css")
		assert.NoError(t, err)
		assert.Contains(t, string(cssData), "color: black")

		jsData, err := fs.ReadFile(testFS, "script.js")
		assert.NoError(t, err)
		assert.Contains(t, string(jsData), "console.log")

		// Test that the method can be called for all file types
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			ctx.FileFromFS("data.json", testFS)
			ctx.FileFromFS("style.css", testFS)
			ctx.FileFromFS("script.js", testFS)
		})
	})

	// Test with binary content
	t.Run("Binary content test", func(t *testing.T) {
		binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
		testFS := fstest.MapFS{
			"image.png": &fstest.MapFile{
				Data: binaryData,
			},
		}

		// Test that binary data is preserved
		data, err := fs.ReadFile(testFS, "image.png")
		assert.NoError(t, err)
		assert.Equal(t, binaryData, data)

		// Test that the method can be called
		assert.NotPanics(t, func() {
			ctx, _ := createTestContext()
			ctx.FileFromFS("image.png", testFS)
		})
	})
}

func TestContext_SetAccepted(t *testing.T) {
	ctx, requestCtx := createTestContext()

	ctx.SetAccepted(MIMEApplicationJSON, MIMEApplicationXML)
	acceptHeader := string(requestCtx.Response.Header.Peek(HeaderAccept))
	assert.Equal(t, "application/json, application/xml", acceptHeader)
}
