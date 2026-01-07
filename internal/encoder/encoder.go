// Package encoder provides a fast, allocation-free JSON encoder.
// It uses pre-computed struct layouts and direct buffer writes.
package encoder

import (
	"reflect"
	"strconv"
	"sync"
	"unsafe"
)

// Buffer is a resizable byte buffer for encoding.
// Uses exponential growth to minimize allocations.
type Buffer struct {
	buf []byte
}

// NewBuffer creates a new buffer with initial capacity.
func NewBuffer(cap int) *Buffer {
	return &Buffer{buf: make([]byte, 0, cap)}
}

// Reset clears the buffer for reuse.
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
}

// Bytes returns the buffer contents.
func (b *Buffer) Bytes() []byte {
	return b.buf
}

// Len returns the current length.
func (b *Buffer) Len() int {
	return len(b.buf)
}

// WriteByte writes a single byte.
func (b *Buffer) WriteByte(c byte) {
	b.buf = append(b.buf, c)
}

// WriteBytes writes multiple bytes.
func (b *Buffer) WriteBytes(p []byte) {
	b.buf = append(b.buf, p...)
}

// WriteString writes a string.
func (b *Buffer) WriteString(s string) {
	b.buf = append(b.buf, s...)
}

// WriteQuotedString writes a JSON-escaped string with quotes.
func (b *Buffer) WriteQuotedString(s string) {
	b.buf = append(b.buf, '"')

	// Fast path: check if escaping needed
	needsEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == '"' || s[i] == '\\' {
			needsEscape = true
			break
		}
	}

	if !needsEscape {
		b.buf = append(b.buf, s...)
	} else {
		b.writeEscapedString(s)
	}

	b.buf = append(b.buf, '"')
}

func (b *Buffer) writeEscapedString(s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			b.buf = append(b.buf, '\\', '"')
		case '\\':
			b.buf = append(b.buf, '\\', '\\')
		case '\n':
			b.buf = append(b.buf, '\\', 'n')
		case '\r':
			b.buf = append(b.buf, '\\', 'r')
		case '\t':
			b.buf = append(b.buf, '\\', 't')
		default:
			if c < 0x20 {
				b.buf = append(b.buf, '\\', 'u', '0', '0')
				b.buf = append(b.buf, hexDigit(c>>4), hexDigit(c&0xf))
			} else {
				b.buf = append(b.buf, c)
			}
		}
	}
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

// WriteInt writes an integer.
func (b *Buffer) WriteInt(v int64) {
	b.buf = strconv.AppendInt(b.buf, v, 10)
}

// WriteUint writes an unsigned integer.
func (b *Buffer) WriteUint(v uint64) {
	b.buf = strconv.AppendUint(b.buf, v, 10)
}

// WriteFloat writes a float.
func (b *Buffer) WriteFloat(v float64, bits int) {
	b.buf = strconv.AppendFloat(b.buf, v, 'g', -1, bits)
}

// WriteBool writes a boolean.
func (b *Buffer) WriteBool(v bool) {
	if v {
		b.buf = append(b.buf, "true"...)
	} else {
		b.buf = append(b.buf, "false"...)
	}
}

// WriteNull writes null.
func (b *Buffer) WriteNull() {
	b.buf = append(b.buf, "null"...)
}

// --- Buffer Pool ---

var bufferPool = sync.Pool{
	New: func() interface{} {
		return NewBuffer(512)
	},
}

// GetBuffer gets a buffer from the pool.
func GetBuffer() *Buffer {
	return bufferPool.Get().(*Buffer)
}

// PutBuffer returns a buffer to the pool.
func PutBuffer(b *Buffer) {
	b.Reset()
	bufferPool.Put(b)
}

// --- Struct Encoder ---

// StructEncoder is a pre-compiled encoder for a struct type.
type StructEncoder struct {
	fields []fieldEncoder
}

type fieldEncoder struct {
	name   string // JSON key name
	offset uintptr
	encode func(b *Buffer, ptr unsafe.Pointer)
}

// Compile creates a StructEncoder for the given type.
func Compile(t reflect.Type) *StructEncoder {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	enc := &StructEncoder{
		fields: make([]fieldEncoder, 0, t.NumField()),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}

		name := f.Name
		if tag != "" {
			if idx := indexOf(tag, ','); idx != -1 {
				name = tag[:idx]
			} else {
				name = tag
			}
		}

		fe := fieldEncoder{
			name:   name,
			offset: f.Offset,
			encode: makeFieldEncoder(f.Type),
		}
		enc.fields = append(enc.fields, fe)
	}

	return enc
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func makeFieldEncoder(t reflect.Type) func(b *Buffer, ptr unsafe.Pointer) {
	switch t.Kind() {
	case reflect.String:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteQuotedString(*(*string)(ptr))
		}
	case reflect.Int:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteInt(int64(*(*int)(ptr)))
		}
	case reflect.Int8:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteInt(int64(*(*int8)(ptr)))
		}
	case reflect.Int16:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteInt(int64(*(*int16)(ptr)))
		}
	case reflect.Int32:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteInt(int64(*(*int32)(ptr)))
		}
	case reflect.Int64:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteInt(*(*int64)(ptr))
		}
	case reflect.Uint:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteUint(uint64(*(*uint)(ptr)))
		}
	case reflect.Uint8:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteUint(uint64(*(*uint8)(ptr)))
		}
	case reflect.Uint16:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteUint(uint64(*(*uint16)(ptr)))
		}
	case reflect.Uint32:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteUint(uint64(*(*uint32)(ptr)))
		}
	case reflect.Uint64:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteUint(*(*uint64)(ptr))
		}
	case reflect.Float32:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteFloat(float64(*(*float32)(ptr)), 32)
		}
	case reflect.Float64:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteFloat(*(*float64)(ptr), 64)
		}
	case reflect.Bool:
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteBool(*(*bool)(ptr))
		}
	default:
		// Fallback for complex types
		return func(b *Buffer, ptr unsafe.Pointer) {
			b.WriteNull()
		}
	}
}

// Encode writes the struct to the buffer.
func (enc *StructEncoder) Encode(b *Buffer, ptr unsafe.Pointer) {
	b.WriteByte('{')
	for i, f := range enc.fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteQuotedString(f.name)
		b.WriteByte(':')
		fieldPtr := unsafe.Pointer(uintptr(ptr) + f.offset)
		f.encode(b, fieldPtr)
	}
	b.WriteByte('}')
}

// --- Encoder Cache ---

var encoderCache sync.Map

// GetEncoder gets or creates a cached encoder for the type.
func GetEncoder(t reflect.Type) *StructEncoder {
	if enc, ok := encoderCache.Load(t); ok {
		return enc.(*StructEncoder)
	}
	enc := Compile(t)
	encoderCache.Store(t, enc)
	return enc
}
