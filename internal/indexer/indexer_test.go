package indexer

import (
	"testing"

	"github.com/vikash-paf/flashjson/internal/tape"
)

func TestIndexSimpleObject(t *testing.T) {
	input := []byte(`{"name":"Alice","age":30}`)

	tp, err := Index(input)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	defer tape.Put(tp)

	// Expected: ObjectStart, Key("name"), String("Alice"), Key("age"), Number(30), ObjectEnd
	if tp.Len() != 6 {
		t.Errorf("expected 6 entries, got %d", tp.Len())
		for i := 0; i < tp.Len(); i++ {
			e := tp.Get(i)
			t.Logf("  [%d] %s offset=%d len=%d", i, tape.TypeName(e.Type), e.Offset, e.Length)
		}
		return
	}

	tests := []struct {
		idx    int
		typ    uint8
		offset int
		length int
	}{
		{0, tape.TypeObjectStart, 0, 0},
		{1, tape.TypeKey, 2, 4},     // "name" without quotes
		{2, tape.TypeString, 9, 5},  // "Alice" without quotes
		{3, tape.TypeKey, 17, 3},    // "age" without quotes
		{4, tape.TypeNumber, 22, 2}, // 30
		{5, tape.TypeObjectEnd, 24, 0},
	}

	for _, tc := range tests {
		e := tp.Get(tc.idx)
		if e.Type != tc.typ || int(e.Offset) != tc.offset || int(e.Length) != tc.length {
			t.Errorf("entry[%d]: expected %s@%d[%d], got %s@%d[%d]",
				tc.idx, tape.TypeName(tc.typ), tc.offset, tc.length,
				tape.TypeName(e.Type), e.Offset, e.Length)
		}
	}
}

func TestIndexArray(t *testing.T) {
	input := []byte(`[1,2,3]`)

	tp, err := Index(input)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	defer tape.Put(tp)

	if tp.Len() != 5 {
		t.Errorf("expected 5 entries, got %d", tp.Len())
	}

	// Check types
	expected := []uint8{
		tape.TypeArrayStart,
		tape.TypeNumber,
		tape.TypeNumber,
		tape.TypeNumber,
		tape.TypeArrayEnd,
	}

	for i, typ := range expected {
		if tp.Get(i).Type != typ {
			t.Errorf("entry[%d]: expected %s, got %s", i, tape.TypeName(typ), tape.TypeName(tp.Get(i).Type))
		}
	}
}

func TestIndexNested(t *testing.T) {
	input := []byte(`{"user":{"name":"Bob","scores":[1,2,3]}}`)

	tp, err := Index(input)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	defer tape.Put(tp)

	// Should have nested structure
	if tp.Get(0).Type != tape.TypeObjectStart {
		t.Error("should start with ObjectStart")
	}
	if tp.Get(tp.Len()-1).Type != tape.TypeObjectEnd {
		t.Error("should end with ObjectEnd")
	}

	// Find "name" key
	found := false
	for i := 0; i < tp.Len(); i++ {
		e := tp.Get(i)
		if e.Type == tape.TypeKey && int(e.Length) == 4 {
			key := string(input[e.Offset : e.Offset+e.Length])
			if key == "name" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("should find 'name' key")
	}
}

func TestIndexLiterals(t *testing.T) {
	tests := []struct {
		input string
		typ   uint8
	}{
		{`true`, tape.TypeTrue},
		{`false`, tape.TypeFalse},
		{`null`, tape.TypeNull},
	}

	for _, tc := range tests {
		tp, err := Index([]byte(tc.input))
		if err != nil {
			t.Errorf("Index(%s) failed: %v", tc.input, err)
			continue
		}

		if tp.Len() != 1 {
			t.Errorf("Index(%s): expected 1 entry, got %d", tc.input, tp.Len())
			tape.Put(tp)
			continue
		}

		if tp.Get(0).Type != tc.typ {
			t.Errorf("Index(%s): expected %s, got %s", tc.input, tape.TypeName(tc.typ), tape.TypeName(tp.Get(0).Type))
		}

		tape.Put(tp)
	}
}

func TestIndexNumbers(t *testing.T) {
	tests := []string{
		`0`,
		`123`,
		`-456`,
		`3.14`,
		`-0.5`,
		`1e10`,
		`1.5e-3`,
		`1E+10`,
	}

	for _, input := range tests {
		tp, err := Index([]byte(input))
		if err != nil {
			t.Errorf("Index(%s) failed: %v", input, err)
			continue
		}

		if tp.Len() != 1 {
			t.Errorf("Index(%s): expected 1 entry, got %d", input, tp.Len())
			tape.Put(tp)
			continue
		}

		if tp.Get(0).Type != tape.TypeNumber {
			t.Errorf("Index(%s): expected Number, got %s", input, tape.TypeName(tp.Get(0).Type))
		}

		tape.Put(tp)
	}
}

func TestIndexStrings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{`""`, ""},
		{`"with spaces"`, "with spaces"},
		{`"escaped\"quote"`, `escaped\"quote`},
		{`"unicode: \u0041"`, `unicode: \u0041`},
	}

	for _, tc := range tests {
		tp, err := Index([]byte(tc.input))
		if err != nil {
			t.Errorf("Index(%s) failed: %v", tc.input, err)
			continue
		}

		if tp.Len() != 1 {
			t.Errorf("Index(%s): expected 1 entry, got %d", tc.input, tp.Len())
			tape.Put(tp)
			continue
		}

		e := tp.Get(0)
		if e.Type != tape.TypeString {
			t.Errorf("Index(%s): expected String, got %s", tc.input, tape.TypeName(e.Type))
		}

		// Verify we can extract the content
		content := string([]byte(tc.input)[e.Offset : e.Offset+e.Length])
		if content != tc.expected {
			t.Errorf("Index(%s): expected content %q, got %q", tc.input, tc.expected, content)
		}

		tape.Put(tp)
	}
}

