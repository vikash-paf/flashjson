// Package indexer provides JSON scanning and tape generation.
// The indexer scans JSON input and produces a Tape - a flat structural index.
package indexer

import (
	"github.com/vikash-paf/flashjson/internal/tape"
)

// Error types for indexer errors
type SyntaxError struct {
	Offset  int
	Message string
}

func (e *SyntaxError) Error() string {
	return e.Message
}

// Indexer scans JSON and builds a Tape.
type Indexer struct {
	input []byte
	pos   int
	tape  *tape.Tape
}

// New creates a new Indexer.
func New() *Indexer {
	return &Indexer{}
}

// Index scans the JSON input and populates the tape.
// The tape should be reset before calling this.
func (idx *Indexer) Index(input []byte, t *tape.Tape) error {
	idx.input = input
	idx.pos = 0
	idx.tape = t

	idx.skipWhitespace()

	if idx.pos >= len(input) {
		return &SyntaxError{0, "empty JSON input"}
	}

	if err := idx.indexValue(); err != nil {
		return err
	}

	idx.skipWhitespace()

	// Should be at end of input
	if idx.pos < len(input) {
		return &SyntaxError{idx.pos, "unexpected data after JSON value"}
	}

	return nil
}

// indexValue indexes any JSON value.
func (idx *Indexer) indexValue() error {
	if idx.pos >= len(idx.input) {
		return &SyntaxError{idx.pos, "unexpected end of input"}
	}

	switch idx.input[idx.pos] {
	case '{':
		return idx.indexObject()
	case '[':
		return idx.indexArray()
	case '"':
		return idx.indexString()
	case 't':
		return idx.indexTrue()
	case 'f':
		return idx.indexFalse()
	case 'n':
		return idx.indexNull()
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return idx.indexNumber()
	default:
		return &SyntaxError{idx.pos, "unexpected character"}
	}
}

// indexObject indexes a JSON object.
func (idx *Indexer) indexObject() error {
	// Record the opening brace
	idx.tape.Append(tape.TypeObjectStart, idx.pos, 0)
	idx.pos++ // skip '{'

	idx.skipWhitespace()

	// Empty object?
	if idx.pos < len(idx.input) && idx.input[idx.pos] == '}' {
		idx.tape.Append(tape.TypeObjectEnd, idx.pos, 0)
		idx.pos++
		return nil
	}

	// Parse key-value pairs
	for {
		idx.skipWhitespace()

		// Expect a key (string)
		if idx.pos >= len(idx.input) || idx.input[idx.pos] != '"' {
			return &SyntaxError{idx.pos, "expected object key"}
		}

		// Index the key
		if err := idx.indexKey(); err != nil {
			return err
		}

		idx.skipWhitespace()

		// Expect colon
		if idx.pos >= len(idx.input) || idx.input[idx.pos] != ':' {
			return &SyntaxError{idx.pos, "expected ':' after object key"}
		}
		idx.pos++ // skip ':'

		idx.skipWhitespace()

		// Index the value
		if err := idx.indexValue(); err != nil {
			return err
		}

		idx.skipWhitespace()

		// Check for comma or end
		if idx.pos >= len(idx.input) {
			return &SyntaxError{idx.pos, "unexpected end of object"}
		}

		if idx.input[idx.pos] == '}' {
			idx.tape.Append(tape.TypeObjectEnd, idx.pos, 0)
			idx.pos++
			return nil
		}

		if idx.input[idx.pos] == ',' {
			idx.pos++ // skip ','
			continue
		}

		return &SyntaxError{idx.pos, "expected ',' or '}' in object"}
	}
}

// indexArray indexes a JSON array.
func (idx *Indexer) indexArray() error {
	idx.tape.Append(tape.TypeArrayStart, idx.pos, 0)
	idx.pos++ // skip '['

	idx.skipWhitespace()

	// Empty array?
	if idx.pos < len(idx.input) && idx.input[idx.pos] == ']' {
		idx.tape.Append(tape.TypeArrayEnd, idx.pos, 0)
		idx.pos++
		return nil
	}

	// Parse elements
	for {
		idx.skipWhitespace()

		if err := idx.indexValue(); err != nil {
			return err
		}

		idx.skipWhitespace()

		if idx.pos >= len(idx.input) {
			return &SyntaxError{idx.pos, "unexpected end of array"}
		}

		if idx.input[idx.pos] == ']' {
			idx.tape.Append(tape.TypeArrayEnd, idx.pos, 0)
			idx.pos++
			return nil
		}

		if idx.input[idx.pos] == ',' {
			idx.pos++
			continue
		}

		return &SyntaxError{idx.pos, "expected ',' or ']' in array"}
	}
}

// indexKey indexes an object key (same as string but with TypeKey).
func (idx *Indexer) indexKey() error {
	start := idx.pos + 1 // After opening quote

	idx.pos++ // skip opening '"'

	for idx.pos < len(idx.input) {
		c := idx.input[idx.pos]

		if c == '"' {
			// End of key
			length := idx.pos - start
			idx.tape.Append(tape.TypeKey, start, length)
			idx.pos++ // skip closing '"'
			return nil
		}

		if c == '\\' {
			// Escape sequence - skip next character
			idx.pos += 2
			continue
		}

		if c < 0x20 {
			return &SyntaxError{idx.pos, "control character in string"}
		}

		idx.pos++
	}

	return &SyntaxError{idx.pos, "unterminated string"}
}

