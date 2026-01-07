package vm

import (
	"strconv"
	"sync"
	"unsafe"

	"github.com/vikash-paf/flashjson/internal/numbers"
	"github.com/vikash-paf/flashjson/internal/tape"
	"github.com/vikash-paf/flashjson/internal/unsafeutil"
)

// Executor executes a compiled Program against a Tape to populate a struct.
type Executor struct {
	input     []byte
	tape      *tape.Tape
	tapeIndex int
}

// NewExecutor creates a new Executor.
func NewExecutor() *Executor {
	return &Executor{}
}

// Execute runs the program, reading from the tape and writing to ptr.
func (e *Executor) Execute(input []byte, t *tape.Tape, prog *Program, ptr unsafe.Pointer) error {
	e.input = input
	e.tape = t
	e.tapeIndex = 0

	return e.execProgram(prog, ptr)
}

func (e *Executor) execProgram(prog *Program, ptr unsafe.Pointer) error {
	for _, op := range prog.OpCodes {
		if e.tapeIndex >= e.tape.Len() && op.Op != OpEnd {
			return nil // EOF
		}

		switch op.Op {
		case OpEnd:
			return nil

		case OpObjectStart:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeObjectStart {
				return &ExecutionError{e.tapeIndex, "expected object start"}
			}
			e.tapeIndex++

		case OpObjectEnd:
			// Skip to end if not already there
			for e.tapeIndex < e.tape.Len() {
				entry := e.tape.Get(e.tapeIndex)
				if entry.Type == tape.TypeObjectEnd {
					e.tapeIndex++
					break
				}
				e.tapeIndex++
			}

		case OpFindKey:
			// Search for key in current object
			found := e.findKey(op.Key)
			if found {
				// Execute child program to read the value
				fieldPtr := unsafe.Pointer(uintptr(ptr) + op.Offset)
				if err := e.execProgram(op.Child, fieldPtr); err != nil {
					return err
				}
			}

		case OpSkipToEnd:
			// Skip remaining keys until ObjectEnd
			depth := 0
			for e.tapeIndex < e.tape.Len() {
				entry := e.tape.Get(e.tapeIndex)
				switch entry.Type {
				case tape.TypeObjectStart, tape.TypeArrayStart:
					depth++
				case tape.TypeObjectEnd:
					if depth == 0 {
						return nil // Don't consume, let ObjectEnd op handle it
					}
					depth--
				case tape.TypeArrayEnd:
					depth--
				}
				e.tapeIndex++
			}

		case OpReadString:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeString {
				return &ExecutionError{e.tapeIndex, "expected string"}
			}
			// Zero-copy string extraction from input buffer
			str := unsafeutil.BytesToString(e.input[entry.Offset : entry.Offset+entry.Length])
			*(*string)(ptr) = str
			e.tapeIndex++

		case OpReadInt:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeNumber {
				return &ExecutionError{e.tapeIndex, "expected number"}
			}
			// Use SWAR fast integer parsing
			numBytes := e.input[entry.Offset : entry.Offset+entry.Length]
			val, ok := numbers.ParseInt64(numBytes)
			if !ok {
				// Fallback: try parsing as float and truncating
				numStr := unsafeutil.BytesToString(numBytes)
				fval, ferr := strconv.ParseFloat(numStr, 64)
				if ferr != nil {
					return &ExecutionError{e.tapeIndex, "invalid integer"}
				}
				val = int64(fval)
			}
			// Write based on size
			switch op.Size {
			case 1:
				*(*int8)(ptr) = int8(val)
			case 2:
				*(*int16)(ptr) = int16(val)
			case 4:
				*(*int32)(ptr) = int32(val)
			case 8:
				*(*int64)(ptr) = val
			default:
				*(*int)(ptr) = int(val)
			}
			e.tapeIndex++

		case OpReadUint:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeNumber {
				return &ExecutionError{e.tapeIndex, "expected number"}
			}
			numStr := string(e.input[entry.Offset : entry.Offset+entry.Length])
			val, err := strconv.ParseUint(numStr, 10, 64)
			if err != nil {
				return &ExecutionError{e.tapeIndex, "invalid unsigned integer"}
			}
			switch op.Size {
			case 1:
				*(*uint8)(ptr) = uint8(val)
			case 2:
				*(*uint16)(ptr) = uint16(val)
			case 4:
				*(*uint32)(ptr) = uint32(val)
			case 8:
				*(*uint64)(ptr) = val
			default:
				*(*uint)(ptr) = uint(val)
			}
			e.tapeIndex++

		case OpReadFloat:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeNumber {
				return &ExecutionError{e.tapeIndex, "expected number"}
			}
			numStr := string(e.input[entry.Offset : entry.Offset+entry.Length])
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return &ExecutionError{e.tapeIndex, "invalid float"}
			}
			if op.Size == 4 {
				*(*float32)(ptr) = float32(val)
			} else {
				*(*float64)(ptr) = val
			}
			e.tapeIndex++

		case OpReadBool:
			entry := e.tape.Get(e.tapeIndex)
			switch entry.Type {
			case tape.TypeTrue:
				*(*bool)(ptr) = true
			case tape.TypeFalse:
				*(*bool)(ptr) = false
			default:
				return &ExecutionError{e.tapeIndex, "expected boolean"}
			}
			e.tapeIndex++

		case OpReadPointer:
			// For pointer fields, we need to allocate if nil
			// This is simplified - full implementation would handle nil
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type == tape.TypeNull {
				// Set to nil
				*(*unsafe.Pointer)(ptr) = nil
				e.tapeIndex++
			} else {
				// Execute child program
				if err := e.execProgram(op.Child, ptr); err != nil {
					return err
				}
			}

		case OpReadSlice:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeArrayStart {
				return &ExecutionError{e.tapeIndex, "expected array"}
			}
			e.tapeIndex++ // skip ArrayStart

			if err := e.readSlice(op, ptr); err != nil {
				return err
			}

		case OpReadMap:
			entry := e.tape.Get(e.tapeIndex)
			if entry.Type != tape.TypeObjectStart {
				return &ExecutionError{e.tapeIndex, "expected object for map"}
			}
			e.tapeIndex++ // skip ObjectStart

			if err := e.readMap(op, ptr); err != nil {
				return err
			}

		case OpReadAny:
			if err := e.readAny(ptr); err != nil {
				return err
			}

		case OpSkipValue:
			e.tapeIndex = e.tape.SkipValue(e.tapeIndex)
		}
	}

	return nil
}

