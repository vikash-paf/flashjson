package tape

import (
	"testing"
)

func TestNewTape(t *testing.T) {
	tape := New(100)

	if tape.Len() != 0 {
		t.Errorf("new tape should be empty, got len=%d", tape.Len())
	}

	if tape.Cap() < 100 {
		t.Errorf("expected cap >= 100, got %d", tape.Cap())
	}
}

func TestTapeAppend(t *testing.T) {
	tape := New(10)

	tape.Append(TypeObjectStart, 0, 0)
	tape.Append(TypeKey, 1, 4)
	tape.Append(TypeString, 8, 5)
	tape.Append(TypeObjectEnd, 14, 0)

	if tape.Len() != 4 {
		t.Errorf("expected len=4, got %d", tape.Len())
	}

	// Verify entries
	e := tape.Get(0)
	if e.Type != TypeObjectStart || e.Offset != 0 {
		t.Errorf("entry 0: expected ObjectStart@0, got %s@%d", TypeName(e.Type), e.Offset)
	}

	e = tape.Get(1)
	if e.Type != TypeKey || e.Offset != 1 || e.Length != 4 {
		t.Errorf("entry 1: expected Key@1[4], got %s@%d[%d]", TypeName(e.Type), e.Offset, e.Length)
	}
}

func TestTapeGrow(t *testing.T) {
	tape := New(2) // Very small initial capacity

	// Add more entries than initial capacity
	for i := 0; i < 100; i++ {
		tape.Append(TypeNumber, i*10, 5)
	}

	if tape.Len() != 100 {
		t.Errorf("expected len=100, got %d", tape.Len())
	}

	// Verify entries are preserved
	for i := 0; i < 100; i++ {
		e := tape.Get(i)
		if e.Type != TypeNumber || int(e.Offset) != i*10 {
			t.Errorf("entry %d corrupted after grow", i)
		}
	}
}

func TestTapeReset(t *testing.T) {
	tape := New(100)

	tape.Append(TypeString, 0, 10)
	tape.Append(TypeString, 10, 10)

	if tape.Len() != 2 {
		t.Errorf("expected len=2, got %d", tape.Len())
	}

	tape.Reset()

	if tape.Len() != 0 {
		t.Errorf("after reset, expected len=0, got %d", tape.Len())
	}

	// Should be able to append again
	tape.Append(TypeNumber, 0, 5)
	if tape.Len() != 1 {
		t.Errorf("after reset and append, expected len=1, got %d", tape.Len())
	}
}

func TestTapeSkipValue(t *testing.T) {
	tape := New(100)

	// Build: {"a":1,"b":{"c":2},"d":3}
	tape.Append(TypeObjectStart, 0, 0) // 0
	tape.Append(TypeKey, 1, 1)         // 1: "a"
	tape.Append(TypeNumber, 4, 1)      // 2: 1
	tape.Append(TypeKey, 6, 1)         // 3: "b"
	tape.Append(TypeObjectStart, 9, 0) // 4: {
	tape.Append(TypeKey, 10, 1)        // 5: "c"
	tape.Append(TypeNumber, 13, 1)     // 6: 2
	tape.Append(TypeObjectEnd, 14, 0)  // 7: }
	tape.Append(TypeKey, 16, 1)        // 8: "d"
	tape.Append(TypeNumber, 19, 1)     // 9: 3
	tape.Append(TypeObjectEnd, 20, 0)  // 10: }

	tests := []struct {
		index    int
		expected int
		name     string
	}{
		{0, 11, "skip entire object"},
		{2, 3, "skip primitive"},
		{4, 8, "skip nested object"},
		{6, 7, "skip number in nested"},
	}

	for _, tc := range tests {
		got := tape.SkipValue(tc.index)
		if got != tc.expected {
			t.Errorf("%s: SkipValue(%d) = %d, expected %d", tc.name, tc.index, got, tc.expected)
		}
	}
}

