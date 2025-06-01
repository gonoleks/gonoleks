package gonoleks

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormDecoder(t *testing.T) {
	t.Run("NewFormDecoder", func(t *testing.T) {
		decoder := NewFormDecoder()
		assert.False(t, decoder.ignoreUnknownKeys)
		assert.Equal(t, "form", decoder.aliasTag)
	})

	t.Run("IgnoreUnknownKeys", func(t *testing.T) {
		decoder := NewFormDecoder()
		decoder.IgnoreUnknownKeys(true)
		assert.True(t, decoder.ignoreUnknownKeys)
	})

	t.Run("SetAliasTag", func(t *testing.T) {
		decoder := NewFormDecoder()
		decoder.SetAliasTag("json")
		assert.Equal(t, "json", decoder.aliasTag)
	})
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single value",
			input:    "name",
			expected: []string{"name"},
		},
		{
			name:     "multiple values",
			input:    "name,email,phone",
			expected: []string{"name", "email", "phone"},
		},
		{
			name:     "values with spaces",
			input:    "name, email , phone",
			expected: []string{"name", "email", "phone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDecode(t *testing.T) {
	type User struct {
		Name     string   `form:"name"`
		Email    string   `form:"email"`
		Age      int      `form:"age"`
		Active   bool     `form:"active"`
		Score    float64  `form:"score"`
		Tags     []string `form:"tags"`
		Ignored  string   `form:"-"`
		Internal string   // No tag
	}

	type Address struct {
		Street string `form:"street"`
		City   string `form:"city"`
	}

	type UserWithAddress struct {
		User
		Address Address
		Zip     string `form:"zip"`
	}

	t.Run("basic struct decoding", func(t *testing.T) {
		values := url.Values{
			"name":     []string{"John Doe"},
			"email":    []string{"john@example.com"},
			"age":      []string{"30"},
			"active":   []string{"true"},
			"score":    []string{"95.5"},
			"tags":     []string{"tag1", "tag2", "tag3"},
			"Ignored":  []string{"should not be set"},
			"Internal": []string{"should be set"},
		}

		var user User
		err := formDecoder.Decode(&user, values)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", user.Name)
		assert.Equal(t, "john@example.com", user.Email)
		assert.Equal(t, 30, user.Age)
		assert.True(t, user.Active)
		assert.Equal(t, 95.5, user.Score)
		assert.Equal(t, []string{"tag1", "tag2", "tag3"}, user.Tags)
		assert.Empty(t, user.Ignored)
		assert.Equal(t, "should be set", user.Internal)
	})

	t.Run("embedded struct", func(t *testing.T) {
		values := url.Values{
			"name":   []string{"John Doe"},
			"email":  []string{"john@example.com"},
			"street": []string{"123 Main St"},
			"city":   []string{"New York"},
			"zip":    []string{"10001"},
		}

		var userWithAddress UserWithAddress
		err := formDecoder.Decode(&userWithAddress, values)
		require.NoError(t, err)

		assert.Equal(t, "John Doe", userWithAddress.Name)
		assert.Equal(t, "john@example.com", userWithAddress.Email)
		assert.Equal(t, "123 Main St", userWithAddress.Address.Street)
		assert.Equal(t, "New York", userWithAddress.Address.City)
		assert.Equal(t, "10001", userWithAddress.Zip)
	})

	t.Run("empty values", func(t *testing.T) {
		values := url.Values{
			"name":   []string{""},
			"age":    []string{""},
			"active": []string{""},
			"score":  []string{""},
		}

		var user User
		err := formDecoder.Decode(&user, values)
		require.NoError(t, err)

		assert.Equal(t, "", user.Name)
		assert.Equal(t, 0, user.Age)
		assert.False(t, user.Active)
		assert.Equal(t, 0.0, user.Score)
	})

	t.Run("invalid values", func(t *testing.T) {
		values := url.Values{
			"age":   []string{"not-a-number"},
			"score": []string{"invalid-float"},
		}

		var user User
		err := formDecoder.Decode(&user, values)
		assert.Error(t, err)
	})

	t.Run("non-pointer destination", func(t *testing.T) {
		values := url.Values{}
		var user User
		err := formDecoder.Decode(user, values)
		assert.Equal(t, ErrInvalidRequestEmptyForm, err)
	})

	t.Run("nil pointer destination", func(t *testing.T) {
		values := url.Values{}
		var user *User
		err := formDecoder.Decode(user, values)
		assert.Equal(t, ErrInvalidRequestEmptyForm, err)
	})

	t.Run("non-struct destination", func(t *testing.T) {
		values := url.Values{}
		var num int
		err := formDecoder.Decode(&num, values)
		assert.Equal(t, ErrInvalidRequestEmptyForm, err)
	})
}

func TestSetFieldValue(t *testing.T) {
	t.Run("slice field with unsupported type", func(t *testing.T) {
		type ComplexSlice struct {
			Items []struct {
				Name string
			} `form:"items"`
		}

		values := url.Values{
			"items": []string{"item1", "item2"},
		}

		var complex ComplexSlice
		err := formDecoder.Decode(&complex, values)
		assert.Error(t, err)
		assert.Equal(t, ErrUnsupportedSliceElementType, err)
	})
}

func TestGetCachedFields(t *testing.T) {
	type TestStruct struct {
		Name     string `form:"name"`
		Email    string `form:"email,alternate"`
		Ignored  string `form:"-"`
		Internal string
	}

	decoder := NewFormDecoder()
	fields := decoder.getCachedFields(reflect.TypeOf(TestStruct{}))

	assert.Len(t, fields, 3) // Ignored field should be skipped

	// Check the first field (Name)
	assert.Equal(t, 0, fields[0].index)
	assert.Equal(t, []string{"name"}, fields[0].names)
	assert.False(t, fields[0].anonymous)
	assert.True(t, fields[0].canSet)

	// Check the second field (Email with multiple names)
	assert.Equal(t, 1, fields[1].index)
	assert.Equal(t, []string{"email", "alternate"}, fields[1].names)
	assert.False(t, fields[1].anonymous)
	assert.True(t, fields[1].canSet)

	// Check the third field (Internal with no tag)
	assert.Equal(t, 3, fields[2].index)
	assert.Equal(t, []string{"Internal"}, fields[2].names)
	assert.False(t, fields[2].anonymous)
	assert.True(t, fields[2].canSet)

	// Test caching
	cachedFields := decoder.getCachedFields(reflect.TypeOf(TestStruct{}))
	assert.Equal(t, fields, cachedFields)
	//assert.Same(t, fields, cachedFields) // Should return the same slice from cache
}