// findKey searches for a key in the current object, positioning tapeIndex at the value.
func (e *Executor) findKey(key string) bool {
	startIndex := e.tapeIndex

	for e.tapeIndex < e.tape.Len() {
		entry := e.tape.Get(e.tapeIndex)

		switch entry.Type {
		case tape.TypeObjectEnd:
			// Key not found, restore position
			e.tapeIndex = startIndex
			return false

		case tape.TypeKey:
			// Compare key
			keyStr := string(e.input[entry.Offset : entry.Offset+entry.Length])
			e.tapeIndex++ // Move to value

			if keyStr == key {
				return true
			}
			// Skip this key's value
			e.tapeIndex = e.tape.SkipValue(e.tapeIndex)

		default:
			e.tapeIndex++
		}
	}

	e.tapeIndex = startIndex
	return false
}

// readSlice reads an array into a slice.
func (e *Executor) readSlice(op OpCode, ptr unsafe.Pointer) error {
	// Count elements first
	startIdx := e.tapeIndex
	count := 0
	depth := 0

	for i := e.tapeIndex; i < e.tape.Len(); i++ {
		entry := e.tape.Get(i)
		switch entry.Type {
		case tape.TypeArrayStart, tape.TypeObjectStart:
			depth++
		case tape.TypeArrayEnd:
			if depth == 0 {
				goto counted
			}
			depth--
		case tape.TypeObjectEnd:
			depth--
		default:
			if depth == 0 {
				count++
			}
		}
	}
counted:

	// For now, store as []interface{}
	// Full implementation would create proper typed slice
	result := make([]interface{}, 0, count)

	for e.tapeIndex < e.tape.Len() {
		entry := e.tape.Get(e.tapeIndex)
		if entry.Type == tape.TypeArrayEnd {
			e.tapeIndex++
			break
		}

		var elem interface{}
		if err := e.readAnyValue(&elem); err != nil {
			return err
		}
		result = append(result, elem)
	}

	_ = startIdx // unused
	*(*[]interface{})(ptr) = result
	return nil
}

