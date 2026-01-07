// Package decoder provides a fast, allocation-minimal JSON decoder.
// It uses pre-computed field maps and avoids the VM overhead.
package decoder

import (
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	"github.com/vikash-paf/flashjson/internal/hash"
	"github.com/vikash-paf/flashjson/internal/numbers"
	"github.com/vikash-paf/flashjson/internal/tape"
	"github.com/vikash-paf/flashjson/internal/unsafeutil"
)

// StructDecoder is a pre-compiled decoder for a struct type.
type StructDecoder struct {
	fields   *hash.FieldMap
	decoders []fieldDecoder
}

type fieldDecoder struct {
	offset uintptr
	decode DecoderFunc
}

type DecoderFunc func(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error)

// Compile creates a StructDecoder for the given type.
func Compile(t reflect.Type) *StructDecoder {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	numFields := t.NumField()
	dec := &StructDecoder{
		fields:   hash.NewFieldMap(numFields),
		decoders: make([]fieldDecoder, numFields),
	}

	for i := 0; i < numFields; i++ {
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

		dec.fields.Put(name, f.Offset, i)
		dec.decoders[i] = fieldDecoder{
			offset: f.Offset,
			decode: makeDecoderFunc(f.Type),
		}
	}

	return dec
}

// Decode executes the decoding process for this struct.
func (dec *StructDecoder) Decode(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	// Root expected to be Object
	entry := t.Get(idx)
	if entry.Type != tape.TypeObjectStart {
		return idx, &DecodeError{"expected object for struct"}
	}
	idx++ // Move past Object start

	for {
		entry = t.Get(idx)
		if entry.Type == tape.TypeObjectEnd {
			return idx + 1, nil
		}

		// Entry is key
		if entry.Type != tape.TypeString && entry.Type != tape.TypeKey {
			return idx, &DecodeError{"expected string key"}
		}

		keyBytes := input[entry.Offset : entry.Offset+entry.Length]
		idx++ // Move to value

		// Lookup field
		if offset, fieldIdx, found := dec.fields.GetBytes(keyBytes); found {
			// Decode value into field
			var err error
			idx, err = dec.decoders[fieldIdx].decode(input, t, idx, unsafe.Pointer(uintptr(ptr)+offset))
			if err != nil {
				return idx, err
			}
		} else {
			// Skip unknown field
			idx = t.SkipValue(idx)
		}
	}
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func makeDecoderFunc(rt reflect.Type) DecoderFunc {
	switch rt.Kind() {
	case reflect.String:
		return decodeString
	case reflect.Int:
		return decodeInt
	case reflect.Int8:
		return decodeInt8
	case reflect.Int16:
		return decodeInt16
	case reflect.Int32:
		return decodeInt32
	case reflect.Int64:
		return decodeInt64
	case reflect.Uint:
		return decodeUint
	case reflect.Uint8:
		return decodeUint8
	case reflect.Uint16:
		return decodeUint16
	case reflect.Uint32:
		return decodeUint32
	case reflect.Uint64:
		return decodeUint64
	case reflect.Float32:
		return decodeFloat32
	case reflect.Float64:
		return decodeFloat64
	case reflect.Bool:
		return decodeBool
	case reflect.Struct:
		subDec := Compile(rt)
		return func(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
			return subDec.Decode(input, t, idx, ptr)
		}
	case reflect.Slice:
		// TODO: Implement Typed Slice Decoder using Reflection/Unsafe for speed
		// For now we skip to avoid crash, but we need full Slice support for completeness.
		// Since we implemented DecodeValue which handles slices generically,
		// we could use that via unsafe fallback?
		// Or assume the user calls DecodeValue for slices?
		// But here we are inside a struct.
		// Let's implement a wrapper that calls DecodeSlice (generic) but using unsafe pointer cast to interface? No.
		// We need a specific slice decoder.
		// Let's use a placeholder that falls back to reflection-based decoding for now.
		return func(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
			// Create a reflect.Value from the pointer
			rv := reflect.NewAt(rt, ptr).Elem()
			val, err := DecodeValue(input, t, idx, rv)
			return val, err
		}
	default:
		return decodeSkip
	}
}

// Primitive decoders

func decodeString(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeString {
		return idx, &DecodeError{"expected string"}
	}
	*(*string)(ptr) = unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
	return idx + 1, nil
}

func decodeInt(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseInt64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*int)(ptr) = int(v)
	return idx + 1, nil
}

func decodeInt8(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseInt64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*int8)(ptr) = int8(v)
	return idx + 1, nil
}