// indexString indexes a JSON string value.
func (idx *Indexer) indexString() error {
	start := idx.pos + 1 // After opening quote

	idx.pos++ // skip opening '"'

	for idx.pos < len(idx.input) {
		c := idx.input[idx.pos]

		if c == '"' {
			// End of string
			length := idx.pos - start
			idx.tape.Append(tape.TypeString, start, length)
			idx.pos++ // skip closing '"'
			return nil
		}

		if c == '\\' {
			// Escape sequence - skip next character
			idx.pos += 2
			continue
		}

		if c < 0x20 {
			return &SyntaxError{idx.pos, "control character in string"}
		}

		idx.pos++
	}

	return &SyntaxError{idx.pos, "unterminated string"}
}

// indexNumber indexes a JSON number.
func (idx *Indexer) indexNumber() error {
	start := idx.pos

	// Optional minus
	if idx.pos < len(idx.input) && idx.input[idx.pos] == '-' {
		idx.pos++
	}

	// Integer part
	if idx.pos < len(idx.input) && idx.input[idx.pos] == '0' {
		idx.pos++
	} else if idx.pos < len(idx.input) && idx.input[idx.pos] >= '1' && idx.input[idx.pos] <= '9' {
		idx.pos++
		for idx.pos < len(idx.input) && idx.input[idx.pos] >= '0' && idx.input[idx.pos] <= '9' {
			idx.pos++
		}
	} else {
		return &SyntaxError{idx.pos, "invalid number"}
	}

	// Fractional part
	if idx.pos < len(idx.input) && idx.input[idx.pos] == '.' {
		idx.pos++
		if idx.pos >= len(idx.input) || idx.input[idx.pos] < '0' || idx.input[idx.pos] > '9' {
			return &SyntaxError{idx.pos, "invalid number: expected digit after decimal"}
		}
		for idx.pos < len(idx.input) && idx.input[idx.pos] >= '0' && idx.input[idx.pos] <= '9' {
			idx.pos++
		}
	}

	// Exponent part
	if idx.pos < len(idx.input) && (idx.input[idx.pos] == 'e' || idx.input[idx.pos] == 'E') {
		idx.pos++
		if idx.pos < len(idx.input) && (idx.input[idx.pos] == '+' || idx.input[idx.pos] == '-') {
			idx.pos++
		}
		if idx.pos >= len(idx.input) || idx.input[idx.pos] < '0' || idx.input[idx.pos] > '9' {
			return &SyntaxError{idx.pos, "invalid number: expected digit in exponent"}
		}
		for idx.pos < len(idx.input) && idx.input[idx.pos] >= '0' && idx.input[idx.pos] <= '9' {
			idx.pos++
		}
	}

	idx.tape.Append(tape.TypeNumber, start, idx.pos-start)
	return nil
}

// indexTrue indexes the literal "true".
func (idx *Indexer) indexTrue() error {
	if idx.pos+4 > len(idx.input) ||
		idx.input[idx.pos+1] != 'r' ||
		idx.input[idx.pos+2] != 'u' ||
		idx.input[idx.pos+3] != 'e' {
		return &SyntaxError{idx.pos, "invalid literal (expected 'true')"}
	}

	idx.tape.Append(tape.TypeTrue, idx.pos, 4)
	idx.pos += 4
	return nil
}

// indexFalse indexes the literal "false".
func (idx *Indexer) indexFalse() error {
	if idx.pos+5 > len(idx.input) ||
		idx.input[idx.pos+1] != 'a' ||
		idx.input[idx.pos+2] != 'l' ||
		idx.input[idx.pos+3] != 's' ||
		idx.input[idx.pos+4] != 'e' {
		return &SyntaxError{idx.pos, "invalid literal (expected 'false')"}
	}

	idx.tape.Append(tape.TypeFalse, idx.pos, 5)
	idx.pos += 5
	return nil
}

// indexNull indexes the literal "null".
func (idx *Indexer) indexNull() error {
	if idx.pos+4 > len(idx.input) ||
		idx.input[idx.pos+1] != 'u' ||
		idx.input[idx.pos+2] != 'l' ||
		idx.input[idx.pos+3] != 'l' {
		return &SyntaxError{idx.pos, "invalid literal (expected 'null')"}
	}

	idx.tape.Append(tape.TypeNull, idx.pos, 4)
	idx.pos += 4
	return nil
}

// skipWhitespace advances past any whitespace characters.
func (idx *Indexer) skipWhitespace() {
	for idx.pos < len(idx.input) {
		switch idx.input[idx.pos] {
		case ' ', '\t', '\n', '\r':
			idx.pos++
		default:
			return
		}
	}
}

// --- Convenience functions ---

// Index is a convenience function that creates an indexer, indexes the input,
// and returns the tape.
func Index(input []byte) (*tape.Tape, error) {
	t := tape.Get()
	idx := New()

	if err := idx.Index(input, t); err != nil {
		tape.Put(t)
		return nil, err
	}

	return t, nil
}
