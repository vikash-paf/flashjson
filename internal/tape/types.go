// Package tape provides the structural index for JSON documents.
// The Tape is a flat array of entries that describe JSON structure
// without storing the actual values - just their positions in the input.
package tape

// Entry type constants
const (
	TypeInvalid     uint8 = iota // 0: Should never appear in valid tape
	TypeObjectStart              // 1: { - start of object
	TypeObjectEnd                // 2: } - end of object
	TypeArrayStart               // 3: [ - start of array
	TypeArrayEnd                 // 4: ] - end of array
	TypeKey                      // 5: object key (always a string)
	TypeString                   // 6: string value
	TypeNumber                   // 7: number (int or float, determined at extraction)
	TypeTrue                     // 8: true
	TypeFalse                    // 9: false
	TypeNull                     // 10: null
)

// TypeName returns a human-readable name for an entry type.
func TypeName(t uint8) string {
	switch t {
	case TypeInvalid:
		return "Invalid"
	case TypeObjectStart:
		return "ObjectStart"
	case TypeObjectEnd:
		return "ObjectEnd"
	case TypeArrayStart:
		return "ArrayStart"
	case TypeArrayEnd:
		return "ArrayEnd"
	case TypeKey:
		return "Key"
	case TypeString:
		return "String"
	case TypeNumber:
		return "Number"
	case TypeTrue:
		return "True"
	case TypeFalse:
		return "False"
	case TypeNull:
		return "Null"
	default:
		return "Unknown"
	}
}

// IsContainer returns true if the type is an object or array start.
func IsContainer(t uint8) bool {
	return t == TypeObjectStart || t == TypeArrayStart
}

// IsContainerEnd returns true if the type is an object or array end.
func IsContainerEnd(t uint8) bool {
	return t == TypeObjectEnd || t == TypeArrayEnd
}

// IsPrimitive returns true if the type is a primitive value (string, number, bool, null).
func IsPrimitive(t uint8) bool {
	return t == TypeString || t == TypeNumber || t == TypeTrue || t == TypeFalse || t == TypeNull
}
