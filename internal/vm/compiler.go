package vm

import (
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// Compiler compiles Go types into Programs (sequences of OpCodes).
type Compiler struct {
	cache sync.Map // map[reflect.Type]*Program
}

// NewCompiler creates a new Compiler.
func NewCompiler() *Compiler {
	return &Compiler{}
}

// Compile returns a Program for the given type.
// Programs are cached for reuse.
func (c *Compiler) Compile(t reflect.Type) (*Program, error) {
	// Check cache first
	if prog, ok := c.cache.Load(t); ok {
		return prog.(*Program), nil
	}

	// Compile the type
	prog, err := c.compileType(t)
	if err != nil {
		return nil, err
	}

	// Cache and return
	c.cache.Store(t, prog)
	return prog, nil
}

func (c *Compiler) compileType(t reflect.Type) (*Program, error) {
	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		elemProg, err := c.compileType(t.Elem())
		if err != nil {
			return nil, err
		}
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadPointer, Child: elemProg},
				{Op: OpEnd},
			},
			IsPointer: true,
		}, nil
	}

	switch t.Kind() {
	case reflect.Struct:
		return c.compileStruct(t)
	case reflect.Slice:
		return c.compileSlice(t)
	case reflect.Map:
		return c.compileMap(t)
	case reflect.String:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadString},
				{Op: OpEnd},
			},
		}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadInt, Size: int(t.Size())},
				{Op: OpEnd},
			},
		}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadUint, Size: int(t.Size())},
				{Op: OpEnd},
			},
		}, nil
	case reflect.Float32, reflect.Float64:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadFloat, Size: int(t.Size())},
				{Op: OpEnd},
			},
		}, nil
	case reflect.Bool:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadBool},
				{Op: OpEnd},
			},
		}, nil
	case reflect.Interface:
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadAny},
				{Op: OpEnd},
			},
		}, nil
	default:
		// Unsupported type - use any
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadAny},
				{Op: OpEnd},
			},
		}, nil
	}
}

func (c *Compiler) compileStruct(t reflect.Type) (*Program, error) {
	opcodes := []OpCode{
		{Op: OpObjectStart},
	}

	// Iterate through fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Parse struct tag
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue // Skip this field
		}

		keyName, opts := parseTag(tag)
		if keyName == "" {
			keyName = field.Name
		}

		// Skip if omitempty and we're unmarshaling (it only matters for marshal)
		_ = opts // opts would be used for omitempty in marshal

		// Compile the field type
		fieldProg, err := c.compileType(field.Type)
		if err != nil {
			return nil, err
		}

		// Add FindKey + Read operations
		opcodes = append(opcodes, OpCode{
			Op:     OpFindKey,
			Key:    keyName,
			Offset: field.Offset,
			Child:  fieldProg,
		})
	}

	opcodes = append(opcodes, OpCode{Op: OpSkipToEnd})
	opcodes = append(opcodes, OpCode{Op: OpObjectEnd})
	opcodes = append(opcodes, OpCode{Op: OpEnd})

	return &Program{OpCodes: opcodes}, nil
}

func (c *Compiler) compileSlice(t reflect.Type) (*Program, error) {
	// Compile element type
	elemProg, err := c.compileType(t.Elem())
	if err != nil {
		return nil, err
	}

	return &Program{
		OpCodes: []OpCode{
			{Op: OpReadSlice, Child: elemProg},
			{Op: OpEnd},
		},
		ElemProgram: elemProg,
	}, nil
}

func (c *Compiler) compileMap(t reflect.Type) (*Program, error) {
	// For now, only support map[string]T
	if t.Key().Kind() != reflect.String {
		return &Program{
			OpCodes: []OpCode{
				{Op: OpReadAny},
				{Op: OpEnd},
			},
		}, nil
	}

	// Compile value type
	valueProg, err := c.compileType(t.Elem())
	if err != nil {
		return nil, err
	}

	return &Program{
		OpCodes: []OpCode{
			{Op: OpReadMap, Child: valueProg},
			{Op: OpEnd},
		},
	}, nil
}

// parseTag parses a struct field's json tag.
// Returns the key name and remaining options.
func parseTag(tag string) (string, string) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

// toJSONKey converts a Go field name to JSON key format.
// By default, Go uses the field name as-is, but we can also provide
// automatic camelCase conversion if needed.
func toJSONKey(name string) string {
	if name == "" {
		return ""
	}
	// Default: use as-is (Go encoding/json behavior)
	return name
}

// toCamelCase converts a Go-style name to camelCase.
// Example: UserName -> userName
func toCamelCase(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// --- Global compiler instance ---

var defaultCompiler = NewCompiler()

// Compile compiles a type using the default compiler.
func Compile(t reflect.Type) (*Program, error) {
	return defaultCompiler.Compile(t)
}

// CompileValue compiles the type of a value using the default compiler.
func CompileValue(v interface{}) (*Program, error) {
	return defaultCompiler.Compile(reflect.TypeOf(v))
}
