package simd

import (
	"sync"

	"github.com/vikash-paf/flashjson/internal/tape"
)

// Indexer is the SIMD-accelerated JSON indexer.
// It falls back to generic indexing if SIMD is not available.
type Indexer struct {
	// Configuration
	useSIMD bool

	// Buffers
	structPos []uint32
}

var indexerPool = sync.Pool{
	New: func() interface{} {
		return &Indexer{
			useSIMD:   UsesSIMD,
			structPos: make([]uint32, 0, 4096), // Pre-allocate
		}
	},
}

// GetIndexer gets an indexer from the pool.
func GetIndexer() *Indexer {
	return indexerPool.Get().(*Indexer)
}

// PutIndexer returns an indexer to the pool.
func PutIndexer(idx *Indexer) {
	idx.structPos = idx.structPos[:0]
	indexerPool.Put(idx)
}

// NewIndexer creates a new SIMD indexer.
func NewIndexer() *Indexer {
	return &Indexer{
		useSIMD: UsesSIMD,
	}
}

// Index scans JSON input and populates the tape.
// Uses SIMD acceleration when available.
func (idx *Indexer) Index(input []byte, t *tape.Tape) error {
	if idx.useSIMD && len(input) >= 64 {
		return idx.indexSIMD(input, t)
	}
	return idx.indexGeneric(input, t)
}

// Global helper for easy access
func Index(input []byte) (*tape.Tape, error) {
	t := tape.Get()
	idx := GetIndexer()
	defer PutIndexer(idx)

	if err := idx.Index(input, t); err != nil {
		tape.Put(t)
		return nil, err
	}
	return t, nil
}

// indexGeneric is the fallback pure-Go implementation.
func (idx *Indexer) indexGeneric(input []byte, t *tape.Tape) error {
	// Use the existing generic indexer
	// This delegates to internal/indexer which we already have
	return indexGenericFallback(input, t)
}

// indexSIMD uses SIMD to find structural characters, then processes them.
func (idx *Indexer) indexSIMD(input []byte, t *tape.Tape) error {
	n := len(input)
	pos := 0

	// Skip leading whitespace
	for pos < n && isWhitespace(input[pos]) {
		pos++
	}

	if pos >= n {
		return &SyntaxError{0, "empty JSON input"}
	}

	// Use SIMD to process chunks
	return idx.processSIMD(input, pos, t)
}

// processSIMD processes the input using SIMD operations.
// It finds structural characters in chunks and builds the tape.
func (idx *Indexer) processSIMD(input []byte, startPos int, t *tape.Tape) error {
	n := len(input)
	pos := startPos

	// For SIMD, we process in two phases:
	// Phase 1: Find all structural character positions using SIMD
	// Phase 2: Build the tape from those positions

	// Reuse buffer
	if cap(idx.structPos) < n/4 {
		idx.structPos = make([]uint32, 0, n/4)
	}
	idx.structPos = idx.structPos[:0]

	// Process 64-byte chunks with SIMD
	for pos+64 <= n {
		mask := findStructuralAVX2(input[pos : pos+64])

		// Extract bit positions from mask
		for mask != 0 {
			// Find lowest set bit
			bitPos := trailingZeros64(mask)
			idx.structPos = append(idx.structPos, uint32(pos+bitPos))
			// Clear the bit
			mask &= mask - 1
		}
		pos += 64
	}

	// Handle remaining bytes with scalar code
	for ; pos < n; pos++ {
		if isStructural(input[pos]) {
			idx.structPos = append(idx.structPos, uint32(pos))
		}
	}

	// Phase 2: Build tape from structural positions
	return buildTapeFromStructural(input, idx.structPos, t)
}

// buildTapeFromStructural builds the tape from structural character positions.
func buildTapeFromStructural(input []byte, positions []uint32, t *tape.Tape) error {
	n := len(input)
	posIdx := 0
	inputPos := 0

	// Skip leading whitespace
	for inputPos < n && isWhitespace(input[inputPos]) {
		inputPos++
	}

	if inputPos >= n {
		return &SyntaxError{0, "empty JSON input"}
	}

	return parseValue(input, &inputPos, positions, &posIdx, t)
}

// parseValue parses any JSON value at the current position.
func parseValue(input []byte, pos *int, structural []uint32, sIdx *int, t *tape.Tape) error {
	skipWhitespace(input, pos)

	if *pos >= len(input) {
		return &SyntaxError{*pos, "unexpected end of input"}
	}

	c := input[*pos]

	switch c {
	case '{':
		return parseObject(input, pos, structural, sIdx, t)
	case '[':
		return parseArray(input, pos, structural, sIdx, t)
	case '"':
		return parseString(input, pos, t, false)
	case 't':
		return parseLiteral(input, pos, t, "true", tape.TypeTrue)
	case 'f':
		return parseLiteral(input, pos, t, "false", tape.TypeFalse)
	case 'n':
		return parseLiteral(input, pos, t, "null", tape.TypeNull)
	default:
		if c == '-' || (c >= '0' && c <= '9') {
			return parseNumber(input, pos, t)
		}
		return &SyntaxError{*pos, "unexpected character"}
	}
}