func decodeInt16(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseInt64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*int16)(ptr) = int16(v)
	return idx + 1, nil
}

func decodeInt32(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseInt64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*int32)(ptr) = int32(v)
	return idx + 1, nil
}

func decodeInt64(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseInt64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*int64)(ptr) = v
	return idx + 1, nil
}

func decodeUint(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseUint64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*uint)(ptr) = uint(v)
	return idx + 1, nil
}

func decodeUint8(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseUint64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*uint8)(ptr) = uint8(v)
	return idx + 1, nil
}

func decodeUint16(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseUint64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*uint16)(ptr) = uint16(v)
	return idx + 1, nil
}

func decodeUint32(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseUint64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*uint32)(ptr) = uint32(v)
	return idx + 1, nil
}

func decodeUint64(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	v, ok := numbers.ParseUint64(input[entry.Offset : entry.Offset+entry.Length])
	if !ok {
		return idx, &DecodeError{"invalid integer"}
	}
	*(*uint64)(ptr) = v
	return idx + 1, nil
}

func decodeFloat32(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return idx, err
	}
	*(*float32)(ptr) = float32(v)
	return idx + 1, nil
}

func decodeFloat64(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type != tape.TypeNumber {
		return idx, &DecodeError{"expected number"}
	}
	s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return idx, err
	}
	*(*float64)(ptr) = v
	return idx + 1, nil
}

func decodeBool(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	entry := t.Get(idx)
	if entry.Type == tape.TypeTrue {
		*(*bool)(ptr) = true
	} else if entry.Type == tape.TypeFalse {
		*(*bool)(ptr) = false
	} else {
		return idx, &DecodeError{"expected boolean"}
	}
	return idx + 1, nil
}

func decodeSkip(input []byte, t *tape.Tape, idx int, ptr unsafe.Pointer) (int, error) {
	return t.SkipValue(idx), nil
}

var (
	errInvalidNumber = &DecodeError{"invalid number"}
	errInvalidBool   = &DecodeError{"invalid boolean"}
)

type DecodeError struct {
	Message string
}

func (e *DecodeError) Error() string {
	return e.Message
}

// --- Decoder Cache ---

var decoderCache sync.Map

// GetDecoder gets or creates a cached decoder for the type.
func GetDecoder(t reflect.Type) *StructDecoder {
	if dec, ok := decoderCache.Load(t); ok {
		return dec.(*StructDecoder)
	}
	dec := Compile(t)
	decoderCache.Store(t, dec)
	return dec
}

// --- Generic Value Decoding ---

// DecodeValue decodes a value into a reflect.Value.
// This is slower than struct decoding but necessary for interface{}, Map, etc.
func DecodeValue(input []byte, t *tape.Tape, idx int, rv reflect.Value) (int, error) {
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		return DecodeValue(input, t, idx, rv.Elem())

	case reflect.Interface:
		if rv.NumMethod() == 0 {
			// empty interface -> decode into default types
			val, nextIdx, err := decodeInterface(input, t, idx)
			if err != nil {
				return idx, err
			}
			rv.Set(reflect.ValueOf(val))
			return nextIdx, nil
		}
		return idx, &DecodeError{"interface unmarshal not supported"}

	case reflect.Map:
		return decodeMap(input, t, idx, rv)

	case reflect.Slice:
		return decodeSlice(input, t, idx, rv)

	case reflect.Struct:
		// Use cached decoder
		dec := GetDecoder(rv.Type())
		return dec.Decode(input, t, idx, unsafe.Pointer(rv.UnsafeAddr()))

	case reflect.String:
		entry := t.Get(idx)
		if entry.Type != tape.TypeString {
			return idx, &DecodeError{"expected string"}
		}
		rv.SetString(unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length]))
		return idx + 1, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		entry := t.Get(idx)
		if entry.Type != tape.TypeNumber {
			return idx, &DecodeError{"expected number"}
		}
		// TODO: Optimized number parsing here too
		// For now simple string conv
		s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			// Try float
			f, err2 := strconv.ParseFloat(s, 64)
			if err2 != nil {
				return idx, err
			}
			v = int64(f)
		}
		rv.SetInt(v)
		return idx + 1, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		entry := t.Get(idx)
		if entry.Type != tape.TypeNumber {
			return idx, &DecodeError{"expected number"}
		}
		s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return idx, err
		}
		rv.SetUint(v)
		return idx + 1, nil

	case reflect.Float32, reflect.Float64:
		entry := t.Get(idx)
		if entry.Type != tape.TypeNumber {
			return idx, &DecodeError{"expected number"}
		}
		s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return idx, err
		}
		rv.SetFloat(v)
		return idx + 1, nil

	case reflect.Bool:
		entry := t.Get(idx)
		if entry.Type == tape.TypeTrue {
			rv.SetBool(true)
		} else if entry.Type == tape.TypeFalse {
			rv.SetBool(false)
		} else {
			return idx, &DecodeError{"expected bool"}
		}
		return idx + 1, nil

	default:
		return t.SkipValue(idx), nil // Skip unsupported
	}
}

