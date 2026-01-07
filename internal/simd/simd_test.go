package simd

import (
	"testing"

	"github.com/vikash-paf/flashjson/internal/tape"
)

func TestCPUDetection(t *testing.T) {
	// Just make sure detection runs without panic
	t.Logf("HasAVX2: %v", HasAVX2)
	t.Logf("HasAVX512: %v", HasAVX512)
	t.Logf("HasNEON: %v", HasNEON)
	t.Logf("UsesSIMD: %v", UsesSIMD)
}

func TestIndexerSimple(t *testing.T) {
	input := []byte(`{"name":"Alice","age":30}`)

	idx := NewIndexer()
	tp := tape.New(64)

	err := idx.Index(input, tp)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	if tp.Len() == 0 {
		t.Error("tape should not be empty")
	}

	// Check first entry is ObjectStart
	e := tp.Get(0)
	if e.Type != tape.TypeObjectStart {
		t.Errorf("expected ObjectStart, got %s", tape.TypeName(e.Type))
	}
}

func TestIndexerLargeInput(t *testing.T) {
	// Create input larger than 64 bytes to exercise SIMD path
	input := []byte(`{"name":"Alice","age":30,"email":"alice@example.com","active":true}`)

	idx := NewIndexer()
	tp := tape.New(128)

	err := idx.Index(input, tp)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Verify structure
	if tp.Get(0).Type != tape.TypeObjectStart {
		t.Error("should start with ObjectStart")
	}
	if tp.Get(tp.Len()-1).Type != tape.TypeObjectEnd {
		t.Error("should end with ObjectEnd")
	}
}

func TestFindStructural(t *testing.T) {
	if !UsesSIMD {
		t.Skip("SIMD not available")
	}

	// Create 64-byte input with known structural positions
	input := make([]byte, 64)
	for i := range input {
		input[i] = 'x' // non-structural
	}
	input[0] = '{'
	input[5] = '"'
	input[10] = ':'
	input[15] = '"'
	input[20] = ','
	input[25] = '}'

	mask := findStructuralAVX2(input)

	expected := uint64(1<<0 | 1<<5 | 1<<10 | 1<<15 | 1<<20 | 1<<25)
	if mask != expected {
		t.Errorf("findStructuralAVX2: got %064b, want %064b", mask, expected)
	}
}

// --- Benchmarks ---

func BenchmarkIndexerGeneric(b *testing.B) {
	input := []byte(`{"name":"Alice","age":30,"email":"alice@example.com","active":true,"scores":[1,2,3,4,5]}`)
	idx := &Indexer{useSIMD: false}
	tp := tape.New(128)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		tp.Reset()
		_ = idx.indexGeneric(input, tp)
	}
}

func BenchmarkIndexerSIMD(b *testing.B) {
	if !UsesSIMD {
		b.Skip("SIMD not available")
	}

	input := []byte(`{"name":"Alice","age":30,"email":"alice@example.com","active":true,"scores":[1,2,3,4,5]}`)
	idx := NewIndexer()
	tp := tape.New(128)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))

	for i := 0; i < b.N; i++ {
		tp.Reset()
		_ = idx.Index(input, tp)
	}
}

func BenchmarkFindStructural(b *testing.B) {
	if !UsesSIMD {
		b.Skip("SIMD not available")
	}

	input := make([]byte, 64)
	copy(input, []byte(`{"name":"Alice","age":30,"email":"alice@example.com"}`))

	b.ResetTimer()
	b.SetBytes(64)

	for i := 0; i < b.N; i++ {
		_ = findStructuralAVX2(input)
	}
}
