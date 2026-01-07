package flashjson

import (
	"bytes"
	"strconv"
)

// marshalValue marshals a Go value to JSON.
// This is a simple implementation - the optimized version would use SIMD.
func marshalValue(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := marshalTo(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func marshalTo(buf *bytes.Buffer, v interface{}) error {
	if v == nil {
		buf.WriteString("null")
		return nil
	}

	switch val := v.(type) {
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}

	case string:
		marshalString(buf, val)

	case int:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int8:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int16:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int32:
		buf.WriteString(strconv.FormatInt(int64(val), 10))
	case int64:
		buf.WriteString(strconv.FormatInt(val, 10))

	case uint:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint8:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint16:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint32:
		buf.WriteString(strconv.FormatUint(uint64(val), 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(val, 10))

	case float32:
		buf.WriteString(strconv.FormatFloat(float64(val), 'g', -1, 32))
	case float64:
		buf.WriteString(strconv.FormatFloat(val, 'g', -1, 64))

	case []interface{}:
		buf.WriteByte('[')
		for i, elem := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := marshalTo(buf, elem); err != nil {
				return err
			}
		}
		buf.WriteByte(']')

	case map[string]interface{}:
		buf.WriteByte('{')
		first := true
		for k, v := range val {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			marshalString(buf, k)
			buf.WriteByte(':')
			if err := marshalTo(buf, v); err != nil {
				return err
			}
		}
		buf.WriteByte('}')

	default:
		// For structs and other types, use reflection
		if err := marshalReflect(buf, v); err != nil {
			return err
		}
	}

	return nil
}

func marshalString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if c < 0x20 {
				buf.WriteString(`\u00`)
				buf.WriteByte(hexDigit(c >> 4))
				buf.WriteByte(hexDigit(c & 0xF))
			} else {
				buf.WriteByte(c)
			}
		}
	}
	buf.WriteByte('"')
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}
