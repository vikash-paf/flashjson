package unsafeutil

import (
	"testing"
	"unsafe"
)

func TestBytesToString(t *testing.T) {
	original := []byte("hello world")
	s := BytesToString(original)

	if s != "hello world" {
		t.Errorf("expected 'hello world', got %q", s)
	}

	// Verify zero-copy by checking that modification affects the string
	// (This is normally bad; we're just testing the zero-copy property)
	originalByte := original[0]
	original[0] = 'X'
	if s[0] != 'X' {
		t.Error("expected zero-copy (string should change with slice)")
	}
	original[0] = originalByte // restore
}

func TestStringToBytes(t *testing.T) {
	s := "hello world"
	b := StringToBytes(s)

	if string(b) != s {
		t.Errorf("expected %q, got %q", s, string(b))
	}
}

func TestBytesToStringEmpty(t *testing.T) {
	var empty []byte
	s := BytesToString(empty)
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestStringPtrOffset(t *testing.T) {
	data := []byte("0123456789")
	base := unsafe.Pointer(&data[0])

	// Extract substring at offset 3, length 4 -> "3456"
	s := StringPtrOffset(base, 3, 4)
	if s != "3456" {
		t.Errorf("expected '3456', got %q", s)
	}
}

// Benchmarks

func BenchmarkBytesToString(b *testing.B) {
	data := []byte("this is a test string for benchmarking zero-copy conversion")

	b.Run("ZeroCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = BytesToString(data)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = string(data)
		}
	})
}

func BenchmarkStringToBytes(b *testing.B) {
	s := "this is a test string for benchmarking zero-copy conversion"

	b.Run("ZeroCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = StringToBytes(s)
		}
	})

	b.Run("Standard", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = []byte(s)
		}
	})
}
