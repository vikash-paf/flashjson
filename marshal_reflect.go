package flashjson

import (
	"bytes"
	"reflect"
	"strings"
)

// marshalReflect marshals a value using reflection.
// Used for structs and other complex types.
func marshalReflect(buf *bytes.Buffer, v interface{}) error {
	rv := reflect.ValueOf(v)
	return marshalReflectValue(buf, rv)
}

func marshalReflectValue(buf *bytes.Buffer, rv reflect.Value) error {
	// Handle pointers
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return marshalTo(buf, rv.Int())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return marshalTo(buf, rv.Uint())

	case reflect.Float32:
		return marshalTo(buf, float32(rv.Float()))

	case reflect.Float64:
		return marshalTo(buf, rv.Float())

	case reflect.String:
		marshalString(buf, rv.String())

	case reflect.Slice:
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			// []byte - encode as base64 or string
			// For now, just encode as array of numbers
			buf.WriteByte('[')
			for i := 0; i < rv.Len(); i++ {
				if i > 0 {
					buf.WriteByte(',')
				}
				if err := marshalTo(buf, rv.Index(i).Interface()); err != nil {
					return err
				}
			}
			buf.WriteByte(']')
			return nil
		}
		fallthrough

	case reflect.Array:
		buf.WriteByte('[')
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := marshalReflectValue(buf, rv.Index(i)); err != nil {
				return err
			}
		}
		buf.WriteByte(']')

	case reflect.Map:
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		buf.WriteByte('{')
		first := true
		iter := rv.MapRange()
		for iter.Next() {
			if !first {
				buf.WriteByte(',')
			}
			first = false

			// Key must be string
			key := iter.Key()
			if key.Kind() != reflect.String {
				continue
			}
			marshalString(buf, key.String())
			buf.WriteByte(':')

			if err := marshalReflectValue(buf, iter.Value()); err != nil {
				return err
			}
		}
		buf.WriteByte('}')

	case reflect.Struct:
		buf.WriteByte('{')
		first := true
		rt := rv.Type()

		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Parse json tag
			tag := field.Tag.Get("json")
			if tag == "-" {
				continue
			}

			name, opts := parseJSONTag(tag)
			if name == "" {
				name = field.Name
			}

			fv := rv.Field(i)

			// Handle omitempty
			if strings.Contains(opts, "omitempty") && isEmptyValue(fv) {
				continue
			}

			if !first {
				buf.WriteByte(',')
			}
			first = false

			marshalString(buf, name)
			buf.WriteByte(':')

			if err := marshalReflectValue(buf, fv); err != nil {
				return err
			}
		}
		buf.WriteByte('}')

	case reflect.Interface:
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return marshalReflectValue(buf, rv.Elem())

	default:
		buf.WriteString("null")
	}

	return nil
}

func parseJSONTag(tag string) (string, string) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	}
	return false
}