// readMap reads an object into a map[string]interface{}.
func (e *Executor) readMap(op OpCode, ptr unsafe.Pointer) error {
	result := make(map[string]interface{})

	for e.tapeIndex < e.tape.Len() {
		entry := e.tape.Get(e.tapeIndex)

		if entry.Type == tape.TypeObjectEnd {
			e.tapeIndex++
			break
		}

		if entry.Type != tape.TypeKey {
			return &ExecutionError{e.tapeIndex, "expected key in map"}
		}

		key := string(e.input[entry.Offset : entry.Offset+entry.Length])
		e.tapeIndex++

		var value interface{}
		if err := e.readAnyValue(&value); err != nil {
			return err
		}
		result[key] = value
	}

	*(*map[string]interface{})(ptr) = result
	return nil
}

// readAny reads any value into an interface{}.
func (e *Executor) readAny(ptr unsafe.Pointer) error {
	var result interface{}
	if err := e.readAnyValue(&result); err != nil {
		return err
	}
	*(*interface{})(ptr) = result
	return nil
}

// readAnyValue reads any JSON value into an interface{}.
func (e *Executor) readAnyValue(result *interface{}) error {
	if e.tapeIndex >= e.tape.Len() {
		return nil
	}

	entry := e.tape.Get(e.tapeIndex)

	switch entry.Type {
	case tape.TypeString:
		*result = string(e.input[entry.Offset : entry.Offset+entry.Length])
		e.tapeIndex++

	case tape.TypeNumber:
		numStr := string(e.input[entry.Offset : entry.Offset+entry.Length])
		// Try int first, then float
		if iv, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			*result = iv
		} else if fv, err := strconv.ParseFloat(numStr, 64); err == nil {
			*result = fv
		}
		e.tapeIndex++

	case tape.TypeTrue:
		*result = true
		e.tapeIndex++

	case tape.TypeFalse:
		*result = false
		e.tapeIndex++

	case tape.TypeNull:
		*result = nil
		e.tapeIndex++

	case tape.TypeObjectStart:
		e.tapeIndex++
		obj := make(map[string]interface{})
		for e.tapeIndex < e.tape.Len() {
			ent := e.tape.Get(e.tapeIndex)
			if ent.Type == tape.TypeObjectEnd {
				e.tapeIndex++
				break
			}
			if ent.Type != tape.TypeKey {
				return &ExecutionError{e.tapeIndex, "expected key"}
			}
			key := string(e.input[ent.Offset : ent.Offset+ent.Length])
			e.tapeIndex++
			var val interface{}
			if err := e.readAnyValue(&val); err != nil {
				return err
			}
			obj[key] = val
		}
		*result = obj

	case tape.TypeArrayStart:
		e.tapeIndex++
		arr := make([]interface{}, 0)
		for e.tapeIndex < e.tape.Len() {
			ent := e.tape.Get(e.tapeIndex)
			if ent.Type == tape.TypeArrayEnd {
				e.tapeIndex++
				break
			}
			var val interface{}
			if err := e.readAnyValue(&val); err != nil {
				return err
			}
			arr = append(arr, val)
		}
		*result = arr

	default:
		e.tapeIndex++
	}

	return nil
}

// ExecutionError represents an error during execution.
type ExecutionError struct {
	TapeIndex int
	Message   string
}

func (e *ExecutionError) Error() string {
	return e.Message
}

// --- Global executor pool ---

var executorPool = sync.Pool{
	New: func() interface{} {
		return NewExecutor()
	},
}

// GetExecutor gets an executor from the pool.
func GetExecutor() *Executor {
	return executorPool.Get().(*Executor)
}

// PutExecutor returns an executor to the pool.
func PutExecutor(e *Executor) {
	executorPool.Put(e)
}
