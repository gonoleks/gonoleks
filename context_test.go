package gonoleks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	"github.com/gonoleks/gonoleks/testdata/protoexample"
)

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

type TestUser struct {
	Name  string `json:"name" xml:"name" form:"name" yaml:"name"`
	Email string `json:"email" xml:"email" form:"email" yaml:"email"`
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

func TestContext_JSON_XML_YAML(t *testing.T) {
	testData := TestUser{Name: "john", Email: "john@example.com"}

	// Test JSON
	ctx, requestCtx := createTestContext()
	err := ctx.JSON(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "john")
	assert.Contains(t, string(requestCtx.Response.Body()), "john@example.com")

	// Test XML
	ctx, requestCtx = createTestContext()
	err = ctx.XML(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "<TestUser>")
	assert.Contains(t, string(requestCtx.Response.Body()), "<name>john</name>")

	// Test YAML
	ctx, requestCtx = createTestContext()
	err = ctx.YAML(StatusOK, testData)
	assert.Nil(t, err)
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Contains(t, string(requestCtx.Response.Body()), "name: john")
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

func TestContext_String_Data(t *testing.T) {
	// Test String
	ctx, requestCtx := createTestContext()
	ctx.String(StatusOK, "Hello %s", "World")
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, "Hello World", string(requestCtx.Response.Body()))

	// Test Data
	ctx, requestCtx = createTestContext()
	ctx.Data(StatusOK, MIMETextPlain, []byte("Hello World"))
	assert.Equal(t, StatusOK, requestCtx.Response.StatusCode())
	assert.Equal(t, MIMETextPlain, string(requestCtx.Response.Header.ContentType()))
	assert.Equal(t, "Hello World", string(requestCtx.Response.Body()))
}

func TestContext_Redirect(t *testing.T) {
	ctx, requestCtx := createTestContext()

	ctx.Redirect(StatusFound, "https://example.com")
	assert.Equal(t, StatusFound, requestCtx.Response.StatusCode())
	assert.Equal(t, "https://example.com", string(requestCtx.Response.Header.Peek(HeaderLocation)))
}

func TestContext_SetAccepted(t *testing.T) {
	ctx, requestCtx := createTestContext()

	ctx.SetAccepted(MIMEApplicationJSON, MIMEApplicationXML)
	acceptHeader := string(requestCtx.Response.Header.Peek(HeaderAccept))
	assert.Equal(t, "application/json, application/xml", acceptHeader)
}
