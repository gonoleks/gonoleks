package gonoleks

import (
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// formDecoderType defines a form decoder that can decode url.Values into a struct
type formDecoderType struct {
	ignoreUnknownKeys bool
	aliasTag          string
	fieldCache        sync.Map // map[reflect.Type][]cachedField
}

type cachedField struct {
	index     int
	names     []string
	anonymous bool
	canSet    bool
}

var formDecoder = NewFormDecoder()

func init() {
	formDecoder.IgnoreUnknownKeys(true)
	formDecoder.SetAliasTag("form")
}

// NewFormDecoder creates a new form decoder
func NewFormDecoder() *formDecoderType {
	return &formDecoderType{
		ignoreUnknownKeys: false,
		aliasTag:          "form",
	}
}

// IgnoreUnknownKeys sets whether the decoder should ignore unknown keys
func (d *formDecoderType) IgnoreUnknownKeys(ignore bool) {
	d.ignoreUnknownKeys = ignore
}

// SetAliasTag sets the tag name used for form field names
func (d *formDecoderType) SetAliasTag(tag string) {
	d.aliasTag = tag
}

// getCachedFields returns cached field info for a struct type
func (d *formDecoderType) getCachedFields(t reflect.Type) []cachedField {
	if v, ok := d.fieldCache.Load(t); ok {
		return v.([]cachedField)
	}

	fields := make([]cachedField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		canSet := field.PkgPath == "" // exported
		name := field.Tag.Get(d.aliasTag)
		if name == "" {
			name = field.Name
		} else if name == "-" {
			continue
		}
		names := splitAndTrim(name)
		fields = append(fields, cachedField{
			index:     i,
			names:     names,
			anonymous: field.Anonymous && field.Type.Kind() == reflect.Struct,
			canSet:    canSet,
		})
	}

	// Store the fields in the cache before returning
	d.fieldCache.Store(t, fields)
	return fields
}

func splitAndTrim(s string) []string {
	if strings.IndexByte(s, ',') == -1 {
		return []string{s}
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// Decode decodes url.Values into a struct
func (d *formDecoderType) Decode(dst any, src url.Values) error {
	dstVal := reflect.ValueOf(dst)
	if dstVal.Kind() != reflect.Ptr || dstVal.IsNil() {
		return ErrInvalidRequestEmptyForm
	}

	dstVal = dstVal.Elem()
	if dstVal.Kind() != reflect.Struct {
		return ErrInvalidRequestEmptyForm
	}

	dstType := dstVal.Type()
	fields := d.getCachedFields(dstType)
	for _, f := range fields {
		fieldVal := dstVal.Field(f.index)
		if !f.canSet {
			continue
		}
		// Handle embedded structs
		if f.anonymous {
			if err := d.Decode(fieldVal.Addr().Interface(), src); err != nil {
				return err
			}
			continue
		}

		// Handle non-anonymous struct fields
		if fieldVal.Kind() == reflect.Struct {
			if err := d.Decode(fieldVal.Addr().Interface(), src); err != nil {
				return err
			}
			continue
		}

		for _, n := range f.names {
			if values, ok := src[n]; ok && len(values) > 0 {
				if err := setFieldValue(fieldVal, values); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// setFieldValue sets the field value based on the form values
func setFieldValue(fieldVal reflect.Value, values []string) error {
	switch fieldVal.Kind() {
	case reflect.String:
		fieldVal.SetString(values[0])
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setIntField(fieldVal, values[0])
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUintField(fieldVal, values[0])
	case reflect.Bool:
		return setBoolField(fieldVal, values[0])
	case reflect.Float32, reflect.Float64:
		return setFloatField(fieldVal, values[0])
	case reflect.Slice:
		return setSliceField(fieldVal, values)
	}
	return nil
}

// setIntField sets an int field's value from a string
func setIntField(field reflect.Value, value string) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	field.SetInt(intVal)
	return nil
}

// setUintField sets a uint field's value from a string
func setUintField(field reflect.Value, value string) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	field.SetUint(uintVal)
	return nil
}

// setBoolField sets a bool field's value from a string
func setBoolField(field reflect.Value, value string) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	field.SetBool(boolVal)
	return nil
}

// setFloatField sets a float field's value from a string
func setFloatField(field reflect.Value, value string) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	field.SetFloat(floatVal)
	return nil
}

// setSliceField sets a slice field's value from a string slice
func setSliceField(field reflect.Value, values []string) error {
	sliceType := field.Type().Elem()
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))

	for i, value := range values {
		elemValue := reflect.New(sliceType).Elem()
		switch sliceType.Kind() {
		case reflect.String:
			elemValue.SetString(value)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			elemValue.SetInt(intVal)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			uintVal, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return err
			}
			elemValue.SetUint(uintVal)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			elemValue.SetBool(boolVal)
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return err
			}
			elemValue.SetFloat(floatVal)
		default:
			return ErrUnsupportedSliceElementType
		}
		slice.Index(i).Set(elemValue)
	}

	field.Set(slice)
	return nil
}
