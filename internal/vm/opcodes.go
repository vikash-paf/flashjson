// Package vm provides the OpCode compiler and executor for JSON unmarshaling.
// It analyzes Go types once and caches OpCodes for fast repeated use.
package vm

// OpType represents the type of operation.
type OpType uint8

const (
	// Control operations
	OpEnd OpType = iota // End of program

	// Container operations
	OpObjectStart // Expect object start
	OpObjectEnd   // Expect object end
	OpArrayStart  // Expect array start
	OpArrayEnd    // Expect array end

	// Field operations
	OpFindKey   // Find a key in object, arg: key name
	OpSkipValue // Skip any value
	OpSkipToEnd // Skip remaining keys in object

	// Read operations (write to field at offset)
	OpReadString     // Read string field
	OpReadInt        // Read int/int8/int16/int32/int64
	OpReadUint       // Read uint/uint8/uint16/uint32/uint64
	OpReadFloat      // Read float32/float64
	OpReadBool       // Read bool
	OpReadNull       // Read null (sets pointer to nil)
	OpReadAny        // Read into interface{}
	OpReadRawMessage // Read into json.RawMessage

	// Complex type operations
	OpReadStruct  // Recursively unmarshal struct
	OpReadSlice   // Read array into slice
	OpReadMap     // Read object into map
	OpReadPointer // Read value into pointer (allocate if needed)

	// Interface support
	OpCallUnmarshaler     // Call custom UnmarshalJSON
	OpCallTextUnmarshaler // Call custom UnmarshalText
)

// OpCode is a single instruction in the unmarshal program.
type OpCode struct {
	Op     OpType
	Key    string   // For OpFindKey: the key to find
	Offset uintptr  // Field offset in struct
	Size   int      // Size of the field (for ints)
	Child  *Program // For nested structs, slices, maps
}

// Program is a compiled sequence of OpCodes for a type.
type Program struct {
	OpCodes []OpCode
	// Type information for creating new instances
	IsPointer   bool
	ElemProgram *Program // For slices: program for element type
}

// OpName returns a human-readable name for the operation.
func OpName(op OpType) string {
	switch op {
	case OpEnd:
		return "End"
	case OpObjectStart:
		return "ObjectStart"
	case OpObjectEnd:
		return "ObjectEnd"
	case OpArrayStart:
		return "ArrayStart"
	case OpArrayEnd:
		return "ArrayEnd"
	case OpFindKey:
		return "FindKey"
	case OpSkipValue:
		return "SkipValue"
	case OpSkipToEnd:
		return "SkipToEnd"
	case OpReadString:
		return "ReadString"
	case OpReadInt:
		return "ReadInt"
	case OpReadUint:
		return "ReadUint"
	case OpReadFloat:
		return "ReadFloat"
	case OpReadBool:
		return "ReadBool"
	case OpReadNull:
		return "ReadNull"
	case OpReadAny:
		return "ReadAny"
	case OpReadRawMessage:
		return "ReadRawMessage"
	case OpReadStruct:
		return "ReadStruct"
	case OpReadSlice:
		return "ReadSlice"
	case OpReadMap:
		return "ReadMap"
	case OpReadPointer:
		return "ReadPointer"
	case OpCallUnmarshaler:
		return "CallUnmarshaler"
	case OpCallTextUnmarshaler:
		return "CallTextUnmarshaler"
	default:
		return "Unknown"
	}
}