func decodeInterface(input []byte, t *tape.Tape, idx int) (interface{}, int, error) {
	entry := t.Get(idx)
	switch entry.Type {
	case tape.TypeObjectStart:
		// Decode as map[string]interface{}
		m := make(map[string]interface{})
		idx++
		for {
			entry := t.Get(idx)
			if entry.Type == tape.TypeObjectEnd {
				return m, idx + 1, nil
			}
			// Key
			// t.Get(idx) is the key
			if entry.Type != tape.TypeString && entry.Type != tape.TypeKey {
				return nil, idx, &DecodeError{"expected key in object"}
			}
			key := string(input[entry.Offset : entry.Offset+entry.Length])
			idx++

			// Value
			val, next, err := decodeInterface(input, t, idx)
			if err != nil {
				return nil, idx, err
			}
			m[key] = val
			idx = next
		}
	case tape.TypeArrayStart:
		// Decode as []interface{}
		slice := make([]interface{}, 0)
		idx++
		for {
			if t.Get(idx).Type == tape.TypeArrayEnd {
				return slice, idx + 1, nil
			}
			val, next, err := decodeInterface(input, t, idx)
			if err != nil {
				return nil, idx, err
			}
			slice = append(slice, val)
			idx = next
		}
	case tape.TypeString:
		s := string(input[entry.Offset : entry.Offset+entry.Length]) // Copy for interface{}
		return s, idx + 1, nil
	case tape.TypeNumber:
		s := unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length])
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, idx, err
		}
		return f, idx + 1, nil // standard JSON uses float64 for numbers
	case tape.TypeTrue:
		return true, idx + 1, nil
	case tape.TypeFalse:
		return false, idx + 1, nil
	case tape.TypeNull:
		return nil, idx + 1, nil
	}
	return nil, idx, &DecodeError{"unknown type"}
}

func decodeMap(input []byte, t *tape.Tape, idx int, rv reflect.Value) (int, error) {
	if t.Get(idx).Type == tape.TypeNull {
		return idx + 1, nil
	}
	if t.Get(idx).Type != tape.TypeObjectStart {
		return idx, &DecodeError{"expected object for map"}
	}
	idx++

	if rv.IsNil() {
		rv.Set(reflect.MakeMap(rv.Type()))
	}

	keyType := rv.Type().Key()
	elemType := rv.Type().Elem()

	for {
		entry := t.Get(idx)
		if entry.Type == tape.TypeObjectEnd {
			return idx + 1, nil
		}

		// Key
		// keyEntry := t.Get(idx) // Already got entry
		if entry.Type != tape.TypeString && entry.Type != tape.TypeKey {
			return idx, &DecodeError{"expected key"}
		}

		idx++
		kVal := reflect.New(keyType).Elem()
		// Basic string key
		if keyType.Kind() == reflect.String {
			kVal.SetString(unsafeutil.BytesToString(input[entry.Offset : entry.Offset+entry.Length]))
		}

		// Value
		eVal := reflect.New(elemType).Elem()
		next, err := DecodeValue(input, t, idx, eVal)
		if err != nil {
			return idx, err
		}
		rv.SetMapIndex(kVal, eVal)
		idx = next
	}
}

func decodeSlice(input []byte, t *tape.Tape, idx int, rv reflect.Value) (int, error) {
	if t.Get(idx).Type == tape.TypeNull {
		return idx + 1, nil
	}
	if t.Get(idx).Type != tape.TypeArrayStart {
		return idx, &DecodeError{"expected array for slice"}
	}
	idx++

	// Reset slice
	rv.SetLen(0)

	elemType := rv.Type().Elem()

	for {
		if t.Get(idx).Type == tape.TypeArrayEnd {
			return idx + 1, nil
		}

		eVal := reflect.New(elemType).Elem()
		next, err := DecodeValue(input, t, idx, eVal)
		if err != nil {
			return idx, err
		}
		rv.Set(reflect.Append(rv, eVal))
		idx = next
	}
}
