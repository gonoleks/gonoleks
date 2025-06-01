package gonoleks

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

type testStruct struct {
	Foo string `json:"foo" xml:"foo" yaml:"foo" toml:"foo" form:"foo" query:"foo" header:"foo" uri:"foo"`
	Bar int    `json:"bar" xml:"bar" yaml:"bar" toml:"bar" form:"bar" query:"bar" header:"bar" uri:"bar"`
}

func createRequestCtx(body []byte, contentType string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody(body)
	ctx.Request.Header.SetContentType(contentType)
	return ctx
}

func createQueryRequestCtx(queryParams map[string]string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	for k, v := range queryParams {
		ctx.QueryArgs().Add(k, v)
	}
	return ctx
}

func createFormRequestCtx(formParams map[string]string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetContentType(MIMEApplicationForm)
	for k, v := range formParams {
		ctx.PostArgs().Add(k, v)
	}
	return ctx
}

func createMultipartFormRequestCtx(formParams map[string]string) (*fasthttp.RequestCtx, error) {
	ctx := &fasthttp.RequestCtx{}
	
	// Create multipart writer
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Add form fields
	for k, v := range formParams {
		err := writer.WriteField(k, v)
		if err != nil {
			return nil, err
		}
	}
	
	// Close writer
	err := writer.Close()
	if err != nil {
		return nil, err
	}
	
	// Set request body and content type
	ctx.Request.SetBody(body.Bytes())
	ctx.Request.Header.SetContentType(writer.FormDataContentType())
	
	return ctx, nil
}

func createHeaderRequestCtx(headers map[string]string) *fasthttp.RequestCtx {
	ctx := &fasthttp.RequestCtx{}
	for k, v := range headers {
		ctx.Request.Header.Set(k, v)
	}
	return ctx
}

func TestJSONBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "json", JSON.Name())
	
	// Test Bind method with valid JSON
	jsonData := []byte(`{"foo":"test","bar":123}`)
	ctx := createRequestCtx(jsonData, MIMEApplicationJSON)
	
	var obj testStruct
	err := JSON.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty body
	ctx = createRequestCtx([]byte{}, MIMEApplicationJSON)
	err = JSON.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyBody, err)
	
	// Test BindBody method
	var obj2 testStruct
	err = JSON.BindBody(jsonData, &obj2)
	require.NoError(t, err)
	assert.Equal(t, "test", obj2.Foo)
	assert.Equal(t, 123, obj2.Bar)
}

func TestXMLBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "xml", XML.Name())
	
	// Test Bind method with valid XML
	xmlData := []byte(`<testStruct><foo>test</foo><bar>123</bar></testStruct>`)
	ctx := createRequestCtx(xmlData, MIMEApplicationXML)
	
	var obj testStruct
	err := XML.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty body
	ctx = createRequestCtx([]byte{}, MIMEApplicationXML)
	err = XML.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyBody, err)
	
	// Test BindBody method
	var obj2 testStruct
	err = XML.BindBody(xmlData, &obj2)
	require.NoError(t, err)
	assert.Equal(t, "test", obj2.Foo)
	assert.Equal(t, 123, obj2.Bar)
}

func TestYAMLBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "yaml", YAML.Name())
	
	// Test Bind method with valid YAML
	yamlData := []byte("foo: test\nbar: 123")
	ctx := createRequestCtx(yamlData, MIMEApplicationYAML)
	
	var obj testStruct
	err := YAML.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty body
	ctx = createRequestCtx([]byte{}, MIMEApplicationYAML)
	err = YAML.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyBody, err)
	
	// Test BindBody method
	var obj2 testStruct
	err = YAML.BindBody(yamlData, &obj2)
	require.NoError(t, err)
	assert.Equal(t, "test", obj2.Foo)
	assert.Equal(t, 123, obj2.Bar)
}

func TestTOMLBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "toml", TOML.Name())
	
	// Test Bind method with valid TOML
	tomlData := []byte("foo = \"test\"\nbar = 123")
	ctx := createRequestCtx(tomlData, MIMEApplicationTOML)
	
	var obj testStruct
	err := TOML.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty body
	ctx = createRequestCtx([]byte{}, MIMEApplicationTOML)
	err = TOML.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyBody, err)
	
	// Test BindBody method
	var obj2 testStruct
	err = TOML.BindBody(tomlData, &obj2)
	require.NoError(t, err)
	assert.Equal(t, "test", obj2.Foo)
	assert.Equal(t, 123, obj2.Bar)
}

func TestFormBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "form", Form.Name())
	
	// Test Bind method with valid form data
	formParams := map[string]string{
		"foo": "test",
		"bar": "123",
	}
	ctx := createFormRequestCtx(formParams)
	
	var obj testStruct
	err := Form.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty form
	ctx = createRequestCtx([]byte{}, MIMEApplicationForm)
	err = Form.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyForm, err)
	
	// Test Bind method with multipart form
	ctx, err = createMultipartFormRequestCtx(formParams)
	require.NoError(t, err)
	
	var obj2 testStruct
	err = Form.Bind(ctx, &obj2)
	require.NoError(t, err)
	assert.Equal(t, "test", obj2.Foo)
	assert.Equal(t, 123, obj2.Bar)
}

func TestQueryBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "query", Query.Name())
	
	// Test Bind method with valid query parameters
	queryParams := map[string]string{
		"foo": "test",
		"bar": "123",
	}
	ctx := createQueryRequestCtx(queryParams)
	
	var obj testStruct
	err := Query.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test Bind method with empty query
	ctx = &fasthttp.RequestCtx{}
	err = Query.Bind(ctx, &obj)
	assert.Equal(t, ErrInvalidRequestEmptyQuery, err)
}

func TestHeaderBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "header", Header.Name())
	
	// Test Bind method with valid headers
	headers := map[string]string{
		"Foo": "test",
		"Bar": "123",
	}
	ctx := createHeaderRequestCtx(headers)
	
	var obj testStruct
	err := Header.Bind(ctx, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
}

func TestUriBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "uri", Uri.Name())
	
	// Test BindUri method with valid parameters
	params := map[string]string{
		"foo": "test",
		"bar": "123",
	}
	
	var obj testStruct
	err := Uri.BindUri(params, &obj)
	require.NoError(t, err)
	assert.Equal(t, "test", obj.Foo)
	assert.Equal(t, 123, obj.Bar)
	
	// Test BindUri method with empty parameters
	err = Uri.BindUri(nil, &obj)
	assert.Equal(t, ErrInvalidUriParams, err)
	
	err = Uri.BindUri(map[string]string{}, &obj)
	assert.Equal(t, ErrInvalidUriParams, err)
}

func TestPlainBinding(t *testing.T) {
	// Test Name method
	assert.Equal(t, "plain", Plain.Name())
	
	// Test Bind method with valid plain text
	plainData := []byte("test plain text")
	ctx := createRequestCtx(plainData, MIMETextPlain)
	
	var plainStr string
	err := Plain.Bind(ctx, &plainStr)
	require.NoError(t, err)
	assert.Equal(t, "test plain text", plainStr)
	
	// Test Bind method with empty body
	ctx = createRequestCtx([]byte{}, MIMETextPlain)
	err = Plain.Bind(ctx, &plainStr)
	assert.Equal(t, ErrInvalidRequestEmptyBody, err)
	
	// Test BindBody method
	var plainStr2 string
	err = Plain.BindBody(plainData, &plainStr2)
	require.NoError(t, err)
	assert.Equal(t, "test plain text", plainStr2)
	
	// Test BindBody method with non-string pointer
	var obj testStruct
	err = Plain.BindBody(plainData, &obj)
	assert.Equal(t, ErrPlainBindPointer, err)
}

func TestDefault(t *testing.T) {
	// Test GET method
	binding := Default(MethodGet, MIMEApplicationJSON)
	assert.Equal(t, Query, binding)
	
	// Test different content types
	testCases := []struct {
		method      string
		contentType string
		expected    Binding
	}{
		{MethodPost, MIMEApplicationJSON, JSON},
		{MethodPost, MIMEApplicationXML, XML},
		{MethodPost, MIMETextXML, XML},
		{MethodPost, MIMEApplicationYAML, YAML},
		{MethodPost, MIMEApplicationTOML, TOML},
		{MethodPost, MIMEApplicationForm, Form},
		{MethodPost, MIMEMultipartForm, Form},
		{MethodPost, MIMETextPlain, Plain},
		{MethodPost, "unknown/type", JSON}, // Default to JSON
	}
	
	for _, tc := range testCases {
		binding := Default(tc.method, tc.contentType)
		assert.Equal(t, tc.expected, binding, "Content type: %s", tc.contentType)
	}
}
