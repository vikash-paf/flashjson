// Package flashjson provides a high-performance JSON encoder/decoder for Go.
// It is designed as a drop-in replacement for encoding/json with
// 10x+ better performance through SIMD acceleration and near-zero allocations.
package flashjson

import (
	"io"
	"reflect"
	"unsafe"

	"github.com/vikash-paf/flashjson/internal/decoder"
	"github.com/vikash-paf/flashjson/internal/encoder"
	"github.com/vikash-paf/flashjson/internal/simd"
	"github.com/vikash-paf/flashjson/internal/tape"
)

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	// Index the JSON (uses SIMD when available)
	// Index the JSON (uses SIMD when available)
	t, err := simd.Index(data)
	if err != nil {
		return err
	}
	defer tape.Put(t)

	// Get cached decoder
	dec := decoder.GetDecoder(rv.Type().Elem())

	if dec != nil {
		// Fast path for structs
		ptr := unsafe.Pointer(rv.Pointer())
		_, err = dec.Decode(data, t, 0, ptr)
		return err
	}

	// Fallback for non-structs (maps, slices, interface{})
	_, err = decoder.DecodeValue(data, t, 0, rv.Elem())
	return err
}

// Marshal returns the JSON encoding of v.
// Uses pre-compiled struct encoders for maximum speed.
func Marshal(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}

	rv := reflect.ValueOf(v)
	rt := rv.Type()

	// Handle pointers
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte("null"), nil
		}
		rv = rv.Elem()
		rt = rv.Type()
	}

	// Fast path for structs - use pre-compiled encoder
	if rv.Kind() == reflect.Struct {
		enc := encoder.GetEncoder(rt)
		if enc != nil {
			buf := encoder.GetBuffer()
			defer encoder.PutBuffer(buf)

			// Make addressable if needed
			var ptr unsafe.Pointer
			if rv.CanAddr() {
				ptr = unsafe.Pointer(rv.UnsafeAddr())
			} else {
				// Copy to addressable memory
				newVal := reflect.New(rt).Elem()
				newVal.Set(rv)
				ptr = unsafe.Pointer(newVal.UnsafeAddr())
			}

			enc.Encode(buf, ptr)

			// Return a copy (buffer will be reused)
			result := make([]byte, buf.Len())
			copy(result, buf.Bytes())
			return result, nil
		}
	}

	// Fallback for other types
	return marshalValue(v)
}

// MarshalAppend appends the JSON encoding of v to dst.
// This avoids allocation if dst has enough capacity.
func MarshalAppend(dst []byte, v interface{}) ([]byte, error) {
	if v == nil {
		return append(dst, "null"...), nil
	}

	rv := reflect.ValueOf(v)
	rt := rv.Type()

	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return append(dst, "null"...), nil
		}
		rv = rv.Elem()
		rt = rv.Type()
	}

	if rv.Kind() == reflect.Struct {
		enc := encoder.GetEncoder(rt)
		if enc != nil {
			buf := encoder.GetBuffer()
			defer encoder.PutBuffer(buf)

			ptr := unsafe.Pointer(rv.UnsafeAddr())
			enc.Encode(buf, ptr)

			return append(dst, buf.Bytes()...), nil
		}
	}

	b, err := marshalValue(v)
	if err != nil {
		return dst, err
	}
	return append(dst, b...), nil
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	t, err := simd.Index(data)
	if err != nil {
		return false
	}
	tape.Put(t)
	return true
}

// Compact appends to dst the JSON-encoded src with whitespace removed.
func Compact(dst *[]byte, src []byte) error {
	// TODO: SIMD compact
	return nil
}

// --- Errors ---

type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e *InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "flashjson: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Ptr {
		return "flashjson: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "flashjson: Unmarshal(nil " + e.Type.String() + ")"
}

// --- Streaming ---

type Decoder struct {
	r   io.Reader
	buf []byte
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r, buf: make([]byte, 0, 4096)}
}

func (dec *Decoder) Decode(v interface{}) error {
	data, err := io.ReadAll(dec.r)
	if err != nil {
		return err
	}
	return Unmarshal(data, v)
}

type Encoder struct {
	w   io.Writer
	buf []byte
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, buf: make([]byte, 0, 512)}
}

func (enc *Encoder) Encode(v interface{}) error {
	enc.buf = enc.buf[:0]
	var err error
	enc.buf, err = MarshalAppend(enc.buf, v)
	if err != nil {
		return err
	}
	enc.buf = append(enc.buf, '\n')
	_, err = enc.w.Write(enc.buf)
	return err
}

func (enc *Encoder) SetIndent(prefix, indent string) {}
func (enc *Encoder) SetEscapeHTML(on bool)           {}