func TestIndexWhitespace(t *testing.T) {
	input := []byte(`  {  "key"  :  "value"  }  `)

	tp, err := Index(input)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	defer tape.Put(tp)

	if tp.Len() != 4 {
		t.Errorf("expected 4 entries, got %d", tp.Len())
	}
}

func TestIndexErrors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{``, "empty input"},
		{`{`, "unterminated object"},
		{`{"key"`, "missing colon"},
		{`{"key":}`, "missing value"},
		{`[1,]`, "trailing comma"},
		{`tru`, "incomplete true"},
		{`fals`, "incomplete false"},
		{`nul`, "incomplete null"},
		{`"unterminated`, "unterminated string"},
		{`123abc`, "trailing garbage after number"},
	}

	for _, tc := range tests {
		tp, err := Index([]byte(tc.input))
		if err == nil {
			t.Errorf("Index(%s) should fail: %s", tc.input, tc.desc)
			tape.Put(tp)
		}
	}
}

func TestIndexEmptyContainers(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{`{}`, 2},   // ObjectStart, ObjectEnd
		{`[]`, 2},   // ArrayStart, ArrayEnd
		{`[[]]`, 4}, // ArrayStart, ArrayStart, ArrayEnd, ArrayEnd
	}

	for _, tc := range tests {
		tp, err := Index([]byte(tc.input))
		if err != nil {
			t.Errorf("Index(%s) failed: %v", tc.input, err)
			continue
		}

		if tp.Len() != tc.expected {
			t.Errorf("Index(%s): expected %d entries, got %d", tc.input, tc.expected, tp.Len())
		}

		tape.Put(tp)
	}
}

// --- Benchmarks ---

func BenchmarkIndexSmall(b *testing.B) {
	input := []byte(`{"name":"Alice","age":30,"active":true}`)
	idx := New()
	tp := tape.New(64)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		tp.Reset()
		_ = idx.Index(input, tp)
	}
}

func BenchmarkIndexMedium(b *testing.B) {
	input := []byte(`{
		"users": [
			{"id": 1, "name": "Alice", "email": "alice@example.com"},
			{"id": 2, "name": "Bob", "email": "bob@example.com"},
			{"id": 3, "name": "Charlie", "email": "charlie@example.com"}
		],
		"total": 3,
		"page": 1
	}`)
	idx := New()
	tp := tape.New(128)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		tp.Reset()
		_ = idx.Index(input, tp)
	}
}

func BenchmarkIndexLarge(b *testing.B) {
	// Build a larger JSON
	input := make([]byte, 0, 10000)
	input = append(input, `{"items":[`...)
	for i := 0; i < 100; i++ {
		if i > 0 {
			input = append(input, ',')
		}
		input = append(input, `{"id":`...)
		input = append(input, []byte(string(rune('0'+i%10)))...)
		input = append(input, `,"name":"item`...)
		input = append(input, []byte(string(rune('0'+i%10)))...)
		input = append(input, `"}`...)
	}
	input = append(input, `]}`...)

	idx := New()
	tp := tape.New(1024)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		tp.Reset()
		_ = idx.Index(input, tp)
	}
}
