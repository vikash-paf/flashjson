package arena

import (
	"testing"
	"unsafe"
)

func TestArenaAlloc(t *testing.T) {
	a := New(1024)

	// Test basic allocation
	buf := a.Alloc(100)
	if len(buf) != 100 {
		t.Errorf("expected len=100, got %d", len(buf))
	}

	// Verify it's zeroed
	for i, b := range buf {
		if b != 0 {
			t.Errorf("byte %d not zeroed: %d", i, b)
		}
	}

	// Test that subsequent allocations don't overlap
	buf2 := a.Alloc(100)
	if &buf[0] == &buf2[0] {
		t.Error("allocations should not overlap")
	}

	// Verify alignment
	if uintptr(unsafe.Pointer(&buf2[0]))%8 != 0 {
		t.Error("allocation not aligned to 8 bytes")
	}
}

func TestArenaAllocNoZero(t *testing.T) {
	a := New(1024)

	// Write some data
	buf1 := a.AllocNoZero(100)
	for i := range buf1 {
		buf1[i] = 0xFF
	}

	// Reset and allocate again
	a.Reset()
	buf2 := a.AllocNoZero(100)

	// Should be the same memory (may still have old data)
	if &buf1[0] != &buf2[0] {
		t.Error("after reset, should reuse same memory")
	}
}

func TestArenaAllocString(t *testing.T) {
	a := New(1024)

	original := "Hello, World!"
	s := a.AllocString(original)

	if s != original {
		t.Errorf("expected %q, got %q", original, s)
	}

	// Verify it's using arena memory
	if a.Used() < len(original) {
		t.Error("arena should have allocated space for string")
	}
}

func TestArenaAllocBytes(t *testing.T) {
	a := New(1024)

	original := []byte{1, 2, 3, 4, 5}
	b := a.AllocBytes(original)

	if len(b) != len(original) {
		t.Errorf("expected len=%d, got %d", len(original), len(b))
	}

	for i := range original {
		if b[i] != original[i] {
			t.Errorf("byte %d: expected %d, got %d", i, original[i], b[i])
		}
	}

	// Verify it's a copy
	original[0] = 99
	if b[0] == 99 {
		t.Error("should be a copy, not sharing memory")
	}
}

func TestArenaReset(t *testing.T) {
	a := New(1024)

	a.Alloc(500)
	if a.Used() < 500 {
		t.Error("should have allocated 500 bytes")
	}

	a.Reset()
	if a.Used() != 0 {
		t.Errorf("after reset, used should be 0, got %d", a.Used())
	}

	// Can allocate again
	a.Alloc(500)
	if a.Used() < 500 {
		t.Error("should be able to allocate after reset")
	}
}

func TestArenaCapacity(t *testing.T) {
	a := New(1024)

	if a.Cap() != 1024 {
		t.Errorf("expected cap=1024, got %d", a.Cap())
	}

	if a.Available() != 1024 {
		t.Errorf("expected available=1024, got %d", a.Available())
	}

	a.Alloc(100)
	if a.Available() >= 1024 {
		t.Error("available should decrease after allocation")
	}
}

func TestArenaCanAlloc(t *testing.T) {
	a := New(1024)

	if !a.CanAlloc(1024) {
		t.Error("should be able to allocate 1024")
	}

	if a.CanAlloc(2000) {
		t.Error("should not be able to allocate 2000")
	}

	a.Alloc(900)
	if a.CanAlloc(500) {
		t.Error("should not be able to allocate 500 after using 900")
	}
}

func TestArenaOverflow(t *testing.T) {
	a := New(100)

	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic on overflow")
		}
	}()

	a.Alloc(200) // Should panic
}

func TestArenaPool(t *testing.T) {
	// Get from pool
	a1 := Get()
	if a1 == nil {
		t.Fatal("Get returned nil")
	}

	// Use it
	a1.Alloc(100)

	// Put back
	Put(a1)

	// Get again - should be reset
	a2 := Get()
	if a2.Used() != 0 {
		t.Error("pooled arena should be reset")
	}

	Put(a2)
}

func TestArenaGetSized(t *testing.T) {
	a := GetSized(100 * 1024) // 100KB

	if a.Cap() < 100*1024 {
		t.Errorf("expected cap >= 100KB, got %d", a.Cap())
	}

	Put(a)
}

func TestArenaEmptyAllocs(t *testing.T) {
	a := New(1024)

	// Empty string
	s := a.AllocString("")
	if s != "" {
		t.Error("empty string should return empty string")
	}

	// Empty bytes
	b := a.AllocBytes(nil)
	if b != nil {
		t.Error("nil bytes should return nil")
	}

	b2 := a.AllocBytes([]byte{})
	if b2 != nil {
		t.Error("empty bytes should return nil")
	}
}

// --- Benchmarks ---

func BenchmarkArenaAlloc(b *testing.B) {
	a := New(DefaultSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !a.CanAlloc(64) {
			a.Reset()
		}
		_ = a.AllocNoZero(64)
	}
}

func BenchmarkArenaAllocVsStandard(b *testing.B) {
	b.Run("Arena", func(b *testing.B) {
		a := New(DefaultSize)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !a.CanAlloc(64) {
				a.Reset()
			}
			_ = a.AllocNoZero(64)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = make([]byte, 64)
		}
	})
}

func BenchmarkArenaAllocString(b *testing.B) {
	a := New(DefaultSize)
	s := "Hello, this is a test string for benchmarking!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !a.CanAlloc(len(s)) {
			a.Reset()
		}
		_ = a.AllocString(s)
	}
}

func BenchmarkArenaPool(b *testing.B) {
	b.Run("Pool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			a := Get()
			_ = a.AllocNoZero(64)
			Put(a)
		}
	})

	b.Run("NewEachTime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			a := New(DefaultSize)
			_ = a.AllocNoZero(64)
			_ = a // prevent optimization
		}
	})
}