func TestTapeSkipArray(t *testing.T) {
	tape := New(100)

	// Build: [1,[2,3],4]
	tape.Append(TypeArrayStart, 0, 0) // 0
	tape.Append(TypeNumber, 1, 1)     // 1: 1
	tape.Append(TypeArrayStart, 3, 0) // 2: [
	tape.Append(TypeNumber, 4, 1)     // 3: 2
	tape.Append(TypeNumber, 6, 1)     // 4: 3
	tape.Append(TypeArrayEnd, 7, 0)   // 5: ]
	tape.Append(TypeNumber, 9, 1)     // 6: 4
	tape.Append(TypeArrayEnd, 10, 0)  // 7: ]

	tests := []struct {
		index    int
		expected int
		name     string
	}{
		{0, 8, "skip entire array"},
		{1, 2, "skip number"},
		{2, 6, "skip nested array"},
	}

	for _, tc := range tests {
		got := tape.SkipValue(tc.index)
		if got != tc.expected {
			t.Errorf("%s: SkipValue(%d) = %d, expected %d", tc.name, tc.index, got, tc.expected)
		}
	}
}

func TestTapeFindKey(t *testing.T) {
	tape := New(100)

	// Build tape for: {"name":"Alice","age":30}
	// Input positions:  0123456789...
	input := []byte(`{"name":"Alice","age":30}`)

	tape.Append(TypeObjectStart, 0, 0) // 0
	tape.Append(TypeKey, 2, 4)         // 1: name (positions 2-5, without quotes)
	tape.Append(TypeString, 9, 5)      // 2: Alice (positions 9-13)
	tape.Append(TypeKey, 17, 3)        // 3: age (positions 17-19)
	tape.Append(TypeNumber, 22, 2)     // 4: 30 (positions 22-23)
	tape.Append(TypeObjectEnd, 24, 0)  // 5

	// Find "name" - should return index 2 (the String value)
	idx := tape.FindKey(0, input, "name")
	if idx != 2 {
		t.Errorf("FindKey(name) = %d, expected 2", idx)
	}

	// Find "age" - should return index 4 (the Number value)
	idx = tape.FindKey(0, input, "age")
	if idx != 4 {
		t.Errorf("FindKey(age) = %d, expected 4", idx)
	}

	// Find non-existent key
	idx = tape.FindKey(0, input, "missing")
	if idx != -1 {
		t.Errorf("FindKey(missing) = %d, expected -1", idx)
	}
}

func TestTapePool(t *testing.T) {
	// Get from pool
	tape1 := Get()
	if tape1 == nil {
		t.Fatal("Get returned nil")
	}

	tape1.Append(TypeString, 0, 5)
	Put(tape1)

	// Get again
	tape2 := Get()
	if tape2.Len() != 0 {
		t.Error("pooled tape should be reset")
	}
	Put(tape2)
}

func TestTypeFunctions(t *testing.T) {
	// Test TypeName
	if TypeName(TypeObjectStart) != "ObjectStart" {
		t.Error("TypeName(TypeObjectStart) failed")
	}
	if TypeName(TypeNull) != "Null" {
		t.Error("TypeName(TypeNull) failed")
	}
	if TypeName(255) != "Unknown" {
		t.Error("TypeName(255) should be Unknown")
	}

	// Test IsContainer
	if !IsContainer(TypeObjectStart) {
		t.Error("ObjectStart should be container")
	}
	if !IsContainer(TypeArrayStart) {
		t.Error("ArrayStart should be container")
	}
	if IsContainer(TypeString) {
		t.Error("String should not be container")
	}

	// Test IsPrimitive
	if !IsPrimitive(TypeString) {
		t.Error("String should be primitive")
	}
	if !IsPrimitive(TypeNumber) {
		t.Error("Number should be primitive")
	}
	if !IsPrimitive(TypeTrue) {
		t.Error("True should be primitive")
	}
	if IsPrimitive(TypeObjectStart) {
		t.Error("ObjectStart should not be primitive")
	}
}

// --- Benchmarks ---

func BenchmarkTapeAppend(b *testing.B) {
	tape := New(1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if tape.Len() >= 1000 {
			tape.Reset()
		}
		tape.Append(TypeString, i*10, 5)
	}
}

func BenchmarkTapeSkipValue(b *testing.B) {
	tape := New(100)

	// Build nested object
	tape.Append(TypeObjectStart, 0, 0)
	for i := 0; i < 10; i++ {
		tape.Append(TypeKey, i*20, 5)
		tape.Append(TypeString, i*20+10, 5)
	}
	tape.Append(TypeObjectEnd, 200, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tape.SkipValue(0)
	}
}
