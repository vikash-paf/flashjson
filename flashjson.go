// Package flashjson provides a high-performance JSON encoder/decoder for Go.
// It is designed as a drop-in replacement for encoding/json with significantly
// better performance through SIMD acceleration and near-zero memory allocations.
//
// Basic usage is identical to encoding/json:
//
//	// Unmarshal
//	var user User
//	err := flashjson.Unmarshal(data, &user)
//
//	// Marshal
//	data, err := flashjson.Marshal(user)
//
// For streaming:
//
//	// Decode from reader
//	dec := flashjson.NewDecoder(reader)
//	err := dec.Decode(&user)
//
//	// Encode to writer
//	enc := flashjson.NewEncoder(writer)
//	err := enc.Encode(user)
package flashjson

import (
	"io"
	"reflect"
	"unsafe"

	"github.com/vikash-paf/flashjson/internal/indexer"
	"github.com/vikash-paf/flashjson/internal/tape"
	"github.com/vikash-paf/flashjson/internal/vm"
)

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v.
//
// Unmarshal uses the inverse of the encodings that Marshal uses,
// allocating maps, slices, and pointers as necessary.
func Unmarshal(data []byte, v interface{}) error {
	// Get value and ensure it's a pointer
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &InvalidUnmarshalError{reflect.TypeOf(v)}
	}

	// Index the JSON
	t, err := indexer.Index(data)
	if err != nil {
		return err
	}
	defer tape.Put(t)

	// Compile the type (cached)
	prog, err := vm.Compile(rv.Type().Elem())
	if err != nil {
		return err
	}

	// Execute the program
	exec := vm.GetExecutor()
	defer vm.PutExecutor(exec)

	ptr := unsafe.Pointer(rv.Pointer())
	return exec.Execute(data, t, prog, ptr)
}

// Marshal returns the JSON encoding of v.
//
// Marshal traverses the value v recursively. If an encountered value
// implements the Marshaler interface, Marshal calls its MarshalJSON method
// to produce JSON.
//
// Note: Currently using standard library for marshal.
// TODO: Implement fast marshal path.
func Marshal(v interface{}) ([]byte, error) {
	// For now, delegate to a simple implementation
	// Full implementation would use SIMD-accelerated encoding
	return marshalValue(v)
}

// MarshalIndent is like Marshal but applies Indent to format the output.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	// Simple implementation for now
	b, err := Marshal(v)
	if err != nil {
		return nil, err
	}
	// TODO: Implement indentation
	return b, nil
}

// Valid reports whether data is a valid JSON encoding.
func Valid(data []byte) bool {
	t, err := indexer.Index(data)
	if err != nil {
		return false
	}
	tape.Put(t)
	return true
}

// Compact appends to dst the JSON-encoded src with
// insignificant space characters elided.
func Compact(dst *[]byte, src []byte) error {
	// TODO: Implement fast compact
	return nil
}

// --- Errors ---

// InvalidUnmarshalError describes an invalid argument passed to Unmarshal.
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

// --- Encoder/Decoder for streaming ---

// Decoder reads and decodes JSON values from an input stream.
type Decoder struct {
	r   io.Reader
	buf []byte
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode reads the next JSON-encoded value from its input
// and stores it in the value pointed to by v.
func (dec *Decoder) Decode(v interface{}) error {
	// Read all data (simple implementation)
	// TODO: Implement streaming decode
	data, err := io.ReadAll(dec.r)
	if err != nil {
		return err
	}
	return Unmarshal(data, v)
}

// Encoder writes JSON values to an output stream.
type Encoder struct {
	w      io.Writer
	indent string
	prefix string
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode writes the JSON encoding of v to the stream.
func (enc *Encoder) Encode(v interface{}) error {
	data, err := Marshal(v)
	if err != nil {
		return err
	}
	if enc.indent != "" {
		// TODO: Apply indent
	}
	data = append(data, '\n')
	_, err = enc.w.Write(data)
	return err
}

// SetIndent instructs the encoder to format each subsequent encoded
// value as if indented by the package-level function Indent(dst, src, prefix, indent).
func (enc *Encoder) SetIndent(prefix, indent string) {
	enc.prefix = prefix
	enc.indent = indent
}

// SetEscapeHTML specifies whether problematic HTML characters should be escaped.
func (enc *Encoder) SetEscapeHTML(on bool) {
	// TODO: Implement HTML escaping option
}