func parseObject(input []byte, pos *int, structural []uint32, sIdx *int, t *tape.Tape) error {
	t.Append(tape.TypeObjectStart, *pos, 0)
	*pos++ // skip '{'

	skipWhitespace(input, pos)

	if *pos < len(input) && input[*pos] == '}' {
		t.Append(tape.TypeObjectEnd, *pos, 0)
		*pos++
		return nil
	}

	for {
		skipWhitespace(input, pos)

		if *pos >= len(input) || input[*pos] != '"' {
			return &SyntaxError{*pos, "expected object key"}
		}

		// Parse key
		if err := parseString(input, pos, t, true); err != nil {
			return err
		}

		skipWhitespace(input, pos)

		if *pos >= len(input) || input[*pos] != ':' {
			return &SyntaxError{*pos, "expected ':'"}
		}
		*pos++ // skip ':'

		skipWhitespace(input, pos)

		// Parse value
		if err := parseValue(input, pos, structural, sIdx, t); err != nil {
			return err
		}

		skipWhitespace(input, pos)

		if *pos >= len(input) {
			return &SyntaxError{*pos, "unexpected end of object"}
		}

		if input[*pos] == '}' {
			t.Append(tape.TypeObjectEnd, *pos, 0)
			*pos++
			return nil
		}

		if input[*pos] == ',' {
			*pos++
			continue
		}

		return &SyntaxError{*pos, "expected ',' or '}'"}
	}
}

func parseArray(input []byte, pos *int, structural []uint32, sIdx *int, t *tape.Tape) error {
	t.Append(tape.TypeArrayStart, *pos, 0)
	*pos++ // skip '['

	skipWhitespace(input, pos)

	if *pos < len(input) && input[*pos] == ']' {
		t.Append(tape.TypeArrayEnd, *pos, 0)
		*pos++
		return nil
	}

	for {
		skipWhitespace(input, pos)

		if err := parseValue(input, pos, structural, sIdx, t); err != nil {
			return err
		}

		skipWhitespace(input, pos)

		if *pos >= len(input) {
			return &SyntaxError{*pos, "unexpected end of array"}
		}

		if input[*pos] == ']' {
			t.Append(tape.TypeArrayEnd, *pos, 0)
			*pos++
			return nil
		}

		if input[*pos] == ',' {
			*pos++
			continue
		}

		return &SyntaxError{*pos, "expected ',' or ']'"}
	}
}

func parseString(input []byte, pos *int, t *tape.Tape, isKey bool) error {
	start := *pos + 1 // after opening quote
	*pos++            // skip '"'

	for *pos < len(input) {
		c := input[*pos]

		if c == '"' {
			length := *pos - start
			if isKey {
				t.Append(tape.TypeKey, start, length)
			} else {
				t.Append(tape.TypeString, start, length)
			}
			*pos++ // skip closing '"'
			return nil
		}

		if c == '\\' {
			*pos += 2 // skip escape sequence
			continue
		}

		if c < 0x20 {
			return &SyntaxError{*pos, "control character in string"}
		}

		*pos++
	}

	return &SyntaxError{*pos, "unterminated string"}
}

func parseNumber(input []byte, pos *int, t *tape.Tape) error {
	start := *pos

	// Optional minus
	if *pos < len(input) && input[*pos] == '-' {
		*pos++
	}

	// Integer part
	if *pos < len(input) && input[*pos] == '0' {
		*pos++
	} else if *pos < len(input) && input[*pos] >= '1' && input[*pos] <= '9' {
		*pos++
		for *pos < len(input) && input[*pos] >= '0' && input[*pos] <= '9' {
			*pos++
		}
	} else {
		return &SyntaxError{*pos, "invalid number"}
	}

	// Fractional part
	if *pos < len(input) && input[*pos] == '.' {
		*pos++
		if *pos >= len(input) || input[*pos] < '0' || input[*pos] > '9' {
			return &SyntaxError{*pos, "invalid number"}
		}
		for *pos < len(input) && input[*pos] >= '0' && input[*pos] <= '9' {
			*pos++
		}
	}

	// Exponent
	if *pos < len(input) && (input[*pos] == 'e' || input[*pos] == 'E') {
		*pos++
		if *pos < len(input) && (input[*pos] == '+' || input[*pos] == '-') {
			*pos++
		}
		if *pos >= len(input) || input[*pos] < '0' || input[*pos] > '9' {
			return &SyntaxError{*pos, "invalid number"}
		}
		for *pos < len(input) && input[*pos] >= '0' && input[*pos] <= '9' {
			*pos++
		}
	}

	t.Append(tape.TypeNumber, start, *pos-start)
	return nil
}

func parseLiteral(input []byte, pos *int, t *tape.Tape, expected string, typ uint8) error {
	if *pos+len(expected) > len(input) {
		return &SyntaxError{*pos, "unexpected end of input"}
	}

	for i := 0; i < len(expected); i++ {
		if input[*pos+i] != expected[i] {
			return &SyntaxError{*pos, "invalid literal"}
		}
	}

	t.Append(typ, *pos, len(expected))
	*pos += len(expected)
	return nil
}

func skipWhitespace(input []byte, pos *int) {
	for *pos < len(input) && isWhitespace(input[*pos]) {
		*pos++
	}
}

func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isStructural(c byte) bool {
	return c == '{' || c == '}' || c == '[' || c == ']' || c == ':' || c == ',' || c == '"'
}

func trailingZeros64(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for x&1 == 0 {
		n++
		x >>= 1
	}
	return n
}

// SyntaxError represents a JSON syntax error.
type SyntaxError struct {
	Offset  int
	Message string
}

func (e *SyntaxError) Error() string {
	return e.Message
}

// indexGenericFallback uses the existing generic indexer.
func indexGenericFallback(input []byte, t *tape.Tape) error {
	pos := 0
	return parseValue(input, &pos, nil, nil, t)
}
